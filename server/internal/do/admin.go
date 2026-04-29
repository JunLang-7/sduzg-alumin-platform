package do

import (
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
)

type AdminListQuery struct {
	Page common.PageQuery
}

func (q AdminListQuery) Normalize() AdminListQuery {
	q.Page = q.Page.Normalize()
	return q
}

type AdminCreateProfile struct {
	Account  string
	RealName *string
	Mobile   *string
}

func (p AdminCreateProfile) Normalize() AdminCreateProfile {
	p.Account = strings.TrimSpace(p.Account)
	p.RealName = trimEmptyStringPointer(p.RealName)
	p.Mobile = trimEmptyStringPointer(p.Mobile)
	return p
}
