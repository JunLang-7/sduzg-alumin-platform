package dto

import (
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
)

type AdminListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

func (r AdminListRequest) ToQuery() do.AdminListQuery {
	return do.AdminListQuery{
		Page: common.PageQuery{
			Page:     r.Page,
			PageSize: r.PageSize,
		},
	}
}

type AdminListItem struct {
	ID          uint64     `json:"id"`
	Account     string     `json:"account"`
	Role        string     `json:"role"`
	RealName    *string    `json:"real_name"`
	Mobile      *string    `json:"mobile"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
