// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"io"
	"strings"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

func wrapErr(err error) error {
	if err == nil {
		return nil
	}

	if err == io.EOF {
		klog.V(5).Infof("error %s", err)
		return nil
	}

	if strings.Contains(err.Error(), "input/output error") {
		klog.V(5).Infof("error %s -> nil", err)
		return nil
	}

	return err
}

func BindPtyStreams(tty TtyStreams, pty PtyStreams) chan error {
	ch := make(chan error)
	go func() {
		if tty.In != nil && pty.In != nil {
			_, err := io.Copy(pty.In, tty.In)
			ch <- wrapErr(err)
		}
	}()
	go func() {
		if tty.Out != nil && pty.Out != nil {
			_, err := io.Copy(tty.Out, pty.Out)
			ch <- wrapErr(err)
		}
	}()
	go func() {
		if tty.ErrOut != nil && pty.ErrOut != nil {
			_, err := io.Copy(tty.ErrOut, pty.ErrOut)
			ch <- wrapErr(err)
		}
	}()

	return ch
}

func BindPty(tty Tty, pty Pty) <-chan error {
	ch := BindPtyStreams(tty.Streams(), pty.Streams())

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
