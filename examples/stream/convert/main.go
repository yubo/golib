package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yubo/golib/stream/convert"
)

func main() {
	in := flag.String("in", "./testdata/example.rec", "record file")
	out := flag.String("out", "./testdata/example.cast", "output file(asciinema styly)")
	flag.Parse()

	if err := convert.Convert(*in, *out, time.Second); err != nil {
		fmt.Printf("err %s", err)
		os.Exit(1)
	}
}
