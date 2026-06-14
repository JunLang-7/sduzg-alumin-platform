package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserStore struct {
	user          *model.User
	usersByID     map[uint64]*model.User
	users         []*model.User
	total         int64
	created       *model.User
	createProfile do.AdminCreateProfile
	createHash    string
	deleteID      uint64
	findErr       error
	listErr       error
	createErr     error
	deleteErr     error
	updateErr     error
	lastLoginAt   time.Time
	updatedUserID uint64
}

func (s *fakeUserStore) FindByAccount(context.Context, string) (*model.User, error) {
	return s.user, s.findErr
}

func (s *fakeUserStore) FindByID(_ context.Context, id uint64) (*model.User, error) {
	if s.usersByID != nil {
		user, ok := s.usersByID[id]
		if !ok {
			return nil, common.ErrUserNotFound
		}
		return user, s.findErr
	}
	return s.user, s.findErr
}

func (s *fakeUserStore) ListAdmins(_ context.Context, _ do.AdminListQuery) ([]*model.User, int64, error) {
	return s.users, s.total, s.listErr
}

func (s *fakeUserStore) CreateAdmin(_ context.Context, profile do.AdminCreateProfile, passwordHash string) (*model.User, error) {
	s.createProfile = profile
	s.createHash = passwordHash
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.created != nil {
		return s.created, nil
	}
	return &model.User{
		ID:           2,
		Account:      profile.Account,
		PasswordHash: passwordHash,
		Role:         common.RoleAdmin,
		RealName:     profile.RealName,
		Mobile:       profile.Mobile,
		Status:       common.UserStatusActive,
	}, nil
}

func (s *fakeUserStore) DeleteAdmin(_ context.Context, id uint64) error {
	s.deleteID = id
	return s.deleteErr
}

func (s *fakeUserStore) UpdateLastLoginAt(_ context.Context, id uint64, loggedInAt time.Time) error {
	s.updatedUserID = id
	s.lastLoginAt = loggedInAt
	return s.updateErr
}

func (s *fakeUserStore) UpdatePasswordHash(_ context.Context, id uint64, passwordHash string) error {
	if s.user != nil && s.user.ID == id {
		s.user.PasswordHash = passwordHash
	}
	return s.updateErr
}

func (s *fakeUserStore) FindByMobile(_ context.Context, _ string) (*model.User, error) {
	return s.user, s.findErr
}

func (s *fakeUserStore) FindByEmail(_ context.Context, _ string) (*model.User, error) {
	return s.user, s.findErr
}

func (s *fakeUserStore) FindByAlumniID(_ context.Context, _ uint64) (*model.User, error) {
	return s.user, s.findErr
}

func (s *fakeUserStore) CreateUser(_ context.Context, _ *model.User) error {
	return s.createErr
}

func (s *fakeUserStore) UpdateMobile(_ context.Context, _ uint64, _ string) error {
	return s.updateErr
}

func (s *fakeUserStore) UpdateEmail(_ context.Context, _ uint64, _ string) error {
	return s.updateErr
}

type fakeLoginAttemptStore struct {
	failureCount int
	recordErr    error
	clearErr     error
	cleared      bool
}

func (s *fakeLoginAttemptStore) FailureCount(context.Context, string) (int, error) {
	return s.failureCount, nil
}

func (s *fakeLoginAttemptStore) RecordFailure(context.Context, string, time.Duration) (int, error) {
	if s.recordErr != nil {
		return 0, s.recordErr
	}
	s.failureCount++
	return s.failureCount, nil
}

func (s *fakeLoginAttemptStore) ClearFailures(context.Context, string) error {
	s.cleared = true
	return s.clearErr
}

type fakeAlumniStoreForAuth struct {
	profile   *model.AlumniProfile
	findErr   error
	updateErr error
	updatedID uint64
	updatedMB string
	updatedEM string
}

func (s *fakeAlumniStoreForAuth) List(_ context.Context, _ do.AlumniListQuery) ([]*model.AlumniProfile, int64, error) {
	return nil, 0, nil
}

func (s *fakeAlumniStoreForAuth) ListAll(_ context.Context, _ do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	return nil, nil
}

