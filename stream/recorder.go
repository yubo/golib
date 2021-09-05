// +build linux darwin

//from github.com/yubo/gotty/rec
package stream

import (
	"encoding/gob"
	"io"
	"os"
)

type RecorderStreams struct {
	In     io.Writer // default: ioutil.Discard
	Out    io.Writer
	ErrOut io.Writer
}

type RecData struct {
	Time int64
	Data []byte
}

type Recorder struct {
	FileName string
	f        *os.File
	enc      *gob.Encoder
}

func (p *Recorder) Streams() RecorderStreams {
	return RecorderStreams{
		In:     WriteFunc(p.WriteIn),
		Out:    WriteFunc(p.WriteOut),
		ErrOut: WriteFunc(p.WriteErrOut),
	}
}

func (p *Recorder) Close() error {
	return p.f.Close()
}

// TERM SHELL COMMAND
func NewRecorder(fileName string) (*Recorder, error) {
	r := &Recorder{FileName: fileName}

	var err error
	if r.f, err = os.Create(fileName); err != nil {
		return nil, err
	}
	r.enc = gob.NewEncoder(r.f)

	return r, nil
}

func (r *Recorder) WriteIn(d []byte) (n int, err error) {
	if err := r.enc.Encode(RecData{
		Time: Nanotime(),
		Data: append([]byte{MsgInput}, d...),
	}); err != nil {
		return 0, err
	}
	return len(d), nil
}
func (r *Recorder) WriteOut(d []byte) (n int, err error) {
	if err := r.enc.Encode(RecData{
		Time: Nanotime(),
		Data: append([]byte{MsgOutput}, d...),
	}); err != nil {
		return 0, err
	}
	return len(d), nil
}
func (r *Recorder) WriteErrOut(d []byte) (n int, err error) {
	if err := r.enc.Encode(RecData{
		Time: Nanotime(),
		Data: append([]byte{MsgErrOutput}, d...),
	}); err != nil {
		return 0, err
	}
	return len(d), nil
}

/*
func RecordRun(cmd *exec.Cmd, fileName string, input bool) error {
	cli := &Client{
		IOStreams: IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: nil,
		},
		Stdin: true,
		TTY:   true,
	}

	t := cli.Setup()
	cli.SizeQueue = t.MonitorSize(t.GetSize()).Ch()

	return t.Safe(func() error {
		cmd.Env = append(os.Environ(), "REC=[REC]")

		pty, err := NewPty()
		defer pty.Close()

		cmd.Stdout = pty.Tty
		cmd.Stdin = pty.Tty
		cmd.Stderr = pty.Tty
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Setsid = true
		cmd.SysProcAttr.Setctty = true
		cmd.SysProcAttr.Ctty = int(pty.Tty.Fd())

		fdx, err := NewFdx(&cli.IOStreams, pty.Pty, RshBuffSize)
		if err != nil {
			return err
		}

		recorder, err := NewRecorder(fileName)
		if err != nil {
			return err
		}
		defer recorder.Close()

		// tty <- cmd
		fdx.RxFilter(func(b []byte) ([]byte, error) {
			recorder.Write(append([]byte{MsgOutput}, b...))
			return b, nil
		})

		// tty -> cmd
		fdx.TxFilter(func(b []byte) ([]byte, error) {
			if input {
				recorder.Write(append([]byte{MsgInput}, b...))
			}
			return b, nil
		})

		go func() {
			for {
				ws, ok := <-cli.SizeQueue
				if !ok {
					return
				}
				pty.Resize(ws.Width, ws.Height)
			}
		}()

		go fdx.Run()

		return cmd.Run()
	})

}
*/
