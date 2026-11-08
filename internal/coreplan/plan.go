// Package coreplan produces reviewable Xray server configs and portable client
// profiles. It never installs services or opens firewall ports.
package coreplan

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Options struct {
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	SNI           string `json:"sni"`
	Cert          string `json:"cert,omitempty"`
	Key           string `json:"key,omitempty"`
	RealityTarget string `json:"realityTarget,omitempty"`
	Port          int    `json:"port"`
	Devices       int    `json:"-"`
	// CoreName is the name the core is installed under. It is not stored; it
	// only lets a second core of the same protocol (reality2) name its links
	// apart from the first.
	CoreName string `json:"-"`
}

// linkLabel is the protocol part of a client link's name: the protocol, or the
// core's own name when that name extends it ("reality2"), so two cores of one
// protocol do not give a user two links with the same name. Cores named
// otherwise ("hy2", "xh") keep the protocol, as their links always had.
func (o Options) linkLabel() string {
	if o.CoreName != o.Protocol && strings.HasPrefix(o.CoreName, o.Protocol) {
		return o.CoreName
	}
	return o.Protocol
}

// State is what must stay the same across regenerations of one core, so that
// adding or removing a client never changes another client's configuration:
// the server keys and paths, the seed client credentials are derived from,
// and each WireGuard client's address.
type State struct {
	ServerPrivate string            `json:"serverPrivate"` // X25519, standard base64
	ShortID       string            `json:"shortId"`
	Path          string            `json:"path"`
	Seed          string            `json:"seed"` // hex, 32 bytes
	WGAddrs       map[string]string `json:"wgAddrs,omitempty"`
	// APIPort is the loopback port of the core's Xray statistics API, used to
	// count each user's traffic (REALITY and XHTTP). 0 means none.
	APIPort int `json:"apiPort,omitempty"`
	// Paused is set while no user may use this core, so it is stopped. It is
	// not the operator switching the core off: the next allowed user starts it.
	Paused bool `json:"paused,omitempty"`
	// Revocation is why a user's access to this core could not be taken away
	// yet; empty when nothing is pending. It is retried on every limits run.
	Revocation string `json:"revocation,omitempty"`
}

// Metered reports whether a protocol's per-user traffic can be read from Xray.
func Metered(protocol string) bool { return protocol == "reality" || protocol == "xhttp" }

// Client is one credential holder. Secret takes part in the derivation, so a
// changed password revokes the client's previous credentials.
type Client struct {
	Name, Secret string
}

// NewState creates fresh server keys and a fresh seed.
func NewState() (State, error) {
	private, _, err := keypair()
	if err != nil {
		return State{}, err
	}
	sid, err := token(8)
	if err != nil {
		return State{}, err
	}
	pathToken, err := token(12)
	if err != nil {
		return State{}, err
	}
	seed, err := token(32)
	if err != nil {
		return State{}, err
	}
	return State{ServerPrivate: private, ShortID: sid, Path: "/" + pathToken, Seed: seed}, nil
}

// derive returns n bytes bound to the core's seed, the protocol and the client.
func (st *State) derive(protocol string, c Client, n int) ([]byte, error) {
	seed, err := hex.DecodeString(st.Seed)
	if err != nil || len(seed) == 0 {
		return nil, fmt.Errorf("invalid core state seed")
	}
	m := hmac.New(sha256.New, seed)
	m.Write([]byte(protocol + "\x00" + c.Name + "\x00" + c.Secret))
	return m.Sum(nil)[:n], nil
}

