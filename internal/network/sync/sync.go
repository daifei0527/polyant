package sync

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/network/host"
	protocolpkg "github.com/daifei0527/polyant/internal/network/protocol"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/index"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/crypto"
	"github.com/daifei0527/polyant/pkg/safeconv"
	"github.com/libp2p/go-libp2p/core/peer"
)

type SyncState string

const (
	SyncStateIdle     SyncState = "idle"
	SyncStateSyncing  SyncState = "syncing"
	SyncStateError    SyncState = "error"
	SyncStateComplete SyncState = "complete"
)

type SyncConfig struct {
	AutoSync               bool
	IntervalSeconds        int
	MirrorCategories       []string
	MaxLocalSizeMB         int
	BatchSize              int
	RequireEntrySignatures bool // R1-B4: 为 true 时拒绝未签名的 P2P 推送条目/评分
}

// VersionVector 是线程安全的版本向量。所有方法内部加锁，可在多 goroutine 间并发使用。
type VersionVector struct {
	mu sync.RWMutex
	m  map[string]int64
}

// NewVersionVector 创建空的线程安全版本向量。
func NewVersionVector() *VersionVector {
	return &VersionVector{m: make(map[string]int64)}
}

// Increment 递增某条目版本号并返回新值。
func (vv *VersionVector) Increment(entryID string) int64 {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	vv.m[entryID]++
	return vv.m[entryID]
}

// Get 返回某条目版本号（不存在返回 0）。
func (vv *VersionVector) Get(entryID string) int64 {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	return vv.m[entryID]
}

// Set 设置某条目版本号。
func (vv *VersionVector) Set(entryID string, ver int64) {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	vv.m[entryID] = ver
}

// Merge 合并另一版本向量（按 key 取 max）。
func (vv *VersionVector) Merge(other map[string]int64) {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	for k, v := range other {
		if cur, ok := vv.m[k]; !ok || v > cur {
			vv.m[k] = v
		}
	}
}

// Diff 返回 other 中比 self 更新的 entryID 列表。
func (vv *VersionVector) Diff(other map[string]int64) []string {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	var needed []string
	for id, theirV := range other {
		if theirV > vv.m[id] {
			needed = append(needed, id)
		}
	}
	return needed
}

// Clone 返回内部 map 的深拷贝。
func (vv *VersionVector) Clone() map[string]int64 {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	out := make(map[string]int64, len(vv.m))
	for k, v := range vv.m {
		out[k] = v
	}
	return out
}

// ToProto 返回可序列化的 map 拷贝（用于 protobuf 传输）。
func (vv *VersionVector) ToProto() map[string]int64 {
	return vv.Clone()
}

// Delete 删除某条目版本记录。
func (vv *VersionVector) Delete(entryID string) {
	vv.mu.Lock()
	defer vv.mu.Unlock()
	delete(vv.m, entryID)
}

// Range 遍历所有条目（持读锁，回调内不可回头改 vv）。
func (vv *VersionVector) Range(fn func(entryID string, ver int64)) {
	vv.mu.RLock()
	defer vv.mu.RUnlock()
	for k, v := range vv.m {
		fn(k, v)
	}
}

// VersionVectorFromProto 从 protobuf map 构造裸 map（供 Merge 消费）。
func VersionVectorFromProto(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

type SyncEngine struct {
	p2pHost    host.P2PHostInterface
	protocol   protocolpkg.ProtocolInterface
	store      *storage.Store
	config     *SyncConfig
	state      SyncState
	versionVec *VersionVector
	lastSync   int64
	mu         sync.RWMutex
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	ctx        context.Context // 服务级 ctx，Start 赋值，Stop 取消
}

func NewSyncEngine(p2pHost host.P2PHostInterface, proto protocolpkg.ProtocolInterface, store *storage.Store, cfg *SyncConfig) *SyncEngine {
	return &SyncEngine{
		p2pHost:    p2pHost,
		protocol:   proto,
		store:      store,
		config:     cfg,
		state:      SyncStateIdle,
		versionVec: NewVersionVector(),
	}
}

// SetProtocol 设置协议处理器（用于解决循环依赖）
func (se *SyncEngine) SetProtocol(proto protocolpkg.ProtocolInterface) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.protocol = proto
}

func (se *SyncEngine) Start(ctx context.Context) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	// 始终保存服务级 ctx，即使 AutoSync=false，以便 HandleMirrorRequest 生产者
	// 能在 Stop() 时被取消。
	ctx, cancel := context.WithCancel(ctx)
	se.ctx = ctx
	se.cancel = cancel

	if !se.config.AutoSync {
		return nil
	}

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
	req := &protocolpkg.SyncRequest{
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
	se.versionVec.Merge(VersionVectorFromProto(resp.ServerVersionVector))

	return nil
}

