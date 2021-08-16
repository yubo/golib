package session

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/yubo/golib/orm"
	_ "github.com/yubo/golib/orm/sqlite"
	"github.com/yubo/golib/util/clock"
)

var (
	driver    = "sqlite3"
	dsn       = "file:test.db?cache=shared&mode=memory"
	available bool
	db        *orm.DB
)

func init() {
	var err error

	if db, err = orm.DbOpen(driver, dsn); err == nil {
		if err = db.DB.Ping(); err == nil {
			available = true
		}
		db.Close()
	}
}

func mustExec(t *testing.T, db *orm.DB, query string, args ...interface{}) (res sql.Result) {
	res, err := db.Exec(query, args...)
	if err != nil {
		if len(query) > 300 {
			query = "[query too large to print]"
		}
		t.Fatalf("error on %s: %s", query, err.Error())
	}
	return res
}

func TestDbSession(t *testing.T) {
	var (
		sess  SessionManager
		store Session
		err   error
		sid   string
	)

	if !available {
		t.Skipf("SQL server not running on %s", dsn)
	}

	cf := &Config{
		CookieName:     "test_sid",
		SidLength:      24,
		HttpOnly:       true,
		Domain:         "",
		Dsn:            dsn,
		gcInterval:     time.Minute,
		cookieLifetime: 24 * time.Hour,
	}

	ctx, cancel := context.WithCancel(context.Background())
	db, _ := orm.DbOpenWithCtx(driver, dsn, ctx)

	if sess, err = StartSession(cf, WithCtx(ctx), WithDB(db)); err != nil {
		t.Fatalf("error NewSession: %s", err.Error())
	}
	defer cancel()

	mustExec(t, db, "DROP TABLE IF EXISTS session;")
	mustExec(t, db, CREATE_TABLE_SQLITE)
	defer db.Exec("DROP TABLE IF EXISTS session")

	r, _ := http.NewRequest("GET", "", bytes.NewBuffer([]byte{}))
	w := httptest.NewRecorder()

	if store, err = sess.Start(w, r); err != nil {
		t.Fatalf("session.Start(): %s", err.Error())
	}

	if n := sess.All(); n != 1 {
		t.Fatalf("sess.All() got %d want %d", n, 1)
	}

	store.Set("abc", "11223344")
	if err = store.Update(w); err != nil {
		t.Fatalf("store.Update(w) got err %s ", err.Error())
	}
	sid = store.Sid()

	// new request
	r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte{}))
	w = httptest.NewRecorder()

	cookie := &http.Cookie{
		Name:     cf.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: cf.HttpOnly,
		Domain:   cf.Domain,
	}
	if cf.CookieLifetime > 0 {
		cookie.Expires = time.Now().Add(cf.cookieLifetime)
	}
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	if store, err = sess.Start(w, r); err != nil {
		t.Fatalf("session.Start(): %s", err.Error())
	}

	if n := sess.All(); n != 1 {
		t.Fatalf("sess.All() got %d want %d", n, 1)
	}

	if v := store.Get("abc"); v != "11223344" {
		t.Fatalf("store.Get('abc') got %s want %s", v, "11223344")
	}

	store.Set("abc", "22334455")

	if v := store.Get("abc"); v != "22334455" {
		t.Fatalf("store.Get('abc') got %s want %s", v, "22334455")
	}

	sess.Destroy(w, r)
	if n := sess.All(); n != 0 {
		t.Fatalf("sess.All() got %d want %d", n, 0)
	}

}

func TestDbSessionGC(t *testing.T) {
	var (
		sess SessionManager
		err  error
	)

	if !available {
		t.Skipf("SQL server not running on %s", dsn)
	}

	cf := &Config{
		CookieName:     "test_sid",
		SidLength:      24,
		HttpOnly:       true,
		Domain:         "",
		Dsn:            dsn,
		gcInterval:     time.Second,
		cookieLifetime: 24 * time.Hour,
	}

	ctx, cancel := context.WithCancel(context.Background())
	db, _ := orm.DbOpenWithCtx(driver, dsn, ctx)
	clock := &clock.FakeClock{}
	clock.SetTime(time.Now())

	if sess, err = StartSession(cf, WithCtx(ctx), WithDB(db), WithClock(clock)); err != nil {
		t.Fatalf("error NewSession: %s", err.Error())
	}
	defer cancel()

	mustExec(t, db, "DROP TABLE IF EXISTS session;")
	mustExec(t, db, CREATE_TABLE_SQLITE)
	defer db.Exec("DROP TABLE IF EXISTS session")

	r, _ := http.NewRequest("GET", "", bytes.NewBuffer([]byte{}))
	w := httptest.NewRecorder()

	if _, err = sess.Start(w, r); err != nil {
		t.Fatalf("session.Start(): %s", err.Error())
	}
	if n := sess.All(); n != 1 {
		t.Fatalf("sess.All() got %d want %d", n, 1)
	}

	clock.SetTime(clock.Now().Add(time.Hour * 25))
	time.Sleep(100 * time.Millisecond)

	if n := sess.All(); n != 0 {
		t.Fatalf("sess.All() got %d want %d", n, 0)
	}

}

