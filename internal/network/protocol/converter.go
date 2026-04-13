package protocol

import (
	awsp "github.com/daifei0527/agentwiki/internal/network/protocol/proto"
)

// ==================== MessageType Conversion ====================

// toProtoMessageType converts domain MessageType to proto MessageType.
func toProtoMessageType(t MessageType) awsp.MessageType {
	switch t {
	case MessageTypeHandshake:
		return awsp.MessageType_MESSAGE_TYPE_HANDSHAKE
	case MessageTypeHandshakeAck:
		return awsp.MessageType_MESSAGE_TYPE_HANDSHAKE_ACK
	case MessageTypeQuery:
		return awsp.MessageType_MESSAGE_TYPE_QUERY
	case MessageTypeQueryResult:
		return awsp.MessageType_MESSAGE_TYPE_QUERY_RESULT
	case MessageTypeSyncRequest:
		return awsp.MessageType_MESSAGE_TYPE_SYNC_REQUEST
	case MessageTypeSyncResponse:
		return awsp.MessageType_MESSAGE_TYPE_SYNC_RESPONSE
	case MessageTypeMirrorRequest:
		return awsp.MessageType_MESSAGE_TYPE_MIRROR_REQUEST
	case MessageTypeMirrorData:
		return awsp.MessageType_MESSAGE_TYPE_MIRROR_DATA
	case MessageTypeMirrorAck:
		return awsp.MessageType_MESSAGE_TYPE_MIRROR_ACK
	case MessageTypePushEntry:
		return awsp.MessageType_MESSAGE_TYPE_PUSH_ENTRY
	case MessageTypePushAck:
		return awsp.MessageType_MESSAGE_TYPE_PUSH_ACK
	case MessageTypeRatingPush:
		return awsp.MessageType_MESSAGE_TYPE_RATING_PUSH
	case MessageTypeRatingAck:
		return awsp.MessageType_MESSAGE_TYPE_RATING_ACK
	case MessageTypeHeartbeat:
		return awsp.MessageType_MESSAGE_TYPE_HEARTBEAT
	case MessageTypeBitfield:
		return awsp.MessageType_MESSAGE_TYPE_BITFIELD
	default:
		return awsp.MessageType_MESSAGE_TYPE_UNKNOWN
	}
}

// fromProtoMessageType converts proto MessageType to domain MessageType.
func fromProtoMessageType(t awsp.MessageType) MessageType {
	switch t {
	case awsp.MessageType_MESSAGE_TYPE_HANDSHAKE:
		return MessageTypeHandshake
	case awsp.MessageType_MESSAGE_TYPE_HANDSHAKE_ACK:
		return MessageTypeHandshakeAck
	case awsp.MessageType_MESSAGE_TYPE_QUERY:
		return MessageTypeQuery
	case awsp.MessageType_MESSAGE_TYPE_QUERY_RESULT:
		return MessageTypeQueryResult
	case awsp.MessageType_MESSAGE_TYPE_SYNC_REQUEST:
		return MessageTypeSyncRequest
	case awsp.MessageType_MESSAGE_TYPE_SYNC_RESPONSE:
		return MessageTypeSyncResponse
	case awsp.MessageType_MESSAGE_TYPE_MIRROR_REQUEST:
		return MessageTypeMirrorRequest
	case awsp.MessageType_MESSAGE_TYPE_MIRROR_DATA:
		return MessageTypeMirrorData
	case awsp.MessageType_MESSAGE_TYPE_MIRROR_ACK:
		return MessageTypeMirrorAck
	case awsp.MessageType_MESSAGE_TYPE_PUSH_ENTRY:
		return MessageTypePushEntry
	case awsp.MessageType_MESSAGE_TYPE_PUSH_ACK:
		return MessageTypePushAck
	case awsp.MessageType_MESSAGE_TYPE_RATING_PUSH:
		return MessageTypeRatingPush
	case awsp.MessageType_MESSAGE_TYPE_RATING_ACK:
		return MessageTypeRatingAck
	case awsp.MessageType_MESSAGE_TYPE_HEARTBEAT:
		return MessageTypeHeartbeat
	case awsp.MessageType_MESSAGE_TYPE_BITFIELD:
		return MessageTypeBitfield
	default:
		return MessageTypeUnknown
	}
}