// processSyncResponse 处理同步响应，将变更合并到本地存储
func (se *SyncEngine) processSyncResponse(ctx context.Context, resp *protocolpkg.SyncResponse) error {
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
			se.versionVec.Set(entry.ID, entry.Version)
			// 更新搜索索引
			if se.store.Search != nil {
				if err := se.store.Search.IndexEntry(&entry); err != nil {
					log.Printf("[Sync] index entry %s failed: %v", entry.ID, err)
				}
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
			se.versionVec.Set(entry.ID, entry.Version)
			// 更新搜索索引
			if se.store.Search != nil {
				if err := se.store.Search.UpdateIndex(&entry); err != nil {
					log.Printf("[Sync] update index %s failed: %v", entry.ID, err)
				}
			}
		}
	}

	// 处理已删除条目
	for _, deletedID := range resp.DeletedEntryIDs {
		if err := se.store.Entry.Delete(ctx, deletedID); err != nil {
			log.Printf("[Sync] Failed to delete entry %s: %v", deletedID, err)
			continue
		}
		se.versionVec.Delete(deletedID)
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
		// 更新条目的平均分
		se.updateEntryScore(ctx, rating.EntryId)
	}

	return nil
}

// updateEntryScore 更新条目的平均评分
func (se *SyncEngine) updateEntryScore(ctx context.Context, entryID string) {
	// 获取条目的所有评分
	ratings, err := se.store.Rating.ListByEntry(ctx, entryID)
	if err != nil {
		log.Printf("[Sync] Failed to list ratings for entry %s: %v", entryID, err)
		return
	}

	if len(ratings) == 0 {
		return
	}

	// 计算平均分
	var totalScore float64
	for _, r := range ratings {
		totalScore += r.Score
	}
	avgScore := totalScore / float64(len(ratings))

	// 获取并更新条目
	entry, err := se.store.Entry.Get(ctx, entryID)
	if err != nil {
		log.Printf("[Sync] Failed to get entry %s: %v", entryID, err)
		return
	}

	entry.Score = avgScore
	entry.ScoreCount = safeconv.Int32FromInt(len(ratings))

	if _, err := se.store.Entry.Update(ctx, entry); err != nil {
		log.Printf("[Sync] Failed to update entry score %s: %v", entryID, err)
	}
}

func (se *SyncEngine) GetState() SyncState {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.state
}

func (se *SyncEngine) GetVersionVector() map[string]int64 {
	return se.versionVec.Clone()
}

func (se *SyncEngine) MergeEntries(ctx context.Context, entries []*model.KnowledgeEntry) error {
	for _, entry := range entries {
		localVersion := se.versionVec.Get(entry.ID)
		if entry.Version > localVersion {
			if _, err := se.store.Entry.Create(ctx, entry); err != nil {
				continue
			}
			se.versionVec.Set(entry.ID, entry.Version)
		}
	}
	return nil
}

