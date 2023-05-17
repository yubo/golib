package api

import (
	"strings"

	"github.com/yubo/golib/util"
)

var (
	maxLimitPage = 500
	defLimitPage = 10
)

func GetLimit(limit int) int {
	if limit <= 0 {
		return defLimitPage
	}

	if limit > maxLimitPage {
		return maxLimitPage
	}

	return limit
}

func SetLimitPage(def, max int) {
	if def > 0 {
		defLimitPage = def
	}
	if max > 0 {
		maxLimitPage = max
	}
}

// deprecated
// GetListOptions provides the options that may be provided for storage list operations.
type GetListOptions struct {
	Query   string
	Orderby []string
	Offset  int
	Limit   int

	// for output count(*)
	Total *int
}

type PageParams struct {
	Offset   int    `param:"query,hidden" description:"offset, priority is more than currentPage"`
	Limit    int    `param:"query,hidden" description:"limit, priority is more than pageSize"`
	PageSize int    `param:"query" description:"page size" default:"10" maximum:"500"`
	Current  int    `param:"query" description:"current page number, start at 1(defualt)" default:"1"`
	Sorter   string `param:"query" description:"column name"`
	Order    string `param:"query" description:"asc(default)/desc" enum:"asc|desc"`
	Dump     bool   `param:"query,hidden" description:""`
}

// TODO: validate query
func (p PageParams) GetListOptions(query string, total *int, orders ...string) (*GetListOptions, error) {
	offset, limit := p.OffsetLimit()
	if sorter := util.SnakeCasedName(p.Sorter); sorter != "" {
		orders = append([]string{"`" + sorter + "` " +
			sqlOrder(p.Order)}, orders...)
	}
	return &GetListOptions{
		Query:   strings.Trim(query, ","),
		Offset:  offset,
		Limit:   limit,
		Total:   total,
		Orderby: orders,
	}, nil
}

func (p *PageParams) GetPageSize() int {
	if p.PageSize == 0 {
		p.PageSize = defLimitPage
	}
	return p.PageSize
}

func (p *PageParams) GetCurPage() int {
	if p.Current == 0 {
		p.Current = 1
	}
	return p.Current
}

func (p *PageParams) OffsetLimit() (offset, limit int) {
	limit = p.Limit

	if limit == 0 {
		limit = p.PageSize
	}

	if limit == 0 {
		limit = defLimitPage
	}

	if limit > maxLimitPage {
		limit = maxLimitPage
	}

	offset = p.Offset

	if offset <= 0 {
		offset = (p.Current - 1) * limit
	}

	if offset < 0 {
		offset = 0
	}

	return
}

func sqlOrder(order string) string {
	switch strings.ToLower(order) {
	case "ascend", "asc":
		return "ASC"
	case "descend", "desc":
		return "DESC"
	default:
		return "ASC"
	}
}
