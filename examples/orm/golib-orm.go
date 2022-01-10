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

	if err := example(); err != nil {
		fmt.Fprintf(os.Stderr, "err %v", err)
		os.Exit(1)
	}
}

type User struct {
	Name      string `sql:",primary_key,where"`
	Age       int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func example() error {
	db, err := orm.Open("sqlite3", "file:test.db?cache=shared&mode=memory")
	if err != nil {
		return err
	}

	// AutoMigrate, create database table of the object(User)
	// SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=`user`
	// CREATE TABLE `user` (`name` text,`age` integer,`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`name`))
	if err := db.AutoMigrate(&User{}); err != nil {
		return err
	}

	// create a user
	// INSERT INTO `user` (`name`, `age`, `created_at`, `updated_at`) VALUES (`tom`, `0`, `2022-01-10 17:23:44.965903 +0800 CST m=+0.002683397`, `2022-01-10 17:23:44.965903 +0800 CST m=+0.002683397`)
	if err := db.Insert(&User{Name: "tom"}); err != nil {
		return err
	}

	// update user's age
	// UPDATE `user` SET `age` = `17`, `updated_at` = `2022-01-10 17:23:44.965975 +0800 CST m=+0.002756070` WHERE `name` = `tom`
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
	// SELECT * FROM `user` WHERE `name` = `tom`
	{
		var user User
		if err := db.Get(&user, orm.WithSelector("name=tom")); err != nil {
			return err
		}
		fmt.Printf("get user %+v\n", user)
		// get user {Name:tom Age:17 CreatedAt:2022-01-10 17:23:44.965903 +0800 +0800 UpdatedAt:2022-01-10 17:23:44.965975 +0800 +0800}
	}

	// list
	// SELECT * FROM `user`
	// SELECT COUNT(*) FROM `user`
	{
		var users []User
		var total int64
		if err := db.List(&users, orm.WithTotal(&total)); err != nil {
			return err
		}
		fmt.Printf("get users: [%d] %+v\n", total, users)
		// get users: [1] [{Name:tom Age:17 CreatedAt:2022-01-10 17:23:44.965903 +0800 +0800 UpdatedAt:2022-01-10 17:23:44.965975 +0800 +0800}]
	}

	// delete
	// DELETE FROM `user` WHERE `name` = `tom`
	if err := db.Delete(&User{}, orm.WithSelector("name=tom")); err != nil {
		return err
	}

	return nil
}