func TestMemSession(t *testing.T) {
	var (
		sess  SessionManager
		store Session
		err   error
		sid   string
	)

	cf := &Config{
		Storage:        "mem",
		CookieName:     "test_sid",
		SidLength:      24,
		HttpOnly:       true,
		Domain:         "",
		Dsn:            dsn,
		gcInterval:     time.Minute,
		cookieLifetime: 24 * time.Hour,
	}

	ctx, cancel := context.WithCancel(context.Background())

	if sess, err = StartSession(cf, WithCtx(ctx)); err != nil {
		t.Fatalf("error NewSession: %s", err.Error())
	}
	defer cancel()

	r, _ := http.NewRequest("GET", "", bytes.NewBuffer([]byte{}))
	w := httptest.NewRecorder()

	if store, err = sess.Start(w, r); err != nil {
		t.Fatalf("session.Start(): %s", err.Error())
	}

	if n := sess.All(); n != 1 {
		t.Fatalf("sess.All() got %d want %d", n, 1)
	}

	store.Set("abc", "11223344")
	if err = store.Update(w); err != nil {
		t.Fatalf("store.Update(w) got err %s ", err.Error())
	}
	sid = store.Sid()

	// new request
	r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte{}))
	w = httptest.NewRecorder()

	cookie := &http.Cookie{
		Name:     cf.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: cf.HttpOnly,
		Domain:   cf.Domain,
	}
	if cf.CookieLifetime > 0 {
		cookie.Expires = time.Now().Add(cf.cookieLifetime)
	}
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	if store, err = sess.Start(w, r); err != nil {
		t.Fatalf("session.Start(): %s", err.Error())
	}

	if n := sess.All(); n != 1 {
		t.Fatalf("sess.All() got %d want %d", n, 1)
	}

	if v := store.Get("abc"); v != "11223344" {
		t.Fatalf("store.Get('abc') got %s want %s", v, "11223344")
	}

	store.Set("abc", "22334455")

	if v := store.Get("abc"); v != "22334455" {
		t.Fatalf("store.Get('abc') got %s want %s", v, "22334455")
	}

	sess.Destroy(w, r)
	if n := sess.All(); n != 0 {
		t.Fatalf("sess.All() got %d want %d", n, 0)
	}

}

func TestMemSessionGC(t *testing.T) {
	var (
		sess SessionManager
		err  error
	)

	cf := &Config{
		Storage:        "mem",
		CookieName:     "test_sid",
		SidLength:      24,
		HttpOnly:       true,
		Domain:         "",
		Dsn:            dsn,
		gcInterval:     time.Second,
		cookieLifetime: 24 * time.Hour,
	}

	ctx, cancel := context.WithCancel(context.Background())
	clock := &clock.FakeClock{}
	clock.SetTime(time.Now())

	if sess, err = StartSession(cf, WithCtx(ctx), WithClock(clock)); err != nil {
		t.Fatalf("error NewSession: %s", err.Error())
	}
	defer cancel()

	r, _ := http.NewRequest("GET", "", bytes.NewBuffer([]byte{}))
	w := httptest.NewRecorder()

	if _, err = sess.Start(w, r); err != nil {
		t.Fatalf("session.Start(): %s", err.Error())
	}
	if n := sess.All(); n != 1 {
		t.Fatalf("sess.All() got %d want %d", n, 1)
	}

	clock.SetTime(clock.Now().Add(time.Hour * 25))
	time.Sleep(100 * time.Millisecond)

	if n := sess.All(); n != 0 {
		t.Fatalf("sess.All() got %d want %d", n, 0)
	}

}
