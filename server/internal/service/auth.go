package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	UserStatusActive   = "active"
	maxLoginFailures   = 5
	loginFailureWindow = 5 * time.Minute
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAccountDisabled     = errors.New("account disabled")
	ErrAccountLocked       = errors.New("account temporarily locked")
	ErrDatabaseUnavailable = errors.New("database unavailable")
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
		return nil, ErrInvalidCredentials
	}
	if s.users == nil {
		logger.Error("User repository is not initialized")
		return nil, ErrDatabaseUnavailable
	}
	// 检查是否登录锁定
	if s.isLoginLocked(ctx, account) {
		logger.Warn("account login temporarily locked", zap.String("account", account))
		return nil, ErrAccountLocked
	}

	// 通过账户查找用户
	user, err := s.users.FindByAccount(ctx, account)
	if errors.Is(err, repository.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return nil, ErrDatabaseUnavailable
	}
	if errors.Is(err, repository.ErrUserNotFound) {
		logger.Warn("user not found", zap.String("account", account))
		if s.recordLoginFailure(ctx, account) {
			return nil, ErrAccountLocked
		}
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		logger.Error("failed to find user by account", zap.String("account", account), zap.Error(err))
		return nil, err
	}

	if user.Status != UserStatusActive {
		logger.Warn("account is disabled", zap.Uint64("user_id", user.ID), zap.String("account", account))
		return nil, ErrAccountDisabled
	}
	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		logger.Warn("invalid password", zap.Uint64("user_id", user.ID), zap.String("account", account))
		// 记录登录失败，如果达到阈值则锁定账号
		if s.recordLoginFailure(ctx, account) {
			return nil, ErrAccountLocked
		}
		return nil, ErrInvalidCredentials
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
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("user_id", user.ID), zap.Error(err))
			return nil, ErrDatabaseUnavailable
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
		return ErrDatabaseUnavailable
	}

	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		logger.Warn("user not found", zap.Uint64("user_id", userID))
		return ErrInvalidCredentials
	}
	if errors.Is(err, repository.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("user_id", userID), zap.Error(err))
		return ErrDatabaseUnavailable
	}
	if err != nil {
		logger.Error("failed to find user", zap.Uint64("user_id", userID), zap.Error(err))
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	// 生成新密码哈希
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("failed to hash new password", zap.Uint64("user_id", userID), zap.Error(err))
		return err
	}

	// 更新密码哈希
	if err := s.users.UpdatePasswordHash(ctx, userID, string(hashed)); err != nil {
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("user_id", userID), zap.Error(err))
			return ErrDatabaseUnavailable
		}
		logger.Error("failed to update password hash", zap.Uint64("user_id", userID), zap.Error(err))
		return err
	}

	return nil
}

// issueAccessToken 生成 JWT 访问令牌
func (s *AuthService) issueAccessToken(user *model.User, issuedAt time.Time, expiresAt time.Time) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	payload := map[string]any{
		"sub":     strconv.FormatUint(user.ID, 10),
		"uid":     user.ID,
		"account": user.Account,
		"role":    user.Role,
		"iss":     s.issuer,
		"iat":     issuedAt.Unix(),
		"exp":     expiresAt.Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		logger.Error("failed to marshal JWT header", zap.Uint64("user_id", user.ID), zap.Error(err))
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		logger.Error("failed to marshal JWT payload", zap.Uint64("user_id", user.ID), zap.Error(err))
		return "", err
	}

	// 使用 base64 URL 编码生成 JWT
	encoder := base64.RawURLEncoding
	unsigned := encoder.EncodeToString(headerJSON) + "." + encoder.EncodeToString(payloadJSON)

	// 使用 HMAC-SHA256 签名 JWT
	mac := hmac.New(sha256.New, s.jwtSecret)
	if _, err := mac.Write([]byte(unsigned)); err != nil {
		logger.Error("failed to sign JWT", zap.Uint64("user_id", user.ID), zap.Error(err))
		return "", err
	}

	return unsigned + "." + encoder.EncodeToString(mac.Sum(nil)), nil
}

// ParseAccessToken 验证并解析简单的 HS256 JWT，返回 uid 字段（用户 ID）
func (s *AuthService) ParseAccessToken(token string) (uint64, error) {
	if token == "" {
		return 0, errors.New("empty token")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, errors.New("invalid token format")
	}

	encoder := base64.RawURLEncoding
	unsigned := parts[0] + "." + parts[1]
	sig, err := encoder.DecodeString(parts[2])
	if err != nil {
		return 0, errors.New("invalid token signature encoding")
	}

	mac := hmac.New(sha256.New, s.jwtSecret)
	if _, err := mac.Write([]byte(unsigned)); err != nil {
		return 0, err
	}
	expected := mac.Sum(nil)
	if !hmac.Equal(expected, sig) {
		return 0, errors.New("invalid token signature")
	}

	// decode payload
	payloadJSON, err := encoder.DecodeString(parts[1])
	if err != nil {
		return 0, errors.New("invalid token payload encoding")
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return 0, errors.New("invalid token payload")
	}

	// extract uid
	uidAny, ok := payload["uid"]
	if !ok {
		return 0, errors.New("uid not found in token")
	}
	expAny, ok := payload["exp"]
	if !ok {
		return 0, errors.New("exp not found in token")
	}
	expiresAt, err := parseUnixTime(expAny)
	if err != nil {
		return 0, errors.New("invalid exp value")
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	if !now().Before(expiresAt) {
		return 0, errors.New("token expired")
	}

	switch v := uidAny.(type) {
	case float64:
		return uint64(v), nil
	case string:
		i, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, errors.New("invalid uid value")
		}
		return i, nil
	default:
		return 0, errors.New("invalid uid type")
	}
}

func parseUnixTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case float64:
		return time.Unix(int64(v), 0), nil
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(i, 0), nil
	default:
		return time.Time{}, errors.New("invalid unix time type")
	}
}
