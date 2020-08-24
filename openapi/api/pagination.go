package api

import (
	"fmt"
	"strings"

	"github.com/yubo/golib/util"
)

var (
	maxLimitPage = 500
	defLimitPage = 10
)

// Pagination2: use offset,limit direct
type Pagination2 struct {
	Dump   *bool   `param:"-" flags:"dump,," description:"dump list without pagination"`
	Offset *int    `param:"query" flags:"-" description:"offset number"`
	Limit  *int    `param:"query" flags:"-" description:"limit number"`
	Sorter *string `param:"query" flags:"-" description:"column name"`
	Order  *string `param:"query" flags:"-" description:"asc(default)/desc"`
}

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

func (p Pagination2) SqlExtra(orders ...string) string {
	limit, offset := func() (int, int) {
		limit := util.IntValue(p.Limit)

		if limit == 0 {
			limit = defLimitPage
		}

		if limit > maxLimitPage {
			limit = maxLimitPage
		}

		offset := util.IntValue(p.Offset)
		if offset <= 0 {
			offset = 0
		}

		return offset, limit
	}()

	var order string
	if sorter := util.SnakeCasedName(util.StringValue(p.Sorter)); sorter != "" {
		orders = append([]string{"`" + sorter + "` " +
			sqlOrder(util.StringValue(p.Order))}, orders...)
	}

	if len(orders) > 0 {
		order = " order by " + strings.Join(orders, ", ")
	}

	return fmt.Sprintf(order+" limit %d, %d", limit, offset)
}

type Pagination struct {
	PageSize    *int    `param:"query" flags:"-" description:"page size"`
	CurrentPage *int    `param:"query" flags:"-" description:"current page number"`
	Sorter      *string `param:"query" flags:"-" description:"column name"`
	Order       *string `param:"query" flags:"-" description:"asc(default)/desc"`
}

type Resp struct {
	Error string `json:"err" description:"error msg"`
}

type RespTotal struct {
	Total int64 `json:"total" description:"total number"`
}

func (p *Pagination) OffsetLimit() (int, int) {
	limit := util.IntValue(p.PageSize)

	if limit == 0 {
		limit = defLimitPage
	}

	if limit > maxLimitPage {
		limit = maxLimitPage
	}

	currentPage := util.IntValue(p.CurrentPage)
	if currentPage <= 1 {
		return 0, limit
	}

	return (currentPage - 1) * limit, limit
}

func (p Pagination) SqlExtra(orders ...string) string {
	offset, limit := p.OffsetLimit()

	var order string
	if sorter := util.SnakeCasedName(util.StringValue(p.Sorter)); sorter != "" {
		orders = append([]string{"`" + sorter + "` " +
			sqlOrder(util.StringValue(p.Order))}, orders...)
	}

	if len(orders) > 0 {
		order = " order by " + strings.Join(orders, ", ")
	}

	return fmt.Sprintf(order+" limit %d, %d", offset, limit)
}

func sqlOrder(order string) string {
	switch order {
	case "ascend", "asc":
		return "asc"
	case "descend", "desc":
		return "desc"
	default:
		return "asc"
	}
}
