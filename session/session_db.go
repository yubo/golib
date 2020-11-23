package session

import (
	"fmt"
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
		"   PRIMARY KEY (`sid`)," +
		"   KEY (`cookie_name`)," +
		"   KEY (`updated_at`)" +
		" ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8;"
)

type dbSession struct {
	*sessionOptions
	config *Config
}

func startDbSession(cf *Config, opts *sessionOptions) (Session, error) {
	if opts.db == nil {
		db, err := orm.DbOpenWithCtx(cf.DbDriver, cf.Dsn, opts.ctx)
		if err != nil {
			return nil, err
		}

		if err := db.Db.Ping(); err != nil {
			return nil, err
		}
		opts.db = db
	}

	util.UntilWithTick(func() {
		opts.db.Exec("delete from "+TABLE_NAME+
			" where updated_at<? and cookie_name=?",
			opts.clock.Now().Unix()-cf.CookieLifetime, cf.CookieName)
	},
		opts.clock.NewTicker(time.Duration(cf.GcInterval)*time.Second).C(),
		opts.ctx.Done())

	return &dbSession{sessionOptions: opts, config: cf}, nil
}

// dbSessionStart generate or read the session id from http request.
// if session id exists, return dbSessionStore with this id.
func (p *dbSession) Start(w http.ResponseWriter, r *http.Request) (store SessionStore, err error) {
	var sid string

	if sid, err = p.getSid(r); err != nil {
		return
	}

	if sid != "" {
		if store, err := p.getSessionStore(sid, false); err == nil {
			return store, nil
		}
	}

	// Generate a new session
	sid = util.RandString(p.config.SidLength)

	store, err = p.getSessionStore(sid, true)
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
		cookie.Expires = p.clock.Now().Add(time.Duration(p.config.CookieLifetime) * time.Second)
	}
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	return
}

func (p *dbSession) StopGC() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *dbSession) Destroy(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return status.Error(codes.Unauthenticated, "Have not login yet")
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	p.delSession(sid)

	cookie = &http.Cookie{Name: p.config.CookieName,
		Path:     "/",
		HttpOnly: p.config.HttpOnly,
		Expires:  p.clock.Now(),
		MaxAge:   -1}

	http.SetCookie(w, cookie)
	return nil
}

func (p *dbSession) Get(sid string) (SessionStore, error) {
	return p.getSessionStore(sid, true)
}

func (p *dbSession) Exist(sid string) bool {
	_, err := p.getSession(sid)
	return !status.NotFound(err)
}

// All count values in mysql session
func (p *dbSession) All() (ret int) {
	err := p.db.Query("select count(*) from "+TABLE_NAME+" where cookie_name=?",
		p.config.CookieName).Row(&ret)
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	return
}

func (p *dbSession) getSid(r *http.Request) (sid string, err error) {
	var cookie *http.Cookie

	cookie, err = r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return sid, nil
	}

	return url.QueryUnescape(cookie.Value)
}

func (p *dbSession) getSession(sid string) (ret *session, err error) {
	err = p.db.Query("select * from "+TABLE_NAME+" where sid=?", sid).Row(&ret)
	return
}

func (p *dbSession) createSession(sid string) (*session, error) {
	ts := p.clock.Now().Unix()
	ret := &session{
		Sid:        sid,
		CookieName: p.config.CookieName,
		CreatedAt:  ts,
		UpdatedAt:  ts,
		Data:       make(map[string]interface{}),
	}
	if err := p.db.Insert(TABLE_NAME, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (p *dbSession) delSession(sid string) error {
	return p.db.ExecNumErr("DELETE FROM "+TABLE_NAME+" where sid=?", sid)
}

func (p *dbSession) getSessionStore(sid string, create bool) (SessionStore, error) {
	sess, err := p.getSession(sid)
	if status.NotFound(err) && create {
		sess, err = p.createSession(sid)
	}
	if err != nil {
		return nil, err
	}
	return &dbSessionStore{dbSession: p, session: sess}, nil
}

// dbSessionStore mysql session store
type dbSessionStore struct {
	sync.RWMutex
	*session
	*dbSession
}

// Set value in mysql session.
// it is temp value in map.
func (p *dbSessionStore) Set(key string, value interface{}) error {
	p.Lock()
	defer p.Unlock()
	p.Data[key] = value
	return nil
}

// Get value from mysql session
func (p *dbSessionStore) Get(key string) interface{} {
	p.RLock()
	defer p.RUnlock()
	if v, ok := p.Data[key]; ok {
		return v
	}
	return nil
}

func (p *dbSessionStore) CreatedAt() int64 {
	return p.session.CreatedAt
}

// Delete value in mysql session
func (p *dbSessionStore) Delete(key string) error {
	p.Lock()
	defer p.Unlock()
	delete(p.Data, key)
	return nil
}

// Flush clear all values in mysql session
func (p *dbSessionStore) Flush() error {
	p.Lock()
	defer p.Unlock()
	p.Data = make(map[string]interface{})
	return nil
}

// Sid get session id of this mysql session store
func (p *dbSessionStore) Sid() string {
	return p.session.Sid
}

func (p *dbSessionStore) Update(w http.ResponseWriter) error {
	p.UpdatedAt = p.clock.Now().Unix()
	return p.db.Update(TABLE_NAME, p.session)
}
