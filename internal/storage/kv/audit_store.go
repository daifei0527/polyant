package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

const auditPrefix = "audit:"

// AuditStore 审计日志存储接口
type AuditStore interface {
	Create(ctx context.Context, log *model.AuditLog) error
	Get(ctx context.Context, id string) (*model.AuditLog, error)
	List(ctx context.Context, filter model.AuditFilter) ([]*model.AuditLog, int64, error)
	DeleteBefore(ctx context.Context, timestamp int64) (int64, error)
	GetStats(ctx context.Context) (*model.AuditStats, error)
}

// KVAuditStore KV 审计日志存储实现
type KVAuditStore struct {
	kv Store
	mu sync.RWMutex
}

// NewAuditStore 创建审计日志存储
func NewAuditStore(kv Store) *KVAuditStore {
	return &KVAuditStore{kv: kv}
}

func (s *KVAuditStore) Create(ctx context.Context, log *model.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("marshal audit log: %w", err)
	}

	// 键格式: audit:{timestamp}:{id}
	// 使用时间戳倒序（用一个大数减去时间戳）便于按时间倒序查询
	key := []byte(fmt.Sprintf("%s%019d:%s", auditPrefix, maxTimestamp-log.Timestamp, log.ID))
	return s.kv.Put(key, data)
}

func (s *KVAuditStore) Get(ctx context.Context, id string) (*model.AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 需要扫描查找，因为 ID 嵌入在键中
	prefix := []byte(auditPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, fmt.Errorf("scan audit logs: %w", err)
	}

	for key, data := range items {
		var log model.AuditLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}
		if log.ID == id {
			return &log, nil
		}
		// 键中包含 ID，检查键
		if strings.Contains(string(key), id) {
			return &log, nil
		}
	}

	return nil, fmt.Errorf("audit log not found: %s", id)
}

func (s *KVAuditStore) List(ctx context.Context, filter model.AuditFilter) ([]*model.AuditLog, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := []byte(auditPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, 0, fmt.Errorf("scan audit logs: %w", err)
	}

	// 解析并过滤
	var logs []*model.AuditLog
	for _, data := range items {
		var log model.AuditLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}

		// 应用过滤器
		if !s.matchFilter(&log, filter) {
			continue
		}

		logs = append(logs, &log)
	}

	// 按时间戳倒序排序（最新的在前）
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp > logs[j].Timestamp
	})

	total := int64(len(logs))

	// 应用分页
	if filter.Offset > 0 {
		if filter.Offset >= len(logs) {
			return []*model.AuditLog{}, total, nil
		}
		logs = logs[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(logs) {
		logs = logs[:filter.Limit]
	}

	return logs, total, nil
}

func (s *KVAuditStore) matchFilter(log *model.AuditLog, filter model.AuditFilter) bool {
	// 操作者过滤
	if filter.OperatorPubkey != "" && log.OperatorPubkey != filter.OperatorPubkey {
		return false
	}

	// 操作类型过滤
	if len(filter.ActionTypes) > 0 {
		found := false
		for _, t := range filter.ActionTypes {
			if log.ActionType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 目标 ID 过滤
	if filter.TargetID != "" && log.TargetID != filter.TargetID {
		return false
	}

	// 成功/失败过滤
	if filter.Success != nil && log.Success != *filter.Success {
		return false
	}

	// 时间范围过滤
	if filter.StartTime > 0 && log.Timestamp < filter.StartTime {
		return false
	}
	if filter.EndTime > 0 && log.Timestamp > filter.EndTime {
		return false
	}

	return true
}

func (s *KVAuditStore) DeleteBefore(ctx context.Context, timestamp int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix := []byte(auditPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return 0, fmt.Errorf("scan audit logs: %w", err)
	}

	var deleted int64
	for key, data := range items {
		var log model.AuditLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}
		if log.Timestamp < timestamp {
			if err := s.kv.Delete([]byte(key)); err != nil {
				continue
			}
			deleted++
		}
	}

	return deleted, nil
}

func (s *KVAuditStore) GetStats(ctx context.Context) (*model.AuditStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := []byte(auditPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, fmt.Errorf("scan audit logs: %w", err)
	}

	stats := &model.AuditStats{
		ActionCounts: make(map[string]int64),
	}

	// 计算今日开始时间戳
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixMilli()

	for _, data := range items {
		var log model.AuditLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}

		stats.TotalLogs++
		if log.Timestamp >= todayStart {
			stats.TodayLogs++
		}
		if !log.Success {
			stats.FailedCount++
		}
		stats.ActionCounts[log.ActionType]++
	}

	return stats, nil
}

// 用于时间戳倒序存储
const maxTimestamp = 9999999999999 // 最大的13位时间戳
