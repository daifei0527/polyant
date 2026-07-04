package handler

import (
	"context"
	"errors"

	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// assertErrIndexFail 是 mockSearchEngine 注入用的索引失败哨兵错误。
var assertErrIndexFail = errors.New("mock index failure")

// mockSearchEngine 实现 index.SearchEngine 用于测试，可注入错误并统计调用次数。
type mockSearchEngine struct {
	indexErr     error // IndexEntry/UpdateIndex/DeleteIndex 返回的错误
	indexCalls   int   // IndexEntry 调用次数
	updateCalls  int   // UpdateIndex 调用次数
	deleteCalls  int   // DeleteIndex 调用次数
	searchResult *index.SearchResult
	searchErr    error
}

func (m *mockSearchEngine) IndexEntry(entry *model.KnowledgeEntry) error {
	m.indexCalls++
	return m.indexErr
}

func (m *mockSearchEngine) UpdateIndex(entry *model.KnowledgeEntry) error {
	m.updateCalls++
	return m.indexErr
}

func (m *mockSearchEngine) DeleteIndex(entryID string) error {
	m.deleteCalls++
	return m.indexErr
}

func (m *mockSearchEngine) Search(ctx context.Context, q index.SearchQuery) (*index.SearchResult, error) {
	if m.searchResult != nil {
		return m.searchResult, m.searchErr
	}
	return &index.SearchResult{Entries: []*model.KnowledgeEntry{}}, m.searchErr
}

func (m *mockSearchEngine) Close() error { return nil }
