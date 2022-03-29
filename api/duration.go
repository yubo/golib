package api

import (
	"encoding/json"
	"time"
)

func NewDuration(val string) Duration {
	d := Duration{}
	if err := d.Set(val); err != nil {
		panic(err)
	}

	return d
}

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
		// try int
		var pd int64
		if err := json.Unmarshal(b, &pd); err != nil {
			return err
		}
		d.Duration = time.Duration(pd) * time.Second
		return nil
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
	s := d.Duration.String()

	// configer isZero checker, "0s" is not supported
	if s == "0s" {
		return json.Marshal(0)
	}
	return json.Marshal(s)
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

// IsZero returns true if the value is nil or time is zero.
func (d *Duration) IsZero() bool {
	if d == nil {
		return true
	}
	return d.Duration == 0
}
