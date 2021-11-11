/*
 * Copyright 2016 yubo. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

package ping

import (
	"context"
	"fmt"
	"testing"
)

func TestPing(t *testing.T) {
	p := NewServer(context.Background())

	if err := p.Run(1000); err != nil {
		t.Error(err)
		return
	}

	ips := [][4]byte{
		{127, 0, 0, 1},
		{8, 8, 8, 8},
	}

	t1 := p.Go(ips, 2, 1, make(chan *Task, 1)).Done
	t2 := p.Go(ips, 1, 1, make(chan *Task, 1)).Done

	for i := 0; i < 2; i++ {
		select {
		case task := <-t1:
			fmt.Printf("task1 ret %v, err %v\n", task.Ret, task.Error)
		case task := <-t2:
			fmt.Printf("task2 ret %v, err %v\n", task.Ret, task.Error)
		}
	}

	p.Kill()
}
