// +build !linux,!darwin

package util

import (
	"errors"
	"syscall"
)

var (
	ErrUnsupported = errors.New("unsupported")
)

func Kill(pid int, sig syscall.Signal) (err error) {
	return ErrUnsupported
}
