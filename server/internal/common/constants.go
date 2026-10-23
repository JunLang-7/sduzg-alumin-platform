package common

// IsKnownUserRole 对数据库角色进行大小写敏感的精确匹配。
func IsKnownUserRole(role string) bool {
	switch role {
	case RoleAlumni, RoleAdmin, RoleSuperAdmin:
		return true
	default:
		return false
	}
}

const (
	RoleAlumni     = "alumni"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"

	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
	UserStatusDeleted  = "deleted"

	AlumniStatusActive  = "active"
	AlumniStatusDeleted = "deleted"

	FileTypeDegreeArchive  = "degree_archive"
	FileTypeAcademicRecord = "academic_record"

	FileStatusActive  = "active"
	FileStatusPending = "pending"
	FileStatusDeleted = "deleted"
)