// ==================== NodeType Conversion ====================

// toProtoNodeType converts domain NodeType to proto NodeType.
func toProtoNodeType(t NodeType) awsp.NodeType {
	switch t {
	case NodeTypeSeed:
		return awsp.NodeType_NODE_TYPE_SEED
	default:
		return awsp.NodeType_NODE_TYPE_LOCAL
	}
}

// fromProtoNodeType converts proto NodeType to domain NodeType.
func fromProtoNodeType(t awsp.NodeType) NodeType {
	switch t {
	case awsp.NodeType_NODE_TYPE_SEED:
		return NodeTypeSeed
	default:
		return NodeTypeLocal
	}
}

// ==================== QueryType Conversion ====================

// toProtoQueryType converts domain QueryType to proto QueryType.
func toProtoQueryType(t QueryType) awsp.QueryType {
	switch t {
	case QueryTypeGlobal:
		return awsp.QueryType_QUERY_TYPE_GLOBAL
	default:
		return awsp.QueryType_QUERY_TYPE_LOCAL
	}
}

// fromProtoQueryType converts proto QueryType to domain QueryType.
func fromProtoQueryType(t awsp.QueryType) QueryType {
	switch t {
	case awsp.QueryType_QUERY_TYPE_GLOBAL:
		return QueryTypeGlobal
	default:
		return QueryTypeLocal
	}
}

// ==================== Handshake Conversion ====================

// toProtoHandshake converts domain Handshake to proto Handshake.
func toProtoHandshake(h *Handshake) *awsp.Handshake {
	if h == nil {
		return nil
	}
	return &awsp.Handshake{
		NodeId:     h.NodeID,
		PeerId:     h.PeerID,
		NodeType:   toProtoNodeType(h.NodeType),
		Version:    h.Version,
		Categories: h.Categories,
		EntryCount: h.EntryCount,
		Signature:  h.Signature,
	}
}

// fromProtoHandshake converts proto Handshake to domain Handshake.
func fromProtoHandshake(h *awsp.Handshake) *Handshake {
	if h == nil {
		return nil
	}
	return &Handshake{
		NodeID:     h.NodeId,
		PeerID:     h.PeerId,
		NodeType:   fromProtoNodeType(h.NodeType),
		Version:    h.Version,
		Categories: h.Categories,
		EntryCount: h.EntryCount,
		Signature:  h.Signature,
	}
}

// ==================== HandshakeAck Conversion ====================

// toProtoHandshakeAck converts domain HandshakeAck to proto HandshakeAck.
func toProtoHandshakeAck(h *HandshakeAck) *awsp.HandshakeAck {
	if h == nil {
		return nil
	}
	return &awsp.HandshakeAck{
		NodeId:       h.NodeID,
		PeerId:       h.PeerID,
		NodeType:     toProtoNodeType(h.NodeType),
		Version:      h.Version,
		Accepted:     h.Accepted,
		RejectReason: h.RejectReason,
		Signature:    h.Signature,
	}
}

// fromProtoHandshakeAck converts proto HandshakeAck to domain HandshakeAck.
func fromProtoHandshakeAck(h *awsp.HandshakeAck) *HandshakeAck {
	if h == nil {
		return nil
	}
	return &HandshakeAck{
		NodeID:       h.NodeId,
		PeerID:       h.PeerId,
		NodeType:     fromProtoNodeType(h.NodeType),
		Version:      h.Version,
		Accepted:     h.Accepted,
		RejectReason: h.RejectReason,
		Signature:    h.Signature,
	}
}

// ==================== Query Conversion ====================

// toProtoQuery converts domain Query to proto Query.
func toProtoQuery(q *Query) *awsp.Query {
	if q == nil {
		return nil
	}
	return &awsp.Query{
		QueryId:    q.QueryID,
		Keyword:    q.Keyword,
		Categories: q.Categories,
		Limit:      q.Limit,
		Offset:     q.Offset,
		QueryType:  toProtoQueryType(q.QueryType),
	}
}

// fromProtoQuery converts proto Query to domain Query.
func fromProtoQuery(q *awsp.Query) *Query {
	if q == nil {
		return nil
	}
	return &Query{
		QueryID:    q.QueryId,
		Keyword:    q.Keyword,
		Categories: q.Categories,
		Limit:      q.Limit,
		Offset:     q.Offset,
		QueryType:  fromProtoQueryType(q.QueryType),
	}
}

