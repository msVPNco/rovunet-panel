package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// A password or URI may contain the old directory string. Only known path
// fields and explicit Caddy filesystem directives may change.
var ownedPathFields = map[string]bool{
	"private_key": true, "secret_file": true, "cert": true, "key": true,
	"certificateFile": true, "keyFile": true, "certificate_file": true, "key_file": true,
	"config_file": true, "config_dir": true, "api_socket": true,
	"certificate_path": true, "cert_path": true, "key_path": true,
}

func renamedPath(s string) string {
	old := "/etc/" + legacy
	if s == old || strings.HasPrefix(s, old+"/") {
		return "/etc/rovunet" + s[len(old):]
	}
	return s
}

func renameJSONPaths(raw []byte) ([]byte, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if d.Decode(&value) != nil {
		return nil, fmt.Errorf("invalid owned JSON; migration did not rewrite it")
	}
	changed := false
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, x := range v {
				if k == "users" || k == "archived" || k == "clients" || k == "items" || k == "settings" {
					continue
				}
				if text, ok := x.(string); ok && ownedPathFields[k] {
					if next := renamedPath(text); next != text {
						v[k] = next
						changed = true
					}
				} else {
					walk(x)
				}
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(value)
	if !changed {
		return raw, nil
	}
	return json.Marshal(value)
}

var caddyToken = regexp.MustCompile(`"(?:\\.|[^"\\])*"|[^\s]+`)

func renameCaddyPaths(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		parts := caddyToken.FindAllStringIndex(line, -1)
		if len(parts) == 0 {
			continue
		}
		directive := line[parts[0][0]:parts[0][1]]
		if directive != "root" && directive != "tls" && directive != "import" {
			continue
		}
		for n := len(parts) - 1; n >= 1; n-- {
			p := parts[n]
			token := line[p[0]:p[1]]
			text := strings.Trim(token, `"`)
			next := renamedPath(text)
			if text == next {
				continue
			}
			if strings.HasPrefix(token, `"`) {
				next = `"` + next + `"`
			}
			line = line[:p[0]] + next + line[p[1]:]
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func rewriteOwnedText(path string, raw []byte) ([]byte, error) {
	switch {
	case strings.HasSuffix(path, ".json"):
		return renameJSONPaths(raw)
	case filepath.Base(path) == "Caddyfile":
		return []byte(renameCaddyPaths(string(raw))), nil
	case filepath.Base(path) == "wg0.conf":
		s := string(raw)
		for _, suffix := range []string{"-socks", "-naive"} {
			s = strings.ReplaceAll(s, "id -u "+legacy+suffix, "id -u rovunet"+suffix)
		}
		return []byte(s), nil
	}
	return raw, nil
}
