package main

import (
	"os"

	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/logs"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/session"
	"k8s.io/klog/v2"

	_ "github.com/yubo/golib/example/auth"
	"github.com/yubo/golib/example/user"
	_ "github.com/yubo/golib/orm/sqlite"
	_ "github.com/yubo/golib/proc/audit"
	_ "github.com/yubo/golib/proc/db"
	_ "github.com/yubo/golib/proc/grpc"
	_ "github.com/yubo/golib/proc/http"
	_ "github.com/yubo/golib/proc/logging"
	_ "github.com/yubo/golib/proc/metrics"
	_ "github.com/yubo/golib/proc/session"
	_ "github.com/yubo/golib/proc/sys"
	_ "github.com/yubo/golib/proc/tracing"

	_ "github.com/yubo/golib/example/metrics"
	_ "github.com/yubo/golib/example/session"
	_ "github.com/yubo/golib/example/tracing"
	_ "github.com/yubo/golib/example/user"
)

const (
	AppName    = "helo"
	moduleName = "helo.main"
)

var (
	hookOps = []proc.HookOps{{
		Hook:     prestart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}, {
		Hook:     stop,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_MODULE,
	}}
	buildReporter proc.Reporter
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	opts := proc.NewOptions().SetName(AppName)
	proc.RegisterHooksWithOptions(hookOps, opts)

	command := proc.NewRootCmd(os.Args[1:])
	if err := command.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func prestart(ops *proc.HookOps, cf *configer.Configer) error {
	klog.Info("prestart")

	popts := ops.Options()

	db := popts.Db()
	if err := db.ExecRows([]byte(session.CREATE_TABLE_SQLITE)); err != nil {
		return err
	}
	if err := db.ExecRows([]byte(user.CREATE_TABLE_SQLITE)); err != nil {
		return err
	}

	buildReporter = proc.NewBuildReporter(popts)
	if err := buildReporter.Start(); err != nil {
		return err
	}

	return nil
}

func start(ops *proc.HookOps, cf *configer.Configer) error {
	klog.Info("start")

	popts := ops.Options()

	buildReporter = proc.NewBuildReporter(popts)
	if err := buildReporter.Start(); err != nil {
		return err
	}

	return nil
}

func stop(ops *proc.HookOps, cf *configer.Configer) error {
	klog.Info("stop")
	buildReporter.Stop()
	return nil
}
