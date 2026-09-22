package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigrateScriptIsLFAndHasBaseline(t *testing.T) {
	content, err := os.ReadFile("migrate.sh")
	// script lives in ../scripts relative to this package when tests run from migrations/
	if err != nil {
		content, err = os.ReadFile("../scripts/migrate.sh")
	}
	if err != nil {
		t.Fatalf("read migrate.sh: %v", err)
	}
	if strings.Contains(string(content), "\r\n") {
		t.Error("migrate.sh must use LF line endings")
	}
	for _, want := range []string{
		"schema_migrations",
		"007_fix_data_domain_encoding.sql",
		"cannot connect to MySQL",
		"waiting for MySQL",
	} {
		if !strings.Contains(string(content), want) {
			t.Errorf("migrate.sh does not contain %q", want)
		}
	}
}
