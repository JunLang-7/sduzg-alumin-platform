package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/xuri/excelize/v2"
)

func buildXLSXReader(headers []string, rows [][]string) (*bytes.Reader, error) {
	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, err
	}

	headerVals := make([]any, len(headers))
	for i, h := range headers {
		headerVals[i] = h
	}
	if err := sw.SetRow("A1", headerVals); err != nil {
		return nil, err
	}

	for i, row := range rows {
		vals := make([]any, len(row))
		for j, v := range row {
			vals[j] = v
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := sw.SetRow(cell, vals); err != nil {
			return nil, err
		}
	}

	if err := sw.Flush(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func importAsSuperAdmin(svc *AlumniService, reader *bytes.Reader) (*dto.AlumniImportResult, error) {
	domainID := uint64(1)
	return svc.Import(context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, &domainID, reader)
}

type fakeImportDataDomainStore struct {
	domains []*model.DataDomain
}

func (s *fakeImportDataDomainStore) ListActiveDataDomains(context.Context) ([]*model.DataDomain, error) {
	return s.domains, nil
}

func (s *fakeImportDataDomainStore) ListDataDomains(context.Context) ([]*model.DataDomain, error) {
	return s.domains, nil
}

func (s *fakeImportDataDomainStore) ListAdminDataDomainIDs(context.Context, uint64) ([]uint64, error) {
	return nil, nil
}

func (s *fakeImportDataDomainStore) ListAdminPermissionCodes(context.Context, uint64) ([]string, error) {
	return nil, nil
}

func TestImportValidatesDataDomainPerRowForMultiDomainAdmin(t *testing.T) {
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{
		{"本科生校友", "2020级", "", "", "", "", "", "", "", "", "", "", "", "", "", common.DataDomainUndergraduate},
		{"MPA校友", "2020级", "", "", "", "", "", "", "", "", "", "", "", "", "", common.DataDomainMPA},
		{"未知领域", "2020级", "", "", "", "", "", "", "", "", "", "", "", "", "", "unknown"},
		{"空领域", "2020级"},
	})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, &fakeImportDataDomainStore{domains: []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Status: common.DataDomainStatusActive},
		{ID: 2, Code: common.DataDomainMPA, Status: common.DataDomainStatusActive},
	}})

	result, err := svc.Import(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{1, 3}}, nil, reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(result.Errors) != 3 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if len(store.batchProfiles) != 1 || store.batchProfiles[0].DataDomainID == nil || *store.batchProfiles[0].DataDomainID != 1 {
		t.Fatalf("unexpected imported data domains: %+v", store.batchProfiles)
	}
}

func TestImportAllValidRows(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "李老师", "王教授", "公共管理", "非全日制", "政府", "山东大学", "主任", "济南市", "男", "13800000000"},
		{"李四", "2021级", "2021级MPA", "2021", "赵老师", "钱教授", "工商管理", "全日制", "金融", "银行", "经理", "青岛市", "女", "13900000000"},
		{"王五", "2022级", "", "", "", "", "", "", "", "", "", "", "", ""},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	result, err := importAsSuperAdmin(svc, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}
	if result.Success != 3 {
		t.Fatalf("expected success 3, got %d", result.Success)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected 0 errors, got %+v", result.Errors)
	}
}

