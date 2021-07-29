// +build !linux,!darwin

package util

import (
	"syscall"
)

func Kill(pid int, sig syscall.Signal) (err error) {
	return ErrUnsupported
}
