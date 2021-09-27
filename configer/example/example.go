package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yubo/golib/cli/globalflag"
	"github.com/yubo/golib/configer"
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
	Address    string `json:"address" flag:"address" default:"127.0.0.1:80" description:"address desc"`
	ServerName string `json:"serverName" flag:"server-name,s" env:"SERVER_NAME" default:"localhost" description:"server name"`
	Timeout    int    `json:"timeout" flag:"timeout" default:"5" description:"connction timeout(Second)"`
	Alias      string `json:"alias" default:"test" description:"set defualt value without flag"`
}

type Ctrl struct {
	AuthProvider map[string]Provider `json:"auth"`
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
	if err := configer.AddConfigs(fs, "http", &Http{}); err != nil {
		klog.Infof("addflag err %s", err)
	}

	if err := configer.AddConfigs(fs, "ctrl", &Ctrl{}); err != nil {
		klog.Infof("addflag err %s", err)
	}

	globalflag.AddGlobalFlags(fs, "example")

	return cmd
}

func rootCmd(cmd *cobra.Command, args []string) error {
	configer.SetOptions(true, false, 5, cmd.Flags())
	conf, err := configer.New()
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
//   alias: test
//   serverName: localhost
//   timeout: 5
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
//   alias: test
//   serverName: a.com
//   timeout: 5
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