func TestImportAuditTruncatesBatchRecords(t *testing.T) {
	writer := &fakeExportOperationLogger{}
	svc := NewAlumniService(&fakeAlumniStore{}, nil).WithOperationLogger(writer)
	domainID := uint64(1)
	total := maxAuditBatchRecords + 1
	created := make([]*model.AlumniProfile, 0, total)
	for i := 0; i < total; i++ {
		created = append(created, &model.AlumniProfile{ID: uint64(i + 1), Name: fmt.Sprintf("校友%04d", i+1)})
	}

	svc.writeImportAudit(
		context.Background(),
		common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin},
		[]do.AlumniCreateProfile{{DataDomainID: &domainID}},
		created,
		&dto.AlumniImportResult{Total: total, Success: total},
	)

	if len(writer.logs) != 1 || writer.logs[0].Detail == nil {
		t.Fatalf("expected one import audit log, got %+v", writer.logs)
	}
	var detail importAuditDetail
	if err := json.Unmarshal([]byte(*writer.logs[0].Detail), &detail); err != nil {
		t.Fatalf("parse import audit detail: %v", err)
	}
	if len(detail.BatchCreatedAlumni) != maxAuditBatchRecords {
		t.Fatalf("saved batch records = %d, want %d", len(detail.BatchCreatedAlumni), maxAuditBatchRecords)
	}
	if detail.BatchCreatedAlumniTotal != total || !detail.BatchCreatedAlumniTruncated {
		t.Fatalf("unexpected truncation metadata: total=%d truncated=%v", detail.BatchCreatedAlumniTotal, detail.BatchCreatedAlumniTruncated)
	}
	if len(*writer.logs[0].Detail) > 256*1024 {
		t.Fatalf("import audit detail is too large: %d bytes", len(*writer.logs[0].Detail))
	}
}

func TestImportAuditKeepsBatchRecordsUnderLimit(t *testing.T) {
	writer := &fakeExportOperationLogger{}
	svc := NewAlumniService(&fakeAlumniStore{}, nil).WithOperationLogger(writer)
	domainID := uint64(1)
	created := make([]*model.AlumniProfile, 0, maxAuditBatchRecords)
	for i := 0; i < maxAuditBatchRecords; i++ {
		created = append(created, &model.AlumniProfile{ID: uint64(i + 1), Name: fmt.Sprintf("校友%04d", i+1)})
	}

	svc.writeImportAudit(
		context.Background(),
		common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin},
		[]do.AlumniCreateProfile{{DataDomainID: &domainID}},
		created,
		&dto.AlumniImportResult{Total: maxAuditBatchRecords, Success: maxAuditBatchRecords},
	)

	if len(writer.logs) != 1 || writer.logs[0].Detail == nil {
		t.Fatalf("expected one import audit log, got %+v", writer.logs)
	}
	var detail importAuditDetail
	if err := json.Unmarshal([]byte(*writer.logs[0].Detail), &detail); err != nil {
		t.Fatalf("parse import audit detail: %v", err)
	}
	if len(detail.BatchCreatedAlumni) != maxAuditBatchRecords || detail.BatchCreatedAlumniTruncated {
		t.Fatalf("under-limit import should keep all records: %+v", detail)
	}
}

