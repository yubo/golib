package session

// mysql session support need create table as sql:
// CREATE TABLE `session` (
//   `sid`			char(128)		NOT NULL,
//   `data`			blob			NULL,
//   `cookie_name`		char(128) 		DEFAULT '',
//   `created_at`		integer unsigned 	DEFAULT '0',
//   `updated_at`		integer unsigned 	DEFAULT '0'	NOT NULL,
//   PRIMARY KEY (`session_key`),
//   KEY (`cookie_name`),
//   KEY (`updated_at`)
// ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8 COLLATE = utf8_unicode_ci
// COMMENT = '[auth] session';

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc/codes"
	//_ "github.com/go-sql-driver/mysql"
)

const (
	TABLE_NAME       = "session"
	CREATE_TABLE_SQL = "CREATE TABLE `session` (" +
		"   `sid`		char(128)		NOT NULL," +
		"   `data`		blob			NULL," +
		"   `cookie_name`	char(128)		DEFAULT ''," +
		"   `created_at`	integer unsigned	DEFAULT '0'," +
		"   `updated_at`	integer unsigned	DEFAULT '0' NOT NULL," +
		"   PRIMARY KEY (`session_key`)," +
		"   KEY (`cookie_name`)," +
		"   KEY (`updated_at`)" +
		" ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8" +
		" COLLATE = utf8_unicode_ci COMMENT = '[auth] session';"
)

var (
	now = time.Now().Unix
)

type Session struct {
	config Config
	db     *orm.Db
}

type Config struct {
	CookieName     string `json:"cookieName"`
	SidLength      int    `json:"sidLength"`
	HttpOnly       bool   `json:"httpOnly"`
	Domain         string `json:"domain"`
	DbDriver       string `json:"dbDriver"`
	Dsn            string `json:"dsn"`
	GcInterval     int64  `json:"gcInterval"`
	CookieLifetime int64  `json:"cookieLifetime"`
}

func StartSession(cf Config, ctx context.Context) (*Session, error) {
	if cf.CookieLifetime == 0 {
		cf.CookieLifetime = cf.GcInterval
	}

	if cf.SidLength == 0 {
		cf.SidLength = 32
	}

	db, err := orm.DbOpenWithCtx(cf.DbDriver, cf.Dsn, ctx)
	if err != nil {
		return nil, err
	}

	if err := db.Db.Ping(); err != nil {
		return nil, err
	}

	util.Until(func() {
		db.Exec("delete from "+TABLE_NAME+" where updated_at<? and cookie_name=?",
			now()-cf.CookieLifetime, cf.CookieName)
	}, time.Duration(cf.GcInterval)*time.Second, ctx.Done())

	return &Session{config: cf, db: db}, nil
}

func StartSessionWithDb(cf Config, ctx context.Context, db *orm.Db) (*Session, error) {
	if cf.CookieLifetime == 0 {
		cf.CookieLifetime = cf.GcInterval
	}

	if cf.SidLength == 0 {
		cf.SidLength = 32
	}

	util.Until(func() {
		db.Exec("delete from "+TABLE_NAME+" where updated_at<? and cookie_name=?",
			now()-cf.CookieLifetime, cf.CookieName)
	}, time.Duration(cf.GcInterval)*time.Second, ctx.Done())

	return &Session{config: cf, db: db}, nil
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (p *Session) Start(w http.ResponseWriter, r *http.Request) (store *SessionStore, err error) {
	var sid string

	if sid, err = p.getSid(r); err != nil {
		return
	}

	if sid != "" {
		if store, err := getSessionStore(p.db, sid, false); err == nil {
			return store, nil
		}
	}

	// Generate a new session
	sid = util.RandString(p.config.SidLength)

	store, err = getSessionStore(p.db, sid, true)
	if err != nil {
		return nil, err
	}
	cookie := &http.Cookie{
		Name:     p.config.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: p.config.HttpOnly,
		Domain:   p.config.Domain,
	}
	if p.config.CookieLifetime > 0 {
		cookie.MaxAge = int(p.config.CookieLifetime)
		cookie.Expires = time.Now().Add(time.Duration(p.config.CookieLifetime) * time.Second)
	}
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	return
}

func (p *Session) Destroy(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return status.Error(codes.Unauthenticated, "Have not login yet")
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	deleteSession(p.db, sid)

	cookie = &http.Cookie{Name: p.config.CookieName,
		Path:     "/",
		HttpOnly: p.config.HttpOnly,
		Expires:  time.Now(),
		MaxAge:   -1}

	http.SetCookie(w, cookie)
	return nil
}

func (p *Session) Get(sid string) (*SessionStore, error) {
	return getSessionStore(p.db, sid, true)
}

func (p *Session) Exist(sid string) bool {
	_, err := getSession(p.db, sid)
	return !status.NotFound(err)
}

// All count values in mysql session
func (p *Session) All() (ret int) {
	p.db.Query("select count(*) from where cookie_name=?"+TABLE_NAME,
		p.config.CookieName).Row(&ret)
	return
}

func (p *Session) getSid(r *http.Request) (sid string, err error) {
	var cookie *http.Cookie

	cookie, err = r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return sid, nil
	}

	return url.QueryUnescape(cookie.Value)
}

// SessionStore mysql session store
type SessionStore struct {
	session
	sync.RWMutex
	db *orm.Db
}

type session struct {
	Sid        string `sql:"sid,where"`
	Data       map[string]interface{}
	CookieName string
	CreatedAt  int64
	UpdatedAt  int64
}

// Set value in mysql session.
// it is temp value in map.
func (p *SessionStore) Set(key string, value interface{}) error {
	p.Lock()
	defer p.Unlock()
	p.Data[key] = value
	return nil
}

// Get value from mysql session
func (p *SessionStore) Get(key string) interface{} {
	p.RLock()
	defer p.RUnlock()
	if v, ok := p.Data[key]; ok {
		return v
	}
	return nil
}

func (p *SessionStore) CreatedAt() int64 {
	return p.session.CreatedAt
}

// Delete value in mysql session
func (p *SessionStore) Delete(key string) error {
	p.Lock()
	defer p.Unlock()
	delete(p.Data, key)
	return nil
}

// Flush clear all values in mysql session
func (p *SessionStore) Flush() error {
	p.Lock()
	defer p.Unlock()
	p.Data = make(map[string]interface{})
	return nil
}

// Sid get session id of this mysql session store
func (p *SessionStore) Sid() string {
	return p.session.Sid
}

func (p *SessionStore) Update(w http.ResponseWriter) error {
	p.UpdatedAt = now()
	return p.db.Update(TABLE_NAME, p.session)
}

func getSession(db *orm.Db, sid string) (ret *session, err error) {
	err = db.Query("select * from "+TABLE_NAME+" where sid=?", sid).Row(&ret)
	return
}

func createSession(db *orm.Db, sid string) (*session, error) {
	ts := now()
	ret := &session{
		Sid:       sid,
		CreatedAt: ts,
		UpdatedAt: ts,
		Data:      make(map[string]interface{}),
	}
	if err := db.Insert(TABLE_NAME, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func deleteSession(db *orm.Db, sid string) error {
	return db.ExecNumErr("DELETE FROM "+TABLE_NAME+" where sid=?", sid)
}

func getSessionStore(db *orm.Db, sid string, create bool) (*SessionStore, error) {
	sess, err := getSession(db, sid)
	if status.NotFound(err) && create {
		sess, err = createSession(db, sid)
	}
	if err != nil {
		return nil, err
	}
	return &SessionStore{db: db, session: *sess}, nil
}
