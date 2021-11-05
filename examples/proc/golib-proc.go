package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/proc"
	"k8s.io/klog/v2"
)

const (
	AppName = "exaple-proc"
)

var (
	hookOps = []proc.HookOps{{
		Hook:     start,
		Owner:    AppName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}}
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	proc.RegisterHooks(hookOps)

	cmd := proc.NewRootCmd(AppName)

	cmd.AddCommand(newHelloCmd())

	return cmd
}

func start(ctx context.Context) error {
	klog.Infof("entering start()")

	klog.Infof("Press ctrl-c to leave process")
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
