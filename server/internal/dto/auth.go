package dto

import "time"

type LoginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResult struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	User        UserDTO   `json:"user"`
}

type UserDTO struct {
	ID       uint64  `json:"id"`
	Account  string  `json:"account"`
	Role     string  `json:"role"`
	RealName *string `json:"real_name,omitempty"`
}
