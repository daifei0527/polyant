package protocol

const AWSPProtocolID = "/agentwiki/sync/2.0.0"

type MessageType int

const (
	MessageTypeUnknown MessageType = iota
	MessageTypeHandshake
	MessageTypeHandshakeAck
	MessageTypeQuery
	MessageTypeQueryResult
	MessageTypeSyncRequest
	MessageTypeSyncResponse
	MessageTypeMirrorRequest
	MessageTypeMirrorData
	MessageTypeMirrorAck
	MessageTypePushEntry
	MessageTypePushAck
	MessageTypeRatingPush
	MessageTypeRatingAck
	MessageTypeHeartbeat
	MessageTypeBitfield
)

type NodeType int

const (
	NodeTypeLocal NodeType = iota
	NodeTypeSeed
)

type QueryType int

const (
	QueryTypeLocal QueryType = iota
	QueryTypeGlobal
)

type MessageHeader struct {
	Type      MessageType `json:"type"`
	MessageID string      `json:"message_id"`
	Timestamp int64       `json:"timestamp"`
	Signature []byte      `json:"signature,omitempty"`
}

type Handshake struct {
	NodeID     string   `json:"node_id"`
	PeerID     string   `json:"peer_id"`
	NodeType   NodeType `json:"node_type"`
	Version    string   `json:"version"`
	Categories []string `json:"categories"`
	EntryCount int64    `json:"entry_count"`
	Signature  []byte   `json:"signature,omitempty"`
}

type HandshakeAck struct {
	NodeID       string   `json:"node_id"`
	PeerID       string   `json:"peer_id"`
	NodeType     NodeType `json:"node_type"`
	Version      string   `json:"version"`
	Accepted     bool     `json:"accepted"`
	RejectReason string   `json:"reject_reason,omitempty"`
	Signature    []byte   `json:"signature,omitempty"`
}

type Query struct {
	QueryID    string     `json:"query_id"`
	Keyword    string     `json:"keyword"`
	Categories []string   `json:"categories"`
	Limit      int32      `json:"limit"`
	Offset     int32      `json:"offset"`
	QueryType  QueryType  `json:"query_type"`
}

type QueryResult struct {
	QueryID    string   `json:"query_id"`
	Entries    [][]byte `json:"entries"`
	TotalCount int32    `json:"total_count"`
	HasMore    bool     `json:"has_more"`
}

type SyncRequest struct {
	RequestID           string            `json:"request_id"`
	LastSyncTimestamp   int64             `json:"last_sync_timestamp"`
	VersionVector       map[string]int64  `json:"version_vector"`
	RequestedCategories []string          `json:"requested_categories"`
}

type SyncResponse struct {
	RequestID            string            `json:"request_id"`
	NewEntries           [][]byte          `json:"new_entries"`
	UpdatedEntries       [][]byte          `json:"updated_entries"`
	DeletedEntryIDs      []string          `json:"deleted_entry_ids"`
	NewRatings           [][]byte          `json:"new_ratings"`
	ServerVersionVector  map[string]int64  `json:"server_version_vector"`
	ServerTimestamp      int64             `json:"server_timestamp"`
}

type MirrorRequest struct {
	RequestID  string   `json:"request_id"`
	Categories []string `json:"categories"`
	FullMirror bool     `json:"full_mirror"`
	BatchSize  int32    `json:"batch_size"`
}

type MirrorData struct {
	RequestID   string   `json:"request_id"`
	BatchIndex  int32    `json:"batch_index"`
	TotalBatches int32   `json:"total_batches"`
	Entries     [][]byte `json:"entries"`
	Categories  [][]byte `json:"categories"`
}

type MirrorAck struct {
	RequestID       string `json:"request_id"`
	Success         bool   `json:"success"`
	ErrorMessage    string `json:"error_message,omitempty"`
	ReceivedEntries int64  `json:"received_entries"`
}

type PushEntry struct {
	EntryID         string `json:"entry_id"`
	Entry           []byte `json:"entry"`
	CreatorSignature []byte `json:"creator_signature"`
}

type PushAck struct {
	EntryID      string `json:"entry_id"`
	Accepted     bool   `json:"accepted"`
	RejectReason string `json:"reject_reason,omitempty"`
	NewVersion   int64  `json:"new_version"`
}

type RatingPush struct {
	Rating       []byte `json:"rating"`
	RaterSignature []byte `json:"rater_signature"`
}

type RatingAck struct {
	RatingID     string `json:"rating_id"`
	Accepted     bool   `json:"accepted"`
	RejectReason string `json:"reject_reason,omitempty"`
}

type Heartbeat struct {
	NodeID     string `json:"node_id"`
	UptimeSeconds int64 `json:"uptime_seconds"`
	EntryCount int64  `json:"entry_count"`
	Timestamp  int64  `json:"timestamp"`
}

type Bitfield struct {
	NodeID        string            `json:"node_id"`
	VersionVector map[string]int64  `json:"version_vector"`
	EntryCount    int64             `json:"entry_count"`
}
