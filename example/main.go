package main

import (
	"fmt"
	"os"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/logs"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
	"k8s.io/klog/v2"

	_ "github.com/yubo/golib/orm/mysql"
	_ "github.com/yubo/golib/proc/audit"
	_ "github.com/yubo/golib/proc/db"
	_ "github.com/yubo/golib/proc/grpc"
	_ "github.com/yubo/golib/proc/http"
	_ "github.com/yubo/golib/proc/logging"
	_ "github.com/yubo/golib/proc/metrics"
	_ "github.com/yubo/golib/proc/sys"
)

const (
	AppName    = "helo"
	moduleName = "helo.main"
)

var (
	hookOps = []proc.HookOps{{
		Hook:     startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}, {
		Hook:     stopHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_MODULE,
	}}
	buildReporter proc.Reporter
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	opts := proc.NewOptions().SetName(AppName)
	proc.RegisterHooksWithOptions(hookOps, opts)

	command := proc.NewRootCmd(os.Args[1:])
	if err := command.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func startHook(ops *proc.HookOps, cf *proc.Configer) error {
	klog.Info("start")

	popts := ops.Options()

	buildReporter = proc.NewBuildReporter(popts)
	if err := buildReporter.Start(); err != nil {
		return err
	}

	installWs(popts.Http())

	// dblogger
	return nil
}

func stopHook(ops *proc.HookOps, cf *proc.Configer) error {
	klog.Info("stop")
	buildReporter.Stop()
	return nil
}

func installWs(server proc.HttpServer) error {
	server.SwaggerTagRegister("helo", "helo demo")

	ws := new(restful.WebService)
	opt := &openapi.WsOption{
		Ws:   ws.Path("/helo").Produces(openapi.MIME_JSON),
		Tags: []string{"helo"},
	}

	openapi.WsRouteBuild(opt, []openapi.WsRoute{{
		Desc: "hello", Acl: "a",
		Method: "GET", SubPath: "/info",
		Handle: helo,
		Input:  heloInput{},
		Output: heloOutput{},
	}})

	server.Add(ws)
	return nil
}

func helo(req *restful.Request, resp *restful.Response) {
	in := &heloInput{}

	if err := openapi.ReadEntity(req, in); err != nil {
		openapi.HttpWriteData(resp, nil, err)
		return
	}

	openapi.HttpWriteData(resp,
		fmt.Sprintf("hello, %s", in.Name), nil)
}

type heloInput struct {
	Name string `param:"query" json:"name"`
}

type heloOutput struct {
	Error string `json:"err" description:"error msg"`
	Data  string `json:"data"`
}
