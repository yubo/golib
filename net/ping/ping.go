/*
 * Copyright 2016 yubo. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

/* addr [4]byte use big endian */
package ping

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/yubo/golib/util/list"
	"k8s.io/klog/v2"
)

type task_entry struct {
	task   *Task
	s_list list.ListHead // for server.ips
	t_list list.ListHead // for task
	recv   *bool
}

type Task struct {
	sync.RWMutex               // for entry_list
	t_list       list.ListHead // server.task_list
	e_list       list.ListHead // Task_entry.t_list
	startTs      int64
	lastTs       int64
	Timeout      int64
	retry        int
	RetryLimit   int
	Ips          [][4]byte
	Ret          []bool
	Done         chan *Task
	Error        error
}

type Server struct {
	sync.RWMutex
	run    bool
	t_list list.ListHead // task list
	ips    map[[4]byte]*list.ListHead
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	errCh  chan error
	done   chan struct{}
}

var (
	ErrAcces      = errors.New("Permission denied (you must be root)")
	ErrNoRun      = errors.New("ping server is not running")
	ErrEmpty      = errors.New("empty list")
	DefaultServer = NewServer(context.Background())
)

func NewServer(ctx context.Context) *Server {
	p := &Server{
		errCh: make(chan error, 2),
		done:  make(chan struct{}),
		run:   false,
		ips:   make(map[[4]byte]*list.ListHead),
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.t_list.Init()
	return p
}

func Run(rate uint32) error {
	return DefaultServer.Run(rate)
}

func Kill() error {
	return DefaultServer.Kill()
}

func Wait() {
	DefaultServer.Wait()
}

func (p *Server) Wait() {
	<-p.done
}

func (p *Server) Run(rate uint32) error {
	p.Lock()
	defer p.Unlock()

	if os.Getuid() != 0 {
		return ErrAcces
	}

	if p.run {
		return errors.New("already running")
	}

	select {
	case <-p.done:
		return errors.New("already done")
	default:
	}

	p.run = true

	go p.tx_loop(rate)
	go p.rx_loop()

	go func() {
		select {
		case err := <-p.errCh:
			if err != nil {
				fmt.Printf("err %s", err)
			}
			p.cancel()
		case <-p.ctx.Done():
		}

		p.wg.Wait()

		p.Lock()
		p.run = false
		close(p.done)
		p.Unlock()
	}()

	return nil
}

func (p *Server) Kill() error {
	p.Lock()
	run := p.run
	p.Unlock()

	if !run {
		return ErrNoRun
	}

	p.cancel()

	p.Wait()
	return nil
}

func Go(ips [][4]byte, timeout, retry int, done chan *Task) *Task {
	return DefaultServer.Go(ips, timeout, retry, done)
}

func (p *Server) Go(ips [][4]byte, timeout, retry int, done chan *Task) *Task {
	var s_list *list.ListHead
	var ok bool

	t := &Task{
		Ips:        ips[:],
		Ret:        make([]bool, len(ips)),
		Timeout:    int64(timeout),
		RetryLimit: retry,
	}
	t.e_list.Init()

	if done == nil {
		done = make(chan *Task, 10) // buffered.
	} else {
		if cap(done) == 0 {
			log.Panic("ping: ping channel is unbuffered")
		}
	}
	t.Done = done

	if !p.run {
		t.Error = ErrNoRun
		t.Done <- t
		return t
	}

	p.Lock()
	t.Lock()

	for i, ip := range t.Ips {
		if s_list, ok = p.ips[ip]; !ok {
			s_list = &list.ListHead{}
			s_list.Init()
			p.ips[ip] = s_list
		}

		te := task_entry{}
		te.task = t
		te.recv = &t.Ret[i]
		t.e_list.AddTail(&te.t_list)
		s_list.AddTail(&te.s_list)
	}
	p.t_list.AddTail(&t.t_list)
	klog.V(4).Infof("task[%p] add to tail", t)

	t.Unlock()
	p.Unlock()

	return t
}

func Call(ips [][4]byte, timeout int, retry int) ([]bool, error) {
	task := <-Go(ips, timeout, retry, make(chan *Task, 1)).Done
	return task.Ret, task.Error
}

func list_to_task(list *list.ListHead) *Task {
	return (*Task)(unsafe.Pointer((uintptr(unsafe.Pointer(list)) -
		unsafe.Offsetof(((*Task)(nil)).t_list))))
}

/* for task_entry */
func list_to_entry(list *list.ListHead) *task_entry {
	return (*task_entry)(unsafe.Pointer((uintptr(unsafe.Pointer(list)) -
		unsafe.Offsetof(((*task_entry)(nil)).t_list))))
}

func list_to_entry_s(list *list.ListHead) *task_entry {
	return (*task_entry)(unsafe.Pointer((uintptr(unsafe.Pointer(list)) -
		unsafe.Offsetof(((*task_entry)(nil)).s_list))))
}

func (p *Task) next(server *Server) (*Task, error) {
	now := time.Now().Unix()

	if p != nil {
		p.Lock()
		p.retry++
		p.lastTs = now
		p.t_list.Del()
		server.Lock()
		server.t_list.AddTail(&p.t_list)
		server.Unlock()
		p.Unlock()
		klog.V(4).Infof("task[%p] move to tail", p)
	}

	for n, _n := server.t_list.Next, server.t_list.Next.Next; n != &server.t_list; n, _n = _n, _n.Next {
		task := list_to_task(n)
		//klog.V(5).Infof("task[%p] check", task)

		if task.retry < task.RetryLimit {
			if now > task.startTs+task.Timeout {
				task.startTs = now
				klog.V(4).Infof("task[%p] get", task)
				return task, nil
			}
			klog.V(4).Infof("task[%p] skip wait until %d now %d",
				task, task.startTs+task.Timeout, now)
			continue
		}

		if now > task.lastTs+task.Timeout {
			/* return task -> done */
			server.Lock()
			task.Lock()

			task.t_list.Del()

			for pos, _pos := task.e_list.Next, task.e_list.Next.Next; pos != &task.e_list; pos, _pos = _pos, _pos.Next {

				/*
				 * Ugly Hack !
				 * rx loop may use this ptr, so ...
				 * remove this entry from e_list
				 * and keep next,prev ptr
				 */
				pos.Next.Prev = pos.Prev
				pos.Prev.Next = pos.Next

				/* TODO: check server.ips[ip], remove empty node */
				/* remove task_entry from server.ips list */
				list_to_entry(pos).s_list.Del()

			}
			task.Unlock()
			server.Unlock()
			task.Done <- task
			klog.V(4).Infof("task[%p] done", task)
		}
	}
	return nil, ErrEmpty
}

func (p *Server) tx_loop(rate uint32) {
	var sa syscall.SockaddrInet4
	var fd, idx int
	var cur *Task
	var err error
	var ip [4]byte

	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()

	if fd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW); err != nil {
		p.errCh <- err
		return
	}
	defer syscall.Close(fd)

	p.wg.Add(1)
	defer p.wg.Add(-1)

	for {
		select {
		case <-p.ctx.Done():
			klog.V(3).Infof("tx routine exit")
			return
		case <-ticker.C:
			if cur == nil {
				if cur, err = cur.next(p); err != nil {
					klog.V(5).Infof("tx loop: %s ", err)
					continue
				}
				idx = 0
			}

			for idx < len(cur.Ips) {
				if !cur.Ret[idx] {
					break
				}
				idx++
			}
			if idx >= len(cur.Ips) {
				if cur, err = cur.next(p); err != nil {
					klog.V(4).Infof("tx loop: %s ", err)
					continue
				}
				idx = 0
			}
			ip = cur.Ips[idx]

			sa.Addr = ip
			klog.V(4).Infof("syscall.Sendto %d.%d.%d.%d",
				uint8(ip[0]), uint8(ip[1]),
				uint8(ip[2]), uint8(ip[3]))
			if err = syscall.Sendto(
				fd,
				pkt([4]byte(ip)),
				0,
				&sa,
			); err != nil {
				klog.V(2).Infof("syscall.Sendto error: %s ", err)

			}
			idx++
		}
	}
}

