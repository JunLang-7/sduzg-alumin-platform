package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestAccessControlMigrationDefinesRequiredTablesAndBackfill(t *testing.T) {
	content, err := os.ReadFile("006_add_admin_access_control.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	migration := string(content)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS data_domains",
		"CREATE TABLE IF NOT EXISTS admin_data_scopes",
		"CREATE TABLE IF NOT EXISTS admin_permissions",
		"ADD COLUMN data_domain_id",
		"'undergraduate'",
		"'academic_graduate'",
		"'mpa'",
		"UPDATE alumni_profiles",
		"MODIFY COLUMN data_domain_id BIGINT UNSIGNED NOT NULL",
		"INSERT IGNORE INTO admin_data_scopes",
		"UNIQUE KEY uk_data_domains_code (code)",
		"PRIMARY KEY (user_id, data_domain_id)",
		"PRIMARY KEY (user_id, permission_code)",
	} {
		if !strings.Contains(migration, want) {
			t.Errorf("migration does not contain %q", want)
		}
	}
}
