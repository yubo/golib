package stream

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/yubo/golib/util/term"
)

// server

type Exec struct {
	pty *os.File // /dev/ptmx
}

func NewExec(cmd *exec.Cmd) (*Exec, error) {
	pty, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	return &Exec{
		pty: pty,
	}, nil
}

func (p *Exec) PtyStreams() PtyStreams {
	return PtyStreams{
		In:     p.pty,
		Out:    p.pty,
		ErrOut: p.pty,
	}
}

func (p *Exec) IsTerminal() bool {
	return term.IsTerminal(p.pty)
}

func (p *Exec) Resize(height, width uint16) error {
	return pty.Setsize(p.pty, &pty.Winsize{Rows: height, Cols: width})
}

func (p *Exec) Getsize() (int, int, error) {
	return pty.Getsize(p.pty)
}
