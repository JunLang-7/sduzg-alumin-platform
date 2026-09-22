package migrations

import (
	"os"
	"strings"
	"testing"
)

func readMigrateScript(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile("../scripts/migrate.sh")
	if err != nil {
		t.Fatalf("read migrate.sh: %v", err)
	}
	return string(content)
}

func TestMigrateScriptIsLFAndHasBaseline(t *testing.T) {
	content := readMigrateScript(t)
	if strings.Contains(content, "\r\n") {
		t.Error("migrate.sh must use LF line endings")
	}
	for _, want := range []string{
		"schema_migrations",
		"007_fix_data_domain_encoding.sql",
		"cannot connect to MySQL",
		"waiting for MySQL",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("migrate.sh does not contain %q", want)
		}
	}
}

func TestMigrateScriptRepairsStaleDomainBaseline(t *testing.T) {
	content := readMigrateScript(t)
	for _, want := range []string{
		"data_domains missing",
		"DELETE FROM schema_migrations",
		"006_add_admin_access_control.sql",
		"007_fix_data_domain_encoding.sql",
		"schema-derived",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("migrate.sh does not contain %q", want)
		}
	}
	// Must not blindly mark 006/007 applied when only users exists.
	if strings.Contains(content, "('005_add_indexes.sql'),\n        ('006_add_admin_access_control.sql'),\n        ('007_fix_data_domain_encoding.sql');") {
		t.Error("baseline must not always include 006/007 without data_domains check")
	}
}

func TestHistoryMigrationRequiresDataDomains(t *testing.T) {
	content, err := os.ReadFile("008_add_history_wiki.sql")
	if err != nil {
		t.Fatalf("read 008: %v", err)
	}
	if !strings.Contains(string(content), "REFERENCES data_domains(id)") {
		t.Fatal("008 should depend on data_domains")
	}
}
