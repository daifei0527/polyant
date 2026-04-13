package protocol

import (
	"testing"

	awsp "github.com/daifei0527/agentwiki/internal/network/protocol/proto"
)

func TestMessageTypeConversion(t *testing.T) {
	testCases := []struct {
		domain    MessageType
		proto     awsp.MessageType
	}{
		{MessageTypeHandshake, awsp.MessageType_MESSAGE_TYPE_HANDSHAKE},
		{MessageTypeHandshakeAck, awsp.MessageType_MESSAGE_TYPE_HANDSHAKE_ACK},
		{MessageTypeQuery, awsp.MessageType_MESSAGE_TYPE_QUERY},
		{MessageTypeQueryResult, awsp.MessageType_MESSAGE_TYPE_QUERY_RESULT},
		{MessageTypeSyncRequest, awsp.MessageType_MESSAGE_TYPE_SYNC_REQUEST},
		{MessageTypeSyncResponse, awsp.MessageType_MESSAGE_TYPE_SYNC_RESPONSE},
		{MessageTypeMirrorRequest, awsp.MessageType_MESSAGE_TYPE_MIRROR_REQUEST},
		{MessageTypeMirrorData, awsp.MessageType_MESSAGE_TYPE_MIRROR_DATA},
		{MessageTypeMirrorAck, awsp.MessageType_MESSAGE_TYPE_MIRROR_ACK},
		{MessageTypePushEntry, awsp.MessageType_MESSAGE_TYPE_PUSH_ENTRY},
		{MessageTypePushAck, awsp.MessageType_MESSAGE_TYPE_PUSH_ACK},
		{MessageTypeRatingPush, awsp.MessageType_MESSAGE_TYPE_RATING_PUSH},
		{MessageTypeRatingAck, awsp.MessageType_MESSAGE_TYPE_RATING_ACK},
		{MessageTypeHeartbeat, awsp.MessageType_MESSAGE_TYPE_HEARTBEAT},
		{MessageTypeBitfield, awsp.MessageType_MESSAGE_TYPE_BITFIELD},
	}

	for _, tc := range testCases {
		t.Run(tc.proto.String(), func(t *testing.T) {
			// Domain to proto
			gotProto := toProtoMessageType(tc.domain)
			if gotProto != tc.proto {
				t.Errorf("toProtoMessageType(%d) = %v, want %v", tc.domain, gotProto, tc.proto)
			}

			// Proto to domain
			gotDomain := fromProtoMessageType(tc.proto)
			if gotDomain != tc.domain {
				t.Errorf("fromProtoMessageType(%v) = %d, want %d", tc.proto, gotDomain, tc.domain)
			}
		})
	}

	// Test unknown types
	if toProtoMessageType(MessageTypeUnknown) != awsp.MessageType_MESSAGE_TYPE_UNKNOWN {
		t.Error("Unknown domain type should map to unknown proto type")
	}
	if fromProtoMessageType(awsp.MessageType_MESSAGE_TYPE_UNKNOWN) != MessageTypeUnknown {
		t.Error("Unknown proto type should map to unknown domain type")
	}
}

func TestNodeTypeConversion(t *testing.T) {
	testCases := []struct {
		domain NodeType
		proto  awsp.NodeType
	}{
		{NodeTypeLocal, awsp.NodeType_NODE_TYPE_LOCAL},
		{NodeTypeSeed, awsp.NodeType_NODE_TYPE_SEED},
	}

	for _, tc := range testCases {
		t.Run(tc.proto.String(), func(t *testing.T) {
			gotProto := toProtoNodeType(tc.domain)
			if gotProto != tc.proto {
				t.Errorf("toProtoNodeType(%d) = %v, want %v", tc.domain, gotProto, tc.proto)
			}

			gotDomain := fromProtoNodeType(tc.proto)
			if gotDomain != tc.domain {
				t.Errorf("fromProtoNodeType(%v) = %d, want %d", tc.proto, gotDomain, tc.domain)
			}
		})
	}
}

func TestQueryTypeConversion(t *testing.T) {
	testCases := []struct {
		domain QueryType
		proto  awsp.QueryType
	}{
		{QueryTypeLocal, awsp.QueryType_QUERY_TYPE_LOCAL},
		{QueryTypeGlobal, awsp.QueryType_QUERY_TYPE_GLOBAL},
	}

	for _, tc := range testCases {
		t.Run(tc.proto.String(), func(t *testing.T) {
			gotProto := toProtoQueryType(tc.domain)
			if gotProto != tc.proto {
				t.Errorf("toProtoQueryType(%d) = %v, want %v", tc.domain, gotProto, tc.proto)
			}

			gotDomain := fromProtoQueryType(tc.proto)
			if gotDomain != tc.domain {
				t.Errorf("fromProtoQueryType(%v) = %d, want %d", tc.proto, gotDomain, tc.domain)
			}
		})
	}
}

