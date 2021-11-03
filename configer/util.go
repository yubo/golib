package configer

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/pflag"
	"sigs.k8s.io/yaml"
)

func SetOptions(allowEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet) {
	DefaultOptions.set(allowEnv, allowEmptyEnv, maxDepth, fs)
}

func AddFlags(f *pflag.FlagSet) {
	DefaultOptions.addFlags(f)
}

func GetTagOpts(sf reflect.StructField) (tag *TagOpts) {
	return DefaultOptions.getTagOpts(sf, nil)
}

func ValueFiles() []string {
	return DefaultOptions.valueFiles
}

// addConfigs: add flags and env from sample's tags
// defualt priority sample > tagsGetter > tags
func AddConfigs(fs *pflag.FlagSet, path string, sample interface{}, opts ...Option) error {
	options := DefaultOptions.deepCopy()
	for _, opt := range opts {
		opt(options)
	}
	options.prefixPath = path

	if v, err := objToMap(sample); err != nil {
		return err
	} else {
		options.defaultValues = pathValueToTable(path, v)
	}

	rv := reflect.Indirect(reflect.ValueOf(sample))
	rt := rv.Type()

	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("Addflag: sample must be a struct, got %v/%v", rv.Kind(), rt)
	}

	return options.addConfigs(parsePath(path), fs, rt)
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

// AddFlagsVar registry var into fs
func AddFlagsVar(fs *pflag.FlagSet, in interface{}) {
	DefaultOptions.addFlagsVar(fs, in, 0)
}

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
