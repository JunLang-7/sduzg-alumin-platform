package handler

import (
	"context"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

// stubAlumniStore 是 handler 测试用的空仓储，具体用例通过嵌入后覆盖所需方法。
type stubAlumniStore struct{}

func (stubAlumniStore) List(context.Context, do.AlumniListQuery) ([]do.AlumniRecord, int64, error) {
	return nil, 0, nil
}
func (stubAlumniStore) FindOnly(context.Context, do.AlumniListQuery) ([]do.AlumniRecord, error) {
	return nil, nil
}
func (stubAlumniStore) ListAll(context.Context, do.AlumniListQuery) ([]do.AlumniExportRow, error) {
	return nil, nil
}
func (stubAlumniStore) CountActive(context.Context) (int64, error) { return 0, nil }
func (stubAlumniStore) GetDetail(context.Context, uint64, []uint64) (*do.AlumniRecord, error) {
	return nil, common.ErrAlumniNotFound
}
func (stubAlumniStore) GetPersonalProfile(context.Context, uint64) (*model.AlumniProfile, error) {
	return nil, common.ErrAlumniNotFound
}
func (stubAlumniStore) GetEnrollment(context.Context, uint64, uint64, []uint64) (*model.AlumniEnrollment, error) {
	return nil, common.ErrEnrollmentNotFound
}
func (stubAlumniStore) Create(context.Context, do.AlumniCreateInput, uint64) (*do.AlumniRecord, error) {
	return nil, common.ErrInvalidRequest
}
func (stubAlumniStore) BatchCreate(context.Context, []do.AlumniCreateInput, uint64) ([]uint64, error) {
	return nil, nil
}
func (stubAlumniStore) Save(context.Context, uint64, do.AlumniSaveInput, uint64, []uint64) (*do.AlumniSaveResult, error) {
	return nil, common.ErrAlumniNotFound
}
func (stubAlumniStore) Delete(context.Context, uint64, uint64, []uint64) error { return nil }
func (stubAlumniStore) UpdateEditableFields(context.Context, uint64, uint64, do.AlumniEditableProfile) error {
	return nil
}
func (stubAlumniStore) FindExistingByDedupKey(context.Context, []do.AlumniDedupKey) (map[string]bool, error) {
	return make(map[string]bool), nil
}
func (stubAlumniStore) FindByMobile(context.Context, string) (*model.AlumniProfile, error) {
	return nil, common.ErrAlumniNotFound
}
func (stubAlumniStore) FindByEmail(context.Context, string) (*model.AlumniProfile, error) {
	return nil, common.ErrAlumniNotFound
}
func (stubAlumniStore) UpdateMobile(context.Context, uint64, string) error { return nil }
func (stubAlumniStore) UpdateEmail(context.Context, uint64, string) error  { return nil }
