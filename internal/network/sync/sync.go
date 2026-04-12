package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/daifei0527/agentwiki/internal/network/host"
	"github.com/daifei0527/agentwiki/internal/network/protocol"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	"github.com/libp2p/go-libp2p/core/peer"
)

type SyncState string

const (
	SyncStateIdle      SyncState = "idle"
	SyncStateSyncing   SyncState = "syncing"
	SyncStateError     SyncState = "error"
	SyncStateComplete  SyncState = "complete"
)

type SyncConfig struct {
	AutoSync         bool
	IntervalSeconds  int
	MirrorCategories []string
	MaxLocalSizeMB   int
	BatchSize        int
}

type VersionVector map[string]int64

func (vv VersionVector) Increment(entryID string) int64 {
	vv[entryID]++
	return vv[entryID]
}

func (vv VersionVector) Get(entryID string) int64 {
	if v, ok := vv[entryID]; ok {
		return v
	}
	return 0
}

func (vv VersionVector) Merge(other VersionVector) VersionVector {
	result := make(VersionVector)
	for k, v := range vv {
		result[k] = v
	}
	for k, v := range other {
		if v > result[k] {
			result[k] = v
		}
	}
	return result
}

func (vv VersionVector) Diff(other VersionVector) []string {
	var needed []string
	for id, theirV := range other {
		if theirV > vv.Get(id) {
			needed = append(needed, id)
		}
	}
	return needed
}

func (vv VersionVector) ToProto() map[string]int64 {
	result := make(map[string]int64)
	for k, v := range vv {
		result[k] = v
	}
	return result
}

func VersionVectorFromProto(m map[string]int64) VersionVector {
	vv := make(VersionVector)
	for k, v := range m {
		vv[k] = v
	}
	return vv
}

type SyncEngine struct {
	p2pHost    *host.P2PHost
	protocol   *protocol.Protocol
	store      *storage.Store
	config     *SyncConfig
	state      SyncState
	versionVec VersionVector
	lastSync   int64
	mu         sync.RWMutex
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewSyncEngine(p2pHost *host.P2PHost, proto *protocol.Protocol, store *storage.Store, cfg *SyncConfig) *SyncEngine {
	return &SyncEngine{
		p2pHost:    p2pHost,
		protocol:   proto,
		store:      store,
		config:     cfg,
		state:      SyncStateIdle,
		versionVec: make(VersionVector),
	}
}

// SetProtocol 设置协议处理器（用于解决循环依赖）
func (se *SyncEngine) SetProtocol(proto *protocol.Protocol) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.protocol = proto
}

func (se *SyncEngine) Start(ctx context.Context) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	if !se.config.AutoSync {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	se.cancel = cancel

	se.wg.Add(1)
	go se.syncLoop(ctx)

	return nil
}

func (se *SyncEngine) Stop() error {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.cancel != nil {
		se.cancel()
	}
	se.wg.Wait()
	return nil
}

func (se *SyncEngine) syncLoop(ctx context.Context) {
	defer se.wg.Done()

	ticker := time.NewTicker(time.Duration(se.config.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			se.IncrementalSync(ctx)
		}
	}
}

func (se *SyncEngine) IncrementalSync(ctx context.Context) error {
	se.mu.Lock()
	if se.state == SyncStateSyncing {
		se.mu.Unlock()
		return nil
	}
	se.state = SyncStateSyncing
	se.mu.Unlock()

	defer func() {
		se.mu.Lock()
		se.state = SyncStateIdle
		se.mu.Unlock()
	}()

	// 获取所有已连接的 peers
	peers := se.p2pHost.GetConnectedPeers()
	if len(peers) == 0 {
		return nil // 没有连接的节点，不需要同步
	}

	// 对每个连接的节点发起增量同步
	var wg sync.WaitGroup
	errChan := make(chan error, len(peers))

	for _, p := range peers {
		wg.Add(1)
		go func(peerID peer.ID) {
			defer wg.Done()
			if err := se.syncWithPeer(ctx, peerID); err != nil {
				log.Printf("[Sync] Failed to sync with peer %s: %v", peerID.String(), err)
				errChan <- err
			}
		}(p)
	}

	wg.Wait()
	close(errChan)

	// 更新最后同步时间
	se.mu.Lock()
	se.lastSync = time.Now().UnixMilli()
	se.mu.Unlock()

	// 返回第一个错误（如果有）
	for err := range errChan {
		return err
	}
	return nil
}

