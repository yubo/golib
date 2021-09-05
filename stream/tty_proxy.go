package stream

import (
	"context"
	"io"
	"sync"
	"unsafe"

	"github.com/yubo/golib/util/list"
	"github.com/yubo/golib/util/term"
	"k8s.io/klog/v2"
)

type ProxyTty struct {
	sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	ttys      *list.ListHead
	recorders *list.ListHead
	//upstreams []*upstream
	in       chan *proxyMsg
	inErr    chan error
	buffSize int

	sizeCh chan *term.TerminalSize

	size term.TerminalSize
}

func NewProxyTty(bsize int) *ProxyTty {
	ctx, cancel := context.WithCancel(context.Background())
	tty := &ProxyTty{
		ctx:       ctx,
		cancel:    cancel,
		in:        make(chan *proxyMsg, 1),
		inErr:     make(chan error, 1),
		buffSize:  bsize,
		ttys:      &list.ListHead{},
		recorders: &list.ListHead{},
		sizeCh:    make(chan *term.TerminalSize),
	}

	tty.ttys.Init()
	tty.recorders.Init()

	return tty
}

func (p *ProxyTty) Bind(pty Pty) <-chan error {
	return BindPty(p, pty)
}

func (t *ProxyTty) GetSize() *term.TerminalSize {
	return &t.size
}

func (p *ProxyTty) Close() error {
	p.cancel()
	return nil
}

func (p *ProxyTty) Streams() TtyStreams {
	return TtyStreams{
		In:     ReadFunc(p.readInChan),
		Out:    WriteFunc(p.writeOut),
		ErrOut: WriteFunc(p.writeErrOut),
	}
}

func (p *ProxyTty) IsTerminal() bool {
	return true
}

func (p *ProxyTty) MonitorSize(initialSizes ...*term.TerminalSize) term.TerminalSizeQueue {
	go func() {
		for _, size := range initialSizes {
			klog.V(6).Infof("size %v", size)
			p.sizeCh <- size
		}
	}()
	return p
}

func (p *ProxyTty) Next() *term.TerminalSize {
	return <-p.sizeCh
}

type ttyEntry struct {
	list   list.ListHead
	in     io.Reader
	out    io.Writer
	errOut io.Writer
	tty    Tty
}

type recorderEntry struct {
	list   list.ListHead
	in     io.Writer
	out    io.Writer
	errOut io.Writer
}

func (p *ProxyTty) AddRecorderStreams(s RecorderStreams) error {
	p.Lock()
	defer p.Unlock()

	entry := &recorderEntry{
		in:     s.In,
		out:    s.Out,
		errOut: s.ErrOut,
	}
	p.recorders.AddTail(&entry.list)

	return nil
}

func (p *ProxyTty) AddTty(tty Tty) error {
	p.Lock()
	defer p.Unlock()

	s := tty.Streams()
	entry := &ttyEntry{
		tty:    tty,
		in:     s.In,
		out:    s.Out,
		errOut: s.ErrOut,
	}

	return p.addTtyEntry(entry)
}

func (p *ProxyTty) addTtyEntry(entry *ttyEntry) error {
	p.ttys.AddTail(&entry.list)

	// start stdin stream
	go func() {
		buff := make([]byte, p.buffSize)
		reader := entry.in
		if reader == nil {
			return
		}
		for {
			select {
			case <-p.ctx.Done():
				return
			default:
			}

			n, err := reader.Read(buff)
			klog.V(6).InfoS("proxy.tty.in.read", "len", n, "err", err)
			if err != nil {
				klog.Errorf("read from tty.in err %s", err)
				return
			}
			_, err = p.writeInChan(buff[:n])
			klog.V(6).InfoS("proxy.in.write", "len", n, "err", err)
			if err != nil {
				klog.Errorf("write to tty.in err %s", err)
				return
			}

		}
	}()

	// start tty monitor Resize
	if entry.tty.IsTerminal() {
		sizeQueue := entry.tty.MonitorSize()
		go func() {
			p.sizeCh <- entry.tty.GetSize()

			for {
				select {
				case <-p.ctx.Done():
					return
				default:
				}
				size := sizeQueue.Next()
				if size == nil {
					return
				}
				p.sizeCh <- size
			}
		}()
	}

	return nil
}

