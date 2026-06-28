package service

import (
	"context"
	"errors"
	"fmt"
	stdmail "net/mail"
	"regexp"
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
	tccommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
	mail "github.com/wneessen/go-mail"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxLoginFailures   = 5
	loginFailureWindow = 5 * time.Minute

	codeSendInterval = 60 * time.Second
	codeDailyMax     = 10
	codeTTL          = 5 * time.Minute
	emailSendTimeout = 15 * time.Second
)

var (
	phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)
	emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

// CodeSender is a pluggable verification code sender.
type CodeSender interface {
	Send(ctx context.Context, target, code string) error
}

// SMSSender sends codes via SMS provider.
type SMSSender struct {
	SecretID   string
	SecretKey  string
	Region     string
	AppID      string
	SignName   string
	TemplateID string
	Endpoint   string

	newClient func() (tencentSMSClient, error)
}

type tencentSMSClient interface {
	SendSmsWithContext(ctx context.Context, request *sms.SendSmsRequest) (*sms.SendSmsResponse, error)
}

func (s *SMSSender) Send(ctx context.Context, target, code string) error {
	if err := s.validate(); err != nil {
		logger.Warn("SMS not configured", zap.String("target", target), zap.Error(err))
		return err
	}
	client, err := s.client()
	if err != nil {
		return fmt.Errorf("create tencent sms client: %w", err)
	}
	request := sms.NewSendSmsRequest()
	request.PhoneNumberSet = tccommon.StringPtrs([]string{normalizeChineseMainlandPhone(target)})
	request.SmsSdkAppId = tccommon.StringPtr(s.AppID)
	request.SignName = tccommon.StringPtr(s.SignName)
	request.TemplateId = tccommon.StringPtr(s.TemplateID)
	request.TemplateParamSet = tccommon.StringPtrs([]string{code})

	if ctx == nil {
		ctx = context.Background()
	}
	resp, err := client.SendSmsWithContext(ctx, request)
	if err != nil {
		return fmt.Errorf("send tencent sms: %w", err)
	}
	if resp == nil || resp.Response == nil || len(resp.Response.SendStatusSet) == 0 {
		return errors.New("send tencent sms: empty response")
	}
	status := resp.Response.SendStatusSet[0]
	if status.Code == nil || *status.Code != "Ok" {
		var message, codeValue string
		if status.Message != nil {
			message = *status.Message
		}
		if status.Code != nil {
			codeValue = *status.Code
		}
		return fmt.Errorf("send tencent sms failed: %s %s", codeValue, message)
	}
	return nil
}

func (s *SMSSender) validate() error {
	switch {
	case strings.TrimSpace(s.SecretID) == "":
		return errors.New("sms tencent secret id not configured")
	case strings.TrimSpace(s.SecretKey) == "":
		return errors.New("sms tencent secret key not configured")
	case strings.TrimSpace(s.Region) == "":
		return errors.New("sms tencent region not configured")
	case strings.TrimSpace(s.AppID) == "":
		return errors.New("sms tencent app id not configured")
	case strings.TrimSpace(s.SignName) == "":
		return errors.New("sms tencent sign name not configured")
	case strings.TrimSpace(s.TemplateID) == "":
		return errors.New("sms tencent template id not configured")
	default:
		return nil
	}
}

func (s *SMSSender) client() (tencentSMSClient, error) {
	if s.newClient != nil {
		return s.newClient()
	}
	credential := tccommon.NewCredential(s.SecretID, s.SecretKey)
	clientProfile := profile.NewClientProfile()
	if strings.TrimSpace(s.Endpoint) != "" {
		clientProfile.HttpProfile.Endpoint = strings.TrimSpace(s.Endpoint)
	}
	return sms.NewClient(credential, s.Region, clientProfile)
}

func normalizeChineseMainlandPhone(target string) string {
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "00")
	if strings.HasPrefix(target, "+") {
		return target
	}
	if strings.HasPrefix(target, "86") {
		return "+" + target
	}
	return "+86" + target
}

// EmailSender sends codes via SMTP.
type EmailSender struct {
	Host     string
	Port     int
	Username string
	Password string
	FromName string
}