// assignWGAddrs keeps every existing client's address and gives new clients
// the lowest free one, so addresses never move between clients.
func (st *State) assignWGAddrs(clients []Client) error {
	if st.WGAddrs == nil {
		st.WGAddrs = map[string]string{}
	}
	want := map[string]bool{}
	for _, c := range clients {
		want[c.Name] = true
	}
	used := map[string]bool{}
	for name, addr := range st.WGAddrs {
		if !want[name] {
			delete(st.WGAddrs, name)
			continue
		}
		used[addr] = true
	}
	names := make([]string, 0, len(clients))
	for _, c := range clients {
		names = append(names, c.Name)
	}
	sort.Strings(names)
	next := 2
	for _, name := range names {
		if _, ok := st.WGAddrs[name]; ok {
			continue
		}
		for ; next < 65534; next++ {
			a := fmt.Sprintf("10.77.%d.%d", next/256, next%256)
			if !used[a] && next%256 != 0 && next%256 != 255 {
				st.WGAddrs[name] = a
				used[a] = true
				next++
				break
			}
		}
		if _, ok := st.WGAddrs[name]; !ok {
			return fmt.Errorf("no free WireGuard address")
		}
	}
	return nil
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

// Generate makes a one-off bundle with o.Devices random credentials.
func Generate(o Options) (Bundle, error) {
	if o.Devices < 1 || o.Devices > 1000 {
		return Bundle{}, fmt.Errorf("host, port 1..65535 and devices 1..1000 required")
	}
	st, err := NewState()
	if err != nil {
		return Bundle{}, err
	}
	clients := make([]Client, o.Devices)
	for i := range clients {
		secret, err := token(16)
		if err != nil {
			return Bundle{}, err
		}
		clients[i] = Client{Name: fmt.Sprintf("device-%03d", i+1), Secret: secret}
	}
	return GenerateFor(o, &st, clients)
}

var (
	// markdownLink matches [text](url), which chat apps and browsers paste in
	// place of a bare domain name.
	markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	hostName     = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)+$`)
)

// cleanHost turns a pasted [name](link) into name and trims spaces and quotes.
func cleanHost(s string) string {
	s = strings.TrimSpace(markdownLink.ReplaceAllString(s, "$1"))
	return strings.Trim(s, "\"'`<> ")
}

// Normalize repairs hostnames in o that were pasted as markdown links.
func Normalize(o *Options) {
	o.Host = cleanHost(o.Host)
	o.SNI = cleanHost(o.SNI)
	o.RealityTarget = cleanHost(o.RealityTarget)
}

// validHost reports whether s is an IP address or a dotted domain name.
func validHost(s string) bool {
	return net.ParseIP(s) != nil || hostName.MatchString(s)
}

// Validate checks the options a core is generated from. It first repairs
// hostnames pasted as markdown links.
func Validate(o *Options) error {
	Normalize(o)
	if o.Port < 1 || o.Port > 65535 || o.Host == "" {
		return fmt.Errorf("host, port 1..65535 and devices 1..1000 required")
	}
	if !validHost(o.Host) {
		return fmt.Errorf("invalid host %q: use the server's public IP address or its domain name", o.Host)
	}
	if o.SNI != "" && !validHost(o.SNI) {
		return fmt.Errorf("invalid server name %q: use a domain name such as www.example.com", o.SNI)
	}
	if _, err := url.ParseRequestURI("https://" + net.JoinHostPort(o.Host, strconv.Itoa(o.Port))); err != nil {
		return fmt.Errorf("invalid host")
	}
	switch o.Protocol {
	case "reality", "xhttp", "hysteria2", "wireguard":
	default:
		return fmt.Errorf("unsupported protocol")
	}
	if o.SNI == "" {
		o.SNI = o.Host
	}
	if (o.Protocol == "xhttp" || o.Protocol == "hysteria2") && (o.Cert == "" || o.Key == "") {
		return fmt.Errorf("TLS certificate and key paths required")
	}
	if o.Protocol == "reality" {
		th, tp, err := net.SplitHostPort(o.RealityTarget)
		if err != nil || !validHost(th) {
			return fmt.Errorf("REALITY target %q must be a domain and port such as www.example.com:443", o.RealityTarget)
		}
		if n, err := strconv.Atoi(tp); err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("REALITY target %q has an invalid port", o.RealityTarget)
		}
	}
	return nil
}

