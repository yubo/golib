package main

import (
	"context"
	"fmt"
	"os"
	"os/user"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/proc"
	"sigs.k8s.io/yaml"
)

type config struct {
	UserName string `json:"userName" flag:"user-name" env:"USER_NAME" description:"user name"`
	UserAge  int    `json:"userAge" flag:"user-age" env:"USER_AGE" description:"user age"`
	City     string `json:"city" flag:"city" env:"USER_CITY" default:"beijing" description:"city"`
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
		fmt.Println(err)
		os.Exit(1)
	}
}

func newServerCmd() *cobra.Command {
	// register hookOps as a module
	proc.RegisterHooks(hookOps)

	// register config{} to configer.Factory
	{
		c := &config{}
		if u, err := user.Current(); err == nil {
			c.UserName = u.Username
		}
		proc.RegisterFlags(moduleName, "golib examples", c)
	}

	return proc.NewRootCmd(moduleName, proc.WithoutLoop())
}

func start(ctx context.Context) error {
	c := proc.ConfigerMustFrom(ctx)

	cf := &config{}
	if err := c.Read(moduleName, cf); err != nil {
		return err
	}

	b, _ := yaml.Marshal(cf)
	fmt.Printf("%s", string(b))

	return nil
}
