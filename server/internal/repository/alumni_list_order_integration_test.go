package repository

import (
	"context"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

// TestAlumniRepositoryListOrdersByPinyinName 校友名册按姓名拼音升序展示，
// 与导入批次顺序无关；同名档案按 id 升序稳定并列。
// 回归背景：列表此前按 id DESC 展示，全量拼音升序导入后表现为「Z→A」，
// 与运营方名册习惯相反（见 #255）。
func TestAlumniRepositoryListOrdersByPinyinName(t *testing.T) {
	db := openLifecycleTestDB(t)
	mpa, _ := lifecycleDomains(t, db)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	ctx := context.Background()

	// 按 id 递增插入，使 id 顺序与拼音顺序完全相反；末尾两个同名档案验证稳定并列。
	seed := []string{"张三", "李四", "王五", "安一", "Alice", "安二", "重名", "重名"}
	var firstDuplicate uint64
	ids := make(map[string]uint64, len(seed))
	for _, name := range seed {
		profile := seedLifecycleAlumni(tx, name, &model.AlumniEnrollment{
			DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2019级",
		})
		if _, seen := ids[name]; name == "重名" && !seen {
			firstDuplicate = profile.ID
		}
		ids[name] = profile.ID
	}

	repo := NewAlumniRepository(tx)
	// 用年级条件隔离迁移种子自带的「测试校友」，只统计本用例的 8 份档案。
	query := do.AlumniListQuery{DataDomainIDs: []uint64{mpa.ID}, Grade: "2019级", Page: common.PageQuery{Page: 1, PageSize: 100}}
	query = query.Normalize()

	// List：分页接口按拼音序返回。
	records, total, err := repo.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if total != int64(len(seed)) {
		t.Fatalf("total = %d, want %d", total, len(seed))
	}
	got := make([]string, 0, len(records))
	for _, record := range records {
		got = append(got, record.Profile.Name)
	}
	// gbk 排序近似拼音序：英文姓名排在中文之前；安二(an-er) 先于 安一(an-yi)；
	// 多音字按常用读音参与排序（重→zhòng）；同名按 id 升序。
	want := []string{"Alice", "安二", "安一", "李四", "王五", "张三", "重名", "重名"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("list order = %v, want %v", got, want)
		}
	}
	if first, second := records[6].Profile.ID, records[7].Profile.ID; first != firstDuplicate || second <= first {
		t.Fatalf("同名并列序不稳定：first=%d second=%d seed=%d", first, second, firstDuplicate)
	}

	// FindOnly：缓存计数路径与 List 同序。
	only, err := repo.FindOnly(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want {
		if only[i].Profile.Name != want[i] {
			t.Fatalf("FindOnly order mismatch at %d: got %s, want %s", i, only[i].Profile.Name, want[i])
		}
	}

	// ListAll：导出与列表同序。
	rows, err := repo.ListAll(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want {
		if rows[i].Profile.Name != want[i] {
			t.Fatalf("ListAll order mismatch at %d: got %s, want %s", i, rows[i].Profile.Name, want[i])
		}
	}

	// 删除拼音靠前的档案后，剩余名册仍按拼音序且不跳变。
	if _, err := repo.Delete(ctx, ids["安一"], 1, []uint64{mpa.ID}); err != nil {
		t.Fatal(err)
	}
	records, _, err = repo.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	got = got[:0]
	for _, record := range records {
		got = append(got, record.Profile.Name)
	}
	wantAfterDelete := []string{"Alice", "安二", "李四", "王五", "张三", "重名", "重名"}
	for i := range wantAfterDelete {
		if got[i] != wantAfterDelete[i] {
			t.Fatalf("post-delete order = %v, want %v", got, wantAfterDelete)
		}
	}
}
