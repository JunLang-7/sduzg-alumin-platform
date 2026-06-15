package limiter

import "github.com/redis/go-redis/v9"

func New(redisClient *redis.Client) Limiter {
	memory := NewMemoryLimiter()
	if redisClient != nil {
		return NewFallbackLimiter(NewRedisLimiter(redisClient), memory)
	}
	return memory
}
