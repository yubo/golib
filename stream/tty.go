// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"io"

	"github.com/yubo/golib/util/term"
)

// TtyStreams provides the standard names for iostreams.  This is useful for embedding and for unit testing.
// Inconsistent and different names make it hard to read and review code
type TtyStreams struct {
	In     io.Reader // os.Stdin
	Out    io.Writer // os.Stdout
	ErrOut io.Writer // os.Stderr
}

type Tty interface {
	Streams() TtyStreams
	IsTerminal() bool
	GetSize() *term.TerminalSize
	MonitorSize(...*term.TerminalSize) term.TerminalSizeQueue
	Bind(pty Pty) <-chan error
}
