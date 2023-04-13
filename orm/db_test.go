package orm

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/clock"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	testDsn       string
	testDriver    string
	testAvailable bool
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

	if os.Getenv("DEBUG") != "" {
		DEBUG = true
	}

	testDriver = env("TEST_DB_DRIVER", "sqlite3")
	testDsn = env("TEST_DB_DSN", "file:test.db?cache=shared&mode=memory&parseTime=true")
	if db, err := Open(testDriver, testDsn); err == nil {
		if err = db.SqlDB().Ping(); err == nil {
			testAvailable = true
		}
		db.Close()
	}
}

func runTests(t *testing.T, tests ...func(db DB, ctx context.Context)) {
	if !testAvailable {
		t.Skipf("SQL server not running on %s", testDsn)
	}

	_runTests(t, testDriver, testDsn, tests...)
}

func _runTests(t *testing.T, driver, dsn string, tests ...func(db DB, ctx context.Context)) {
	db, err := Open(driver, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	db.Exec(ctx, "DROP TABLE IF EXISTS test")

	for _, test := range tests {
		test(db, ctx)
		db.Exec(ctx, "DROP TABLE IF EXISTS test")
	}
}

func TestAutoMigrate(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		{
			type test struct {
				ID   *int   `sql:",primary_key,auto_increment=1000"`
				Name string `sql:",index,unique"`
			}

			// mysql: CREATE TABLE `test` (`id` bigint AUTO_INCREMENT,`name` varchar(255) UNIQUE,PRIMARY KEY (`id`),INDEX (`name`) ) auto_increment=1000
			if err := db.AutoMigrate(ctx, &test{}); err != nil {
				t.Error(err)
			}
		}

		{
			type test struct {
				ID          *int `sql:",index,auto_increment=1000"`
				Name        string
				DisplayName string
			}

			// mysql: ALTER TABLE `test` ADD `display_name` varchar(255)
			if err := db.AutoMigrate(ctx, &test{}); err != nil {
				t.Error(err)
			}
		}

		{
			type test struct {
				ID          *int `sql:",index,auto_increment=1000"`
				Name        string
				DisplayName string
				CreatedAt   int64 `sql:",auto_createtime"`
				UpdatedAt   int64 `sql:",auto_updatetime"`
			}

			// mysql: ALTER TABLE `test` ADD `created_at` bigint
			// mysql: ALTER TABLE `test` ADD `updated_at` bigint
			if err := db.AutoMigrate(ctx, &test{}); err != nil {
				t.Error(err)
			}
		}
	})
}

func TestInsert(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		// raw sql
		db.Exec(ctx, "CREATE TABLE test (value int)")
		if _, err := db.Exec(ctx, "INSERT INTO test VALUES (?)", 1); err != nil {
			t.Error(err)
		}

		var v int
		err := db.Query(ctx, "SELECT value FROM test").Row(&v)
		assert.NoError(t, err)
		assert.Equal(t, 1, v)
	}, func(db DB, ctx context.Context) {
		type test struct {
			ID    *int `sql:",primary_key,auto_increment=1000"`
			Value int
		}

		err := db.AutoMigrate(ctx, &test{})
		assert.NoError(t, err)

		n, err := db.InsertLastId(ctx, &test{Value: 1})
		assert.NoError(t, err)
		assert.Equal(t, 1000, int(n))

		var got test
		err = db.Get(ctx, &got, WithSelector("value=1"))
		assert.NoError(t, err)
		assert.Equal(t, test{util.Int(1000), 1}, got)
	}, func(db DB, ctx context.Context) {
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
			Admin bool   // bool
			User         // inline
			Group Group  `sql:",inline"`        // inline
			Role  Role   `sql:"role,size=1024"` // use json.Marshal as []byte
		}

		v := test{Name: "test", User: User{1, "user-name"}, Admin: true, Group: Group{2, "group-name"}, Role: Role{3, "role-name"}}

		err := db.AutoMigrate(ctx, &v)
		assert.NoError(t, err)

		err = db.Insert(ctx, &v)
		assert.NoError(t, err)

		var got test
		err = db.Get(ctx, &got, WithSelector("name=test"))
		assert.NoError(t, err)
		assert.Equal(t, v, got, "test struct")
	})
}

