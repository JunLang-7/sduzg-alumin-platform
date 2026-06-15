package limiter

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubLimiter struct {
	result limiterResult
	err    error
	calls  int
}

type limiterResult = Result

func (s *stubLimiter) Allow(_ context.Context, _ string, _ Rule) (Result, error) {
	s.calls++
	return s.result, s.err
}

func TestFallbackLimiterUsesMemoryWhenPrimaryErrors(t *testing.T) {
	primary := &stubLimiter{err: errors.New("redis unavailable")}
	fallback := &stubLimiter{result: Result{Allowed: true, Remaining: 1}}
	limiter := NewFallbackLimiter(primary, fallback)

	result, err := limiter.Allow(context.Background(), "key", Rule{
		Name:   "global",
		Limit:  1,
		Burst:  1,
		Period: time.Minute,
	})
	if err != nil {
		t.Fatalf("expected fallback error to be nil, got %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected fallback to allow request")
	}
	if primary.calls != 1 || fallback.calls != 1 {
		t.Fatalf("expected both primary and fallback to be called, got primary=%d fallback=%d", primary.calls, fallback.calls)
	}
}

func TestFallbackLimiterReturnsPrimaryResultWhenAvailable(t *testing.T) {
	primary := &stubLimiter{result: Result{Allowed: false}}
	fallback := &stubLimiter{result: Result{Allowed: true}}
	limiter := NewFallbackLimiter(primary, fallback)

	result, err := limiter.Allow(context.Background(), "key", Rule{
		Name:   "global",
		Limit:  1,
		Burst:  1,
		Period: time.Minute,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Allowed {
		t.Fatal("expected primary rejection to be preserved")
	}
	if fallback.calls != 0 {
		t.Fatalf("expected fallback not to be called, got %d calls", fallback.calls)
	}
}