func TestHandshakeConversion(t *testing.T) {
	domain := &Handshake{
		NodeID:     "node-001",
		PeerID:     "peer-001",
		NodeType:   NodeTypeSeed,
		Version:    "1.0.0",
		Categories: []string{"tech", "science"},
		EntryCount: 100,
		Signature:  []byte("test-sig"),
	}

	proto := toProtoHandshake(domain)
	if proto == nil {
		t.Fatal("toProtoHandshake returned nil")
	}

	if proto.NodeId != domain.NodeID {
		t.Errorf("NodeId: got %q, want %q", proto.NodeId, domain.NodeID)
	}
	if proto.PeerId != domain.PeerID {
		t.Errorf("PeerId: got %q, want %q", proto.PeerId, domain.PeerID)
	}
	if proto.NodeType != awsp.NodeType_NODE_TYPE_SEED {
		t.Errorf("NodeType: got %v, want %v", proto.NodeType, awsp.NodeType_NODE_TYPE_SEED)
	}

	// Convert back
	gotDomain := fromProtoHandshake(proto)
	if gotDomain == nil {
		t.Fatal("fromProtoHandshake returned nil")
	}

	if gotDomain.NodeID != domain.NodeID {
		t.Errorf("NodeID round-trip failed: got %q, want %q", gotDomain.NodeID, domain.NodeID)
	}
	if gotDomain.EntryCount != domain.EntryCount {
		t.Errorf("EntryCount round-trip failed: got %d, want %d", gotDomain.EntryCount, domain.EntryCount)
	}
	if len(gotDomain.Categories) != len(domain.Categories) {
		t.Errorf("Categories length: got %d, want %d", len(gotDomain.Categories), len(domain.Categories))
	}
}

func TestQueryConversion(t *testing.T) {
	domain := &Query{
		QueryID:    "query-001",
		Keyword:    "golang protobuf",
		Categories: []string{"programming"},
		Limit:      10,
		Offset:     5,
		QueryType:  QueryTypeGlobal,
	}

	proto := toProtoQuery(domain)
	if proto == nil {
		t.Fatal("toProtoQuery returned nil")
	}

	if proto.QueryId != domain.QueryID {
		t.Errorf("QueryId: got %q, want %q", proto.QueryId, domain.QueryID)
	}
	if proto.Keyword != domain.Keyword {
		t.Errorf("Keyword: got %q, want %q", proto.Keyword, domain.Keyword)
	}
	if proto.QueryType != awsp.QueryType_QUERY_TYPE_GLOBAL {
		t.Errorf("QueryType: got %v, want %v", proto.QueryType, awsp.QueryType_QUERY_TYPE_GLOBAL)
	}

	gotDomain := fromProtoQuery(proto)
	if gotDomain == nil {
		t.Fatal("fromProtoQuery returned nil")
	}

	if gotDomain.QueryID != domain.QueryID {
		t.Errorf("QueryID round-trip failed")
	}
	if gotDomain.Limit != domain.Limit {
		t.Errorf("Limit round-trip failed")
	}
}

func TestSyncRequestConversion(t *testing.T) {
	domain := &SyncRequest{
		RequestID:         "sync-001",
		LastSyncTimestamp: 1712995200000,
		VersionVector: map[string]int64{
			"node-001": 100,
			"node-002": 50,
		},
		RequestedCategories: []string{"tech", "news"},
	}

	proto := toProtoSyncRequest(domain)
	if proto == nil {
		t.Fatal("toProtoSyncRequest returned nil")
	}

	if proto.RequestId != domain.RequestID {
		t.Errorf("RequestId: got %q, want %q", proto.RequestId, domain.RequestID)
	}
	if proto.VersionVector["node-001"] != 100 {
		t.Errorf("VersionVector[node-001]: got %d, want 100", proto.VersionVector["node-001"])
	}

	gotDomain := fromProtoSyncRequest(proto)
	if gotDomain == nil {
		t.Fatal("fromProtoSyncRequest returned nil")
	}

	if gotDomain.RequestID != domain.RequestID {
		t.Errorf("RequestID round-trip failed")
	}
	if len(gotDomain.VersionVector) != len(domain.VersionVector) {
		t.Errorf("VersionVector size round-trip failed")
	}
}