func (se *SyncEngine) HandleSyncRequest(ctx context.Context, req *protocolpkg.SyncRequest) (*protocolpkg.SyncResponse, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	clientVV := VersionVectorFromProto(req.VersionVector)

	resp := &protocolpkg.SyncResponse{
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
			if clientVV[entry.ID] < entry.Version {
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

	// 获取指定时间戳之后新增/更新的评分
	if se.store.Rating != nil {
		ratings, err := se.store.Rating.ListRatedAfter(ctx, req.LastSyncTimestamp)
		if err != nil {
			return nil, fmt.Errorf("list ratings: %w", err)
		}
		for _, rating := range ratings {
			data, err := rating.ToJSON()
			if err != nil {
				continue
			}
			resp.NewRatings = append(resp.NewRatings, data)
		}
	}

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

func (se *SyncEngine) HandleMirrorRequest(ctx context.Context, req *protocolpkg.MirrorRequest) (<-chan *protocolpkg.MirrorData, error) {
	dataCh := make(chan *protocolpkg.MirrorData, 10)

	se.wg.Add(1)
	go func() {
		defer se.wg.Done()
		defer close(dataCh)

		// 使用服务级 ctx，使 Stop() 能取消生产者
		prodCtx := se.ctx
		if prodCtx == nil {
			prodCtx = ctx
		}

		// 获取所有条目，应用分类过滤
		entries, _, err := se.store.Entry.List(prodCtx, storage.EntryFilter{Limit: 10000})
		if err != nil {
			log.Printf("[Sync] HandleMirrorRequest list entries failed: %v", err)
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

			select {
			case dataCh <- &protocolpkg.MirrorData{
				RequestID:    req.RequestID,
				BatchIndex:   safeconv.Int32FromInt(i),
				TotalBatches: safeconv.Int32FromInt(totalBatches),
				Entries:      entryData,
			}:
			case <-prodCtx.Done():
				return
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
	// 比较内容哈希，如果相同说明内容一致，直接更新版本即可。
	// 注意：必须比对【重算】的远端哈希，而非远端提供的 ContentHash 字段值——
	// 该字段在传输中可被伪造。直接比字段会让攻击者把 ContentHash 伪造成本地值，
	// 伪装"内容未变"以绕过冲突检测。重算比对确保只有内容真正一致才走快捷路径。
	if localEntry.ContentHash == remoteEntry.ComputeContentHash() {
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
func (se *SyncEngine) HandleHandshake(ctx context.Context, h *protocolpkg.Handshake) (*protocolpkg.HandshakeAck, error) {
	return &protocolpkg.HandshakeAck{
		NodeID:   se.p2pHost.NodeID(),
		PeerID:   se.p2pHost.ID().String(),
		NodeType: protocolpkg.NodeTypeLocal,
		Version:  "1.0.0",
		Accepted: true,
	}, nil
}

// HandleQuery 处理查询请求
func (se *SyncEngine) HandleQuery(ctx context.Context, q *protocolpkg.Query) (*protocolpkg.QueryResult, error) {
	filter := index.SearchQuery{
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

	return &protocolpkg.QueryResult{
		QueryID:    q.QueryID,
		Entries:    entries,
		TotalCount: safeconv.Int32FromInt(result.TotalCount),
		HasMore:    result.HasMore,
	}, nil
}

// verifyPushedEntrySignature 校验 P2P 推送条目的内容签名。
// 返回非 nil PushAck 表示拒绝（调用方直接返回）；nil 表示放行。
// 软上线策略：有签名则必须验过；无签名时仅当 RequireEntrySignatures=true 才拒绝，否则记日志放行（兼容历史数据）。
func (se *SyncEngine) verifyPushedEntrySignature(entry *model.KnowledgeEntry, wireSig []byte) *protocolpkg.PushAck {
	sig := entry.Signature
	if len(sig) == 0 {
		sig = wireSig // 兼容仅在线段携带签名、entry JSON 未带的旧客户端
	}
	requireSigs := se.config != nil && se.config.RequireEntrySignatures
	if len(sig) == 0 {
		if requireSigs {
			log.Printf("[security] rejected unsigned entry (require_entry_signatures=true): entryId=%s createdBy=%s", entry.ID, entry.CreatedBy)
			return &protocolpkg.PushAck{EntryID: entry.ID, Accepted: false, RejectReason: "unsigned entry rejected (require_entry_signatures)"}
		}
		log.Printf("[security] accepted unsigned entry: entryId=%s createdBy=%s", entry.ID, entry.CreatedBy)
		return nil
	}
	pub, err := base64.StdEncoding.DecodeString(entry.CreatedBy)
	if err != nil || !crypto.VerifyContent(ed25519.PublicKey(pub), sig, entry.Title, entry.Content, entry.Category) {
		log.Printf("[security] rejected forged entry signature: entryId=%s createdBy=%s", entry.ID, entry.CreatedBy)
		return &protocolpkg.PushAck{EntryID: entry.ID, Accepted: false, RejectReason: "forged entry signature"}
	}
	return nil
}

// verifyPushedRatingSignature 校验 P2P 推送评分的签名。语义同 verifyPushedEntrySignature。
func (se *SyncEngine) verifyPushedRatingSignature(rating *model.Rating, wireSig []byte) *protocolpkg.RatingAck {
	sig := rating.Signature
	if len(sig) == 0 {
		sig = wireSig
	}
	requireSigs := se.config != nil && se.config.RequireEntrySignatures
	if len(sig) == 0 {
		if requireSigs {
			log.Printf("[security] rejected unsigned rating (require_entry_signatures=true): ratingId=%s rater=%s", rating.ID, rating.RaterPubkey)
			return &protocolpkg.RatingAck{Accepted: false, RejectReason: "unsigned rating rejected (require_entry_signatures)"}
		}
		log.Printf("[security] accepted unsigned rating: ratingId=%s rater=%s", rating.ID, rating.RaterPubkey)
		return nil
	}
	pub, err := base64.StdEncoding.DecodeString(rating.RaterPubkey)
	if err != nil || !crypto.VerifyRating(ed25519.PublicKey(pub), sig, rating.EntryId, rating.RaterPubkey, rating.Score) {
		log.Printf("[security] rejected forged rating signature: ratingId=%s rater=%s", rating.ID, rating.RaterPubkey)
		return &protocolpkg.RatingAck{Accepted: false, RejectReason: "forged rating signature"}
	}
	return nil
}

// HandlePushEntry 处理条目推送
func (se *SyncEngine) HandlePushEntry(ctx context.Context, e *protocolpkg.PushEntry) (*protocolpkg.PushAck, error) {
	var entry model.KnowledgeEntry
	if err := entry.FromJSON(e.Entry); err != nil {
		return &protocolpkg.PushAck{
			EntryID:      e.EntryID,
			Accepted:     false,
			RejectReason: "invalid entry data",
		}, nil
	}

	// R1-B4：校验内容签名（软上线 + RequireEntrySignatures 开关），防 peer 伪造条目。
	if ack := se.verifyPushedEntrySignature(&entry, e.CreatorSignature); ack != nil {
		return ack, nil
	}

	// 检查条目是否已存在
	existing, err := se.store.Entry.Get(ctx, entry.ID)
	if err != nil {
		// 条目不存在，创建新条目
		created, err := se.store.Entry.Create(ctx, &entry)
		if err != nil {
			return &protocolpkg.PushAck{
				EntryID:      e.EntryID,
				Accepted:     false,
				RejectReason: err.Error(),
			}, nil
		}
		return &protocolpkg.PushAck{
			EntryID:    e.EntryID,
			Accepted:   true,
			NewVersion: created.Version,
		}, nil
	}

	// 条目已存在，检查版本
	if entry.Version > existing.Version {
		updated, err := se.store.Entry.Update(ctx, &entry)
		if err != nil {
			return &protocolpkg.PushAck{
				EntryID:      e.EntryID,
				Accepted:     false,
				RejectReason: err.Error(),
			}, nil
		}
		return &protocolpkg.PushAck{
			EntryID:    e.EntryID,
			Accepted:   true,
			NewVersion: updated.Version,
		}, nil
	}

	return &protocolpkg.PushAck{
		EntryID:      e.EntryID,
		Accepted:     false,
		RejectReason: "local version is newer or equal",
	}, nil
}

// HandleRatingPush 处理评分推送
func (se *SyncEngine) HandleRatingPush(ctx context.Context, r *protocolpkg.RatingPush) (*protocolpkg.RatingAck, error) {
	var rating model.Rating
	if err := rating.FromJSON(r.Rating); err != nil {
		return &protocolpkg.RatingAck{
			Accepted:     false,
			RejectReason: "invalid rating data",
		}, nil
	}

	// R1-B4：校验评分签名（软上线 + RequireEntrySignatures 开关），防 peer 冒充评分者。
	if ack := se.verifyPushedRatingSignature(&rating, r.RaterSignature); ack != nil {
		return ack, nil
	}

	created, err := se.store.Rating.Create(ctx, &rating)
	if err != nil {
		return &protocolpkg.RatingAck{
			Accepted:     false,
			RejectReason: err.Error(),
		}, nil
	}

	return &protocolpkg.RatingAck{
		RatingID: created.ID,
		Accepted: true,
	}, nil
}

// HandleHeartbeat 处理心跳
func (se *SyncEngine) HandleHeartbeat(ctx context.Context, h *protocolpkg.Heartbeat) error {
	// 心跳处理：可以更新节点状态或记录活跃度
	// 目前简单忽略，后续可以扩展节点状态跟踪
	return nil
}

// HandleBitfield 处理位图同步
func (se *SyncEngine) HandleBitfield(ctx context.Context, b *protocolpkg.Bitfield) error {
	// 合并远端版本向量（versionVec 自保护，无需 se.mu）
	se.versionVec.Merge(VersionVectorFromProto(b.VersionVector))
	return nil
}

// HandleMirrorData 处理接收到的镜像数据批次：反序列化 entries，逐个经冲突仲裁落库。
func (se *SyncEngine) HandleMirrorData(ctx context.Context, d *protocolpkg.MirrorData) error {
	if d == nil {
		return fmt.Errorf("nil mirror data")
	}
	for _, entryJSON := range d.Entries {
		var entry model.KnowledgeEntry
		if err := json.Unmarshal(entryJSON, &entry); err != nil {
			log.Printf("[Sync] mirror data unmarshal entry failed: %v", err)
			continue
		}
		localVersion := se.versionVec.Get(entry.ID)
		merged, err := se.resolveConflictAndMerge(ctx, &entry, localVersion)
		if err != nil {
			log.Printf("[Sync] mirror merge entry %s failed: %v", entry.ID, err)
			continue
		}
		se.versionVec.Set(merged.ID, merged.Version)
		if se.store.Search != nil {
			if err := se.store.Search.IndexEntry(merged); err != nil {
				log.Printf("[Sync] mirror index entry %s failed: %v", merged.ID, err)
			}
		}
	}
	return nil
}
