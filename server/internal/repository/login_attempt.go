package repository

import (
	"context"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/redis/go-redis/v9"
)

const loginFailureKeyPrefix = "auth:login_failure:"

type LoginAttemptStore interface {
	FailureCount(ctx context.Context, account string) (int, error)
	RecordFailure(ctx context.Context, account string, window time.Duration) (int, error)
	ClearFailures(ctx context.Context, account string) error
}

type LoginAttemptRepository struct {
	redis *redis.Client
}

func NewLoginAttemptRepository(redisClient *redis.Client) *LoginAttemptRepository {
	return &LoginAttemptRepository{redis: redisClient}
}

// FailureCount 获取指定账号的登录失败次数
func (r *LoginAttemptRepository) FailureCount(ctx context.Context, account string) (int, error) {
	if r.redis == nil {
		return 0, common.ErrCacheUnavailable
	}

	count, err := r.redis.Get(ctx, loginFailureKey(account)).Int()
	if err == nil {
		return count, nil
	}
	if err == redis.Nil {
		return 0, nil
	}
	return 0, err
}

// RecordFailure 记录一次登录失败，并返回当前失败次数。如果达到锁定阈值，返回 true。
func (r *LoginAttemptRepository) RecordFailure(ctx context.Context, account string, window time.Duration) (int, error) {
	if r.redis == nil {
		return 0, common.ErrCacheUnavailable
	}

	key := loginFailureKey(account)
	count, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if err := r.redis.Expire(ctx, key, window).Err(); err != nil {
			return 0, err
		}
	}

	return int(count), nil
}

// ClearFailures 清除指定账号的登录失败记录
func (r *LoginAttemptRepository) ClearFailures(ctx context.Context, account string) error {
	if r.redis == nil {
		return common.ErrCacheUnavailable
	}

	return r.redis.Del(ctx, loginFailureKey(account)).Err()
}

func loginFailureKey(account string) string {
	return loginFailureKeyPrefix + strings.ToLower(strings.TrimSpace(account))
}
