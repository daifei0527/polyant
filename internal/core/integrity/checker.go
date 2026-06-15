// Package integrity 提供知识条目的周期性完整性校验。
//
// IntegrityChecker 是一个后台守护进程，周期遍历已发布条目，重算 ComputeContentHash
// 并与存储的 ContentHash 比对。不一致意味着条目在落盘后被直接篡改（绕过会重算哈希的
// Create/Update 路径），或写入了哈希不一致的条目。发现不一致即记录审计日志并告警。
//
// 注意：本守护进程只覆盖内容哈希完整性（rel-data-integrity-sha256）。条目内容的
// 创作者签名校验（抗主动篡改）依赖签名架构，属未来工作（见选举设计 spec 的已知限制）。
package integrity

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// AuditRecorder 记录完整性异常事件。internal/core/audit.Service 隐式满足此接口；
// 为 nil 时守护进程仅输出告警日志（便于测试与未配置审计的环境）。
type AuditRecorder interface {
	Log(ctx context.Context, log *model.AuditLog) error
}

// IntegrityChecker 知识完整性守护进程。照搬 ElectionAutoCloser / LevelUpgradeChecker
// 的后台任务范式：Start 派生 cancelable ctx 并起 loop goroutine，Stop cancel+Wait。
type IntegrityChecker struct {
	store     *storage.Store
	audit     AuditRecorder
	interval  time.Duration
	batchSize int

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewIntegrityChecker 创建完整性守护进程。interval<=0 默认 15 分钟；batchSize<=0 默认 500。
func NewIntegrityChecker(store *storage.Store, audit AuditRecorder, interval time.Duration, batchSize int) *IntegrityChecker {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 500
	}
	return &IntegrityChecker{
		store:     store,
		audit:     audit,
		interval:  interval,
		batchSize: batchSize,
	}
}

// Start 启动后台周期校验（启动时立即跑一次，之后按 interval 周期执行）。
func (c *IntegrityChecker) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go c.loop(ctx)
	log.Printf("[IntegrityChecker] Started, interval: %v, batch size: %d", c.interval, c.batchSize)
	return nil
}

// Stop 停止后台任务并等待退出。
func (c *IntegrityChecker) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return nil
}

func (c *IntegrityChecker) loop(ctx context.Context) {
	defer c.wg.Done()
	c.checkOnce(ctx) // 启动时立即扫一次
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkOnce(ctx)
		}
	}
}

// checkOnce 分页扫描所有已发布条目并逐条校验。单次循环内 recover 防止 panic 杀进程。
func (c *IntegrityChecker) checkOnce(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[IntegrityChecker] panic during check, aborted this cycle: %v", r)
		}
	}()

	offset := 0
	for {
		entries, total, err := c.store.Entry.List(ctx, storage.EntryFilter{
			Status: model.EntryStatusPublished,
			Limit:  c.batchSize,
			Offset: offset,
		})
		if err != nil {
			log.Printf("[IntegrityChecker] list entries failed (offset=%d): %v", offset, err)
			return
		}

		for _, e := range entries {
			c.verifyEntry(ctx, e)
		}

		offset += len(entries)
		if len(entries) == 0 || offset >= int(total) {
			break
		}
	}
}

// verifyEntry 校验单条目的 ContentHash。空 ContentHash 视为未受完整性保护（旧数据），跳过。
func (c *IntegrityChecker) verifyEntry(ctx context.Context, e *model.KnowledgeEntry) {
	if e.ContentHash == "" {
		return
	}

	expected := e.ComputeContentHash()
	if e.ContentHash == expected {
		return
	}

	log.Printf("[IntegrityChecker] TAMPER DETECTED entry=%s stored_hash=%s recomputed=%s",
		e.ID, e.ContentHash, expected)

	if c.audit != nil {
		al := model.NewAuditLog()
		al.ActionType = "integrity.tamper"
		al.TargetType = "entry"
		al.TargetID = e.ID
		al.Success = false
		al.ErrorMessage = "content hash mismatch: stored=" + e.ContentHash + " recomputed=" + expected
		if err := c.audit.Log(ctx, al); err != nil {
			log.Printf("[IntegrityChecker] failed to record audit for entry %s: %v", e.ID, err)
		}
	}
}
