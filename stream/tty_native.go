// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"context"
	"fmt"
	"io"
	"sync"

	mobyterm "github.com/yubo/golib/term/moby/term"
	"github.com/yubo/golib/term"
)

type NativeTty struct {
	sync.RWMutex
	in        io.Reader
	out       io.Writer
	errOut    io.Writer
	stdin     bool
	isTty     bool
	tty       *term.TTY
	sizeQueue term.TerminalSizeQueue
	err       error
	ctx       context.Context
	cancel    context.CancelFunc
	streams   TtyStreams
	// for testing
	overrideStreams func() (io.ReadCloser, io.Writer, io.Writer)
	isTerminalIn    func(t *term.TTY) bool
}

var _ Tty = &NativeTty{}

func NewNativeTty(in io.Reader, out, errOut io.Writer) (*NativeTty, error) {
	p := &NativeTty{
		in:     in,
		out:    out,
		errOut: errOut,
		stdin:  true,
		isTty:  true,
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())

	p.tty = p.SetupTTY()

	if p.tty.Raw {
		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		p.errOut = nil
	}

	if p.in != nil {
		p.streams.Stdin = ReadFunc(p.io(p.in.Read))
	}
	if p.out != nil {
		p.streams.Stdout = WriteFunc(p.io(p.out.Write))
	}
	if p.errOut != nil {
		p.streams.Stderr = WriteFunc(p.io(p.errOut.Write))
	}

	return p, nil
}

func (p *NativeTty) Safe(fn term.SafeFunc) error {
	return p.tty.Safe(fn)
}

func (p *NativeTty) Streams() TtyStreams {
	return p.streams
}

func (p *NativeTty) Close() error {
	p.cancel()

	return p.err
}

func (p *NativeTty) Err() error {
	p.RLock()
	defer p.RUnlock()
	return p.err
}

func (p *NativeTty) Done() <-chan struct{} {
	return p.ctx.Done()
}

func (p *NativeTty) CopyToPty(pty Pty) <-chan error {
	return CopyToPty(p, pty)
}

func (p *NativeTty) IsTerminal() bool {
	return p.tty.IsTerminalIn()
}

func (p *NativeTty) GetSize() *term.TerminalSize {
	return p.tty.GetSize()
}

func (p *NativeTty) MonitorSize(size ...*term.TerminalSize) term.TerminalSizeQueue {
	return p.tty.MonitorSize(size...)
}

func (p *NativeTty) SetupTTY() *term.TTY {
	t := &term.TTY{
		Out: p.out,
	}

	if !p.stdin {
		// need to nil out o.In to make sure we don't create a stream for stdin
		p.in = nil
		p.isTty = false
		return t
	}

	t.In = p.in
	if !p.isTty {
		return t
	}

	if p.isTerminalIn == nil {
		p.isTerminalIn = func(tty *term.TTY) bool {
			return tty.IsTerminalIn()
		}
	}
	if !p.isTerminalIn(t) {
		p.isTty = false

		if p.errOut != nil {
			fmt.Fprintln(p.errOut, "Unable to use a TTY - input is not a terminal or the right kind of file")
		}

		return t
	}

	// if we get to here, the user wants to attach stdin, wants a TTY, and o.In is a terminal, so we
	// can safely set t.Raw to true
	t.Raw = true

	if p.overrideStreams == nil {
		// use mobyterm.StdStreams() to get the right I/O handles on Windows
		p.overrideStreams = mobyterm.StdStreams
	}
	stdin, stdout, _ := p.overrideStreams()
	p.in = stdin
	t.In = stdin
	if p.out != nil {
		p.out = stdout
		t.Out = stdout
	}

	return t
}

func (p *NativeTty) io(fn func([]byte) (int, error)) func([]byte) (int, error) {
	return func(b []byte) (int, error) {
		select {
		case <-p.ctx.Done():
			return 0, io.EOF
		default:
		}

		n, err := fn(b)
		if err != nil {
			p.Lock()
			p.err = err
			p.Unlock()
			p.cancel()
			return 0, err
		}

		return n, nil
	}
}
