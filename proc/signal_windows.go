package proc

import (
	"os"
)

var shutdownSignals = []os.Signal{os.Interrupt}
var reloadSignals = []os.Signal{}
