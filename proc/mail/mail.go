package db

import (
	"fmt"

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
		Hook:     _module.test,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.preStart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.preStart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_SYS,
	}}
)

func (p *Module) test(ops *proc.HookOps, configer *proc.Configer) error {
	c := &mail.Config{}
	if err := configer.Read(p.name, c); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}

	return nil
}

// Because some configuration may be stored in the database,
// set the db.connect into sys.db.prestart
func (p *Module) preStart(ops *proc.HookOps, configer *proc.Configer) (err error) {
	popts := ops.Options()

	c := &mail.Config{}
	if err := configer.Read(p.name, c); err != nil {
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

/*
* emailmail.NewMail()
 */
func (p *Module) NewMail(tpl proc.Executer, data interface{}) (*mail.MailContext, error) {
	return p.config.NewMail(tpl, data)
}

func (p *Module) SendMail(subject, to []string, tpl proc.Executer, data interface{}) error {
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
