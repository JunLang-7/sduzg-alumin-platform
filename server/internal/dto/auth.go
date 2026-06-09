package dto

import "time"

type LoginRequest struct {
	Account   string `json:"account"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Code      string `json:"code"`
	GrantType string `json:"grant_type"` // "password" | "sms_code" | "email_code"
}

// LoginIdentifier returns the primary login identifier.
// Returns phone if set, email if set, otherwise account.
func (r LoginRequest) LoginIdentifier() string {
	if r.Mobile != "" {
		return r.Mobile
	}
	if r.Email != "" {
		return r.Email
	}
	return r.Account
}

type LoginResult struct {
	AccessToken       string    `json:"access_token"`
	TokenType         string    `json:"token_type"`
	ExpiresAt         time.Time `json:"expires_at"`
	User              UserDTO   `json:"user"`
	RegistrationToken string    `json:"registration_token,omitempty"`
}

type LogoutResult struct {
	LoggedOut bool `json:"logged_out"`
}

type UserDTO struct {
	ID       uint64  `json:"id"`
	Account  string  `json:"account"`
	Role     string  `json:"role"`
	RealName *string `json:"real_name,omitempty"`
	AlumniID *uint64 `json:"alumni_id,omitempty"`
	Mobile   *string `json:"mobile,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword     string `json:"old_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

type SetupPasswordRequest struct {
	RegistrationToken string `json:"registration_token" binding:"required"`
	NewPassword       string `json:"new_password" binding:"required,min=8"`
	ConfirmPassword   string `json:"confirm_password" binding:"required"`
}

type SetupPasswordResult struct {
	AccessToken string  `json:"access_token"`
	TokenType   string  `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	User        UserDTO `json:"user"`
}

type VerifyCodeRequest struct {
	Target  string `json:"target" binding:"required"`
	Purpose string `json:"purpose" binding:"required,oneof=login"`
}

type VerifyCodeResult struct {
	ExpireAt    time.Time `json:"expire_at"`
	ResendAfter int       `json:"resend_after"` // seconds
}
