package configer

import (
	"fmt"

	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/errors"
	"github.com/yubo/golib/util/yaml"
)

type ParsedConfiger interface {
	// ValueFiles: return files list, which set by --set-file
	ValueFiles() []string
	// Envs: return envs list from fields, used with flags
	Envs() []string
	// Envs: return flags list
	Flags() []string

	//Set(path string, v interface{}) error
	GetConfiger(path string) (ParsedConfiger, error)
	GetRaw(path string) (interface{}, error)
	MustGetRaw(path string) interface{}
	GetString(path string) (string, error)
	GetBool(path string) (bool, error)
	GetBoolDef(path string, def bool) bool
	GetFloat64(path string) (float64, error)
	GetFloat64Def(path string, def float64) float64
	GetInt64(path string) (int64, error)
	GetInt64Def(path string, def int64) int64
	GetInt(path string) (int, error)
	GetIntDef(path string, def int) int
	IsSet(path string) bool
	Read(path string, into interface{}) error
	String() string
}

var _ ParsedConfiger = new(parsedConfiger)

type parsedConfiger struct {
	*configer
}

// Set deprecated
//func (p *parsedConfiger) Set(path string, v interface{}) error {
//	if path == "" {
//		b, err := yaml.Marshal(v)
//		if err != nil {
//			return err
//		}
//		return yaml.Unmarshal(b, &p.data)
//	}
//
//	ps := strings.Split(path, ".")
//	src := map[string]interface{}{ps[len(ps)-1]: v}
//
//	for i := len(ps) - 2; i >= 0; i-- {
//		src = map[string]interface{}{ps[i]: src}
//	}
//
//	p.data = mergeValues(p.data, src)
//
//	return nil
//}

func (p *parsedConfiger) GetConfiger(path string) (ParsedConfiger, error) {
	if p == nil || !p.parsed {
		return nil, errors.New("invalid configer")
	}

	raw, err := p.GetRaw(path)
	if err != nil {
		return nil, err
	}

	// noneed deepCopy
	out := &parsedConfiger{&configer{}}
	*out.configer = *p.configer

	out.path = append(clonePath(p.path), parsePath(path)...)
	out.data = raw.(map[string]interface{})

	return out, nil
}

func (p *parsedConfiger) MustGetRaw(path string) interface{} {
	data, err := p.GetRaw(path)
	if err != nil {
		panic(err)
	}
	return data
}

func (p *parsedConfiger) GetRaw(path string) (interface{}, error) {
	if path == "" {
		return Values(p.data), nil
	}

	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (p *parsedConfiger) GetString(path string) (string, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return "", nil
	}

	return util.ToString(v), nil
}

func (p *parsedConfiger) GetBool(path string) (bool, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return false, err
	}

	return util.ToBool(v), nil
}

func (p *parsedConfiger) GetBoolDef(path string, def bool) bool {
	v, err := p.GetBool(path)
	if err != nil {
		return def
	}
	return v
}

func (p *parsedConfiger) GetFloat64(path string) (float64, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return 0, err
	}

	return util.ToFloat64(v), nil
}

func (p *parsedConfiger) GetFloat64Def(path string, def float64) float64 {
	v, err := p.GetFloat64(path)
	if err != nil {
		return def
	}

	return v
}

func (p *parsedConfiger) GetInt64(path string) (int64, error) {
	v, err := p.GetFloat64(path)
	if err != nil {
		return 0, err
	}

	return util.ToInt64(v), nil
}

func (p *parsedConfiger) GetInt64Def(path string, def int64) int64 {
	v, err := p.GetInt64(path)
	if err != nil {
		return def
	}
	return v
}

func (p *parsedConfiger) GetInt(path string) (int, error) {
	v, err := p.GetFloat64(path)
	if err != nil {
		return 0, err
	}

	return util.ToInt(v), nil
}

func (p *parsedConfiger) GetIntDef(path string, def int) int {
	v, err := p.GetInt(path)
	if err != nil {
		return def
	}
	return v
}

func (p *parsedConfiger) IsSet(path string) bool {
	_, err := Values(p.data).PathValue(path)
	return err == nil
}

func (p *parsedConfiger) Read(path string, into interface{}) error {
	if into == nil {
		return nil
	}

	v, err := p.GetRaw(path)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(v)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%#v", v))
	}

	err = yaml.Unmarshal(data, into)
	if err != nil {
		return errors.Wrap(err, string(data))
	}

	if v, ok := into.(validator); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (p *parsedConfiger) String() string {
	buf, err := yaml.Marshal(p.data)
	if err != nil {
		return err.Error()
	}
	return string(buf)
}

func (p *parsedConfiger) Document() string {
	buf, err := yaml.Marshal(p.data)
	if err != nil {
		return err.Error()
	}
	return string(buf)
}
