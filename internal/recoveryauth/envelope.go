package recoveryauth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type Envelope struct {
	Payload   []byte `json:"payload"`
	Signature []byte `json:"signature"`
}

func Sign(value any, private ed25519.PrivateKey) ([]byte, error) {
	p, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	return json.Marshal(Envelope{p, ed25519.Sign(private, p)})
}
func Verify(encoded []byte, publicHex string, value any) error {
	key, e := hex.DecodeString(publicHex)
	if e != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("invalid externally pinned recovery key")
	}
	var env Envelope
	d := json.NewDecoder(bytes.NewReader(encoded))
	d.DisallowUnknownFields()
	if e = d.Decode(&env); e != nil {
		return e
	}
	if !ed25519.Verify(key, env.Payload, env.Signature) {
		return errors.New("recovery authentication failed")
	}
	d = json.NewDecoder(bytes.NewReader(env.Payload))
	d.DisallowUnknownFields()
	return d.Decode(value)
}
