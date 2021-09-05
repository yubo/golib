package stream

import (
	"io"
	"strings"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

var (
	RealCrash = false
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

func crash() {
	if r := recover(); r != nil {
		if RealCrash {
			panic(r)
		}
	}
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
