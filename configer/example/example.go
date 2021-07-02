package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/staging/cli/globalflag"
	"k8s.io/klog/v2"
)

type templateFile struct {
	name     string
	contents string
}

type Config struct {
	Ctrl Ctrl `json:"ctrl"`
	Http Http `json:"http"`
}

type Http struct {
	Address    string        `json:"address" flag:"address" default:"127.0.0.1:80" description:"address desc"`
	ServerName string        `json:"serverName" flag:"server-name,s" env:"SERVER_NAME" default:"localhost" description:"server name"`
	Timeout    time.Duration `json:"timeout" flag:"timeout" default:"5s" description:"connction timeout"`
}

type Ctrl struct {
	AuthProvider map[string]Provider `json"auth"`
}

type Provider struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectUrl  string `json:"redirectUrl"`
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		klog.Fatal(err)
	}

}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "example",
		Short:        "example",
		SilenceUsage: true,
		RunE:         rootCmd,
	}

	fs := cmd.Flags()
	if err := configer.AddFlags(fs, "http", &Http{}); err != nil {
		klog.Infof("addflag err %s", err)
	}

	if err := configer.AddFlags(fs, "ctrl", &Ctrl{}); err != nil {
		klog.Infof("addflag err %s", err)
	}

	configer.Setting.AddFlags(fs)
	globalflag.AddGlobalFlags(fs, "example")

	return cmd
}

func rootCmd(cmd *cobra.Command, args []string) error {
	conf, err := configer.New(configer.WithFlag(cmd.Flags(), true, false, 5))
	if err != nil {
		klog.Fatal(err)
	}

	fmt.Printf("%+v\n", conf)
	return nil
}

// output:
// ./example  -f ./example.yaml
// ctrl:
//   auth:
//     google:
//       clientId: client-id
//       clientSecret: client-secret
//       redirectUrl: http://auth.example.com/v1/auth/callback
// http:
//   address: 127.0.0.1:80
//   serverName: localhost
//
// ./example  -f ./example.yaml  --address=1.1.1.1
// ctrl:
//   auth:
//     google:
//       clientId: client-id
//       clientSecret: client-secret
//       redirectUrl: http://auth.example.com/v1/auth/callback
// http:
//   address: 1.1.1.1
//   serverName: localhost
//
// SERVER_NAME=a.com ./example  -f ./example.yaml
// ctrl:
//   auth:
//     google:
//       clientId: client-id
//       clientSecret: client-secret
//       redirectUrl: http://auth.example.com/v1/auth/callback
// http:
//   address: 127.0.0.1:80
//   serverName: a.com
//

// ./example  -h
// example
//
// Usage:
//   example [flags]
//
// Flags:
//       --address string           address desc (default "127.0.0.1:80")
//   -h, --help                     help for example
//   -s, --server-name string       server name (env SERVER_NAME) (default "localhost")
//       --set stringArray          set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
//       --set-file stringArray     set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
//       --set-string stringArray   set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
//   -f, --values strings           specify values in a YAML file or a URL (can specify multiple)
