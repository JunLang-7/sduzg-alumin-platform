package limiter

import (
	"net/http"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
)

const defaultPeriod = time.Minute

func RuleForRequest(method string, path string, cfg config.RateLimitConfig) (Rule, bool) {
	if !cfg.Enabled || isHealthCheck(path) {
		return Rule{}, false
	}

	switch {
	case method == http.MethodPost && path == "/api/v1/auth/login":
		return newRule("auth_login", cfg.AuthRPM, minPositive(3, cfg.AuthRPM)), true
	case method == http.MethodPost && path == "/api/v1/auth/verify-code/send":
		return newRule("verify_code", cfg.VerifyCodeRPM, 1), true
	case strings.HasPrefix(path, "/api/v1/admin/") || strings.HasPrefix(path, "/api/v1/super-admin/"):
		return newRule("admin", cfg.AdminRPM, minPositive(5, cfg.AdminRPM)), true
	default:
		return newRule("global", cfg.GlobalRPM, cfg.GlobalRPM), true
	}
}

func isHealthCheck(path string) bool {
	return path == "/api/v1/health/live" || path == "/api/v1/health/ready"
}

func newRule(name string, limit int, burst int) Rule {
	return Rule{
		Name:   name,
		Limit:  limit,
		Burst:  burst,
		Period: defaultPeriod,
	}
}

func minPositive(a int, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 || a < b {
		return a
	}
	return b
}
