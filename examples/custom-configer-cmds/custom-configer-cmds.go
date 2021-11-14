package main

import (
	"context"
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
	moduleName = "custom-configer-cmds"
	modulePath = "hello"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(proc.PrintErrln(err))
	}
}

type config struct {
	UserName string `json:"userName" flag:"user-name" default:"tom" description:"user name"`
	Cmd      string `json:"cmd"`
}

func newRootCmd() *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())
	ctx := context.TODO()

	cmd := &cobra.Command{
		Use: moduleName,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// customize parse options
			// and get the parsed configer
			cfg, err := proc.Parse(cmd.Flags(),
				configer.WithOverrideYaml(modulePath, "cmd: "+cmd.Name()),
			)
			if err != nil {
				return err
			}
			klog.Infof("get configer from prerun %s: %s", cmd.Name(), cfg.String())
			return nil
		},
	}

	cmd.AddCommand(
		newHelloCmd(),
		newStartCmd(),
	)

	proc.Init(cmd, proc.WithContext(ctx))

	return cmd
}

func newStartCmd() *cobra.Command {
	cf := &config{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "start demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.Infof("[start] hi %s", cf.UserName)

			klog.Infof("Press ctrl-c to leave the daemon process")

			return proc.Start(cmd.Flags())
		},
	}

	proc.RegisterFlags("hello", "server", cf)

	return cmd
}

func newHelloCmd() *cobra.Command {
	cf := &config{}
	cmd := &cobra.Command{
		Use:   "hello",
		Short: "hello demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.Infof("[hello] hi %s", cf.UserName)
			return nil
		},
	}

	configer.FlagSet(cmd.Flags(), cf)

	return cmd
}

func init() {
	// register global sample config schema
	proc.RegisterFlags(modulePath, moduleName, &config{})
}
