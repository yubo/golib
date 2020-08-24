package proc

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/spf13/pflag"
	"github.com/yubo/golib/openapi/api"
	"k8s.io/klog/v2"
)

// type {{{
type HookFn func(ops *HookOps, cf *Configer) error

func New(env *Settings) *Settings {
	if env.Debug == false {
		env.Debug = getenvBool(strings.ToUpper(env.Name) + "_DEBUG")
	}
	if env.Config == "" {
		env.Config = os.Getenv(strings.ToUpper(env.Name) + "_CONFIG")
	}
	return env
}

type Settings struct {
	Name       string
	Config     string
	Changelog  string
	Debug      bool
	TestConfig bool
	Version    api.Version
	Asset      func(string) ([]byte, error)
}

func (s *Settings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&s.Config, "config", "c", s.Config,
		fmt.Sprintf("config file path of your %s server.", s.Name))
	fs.BoolVarP(&s.TestConfig, "test", "t", s.TestConfig,
		fmt.Sprintf("test config file path of your %s server.", s.Name))
	fs.BoolVar(&s.Debug, "debug", s.Debug, "enable verbose output")
}

type HookOps struct {
	Hook     HookFn
	Owner    string
	HookNum  int
	Priority int
	Data     interface{}
}

type HookOpsBucket []*HookOps

func (p HookOpsBucket) Len() int {
	return len(p)
}

func (p HookOpsBucket) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p HookOpsBucket) Less(i, j int) bool {
	return p[i].Priority < p[j].Priority
}

// }}}

// const {{{

const (
	ACTION_START = iota
	ACTION_RELOAD
	ACTION_STOP
	ACTION_TEST
	ACTION_SIZE
)
const (
	STATUS_INIT = iota
	STATUS_PENDING
	STATUS_RUNNING
	STATUS_RELOADING
	STATUS_EXIT
)

const (
	_ = iota
	PRI_PRE_SYS
	PRI_PRE_MODULE
	PRI_MODULE
	PRI_POST_MODULE
	PRI_SYS
	PRI_POST_SYS
)

// }}}

var (
	hookOps  [ACTION_SIZE]HookOpsBucket
	configer *Configer
	Status   uint32
)

func init() {
	for i := 0; i < ACTION_SIZE; i++ {
		hookOps[i] = HookOpsBucket([]*HookOps{})
	}
}

func RegisterHooks(in []HookOps) error {
	for i, _ := range in {
		v := &in[i]
		if v.HookNum < 0 || v.HookNum >= ACTION_SIZE {
			return fmt.Errorf("invalid HookNum %d", v.HookNum)
		}
		hookOps[v.HookNum] = append(hookOps[v.HookNum], v)
	}
	return nil
}

// procInit
// alloc configer
// parse configfile
// validate config each module
// sort hook options
func procInit(configFile string) (cf *Configer, err error) {
	if cf, err = NewConfiger(configFile); err != nil {
		return nil, err
	}

	if err = cf.Prepare(); err != nil {
		return nil, err
	}

	Status = STATUS_PENDING

	for i := 0; i < ACTION_SIZE; i++ {
		sort.Sort(hookOps[i])
	}

	configer = cf
	return
}

func hookNumName(n int) string {
	switch n {
	case ACTION_START:
		return "start"
	case ACTION_RELOAD:
		return "reload"
	case ACTION_STOP:
		return "stop"
	case ACTION_TEST:
		return "test"
	default:
		return "unknown"
	}
}

func dbgOps(ops *HookOps) {
	klog.V(5).Infof("hook %s %s[%d] %s()",
		hookNumName(ops.HookNum),
		ops.Owner,
		ops.Priority,
		runtime.FuncForPC(reflect.ValueOf(ops.Hook).
			Pointer()).Name())
}

// only be called once
func procStart() error {
	for _, ops := range hookOps[ACTION_START] {
		dbgOps(ops)
		if err := ops.Hook(ops, configer); err != nil {
			return err
		}
	}
	atomic.StoreUint32(&Status, STATUS_RUNNING)
	return nil
}

// reverse order
func procStop() error {
	ss := hookOps[ACTION_STOP]
	for i := len(ss) - 1; i >= 0; i-- {
		ops := ss[i]

		dbgOps(ops)
		if err := ops.Hook(ops, configer); err != nil {
			return err
		}
	}
	atomic.StoreUint32(&Status, STATUS_EXIT)
	return nil
}

func procTest() error {
	for _, ops := range hookOps[ACTION_TEST] {
		dbgOps(ops)
		if err := ops.Hook(ops, configer); err != nil {
			return err
		}
	}
	return nil
}

func procReload() error {
	atomic.StoreUint32(&Status, STATUS_RELOADING)

	if err := configer.Prepare(); err != nil {
		return err
	}

	for _, ops := range hookOps[ACTION_RELOAD] {
		dbgOps(ops)
		if err := ops.Hook(ops, configer); err != nil {
			return err
		}
	}
	atomic.StoreUint32(&Status, STATUS_RUNNING)
	return nil
}

func envOr(name string, defs ...string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	for _, def := range defs {
		if def != "" {
			return def
		}
	}
	return ""
}

func getenvBool(str string) bool {
	b, _ := strconv.ParseBool(os.Getenv(str))
	return b
}
