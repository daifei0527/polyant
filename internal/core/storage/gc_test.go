package storage

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
)

// countingStore wraps a real PebbleStore to count RunGC calls.
type countingStore struct {
	kv.Store
	gcCalls int32
}

func (c *countingStore) RunGC() error { atomic.AddInt32(&c.gcCalls, 1); return c.Store.RunGC() }

func TestGarbageCollector_RunsOnInterval(t *testing.T) {
	real, err := kv.NewPebbleStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewPebbleStore: %v", err)
	}
	defer real.Close()
	cs := &countingStore{Store: real}

	gc := NewGarbageCollector(cs, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := gc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gc.Stop()

	time.Sleep(180 * time.Millisecond) // ~3-4 ticks
	if got := atomic.LoadInt32(&cs.gcCalls); got < 2 {
		t.Errorf("RunGC called %d times, want >=2", got)
	}
}

func TestGarbageCollector_StopCancels(t *testing.T) {
	real, _ := kv.NewPebbleStore(t.TempDir())
	defer real.Close()
	gc := NewGarbageCollector(real, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gc.Start(ctx)
	done := make(chan struct{})
	go func() { gc.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within 2s")
	}
}
