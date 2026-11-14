package do

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

// enrollmentGradePattern 只在年级以 4 位入学年份开头时视为可解析，
// 例如 "2015"、"2015级"、"2015-2019"；"未知"、"15级" 等按不可解析处理，
// 排在同层次末尾，避免臆测入学年份。
var enrollmentGradePattern = regexp.MustCompile(`^\s*(\d{4})(?:\D|$)`)

// TrainingLevelRank 返回培养层次的展示顺序权重：本科 → 硕士 → 博士 → 待核验。
// 未识别的层次与待核验同组，保证历史脏数据也能稳定排序。
func TrainingLevelRank(level string) int {
	switch strings.TrimSpace(level) {
	case common.TrainingLevelUndergraduate:
		return 0
	case common.TrainingLevelMaster:
		return 1
	case common.TrainingLevelDoctor:
		return 2
	case common.TrainingLevelUnverified:
		return 3
	default:
		return 4
	}
}

// ParseEnrollmentGrade 解析入学年级中的 4 位年份。
// 返回值 ok 为 false 表示缺失或无法解析，调用方应把该学籍排在同层次末尾。
func ParseEnrollmentGrade(grade string) (int, bool) {
	matches := enrollmentGradePattern.FindStringSubmatch(grade)
	if len(matches) != 2 {
		return 0, false
	}
	year, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return year, true
}

// enrollmentSortLess 是学籍的唯一展示顺序：
// 层次权重升序 → 可解析年级升序（缺失/不可解析排末尾）→ 学籍 ID 升序。
func enrollmentSortLess(a, b *model.AlumniEnrollment) bool {
	rankA, rankB := TrainingLevelRank(a.TrainingLevel), TrainingLevelRank(b.TrainingLevel)
	if rankA != rankB {
		return rankA < rankB
	}
	gradeA, okA := ParseEnrollmentGrade(a.Grade)
	gradeB, okB := ParseEnrollmentGrade(b.Grade)
	if okA != okB {
		return okA
	}
	if okA && gradeA != gradeB {
		return gradeA < gradeB
	}
	return a.ID < b.ID
}

// SortEnrollments 就地对学籍段排序，供详情、列表摘要、本人资料与导出复用同一顺序。
func SortEnrollments(enrollments []*model.AlumniEnrollment) {
	sort.SliceStable(enrollments, func(i, j int) bool {
		return enrollmentSortLess(enrollments[i], enrollments[j])
	})
}

// SortExportRows 按「个人档案顺序 + 学籍展示顺序」排列导出行。
// 同一人的学籍行顺序与详情页一致；个人之间的顺序保持仓储返回顺序。
func SortExportRows(rows []AlumniExportRow) {
	if len(rows) < 2 {
		return
	}
	profileOrder := make(map[uint64]int, len(rows))
	next := 0
	for _, row := range rows {
		if row.Profile == nil {
			continue
		}
		if _, ok := profileOrder[row.Profile.ID]; !ok {
			profileOrder[row.Profile.ID] = next
			next++
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		leftRank, rightRank := exportProfileRank(left, profileOrder), exportProfileRank(right, profileOrder)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if left.Enrollment == nil || right.Enrollment == nil {
			return left.Enrollment != nil
		}
		return enrollmentSortLess(left.Enrollment, right.Enrollment)
	})
}

func exportProfileRank(row AlumniExportRow, order map[uint64]int) int {
	if row.Profile == nil {
		return len(order)
	}
	if rank, ok := order[row.Profile.ID]; ok {
		return rank
	}
	return len(order)
}
