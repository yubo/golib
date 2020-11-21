package session

import (
	"fmt"
	"io"

	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/session"
	"k8s.io/klog"
)

const (
	moduleName = "sys.session"
)

type Module struct {
	config  *session.Config
	name    string
	session *session.Session
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.testHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.stopHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_MODULE,
	}}
)

func (p *Module) testHook(ops *proc.HookOps, cf *proc.Configer) error {
	c := &session.Config{}
	if err := cf.Read(p.name, c); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}

	return nil
}

func (p *Module) startHook(ops *proc.HookOps, cf *proc.Configer) (err error) {
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	popts := ops.Options()

	c := &mail.Config{}
	if err := cf.Read(p.name, c); err != nil {
		return err
	}
	p.config = c

	if p.db = popts.Db(); p.db == nil {
		return fmt.Errorf("%s start err: unable get db from options", p.name)
	}

	if p.session, err = session.StartSessionWithDb(p.Session, p.ctx, p.db); err != nil {
		return fmt.Errorf("%s start err: %s", p.name, err)
	}

	popts = popts.SetSession(p.session)

	ops.SetOptions(popts)
	return nil
}

func (p *Module) stopHook(ops *proc.HookOps, cf *proc.Configer) error {
	p.cancel()
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