func (s *fakeAlumniStoreForAuth) GetByID(_ context.Context, _ uint64) (*model.AlumniProfile, error) {
	return nil, common.ErrAlumniNotFound
}

func (s *fakeAlumniStoreForAuth) Create(_ context.Context, _ *do.AlumniCreateProfile, _ uint64) (*model.AlumniProfile, error) {
	return nil, nil
}

func (s *fakeAlumniStoreForAuth) BatchCreate(_ context.Context, _ []do.AlumniCreateProfile, _ uint64) error {
	return nil
}

func (s *fakeAlumniStoreForAuth) FindExistingByDedupKey(_ context.Context, _ []do.AlumniDedupKey) (map[string]bool, error) {
	return nil, nil
}

func (s *fakeAlumniStoreForAuth) Update(_ context.Context, _ uint64, _ uint64, _ do.AlumniUpdateProfile) error {
	return nil
}

func (s *fakeAlumniStoreForAuth) Delete(_ context.Context, _ uint64, _ uint64) error {
	return nil
}

func (s *fakeAlumniStoreForAuth) UpdateEditableFields(_ context.Context, _ uint64, _ uint64, _ do.AlumniEditableProfile) error {
	return nil
}

func (s *fakeAlumniStoreForAuth) CountActive(_ context.Context) (int64, error) {
	return 0, nil
}

func (s *fakeAlumniStoreForAuth) FindOnly(_ context.Context, _ do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	return nil, nil
}

func (s *fakeAlumniStoreForAuth) FindByMobile(_ context.Context, _ string) (*model.AlumniProfile, error) {
	return s.profile, s.findErr
}

func (s *fakeAlumniStoreForAuth) FindByEmail(_ context.Context, _ string) (*model.AlumniProfile, error) {
	return s.profile, s.findErr
}

func (s *fakeAlumniStoreForAuth) UpdateMobile(_ context.Context, id uint64, mobile string) error {
	s.updatedID = id
	s.updatedMB = mobile
	return s.updateErr
}

func (s *fakeAlumniStoreForAuth) UpdateEmail(_ context.Context, id uint64, email string) error {
	s.updatedID = id
	s.updatedEM = email
	return s.updateErr
}

type fakeVerifyCodeStore struct {
	savedTarget string
	savedCode   string
	verifyOk    bool
	verifyErr   error
	sendCount   int64
	lastSend    time.Time
}

func (s *fakeVerifyCodeStore) Save(_ context.Context, target, code string) error {
	s.savedTarget = target
	s.savedCode = code
	return nil
}

func (s *fakeVerifyCodeStore) Verify(_ context.Context, _, code string) (bool, error) {
	if s.verifyErr != nil {
		return false, s.verifyErr
	}
	return code == s.savedCode || (s.verifyOk && code != ""), nil
}

func (s *fakeVerifyCodeStore) IncrementSendCount(_ context.Context, _ string) (int64, error) {
	s.sendCount++
	return s.sendCount, nil
}

func (s *fakeVerifyCodeStore) LastSendTime(_ context.Context, _ string) (time.Time, error) {
	return s.lastSend, nil
}

// ============================================================
// Tests
// ============================================================

// contactUserStore is a UserStore for UpdateContact tests that returns
// ErrUserNotFound from FindByMobile/FindByEmail so uniqueness checks pass.
type contactUserStore struct {
	fakeUserStore
}

func (s *contactUserStore) FindByMobile(_ context.Context, _ string) (*model.User, error) {
	return nil, common.ErrUserNotFound
}

func (s *contactUserStore) FindByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, common.ErrUserNotFound
}

// contactUserStoreDuplicate returns a user from FindByMobile to simulate duplicate.
type contactUserStoreDuplicate struct {
	fakeUserStore
}

func (s *contactUserStoreDuplicate) FindByMobile(_ context.Context, _ string) (*model.User, error) {
	return &model.User{ID: 99}, nil
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
			Status:       common.UserStatusActive,
		},
	}
	attempts := &fakeLoginAttemptStore{failureCount: 3}
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	svc := NewAuthService(store, nil, attempts, nil, config.Config{
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
	if !attempts.cleared {
		t.Fatal("expected login failures to be cleared")
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
			Status:       common.UserStatusActive,
		},
	}, nil, &fakeLoginAttemptStore{}, nil, config.Config{})

	_, err = svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "wrong-password",
	})
	if !errors.Is(err, common.ErrInvalidCredentials) {
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
	}, nil, &fakeLoginAttemptStore{}, nil, config.Config{})

	_, err = svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "Admin@123456",
	})
	if !errors.Is(err, common.ErrAccountDisabled) {
		t.Fatalf("expected account disabled, got %v", err)
	}
}

