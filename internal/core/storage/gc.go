// Package storage 提供存储层的后台维护任务。
//
// GarbageCollector 周期调用 kv.Store.RunGC 回收空间（Pebble Compact /
// Badger RunValueLogGC），与 IntegrityChecker 保持相同的后台任务范式：
// cancel+wg、Start/Stop、ticker+select 循环、每周期 recover 防止 panic 杀进程。
package storage

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
)

// GarbageCollector periodically calls kv.Store.RunGC to reclaim space.
type GarbageCollector struct {
	kvStore  kv.Store
	interval time.Duration
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewGarbageCollector returns a GC runner. interval <=0 disables (Start is a no-op).
func NewGarbageCollector(kvStore kv.Store, interval time.Duration) *GarbageCollector {
	return &GarbageCollector{kvStore: kvStore, interval: interval}
}

// Start starts the periodic GC loop. If interval <=0, logs and returns nil (disabled).
func (g *GarbageCollector) Start(ctx context.Context) error {
	if g.interval <= 0 {
		log.Printf("[GarbageCollector] disabled (interval <= 0)")
		return nil
	}
	ctx, g.cancel = context.WithCancel(ctx)
	g.wg.Add(1)
	go g.loop(ctx)
	log.Printf("[GarbageCollector] started, interval: %v", g.interval)
	return nil
}

// Stop cancels the GC loop and waits for it to finish.
func (g *GarbageCollector) Stop() error {
	if g.cancel != nil {
		g.cancel()
	}
	g.wg.Wait()
	return nil
}

func (g *GarbageCollector) loop(ctx context.Context) {
	defer g.wg.Done()
	g.runOnce() // run on startup
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.runOnce()
		}
	}
}

func (g *GarbageCollector) runOnce() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[GarbageCollector] panic during GC, aborted this cycle: %v", r)
		}
	}()
	if err := g.kvStore.RunGC(); err != nil {
		log.Printf("[GarbageCollector] RunGC error: %v", err)
	}
}