func (s *EmailSender) Send(ctx context.Context, target, code string) error {
	if err := s.validate(); err != nil {
		logger.Warn("Email not configured", zap.String("target", target), zap.Error(err))
		return err
	}
	fromName := sanitizeMailHeader(s.FromName)
	if fromName == "" {
		fromName = s.Username
	}
	msg := mail.NewMsg()
	from := (&stdmail.Address{Name: fromName, Address: s.Username}).String()
	if err := msg.From(from); err != nil {
		return fmt.Errorf("set email sender: %w", err)
	}
	if err := msg.To(sanitizeMailHeader(target)); err != nil {
		return fmt.Errorf("set email recipient: %w", err)
	}
	msg.Subject("山东大学政治学与公共管理学院校友平台邮箱登录验证码")
	msg.SetBodyString(mail.TypeTextPlain, buildEmailCodeBody(target, code))

	options := []mail.Option{
		mail.WithPort(s.Port),
		mail.WithTimeout(emailSendTimeout),
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		mail.WithUsername(s.Username),
		mail.WithPassword(s.Password),
	}
	if s.Port == 465 {
		options = append(options, mail.WithSSL())
	} else {
		options = append(options, mail.WithTLSPolicy(mail.TLSMandatory))
	}
	client, err := mail.NewClient(s.Host, options...)
	if err != nil {
		return fmt.Errorf("create email client: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sendCtx, cancel := context.WithTimeout(ctx, emailSendTimeout)
	defer cancel()
	if err := client.DialAndSendWithContext(sendCtx, msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func (s *EmailSender) validate() error {
	switch {
	case strings.TrimSpace(s.Host) == "":
		return errors.New("email host not configured")
	case strings.TrimSpace(s.Username) == "":
		return errors.New("email username not configured")
	case strings.TrimSpace(s.Password) == "":
		return errors.New("email password not configured")
	case s.Port < 1 || s.Port > 65535:
		return fmt.Errorf("email port out of range: %d", s.Port)
	default:
		return nil
	}
}

func buildEmailCodeBody(target, code string) string {
	return fmt.Sprintf(`亲爱的 %s 先生/女士，

您好！感谢您登录山东大学政治学与公共管理学院校友平台。请查收您的邮箱验证码：%s。该验证码5分钟内有效。

祝好，
山东大学政治学与公共管理学院

注意：这是自动发送邮件，请勿直接回复。`, sanitizeMailHeader(target), code)
}

func sanitizeMailHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\x00", "")
	return strings.TrimSpace(value)
}

// mockSender logs the code when services are not configured.
type mockSender struct{}

func (m *mockSender) Send(_ context.Context, target, code string) error {
	logger.Info("Mock sender", zap.String("target", target), zap.String("code", code))
	return nil
}

type routedCodeSender struct {
	sms      CodeSender
	email    CodeSender
	fallback CodeSender
}

func (s *routedCodeSender) Send(ctx context.Context, target, code string) error {
	switch {
	case phoneRegex.MatchString(target):
		if s.sms != nil {
			return s.sms.Send(ctx, target, code)
		}
		if s.fallback != nil {
			return s.fallback.Send(ctx, target, code)
		}
		return errors.New("sms sender not configured")
	case emailRegex.MatchString(target):
		if s.email != nil {
			return s.email.Send(ctx, target, code)
		}
		if s.fallback != nil {
			return s.fallback.Send(ctx, target, code)
		}
		return errors.New("email sender not configured")
	default:
		return common.ErrInvalidRequest
	}
}

func (s *routedCodeSender) hasSender(isPhone, isEmail bool) bool {
	switch {
	case isPhone:
		return s.sms != nil
	case isEmail:
		return s.email != nil
	default:
		return false
	}
}

// resolveCodeSender returns the appropriate CodeSender based on config.
func resolveCodeSender(cfg config.Config) CodeSender {
	var smsSender CodeSender
	var emailSender CodeSender
	fallback := &mockSender{}
	if cfg.Email.Enabled {
		sender := &EmailSender{Host: cfg.Email.Host, Port: cfg.Email.Port, Username: cfg.Email.Username, Password: cfg.Email.Password, FromName: cfg.Email.FromName}
		if err := sender.validate(); err != nil {
			logger.Warn("Email sender disabled due to incomplete config", zap.Error(err))
		} else {
			emailSender = sender
		}
	}
	if cfg.SMS.Enabled {
		sender := &SMSSender{
			SecretID:   cfg.SMS.SecretID,
			SecretKey:  cfg.SMS.SecretKey,
			Region:     cfg.SMS.Region,
			AppID:      cfg.SMS.AppID,
			SignName:   cfg.SMS.SignName,
			TemplateID: cfg.SMS.TemplateID,
			Endpoint:   cfg.SMS.Endpoint,
		}
		if err := sender.validate(); err != nil {
			logger.Warn("SMS sender disabled due to incomplete config", zap.Error(err))
		} else {
			smsSender = sender
		}
	}
	return &routedCodeSender{sms: smsSender, email: emailSender, fallback: fallback}
}

// AuthService handles authentication.
type AuthService struct {
	users         repository.UserStore
	alumni        repository.AlumniStore
	loginAttempts repository.LoginAttemptStore
	verifyCode    repository.VerifyCodeStore
	codeSender    CodeSender
	cfg           config.Config
	jwtSecret     []byte
	accessTTL     time.Duration
	issuer        string
	now           func() time.Time
}

func NewAuthService(
	users repository.UserStore,
	alumni repository.AlumniStore,
	loginAttempts repository.LoginAttemptStore,
	verifyCode repository.VerifyCodeStore,
	cfg config.Config,
) *AuthService {
	return &AuthService{
		users:         users,
		alumni:        alumni,
		loginAttempts: loginAttempts,
		verifyCode:    verifyCode,
		codeSender:    resolveCodeSender(cfg),
		cfg:           cfg,
		jwtSecret:     []byte(cfg.Auth.JWTSecret),
		accessTTL:     cfg.Auth.AccessTokenTTL,
		issuer:        cfg.App.Name,
		now:           time.Now,
	}
}

// detectLoginType classifies an identifier string.
func detectLoginType(id string) string {
	if phoneRegex.MatchString(id) {
		return "mobile"
	}
	if emailRegex.MatchString(id) {
		return "email"
	}
	return "account"
}

// Login dispatches to password or code-based login.
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResult, error) {
	switch req.GrantType {
	case "sms_code":
		return s.loginWithSMSCode(ctx, req)
	case "email_code":
		return s.loginWithEmailCode(ctx, req)
	default:
		return s.loginWithPassword(ctx, req)
	}
}

func (s *AuthService) loginWithPassword(ctx context.Context, req dto.LoginRequest) (*dto.LoginResult, error) {
	identifier := strings.TrimSpace(req.LoginIdentifier())
	if identifier == "" || req.Password == "" {
		return nil, common.ErrInvalidCredentials
	}
	if s.users == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	if s.isLoginLocked(ctx, identifier) {
		return nil, common.ErrAccountLocked
	}

	typ := detectLoginType(identifier)
	user, err := s.findUser(ctx, typ, identifier)
	if err != nil {
		if errors.Is(err, common.ErrUserNotFound) {
			s.recordLoginFailure(ctx, identifier)
			return nil, common.ErrInvalidCredentials
		}
		return nil, err
	}

	if user.Status != common.UserStatusActive {
		return nil, common.ErrAccountDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		if s.recordLoginFailure(ctx, identifier) {
			return nil, common.ErrAccountLocked
		}
		return nil, common.ErrInvalidCredentials
	}

	return s.completeLogin(ctx, user, identifier)
}

func (s *AuthService) findUser(ctx context.Context, typ, identifier string) (*model.User, error) {
	var user *model.User
	var err error
	switch typ {
	case "mobile":
		user, err = s.users.FindByMobile(ctx, identifier)
	case "email":
		user, err = s.users.FindByEmail(ctx, identifier)
	default:
		user, err = s.users.FindByAccount(ctx, identifier)
		if err == nil && user != nil && user.Role != common.RoleAdmin && user.Role != common.RoleSuperAdmin {
			return nil, common.ErrUserNotFound
		}
	}
	return user, err
}

func (s *AuthService) loginWithSMSCode(ctx context.Context, req dto.LoginRequest) (*dto.LoginResult, error) {
	phone := strings.TrimSpace(req.Mobile)
	code := strings.TrimSpace(req.Code)
	if phone == "" || code == "" || !phoneRegex.MatchString(phone) {
		return nil, common.ErrInvalidCredentials
	}
	if s.isLoginLocked(ctx, phone) {
		return nil, common.ErrAccountLocked
	}

	ok, err := s.verifyCode.Verify(ctx, phone, code)
	if err != nil {
		if errors.Is(err, common.ErrCodeInvalid) {
			s.recordLoginFailure(ctx, phone)
			return nil, common.ErrCodeInvalid
		}
		return nil, common.ErrCodeExpired
	}
	if !ok {
		s.recordLoginFailure(ctx, phone)
		return nil, common.ErrCodeInvalid
	}

	profile, err := s.alumni.FindByMobile(ctx, phone)
	if errors.Is(err, common.ErrAlumniNotFound) {
		return nil, common.ErrAlumniNotMatch
	}
	if err != nil {
		return nil, err
	}

	user, err := s.users.FindByMobile(ctx, phone)
	if err == nil {
		return s.completeLogin(ctx, user, phone)
	}
	if !errors.Is(err, common.ErrUserNotFound) {
		return nil, err
	}

	// No user yet — require password setup before creating the account.
	regToken, err := s.issueRegistrationToken(phone, "", profile.ID)
	if err != nil {
		return nil, err
	}
	return &dto.LoginResult{
		RegistrationToken: regToken,
	}, nil
}

func (s *AuthService) loginWithEmailCode(ctx context.Context, req dto.LoginRequest) (*dto.LoginResult, error) {
	email := strings.TrimSpace(req.Email)
	code := strings.TrimSpace(req.Code)
	if email == "" || code == "" || !emailRegex.MatchString(email) {
		return nil, common.ErrInvalidCredentials
	}
	if s.isLoginLocked(ctx, email) {
		return nil, common.ErrAccountLocked
	}

	ok, err := s.verifyCode.Verify(ctx, email, code)
	if err != nil {
		if errors.Is(err, common.ErrCodeInvalid) {
			s.recordLoginFailure(ctx, email)
			return nil, common.ErrCodeInvalid
		}
		return nil, common.ErrCodeExpired
	}
	if !ok {
		s.recordLoginFailure(ctx, email)
		return nil, common.ErrCodeInvalid
	}

	profile, err := s.alumni.FindByEmail(ctx, email)
	if errors.Is(err, common.ErrAlumniNotFound) {
		return nil, common.ErrAlumniNotMatch
	}
	if err != nil {
		return nil, err
	}

	lowerEmail := strings.ToLower(email)
	user, err := s.users.FindByEmail(ctx, lowerEmail)
	if err == nil {
		return s.completeLogin(ctx, user, email)
	}
	if !errors.Is(err, common.ErrUserNotFound) {
		return nil, err
	}

	// No user yet — require password setup before creating the account.
	regToken, err := s.issueRegistrationToken("", lowerEmail, profile.ID)
	if err != nil {
		return nil, err
	}
	return &dto.LoginResult{
		RegistrationToken: regToken,
	}, nil
}

func (s *AuthService) createUserForAlumni(ctx context.Context, mobile, email string, alumniID uint64, passwordHash string) (*model.User, error) {
	account := mobile
	if account == "" && email != "" {
		account = email
	}

	var realName *string
	if s.alumni != nil {
		if profile, err := s.alumni.GetByID(ctx, alumniID); err == nil {
			realName = &profile.Name
		}
	}

	newUser := &model.User{
		Account:      account,
		PasswordHash: passwordHash,
		Role:         common.RoleAlumni,
		AlumniID:     &alumniID,
		RealName:     realName,
		Status:       common.UserStatusActive,
	}
	if mobile != "" {
		newUser.Mobile = &mobile
	}
	if email != "" {
		newUser.Email = &email
	}
	if err := s.users.CreateUser(ctx, newUser); err != nil {
		return nil, err
	}
	// Re-read to get generated ID
	if mobile != "" {
		return s.users.FindByMobile(ctx, mobile)
	}
	return s.users.FindByEmail(ctx, email)
}

func (s *AuthService) completeLogin(ctx context.Context, user *model.User, identifier string) (*dto.LoginResult, error) {
	issuedAt := s.now()
	expiresAt := issuedAt.Add(s.accessTTL)

	token, err := s.issueAccessToken(user, issuedAt, expiresAt)
	if err != nil {
		return nil, err
	}

	if err := s.users.UpdateLastLoginAt(ctx, user.ID, issuedAt); err != nil {
		return nil, err
	}

	s.clearLoginFailures(ctx, identifier)

	return &dto.LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User: dto.UserDTO{
			ID:       user.ID,
			Account:  user.Account,
			Role:     user.Role,
			RealName: user.RealName,
			AlumniID: user.AlumniID,
			Mobile:   user.Mobile,
		},
	}, nil
}

// SendVerifyCode sends a verification code.
// When neither SMS nor email is configured, uses fixed code "888888".
func (s *AuthService) SendVerifyCode(ctx context.Context, req dto.VerifyCodeRequest) (*dto.VerifyCodeResult, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, common.ErrInvalidRequest
	}
	isPhone := phoneRegex.MatchString(target)
	isEmail := emailRegex.MatchString(target)
	if !isPhone && !isEmail {
		return nil, common.ErrInvalidRequest
	}

	if s.verifyCode != nil {
		last, err := s.verifyCode.LastSendTime(ctx, target)
		if err == nil && time.Since(last) < codeSendInterval {
			remaining := max(int(codeSendInterval-time.Since(last)), 1)
			return nil, fmt.Errorf("请勿频繁发送，%d 秒后可重新获取", remaining)
		}
	}

	if s.verifyCode != nil {
		cnt, _ := s.verifyCode.IncrementSendCount(ctx, target)
		if cnt > codeDailyMax {
			return nil, fmt.Errorf("今日发送次数已达上限")
		}
	}

	code := generateCode(s.cfg, s.codeSender, isPhone, isEmail)
	if s.verifyCode != nil {
		if err := s.verifyCode.Save(ctx, target, code); err != nil {
			return nil, err
		}
	}

	if err := s.codeSender.Send(ctx, target, code); err != nil {
		logger.Warn("failed to send code", zap.String("target", target), zap.Error(err))
		return nil, fmt.Errorf("验证码发送失败，请稍后重试")
	}

	return &dto.VerifyCodeResult{
		ExpireAt:    time.Now().Add(codeTTL),
		ResendAfter: 60,
	}, nil
}

