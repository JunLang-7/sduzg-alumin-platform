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
	FileStatusDraft   = "draft"
	FileStatusPending = "pending"
	FileStatusDeleted = "deleted"

	// 培养层次与数据域是两个维度：本科/硕士/博士为用户明确选择，
	// unverified 表示历史数据无法可靠区分硕士/博士，需要人工核验。
	TrainingLevelUndergraduate = "undergraduate"
	TrainingLevelMaster        = "master"
	TrainingLevelDoctor        = "doctor"
	TrainingLevelUnverified    = "unverified"
)
