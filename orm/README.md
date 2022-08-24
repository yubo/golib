# orm

## Quick Start

* Open orm
```go
db, err := orm.Open(driverName, dataSourceName)
```

* Define table struct to database and automigrate
```go
type User struct {
    Id   *int   `sql:",index,auto_increment=1000"`
    Name string `sql:",primary_key"`
}

// create table `user` if not exist
db.AutoMigrate(&User{})
// msyql: CREATE TABLE `test` (`id` bigint AUTO_INCREMENT,`name` varchar(255),PRIMARY KEY (`name`),INDEX (`id`) ) auto_increment=1000
```

* `Exec` runs a SQL string, it returns `error`

```go
err := db.ExecNumErr("delete from user where name=?", "test")
```


* `Insert` one record to database
```go
err := db.Insert(&user)
// insert into user () values ()

err := db.Insert(&user, orm.WithTable("system_user"))
// insert into system_user () values ()
```

* `Query` query one record from database

```go
err := db.Query("select * from user limit 1").Row(&user)
```

* check if one record or affected exist with query/exec
```go
import "github.com/yubo/golib/api/errors"

if errors.IsNotFound(err) {
	// do something
}
```

* `Rows` query multiple records from database
```go
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
```go
type User struct {
    Name   *string `sql:",where"`
    Age    *int
    Passwd *string
}

db.Update(&user)
// if user.Passwd == nil
// update user set age=? where name = ?
// else
// update user set age=?, passwd=? where name = ?

// with selector
passwd := ""
db.Update(&user{Passwd:&passwd}, orm.WithSelector("age<16"))
// update user set passwd='' where age < 16

db.Update(&user, orm.WithTable("system_user"))
// update system_user set ... where name = ?
```

* Transation
```go
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

## tags
```
type User struct {
        ID    int64   `sql:"<sqltag>"`
}

sqltag: <key>[=<value>],...
```

sql tags:
  - `name`: name of the field
  - `where`
  - `inline`
  - `index`
  - `primary_key`
  - `auto_increment`
  - `default`
  - `size`
  - `precision`
  - `scale`
  - `not_null`
  - `unique`
  - `comment`
  - `auto_createtime`
  - `auto_updatetime`
  - `type`