// generateCode returns "888888" when the target sender channel is not enabled,
// otherwise a random 6-digit code.
func generateCode(cfg config.Config, sender CodeSender, isPhone, isEmail bool) string {
	if routed, ok := sender.(*routedCodeSender); ok && !routed.hasSender(isPhone, isEmail) {
		return "888888"
	}
	if isPhone && !cfg.SMS.Enabled {
		return "888888"
	}
	if isEmail && !cfg.Email.Enabled {
		return "888888"
	}
	code, err := repository.GenerateRandomCode()
	if err != nil {
		logger.Error("failed to generate random code", zap.Error(err))
		return "888888"
	}
	return code
}

// UpdateContact updates the alumni's phone and/or email with code verification.
func (s *AuthService) UpdateContact(ctx context.Context, userID uint64, req dto.UpdateContactRequest) error {
	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, common.ErrUserNotFound) {
		return common.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if user.Role != common.RoleAlumni {
		return common.ErrPermissionDenied
	}
	if req.Mobile == nil && req.Email == nil {
		return common.ErrInvalidRequest
	}

	currentMobile := strOrEmpty(user.Mobile)
	currentEmail := strOrEmpty(user.Email)

	if req.Mobile != nil {
		newVal := strings.TrimSpace(*req.Mobile)
		if !phoneRegex.MatchString(newVal) {
			return common.ErrInvalidRequest
		}
		if newVal != currentMobile {
			if _, ferr := s.users.FindByMobile(ctx, newVal); ferr == nil {
				return common.ErrAccountAlreadyExists
			}
		}
		verifyTarget := currentMobile
		if verifyTarget == "" {
			verifyTarget = newVal
		}
		if ok, _ := s.verifyCode.Verify(ctx, verifyTarget, req.Code); !ok {
			return common.ErrCodeInvalid
		}
		if err := s.users.UpdateMobile(ctx, userID, newVal); err != nil {
			return err
		}
		if user.AlumniID != nil && s.alumni != nil {
			_ = s.alumni.UpdateMobile(ctx, *user.AlumniID, newVal)
		}
		if currentMobile != "" {
			s.clearLoginFailures(ctx, currentMobile)
		}
	}

	if req.Email != nil {
		newVal := strings.ToLower(strings.TrimSpace(*req.Email))
		if !emailRegex.MatchString(newVal) {
			return common.ErrInvalidRequest
		}
		if newVal != strings.ToLower(currentEmail) {
			if _, ferr := s.users.FindByEmail(ctx, newVal); ferr == nil {
				return common.ErrAccountAlreadyExists
			}
		}
		verifyTarget := currentEmail
		if verifyTarget == "" {
			verifyTarget = newVal
		}
		if ok, _ := s.verifyCode.Verify(ctx, verifyTarget, req.Code); !ok {
			return common.ErrCodeInvalid
		}
		if err := s.users.UpdateEmail(ctx, userID, newVal); err != nil {
			return err
		}
		if user.AlumniID != nil && s.alumni != nil {
			_ = s.alumni.UpdateEmail(ctx, *user.AlumniID, newVal)
		}
		if currentEmail != "" {
			s.clearLoginFailures(ctx, strings.ToLower(currentEmail))
		}
	}

	return nil
}

