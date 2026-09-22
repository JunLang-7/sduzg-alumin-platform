package migrations

import (
	"os"
	"path/filepath"
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

func TestMigrateScriptIsLFAndMetadataDriven(t *testing.T) {
	content := readMigrateScript(t)
	if strings.Contains(content, "\r\n") {
		t.Error("migrate.sh must use LF line endings")
	}
	for _, want := range []string{
		"schema_migrations",
		"cannot connect to MySQL",
		"waiting for MySQL",
		"migrate: proves",
		"migrate: requires",
		"baseline_from_schema",
		"repair_stale_marks",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("migrate.sh does not contain %q", want)
		}
	}
	// Filenames must not be hardcoded for baseline/repair lists.
	for _, banned := range []string{
		"006_add_admin_access_control.sql",
		"007_fix_data_domain_encoding.sql",
		"001_init_schema.sql",
		"('005_add_indexes.sql')",
	} {
		if strings.Contains(content, banned) {
			t.Errorf("migrate.sh must not hardcode migration file %q", banned)
		}
	}
}

func TestMigrationFilesDeclareProvesMetadata(t *testing.T) {
	files, err := filepath.Glob("[0-9][0-9][0-9]_*.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no migration files found")
	}

	// Checkpoints that prove progressive schema presence on old volumes.
	proves := map[string]string{
		"001_init_schema.sql":              "proves users",
		"003_add_alumni_files.sql":         "proves alumni_files",
		"006_add_admin_access_control.sql": "proves data_domains",
		"008_add_history_wiki.sql":         "proves history_entries",
		"009_add_migration_tracking.sql":   "proves schema_migrations",
	}
	for name, want := range proves {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(content), "-- migrate: "+want) {
			t.Errorf("%s missing %q", name, "-- migrate: "+want)
		}
	}

	// 007/008 must declare data_domains dependency for stale-mark repair.
	for _, name := range []string{
		"007_fix_data_domain_encoding.sql",
		"008_add_history_wiki.sql",
	} {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(content), "-- migrate: requires data_domains") {
			t.Errorf("%s missing requires data_domains", name)
		}
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
