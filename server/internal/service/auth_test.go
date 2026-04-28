package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserStore struct {
	user          *model.User
	findErr       error
	updateErr     error
	lastLoginAt   time.Time
	updatedUserID uint64
}

func (s *fakeUserStore) FindByAccount(context.Context, string) (*model.User, error) {
	return s.user, s.findErr
}

func (s *fakeUserStore) UpdateLastLoginAt(_ context.Context, id uint64, loggedInAt time.Time) error {
	s.updatedUserID = id
	s.lastLoginAt = loggedInAt
	return s.updateErr
}

func TestAuthServiceLoginSuccess(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Admin@123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate password hash: %v", err)
	}
	realName := "系统管理员"
	store := &fakeUserStore{
		user: &model.User{
			ID:           1,
			Account:      "admin",
			PasswordHash: string(passwordHash),
			Role:         "super_admin",
			RealName:     &realName,
			Status:       UserStatusActive,
		},
	}
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	svc := NewAuthService(store, config.Config{
		App: config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{
			JWTSecret:      "test-secret",
			AccessTokenTTL: time.Hour,
		},
	})
	svc.now = func() time.Time { return now }

	result, err := svc.Login(context.Background(), dto.LoginRequest{
		Account:  " admin ",
		Password: "Admin@123456",
	})
	if err != nil {
		t.Fatalf("expected successful login, got %v", err)
	}
	if result.User.Role != "super_admin" {
		t.Fatalf("expected super_admin role, got %q", result.User.Role)
	}
	if result.TokenType != "Bearer" {
		t.Fatalf("expected bearer token type, got %q", result.TokenType)
	}
	if len(strings.Split(result.AccessToken, ".")) != 3 {
		t.Fatalf("expected JWT with three segments, got %q", result.AccessToken)
	}
	if !result.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expected expiry %s, got %s", now.Add(time.Hour), result.ExpiresAt)
	}
	if store.updatedUserID != 1 || !store.lastLoginAt.Equal(now) {
		t.Fatalf("expected last login update for user 1 at %s", now)
	}
}

func TestAuthServiceLoginInvalidPassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Admin@123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate password hash: %v", err)
	}
	svc := NewAuthService(&fakeUserStore{
		user: &model.User{
			ID:           1,
			Account:      "admin",
			PasswordHash: string(passwordHash),
			Role:         "super_admin",
			Status:       UserStatusActive,
		},
	}, config.Config{})

	_, err = svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestAuthServiceLoginDisabledUser(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Admin@123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate password hash: %v", err)
	}
	svc := NewAuthService(&fakeUserStore{
		user: &model.User{
			ID:           1,
			Account:      "admin",
			PasswordHash: string(passwordHash),
			Role:         "super_admin",
			Status:       "disabled",
		},
	}, config.Config{})

	_, err = svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "Admin@123456",
	})
	if !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("expected account disabled, got %v", err)
	}
}

func TestAuthServiceLoginUserNotFound(t *testing.T) {
	svc := NewAuthService(&fakeUserStore{findErr: repository.ErrUserNotFound}, config.Config{})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "Admin@123456",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}