// syncWithPeer 与单个对等节点执行增量同步
func (se *SyncEngine) syncWithPeer(ctx context.Context, peerID peer.ID) error {
	// 构建同步请求
	req := &protocol.SyncRequest{
		RequestID:           peerID.String(),
		LastSyncTimestamp:   se.lastSync,
		VersionVector:       se.versionVec.ToProto(),
		RequestedCategories: se.config.MirrorCategories,
	}

	// 发送同步请求
	resp, err := se.protocol.SendSyncRequest(ctx, peerID, req)
	if err != nil {
		return fmt.Errorf("send sync request: %w", err)
	}

	// 处理响应中的新条目和更新条目
	if err := se.processSyncResponse(ctx, resp); err != nil {
		return fmt.Errorf("process sync response: %w", err)
	}

	// 合并远端版本向量到本地
	remoteVV := VersionVectorFromProto(resp.ServerVersionVector)
	se.mu.Lock()
	se.versionVec = se.versionVec.Merge(remoteVV)
	se.mu.Unlock()

	return nil
}

// processSyncResponse 处理同步响应，将变更合并到本地存储
func (se *SyncEngine) processSyncResponse(ctx context.Context, resp *protocol.SyncResponse) error {
	// 处理新条目
	for _, data := range resp.NewEntries {
		var entry model.KnowledgeEntry
		if err := entry.FromJSON(data); err != nil {
			log.Printf("[Sync] Failed to parse new entry: %v", err)
			continue
		}
		// 检查版本，远端版本高于本地才合并
		localVersion := se.versionVec.Get(entry.ID)
		if entry.Version > localVersion {
			if _, err := se.resolveConflictAndMerge(ctx, &entry, localVersion); err != nil {
				log.Printf("[Sync] Failed to merge entry %s: %v", entry.ID, err)
				continue
			}
			se.versionVec[entry.ID] = entry.Version
			// 更新搜索索引
			if se.store.Search != nil {
				se.store.Search.IndexEntry(&entry)
			}
		}
	}

	// 处理更新条目
	for _, data := range resp.UpdatedEntries {
		var entry model.KnowledgeEntry
		if err := entry.FromJSON(data); err != nil {
			log.Printf("[Sync] Failed to parse updated entry: %v", err)
			continue
		}
		localVersion := se.versionVec.Get(entry.ID)
		if entry.Version > localVersion {
			if _, err := se.resolveConflictAndMerge(ctx, &entry, localVersion); err != nil {
				log.Printf("[Sync] Failed to merge entry %s: %v", entry.ID, err)
				continue
			}
			se.versionVec[entry.ID] = entry.Version
			// 更新搜索索引
			if se.store.Search != nil {
				se.store.Search.UpdateIndex(&entry)
			}
		}
	}

	// 处理已删除条目
	for _, deletedID := range resp.DeletedEntryIDs {
		if err := se.store.Entry.Delete(ctx, deletedID); err != nil {
			log.Printf("[Sync] Failed to delete entry %s: %v", deletedID, err)
			continue
		}
		se.mu.Lock()
		delete(se.versionVec, deletedID)
		se.mu.Unlock()
		// 从搜索索引中删除
		if se.store.Search != nil {
			if err := se.store.Search.DeleteIndex(deletedID); err != nil {
				log.Printf("[Sync] Failed to delete index for entry %s: %v", deletedID, err)
			}
		}
	}

	// 处理新评分
	for _, data := range resp.NewRatings {
		var rating model.Rating
		if err := rating.FromJSON(data); err != nil {
			log.Printf("[Sync] Failed to parse rating: %v", err)
			continue
		}
		if _, err := se.store.Rating.Create(ctx, &rating); err != nil {
			log.Printf("[Sync] Failed to create rating: %v", err)
			continue
		}
		// TODO: 更新条目的平均分
	}

	return nil
}

