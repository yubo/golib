package proc

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func NewRootCmd(args []string) *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	if _module.options == nil {
		panic(errNotSetted)
	}

	appName := _module.options.Name()

	cmd := &cobra.Command{
		Use:          appName,
		Short:        fmt.Sprintf("%s service", appName),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return startCmd()
		},
	}

	fs := cmd.PersistentFlags()

	fs.StringVarP(&_module.config, "config", "c",
		os.Getenv(strings.ToUpper(appName)+"_CONFIG"),
		fmt.Sprintf("config file path of your %s server.", appName))
	fs.BoolVarP(&_module.test, "test", "t", false,
		fmt.Sprintf("test config file path of your %s server.", appName))

	fs.ParseErrorsWhitelist.UnknownFlags = true
	fs.Parse(args)

	cmd.AddCommand(
		newStartCmd(),
		newVersionCmd(),
	)

	return cmd
}

// handle
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: fmt.Sprintf("start %s service", _module.options.Name()),
		RunE: func(cmd *cobra.Command, args []string) error {
			return startCmd()
		},
	}
	return cmd
}

func startCmd() error {
	klog.Infof("config file %s\n", _module.config)

	if _module.test {
		return _module.testConfig()
	}

	return _module.start()
}

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "show version, git commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Go Runtime version: %s\n", goVersion)
			fmt.Printf("OS:                 %s\n", goOs)
			fmt.Printf("Arch:               %s\n", goArch)
			fmt.Printf("Build Version:      %s\n", Version)
			fmt.Printf("Build Revision:     %s\n", Revision)
			fmt.Printf("Build Branch:       %s\n", Branch)
			fmt.Printf("Build User:         %s\n", Builder)
			fmt.Printf("Build Date:         %s\n", BuildDate)
			fmt.Printf("Build TimeUnix:     %s\n", BuildTimeUnix)
			return nil
		},
	}
	return cmd
}
