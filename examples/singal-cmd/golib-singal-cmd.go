package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
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
	klog.Infof("entering start()")

	c := configer.ConfigerMustFrom(ctx)
	cf := &config{}

	if err := c.Read(modulePath, cf); err != nil {
		return err
	}

	klog.Infof("hello %s", cf.UserName)

	klog.Infof("Press ctrl-c to leave the process")
	return nil
}

func newHelloCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "hello",
		Short:        "hello",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.Infof("hello")
			return nil
		},
	}
}

func init() {
	proc.RegisterHooks(hookOps)
	proc.RegisterFlags(modulePath, moduleName, &config{})
}