func (se *SyncEngine) GetState() SyncState {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.state
}

func (se *SyncEngine) GetVersionVector() VersionVector {
	se.mu.RLock()
	defer se.mu.RUnlock()

	vv := make(VersionVector)
	for k, v := range se.versionVec {
		vv[k] = v
	}
	return vv
}

func (se *SyncEngine) MergeEntries(ctx context.Context, entries []*model.KnowledgeEntry) error {
	for _, entry := range entries {
		localVersion := se.versionVec.Get(entry.ID)
		if entry.Version > localVersion {
			if _, err := se.store.Entry.Create(ctx, entry); err != nil {
				continue
			}
			se.versionVec[entry.ID] = entry.Version
		}
	}
	return nil
}

func (se *SyncEngine) HandleSyncRequest(ctx context.Context, req *protocol.SyncRequest) (*protocol.SyncResponse, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	clientVV := VersionVectorFromProto(req.VersionVector)

	resp := &protocol.SyncResponse{
		RequestID:           req.RequestID,
		NewEntries:          [][]byte{},
		UpdatedEntries:      [][]byte{},
		DeletedEntryIDs:     []string{},
		NewRatings:          [][]byte{},
		ServerVersionVector: se.versionVec.ToProto(),
		ServerTimestamp:     time.Now().UnixMilli(),
	}

	// 获取所有已更新的条目（包括删除），应用分类过滤
	filter := storage.EntryFilter{
		Limit:   1000,
		OrderBy: "updated_at",
	}

	// 如果客户端请求了特定分类，只返回这些分类的条目
	requestedCategories := req.RequestedCategories
	if len(requestedCategories) == 0 {
		// 如果客户端没有指定，使用我们自己的 MirrorCategories 配置
		requestedCategories = se.config.MirrorCategories
	}

	entries, _, err := se.store.Entry.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	for _, entry := range entries {
		// 分类过滤：如果配置了分类过滤，只同步指定分类下的条目
		if !se.categoryMatches(entry.Category, requestedCategories) {
			continue
		}

		if entry.UpdatedAt > req.LastSyncTimestamp {
			if clientVV.Get(entry.ID) < entry.Version {
				// 如果条目已删除，只需要发送ID通知对方删除
				if entry.Status == model.EntryStatusDeleted {
					resp.DeletedEntryIDs = append(resp.DeletedEntryIDs, entry.ID)
				} else {
					data, err := entry.ToJSON()
					if err != nil {
						continue
					}
					if entry.Version == 1 {
						resp.NewEntries = append(resp.NewEntries, data)
					} else {
						resp.UpdatedEntries = append(resp.UpdatedEntries, data)
					}
				}
			}
		}
	}

	// TODO: 需要在 EntryStore 添加 ListUpdatedAfter 接口来获取在指定时间戳后新增/更新的评分
	// 暂时先不查询评分，后续存储层完善后补全

	return resp, nil
}

// categoryMatches 检查条目分类是否匹配过滤列表
// 支持前缀匹配（如 "tech" 匹配 "tech/programming"）
func (se *SyncEngine) categoryMatches(entryCategory string, filterCategories []string) bool {
	if len(filterCategories) == 0 {
		return true // 没有过滤，接受所有分类
	}

	for _, cat := range filterCategories {
		// 精确匹配
		if entryCategory == cat {
			return true
		}
		// 前缀匹配：过滤分类 "tech" 匹配 "tech/programming", "tech/go"
		if len(entryCategory) > len(cat) && entryCategory[:len(cat)] == cat && entryCategory[len(cat)] == '/' {
			return true
		}
	}
	return false
}

