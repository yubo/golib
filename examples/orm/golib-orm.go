package main

import (
	"fmt"
	"os"
	"time"

	"github.com/yubo/golib/orm"

	_ "github.com/yubo/golib/orm/sqlite"
)

func main() {
	orm.DEBUG = true
	// auto migrate
	if err := example(); err != nil {
		fmt.Fprintf(os.Stderr, "err %v", err)
		os.Exit(1)
	}
}

// output:
// I0110 16:56:03.154069   49176 base.go:96] SELECT count(*) FROM sqlite_master WHERE type='table' AND name=`user`
// I0110 16:56:03.154274   49176 base.go:34] CREATE TABLE `user` (`name` text,`age` integer,`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`name`))
// I0110 16:56:03.154685   49176 base.go:123] insert into `user` (`name`, `age`, `created_at`, `updated_at`) values (`tom`, `0`, `2022-01-10 16:56:03.154646 +0800 CST m=+0.002946117`, `2022-01-10 16:56:03.154646 +0800 CST m=+0.002946117`)
// I0110 16:56:03.154773   49176 base.go:197] update `user` set `age` = `17`, `updated_at` = `2022-01-10 16:56:03.154762 +0800 CST m=+0.003061321` where `name` = `tom`
// I0110 16:56:03.154896   49176 base.go:182] select * from `user` where `name` = `tom`
// get user {Name:tom Age:17 CreatedAt:2022-01-10 16:56:03.154646 +0800 +0800 UpdatedAt:2022-01-10 16:56:03.154762 +0800 +0800}
// I0110 16:56:03.155034   49176 base.go:157] select * from `user`
// get users: [1] [{Name:tom Age:17 CreatedAt:2022-01-10 16:56:03.154646 +0800 +0800 UpdatedAt:2022-01-10 16:56:03.154762 +0800 +0800}]
// I0110 16:56:03.155189   49176 base.go:212] delete from `user` where `name` = `tom`

func example() error {

	db, err := orm.Open("sqlite3", "file:test.db?cache=shared&mode=memory")
	if err != nil {
		return err
	}

	// AutoMigrate, create database table of the object(User)
	if err := db.AutoMigrate(&User{}); err != nil {
		return err
	}

	// create a user
	if err := db.Insert(&User{Name: "tom"}); err != nil {
		return err
	}

	// update user's age
	{
		user := User{
			Name: "tom",
			Age:  17,
		}
		if err := db.Update(&user); err != nil {
			return err
		}
	}

	// get a user named Tom
	{
		var user User
		if err := db.Get(&user, orm.WithSelector("name=tom")); err != nil {
			return err
		}
		fmt.Printf("get user %+v\n", user)
	}

	// list
	{
		var users []User
		var total int64
		if err := db.List(&users, orm.WithTotal(&total)); err != nil {
			return err
		}
		fmt.Printf("get users: [%d] %+v\n", total, users)
	}

	// delete
	if err := db.Delete(&User{}, orm.WithSelector("name=tom")); err != nil {
		return err
	}

	return nil
}

type User struct {
	Name      string `sql:",primary_key,where"`
	Age       int
	CreatedAt time.Time
	UpdatedAt time.Time
}
