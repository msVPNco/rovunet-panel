package coreplan

import (
	"fmt"
	"sort"
)

// NativeQuotaBridge is an internal hop to the panel's durable TCP budget relay.
// It never changes the public inbound, user credential or subscription item ID.
type NativeQuotaBridge struct {
	Port       int    `json:"port"`
	Path       string `json:"path"`
	SecretFile string `json:"secretFile"`
	Directory  string `json:"directory"`
	Store      string `json:"store"`
}

func BridgeProtocol(p string) bool {
	return p == "reality" || p == "xhttp" || p == "tlsvision" || p == "xhttp-reality"
}
func applyQuotaBridge(root object, o Options) error {
	if o.QuotaBridge == nil {
		return nil
	}
	b := o.QuotaBridge
	if !BridgeProtocol(o.Protocol) || b.Port < 1 || b.Port > 65535 || b.Path == "" || b.Path[0] != '/' {
		return fmt.Errorf("invalid native quota bridge")
	}
	outs, _ := root["outbounds"].([]object)
	routing, _ := root["routing"].(object)
	if routing == nil {
		routing = object{}
	}
	rules, _ := routing["rules"].([]object)
	names := make([]string, 0, len(o.QuotaClients))
	for user := range o.QuotaClients {
		names = append(names, user)
	}
	sort.Strings(names)
	for _, user := range names {
		id := o.QuotaClients[user]
		// The outbound is intentionally TCP-only: native SQL UDP accounting is not admitted.
		tag := "quota-" + user
		rules = append(rules, object{"type": "field", "user": []string{user}, "outboundTag": tag})
		outs = append(outs, object{"tag": tag, "protocol": "vless", "settings": object{"vnext": []object{{"address": "127.0.0.1", "port": b.Port, "users": []object{{"id": id, "encryption": "none"}}}}}, "streamSettings": object{"network": "ws", "security": "none", "wsSettings": object{"path": b.Path}}})
	}
	routing["rules"] = rules
	root["routing"] = routing
	root["outbounds"] = outs
	return nil
}