// ==================== QueryResult Conversion ====================

// toProtoQueryResult converts domain QueryResult to proto QueryResult.
func toProtoQueryResult(q *QueryResult) *awsp.QueryResult {
	if q == nil {
		return nil
	}
	return &awsp.QueryResult{
		QueryId:    q.QueryID,
		Entries:    q.Entries,
		TotalCount: q.TotalCount,
		HasMore:    q.HasMore,
	}
}

// fromProtoQueryResult converts proto QueryResult to domain QueryResult.
func fromProtoQueryResult(q *awsp.QueryResult) *QueryResult {
	if q == nil {
		return nil
	}
	return &QueryResult{
		QueryID:    q.QueryId,
		Entries:    q.Entries,
		TotalCount: q.TotalCount,
		HasMore:    q.HasMore,
	}
}

// ==================== SyncRequest Conversion ====================

// toProtoSyncRequest converts domain SyncRequest to proto SyncRequest.
func toProtoSyncRequest(r *SyncRequest) *awsp.SyncRequest {
	if r == nil {
		return nil
	}
	return &awsp.SyncRequest{
		RequestId:           r.RequestID,
		LastSyncTimestamp:   r.LastSyncTimestamp,
		VersionVector:       r.VersionVector,
		RequestedCategories: r.RequestedCategories,
	}
}

// fromProtoSyncRequest converts proto SyncRequest to domain SyncRequest.
func fromProtoSyncRequest(r *awsp.SyncRequest) *SyncRequest {
	if r == nil {
		return nil
	}
	return &SyncRequest{
		RequestID:           r.RequestId,
		LastSyncTimestamp:   r.LastSyncTimestamp,
		VersionVector:       r.VersionVector,
		RequestedCategories: r.RequestedCategories,
	}
}

// ==================== SyncResponse Conversion ====================

// toProtoSyncResponse converts domain SyncResponse to proto SyncResponse.
func toProtoSyncResponse(r *SyncResponse) *awsp.SyncResponse {
	if r == nil {
		return nil
	}
	return &awsp.SyncResponse{
		RequestId:           r.RequestID,
		NewEntries:          r.NewEntries,
		UpdatedEntries:      r.UpdatedEntries,
		DeletedEntryIds:     r.DeletedEntryIDs,
		NewRatings:          r.NewRatings,
		ServerVersionVector: r.ServerVersionVector,
		ServerTimestamp:     r.ServerTimestamp,
	}
}

// fromProtoSyncResponse converts proto SyncResponse to domain SyncResponse.
func fromProtoSyncResponse(r *awsp.SyncResponse) *SyncResponse {
	if r == nil {
		return nil
	}
	return &SyncResponse{
		RequestID:            r.RequestId,
		NewEntries:           r.NewEntries,
		UpdatedEntries:       r.UpdatedEntries,
		DeletedEntryIDs:      r.DeletedEntryIds,
		NewRatings:           r.NewRatings,
		ServerVersionVector:  r.ServerVersionVector,
		ServerTimestamp:      r.ServerTimestamp,
	}
}

// ==================== MirrorRequest Conversion ====================

// toProtoMirrorRequest converts domain MirrorRequest to proto MirrorRequest.
func toProtoMirrorRequest(r *MirrorRequest) *awsp.MirrorRequest {
	if r == nil {
		return nil
	}
	return &awsp.MirrorRequest{
		RequestId:  r.RequestID,
		Categories: r.Categories,
		FullMirror: r.FullMirror,
		BatchSize:  r.BatchSize,
	}
}

// fromProtoMirrorRequest converts proto MirrorRequest to domain MirrorRequest.
func fromProtoMirrorRequest(r *awsp.MirrorRequest) *MirrorRequest {
	if r == nil {
		return nil
	}
	return &MirrorRequest{
		RequestID:  r.RequestId,
		Categories: r.Categories,
		FullMirror: r.FullMirror,
		BatchSize:  r.BatchSize,
	}
}

// ==================== MirrorData Conversion ====================

