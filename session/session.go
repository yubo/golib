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

	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/clock"
	"google.golang.org/grpc/codes"
	//_ "github.com/go-sql-driver/mysql"
)

type storage interface {
	all() int
	get(sid string) (*sessionConnect, error)
	insert(*sessionConnect) error
	del(sid string) error
	update(*sessionConnect) error
}

type Config struct {
	CookieName     string `json:"cookieName"`
	SidLength      int    `json:"sidLength"`
	HttpOnly       bool   `json:"httpOnly"`
	Domain         string `json:"domain"`
	GcInterval     int64  `json:"gcInterval"`
	CookieLifetime int64  `json:"cookieLifetime"`
	DbDriver       string `json:"dbDriver"`
	Dsn            string `json:"dsn"`
	Storage        string `json:"storage" description:"mem|db(defualt)"`
}

func (p *Config) Validate() error {
	if p.CookieLifetime == 0 {
		p.CookieLifetime = p.GcInterval
	}

	if p.SidLength == 0 {
		p.SidLength = 32
	}

	return nil
}

func StartSession(cf *Config, opts ...SessionOption) (*Session, error) {
	sopts := &sessionOptions{}

	for _, opt := range opts {
		opt.apply(sopts)
	}

	if sopts.ctx == nil {
		sopts.ctx, sopts.cancel = context.WithCancel(context.Background())
	}

	if sopts.clock == nil {
		sopts.clock = clock.RealClock{}
	}

	var storage storage
	var err error
	if cf.Storage == "mem" {
		storage, err = newMemStorage(cf, sopts)
	} else {
		storage, err = newDbStorage(cf, sopts)
	}

	if err != nil {
		return nil, err
	}

	return &Session{
		storage:        storage,
		config:         cf,
		sessionOptions: sopts,
	}, nil
}

type sessionConnect struct {
	Sid        string `sql:"sid,where"`
	Data       map[string]interface{}
	CookieName string
	CreatedAt  int64
	UpdatedAt  int64
}

type Session struct {
	storage
	*sessionOptions
	config *Config
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (p *Session) Start(w http.ResponseWriter, r *http.Request) (store *SessionStore, err error) {
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

func (p *Session) StopGC() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *Session) Destroy(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return status.Error(codes.Unauthenticated, "Have not login yet")
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	p.del(sid)

	cookie = &http.Cookie{Name: p.config.CookieName,
		Path:     "/",
		HttpOnly: p.config.HttpOnly,
		Expires:  p.clock.Now(),
		MaxAge:   -1}

	http.SetCookie(w, cookie)
	return nil
}

func (p *Session) Get(sid string) (*SessionStore, error) {
	return p.getSessionStore(sid, true)
}

func (p *Session) Exist(sid string) bool {
	_, err := p.get(sid)
	return !status.NotFound(err)
}

// All count values in mysql session
func (p *Session) All() int {
	return p.all()
}

func (p *Session) getSid(r *http.Request) (sid string, err error) {
	var cookie *http.Cookie

	cookie, err = r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return sid, nil
	}

	return url.QueryUnescape(cookie.Value)
}

func (p *Session) getSessionStore(sid string, create bool) (*SessionStore, error) {
	sc, err := p.get(sid)
	if status.NotFound(err) && create {
		ts := p.clock.Now().Unix()
		sc = &sessionConnect{
			Sid:        sid,
			CookieName: p.config.CookieName,
			CreatedAt:  ts,
			UpdatedAt:  ts,
			Data:       make(map[string]interface{}),
		}
		err = p.insert(sc)
	}
	if err != nil {
		return nil, err
	}
	return &SessionStore{Session: p, sessionConnect: sc}, nil
}

// SessionStore mysql session store
type SessionStore struct {
	sync.RWMutex
	*sessionConnect
	*Session
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
	return p.sessionConnect.CreatedAt
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
	return p.sessionConnect.Sid
}

func (p *SessionStore) Update(w http.ResponseWriter) error {
	p.UpdatedAt = p.clock.Now().Unix()
	return p.update(p.sessionConnect)
}
