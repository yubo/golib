package proc

import (
	"context"
	"math/rand"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/cli/globalflag"
	"github.com/yubo/golib/configer"
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
			return DefaultProcess.Start(cmd)
		},
	}

	InitProcFlags(cmd, true, true, false, 5)

	DefaultProcess.ctx, DefaultProcess.cancel = context.WithCancel(ctx)

	return cmd
}

func InitProcFlags(cmd *cobra.Command, group, allowEnv, allowEmptyEnv bool, maxDepth int) {

	// add flags
	fs := cmd.Flags()
	fs.ParseErrorsWhitelist.UnknownFlags = true

	nfs := NamedFlagSets()

	// add klog, logs, help flags
	globalflag.AddGlobalFlags(nfs.FlagSet("global"))

	// add configer flags
	configer.AddFlags(nfs.FlagSet("global"))

	// add process flags
	AddFlags(nfs.FlagSet("global"))

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

func RegisterFlags(configPath, groupName string, sample interface{}, opts ...configer.ConfigerOption) {
	configer.RegisterConfigFields(NamedFlagSets().FlagSet(groupName), configPath, sample, opts...)
}
