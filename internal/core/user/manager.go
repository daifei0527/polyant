package user

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/agentwiki/agentwiki/internal/auth/ed25519"
	"github.com/agentwiki/agentwiki/internal/storage"
	"github.com/agentwiki/agentwiki/internal/storage/model"
)

var (
	ErrUserAlreadyExists   = fmt.Errorf("用户已存在")
	ErrUserNotFound        = fmt.Errorf("用户不存在")
	ErrInvalidSignature    = fmt.Errorf("签名验证失败")
	ErrEmailAlreadyUsed    = fmt.Errorf("邮箱已被使用")
	ErrInvalidEmail        = fmt.Errorf("无效的邮箱地址")
)

type UserManager struct {
	store *storage.Store
}

func NewUserManager(store *storage.Store) *UserManager {
	return &UserManager{
		store: store,
	}
}

func (m *UserManager) Register(ctx context.Context, publicKey, agentName string) (*model.User, error) {
	pubKeyHash := HashPublicKey(publicKey)
	_, err := m.store.User.Get(ctx, pubKeyHash)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}

	now := time.Now().UnixMilli()

	user := &model.User{
		PublicKey:      publicKey,
		AgentName:      agentName,
		UserLevel:      model.UserLevelLv0,
		Email:          "",
		EmailVerified:  false,
		RegisteredAt:   now,
		LastActive:     now,
		ContributionCnt: 0,
		RatingCnt:      0,
		NodeId:         "",
		Status:         "active",
	}

	_, err = m.store.User.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return user, nil
}

func (m *UserManager) RegisterWithSignature(ctx context.Context, publicKey, agentName string, signature []byte) (*model.User, error) {
	message := []byte(fmt.Sprintf("register:%s:%s", publicKey, agentName))
	pubKeyBytes, err := ed25519.StringToPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("解析公钥失败: %w", err)
	}

	if !ed25519.Verify(pubKeyBytes, message, signature) {
		return nil, ErrInvalidSignature
	}

	return m.Register(ctx, publicKey, agentName)
}

func (m *UserManager) GetByPublicKey(ctx context.Context, publicKey string) (*model.User, error) {
	pubKeyHash := HashPublicKey(publicKey)
	user, err := m.store.User.Get(ctx, pubKeyHash)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (m *UserManager) UpdateLastActive(ctx context.Context, publicKey string) error {
	user, err := m.GetByPublicKey(ctx, publicKey)
	if err != nil {
		return err
	}

	user.LastActive = time.Now().UnixMilli()
	_, err = m.store.User.Update(ctx, user)
	return err
}

func (m *UserManager) VerifyEmail(ctx context.Context, publicKey, email string) error {
	user, err := m.GetByPublicKey(ctx, publicKey)
	if err != nil {
		return err
	}

	if user.Email != email {
		return ErrInvalidEmail
	}

	user.EmailVerified = true
	if user.UserLevel == model.UserLevelLv0 {
		user.UserLevel = model.UserLevelLv1
	}

	_, err = m.store.User.Update(ctx, user)
	return err
}

func (m *UserManager) SetEmail(ctx context.Context, publicKey, email string) error {
	user, err := m.GetByPublicKey(ctx, publicKey)
	if err != nil {
		return err
	}

	user.Email = email
	user.EmailVerified = false
	_, err = m.store.User.Update(ctx, user)
	return err
}

func (m *UserManager) CheckLevelUpgrade(ctx context.Context, user *model.User) (int32, bool) {
	newLevel := user.UserLevel

	switch user.UserLevel {
	case model.UserLevelLv1:
		if user.ContributionCnt >= 10 && user.RatingCnt >= 20 {
			newLevel = model.UserLevelLv2
		}
	case model.UserLevelLv2:
		if user.ContributionCnt >= 50 && user.RatingCnt >= 100 {
			newLevel = model.UserLevelLv3
		}
	case model.UserLevelLv3:
		if user.ContributionCnt >= 200 && user.RatingCnt >= 500 {
			newLevel = model.UserLevelLv4
		}
	}

	if newLevel > user.UserLevel {
		user.UserLevel = newLevel
		m.store.User.Update(ctx, user)
		return newLevel, true
	}

	return user.UserLevel, false
}

func (m *UserManager) IncrementContribution(ctx context.Context, publicKey string) error {
	user, err := m.GetByPublicKey(ctx, publicKey)
	if err != nil {
		return err
	}

	user.ContributionCnt++
	m.store.User.Update(ctx, user)
	m.CheckLevelUpgrade(ctx, user)
	return nil
}

func HashPublicKey(publicKey string) string {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		hash := sha256.Sum256([]byte(publicKey))
		return hex.EncodeToString(hash[:])
	}
	hash := sha256.Sum256(pubKeyBytes)
	return hex.EncodeToString(hash[:])
}
