package election

import (
	"context"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAutoCloserTestService(t *testing.T) *ElectionService {
	store := NewMemoryStore()
	return NewElectionService(kv.NewElectionStore(store), kv.NewCandidateStore(store), kv.NewVoteStore(store))
}

// TestElectionAutoCloser_ClosesExpired: 到期的 active 选举应被自动关闭。
func TestElectionAutoCloser_ClosesExpired(t *testing.T) {
	svc := newAutoCloserTestService(t)
	ctx := context.Background()

	e, err := svc.CreateElection(ctx, "expired", "desc", "creator", 1, 0, false)
	require.NoError(t, err)
	// 手动把 EndTime 设到过去，模拟到期
	e.EndTime = time.Now().Add(-time.Hour).UnixMilli()
	require.NoError(t, svc.electionStore.Update(ctx, e))

	closer := NewElectionAutoCloser(svc, time.Hour)
	closer.closeExpired(ctx) // 同步触发

	got, err := svc.GetElection(ctx, e.ID)
	require.NoError(t, err)
	assert.Equal(t, model.ElectionStatusClosed, got.Status, "到期选举应被自动关闭")
}

// TestElectionAutoCloser_LeavesActive: 未到期的 active 选举不应被关闭。
func TestElectionAutoCloser_LeavesActive(t *testing.T) {
	svc := newAutoCloserTestService(t)
	ctx := context.Background()

	e, err := svc.CreateElection(ctx, "active", "desc", "creator", 1, 7, false) // 7 天后到期
	require.NoError(t, err)

	closer := NewElectionAutoCloser(svc, time.Hour)
	closer.closeExpired(ctx)

	got, err := svc.GetElection(ctx, e.ID)
	require.NoError(t, err)
	assert.Equal(t, model.ElectionStatusActive, got.Status, "未到期选举应保持 active")
}

// TestElectionAutoCloser_DefaultInterval: interval<=0 时使用默认值（不 panic、启动即跑）。
func TestElectionAutoCloser_DefaultInterval(t *testing.T) {
	svc := newAutoCloserTestService(t)
	closer := NewElectionAutoCloser(svc, 0)
	require.NoError(t, closer.Start(context.Background()))
	require.NoError(t, closer.Stop())
}