func TestImportWritesAuditWithoutProfileValues(t *testing.T) {
	reader, err := buildXLSXReader(alumniColumnHeaders, [][]string{{"张三", "2020级", "", "", "", "", "", "", "", "山东大学"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	writer := &fakeExportOperationLogger{}
	domainID := uint64(1)
	svc := NewAlumniService(&fakeAlumniStore{}, nil).WithOperationLogger(writer)

	result, err := svc.Import(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, &domainID, reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(writer.logs) != 1 {
		t.Fatalf("expected one successful import audit, result=%+v logs=%+v", result, writer.logs)
	}
	log := writer.logs[0]
	if log.Action != AuditActionImport || log.TargetType != auditTargetBatch || log.OperatorID != 7 || log.OperatorRole != common.RoleSuperAdmin || log.Detail == nil {
		t.Fatalf("unexpected import audit log: %+v", log)
	}
	if !strings.Contains(*log.Detail, "张三") {
		t.Fatalf("import audit detail does not contain the created alumni name: %s", *log.Detail)
	}
	if strings.Contains(*log.Detail, "山东大学") {
		t.Fatalf("import audit detail contains sensitive profile value: %s", *log.Detail)
	}
	var detail importAuditDetail
	if err := json.Unmarshal([]byte(*log.Detail), &detail); err != nil {
		t.Fatalf("parse import audit detail: %v", err)
	}
	if detail.Total != 1 || detail.Success != 1 || detail.Failed != 0 || len(detail.DataDomainIDs) != 1 || detail.DataDomainIDs[0] != domainID || !detail.SensitiveFieldsIncluded {
		t.Fatalf("unexpected import audit detail: %+v", detail)
	}
	if len(detail.BatchCreatedAlumni) != 1 || detail.BatchCreatedAlumni[0].Name != "张三" {
		t.Fatalf("expected created alumni in import detail: %+v", detail.BatchCreatedAlumni)
	}
	if _, ok := detail.BatchCreatedAlumni[0].FieldValues["work_unit"]; ok {
		t.Fatalf("sensitive work unit was persisted in import detail: %+v", detail.BatchCreatedAlumni[0].FieldValues)
	}
}

func TestImportForcesAssignedDataDomainAndReportsUnauthorizedSensitiveWrite(t *testing.T) {
	reader, err := buildXLSXReader(alumniColumnHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)
	assignedDomainID := uint64(2)
	_, err = svc.Import(context.Background(), common.AccessContext{
		UserID:    7,
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{assignedDomainID},
	}, nil, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if len(store.batchProfiles) != 1 || store.batchProfiles[0].DataDomainID == nil || *store.batchProfiles[0].DataDomainID != assignedDomainID {
		t.Fatalf("expected imported profile to use assigned domain %d, got %+v", assignedDomainID, store.batchProfiles)
	}

	reader, err = buildXLSXReader(alumniColumnHeaders, [][]string{{"李四", "2020级", "", "", "", "", "", "", "", "", "主任"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	result, err := svc.Import(context.Background(), common.AccessContext{
		UserID:    7,
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{assignedDomainID},
	}, nil, reader)
	if err != nil {
		t.Fatalf("expected partial import result, got %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 || result.Errors[0].Row != 2 || result.Errors[0].Message != "无权导入敏感字段" {
		t.Fatalf("expected sensitive row error, got %+v", result)
	}
}

func TestImportKeepsRowsWithDifferentMobile(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13900000000"},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	result, err := importAsSuperAdmin(svc, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 2 {
		t.Fatalf("expected success 2 for different mobile values, got %d with errors %+v", result.Success, result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no duplicate errors, got %+v", result.Errors)
	}
}

func TestImportDeduplicatesRowsWithSameMobile(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	result, err := importAsSuperAdmin(svc, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1 for duplicate mobile values, got %d with errors %+v", result.Success, result.Errors)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 duplicate error, got %+v", result.Errors)
	}
}

func TestImportDeduplicatesRowsWithPaddedMobile(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", " 13800000000 "},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	result, err := importAsSuperAdmin(svc, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1 for padded duplicate mobile values, got %d with errors %+v", result.Success, result.Errors)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 duplicate error, got %+v", result.Errors)
	}
}

func TestImportDeduplicatesRowsWithEmptyMobile(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", ""},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", " "},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	result, err := importAsSuperAdmin(svc, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1 for empty duplicate mobile values, got %d with errors %+v", result.Success, result.Errors)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 duplicate error, got %+v", result.Errors)
	}
}

func TestImportPartialErrors(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级"},
		{"", "2021级"},
		{"王五", ""},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	result, err := importAsSuperAdmin(svc, reader)
	if err != nil {
		t.Fatalf("expected import success (partial), got %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1, got %d", result.Success)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(result.Errors))
	}
}

func TestImportEmptyFile(t *testing.T) {
	headers := alumniColumnHeaders
	reader, err := buildXLSXReader(headers, nil)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	_, err = importAsSuperAdmin(svc, reader)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestImportRejectsMalformedWorkbook(t *testing.T) {
	// A ZIP signature alone must not be treated as a valid workbook. This also
	// guards the import path against malformed uploaded spreadsheet data.
	reader := bytes.NewReader([]byte{'P', 'K', 0x03, 0x04, 0xff, 0x00, 0x00})

	_, err := importAsSuperAdmin(NewAlumniService(&fakeAlumniStore{}, nil), reader)
	if err != common.ErrInvalidRequest {
		t.Fatalf("expected invalid request for malformed workbook, got %v", err)
	}
}

func TestImportHeaderMismatch(t *testing.T) {
	badHeaders := []string{"名称", "年级"}
	reader, err := buildXLSXReader(badHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	_, err = importAsSuperAdmin(svc, reader)
	if err == nil {
		t.Fatal("expected error for header mismatch")
	}
}

func TestImportDatabaseUnavailable(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{{"张三", "2020级"}}
	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	svc := NewAlumniService(nil, nil)

	_, err = importAsSuperAdmin(svc, reader)
	if err != common.ErrDatabaseUnavailable {
		t.Fatalf("expected database unavailable, got %v", err)
	}
}

func TestParseRowToProfile(t *testing.T) {
	row := []string{"张三", "2020级", "2020级MPA", "2020", "李老师", "王教授", "公共管理", "非全日制", "政府", "山东大学", "主任", "济南市", "男", "13800000000"}
	profile := parseRowToProfile(row)
	profile = profile.Normalize()

	if profile.Name != "张三" {
		t.Fatalf("expected Name 张三, got %q", profile.Name)
	}
	if profile.Grade != "2020级" {
		t.Fatalf("expected Grade 2020级, got %q", profile.Grade)
	}
	if profile.ClassName == nil || *profile.ClassName != "2020级MPA" {
		t.Fatalf("expected ClassName 2020级MPA, got %v", profile.ClassName)
	}
	if profile.Mobile == nil || *profile.Mobile != "13800000000" {
		t.Fatalf("expected Mobile 13800000000, got %v", profile.Mobile)
	}
	if profile.Status != common.AlumniStatusActive {
		t.Fatalf("expected Status active, got %q", profile.Status)
	}
}

func TestParseRowToProfileEmptyOptionalFields(t *testing.T) {
	row := []string{"王五", "2022级"}
	profile := parseRowToProfile(row)
	profile = profile.Normalize()

	if profile.Name != "王五" {
		t.Fatalf("expected Name 王五, got %q", profile.Name)
	}
	if profile.ClassName != nil {
		t.Fatalf("expected nil ClassName, got %v", *profile.ClassName)
	}
	if profile.Mobile != nil {
		t.Fatalf("expected nil Mobile, got %v", *profile.Mobile)
	}
}

func TestParseRowToProfileShortRow(t *testing.T) {
	row := []string{"测试"}
	profile := parseRowToProfile(row)
	profile = profile.Normalize()

	if profile.Name != "测试" {
		t.Fatalf("expected Name 测试, got %q", profile.Name)
	}
	if profile.Grade != "" {
		t.Fatalf("expected empty Grade, got %q", profile.Grade)
	}
}

func TestImportResultErrorsIncludeRowNumbers(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级"},
		{"", "2021级"},
		{"李四", ""},
		{"王五", ""},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	result, err := importAsSuperAdmin(svc, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}

	if len(result.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(result.Errors))
	}
	if result.Errors[0].Row != 3 {
		t.Fatalf("expected first error at row 3, got %d", result.Errors[0].Row)
	}
	if result.Errors[1].Row != 4 {
		t.Fatalf("expected second error at row 4, got %d", result.Errors[1].Row)
	}
	if result.Errors[2].Row != 5 {
		t.Fatalf("expected third error at row 5, got %d", result.Errors[2].Row)
	}
}

func TestImportAcceptsDataDomainCodeNameAndID(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{name: "code", value: "mpa"},
		{name: "code is case insensitive", value: "MPA"},
		{name: "display name", value: "MPA专业学位研究生"},
		{name: "numeric id", value: "2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := make([]string, len(alumniImportColumnHeaders))
			row[0] = "张三"
			row[1] = "2020级"
			row[len(row)-1] = tc.value
			reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{row})
			if err != nil {
				t.Fatalf("build xlsx: %v", err)
			}
			store := &fakeAlumniStore{}
			result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
				context.Background(),
				common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
				nil,
				reader,
			)
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			if result.Success != 1 || len(store.batchProfiles) != 1 {
				t.Fatalf("unexpected import result: %+v", result)
			}
			domain := store.batchProfiles[0].DataDomainID
			if domain == nil || *domain != 2 {
				t.Fatalf("data domain = %v, want 2", domain)
			}
		})
	}
}

