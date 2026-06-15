package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/limiter"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RateLimit(cfg config.RateLimitConfig, rateLimiter limiter.Limiter, log *zap.Logger) gin.HandlerFunc {
	if log == nil {
		log = zap.NewNop()
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		rule, ok := limiter.RuleForRequest(c.Request.Method, path, cfg)
		if !ok || rateLimiter == nil || rule.Limit <= 0 {
			c.Next()
			return
		}

		key := rateLimitKey(c, rule, path)
		key = scopedRequestKey(c, rule, key)

		result, err := rateLimiter.Allow(c.Request.Context(), key, rule)
		if err != nil {
			log.Warn("rate limit unavailable",
				zap.Error(err),
				zap.String("path", path),
				zap.String("rule", rule.Name),
			)
			c.Next()
			return
		}
		if result.Allowed {
			c.Next()
			return
		}

		if result.RetryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
		}
		response.Fail(c, http.StatusTooManyRequests, response.CodeTooManyRequests, "请求过于频繁，请稍后再试")
		c.Abort()
	}
}

func rateLimitKey(c *gin.Context, rule limiter.Rule, path string) string {
	if userID, ok := CurrentUserID(c); ok {
		return fmt.Sprintf("rate_limit:%s:user:%d:%s", rule.Name, userID, path)
	}
	return fmt.Sprintf("rate_limit:%s:ip:%s:%s", rule.Name, c.ClientIP(), path)
}

func scopedRequestKey(c *gin.Context, rule limiter.Rule, fallback string) string {
	switch rule.Name {
	case "auth_login":
		if account := requestBodyField(c, "account"); account != "" {
			return fmt.Sprintf("rate_limit:%s:ip:%s:account:%s:%s", rule.Name, c.ClientIP(), account, c.Request.URL.Path)
		}
	case "verify_code":
		if target := requestBodyField(c, "target"); target != "" {
			return fmt.Sprintf("rate_limit:%s:ip:%s:target:%s:%s", rule.Name, c.ClientIP(), target, c.Request.URL.Path)
		}
	}
	return fallback
}

func requestBodyField(c *gin.Context, field string) string {
	if c.Request.Body == nil {
		return ""
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	value, ok := payload[field].(string)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(value))
}
