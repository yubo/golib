package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yubo/golib/orm"
	"k8s.io/klog/v2"

	_ "github.com/yubo/golib/orm/sqlite"
)

func main() {
	orm.DEBUG = true

	if err := example(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "err %v", err)
		os.Exit(1)
	}
}

type User struct {
	Name      *string `sql:"primary_key,where"`
	Age       *int
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

func example(ctx context.Context) error {
	username := "tom"
	userage := 17

	db, err := orm.Open("sqlite3", "file:test.db?cache=shared&mode=memory")
	if err != nil {
		return err
	}

	// AutoMigrate, create database table of the object(User)
	// SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=`user`
	// CREATE TABLE `user` (`name` text,`age` integer,`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`name`))
	if err := db.AutoMigrate(ctx, &User{}); err != nil {
		return err
	}

	// create a user
	// INSERT INTO `user` (`name`, `created_at`, `updated_at`) VALUES (`tom`, `2022-01-10 17:23:44.965903 +0800 CST m=+0.002683397`, `2022-01-10 17:23:44.965903 +0800 CST m=+0.002683397`)
	if err := db.Insert(ctx, &User{Name: &username}); err != nil {
		return err
	}

	// update user's age
	// UPDATE `user` SET `age` = `17`, `updated_at` = `2022-01-10 17:23:44.965975 +0800 CST m=+0.002756070` WHERE `name` = `tom`
	{
		user := User{
			Name: &username,
			Age:  &userage,
		}
		if err := db.Update(ctx, &user); err != nil {
			return err
		}
	}

	// get
	// SELECT * FROM `user` WHERE `name` = `tom`
	{
		var user User
		if err := db.Get(ctx, &user, orm.WithSelector("name=tom")); err != nil {
			return err
		}
		klog.InfoS("get", "user", user)
		// get user {Name:tom Age:17 CreatedAt:2022-01-10 17:23:44.965903 +0800 +0800 UpdatedAt:2022-01-10 17:23:44.965975 +0800 +0800}
	}

	// list
	// SELECT `name`, `age` FROM `user` WHERE `name` like `%tom%` and `age` > `10`
	// SELECT COUNT(*) FROM `user` WHERE `name` like `%tom%` and `age` > `10`
	{
		var users []User
		var total int
		if err := db.List(ctx, &users, orm.WithTotal(&total), orm.WithSelector("name=~tom,age>10"), orm.WithCols("name", "age")); err != nil {
			return err
		}
		klog.InfoS("list", "total", total, "users", users)
		// get users: [1] [{Name:tom Age:17 CreatedAt:2022-01-10 17:23:44.965903 +0800 +0800 UpdatedAt:2022-01-10 17:23:44.965975 +0800 +0800}]
	}

	// delete
	// DELETE FROM `user` WHERE `name` = `tom`
	if err := db.Delete(ctx, &User{}, orm.WithSelector("name=tom")); err != nil {
		return err
	}

	return nil
}
