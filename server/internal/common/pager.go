package common

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type PageQuery struct {
	Page     int
	PageSize int
}

func (q PageQuery) Normalize() PageQuery {
	if q.Page <= 0 {
		q.Page = DefaultPage
	}
	if q.PageSize <= 0 {
		q.PageSize = DefaultPageSize
	}
	if q.PageSize > MaxPageSize {
		q.PageSize = MaxPageSize
	}
	return q
}

func (q PageQuery) Offset() int {
	q = q.Normalize()
	return (q.Page - 1) * q.PageSize
}

type Pager[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

func NewPager[T any](items []T, query PageQuery, total int64) Pager[T] {
	query = query.Normalize()
	if items == nil {
		items = make([]T, 0)
	}

	return Pager[T]{
		Items:    items,
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    total,
	}
}
