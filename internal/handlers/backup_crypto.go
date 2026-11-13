package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"filippo.io/age"
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/backup"
	"golang.org/x/term"
)

// ReadPrivateKeyFile refuses symlinks, shared permissions, other owners and
// oversized input. Only the file path, never its contents, enters argv.
func ReadPrivateKeyFile(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("cannot open private backup key file")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || !ok || st.Uid != uint32(os.Geteuid()) || info.Size() > 64<<10 {
		return nil, errors.New("backup key file must be an owner-only regular file owned by the current user (maximum 64 KiB)")
	}
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return nil, errors.New("cannot read bounded private backup key file")
	}
	return b, nil
}

func backupPassphrase(file string, confirm bool) (string, error) {
	if file != "" {
		b, err := ReadPrivateKeyFile(file)
		if err != nil {
			return "", err
		}
		b = bytes.TrimSuffix(bytes.TrimSuffix(b, []byte("\n")), []byte("\r"))
		if len(b) < 12 || len(b) > 4096 {
			return "", errors.New("backup passphrase must be 12 to 4096 bytes")
		}
		return string(b), nil
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil || !term.IsTerminal(int(tty.Fd())) {
		if tty != nil {
			tty.Close()
		}
		return "", errors.New("backup encryption needs a terminal, --recipient (public key), or an owner-only --passphrase-file; plaintext export requires --plaintext")
	}
	defer tty.Close()
	read := func(label string) ([]byte, error) {
		fmt.Fprint(tty, label)
		b, err := term.ReadPassword(int(tty.Fd()))
		fmt.Fprintln(tty)
		return b, err
	}
	first, err := read("Backup passphrase (hidden): ")
	if err != nil {
		return "", err
	}
	if len(first) < 12 || len(first) > 4096 {
		return "", errors.New("backup passphrase must be 12 to 4096 bytes")
	}
	if confirm {
		second, err := read("Repeat backup passphrase (hidden): ")
		if err != nil {
			return "", err
		}
		if !bytes.Equal(first, second) {
			return "", errors.New("backup passphrases do not match")
		}
	}
	return string(first), nil
}

func protectBackup(text string, ctx *actions.Context) (string, error) {
	plain := ctx.GetArg("plaintext") == "true"
	recipient, passfile := ctx.GetArg("recipient"), ctx.GetArg("passphrase-file")
	if plain {
		if recipient != "" || passfile != "" {
			return "", errors.New("--plaintext cannot be combined with encryption options")
		}
		return text, nil
	}
	if recipient != "" {
		if passfile != "" {
			return "", errors.New("choose --recipient or --passphrase-file")
		}
		r, err := age.ParseX25519Recipient(recipient)
		if err != nil {
			return "", errors.New("invalid age X25519 public recipient")
		}
		return backup.Encrypt(text, r)
	}
	pass, err := backupPassphrase(passfile, true)
	if err != nil {
		return "", err
	}
	r, err := age.NewScryptRecipient(pass)
	if err != nil {
		return "", errors.New("cannot initialize passphrase encryption")
	}
	return backup.Encrypt(text, r)
}

// DecodePrivateBackup verifies encryption before any restore mutation. Old
// Rovunet/SlipGate plaintext formats remain accepted without a key.
func DecodePrivateBackup(text, identityFile, passphraseFile string) (*backup.Bundle, error) {
	if !backup.IsEncrypted(text) {
		return backup.Decode(text)
	}
	if identityFile != "" {
		if passphraseFile != "" {
			return nil, errors.New("choose --identity-file or --passphrase-file")
		}
		raw, err := ReadPrivateKeyFile(identityFile)
		if err != nil {
			return nil, err
		}
		ids, err := age.ParseIdentities(strings.NewReader(string(raw)))
		if err != nil {
			return nil, errors.New("invalid age identity file")
		}
		if len(ids) > 32 {
			return nil, errors.New("too many backup identities")
		}
		return backup.DecodeEncrypted(text, ids...)
	}
	pass, err := backupPassphrase(passphraseFile, false)
	if err != nil {
		return nil, err
	}
	id, err := age.NewScryptIdentity(pass)
	if err != nil {
		return nil, errors.New("cannot initialize backup decryption")
	}
	// A hostile header must not force unbounded CPU/memory before authentication.
	id.SetMaxWorkFactor(18)
	return backup.DecodeEncrypted(text, id)
}
