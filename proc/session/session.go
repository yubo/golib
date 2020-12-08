package session

import (
	"context"
	"fmt"

	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/session"
)

const (
	moduleName = "sys.session"
)

type Module struct {
	config  *session.Config
	name    string
	db      *orm.Db
	ctx     context.Context
	cancel  context.CancelFunc
	session *session.Manager
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.testHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.stop,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_MODULE,
	}}
)

func (p *Module) testHook(ops *proc.HookOps, cf *configer.Configer) error {
	c := &session.Config{}
	if err := cf.Read(p.name, c); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}

	return nil
}

func (p *Module) start(ops *proc.HookOps, cf *configer.Configer) (err error) {
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	popts := ops.Options()

	c := &session.Config{}
	if err := cf.Read(p.name, c); err != nil {
		return err
	}
	p.config = c

	if p.db = popts.Db(); p.db == nil {
		return fmt.Errorf("%s start err: unable get db from options", p.name)
	}

	if p.session, err = session.StartSession(p.config,
		session.WithCtx(p.ctx), session.WithDb(p.db)); err != nil {
		return fmt.Errorf("%s start err: %s", p.name, err)
	}

	popts = popts.SetSession(p.session)

	ops.SetOptions(popts)
	return nil
}

func (p *Module) stop(ops *proc.HookOps, cf *configer.Configer) error {
	p.cancel()
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