func TestQueryRows(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		db.Exec(ctx, "CREATE TABLE test (value int)")
		db.Exec(ctx, "INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)

		{
			var v []int
			err := db.Query(ctx, "SELECT value FROM test WHERE value in (1, 2, 3)").Rows(&v)
			assert.NoError(t, err)
			assert.Equal(t, 3, len(v))
		}

		{
			var v []int
			err := db.Query(ctx, "SELECT value FROM test WHERE value IN ('1', '2', '3')").Rows(&v)
			assert.NoError(t, err)
			assert.Equal(t, 3, len(v))
		}

		{
			var v []int
			iter, err := db.Query(ctx, "SELECT value FROM test").Iterator()
			assert.NoError(t, err)
			defer iter.Close()

			for iter.Next() {
				var i int
				iter.Row(i)
				v = append(v, i)
			}

			assert.Equal(t, 3, len(v))
		}
	})
}

func TestDel(t *testing.T) {
	runTests(t,
		func(db DB, ctx context.Context) {
			db.Exec(ctx, "CREATE TABLE test (value int)")
			db.Exec(ctx, "INSERT INTO test VALUES (?)", 1)

			n, err := db.ExecNum(ctx, "DELETE FROM test WHERE value = ?", 1)
			assert.NoError(t, err)
			assert.Equal(t, 1, int(n))

			n, err = db.ExecNum(ctx, "DELETE FROM test WHERE value = ?", 1)
			assert.NoError(t, err)
			assert.Equal(t, 0, int(n))

			err = db.ExecNumErr(ctx, "DELETE FROM test WHERE value = ?", 1)
			assert.Error(t, err)
		},
	)
}

func TestExecNum(t *testing.T) {
	runTests(t,
		func(db DB, ctx context.Context) {
			db.Exec(ctx, "CREATE TABLE test (value int)")

			n, err := db.ExecNum(ctx, "INSERT INTO test VALUES (?)", 1)
			assert.NoError(t, err)
			assert.Equal(t, 1, int(n))

			_, err = db.ExecNum(ctx, "UPDATE test SET value=? WHERE value=?", 2, 1)
			assert.NoError(t, err)
			assert.Equal(t, 1, int(n))

			_, err = db.ExecNum(ctx, "DELETE FROM test  WHERE value=?", 2)
			assert.NoError(t, err)
			assert.Equal(t, 1, int(n))
		},
	)
}

func TestQueryRowStruct(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		type test struct {
			X int64
		}

		err := db.AutoMigrate(ctx, &test{})
		assert.NoError(t, err)

		err = db.Insert(ctx, &test{1})
		assert.NoError(t, err)

		{
			v := test{}
			err := db.Query(ctx, "SELECT * FROM test").Row(&v)
			assert.NoError(t, err)
			assert.Equal(t, test{1}, v)
		}

		{
			v := test{2}
			err := db.Query(ctx, "SELECT * FROM test").Row(&v)
			assert.NoError(t, err)
			assert.Equal(t, test{1}, v)
		}

		{
			var v *test // nil ptr
			err := db.Query(ctx, "SELECT * FROM test").Row(&v)
			assert.NoError(t, err)
			assert.Equal(t, test{1}, *v)
		}

		{
			var v *int
			err := db.Query(ctx, "SELECT x FROM test").Row(&v)
			assert.NoError(t, err)
			assert.Equal(t, 1, *v)
		}
	}, func(db DB, ctx context.Context) {
		type test struct {
			X *int64
			Y *int64
		}

		err := db.AutoMigrate(ctx, &test{})
		assert.NoError(t, err)

		err = db.Insert(ctx, &test{util.Int64(1), nil})
		assert.NoError(t, err)

		{
			v := test{util.Int64(2), util.Int64(2)}
			err := db.Query(ctx, "SELECT * FROM test").Row(&v)
			assert.NoError(t, err)
			assert.Equal(t, test{util.Int64(1), nil}, v)
		}

		{
			v := test{util.Int64(2), util.Int64(2)}
			err := db.Query(ctx, "SELECT x FROM test").Row(&v)
			assert.NoError(t, err)
			assert.Equal(t, test{util.Int64(1), util.Int64(2)}, v)
		}

		{
			var v *test // nil ptr
			err := db.Query(ctx, "SELECT * FROM test").Row(&v)
			assert.NoError(t, err)
			assert.Equal(t, test{util.Int64(1), nil}, *v)
		}

		{
			var v *test
			err := db.Query(ctx, "SELECT * FROM test where 1 = 2").Row(&v)
			assert.Error(t, err)
			assert.Nil(t, v)
		}
	}, func(db DB, ctx context.Context) {
		type test struct {
			X int64
		}

		// CREATE TABLE `test` (`x` integer,`point_y` integer)
		err := db.AutoMigrate(ctx, &test{})
		assert.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO test VALUES (?), (?), (?)", 1, 2, 3)
		assert.NoError(t, err)

		var v []test
		err = db.Query(ctx, "SELECT * FROM test").Rows(&v)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(v))
	})
}

