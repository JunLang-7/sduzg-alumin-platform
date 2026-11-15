package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"gorm.io/gorm"
)

// TestAuditCrossDomainCorrectionRequiresBothSides 固定跨类别更正事件的可见性合同：
// 事件在目标类别下声明归属，但只有同时拥有原类别与目标类别的管理员才能读取，
// 仅持有单侧权限时不展示，避免泄露另一类别的字段旧值。
func TestAuditCrossDomainCorrectionRequiresBothSides(t *testing.T) {
	db := openLifecycleTestDB(t)
	mpa, academic := lifecycleDomains(t, db)

	rollback := errRollbackAuditVisibility
	err := db.Transaction(func(tx *gorm.DB) error {
		profile := seedLifecycleAlumni(tx, "跨域更审计可见性测试",
			&model.AlumniEnrollment{DataDomainID: academic.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2020"},
		)
		operator := model.User{Account: "audit-visibility-operator", PasswordHash: "x", Role: common.RoleSuperAdmin, Status: common.UserStatusActive}
		if err := tx.Create(&operator).Error; err != nil {
			return err
		}

		detail := func(payload map[string]any) *string {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return nil
			}
			text := string(encoded)
			return &text
		}
		targetID := profile.ID
		logs := []model.OperationLog{
			{
				OperatorID: operator.ID, OperatorRole: common.RoleSuperAdmin, Action: "update",
				TargetType: auditTargetAlumni, TargetID: &targetID,
				// 同类别内的普通学籍修改：仅声明本类别即可见。
				Detail: detail(map[string]any{
					"data_domain_ids": []uint64{mpa.ID},
					"changes":         []map[string]string{{"field_name": "grade", "old_value": "2019", "new_value": "2020"}},
				}),
			},
			{
				OperatorID: operator.ID, OperatorRole: common.RoleSuperAdmin, Action: "update",
				TargetType: auditTargetAlumni, TargetID: &targetID,
				// 跨类别更正：目标类别声明归属，但要求双侧权限。
				Detail: detail(map[string]any{
					"data_domain_ids":          []uint64{academic.ID},
					"required_data_domain_ids": []uint64{mpa.ID, academic.ID},
					"changes":                  []map[string]string{{"field_name": "data_domain_id", "old_value": "MPA专业学位研究生", "new_value": "学术学位研究生"}},
				}),
			},
			{
				OperatorID: operator.ID, OperatorRole: common.RoleSuperAdmin, Action: "update",
				TargetType: auditTargetAlumni, TargetID: &targetID,
				// 目标类别内部的普通修改，任何时间都应可见于目标类别管理员。
				Detail: detail(map[string]any{
					"data_domain_ids": []uint64{academic.ID},
					"changes":         []map[string]string{{"field_name": "major", "old_value": "公共管理", "new_value": "行政管理"}},
				}),
			},
		}
		if err := tx.Create(&logs).Error; err != nil {
			return err
		}

		repo := NewAuditRepository(tx)
		ctx := context.Background()
		listFor := func(domainIDs ...uint64) []uint64 {
			items, total, err := repo.List(ctx, do.AuditQuery{
				Page:                  common.PageQuery{Page: 1, PageSize: 50},
				RestrictToDataDomains: true,
				DataDomainIDs:         domainIDs,
			})
			if err != nil {
				t.Fatalf("list audit for domains %v: %v", domainIDs, err)
			}
			if int(total) != len(items) {
				t.Fatalf("total %d != items %d", total, len(items))
			}
			ids := make([]uint64, 0, len(items))
			for _, item := range items {
				ids = append(ids, item.ID)
			}
			return ids
		}

		crossID := logs[1].ID
		contains := func(ids []uint64, target uint64) bool {
			for _, id := range ids {
				if id == target {
					return true
				}
			}
			return false
		}

		if ids := listFor(academic.ID); contains(ids, crossID) {
			t.Errorf("single-sided target domain admin must not read cross-domain correction event: %v", ids)
		}
		if ids := listFor(mpa.ID); contains(ids, crossID) {
			t.Errorf("single-sided source domain admin must not read cross-domain correction event: %v", ids)
		}
		if ids := listFor(academic.ID, mpa.ID); !contains(ids, crossID) {
			t.Errorf("dual-sided admin should read cross-domain correction event: %v", ids)
		}
		if ids := listFor(academic.ID); !contains(ids, logs[2].ID) {
			t.Errorf("target domain admin should read same-domain updates: %v", ids)
		}
		if ids := listFor(mpa.ID); !contains(ids, logs[0].ID) {
			t.Errorf("scoped admin should read own-domain updates: %v", ids)
		}

		// 详情接口复用同一过滤条件，同样不能返回单侧不可见的更正事件。
		if _, err := repo.GetByID(ctx, crossID, do.AuditQuery{RestrictToDataDomains: true, DataDomainIDs: []uint64{academic.ID}}); err == nil {
			t.Errorf("detail endpoint leaked cross-domain correction event to a single-sided admin")
		}
		if _, err := repo.GetByID(ctx, crossID, do.AuditQuery{RestrictToDataDomains: true, DataDomainIDs: []uint64{mpa.ID, academic.ID}}); err != nil {
			t.Errorf("dual-sided admin should read correction detail, got %v", err)
		}

		return rollback
	})
	if err != rollback {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

var errRollbackAuditVisibility = errAuditVisibilityRollback("rollback audit visibility test")

type errAuditVisibilityRollback string

func (e errAuditVisibilityRollback) Error() string { return string(e) }
