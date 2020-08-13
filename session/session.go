package session

// mysql session support need create table as sql:
//	CREATE TABLE `session` (
//	`session_key` char(64) NOT NULL,
//	`session_data` blob,
//	`time` int(11) unsigned NOT NULL,
//	PRIMARY KEY (`session_key`)
//	) ENGINE=MyISAM DEFAULT CHARSET=utf8;
//
import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	//_ "github.com/go-sql-driver/mysql"
)

const (
	TABLE_NAME = "session"
)

type Session struct {
	config SessionConfig
	db     *orm.Db
}

type SessionConfig struct {
	CookieName     string `json:"cookieName"`
	SidLength      int    `json:"sidLength"`
	HttpOnly       bool   `json:"httpOnly"`
	Domain         string `json:"domain"`
	DbDriver       string `json:"dbDriver"`
	Dsn            string `json:"dsn"`
	GcInterval     int64  `json:"gcInterval"`
	CookieLifetime int64  `json:"cookieLifetime"`
}

func StartSession(cf SessionConfig, ctx context.Context) (*Session, error) {

	if cf.CookieLifetime == 0 {
		cf.CookieLifetime = cf.GcInterval
	}

	if cf.SidLength == 0 {
		cf.SidLength = 16
	}

	db, err := orm.DbOpenWithCtx(cf.DbDriver, cf.Dsn, ctx)
	if err != nil {
		return nil, err
	}

	if err := db.Db.Ping(); err != nil {
		return nil, err
	}

	util.Until(func() {
		db.Exec("delete from "+TABLE_NAME+" where time < ?", time.Now().Unix()-cf.CookieLifetime)
	}, time.Duration(cf.GcInterval)*time.Second, ctx.Done())

	return &Session{config: cf, db: db}, nil
}

func StartSessionWithDb(cf SessionConfig, ctx context.Context, db *orm.Db) (*Session, error) {
	if cf.CookieLifetime == 0 {
		cf.CookieLifetime = cf.GcInterval
	}

	if cf.SidLength == 0 {
		cf.SidLength = 16
	}

	util.Until(func() {
		db.Exec("delete from "+TABLE_NAME+" where time < ?", time.Now().Unix()-cf.CookieLifetime)
	}, time.Duration(cf.GcInterval)*time.Second, ctx.Done())

	return &Session{config: cf, db: db}, nil
}

func (p *Session) getSid(r *http.Request) (sid string, err error) {
	var cookie *http.Cookie

	cookie, err = r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return sid, nil
	}

	return url.QueryUnescape(cookie.Value)
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (p *Session) Start(w http.ResponseWriter, r *http.Request) (store *SessionStore, err error) {
	var sid string

	if sid, err = p.getSid(r); err != nil {
		return
	}

	if sid != "" && p.Exist(sid) {
		return p.Get(sid)
	}

	// Generate a new session
	sid = util.RandString(p.config.SidLength)

	store, err = p.Get(sid)
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

func (p *Session) Destroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	p.db.Exec("DELETE FROM "+TABLE_NAME+" where session_key=?", sid)

	cookie = &http.Cookie{Name: p.config.CookieName,
		Path:     "/",
		HttpOnly: p.config.HttpOnly,
		Expires:  time.Now(),
		MaxAge:   -1}

	http.SetCookie(w, cookie)
}

func (p *Session) Get(sid string) (*SessionStore, error) {
	var sessiondata []byte
	var kv map[string]interface{}
	var createdAt int64

	err := p.db.Query("select session_data, time from "+TABLE_NAME+" where session_key=?", sid).Row(&sessiondata, &createdAt)

	if status.NotFound(err) {
		p.db.Exec("insert into "+TABLE_NAME+"(`session_key`,`session_data`,`time`) values(?,?,?)",
			sid, "", time.Now().Unix())
	}

	if len(sessiondata) == 0 {
		kv = make(map[string]interface{})
	} else {
		err = json.Unmarshal(sessiondata, &kv)
		if err != nil {
			return nil, err
		}
	}

	return &SessionStore{db: p.db, sid: sid, values: kv, createdAt: createdAt}, nil

}

func (p *Session) Exist(sid string) bool {
	var sessiondata []byte
	err := p.db.Query("select session_data from "+TABLE_NAME+" where session_key=?", sid).Row(&sessiondata)
	return !status.NotFound(err)
}

// All count values in mysql session
func (p *Session) All() (ret int) {
	p.db.Query("select count(*) from " + TABLE_NAME).Row(&ret)
	return
}

// SessionStore mysql session store
type SessionStore struct {
	sync.RWMutex
	db        *orm.Db
	sid       string
	values    map[string]interface{}
	createdAt int64
}

// Set value in mysql session.
// it is temp value in map.
func (p *SessionStore) Set(key string, value interface{}) error {
	p.Lock()
	defer p.Unlock()
	p.values[key] = value
	return nil
}

// Get value from mysql session
func (p *SessionStore) Get(key string) interface{} {
	p.RLock()
	defer p.RUnlock()
	if v, ok := p.values[key]; ok {
		return v
	}
	return nil
}

func (p *SessionStore) CreatedAt() int64 {
	return p.createdAt
}

// Delete value in mysql session
func (p *SessionStore) Delete(key string) error {
	p.Lock()
	defer p.Unlock()
	delete(p.values, key)
	return nil
}

// Flush clear all values in mysql session
func (p *SessionStore) Flush() error {
	p.Lock()
	defer p.Unlock()
	p.values = make(map[string]interface{})
	return nil
}

// Sid get session id of this mysql session store
func (p *SessionStore) Sid() string {
	return p.sid
}

func (p *SessionStore) Update(w http.ResponseWriter) error {
	b, err := json.Marshal(p.values)
	if err != nil {
		return err
	}
	_, err = p.db.Exec("UPDATE "+TABLE_NAME+" set `session_data`=?, `time`=? where session_key=?",
		b, time.Now().Unix(), p.sid)
	return err
}
