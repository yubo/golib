package orm

import (
	"database/sql"
	"fmt"
	"os"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"

	_ "github.com/yubo/golib/orm/mysql"
	_ "github.com/yubo/golib/orm/sqlite"
)

var (
	dsn       string
	driver    string
	available bool
)

func envDef(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// See https://github.com/go-sql-driver/mysql/wiki/Testing
func init() {
	driver = envDef("TEST_DB_DRIVER", "sqlite3")
	dsn = envDef("TEST_DB_DSN", "file:test2.db?cache=shared&mode=memory")
	if db, err := DbOpen(driver, dsn); err == nil {
		if err = db.Db.Ping(); err == nil {
			available = true
		}
		db.Close()
	}

}

type DBTest struct {
	*testing.T
	db *Db
}

func runTests(t *testing.T, dsn string, tests ...func(dbt *DBTest)) {
	var (
		err error
		db  *Db
		dbt *DBTest
	)

	if !available {
		t.Skipf("SQL server not running on %s", dsn)
	}

	db, err = DbOpen(driver, dsn)
	if err != nil {
		t.Fatalf("error connecting: %s", err.Error())
	}
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test")

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
	dbt.Fatalf("error on %s %s: %s\n %s", method, query, err.Error(), status.GetDetail(err))
}

func (dbt *DBTest) queryRow(output interface{}, query string, args ...interface{}) {
	dbt.db.Query(query, args...).Row(output)
}

func (dbt *DBTest) mustQueryRow(output interface{}, query string, args ...interface{}) {
	err := dbt.db.Query(query, args...).Row(output)
	if err != nil {
		dbt.fail("query row", query, err)
	}
}

func (dbt *DBTest) mustQueryRows(output interface{}, query string, args ...interface{}) {
	err := dbt.db.Query(query, args...).Rows(output)
	if err != nil {
		dbt.fail("query rows", query, err)
	}
}

func (dbt *DBTest) mustExecNum(query string, args ...interface{}) {
	err := dbt.db.ExecNumErr(query, args...)
	if err != nil {
		dbt.fail("execNum", query, err)
	}
}

func (dbt *DBTest) mustExec(query string, args ...interface{}) (res sql.Result) {
	res, err := dbt.db.Exec(query, args...)
	if err != nil {
		dbt.fail("exec", query, err)
	}
	return res
}

func TestInsert(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		var v int
		dbt.mustExec("CREATE TABLE test (value int);")

		dbt.mustExec("INSERT INTO test VALUES (?);", 1)

		dbt.mustQueryRow(&v, "SELECT value FROM test;")

		dbt.mustExec("DROP TABLE IF EXISTS test;")
	})
}

func TestQueryRows(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		var v []int
		dbt.mustExec("CREATE TABLE test (value int)")

		dbt.mustExec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

		dbt.mustQueryRows(&v, "SELECT value FROM test")

		if len(v) != 3 {
			t.Fatalf("query rows want 3 got %d", len(v))
		}

		dbt.mustExec("DROP TABLE IF EXISTS test")
	})
	runTests(t, dsn, func(dbt *DBTest) {
		var v []*int
		dbt.mustExec("CREATE TABLE test (value int)")

		dbt.mustExec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

		dbt.mustQueryRows(&v, "SELECT value FROM test")

		if len(v) != 3 {
			t.Fatalf("query rows want 3 got %d", len(v))
		}

		dbt.mustExec("DROP TABLE IF EXISTS test")
	})
}

func TestDelRows(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		dbt.mustExec("CREATE TABLE test (value int)")

		dbt.mustExec("INSERT INTO test VALUES (?)", 1)

		n, err := dbt.db.ExecNum("delete from test where value = ?", 1)
		if n != 1 {
			dbt.fail("execNum",
				fmt.Sprintf("got %d want 1", n), err)
		}

		n, err = dbt.db.ExecNum("delete from test where value = ?", 1)
		if err != nil || n != 0 {
			dbt.fail("execNum", fmt.Sprintf("got (%d, %v) want (0, nil)", n, err), err)
		}

		dbt.mustExec("DROP TABLE IF EXISTS test")
	})
}

func TestExecNum(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		dbt.mustExec("CREATE TABLE test (value int)")

		dbt.mustExecNum("INSERT INTO test VALUES (?)", 1)
		dbt.mustExecNum("update test set value=? where value=?", 2, 1)
		dbt.mustExecNum("delete from test  where value=?", 2)

		dbt.mustExec("DROP TABLE IF EXISTS test;")
	})
}

