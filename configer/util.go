package configer

import (
	"fmt"
	"net"
	"reflect"
	"strings"

	"github.com/spf13/pflag"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/validation/field"
	"github.com/yubo/golib/util/yaml"
	"k8s.io/klog/v2"
)

type fieldValidator interface {
	Validate(fldPath *field.Path) error
}

type validator interface {
	Validate() error
}

// ErrNoTable indicates that a chart does not have a matching table.
type ErrNoTable struct {
	Key string
}

func (e ErrNoTable) Error() string { return fmt.Sprintf("%q is not a table", e.Key) }

// ErrNoValue indicates that Values does not contain a key with a value
type ErrNoValue struct {
	Key string
}

func (e ErrNoValue) Error() string { return fmt.Sprintf("%q is not a value", e.Key) }

func indirectValue(rv reflect.Value, rt reflect.Type) (reflect.Value, reflect.Type) {
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rt.Elem()))
		}
		rv = rv.Elem()
		rt = rv.Type()
	}
	return rv, rt
}

// merge path.bytes -> into
func mergePathYaml(into map[string]interface{}, path string, yaml []byte) (map[string]interface{}, error) {
	values, err := yamlToValues(yaml)
	if err != nil {
		return nil, err
	}

	return mergePathValues(into, path, values), nil
}

func mergePathObj(into map[string]interface{}, path string, obj interface{}) (map[string]interface{}, error) {
	values, err := objToValues(obj)
	if err != nil {
		return nil, err
	}

	return mergePathValues(into, path, values), nil
}

func mergePathValue(into map[string]interface{}, path string, value interface{}) map[string]interface{} {
	return mergeValues(into, pathValueToValues(path, value))

}

func mergePathFields(into map[string]interface{}, path []string, fields []*configField) map[string]interface{} {
	for _, f := range fields {
		if v := f.defaultValue; v != nil {
			into = mergeValues(into, pathValueToValues(
				joinPath(append(path, f.configPath)...), v))
		}
	}
	return into
}

func mergePathValues(into map[string]interface{}, path string, values map[string]interface{}) map[string]interface{} {
	return mergeValues(into, pathValueToValues(path, values))
}

// Merges source and into map, preferring values from the source map ( values -> into)
func mergeValues(into map[string]interface{}, values map[string]interface{}) map[string]interface{} {
	for k, v := range values {
		// If the key doesn't exist already, then just set the key to that value
		if _, ok := into[k]; !ok {
			into[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			into[k] = v
			continue
		}
		intoMap, isMap := into[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			into[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		into[k] = mergeValues(intoMap, nextMap)
	}
	return into
}

func clonePath(path []string) []string {
	ret := make([]string, len(path))
	copy(ret, path)
	return ret
}

func objToValues(in interface{}) (map[string]interface{}, error) {
	b, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func yamlToValues(y []byte) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	if err := yaml.Unmarshal(y, &out); err != nil {
		return nil, err
	}

	return out, nil
}

type zeroChecker interface {
	IsZero() bool
}

func isZero(in interface{}) bool {
	if in == nil {
		return true
	}
	if f, ok := in.(zeroChecker); ok {
		return f.IsZero()
	}

	return reflect.ValueOf(in).IsZero()
}

func pathValueToValues(path string, value interface{}) map[string]interface{} {
	paths := parsePath(path)
	p := value

	for i := len(paths) - 1; i >= 0; i-- {
		p = map[string]interface{}{paths[i]: p}
	}
	return p.(map[string]interface{})
}

func prepareValue(rv reflect.Value, rt reflect.Type) {
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		rv.Set(reflect.New(rt.Elem()))
	}
}

type FieldTag struct {
	name string // field name
	json string // json:"{json}"
	skip bool   // if json:"-"

	Flag        []string // flag:"{long},{short}"
	Arg         string   // arg:"{arg}"  args[0] arg1... -- arg2... (deprecated)
	Default     string   // default:"{default}"
	Env         string   // env:"{env}"
	Description string   // description:"{description}"
	Deprecated  string   // deprecated:""
}

func (p FieldTag) String() string {
	return fmt.Sprintf("json %s flag %v env %s description %s",
		p.json, p.Flag, p.Env, p.Description)
}

func (p FieldTag) Skip() bool {
	return p.skip
}

func GetFieldTag(sf reflect.StructField) (tag *FieldTag) {
	tag = &FieldTag{name: sf.Name}
	if sf.Anonymous {
		return
	}

	json, opts := parseTag(sf.Tag.Get("json"))
	if json == "-" {
		tag.skip = true
		return
	}

	if json != "" {
		tag.json = json
	} else {
		tag.json = sf.Name
	}
	if opts.Contains("arg1") {
		tag.Arg = "arg1"
	} else if opts.Contains("arg2") {
		tag.Arg = "arg2"
	}

	if flag := strings.Split(strings.TrimSpace(sf.Tag.Get("flag")), ","); len(flag) > 0 && flag[0] != "" && flag[0] != "-" {
		tag.Flag = flag
	}

	tag.Default = sf.Tag.Get("default")
	tag.Description = sf.Tag.Get("description")
	tag.Deprecated = sf.Tag.Get("deprecated")
	tag.Env = strings.Replace(strings.ToUpper(sf.Tag.Get("env")), "-", "_", -1)
	if tag.Env != "" {
		tag.Description = fmt.Sprintf("%s (env %s)", tag.Description, tag.Env)
	}

	return
}

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == optionName {
			return true
		}
		s = next
	}
	return false
}

func ToString(v interface{}) string {
	switch v.(type) {
	case []interface{}, map[string]interface{}:
		b, _ := yaml.Marshal(v)
		return string(b)
	default:
		return util.ToString(v)
	}
}