func TestAuthServiceLoginUserNotFound(t *testing.T) {
	svc := NewAuthService(&fakeUserStore{findErr: common.ErrUserNotFound}, nil, &fakeLoginAttemptStore{}, nil, config.Config{})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "Admin@123456",
	})
	if !errors.Is(err, common.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestAuthServiceLoginLocksOnFifthFailure(t *testing.T) {
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
			Status:       common.UserStatusActive,
		},
	}, nil, &fakeLoginAttemptStore{failureCount: 4}, nil, config.Config{})

	_, err = svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "wrong-password",
	})
	if !errors.Is(err, common.ErrAccountLocked) {
		t.Fatalf("expected account locked, got %v", err)
	}
}

func TestAuthServiceLoginRejectsLockedAccount(t *testing.T) {
	svc := NewAuthService(&fakeUserStore{}, nil, &fakeLoginAttemptStore{failureCount: 5}, nil, config.Config{})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "Admin@123456",
	})
	if !errors.Is(err, common.ErrAccountLocked) {
		t.Fatalf("expected account locked, got %v", err)
	}
}

func TestAuthServiceLogout(t *testing.T) {
	svc := NewAuthService(nil, nil, nil, nil, config.Config{})

	result, err := svc.Logout(context.Background())
	if err != nil {
		t.Fatalf("expected logout success, got %v", err)
	}
	if !result.LoggedOut {
		t.Fatal("expected logged out result")
	}
}

func TestAuthServiceParseAccessToken(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	svc := NewAuthService(nil, nil, nil, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})
	svc.now = func() time.Time { return now }

	token, err := svc.issueAccessToken(&model.User{ID: 1, Account: "admin", Role: "super_admin"}, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	uid, err := svc.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("expected token to parse, got %v", err)
	}
	if uid != 1 {
		t.Fatalf("expected uid 1, got %d", uid)
	}
}

func TestAuthServiceParseAccessTokenRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	svc := NewAuthService(nil, nil, nil, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})
	svc.now = func() time.Time { return now }

	token, err := svc.issueAccessToken(&model.User{ID: 1, Account: "admin", Role: "super_admin"}, now.Add(-2*time.Hour), now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	if _, err := svc.ParseAccessToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

// ============================================================
// New tests: Phone/Email password login auto-detection
// ============================================================

func TestAuthServiceLoginByPhonePassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate password hash: %v", err)
	}
	store := &fakeUserStore{
		user: &model.User{
			ID:           2,
			Account:      "13800138000",
			PasswordHash: string(passwordHash),
			Role:         common.RoleAlumni,
			Status:       common.UserStatusActive,
		},
	}

	svc := NewAuthService(store, nil, &fakeLoginAttemptStore{}, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret", AccessTokenTTL: time.Hour},
	})
	svc.now = func() time.Time { return time.Now() }

	result, err := svc.Login(context.Background(), dto.LoginRequest{
		Mobile:   "13800138000",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("expected successful login by phone, got %v", err)
	}
	if result.User.Role != common.RoleAlumni {
		t.Fatalf("expected alumni role, got %q", result.User.Role)
	}
}

func TestAuthServiceLoginByEmailPassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate password hash: %v", err)
	}
	store := &fakeUserStore{
		user: &model.User{
			ID:           3,
			Account:      "user@example.com",
			PasswordHash: string(passwordHash),
			Role:         common.RoleAlumni,
			Status:       common.UserStatusActive,
		},
	}

	svc := NewAuthService(store, nil, &fakeLoginAttemptStore{}, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret", AccessTokenTTL: time.Hour},
	})
	svc.now = func() time.Time { return time.Now() }

	result, err := svc.Login(context.Background(), dto.LoginRequest{
		Email:    "user@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("expected successful login by email, got %v", err)
	}
	if result.User.Role != common.RoleAlumni {
		t.Fatalf("expected alumni role, got %q", result.User.Role)
	}
}

