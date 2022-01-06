package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/util"
)

func TestMysqlMigrate(t *testing.T) {
	DEBUG = true
	driver = envDef("TEST_DB_DRIVER", "mysql")
	dsn = envDef("TEST_DB_DSN", "root:1234@tcp(127.0.0.1:3306)/test?parseTime=true")

	db, err := Open(driver, dsn)
	if err != nil {
		t.Logf("open(%s, %s) err: %s, skip test", driver, dsn, err)
		return
	}

	t.Run("create table", func(t *testing.T) {
		type User1 struct {
			Name string
			Age  int
		}

		db.DropTable(&SqlOptions{table: "user"})
		if err = db.AutoMigrate(&User1{}, WithTable("user")); err != nil {
			t.Fatal(err)
		}

		u1 := User1{Name: "tom", Age: 14}
		if err = db.Insert(&u1, WithTable("user")); err != nil {
			t.Fatal(err)
		}

		var u2 User1
		if err = db.Get(&u2, WithSelector("name=tom"), WithTable("user")); err != nil {
			t.Fatal(err)
		}
		assert.Empty(t, err)
		assert.Equalf(t, &u1, &u2, "user get")

	})

	t.Run("update table", func(t *testing.T) {
		type User2 struct {
			Name     string `sql:",where"`
			Age      int
			Address  string
			NickName *string `sql:",size=1024"`
		}

		if err = db.AutoMigrate(&User2{}, WithTable("user")); err != nil {
			t.Fatal(err)
		}

		u1 := User2{Name: "tom", Age: 14, Address: "beijing", NickName: util.String("t")}
		if err = db.Update(&u1, WithTable("user")); err != nil {
			t.Fatal(err)
		}

		var u2 User2
		if err = db.Get(&u2, WithSelector("name=tom"), WithTable("user")); err != nil {
			t.Fatal(err)
		}
		assert.Empty(t, err)
		assert.Equalf(t, &u1, &u2, "user get")
	})

	//db.DropTable(&SqlOptions{table: "user"})
}
