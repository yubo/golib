package main

import (
	"os"

	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/logs"
	"github.com/yubo/golib/proc"
	"k8s.io/klog/v2"

	_ "github.com/yubo/golib/example/auth"
	_ "github.com/yubo/golib/orm/mysql"
	_ "github.com/yubo/golib/proc/audit"
	_ "github.com/yubo/golib/proc/db"
	_ "github.com/yubo/golib/proc/grpc"
	_ "github.com/yubo/golib/proc/http"
	_ "github.com/yubo/golib/proc/logging"
	_ "github.com/yubo/golib/proc/metrics"
	_ "github.com/yubo/golib/proc/session"
	_ "github.com/yubo/golib/proc/sys"
	_ "github.com/yubo/golib/proc/tracing"

	_ "github.com/yubo/golib/example/echo"
	_ "github.com/yubo/golib/example/metrics"
	_ "github.com/yubo/golib/example/session"
	_ "github.com/yubo/golib/example/tracing"
)

const (
	AppName    = "helo"
	moduleName = "helo.main"
)

var (
	hookOps = []proc.HookOps{{
		Hook:     startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}, {
		Hook:     stopHook,
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

func startHook(ops *proc.HookOps, cf *configer.Configer) error {
	klog.Info("start")

	popts := ops.Options()

	buildReporter = proc.NewBuildReporter(popts)
	if err := buildReporter.Start(); err != nil {
		return err
	}

	return nil
}

func stopHook(ops *proc.HookOps, cf *configer.Configer) error {
	klog.Info("stop")
	buildReporter.Stop()
	return nil
}
