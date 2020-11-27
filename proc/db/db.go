package db

import (
	"context"
	"fmt"

	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/util"
)

const (
	moduleName = "sys.db"
)

type Config struct {
	Driver string `json:"driver" description:"default: mysql"`
	Dsn    string `json:"dsn"`
}

func (p Config) String() string {
	return util.Prettify(p)
}

func (p *Config) Validate() error {
	if p.Dsn == "" {
		return nil
	}
	if p.Driver == "" {
		p.Driver = "mysql"
	}
	return nil
}

type Module struct {
	*Config
	oldConfig *Config
	name      string
	db        *orm.Db
	ctx       context.Context
	cancel    context.CancelFunc
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.test,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.stop,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_PRE_SYS,
	}, {
		// reload.start
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_SYS,
	}}
)

func (p *Module) test(ops *proc.HookOps, configer *proc.Configer) error {
	c := &Config{}
	if err := configer.Read(p.name, c); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}

	if p.Dsn != "" {
		if db, err := orm.DbOpen(p.Driver, p.Dsn); err != nil {
			return err
		} else {
			db.Close()
		}
	}
	return nil
}

// Because some configuration may be stored in the database,
// set the db.connect into sys.db.prestart
func (p *Module) start(ops *proc.HookOps, configer *proc.Configer) (err error) {
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())

	popts := ops.Options()

	c := &Config{}
	if err := configer.Read(p.name, c); err != nil {
		return err
	}

	p.Config, p.oldConfig = c, p.Config

	// db
	if p.Driver != "" && p.Dsn != "" {
		if p.db, err = orm.DbOpenWithCtx(p.Driver, p.Dsn, p.ctx); err != nil {
			return err
		}
		popts = popts.SetDb(p.db)
	}

	ops.SetOptions(popts)
	return nil
}

func (p *Module) stop(ops *proc.HookOps, configer *proc.Configer) error {
	p.cancel()
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
