package limiter

import (
	"context"
	"sync"
	"time"

	"github.com/juju/ratelimit"
)

type MemoryLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*memoryBucket
	bucketTTL       time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

type memoryBucket struct {
	bucket   *ratelimit.Bucket
	lastSeen time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		buckets:         make(map[string]*memoryBucket),
		bucketTTL:       30 * time.Minute,
		cleanupInterval: time.Minute,
	}
}

func (l *MemoryLimiter) Allow(_ context.Context, key string, rule Rule) (Result, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.cleanup(now)

	entry := l.buckets[key]
	if entry == nil {
		burst := effectiveBurst(rule)
		entry = &memoryBucket{
			bucket: ratelimit.NewBucketWithQuantum(rule.Period/time.Duration(max(rule.Limit, 1)), int64(burst), 1),
		}
		l.buckets[key] = entry
	}
	entry.lastSeen = now

	if entry.bucket.TakeAvailable(1) == 1 {
		return Result{
			Allowed:   true,
			Remaining: int(entry.bucket.Available()),
		}, nil
	}

	return Result{
		Allowed:    false,
		RetryAfter: rule.Period / time.Duration(max(rule.Limit, 1)),
	}, nil
}

func (l *MemoryLimiter) cleanup(now time.Time) {
	if l.cleanupInterval > 0 && !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < l.cleanupInterval {
		return
	}
	l.lastCleanup = now

	for key, entry := range l.buckets {
		if entry == nil || now.Sub(entry.lastSeen) > l.bucketTTL {
			delete(l.buckets, key)
		}
	}
}

func effectiveBurst(rule Rule) int {
	if rule.Burst > 0 {
		return rule.Burst
	}
	return max(rule.Limit, 1)
}
