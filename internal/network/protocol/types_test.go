package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapability(t *testing.T) {
	cap := Capability{
		Type:      CapabilityRelay,
		Limit:     100,
		Available: true,
	}

	assert.Equal(t, CapabilityRelay, cap.Type)
	assert.Equal(t, 100, cap.Limit)
	assert.True(t, cap.Available)
}

func TestHandshakeWithCapabilities(t *testing.T) {
	h := Handshake{
		NodeID:   "node-1",
		PeerID:   "peer-1",
		NodeType: NodeTypeSeed,
		Version:  "2.0.0",
		Capabilities: []Capability{
			{Type: CapabilityRelay, Limit: 100, Available: true},
			{Type: CapabilityMirror, Limit: 50, Available: true},
		},
	}

	assert.Len(t, h.Capabilities, 2)
	assert.Equal(t, NodeTypeSeed, h.NodeType)
}

func TestNodeTypeUser(t *testing.T) {
	assert.Equal(t, NodeType(2), NodeTypeUser)
}
