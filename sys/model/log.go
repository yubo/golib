package model

import (
	"strings"

	"github.com/yubo/golib/openapi/api"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/util"
)

func genLogSql(userName, action, target *string, start, end *int64) (where string, args []interface{}) {
	a := []string{}
	b := []interface{}{}
	if util.StringValue(userName) != "" {
		a = append(a, "user_name like ?")
		b = append(b, "%"+*userName+"%")
	}
	if util.StringValue(action) != "" {
		a = append(a, "action like ?")
		b = append(b, "%"+*action+"%")
	}
	if util.StringValue(target) != "" {
		a = append(a, "target like ?")
		b = append(b, "%"+*target+"%")
	}
	if util.Int64Value(start) > 0 {
		a = append(a, "created_at > ?")
		b = append(b, *start)
	}
	if util.Int64Value(end) > 0 {
		a = append(a, "created_at < ?")
		b = append(b, *end)
	}
	if len(a) > 0 {
		where = " where " + strings.Join(a, " and ")
		args = b
	}
	return
}

func GetLogsCnt(db *orm.Db, userName, action, target *string, start, end *int64) (ret int64, err error) {
	sql, args := genLogSql(userName, action, target, start, end)
	err = db.Query("select count(*) from log"+sql, args...).Row(&ret)
	return
}

func GetLogs(db *orm.Db, userName, action, target *string, start, end *int64, sqlExtra string) (logs []api.Log, err error) {
	sql, args := genLogSql(userName, action, target, start, end)
	err = db.Query(`select * from log`+sql+sqlExtra, args...).Rows(&logs)
	return
}

func GetLog(db *orm.Db, id *int64) (ret *api.Log, err error) {
	err = db.Query("select * from log where id = ?", util.Int64Value(id)).Row(&ret)
	return
}

func CreateLog(db *orm.Db, in *api.CreateLogInput) error {
	return db.Insert("log", in)
}
