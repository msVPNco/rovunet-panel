package dnsrouter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"net"
	"testing"
)

// buildProbeQuery assembles a DNS query byte-for-byte compatible with
// handleVerify's parser: a question whose first label is
// base32(nonce[16]||clientProof[16]) under the given domain, QTYPE=TXT,
// plus (optionally) an EDNS0 OPT additional record advertising
// ednsPayloadSize as its UDP payload size.
func buildProbeQuery(t *testing.T, pubkey []byte, domainLabels []string, nonce [16]byte, includeEDNS0 bool, ednsPayloadSize uint16) []byte {
	t.Helper()

	mac := hmac.New(sha256.New, pubkey)
	mac.Write(nonce[:])
	clientProof := mac.Sum(nil)[:16]

	var probe []byte
	probe = append(probe, nonce[:]...)
	probe = append(probe, clientProof...)
	encodedLabel := verifyEncoding.EncodeToString(probe)

	var packet []byte
	// Header: ID, flags(QR=0), QDCOUNT=1, ANCOUNT=0, NSCOUNT=0, ARCOUNT
	packet = append(packet, 0x12, 0x34) // ID
	packet = append(packet, 0x00, 0x00) // flags
	packet = append(packet, 0x00, 0x01) // QDCOUNT
	packet = append(packet, 0x00, 0x00) // ANCOUNT
	packet = append(packet, 0x00, 0x00) // NSCOUNT
	if includeEDNS0 {
		packet = append(packet, 0x00, 0x01) // ARCOUNT=1
	} else {
		packet = append(packet, 0x00, 0x00) // ARCOUNT=0
	}

	// Question: probe label, then each domain label, then root.
	packet = append(packet, byte(len(encodedLabel)))
	packet = append(packet, encodedLabel...)
	for _, lbl := range domainLabels {
		packet = append(packet, byte(len(lbl)))
		packet = append(packet, lbl...)
	}
	packet = append(packet, 0x00)       // root terminator
	packet = append(packet, 0x00, 0x10) // QTYPE = TXT (16)
	packet = append(packet, 0x00, 0x01) // QCLASS = IN

	if includeEDNS0 {
		packet = append(packet, 0x00)       // OPT name: root
		packet = append(packet, 0x00, 0x29) // TYPE: OPT (41)
		var sizeBuf [2]byte
		binary.BigEndian.PutUint16(sizeBuf[:], ednsPayloadSize)
		packet = append(packet, sizeBuf[:]...)          // CLASS: UDP payload size
		packet = append(packet, 0x00, 0x00, 0x00, 0x00) // TTL
		packet = append(packet, 0x00, 0x00)             // RDLENGTH
	}

	return packet
}

// TestHandleVerifyClampsOversizedEDNS0PayloadSize is the regression test for
// the remote panic: a crafted query advertising an EDNS0 UDP payload size
// of 65535 (the wire maximum, far past the 4096-byte padding buffer) must
// not crash handleVerify — the response must come back, clamped, not panic.
func TestHandleVerifyClampsOversizedEDNS0PayloadSize(t *testing.T) {
	pubkey := []byte("test-pubkey-0123456789abcdef")
	r := New("127.0.0.1:0")
	r.AddVerifyRoute("tunnel.example", pubkey, 0)

	var nonce [16]byte
	// nonce[14:16] must fall outside 200..4096 so handleVerify falls through
	// to the parseEDNS0PayloadSize-derived path instead of the nonce-encoded
	// desired size.
	nonce[14], nonce[15] = 0x00, 0x00

	packet := buildProbeQuery(t, pubkey, []string{"tunnel", "example"}, nonce, true, 65535)

	var resp []byte
	ok := r.handleVerify(packet, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}, func(msg []byte) {
		resp = msg
	})
	if !ok {
		t.Fatal("expected handleVerify to recognize the probe and respond")
	}
	if resp == nil {
		t.Fatal("expected a response to be sent")
	}
	// A legitimate response targets ~75%-100% of mtu (clamped to
	// maxPaddingSize here), so a few hundred bytes above maxPaddingSize for
	// header/question/answer/OPT overhead is normal and expected. What this
	// guards against is the unclamped bug: an EDNS0 size of 65535 flowing
	// straight into targetData, which would either panic outright (caught
	// separately, since a panic fails the whole test) or, had the panic
	// been avoided some other way, produce a response tens of kilobytes
	// larger than this bound.
	const maxReasonableResponse = maxPaddingSize + 256
	if len(resp) > maxReasonableResponse {
		t.Fatalf("response unexpectedly large (%d bytes, want <= %d); EDNS0 size was not clamped", len(resp), maxReasonableResponse)
	}
}

// TestParseEDNS0PayloadSizeClampsToMaxPadding checks the clamp directly,
// independent of handleVerify's other logic.
func TestParseEDNS0PayloadSizeClampsToMaxPadding(t *testing.T) {
	pubkey := []byte("k")
	var nonce [16]byte
	packet := buildProbeQuery(t, pubkey, []string{"x"}, nonce, true, 65535)
	// qEnd mirrors handleVerify's computation for this fixed-shape query:
	// header(12) + probe label + domain labels + root + QTYPE/QCLASS(4).
	qEnd := len(packet) - 11 // 11 = OPT RR length appended by buildProbeQuery
	got := parseEDNS0PayloadSize(packet, qEnd)
	if got > maxPaddingSize {
		t.Fatalf("parseEDNS0PayloadSize returned %d, want <= %d", got, maxPaddingSize)
	}
	if got != maxPaddingSize {
		t.Fatalf("parseEDNS0PayloadSize returned %d, want exactly %d (65535 clamped)", got, maxPaddingSize)
	}
}
