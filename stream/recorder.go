// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package stream

import (
	"encoding/gob"
	"encoding/json"
	"io"
	"os"

	"github.com/yubo/golib/util/term"
)

type RecorderStreams struct {
	In     io.Writer
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

func (r *Recorder) WriteIn(d []byte) (int, error) {
	if err := r.put(MsgInput, d); err != nil {
		return 0, err
	}

	return len(d), nil
}

func (r *Recorder) WriteOut(d []byte) (int, error) {
	if err := r.put(MsgOutput, d); err != nil {
		return 0, err
	}

	return len(d), nil

}

func (r *Recorder) WriteErrOut(d []byte) (int, error) {
	if err := r.put(MsgErrOutput, d); err != nil {
		return 0, err
	}

	return len(d), nil
}

func (r *Recorder) Resize(size *term.TerminalSize) error {
	d, err := json.Marshal(size)
	if err != nil {
		return err
	}

	return r.put(MsgResize, d)
}

func (r *Recorder) put(msgType byte, data []byte) error {
	return r.enc.Encode(RecData{
		Time: Nanotime(),
		Data: append([]byte{msgType}, data...),
	})
}
