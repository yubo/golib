// like logs.Options, as a register module
package register

import (
	"context"
	"fmt"
	"strings"

	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/logs"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/errors"
)

const (
	moduleName       = "logs"
	defaultLogFormat = "text"
)

type module struct {
	name string
}

var (
	_module = &module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.init,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_SYS_INIT - 1, // priority over other modules
	}}
)

func (p *module) init(ctx context.Context) (err error) {
	configer := proc.ConfigerMustFrom(ctx)

	cf := newConfig()
	if err := configer.Read(p.name, cf); err != nil {
		return err
	}

	if err := cf.initLogs(); err != nil {
		return err
	}

	return nil
}

func (p *config) initLogs() error {
	o := logs.Options{
		LogFormat:       p.Format,
		LogSanitization: p.Sanitization,
	}

	if err := errors.NewAggregate(o.Validate()); err != nil {
		return err
	}

	o.Apply()

	return nil
}

// List of logs (k8s.io/klog + github.com/yubo/golib/logs) flags supported by all logging formats
var supportedLogsFlags = map[string]struct{}{
	"v": {},
	// TODO: support vmodule after 1.19 Alpha
}

type config struct {
	Format       string `json:"format" flag:"logging-format" default:"text"`
	Sanitization bool   `json:"sanitization" flag:"experimental-logging-sanitization" description:"[Experimental] When enabled prevents logging of fields tagged as sensitive (passwords, keys, tokens). \nRuntime log sanitization may introduce significant computation overhead and therefore should not be enabled in production."`
}

func newConfig() *config {
	return &config{
		Format: defaultLogFormat,
	}
}

func (p config) String() string {
	return util.Prettify(p)
}

func (p config) tags() map[string]*configer.FieldTag {
	unsupportedFlags := fmt.Sprintf("--%s", strings.Join(logs.UnsupportedLoggingFlags(), ", --"))
	formats := fmt.Sprintf(`"%s"`, strings.Join(logs.RegistryList(), `", "`))

	return map[string]*configer.FieldTag{
		"format": {Description: fmt.Sprintf("Sets the log format. Permitted formats: %s.\nNon-default formats don't honor these flags: %s.\nNon-default choices are currently alpha and subject to change without warning.", formats, unsupportedFlags)},
	}

}

func init() {
	proc.RegisterHooks(hookOps)

	cf := newConfig()
	proc.RegisterFlags(moduleName, "logs", cf, configer.WithTags(cf.tags))
}
