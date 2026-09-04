package middleware

import (
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
)

// RequireRoles 校验当前授权上下文是否拥有指定角色之一。
func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		access, ok := CurrentAccessContext(c)
		if !ok {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		if _, ok := allowed[access.Role]; !ok {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "permission denied")
			c.Abort()
			return
		}

		c.Next()
	}
}
