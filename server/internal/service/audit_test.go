package service

import "testing"

func TestMapAuditChangeMasksSensitiveValues(t *testing.T) {
	change := mapAuditChange(AuditChange{
		FieldName:  "mobile",
		FieldLabel: "手机号",
		OldValue:   "13812345678",
		NewValue:   "13987654321",
	})
	if !change.Sensitive {
		t.Fatal("expected mobile change to be marked sensitive")
	}
	if change.OldValue != "已填写" || change.NewValue != "已修改" || change.CurrentValue != "已修改" {
		t.Fatalf("unexpected masked values: %+v", change)
	}
}

func TestMapAuditChangePreservesNonSensitiveValues(t *testing.T) {
	change := mapAuditChange(AuditChange{
		FieldName:    "work_unit",
		FieldLabel:   "工作单位",
		OldValue:     "山东某单位",
		NewValue:     "济南某单位",
		CurrentValue: "济南某单位",
	})
	if change.Sensitive {
		t.Fatal("did not expect work unit change to be marked sensitive")
	}
	if change.OldValue != "山东某单位" || change.NewValue != "济南某单位" {
		t.Fatalf("unexpected non-sensitive values: %+v", change)
	}
}