func TestAuthServiceLoginByAccountRestrictsAlumni(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate password hash: %v", err)
	}
	store := &fakeUserStore{
		user: &model.User{
			ID:           4,
			Account:      "alumni_user",
			PasswordHash: string(passwordHash),
			Role:         common.RoleAlumni,
			Status:       common.UserStatusActive,
		},
	}

	svc := NewAuthService(store, nil, &fakeLoginAttemptStore{}, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	// alumni trying to login via `account` field -> should be rejected
	_, err = svc.Login(context.Background(), dto.LoginRequest{
		Account:  "alumni_user",
		Password: "Password123",
	})
	if !errors.Is(err, common.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials for alumni using account login, got %v", err)
	}
}

func TestAuthServiceLoginAdminAccountStillWorks(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Admin@123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate password hash: %v", err)
	}
	store := &fakeUserStore{
		user: &model.User{
			ID:           1,
			Account:      "admin",
			PasswordHash: string(passwordHash),
			Role:         common.RoleAdmin,
			Status:       common.UserStatusActive,
		},
	}

	svc := NewAuthService(store, nil, &fakeLoginAttemptStore{}, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret", AccessTokenTTL: time.Hour},
	})
	svc.now = func() time.Time { return time.Now() }

	result, err := svc.Login(context.Background(), dto.LoginRequest{
		Account:  "admin",
		Password: "Admin@123456",
	})
	if err != nil {
		t.Fatalf("expected successful admin login, got %v", err)
	}
	if result.User.Role != common.RoleAdmin {
		t.Fatalf("expected admin role, got %q", result.User.Role)
	}
}

func TestAuthServiceLoginByPhoneNotFoundRecordsFailure(t *testing.T) {
	store := &fakeUserStore{findErr: common.ErrUserNotFound}
	attempts := &fakeLoginAttemptStore{}

	svc := NewAuthService(store, nil, attempts, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Mobile:   "13900001111",
		Password: "wrong",
	})
	if !errors.Is(err, common.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if attempts.failureCount == 0 {
		t.Fatal("expected failure to be recorded")
	}
}

// ============================================================
// Tests: SMS code login
// ============================================================

func TestAuthServiceSmsCodeLoginExistingUser(t *testing.T) {
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("temp"), bcrypt.MinCost)
	realName := "张三"
	alumniID := uint64(10)
	store := &fakeUserStore{
		user: &model.User{
			ID:           5,
			Account:      "13800138000",
			PasswordHash: string(passwordHash),
			Role:         common.RoleAlumni,
			Status:       common.UserStatusActive,
			RealName:     &realName,
			AlumniID:     &alumniID,
		},
	}
	alumni := &fakeAlumniStoreForAuth{
		profile: &model.AlumniProfile{
			ID:     10,
			Name:   "张三",
			Grade:  "2020级",
			Mobile: new("13800138000"),
		},
	}
	verifyCode := &fakeVerifyCodeStore{
		savedCode: "123456",
		verifyOk:  true,
	}

	svc := NewAuthService(store, alumni, &fakeLoginAttemptStore{}, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret", AccessTokenTTL: time.Hour},
	})
	svc.now = func() time.Time { return time.Now() }

	result, err := svc.Login(context.Background(), dto.LoginRequest{
		Mobile:    "13800138000",
		Code:      "123456",
		GrantType: "sms_code",
	})
	if err != nil {
		t.Fatalf("expected sms login success, got %v", err)
	}
	if result.User.Role != common.RoleAlumni {
		t.Fatalf("expected alumni role, got %q", result.User.Role)
	}
}

func TestAuthServiceSmsCodeLoginNoAlumniProfile(t *testing.T) {
	alumni := &fakeAlumniStoreForAuth{findErr: common.ErrAlumniNotFound}
	verifyCode := &fakeVerifyCodeStore{savedCode: "123456", verifyOk: true}

	svc := NewAuthService(nil, alumni, &fakeLoginAttemptStore{}, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Mobile:    "13800138000",
		Code:      "123456",
		GrantType: "sms_code",
	})
	if !errors.Is(err, common.ErrAlumniNotMatch) {
		t.Fatalf("expected alumni not match error, got %v", err)
	}
}

