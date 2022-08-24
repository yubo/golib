// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"context"
	"fmt"
	"io"
	"sync"
	"unsafe"

	mobyterm "github.com/yubo/golib/term/moby/term"
	"github.com/yubo/golib/util/list"
	"github.com/yubo/golib/term"
)

type ProxyTty struct {
	sync.RWMutex
	ttys      *list.ListHead
	recorders *list.ListHead
	stdin     io.ReadCloser
	stdinPipe io.WriteCloser
	sizeCh    chan *term.TerminalSize
	size      *term.TerminalSize
	err       error
	ctx       context.Context
	cancel    context.CancelFunc
}

var _ Tty = &ProxyTty{}

func NewProxyTty(parent context.Context, bsize int) *ProxyTty {
	ctx, cancel := context.WithCancel(parent)
	p := &ProxyTty{
		ctx:       ctx,
		cancel:    cancel,
		ttys:      &list.ListHead{},
		recorders: &list.ListHead{},
		sizeCh:    make(chan *term.TerminalSize),
		size:      &term.TerminalSize{},
	}

	p.stdin, p.stdinPipe = io.Pipe()

	p.ttys.Init()
	p.recorders.Init()

	return p
}

func (p *ProxyTty) CopyToPty(pty Pty) <-chan error {
	return CopyToPty(p, pty)
}

func (t *ProxyTty) GetSize() *term.TerminalSize {
	return &term.TerminalSize{
		Height: t.size.Height,
		Width:  t.size.Width,
	}
}

func (p *ProxyTty) Close() error {
	p.Lock()
	defer p.Unlock()

	// close ttys
	h := p.ttys
	for p1 := h.Next; p1 != h; p1 = p1.Next {
		list2ttyEntry(p1).tty.Close()
	}

	// close recorders
	h = p.recorders
	for p1 := h.Next; p1 != h; p1 = p1.Next {
		list2recorderEntry(p1).recorder.Close()
	}

	p.stdin.Close()

	p.cancel()
	return nil
}

func (p *ProxyTty) Err() error {
	p.RLock()
	defer p.RUnlock()
	return p.err
}

func (p *ProxyTty) Done() <-chan struct{} {
	return p.ctx.Done()
}

func (p *ProxyTty) Streams() TtyStreams {
	return TtyStreams{
		Stdin:  p.stdin,
		Stdout: WriteFunc(p.writeOut),
		Stderr: WriteFunc(p.writeErrOut),
	}
}

func (p *ProxyTty) IsTerminal() bool {
	return true
}

func (p *ProxyTty) MonitorSize(initialSizes ...*term.TerminalSize) term.TerminalSizeQueue {
	go func() {
		for _, size := range initialSizes {
			debug().Infof("size %v", size)
			p.sizeCh <- size
		}
	}()
	return p
}

func (p *ProxyTty) Next() *term.TerminalSize {
	select {
	case <-p.ctx.Done():
		return nil
	case <-p.sizeCh:
	}

	p.RLock()
	defer p.RUnlock()

	// find minSize by tty.GetSize()
	size := &term.TerminalSize{}
	h := p.ttys
	for p1 := h.Next; p1 != h; p1 = p1.Next {
		size = minSize(size, list2ttyEntry(p1).tty.GetSize())
	}

	p.size = size

	if size.Height == 0 || size.Width == 0 {
		return size
	}

	// send to recorder
	h = p.recorders
	for p1 := h.Next; p1 != h; p1 = p1.Next {
		list2recorderEntry(p1).recorder.Resize(size)
	}
	return size
}

// s1 !== nil
func minSize(s1, s2 *term.TerminalSize) *term.TerminalSize {
	if s2 == nil || s2.Height == 0 || s2.Width == 0 {
		return s1
	}

	if (s1.Height == 0 || s1.Width == 0) || (s2.Height < s1.Height && s2.Width < s1.Width) {
		return s2
	}

	return s1
}

type ttyEntry struct {
	list    list.ListHead
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	tty     Tty
	options *Options
}

type recorderEntry struct {
	list     list.ListHead
	in       io.Writer
	out      io.Writer
	errOut   io.Writer
	recorder Recorder
}

type Options struct {
	detach     bool
	detachKeys []byte
	err        error
}

type Opt func(*Options)

func WithDetach(detach bool, detachKeys string) Opt {
	return func(o *Options) {
		o.detach = detach
		o.detachKeys, o.err = mobyterm.ToBytes(detachKeys)
	}
}

func (p *ProxyTty) AddRecorder(r Recorder) error {
	p.Lock()
	defer p.Unlock()

	s := r.Streams()
	entry := &recorderEntry{
		in:       s.Stdin,
		out:      s.Stdout,
		errOut:   s.Stderr,
		recorder: r,
	}
	p.recorders.AddTail(&entry.list)

	return nil
}