// verifyCodeThenIgnore
// Note: This method needs VerifyCodeStore interface addition — let's just inline.
// We'll call s.verifyCode.Verify directly instead.
// <placeholder resolved below>

func (s *AuthService) isLoginLocked(ctx context.Context, identifier string) bool {
	if s.loginAttempts == nil {
		return false
	}
	cnt, _ := s.loginAttempts.FailureCount(ctx, identifier)
	return cnt >= maxLoginFailures
}

func (s *AuthService) recordLoginFailure(ctx context.Context, identifier string) bool {
	if s.loginAttempts == nil {
		return false
	}
	cnt, _ := s.loginAttempts.RecordFailure(ctx, identifier, loginFailureWindow)
	return cnt >= maxLoginFailures
}

func (s *AuthService) clearLoginFailures(ctx context.Context, identifier string) {
	if s.loginAttempts != nil {
		_ = s.loginAttempts.ClearFailures(ctx, identifier)
	}
}

func (s *AuthService) Logout(context.Context) (*dto.LogoutResult, error) {
	return &dto.LogoutResult{LoggedOut: true}, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uint64, oldPassword, newPassword string) error {
	if s.users == nil {
		return common.ErrDatabaseUnavailable
	}
	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, common.ErrUserNotFound) {
		return common.ErrInvalidCredentials
	}
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return common.ErrInvalidCredentials
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.UpdatePasswordHash(ctx, userID, string(hashed))
}