func ToStringMapString(str string) map[string]string {
	v := map[string]string{}
	if err := yaml.Unmarshal([]byte(str), &v); err != nil {
		v = util.ToStringMapString(str)
	}

	return v
}

func ToStringArrayVar(str string) []string {
	v := []string{}
	if err := yaml.Unmarshal([]byte(str), &v); err != nil {
		v = util.ToStringSlice(str)
	}

	return v
}

func ToIntSlice(str string) []int {
	v := []int{}
	if err := yaml.Unmarshal([]byte(str), &v); err != nil {
		v = util.ToIntSlice(str)
	}

	return v
}

func ToFloat64Slice(str string) []float64 {
	v := []float64{}
	if err := yaml.Unmarshal([]byte(str), &v); err != nil {
		v, _ = ToFloat64SliceE(str)
	}

	return v
}

// ToFloat64SliceE casts an interface to a []time.Duration type.
func ToFloat64SliceE(i interface{}) ([]float64, error) {
	if i == nil {
		return []float64{}, fmt.Errorf("unable to cast %#v of type %T to []float64", i, i)
	}

	switch v := i.(type) {
	case []float64:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]float64, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := util.ToFloat64E(s.Index(j).Interface())
			if err != nil {
				return []float64{}, fmt.Errorf("unable to cast %#v of type %T to []float64", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []float64{}, fmt.Errorf("unable to cast %#v of type %T to []float64", i, i)
	}
}

func ToIPSlice(str string) []net.IP {
	v := []net.IP{}
	if err := yaml.Unmarshal([]byte(str), &v); err != nil {
		v, _ = ToIPSliceE(str)
	}

	return v
}

func ToIPSliceE(i interface{}) ([]net.IP, error) {
	if i == nil {
		return []net.IP{}, fmt.Errorf("unable to cast %#v of type %T to []net.IP", i, i)
	}

	switch v := i.(type) {
	case []net.IP:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]net.IP, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := util.ToIPE(s.Index(j).Interface())
			if err != nil {
				return []net.IP{}, fmt.Errorf("unable to cast %#v of type %T to []net.IP", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []net.IP{}, fmt.Errorf("unable to cast %#v of type %T to []net.IP", i, i)
	}
}

type configField struct {
	fs           *pflag.FlagSet
	envName      string      // env name
	flag         string      // flag
	shothand     string      // flag shothand
	configPath   string      // config path
	flagValue    interface{} // flag's value
	defaultValue interface{} // field's default value
}

func (f configField) getFlagValue() interface{} {
	if f.flag != "" && f.fs.Changed(f.flag) {
		return reflect.ValueOf(f.flagValue).Elem().Interface()
	}

	return nil
}

type defaultSetter interface {
	SetDefault(string) error
}

func newConfigFieldByValue(value pflag.Value, fs *pflag.FlagSet, path string, tag *FieldTag, defValue string) *configField {
	rt := reflect.Indirect(reflect.ValueOf(value)).Type()
	def := reflect.New(rt).Interface().(pflag.Value)

	// set value
	if defValue != "" {
		if d, ok := def.(defaultSetter); ok {
			d.SetDefault(defValue)
			value.(defaultSetter).SetDefault(defValue)
		} else {
			// the changed flag may be affected
			def.Set(defValue)
			value.Set(defValue)
		}
	}

	field := &configField{
		fs:         fs,
		configPath: path,
		envName:    tag.Env,
		flagValue:  value,
	}

	if tag.Default != "" {
		field.defaultValue = def
	}

	switch len(tag.Flag) {
	case 0:
		return field
	// nothing
	case 1:
		field.flag = tag.Flag[0]
		fs.Var(value, tag.Flag[0], tag.Description)
	case 2:
		field.flag = tag.Flag[0]
		field.shothand = tag.Flag[1]
		fs.VarP(value, tag.Flag[0], tag.Flag[1], tag.Description)
	default:
		panic("invalid flag value")
	}

	if len(field.flag) > 0 && len(tag.Deprecated) > 0 {
		fs.MarkDeprecated(field.flag, tag.Deprecated)
		fs.Lookup(field.flag).Hidden = false
	}

	return field
}

func newConfigField(value interface{}, fs *pflag.FlagSet, path string, tag *FieldTag, varFn, varPFn, def interface{}) *configField {
	field := &configField{
		fs:         fs,
		configPath: path,
		envName:    tag.Env,
	}

	if tag.Default != "" {
		field.defaultValue = def
	}

	// add flag
	switch len(tag.Flag) {
	case 0:
		// nothing
		return field
	case 1:
		field.flag = tag.Flag[0]
		reflect.ValueOf(varFn).Call([]reflect.Value{
			reflect.ValueOf(value),
			reflect.ValueOf(tag.Flag[0]),
			reflect.ValueOf(def),
			reflect.ValueOf(tag.Description),
		})
		field.flagValue = value
	case 2:
		field.flag = tag.Flag[0]
		field.shothand = tag.Flag[1]
		reflect.ValueOf(varPFn).Call([]reflect.Value{
			reflect.ValueOf(value),
			reflect.ValueOf(tag.Flag[0]),
			reflect.ValueOf(tag.Flag[1]),
			reflect.ValueOf(def),
			reflect.ValueOf(tag.Description),
		})
		field.flagValue = value
	default:
		panic("invalid flag value")
	}

	if len(field.flag) > 0 && len(tag.Deprecated) > 0 {
		fs.MarkDeprecated(field.flag, tag.Deprecated)
		fs.Lookup(field.flag).Hidden = false
	}

	return field
}

func dlog(msg string, keysAndValues ...interface{}) {
	if DEBUG {
		klog.InfoSDepth(2, msg, keysAndValues...)
	}
}
