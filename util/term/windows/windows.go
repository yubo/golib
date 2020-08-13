// These files implement ANSI-aware input and output streams for use by the Docker Windows client.
// When asked for the set of standard streams (e.g., stdin, stdout, stderr), the code will create
// and return pseudo-streams that convert ANSI sequences to / from Windows Console API calls.

package windowsconsole

import (
	"io/ioutil"
	"sync"

	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger
var initOnce sync.Once

func initLogger() {
	initOnce.Do(func() {
		logFile := ioutil.Discard

		logger = &logrus.Logger{
			Out:       logFile,
			Formatter: new(logrus.TextFormatter),
			Level:     logrus.DebugLevel,
		}
	})
}
