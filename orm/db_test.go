package orm

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/util"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	dsn             string
	driver          string
	available       bool
	testCreatedTime time.Time
	testUpdatedTime time.Time
	testTable       = "test"
)

// See https://github.com/go-sql-driver/mysql/wiki/Testing
func init() {
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	driver = env("TEST_DB_DRIVER", "sqlite3")
	dsn = env("TEST_DB_DSN", "file:test.db?cache=shared&mode=memory")
	if db, err := Open(driver, dsn); err == nil {
		if err = db.SqlDB().Ping(); err == nil {
			available = true
		}
		db.Close()
	}
	testCreatedTime = time.Unix(1000, 0)
	testUpdatedTime = time.Unix(2000, 0)
}

func runTests(t *testing.T, dsn string, tests ...func(db DB)) {
	var (
		err error
		db  DB
	)

	if !available {
		t.Skipf("SQL server not running on %s", dsn)
	}

	db, err = Open(driver, dsn)
	if err != nil {
		t.Fatalf("error connecting: %s", err.Error())
	}
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test")

	for _, test := range tests {
		test(db)
		db.Exec("DROP TABLE IF EXISTS test")
	}
}

func TestInsert(t *testing.T) {
	runTests(t, dsn,
		func(db DB) {
			db.Exec("CREATE TABLE test (value int)")
			_, err := db.Exec("INSERT INTO test VALUES (?)", 1)
			require.NoError(t, err)

			var v int
			err = db.Query("SELECT value FROM test").Row(&v)
			require.NoError(t, err)
			require.Equal(t, 1, v)
		},
		func(db DB) {

			if driver != "mysql" {
				t.Skipf("skip insert with auto_increment for %s", dsn)
			}

			type foo struct {
				Id    int
				Value int
			}

			_, err := db.Exec(`CREATE TABLE test
(
  id int not null auto_increment,
  value int,
  PRIMARY KEY (id)
) ENGINE=InnoDB auto_increment=1000;`)
			require.NoError(t, err)

			n, err := db.InsertLastId(&foo{Value: 1}, WithTable("test"))
			require.NoError(t, err)
			require.Equal(t, 1000, n)

		},
	)
}

func TestQueryRows(t *testing.T) {
	runTests(t, dsn,
		func(db DB) {
			db.Exec("CREATE TABLE test (value int)")
			db.Exec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

			var v []int
			db.Query("SELECT value FROM test").Rows(&v)
			require.Equal(t, 3, len(v))
		},
		func(db DB) {
			db.Exec("CREATE TABLE test (value int)")
			db.Exec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

			var v []*int
			db.Query("SELECT value FROM test").Rows(&v)
			require.Equal(t, 3, len(v))
		},
		func(db DB) {
			db.Exec("CREATE TABLE test (value int)")
			db.Exec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

			var v []*int
			iter, err := db.Query("SELECT value FROM test").Iterator()
			if err != nil {
				t.Fatalf("query rows iter %s", err)
			}
			defer iter.Close()

			for iter.Next() {
				i := new(int)
				iter.Row(i)
				v = append(v, i)
			}

			require.Equal(t, 3, len(v))
		})
}

func TestDelRows(t *testing.T) {
	runTests(t, dsn, func(db DB) {
		db.Exec("CREATE TABLE test (value int)")
		db.Exec("INSERT INTO test VALUES (?)", 1)

		n, err := db.ExecNum("DELETE FROM test WHERE value = ?", 1)
		require.NoError(t, err)
		require.Equal(t, 1, int(n))

		n, err = db.ExecNum("DELETE FROM test WHERE value = ?", 1)
		require.NoError(t, err)
		require.Equal(t, 0, int(n))

		err = db.ExecNumErr("DELETE FROM test WHERE value = ?", 1)
		require.Error(t, err)
	})
}

func TestExecNum(t *testing.T) {
	runTests(t, dsn, func(db DB) {
		db.Exec("CREATE TABLE test (value int)")

		n, err := db.ExecNum("INSERT INTO test VALUES (?)", 1)
		require.NoError(t, err)
		require.Equal(t, 1, int(n))

		_, err = db.ExecNum("UPDATE test SET value=? WHERE value=?", 2, 1)
		require.NoError(t, err)
		require.Equal(t, 1, int(n))

		_, err = db.ExecNum("DELETE FROM test  WHERE value=?", 2)
		require.NoError(t, err)
		require.Equal(t, 1, int(n))
	})
}