// SetupPassword validates the registration token, creates the user with the
// given password, and returns a full login result.
func (s *AuthService) SetupPassword(ctx context.Context, req dto.SetupPasswordRequest) (*dto.SetupPasswordResult, error) {
	if s.users == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	reg, err := s.parseRegistrationToken(req.RegistrationToken)
	if err != nil {
		return nil, errors.New("invalid or expired registration token")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user, err := s.createUserForAlumni(ctx, reg.Mobile, reg.Email, reg.AlumniID, string(hashed))
	if err != nil {
		return nil, err
	}

	issuedAt := s.now()
	expiresAt := issuedAt.Add(s.accessTTL)
	token, err := s.issueAccessToken(user, issuedAt, expiresAt)
	if err != nil {
		return nil, err
	}

	return &dto.SetupPasswordResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User: dto.UserDTO{
			ID:       user.ID,
			Account:  user.Account,
			Role:     user.Role,
			RealName: user.RealName,
			AlumniID: user.AlumniID,
			Mobile:   user.Mobile,
		},
	}, nil
}

type customClaims struct {
	jwt.RegisteredClaims
	UID     uint64 `json:"uid"`
	Account string `json:"account"`
	Role    string `json:"role"`
}

type registrationClaims struct {
	jwt.RegisteredClaims
	Mobile   string `json:"mobile"`
	Email    string `json:"email"`
	AlumniID uint64 `json:"alumni_id"`
}

func (s *AuthService) issueRegistrationToken(mobile, email string, alumniID uint64) (string, error) {
	now := s.now()
	claims := registrationClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
		Mobile:   mobile,
		Email:    email,
		AlumniID: alumniID,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *AuthService) parseRegistrationToken(tokenString string) (*registrationClaims, error) {
	claims := &registrationClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (s *AuthService) issueAccessToken(user *model.User, issuedAt, expiresAt time.Time) (string, error) {
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
		return "", err
	}
	return token, nil
}

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

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
