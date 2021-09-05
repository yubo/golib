package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yubo/golib/stream"
	"k8s.io/klog/v2"
)

/*
nativeTty -> player
*/

func do() error {
	// tty native
	nativeTty, err := stream.NewNativeTty(os.Stdin, os.Stdout, os.Stderr)
	klog.Infof("native %v err %v", nativeTty, err)
	if err != nil {
		return err
	}
	defer nativeTty.Close()

	// player
	player, err := stream.NewPlayer("/tmp/test.rec", 1, true, time.Second)
	klog.Infof("player %v err %v", player, err)
	if err != nil {
		return err
	}
	defer player.Close()

	// run
	return nativeTty.Safe(func() error {
		return <-nativeTty.Bind(player)
		//return <-util.BindPtyStreams(nativeTty.Streams(), player.Streams())
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