// toProtoMirrorData converts domain MirrorData to proto MirrorData.
func toProtoMirrorData(d *MirrorData) *awsp.MirrorData {
	if d == nil {
		return nil
	}
	return &awsp.MirrorData{
		RequestId:    d.RequestID,
		BatchIndex:   d.BatchIndex,
		TotalBatches: d.TotalBatches,
		Entries:      d.Entries,
		Categories:   d.Categories,
	}
}

// fromProtoMirrorData converts proto MirrorData to domain MirrorData.
func fromProtoMirrorData(d *awsp.MirrorData) *MirrorData {
	if d == nil {
		return nil
	}
	return &MirrorData{
		RequestID:    d.RequestId,
		BatchIndex:   d.BatchIndex,
		TotalBatches: d.TotalBatches,
		Entries:      d.Entries,
		Categories:   d.Categories,
	}
}

// ==================== MirrorAck Conversion ====================

// toProtoMirrorAck converts domain MirrorAck to proto MirrorAck.
func toProtoMirrorAck(a *MirrorAck) *awsp.MirrorAck {
	if a == nil {
		return nil
	}
	return &awsp.MirrorAck{
		RequestId:       a.RequestID,
		Success:         a.Success,
		ErrorMessage:    a.ErrorMessage,
		ReceivedEntries: a.ReceivedEntries,
	}
}

// fromProtoMirrorAck converts proto MirrorAck to domain MirrorAck.
func fromProtoMirrorAck(a *awsp.MirrorAck) *MirrorAck {
	if a == nil {
		return nil
	}
	return &MirrorAck{
		RequestID:       a.RequestId,
		Success:         a.Success,
		ErrorMessage:    a.ErrorMessage,
		ReceivedEntries: a.ReceivedEntries,
	}
}

// ==================== PushEntry Conversion ====================

// toProtoPushEntry converts domain PushEntry to proto PushEntry.
func toProtoPushEntry(e *PushEntry) *awsp.PushEntry {
	if e == nil {
		return nil
	}
	return &awsp.PushEntry{
		EntryId:          e.EntryID,
		Entry:            e.Entry,
		CreatorSignature: e.CreatorSignature,
	}
}

// fromProtoPushEntry converts proto PushEntry to domain PushEntry.
func fromProtoPushEntry(e *awsp.PushEntry) *PushEntry {
	if e == nil {
		return nil
	}
	return &PushEntry{
		EntryID:          e.EntryId,
		Entry:            e.Entry,
		CreatorSignature: e.CreatorSignature,
	}
}

// ==================== PushAck Conversion ====================

// toProtoPushAck converts domain PushAck to proto PushAck.
func toProtoPushAck(a *PushAck) *awsp.PushAck {
	if a == nil {
		return nil
	}
	return &awsp.PushAck{
		EntryId:      a.EntryID,
		Accepted:     a.Accepted,
		RejectReason: a.RejectReason,
		NewVersion:   a.NewVersion,
	}
}

// fromProtoPushAck converts proto PushAck to domain PushAck.
func fromProtoPushAck(a *awsp.PushAck) *PushAck {
	if a == nil {
		return nil
	}
	return &PushAck{
		EntryID:      a.EntryId,
		Accepted:     a.Accepted,
		RejectReason: a.RejectReason,
		NewVersion:   a.NewVersion,
	}
}

// ==================== RatingPush Conversion ====================

// toProtoRatingPush converts domain RatingPush to proto RatingPush.
func toProtoRatingPush(r *RatingPush) *awsp.RatingPush {
	if r == nil {
		return nil
	}
	return &awsp.RatingPush{
		Rating:         r.Rating,
		RaterSignature: r.RaterSignature,
	}
}

// fromProtoRatingPush converts proto RatingPush to domain RatingPush.
func fromProtoRatingPush(r *awsp.RatingPush) *RatingPush {
	if r == nil {
		return nil
	}
	return &RatingPush{
		Rating:         r.Rating,
		RaterSignature: r.RaterSignature,
	}
}

// ==================== RatingAck Conversion ====================

// toProtoRatingAck converts domain RatingAck to proto RatingAck.
func toProtoRatingAck(a *RatingAck) *awsp.RatingAck {
	if a == nil {
		return nil
	}
	return &awsp.RatingAck{
		RatingId:     a.RatingID,
		Accepted:     a.Accepted,
		RejectReason: a.RejectReason,
	}
}

