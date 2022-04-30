package proc

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/cli/flag"
	"github.com/yubo/golib/util/term"
)

func PrintErrln(err error) int {
	if err == nil || err == DryRunErr {
		return 0
	}

	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	return 1
}

func envOr(name string, defs ...string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	for _, def := range defs {
		if def != "" {
			return def
		}
	}
	return ""
}

func getenvBool(str string) bool {
	b, _ := strconv.ParseBool(os.Getenv(str))
	return b
}

func sigContains(v os.Signal, sigs []os.Signal) bool {
	for _, sig := range sigs {
		if sig == v {
			return true
		}
	}
	return false
}

func nameOfFunction(f interface{}) string {
	fun := runtime.FuncForPC(reflect.ValueOf(f).Pointer())
	tokenized := strings.Split(fun.Name(), ".")
	last := tokenized[len(tokenized)-1]
	last = strings.TrimSuffix(last, ")·fm") // < Go 1.5
	last = strings.TrimSuffix(last, ")-fm") // Go 1.5
	last = strings.TrimSuffix(last, "·fm")  // < Go 1.5
	last = strings.TrimSuffix(last, "-fm")  // Go 1.5
	return last
}

func setGroupCommandFunc(cmd *cobra.Command, nfs flag.NamedFlagSets) {
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.GetTerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		flag.PrintSections(cmd.OutOrStderr(), nfs, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		flag.PrintSections(cmd.OutOrStdout(), nfs, cols)
	})
}
