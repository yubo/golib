// This module is used for display, it will allow all access,
// you should replace it with your own module, do not use it in the production
package aclfilter

import (
	restful "github.com/emicklei/go-restful"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/proc"
	"k8s.io/klog/v2"
)

const (
	moduleName = "http.aclfilter.getter.nop"
)

type AclHandle func(aclName string) (restful.FilterFunction, string, error)

type Module struct {
	name   string
	db     *orm.Db
	server proc.HttpServer
}

func (p *Module) GetAclFilter(acl string) (restful.FilterFunction, string, error) {
	klog.V(3).Infof("%s acl filter %s", p.name, acl)
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		chain.ProcessFilter(req, resp)
	}, acl, nil
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_MODULE,
	}}
)

func (p *Module) preStartHook(ops *proc.HookOps, configer *proc.Configer) error {
	popts := ops.Options()

	popts = popts.Set(proc.AclFilterGetterName, p)

	ops.SetOptions(popts)
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
