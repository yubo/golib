package session

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/util"
)

const (
	TABLE_NAME       = "session"
	CREATE_TABLE_SQL = "CREATE TABLE `session` (" +
		"   `sid` char(128) NOT NULL," +
		"   `data` blob NULL," +
		"   `cookie_name` char(128) DEFAULT ''," +
		"   `created_at` integer unsigned DEFAULT '0'," +
		"   `updated_at` integer unsigned DEFAULT '0' NOT NULL," +
		"   PRIMARY KEY (`sid`)," +
		"   KEY (`cookie_name`)," +
		"   KEY (`updated_at`)" +
		" ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8;"
)

func newDbStorage(cf *Config, opts *sessionOptions) (storage, error) {
	st := &dbStorage{config: cf, db: opts.db}

	if st.db == nil {
		var err error
		st.db, err = orm.DbOpenWithCtx(cf.DbDriver, cf.Dsn, opts.ctx)
		if err != nil {
			return nil, err
		}
	}

	if err := st.db.Db.Ping(); err != nil {
		return nil, err
	}

	util.UntilWithTick(func() {
		st.db.Exec("delete from "+
			TABLE_NAME+" where updated_at<? and cookie_name=?",
			opts.clock.Now().Unix()-cf.CookieLifetime, cf.CookieName)
	},
		opts.clock.NewTicker(time.Duration(cf.GcInterval)*time.Second).C(),
		opts.ctx.Done())

	return st, nil
}

type dbStorage struct {
	db     *orm.Db
	config *Config
}

func (p *dbStorage) all() (ret int) {
	err := p.db.Query("select count(*) from "+TABLE_NAME+" where cookie_name=?",
		p.config.CookieName).Row(&ret)
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	return
}

func (p *dbStorage) get(sid string) (ret *sessionConnect, err error) {
	err = p.db.Query("select * from "+TABLE_NAME+" where sid=?", sid).Row(&ret)
	return
}

func (p *dbStorage) insert(s *sessionConnect) error {
	return p.db.Insert(TABLE_NAME, s)
}

func (p *dbStorage) del(sid string) error {
	return p.db.ExecNumErr("DELETE FROM "+TABLE_NAME+" where sid=?", sid)
}

func (p *dbStorage) update(s *sessionConnect) error {
	return p.db.Update(TABLE_NAME, s)
}
