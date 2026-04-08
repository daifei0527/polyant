package rating

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentwiki/agentwiki/internal/storage"
	"github.com/agentwiki/agentwiki/internal/storage/model"
	"github.com/agentwiki/agentwiki/internal/core/user"
)

var (
	ErrScoreOutOfRange  = fmt.Errorf("评分值超出范围 (1.0-5.0)")
	ErrPermissionDenied = fmt.Errorf("权限不足，无法评分")
	ErrDuplicateRating  = fmt.Errorf("重复评分")
)

func GetLevelWeight(level int32) float64 {
	switch level {
	case model.UserLevelLv0:
		return 0.0
	case model.UserLevelLv1:
		return 1.0
	case model.UserLevelLv2:
		return 1.2
	case model.UserLevelLv3:
		return 1.5
	case model.UserLevelLv4:
		return 2.0
	case model.UserLevelLv5:
		return 3.0
	default:
		return 0.0
	}
}

type RatingCalculator struct {
	store  *storage.Store
	mu     sync.RWMutex
}

func NewRatingCalculator(store *storage.Store) *RatingCalculator {
	return &RatingCalculator{
		store: store,
	}
}

func (rc *RatingCalculator) SubmitRating(ctx context.Context, entryID string, rater *model.User, score float64, comment string) (*model.Rating, error) {
	if score < 1.0 || score > 5.0 {
		return nil, ErrScoreOutOfRange
	}

	if rater.UserLevel < model.UserLevelLv1 {
		return nil, ErrPermissionDenied
	}

	existing, err := rc.store.Rating.ListByEntry(ctx, entryID)
	if err == nil {
		raterHash := user.HashPublicKey(rater.PublicKey)
		for _, r := range existing {
			if r.RaterPubkey == raterHash {
				return nil, ErrDuplicateRating
			}
		}
	}

	weight := GetLevelWeight(rater.UserLevel)
	rating := &model.Rating{
		ID:            fmt.Sprintf("%d", time.Now().UnixNano()),
		EntryId:       entryID,
		RaterPubkey:   rater.PublicKey,
		Score:         score,
		Weight:        weight,
		WeightedScore: score * weight,
		RatedAt:       time.Now().UnixMilli(),
		Comment:       comment,
	}

	if _, err := rc.store.Rating.Create(ctx, rating); err != nil {
		return nil, fmt.Errorf("保存评分失败: %w", err)
	}

	newScore := rc.RecalculateEntryScore(ctx, entryID)
	entry, err := rc.store.Entry.Get(ctx, entryID)
	if err == nil && entry != nil {
		entry.Score = newScore
		entry.ScoreCount = int32(len(existing) + 1)
		rc.store.Entry.Update(ctx, entry)
	}

	rater.RatingCnt++
	rc.store.User.Update(ctx, rater)

	return rating, nil
}

func (rc *RatingCalculator) RecalculateEntryScore(ctx context.Context, entryID string) float64 {
	ratings, err := rc.store.Rating.ListByEntry(ctx, entryID)
	if err != nil || len(ratings) == 0 {
		return 0.0
	}

	var totalWeightedScore float64
	var totalWeight float64

	for _, r := range ratings {
		totalWeightedScore += r.WeightedScore
		totalWeight += r.Weight
	}

	if totalWeight == 0 {
		return 0.0
	}

	return totalWeightedScore / totalWeight
}

func (rc *RatingCalculator) GetUserRatings(ctx context.Context, publicKey string) ([]*model.Rating, error) {
	raterHash := user.HashPublicKey(publicKey)
	allRatings := make([]*model.Rating, 0)
	
	entries, _, err := rc.store.Entry.List(ctx, storage.EntryFilter{Limit: 10000})
	if err != nil {
		return nil, err
	}
	
	for _, entry := range entries {
		ratings, err := rc.store.Rating.ListByEntry(ctx, entry.ID)
		if err != nil {
			continue
		}
		for _, r := range ratings {
			if r.RaterPubkey == raterHash {
				allRatings = append(allRatings, r)
			}
		}
	}
	
	return allRatings, nil
}

func (rc *RatingCalculator) GetEntryRatings(ctx context.Context, entryID string) ([]*model.Rating, error) {
	return rc.store.Rating.ListByEntry(ctx, entryID)
}
