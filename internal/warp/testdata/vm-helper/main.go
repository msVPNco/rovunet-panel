// vm-helper exercises the real WARP setup only in an owner-created lab VM.
// It never prints account identities, tokens, keys or the egress address.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/msVPNco/rovunet-panel/internal/warp"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

func run() int {
	marker, err := os.ReadFile("/etc/rovunet-lab-authorized")
	if err != nil || !strings.Contains(string(marker), "Owned disposable Rovunet validation VM") || os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "This helper requires the explicitly identified disposable root lab.")
		return 1
	}
	if len(os.Args) == 2 && os.Args[1] == "disable" {
		if err := warp.Disable(); err != nil {
			fmt.Fprintln(os.Stderr, "Lab WARP cleanup failed")
			return 1
		}
		fmt.Println("Owned VM WARP disabled")
		return 0
	}
	result := map[string]any{"scope": "Owned full VM; real provider registration, WireGuard and scoped SOCKS routing", "provider_payload_verified": false}
	emit := func() { json.NewEncoder(os.Stdout).Encode(result) }
	cfg, err := config.Load()
	if err != nil {
		result["stage"] = "config"
		emit()
		return 1
	}
	if err := warp.Setup(cfg, func(string) {}); err != nil {
		result["stage"] = "setup"
		result["error_class"] = "provider_or_setup_unavailable"
		prefix := strings.SplitN(err.Error(), ":", 2)[0]
		switch prefix {
		case "register WARP", "install wireguard-tools", "create socks user", "create naive user", "set naive capability", "generate wg config", "create service":
			result["failed_setup_step"] = prefix
		}
		if status := regexp.MustCompile(`WARP API returned ([0-9]{3}):`).FindStringSubmatch(err.Error()); len(status) == 2 {
			result["provider_http_status"] = status[1]
		}
		emit()
		return 2
	}
	defer warp.Disable()
	if err := warp.Enable(); err != nil {
		result["stage"] = "interface_enable"
		emit()
		return 2
	}
	result["interface_enabled"] = true
	proxy.RunAsUser = warp.SocksUser
	if err := proxy.SetupSOCKSWithUsers(config.UnmeteredUsers(cfg.Users)); err != nil {
		result["stage"] = "scoped_proxy"
		emit()
		return 2
	}
	if len(cfg.Users) == 0 {
		result["stage"] = "no_lab_account"
		emit()
		return 2
	}
	u := cfg.Users[0]
	endpoint := &url.URL{Scheme: "socks5h", Host: "127.0.0.1:1080", User: url.UserPassword(u.Username, u.Password)}
	tr := &http.Transport{Proxy: http.ProxyURL(endpoint)}
	client := &http.Client{Timeout: 20 * time.Second, Transport: tr}
	var body []byte
	for i := 0; i < 3; i++ {
		resp, err := client.Get("https://www.cloudflare.com/cdn-cgi/trace")
		if err == nil {
			body, _ = io.ReadAll(io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
			if strings.Contains(string(body), "\nwarp=on") {
				break
			}
		}
		time.Sleep(time.Second)
	}
	result["provider_payload_verified"] = strings.Contains(string(body), "\nwarp=on")
	result["stage"] = "payload"
	emit()
	if result["provider_payload_verified"] != true {
		return 2
	}
	return 0
}

func main() { os.Exit(run()) }
