package orm

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/clock"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	dsn       string
	driver    string
	available bool
)

// See https://github.com/go-sql-driver/mysql/wiki/Testing
func init() {
	RegisterMysql()
	RegisterSqlite()

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
}

func runTests(t *testing.T, tests ...func(db DB)) {
	if !available {
		t.Skipf("SQL server not running on %s", dsn)
	}

	_runTests(t, driver, dsn, tests...)
}

func _runTests(t *testing.T, driver, dsn string, tests ...func(db DB)) {
	db, err := Open(driver, dsn)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test")

	for _, test := range tests {
		test(db)
		db.Exec("DROP TABLE IF EXISTS test")
	}

}

func TestInsert(t *testing.T) {
	runTests(t,
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

		})
}

func TestQueryRows(t *testing.T) {
	runTests(t,
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
			require.NoError(t, err)
			defer iter.Close()

			for iter.Next() {
				i := new(int)
				iter.Row(i)
				v = append(v, i)
			}

			require.Equal(t, 3, len(v))
		},
	)
}

func TestDelRows(t *testing.T) {
	runTests(t,
		func(db DB) {
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
		},
	)
}

func TestExecNum(t *testing.T) {
	runTests(t,
		func(db DB) {
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
		},
	)
}

func TestQueryRowStruct(t *testing.T) {
	runTests(t,
		func(db DB) {
			type test struct {
				X       int64
				Y       int64 `sql:"point_y"`
				Private int64
				private int64
			}

			err := db.AutoMigrate(&test{})
			require.NoError(t, err)

			err = db.Insert(&test{X: 1, Y: 2})
			require.NoError(t, err)

			{
				v := test{}
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, test{1, 2, 0, 0}, v)
			}

			{
				v := test{0, 0, 3, 0}
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, test{1, 2, 0, 0}, v)
			}

			{
				var v *test
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, test{1, 2, 0, 0}, *v)
			}

			{
				var v *int
				err := db.Query("SELECT point_y FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, 2, *v)
			}
		},
		func(db DB) {
			type test struct {
				X       *int64
				Y       *int64 `sql:"point_y"`
				Private *int64
				private *int64
			}

			err := db.AutoMigrate(&test{})
			require.NoError(t, err)

			err = db.Insert(&test{util.Int64(1), util.Int64(2), nil, nil})
			require.NoError(t, err)

			{
				var v test
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, test{util.Int64(1), util.Int64(2), nil, nil}, v)
			}

			{
				var v *test
				err := db.Query("SELECT * FROM test").Row(&v)
				require.NoError(t, err)
				require.Equal(t, test{util.Int64(1), util.Int64(2), nil, nil}, *v)
			}

			{
				var v *test
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
			type test struct {
				A []string
				B map[string]string
				C *Point
				D Point
				E []byte
				F **string
				G *string
				N int
			}

			// CREATE TABLE `test` (`a` blob,`b` blob,`c` blob,`d` blob,`e` blob,`f` ,`g` text,`n` integer)
			err := db.AutoMigrate(&test{})
			require.NoError(t, err)

			v := util.String("string")
			cases := []test{{
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

				got := test{}
				db.Query("SELECT * FROM test WHERE n = ?", c.N).Row(&got)
				require.Equal(t, c, got)
			}
		},
		func(db DB) {
			type test struct {
				X int64
				Y int64 `sql:"point_y"`
				z int64
			}

			// CREATE TABLE `test` (`x` integer,`point_y` integer)
			err := db.AutoMigrate(&test{})
			require.NoError(t, err)

			_, err = db.Exec("INSERT INTO test VALUES (?, ?), (?, ?), (?, ?)", 1, 2, 3, 4, 5, 6)
			require.NoError(t, err)

			var v []test
			err = db.Query("SELECT * FROM test").Rows(&v)
			require.NoError(t, err)
			require.Equal(t, 3, len(v))
		},
		func(db DB) {
			type test struct {
				X int64
				Y int64 `sql:"point_y"`
				z int64
			}

			// CREATE TABLE `test` (`x` integer,`point_y` integer)
			err := db.AutoMigrate(&test{})
			require.NoError(t, err)

			_, err = db.Exec("INSERT INTO test VALUES (?, ?), (?, ?), (?, ?)", 1, 2, 3, 4, 5, 6)
			require.NoError(t, err)

			var v []*test
			err = db.Query("SELECT * FROM test").Rows(&v)
			require.NoError(t, err)
			require.Equal(t, 3, len(v))
		},
	)
}

func TestPing(t *testing.T) {
	runTests(t, func(db DB) {
		err := db.SqlDB().Ping()
		require.NoError(t, err)
	})
}

