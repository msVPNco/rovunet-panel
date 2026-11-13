// signing-vault is an offline Windows operator tool, separate from the server.
// Production bytes use current-user DPAPI; portable recovery uses age.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	"github.com/msVPNco/rovunet-panel/internal/releaseauth"
)

const purpose = "Rovunet.Panel.ProductionSigning.v1"

type publicIdentity struct {
	Schema    int    `json:"schema"`
	ID        string `json:"id"`
	SPKI      string `json:"spki"`
	Purpose   string `json:"purpose"`
	Created   string `json:"created_utc"`
	Recipient string `json:"recovery_recipient"`
}

func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("refusing to overwrite an existing vault/recovery file")
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func readBounded(path string, max int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > max {
		return nil, errors.New("vault input is missing, non-regular or oversized")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(data)) > max {
		return nil, errors.New("cannot read bounded vault input")
	}
	return data, nil
}

func identityFor(key ed25519.PrivateKey, recipient string) (publicIdentity, error) {
	der, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return publicIdentity{}, err
	}
	sum := sha256.Sum256(der)
	return publicIdentity{Schema: 1, ID: hex.EncodeToString(sum[:]), SPKI: base64.StdEncoding.EncodeToString(der),
		Purpose: "production", Created: time.Now().UTC().Format(time.RFC3339), Recipient: recipient}, nil
}

func parsePrivate(der []byte) (ed25519.PrivateKey, error) {
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, errors.New("invalid recovered signing identity")
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("recovered signing identity is not Ed25519")
	}
	return key, nil
}

func loadPublic(vault string) (publicIdentity, error) {
	data, err := readBounded(filepath.Join(vault, "identity-public.json"), 16<<10)
	if err != nil {
		return publicIdentity{}, err
	}
	var pub publicIdentity
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&pub) != nil || d.Decode(new(any)) != io.EOF || pub.Schema != 1 || pub.Purpose != "production" {
		return pub, errors.New("invalid vault public metadata")
	}
	return pub, nil
}

func loadPrivate(vault string) (ed25519.PrivateKey, []byte, publicIdentity, error) {
	pub, err := loadPublic(vault)
	if err != nil {
		return nil, nil, pub, err
	}
	sealed, err := readBounded(filepath.Join(vault, "production.dpapi"), 1<<20)
	if err != nil {
		return nil, nil, pub, err
	}
	der, err := unseal(sealed, []byte(purpose))
	if err != nil {
		return nil, nil, pub, errors.New("current Windows account cannot unlock this production vault")
	}
	key, err := parsePrivate(der)
	if err != nil {
		clear(der)
		return nil, nil, pub, err
	}
	actual, err := identityFor(key, pub.Recipient)
	if err != nil || actual.ID != pub.ID || actual.SPKI != pub.SPKI {
		clear(der)
		clear(key)
		return nil, nil, pub, errors.New("sealed key/public fingerprint mismatch")
	}
	return key, der, pub, nil
}

func recoverDER(recovery string) ([]byte, publicIdentity, error) {
	pub, err := loadPublic(recovery)
	if err != nil {
		return nil, pub, err
	}
	secret, err := readBounded(filepath.Join(recovery, "recovery.agekey"), 16<<10)
	if err != nil {
		return nil, pub, err
	}
	defer clear(secret)
	identities, err := age.ParseIdentities(bytes.NewReader(secret))
	if err != nil {
		return nil, pub, errors.New("invalid recovery identity")
	}
	cipher, err := readBounded(filepath.Join(recovery, "production-backup.age"), 64<<10)
	if err != nil {
		return nil, pub, err
	}
	reader, err := age.Decrypt(armor.NewReader(bytes.NewReader(cipher)), identities...)
	if err != nil {
		return nil, pub, errors.New("recovery decryption failed")
	}
	der, err := io.ReadAll(io.LimitReader(reader, (16<<10)+1))
	if err != nil || len(der) > 16<<10 {
		return nil, pub, errors.New("recovery authentication failed")
	}
	key, err := parsePrivate(der)
	if err != nil {
		clear(der)
		return nil, pub, err
	}
	defer clear(key)
	actual, err := identityFor(key, pub.Recipient)
	if err != nil || actual.ID != pub.ID || actual.SPKI != pub.SPKI {
		clear(der)
		return nil, pub, errors.New("recovered identity does not match independently recorded fingerprint")
	}
	return der, pub, nil
}

func initialize(vault, recovery string) (publicIdentity, error) {
	var none publicIdentity
	for _, p := range []string{filepath.Join(vault, "production.dpapi"), filepath.Join(vault, "identity-public.json"),
		filepath.Join(recovery, "recovery.agekey"), filepath.Join(recovery, "production-backup.age"), filepath.Join(recovery, "identity-public.json")} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			return none, errors.New("vault/recovery destination already exists; no identity rotated")
		}
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return none, err
	}
	defer clear(private)
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return none, err
	}
	defer clear(der)
	recoveryKey, err := age.GenerateX25519Identity()
	if err != nil {
		return none, err
	}
	pub, err := identityFor(private, recoveryKey.Recipient().String())
	if err != nil {
		return none, err
	}
	sealed, err := seal(der, []byte(purpose))
	if err != nil {
		return none, err
	}
	var cipher bytes.Buffer
	armored := armor.NewWriter(&cipher)
	encrypt, err := age.Encrypt(armored, recoveryKey.Recipient())
	if err != nil {
		return none, err
	}
	if _, err = encrypt.Write(der); err != nil {
		return none, err
	}
	if err = encrypt.Close(); err != nil {
		return none, err
	}
	if err = armored.Close(); err != nil {
		return none, err
	}
	// Portable recovery secret: private Windows ACL, later copied offline.
	secret := []byte(recoveryKey.String() + "\n")
	defer clear(secret)
	if err = writeNew(filepath.Join(recovery, "recovery.agekey"), secret); err != nil {
		return none, err
	}
	if err = writeNew(filepath.Join(recovery, "production-backup.age"), cipher.Bytes()); err != nil {
		return none, err
	}
	if err = writeNew(filepath.Join(vault, "production.dpapi"), sealed); err != nil {
		return none, err
	}
	data, _ := json.MarshalIndent(pub, "", "  ")
	data = append(data, '\n')
	if err = writeNew(filepath.Join(recovery, "identity-public.json"), data); err != nil {
		return none, err
	}
	if err = writeNew(filepath.Join(vault, "identity-public.json"), data); err != nil {
		return none, err
	}
	return pub, nil
}

