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

	// 数据域编码是培养类别的稳定标识，培养层次必须与类别组成合法组合。
	DataDomainCodeUndergraduate    = "undergraduate"
	DataDomainCodeAcademicGraduate = "academic_graduate"
	DataDomainCodeMPA              = "mpa"
)

// AllowedDataDomainCodes 返回某个培养层次允许的培养类别编码。
// unverified 为历史数据的待核验状态，不约束类别，避免无关编辑被迫猜测层次。
// 返回 nil 表示该层次未知，任何类别都不合法。
func AllowedDataDomainCodes(trainingLevel string) []string {
	switch trainingLevel {
	case TrainingLevelUndergraduate:
		return []string{DataDomainUndergraduate}
	case TrainingLevelMaster:
		return []string{DataDomainAcademicGraduate, DataDomainMPA}
	case TrainingLevelDoctor:
		return []string{DataDomainAcademicGraduate}
	case TrainingLevelUnverified:
		return []string{DataDomainUndergraduate, DataDomainAcademicGraduate, DataDomainMPA}
	default:
		return nil
	}
}

// TrainingLevelAllowsDataDomainCode 判断培养层次与培养类别的组合是否合法。
func TrainingLevelAllowsDataDomainCode(trainingLevel, dataDomainCode string) bool {
	for _, code := range AllowedDataDomainCodes(trainingLevel) {
		if code == dataDomainCode {
			return true
		}
	}
	return false
}