// GenerateFor builds the server config and one client profile per client,
// in order, from st. It is deterministic for the same options, state and
// clients; st's WireGuard addresses are updated for the client set.
func GenerateFor(o Options, st *State, clients []Client) (Bundle, error) {
	if err := Validate(&o); err != nil {
		return Bundle{}, err
	}
	private, err := ecdh.X25519().NewPrivateKey(mustDecode(st.ServerPrivate))
	if err != nil {
		return Bundle{}, fmt.Errorf("invalid core state key: %w", err)
	}
	serverPrivate := st.ServerPrivate
	serverPublic := base64.StdEncoding.EncodeToString(private.PublicKey().Bytes())
	sid, path := st.ShortID, st.Path
	if o.Protocol == "wireguard" {
		if err := st.assignWGAddrs(clients); err != nil {
			return Bundle{}, err
		}
	}
	inbound := object{"listen": "0.0.0.0", "port": o.Port}
	stream := object{}
	users := []object{}
	profiles := []string{}
	extension := "txt"
	peers := []object{}
	for _, c := range clients {
		name := fmt.Sprintf("Rovunet-%s-%s", o.linkLabel(), c.Name)
		q := url.Values{}
		var scheme, credential string
		switch o.Protocol {
		case "reality", "xhttp":
			b, err := st.derive(o.Protocol, c, 16)
			if err != nil {
				return Bundle{}, err
			}
			b[6] = b[6]&0x0f | 0x40 // RFC 4122 version 4 layout
			b[8] = b[8]&0x3f | 0x80
			raw := hex.EncodeToString(b)
			id := raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
			u := object{"id": id, "email": c.Name} // email names the user in Xray's statistics
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
			b, err := st.derive(o.Protocol, c, 24)
			if err != nil {
				return Bundle{}, err
			}
			credential = hex.EncodeToString(b)
			users = append(users, object{"auth": credential})
			scheme = "hysteria2"
			q.Set("sni", o.SNI)
		case "wireguard":
			b, err := st.derive(o.Protocol, c, 32)
			if err != nil {
				return Bundle{}, err
			}
			k, err := ecdh.X25519().NewPrivateKey(b)
			if err != nil {
				return Bundle{}, err
			}
			private := base64.StdEncoding.EncodeToString(k.Bytes())
			public := base64.StdEncoding.EncodeToString(k.PublicKey().Bytes())
			address := st.WGAddrs[c.Name]
			peers = append(peers, object{"publicKey": public, "allowedIPs": []string{address + "/32"}})
			profiles = append(profiles, fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s/32\nDNS = 1.1.1.1\nMTU = 1280\n\n[Peer]\nPublicKey = %s\nEndpoint = %s\nAllowedIPs = 0.0.0.0/0\nPersistentKeepalive = 25\n", private, address, serverPublic, net.JoinHostPort(o.Host, strconv.Itoa(o.Port))))
			continue
		}
		link := url.URL{Scheme: scheme, User: url.User(credential), Host: net.JoinHostPort(o.Host, strconv.Itoa(o.Port)), RawQuery: q.Encode(), Fragment: name}
		profiles = append(profiles, link.String())
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
	root := object{"log": object{"loglevel": "warning"}, "inbounds": []object{inbound}, "outbounds": []object{{"protocol": "freedom"}}}
	if st.APIPort > 0 && Metered(o.Protocol) {
		// Loopback-only statistics API (the classic form: a dokodemo-door
		// inbound routed to the api handler), plus per-user counters.
		root["inbounds"] = []object{inbound, {
			"tag": "api", "listen": "127.0.0.1", "port": st.APIPort,
			"protocol": "dokodemo-door", "settings": object{"address": "127.0.0.1"},
		}}
		root["stats"] = object{}
		root["api"] = object{"tag": "api", "services": []string{"StatsService"}}
		root["policy"] = object{"levels": object{"0": object{"statsUserUplink": true, "statsUserDownlink": true}}}
		root["routing"] = object{"rules": []object{{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"}}}
	}
	server, err := json.MarshalIndent(root, "", "  ")
	return Bundle{Server: server, Clients: profiles, Extension: extension}, err
}
func mustDecode(s string) []byte { b, _ := base64.StdEncoding.DecodeString(s); return b }
