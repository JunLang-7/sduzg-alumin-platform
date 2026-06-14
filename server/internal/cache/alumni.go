package cache

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	alumniCountKey      = "alumni:count"
	alumniExportVersion = "alumni:export:version"
	alumniExportKeyFmt  = "alumni:export:%d:%x"
	alumniExportTTL     = 2 * time.Hour
)

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

// ExportCache 导出结果缓存，版本号机制实现一键失效。
type ExportCache struct {
	client *redis.Client
}

func NewExportCache(client *redis.Client) *ExportCache {
	return &ExportCache{client: client}
}

// buildKey 根据当前版本和查询条件的 MD5 生成缓存 key。
func (c *ExportCache) buildKey(ctx context.Context, query any) (string, error) {
	version, err := c.client.Get(ctx, alumniExportVersion).Int64()
	if err != nil && err != redis.Nil {
		return "", err
	}
	data, err := json.Marshal(query)
	if err != nil {
		return "", fmt.Errorf("marshal query: %w", err)
	}
	hash := md5.Sum(data)
	return fmt.Sprintf(alumniExportKeyFmt, version, hash), nil
}

// Get 命中时返回序列化的校友列表 JSON 字节。
func (c *ExportCache) Get(ctx context.Context, query any) ([]byte, error) {
	if c.client == nil {
		return nil, redis.Nil
	}
	key, err := c.buildKey(ctx, query)
	if err != nil {
		return nil, err
	}
	return c.client.Get(ctx, key).Bytes()
}

// Set 缓存序列化的校友列表，TTL 20 分钟。
func (c *ExportCache) Set(ctx context.Context, query any, data []byte) error {
	if c.client == nil {
		return nil
	}
	key, err := c.buildKey(ctx, query)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, alumniExportTTL).Err()
}

// Invalidate 递增版本号，使所有旧缓存自动失效。
func (c *ExportCache) Invalidate(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	return c.client.Incr(ctx, alumniExportVersion).Err()
}
