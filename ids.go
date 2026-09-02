package main

import (
	"crypto/rand"
	"encoding/base64"
)

// newID returns a new random identifier for a paste. It reads 16 bytes from
// crypto/rand and encodes them with base64.RawURLEncoding.
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
