package protocol

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TestMirrorDialTarget_IsRequesterPeer: the mirror-data stream must dial the
// requester's real peer (threaded from the stream), never the
// MirrorRequest.RequestID correlation id (P1.4). End-to-end coverage of the
// full mirror round-trip lands in the P3.1 mocknet testbed.
func TestMirrorDialTarget_IsRequesterPeer(t *testing.T) {
	requester := peer.ID("QmRequesterPeer")
	req := &MirrorRequest{RequestID: "corr-123"}

	got := mirrorDialTarget(requester, req)
	if got != requester {
		t.Errorf("mirror dial target = %s, want requester peer %s", got, requester)
	}
	// Must never decode the correlation id as a peer id (the old bug).
	if got == peer.ID(req.RequestID) {
		t.Error("mirror dial target must not be the RequestID correlation id")
	}
}
