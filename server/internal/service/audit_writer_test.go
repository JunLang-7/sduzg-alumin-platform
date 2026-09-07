package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

func TestProfileAuditChangesSupportsCreateUpdateDelete(t *testing.T) {
	oldUnit := "山东某单位"
	newUnit := "济南某单位"
	before := &model.AlumniProfile{
		ID:       42,
		Name:     "张三",
		Grade:    "2020级",
		WorkUnit: &oldUnit,
	}
	after := &model.AlumniProfile{
		ID:       42,
		Name:     "张三",
		Grade:    "2020级",
		WorkUnit: &newUnit,
	}

	tests := []struct {
		name       string
		action     string
		before     *model.AlumniProfile
		after      *model.AlumniProfile
		wantFields []string
	}{
		{name: "create", action: AuditActionCreate, after: after, wantFields: []string{"name", "grade", "work_unit"}},
		{name: "update", action: AuditActionUpdate, before: before, after: after, wantFields: []string{"work_unit"}},
		{name: "delete", action: AuditActionDelete, before: before, wantFields: []string{"status"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAuditAction(tt.action); got != tt.action {
				t.Fatalf("normalizeAuditAction(%q) = %q, want %q", tt.action, got, tt.action)
			}
			changes := profileAuditChanges(tt.action, tt.before, tt.after)
			if len(changes) != len(tt.wantFields) {
				t.Fatalf("profileAuditChanges() returned %d changes, want %d: %+v", len(changes), len(tt.wantFields), changes)
			}
			for i, field := range tt.wantFields {
				if changes[i].FieldName != field {
					t.Fatalf("change %d field = %q, want %q", i, changes[i].FieldName, field)
				}
			}
		})
	}
}

func TestNormalizeAuditActionRejectsUnknownOperationType(t *testing.T) {
	if got := normalizeAuditAction("rewrite"); got != "" {
		t.Fatalf("normalizeAuditAction(unknown) = %q, want empty string", got)
	}
}

func TestProfileAuditChangesMasksSensitiveValues(t *testing.T) {
	oldMobile := "13800000000"
	newMobile := "13900000000"
	oldEmail := "old@example.com"
	newEmail := "new@example.com"
	oldAddress := "旧地址"
	newAddress := "新地址"
	changes := profileAuditChanges(AuditActionUpdate,
		&model.AlumniProfile{Mobile: &oldMobile, Email: &oldEmail, MailingAddress: &oldAddress},
		&model.AlumniProfile{Mobile: &newMobile, Email: &newEmail, MailingAddress: &newAddress},
	)

	payload, err := json.Marshal(changes)
	if err != nil {
		t.Fatalf("marshal changes: %v", err)
	}
	text := string(payload)
	for _, raw := range []string{oldMobile, newMobile, oldEmail, newEmail, oldAddress, newAddress} {
		if strings.Contains(text, raw) {
			t.Fatalf("sensitive raw value %q leaked in audit diff: %s", raw, text)
		}
	}
	if len(changes) != 3 {
		t.Fatalf("sensitive changes = %d, want 3", len(changes))
	}
	for _, change := range changes {
		if !change.Sensitive {
			t.Errorf("field %q is not marked sensitive", change.FieldName)
		}
		if change.OldValue != "已填写" || change.NewValue != "已修改" {
			t.Errorf("field %q transition = %q -> %q, want filled -> modified", change.FieldName, change.OldValue, change.NewValue)
		}
	}
}
