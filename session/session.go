package session

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/clock"
	"google.golang.org/grpc/codes"
)

type storage interface {
	all() int
	get(sid string) (*session, error)
	insert(*session) error
	del(sid string) error
	update(*session) error
}

type Config struct {
	CookieName     string `json:"cookieName"`
	SidLength      int    `json:"sidLength"`
	HttpOnly       bool   `json:"httpOnly"`
	Domain         string `json:"domain"`
	GcInterval     int64  `json:"gcInterval"`
	CookieLifetime int64  `json:"cookieLifetime"`
	MaxIdleTime    int64  `json:"maxIdleTime" description:"session timeout"`
	DbDriver       string `json:"dbDriver"`
	Dsn            string `json:"dsn"`
	Storage        string `json:"storage" description:"mem|db(defualt)"`
}

func (p *Config) Validate() error {
	if p.CookieLifetime == 0 {
		p.CookieLifetime = 24 * 3600
	}

	if p.SidLength == 0 {
		p.SidLength = 32
	}

	if p.MaxIdleTime == 0 {
		p.MaxIdleTime = 3600
	}

	return nil
}

func StartSession(cf *Config, opts ...Option) (*Manager, error) {
	sopts := &options{}

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

	return &Manager{
		storage: storage,
		config:  cf,
		options: sopts,
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

type Manager struct {
	storage
	options *options
	config  *Config
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (p *Manager) Start(w http.ResponseWriter, r *http.Request) (store *SessionStore, err error) {
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
		// cookie.Expires = p.clock.Now().Add(time.Duration(p.config.CookieLifetime) * time.Second)
	}
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	return
}

func (p *Manager) StopGC() {
	if p.options.cancel != nil {
		p.options.cancel()
	}
}

func (p *Manager) Destroy(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return status.Error(codes.Unauthenticated, "Have not login yet")
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	p.del(sid)

	cookie = &http.Cookie{Name: p.config.CookieName,
		Path:     "/",
		HttpOnly: p.config.HttpOnly,
		Expires:  p.options.clock.Now(),
		MaxAge:   -1}

	http.SetCookie(w, cookie)
	return nil
}

func (p *Manager) Get(sid string) (*SessionStore, error) {
	return p.getSessionStore(sid, true)
}

func (p *Manager) Exist(sid string) bool {
	_, err := p.get(sid)
	return !status.NotFound(err)
}

// All count values in mysql session
func (p *Manager) All() int {
	return p.all()
}

func (p *Manager) getSid(r *http.Request) (sid string, err error) {
	var cookie *http.Cookie

	cookie, err = r.Cookie(p.config.CookieName)
	if err != nil || cookie.Value == "" {
		return sid, nil
	}

	return url.QueryUnescape(cookie.Value)
}

func (p *Manager) getSessionStore(sid string, create bool) (*SessionStore, error) {
	sc, err := p.get(sid)
	if status.NotFound(err) && create {
		ts := p.options.clock.Now().Unix()
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
	return &SessionStore{manager: p, connect: sc}, nil
}

// SessionStore mysql session store
type SessionStore struct {
	sync.RWMutex
	connect *session
	manager *Manager
}

// Set value in mysql session.
// it is temp value in map.
func (p *SessionStore) Set(key, value string) error {
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
func (p *SessionStore) Get(key string) string {
	p.RLock()
	defer p.RUnlock()

	switch strings.ToLower(key) {
	case "username":
		return p.connect.UserName
	default:
		return p.connect.Data[key]
	}
}

func (p *SessionStore) CreatedAt() int64 {
	return p.connect.CreatedAt
}

// Delete value in mysql session
func (p *SessionStore) Delete(key string) error {
	p.Lock()
	defer p.Unlock()
	delete(p.connect.Data, key)
	return nil
}

// Reset clear all values in mysql session
func (p *SessionStore) Reset() error {
	p.Lock()
	defer p.Unlock()
	p.connect.UserName = ""
	p.connect.Data = make(map[string]string)
	return nil
}

// Sid get session id of this mysql session store
func (p *SessionStore) Sid() string {
	return p.connect.Sid
}

func (p *SessionStore) Update(w http.ResponseWriter) error {
	p.connect.UpdatedAt = p.manager.options.clock.Now().Unix()
	return p.manager.update(p.connect)
}
