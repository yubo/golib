package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/yubo/golib/stream"
)

/*
recorder ---┬--> ttyProxy -> pty
nativeTty --┘
*/

func record(outfile string) error {
	// tty native
	nativeTty, err := stream.NewNativeTty(os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	defer nativeTty.Close()

	// recorder
	fd, err := os.Create(outfile)
	if err != nil {
		return err
	}
	recorder, err := stream.NewRecorder(fd)
	if err != nil {
		return err
	}
	defer recorder.Close()

	// tty proxy
	tty := stream.NewProxyTty(context.Background(), 1024)
	if err := tty.AddTty(nativeTty); err != nil {
		return err
	}
	if err := tty.AddRecorder(recorder); err != nil {
		return err
	}

	// pty
	pty, err := stream.NewCmdPty(exec.Command("bash"))
	if err != nil {
		return err
	}
	defer pty.Close()

	// run
	return nativeTty.Safe(func() error {
		return <-tty.CopyToPty(pty)
	})
}

func main() {
	recfile := flag.String("file", "./testdata/example.rec", "record file")
	flag.Parse()

	if err := record(*recfile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
