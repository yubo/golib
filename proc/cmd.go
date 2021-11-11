package proc

import (
	"math/rand"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/configer"
)

func NewRootCmd(opts ...ProcessOption) *cobra.Command {
	return DefaultProcess.NewRootCmd(opts...)
}

func InitProcFlags(cmd *cobra.Command) {
	DefaultProcess.AddGlobalFlags(cmd)
}

// with flag section
func (p *Process) NewRootCmd(opts ...ProcessOption) *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	cmd := &cobra.Command{
		Use:          p.name,
		Short:        p.description,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return p.Start()
		},
	}

	p.Init(cmd, opts...)

	p.AddRegisteredFlags(cmd.Flags())

	return cmd
}

func RegisterFlags(configPath, groupName string, sample interface{}, opts ...configer.ConfigFieldsOption) {
	configer.Register(NamedFlagSets().FlagSet(groupName), configPath, sample, opts...)
}
