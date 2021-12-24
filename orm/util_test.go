package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/queries"
	"github.com/yubo/golib/util"
)

func TestGenInsertSql(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}
	cases := []struct {
		table  string
		sample *User
		query  string
		args   []interface{}
		isErr  bool
	}{
		{
			isErr: true,
		},
		{
			table:  "user",
			sample: &User{"tom", 14},
			query:  "insert into `user` (`name`, `age`) values (?, ?)",
			args:   []interface{}{"tom", 14},
			isErr:  false,
		},
	}

	for i, c := range cases {
		query, args, err := GenInsertSql(c.table, c.sample)
		assert.Equal(t, c.query, query, "case-%d", i)
		assert.Equal(t, c.args, args, "case-%d", i)
		assert.Equal(t, c.isErr, err != nil, "case-%d", i)

	}
}

func TestGenListSql(t *testing.T) {
	cases := []struct {
		table      string
		cols       []string
		selector   string
		orderby    []string
		offset     *int64
		limit      *int64
		query      string
		queryCount string
		args       []interface{}
		isErr      bool
	}{
		{
			isErr: true,
		},
		{
			table:      "user",
			cols:       []string{"name", "age"},
			selector:   "name=tom",
			orderby:    []string{"name desc"},
			offset:     util.Int64(0),
			limit:      util.Int64(10),
			query:      "select `name`, `age` from `user` where `name` = ? order by name desc limit 0, 10",
			queryCount: "select count(*) from `user` where `name` = ?",
			args:       []interface{}{"tom"},
			isErr:      false,
		},
	}

	for i, c := range cases {
		selector, _ := queries.Parse(c.selector)

		query, queryCount, args, err := GenListSql(c.table, c.cols, selector, c.orderby, c.offset, c.limit)
		assert.Equal(t, c.query, query, "case-%d", i)
		assert.Equal(t, c.queryCount, queryCount, "case-%d", i)
		assert.Equal(t, c.args, args, "case-%d", i)
		assert.Equal(t, c.isErr, err != nil, "case-%d", i)

	}
}

func TestGenGetSql(t *testing.T) {
	cases := []struct {
		table    string
		cols     []string
		selector string
		query    string
		args     []interface{}
		isErr    bool
	}{
		{
			isErr: true,
		},
		{
			table:    "user",
			cols:     []string{"name", "age"},
			selector: "name=tom",
			query:    "select `name`, `age` from `user` where `name` = ?",
			args:     []interface{}{"tom"},
			isErr:    false,
		},
	}

	for i, c := range cases {
		selector, _ := queries.Parse(c.selector)

		query, args, err := GenGetSql(c.table, c.cols, selector)
		assert.Equal(t, c.query, query, "case-%d", i)
		assert.Equal(t, c.args, args, "case-%d", i)
		assert.Equal(t, c.isErr, err != nil, "case-%d", i)

	}
}

func TestGenUpdateSql(t *testing.T) {
	type User struct {
		Name string `sql:",where"`
		Age  int
	}
	cases := []struct {
		table  string
		sample *User
		query  string
		args   []interface{}
		isErr  bool
	}{
		{
			isErr: true,
		},
		{
			table:  "user",
			sample: &User{"tom", 14},
			query:  "update `user` set `age` = ? where `name` = ?",
			args:   []interface{}{14, "tom"},
			isErr:  false,
		},
	}

	for i, c := range cases {
		query, args, err := GenUpdateSql(c.table, c.sample)
		assert.Equal(t, c.query, query, "case-%d", i)
		assert.Equal(t, c.args, args, "case-%d", i)
		assert.Equal(t, c.isErr, err != nil, "case-%d", i)

	}
}

func TestGenDeleteSql(t *testing.T) {
	cases := []struct {
		table    string
		selector string
		query    string
		args     []interface{}
		isErr    bool
	}{
		{
			isErr: true,
		},
		{
			table:    "user",
			selector: "name=tom",
			query:    "delete from `user` where `name` = ?",
			args:     []interface{}{"tom"},
			isErr:    false,
		},
	}

	for i, c := range cases {
		selector, _ := queries.Parse(c.selector)

		query, args, err := GenDeleteSql(c.table, selector)
		assert.Equal(t, c.query, query, "case-%d", i)
		assert.Equal(t, c.args, args, "case-%d", i)
		assert.Equal(t, c.isErr, err != nil, "case-%d", i)

	}

}
