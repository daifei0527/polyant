package sync

import (
	"sync"
	"time"
)

// SyncTask 同步任务
type SyncTask struct {
	EntryID    string    `json:"entry_id"`
	Action     string    `json:"action"` // create, update, delete
	CreatedAt  time.Time `json:"created_at"`
	RetryCount int       `json:"retry_count"`
	MaxRetry   int       `json:"max_retry"`
	NextRetry  time.Time `json:"next_retry"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	EntryID      string    `json:"entry_id"`
	LocalSaved   bool      `json:"local_saved"`
	SyncedToSeed bool      `json:"synced_to_seed"`
	SyncedNodes  []string  `json:"synced_nodes"`
	RetryCount   int       `json:"retry_count"`
	LastSyncAt   time.Time `json:"last_sync_at"`
}

// SyncQueue 同步队列
type SyncQueue struct {
	pending  []*SyncTask
	retry    []*SyncTask
	statuses map[string]*SyncStatus
	mu       sync.Mutex
	offline  bool
}

// NewSyncQueue 创建同步队列
func NewSyncQueue() *SyncQueue {
	return &SyncQueue{
		pending:  make([]*SyncTask, 0),
		retry:    make([]*SyncTask, 0),
		statuses: make(map[string]*SyncStatus),
	}
}

// Add 添加同步任务
func (q *SyncQueue) Add(task *SyncTask) {
	if task == nil || task.EntryID == "" {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	q.pending = append(q.pending, task)

	// 初始化状态
	q.statuses[task.EntryID] = &SyncStatus{
		EntryID:    task.EntryID,
		LocalSaved: true,
	}
}

// Process 处理待同步任务
func (q *SyncQueue) Process(syncFn func(*SyncTask) error) {
	// Check offline mode and collect tasks under lock
	q.mu.Lock()
	if q.offline {
		q.mu.Unlock()
		return
	}

	now := time.Now()

	// Collect pending tasks to process
	tasksToProcess := make([]*SyncTask, len(q.pending))
	copy(tasksToProcess, q.pending)
	q.pending = q.pending[:0]

	// Collect retry tasks ready for processing
	var retryToProcess []*SyncTask
	var retryToKeep []*SyncTask
	for _, task := range q.retry {
		if now.After(task.NextRetry) {
			retryToProcess = append(retryToProcess, task)
		} else {
			retryToKeep = append(retryToKeep, task)
		}
	}
	q.retry = retryToKeep
	q.mu.Unlock()

	// Process tasks without holding the lock
	type taskResult struct {
		entryID string
		success bool
		retries int
		dropped bool
	}

	var results []taskResult

	// Process pending tasks
	for _, task := range tasksToProcess {
		if err := syncFn(task); err != nil {
			task.RetryCount++
			if task.RetryCount < task.MaxRetry {
				task.NextRetry = now.Add(time.Duration(task.RetryCount*task.RetryCount) * time.Second)
				results = append(results, taskResult{entryID: task.EntryID, success: false, retries: task.RetryCount})
			} else {
				// Task dropped due to max retries exceeded
				results = append(results, taskResult{entryID: task.EntryID, success: false, dropped: true})
			}
		} else {
			results = append(results, taskResult{entryID: task.EntryID, success: true})
		}
	}

	// Process retry tasks
	for _, task := range retryToProcess {
		if err := syncFn(task); err != nil {
			task.RetryCount++
			if task.RetryCount < task.MaxRetry {
				task.NextRetry = now.Add(time.Duration(task.RetryCount*task.RetryCount) * time.Second)
				results = append(results, taskResult{entryID: task.EntryID, success: false, retries: task.RetryCount})
			} else {
				// Task dropped due to max retries exceeded
				results = append(results, taskResult{entryID: task.EntryID, success: false, dropped: true})
			}
		} else {
			results = append(results, taskResult{entryID: task.EntryID, success: true})
		}
	}

	// Reacquire lock to update state
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, result := range results {
		if result.dropped {
			// Clean up status for dropped tasks
			delete(q.statuses, result.entryID)
			continue
		}

		status, ok := q.statuses[result.entryID]
		if !ok {
			continue
		}

		if result.success {
			status.SyncedToSeed = true
			status.LastSyncAt = now
		} else {
			status.RetryCount = result.retries
			// Find the task and add to retry queue
			for _, task := range tasksToProcess {
				if task.EntryID == result.entryID {
					q.retry = append(q.retry, task)
					break
				}
			}
			for _, task := range retryToProcess {
				if task.EntryID == result.entryID {
					q.retry = append(q.retry, task)
					break
				}
			}
		}
	}
}

// PendingCount 返回待处理任务数
func (q *SyncQueue) PendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

// RetryCount 返回重试任务数
func (q *SyncQueue) RetryCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.retry)
}

// GetStatus 获取同步状态
func (q *SyncQueue) GetStatus(entryID string) *SyncStatus {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.statuses[entryID]
}

// EnableOfflineMode 启用离线模式
func (q *SyncQueue) EnableOfflineMode() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.offline = true
}

// DisableOfflineMode 禁用离线模式
func (q *SyncQueue) DisableOfflineMode() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.offline = false
}

// IsOffline 返回是否离线模式
func (q *SyncQueue) IsOffline() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.offline
}

// ProcessPending 处理所有待处理任务（恢复在线时调用）
func (q *SyncQueue) ProcessPending(syncFn func(*SyncTask) error) {
	q.DisableOfflineMode()
	q.Process(syncFn)
}
