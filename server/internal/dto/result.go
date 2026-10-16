package dto

// DeletedResult 是删除类接口的成功响应。
type DeletedResult struct {
	Deleted bool `json:"deleted" example:"true"`
}

// UpdatedResult 是更新类接口的成功响应。
type UpdatedResult struct {
	Updated bool `json:"updated" example:"true"`
}

// ConfirmedResult 是确认类接口的成功响应。
type ConfirmedResult struct {
	Confirmed bool `json:"confirmed" example:"true"`
}
