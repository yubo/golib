package sqlite

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/yubo/golib/orm/driver"
)

func init() {
	driver.RegisterSqlite()
}
