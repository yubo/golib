package main

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
)

type Position struct {
	X int
	Y int
}

type User struct {
	Name     string
	Age      int
	B        bool
	ABC      bool
	Position Position
}

func main() {
	engine, err := xorm.NewEngine("mysql", "debian-sys-maint:R4uMntLtlanGocvW@/test")
	if err != nil {
		fmt.Println(err)
	}
	user := &User{Name: "tom"}
	engine.ShowSQL(true)

	engine.DropTables(user)
	err = engine.Sync(user)
	if err != nil {
		fmt.Println(err)
		return
	}
	return

	/*
		_, err = engine.Where("name = ?", user.Name).Update(user)
		if err != nil {
			fmt.Println(err)
			return
		}
		out := User{}
		engine.Where("name= ?", user.Name).Find(&out)
		fmt.Printf("%v\n", out)
	*/
}
