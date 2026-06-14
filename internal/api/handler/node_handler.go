package handler

import (
	"context"
	"log"
	"net/http"
	"time"
)

// NodeHandler 节点 HTTP 处理器
// 负责节点状态查询和手动触发同步
type NodeHandler struct {
	nodeID      string
	nodeType    string
	version     string
	startTime   time.Time
	entryStore  EntryCounter
	syncTrigger SyncTrigger
	lastSync    int64
}

// EntryCounter 条目计数接口（仅需要 Count 方法）
type EntryCounter interface {
	Count(ctx interface{}) (int64, error)
}

// SyncTrigger 触发增量同步（由 sync.SyncEngine 实现）。
type SyncTrigger interface {
	IncrementalSync(ctx context.Context) error
}

// NewNodeHandler 创建新的 NodeHandler 实例
func NewNodeHandler(nodeID, nodeType, version string, entryStore interface{}) *NodeHandler {
	h := &NodeHandler{
		nodeID:    nodeID,
		nodeType:  nodeType,
		version:   version,
		startTime: time.Now(),
		lastSync:  0,
	}
	// 使用类型断言设置 entryStore
	if es, ok := entryStore.(EntryCounter); ok {
		h.entryStore = es
	}
	return h
}

// GetNodeStatusHandler 获取节点状态
// GET /api/v1/node/status
// 返回节点基本信息、运行时长、条目数量等
func (h *NodeHandler) GetNodeStatusHandler(w http.ResponseWriter, r *http.Request) {
	uptime := int64(time.Since(h.startTime).Seconds())

	var entryCount int64
	if h.entryStore != nil {
		entryCount, _ = h.entryStore.Count(r.Context())
	}

	resp := &NodeStatusResponse{
		NodeID:     h.nodeID,
		NodeType:   h.nodeType,
		Version:    h.version,
		EntryCount: entryCount,
		Uptime:     uptime,
		LastSync:   h.lastSync,
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    resp,
	})
}

// TriggerSyncHandler 触发手动同步
// GET /api/v1/node/sync
// 触发节点与种子节点之间的数据同步
func (h *NodeHandler) TriggerSyncHandler(w http.ResponseWriter, r *http.Request) {
	h.lastSync = time.Now().UnixMilli()

	// 异步发起增量同步（失败仅记日志，不阻塞响应；panic 不杀进程）
	if h.syncTrigger != nil {
		go func() {
			defer func() {
				if rv := recover(); rv != nil {
					log.Printf("[NodeHandler] IncrementalSync panic: %v", rv)
				}
			}()
			if err := h.syncTrigger.IncrementalSync(context.Background()); err != nil {
				log.Printf("[NodeHandler] IncrementalSync failed: %v", err)
			}
		}()
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "sync triggered",
		Data: map[string]interface{}{
			"triggered_at": h.lastSync,
			"status":       "syncing",
		},
	})
}

// SetLastSync 设置最后同步时间（供外部调用）
func (h *NodeHandler) SetLastSync(t int64) {
	h.lastSync = t
}

// SetSyncTrigger 注入同步触发器，使 /node/sync 真正发起增量同步。
func (h *NodeHandler) SetSyncTrigger(t SyncTrigger) {
	h.syncTrigger = t
}
