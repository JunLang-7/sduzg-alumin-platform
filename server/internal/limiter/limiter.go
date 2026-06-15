package limiter

import (
	"context"
	"time"
)

type Rule struct {
	Name   string
	Limit  int
	Burst  int
	Period time.Duration
}

type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string, rule Rule) (Result, error)
}
