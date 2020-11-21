package db

import (
	"fmt"
	"io"

	"github.com/yubo/golib/mail"
	"github.com/yubo/golib/proc"
	"k8s.io/klog"
)

const (
	moduleName = "sys.mail"
)

type Module struct {
	config *mail.Config
	name   string
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.testHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_SYS,
	}}
)

func (p *Module) testHook(ops *proc.HookOps, cf *proc.Configer) error {
	c := &mail.Config{}
	if err := cf.Read(p.name, c); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}

	return nil
}

// Because some configuration may be stored in the database,
// set the db.connect into sys.db.prestart
func (p *Module) preStartHook(ops *proc.HookOps, cf *proc.Configer) (err error) {
	popts := ops.Options()

	c := &mail.Config{}
	if err := cf.Read(p.name, c); err != nil {
		return err
	}
	p.config = c

	if !c.Enabled {
		return nil
	}

	popts = popts.SetMail(p)

	ops.SetOptions(popts)
	return nil
}

type Executer interface {
	Execute(wr io.Writer, data interface{}) error
}

/*
* emailmail.NewMail()
 */
func (p *Module) NewMail(tpl Executer, data interface{}) (*mail.MailContext, error) {
	return p.config.NewMail(tpl, data)
}

func (p *Module) SendMail(subject, to []string, tpl Executer, data interface{}) error {
	eml, err := p.config.NewMail(tpl, data)
	if err != nil {
		return err
	}

	eml.SetHeader("Subject", subject...)
	eml.SetHeader("To", to...)
	go func() {
		if err := eml.DialAndSend(); err != nil {
			klog.Error(err)
		}
	}()
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
