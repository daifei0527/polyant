package kv_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// errStore 实现 kv.Store，Get 恒返回一个非 ErrKeyNotFound 的错误，用于测试错误传播。
type errStore struct{ err error }

func (s *errStore) Put(k, v []byte) error                    { return nil }
func (s *errStore) Get(k []byte) ([]byte, error)             { return nil, s.err }
func (s *errStore) Delete(k []byte) error                    { return nil }
func (s *errStore) Scan(p []byte) (map[string][]byte, error) { return nil, nil }
func (s *errStore) Close() error                             { return nil }

// TestKVCandidateStore_UpdateVoteCount_Concurrent: 并发投票计数不得丢失。
func TestKVCandidateStore_UpdateVoteCount_Concurrent(t *testing.T) {
	store := NewMemoryStore()
	cs := kv.NewCandidateStore(store)
	ctx := context.Background()

	if err := cs.Add(ctx, &model.Candidate{ElectionID: "e1", UserID: "u1", VoteCount: 0}); err != nil {
		t.Fatalf("Add candidate: %v", err)
	}

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := cs.UpdateVoteCount(ctx, "e1", "u1", 1); err != nil {
				t.Errorf("UpdateVoteCount: %v", err)
			}
		}()
	}
	wg.Wait()

	got, err := cs.Get(ctx, "e1", "u1")
	if err != nil {
		t.Fatalf("Get candidate: %v", err)
	}
	if got.VoteCount != int32(n) {
		t.Errorf("并发计票丢失（UpdateVoteCount 非原子）: got %d, want %d", got.VoteCount, n)
	}
}

// TestKVVoteStore_HasVoted_PropagatesError: KV 故障时 HasVoted 必须传播错误，
// 而非吞掉错误当作"未投"（否则故障期间会允许重复投票）。
func TestKVVoteStore_HasVoted_PropagatesError(t *testing.T) {
	vs := kv.NewVoteStore(&errStore{err: fmt.Errorf("disk failure")})
	got, err := vs.HasVoted(context.Background(), "voter1", "e1")
	if err == nil {
		t.Fatal("HasVoted 应传播非 ErrKeyNotFound 的存储错误，而非当作未投")
	}
	if got {
		t.Error("存储错误时 HasVoted 应返回 false")
	}
}

// TestKVVoteStore_HasVoted_NotVoted: 未投票时返回 (false, nil)。
func TestKVVoteStore_HasVoted_NotVoted(t *testing.T) {
	vs := kv.NewVoteStore(NewMemoryStore())
	got, err := vs.HasVoted(context.Background(), "voter1", "e1")
	if err != nil {
		t.Fatalf("HasVoted 未投票不应出错: %v", err)
	}
	if got {
		t.Error("未投票时 HasVoted 应返回 false")
	}
}
