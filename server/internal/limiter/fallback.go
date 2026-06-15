package limiter

import "context"

type FallbackLimiter struct {
	primary  Limiter
	fallback Limiter
}

func NewFallbackLimiter(primary Limiter, fallback Limiter) *FallbackLimiter {
	return &FallbackLimiter{
		primary:  primary,
		fallback: fallback,
	}
}

func (l *FallbackLimiter) Allow(ctx context.Context, key string, rule Rule) (Result, error) {
	if l.primary == nil {
		return l.fallback.Allow(ctx, key, rule)
	}

	result, err := l.primary.Allow(ctx, key, rule)
	if err == nil {
		return result, nil
	}
	if l.fallback == nil {
		return Result{}, err
	}
	return l.fallback.Allow(ctx, key, rule)
}