func (se *SyncEngine) HandleMirrorRequest(ctx context.Context, req *protocol.MirrorRequest) (<-chan *protocol.MirrorData, error) {
	dataCh := make(chan *protocol.MirrorData, 10)

	go func() {
		defer close(dataCh)

		// 获取所有条目，应用分类过滤
		entries, _, err := se.store.Entry.List(ctx, storage.EntryFilter{Limit: 10000})
		if err != nil {
			return
		}

		// 确定过滤的分类
		filterCategories := req.Categories
		if len(filterCategories) == 0 {
			filterCategories = se.config.MirrorCategories
		}

		// 过滤后条目
		filteredEntries := make([]*model.KnowledgeEntry, 0, len(entries))
		for _, e := range entries {
			// 只包含非删除的条目进行镜像
			if e.Status == model.EntryStatusDeleted {
				continue
			}
			if se.categoryMatches(e.Category, filterCategories) {
				filteredEntries = append(filteredEntries, e)
			}
		}

		batchSize := int(req.BatchSize)
		if batchSize <= 0 {
			batchSize = se.config.BatchSize
		}
		if batchSize <= 0 {
			batchSize = 100
		}

		totalBatches := (len(filteredEntries) + batchSize - 1) / batchSize

		for i := 0; i < totalBatches; i++ {
			start := i * batchSize
			end := start + batchSize
			if end > len(filteredEntries) {
				end = len(filteredEntries)
			}

			batch := filteredEntries[start:end]
			var entryData [][]byte
			for _, e := range batch {
				data, err := e.ToJSON()
				if err != nil {
					continue
				}
				entryData = append(entryData, data)
			}

			dataCh <- &protocol.MirrorData{
				RequestID:    req.RequestID,
				BatchIndex:   int32(i),
				TotalBatches: int32(totalBatches),
				Entries:      entryData,
			}
		}
	}()

	return dataCh, nil
}

// resolveConflictAndMerge 使用三向合并策略解决冲突并合并条目
// 三向合并原理：
// - 如果本地没有该条目 -> 直接接受远端（远端版本一定更新）
// - 如果远端版本 == 本地版本 + 1 且只有一方修改 -> 直接合并（无冲突）
// - 如果两边都修改过但内容相同 -> 接受远端版本
// - 如果两边内容真的不同 -> 发生冲突，使用"最后写入胜利"（基于更新时间戳），记录冲突日志
func (se *SyncEngine) resolveConflictAndMerge(ctx context.Context, remoteEntry *model.KnowledgeEntry, localVersion int64) (*model.KnowledgeEntry, error) {
	// 情况1：本地不存在该条目 -> 直接创建
	if localVersion == 0 {
		return se.store.Entry.Create(ctx, remoteEntry)
	}

	// 获取本地现有条目
	localEntry, err := se.store.Entry.Get(ctx, remoteEntry.ID)
	if err != nil {
		// 本地获取失败，尝试创建
		return se.store.Entry.Create(ctx, remoteEntry)
	}

	// 情况2：本地版本比远端旧，且本地内容未修改（即只有远端修改）-> 直接接受远端
	// 比较内容哈希，如果相同说明内容一致，直接更新版本即可
	if localEntry.ContentHash == remoteEntry.ContentHash {
		// 内容相同，只需要更新版本号
		remoteEntry.Version = max(remoteEntry.Version, localEntry.Version)
		return se.store.Entry.Update(ctx, remoteEntry)
	}

	// 情况3：冲突检测 -> 两边都修改了内容（并发修改）
	log.Printf("[Sync] Conflict detected on entry %s, using last-write-wins", remoteEntry.ID)

	// 三向合并策略：基于更新时间戳的最后写入胜利
	if remoteEntry.UpdatedAt > localEntry.UpdatedAt {
		// 远端更新更新，接受远端
		return se.store.Entry.Update(ctx, remoteEntry)
	}
	// 本地更新更新，保留本地，但版本号取最大值
	localEntry.Version = max(remoteEntry.Version, localEntry.Version)
	return se.store.Entry.Update(ctx, localEntry)
}

// max 返回两个int64的最大值
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// HandleHandshake 处理握手请求
func (se *SyncEngine) HandleHandshake(ctx context.Context, h *protocol.Handshake) (*protocol.HandshakeAck, error) {
	return &protocol.HandshakeAck{
		NodeID:   se.p2pHost.NodeID(),
		PeerID:   se.p2pHost.ID().String(),
		NodeType: protocol.NodeTypeLocal,
		Version:  "1.0.0",
		Accepted: true,
	}, nil
}