func TestImportInvalidDataDomainListsOptions(t *testing.T) {
	row := make([]string, len(alumniImportColumnHeaders))
	row[0] = "张三"
	row[1] = "2020级"
	row[len(row)-1] = "unknown"
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{row})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	result, err := NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		reader,
	)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	message := result.Errors[0].Message
	if !strings.Contains(message, "可选值") || !strings.Contains(message, "undergraduate（本科生）") || !strings.Contains(message, "mpa（MPA专业学位研究生）") {
		t.Fatalf("expected actionable domain error, got %q", message)
	}
}

func TestImportRequiresDomainColumnWithoutTargetDomain(t *testing.T) {
	reader, err := buildXLSXReader(alumniColumnHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	_, err = NewAlumniService(&fakeAlumniStore{}, nil).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		reader,
	)
	if !errors.Is(err, common.ErrImportDomainRequired) {
		t.Fatalf("Import() error = %v, want ErrImportDomainRequired", err)
	}
}

func TestImportTemplateExplainsDomainsAndExampleIsImportable(t *testing.T) {
	template, err := buildTemplateXLSX()
	if err != nil {
		t.Fatalf("build template: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(template.Data))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("read data sheet: %v", err)
	}
	if len(rows) != 1 || len(rows[0]) != len(alumniImportColumnHeaders) {
		t.Fatalf("unexpected template data sheet: %d rows", len(rows))
	}
	if rows[0][len(rows[0])-1] != "所属领域" {
		t.Fatalf("expected 所属领域 header, got %q", rows[0][len(rows[0])-1])
	}

	notes, err := f.GetRows("填写说明")
	if err != nil {
		t.Fatalf("read note sheet: %v", err)
	}
	var noteText strings.Builder
	for _, row := range notes {
		noteText.WriteString(strings.Join(row, " "))
		noteText.WriteString("\n")
	}
	for _, want := range []string{"undergraduate", "academic_graduate", "mpa", "MPA专业学位研究生", "示例校友", "2024级", "数字 ID"} {
		if !strings.Contains(noteText.String(), want) {
			t.Fatalf("template guide missing %q: %s", want, noteText.String())
		}
	}

	row := make([]string, len(alumniImportColumnHeaders))
	row[0] = "示例校友"
	row[1] = "2024级"
	row[len(row)-1] = "mpa"
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{row})
	if err != nil {
		t.Fatalf("build example xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		reader,
	)
	if err != nil {
		t.Fatalf("example row import failed: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("example row should import successfully: %+v", result)
	}
}

func TestImportTemplateBlankRowIsRejected(t *testing.T) {
	template, err := buildTemplateXLSX()
	if err != nil {
		t.Fatalf("build template: %v", err)
	}
	_, err = NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		bytes.NewReader(template.Data),
	)
	if !errors.Is(err, common.ErrImportNoDataRows) {
		t.Fatalf("Import() error = %v, want ErrImportNoDataRows", err)
	}
}

func TestSingleDomainAdminRejectsCrossDomainRows(t *testing.T) {
	row := make([]string, len(alumniImportColumnHeaders))
	row[0] = "跨域校友"
	row[1] = "2020级"
	row[len(row)-1] = "mpa"
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{row})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{1}},
		nil,
		reader,
	)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 {
		t.Fatalf("cross-domain row should be rejected, got %+v", result)
	}
	if !strings.Contains(result.Errors[0].Message, "不一致") {
		t.Fatalf("expected mismatch message, got %q", result.Errors[0].Message)
	}
	if len(store.batchProfiles) != 0 {
		t.Fatalf("cross-domain row must not be created: %+v", store.batchProfiles)
	}
}

