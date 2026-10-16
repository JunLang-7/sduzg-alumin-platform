package dto

// 以下分页响应类型只用于接口文档。
//
// swag 稳定版无法在注解里表达 common.Pager[T] 这类泛型实例化，因此这里按相同字段
// 重复声明分页响应。page_shape_test.go 会断言它们与 common.Pager[T] 的字段结构一致，
// 修改 common.Pager 时若不同步更新这里，测试会失败。

// AlumniListItemPage 是校友档案列表的分页响应。
type AlumniListItemPage struct {
	Items    []AlumniListItem `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}

// AdminListItemPage 是管理员账号列表的分页响应。
type AdminListItemPage struct {
	Items    []AdminListItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
}

// AuditOperationPage 是操作历史列表的分页响应。
type AuditOperationPage struct {
	Items    []AuditOperation `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}
