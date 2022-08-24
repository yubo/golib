//go:build linux || darwin
// +build linux darwin

// convert stream rec format to github.com/asciinema/asciinema format
package convert

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/yubo/golib/stream"
	"github.com/yubo/golib/term"
)

type Duration float64

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`%.6f`, d)), nil
}

type Asciicast struct {
	Version   int      `json:"version"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Timestamp int64    `json:"timestamp"`
	Duration  Duration `json:"duration,omitempty"`
	Command   string   `json:"command,omitempty"`
	Title     string   `json:"title,omitempty"`
	Env       *Env     `json:"env,omitempty"`
	Stdout    []Frame  `json:"stdout,omitempty"`
}

type Env struct {
	Term  string `json:"TERM"`
	Shell string `json:"SHELL"`
}

type Stream struct {
	Frames        []Frame
	maxWait       int64
	lastWriteTime int64
	elapsedTime   int64
	init          bool
}

func (s *Stream) Write(time int64, p []byte) (int, error) {
	if !s.init {
		s.lastWriteTime = time
		s.init = true
	}
	frame := Frame{}
	frame.Delay = s.incrementElapsedTime(time)
	frame.Data = make([]byte, len(p))
	copy(frame.Data, p)
	s.Frames = append(s.Frames, frame)

	return len(p), nil
}

func (s *Stream) Close() {
	s.incrementElapsedTime(s.lastWriteTime)
}

func (s *Stream) incrementElapsedTime(time int64) float64 {
	d := time - s.lastWriteTime

	if s.maxWait > 0 && d > s.maxWait {
		d = s.maxWait
	}

	s.elapsedTime += d
	s.lastWriteTime = time

	return nano2sec(s.elapsedTime)
}

func Save(asciicast *Asciicast, frames []Frame, path string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644); err != nil {
		return
	}

	defer func() {
		if e := f.Close(); e != nil && err == nil {
			err = e
		}
	}()

	var bytes []byte
	if bytes, err = json.Marshal(asciicast); err != nil {
		return
	}

	if _, err = f.Write(bytes); err != nil {
		return err
	}
	f.Write([]byte("\n"))

	for _, v := range frames {
		if bytes, err = json.Marshal(v); err != nil {
			return
		}
		if _, err = f.Write(bytes); err != nil {
			return err
		}
		f.Write([]byte("\n"))
	}

	return nil
}

func Convert(src, dst string, wait time.Duration) error {
	fp, err := os.OpenFile(src, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer fp.Close()

	dec := gob.NewDecoder(fp)
	s := &Stream{maxWait: int64(wait)}
	metadata := &Asciicast{Version: 2, Env: &Env{}, Timestamp: time.Now().Unix()}
	var frame stream.RecData

	for {
		if err = dec.Decode(&frame); err != nil {
			if err == io.EOF {
				break
			}
		}

		msgType, data := frame.Data[0], frame.Data[1:]
		switch msgType {
		case stream.MsgResize:
			var size term.TerminalSize
			err = json.Unmarshal(data, &size)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Malformed remote command")
			} else {
				if size.Height > 0 && size.Width > 0 {
					metadata.Height = int(size.Height)
					metadata.Width = int(size.Width)
				}
			}
		case stream.MsgOutput, stream.MsgErrOutput:
			s.Write(frame.Time, data)
		case stream.MsgInput:
		default:
			fmt.Fprintf(os.Stderr, "unknow type(%d) context(%s)", msgType, string(data))
		}

	}
	//metadata.Stdout = s.Frames
	metadata.Duration = Duration(nano2sec(s.elapsedTime))
	return Save(metadata, s.Frames, dst)
}

func nano2sec(d int64) float64 {
	sec := d / 1000000000
	nsec := d % 1000000000
	return float64(sec) + float64(nsec)*1e-9
}
