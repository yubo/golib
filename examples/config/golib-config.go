package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/proc"
	"sigs.k8s.io/yaml"
)

type config struct {
	UserName string `json:"userName" flag:"user-name" description:"user name"`
	UserAge  int    `json:"userAge" flag:"user-age" description:"user age"`
	City     string `json:"city" flag:"city" default:"beijing" description:"city"`
	Phone    string `json:"phone" flag:"phone" description:"phone number"`
}

const (
	moduleName = "golibConfig"
)

var (
	hookOps = []proc.HookOps{{
		Hook:     start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}}
)

func main() {
	if err := newServerCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newServerCmd() *cobra.Command {
	proc.RegisterHooks(hookOps)
	proc.RegisterFlags(moduleName, "golib examples", &config{})
	ctx := proc.WithName(context.Background(), "golibConfig")

	return proc.NewRootCmd(ctx)
}

func start(ctx context.Context) error {
	c := proc.ConfigerMustFrom(ctx)

	cf := &config{}
	if err := c.Read(moduleName, cf); err != nil {
		return err
	}

	b, _ := yaml.Marshal(cf)
	fmt.Printf("[%s]\n%s\n", moduleName, string(b))

	os.Exit(0)
	return nil
}
