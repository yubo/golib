package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/yubo/golib/stream"
	"k8s.io/klog/v2"
)

/*
recorder ---┬--> ttyProxy -> pty
nativeTty --┘
*/

func do() error {
	// tty native
	nativeTty, err := stream.NewNativeTty(os.Stdin, os.Stdout, os.Stderr)
	klog.Infof("native %v err %v", nativeTty, err)
	if err != nil {
		return err
	}
	defer nativeTty.Close()

	// recorder
	recorder, err := stream.NewRecorder("/tmp/test.rec")
	klog.Infof("recorder %v err %v", recorder, err)
	if err != nil {
		return err
	}
	defer recorder.Close()

	// tty rpoxy
	tty := stream.NewProxyTty(1024)
	if err := tty.AddTty(nativeTty); err != nil {
		return err
	}
	if err := tty.AddRecorder(recorder); err != nil {
		return err
	}

	// pty
	pty, err := stream.NewCmdPty(exec.Command("bash"))
	klog.Infof("pty %v err %v", pty, err)
	if err != nil {
		return err
	}
	defer pty.Close()

	// run
	return nativeTty.Safe(func() error {
		return <-tty.Bind(pty)
	})
}

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "false")
	flag.Set("alsologtostderr", "false")
	flag.Parse()

	fd, err := os.OpenFile("/tmp/rec.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer fd.Close()

	klog.SetOutput(fd)
	defer klog.Flush()

	if err := do(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
