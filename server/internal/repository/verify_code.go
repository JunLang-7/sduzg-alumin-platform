package repository

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/redis/go-redis/v9"
)

const (
	verifyCodeKeyPrefix    = "alumni:verify_code:"
	verificationCodeTTL    = 5 * time.Minute
	verifyCodeSendCountKey = "alumni:verify_code:send_count:"

	// verifyCodeLua 原子比较并删除验证码。
	// KEYS[1] = code key, ARGV[1] = input code.
	// 返回 1: 验证成功并已删除, 0: 验证码不匹配, -1: key 不存在(已过期).
	verifyCodeLua = `
local stored = redis.call("GET", KEYS[1])
if stored == false then
  return -1
end
if stored ~= ARGV[1] then
  return 0
end
return redis.call("DEL", KEYS[1])
`
)

// CodeSender 可插拔的验证码发送接口。
type CodeSender interface {
	Send(ctx context.Context, target, code string) error
}

// VerifyCodeStore 验证码 Redis 存储接口。
type VerifyCodeStore interface {
	// Save 存储验证码，TTL 5 分钟
	Save(ctx context.Context, target, code string) error
	// Verify 验证并消费验证码（一次性），返回是否成功
	Verify(ctx context.Context, target, code string) (bool, error)
	// IncrementSendCount 增加发送计数，返回当日累计发送次数
	IncrementSendCount(ctx context.Context, target string) (int64, error)
	// LastSendTime 获取同一目标上一次发送验证码的时间
	LastSendTime(ctx context.Context, target string) (time.Time, error)
}

type verifyCodeStore struct {
	redis *redis.Client
}

func NewVerifyCodeStore(redisClient *redis.Client) *verifyCodeStore {
	return &verifyCodeStore{redis: redisClient}
}

func normalizeTarget(target string) string {
	return strings.ToLower(strings.TrimSpace(target))
}

func codeKey(target string) string {
	return verifyCodeKeyPrefix + normalizeTarget(target)
}

func sendCountKey(target string) string {
	return verifyCodeSendCountKey + normalizeTarget(target)
}

// GenerateRandomCode 生成 6 位随机数字验证码。
func GenerateRandomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", fmt.Errorf("generate random code: %w", err)
	}
	code := n.Int64() + 100000
	return fmt.Sprintf("%06d", code), nil
}

func (s *verifyCodeStore) Save(ctx context.Context, target, code string) error {
	if s.redis == nil {
		return common.ErrCacheUnavailable
	}

	return s.redis.Set(ctx, codeKey(target), code, verificationCodeTTL).Err()
}

func (s *verifyCodeStore) Verify(ctx context.Context, target, code string) (bool, error) {
	if s.redis == nil {
		return false, common.ErrCacheUnavailable
	}

	key := codeKey(target)
	result, err := s.redis.Eval(ctx, verifyCodeLua, []string{key}, code).Int()
	if err != nil {
		return false, err
	}

	switch result {
	case 1:
		return true, nil
	case 0:
		return false, common.ErrCodeInvalid
	default: // -1: key not found
		return false, common.ErrCodeExpired
	}
}

func (s *verifyCodeStore) IncrementSendCount(ctx context.Context, target string) (int64, error) {
	if s.redis == nil {
		return 0, common.ErrCacheUnavailable
	}

	key := sendCountKey(target)
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	// 首次设置 TTL 到当天 23:59:59
	if count == 1 {
		now := time.Now()
		endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		ttl := endOfDay.Sub(now)
		if err := s.redis.Expire(ctx, key, ttl).Err(); err != nil {
			return 0, err
		}
	}

	return count, nil
}

// LastSendTime 获取同一目标上一次发送验证码的时间，返回时间戳和剩余等待秒数。
func (s *verifyCodeStore) LastSendTime(ctx context.Context, target string) (time.Time, error) {
	if s.redis == nil {
		return time.Time{}, common.ErrCacheUnavailable
	}

	// 使用 Redis key 前缀匹配找到该 target 相关的验证码 key
	key := codeKey(target)
	ttl, err := s.redis.TTL(ctx, key).Result()
	if err != nil {
		return time.Time{}, err
	}
	if ttl <= 0 {
		return time.Time{}, redis.Nil
	}

	// 获取上一次设置的时间（通过 key 的创建时间）
	_, err = s.redis.Get(ctx, key).Result()
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-ttl), nil
}
