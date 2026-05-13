package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxLoginFailures   = 5
	loginFailureWindow = 5 * time.Minute
)

type AuthService struct {
	users          repository.UserStore
	loginAttempts  repository.LoginAttemptStore
	jwtSecret      []byte
	accessTokenTTL time.Duration
	issuer         string
	now            func() time.Time
}

func NewAuthService(users repository.UserStore, loginAttempts repository.LoginAttemptStore, cfg config.Config) *AuthService {
	return &AuthService{
		users:          users,
		loginAttempts:  loginAttempts,
		jwtSecret:      []byte(cfg.Auth.JWTSecret),
		accessTokenTTL: cfg.Auth.AccessTokenTTL,
		issuer:         cfg.App.Name,
		now:            time.Now,
	}
}

// Login 验证用户凭据并颁发访问令牌
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResult, error) {
	account := strings.TrimSpace(req.Account)
	if account == "" || req.Password == "" {
		logger.Error("Login failed", zap.String("account", account))
		return nil, common.ErrInvalidCredentials
	}
	if s.users == nil {
		logger.Error("User repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	// 检查是否登录锁定
	if s.isLoginLocked(ctx, account) {
		logger.Warn("account login temporarily locked", zap.String("account", account))
		return nil, common.ErrAccountLocked
	}

	// 通过账户查找用户
	user, err := s.users.FindByAccount(ctx, account)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrUserNotFound) {
		logger.Warn("user not found", zap.String("account", account))
		if s.recordLoginFailure(ctx, account) {
			return nil, common.ErrAccountLocked
		}
		return nil, common.ErrInvalidCredentials
	}
	if err != nil {
		logger.Error("failed to find user by account", zap.String("account", account), zap.Error(err))
		return nil, err
	}

	if user.Status != common.UserStatusActive {
		logger.Warn("account is disabled", zap.Uint64("user_id", user.ID), zap.String("account", account))
		return nil, common.ErrAccountDisabled
	}
	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		logger.Warn("invalid password", zap.Uint64("user_id", user.ID), zap.String("account", account))
		// 记录登录失败，如果达到阈值则锁定账号
		if s.recordLoginFailure(ctx, account) {
			return nil, common.ErrAccountLocked
		}
		return nil, common.ErrInvalidCredentials
	}

	issuedAt := s.now()
	expiresAt := issuedAt.Add(s.accessTokenTTL)
	// 颁发访问令牌
	token, err := s.issueAccessToken(user, issuedAt, expiresAt)
	if err != nil {
		logger.Error("failed to issue access token", zap.Uint64("user_id", user.ID), zap.Error(err))
		return nil, err
	}
	// 更新用户最后登录时间
	if err := s.users.UpdateLastLoginAt(ctx, user.ID, issuedAt); err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("user_id", user.ID), zap.Error(err))
			return nil, common.ErrDatabaseUnavailable
		}
		logger.Error("failed to update last login time", zap.Uint64("user_id", user.ID), zap.Error(err))
		return nil, err
	}
	// 登录成功，清除登录失败记录
	s.clearLoginFailures(ctx, account)

	return &dto.LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User: dto.UserDTO{
			ID:       user.ID,
			Account:  user.Account,
			Role:     user.Role,
			RealName: user.RealName,
		},
	}, nil
}

// isLoginLocked 检查账号是否因连续登录失败而被临时锁定
func (s *AuthService) isLoginLocked(ctx context.Context, account string) bool {
	if s.loginAttempts == nil {
		return false
	}

	count, err := s.loginAttempts.FailureCount(ctx, account)
	if err != nil {
		logger.Warn("failed to read login failure count", zap.String("account", account), zap.Error(err))
		return false
	}
	return count >= maxLoginFailures
}

// recordLoginFailure 记录一次登录失败，并返回当前失败次数。如果达到锁定阈值，返回 true。
func (s *AuthService) recordLoginFailure(ctx context.Context, account string) bool {
	if s.loginAttempts == nil {
		return false
	}

	count, err := s.loginAttempts.RecordFailure(ctx, account, loginFailureWindow)
	if err != nil {
		logger.Warn("failed to record login failure", zap.String("account", account), zap.Error(err))
		return false
	}
	return count >= maxLoginFailures
}

// clearLoginFailures 清除指定账号的登录失败记录
func (s *AuthService) clearLoginFailures(ctx context.Context, account string) {
	if s.loginAttempts == nil {
		return
	}

	if err := s.loginAttempts.ClearFailures(ctx, account); err != nil {
		logger.Warn("failed to clear login failures", zap.String("account", account), zap.Error(err))
	}
}

// Logout 完成退出登录。当前 JWT 为无状态模式，服务端不保存会话，客户端删除 token 即可。
func (s *AuthService) Logout(context.Context) (*dto.LogoutResult, error) {
	return &dto.LogoutResult{
		LoggedOut: true,
	}, nil
}

// ChangePassword 校验旧密码并更新为新密码
func (s *AuthService) ChangePassword(ctx context.Context, userID uint64, oldPassword string, newPassword string) error {
	if s.users == nil {
		return common.ErrDatabaseUnavailable
	}

	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, common.ErrUserNotFound) {
		logger.Warn("user not found", zap.Uint64("user_id", userID))
		return common.ErrInvalidCredentials
	}
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("user_id", userID), zap.Error(err))
		return common.ErrDatabaseUnavailable
	}
	if err != nil {
		logger.Error("failed to find user", zap.Uint64("user_id", userID), zap.Error(err))
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return common.ErrInvalidCredentials
	}

	// 生成新密码哈希
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("failed to hash new password", zap.Uint64("user_id", userID), zap.Error(err))
		return err
	}

	// 更新密码哈希
	if err := s.users.UpdatePasswordHash(ctx, userID, string(hashed)); err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("user_id", userID), zap.Error(err))
			return common.ErrDatabaseUnavailable
		}
		logger.Error("failed to update password hash", zap.Uint64("user_id", userID), zap.Error(err))
		return err
	}

	return nil
}

type customClaims struct {
	jwt.RegisteredClaims
	UID     uint64 `json:"uid"`
	Account string `json:"account"`
	Role    string `json:"role"`
}

func (s *AuthService) issueAccessToken(user *model.User, issuedAt time.Time, expiresAt time.Time) (string, error) {
	claims := customClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   strconv.FormatUint(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		UID:     user.ID,
		Account: user.Account,
		Role:    user.Role,
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		logger.Error("failed to sign JWT", zap.Uint64("user_id", user.ID), zap.Error(err))
		return "", err
	}
	return token, nil
}

// ParseAccessToken 验证并解析 JWT，返回 uid 字段（用户 ID）
func (s *AuthService) ParseAccessToken(tokenString string) (uint64, error) {
	if tokenString == "" {
		return 0, errors.New("empty token")
	}

	claims := &customClaims{}
	opts := []jwt.ParserOption{}
	if s.now != nil {
		opts = append(opts, jwt.WithTimeFunc(s.now))
	}

	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	}, opts...)
	if err != nil {
		return 0, err
	}

	return claims.UID, nil
}