func TestQueryRowStruct(t *testing.T) {
	runTests(t, dsn,
		func(db DB) {
			type foo struct {
				PointX  int64
				PointY  int64 `sql:"point_y"`
				Private int64
				private int64
			}

			db.Exec("CREATE TABLE test (point_x int, point_y int, point_z int)")
			db.Exec("INSERT INTO test VALUES (?, ?, ?)", 1, 2, 3)

			{
				v := foo{}
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, foo{1, 2, 0, 0}, v)
			}

			{
				v := foo{0, 0, 3, 0}
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, foo{1, 2, 3, 0}, v)
			}

			{
				var v *foo
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, foo{1, 2, 0, 0}, *v)
			}

			{
				var v *int
				err := db.Query("SELECT point_x FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, 1, *v)
			}
		},
		func(db DB) {
			db.Exec("CREATE TABLE test (point_x int, point_y int, point_z int)")
			db.Exec("INSERT INTO test VALUES (?, ?, ?)", 1, 2, 3)

			type foo struct {
				PointX  *int64
				PointY  *int64 `sql:"point_y"`
				Private *int64
				private *int64
			}

			{
				var v foo
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, foo{util.Int64(1), util.Int64(2), nil, nil}, v)
			}

			{
				var v *foo
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, foo{util.Int64(1), util.Int64(2), nil, nil}, *v)
			}

			{
				var v *foo
				err := db.Query("SELECT * FROM test where 1 = 2").Row(&v)
				require.Error(t, err)
				require.Nil(t, v)
			}
		},
		func(db DB) {
			type Point struct {
				X int
				Y int
			}
			type foo struct {
				A []string
				B map[string]string
				C *Point
				D Point
				E []byte
				F **string
				G *string
				N int
			}

			db.Exec("CREATE TABLE test (a blob, b blob, c blob, d blob, e blob, f blob, g blob, n int)")

			v := util.String("string")
			cases := []foo{{
				A: []string{"a", "b"},
				B: map[string]string{"c": "d", "e": "f"},
				C: &Point{1, 2},
				D: Point{3, 4},
				E: []byte{'5', '6'},
				F: &v,
				G: util.String("hello"),
				N: 0,
			}, {
				A: nil,
				B: nil,
				C: nil,
				D: Point{},
				E: nil,
				F: nil,
				G: nil,
				N: 1,
			}}

			for _, c := range cases {
				err := db.Insert(c, WithTable("test"))
				require.NoError(t, err)

				got := foo{}
				db.Query("SELECT * FROM test WHERE n = ?", c.N).Row(&got)
				require.Equal(t, c, got)
			}
		},
		func(db DB) {
			var foo []struct {
				PointX  int64
				PointY  int64 `sql:"point_y"`
				Private int64 `sql:"-"`
				private int64
			}

			db.Exec("CREATE TABLE test (point_x int, point_y int)")
			db.Exec("INSERT INTO test VALUES (?, ?), (?, ?), (?, ?)", 1, 2, 3, 4, 5, 6)

			err := db.Query("SELECT * FROM test").Rows(&foo)
			require.NoError(t, err)
			require.Equal(t, 3, len(foo))
		},
		func(db DB) {
			var foo []*struct {
				PointX int64
				PointY int64 `sql:"point_y"`
			}

			db.Exec("CREATE TABLE test (point_x int, point_y int)")
			db.Exec("INSERT INTO test VALUES (?, ?), (?, ?), (?, ?)", 1, 2, 3, 4, 5, 6)

			err := db.Query("SELECT * FROM test").Rows(&foo)
			require.NoError(t, err)
			require.Equal(t, 3, len(foo))
		},
	)
}

func TestPing(t *testing.T) {
	runTests(t, dsn, func(db DB) {
		err := db.SqlDB().Ping()
		require.NoError(t, err)
	})
}

func TestSqlArg(t *testing.T) {
	runTests(t, dsn, func(db DB) {
		db.Exec("CREATE TABLE test (value int);")
		db.Exec("INSERT INTO test VALUES (?);", 1)

		var v int
		db.Query("SELECT value FROM test WHERE value=?;", 1).Row(&v)
		require.Equal(t, 1, v)
	})

	runTests(t, dsn, func(db DB) {
		a := 1
		db.Exec("CREATE TABLE test (value int);")
		db.Exec("INSERT INTO test VALUES (?);", &a)

		var v int
		db.Query("SELECT value FROM test WHERE value=?;", &a).Row(&v)
		require.Equal(t, 1, v)
	})

	runTests(t, dsn, func(db DB) {
		type foo struct {
			PointX  *int
			PointY  *int `sql:"point_y"`
			Private *int `sql:"-"`
			private *int
		}
		x := 1

		db.Exec("CREATE TABLE test (point_x int, point_y int)")
		db.Exec("INSERT INTO test VALUES (?, ?)", &x, nil)

		v := foo{}
		db.Query("SELECT * FROM test;").Row(&v)
		require.Equal(t, v, foo{&x, nil, nil, nil})
	})

}

func TestTx(t *testing.T) {
	if driver != "mysql" {
		t.Skipf("TestTx skiped for %s", dsn)
	}
	runTests(t, dsn,
		func(db DB) {
			db.Exec("CREATE TABLE test (value int) ENGINE=InnoDB;")

			tx, err := db.Begin()
			require.NoError(t, err)

			a := 1
			_, err = tx.Exec("INSERT INTO test VALUES (?);", &a)
			require.NoError(t, err)

			err = tx.Commit()
			require.NoError(t, err)

			var v int
			db.Query("SELECT value FROM test WHERE value=?;", &a).Row(&v)
			require.Equal(t, 1, v)
		},
		func(db DB) {
			db.Exec("CREATE TABLE test (value int) ENGINE=InnoDB;")

			tx, err := db.Begin()
			require.NoError(t, err)

			a := 1
			_, err = tx.Exec("INSERT INTO test VALUES (?);", &a)
			require.NoError(t, err)

			err = tx.Rollback()
			require.NoError(t, err)

			var v int
			db.Query("SELECT value FROM test WHERE value=?;", &a).Row(&v)
			assert.Equal(t, 0, v)
		},
	)

}

func TestList(t *testing.T) {
	runTests(t, dsn, func(db DB) {
		db.Exec("CREATE TABLE test (value int)")
		db.Exec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

		var v []int
		err := db.Query("SELECT value FROM test WHERE value in (1, 2, 3)").Rows(&v)
		require.NoError(t, err)
		require.Equal(t, 3, len(v))
	})

	runTests(t, dsn, func(db DB) {
		db.Exec("CREATE TABLE test (value int)")
		db.Exec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

		var v []int
		err := db.Query("SELECT value FROM test WHERE value IN ('1', '2', '3')").Rows(&v)
		require.NoError(t, err)
		require.Equal(t, 3, len(v))
	})
}
