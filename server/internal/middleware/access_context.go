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

const AccessContextKey = "access_context"

// UserFinder 根据用户 ID 查询当前用户状态和角色。
type UserFinder interface {
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}

// AdminAccessControlReader 查询管理员有效数据域和功能权限。
type AdminAccessControlReader interface {
	ListActiveDataDomains(ctx context.Context) ([]*model.DataDomain, error)
	ListAdminDataDomainIDs(ctx context.Context, userID uint64) ([]uint64, error)
	ListAdminPermissionCodes(ctx context.Context, userID uint64) ([]string, error)
}

// AccessContextLoader 从当前数据库状态构造请求级授权上下文。
type AccessContextLoader struct {
	users         UserFinder
	accessControl AdminAccessControlReader
}

func NewAccessContextLoader(users UserFinder, accessControl AdminAccessControlReader) *AccessContextLoader {
	return &AccessContextLoader{users: users, accessControl: accessControl}
}

// Load 加载一次用户、数据域和权限信息；任一异常均返回错误以默认拒绝请求。
func (l *AccessContextLoader) Load(ctx context.Context, userID uint64) (*common.AccessContext, error) {
	if l == nil || l.users == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	user, err := l.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.ErrUserNotFound
	}
	if user.Status != common.UserStatusActive {
		return nil, common.ErrAccountDisabled
	}

	access := &common.AccessContext{
		UserID:      user.ID,
		Role:        user.Role,
		AlumniID:    user.AlumniID,
		DomainIDs:   []uint64{},
		Permissions: map[string]bool{},
	}
	if access.IsSuperAdmin() {
		return access, nil
	}
	if user.Role != common.RoleAdmin {
		return access, nil
	}
	if l.accessControl == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	activeDomains, err := l.accessControl.ListActiveDataDomains(ctx)
	if err != nil {
		return nil, err
	}
	activeDomainIDs := make(map[uint64]struct{}, len(activeDomains))
	for _, domain := range activeDomains {
		if domain != nil && domain.ID != 0 {
			activeDomainIDs[domain.ID] = struct{}{}
		}
	}

	domainIDs, err := l.accessControl.ListAdminDataDomainIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	permissionCodes, err := l.accessControl.ListAdminPermissionCodes(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, domainID := range domainIDs {
		if _, ok := activeDomainIDs[domainID]; ok {
			access.DomainIDs = append(access.DomainIDs, domainID)
		}
	}
	for _, code := range permissionCodes {
		if code != "" {
			access.Permissions[code] = true
		}
	}
	return access, nil
}

// LoadAccessContext 在认证完成后加载授权上下文，并写入 Gin 上下文供后续中间件和业务层复用。
func LoadAccessContext(loader *AccessContextLoader, whitelist ...string) gin.HandlerFunc {
	whiteSet := make(map[string]struct{}, len(whitelist))
	for _, path := range whitelist {
		whiteSet[path] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := whiteSet[c.FullPath()]; ok {
			c.Next()
			return
		}

		userID, ok := CurrentUserID(c)
		if !ok {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		access, err := loader.Load(c.Request.Context(), userID)
		if err != nil {
			switch {
			case errors.Is(err, common.ErrDatabaseUnavailable):
				response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
			case errors.Is(err, common.ErrUserNotFound):
				response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
			case errors.Is(err, common.ErrAccountDisabled):
				response.Fail(c, http.StatusForbidden, response.CodeForbidden, "account is disabled")
			default:
				response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
			}
			c.Abort()
			return
		}

		c.Set(AccessContextKey, access)
		c.Next()
	}
}

// CurrentAccessContext 从 Gin 上下文中获取当前请求的授权上下文。
func CurrentAccessContext(c *gin.Context) (*common.AccessContext, bool) {
	value, exists := c.Get(AccessContextKey)
	if !exists {
		return nil, false
	}

	access, ok := value.(*common.AccessContext)
	return access, ok && access != nil
}
