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

func NewRootCmd(ctx context.Context) *cobra.Command {
	return DefaultProcess.NewRootCmd(ctx)
}

func InitProcFlags(cmd *cobra.Command) {
	DefaultProcess.InitProcFlags(cmd)
}

// with flag section
func (p *Process) NewRootCmd(ctx context.Context) *cobra.Command {
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

	p.InitProcFlags(cmd)

	DefaultProcess.ctx, DefaultProcess.cancel = context.WithCancel(ctx)

	return cmd
}

func (p *Process) InitProcFlags(cmd *cobra.Command) {
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

	// add registed fs into cmd.Flags
	for _, f := range nfs.FlagSets {
		fs.AddFlagSet(f)
	}

	if p.group {
		setGroupCommandFunc(cmd)
	}

}

func RegisterFlags(configPath, groupName string, sample interface{}, opts ...configer.ConfigFieldsOption) {
	configer.RegisterConfigFields(NamedFlagSets().FlagSet(groupName), configPath, sample, opts...)
}
