package do

import (
	"slices"
	"testing"
)

func TestAlumniListQueryNormalizeSortsAndDeduplicatesDataDomainIDs(t *testing.T) {
	query := AlumniListQuery{DataDomainIDs: []uint64{3, 1, 2, 1, 3}}.Normalize()
	if !slices.Equal(query.DataDomainIDs, []uint64{1, 2, 3}) {
		t.Fatalf("normalized data domains = %v, want [1 2 3]", query.DataDomainIDs)
	}
}

func TestAlumniDedupKeyIncludesMobile(t *testing.T) {
	base := AlumniDedupKey{Name: "张三", Grade: "2020级", ClassName: "MPA", Cohort: "2020", DataDomainID: 3}
	withMobileA := base
	withMobileA.Mobile = "13800000000"
	withMobileB := base
	withMobileB.Mobile = "13900000000"

	if withMobileA.Key() == withMobileB.Key() {
		t.Fatalf("expected different keys for different mobile values, got %q", withMobileA.Key())
	}
}

func TestAlumniDedupKeyTrimsFields(t *testing.T) {
	trimmed := AlumniDedupKey{Name: "张三", Grade: "2020级", ClassName: "MPA", Cohort: "2020", Mobile: "13800000000", DataDomainID: 3}
	padded := AlumniDedupKey{Name: " 张三 ", Grade: " 2020级 ", ClassName: " MPA ", Cohort: " 2020 ", Mobile: " 13800000000 ", DataDomainID: 3}

	if trimmed.Key() != padded.Key() {
		t.Fatalf("expected padded key %q to match trimmed key %q", padded.Key(), trimmed.Key())
	}
}

func TestAlumniPersonalInputNormalizeTrimsMobile(t *testing.T) {
	mobile := " 13800000000 "
	profile := AlumniPersonalInput{Name: "张三", Mobile: &mobile}.Normalize()

	if profile.Mobile == nil || *profile.Mobile != "13800000000" {
		t.Fatalf("expected trimmed mobile, got %+v", profile.Mobile)
	}
}

func TestAlumniPersonalInputNormalizeEmptyMobile(t *testing.T) {
	mobile := " "
	profile := AlumniPersonalInput{Name: "张三", Mobile: &mobile}.Normalize()

	if profile.Mobile != nil {
		t.Fatalf("expected empty mobile to be nil, got %+v", profile.Mobile)
	}
}

func TestAlumniEnrollmentInputNormalizeTrimsAndDeduplicatesFileIDs(t *testing.T) {
	grade := " 2026 "
	enrollment := AlumniEnrollmentInput{
		Grade:           grade,
		TrainingLevel:   " master ",
		ActivateFileIDs: []uint64{3, 1, 3, 0},
		RemoveFileIDs:   []uint64{2, 2},
	}.Normalize()

	if enrollment.Grade != "2026" || enrollment.TrainingLevel != "master" {
		t.Fatalf("unexpected normalized enrollment: %+v", enrollment)
	}
	if !slices.Equal(enrollment.ActivateFileIDs, []uint64{3, 1}) {
		t.Fatalf("activate file ids = %v, want [3 1]", enrollment.ActivateFileIDs)
	}
	if !slices.Equal(enrollment.RemoveFileIDs, []uint64{2}) {
		t.Fatalf("remove file ids = %v, want [2]", enrollment.RemoveFileIDs)
	}
}

func TestAlumniListQueryIsUnfilteredExcludesEnrollmentFilters(t *testing.T) {
	if !(AlumniListQuery{}).IsUnfiltered() {
		t.Fatal("empty query should be unfiltered")
	}
	if (AlumniListQuery{Grade: "2026"}).IsUnfiltered() {
		t.Fatal("enrollment filter must disable cached count")
	}
	if (AlumniListQuery{TrainingLevel: "master"}).IsUnfiltered() {
		t.Fatal("training level filter must disable cached count")
	}
}