// tty will be closed by proxyTty, when proxyTty at closed
func (p *ProxyTty) AddTty(tty Tty, opts ...Opt) error {
	p.Lock()
	defer p.Unlock()

	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	if options.err != nil {
		return options.err
	}

	s := tty.Streams()
	entry := &ttyEntry{
		tty:     tty,
		stdin:   s.Stdin,
		stdout:  s.Stdout,
		stderr:  s.Stderr,
		options: options,
	}

	return p.addTtyEntry(entry)
}

func (p *ProxyTty) addTtyEntry(entry *ttyEntry) error {
	p.ttys.AddTail(&entry.list)

	// start stdin stream
	go func() {
		reader := entry.stdin
		if reader == nil {
			return
		}
		debug().Infof("attach: stdin: begin")
		defer debug().Infof("attach: stdin: end")

		if entry.tty.IsTerminal() {
			reader = NewEscapeProxy(reader, entry.options.detachKeys)
		}

		reader = p.recorderStdinProxy(reader)

		_, err := io.Copy(p.stdinPipe, reader)

		if err != nil {
			debug().Infof("error on attach stdin")
			p.Lock()
			p.err = fmt.Errorf("error on attach stdin: [%w]", err)
			p.Unlock()
		}
		entry.tty.Close()
	}()

	// start tty monitor Resize
	if entry.tty.IsTerminal() {
		sizeQueue := entry.tty.MonitorSize()
		go func() {
			p.sizeCh <- entry.tty.GetSize()
			debug().Infof("attach: sizeQueue: begin")
			defer debug().Infof("attach: sizeQueue: end")

			for {
				size := sizeQueue.Next()
				if size == nil {
					entry.tty.Close()
					return
				}
				p.sizeCh <- size
			}
		}()
	}

	return nil
}

func list2ttyEntry(list *list.ListHead) *ttyEntry {
	return (*ttyEntry)(unsafe.Pointer((uintptr(unsafe.Pointer(list)) - unsafe.Offsetof(((*ttyEntry)(nil)).list))))
}

func list2recorderEntry(list *list.ListHead) *recorderEntry {
	return (*recorderEntry)(unsafe.Pointer((uintptr(unsafe.Pointer(list)) - unsafe.Offsetof(((*recorderEntry)(nil)).list))))
}

func (p *ProxyTty) writeOut(b []byte) (int, error) {
	p.Lock()
	defer p.Unlock()
	debug().InfoS("entering proxy.out.write", "len", len(b))

	select {
	case <-p.ctx.Done():
		return 0, io.EOF
	default:
	}

	h := p.ttys
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		entryWrite("tty.out", p1, list2ttyEntry(p1).stdout, b)
	}

	h = p.recorders
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		entryWrite("recorder.out", p1, list2recorderEntry(p1).out, b)
	}

	debug().InfoS("leaving proxy.out.write", "len", len(b), "data", string(b))
	return len(b), nil
}

func (p *ProxyTty) writeErrOut(b []byte) (int, error) {
	p.Lock()
	defer p.Unlock()
	debug().InfoS("entering proxy.ErrOut.write", "len", len(b))

	select {
	case <-p.ctx.Done():
		return 0, io.EOF
	default:
	}

	h := p.ttys
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		entryWrite("tty.errout", p1, list2ttyEntry(p1).stderr, b)
	}

	h = p.recorders
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		entryWrite("recorder.errout", p1, list2recorderEntry(p1).errOut, b)
	}

	debug().InfoS("leaving proxy.ErrOut.Write", "len", len(b))

	return len(b), nil
}

func entryWrite(msg string, list *list.ListHead, writer io.Writer, b []byte) {
	if writer == nil {
		return
	}
	_, err := writer.Write(b)
	if err != nil {
		debug().Infof("write %s err %v, remove", msg, err)
		list.Del()
		return
	}
}

type recorderStdinProxy struct {
	r io.Reader
	p *ProxyTty
}

func (p *ProxyTty) recorderStdinProxy(reader io.Reader) io.Reader {
	return &recorderStdinProxy{
		p: p,
		r: reader,
	}

}

func (r *recorderStdinProxy) Read(buf []byte) (n int, err error) {
	n, err = r.r.Read(buf)
	if err != nil {
		return n, err
	}

	r.p.Lock()
	defer r.p.Unlock()

	h := r.p.recorders
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		entryWrite("recorder.in", p1, list2recorderEntry(p1).in, buf[:n])
	}

	return n, err
}