func TestSqlArg(t *testing.T) {
	runTests(t, func(db DB) {
		db.Exec("CREATE TABLE test (value int);")
		db.Exec("INSERT INTO test VALUES (?);", 1)

		var v int
		db.Query("SELECT value FROM test WHERE value=?;", 1).Row(&v)
		require.Equal(t, 1, v)
	})

	runTests(t, func(db DB) {
		a := 1
		db.Exec("CREATE TABLE test (value int);")
		db.Exec("INSERT INTO test VALUES (?);", &a)

		var v int
		db.Query("SELECT value FROM test WHERE value=?;", &a).Row(&v)
		require.Equal(t, 1, v)
	})

	runTests(t, func(db DB) {
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
	runTests(t,
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
	runTests(t, func(db DB) {
		db.Exec("CREATE TABLE test (value int)")
		db.Exec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

		var v []int
		err := db.Query("SELECT value FROM test WHERE value in (1, 2, 3)").Rows(&v)
		require.NoError(t, err)
		require.Equal(t, 3, len(v))
	})

	runTests(t, func(db DB) {
		db.Exec("CREATE TABLE test (value int)")
		db.Exec("INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

		var v []int
		err := db.Query("SELECT value FROM test WHERE value IN ('1', '2', '3')").Rows(&v)
		require.NoError(t, err)
		require.Equal(t, 3, len(v))
	})
}

func TestStructCRUD(t *testing.T) {
	runTests(t, func(db DB) {
		createdAt := time.Unix(1000, 0).UTC()
		updatedAt := time.Unix(2000, 0).UTC()

		c := &clock.FakeClock{}
		SetClock(c)
		c.SetTime(createdAt)

		type test struct {
			Name      string `sql:",where"`
			Age       int
			Address   string
			NickName  *string `sql:",size=1024"`
			CreatedAt time.Time
			UpdatedAt time.Time
		}

		// sqlite: CREATE TABLE `test` (`name` text,`age` integer,`address` text,`nick_name` text,`created_at` datetime,`updated_at` datetime)
		err := db.AutoMigrate(&test{})
		require.NoError(t, err)

		// create
		user := test{Name: "tom", Age: 14}
		err = db.Insert(&user)
		require.NoError(t, err)

		user.CreatedAt = createdAt
		user.UpdatedAt = createdAt

		// read
		var got test
		err = db.Get(&got, WithSelector("name=tom"))
		require.NoError(t, err)
		assert.Equalf(t, user, got, "user get")

		// update
		c.SetTime(updatedAt)
		user = test{Name: "tom", Age: 14, Address: "beijing", NickName: util.String("t")}
		err = db.Update(&user)
		require.NoError(t, err)
		user.CreatedAt = createdAt
		user.UpdatedAt = updatedAt

		got = test{}
		err = db.Get(&got, WithSelector("name=tom"))
		require.NoError(t, err)
		assert.Equalf(t, util.JsonStr(user), util.JsonStr(got), "user get")

		// delete
		user = test{Name: "tom"}
		err = db.Delete(&user, WithSelector("name=tom"))
		require.NoError(t, err)
	})
}

func TestStruct(t *testing.T) {
	runTests(t, func(db DB) {
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

		type test struct {
			Name  string //
			User         // inline
			Group Group  `sql:",inline"`        // inline
			Role  Role   `sql:"role,size=1024"` // use json.Marshal as []byte
		}

		v := test{Name: "test", User: User{1, "user-name"}, Group: Group{2, "group-name"}, Role: Role{3, "role-name"}}

		err := db.AutoMigrate(&v)
		require.NoError(t, err)

		err = db.Insert(&v)
		require.NoError(t, err)

		var got test
		err = db.Get(&got, WithSelector("name=test"))
		require.NoError(t, err)
		assert.Equal(t, v, got, "test struct")
	})
}

func TestTime(t *testing.T) {
	runTests(t, func(db DB) {
		createdAt := time.Unix(1000, 0).UTC()
		updatedAt := time.Unix(2000, 0).UTC()
		c := &clock.FakeClock{}
		SetClock(c)
		c.SetTime(createdAt)

		type test struct {
			Name      string     `sql:",where"`
			TimeSec   int64      `sql:",auto_updatetime"`
			TimeMilli int64      `sql:",auto_updatetime=milli"`
			TimeNano  int64      `sql:",auto_updatetime=nano"`
			Time      time.Time  `sql:",auto_updatetime"`
			TimeP     *time.Time `sql:",auto_updatetime"`
		}

		v := test{Name: "test"}
		err := db.AutoMigrate(&v)
		require.NoError(t, err)

		err = db.Insert(&v)
		require.NoError(t, err)

		v = test{}
		err = db.Get(&v, WithSelector("name=test"))
		require.NoError(t, err)
		assert.Equal(t, createdAt.Unix(), v.TimeSec, "time sec")
		assert.Equal(t, createdAt.UnixMilli(), v.TimeMilli, "time milli")
		assert.Equal(t, createdAt.UnixNano(), v.TimeNano, "time nano")
		assert.WithinDuration(t, createdAt, v.Time, time.Second, "time")
		assert.WithinDuration(t, createdAt, *v.TimeP, time.Second, "time")

		c.SetTime(updatedAt)
		err = db.Update(&v)
		require.NoError(t, err)

		v = test{}
		err = db.Get(&v, WithSelector("name=test"))
		require.NoError(t, err)
		assert.Equal(t, updatedAt.Unix(), v.TimeSec, "time sec")
		assert.Equal(t, updatedAt.UnixMilli(), v.TimeMilli, "time milli")
		assert.Equal(t, updatedAt.UnixNano(), v.TimeNano, "time nano")
		assert.WithinDuration(t, updatedAt, v.Time, time.Second, "time")
		assert.WithinDuration(t, updatedAt, *v.TimeP, time.Second, "time")
	})
}
