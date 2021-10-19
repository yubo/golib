package api

import (
	"encoding/json"
	"time"

	"github.com/spf13/pflag"
)

// Duration is a wrapper around time.Duration which supports correct
// marshaling to YAML and JSON. In particular, it marshals into strings, which
// can be used as map keys in json.
type Duration struct {
	time.Duration `protobuf:"varint,1,opt,name=duration,casttype=time.Duration"`
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}

	pd, err := time.ParseDuration(str)
	if err != nil {
		return err
	}
	d.Duration = pd
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d Duration) String() string {
	return d.Duration.String()
}

func (d *Duration) Set(val string) error {
	v, err := time.ParseDuration(val)
	*d = Duration{v}
	return err
}

func (d *Duration) Type() string {
	return "duration"
}

func newDurationValue(val Duration, p *Duration) *Duration {
	*p = val
	return p
}

func (d *Duration) NewFlagSet(f *pflag.FlagSet) interface{} {
	return func(name string, value Duration, usage string) *Duration {
		p := new(Duration)
		f.VarP(newDurationValue(value, p), name, "", usage)
		return p
	}
}

func (d *Duration) NewFlagSetP(f *pflag.FlagSet) interface{} {
	return func(name, shorthand string, value Duration, usage string) *Duration {
		p := new(Duration)
		f.VarP(newDurationValue(value, p), name, shorthand, usage)
		return p
	}
}

func (d *Duration) New(val string) interface{} {
	p := new(Duration)
	p.Set(val)
	return *p
}
