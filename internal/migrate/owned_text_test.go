package migrate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPathMigrationCannotRewriteCredentialsOrFeedLinks(t *testing.T) {
	secret := "/etc/" + legacy + "/secret-password"
	m := map[string]any{"users": []any{map[string]any{"password": secret, "key": secret}}, "tunnels": []any{map[string]any{"domain": legacy + "-example.test", "dnstt": map[string]any{"private_key": "/etc/" + legacy + "/tunnels/a/server.key"}, "naive": map[string]any{"password": secret}}}, "items": []any{map[string]any{"link": "rovunet://" + secret}}, "unknown": secret}
	raw, _ := json.Marshal(m)
	next, err := rewriteOwnedText("config.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(next, &got)
	u := got["users"].([]any)[0].(map[string]any)
	if u["password"] != secret || u["key"] != secret || got["unknown"] != secret {
		t.Fatal("credential or unknown metadata changed")
	}
	tun := got["tunnels"].([]any)[0].(map[string]any)
	if tun["naive"].(map[string]any)["password"] != secret || tun["domain"] != legacy+"-example.test" {
		t.Fatal("tunnel credentials/domain changed")
	}
	if tun["dnstt"].(map[string]any)["private_key"] != "/etc/rovunet/tunnels/a/server.key" {
		t.Fatal("owned key path not migrated")
	}
	if got["items"].([]any)[0].(map[string]any)["link"] != "rovunet://"+secret {
		t.Fatal("published feed link changed")
	}
	caddy := "root * /etc/" + legacy + "/subscriptions\n tls /etc/" + legacy + "/a.pem /etc/" + legacy + "/a.key\n basic_auth alice \"" + secret + "\"\n"
	out, _ := rewriteOwnedText("Caddyfile", []byte(caddy))
	if !strings.Contains(string(out), "basic_auth alice \""+secret+"\"") || !strings.Contains(string(out), "root * /etc/rovunet/subscriptions") {
		t.Fatal("Caddy migration altered authentication")
	}
}
