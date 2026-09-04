package repository

import (
	"errors"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"gorm.io/gorm"
)

func TestMapDataDomainLookupError(t *testing.T) {
	t.Parallel()

	if err := mapDataDomainLookupError(gorm.ErrRecordNotFound); !errors.Is(err, common.ErrDataDomainUnavailable) {
		t.Fatalf("mapDataDomainLookupError(record not found) = %v, want ErrDataDomainUnavailable", err)
	}

	databaseErr := errors.New("database query failed")
	if err := mapDataDomainLookupError(databaseErr); !errors.Is(err, databaseErr) {
		t.Fatalf("mapDataDomainLookupError(database error) = %v, want original error", err)
	}
}
