// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"io"
	"strings"
	"syscall"
	"time"

	"github.com/yubo/golib/util/term/moby/term"
	"k8s.io/klog/v2"
)

var defaultEscapeSequence = []byte{16, 17} // ctrl-p, ctrl-q

func CopyPtyStreams(tty TtyStreams, pty PtyStreams) chan error {
	ch := make(chan error)
	go func() {
		if tty.Stdin != nil && pty.Stdin != nil {
			_, err := io.Copy(pty.Stdin, tty.Stdin)
			ch <- err
		}
	}()
	go func() {
		if tty.Stdout != nil && pty.Stdout != nil {
			_, err := io.Copy(tty.Stdout, pty.Stdout)
			ch <- err
		}
	}()
	go func() {
		if tty.Stderr != nil && pty.Stderr != nil {
			_, err := io.Copy(tty.Stderr, pty.Stderr)
			ch <- err
		}
	}()

	return ch
}

func CopyToPty(tty Tty, pty Pty) <-chan error {
	ch := CopyPtyStreams(tty.Streams(), pty.Streams())

	klog.Infof("bind tty %v pty %v", tty.IsTerminal(), pty.IsTerminal())
	if tty.IsTerminal() && pty.IsTerminal() {
		if sizeQueue := tty.MonitorSize(tty.GetSize()); sizeQueue != nil {
			go func() {
				for {
					size := sizeQueue.Next()
					if size == nil {
						ch <- io.EOF
						return
					}
					if size.Height < 1 || size.Width < 1 {
						continue
					}
					pty.Resize(size)
				}
			}()
		}
	}

	return ch
}

func Nanotime() int64 {
	return time.Now().UnixNano()
}

func ioctl(fd, cmd, ptr uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr); e != 0 {
		return e
	}

	return nil
}

func EnvsToKv(envs []string) map[string]string {
	data := map[string]string{}
	for _, env := range envs {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) == 2 {
			data[pair[0]] = pair[1]
		}
	}
	return data
}

func copyBytes(dst, src []byte) (int, error) {
	if len(dst) < len(src) {
		return 0, io.ErrShortBuffer
	}

	copy(dst, src)
	return len(src), nil
}

func debug() klog.Verbose {
	return klog.V(debugLevel)
}

func copyEscapable(dst io.Writer, src io.Reader, keys []byte) (written int64, err error) {
	if len(keys) == 0 {
		keys = defaultEscapeSequence
	}

	pr := term.NewEscapeProxy(src, keys)

	return io.Copy(dst, pr)
}

func NewEscapeProxy(reader io.Reader, keys []byte) io.Reader {
	if len(keys) == 0 {
		keys = defaultEscapeSequence
	}

	return term.NewEscapeProxy(reader, keys)
}
