package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestDataDomainMigrationsUseUTF8AndRepairDisplayNames(t *testing.T) {
	initialMigration, err := os.ReadFile("006_add_admin_access_control.sql")
	if err != nil {
		t.Fatalf("read initial migration: %v", err)
	}
	if !strings.Contains(string(initialMigration), "SET NAMES utf8mb4") {
		t.Fatal("initial migration must declare utf8mb4 before inserting Chinese display names")
	}

	repairMigration, err := os.ReadFile("007_fix_data_domain_encoding.sql")
	if err != nil {
		t.Fatalf("read repair migration: %v", err)
	}
	for _, want := range []string{
		"SET NAMES utf8mb4",
		"WHEN 'undergraduate' THEN '本科生'",
		"WHEN 'academic_graduate' THEN '学术学位研究生'",
		"WHEN 'mpa' THEN 'MPA专业学位研究生'",
	} {
		if !strings.Contains(string(repairMigration), want) {
			t.Errorf("repair migration does not contain %q", want)
		}
	}
}
