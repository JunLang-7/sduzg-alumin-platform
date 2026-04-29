package middleware

import (
	"net/http"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
)

const CurrentUserIDKey = "current_user_id"

type AccessTokenParser interface {
	ParseAccessToken(token string) (uint64, error)
}

// Auth 验证 Bearer 访问令牌，并将当前用户 ID 写入 Gin 上下文。
func Auth(parser AccessTokenParser, whitelist ...string) gin.HandlerFunc {
	whiteSet := make(map[string]struct{}, len(whitelist))
	for _, path := range whitelist {
		whiteSet[path] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := whiteSet[c.FullPath()]; ok {
			c.Next()
			return
		}

		if parser == nil {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid authorization header")
			c.Abort()
			return
		}

		userID, err := parser.ParseAccessToken(token)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token")
			c.Abort()
			return
		}

		c.Set(CurrentUserIDKey, userID)
		c.Next()
	}
}

// CurrentUserID 从 Gin 上下文中获取当前用户 ID。如果未找到或类型不匹配，返回 0 和 false。
func CurrentUserID(c *gin.Context) (uint64, bool) {
	value, exists := c.Get(CurrentUserIDKey)
	if !exists {
		return 0, false
	}

	userID, ok := value.(uint64)
	return userID, ok
}

func bearerToken(authHeader string) (string, bool) {
	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}

	return parts[1], true
}
