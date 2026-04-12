package sync

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/daifei0527/agentwiki/internal/network/host"
	"github.com/daifei0527/agentwiki/internal/network/protocol"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/index"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RemoteQueryService 远程查询服务
// 负责向种子节点发送查询请求并合并结果
type RemoteQueryService struct {
	p2pHost  *host.P2PHost
	protocol *protocol.Protocol
	store    *storage.Store
	config   *RemoteQueryConfig
	mu       sync.RWMutex
}

// RemoteQueryConfig 远程查询配置
type RemoteQueryConfig struct {
	// EnableRemoteQuery 是否启用远程查询
	EnableRemoteQuery bool
	// MinLocalResults 本地结果少于此数量时才查询远程
	MinLocalResults int
	// QueryTimeout 单次远程查询超时
	QueryTimeout time.Duration
	// MaxRemotePeers 最多查询多少个远程节点
	MaxRemotePeers int
	// CacheResults 是否缓存远程结果到本地
	CacheResults bool
}

// DefaultRemoteQueryConfig 默认配置
func DefaultRemoteQueryConfig() *RemoteQueryConfig {
	return &RemoteQueryConfig{
		EnableRemoteQuery: true,
		MinLocalResults:   10,
		QueryTimeout:      5 * time.Second,
		MaxRemotePeers:    3,
		CacheResults:      true,
	}
}

// NewRemoteQueryService 创建远程查询服务
func NewRemoteQueryService(p2pHost *host.P2PHost, proto *protocol.Protocol, store *storage.Store, cfg *RemoteQueryConfig) *RemoteQueryService {
	if cfg == nil {
		cfg = DefaultRemoteQueryConfig()
	}
	return &RemoteQueryService{
		p2pHost:  p2pHost,
		protocol: proto,
		store:    store,
		config:   cfg,
	}
}

// SetProtocol 设置协议处理器
func (s *RemoteQueryService) SetProtocol(proto *protocol.Protocol) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.protocol = proto
}

// SearchWithRemote 本地搜索并在结果不足时查询远程
func (s *RemoteQueryService) SearchWithRemote(ctx context.Context, query index.SearchQuery) (*index.SearchResult, error) {
	// 1. 先执行本地搜索
	localResult, err := s.store.Search.Search(ctx, query)
	if err != nil {
		localResult = &index.SearchResult{}
	}

	// 2. 检查是否需要远程查询
	if !s.config.EnableRemoteQuery || len(localResult.Entries) >= s.config.MinLocalResults {
		return localResult, nil
	}

	// 3. 查询远程节点
	remoteEntries := s.queryRemote(ctx, query)

	// 4. 合并结果
	merged := s.mergeResults(localResult.Entries, remoteEntries, query.Limit)

	// 5. 缓存远程结果
	if s.config.CacheResults && len(remoteEntries) > 0 {
		go s.cacheRemoteEntries(context.Background(), remoteEntries)
	}

	return &index.SearchResult{
		TotalCount: len(merged),
		HasMore:    len(merged) > query.Limit,
		Entries:    merged,
	}, nil
}

// queryRemote 向远程节点发送查询请求
func (s *RemoteQueryService) queryRemote(ctx context.Context, query index.SearchQuery) []*model.KnowledgeEntry {
	s.mu.RLock()
	proto := s.protocol
	s.mu.RUnlock()

	if proto == nil {
		return nil
	}

	// 获取已连接的节点
	peers := s.p2pHost.GetConnectedPeers()
	if len(peers) == 0 {
		return nil
	}

	// 限制查询节点数量
	maxPeers := s.config.MaxRemotePeers
	if len(peers) < maxPeers {
		maxPeers = len(peers)
	}

	// 并发查询多个节点
	var wg sync.WaitGroup
	resultsChan := make(chan []*model.KnowledgeEntry, maxPeers)

	queryCtx, cancel := context.WithTimeout(ctx, s.config.QueryTimeout)
	defer cancel()

	for i := 0; i < maxPeers; i++ {
		wg.Add(1)
		go func(peerID peer.ID) {
			defer wg.Done()
			entries := s.queryPeer(queryCtx, peerID, query)
			if len(entries) > 0 {
				resultsChan <- entries
			}
		}(peers[i])
	}

	// 等待所有查询完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 收集所有结果
	var allEntries []*model.KnowledgeEntry
	seen := make(map[string]bool)

	for entries := range resultsChan {
		for _, e := range entries {
			if !seen[e.ID] {
				seen[e.ID] = true
				allEntries = append(allEntries, e)
			}
		}
	}

	return allEntries
}

// queryPeer 查询单个节点
func (s *RemoteQueryService) queryPeer(ctx context.Context, peerID peer.ID, query index.SearchQuery) []*model.KnowledgeEntry {
	s.mu.RLock()
	proto := s.protocol
	s.mu.RUnlock()

	if proto == nil {
		return nil
	}

	// 构建查询请求
	req := &protocol.Query{
		QueryID:    generateQueryID(),
		Keyword:    query.Keyword,
		Categories: query.Categories,
		Limit:      int32(query.Limit),
		Offset:     int32(query.Offset),
		QueryType:  protocol.QueryTypeGlobal,
	}

	// 发送查询
	result, err := proto.SendQuery(ctx, peerID, req)
	if err != nil {
		log.Printf("[RemoteQuery] Failed to query peer %s: %v", peerID.String(), err)
		return nil
	}

	// 解析结果
	var entries []*model.KnowledgeEntry
	for _, data := range result.Entries {
		var entry model.KnowledgeEntry
		if err := entry.FromJSON(data); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}

	return entries
}

// mergeResults 合并本地和远程结果
func (s *RemoteQueryService) mergeResults(local, remote []*model.KnowledgeEntry, limit int) []*model.KnowledgeEntry {
	// 去重
	seen := make(map[string]bool)
	var merged []*model.KnowledgeEntry

	// 先添加本地结果
	for _, e := range local {
		if !seen[e.ID] {
			seen[e.ID] = true
			merged = append(merged, e)
		}
	}

	// 添加远程结果
	for _, e := range remote {
		if !seen[e.ID] {
			seen[e.ID] = true
			merged = append(merged, e)
		}
	}

	// 按评分排序
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	// 限制返回数量
	if len(merged) > limit {
		merged = merged[:limit]
	}

	return merged
}

// cacheRemoteEntries 缓存远程条目到本地
func (s *RemoteQueryService) cacheRemoteEntries(ctx context.Context, entries []*model.KnowledgeEntry) {
	for _, entry := range entries {
		// 检查本地是否已存在
		existing, _ := s.store.Entry.Get(ctx, entry.ID)
		if existing != nil {
			continue
		}

		// 创建本地副本
		if _, err := s.store.Entry.Create(ctx, entry); err != nil {
			log.Printf("[RemoteQuery] Failed to cache entry %s: %v", entry.ID, err)
			continue
		}

		// 建立索引
		if s.store.Search != nil {
			s.store.Search.IndexEntry(entry)
		}
	}
}

// generateQueryID 生成查询ID
func generateQueryID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
