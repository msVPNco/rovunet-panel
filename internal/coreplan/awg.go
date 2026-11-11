package coreplan

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
)

// The v3 fields below are supported by the pinned server and Android alpha18.1.
// This is AmneziaWG, not a WireGuard file with a different display label.
type AWGObfuscation struct {
	Jc, Jmin, Jmax, S1, S2, S3, S4 int
	H1, H2, H3, H4                 uint32
	HeaderProtectionKey            string
	ContentPaddingAddition         string
}

type AWGPeer struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"`
	Address   string `json:"address"`
}

type AWGServer struct {
	PrivateKey  string         `json:"privateKey"`
	Address     string         `json:"address"`
	Subnet      string         `json:"subnet"`
	Egress      string         `json:"egress"`
	Port        int            `json:"port"`
	Obfuscation AWGObfuscation `json:"obfuscation"`
	Peers       []AWGPeer      `json:"peers"`
}

func (a AWGObfuscation) Fields() string {
	return fmt.Sprintf("Jc = %d\nJmin = %d\nJmax = %d\nS1 = %d\nS2 = %d\nS3 = %d\nS4 = %d\nH1 = %d\nH2 = %d\nH3 = %d\nH4 = %d\nHeaderProtectionKey = %s\nContentPaddingAddition = %s\n", a.Jc, a.Jmin, a.Jmax, a.S1, a.S2, a.S3, a.S4, a.H1, a.H2, a.H3, a.H4, a.HeaderProtectionKey, a.ContentPaddingAddition)
}

func (s AWGServer) SetConf() string {
	v := fmt.Sprintf("[Interface]\nPrivateKey = %s\nListenPort = %d\n%s", s.PrivateKey, s.Port, s.Obfuscation.Fields())
	for _, p := range s.Peers {
		v += fmt.Sprintf("\n[Peer]\nPublicKey = %s\nAllowedIPs = %s/32\n", p.PublicKey, p.Address)
	}
	return v
}

func generateAWG(o Options, st *State, clients []Client) (Bundle, error) {
	if st.AWG == nil {
		var random [48]byte
		if _, err := rand.Read(random[:]); err != nil {
			return Bundle{}, err
		}
		st.AWG = &AWGObfuscation{Jc: 4, Jmin: 40, Jmax: 100, S1: 32, S2: 48, S3: 24, S4: 32,
			H1: binary.LittleEndian.Uint32(random[0:4]), H2: binary.LittleEndian.Uint32(random[4:8]), H3: binary.LittleEndian.Uint32(random[8:12]), H4: binary.LittleEndian.Uint32(random[12:16]), HeaderProtectionKey: base64.StdEncoding.EncodeToString(random[16:48]), ContentPaddingAddition: "0-32"}
	}
	ip, _, _ := net.ParseCIDR(o.Subnet)
	ip = ip.To4()
	base := fmt.Sprintf("%d.%d.%d.", ip[0], ip[1], ip[2])
	if st.WGAddrs == nil {
		st.WGAddrs = map[string]string{}
	}
	want, used := map[string]bool{}, map[string]bool{}
	for _, c := range clients {
		want[c.Name] = true
	}
	for name, addr := range st.WGAddrs {
		if !want[name] {
			delete(st.WGAddrs, name)
			continue
		}
		if used[addr] {
			return Bundle{}, fmt.Errorf("duplicate AmneziaWG peer address")
		}
		used[addr] = true
	}
	names := make([]string, 0, len(clients))
	for _, c := range clients {
		names = append(names, c.Name)
	}
	sort.Strings(names)
	for _, name := range names {
		if st.WGAddrs[name] != "" {
			continue
		}
		for i := 2; i <= 254; i++ {
			addr := base + strconv.Itoa(i)
			if !used[addr] {
				st.WGAddrs[name] = addr
				used[addr] = true
				break
			}
		}
		if st.WGAddrs[name] == "" {
			return Bundle{}, fmt.Errorf("AmneziaWG /24 peer capacity reached")
		}
	}
	private, err := ecdh.X25519().NewPrivateKey(mustDecode(st.ServerPrivate))
	if err != nil {
		return Bundle{}, err
	}
	public := base64.StdEncoding.EncodeToString(private.PublicKey().Bytes())
	server := AWGServer{PrivateKey: st.ServerPrivate, Address: base + "1/24", Subnet: o.Subnet, Egress: o.Egress, Port: o.Port, Obfuscation: *st.AWG, Peers: []AWGPeer{}}
	b := Bundle{Extension: "conf"}
	for _, c := range clients {
		secret, err := st.derive("amneziawg", c, 32)
		if err != nil {
			return Bundle{}, err
		}
		key, err := ecdh.X25519().NewPrivateKey(secret)
		if err != nil {
			return Bundle{}, err
		}
		address := st.WGAddrs[c.Name]
		server.Peers = append(server.Peers, AWGPeer{Name: c.Name, PublicKey: base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()), Address: address})
		b.Clients = append(b.Clients, fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s/32\nDNS = 1.1.1.1\nMTU = 1280\n%s\n[Peer]\nPublicKey = %s\nEndpoint = %s\nAllowedIPs = 0.0.0.0/0\nPersistentKeepalive = 25\n", base64.StdEncoding.EncodeToString(key.Bytes()), address, st.AWG.Fields(), public, net.JoinHostPort(o.Host, strconv.Itoa(o.Port))))
	}
	b.Server, err = json.MarshalIndent(server, "", "  ")
	return b, err
}
