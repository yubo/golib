# orm

## Quick Start

* Open orm
```Go
db, err := orm.Open(driverName, dataSourceName)
```

* Define table struct to database

```Go
type User struct {
    Name   string
    Age    int
    Passwd *string
}
```

* `Exec` runs a SQL string, it returns `error`
```Go
err := db.ExecNumErr("delete from user where name=?", "test")
```


* `Insert` one record to database
```Go
err := db.Insert(&user)
// insert into user () values ()

err := db.Insert(&user, orm.WithTable("system_user"))
// insert into system_user () values ()
```

* `Query` query one record from database

```Go
err := db.Query("select * from user limit 1").Row(&user)
```

* check if one record or affected exist with query/exec
```Go
import "github.com/yubo/golib/api/errors"

if errors.IsNotFound(err) {
	// do something
}
```

* `Rows` query multiple records from database
```Go
var users []User
err := db.Query("select * from user where age > ?", 10).Rows(&users)
```

* `Rows` query multiple records from database with Count
```
var users []User
var total int64
err := db.Query("select * from user where age > ?", 10).Count(&total).Rows(&users)
// select * from user where age > 10
// select count(*) from user where age > 10
```

* `Update` update one record
```Go
type User struct {
    Name   string `sql:",where"`
    Age    int
    Passwd *string
}

affected, err := db.Update(&user)
// if user.Passwd == nil
// update user set age=? where name = ?
// else
// update user set age=?, passwd=? where name = ?

affected, err := db.Update(&user, orm.WithTable("system_user"))
// update system_user set ... where name = ?
```

* Transation
```Go
tx, err := db.Begin()
if err != nil {
	return err
}

// do something...

if err := tx.Insert(&user); err != nil {
	tx.Rollback()
	return err
}

return tx.Commit()
```
