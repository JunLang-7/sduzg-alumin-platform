package do

import (
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

func enrollmentForSort(id uint64, level, grade string) *model.AlumniEnrollment {
	return &model.AlumniEnrollment{ID: id, TrainingLevel: level, Grade: grade}
}

func TestSortEnrollmentsFollowsLevelThenGradeThenID(t *testing.T) {
	items := []*model.AlumniEnrollment{
		enrollmentForSort(9, common.TrainingLevelUnverified, "2010"),
		enrollmentForSort(8, common.TrainingLevelDoctor, "2013"),
		enrollmentForSort(7, common.TrainingLevelMaster, "2019"),
		enrollmentForSort(6, common.TrainingLevelMaster, "2015级"),
		enrollmentForSort(5, common.TrainingLevelUndergraduate, "2011"),
		enrollmentForSort(4, common.TrainingLevelMaster, "未知"),
		enrollmentForSort(3, common.TrainingLevelMaster, ""),
		enrollmentForSort(2, common.TrainingLevelMaster, "2015届"),
		enrollmentForSort(1, common.TrainingLevelDoctor, "2013"),
	}

	SortEnrollments(items)

	want := []uint64{5, 2, 6, 7, 3, 4, 1, 8, 9}
	for index, item := range items {
		if item.ID != want[index] {
			got := make([]uint64, 0, len(items))
			for _, candidate := range items {
				got = append(got, candidate.ID)
			}
			t.Fatalf("sorted ids = %v, want %v", got, want)
		}
	}
}

func TestSortEnrollmentsStartsFromEarliestExistingLevel(t *testing.T) {
	items := []*model.AlumniEnrollment{
		enrollmentForSort(2, common.TrainingLevelDoctor, "2018"),
		enrollmentForSort(1, common.TrainingLevelMaster, "2014"),
	}

	SortEnrollments(items)

	if items[0].ID != 1 || items[1].ID != 2 {
		t.Fatalf("expected master before doctor when no undergraduate exists, got %d then %d", items[0].ID, items[1].ID)
	}
}

func TestSortEnrollmentsKeepsSameLevelAndGradeStableByID(t *testing.T) {
	items := []*model.AlumniEnrollment{
		enrollmentForSort(31, common.TrainingLevelMaster, "2020"),
		enrollmentForSort(12, common.TrainingLevelMaster, "2020"),
		enrollmentForSort(23, common.TrainingLevelMaster, "2020"),
	}

	SortEnrollments(items)

	if items[0].ID != 12 || items[1].ID != 23 || items[2].ID != 31 {
		t.Fatalf("same level and grade should fall back to id order, got %d/%d/%d", items[0].ID, items[1].ID, items[2].ID)
	}
}

func TestParseEnrollmentGrade(t *testing.T) {
	cases := []struct {
		grade string
		year  int
		ok    bool
	}{
		{"2015", 2015, true},
		{"2015级", 2015, true},
		{" 2015 级", 2015, true},
		{"2015-2019", 2015, true},
		{"15级", 0, false},
		{"", 0, false},
		{"未知", 0, false},
	}
	for _, testCase := range cases {
		year, ok := ParseEnrollmentGrade(testCase.grade)
		if year != testCase.year || ok != testCase.ok {
			t.Fatalf("ParseEnrollmentGrade(%q) = %d, %t; want %d, %t", testCase.grade, year, ok, testCase.year, testCase.ok)
		}
	}
}

func TestSortExportRowsKeepsPerProfileOrderAndAlumniSequence(t *testing.T) {
	first := &model.AlumniProfile{ID: 100}
	second := &model.AlumniProfile{ID: 200}
	rows := []AlumniExportRow{
		{Profile: first, Enrollment: enrollmentForSort(4, common.TrainingLevelMaster, "2018")},
		{Profile: second, Enrollment: enrollmentForSort(5, common.TrainingLevelUndergraduate, "2010")},
		{Profile: first, Enrollment: enrollmentForSort(2, common.TrainingLevelUndergraduate, "2012")},
		{Profile: second, Enrollment: enrollmentForSort(1, common.TrainingLevelMaster, "2016")},
	}

	SortExportRows(rows)

	wantPairs := [][2]uint64{{100, 2}, {100, 4}, {200, 5}, {200, 1}}
	for index, row := range rows {
		if row.Profile.ID != wantPairs[index][0] || row.Enrollment.ID != wantPairs[index][1] {
			t.Fatalf("row %d = profile %d enrollment %d, want %v", index, row.Profile.ID, row.Enrollment.ID, wantPairs[index])
		}
	}
}

func TestTrainingLevelAllowsDataDomainCode(t *testing.T) {
	cases := []struct {
		level string
		code  string
		want  bool
	}{
		{common.TrainingLevelUndergraduate, common.DataDomainUndergraduate, true},
		{common.TrainingLevelUndergraduate, common.DataDomainMPA, false},
		{common.TrainingLevelMaster, common.DataDomainAcademicGraduate, true},
		{common.TrainingLevelMaster, common.DataDomainMPA, true},
		{common.TrainingLevelMaster, common.DataDomainUndergraduate, false},
		{common.TrainingLevelDoctor, common.DataDomainAcademicGraduate, true},
		{common.TrainingLevelDoctor, common.DataDomainMPA, false},
		{common.TrainingLevelUnverified, common.DataDomainMPA, true},
		{"", common.DataDomainMPA, false},
	}
	for _, testCase := range cases {
		if got := common.TrainingLevelAllowsDataDomainCode(testCase.level, testCase.code); got != testCase.want {
			t.Fatalf("TrainingLevelAllowsDataDomainCode(%q, %q) = %t, want %t", testCase.level, testCase.code, got, testCase.want)
		}
	}
}
