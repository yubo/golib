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
	ctx := proc.WithName(context.Background(), AppName)

	cmd := proc.NewRootCmd(ctx)
	cmd.AddCommand(newHelloCmd())

	return cmd
}

func start(ctx context.Context) error {
	klog.Infof("entering start()")
	defer klog.Infof("leaving start()")

	return nil
}

func newHelloCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "hello",
		Short:        "hello",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.Infof("entering hello()")
			defer klog.Infof("leaving hello()")
			return nil
		},
	}
}