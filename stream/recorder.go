// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"encoding/gob"
	"encoding/json"
	"io"

	"github.com/yubo/golib/util/term"
)

type RecorderStreams struct {
	Stdin  io.Writer
	Stdout io.Writer
	Stderr io.Writer
}

type RecData struct {
	Time int64
	Data []byte
}

type RecorderFactory func(out io.WriteCloser) (Recorder, error)

type Recorder interface {
	Close() error
	Streams() RecorderStreams
	Resize(size *term.TerminalSize) error
	Info(info []byte) error
}

var _ Recorder = &streamRecorder{}

type streamRecorder struct {
	out io.WriteCloser
	enc *gob.Encoder
}

func (p *streamRecorder) Streams() RecorderStreams {
	return RecorderStreams{
		Stdin:  WriteFunc(p.writeIn),
		Stdout: WriteFunc(p.writeOut),
		Stderr: WriteFunc(p.writeErrOut),
	}
}

func (p *streamRecorder) Close() error {
	return p.out.Close()
}

func NewRecorder(out io.WriteCloser) (Recorder, error) {
	return &streamRecorder{
		out: out,
		enc: gob.NewEncoder(out),
	}, nil
}

func (r *streamRecorder) Info(info []byte) error {
	_, err := r.write(MsgInfo, info)
	return err
}

func (r *streamRecorder) Resize(size *term.TerminalSize) error {
	d, err := json.Marshal(size)
	if err != nil {
		return err
	}

	_, err = r.write(MsgResize, d)
	return err
}

func (r *streamRecorder) writeIn(d []byte) (int, error) {
	return r.write(MsgInput, d)
}

func (r *streamRecorder) writeOut(d []byte) (int, error) {
	return r.write(MsgOutput, d)
}

func (r *streamRecorder) writeErrOut(d []byte) (int, error) {
	return r.write(MsgErrOutput, d)
}

func (r *streamRecorder) write(msgType byte, data []byte) (int, error) {
	if err := r.enc.Encode(RecData{
		Time: Nanotime(),
		Data: append([]byte{msgType}, data...),
	}); err != nil {
		return 0, err
	}

	return len(data), nil
}
