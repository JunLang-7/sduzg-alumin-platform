package common

import "testing"

func TestAccessContextAuthorizesAdminDomainsAndPermissions(t *testing.T) {
	access := AccessContext{
		UserID:      7,
		Role:        RoleAdmin,
		DomainIDs:   []uint64{2, 5},
		Permissions: map[string]bool{PermissionAlumniSensitiveRead: true},
	}

	if !access.CanAccessDomain(2) || !access.CanAccessDomain(5) || access.CanAccessDomain(3) {
		t.Fatalf("unexpected domain access result: %+v", access)
	}
	if !access.HasPermission(PermissionAlumniSensitiveRead) {
		t.Fatal("expected sensitive read permission")
	}
	if access.HasPermission(PermissionAlumniFilesManage) {
		t.Fatal("did not expect file management permission")
	}
}

func TestAccessContextDefaultsToDenyAndSuperAdminIsImplicitlyAllowed(t *testing.T) {
	admin := AccessContext{UserID: 7, Role: RoleAdmin}
	if admin.CanAccessDomain(1) || admin.HasPermission(PermissionAlumniSensitiveRead) {
		t.Fatalf("admin without assignments must be denied: %+v", admin)
	}

	superAdmin := AccessContext{UserID: 1, Role: RoleSuperAdmin}
	if !superAdmin.IsSuperAdmin() {
		t.Fatal("expected super admin context")
	}
	if !superAdmin.CanAccessDomain(99) {
		t.Fatal("expected super admin to access every data domain")
	}
	if !superAdmin.HasPermission(PermissionAlumniFilesManage) {
		t.Fatal("expected super admin to have every permission")
	}
}
