package protocol

import (
	"bytes"
	"testing"

	"github.com/daifei0527/agentwiki/internal/network/protocol/proto"
)

func BenchmarkProtobufCodec_Encode(b *testing.B) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_HANDSHAKE,
			MessageId: "bench-msg",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_Handshake{
			Handshake: &proto.Handshake{
				NodeId:     "node-1",
				PeerId:     "peer-1",
				NodeType:   proto.NodeType_NODE_TYPE_LOCAL,
				Version:    "2.0.0",
				Categories: []string{"tech", "science", "business"},
				EntryCount: 1000,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(msg)
	}
}

func BenchmarkProtobufCodec_Decode(b *testing.B) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_HANDSHAKE,
			MessageId: "bench-msg",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_Handshake{
			Handshake: &proto.Handshake{
				NodeId:     "node-1",
				PeerId:     "peer-1",
				NodeType:   proto.NodeType_NODE_TYPE_LOCAL,
				Version:    "2.0.0",
				Categories: []string{"tech", "science", "business"},
				EntryCount: 1000,
			},
		},
	}

	encoded, _ := codec.Encode(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(encoded)
		codec.Decode(reader)
	}
}

func BenchmarkProtobufCodec_RoundTrip(b *testing.B) {
	codec := NewProtobufCodec()

	msg := &proto.Message{
		Header: &proto.MessageHeader{
			Type:      proto.MessageType_MESSAGE_TYPE_SYNC_REQUEST,
			MessageId: "bench-sync",
			Timestamp: 1234567890,
		},
		Payload: &proto.Message_SyncRequest{
			SyncRequest: &proto.SyncRequest{
				RequestId:           "sync-1",
				LastSyncTimestamp:   1234560000,
				VersionVector:       map[string]int64{"e1": 100, "e2": 200, "e3": 300},
				RequestedCategories: []string{"tech", "science"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := codec.Encode(msg)
		codec.Decode(bytes.NewReader(encoded))
	}
}