func TestSingleDomainAdminCanImportMatchingDomainRows(t *testing.T) {
	row := make([]string, len(alumniImportColumnHeaders))
	row[0] = "MPA校友"
	row[1] = "2020级"
	row[len(row)-1] = "mpa"
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{row})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{2}},
		nil,
		reader,
	)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(store.batchProfiles) != 1 {
		t.Fatalf("matching row should import, got %+v", result)
	}
	if domain := store.batchProfiles[0].DataDomainID; domain == nil || *domain != 2 {
		t.Fatalf("data domain = %v, want 2", domain)
	}
}

func TestImportRejectsDisabledDataDomain(t *testing.T) {
	domains := &fakeImportDataDomainStore{domains: []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Name: "本科生", Status: common.DataDomainStatusActive},
		{ID: 3, Code: "legacy_program", Name: "历史项目", Status: common.DataDomainStatusDisabled},
	}}
	row := make([]string, len(alumniImportColumnHeaders))
	row[0] = "历史校友"
	row[1] = "2010级"
	row[len(row)-1] = "legacy_program"
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{row})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	result, err := NewAlumniService(&fakeAlumniStore{}, nil, domains).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		reader,
	)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if !strings.Contains(result.Errors[0].Message, "已停用") {
		t.Fatalf("expected disabled domain message, got %q", result.Errors[0].Message)
	}
}

