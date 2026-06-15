package limiter

import (
	"context"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	limiter *redis_rate.Limiter
}

func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	if client == nil {
		return nil
	}
	return &RedisLimiter{limiter: redis_rate.NewLimiter(client)}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, rule Rule) (Result, error) {
	result, err := l.limiter.Allow(ctx, key, redis_rate.Limit{
		Rate:   rule.Limit,
		Burst:  effectiveBurst(rule),
		Period: rule.Period,
	})
	if err != nil {
		return Result{}, err
	}

	return Result{
		Allowed:    result.Allowed > 0,
		Remaining:  result.Remaining,
		RetryAfter: result.RetryAfter,
	}, nil
}
