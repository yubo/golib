// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"fmt"
	"io"

	mobyterm "github.com/moby/term"
	"github.com/yubo/golib/util/term"
)

type NativeTty struct {
	TtyStreams
	Stdin bool
	TTY   bool

	// for testing
	overrideStreams func() (io.ReadCloser, io.Writer, io.Writer)
	isTerminalIn    func(t *term.TTY) bool

	tty       *term.TTY
	sizeQueue term.TerminalSizeQueue
}

func NewNativeTty(in io.Reader, out, errOut io.Writer) (*NativeTty, error) {
	t := &NativeTty{
		TtyStreams: TtyStreams{
			In:     in,
			Out:    out,
			ErrOut: errOut,
		},
		Stdin: true,
		TTY:   true,
	}

	t.tty = t.SetupTTY()

	//if t.tty.Raw {
	//	// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
	//	// true
	//	t.ErrOut = nil
	//}

	return t, nil
}

func (t *NativeTty) Safe(fn term.SafeFunc) error {
	return t.tty.Safe(fn)
}

func (t *NativeTty) Streams() TtyStreams {
	return t.TtyStreams
}

func (t *NativeTty) Close() error {
	return nil
}

func (t *NativeTty) Bind(pty Pty) <-chan error {
	return BindPty(t, pty)
}

func (t *NativeTty) IsTerminal() bool {
	return t.tty.IsTerminalIn()
}

func (t *NativeTty) GetSize() *term.TerminalSize {
	return t.tty.GetSize()
}

func (t *NativeTty) MonitorSize(size ...*term.TerminalSize) term.TerminalSizeQueue {
	return t.tty.MonitorSize(size...)
}

func (o *NativeTty) SetupTTY() *term.TTY {
	t := &term.TTY{
		Out: o.Out,
	}

	if !o.Stdin {
		// need to nil out o.In to make sure we don't create a stream for stdin
		o.In = nil
		o.TTY = false
		return t
	}

	t.In = o.In
	if !o.TTY {
		return t
	}

	if o.isTerminalIn == nil {
		o.isTerminalIn = func(tty *term.TTY) bool {
			return tty.IsTerminalIn()
		}
	}
	if !o.isTerminalIn(t) {
		o.TTY = false

		if o.ErrOut != nil {
			fmt.Fprintln(o.ErrOut, "Unable to use a TTY - input is not a terminal or the right kind of file")
		}

		return t
	}

	// if we get to here, the user wants to attach stdin, wants a TTY, and o.In is a terminal, so we
	// can safely set t.Raw to true
	t.Raw = true

	if o.overrideStreams == nil {
		// use mobyterm.StdStreams() to get the right I/O handles on Windows
		o.overrideStreams = mobyterm.StdStreams
	}
	stdin, stdout, _ := o.overrideStreams()
	o.In = stdin
	t.In = stdin
	if o.Out != nil {
		o.Out = stdout
		t.Out = stdout
	}

	return t
}