func (p *Server) rx_loop() {
	var buf []byte = make([]byte, 1500)
	var numRead int
	var err error
	var src [4]byte
	var fd int
	var fp *os.File

	if fd, err = syscall.Socket(syscall.AF_INET,
		syscall.SOCK_RAW, syscall.IPPROTO_ICMP); err != nil {
		p.errCh <- err
	}

	fp = os.NewFile(uintptr(fd), fmt.Sprintf("ping/server/rx_fp"))
	defer fp.Close()

	p.wg.Add(1)

	// ugly hack, fp.Read unsupported SetDeadline
	go func() {
		<-p.ctx.Done()
		p.wg.Add(-1)
	}()

	for {
		select {
		case <-p.ctx.Done():
			klog.V(3).Infof("rx routine exit")
			return
		default:
			/* recv icmp */
			if numRead, err = fp.Read(buf); err != nil {
				klog.Error(err)
			}
			if buf[0x14] == 0x00 {
				src[0], src[1], src[2], src[3] =
					buf[0x0c], buf[0x0d], buf[0x0e], buf[0x0f]
				klog.V(3).Infof("rx_loop read[%d] % X\n",
					numRead, buf[:numRead])
				klog.V(3).Infof("%d.%d.%d.%d",
					src[0], src[1], src[2], src[3])
				if h, ok := p.ips[src]; ok {
					for p := h.Next; p != h; p = p.Next {
						*(list_to_entry_s(p).recv) = true
					}
				}
			}
		}
	}
}
