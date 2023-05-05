package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			query:  "INSERT INTO `user` (`name`, `age`) VALUES (?, ?)",
			args:   []interface{}{"tom", 14},
			isErr:  false,
		},
	}

	for i, c := range cases {
		query, args, err := GenInsertSql(c.table, c.sample, &nonDriver{})
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
		offset     int
		limit      int
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
			orderby:    []string{"name DESC"},
			offset:     0,
			limit:      10,
			query:      "SELECT `name`, `age` FROM `user` WHERE `name` = ? ORDER BY name DESC LIMIT 0, 10",
			queryCount: "SELECT COUNT(*) FROM `user` WHERE `name` = ?",
			args:       []interface{}{"tom"},
			isErr:      false,
		},
	}

	for i, c := range cases {
		selector, _ := Parse(c.selector)

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
			query:    "SELECT `name`, `age` FROM `user` WHERE `name` = ?",
			args:     []interface{}{"tom"},
			isErr:    false,
		},
	}

	for i, c := range cases {
		selector, _ := Parse(c.selector)

		query, args, err := GenGetSql(c.table, c.cols, selector)
		assert.Equal(t, c.query, query, "case-%d", i)
		assert.Equal(t, c.args, args, "case-%d", i)
		assert.Equal(t, c.isErr, err != nil, "case-%d", i)

	}
}

func newSelector(s string) Selector {
	selector, _ := Parse(s)
	return selector
}

func TestGenUpdateSql(t *testing.T) {
	type User struct {
		Name   *string `sql:"where"`
		Age    *int
		Passwd *string
	}

	cases := []struct {
		table    string
		sample   *User
		query    string
		args     []interface{}
		isErr    bool
		selector Selector
	}{
		{
			isErr: true,
		},
		{
			table:  "user",
			sample: &User{util.String("tom"), util.Int(14), nil},
			query:  "UPDATE `user` SET `age` = ? WHERE `name` = ?",
			args:   []interface{}{14, "tom"},
			isErr:  false,
		},
		{
			table:    "user",
			sample:   &User{Age: util.Int(14)},
			query:    "UPDATE `user` SET `age` = ? WHERE `name` = ?",
			args:     []interface{}{14, "tom"},
			isErr:    false,
			selector: newSelector("name=tom"),
		},
		{
			table:    "user",
			sample:   &User{Passwd: util.String("")},
			query:    "UPDATE `user` SET `passwd` = ? WHERE `age` < ?",
			args:     []interface{}{"", "16"},
			isErr:    false,
			selector: newSelector("age<16"),
		},
	}

	for i, c := range cases {
		query, args, err := GenUpdateSql(c.table, c.sample, &nonDriver{}, c.selector)
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
			query:    "DELETE FROM `user` WHERE `name` = ?",
			args:     []interface{}{"tom"},
			isErr:    false,
		},
	}

	for i, c := range cases {
		selector, _ := Parse(c.selector)

		query, args, err := GenDeleteSql(c.table, selector)
		assert.Equal(t, c.query, query, "case-%d", i)
		assert.Equal(t, c.args, args, "case-%d", i)
		assert.Equal(t, c.isErr, err != nil, "case-%d", i)

	}
}
