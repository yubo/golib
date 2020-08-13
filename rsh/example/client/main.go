package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/rsh"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

type ExecOption struct {
	Cmd  []string `param:"query" description:"cmd desc"`
	Foo  string   `param:"query" description:"foo desc"`
	Bar  int      `param:"query" description:"bar desc"`
	Data string   `param:"data" description:"data desc"`
}

func main() {

	klog.InitFlags(nil)
	flag.Set("v", "3")
	flag.Parse()

	opt := &openapi.RequestOption{
		Url:    "http://localhost:18080/exec",
		Bearer: util.String("1"),
		Input: &ExecOption{
			Cmd:  os.Args[1:],
			Foo:  os.Args[0],
			Bar:  len(os.Args),
			Data: "hello,world",
		},
	}

	if err := rsh.RshRequest(opt); err != nil {
		fmt.Printf("[err]Communication error: %v\n", err)
	}
	os.Exit(0)

}
