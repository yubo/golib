// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"context"
	"io"
	"sync"

	"github.com/yubo/golib/term"
	"k8s.io/klog/v2"
)

type StreamTty struct {
	sync.RWMutex
	in      io.Reader
	out     io.WriteCloser
	errOut  io.WriteCloser
	isTty   bool
	resize  <-chan term.TerminalSize
	size    *term.TerminalSize
	sizeCh  chan *term.TerminalSize
	err     error
	ctx     context.Context
	cancel  context.CancelFunc
	streams TtyStreams
}

var _ Tty = &StreamTty{}

func NewStreamTty(ctx context.Context, in io.Reader, out, errOut io.WriteCloser, isTty bool, resize <-chan term.TerminalSize) *StreamTty {
	klog.V(10).Infof("istty %v", isTty)
	p := &StreamTty{
		in:     in,
		out:    out,
		errOut: errOut,
		isTty:  isTty,
		resize: resize,
		sizeCh: make(chan *term.TerminalSize),
	}
	p.ctx, p.cancel = context.WithCancel(ctx)

	if isTty {
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

	return p
}

func (p *StreamTty) Close() error {
	if p.out != nil {
		p.out.Close()
	}
	if p.errOut != nil {
		p.errOut.Close()
	}

	p.cancel()

	return nil
}

func (p *StreamTty) Err() error {
	p.RLock()
	defer p.RUnlock()
	return p.err
}

func (p *StreamTty) Done() <-chan struct{} {
	return p.ctx.Done()
}

func (p *StreamTty) Streams() TtyStreams {
	return p.streams
}

func (p *StreamTty) IsTerminal() bool {
	return p.isTty
}

func (p *StreamTty) GetSize() *term.TerminalSize {
	p.RLock()
	defer p.RUnlock()

	if p.size == nil {
		return &term.TerminalSize{}
	}

	return &term.TerminalSize{
		Height: p.size.Height,
		Width:  p.size.Width,
	}
}

func (p *StreamTty) MonitorSize(initialSizes ...*term.TerminalSize) term.TerminalSizeQueue {
	go func() {
		for _, size := range initialSizes {
			debug().Infof("size %v", size)
			p.sizeCh <- size
		}
		for size := range p.resize {
			p.sizeCh <- &size
		}
	}()

	return p
}

func (p *StreamTty) Next() *term.TerminalSize {
	select {
	case <-p.ctx.Done():
		return nil
	case size := <-p.sizeCh:
		p.RLock()
		defer p.RUnlock()

		p.size = size
		return size
	}
}

func (p *StreamTty) CopyToPty(pty Pty) <-chan error {
	return CopyToPty(p, pty)
}

func (p *StreamTty) io(fn func([]byte) (int, error)) func([]byte) (int, error) {
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
