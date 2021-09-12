// Copyright 2021 yubo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//from github.com/yubo/gotty/rec
package stream

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/yubo/golib/util/clock"
	"github.com/yubo/golib/util/term"
	"k8s.io/klog/v2"
)

const (
	SampleTime = time.Second / 100 // 100Hz
)

var (
	MagicCode = []byte{0xff, 0xf1, 0xf2, 0xf3}
)

type PlayerStreams struct {
	Stdin  io.ReadWriter // read ctl
	Stdout io.Writer
	Stderr io.Writer
}

type CtlMsg struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type Player struct {
	filename string
	ctx      context.Context
	cancel   context.CancelFunc
	speed    int64
	repeat   bool
	pause    int64
	maxWait  time.Duration
	outCh    chan []byte
	errOutCh chan []byte
}

func (p *Player) Streams() PtyStreams {
	return PtyStreams{
		Stdin:  WriteFunc(p.writeIn),
		Stdout: ReadFunc(p.readOut),
		Stderr: ReadFunc(p.readErrOut),
	}
}

// canot support resize method
func (p *Player) IsTerminal() bool {
	return false
}

func (p *Player) Resize(*term.TerminalSize) error {
	return nil
}

func NewPlayer(fileName string, speed int64, repeat bool, wait time.Duration) (*Player, error) {
	p := &Player{
		filename: fileName,
		speed:    speed,
		repeat:   repeat,
		maxWait:  wait,
		outCh:    make(chan []byte, 10),
		errOutCh: make(chan []byte, 10),
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())

	if err := p.run(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Player) readOut(b []byte) (int, error) {
	data, ok := <-p.outCh
	if !ok {
		return 0, io.EOF
	}

	return copyBytes(b, data)
}

func (p *Player) readErrOut(b []byte) (int, error) {
	data, ok := <-p.errOutCh
	if !ok {
		return 0, io.EOF
	}

	return copyBytes(b, data)
}

func (p *Player) writeIn(b []byte) (n int, err error) {
	n = len(b)

	if len(b) != 1 {
		klog.Infof("player Write %d %v", len(b), b)
		return
	}

	switch c := b[0]; c {
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		debug().Infof("spped %d", c-'0')
		atomic.StoreInt64(&p.speed, int64(c-'0'))
	case 3, 4, 'q':
		return 0, io.EOF
	case ' ', 'p':
		pause := atomic.AddInt64(&p.pause, 1)&0x01 == 0
		debug().InfoS("player", "pause", pause)
	}

	return
}

func (p *Player) Close() error {
	p.cancel()
	return nil
}
func readFrame(decoder *gob.Decoder) (*RecData, error) {
	data := &RecData{}
	if err := decoder.Decode(data); err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Player) run() error {
	fd, err := os.OpenFile(p.filename, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	decoder := gob.NewDecoder(fd)

	frame, err := readFrame(decoder)
	if err != nil {
		return err
	}

	startTime := time.Unix(0, frame.Time)
	clock := clock.NewFakeClock(startTime)

	go func() {
		tick := time.NewTicker(SampleTime)
		defer tick.Stop()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-tick.C:
				if atomic.LoadInt64(&p.pause)&0x01 == 0 {
					clock.Step(SampleTime * time.Duration(atomic.LoadInt64(&p.speed)))
				}
			}
		}
	}()

	go func() {
		defer fd.Close()
		p.sendMsg(frame, clock)

		for {
			if frame, err = readFrame(decoder); err != nil {
				if err == io.EOF && p.repeat {
					clock.SetTime(startTime)
					fd.Seek(0, 0)
					decoder = gob.NewDecoder(fd)
					continue
				}
				p.cancel()
				return
			}
			p.sendMsg(frame, clock)
		}
	}()

	return nil
}

func (p *Player) sendMsg(frame *RecData, clock *clock.FakeClock) {
	wait := time.Unix(0, frame.Time).Sub(clock.Now())
	if wait > p.maxWait {
		clock.Step(wait - p.maxWait)
		wait = p.maxWait
	}
	debug().Infof("wait %v", wait)
	<-clock.After(wait)

	msgType, data := frame.Data[0], frame.Data[1:]
	switch msgType {
	case MsgInput:
		debug().InfoS("player in", "len", len(data))
	case MsgOutput:
		debug().InfoS("player out", "len", len(data), "content", string(data))
		p.outCh <- data
	case MsgErrOutput:
		debug().InfoS("player errOut", "len", len(data))
		p.errOutCh <- data
	case MsgResize:
		var size term.TerminalSize
		err := json.Unmarshal(data, &size)
		if err != nil {
			debug().Infof("json unmarshal err %s", err)
			return
		}
		debug().InfoS("player resize", "width", size.Width, "height", size.Height)
	default:
		klog.Infof("unknow type(%d) data(%s)", msgType, string(data))
	}
}
