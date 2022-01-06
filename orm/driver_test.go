package orm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/clock"
)

var (
	testdb DB
)

func TestMysqlMigrate(t *testing.T) {
	//DEBUG = true
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

	testdb.DropTable(&SqlOptions{table: testTable})
}

func TestSqliteMigrate(t *testing.T) {
	//DEBUG = true
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

	testdb.DropTable(&SqlOptions{table: testTable})
}

func testCreateTable(t *testing.T) {
	c := &clock.FakeClock{}
	SetClock(c)
	c.SetTime(testCreatedTime)

	type User1 struct {
		ID        int64  `sql:"id,auto_increment,primary_key"`
		Name      string `sql:",index"`
		Age       int    `sql:",index"`
		CreatedAt time.Time
		UpdatedAt time.Time
	}

	testdb.DropTable(&SqlOptions{table: testTable})
	if err := testdb.AutoMigrate(&User1{}, WithTable(testTable)); err != nil {
		t.Fatal(err)
	}

	u1 := User1{ID: 1, Name: "tom", Age: 14}
	if err := testdb.Insert(&u1, WithTable(testTable)); err != nil {
		t.Fatal(err)
	}
	u1.CreatedAt = testCreatedTime
	u1.UpdatedAt = testCreatedTime

	var u2 User1
	if err := testdb.Get(&u2, WithSelector("name=tom"), WithTable(testTable)); err != nil {
		t.Fatal(err)
	}
	assert.Equalf(t, util.JsonStr(u1), util.JsonStr(u2), "user get")
}

func testUpdateTable(t *testing.T) {
	c := &clock.FakeClock{}
	SetClock(c)
	c.SetTime(testUpdatedTime)

	type User2 struct {
		Name      string `sql:",where"`
		Age       int
		Address   string
		NickName  *string `sql:",size=1024"`
		CreatedAt time.Time
		UpdatedAt time.Time
	}

	if err := testdb.AutoMigrate(&User2{}, WithTable(testTable)); err != nil {
		t.Fatal(err)
	}

	u1 := User2{Name: "tom", Age: 14, Address: "beijing", NickName: util.String("t")}
	if err := testdb.Update(&u1, WithTable(testTable)); err != nil {
		t.Fatal(err)
	}
	u1.CreatedAt = testCreatedTime
	u1.UpdatedAt = testUpdatedTime

	var u2 User2
	if err := testdb.Get(&u2, WithSelector("name=tom"), WithTable(testTable)); err != nil {
		t.Fatal(err)
	}
	assert.Equalf(t, util.JsonStr(u1), util.JsonStr(u2), "user get")
}

func TestTime(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		c := &clock.FakeClock{}
		SetClock(c)
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

		if err := dbt.db.AutoMigrate(&v, WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		if err := dbt.db.Insert(&v, WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		if err := dbt.db.Get(&v, WithSelector("name=test"), WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, testCreatedTime.Unix(), v.TimeSec, "time sec")
		assert.Equal(t, testCreatedTime.UnixNano()/1e6, v.TimeMilli, "time milli")
		assert.Equal(t, testCreatedTime.UnixNano(), v.TimeNano, "time nano")
		assert.WithinDuration(t, testCreatedTime, v.Time, time.Second, "time")
		assert.WithinDuration(t, testCreatedTime, *v.TimeP, time.Second, "time")

		dbt.db.DropTable(&SqlOptions{table: testTable})
	})
}

func TestStruct(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		//DEBUG = true

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
			Name  string
			User        // inline
			Group Group `sql:",inline"`        // inline
			Role  Role  `sql:"role,size=1024"` // use json.Marshal as []byte
		}

		v := Test{Name: "test", User: User{1, "user-name"}, Group: Group{2, "group-name"}, Role: Role{3, "role-name"}}

		if err := dbt.db.AutoMigrate(&v, WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		if err := dbt.db.Insert(&v, WithTable(testTable)); err != nil {
			t.Fatal(err)
		}

		var got Test
		if err := dbt.db.Get(&got, WithSelector("name=test"), WithTable(testTable)); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, v, got, "test struct")

		dbt.db.DropTable(&SqlOptions{table: testTable})
	})
}
