package common

// AccessContext 表示一次请求内已完成加载的授权信息。
// JWT 只提供用户身份，角色、数据域和权限码均以当前数据库记录为准。
type AccessContext struct {
	UserID      uint64
	Role        string
	AlumniID    *uint64
	DomainIDs   []uint64
	Permissions map[string]bool
}

// IsSuperAdmin 判断当前用户是否为拥有全域、全权限的超级管理员。
func (c AccessContext) IsSuperAdmin() bool {
	return c.Role == RoleSuperAdmin
}

// IsAdministrator 判断当前用户是否为管理员或超级管理员。
func (c AccessContext) IsAdministrator() bool {
	return c.Role == RoleAdmin || c.IsSuperAdmin()
}

// CanAccessDomain 判断当前用户是否可访问指定的数据域。
func (c AccessContext) CanAccessDomain(domainID uint64) bool {
	if domainID == 0 {
		return false
	}
	if c.IsSuperAdmin() {
		return true
	}
	for _, assignedID := range c.DomainIDs {
		if assignedID == domainID {
			return true
		}
	}
	return false
}

// HasPermission 判断当前用户是否拥有指定功能权限。
func (c AccessContext) HasPermission(permission string) bool {
	if permission == "" {
		return false
	}
	if c.IsSuperAdmin() {
		return true
	}
	return c.Permissions[permission]
}
