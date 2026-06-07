package cache

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const alumniCountKey = "alumni:count"

type CountCache struct {
	client *redis.Client
}

func NewCountCache(client *redis.Client) *CountCache {
	return &CountCache{client: client}
}

// Get returns the cached count. hit is false when the key doesn't exist or Redis is unavailable.
func (c *CountCache) Get(ctx context.Context) (count int64, hit bool, err error) {
	if c.client == nil {
		return 0, false, nil
	}
	val, err := c.client.Get(ctx, alumniCountKey).Result()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false, nil
	}
	return n, true, nil
}

// Set stores the count without expiration (persisted).
func (c *CountCache) Set(ctx context.Context, count int64) error {
	if c.client == nil {
		return nil
	}
	return c.client.Set(ctx, alumniCountKey, count, 0).Err()
}

// IncrBy atomically adjusts the counter by delta.
func (c *CountCache) IncrBy(ctx context.Context, delta int64) error {
	if c.client == nil {
		return nil
	}
	return c.client.IncrBy(ctx, alumniCountKey, delta).Err()
}