func TestSingleDomainAdminAllowsBlankDomain(t *testing.T) {
	row := make([]string, len(alumniImportColumnHeaders))
	row[0] = "空白领域校友"
	row[1] = "2020级"
	row[len(row)-1] = ""
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{row})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{1}},
		nil,
		reader,
	)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(store.batchProfiles) != 1 {
		t.Fatalf("blank domain should use the only authorized domain, got %+v", result)
	}
	if domain := store.batchProfiles[0].DataDomainID; domain == nil || *domain != 1 {
		t.Fatalf("data domain = %v, want 1", domain)
	}
}

func TestSingleDomainAdminRejectsExplicitUnauthorizedDomain(t *testing.T) {
	reader, err := buildXLSXReader(alumniColumnHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	otherDomainID := uint64(2)
	_, err = NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{1}},
		&otherDomainID,
		reader,
	)
	if !errors.Is(err, common.ErrPermissionDenied) {
		t.Fatalf("Import() error = %v, want ErrPermissionDenied", err)
	}
}

func TestImportRestoresFormulaGuardPrefixes(t *testing.T) {
	cases := map[string]string{
		"'=SUM(A1:A2)":    "=SUM(A1:A2)",
		"'+8613800000000": "+8613800000000",
		"'-1":             "-1",
		"'@user":          "@user",
		"'normal":         "'normal",
	}
	var b strings.Builder
	b.WriteString(strings.Join(alumniImportColumnHeaders, ","))
	b.WriteString("\n")
	for name := range cases {
		row := make([]string, len(alumniImportColumnHeaders))
		row[0] = name
		row[1] = "2020级"
		row[len(row)-1] = "undergraduate"
		b.WriteString(strings.Join(row, ","))
		b.WriteString("\n")
	}

	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		strings.NewReader(b.String()),
	)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != len(cases) || len(store.batchProfiles) != len(cases) {
		t.Fatalf("unexpected import result: %+v", result)
	}
	got := make(map[string]bool, len(store.batchProfiles))
	for _, profile := range store.batchProfiles {
		got[profile.Name] = true
	}
	for input, want := range cases {
		if !got[want] {
			t.Fatalf("input %q was not restored to %q; got %v", input, want, got)
		}
	}
}

func TestImportCSVRowLimit(t *testing.T) {
	row := make([]string, len(alumniImportColumnHeaders))
	row[0] = "张三"
	row[1] = "2020级"
	row[len(row)-1] = "undergraduate"
	line := strings.Join(row, ",") + "\n"

	var b strings.Builder
	b.WriteString(strings.Join(alumniImportColumnHeaders, ","))
	b.WriteString("\n")
	for i := 0; i < maxImportDataRows+1; i++ {
		b.WriteString(line)
	}

	_, err := NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		strings.NewReader(b.String()),
	)
	if !errors.Is(err, common.ErrImportRowLimit) {
		t.Fatalf("Import() error = %v, want ErrImportRowLimit", err)
	}
}

func TestImportInvalidHeaderReturnsBadRequestError(t *testing.T) {
	_, err := NewAlumniService(&fakeAlumniStore{}, nil).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		strings.NewReader("foo,bar\n1,2\n"),
	)
	if !errors.Is(err, common.ErrImportInvalidHeader) {
		t.Fatalf("Import() error = %v, want ErrImportInvalidHeader", err)
	}
}
