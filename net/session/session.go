package session

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/yubo/golib/staging/api/errors"
	"github.com/yubo/golib/staging/util/clock"
	"github.com/yubo/golib/util"
)

const (
	DefCookieName = "sid"
)

type storage interface {
	all() int
	get(sid string) (*session, error)
	insert(*session) error
	del(sid string) error
	update(*session) error
}

type Config struct {
	CookieName     string
	SidLength      int
	HttpOnly       bool
	Domain         string
	GcInterval     time.Duration
	CookieLifetime time.Duration
	MaxIdleTime    time.Duration `description:"session timeout"`
	DbDriver       string
	Dsn            string
	Storage        string `description:"mem|db(defualt)"`
}

func (p *Config) Validate() error {
	if p == nil {
		return nil
	}
	if p.CookieLifetime == 0 {
		p.CookieLifetime = 24 * time.Hour
	}

	if p.SidLength == 0 {
		p.SidLength = 32
	}

	if p.MaxIdleTime == 0 {
		p.MaxIdleTime = time.Hour
	}

	if p.Storage == "" {
		p.Storage = "db"
	}

	if p.CookieName == "" {
		p.CookieName = DefCookieName
	}

	if p.Storage != "db" && p.Storage != "mem" {
		return fmt.Errorf("storage %s is invalid, should be mem or db(default)", p.Storage)
	}

	return nil
}

type SessionManager interface {
	Start(w http.ResponseWriter, r *http.Request) (store Session, err error)
	StopGC()
	Destroy(w http.ResponseWriter, r *http.Request) error
	Get(sid string) (Session, error)
	All() int
}

type Session interface {
	Set(key, value string) error
	Get(key string) string
	CreatedAt() int64
	Delete(key string) error
	Reset() error
	Sid() string
	Update(w http.ResponseWriter) error
}

func StartSession(cf *Config, opts_ ...Option) (SessionManager, error) {
	opts := &options{}

	for _, opt := range opts_ {
		opt.apply(opts)
	}

	if opts.ctx == nil {
		opts.ctx, opts.cancel = context.WithCancel(context.Background())
	}

	if opts.clock == nil {
		opts.clock = clock.RealClock{}
	}

	var storage storage
	var err error
	if cf.Storage == "mem" {
		storage, err = newMemStorage(cf, opts)
	} else {
		storage, err = newDbStorage(cf, opts)
	}

	if err != nil {
		return nil, err
	}

	return &sessionManager{
		storage: storage,
		config:  cf,
		options: opts,
	}, nil
}

type session struct {
	Sid        string `sql:"sid,where"`
	UserName   string
	Data       map[string]string
	CookieName string
	CreatedAt  int64
	UpdatedAt  int64
}

type sessionManager struct {
	storage
	*options
	config *Config
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (p *sessionManager) Start(w http.ResponseWriter, r *http.Request) (store Session, err error) {
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
		cookie.MaxAge = int(p.config.CookieLifetime.Seconds())
	}
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	return
}

func (p *sessionManager) StopGC() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *sessionManager) Destroy(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return errors.NewUnauthorized("Have not login yet")
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

func (p *sessionManager) Get(sid string) (Session, error) {
	return p.getSessionStore(sid, true)
}

func (p *sessionManager) Exist(sid string) bool {
	_, err := p.get(sid)
	return !errors.IsNotFound(err)
}

// All count values in mysql session
func (p *sessionManager) All() int {
	return p.all()
}

func (p *sessionManager) getSid(r *http.Request) (sid string, err error) {
	var cookie *http.Cookie

	cookie, err = r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return sid, nil
	}

	return url.QueryUnescape(cookie.Value)
}

func (p *sessionManager) getSessionStore(sid string, create bool) (Session, error) {
	sc, err := p.get(sid)
	if errors.IsNotFound(err) && create {
		ts := p.clock.Now().Unix()
		sc = &session{
			Sid:        sid,
			CookieName: p.config.CookieName,
			CreatedAt:  ts,
			UpdatedAt:  ts,
			Data:       make(map[string]string),
		}
		err = p.insert(sc)
	}
	if err != nil {
		return nil, err
	}
	return &sessionStore{manager: p, connect: sc}, nil
}

// sessionStore mysql session store
type sessionStore struct {
	sync.RWMutex
	connect *session
	manager *sessionManager
}

// Set value in mysql session.
// it is temp value in map.
func (p *sessionStore) Set(key, value string) error {
	p.Lock()
	defer p.Unlock()

	switch strings.ToLower(key) {
	case "username":
		p.connect.UserName = value
	default:
		p.connect.Data[key] = value
	}
	return nil
}

// Get value from mysql session
func (p *sessionStore) Get(key string) string {
	p.RLock()
	defer p.RUnlock()

	switch strings.ToLower(key) {
	case "username":
		return p.connect.UserName
	default:
		return p.connect.Data[key]
	}
}

func (p *sessionStore) CreatedAt() int64 {
	return p.connect.CreatedAt
}

// Delete value in mysql session
func (p *sessionStore) Delete(key string) error {
	p.Lock()
	defer p.Unlock()
	delete(p.connect.Data, key)
	return nil
}

// Reset clear all values in mysql session
func (p *sessionStore) Reset() error {
	p.Lock()
	defer p.Unlock()
	p.connect.UserName = ""
	p.connect.Data = make(map[string]string)
	return nil
}

// Sid get session id of this mysql session store
func (p *sessionStore) Sid() string {
	return p.connect.Sid
}

func (p *sessionStore) Update(w http.ResponseWriter) error {
	p.connect.UpdatedAt = p.manager.clock.Now().Unix()
	return p.manager.update(p.connect)
}

type key int

const (
	sessionKey key = iota
)

// WithSession returns a copy of the parent context
// and associates it with an sessionStore.
func WithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// SessionFrom returns the sessionStore bound to the context, if any.
func SessionFrom(ctx context.Context) (session Session, ok bool) {
	session, ok = ctx.Value(sessionKey).(Session)
	return
}

/*
func Filter(manager SessionManager) restful.FilterFunction {
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		session, err := manager.Start(resp, req.Request)
		if err != nil {
			openapi.HttpWriteErr(resp, fmt.Errorf("session start err %s", err))
			return
		}
		ctx := WithSession(req.Request.Context(), session)
		req.Request.WithContext(ctx)

		chain.ProcessFilter(req, resp)

		session.Update(resp)
	}
}
*/
