package main

import (
	"context"
	"os"

	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/proc"
	"k8s.io/klog/v2"
)

const (
	moduleName = "golib-singal-cmd"
	modulePath = "hello"
)

var (
	hookOps = []proc.HookOps{{
		Hook:     start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}}
)

type config struct {
	UserName string `json:"userName" flag:"user-name" default:"tom" description:"user name"`
}

func main() {
	if err := proc.NewRootCmd().Execute(); err != nil {
		os.Exit(proc.PrintErrln(err))
	}
}

func start(ctx context.Context) error {
	cf := &config{}
	if err := configer.ConfigerMustFrom(ctx).Read(modulePath, cf); err != nil {
		return err
	}

	klog.Infof("hello %s", cf.UserName)
	klog.Infof("Press ctrl-c to leave the process")

	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
	// register sample config schema
	proc.AddConfig(modulePath, &config{}, proc.WithConfigGroup(moduleName))
}
