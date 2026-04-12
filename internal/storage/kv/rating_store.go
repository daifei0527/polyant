package kv

import (
	"fmt"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// RatingStore 提供评分的CRUD操作
type RatingStore struct {
	store Store
}

// NewRatingStore 创建一个新的评分存储实例
func NewRatingStore(store Store) *RatingStore {
	return &RatingStore{store: store}
}

// ratingKey 生成评分的存储键
func ratingKey(entryId, raterPubkey string) string {
	return PrefixRating + entryId + ":" + raterPubkey
}

// CreateRating 创建一个新的评分
func (rs *RatingStore) CreateRating(rating *model.Rating) error {
	if rating.EntryId == "" || rating.RaterPubkey == "" {
		return fmt.Errorf("entry id and rater public key must not be empty")
	}

	key := []byte(ratingKey(rating.EntryId, rating.RaterPubkey))

	// 检查是否已存在
	_, err := rs.store.Get(key)
	if err == nil {
		return fmt.Errorf("rating from %s for entry %s already exists", rating.RaterPubkey, rating.EntryId)
	}

	// 设置评分时间
	if rating.RatedAt == 0 {
		rating.RatedAt = time.Now().Unix()
	}

	// 计算加权评分
	rating.WeightedScore = rating.Score * rating.Weight

	data, err := rating.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize rating: %w", err)
	}

	return rs.store.Put(key, data)
}

// GetRatingsByEntry 获取指定条目的所有评分
func (rs *RatingStore) GetRatingsByEntry(entryId string) ([]*model.Rating, error) {
	prefix := PrefixRating + entryId + ":"

	ratings, err := ScanAndParse(rs.store, prefix, func(data []byte) (*model.Rating, error) {
		rating := &model.Rating{}
		if err := rating.FromJSON(data); err != nil {
			return nil, err
		}
		return rating, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get ratings for entry %s: %w", entryId, err)
	}

	return ratings, nil
}

// GetRating 获取指定用户对指定条目的评分
func (rs *RatingStore) GetRating(entryId, raterPubkey string) (*model.Rating, error) {
	key := []byte(ratingKey(entryId, raterPubkey))

	data, err := rs.store.Get(key)
	if err != nil {
		if err == ErrKeyNotFound {
			return nil, fmt.Errorf("rating from %s for entry %s not found", raterPubkey, entryId)
		}
		return nil, fmt.Errorf("failed to get rating: %w", err)
	}

	rating := &model.Rating{}
	if err := rating.FromJSON(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize rating: %w", err)
	}

	return rating, nil
}

// UpdateEntryScore 重新计算指定条目的加权平均评分
// 返回新的加权平均分
func (rs *RatingStore) UpdateEntryScore(entryId string) (float64, error) {
	ratings, err := rs.GetRatingsByEntry(entryId)
	if err != nil {
		return 0, fmt.Errorf("failed to get ratings for score update: %w", err)
	}

	if len(ratings) == 0 {
		return 0, nil
	}

	// 计算加权平均分
	var totalWeightedScore float64
	var totalWeight float64

	for _, r := range ratings {
		totalWeightedScore += r.WeightedScore
		totalWeight += r.Weight
	}

	var avgScore float64
	if totalWeight > 0 {
		avgScore = totalWeightedScore / totalWeight
	}

	return avgScore, nil
}
