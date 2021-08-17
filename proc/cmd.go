package proc

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	cliflag "github.com/yubo/golib/cli/flag"
	"github.com/yubo/golib/cli/globalflag"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/util/term"
	"k8s.io/klog/v2"
)

func ApplyToCmd(ctx context.Context, cmd *cobra.Command) error {
	name := NameFrom(ctx)
	proc.ctx = ctx

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if klog.V(5).Enabled() {
			fs := cmd.Flags()
			cliflag.PrintFlags(fs)
		}
		return startCmd()
	}

	fs := cmd.Flags()
	fs.ParseErrorsWhitelist.UnknownFlags = true

	globalflag.AddGlobalFlags(fs, name)

	return nil
}

// with cliflag section
func NewRootCmd(ctx context.Context) *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	name := NameFrom(ctx)

	cmd := &cobra.Command{
		Use:          name,
		Short:        fmt.Sprintf("%s service", name),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if klog.V(5).Enabled() {
				fs := cmd.Flags()
				cliflag.PrintFlags(fs)
			}
			return startCmd()
		},
	}

	// add flags
	fs := cmd.Flags()
	fs.ParseErrorsWhitelist.UnknownFlags = true
	configer.SetOptions(true, false, 5, fs)
	namedFlagSets := NamedFlagSets()
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), name)
	configer.Setting.AddFlags(namedFlagSets.FlagSet("global"))
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.GetTerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), *namedFlagSets, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), *namedFlagSets, cols)
	})

	proc.ctx = ctx

	return cmd
}

func RegisterFlags(path, groupName string, sample interface{}) {
	configer.AddConfigs(NamedFlagSets().FlagSet(groupName), path, sample)
}

func startCmd() error {
	return proc.start()
}
