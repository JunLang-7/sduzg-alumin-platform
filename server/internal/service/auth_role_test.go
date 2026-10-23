package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthServiceLoginRejectsInvalidDatabaseRole(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"", "owner", "Admin", "admin "} {
		for _, req := range []dto.LoginRequest{
			{Account: "manager", Password: "Password123"},
			{Mobile: "13800138000", Code: "123456", GrantType: "sms_code"},
			{Email: "manager@example.com", Code: "123456", GrantType: "email_code"},
		} {
			t.Run(role+"/"+req.GrantType+req.Account, func(t *testing.T) {
				users := &fakeUserStore{user: &model.User{ID: 7, Role: role, Status: common.UserStatusActive, PasswordHash: string(hash)}}
				attempts := &fakeLoginAttemptStore{failureCount: 2}
				svc := NewAuthService(users, &fakeAlumniStoreForAuth{profile: &model.AlumniProfile{ID: 10}}, attempts,
					&fakeVerifyCodeStore{savedCode: "123456", verifyOk: true}, config.Config{
						Auth: config.AuthConfig{JWTSecret: "test-secret", AccessTokenTTL: time.Hour},
					})
				result, err := svc.Login(context.Background(), req)
				if result != nil || !errors.Is(err, common.ErrInvalidAccountRole) {
					t.Fatalf("invalid role must not receive a token: result=%+v err=%v", result, err)
				}
				if users.updatedUserID != 0 || !users.lastLoginAt.IsZero() || attempts.cleared || attempts.failureCount != 2 {
					t.Fatal("invalid role must not record a successful login or clear failure history")
				}
			})
		}
	}
}

func TestInvalidRoleDoesNotRevealRoleWhenPasswordIsWrong(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUserStore{user: &model.User{ID: 7, Role: "Admin", Status: common.UserStatusActive, PasswordHash: string(hash)}}
	attempts := &fakeLoginAttemptStore{}
	svc := NewAuthService(users, nil, attempts, nil, config.Config{})
	result, err := svc.Login(context.Background(), dto.LoginRequest{Account: "manager", Password: "wrong-password"})
	if result != nil || !errors.Is(err, common.ErrInvalidCredentials) || attempts.failureCount != 1 {
		t.Fatalf("wrong credentials must retain the generic failure: result=%+v err=%v failures=%d", result, err, attempts.failureCount)
	}
}

func TestCompleteLoginAcceptsEveryKnownDatabaseRole(t *testing.T) {
	for _, role := range []string{common.RoleAlumni, common.RoleAdmin, common.RoleSuperAdmin} {
		t.Run(role, func(t *testing.T) {
			user := &model.User{ID: 7, Role: role, Status: common.UserStatusActive}
			users := &fakeUserStore{user: user}
			attempts := &fakeLoginAttemptStore{failureCount: 2}
			svc := NewAuthService(users, nil, attempts, nil, config.Config{Auth: config.AuthConfig{JWTSecret: "test-secret", AccessTokenTTL: time.Hour}})
			result, err := svc.completeLogin(context.Background(), user, "test-identifier")
			if err != nil {
				t.Fatal(err)
			}
			uid, err := svc.ParseAccessToken(result.AccessToken)
			if err != nil || uid != user.ID || result.User.Role != role || users.updatedUserID != user.ID || !attempts.cleared {
				t.Fatalf("known role login failed: uid=%d result=%+v err=%v", uid, result, err)
			}
		})
	}
}
