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

	"github.com/yubo/golib/util/clock"
	//_ "github.com/go-sql-driver/mysql"
)

type Session interface {
	Start(w http.ResponseWriter, r *http.Request) (SessionStore, error)
	Destroy(w http.ResponseWriter, r *http.Request) error
	Get(sid string) (SessionStore, error)
	Exist(sid string) bool
	All() (ret int)
	StopGC()
}

type SessionStore interface {
	Set(key string, value interface{}) error
	Get(key string) interface{}
	CreatedAt() int64
	Delete(key string) error
	Flush() error
	Sid() string
	Update(w http.ResponseWriter) error
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

func StartSession(cf *Config, opts ...SessionOption) (Session, error) {
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

	/*
		if sopts.mem {
			return startMemSession(cf, sopts)
		}
	*/

	return startDbSession(cf, sopts)
}

type session struct {
	Sid        string `sql:"sid,where"`
	Data       map[string]interface{}
	CookieName string
	CreatedAt  int64
	UpdatedAt  int64
}
