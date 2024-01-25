package webgockets

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
)

func handshakeKeyRequest() (string, error) {
	p := make([]byte, 16)

	// Randomize 16 bytes.
	if _, err := io.ReadFull(rand.Reader, p); err != nil {
		return "", fmt.Errorf("failed to write random bytes: %w", err)
	}

	// Base64.
	return base64.StdEncoding.EncodeToString(p), nil
}

// keyGUID is the shared guid to append to the randomized key.
var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func handshakeAcceptKey(key string) (string, error) {
	h := sha1.New()

	if _, err := h.Write([]byte(key)); err != nil {
		return "", fmt.Errorf("failed to write key to hash: %w", err)
	}

	if _, err := h.Write(keyGUID); err != nil {
		return "", fmt.Errorf("failed to write key to hash: %w", err)
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
