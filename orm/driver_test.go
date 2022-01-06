package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/util"
)

var testdb DB

func TestMysqlMigrate(t *testing.T) {
	DEBUG = true
	driver = envDef("TEST_DB_DRIVER", "mysql")
	dsn = envDef("TEST_DB_DSN", "root:1234@tcp(127.0.0.1:3306)/test?parseTime=true")
	var err error
	if testdb, err = Open(driver, dsn); err != nil {
		t.Logf("open(%s, %s) err: %s, skip test", driver, dsn, err)
		return
	}

	defer testdb.Close()

	t.Run("create table", testCreateTable)

	t.Run("update table", testUpdateTable)
}

func TestSqliteMigrate(t *testing.T) {
	DEBUG = true
	driver = "sqlite3"
	dsn = "file:test.db?cache=shared&mode=memory"

	var err error
	if testdb, err = Open(driver, dsn); err != nil {
		t.Logf("open(%s, %s) err: %s, skip test", driver, dsn, err)
		return
	}

	defer testdb.Close()

	t.Run("create table", testCreateTable)
	t.Run("update table", testUpdateTable)
}

func testCreateTable(t *testing.T) {
	type User1 struct {
		ID   int64  `sql:"id,auto_increment,primary_key"`
		Name string `sql:",index"`
		Age  int    `sql:",index"`
	}

	testdb.DropTable(&SqlOptions{table: "user"})
	if err := testdb.AutoMigrate(&User1{}, WithTable("user")); err != nil {
		t.Fatal(err)
	}

	u1 := User1{ID: 1, Name: "tom", Age: 14}
	if err := testdb.Insert(&u1, WithTable("user")); err != nil {
		t.Fatal(err)
	}

	var u2 User1
	if err := testdb.Get(&u2, WithSelector("name=tom"), WithTable("user")); err != nil {
		t.Fatal(err)
	}
	assert.Equalf(t, &u1, &u2, "user get")
}

func testUpdateTable(t *testing.T) {
	type User2 struct {
		Name     string `sql:",where"`
		Age      int
		Address  string
		NickName *string `sql:",size=1024"`
	}

	if err := testdb.AutoMigrate(&User2{}, WithTable("user")); err != nil {
		t.Fatal(err)
	}

	u1 := User2{Name: "tom", Age: 14, Address: "beijing", NickName: util.String("t")}
	if err := testdb.Update(&u1, WithTable("user")); err != nil {
		t.Fatal(err)
	}

	var u2 User2
	if err := testdb.Get(&u2, WithSelector("name=tom"), WithTable("user")); err != nil {
		t.Fatal(err)
	}
	assert.Equalf(t, &u1, &u2, "user get")
}
