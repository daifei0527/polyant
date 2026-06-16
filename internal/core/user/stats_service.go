package user

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

const defaultStatsCacheTTL = 60 * time.Second

// StatsService 统计服务。
//
// 这些都是管理员仪表盘端点（非用户热路径，约分钟级轮询），原本每次请求都
// List(100000) 全量用户并在内存聚合/排序/分页。改为按 TTL 缓存计算结果：
// 缓存命中 O(1)，过期才重算。
//
// 不采用"维护式计数器/直方图"方案的原因：active-users(30天) 是时间衰减维度，
// 而 LastActive 在每次认证请求时都会更新（高写频率）。为它维护计数器或按日
// 直方图会在热路径引入写放大与非原子 RMW 漂移，得不偿失；对非实时的管理员
// 统计，~60s 的陈旧完全可以接受。
type StatsService struct {
	store *storage.Store
	ttl   time.Duration

	mu sync.RWMutex

	userStats   *model.UserStats
	userStatsAt time.Time

	contrib   map[string]*cachedContrib // sortBy -> 排序后的全量列表
	contribAt map[string]time.Time

	activityTrend     map[int][]ActivityTrend // days -> 趋势
	activityTrendAt   map[int]time.Time
	registrationTrend   map[int][]RegistrationTrend
	registrationTrendAt map[int]time.Time
}

type cachedContrib struct {
	list  []UserContribution
	total int64
}

// NewStatsService 创建统计服务（默认 60s 缓存）。
func NewStatsService(store *storage.Store) *StatsService {
	return &StatsService{
		store:               store,
		ttl:                 defaultStatsCacheTTL,
		contrib:             make(map[string]*cachedContrib),
		contribAt:           make(map[string]time.Time),
		activityTrend:       make(map[int][]ActivityTrend),
		activityTrendAt:     make(map[int]time.Time),
		registrationTrend:   make(map[int][]RegistrationTrend),
		registrationTrendAt: make(map[int]time.Time),
	}
}

// SetCacheTTL 设置缓存 TTL；<=0 禁用缓存（每次重算，测试用）。变更时清空缓存。
func (s *StatsService) SetCacheTTL(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ttl = d
	s.invalidateLocked()
}

func (s *StatsService) invalidateLocked() {
	s.userStats = nil
	s.contrib = make(map[string]*cachedContrib)
	s.contribAt = make(map[string]time.Time)
	s.activityTrend = make(map[int][]ActivityTrend)
	s.activityTrendAt = make(map[int]time.Time)
	s.registrationTrend = make(map[int][]RegistrationTrend)
	s.registrationTrendAt = make(map[int]time.Time)
}

// fresh 报告给定时间戳是否仍在 TTL 内（ttl<=0 时永远返回 false，即禁用缓存）。
func (s *StatsService) fresh(at time.Time) bool {
	return s.ttl > 0 && time.Since(at) < s.ttl
}

// GetUserStats 获取用户统计概览
func (s *StatsService) GetUserStats(ctx context.Context) (*model.UserStats, error) {
	s.mu.RLock()
	if s.userStats != nil && s.fresh(s.userStatsAt) {
		out := *s.userStats
		s.mu.RUnlock()
		return &out, nil
	}
	s.mu.RUnlock()

	users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
	if err != nil {
		return nil, err
	}

	stats := &model.UserStats{TotalUsers: total}
	now := time.Now().UnixMilli()
	thirtyDaysAgo := now - 30*24*60*60*1000

	for _, u := range users {
		switch u.UserLevel {
		case model.UserLevelLv0:
			stats.Lv0Count++
		case model.UserLevelLv1:
			stats.Lv1Count++
		case model.UserLevelLv2:
			stats.Lv2Count++
		case model.UserLevelLv3:
			stats.Lv3Count++
		case model.UserLevelLv4:
			stats.Lv4Count++
		case model.UserLevelLv5:
			stats.Lv5Count++
		}

		if u.LastActive > thirtyDaysAgo {
			stats.ActiveUsers++
		}

		if u.Status == model.UserStatusBanned {
			stats.BannedCount++
		}

		stats.TotalContribs += int64(u.ContributionCnt)
		stats.TotalRatings += int64(u.RatingCnt)
	}

	s.mu.Lock()
	s.userStats = stats
	s.userStatsAt = time.Now()
	s.mu.Unlock()

	return stats, nil
}

// UserContribution 用户贡献明细
type UserContribution struct {
	UserID           string  `json:"userId"`
	UserName         string  `json:"userName"`
	EntryCount       int64   `json:"entryCount"`
	EditCount        int64   `json:"editCount"`
	RatingGivenCount int64   `json:"ratingGivenCount"`
	RatingRecvCount  int64   `json:"ratingRecvCount"`
	AvgRatingRecv    float64 `json:"avgRatingRecv"`
}

