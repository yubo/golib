// the configer is not thread safe,
// make sure not use it after call process.Start()
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/yubo/golib/proc/strvals"
	"github.com/yubo/golib/util/template"
	yaml2 "gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

type Configer struct {
	*options

	data       map[string]interface{}
	configFile string
}

func newConfiger(yml []byte) (*Configer, error) {
	var data map[string]interface{}
	err := yaml.Unmarshal(yml, &data)
	if err != nil {
		return nil, err
	}
	return ToConfiger(data), nil
}

func ToConfiger(in interface{}) *Configer {
	if data, ok := in.(map[string]interface{}); ok {
		return &Configer{data: data}
	}
	return &Configer{data: map[string]interface{}{}}
}

func NewConfiger(configFile string, opts_ ...Option) (configer *Configer, err error) {
	opts := &options{}
	for _, opt := range opts_ {
		opt.apply(opts)
	}

	if configFile, err = filepath.Abs(configFile); err != nil {
		return nil, err
	}

	opts.valueFiles = append([]string{configFile}, opts.valueFiles...)

	if err = opts.Validate(); err != nil {
		return nil, err
	}

	configer = &Configer{
		data:       map[string]interface{}{},
		options:    opts,
		configFile: configFile,
	}

	return
}

func (p *Configer) ConfigFilePath() string {
	return p.configFile
}

func (p *Configer) GetConfiger(path string) *Configer {
	return ToConfiger(p.GetRaw(path))
}

// base < config < valueFile < value
func (p *Configer) Prepare() error {
	base := map[string]interface{}{}

	// base
	if len(p.base) > 0 {
		if err := yaml.Unmarshal(p.base, &base); err != nil {
			return err
		}
	}

	base, err := yaml2Values(base, p.base)
	if err != nil {
		return err
	}

	for path, b := range p.bases {
		if base, err = yaml2ValuesWithPath(base, []byte(b), path); err != nil {
			return err
		}
	}

	// configFile & valueFile
	for _, filePath := range p.valueFiles {
		currentMap := map[string]interface{}{}

		bytes, err := template.ParseTemplateFile(nil, filePath)
		if err != nil {
			return err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return fmt.Errorf("failed to parse %s: %s", filePath, err)
		}
		// Merge with the previous map
		base = mergeValues(base, currentMap)
	}

	// User specified a value via --set
	for _, value := range p.values {
		if err := strvals.ParseInto(value, base); err != nil {
			return fmt.Errorf("failed parsing --set data: %s", err)
		}
	}

	// User specified a value via --set-string
	for _, value := range p.stringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return fmt.Errorf("failed parsing --set-string data: %s", err)
		}
	}

	// User specified a value via --set-file
	for _, value := range p.fileValues {
		reader := func(rs []rune) (interface{}, error) {
			bytes, err := ioutil.ReadFile(string(rs))
			return string(bytes), err
		}
		if err := strvals.ParseIntoFile(value, base, reader); err != nil {
			return fmt.Errorf("failed parsing --set-file data: %s", err)
		}
	}

	p.data = base
	return nil
}

func (p *Configer) GetRaw(path string) interface{} {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		klog.V(3).Infof("get %s err %v", path, err)
		return nil
	}
	return v
}

func (p *Configer) GetStr(path string) string {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return ""
	}

	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func (p *Configer) GetBool(path string) (bool, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return false, err
	}

	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%v is not bool", path)
	}
	return b, nil
}

func (p *Configer) GetBoolDef(path string, def bool) bool {
	v, err := p.GetBool(path)
	if err != nil {
		return def
	}
	return v
}

func (p *Configer) GetFloat64(path string) (float64, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return 0, err
	}

	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("%v is not number", path)
	}

	return f, nil
}

func (p *Configer) GetFloat64Def(path string, def float64) float64 {
	v, err := p.GetFloat64(path)
	if err != nil {
		return def
	}

	return v
}

func (p *Configer) GetInt64(path string) (int64, error) {
	v, err := p.GetFloat64(path)
	if err != nil {
		return 0, err
	}

	return int64(v), nil
}

func (p *Configer) GetInt64Def(path string, def int64) int64 {
	v, err := p.GetInt64(path)
	if err != nil {
		return def
	}
	return v
}

func (p *Configer) GetInt(path string) (int, error) {
	v, err := p.GetFloat64(path)
	if err != nil {
		return 0, err
	}

	return int(v), nil
}