func TestSyncResponseConversion(t *testing.T) {
	domain := &SyncResponse{
		RequestID:       "sync-001",
		NewEntries:      [][]byte{[]byte("entry1"), []byte("entry2")},
		UpdatedEntries:  [][]byte{[]byte("entry3")},
		DeletedEntryIDs: []string{"old-1", "old-2"},
		NewRatings:      [][]byte{[]byte("rating1")},
		ServerVersionVector: map[string]int64{
			"server": 200,
		},
		ServerTimestamp: 1712995200000,
	}

	proto := toProtoSyncResponse(domain)
	if proto == nil {
		t.Fatal("toProtoSyncResponse returned nil")
	}

	if proto.RequestId != domain.RequestID {
		t.Errorf("RequestId: got %q, want %q", proto.RequestId, domain.RequestID)
	}
	if len(proto.NewEntries) != 2 {
		t.Errorf("NewEntries length: got %d, want 2", len(proto.NewEntries))
	}

	gotDomain := fromProtoSyncResponse(proto)
	if gotDomain == nil {
		t.Fatal("fromProtoSyncResponse returned nil")
	}

	if gotDomain.RequestID != domain.RequestID {
		t.Errorf("RequestID round-trip failed")
	}
	if len(gotDomain.NewEntries) != len(domain.NewEntries) {
		t.Errorf("NewEntries length round-trip failed")
	}
}

func TestMessageConversion(t *testing.T) {
	testCases := []struct {
		name    string
		msgType MessageType
		payload interface{}
	}{
		{"Handshake", MessageTypeHandshake, &Handshake{NodeID: "node-001", NodeType: NodeTypeSeed}},
		{"HandshakeAck", MessageTypeHandshakeAck, &HandshakeAck{NodeID: "node-002", Accepted: true}},
		{"Query", MessageTypeQuery, &Query{QueryID: "q-001", Keyword: "test", QueryType: QueryTypeGlobal}},
		{"QueryResult", MessageTypeQueryResult, &QueryResult{QueryID: "q-001", TotalCount: 10, HasMore: true}},
		{"SyncRequest", MessageTypeSyncRequest, &SyncRequest{RequestID: "sync-001"}},
		{"SyncResponse", MessageTypeSyncResponse, &SyncResponse{RequestID: "sync-001"}},
		{"MirrorRequest", MessageTypeMirrorRequest, &MirrorRequest{RequestID: "mirror-001", FullMirror: true}},
		{"MirrorData", MessageTypeMirrorData, &MirrorData{RequestID: "mirror-001", BatchIndex: 1}},
		{"MirrorAck", MessageTypeMirrorAck, &MirrorAck{RequestID: "mirror-001", Success: true}},
		{"PushEntry", MessageTypePushEntry, &PushEntry{EntryID: "entry-001", Entry: []byte("data")}},
		{"PushAck", MessageTypePushAck, &PushAck{EntryID: "entry-001", Accepted: true}},
		{"RatingPush", MessageTypeRatingPush, &RatingPush{Rating: []byte("rating-data")}},
		{"RatingAck", MessageTypeRatingAck, &RatingAck{RatingID: "rating-001", Accepted: true}},
		{"Heartbeat", MessageTypeHeartbeat, &Heartbeat{NodeID: "node-001", UptimeSeconds: 3600}},
		{"Bitfield", MessageTypeBitfield, &Bitfield{NodeID: "node-001", EntryCount: 1000}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &Message{
				Header:  NewMessageHeader(tc.msgType),
				Payload: tc.payload,
			}

			// Convert to proto
			protoMsg := toProtoMessage(msg)
			if protoMsg == nil {
				t.Fatal("toProtoMessage returned nil")
			}

			// Verify header
			if protoMsg.Header == nil {
				t.Fatal("proto message header is nil")
			}
			expectedProtoType := toProtoMessageType(tc.msgType)
			if protoMsg.Header.Type != expectedProtoType {
				t.Errorf("Header.Type: got %v, want %v", protoMsg.Header.Type, expectedProtoType)
			}

			// Convert back to domain
			gotMsg := fromProtoMessage(protoMsg)
			if gotMsg == nil {
				t.Fatal("fromProtoMessage returned nil")
			}

			// Verify type
			if gotMsg.Header.Type != tc.msgType {
				t.Errorf("MessageType round-trip failed: got %v, want %v", gotMsg.Header.Type, tc.msgType)
			}

			// Verify payload is not nil
			if gotMsg.Payload == nil {
				t.Error("Payload should not be nil after round-trip")
			}
		})
	}
}

func TestNilConversion(t *testing.T) {
	// Test nil conversions
	if toProtoHandshake(nil) != nil {
		t.Error("toProtoHandshake(nil) should return nil")
	}
	if fromProtoHandshake(nil) != nil {
		t.Error("fromProtoHandshake(nil) should return nil")
	}
	if toProtoMessage(nil) != nil {
		t.Error("toProtoMessage(nil) should return nil")
	}
	if fromProtoMessage(nil) != nil {
		t.Error("fromProtoMessage(nil) should return nil")
	}
}
