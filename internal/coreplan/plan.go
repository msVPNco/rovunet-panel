// Package coreplan produces reviewable Xray server configs and portable client
// profiles. It never installs services or opens firewall ports.
package coreplan

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

type Options struct {
	Protocol, Host, SNI, Cert, Key, RealityTarget string
	Port, Devices                                 int
}
type Bundle struct {
	Server    json.RawMessage
	Clients   []string
	Extension string
}
type object = map[string]any

func token(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func keypair() (string, string, error) {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(k.Bytes()), base64.StdEncoding.EncodeToString(k.PublicKey().Bytes()), nil
}
func Generate(o Options) (Bundle, error) {
	if o.Port < 1 || o.Port > 65535 || o.Devices < 1 || o.Devices > 1000 || o.Host == "" {
		return Bundle{}, fmt.Errorf("host, port 1..65535 and devices 1..1000 required")
	}
	if _, err := url.ParseRequestURI("https://" + net.JoinHostPort(o.Host, strconv.Itoa(o.Port))); err != nil {
		return Bundle{}, fmt.Errorf("invalid host")
	}
	switch o.Protocol {
	case "reality", "xhttp", "hysteria2", "wireguard":
	default:
		return Bundle{}, fmt.Errorf("unsupported protocol")
	}
	if o.SNI == "" {
		o.SNI = o.Host
	}
	if (o.Protocol == "xhttp" || o.Protocol == "hysteria2") && (o.Cert == "" || o.Key == "") {
		return Bundle{}, fmt.Errorf("TLS certificate and key paths required")
	}
	if o.Protocol == "reality" {
		if _, _, err := net.SplitHostPort(o.RealityTarget); err != nil {
			return Bundle{}, fmt.Errorf("REALITY target must be host:port")
		}
	}
	inbound := object{"listen": "0.0.0.0", "port": o.Port}
	stream := object{}
	users := []object{}
	clients := []string{}
	extension := "txt"
	var serverPrivate, serverPublic string
	if o.Protocol == "reality" || o.Protocol == "wireguard" {
		var err error
		serverPrivate, serverPublic, err = keypair()
		if err != nil {
			return Bundle{}, err
		}
	}
	sid, err := token(8)
	if err != nil {
		return Bundle{}, err
	}
	pathToken, err := token(12)
	if err != nil {
		return Bundle{}, err
	}
	path := "/" + pathToken
	peers := []object{}
	for i := 0; i < o.Devices; i++ {
		name := fmt.Sprintf("Rovunet-%s-device-%03d", o.Protocol, i+1)
		q := url.Values{}
		var scheme, credential string
		switch o.Protocol {
		case "reality", "xhttp":
			raw, err := token(16)
			if err != nil {
				return Bundle{}, err
			}
			id := raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
			u := object{"id": id}
			q.Set("encryption", "none")
			q.Set("sni", o.SNI)
			q.Set("fp", "chrome")
			if o.Protocol == "reality" {
				u["flow"] = "xtls-rprx-vision"
				q.Set("flow", "xtls-rprx-vision")
				q.Set("type", "tcp")
				q.Set("security", "reality")
				q.Set("pbk", base64.RawURLEncoding.EncodeToString(mustDecode(serverPublic)))
				q.Set("sid", sid)
			} else {
				q.Set("type", "xhttp")
				q.Set("security", "tls")
				q.Set("path", path)
				q.Set("host", o.SNI)
				q.Set("mode", "auto")
			}
			users = append(users, u)
			scheme = "vless"
			credential = id
		case "hysteria2":
			credential, err = token(24)
			if err != nil {
				return Bundle{}, err
			}
			users = append(users, object{"auth": credential})
			scheme = "hysteria2"
			q.Set("sni", o.SNI)
		case "wireguard":
			private, public, err := keypair()
			if err != nil {
				return Bundle{}, err
			}
			address := fmt.Sprintf("10.77.%d.%d", (i+2)/256, (i+2)%256)
			peers = append(peers, object{"publicKey": public, "allowedIPs": []string{address + "/32"}})
			clients = append(clients, fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s/32\nDNS = 1.1.1.1\nMTU = 1280\n\n[Peer]\nPublicKey = %s\nEndpoint = %s\nAllowedIPs = 0.0.0.0/0\nPersistentKeepalive = 25\n", private, address, serverPublic, net.JoinHostPort(o.Host, strconv.Itoa(o.Port))))
			continue
		}
		link := url.URL{Scheme: scheme, User: url.User(credential), Host: net.JoinHostPort(o.Host, strconv.Itoa(o.Port)), RawQuery: q.Encode(), Fragment: name}
		clients = append(clients, link.String())
	}
	switch o.Protocol {
	case "reality":
		inbound["protocol"] = "vless"
		inbound["settings"] = object{"clients": users, "decryption": "none"}
		stream = object{"network": "raw", "security": "reality", "realitySettings": object{"target": o.RealityTarget, "serverNames": []string{o.SNI}, "privateKey": base64.RawURLEncoding.EncodeToString(mustDecode(serverPrivate)), "shortIds": []string{sid}}}
	case "xhttp":
		inbound["protocol"] = "vless"
		inbound["settings"] = object{"clients": users, "decryption": "none"}
		stream = object{"network": "xhttp", "security": "tls", "xhttpSettings": object{"path": path, "mode": "auto"}}
	case "hysteria2":
		inbound["protocol"] = "hysteria"
		inbound["settings"] = object{"version": 2, "users": users}
		stream = object{"network": "hysteria", "security": "tls", "hysteriaSettings": object{"version": 2}}
	case "wireguard":
		inbound["protocol"] = "wireguard"
		inbound["settings"] = object{"secretKey": serverPrivate, "address": []string{"10.77.0.1/32"}, "peers": peers, "noKernelTun": true, "mtu": 1280}
		extension = "conf"
	}
	if o.Protocol == "xhttp" || o.Protocol == "hysteria2" {
		alpn := []string{"h2", "http/1.1"}
		if o.Protocol == "hysteria2" {
			alpn = []string{"h3"}
		}
		stream["tlsSettings"] = object{"alpn": alpn, "certificates": []object{{"certificateFile": o.Cert, "keyFile": o.Key}}}
	}
	if len(stream) > 0 {
		inbound["streamSettings"] = stream
	}
	server, err := json.MarshalIndent(object{"log": object{"loglevel": "warning"}, "inbounds": []object{inbound}, "outbounds": []object{{"protocol": "freedom"}}}, "", "  ")
	return Bundle{Server: server, Clients: clients, Extension: extension}, err
}
func mustDecode(s string) []byte { b, _ := base64.StdEncoding.DecodeString(s); return b }
