package dto

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
)

// jsonShape 返回结构体的 JSON 字段序列，形如 "items:[]dto.AlumniListItem"。
// 只比较字段名与数组层级，不比较具体类型名，因为文档类型与运行时类型的元素名相同。
func jsonShape(t *testing.T, value any) []string {
	t.Helper()

	typ := reflect.TypeOf(value)
	fields := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			t.Fatalf("字段 %s.%s 缺少可用的 json tag", typ.Name(), field.Name)
		}

		shape := field.Type.Kind().String()
		if field.Type.Kind() == reflect.Slice {
			shape = "[]" + field.Type.Elem().Kind().String()
		}
		fields = append(fields, name+":"+shape)
	}

	return fields
}

func assertSameShape(t *testing.T, docType, runtimeType any) {
	t.Helper()

	docShape := jsonShape(t, docType)
	runtimeShape := jsonShape(t, runtimeType)
	if !reflect.DeepEqual(docShape, runtimeShape) {
		t.Fatalf("文档类型与运行时分页结构不一致：\n文档:   %v\n运行时: %v", docShape, runtimeShape)
	}
}

// TestPaginatedSchemasMatchPager 保证接口文档里的分页响应与运行时真实返回的
// common.Pager[T] 结构一致；common.Pager 变更而文档类型未同步时此测试失败。
func TestPaginatedSchemasMatchPager(t *testing.T) {
	assertSameShape(t, AlumniListItemPage{}, common.Pager[AlumniListItem]{})
	assertSameShape(t, AdminListItemPage{}, common.Pager[AdminListItem]{})
	assertSameShape(t, AuditOperationPage{}, common.Pager[AuditOperation]{})
}
