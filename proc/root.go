package proc

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

func NewRootCmd(settings *Settings, args []string) *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	cmd := &cobra.Command{
		Use:          settings.Name,
		Short:        fmt.Sprintf("%s service", settings.Name),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return startCmd(settings)
		},
	}

	flags := cmd.PersistentFlags()
	settings.AddFlags(flags)

	flags.ParseErrorsWhitelist.UnknownFlags = true
	flags.Parse(args)

	cmd.AddCommand(
		newStartCmd(settings),
		newVersionCmd(settings),
	)

	if settings.Changelog != "" {
		cmd.AddCommand(newChangelogCmd(settings))
	}

	if settings.Asset != nil {
		cmd.AddCommand(newResetDbCmd(settings))
	}

	return cmd
}

// handle
func newStartCmd(settings *Settings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: fmt.Sprintf("start %s service", settings.Name),
		RunE: func(cmd *cobra.Command, args []string) error {
			return startCmd(settings)
		},
	}
	return cmd
}

func startCmd(settings *Settings) error {
	klog.Infof("config file %s\n", settings.Config)
	if settings.TestConfig {
		return ConfigTest(settings.Config)
	}
	return MainLoop(settings.Config)
}

func newVersionCmd(settings *Settings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "show version, git commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			os.Stdout.Write(util.Table(settings.Version))
			return nil
		},
	}
	return cmd
}

func newChangelogCmd(settings *Settings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "list changelog",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(settings.Changelog)
			return nil
		},
	}
	return cmd
}

func newResetDbCmd(settings *Settings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resetdb",
		Short: "drop table if exist and initialize it",
		RunE: func(cmd *cobra.Command, args []string) error {
			return resetDb(settings)
		},
	}
	return cmd
}
