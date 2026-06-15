package limiter

import (
	"context"
	"testing"
	"time"
)

func TestMemoryLimiterAllowsUntilLimit(t *testing.T) {
	limiter := NewMemoryLimiter()
	rule := Rule{
		Name:   "global",
		Limit:  2,
		Burst:  2,
		Period: time.Minute,
	}

	for i := range 2 {
		result, err := limiter.Allow(context.Background(), "test-key", rule)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !result.Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	result, err := limiter.Allow(context.Background(), "test-key", rule)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Allowed {
		t.Fatal("expected request over limit to be rejected")
	}
	if result.RetryAfter <= 0 {
		t.Fatalf("expected retry after to be positive, got %s", result.RetryAfter)
	}
}

func TestMemoryLimiterCleansUpStaleBuckets(t *testing.T) {
	limiter := NewMemoryLimiter()
	limiter.cleanupInterval = 0
	limiter.bucketTTL = time.Minute

	limiter.buckets["stale-key"] = &memoryBucket{lastSeen: time.Now().Add(-2 * time.Minute)}
	limiter.buckets["fresh-key"] = &memoryBucket{lastSeen: time.Now()}

	_, err := limiter.Allow(context.Background(), "new-key", Rule{
		Name:   "global",
		Limit:  1,
		Burst:  1,
		Period: time.Minute,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if _, ok := limiter.buckets["stale-key"]; ok {
		t.Fatal("expected stale bucket to be cleaned up")
	}
	if _, ok := limiter.buckets["fresh-key"]; !ok {
		t.Fatal("expected fresh bucket to be kept")
	}
}