type proxyMsg struct {
	data []byte
	done chan error
}

func (p *ProxyTty) readInChan(b []byte) (int, error) {
	klog.V(6).InfoS("entering proxy.in.read")
	msg, ok := <-p.in
	if !ok {
		return 0, io.EOF
	}

	if len(b) < len(msg.data) {
		msg.done <- io.ErrShortBuffer
		return 0, io.ErrShortBuffer
	}

	copy(b, msg.data)

	p.Lock()
	defer p.Unlock()

	h := p.recorders
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		n, err := list2recorderEntry(p1).in.Write(msg.data)
		if err != nil {
			klog.V(6).Infof("write recorder.in err %v, remove", err)
			p1.Del()
			continue
		}
		if n != len(msg.data) {
			klog.V(6).Infof("write recorder.in err %v, remove", io.ErrShortWrite)
			p1.Del()
			continue
		}
	}

	msg.done <- nil

	klog.V(6).InfoS("leaving proxy.in.read", "len", len(msg.data))
	return len(msg.data), nil
}

func (p *ProxyTty) writeInChan(b []byte) (int, error) {
	msg := &proxyMsg{data: b, done: p.inErr}
	select {
	case <-p.ctx.Done():
		return 0, io.EOF
	case p.in <- msg:
		if err := <-msg.done; err != nil {
			return 0, err
		}
		return len(b), nil
	}
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
	klog.V(6).InfoS("entering proxy.out.write", "len", len(b))

	select {
	case <-p.ctx.Done():
		return 0, io.EOF
	default:
	}

	h := p.ttys
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		out := list2ttyEntry(p1).out
		if out == nil {
			continue
		}
		n, err := out.Write(b)
		if err != nil {
			klog.V(6).Infof("write tty.out err %v, remove", err)
			p1.Del()
			continue
		}
		if n != len(b) {
			klog.V(6).Infof("write tty.out err %v, remove", io.ErrShortWrite)
			p1.Del()
			continue
		}
	}

	h = p.recorders
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		out := list2recorderEntry(p1).out
		if out == nil {
			continue
		}
		n, err := out.Write(b)
		if err != nil {
			klog.V(6).Infof("write recorder.out err %v, remove", err)
			p1.Del()
			continue
		}
		if n != len(b) {
			klog.V(6).Infof("write recorder.out err %v, remove", io.ErrShortWrite)
			p1.Del()
			continue
		}
	}

	klog.V(6).InfoS("leaving proxy.out.write", "len", len(b), "data", string(b))
	return len(b), nil
}

func (p *ProxyTty) writeErrOut(b []byte) (int, error) {
	p.Lock()
	defer p.Unlock()
	klog.V(6).InfoS("entering proxy.ErrOut.write", "len", len(b))

	select {
	case <-p.ctx.Done():
		return 0, io.EOF
	default:
	}

	h := p.ttys
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		errOut := list2ttyEntry(p1).errOut
		if errOut == nil {
			continue
		}
		n, err := errOut.Write(b)
		if err != nil {
			klog.V(6).Infof("write tty.errout err %v, remove", err)
			p1.Del()
			continue
		}
		if n != len(b) {
			klog.V(6).Infof("write tty.errout err %v, remove", err)
			p1.Del()
			continue
		}
	}

	h = p.recorders
	for p1, p2 := h.Next, h.Next.Next; p1 != h; p1, p2 = p2, p2.Next {
		errOut := list2recorderEntry(p1).errOut
		if errOut == nil {
			continue
		}
		n, err := errOut.Write(b)
		if err != nil {
			klog.V(6).Infof("write recorder.errout err %v, remove", err)
			p1.Del()
			continue
		}
		if n != len(b) {
			klog.V(6).Infof("write recorder.errout err %v, remove", err)
			p1.Del()
			continue
		}
	}

	klog.V(6).InfoS("leaving proxy.ErrOut.Write", "len", len(b))

	return len(b), nil
}
