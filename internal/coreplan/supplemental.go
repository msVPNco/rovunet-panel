package coreplan

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// Supplemental lists six additional configurations, not six distinct protocols.
func Supplemental(p string) bool {
	switch p {
	case "anytls", "tuic", "mieru-tcp", "mieru-udp", "tlsvision", "xhttp-reality":
		return true
	}
	return false
}
func SupplementalRuntime(p string) string {
	switch p {
	case "anytls", "tuic":
		return "sing-box"
	case "mieru-tcp", "mieru-udp":
		return "mita"
	}
	return ""
}
func generateSupplemental(o Options, st *State, clients []Client) (Bundle, error) {
	var users []object
	links := make([]string, 0, len(clients))
	var reality object
	if o.Protocol == "xhttp-reality" {
		raw, e := base64.StdEncoding.DecodeString(st.ServerPrivate)
		if e != nil {
			return Bundle{}, e
		}
		key, e := ecdh.X25519().NewPrivateKey(raw)
		if e != nil {
			return Bundle{}, e
		}
		reality = object{"target": o.RealityTarget, "serverNames": []string{o.SNI}, "privateKey": base64.RawURLEncoding.EncodeToString(raw), "shortIds": []string{st.ShortID}}
		_ = key
	}
	for _, c := range clients {
		b, e := st.derive(o.Protocol, c, 32)
		if e != nil {
			return Bundle{}, e
		}
		password := base64.RawURLEncoding.EncodeToString(b)
		uuidBytes := append([]byte(nil), b[:16]...)
		uuidBytes[6] = uuidBytes[6]&15 | 64
		uuidBytes[8] = uuidBytes[8]&63 | 128
		h := hex.EncodeToString(uuidBytes)
		id := fmt.Sprintf("%s-%s-%s-%s-%s", h[:8], h[8:12], h[12:16], h[16:20], h[20:])
		q := url.Values{}
		u := url.URL{Host: net.JoinHostPort(o.Host, strconv.Itoa(o.Port)), Fragment: "Rovunet-" + o.linkLabel() + "-" + c.Name}
		switch o.Protocol {
		case "anytls":
			users = append(users, object{"name": c.Name, "password": password})
			u.Scheme = "anytls"
			u.User = url.User(password)
			q.Set("sni", o.SNI)
		case "tuic":
			users = append(users, object{"name": c.Name, "uuid": id, "password": password})
			u.Scheme = "tuic"
			u.User = url.UserPassword(id, password)
			q.Set("sni", o.SNI)
			q.Set("alpn", "h3")
			q.Set("congestion_control", "bbr")
			q.Set("udp_relay_mode", "native")
		case "mieru-tcp", "mieru-udp":
			users = append(users, object{"name": c.Name, "password": password})
			u.Scheme = "mierus"
			u.User = url.UserPassword(c.Name, password)
			u.Host = o.Host
			if net.ParseIP(o.Host) != nil && net.ParseIP(o.Host).To4() == nil {
				u.Host = "[" + o.Host + "]"
			}
			q.Set("profile", o.linkLabel())
			q.Set("port", strconv.Itoa(o.Port))
			q.Set("protocol", "TCP")
			if o.Protocol == "mieru-udp" {
				q.Set("protocol", "UDP")
			}
		case "tlsvision", "xhttp-reality":
			user := object{"id": id, "email": c.Name}
			u.Scheme = "vless"
			u.User = url.User(id)
			q.Set("encryption", "none")
			q.Set("sni", o.SNI)
			q.Set("fp", "chrome")
			if o.Protocol == "tlsvision" {
				user["flow"] = "xtls-rprx-vision"
				q.Set("flow", "xtls-rprx-vision")
				q.Set("type", "tcp")
				q.Set("security", "tls")
			} else {
				q.Set("type", "xhttp")
				q.Set("security", "reality")
				q.Set("path", st.Path)
				q.Set("mode", "auto")
				q.Set("sid", st.ShortID)
				raw, _ := base64.StdEncoding.DecodeString(st.ServerPrivate)
				key, _ := ecdh.X25519().NewPrivateKey(raw)
				q.Set("pbk", base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()))
			}
			users = append(users, user)
		}
		u.RawQuery = q.Encode()
		links = append(links, u.String())
	}
	var root object
	switch o.Protocol {
	case "anytls", "tuic":
		tls := object{"enabled": true, "certificate_path": o.Cert, "key_path": o.Key}
		inbound := object{"type": o.Protocol, "tag": "proxy", "listen": "0.0.0.0", "listen_port": o.Port, "users": users, "tls": tls}
		if o.Protocol == "tuic" {
			tls["alpn"] = []string{"h3"}
			inbound["congestion_control"] = "bbr"
			inbound["zero_rtt_handshake"] = false
		}
		root = object{"log": object{"level": "warn"}, "inbounds": []object{inbound}, "outbounds": []object{{"type": "direct", "tag": "direct"}}, "route": object{"rules": []object{{"ip_is_private": true, "action": "reject"}}, "final": "direct"}}
	case "mieru-tcp", "mieru-udp":
		transport := "TCP"
		if o.Protocol == "mieru-udp" {
			transport = "UDP"
		}
		root = object{"portBindings": []object{{"port": o.Port, "protocol": transport}}, "users": users, "loggingLevel": "WARN", "mtu": 1400}
	case "tlsvision", "xhttp-reality":
		stream := object{"network": "raw", "security": "tls", "tlsSettings": object{"alpn": []string{"h2", "http/1.1"}, "certificates": []object{{"certificateFile": o.Cert, "keyFile": o.Key}}}}
		if o.Protocol == "xhttp-reality" {
			stream = object{"network": "xhttp", "security": "reality", "realitySettings": reality, "xhttpSettings": object{"path": st.Path, "mode": "auto"}}
		}
		root = object{"log": object{"loglevel": "warning"}, "inbounds": []object{{"listen": "0.0.0.0", "port": o.Port, "tag": "proxy", "protocol": "vless", "settings": object{"clients": users, "decryption": "none"}, "streamSettings": stream}}, "outbounds": []object{{"protocol": "freedom"}}}
	}
	if o.QuotaBridge != nil && st.APISocket != "" {
		root["api"] = object{"tag": "api", "listen": st.APISocket, "services": []string{"StatsService", "HandlerService"}}
		root["stats"] = object{}
		root["policy"] = object{"levels": object{"0": object{"statsUserUplink": true, "statsUserDownlink": true}}}
	}
	if e := applyQuotaBridge(root, o); e != nil {
		return Bundle{}, e
	}
	data, e := json.MarshalIndent(root, "", "  ")
	return Bundle{Server: data, Clients: links, Extension: "txt"}, e
}
