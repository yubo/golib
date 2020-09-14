package api

import (
	"github.com/yubo/golib/util"
)

type Version struct {
	Version   string `json:"version,omitempty"`
	Release   string `json:"release,omitempty"`
	Git       string `json:"git,omitempty"`
	Go        string `json:"go,omitempty"`
	Os        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Builder   string `json:"builder,omitempty"`
	BuildTime int64  `json:"buildTime,omitempty" out:",date"`
}

func (p Version) String() string {
	return util.Prettify(p)
}
