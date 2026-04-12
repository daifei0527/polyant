package user

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// LevelUpgradeChecker 用户层级升级检查器
// 定期检查所有用户是否满足升级条件
type LevelUpgradeChecker struct {
	store        *storage.Store
	interval     time.Duration
	running      bool
	mu           sync.RWMutex
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewLevelUpgradeChecker 创建升级检查器
func NewLevelUpgradeChecker(store *storage.Store, interval time.Duration) *LevelUpgradeChecker {
	if interval == 0 {
		interval = time.Hour // 默认每小时检查一次
	}
	return &LevelUpgradeChecker{
		store:    store,
		interval: interval,
	}
}

// Start 启动定时检查
func (c *LevelUpgradeChecker) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.running = true

	c.wg.Add(1)
	go c.checkLoop(ctx)

	log.Printf("[LevelUpgradeChecker] Started, interval: %v", c.interval)
	return nil
}

// Stop 停止定时检查
func (c *LevelUpgradeChecker) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.running = false

	log.Printf("[LevelUpgradeChecker] Stopped")
	return nil
}

// checkLoop 定时检查循环
func (c *LevelUpgradeChecker) checkLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	c.checkAllUsers(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkAllUsers(ctx)
		}
	}
}

// checkAllUsers 检查所有用户的升级条件
func (c *LevelUpgradeChecker) checkAllUsers(ctx context.Context) {
	// 获取所有用户
	users, _, err := c.store.User.List(ctx, storage.UserFilter{Limit: 10000})
	if err != nil {
		log.Printf("[LevelUpgradeChecker] Failed to list users: %v", err)
		return
	}

	upgradedCount := 0
	for _, user := range users {
		// 跳过已 suspended 的用户
		if user.Status == "suspended" {
			continue
		}

		// 检查是否可以升级
		if newLevel, upgraded := c.checkUpgrade(user); upgraded {
			upgradedCount++
			log.Printf("[LevelUpgradeChecker] User %s upgraded to Lv%d", user.AgentName, newLevel)
		}
	}

	if upgradedCount > 0 {
		log.Printf("[LevelUpgradeChecker] Total %d users upgraded", upgradedCount)
	}
}

// checkUpgrade 检查单个用户是否满足升级条件
func (c *LevelUpgradeChecker) checkUpgrade(user *model.User) (int32, bool) {
	var newLevel int32
	var upgraded bool

	switch user.UserLevel {
	case model.UserLevelLv0:
		// Lv0 -> Lv1: 需要邮箱验证（不在定时任务中处理）
		return user.UserLevel, false

	case model.UserLevelLv1:
		// Lv1 -> Lv2: 贡献 ≥10 条目，评分 ≥20 次
		if user.ContributionCnt >= 10 && user.RatingCnt >= 20 {
			newLevel = model.UserLevelLv2
			upgraded = true
		}

	case model.UserLevelLv2:
		// Lv2 -> Lv3: 贡献 ≥50 条目，评分 ≥100 次
		if user.ContributionCnt >= 50 && user.RatingCnt >= 100 {
			newLevel = model.UserLevelLv3
			upgraded = true
		}

	case model.UserLevelLv3:
		// Lv3 -> Lv4: 贡献 ≥200 条目，评分 ≥500 次
		if user.ContributionCnt >= 200 && user.RatingCnt >= 500 {
			newLevel = model.UserLevelLv4
			upgraded = true
		}

	case model.UserLevelLv4:
		// Lv4 -> Lv5: 需要投票选举（不在定时任务中处理）
		return user.UserLevel, false

	case model.UserLevelLv5:
		// Lv5 是最高级别
		return user.UserLevel, false
	}

	if upgraded {
		user.UserLevel = newLevel
		// 更新到存储
		ctx := context.Background()
		if _, err := c.store.User.Update(ctx, user); err != nil {
			log.Printf("[LevelUpgradeChecker] Failed to update user level: %v", err)
			return user.UserLevel, false
		}
	}

	return newLevel, upgraded
}

// CheckUserUpgrade 手动检查单个用户的升级条件（用于即时触发）
func (c *LevelUpgradeChecker) CheckUserUpgrade(ctx context.Context, user *model.User) (int32, bool) {
	return c.checkUpgrade(user)
}

// GetLevelThresholds 获取各级别升级条件
func GetLevelThresholds() map[int32]struct {
	Contribution int32
	Rating       int32
} {
	return map[int32]struct {
		Contribution int32
		Rating       int32
	}{
		model.UserLevelLv1: {Contribution: 0, Rating: 0},      // 邮箱验证
		model.UserLevelLv2: {Contribution: 10, Rating: 20},    // 活跃贡献者
		model.UserLevelLv3: {Contribution: 50, Rating: 100},   // 资深贡献者
		model.UserLevelLv4: {Contribution: 200, Rating: 500},  // 专家贡献者
		model.UserLevelLv5: {Contribution: 0, Rating: 0},      // 投票选举
	}
}
