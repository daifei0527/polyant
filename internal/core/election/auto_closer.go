package election

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/storage/model"
)

// ElectionAutoCloser 周期检查到期的 active 选举并自动关闭（结算当选者）。
// 选举 EndTime 到期后若仍为 active，由本后台任务调用 CloseElection 关闭并结算，
// 避免过期选举永远停留在 active 状态。
type ElectionAutoCloser struct {
	svc      *ElectionService
	interval time.Duration
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewElectionAutoCloser 创建选举自动关闭器。interval<=0 时默认 5 分钟。
func NewElectionAutoCloser(svc *ElectionService, interval time.Duration) *ElectionAutoCloser {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &ElectionAutoCloser{svc: svc, interval: interval}
}

// Start 启动后台周期任务（启动时立即跑一次，之后按 interval 周期执行）。
func (c *ElectionAutoCloser) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go c.loop(ctx)
	return nil
}

// Stop 停止后台任务并等待退出。
func (c *ElectionAutoCloser) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return nil
}

func (c *ElectionAutoCloser) loop(ctx context.Context) {
	defer c.wg.Done()
	c.closeExpired(ctx) // 启动时立即扫一次
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.closeExpired(ctx)
		}
	}
}

// closeExpired 关闭所有已到期（IsExpired）但仍为 active 的选举。
func (c *ElectionAutoCloser) closeExpired(ctx context.Context) {
	elections, err := c.svc.ListElections(ctx, model.ElectionStatusActive)
	if err != nil {
		log.Printf("[ElectionAutoCloser] list active elections failed: %v", err)
		return
	}
	for _, e := range elections {
		if !e.IsExpired() {
			continue
		}
		if _, err := c.svc.CloseElection(ctx, e.ID); err != nil {
			log.Printf("[ElectionAutoCloser] close expired election %s failed: %v", e.ID, err)
		} else {
			log.Printf("[ElectionAutoCloser] auto-closed expired election %s", e.ID)
		}
	}
}
