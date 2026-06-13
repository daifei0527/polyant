package main

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubKeyToHex_FromBase64(t *testing.T) {
	// 32 raw bytes (Ed25519 public key length), base64-encoded as the client stores it
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)

	got, err := pubKeyToHex(b64)
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(raw), got)
}

func TestPubKeyToHex_InvalidBase64(t *testing.T) {
	_, err := pubKeyToHex("!!!not-base64!!!")
	assert.Error(t, err)
}
