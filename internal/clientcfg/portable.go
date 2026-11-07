package clientcfg

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anonvector/slipgate/internal/config"
)

// GeneratePortableURI uses a standard link where the installed app engine can
// preserve its meaning. DNS and chained configurations retain the native format.
func GeneratePortableURI(t *config.TunnelConfig, b *config.BackendConfig, cfg *config.Config, opts URIOptions, format string) (string, error) {
	if t == nil {
		return "", fmt.Errorf("missing tunnel")
	}
	if t.Transport == config.TransportVLESS && t.VLESS == nil {
		return "", fmt.Errorf("missing VLESS settings")
	}
	switch format {
	case "auto", "standard", "veyra", "rovunet", "legacy":
	default:
		return "", fmt.Errorf("unknown export format")
	}
	if t.Transport == config.TransportVLESS && format != "legacy" {
		if VLESSEndpointFor(t, cfg).SelfSigned {
			return "", fmt.Errorf("Rovunet requires a trusted TLS certificate; configure a certificate front or CDN")
		}
	}
	if (format == "auto" || format == "standard") && t.Transport == config.TransportVLESS {
		return GenerateVLESSLink(t, cfg, opts.Username, opts.Password)
	}
	if format == "standard" {
		return "", fmt.Errorf("no lossless standard exporter for this transport; use auto or rovunet")
	}
	link, err := GenerateURI(t, b, cfg, opts)
	if err != nil {
		return "", err
	}
	if format == "legacy" {
		return link, nil
	}
	if format == "veyra" {
		return "veyra://" + strings.TrimPrefix(link, "slipnet://"), nil
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "slipnet://"))
	if err != nil {
		return "", err
	}
	fields := strings.Split(string(payload), "|")
	for len(fields) < TotalFieldsV32 {
		fields = append(fields, "")
	}
	fields[0] = "32"
	fields[FSoarDiscoveryList] = "soar"
	// Group identity is an explicit server/user namespace, not a protocol endpoint.
	server, _ := ServerAddress(cfg)
	identity := sha256.Sum256([]byte(server + "\x00" + opts.Username))
	fields[FCollectionID] = b64(fmt.Sprintf("panel:%x", identity[:16]))
	label := server
	if opts.Username != "" {
		label = opts.Username + " — " + server
	}
	if chars := []rune(label); len(chars) > 120 {
		label = string(chars[:120])
	}
	fields[FCollectionName] = b64(label)

	// v33: carry SOCKS/SSH credentials losslessly in one more field instead
	// of relying solely on the plain fields below, which the "|" join a few
	// lines down can't let a credential contain without desynchronizing
	// every field after it (see FCredentialsV33). Build this from the
	// fields' current values before sanitizing them in place.
	credentials, err := json.Marshal(map[string]string{
		"socksUser": fields[FSOCKSUser],
		"socksPass": fields[FSOCKSPass],
		"sshUser":   fields[FSSHUser],
		"sshPass":   fields[FSSHPass],
	})
	if err != nil {
		return "", err
	}
	for len(fields) < TotalFieldsV33 {
		fields = append(fields, "")
	}
	fields[FCredentialsV33] = base64.StdEncoding.EncodeToString(credentials)
	// The app's v33 importer overwrites these four from FCredentialsV33
	// above, so their only remaining job is to keep the "|"-delimited wire
	// string itself parseable — strip "|" the same way the app's own
	// exporter's sanitize() does, rather than leave it to corrupt the split.
	fields[FSOCKSUser] = stripFieldDelimiter(fields[FSOCKSUser])
	fields[FSOCKSPass] = stripFieldDelimiter(fields[FSOCKSPass])
	fields[FSSHUser] = stripFieldDelimiter(fields[FSSHUser])
	fields[FSSHPass] = stripFieldDelimiter(fields[FSSHPass])
	fields[0] = "33"

	return "rovunet://" + base64.StdEncoding.EncodeToString([]byte(strings.Join(fields, "|"))), nil
}

// stripFieldDelimiter removes "|" from v, matching the app's own exporter
// (ConfigExporter.kt's sanitize()) so a plain, non-lossless copy of a field
// can sit safely inside the "|"-joined wire string.
func stripFieldDelimiter(v string) string {
	return strings.ReplaceAll(v, "|", "")
}
