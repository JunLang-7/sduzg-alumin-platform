// Regenerate the deletion-request model after applying its schema migration.
package main

import (
	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
	"log"
	"os"
	"path/filepath"
)

func main() {
	dsn := os.Getenv("HISTORY_MODELGEN_DSN")
	if dsn == "" {
		log.Fatal("HISTORY_MODELGEN_DSN is required")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("cannot connect to model generation database")
	}
	g := gen.NewGenerator(gen.Config{OutPath: filepath.Join("tmp", "history-deletion-query"), ModelPkgPath: filepath.Join("internal", "model"), FieldNullable: true, FieldSignable: true, FieldWithTypeTag: true, FieldWithIndexTag: true})
	g.UseDB(db)
	g.GenerateModel("history_entry_deletion_requests", gen.FieldJSONTag("pending_entry_id", "-"))
	g.Execute()
}
