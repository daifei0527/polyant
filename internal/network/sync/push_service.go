package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/daifei0527/agentwiki/internal/network/host"
	"github.com/daifei0527/agentwiki/internal/network/protocol"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// PushService 条目推送服务
// 负责将本地创建的条目推送到种子节点
type PushService struct {
	p2pHost  *host.P2PHost
	protocol *protocol.Protocol
	config   *PushConfig
	queue    chan *pushTask
	wg       sync.WaitGroup
	mu       sync.RWMutex
	running  bool
	cancel   context.CancelFunc
}

// PushConfig 推送配置
type PushConfig struct {
	// EnablePush 是否启用推送
	EnablePush bool
	// QueueSize 推送队列大小
	QueueSize int
	// Workers 推送 worker 数量
	Workers int
	// RetryCount 重试次数
	RetryCount int
	// RetryDelay 重试延迟
	RetryDelay time.Duration
}

// DefaultPushConfig 默认推送配置
func DefaultPushConfig() *PushConfig {
	return &PushConfig{
		EnablePush: true,
		QueueSize:  1000,
		Workers:    3,
		RetryCount: 3,
		RetryDelay: 5 * time.Second,
	}
}

// pushTask 推送任务
type pushTask struct {
	entry     *model.KnowledgeEntry
	signature []byte
	retries   int
}

// NewPushService 创建推送服务
func NewPushService(p2pHost *host.P2PHost, cfg *PushConfig) *PushService {
	if cfg == nil {
		cfg = DefaultPushConfig()
	}
	return &PushService{
		p2pHost: p2pHost,
		config:  cfg,
		queue:   make(chan *pushTask, cfg.QueueSize),
	}
}

// SetProtocol 设置协议处理器
func (s *PushService) SetProtocol(proto *protocol.Protocol) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.protocol = proto
}

// Start 启动推送服务
func (s *PushService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if !s.config.EnablePush {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true

	// 启动 worker
	for i := 0; i < s.config.Workers; i++ {
		s.wg.Add(1)
		go s.pushWorker(ctx, i)
	}

	log.Printf("[PushService] Started with %d workers", s.config.Workers)
	return nil
}

// Stop 停止推送服务
func (s *PushService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.running = false

	log.Printf("[PushService] Stopped")
	return nil
}

// PushEntry 将条目加入推送队列
func (s *PushService) PushEntry(entry *model.KnowledgeEntry, signature []byte) error {
	if !s.config.EnablePush {
		return nil
	}

	task := &pushTask{
		entry:     entry,
		signature: signature,
		retries:   0,
	}

	select {
	case s.queue <- task:
		log.Printf("[PushService] Queued entry %s for push", entry.ID)
		return nil
	default:
		log.Printf("[PushService] Push queue full, dropping entry %s", entry.ID)
		return ErrPushQueueFull
	}
}

// pushWorker 推送工作协程
func (s *PushService) pushWorker(ctx context.Context, id int) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case task := <-s.queue:
			s.processPushTask(ctx, task)
		}
	}
}

// processPushTask 处理推送任务
func (s *PushService) processPushTask(ctx context.Context, task *pushTask) {
	s.mu.RLock()
	proto := s.protocol
	s.mu.RUnlock()

	if proto == nil {
		log.Printf("[PushService] Protocol not set, skipping push")
		return
	}

	// 获取种子节点列表
	peers := s.p2pHost.GetConnectedPeers()
	if len(peers) == 0 {
		s.handlePushRetry(ctx, task)
		return
	}

	// 序列化条目
	entryData, err := task.entry.ToJSON()
	if err != nil {
		log.Printf("[PushService] Failed to serialize entry: %v", err)
		return
	}

	// 构建推送消息
	pushMsg := &protocol.PushEntry{
		EntryID:         task.entry.ID,
		Entry:           entryData,
		CreatorSignature: task.signature,
	}

	// 推送到第一个可用的种子节点
	pushed := false
	for _, peerID := range peers {
		ack, err := proto.SendPushEntry(ctx, peerID, pushMsg)
		if err != nil {
			log.Printf("[PushService] Failed to push to peer %s: %v", peerID.String(), err)
			continue
		}

		if ack.Accepted {
			log.Printf("[PushService] Entry %s pushed to peer %s (version: %d)",
				task.entry.ID, peerID.String(), ack.NewVersion)
			pushed = true
			break
		} else {
			log.Printf("[PushService] Entry %s rejected by peer %s: %s",
				task.entry.ID, peerID.String(), ack.RejectReason)
		}
	}

	if !pushed {
		s.handlePushRetry(ctx, task)
	}
}

// handlePushRetry 处理推送重试
func (s *PushService) handlePushRetry(ctx context.Context, task *pushTask) {
	task.retries++
	if task.retries >= s.config.RetryCount {
		log.Printf("[PushService] Entry %s push failed after %d retries", task.entry.ID, task.retries)
		return
	}

	// 延迟重试
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.config.RetryDelay):
			select {
			case s.queue <- task:
			default:
				log.Printf("[PushService] Queue full, dropping retry for entry %s", task.entry.ID)
			}
		}
	}()
}

// PushRating 推送评分
func (s *PushService) PushRating(ctx context.Context, rating *model.Rating, signature []byte) error {
	s.mu.RLock()
	proto := s.protocol
	s.mu.RUnlock()

	if proto == nil || !s.config.EnablePush {
		return nil
	}

	peers := s.p2pHost.GetConnectedPeers()
	if len(peers) == 0 {
		return ErrNoPeersConnected
	}

	// 序列化评分
	ratingData, err := rating.ToJSON()
	if err != nil {
		return err
	}

	// 构建推送消息
	pushMsg := &protocol.RatingPush{
		Rating:         ratingData,
		RaterSignature: signature,
	}

	// 推送到种子节点
	for _, peerID := range peers {
		ack, err := proto.SendRatingPush(ctx, peerID, pushMsg)
		if err != nil {
			continue
		}

		if ack.Accepted {
			log.Printf("[PushService] Rating %s pushed to peer %s", rating.ID, peerID.String())
			return nil
		}
	}

	return ErrPushFailed
}

// GetQueueSize 获取队列大小
func (s *PushService) GetQueueSize() int {
	return len(s.queue)
}

// 错误定义
var (
	ErrPushQueueFull    = fmt.Errorf("push queue full")
	ErrNoPeersConnected = fmt.Errorf("no peers connected")
	ErrPushFailed       = fmt.Errorf("push failed")
)
