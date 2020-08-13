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
	"github.com/yubo/golib/util"
)

var (
	dsn       string
	available bool
)

// See https://github.com/go-sql-driver/mysql/wiki/Testing
func init() {
	dsn = util.EnvDef("MYSQL_TEST_DSN", "root:12341234@tcp(localhost:3306)/test?parseTime=true&timeout=30s")
	if db, err := orm.DbOpen("mysql", dsn); err == nil {
		if err = db.Db.Ping(); err == nil {
			available = true
		}
		db.Close()
	}
}

func mustExec(t *testing.T, db *orm.Db, query string, args ...interface{}) (res sql.Result) {
	res, err := db.Exec(query, args...)
	if err != nil {
		if len(query) > 300 {
			query = "[query too large to print]"
		}
		t.Fatalf("error on %s: %s", query, err.Error())
	}
	return res
}

func TestSession(t *testing.T) {
	var (
		sess  *Session
		err   error
		store *SessionStore
		sid   string
	)

	if !available {
		t.Skipf("MySQL server not running on %s", dsn)
	}

	cf := SessionConfig{
		CookieName:     "test_sid",
		SidLength:      24,
		HttpOnly:       true,
		Domain:         "",
		Dsn:            dsn,
		GcInterval:     60,
		CookieLifetime: 86400,
	}

	ctx, cancel := context.WithCancel(context.Background())
	if sess, err = StartSession(cf, ctx); err != nil {
		t.Fatalf("error NewSession: %s", err.Error())
	}
	defer cancel()

	mustExec(t, sess.db, "DROP TABLE IF EXISTS session;")
	mustExec(t, sess.db, "CREATE TABLE `session` ( `session_key` char(64) NOT NULL, `session_data` blob, `time` int(11) unsigned NOT NULL, PRIMARY KEY (`session_key`)) ENGINE=MyISAM DEFAULT CHARSET=utf8; ")
	defer sess.db.Exec("DROP TABLE IF EXISTS session")

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
		cookie.Expires = time.Now().Add(time.Duration(cf.CookieLifetime) * time.Second)
	}
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	if store, err = sess.Start(w, r); err != nil {
		t.Fatalf("session.Start(): %s", err.Error())
	}

	if n := sess.All(); n != 1 {
		t.Fatalf("sess.All() got %d want %d", n, 1)
	}

	if v, ok := store.Get("abc").(string); !(ok && v == "11223344") {
		t.Fatalf("store.Get('abc') got %s want %s", v, "11223344")
	}

	store.Set("abc", "22334455")

	if v, ok := store.Get("abc").(string); !(ok && v == "22334455") {
		t.Fatalf("store.Get('abc') got %s want %s", v, "22334455")
	}

	sess.Destroy(w, r)
	if n := sess.All(); n != 0 {
		t.Fatalf("sess.All() got %d want %d", n, 0)
	}

}

func TestSessionGC(t *testing.T) {
	var (
		sess *Session
		err  error
	)

	if !available {
		t.Skipf("MySQL server not running on %s", dsn)
	}

	cf := SessionConfig{
		CookieName:     "test_sid",
		SidLength:      24,
		HttpOnly:       true,
		Domain:         "",
		Dsn:            dsn,
		GcInterval:     1,
		CookieLifetime: 86400,
	}

	ctx, cancel := context.WithCancel(context.Background())
	if sess, err = StartSession(cf, ctx); err != nil {
		t.Fatalf("error NewSession: %s", err.Error())
	}
	defer cancel()

	mustExec(t, sess.db, "DROP TABLE IF EXISTS session;")
	mustExec(t, sess.db, "CREATE TABLE `session` ( `session_key` char(64) NOT NULL, `session_data` blob, `time` int(11) unsigned NOT NULL, PRIMARY KEY (`session_key`)) ENGINE=MyISAM DEFAULT CHARSET=utf8; ")
	//defer sess.db.Exec("DROP TABLE IF EXISTS session")

	r, _ := http.NewRequest("GET", "", bytes.NewBuffer([]byte{}))
	w := httptest.NewRecorder()

	if _, err = sess.Start(w, r); err != nil {
		t.Fatalf("session.Start(): %s", err.Error())
	}
	if n := sess.All(); n != 1 {
		t.Fatalf("sess.All() got %d want %d", n, 1)
	}

	time.Sleep(time.Millisecond * 3000)
	if n := sess.All(); n != 0 {
		t.Fatalf("sess.All() got %d want %d", n, 0)
	}

}
