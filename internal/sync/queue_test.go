package sync

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSyncQueue(t *testing.T) {
	q := NewSyncQueue()

	// 测试添加任务
	task := &SyncTask{
		EntryID:   "entry-1",
		Action:    "create",
		CreatedAt: time.Now(),
		MaxRetry:  3,
	}

	q.Add(task)
	assert.Equal(t, 1, q.PendingCount())
}

func TestSyncQueueProcess(t *testing.T) {
	q := NewSyncQueue()

	task1 := &SyncTask{
		EntryID:   "entry-1",
		Action:    "create",
		CreatedAt: time.Now(),
		MaxRetry:  3,
	}
	task2 := &SyncTask{
		EntryID:   "entry-2",
		Action:    "update",
		CreatedAt: time.Now(),
		MaxRetry:  3,
	}

	q.Add(task1)
	q.Add(task2)

	// 模拟处理
	processed := 0
	q.Process(func(task *SyncTask) error {
		processed++
		return nil
	})

	assert.Equal(t, 2, processed)
	assert.Equal(t, 0, q.PendingCount())
}

func TestSyncQueueRetry(t *testing.T) {
	q := NewSyncQueue()

	task := &SyncTask{
		EntryID:   "entry-1",
		Action:    "create",
		CreatedAt: time.Now(),
		MaxRetry:  2,
	}

	q.Add(task)

	// 第一次处理失败
	callCount := 0
	q.Process(func(task *SyncTask) error {
		callCount++
		if callCount < 2 {
			return fmt.Errorf("network error")
		}
		return nil
	})

	// 应该有一次待重试
	assert.Equal(t, 1, q.RetryCount())
}

func TestSyncStatus(t *testing.T) {
	status := &SyncStatus{
		EntryID:      "entry-1",
		LocalSaved:   true,
		SyncedToSeed: false,
		RetryCount:   0,
	}

	assert.True(t, status.LocalSaved)
	assert.False(t, status.SyncedToSeed)
}
