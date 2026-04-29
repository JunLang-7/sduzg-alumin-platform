package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
)

type UserFinder interface {
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}

// RequireRoles 校验当前登录用户是否拥有指定角色之一。
func RequireRoles(users UserFinder, roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		userID, ok := CurrentUserID(c)
		if !ok {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		if users == nil {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
			c.Abort()
			return
		}

		user, err := users.FindByID(c.Request.Context(), userID)
		switch {
		case errors.Is(err, common.ErrDatabaseUnavailable):
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
			c.Abort()
			return
		case errors.Is(err, common.ErrUserNotFound):
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		case err != nil:
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
			c.Abort()
			return
		}

		if user.Status != common.UserStatusActive {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "account is disabled")
			c.Abort()
			return
		}
		if _, ok := allowed[user.Role]; !ok {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "permission denied")
			c.Abort()
			return
		}

		c.Next()
	}
}
