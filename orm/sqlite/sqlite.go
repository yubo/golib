package sqlite

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/yubo/golib/orm"
)

func init() {
	orm.RegisterSqlite()
}