func TestAuthServiceSmsCodeLoginInvalidCode(t *testing.T) {
	alumni := &fakeAlumniStoreForAuth{
		profile: &model.AlumniProfile{ID: 1, Name: "张三", Grade: "2020"},
	}
	verifyCode := &fakeVerifyCodeStore{verifyOk: false}

	svc := NewAuthService(nil, alumni, &fakeLoginAttemptStore{}, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Mobile:    "13800138000",
		Code:      "000000",
		GrantType: "sms_code",
	})
	if !errors.Is(err, common.ErrCodeInvalid) {
		t.Fatalf("expected code invalid error, got %v", err)
	}
}

func TestAuthServiceSmsCodeLoginAccountLocked(t *testing.T) {
	attempts := &fakeLoginAttemptStore{failureCount: 5}

	svc := NewAuthService(nil, nil, attempts, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Mobile:    "13800138000",
		Code:      "123456",
		GrantType: "sms_code",
	})
	if !errors.Is(err, common.ErrAccountLocked) {
		t.Fatalf("expected account locked error, got %v", err)
	}
}

// ============================================================
// Tests: Email code login
// ============================================================

func TestAuthServiceEmailCodeLoginExistingUser(t *testing.T) {
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("temp"), bcrypt.MinCost)
	alumniID := uint64(11)
	store := &fakeUserStore{
		user: &model.User{
			ID:           6,
			Account:      "user@example.com",
			PasswordHash: string(passwordHash),
			Role:         common.RoleAlumni,
			Status:       common.UserStatusActive,
			AlumniID:     &alumniID,
		},
	}
	alumni := &fakeAlumniStoreForAuth{
		profile: &model.AlumniProfile{
			ID:    11,
			Name:  "李四",
			Grade: "2021级",
			Email: new("user@example.com"),
		},
	}
	verifyCode := &fakeVerifyCodeStore{savedCode: "654321", verifyOk: true}

	svc := NewAuthService(store, alumni, &fakeLoginAttemptStore{}, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret", AccessTokenTTL: time.Hour},
	})
	svc.now = func() time.Time { return time.Now() }

	result, err := svc.Login(context.Background(), dto.LoginRequest{
		Email:     "User@Example.com",
		Code:      "654321",
		GrantType: "email_code",
	})
	if err != nil {
		t.Fatalf("expected email code login success, got %v", err)
	}
	if result.User.Role != common.RoleAlumni {
		t.Fatalf("expected alumni role, got %q", result.User.Role)
	}
}

func TestAuthServiceEmailCodeLoginNoAlumniProfile(t *testing.T) {
	alumni := &fakeAlumniStoreForAuth{findErr: common.ErrAlumniNotFound}
	verifyCode := &fakeVerifyCodeStore{savedCode: "123456", verifyOk: true}

	svc := NewAuthService(nil, alumni, &fakeLoginAttemptStore{}, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Email:     "nobody@example.com",
		Code:      "123456",
		GrantType: "email_code",
	})
	if !errors.Is(err, common.ErrAlumniNotMatch) {
		t.Fatalf("expected alumni not match error, got %v", err)
	}
}

// ============================================================
// Tests: SendVerifyCode
// ============================================================

