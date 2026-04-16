package protocol

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// MockProtocol 用于测试的 Mock 实现
// 注意：此类型设计为单线程测试使用
type MockProtocol struct {
	mu sync.RWMutex

	// 可编程的响应
	handshakeAck  *HandshakeAck
	handshakeError error
	syncResponse   *SyncResponse
	syncError      error
	queryResult    *QueryResult
	queryError     error
	pushAck        *PushAck
	pushError      error
	ratingAck      *RatingAck
	ratingError    error

	// 记录调用
	handshakeRequests []*Handshake
	syncRequests      []*SyncRequest
	queryRequests     []*Query
	pushRequests      []*PushEntry
	ratingRequests    []*RatingPush
}

// NewMockProtocol 创建 Mock 协议
func NewMockProtocol() *MockProtocol {
	return &MockProtocol{
		handshakeRequests: make([]*Handshake, 0),
		syncRequests:      make([]*SyncRequest, 0),
		queryRequests:     make([]*Query, 0),
		pushRequests:      make([]*PushEntry, 0),
		ratingRequests:    make([]*RatingPush, 0),
	}
}

// SendHandshake 发送握手请求
func (m *MockProtocol) SendHandshake(ctx context.Context, peerID peer.ID, h *Handshake) (*HandshakeAck, error) {
	m.mu.Lock()
	m.handshakeRequests = append(m.handshakeRequests, h)
	m.mu.Unlock()

	if m.handshakeError != nil {
		return nil, m.handshakeError
	}
	return m.handshakeAck, nil
}

// SendSyncRequest 发送同步请求
func (m *MockProtocol) SendSyncRequest(ctx context.Context, peerID peer.ID, req *SyncRequest) (*SyncResponse, error) {
	m.mu.Lock()
	m.syncRequests = append(m.syncRequests, req)
	m.mu.Unlock()

	if m.syncError != nil {
		return nil, m.syncError
	}
	return m.syncResponse, nil
}

// SendQuery 发送查询
func (m *MockProtocol) SendQuery(ctx context.Context, peerID peer.ID, req *Query) (*QueryResult, error) {
	m.mu.Lock()
	m.queryRequests = append(m.queryRequests, req)
	m.mu.Unlock()

	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.queryResult, nil
}

// SendPushEntry 发送条目推送
func (m *MockProtocol) SendPushEntry(ctx context.Context, peerID peer.ID, req *PushEntry) (*PushAck, error) {
	m.mu.Lock()
	m.pushRequests = append(m.pushRequests, req)
	m.mu.Unlock()

	if m.pushError != nil {
		return nil, m.pushError
	}
	return m.pushAck, nil
}

// SendRatingPush 发送评分推送
func (m *MockProtocol) SendRatingPush(ctx context.Context, peerID peer.ID, req *RatingPush) (*RatingAck, error) {
	m.mu.Lock()
	m.ratingRequests = append(m.ratingRequests, req)
	m.mu.Unlock()

	if m.ratingError != nil {
		return nil, m.ratingError
	}
	return m.ratingAck, nil
}

// --- 测试辅助方法 ---

// SetHandshakeAck 设置握手响应
func (m *MockProtocol) SetHandshakeAck(ack *HandshakeAck) {
	m.handshakeAck = ack
}

// SetHandshakeError 设置握手错误
func (m *MockProtocol) SetHandshakeError(err error) {
	m.handshakeError = err
}

// SetSyncResponse 设置同步响应
func (m *MockProtocol) SetSyncResponse(resp *SyncResponse) {
	m.syncResponse = resp
}

// SetSyncError 设置同步错误
func (m *MockProtocol) SetSyncError(err error) {
	m.syncError = err
}

// SetQueryResult 设置查询结果
func (m *MockProtocol) SetQueryResult(result *QueryResult) {
	m.queryResult = result
}

// SetQueryError 设置查询错误
func (m *MockProtocol) SetQueryError(err error) {
	m.queryError = err
}

// SetPushAck 设置推送确认
func (m *MockProtocol) SetPushAck(ack *PushAck) {
	m.pushAck = ack
}

// SetPushError 设置推送错误
func (m *MockProtocol) SetPushError(err error) {
	m.pushError = err
}

// SetRatingAck 设置评分确认
func (m *MockProtocol) SetRatingAck(ack *RatingAck) {
	m.ratingAck = ack
}

// SetRatingError 设置评分错误
func (m *MockProtocol) SetRatingError(err error) {
	m.ratingError = err
}

// GetSyncRequests 获取同步请求记录
func (m *MockProtocol) GetSyncRequests() []*SyncRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncRequests
}

// GetHandshakeRequests 获取握手请求记录
func (m *MockProtocol) GetHandshakeRequests() []*Handshake {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.handshakeRequests
}

// Reset 重置所有状态
func (m *MockProtocol) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handshakeRequests = make([]*Handshake, 0)
	m.syncRequests = make([]*SyncRequest, 0)
	m.queryRequests = make([]*Query, 0)
	m.pushRequests = make([]*PushEntry, 0)
	m.ratingRequests = make([]*RatingPush, 0)
	m.handshakeAck = nil
	m.handshakeError = nil
	m.syncResponse = nil
	m.syncError = nil
	m.queryResult = nil
	m.queryError = nil
	m.pushAck = nil
	m.pushError = nil
	m.ratingAck = nil
	m.ratingError = nil
}

// 确保 MockProtocol 实现接口
var _ ProtocolInterface = (*MockProtocol)(nil)
