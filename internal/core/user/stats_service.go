package user

import (
	"context"
	"sort"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// StatsService 统计服务
type StatsService struct {
	store *storage.Store
}

// NewStatsService 创建统计服务
func NewStatsService(store *storage.Store) *StatsService {
	return &StatsService{store: store}
}

// GetUserStats 获取用户统计概览
func (s *StatsService) GetUserStats(ctx context.Context) (*model.UserStats, error) {
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

// GetContributionStats 获取贡献明细统计
func (s *StatsService) GetContributionStats(ctx context.Context, offset, limit int, sortBy string) ([]UserContribution, int64, error) {
	users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
	if err != nil {
		return nil, 0, err
	}

	var contribs []UserContribution
	for _, u := range users {
		contribs = append(contribs, UserContribution{
			UserID:           u.PublicKey,
			UserName:         u.AgentName,
			EntryCount:       int64(u.ContributionCnt),
			RatingGivenCount: int64(u.RatingCnt),
		})
	}

	// 排序
	sort.Slice(contribs, func(i, j int) bool {
		switch sortBy {
		case "entry_count":
			return contribs[i].EntryCount > contribs[j].EntryCount
		case "rating_given_count":
			return contribs[i].RatingGivenCount > contribs[j].RatingGivenCount
		default:
			return contribs[i].EntryCount > contribs[j].EntryCount
		}
	})

	// 分页
	if offset >= len(contribs) {
		return []UserContribution{}, total, nil
	}
	end := offset + limit
	if end > len(contribs) {
		end = len(contribs)
	}
	return contribs[offset:end], total, nil
}

// ActivityTrend 活跃度趋势
type ActivityTrend struct {
	Date        string `json:"date"`
	DAU         int64  `json:"dau"`
	NewUsers    int64  `json:"newUsers"`
	ActionCount int64  `json:"actionCount"`
}

// GetActivityTrend 获取活跃度趋势
func (s *StatsService) GetActivityTrend(ctx context.Context, days int) ([]ActivityTrend, error) {
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
			// 检查是否在该天活跃
			if u.LastActive >= dayStart && u.LastActive < dayEnd {
				dau++
			}
			// 检查是否在该天注册
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

	return trend, nil
}

// RegistrationTrend 注册趋势
type RegistrationTrend struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
	Total int64  `json:"total"`
}

// GetRegistrationTrend 获取注册趋势
func (s *StatsService) GetRegistrationTrend(ctx context.Context, days int) ([]RegistrationTrend, error) {
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

	return trend, nil
}