func TestAuthServiceSendVerifyCodeSuccess(t *testing.T) {
	verifyCode := &fakeVerifyCodeStore{}
	svc := NewAuthService(nil, nil, nil, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	result, err := svc.SendVerifyCode(context.Background(), dto.VerifyCodeRequest{
		Target:  "13800138000",
		Purpose: "login",
	})
	if err != nil {
		t.Fatalf("expected send success, got %v", err)
	}
	if result.ResendAfter != 60 {
		t.Fatalf("expected resend_after 60, got %d", result.ResendAfter)
	}
	if result.ExpireAt.Before(time.Now()) {
		t.Fatal("expected expire_at in the future")
	}
	if verifyCode.savedTarget != "13800138000" {
		t.Fatalf("expected saved target 13800138000, got %q", verifyCode.savedTarget)
	}
}

func TestAuthServiceSendVerifyCodeInvalidTarget(t *testing.T) {
	svc := NewAuthService(nil, nil, nil, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	_, err := svc.SendVerifyCode(context.Background(), dto.VerifyCodeRequest{
		Target:  "not-a-phone-or-email",
		Purpose: "login",
	})
	if !errors.Is(err, common.ErrInvalidRequest) {
		t.Fatalf("expected invalid request for bad target, got %v", err)
	}
}

func TestAuthServiceSendVerifyCodeToEmailSuccess(t *testing.T) {
	verifyCode := &fakeVerifyCodeStore{}
	svc := NewAuthService(nil, nil, nil, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	result, err := svc.SendVerifyCode(context.Background(), dto.VerifyCodeRequest{
		Target:  "alumni@sdu.edu.cn",
		Purpose: "login",
	})
	if err != nil {
		t.Fatalf("expected email send success, got %v", err)
	}
	if verifyCode.savedTarget != "alumni@sdu.edu.cn" {
		t.Fatalf("expected saved target alumni@sdu.edu.cn, got %q", verifyCode.savedTarget)
	}
	if result.ResendAfter != 60 {
		t.Fatalf("expected resend_after 60, got %d", result.ResendAfter)
	}
}

func TestEmailSenderMissingPasswordReturnsError(t *testing.T) {
	sender := &EmailSender{
		Host:     "smtp.example.com",
		Port:     25,
		Username: "sender@example.com",
	}

	err := sender.Send(context.Background(), "alumni@sdu.edu.cn", "123456")
	if err == nil {
		t.Fatal("expected missing password error")
	}
	if !strings.Contains(err.Error(), "email password not configured") {
		t.Fatalf("expected missing password error, got %v", err)
	}
}

func TestEmailSenderInvalidPortReturnsError(t *testing.T) {
	sender := &EmailSender{
		Host:     "smtp.example.com",
		Port:     0,
		Username: "sender@example.com",
		Password: "secret",
	}

	err := sender.Send(context.Background(), "alumni@sdu.edu.cn", "123456")
	if err == nil {
		t.Fatal("expected invalid port error")
	}
	if !strings.Contains(err.Error(), "email port out of range") {
		t.Fatalf("expected invalid port error, got %v", err)
	}
}

type failingCodeSender struct {
	err error
}

func (s *failingCodeSender) Send(context.Context, string, string) error {
	return s.err
}

func TestAuthServiceSendVerifyCodeReturnsSendError(t *testing.T) {
	verifyCode := &fakeVerifyCodeStore{}
	svc := NewAuthService(nil, nil, nil, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})
	svc.codeSender = &failingCodeSender{err: errors.New("smtp unavailable")}

	_, err := svc.SendVerifyCode(context.Background(), dto.VerifyCodeRequest{
		Target:  "alumni@sdu.edu.cn",
		Purpose: "login",
	})
	if err == nil {
		t.Fatal("expected send error")
	}
	if !strings.Contains(err.Error(), "send verify code") {
		t.Fatalf("expected send verify code error, got %v", err)
	}
}

func TestAuthServiceSendVerifyCodeRateLimited(t *testing.T) {
	verifyCode := &fakeVerifyCodeStore{
		lastSend: time.Now(),
	}
	svc := NewAuthService(nil, nil, nil, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	_, err := svc.SendVerifyCode(context.Background(), dto.VerifyCodeRequest{
		Target:  "13800138000",
		Purpose: "login",
	})
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
}

func TestAuthServiceSendVerifyCodeDailyLimit(t *testing.T) {
	verifyCode := &fakeVerifyCodeStore{
		sendCount: 10,
	}
	svc := NewAuthService(nil, nil, nil, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	_, err := svc.SendVerifyCode(context.Background(), dto.VerifyCodeRequest{
		Target:  "13800138000",
		Purpose: "login",
	})
	if err == nil {
		t.Fatal("expected daily limit error, got nil")
	}
}

// ============================================================
// Tests: UpdateContact
// ============================================================

func TestAuthServiceUpdateMobile(t *testing.T) {
	newMobile := "13800138001"
	alumniID := uint64(10)
	store := &contactUserStore{}
	store.fakeUserStore = fakeUserStore{
		user: &model.User{
			ID:       5,
			Account:  "13800138000",
			Role:     common.RoleAlumni,
			Status:   common.UserStatusActive,
			Mobile:   new("13800138000"),
			AlumniID: &alumniID,
		},
	}
	alumni := &fakeAlumniStoreForAuth{}
	verifyCode := &fakeVerifyCodeStore{verifyOk: true}

	svc := NewAuthService(store, alumni, &fakeLoginAttemptStore{}, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	err := svc.UpdateContact(context.Background(), 5, dto.UpdateContactRequest{
		Mobile: &newMobile,
		Code:   "123456",
	})
	if err != nil {
		t.Fatalf("expected update success, got %v", err)
	}
}

func TestAuthServiceUpdateEmail(t *testing.T) {
	newEmail := "new@example.com"
	alumniID := uint64(10)
	store := &contactUserStore{}
	store.fakeUserStore = fakeUserStore{
		user: &model.User{
			ID:       5,
			Account:  "user@example.com",
			Role:     common.RoleAlumni,
			Status:   common.UserStatusActive,
			Email:    new("old@example.com"),
			AlumniID: &alumniID,
		},
	}
	alumni := &fakeAlumniStoreForAuth{}
	verifyCode := &fakeVerifyCodeStore{verifyOk: true}

	svc := NewAuthService(store, alumni, &fakeLoginAttemptStore{}, verifyCode, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	err := svc.UpdateContact(context.Background(), 5, dto.UpdateContactRequest{
		Email: &newEmail,
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("expected update success, got %v", err)
	}
}

func TestAuthServiceUpdateContactNotAlumni(t *testing.T) {
	store := &fakeUserStore{
		user: &model.User{
			ID:     1,
			Role:   common.RoleAdmin,
			Status: common.UserStatusActive,
		},
	}

	svc := NewAuthService(store, nil, nil, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	newMobile := "13800138000"
	err := svc.UpdateContact(context.Background(), 1, dto.UpdateContactRequest{
		Mobile: &newMobile,
		Code:   "123456",
	})
	if !errors.Is(err, common.ErrPermissionDenied) {
		t.Fatalf("expected permission denied for admin, got %v", err)
	}
}

func TestAuthServiceUpdateContactNoFields(t *testing.T) {
	store := &fakeUserStore{
		user: &model.User{
			ID:     5,
			Role:   common.RoleAlumni,
			Status: common.UserStatusActive,
		},
	}

	svc := NewAuthService(store, nil, nil, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	err := svc.UpdateContact(context.Background(), 5, dto.UpdateContactRequest{Code: "123456"})
	if !errors.Is(err, common.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestAuthServiceUpdateContactDuplicateMobile(t *testing.T) {
	alumniID := uint64(10)
	store := &contactUserStoreDuplicate{}
	store.fakeUserStore = fakeUserStore{
		user: &model.User{
			ID:       5,
			Account:  "13800138000",
			Role:     common.RoleAlumni,
			Status:   common.UserStatusActive,
			Mobile:   new("13800138000"),
			AlumniID: &alumniID,
		},
	}

	svc := NewAuthService(store, nil, nil, nil, config.Config{
		App:  config.AppConfig{Name: "test-api"},
		Auth: config.AuthConfig{JWTSecret: "test-secret"},
	})

	dupMobile := "13900000000"
	err := svc.UpdateContact(context.Background(), 5, dto.UpdateContactRequest{
		Mobile: &dupMobile,
		Code:   "123456",
	})
	if !errors.Is(err, common.ErrAccountAlreadyExists) {
		t.Fatalf("expected account already exists for duplicate mobile, got %v", err)
	}
}

func TestDetectLoginType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"13800138000", "mobile"},
		{"13912345678", "mobile"},
		{"user@example.com", "email"},
		{"foo@bar.cn", "email"},
		{"admin", "account"},
		{"abc123", "account"},
		{"", "account"},
	}
	for _, tc := range tests {
		got := detectLoginType(tc.input)
		if got != tc.expected {
			t.Errorf("detectLoginType(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

//go:fix inline
func ptr[T any](v T) *T { return new(v) }