// fromProtoRatingAck converts proto RatingAck to domain RatingAck.
func fromProtoRatingAck(a *awsp.RatingAck) *RatingAck {
	if a == nil {
		return nil
	}
	return &RatingAck{
		RatingID:     a.RatingId,
		Accepted:     a.Accepted,
		RejectReason: a.RejectReason,
	}
}

// ==================== Heartbeat Conversion ====================

// toProtoHeartbeat converts domain Heartbeat to proto Heartbeat.
func toProtoHeartbeat(h *Heartbeat) *awsp.Heartbeat {
	if h == nil {
		return nil
	}
	return &awsp.Heartbeat{
		NodeId:        h.NodeID,
		UptimeSeconds: h.UptimeSeconds,
		EntryCount:    h.EntryCount,
		Timestamp:     h.Timestamp,
	}
}

// fromProtoHeartbeat converts proto Heartbeat to domain Heartbeat.
func fromProtoHeartbeat(h *awsp.Heartbeat) *Heartbeat {
	if h == nil {
		return nil
	}
	return &Heartbeat{
		NodeID:        h.NodeId,
		UptimeSeconds: h.UptimeSeconds,
		EntryCount:    h.EntryCount,
		Timestamp:     h.Timestamp,
	}
}

// ==================== Bitfield Conversion ====================

// toProtoBitfield converts domain Bitfield to proto Bitfield.
func toProtoBitfield(b *Bitfield) *awsp.Bitfield {
	if b == nil {
		return nil
	}
	return &awsp.Bitfield{
		NodeId:        b.NodeID,
		VersionVector: b.VersionVector,
		EntryCount:    b.EntryCount,
	}
}

// fromProtoBitfield converts proto Bitfield to domain Bitfield.
func fromProtoBitfield(b *awsp.Bitfield) *Bitfield {
	if b == nil {
		return nil
	}
	return &Bitfield{
		NodeID:        b.NodeId,
		VersionVector: b.VersionVector,
		EntryCount:    b.EntryCount,
	}
}

// ==================== Message Conversion ====================

// toProtoMessage converts domain Message to proto Message.
func toProtoMessage(msg *Message) *awsp.Message {
	if msg == nil {
		return nil
	}

	protoMsg := &awsp.Message{
		Header: &awsp.MessageHeader{
			Type:      toProtoMessageType(msg.Header.Type),
			MessageId: msg.Header.MessageID,
			Timestamp: msg.Header.Timestamp,
			Signature: msg.Header.Signature,
		},
	}

	// Set the appropriate payload based on message type
	switch payload := msg.Payload.(type) {
	case *Handshake:
		protoMsg.Payload = &awsp.Message_Handshake{
			Handshake: toProtoHandshake(payload),
		}
	case *HandshakeAck:
		protoMsg.Payload = &awsp.Message_HandshakeAck{
			HandshakeAck: toProtoHandshakeAck(payload),
		}
	case *Query:
		protoMsg.Payload = &awsp.Message_Query{
			Query: toProtoQuery(payload),
		}
	case *QueryResult:
		protoMsg.Payload = &awsp.Message_QueryResult{
			QueryResult: toProtoQueryResult(payload),
		}
	case *SyncRequest:
		protoMsg.Payload = &awsp.Message_SyncRequest{
			SyncRequest: toProtoSyncRequest(payload),
		}
	case *SyncResponse:
		protoMsg.Payload = &awsp.Message_SyncResponse{
			SyncResponse: toProtoSyncResponse(payload),
		}
	case *MirrorRequest:
		protoMsg.Payload = &awsp.Message_MirrorRequest{
			MirrorRequest: toProtoMirrorRequest(payload),
		}
	case *MirrorData:
		protoMsg.Payload = &awsp.Message_MirrorData{
			MirrorData: toProtoMirrorData(payload),
		}
	case *MirrorAck:
		protoMsg.Payload = &awsp.Message_MirrorAck{
			MirrorAck: toProtoMirrorAck(payload),
		}
	case *PushEntry:
		protoMsg.Payload = &awsp.Message_PushEntry{
			PushEntry: toProtoPushEntry(payload),
		}
	case *PushAck:
		protoMsg.Payload = &awsp.Message_PushAck{
			PushAck: toProtoPushAck(payload),
		}
	case *RatingPush:
		protoMsg.Payload = &awsp.Message_RatingPush{
			RatingPush: toProtoRatingPush(payload),
		}
	case *RatingAck:
		protoMsg.Payload = &awsp.Message_RatingAck{
			RatingAck: toProtoRatingAck(payload),
		}
	case *Heartbeat:
		protoMsg.Payload = &awsp.Message_Heartbeat{
			Heartbeat: toProtoHeartbeat(payload),
		}
	case *Bitfield:
		protoMsg.Payload = &awsp.Message_Bitfield{
			Bitfield: toProtoBitfield(payload),
		}
	}

	return protoMsg
}

