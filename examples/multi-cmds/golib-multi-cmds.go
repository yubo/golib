package main

import (
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/proc"
	"k8s.io/klog/v2"
)

const (
	moduleName = "golib-multi-cmds"
	modulePath = "hello"
)

func main() {

	cmd := newRootCmd()
	cmd.AddCommand(
		newHelloCmd(),
		newStartCmd(),
	)

	if err := cmd.Execute(); err != nil {
		os.Exit(proc.PrintErrln(err))
	}
}

type config struct {
	UserName string `json:"userName" flag:"user-name" default:"tom" description:"user name"`
}

func newRootCmd() *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	return &cobra.Command{Use: moduleName}
}

func newStartCmd() *cobra.Command {
	cf := &config{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "start demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.Infof("hello %s", cf.UserName)
			return proc.Start(cmd.Flags())
		},
	}

	proc.RegisterFlags("hello", "server", cf)

	proc.Init(cmd)
	return cmd
}

func newHelloCmd() *cobra.Command {
	cf := &config{}
	cmd := &cobra.Command{
		Use:   "hello",
		Short: "hello demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.Infof("hello %s", cf.UserName)
			return nil
		},
	}

	configer.FlagSet(cmd.Flags(), cf)

	return cmd
}