func TestQueryRowStruct(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		type vt struct {
			PointX  int64
			PointY  int64 `sql:"point_y"`
			Private int64
			private int64
		}

		type vt2 struct {
			PointX  *int64
			PointY  *int64 `sql:"point_y"`
			Private *int64
			private *int64
		}

		dbt.mustExec("CREATE TABLE test (point_x int, point_y, point_z int)")
		dbt.mustExec("INSERT INTO test VALUES (?, ?, ?)", 1, 2, 3)

		{
			got := vt{}
			want := vt{1, 2, 0, 0}
			dbt.mustQueryRow(&got, "SELECT * FROM test")
			require.Equal(t, got, want)
		}

		{
			got := vt2{}
			want := vt2{util.Int64(1), util.Int64(2), nil, nil}
			dbt.mustQueryRow(&got, "SELECT * FROM test")
			require.Equal(t, got, want)
		}

		{
			var got *vt2
			var want *vt2
			dbt.queryRow(&got, "SELECT * FROM test where point_x = 0")
			require.Equal(t, got, want)
		}

		{
			var got *vt2
			want := &vt2{util.Int64(1), util.Int64(2), nil, nil}
			dbt.mustQueryRow(&got, "SELECT * FROM test")
			require.Equal(t, got, want)
		}

		{
			v := vt{Private: 3}
			dbt.mustQueryRow(&v, "SELECT * FROM test")
			if v.PointX != 1 {
				t.Fatalf("query PointX want 1 got %d", v.PointX)
			}
			if v.PointY != 2 {
				t.Fatalf("query PointY want 1 got %d", v.PointY)
			}
			if v.Private != 3 {
				t.Fatalf("query Private want 3 got %d", v.Private)
			}
		}
		{
			var v *vt
			dbt.mustQueryRow(&v, "SELECT * FROM test")
			if v.PointX != 1 {
				t.Fatalf("query PointX want 1 got %d", v.PointX)
			}
			if v.PointY != 2 {
				t.Fatalf("query PointY want 1 got %d", v.PointY)
			}
		}
		{
			var x *int64
			dbt.mustQueryRow(&x, "SELECT point_x FROM test")
			if *x != 1 {
				t.Fatalf("query PointX want 1 got %d", *x)
			}
		}

		dbt.mustExec("DROP TABLE IF EXISTS test")
	})
}

func TestQueryRowStruct2(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		type Point struct {
			X int
			Y int
		}
		type vt struct {
			A []string
			B map[string]string
			C *Point
			D Point
			E []byte
			F **string
			G *string
			N int
		}

		dbt.mustExec("CREATE TABLE test (a blob, b blob, c blob, d blob, e blob, f blob, g blob, n int)")

		v := util.String("string")
		cases := []vt{{
			[]string{"a", "b"},
			map[string]string{"c": "d", "e": "f"},
			&Point{1, 2},
			Point{3, 4},
			[]byte{'5', '6'},
			&v,
			util.String("hello"),
			0,
		}, {
			nil,
			nil,
			nil,
			Point{},
			nil,
			nil,
			nil,
			1,
		}}

		for _, c := range cases {
			if err := dbt.db.Insert("test", c); err != nil {
				t.Fatal(err)
			}
			got := vt{}
			dbt.mustQueryRow(&got, "SELECT * FROM test where n = ?", c.N)
			require.Equal(t, c, got)
		}
		dbt.mustExec("DROP TABLE IF EXISTS test")
	})
}

func TestQueryRowsStruct(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		var v []struct {
			PointX  int64
			PointY  int64 `sql:"point_y"`
			Private int64 `sql:"-"`
			private int64
		}

		dbt.mustExec("CREATE TABLE test (point_x int, point_y int)")

		dbt.mustExec("INSERT INTO test VALUES (?, ?), (?, ?), (?, ?)", 1, 2, 3, 4, 5, 6)

		dbt.mustQueryRows(&v, "SELECT * FROM test")

		if len(v) != 3 {
			t.Fatalf("query rows want 3 got %d", len(v))
		}
		if v[2].PointX != 5 {
			t.Fatalf("v[2].PointX want 5 got %d", v[2].PointX)
		}
		if v[2].PointY != 6 {
			t.Fatalf("v[2].PointY want 6 got %d", v[2].PointY)
		}

		dbt.mustExec("DROP TABLE IF EXISTS test")
	})
}

func TestQueryRowsStructPtr(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		var v []*struct {
			PointX int64
			PointY int64 `sql:"point_y"`
		}

		dbt.mustExec("CREATE TABLE test (point_x int, point_y int)")

		dbt.mustExec("INSERT INTO test VALUES (?, ?), (?, ?), (?, ?)", 1, 2, 3, 4, 5, 6)

		dbt.mustQueryRows(&v, "SELECT * FROM test")

		if len(v) != 3 {
			t.Fatalf("query rows want 3 got %d", len(v))
		}
		if v[2].PointX != 5 {
			t.Fatalf("v[2].PointX want 5 got %d", v[2].PointX)
		}
		if v[2].PointY != 6 {
			t.Fatalf("v[2].PointY want 6 got %d", v[2].PointY)
		}

		dbt.mustExec("DROP TABLE IF EXISTS test")
	})
}

