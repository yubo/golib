// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

const (
	MsgInput     byte = '0' + iota // User input typically from a keyboard
	MsgOutput                      // Normal output to the terminal
	MsgErrOutput                   // Normal output to the terminal
	MsgResize                      // Notify that the browser size has been changed
	MsgPing
	MsgCtl
	MsgAction // custom Action
)

type ReadFunc func(p []byte) (n int, err error)

func (f ReadFunc) Read(p []byte) (int, error) {
	return f(p)
}

type WriteFunc func(p []byte) (n int, err error)

func (f WriteFunc) Write(p []byte) (int, error) {
	return f(p)
}

type CloseFunc func() (err error)

func (f CloseFunc) Close() error {
	return f()
}
