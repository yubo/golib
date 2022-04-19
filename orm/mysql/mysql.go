package mysql

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/yubo/golib/orm"
)

func init() {
	orm.RegisterMysql()
}
