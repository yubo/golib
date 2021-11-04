package configer

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/api"
)

func TestRegisterConfigFields(t *testing.T) {
	var cases = []struct {
		fn   func(string) interface{}
		data [][4]string
	}{{
		func(val string) interface{} {
			type Foo struct {
				Duration api.Duration `json:"duration" flag:"timeout" default:"5s"`
			}
			if val == "" {
				return &Foo{}
			}
			v, _ := strconv.Atoi(val)
			return &Foo{api.Duration{
				Duration: time.Duration(v) * time.Second,
			}}
		}, [][4]string{
			// name, flag, yaml, expected
			{"duration default", "", "", "5"},
			{"duration flag", "timeout=10s", "", "10"},
			{"duration file", "", "duration: 20s", "20"},
		},
	}, {
		func(val string) interface{} {
			type Foo struct {
				IP net.IP `json:"ip" flag:"ip" default:"1.1.1.1"`
			}
			if val == "" {
				return &Foo{}
			}
			return &Foo{IP: net.ParseIP(val)}
		}, [][4]string{
			// name, flag, yaml, expected
			{"ip default", "", "", "1.1.1.1"},
			{"ip flag", "ip=2.2.2.2", "", "2.2.2.2"},
			{"ip file", "", "ip: 3.3.3.3", "3.3.3.3"},
		},
	}}
	for _, c1 := range cases {
		fn := c1.fn
		for _, c := range c1.data {
			name, args, yaml, want, got := c[0], c[1], c[2], fn(c[3]), fn("")

			t.Run(name, func(t *testing.T) {
				factory := NewFactory()

				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				factory.SetOptions(true, false, 5, fs)

				err := factory.RegisterConfigFields(fs, "", got)
				assert.NoError(t, err)

				setFlags(fs, args)

				cfg, err := factory.NewConfiger(WithDefaultYaml("", yaml))
				assert.NoError(t, err)

				cfg.Read("", got)
				assert.Equal(t, want, got)
			})
		}
	}
}
