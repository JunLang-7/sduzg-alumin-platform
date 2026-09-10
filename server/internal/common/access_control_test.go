package common

import "testing"

func TestDataDomainCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want bool
	}{
		{DataDomainUndergraduate, true},
		{DataDomainAcademicGraduate, true},
		{DataDomainMPA, true},
		{"graduate", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := IsKnownDataDomainCode(tt.code); got != tt.want {
				t.Fatalf("IsKnownDataDomainCode(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestDataDomainDisplayNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want string
	}{
		{DataDomainUndergraduate, "本科生"},
		{DataDomainAcademicGraduate, "学术学位研究生"},
		{DataDomainMPA, "MPA专业学位研究生"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := DataDomainDisplayName(tt.code); got != tt.want {
				t.Fatalf("DataDomainDisplayName(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestAdminPermissionCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want bool
	}{
		{PermissionAlumniSensitiveRead, true},
		{PermissionAlumniFilesManage, true},
		{"alumni.sensitive.write", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := IsKnownAdminPermission(tt.code); got != tt.want {
				t.Fatalf("IsKnownAdminPermission(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}
