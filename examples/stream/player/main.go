package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yubo/golib/stream"
)

/*
nativeTty -> player
*/

func play(file string) error {
	// tty native
	nativeTty, err := stream.NewNativeTty(os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	defer nativeTty.Close()

	// player
	player, err := stream.NewPlayer(file, 1, true, time.Second)
	if err != nil {
		return err
	}
	defer player.Close()

	return nativeTty.Safe(func() error {
		return <-nativeTty.CopyToPty(player)
	})
}

func main() {
	recfile := flag.String("file", "./testdata/example.rec", "record file")
	flag.Parse()

	if err := play(*recfile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