func TestPing(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		err := db.SqlDB().Ping()
		assert.NoError(t, err)
	})
}

func TestSqlArg(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		db.Exec(ctx, "CREATE TABLE test (value int);")
		db.Exec(ctx, "INSERT INTO test VALUES (?);", 1)

		var v int
		db.Query(ctx, "SELECT value FROM test WHERE value=?;", 1).Row(&v)
		assert.Equal(t, 1, v)
	}, func(db DB, ctx context.Context) {
		a := 1
		db.Exec(ctx, "CREATE TABLE test (value int);")
		db.Exec(ctx, "INSERT INTO test VALUES (?);", &a)

		var v int
		db.Query(ctx, "SELECT value FROM test WHERE value=?;", &a).Row(&v)
		assert.Equal(t, 1, v)
	}, func(db DB, ctx context.Context) {
		type foo struct {
			PointX  *int
			PointY  *int `sql:"point_y"`
			Private *int `sql:"-"`
			private *int
		}
		x := 1

		db.Exec(ctx, "CREATE TABLE test (point_x int, point_y int)")
		db.Exec(ctx, "INSERT INTO test VALUES (?, ?)", &x, nil)

		v := foo{}
		db.Query(ctx, "SELECT * FROM test;").Row(&v)
		assert.Equal(t, v, foo{&x, nil, nil, nil})
	})
}

func TestTx(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		db.Exec(ctx, "CREATE TABLE test (value int)")

		tx, err := db.Begin()
		assert.NoError(t, err)

		a := 1
		_, err = tx.Exec(ctx, "INSERT INTO test VALUES (?);", &a)
		assert.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)

		var v int
		db.Query(ctx, "SELECT value FROM test WHERE value=?;", &a).Row(&v)
		assert.Equal(t, 1, v)
	}, func(db DB, ctx context.Context) {
		db.Exec(ctx, "CREATE TABLE test (value int)")

		tx, err := db.Begin()
		assert.NoError(t, err)

		a := 1
		_, err = tx.Exec(ctx, "INSERT INTO test VALUES (?);", &a)
		assert.NoError(t, err)

		err = tx.Rollback()
		assert.NoError(t, err)

		var v int
		db.Query(ctx, "SELECT value FROM test WHERE value=?;", &a).Row(&v)
		assert.Equal(t, 0, v)
	})
}

func TestTxWithContext(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		db.Exec(ctx, "CREATE TABLE test (value int)")

		tx, err := db.Begin()
		assert.NoError(t, err)

		a := 1
		_, err = db.Exec(WithDB(ctx, tx), "INSERT INTO test VALUES (?);", &a)
		assert.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)

		var v int
		db.Query(ctx, "SELECT value FROM test WHERE value=?;", &a).Row(&v)
		assert.Equal(t, 1, v)
	}, func(db DB, ctx context.Context) {
		db.Exec(ctx, "CREATE TABLE test (value int)")

		tx, err := db.Begin()
		assert.NoError(t, err)

		a := 1
		_, err = db.Exec(WithDB(ctx, tx), "INSERT INTO test VALUES (?);", &a)
		assert.NoError(t, err)

		err = tx.Rollback()
		assert.NoError(t, err)

		var v int
		db.Query(ctx, "SELECT value FROM test WHERE value=?;", &a).Row(&v)
		assert.Equal(t, 0, v)
	})
}

func TestCRUD(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
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

		// mysql: CREATE TABLE `test` (`name` varchar(255),`age` bigint,`address` varchar(255),`nick_name` varchar(1024),`created_at` datetime NULL,`updated_at` datetime NULL)
		err := db.AutoMigrate(ctx, &test{})
		assert.NoError(t, err)

		// create
		user := test{Name: "tom", Age: 14}
		err = db.Insert(ctx, &user)
		assert.NoError(t, err)

		user.CreatedAt = createdAt
		user.UpdatedAt = createdAt

		// read
		var got test
		err = db.Get(ctx, &got, WithSelector("name=tom"))
		assert.NoError(t, err)
		assert.Equalf(t, user, got, "user get")

		// update
		c.SetTime(updatedAt)
		user = test{Name: "tom", Age: 14, Address: "beijing", NickName: util.String("t")}
		err = db.Update(ctx, &user)
		assert.NoError(t, err)
		user.CreatedAt = createdAt
		user.UpdatedAt = updatedAt

		got = test{}
		err = db.Get(ctx, &got, WithSelector("name=tom"))
		assert.NoError(t, err)
		assert.Equalf(t, user, got, "user get")

		// delete
		user = test{Name: "tom"}
		err = db.Delete(ctx, &user, WithSelector("name=tom"))
		assert.NoError(t, err)
	})
}

