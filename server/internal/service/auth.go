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
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

const (
	UserStatusActive = "active"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAccountDisabled     = errors.New("account disabled")
	ErrDatabaseUnavailable = errors.New("database unavailable")
)

type AuthService struct {
	users          repository.UserStore
	jwtSecret      []byte
	accessTokenTTL time.Duration
	issuer         string
	now            func() time.Time
}

func NewAuthService(users repository.UserStore, cfg config.Config) *AuthService {
	return &AuthService{
		users:          users,
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
		return nil, ErrInvalidCredentials
	}
	if s.users == nil {
		return nil, ErrDatabaseUnavailable
	}

	// 通过账户查找用户
	user, err := s.users.FindByAccount(ctx, account)
	if errors.Is(err, repository.ErrDatabaseUnavailable) {
		return nil, ErrDatabaseUnavailable
	}
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if user.Status != UserStatusActive {
		return nil, ErrAccountDisabled
	}
	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	issuedAt := s.now()
	expiresAt := issuedAt.Add(s.accessTokenTTL)
	// 颁发访问令牌
	token, err := s.issueAccessToken(user, issuedAt, expiresAt)
	if err != nil {
		return nil, err
	}
	// 更新用户最后登录时间
	if err := s.users.UpdateLastLoginAt(ctx, user.ID, issuedAt); err != nil {
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return nil, ErrDatabaseUnavailable
		}
		return nil, err
	}

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
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// 使用 base64 URL 编码生成 JWT
	encoder := base64.RawURLEncoding
	unsigned := encoder.EncodeToString(headerJSON) + "." + encoder.EncodeToString(payloadJSON)

	// 使用 HMAC-SHA256 签名 JWT
	mac := hmac.New(sha256.New, s.jwtSecret)
	if _, err := mac.Write([]byte(unsigned)); err != nil {
		return "", err
	}

	return unsigned + "." + encoder.EncodeToString(mac.Sum(nil)), nil
}
