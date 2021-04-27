package proc

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	cliflag "github.com/yubo/golib/staging/cli/flag"
	"github.com/yubo/golib/staging/cli/globalflag"
	"github.com/yubo/golib/staging/util/term"
	"k8s.io/klog/v2"
)

func NewRootCmd(ctx context.Context, args []string) *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	name := NameFrom(ctx)
	_module.ctx = ctx
	_module.options = newOptions(name)

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

	fs := cmd.Flags()
	fs.ParseErrorsWhitelist.UnknownFlags = true

	namedFlagSets := NamedFlagSets()
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), name)
	_module.options.addFlags(namedFlagSets.FlagSet("global"), name)

	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), *namedFlagSets, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), *namedFlagSets, cols)
	})

	return cmd
}

func startCmd() error {
	klog.Infof("config file %s\n", _module.options.configFile)

	if _module.options.test {
		return _module.testConfig()
	}

	return _module.start()
}
