package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yubo/golib/term"
	"github.com/yubo/golib/util"
)

const (
	// http://ascii-table.com/ansi-escape-sequences-vt-100.php
	EL0 = "\033[K"
)

var out = os.Stdout

type Item struct {
	Field1 int
	Field2 int
	Field3 int
	Field4 int
	Field5 int
	Field6 int
	Field7 int
}

func printf(out io.Writer, in []byte) int {
	lines := strings.Split(strings.TrimSpace(string(in)), "\n")
	for _, line := range lines {
		fmt.Printf("\r%s\033[K\n", strings.TrimSpace(line))
	}
	return len(lines)
}

func pageDemo(out io.Writer, page, pageSize int) int {
	items := []*Item{}
	for i := 0; i < pageSize; i++ {
		j := page
		items = append(items, &Item{j, j, j, j, j, j, j})
	}
	n := printf(out, util.Table(items))
	return n
}

func footBar(out io.Writer, page int) {
	fmt.Printf("\rpage %02d ", page)
}

func demo(out io.Writer, page, pageSize int) {
	n := pageDemo(out, page, pageSize)
	for {
	retry:
		a, b, err := term.Getch()
		if err != nil {
			return
		}

		switch a {
		case 'q', byte(3):
			return
		}

		switch b {
		case term.TERM_CODE_UP, term.TERM_CODE_LEFT:
			if page == 0 {
				goto retry
			}
			page--
		case term.TERM_CODE_DOWN, term.TERM_CODE_RIGHT:
			page++
		}
		fmt.Fprintf(out, "\033[%dA", n)
		n = pageDemo(out, page, pageSize)
		footBar(out, page)
	}
}

func main() {
	demo(out, 0, 10)
}