func TestPing(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		if err := dbt.db.Db.Ping(); err != nil {
			dbt.fail("Ping", "Ping", err)
		}
	})
}

type User struct {
	SQLModel `sql:",inline"`
	Name     string `sql:"name" tname:"fn_group"`
	Auth     int    `sql:"auth"`
}

type SQLModel struct {
	Id       int       `beedb:"PK" sql:"id"`
	Created  time.Time `sql:"created"`
	Modified time.Time `sql:"modified"`
}

func TestUpdateSql(t *testing.T) {
	type vt struct {
		PointX  int
		PointY  int `sql:"point_y"`
		Private int `sql:",where"`
		private int
	}

	cases := []struct {
		tab    string
		sample vt
		sql    string
		args   []interface{}
	}{{
		"vt",
		vt{1, 2, 3, 4},
		"update vt set point_x=?, point_y=? where private=?",
		[]interface{}{1, 2, 3},
	}}
	for _, c := range cases {
		if sql, args, err := GenUpdateSql("vt", c.sample); err != nil {
			t.Fatal(err)
		} else {
			require.Equal(t, c.sql, sql)
			require.Equal(t, c.args, args)
		}
	}
}

func TestSqlArg(t *testing.T) {

	runTests(t, dsn, func(dbt *DBTest) {
		a := 1
		var v int
		dbt.mustExec("CREATE TABLE test (value int);")

		dbt.mustExec("INSERT INTO test VALUES (?);", a)

		dbt.mustQueryRow(&v, "SELECT value FROM test where value=?;", a)
		require.Equal(t, 1, v)

		dbt.mustExec("DROP TABLE IF EXISTS test;")
	})

	runTests(t, dsn, func(dbt *DBTest) {
		a := 1
		var v int
		dbt.mustExec("CREATE TABLE test (value int);")

		dbt.mustExec("INSERT INTO test VALUES (?);", &a)

		dbt.mustQueryRow(&v, "SELECT value FROM test where value=?;", &a)
		require.Equal(t, 1, v)

		dbt.mustExec("DROP TABLE IF EXISTS test;")
	})

	runTests(t, dsn, func(dbt *DBTest) {

		type vt struct {
			PointX  *int
			PointY  *int `sql:"point_y"`
			Private *int `sql:"-"`
			private *int
		}
		pointX := 1

		dbt.mustExec("CREATE TABLE test (point_x, point_y int);")

		dbt.mustExec("INSERT INTO test VALUES (?, ?);", &pointX, nil)

		v := vt{}
		dbt.mustQueryRow(&v, "SELECT * FROM test;")
		require.Equal(t, v, vt{&pointX, nil, nil, nil})

		// dbt.mustQueryRow(&v, "SELECT value FROM test where b = ?;", 0)
		// require.Equal(t, 1, v)

		dbt.mustExec("DROP TABLE IF EXISTS test;")
	})

}

func TestTx(t *testing.T) {
	if driver != "mysql" {
		return
	}
	runTests(t, dsn, func(dbt *DBTest) {
		a := 1
		var v int
		dbt.mustExec("CREATE TABLE test (value int) ENGINE=InnoDB;")

		tx, err := dbt.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec("INSERT INTO test VALUES (?);", &a); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}

		dbt.mustQueryRow(&v, "SELECT value FROM test where value=?;", &a)
		require.Equal(t, 1, v)

		dbt.mustExec("DROP TABLE IF EXISTS test;")
	})

	runTests(t, dsn, func(dbt *DBTest) {
		a := 1
		var v int
		dbt.mustExec("CREATE TABLE test (value int) ENGINE=InnoDB;")

		tx, err := dbt.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec("INSERT INTO test VALUES (?);", &a); err != nil {
			t.Fatal(err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}

		dbt.queryRow(&v, "SELECT value FROM test where value=?;", &a)
		require.Equal(t, 0, v)

		dbt.mustExec("DROP TABLE IF EXISTS test;")
	})

	runTests(t, dsn, func(dbt *DBTest) {
		type test struct {
			Id    *int
			Value *int
		}

		dbt.mustExec(`CREATE TABLE test (
id int not null auto_increment,
value int,
PRIMARY KEY (id)
) ENGINE=InnoDB auto_increment=1000;`)

		tx, err := dbt.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 10; i++ {
			if id, err := tx.InsertLastId("test", &test{Value: &i}); err != nil {
				t.Fatal(err)
			} else {
				t.Logf("id %d", id)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}

		dbt.mustExec("DROP TABLE IF EXISTS test;")
	})

}
