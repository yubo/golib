// +build linux darwin

//from github.com/yubo/gotty/rec
package stream

import (
	"encoding/gob"
	"encoding/json"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/term"
	"k8s.io/klog/v2"
)

const (
	SampleTime = 100 * time.Millisecond // 10Hz
)

type PlayerStreams struct {
	In     io.ReadWriter // read ctl
	Out    io.Writer
	ErrOut io.Writer
}

type CtlMsg struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type Player struct {
	FileName  string
	f         *os.File
	dec       *gob.Decoder
	size      term.TerminalSize
	d         RecData
	pending   bool
	speed     int64
	repeat    bool
	sync      bool
	fileStart int64
	done      chan struct{}
	ctlMsgCh  chan *CtlMsg

	pause    int64
	playTime int64
	start    int64
	offset   int64
	maxWait  int64
}

func (p *Player) Streams() PtyStreams {
	return PtyStreams{
		In:     WriteFunc(p.Write),
		Out:    ReadFunc(p.Read),
		ErrOut: ReadFunc(p.Read),
	}
}

func (p *Player) IsTerminal() bool {
	return true
}

func (p *Player) Resize(*term.TerminalSize) error {
	return nil
}

func (p *Player) GetSize() error {
	return nil
}

// slave(tty) -> master(exec)
// in reader -> in writer
// out writer <- out reader
// errout writer <- errout reader

func NewPlayer(fileName string, speed int64, repeat bool, wait time.Duration) (*Player, error) {
	var err error

	p := &Player{FileName: fileName,
		speed:    speed,
		repeat:   repeat,
		sync:     false,
		maxWait:  int64(wait),
		done:     make(chan struct{}),
		ctlMsgCh: make(chan *CtlMsg, 8),
	}

	if p.f, err = os.OpenFile(fileName, os.O_RDONLY, 0); err != nil {
		return nil, err
	}
	p.dec = gob.NewDecoder(p.f)

	p.run()

	return p, nil
}

func (p *Player) Read(d []byte) (n int, err error) {
	for {
		if !p.pending {
			if err = p.dec.Decode(&p.d); err != nil {
				if p.repeat && err == io.EOF {
					p.start = Nanotime()
					p.offset = 0
					atomic.StoreInt64(&p.playTime, 0)
					p.f.Seek(0, 0)
					p.dec = gob.NewDecoder(p.f)
					continue
				} else {
					return 0, err
				}
			}
			p.pending = true
		}

		typ, data := p.d.Data[0], p.d.Data[1:]

		switch typ {
		case MsgResize:
			err = json.Unmarshal(data, &p.size)
			if err != nil {
				klog.V(6).Infof("json unmarshal err %s", err)
				continue
			}
			p.pending = false
			continue
		case MsgInput:
			klog.V(6).Infof("input msg %s", data)
			p.pending = false
			continue
		case MsgOutput:
			wait := p.d.Time - p.fileStart - p.offset -
				atomic.LoadInt64(&p.playTime)*p.speed
			if wait > p.maxWait {
				p.offset += wait - p.maxWait
			}
			for {
				wait = p.d.Time - p.fileStart - p.offset -
					atomic.LoadInt64(&p.playTime)*p.speed
				if wait <= 0 {
					break
				}

				// check chan before sleep
				select {
				case msg := <-p.ctlMsgCh:
					b := append([]byte{MsgCtl}, []byte(util.JsonStr(msg))...)
					klog.Infof("%s", string(b))
					n = copy(d, b[:])
					return
				default:
				}

				time.Sleep(time.Duration(MaxInt64(int64(SampleTime), wait)))
			}

			// synchronization time axis
			// expect wait == 0 when sending msg
			if p.sync {
				p.offset = p.d.Time - p.fileStart - atomic.LoadInt64(&p.playTime)*p.speed
			}
			n = copy(d, data)
			p.pending = false

			return
		default:
			klog.Infof("unknow type(%d) context(%s)", typ, string(data))
			continue
		}
	}
}

func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (p *Player) run() {
	p.fileStart = p.d.Time
	p.start = Nanotime()
	go func() {
		tick := time.NewTicker(SampleTime)
		defer tick.Stop()

		lastTime := p.start
		now := p.start

		for {
			select {
			case <-p.done:
				return
			case t := <-tick.C:
				now = t.UnixNano()
				if atomic.LoadInt64(&p.pause)&0x01 == 0 {
					atomic.AddInt64(&p.playTime, now-lastTime)
				}
				lastTime = now
			}
		}
	}()
}

func (p *Player) Write(b []byte) (n int, err error) {
	n = len(b)

	if len(b) != 1 {
		klog.Infof("player Write %d %v", len(b), b)
		return
	}

	switch b[0] {
	case 3, 4, 'q':
		return 0, io.EOF
	case ' ', 'p':
		if atomic.AddInt64(&p.pause, 1)&0x01 == 0 {
			p.ctlMsgCh <- &CtlMsg{Type: "unpause"}
		} else {
			p.ctlMsgCh <- &CtlMsg{Type: "pause"}
		}
	}

	return
}

func (p *Player) Close() error {
	close(p.done)
	return p.f.Close()
}
