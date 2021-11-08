package session

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/util"
)

const (
	CREATE_TABLE_MYSQL = "CREATE TABLE IF NOT EXISTS `session` (" +
		"   `sid` char(128) NOT NULL," +
		"   `data` blob NULL," +
		"   `user_name` char(128) DEFAULT ''," +
		"   `cookie_name` char(128) DEFAULT ''," +
		"   `created_at` bigint unsigned DEFAULT '0'," +
		"   `updated_at` bigint unsigned DEFAULT '0' NOT NULL," +
		"   PRIMARY KEY (`sid`)," +
		"   KEY (`cookie_name`)," +
		"   KEY (`updated_at`)" +
		" ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8;"
	CREATE_TABLE_SQLITE = "CREATE TABLE IF NOT EXISTS `session` (" +
		"   `sid` char(128) PRIMARY KEY NOT NULL," +
		"   `data` blob NULL," +
		"   `user_name` char(128) DEFAULT ''," +
		"   `cookie_name` char(128) DEFAULT ''," +
		"   `created_at` integer unsigned DEFAULT '0'," +
		"   `updated_at` integer unsigned DEFAULT '0' NOT NULL" +
		" );" +
		" CREATE INDEX `session_key_cookie_name` on `session` (`cookie_name`);" +
		" CREATE INDEX `session_key_updated_at` on `session` (`updated_at`);"
)

func newDbStorage(cf *Config, opts *Options) (storage, error) {
	st := &dbStorage{config: cf, db: opts.db}

	if st.db == nil && cf.Dsn != "" {
		var err error
		st.db, err = orm.Open(cf.DbDriver, cf.Dsn, orm.WithContext(opts.ctx))
		if err != nil {
			return nil, err
		}
	}

	if st.db == nil {
		return nil, fmt.Errorf("unable get db storage")
	}

	if err := st.db.RawDB().Ping(); err != nil {
		return nil, err
	}

	util.UntilWithTick(func() {
		st.db.Exec("delete from session where updated_at<? and cookie_name=?",
			opts.clock.Now().Unix()-int64(cf.maxIdleTime.Seconds()), cf.CookieName)
	},
		opts.clock.NewTicker(cf.gcInterval).C(),
		opts.ctx.Done())

	return st, nil
}

type dbStorage struct {
	db     orm.DB
	config *Config
}

func (p *dbStorage) all() (ret int) {
	err := p.db.Query("select count(*) from session where cookie_name=?",
		p.config.CookieName).Row(&ret)
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	return
}

func (p *dbStorage) get(sid string) (ret *sessionConn, err error) {
	err = p.db.Query("select * from session where sid=?", sid).Row(&ret)
	return
}

func (p *dbStorage) insert(s *sessionConn) error {
	return p.db.Insert(s, orm.WithTable("session"))
}

func (p *dbStorage) del(sid string) error {
	return p.db.ExecNumErr("DELETE FROM session where sid=?", sid)
}

func (p *dbStorage) update(s *sessionConn) error {
	return p.db.Update(s, orm.WithTable("session"))
}
