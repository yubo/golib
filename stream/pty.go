// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"io"
	"os"
	"os/exec"

	"github.com/yubo/golib/term/creack/pty"
	"github.com/yubo/golib/term"
)

type Pty interface {
	Streams() PtyStreams
	IsTerminal() bool
	Resize(*term.TerminalSize) error
	Close() error
}

type PtyStreams struct {
	Stdin  io.Writer // /dev/ptmx
	Stdout io.Reader // /dev/ptmx
	Stderr io.Reader // /dev/ptmx
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
		Stdin:  p.pty,
		Stdout: p.pty,
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