func TestTypeRaw(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		type test struct {
			Id           int
			Int          int
			Int8         int8
			Int16        int16
			Int32        int32
			Int64        int64
			Uint         uint
			UInt8        uint8
			Uint16       uint16
			Uint32       uint32
			Uint64       uint64
			String       string
			Byte         byte
			Bool         bool
			Time         time.Time
			SliceString  []string
			SliceStringP []*string
			MapString    map[string]string
		}

		err := db.AutoMigrate(ctx, &test{})
		assert.NoError(t, err)

		ts := time.Unix(1000, 0).UTC()

		cases := []struct {
			Key  string
			Data test
		}{{
			Key:  "1",
			Data: test{Id: 1, Time: ts},
		}, {
			Key: "2",
			Data: test{
				Id:           2,
				Int:          1,
				Int8:         2,
				Int16:        3,
				Int32:        4,
				Int64:        5,
				Uint:         6,
				UInt8:        7,
				Uint16:       8,
				Uint32:       9,
				Uint64:       10,
				String:       "a",
				Byte:         'b',
				Bool:         true,
				Time:         ts,
				SliceString:  []string{"a", "b"},
				SliceStringP: []*string{util.String("a"), util.String("b")},
				MapString:    map[string]string{"a": "a", "b": "b"},
			},
		}}

		for _, c := range cases {
			err := db.Insert(ctx, &c.Data)
			if err != nil {
				t.Errorf("%v: error %v", c.Key, err)
			}

			var got test
			err = db.Query(ctx, "SELECT * FROM test where id=?", c.Data.Id).Row(&got)
			if err != nil {
				t.Errorf("%v: error %v", c.Key, err)
			}

			assert.Equal(t, c.Data, got)
		}
	})
}

func TestTypePtr(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		type test struct {
			Id       int
			IntP     *int
			Int8P    *int8
			Int16P   *int16
			Int32P   *int32
			Int64P   *int64
			UintP    *uint
			UInt8P   *uint8
			Uit16P   *uint16
			Uint32P  *uint32
			Uint64P  *uint64
			StringP  *string
			StringPP **string
			ByteP    *byte
			BoolP    *bool
			TimeP    *time.Time
		}

		err := db.AutoMigrate(ctx, &test{})
		assert.NoError(t, err)

		ts := time.Unix(1000, 0).UTC()
		stringP := util.String("a")
		emptyStringP := util.String("")

		cases := []struct {
			Key  string
			Data test
		}{{
			Key:  "1",
			Data: test{Id: 1},
		}, {
			Key: "2",
			Data: test{
				Id:       2,
				IntP:     util.Int(1),
				Int8P:    util.Int8(2),
				Int16P:   util.Int16(3),
				Int32P:   util.Int32(4),
				Int64P:   util.Int64(5),
				UintP:    util.Uint(6),
				UInt8P:   util.Uint8(7),
				Uit16P:   util.Uint16(8),
				Uint32P:  util.Uint32(9),
				Uint64P:  util.Uint64(10),
				StringP:  stringP,
				StringPP: &stringP,
				ByteP:    util.Byte('b'),
				BoolP:    util.Bool(true),
				TimeP:    util.Time(ts),
			},
		}, {
			Key: "3",
			Data: test{
				Id:       3,
				IntP:     util.Int(0),
				Int8P:    util.Int8(0),
				Int16P:   util.Int16(0),
				Int32P:   util.Int32(0),
				Int64P:   util.Int64(0),
				UintP:    util.Uint(0),
				UInt8P:   util.Uint8(0),
				Uit16P:   util.Uint16(0),
				Uint32P:  util.Uint32(0),
				Uint64P:  util.Uint64(0),
				StringP:  emptyStringP,
				StringPP: &emptyStringP,
				ByteP:    util.Byte(0),
				BoolP:    util.Bool(false),
				TimeP:    util.Time(ts),
			},
		}}

		for _, c := range cases {
			err := db.Insert(ctx, &c.Data)
			if err != nil {
				t.Errorf("%v: error %v", c.Key, err)
			}

			var got test
			err = db.Query(ctx, "SELECT * FROM test where id=?", c.Data.Id).Row(&got)
			if err != nil {
				t.Errorf("%v: error %v", c.Key, err)
			}

			assert.Equal(t, c.Data, got)
		}
	})

}