// fromProtoMessage converts proto Message to domain Message.
func fromProtoMessage(msg *awsp.Message) *Message {
	if msg == nil {
		return nil
	}

	domainMsg := &Message{
		Header: &MessageHeader{
			Type:      fromProtoMessageType(msg.Header.GetType()),
			MessageID: msg.Header.GetMessageId(),
			Timestamp: msg.Header.GetTimestamp(),
			Signature: msg.Header.GetSignature(),
		},
	}

	// Extract the appropriate payload based on message type
	switch msg.Header.GetType() {
	case awsp.MessageType_MESSAGE_TYPE_HANDSHAKE:
		if h := msg.GetHandshake(); h != nil {
			domainMsg.Payload = fromProtoHandshake(h)
		}
	case awsp.MessageType_MESSAGE_TYPE_HANDSHAKE_ACK:
		if h := msg.GetHandshakeAck(); h != nil {
			domainMsg.Payload = fromProtoHandshakeAck(h)
		}
	case awsp.MessageType_MESSAGE_TYPE_QUERY:
		if q := msg.GetQuery(); q != nil {
			domainMsg.Payload = fromProtoQuery(q)
		}
	case awsp.MessageType_MESSAGE_TYPE_QUERY_RESULT:
		if q := msg.GetQueryResult(); q != nil {
			domainMsg.Payload = fromProtoQueryResult(q)
		}
	case awsp.MessageType_MESSAGE_TYPE_SYNC_REQUEST:
		if r := msg.GetSyncRequest(); r != nil {
			domainMsg.Payload = fromProtoSyncRequest(r)
		}
	case awsp.MessageType_MESSAGE_TYPE_SYNC_RESPONSE:
		if r := msg.GetSyncResponse(); r != nil {
			domainMsg.Payload = fromProtoSyncResponse(r)
		}
	case awsp.MessageType_MESSAGE_TYPE_MIRROR_REQUEST:
		if r := msg.GetMirrorRequest(); r != nil {
			domainMsg.Payload = fromProtoMirrorRequest(r)
		}
	case awsp.MessageType_MESSAGE_TYPE_MIRROR_DATA:
		if d := msg.GetMirrorData(); d != nil {
			domainMsg.Payload = fromProtoMirrorData(d)
		}
	case awsp.MessageType_MESSAGE_TYPE_MIRROR_ACK:
		if a := msg.GetMirrorAck(); a != nil {
			domainMsg.Payload = fromProtoMirrorAck(a)
		}
	case awsp.MessageType_MESSAGE_TYPE_PUSH_ENTRY:
		if e := msg.GetPushEntry(); e != nil {
			domainMsg.Payload = fromProtoPushEntry(e)
		}
	case awsp.MessageType_MESSAGE_TYPE_PUSH_ACK:
		if a := msg.GetPushAck(); a != nil {
			domainMsg.Payload = fromProtoPushAck(a)
		}
	case awsp.MessageType_MESSAGE_TYPE_RATING_PUSH:
		if r := msg.GetRatingPush(); r != nil {
			domainMsg.Payload = fromProtoRatingPush(r)
		}
	case awsp.MessageType_MESSAGE_TYPE_RATING_ACK:
		if a := msg.GetRatingAck(); a != nil {
			domainMsg.Payload = fromProtoRatingAck(a)
		}
	case awsp.MessageType_MESSAGE_TYPE_HEARTBEAT:
		if h := msg.GetHeartbeat(); h != nil {
			domainMsg.Payload = fromProtoHeartbeat(h)
		}
	case awsp.MessageType_MESSAGE_TYPE_BITFIELD:
		if b := msg.GetBitfield(); b != nil {
			domainMsg.Payload = fromProtoBitfield(b)
		}
	}

	return domainMsg
}
