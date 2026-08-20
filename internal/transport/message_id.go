package transport

import (
	"crypto/rand"
	"encoding/hex"
)

// NewMessageID creates
func NewMessageID() string {
	var b = make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)
}