func check(vault, recovery string) (publicIdentity, error) {
	key, der, pub, err := loadPrivate(vault)
	if err != nil {
		return pub, err
	}
	defer clear(key)
	defer clear(der)
	recovered, _, err := recoverDER(recovery)
	if err != nil {
		return pub, err
	}
	defer clear(recovered)
	if !bytes.Equal(der, recovered) {
		return pub, errors.New("recovery differs from the sealed production identity")
	}
	challenge := make([]byte, 64)
	if _, err := rand.Read(challenge); err != nil {
		return pub, err
	}
	if !ed25519.Verify(key.Public().(ed25519.PublicKey), challenge, ed25519.Sign(key, challenge)) {
		return pub, errors.New("recovery challenge signature failed")
	}
	return pub, nil
}

func recoverVault(vault, recovery string) (publicIdentity, error) {
	der, pub, err := recoverDER(recovery)
	if err != nil {
		return pub, err
	}
	defer clear(der)
	sealed, err := seal(der, []byte(purpose))
	if err != nil {
		return pub, err
	}
	if err = writeNew(filepath.Join(vault, "production.dpapi"), sealed); err != nil {
		return pub, err
	}
	data, _ := json.MarshalIndent(pub, "", "  ")
	err = writeNew(filepath.Join(vault, "identity-public.json"), append(data, '\n'))
	return pub, err
}

func signManifest(vault, manifest, signature, tag string) (publicIdentity, error) {
	key, der, pub, err := loadPrivate(vault)
	if err != nil {
		return pub, err
	}
	defer clear(key)
	defer clear(der)
	raw, err := readBounded(manifest, releaseauth.MaxManifestSize)
	if err != nil {
		return pub, err
	}
	ring, _ := json.Marshal(releaseauth.Keyring{Schema: 1, Keys: []releaseauth.Key{{ID: pub.ID, SPKI: pub.SPKI, Purpose: "production"}}})
	verifier, err := releaseauth.NewVerifier(ring, "production")
	if err != nil {
		return pub, err
	}
	sig := ed25519.Sign(key, raw)
	if _, err = verifier.Verify(raw, sig, "msVPNco/rovunet-panel", tag); err != nil {
		return pub, errors.New("refused signing outside the production manifest/repository/tag contract")
	}
	if err = writeNew(signature, sig); err != nil {
		return pub, err
	}
	return pub, nil
}

func run() error {
	if runtime.GOOS != "windows" {
		return errors.New("the production vault uses Windows current-user DPAPI")
	}
	if len(os.Args) < 2 {
		return errors.New("usage: signing-vault init|check|recover|sign --vault PATH [--recovery PATH]")
	}
	fs := flag.NewFlagSet("signing-vault", flag.ContinueOnError)
	vault := fs.String("vault", "", "Existing private Windows vault folder")
	recovery := fs.String("recovery", "", "Existing private portable recovery folder")
	manifest := fs.String("manifest", "", "Production manifest to sign")
	signature := fs.String("signature", "", "Detached signature output")
	tag := fs.String("tag", "", "Exact expected tag")
	expected := fs.String("expected-key-id", "", "Independently retained public fingerprint; required except init")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *vault == "" || fs.NArg() != 0 {
		return errors.New("a private vault directory is required")
	}
	if os.Args[1] != "init" {
		id, err := hex.DecodeString(*expected)
		if err != nil || len(id) != 32 || hex.EncodeToString(id) != *expected {
			return errors.New("an independently retained expected public fingerprint is required")
		}
		location := *vault
		if os.Args[1] == "recover" {
			location = *recovery
		}
		pub, err := loadPublic(location)
		if err != nil || pub.ID != *expected {
			return errors.New("vault/recovery does not match the independently retained expected fingerprint")
		}
	}
	var pub publicIdentity
	var err error
	switch os.Args[1] {
	case "init":
		if *recovery == "" {
			return errors.New("private recovery directory required")
		}
		pub, err = initialize(*vault, *recovery)
	case "check":
		pub, err = check(*vault, *recovery)
	case "recover":
		pub, err = recoverVault(*vault, *recovery)
	case "sign":
		if *manifest == "" || *signature == "" || *tag == "" {
			return errors.New("manifest/signature/tag required")
		}
		pub, err = signManifest(*vault, *manifest, *signature, *tag)
	default:
		return errors.New("unknown vault operation")
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"success": true, "operation": os.Args[1], "key_id": pub.ID, "spki": pub.SPKI, "purpose": pub.Purpose, "recovery_recipient": pub.Recipient})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Signing vault operation stopped:", err)
		os.Exit(1)
	}
}
