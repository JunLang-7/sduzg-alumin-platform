package common

const (
	// DataDomainStatusActive 表示数据域可分配、可查询。
	DataDomainStatusActive = "active"
	// DataDomainStatusDisabled 表示数据域保留但不可继续分配。
	DataDomainStatusDisabled = "disabled"

	// DataDomainUndergraduate 表示本科生校友数据域。
	DataDomainUndergraduate = "undergraduate"
	// DataDomainAcademicGraduate 表示学术学位硕士、博士校友数据域。
	DataDomainAcademicGraduate = "academic_graduate"
	// DataDomainMPA 表示 MPA 专业学位校友数据域。
	DataDomainMPA = "mpa"
)

const (
	// PermissionAlumniSensitiveRead 允许查看受保护的校友档案字段。
	PermissionAlumniSensitiveRead = "alumni.sensitive.read"
	// PermissionAlumniFilesManage 允许管理校友档案文件。
	PermissionAlumniFilesManage = "alumni.files.manage"
)

var knownDataDomainCodes = map[string]struct{}{
	DataDomainUndergraduate:    {},
	DataDomainAcademicGraduate: {},
	DataDomainMPA:              {},
}

var knownAdminPermissions = map[string]struct{}{
	PermissionAlumniSensitiveRead: {},
	PermissionAlumniFilesManage:   {},
}

// IsKnownDataDomainCode 判断 code 是否为系统支持的校友数据域。
func IsKnownDataDomainCode(code string) bool {
	_, ok := knownDataDomainCodes[code]
	return ok
}

// IsKnownAdminPermission 判断 code 是否为系统支持的管理员权限。
func IsKnownAdminPermission(code string) bool {
	_, ok := knownAdminPermissions[code]
	return ok
}
