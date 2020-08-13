package sys

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-systemd/daemon"
	restful "github.com/emicklei/go-restful"
	"github.com/yubo/golib/logs"
	"github.com/yubo/golib/mail"
	"github.com/yubo/golib/openapi"
	restApi "github.com/yubo/golib/openapi/api"
	"github.com/yubo/golib/openapi/urlencoded"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

// type {{{

type Config struct {
	GrpcAddr           string      `json:"grpcAddr"`
	GrpcMaxRecvMsgSize int         `json:"grpcMaxRecvMsgSize"`
	HttpAddr           string      `json:"httpAddr"`
	HttpCross          bool        `json:"httpCross"`
	Profile            bool        `json:"profile"`
	PidFile            string      `json:"pidFile"`
	LogLevel           int         `json:"logLevel"`
	WatchdogSec        int         `json:"watchdogSec" description:"The time to feed the dog"`
	DbDriver           string      `json:"dbDriver"`
	Dsn                string      `json:"dsn"`
	MaxLimitPage       int         `json:"maxLimitPage" description:"respful api query max limit"`
	DefLimitPage       int         `json:"defLimitPage" description:"respful api query default limit"`
	Mail               mail.Config `json:"mail"`
}

func (p Config) String() string {
	return util.Prettify(p)
}

func (p *Config) Validate() error {
	if p.HttpAddr == "" {
		return fmt.Errorf("sys: httpAddr must be set")
	}
	if p.DbDriver == "" {
		return fmt.Errorf("sys: dbDriver must be set")
	}
	if p.Dsn == "" {
		return fmt.Errorf("sys: dsn must be set")
	}
	if err := p.Mail.Validate(); err != nil {
		return fmt.Errorf("sys: %s", err)
	}

	return nil
}

type Module struct {
	*Config
	oldConfig     *Config
	Name          string
	db            *orm.Db
	ctx           context.Context
	cancel        context.CancelFunc
	cpuFd         *os.File
	heapFd        *os.File
	stats         *util.Stats
	restContainer *restful.Container
	serveMux      *http.ServeMux // http
	grpcServer    *grpc.Server   // grpc
}

// }}}

const (
	moduleName = "sys"
)

var (
	EEXIST  = errors.New("Process exists")
	_module = &Module{Name: moduleName}
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
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_SYS,
	}}
)

func init() {
	restful.RegisterEntityAccessor(openapi.MIME_URL_ENCODED, urlencoded.NewEntityAccessor())
	proc.RegisterHooks(hookOps)
}

func (p *Module) testHook(ops *proc.HookOps, cf *proc.Configer) (err error) {
	c := &Config{}
	if err := cf.Read(p.Name, c); err != nil {
		return fmt.Errorf("%s read config err: %s", p.Name, err)
	}
	return c.Validate()
}

func (p *Module) preStartHook(ops *proc.HookOps, cf *proc.Configer) (err error) {
	if ops.HookNum != proc.ACTION_START {
		p.cancel()
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())

	klog.V(5).Infof("%v", cf)
	c := &Config{}
	if err := cf.Read(p.Name, c); err != nil {
		return err
	}

	p.Config, p.oldConfig = c, p.Config

	logs.FlagSet.Set("v", fmt.Sprintf("%d", p.LogLevel))
	klog.Infof("log level %d", p.LogLevel)

	// db
	if p.DbDriver != "" && p.Dsn != "" {
		if p.db, err = orm.DbOpenWithCtx(p.DbDriver, p.Dsn, p.ctx); err != nil {
			return err
		}
	}

	// grpc api
	p.grpcPrestart()

	// restful api
	p.restfulPrestart()

	// profile debug api
	if p.Profile {
		p.InstallProfiling()
	}

	// restful api debug
	if p.LogLevel >= 8 {
		p.RestFilter(DbgFilter)
	}

	restApi.SetLimitPage(p.DefLimitPage, p.MaxLimitPage)

	// restful api log
	p.RestFilter(p.LogFilter)

	if ops.HookNum == proc.ACTION_START {
		p.stats = util.NewStats(statsKeys, statsValues)
		p.StatsRegister(p.Name, p.stats)
	}

	return nil
}

func (p *Module) GetDb() (*orm.Db, error) {
	if p.db == nil {
		return nil, fmt.Errorf("sys module is not ready")
	}

	return p.db, nil
}

func (p *Module) startHook(ops *proc.HookOps, cf *proc.Configer) error {

	// install after all module registered()
	p.InstallApidocs()

	if err := p.restfulStart(); err != nil {
		return err
	}

	if err := p.grpcStart(); err != nil {
		return err
	}

	// watch dog
	if t := p.WatchdogSec; t > 0 {
		daemon.SdNotify(false, "READY=1")
		go util.Until(
			func() {
				p.stats.Inc(ST_WATCHDOG_NOTIFY, 1)
				daemon.SdNotify(false, "WATCHDOG=1")
			},
			time.Duration(t)*time.Second,
			p.ctx.Done(),
		)
	}

	return nil
}

func (p *Module) stopHook(ops *proc.HookOps, cf *proc.Configer) error {
	p.cancel()
	return nil
}

func (p *Module) GetMailConfig() (*mail.Config, error) {
	if p.Config == nil {
		return nil, fmt.Errorf("sys module is not ready")
	}
	return &_module.Config.Mail, nil
}

func GetModule() *Module {
	return _module
}
