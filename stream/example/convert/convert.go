package main

import (
	"fmt"
	"os"
	"time"

	"github.com/yubo/golib/stream/convert"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage:%s <src.rec> <dst.cast>\n", os.Args[0])
		os.Exit(1)
	}
	if err := convert.Convert(os.Args[1], os.Args[2], time.Second); err != nil {
		fmt.Printf("err %s", err)
		os.Exit(1)
	}
}
