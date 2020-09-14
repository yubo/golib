package sys

import (
	"github.com/yubo/golib/util"

	sys "github.com/yubo/golib/sys/api"
)

var (
	Metrics = map[string]sys.MetricsInterface{}
)

func (p *Module) StatsRegister(name string, s sys.MetricsInterface) error {
	if _, ok := Metrics[name]; ok {
		return util.ErrExist
	}
	Metrics[name] = s
	return nil
}
