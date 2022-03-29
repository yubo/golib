package driver

import (
	"os"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/clock"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	testdb          orm.DB
	testCreatedTime time.Time
	testUpdatedTime time.Time
	testTable       = "test"
)

func init() {
	testCreatedTime = time.Unix(1000, 0)
	testUpdatedTime = time.Unix(2000, 0)

	RegisterMysql()
	RegisterSqlite()
}

func envDef(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

type DBTest struct {
	*testing.T
	db orm.DB
}

func runTests(t *testing.T, driver, dsn string, tests ...func(dbt *DBTest)) {
	var (
		err error
		db  orm.DB
		dbt *DBTest
	)

	db, err = orm.Open(driver, dsn)
	if err != nil {
		t.Fatalf("error connecting: %s", err.Error())
	}
	defer db.Close()

	if _, err = db.Exec("DROP TABLE IF EXISTS test"); err != nil {
		t.Fatalf("drop table: %s", err.Error())
	}

	dbt = &DBTest{t, db}
	for _, test := range tests {
		test(dbt)
		dbt.db.Exec("DROP TABLE IF EXISTS test")
	}
}

func (dbt *DBTest) fail(method, query string, err error) {
	if len(query) > 300 {
		query = "[query too large to print]"
	}
	dbt.Log(string(debug.Stack()))
	dbt.Fatalf("error on %s %s: %s", method, query, err.Error())
}

func TestMysqlMigrate(t *testing.T) {
	driver := envDef("TEST_DB_DRIVER", "mysql")
	dsn := envDef("TEST_DB_DSN", "root:1234@tcp(127.0.0.1:3306)/test?parseTime=true")

	runTests(t, driver, dsn, func(dbt *DBTest) {
		testdb = dbt.db
		//orm.DEBUG = true

		t.Run("create table", testCreateTable)
		t.Run("update table", testUpdateTable)
	})
}

func TestSqliteMigrate(t *testing.T) {
	driver := "sqlite3"
	dsn := "file:test.db?cache=shared&mode=memory"

	runTests(t, driver, dsn, func(dbt *DBTest) {
		testdb = dbt.db
		//orm.DEBUG = true

		t.Run("create table", testCreateTable)
		t.Run("update table", testUpdateTable)
	})

}

func testCreateTable(t *testing.T) {
	c := &clock.FakeClock{}
	orm.SetClock(c)
	c.SetTime(testCreatedTime)

	type User1 struct {
		ID        int64     `sql:"id,auto_increment,primary_key"`
		Name      string    `sql:",index"`
		Age       int       `sql:",index"`
		CreatedAt time.Time `sql:",auto_createtime"`
		UpdatedAt time.Time `sql:",auto_updatetime"`
	}

	if err := testdb.AutoMigrate(&User1{}, orm.WithTable(testTable)); err != nil {
		t.Fatal(err)
	}

	u1 := User1{ID: 1, Name: "tom", Age: 14}
	if err := testdb.Insert(&u1, orm.WithTable(testTable)); err != nil {
		t.Fatal(err)
	}
	u1.CreatedAt = testCreatedTime
	u1.UpdatedAt = testCreatedTime

	var u2 User1
	if err := testdb.Get(&u2, orm.WithSelector("name=tom"), orm.WithTable(testTable)); err != nil {
		t.Fatal(err)
	}
	assert.Equalf(t, util.JsonStr(u1), util.JsonStr(u2), "user get")
}

func testUpdateTable(t *testing.T) {
	c := &clock.FakeClock{}
	orm.SetClock(c)
	c.SetTime(testUpdatedTime)

	type User2 struct {
		Name      string `sql:",where"`
		Age       int
		Address   string
		NickName  *string `sql:",size=1024"`
		CreatedAt time.Time
		UpdatedAt time.Time
	}

	if err := testdb.AutoMigrate(&User2{}, orm.WithTable(testTable)); err != nil {
		t.Fatal(err)
	}

	u1 := User2{Name: "tom", Age: 14, Address: "beijing", NickName: util.String("t")}
	if err := testdb.Update(&u1, orm.WithTable(testTable)); err != nil {
		t.Fatal(err)
	}
	u1.CreatedAt = testCreatedTime
	u1.UpdatedAt = testUpdatedTime

	var u2 User2
	if err := testdb.Get(&u2, orm.WithSelector("name=tom"), orm.WithTable(testTable)); err != nil {
		t.Fatal(err)
	}
	assert.Equalf(t, util.JsonStr(u1), util.JsonStr(u2), "user get")
}

func TestTime(t *testing.T) {
	driver := "sqlite3"
	dsn := "file:test.db?cache=shared&mode=memory"

	runTests(t, driver, dsn, func(dbt *DBTest) {
		c := &clock.FakeClock{}
		orm.SetClock(c)
		c.SetTime(testCreatedTime)

		type test struct {
			Name      string
			TimeSec   int64      `sql:",auto_createtime"`
			TimeMilli int64      `sql:",auto_createtime=milli"`
			TimeNano  int64      `sql:",auto_createtime=nano"`
			Time      time.Time  `sql:",auto_createtime"`
			TimeP     *time.Time `sql:",auto_createtime"`
		}

		v := test{Name: "test"}

		if err := dbt.db.AutoMigrate(&v, orm.WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		if err := dbt.db.Insert(&v, orm.WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		if err := dbt.db.Get(&v, orm.WithSelector("name=test"), orm.WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, testCreatedTime.Unix(), v.TimeSec, "time sec")
		assert.Equal(t, testCreatedTime.UnixNano()/1e6, v.TimeMilli, "time milli")
		assert.Equal(t, testCreatedTime.UnixNano(), v.TimeNano, "time nano")
		assert.WithinDuration(t, testCreatedTime, v.Time, time.Second, "time")
		assert.WithinDuration(t, testCreatedTime, *v.TimeP, time.Second, "time")

	})
}

func TestStruct(t *testing.T) {
	driver := "sqlite3"
	dsn := "file:test.db?cache=shared&mode=memory"

	runTests(t, driver, dsn, func(dbt *DBTest) {
		//orm.DEBUG = true

		type User struct {
			UserID   int
			UserName string
		}
		type Group struct {
			GroupID   int
			GroupName string
		}

		type Role struct {
			RouteID  int
			RoleName string
		}

		type Test struct {
			Name  string //
			User         // inline
			Group Group  `sql:",inline"`        // inline
			Role  Role   `sql:"role,size=1024"` // use json.Marshal as []byte
		}

		v := Test{Name: "test", User: User{1, "user-name"}, Group: Group{2, "group-name"}, Role: Role{3, "role-name"}}

		if err := dbt.db.AutoMigrate(&v, orm.WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		if err := dbt.db.Insert(&v, orm.WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		var got Test
		if err := dbt.db.Get(&got, orm.WithSelector("name=test"), orm.WithTable(testTable)); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, v, got, "test struct")
	})
}
