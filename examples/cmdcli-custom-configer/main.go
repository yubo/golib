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
	User string `json:"user" flag:"user" default:"tom" description:"user name"`
	Cmd  string `json:"cmd" default:"main" description:"cmd"`
}

func newRootCmd() *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

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

	return cmd
}

// start with mainloop
func newStartCmd() *cobra.Command {
	cf := &config{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "start demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := proc.ReadConfig(modulePath, cf); err != nil {
				return err
			}

			klog.Infof("%s hi %s", cf.Cmd, cf.User)

			klog.Infof("Press ctrl-c to leave the daemon process")

			return proc.Start(cmd.Flags())
		},
	}

	proc.Init(cmd, proc.WithContext(context.TODO()))

	return cmd
}

// customize cmd
func newHelloCmd() *cobra.Command {
	type config struct {
		Object string `json:"object" flag:"obj" default:"world" description:"hello,world"`
	}
	cf := &config{}

	cmd := &cobra.Command{
		Use:   "hello",
		Short: "hello demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.Infof("hi %s", cf.Object)
			return nil
		},
	}

	configer.FlagSet(cmd.Flags(), cf)

	return cmd
}

func init() {
	// register sample config schema
	proc.AddConfig(modulePath, &config{}, proc.WithConfigGroup("start"))
}