// GetContributionStats 获取贡献明细统计。全量排序结果按 sortBy 缓存，分页仅切片。
func (s *StatsService) GetContributionStats(ctx context.Context, offset, limit int, sortBy string) ([]UserContribution, int64, error) {
	s.mu.RLock()
	cc, ok := s.contrib[sortBy]
	fresh := ok && s.fresh(s.contribAt[sortBy])
	s.mu.RUnlock()

	if !fresh {
		users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
		if err != nil {
			return nil, 0, err
		}

		contribs := make([]UserContribution, 0, len(users))
		for _, u := range users {
			contribs = append(contribs, UserContribution{
				UserID:           u.PublicKey,
				UserName:         u.AgentName,
				EntryCount:       int64(u.ContributionCnt),
				RatingGivenCount: int64(u.RatingCnt),
			})
		}

		sort.Slice(contribs, func(i, j int) bool {
			switch sortBy {
			case "rating_given_count":
				return contribs[i].RatingGivenCount > contribs[j].RatingGivenCount
			default: // "entry_count" 及默认
				return contribs[i].EntryCount > contribs[j].EntryCount
			}
		})

		cc = &cachedContrib{list: contribs, total: total}
		s.mu.Lock()
		s.contrib[sortBy] = cc
		s.contribAt[sortBy] = time.Now()
		s.mu.Unlock()
	}

	if offset >= len(cc.list) {
		return []UserContribution{}, cc.total, nil
	}
	end := offset + limit
	if end > len(cc.list) {
		end = len(cc.list)
	}
	out := make([]UserContribution, end-offset)
	copy(out, cc.list[offset:end])
	return out, cc.total, nil
}

// ActivityTrend 活跃度趋势
type ActivityTrend struct {
	Date        string `json:"date"`
	DAU         int64  `json:"dau"`
	NewUsers    int64  `json:"newUsers"`
	ActionCount int64  `json:"actionCount"`
}

// GetActivityTrend 获取活跃度趋势（按 days 缓存）
func (s *StatsService) GetActivityTrend(ctx context.Context, days int) ([]ActivityTrend, error) {
	s.mu.RLock()
	cached, ok := s.activityTrend[days]
	fresh := ok && s.fresh(s.activityTrendAt[days])
	s.mu.RUnlock()
	if fresh {
		return copyActivityTrend(cached), nil
	}

	users, _, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	trend := make([]ActivityTrend, days)

	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location()).UnixMilli()
		dayEnd := dayStart + 24*60*60*1000

		var dau, newUsers int64
		for _, u := range users {
			if u.LastActive >= dayStart && u.LastActive < dayEnd {
				dau++
			}
			if u.RegisteredAt >= dayStart && u.RegisteredAt < dayEnd {
				newUsers++
			}
		}

		trend[days-1-i] = ActivityTrend{
			Date:     dateStr,
			DAU:      dau,
			NewUsers: newUsers,
		}
	}

	s.mu.Lock()
	s.activityTrend[days] = trend
	s.activityTrendAt[days] = time.Now()
	s.mu.Unlock()
	return copyActivityTrend(trend), nil
}

// RegistrationTrend 注册趋势
type RegistrationTrend struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
	Total int64  `json:"total"`
}

// GetRegistrationTrend 获取注册趋势（按 days 缓存）
func (s *StatsService) GetRegistrationTrend(ctx context.Context, days int) ([]RegistrationTrend, error) {
	s.mu.RLock()
	cached, ok := s.registrationTrend[days]
	fresh := ok && s.fresh(s.registrationTrendAt[days])
	s.mu.RUnlock()
	if fresh {
		return copyRegistrationTrend(cached), nil
	}

	users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	trend := make([]RegistrationTrend, days)
	cumulative := total

	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location()).UnixMilli()
		dayEnd := dayStart + 24*60*60*1000

		var count int64
		for _, u := range users {
			if u.RegisteredAt >= dayStart && u.RegisteredAt < dayEnd {
				count++
			}
		}

		trend[days-1-i] = RegistrationTrend{
			Date:  dateStr,
			Count: count,
			Total: cumulative,
		}
		cumulative -= count
	}

	s.mu.Lock()
	s.registrationTrend[days] = trend
	s.registrationTrendAt[days] = time.Now()
	s.mu.Unlock()
	return copyRegistrationTrend(trend), nil
}

func copyActivityTrend(in []ActivityTrend) []ActivityTrend {
	out := make([]ActivityTrend, len(in))
	copy(out, in)
	return out
}

func copyRegistrationTrend(in []RegistrationTrend) []RegistrationTrend {
	out := make([]RegistrationTrend, len(in))
	copy(out, in)
	return out
}
