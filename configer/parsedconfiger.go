package configer

import (
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/yaml"
	"k8s.io/klog/v2"
)

type ParsedConfiger interface {
	// ValueFiles: return files list, which set by --set-file
	ValueFiles() []string
	// Envs: return envs list from fields, used with flags
	Envs() []string
	// Envs: return flags list
	Flags() []string

	//Set(path string, v interface{}) error
	GetConfiger(path string) ParsedConfiger
	GetRaw(path string) interface{}
	GetString(path string) string
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

func (p *parsedConfiger) GetConfiger(path string) ParsedConfiger {
	if p == nil || !p.parsed {
		return nil
	}

	data, _ := p.GetRaw(path).(map[string]interface{})

	// noneed deepCopy
	out := new(parsedConfiger)
	*out = *p

	out.path = append(clonePath(p.path), parsePath(path)...)
	out.data = data

	return out
}

func (p *parsedConfiger) GetRaw(path string) interface{} {
	if path == "" {
		return Values(p.data)
	}

	v, err := Values(p.data).PathValue(path)
	if err != nil {
		dlog("get pathValue err, ignored", "path", path, "v", v, "err", err)
		return nil
	}
	return v
}

func (p *parsedConfiger) GetString(path string) string {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return ""
	}

	return util.ToString(v)
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

	if v := p.GetRaw(path); v != nil {
		data, err := yaml.Marshal(v)
		dlog("marshal", "v", v, "data", string(data), "err", err)
		if err != nil {
			return err
		}

		err = yaml.Unmarshal(data, into)
		if err != nil {
			dlog("unmarshal", "data", string(data), "err", err)
			return err
		}
	}

	if v, ok := into.(validator); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}

	if DEBUG {
		b, _ := yaml.Marshal(into)
		klog.Infof("Read \n[%s]\n%s", path, string(b))
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