type (
	CustomInt    int
	CustomString string
	CustomBytes  []byte
	CustomTime   time.Time
)

func (t *CustomTime) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*time.Time)(t))
}
func (t CustomTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t))
}

func TestTypeCustom(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		type Point struct {
			X int
			Y int
		}
		type test struct {
			Id           int
			CustomInt    CustomInt
			CustomString CustomString
			CustomBytes  CustomBytes
			CustomTime   CustomTime
			Point        Point
			PointP       *Point
		}

		err := db.AutoMigrate(ctx, &test{})
		assert.NoError(t, err)

		ts := time.Unix(1000, 0)

		cases := []struct {
			Key  string
			Data test
		}{{
			Key:  "1",
			Data: test{Id: 1},
		}, {
			Key: "2",
			Data: test{
				Id:           2,
				CustomInt:    CustomInt(1),
				CustomString: CustomString("2"),
				CustomBytes:  CustomBytes("3"),
				CustomTime:   CustomTime(ts),
				Point:        Point{X: 1, Y: 2},
				PointP:       &Point{X: 1, Y: 2},
			},
		}}

		for _, c := range cases {
			err := db.Insert(ctx, &c.Data)
			if err != nil {
				t.Errorf("%v: error %v", c.Key, err)
			}

			var got test
			err = db.Query(ctx, "SELECT * FROM test where id=?", c.Data.Id).Row(&got)
			if err != nil {
				t.Errorf("%v: error %v", c.Key, err)
			}
			assert.Equalf(t, c.Data, got, "custom type")
		}
	})
}

func TestTime(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
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
		err := db.AutoMigrate(ctx, &v)
		assert.NoError(t, err)

		err = db.Insert(ctx, &v)
		assert.NoError(t, err)

		v = test{}
		err = db.Get(ctx, &v, WithSelector("name=test"))
		assert.NoError(t, err)

		assert.Equal(t, createdAt.Unix(), v.TimeSec, "time sec")
		assert.Equal(t, createdAt.UnixMilli(), v.TimeMilli, "time milli")
		assert.Equal(t, createdAt.UnixNano(), v.TimeNano, "time nano")
		assert.WithinDuration(t, createdAt, v.Time, time.Second, "time")
		assert.WithinDuration(t, createdAt, *v.TimeP, time.Second, "time")

		c.SetTime(updatedAt)
		err = db.Update(ctx, &v)
		assert.NoError(t, err)

		v = test{}
		err = db.Get(ctx, &v, WithSelector("name=test"))
		assert.NoError(t, err)
		assert.Equal(t, updatedAt.Unix(), v.TimeSec, "time sec")
		assert.Equal(t, updatedAt.UnixMilli(), v.TimeMilli, "time milli")
		assert.Equal(t, updatedAt.UnixNano(), v.TimeNano, "time nano")
		assert.WithinDuration(t, updatedAt, v.Time, time.Second, "time")
		assert.WithinDuration(t, updatedAt, *v.TimeP, time.Second, "time")
	}, func(db DB, ctx context.Context) {
		type test struct {
			Time time.Time
		}

		t1 := time.Unix(1000, 0).UTC()
		v := test{Time: t1}
		err := db.AutoMigrate(ctx, &v)
		assert.NoError(t, err)

		err = db.Insert(ctx, &v)
		assert.NoError(t, err)

		v = test{}
		err = db.Query(ctx, "select * from test where time = ?", t1).Row(&v)
		assert.NoError(t, err)
		assert.Equal(t, t1, v.Time)

		err = db.Query(ctx, "select * from test where time > ?", t1.Add(-time.Second)).Row(&v)
		assert.NoError(t, err)
		assert.Equal(t, t1, v.Time)

		err = db.Query(ctx, "select * from test where time < ?", t1.Add(time.Second)).Row(&v)
		assert.NoError(t, err)
		assert.Equal(t, t1, v.Time)
	})
}
