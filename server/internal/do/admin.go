package do

import "github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"

type AdminListQuery struct {
	Page common.PageQuery
}

func (q AdminListQuery) Normalize() AdminListQuery {
	q.Page = q.Page.Normalize()
	return q
}
