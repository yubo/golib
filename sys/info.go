package sys

import (
	"github.com/yubo/golib/openapi/api"
	"github.com/yubo/golib/util"
)

var (
	Metrics = map[string]api.MetricsInterface{}
)

func (p *Module) StatsRegister(name string, s api.MetricsInterface) error {
	if _, ok := Metrics[name]; ok {
		return util.ErrExist
	}
	Metrics[name] = s
	return nil
}
