// this is a sample custom auth module
package auth

import "github.com/yubo/golib/util"

const (
	moduleName = "auth"
)

type Config struct {
	Name string `json:"name"`
}

func (p Config) String() string {
	return util.Prettify(p)
}

type Module struct {
	*Config
	name string
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.testHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_SYS,
	}, {
		Hook:     _module.stopHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_SYS,
	}, {
		// reload.represtart
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_SYS,
	}, {
		// reload.start
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_SYS,
	}}
)
