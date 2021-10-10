package logs

//import (
//	"context"
//	"flag"
//	"fmt"
//	"log"
//	"strings"
//
//	"github.com/go-logr/logr"
//	"github.com/spf13/pflag"
//	"github.com/yubo/golib/api"
//	"github.com/yubo/golib/configer"
//	"github.com/yubo/golib/logs/sanitization"
//	"github.com/yubo/golib/proc"
//	"github.com/yubo/golib/util"
//	"github.com/yubo/golib/util/errors"
//	"github.com/yubo/golib/util/wait"
//	"k8s.io/klog/v2"
//)
//
//const (
//	moduleName        = "logs"
//	logFormatFlagName = "logging-format"
//	defaultLogFormat  = "text"
//)
//
//// List of logs (k8s.io/klog + github.com/yubo/golib/logs) flags supported by all logging formats
//var supportedLogsFlags = map[string]struct{}{
//	"v": {},
//	// TODO: support vmodule after 1.19 Alpha
//}
//
//type config struct {
//	FlushFreq    api.Duration `json:"flushFreq" flag:"log-flush-frequency" default:"5s" description:"Maximum number of seconds between log flushes"`
//	Format       string       `json:"format" flag:"logging-format"`
//	Sanitization bool         `json:"sanitization" flag:"experimental-logging-sanitization" description:"[Experimental] When enabled prevents logging of fields tagged as sensitive (passwords, keys, tokens). \nRuntime log sanitization may introduce significant computation overhead and therefore should not be enabled in production."`
//}
//
//func newConfig() *config {
//	return &config{
//		Format: defaultLogFormat,
//	}
//}
//
//func (p *config) tagOpts(fieldName string) *configer.TagOpts {
//	unsupportedFlags := fmt.Sprintf("--%s", strings.Join(unsupportedLoggingFlags(), ", --"))
//	formats := fmt.Sprintf(`"%s"`, strings.Join(logRegistry.List(), `", "`))
//
//	// No new log formats should be added after generation is of flag options
//	logRegistry.Freeze()
//
//	switch fieldName {
//	case "Format":
//		return &configer.TagOpts{
//			Name:        "Format",
//			Json:        "format",
//			Flag:        []string{"logging-format"},
//			Default:     defaultLogFormat,
//			Description: fmt.Sprintf("Sets the log format. Permitted formats: %s.\nNon-default formats don't honor these flags: %s.\nNon-default choices are currently alpha and subject to change without warning.", formats, unsupportedFlags),
//		}
//	default:
//		return nil
//	}
//}
//
//func (p config) String() string {
//	return util.Prettify(p)
//}
//
//func (p *config) Validate() error {
//	errs := []error{}
//	if p.Format != defaultLogFormat {
//		allFlags := unsupportedLoggingFlags()
//		for _, fname := range allFlags {
//			if flagIsSet(fname) {
//				errs = append(errs, fmt.Errorf("non-default logging format doesn't honor flag: %s", fname))
//			}
//		}
//	}
//	if _, err := p.Get(); err != nil {
//		errs = append(errs, fmt.Errorf("unsupported log format: %s", p.Format))
//	}
//	return errors.NewAggregate(errs)
//}
//
//// Apply set klog logger from LogFormat type
//func (p *config) Apply() {
//	// if log format not exists, use nil loggr
//	loggr, _ := p.Get()
//	klog.SetLogger(loggr)
//	if p.Sanitization {
//		klog.SetLogFilter(&sanitization.SanitizingFilter{})
//	}
//}
//
//// Get logger with LogFormat field
//func (p *config) Get() (logr.Logger, error) {
//	return logRegistry.Get(p.Format)
//}
//
//type module struct {
//	*config
//	name string
//}
//
//var (
//	_module = &module{name: moduleName}
//	hookOps = []proc.HookOps{{
//		Hook:     _module.start,
//		Owner:    moduleName,
//		HookNum:  proc.ACTION_START,
//		Priority: proc.PRI_SYS_INIT - 1,
//	}, {
//		Hook:     _module.stop,
//		Owner:    moduleName,
//		HookNum:  proc.ACTION_STOP,
//		Priority: proc.PRI_SYS_INIT - 1,
//	}}
//)
//
//// Because some configuration may be stored in the database,
//// set the db.connect into sys.db.prestart
//func (p *module) start(ctx context.Context) (err error) {
//	configer := proc.ConfigerMustFrom(ctx)
//
//	cf := &config{}
//	if err := configer.Read(p.name, cf); err != nil {
//		return err
//	}
//
//	p.config = cf
//
//	log.SetOutput(KlogWriter{})
//	log.SetFlags(0)
//	// The default glog flush interval is 5 seconds.
//	go wait.Until(klog.Flush, cf.FlushFreq.Duration, ctx.Done())
//
//	namedFlagSets := proc.NamedFlagSets()
//	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), name)
//
//	p.Apply()
//
//	return nil
//}
//
//func (p *module) stop(ctx context.Context) error {
//	FlushLogs()
//	return nil
//}
//
//func Register() {
//	proc.RegisterHooks(hookOps)
//
//	cf := newConfig()
//	proc.RegisterFlags("logs", "logs", cf, configer.WithTagOptsGetter(cf.tagOpts))
//}
//
//func flagIsSet(name string) bool {
//	f := flag.Lookup(name)
//	if f != nil {
//		return f.DefValue != f.Value.String()
//	}
//	pf := pflag.Lookup(name)
//	if pf != nil {
//		return pf.DefValue != pf.Value.String()
//	}
//	panic("failed to lookup unsupported log flag")
//}
//
//func unsupportedLoggingFlags() []string {
//	allFlags := []string{}
//
//	// k8s.io/klog flags
//	fs := &flag.FlagSet{}
//	klog.InitFlags(fs)
//	fs.VisitAll(func(flag *flag.Flag) {
//		if _, found := supportedLogsFlags[flag.Name]; !found {
//			allFlags = append(allFlags, strings.Replace(flag.Name, "_", "-", -1))
//		}
//	})
//
//	// github.com/yubo/golib/logs flags
//	//pfs := &pflag.FlagSet{}
//	//AddFlags(pfs)
//	//pfs.VisitAll(func(flag *pflag.Flag) {
//	//	if _, found := supportedLogsFlags[flag.Name]; !found {
//	//		allFlags = append(allFlags, flag.Name)
//	//	}
//	//})
//	allFlags = append(allFlags, "log-flush-frequency")
//	return allFlags
//}