// HandleQuery 处理查询请求
func (se *SyncEngine) HandleQuery(ctx context.Context, q *protocol.Query) (*protocol.QueryResult, error) {
	filter := storage.SearchQuery{
		Keyword:    q.Keyword,
		Categories: q.Categories,
		Limit:      int(q.Limit),
		Offset:     int(q.Offset),
	}

	result, err := se.store.Search.Search(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	entries := make([][]byte, 0, len(result.Entries))
	for _, e := range result.Entries {
		data, err := e.ToJSON()
		if err != nil {
			continue
		}
		entries = append(entries, data)
	}

	return &protocol.QueryResult{
		QueryID:    q.QueryID,
		Entries:    entries,
		TotalCount: int32(result.TotalCount),
		HasMore:    result.HasMore,
	}, nil
}

// HandlePushEntry 处理条目推送
func (se *SyncEngine) HandlePushEntry(ctx context.Context, e *protocol.PushEntry) (*protocol.PushAck, error) {
	var entry model.KnowledgeEntry
	if err := entry.FromJSON(e.Entry); err != nil {
		return &protocol.PushAck{
			EntryID:      e.EntryID,
			Accepted:     false,
			RejectReason: "invalid entry data",
		}, nil
	}

	// 检查条目是否已存在
	existing, err := se.store.Entry.Get(ctx, entry.ID)
	if err != nil {
		// 条目不存在，创建新条目
		created, err := se.store.Entry.Create(ctx, &entry)
		if err != nil {
			return &protocol.PushAck{
				EntryID:      e.EntryID,
				Accepted:     false,
				RejectReason: err.Error(),
			}, nil
		}
		return &protocol.PushAck{
			EntryID:    e.EntryID,
			Accepted:   true,
			NewVersion: created.Version,
		}, nil
	}

	// 条目已存在，检查版本
	if entry.Version > existing.Version {
		updated, err := se.store.Entry.Update(ctx, &entry)
		if err != nil {
			return &protocol.PushAck{
				EntryID:      e.EntryID,
				Accepted:     false,
				RejectReason: err.Error(),
			}, nil
		}
		return &protocol.PushAck{
			EntryID:    e.EntryID,
			Accepted:   true,
			NewVersion: updated.Version,
		}, nil
	}

	return &protocol.PushAck{
		EntryID:      e.EntryID,
		Accepted:     false,
		RejectReason: "local version is newer or equal",
	}, nil
}

// HandleRatingPush 处理评分推送
func (se *SyncEngine) HandleRatingPush(ctx context.Context, r *protocol.RatingPush) (*protocol.RatingAck, error) {
	var rating model.Rating
	if err := rating.FromJSON(r.Rating); err != nil {
		return &protocol.RatingAck{
			Accepted:     false,
			RejectReason: "invalid rating data",
		}, nil
	}

	created, err := se.store.Rating.Create(ctx, &rating)
	if err != nil {
		return &protocol.RatingAck{
			Accepted:     false,
			RejectReason: err.Error(),
		}, nil
	}

	return &protocol.RatingAck{
		RatingID: created.ID,
		Accepted: true,
	}, nil
}

// HandleHeartbeat 处理心跳
func (se *SyncEngine) HandleHeartbeat(ctx context.Context, h *protocol.Heartbeat) error {
	// 心跳处理：可以更新节点状态或记录活跃度
	// 目前简单忽略，后续可以扩展节点状态跟踪
	return nil
}

// HandleBitfield 处理位图同步
func (se *SyncEngine) HandleBitfield(ctx context.Context, b *protocol.Bitfield) error {
	// 合并远端版本向量
	remoteVV := VersionVectorFromProto(b.VersionVector)
	se.mu.Lock()
	se.versionVec = se.versionVec.Merge(remoteVV)
	se.mu.Unlock()
	return nil
}
