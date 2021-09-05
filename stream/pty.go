package stream

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/yubo/golib/util/term"
)

type Pty interface {
	Streams() PtyStreams
	IsTerminal() bool
	Resize(*term.TerminalSize) error
}

type PtyStreams struct {
	In     io.Writer // /dev/ptmx
	Out    io.Reader // /dev/ptmx
	ErrOut io.Reader // /dev/ptmx
}

type CmdPty struct {
	pty *os.File
}

func NewCmdPty(cmd *exec.Cmd) (*CmdPty, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	return &CmdPty{
		pty: ptmx,
	}, nil
}

func (p *CmdPty) Streams() PtyStreams {
	return PtyStreams{
		In:  p.pty,
		Out: p.pty,
	}
}

func (p *CmdPty) IsTerminal() bool {
	return term.IsTerminal(p.pty)
}

func (p *CmdPty) Resize(size *term.TerminalSize) error {
	if size.Height > 0 && size.Width > 0 {
		return pty.Setsize(p.pty, &pty.Winsize{Rows: size.Height, Cols: size.Width})
	}
	return nil
}

func (p *CmdPty) Close() error {
	return p.pty.Close()
}
