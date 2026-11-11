package backup

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func coreTLSName(name string) bool {
	return fileNameRegex.MatchString(name) && strings.HasPrefix(name, "tls-") && strings.HasSuffix(name, ".pem")
}

func walkCoreJSON(value any, change func(string, string) (string, error)) error {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if text, ok := v[k].(string); ok && (k == "certificateFile" || k == "keyFile" || k == "cert" || k == "key") {
				text, err := change(k, text)
				if err != nil {
					return err
				}
				v[k] = text
			} else if err := walkCoreJSON(v[k], change); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := walkCoreJSON(item, change); err != nil {
				return err
			}
		}
	}
	return nil
}

func captureCoreTLS(files map[string]File, dir string) error {
	paths, sources := map[string]string{}, map[string]string{}
	prior := map[string]string{}
	if f, ok := files["tls-sources.json"]; ok {
		_ = json.Unmarshal(f.Data, &prior)
	}
	change := func(kind, path string) (string, error) {
		if path == "" {
			return path, nil
		}
		if found := paths[path]; found != "" {
			return found, nil
		}
		if coreTLSName(path) {
			if _, ok := files[path]; ok {
				return path, nil
			}
		}
		original := path
		if !filepath.IsAbs(path) {
			original = filepath.Join(dir, path)
		}
		f, err := readFile(original)
		if err != nil {
			return "", fmt.Errorf("core TLS backup: %w", err)
		}
		base := "tls-cert.pem"
		if kind == "key" || kind == "keyFile" {
			base = "tls-key.pem"
		}
		name := freeName(files, base)
		files[name] = f
		paths[path] = name
		if source := prior[filepath.Base(path)]; source != "" && filepath.Dir(path) == dir {
			sources[name] = source
		} else {
			sources[name] = original
		}
		return name, nil
	}
	// Server first discovers the certificate paths; options and the applied
	// snapshot then receive the same private, self-contained references.
	for _, name := range []string{"server.json", "options.json", "applied.json"} {
		f, ok := files[name]
		if !ok {
			continue
		}
		var v any
		if json.Unmarshal(f.Data, &v) != nil {
			continue
		}
		if err := walkCoreJSON(v, change); err != nil {
			return err
		}
		f.Data, _ = json.Marshal(v)
		files[name] = f
	}
	if len(sources) > 0 {
		b, _ := json.Marshal(sources)
		files["tls-sources.json"] = File{Mode: 0600, Data: b}
	}
	return nil
}

func restoreCoreTLS(files map[string]File, dir string) error {
	for _, name := range []string{"server.json", "options.json", "applied.json"} {
		f, ok := files[name]
		if !ok {
			continue
		}
		var v any
		if json.Unmarshal(f.Data, &v) != nil {
			continue
		}
		if err := walkCoreJSON(v, func(kind, path string) (string, error) {
			name := filepath.Base(path)
			if coreTLSName(name) {
				if _, ok := files[name]; !ok {
					return "", fmt.Errorf("core TLS file missing")
				}
				return filepath.Join(dir, name), nil
			}
			return path, nil
		}); err != nil {
			return err
		}
		f.Data, _ = json.Marshal(v)
		files[name] = f
	}
	return nil
}
