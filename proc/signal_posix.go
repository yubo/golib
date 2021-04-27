// +build !windows

package proc

import (
	"os"
	"syscall"
)

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
var reloadSignals = []os.Signal{}
