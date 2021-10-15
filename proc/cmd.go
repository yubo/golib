package proc

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/cli/flag"
	"github.com/yubo/golib/cli/globalflag"
	"github.com/yubo/golib/configer"
	"k8s.io/klog/v2"
)

// with flag section
func NewRootCmd(ctx context.Context) *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	name := NameFrom(ctx)

	cmd := &cobra.Command{
		Use:          name,
		Short:        DescriptionFrom(ctx),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if klog.V(5).Enabled() {
				flag.PrintFlags(cmd.Flags())
			}
			return proc.Start()
		},
	}

	InitFlags(cmd, true, true, false, 5)

	proc.ctx, proc.cancel = context.WithCancel(ctx)

	return cmd
}

func InitFlags(cmd *cobra.Command, group, allowEnv, allowEmptyEnv bool, maxDepth int) {

	// add flags
	fs := cmd.Flags()
	fs.ParseErrorsWhitelist.UnknownFlags = true

	nfs := NamedFlagSets()

	// add klog, logs, help flags
	globalflag.AddGlobalFlags(nfs.FlagSet("global"))

	// add configer flags
	configer.AddFlags(nfs.FlagSet("global"))

	// set configer options for init configer options fro parser
	configer.SetOptions(allowEnv, allowEmptyEnv, maxDepth, fs)

	// add registed fs into cmd.Flags
	for _, f := range nfs.FlagSets {
		fs.AddFlagSet(f)
	}

	if group {
		setGroupCommandFunc(cmd)
	}

}

func RegisterFlags(path, groupName string, sample interface{}, opts ...configer.Option) {
	configer.AddConfigs(NamedFlagSets().FlagSet(groupName), path, sample, opts...)
}