func (p *Configer) GetIntDef(path string, def int) int {
	v, err := p.GetInt(path)
	if err != nil {
		return def
	}
	return v
}

type validator interface {
	Validate() error
}

type codec interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
	Name() string
}

type jsonCodec struct{}

func (c jsonCodec) Marshal(v interface{}) ([]byte, error)      { return json.Marshal(v) }
func (c jsonCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
func (c jsonCodec) Name() string                               { return "json" }

type yamlCodec struct{}

func (c yamlCodec) Marshal(v interface{}) ([]byte, error)      { return yaml2.Marshal(v) }
func (c yamlCodec) Unmarshal(data []byte, v interface{}) error { return yaml2.Unmarshal(data, v) }
func (c yamlCodec) Name() string                               { return "yaml" }

var (
	codecJson = jsonCodec{}
	codecYaml = yamlCodec{}
)

func (p *Configer) Read(path string, dest interface{}, opts_ ...Option) error {
	return p.read(codecJson, path, dest, opts_...)
}

// ReadYaml use for yaml tags `yaml:"key"`
func (p *Configer) ReadYaml(path string, dest interface{}, opts_ ...Option) error {
	return p.read(codecYaml, path, dest, opts_...)
}

func (p *Configer) read(codec codec, path string, dest interface{}, opts_ ...Option) error {
	opts := &options{}
	for _, opt := range opts_ {
		opt.apply(opts)
	}

	// ignore error
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		klog.V(5).InfoS("get pathValue err, ignored", "path", path, "v", v, "err", err)
	}

	if dest == nil {
		return nil
	}

	if v != nil {
		data, err := codec.Marshal(v)
		//klog.V(5).InfoS("marshal", "v", v, "data", string(data), "err", err)
		if err != nil {
			return err
		}

		err = codec.Unmarshal(data, dest)
		if err != nil {
			klog.V(5).InfoS("unmarshal", "type", codec.Name(), "data", string(data), "err", err)
			return err
		}
	}

	// check configer override
	for _, v := range append(p.options.override, opts.override...) {
		data := []byte{}
		if b, ok := v.([]byte); ok {
			data = b
		} else if s, ok := v.(string); ok {
			data = []byte(s)
		} else {
			data, err = codec.Marshal(v)
			if err != nil {
				klog.V(5).InfoS("marshal", "v", v, "data", string(data), "err", err)
				return err
			}
		}

		err = codec.Unmarshal(data, dest)
		if err != nil {
			klog.V(5).InfoS("unmarshal", "data", string(data), "dest", dest, "err", err)
			return err
		}
	}

	if v, ok := dest.(validator); ok {
		return v.Validate()
	}
	return nil
}

// just for yaml
func readYaml(obj, dest interface{}) error {
	data, err := yaml2.Marshal(obj)
	if err != nil {
		return err
	}

	if err := yaml2.Unmarshal(data, dest); err != nil {
		return err
	}

	if v, ok := dest.(validator); ok {
		return v.Validate()
	}

	return nil
}

func (p *Configer) Unmarshal(dest interface{}) error {
	data, err := json.Marshal(p.data)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return err
	}

	if v, ok := dest.(validator); ok {
		return v.Validate()
	}
	return nil
}

// just for dump
func _yaml(v map[string]interface{}) string {
	buf, err := yaml.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(buf)
}

func (p *Configer) String() string {
	return _yaml(p.data)
}

func yaml2Values(dest map[string]interface{}, bytes []byte) (map[string]interface{}, error) {
	return yaml2ValuesWithPath(dest, bytes, "")
}

func yaml2ValuesWithPath(dest map[string]interface{}, bytes []byte, path string) (map[string]interface{}, error) {
	currentMap := map[string]interface{}{}
	if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
		return dest, err
	}

	if len(path) > 0 {
		ps := strings.Split(path, ".")
		for i := len(ps) - 1; i >= 0; i-- {
			currentMap = map[string]interface{}{ps[i]: currentMap}
		}
	}

	dest = mergeValues(dest, currentMap)
	return dest, nil
}

// Merges source and destination map, preferring values from the source map
func mergeValues(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, ok := dest[k]; !ok {
			dest[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			dest[k] = v
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := dest[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			dest[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		dest[k] = mergeValues(destMap, nextMap)
	}
	return dest
}
