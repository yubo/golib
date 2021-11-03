package configer

import (
	"fmt"
	"reflect"
	"strings"

	"sigs.k8s.io/yaml"
)

// merge path.bytes -> into
func yaml2ValuesWithPath(into map[string]interface{}, path string, data []byte) (map[string]interface{}, error) {
	currentMap := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &currentMap); err != nil {
		return into, err
	}

	if len(path) > 0 {
		ps := strings.Split(path, ".")
		for i := len(ps) - 1; i >= 0; i-- {
			currentMap = map[string]interface{}{ps[i]: currentMap}
		}
	}

	into = mergeValues(into, currentMap)
	return into, nil
}

// Merges source and into map, preferring values from the source map ( src > into)
func mergeValues(into map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
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

func objToMap(in interface{}) (map[string]interface{}, error) {
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

func yamlToMap(y []byte) (map[string]interface{}, error) {
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

func pathValueToTable(path string, val interface{}) map[string]interface{} {
	paths := parsePath(path)
	p := val

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

type TagOpts struct {
	name string // field name
	json string // json:"{json}"
	skip bool   // if json:"-"

	Flag        []string // flag:"{long},{short}"
	Default     string   // default:"{default}"
	Env         string   // env:"{env}"
	Description string   // description:"{description}"
	Deprecated  string   // deprecated:""
	Arg         string   // arg:"{arg}"  args[0] arg1... -- arg2... (deprecated)

}

func (p TagOpts) Skip() bool {
	return p.skip
}

func (p TagOpts) String() string {
	return fmt.Sprintf("json %s flag %v env %s description %s",
		p.json, p.Flag, p.Env, p.Description)
}

func getTagOpts(sf reflect.StructField) (tag *TagOpts) {
	tag = &TagOpts{name: sf.Name}
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
	}
	if opts.Contains("arg2") {
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
