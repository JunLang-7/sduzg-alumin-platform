package common

const (
	// DataDomainStatusActive means a domain can be assigned and queried.
	DataDomainStatusActive = "active"
	// DataDomainStatusDisabled means a domain is retained but cannot be assigned.
	DataDomainStatusDisabled = "disabled"

	// DataDomainUndergraduate covers undergraduate alumni.
	DataDomainUndergraduate = "undergraduate"
	// DataDomainAcademicGraduate covers academic master's and doctoral alumni.
	DataDomainAcademicGraduate = "academic_graduate"
	// DataDomainMPA covers MPA professional-degree alumni.
	DataDomainMPA = "mpa"
)

const (
	// PermissionAlumniSensitiveRead allows access to protected alumni profile fields.
	PermissionAlumniSensitiveRead = "alumni.sensitive.read"
	// PermissionAlumniFilesManage allows managing alumni archive files.
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

// IsKnownDataDomainCode reports whether code is a supported alumni data domain.
func IsKnownDataDomainCode(code string) bool {
	_, ok := knownDataDomainCodes[code]
	return ok
}

// IsKnownAdminPermission reports whether code is a supported administrator permission.
func IsKnownAdminPermission(code string) bool {
	_, ok := knownAdminPermissions[code]
	return ok
}
