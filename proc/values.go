/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proc

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/klog/v2"
)

// ErrNoTable indicates that a chart does not have a matching table.
type ErrNoTable error

// ErrNoValue indicates that Values does not contain a key with a value
type ErrNoValue error

// GlobalKey is the name of the Values key that is used for storing global vars.
const GlobalKey = "global"

// Values represents a collection of chart values.
type Values map[string]interface{}

// YAML encodes the Values into a YAML string.
func (v Values) YAML() (string, error) {
	b, err := yaml.Marshal(v)
	return string(b), err
}

// Table gets a table (YAML subsection) from a Values object.
//
// The table is returned as a Values.
//
// Compound table names may be specified with dots:
//
//	foo.bar
//
// The above will be evaluated as "The table bar inside the table
// foo".
//
// An ErrNoTable is returned if the table does not exist.
func (v Values) Table(name string) (Values, error) {
	names := strings.Split(name, ".")
	table := v
	var err error

	for _, n := range names {
		table, err = tableLookup(table, n)
		if err != nil {
			return table, err
		}
	}
	return table, err
}

// AsMap is a utility function for converting Values to a map[string]interface{}.
//
// It protects against nil map panics.
func (v Values) AsMap() map[string]interface{} {
	if v == nil || len(v) == 0 {
		return map[string]interface{}{}
	}
	return v
}

// Encode writes serialized Values information to the given io.Writer.
func (v Values) Encode(w io.Writer) error {
	//return yaml.NewEncoder(w).Encode(v)
	out, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func tableLookup(v Values, simple string) (Values, error) {
	v2, ok := v[simple]
	if !ok {
		return v, ErrNoTable(fmt.Errorf("no table named %q (%v)", simple, v))
	}
	if vv, ok := v2.(map[string]interface{}); ok {
		return vv, nil
	}

	// This catches a case where a value is of type Values, but doesn't (for some
	// reason) match the map[string]interface{}. This has been observed in the
	// wild, and might be a result of a nil map of type Values.
	if vv, ok := v2.(Values); ok {
		return vv, nil
	}

	var e ErrNoTable = fmt.Errorf("no table named %q", simple)
	return map[string]interface{}{}, e
}

// ReadValues will parse YAML byte data into a Values.
func ReadValues(data []byte) (vals Values, err error) {
	err = yaml.Unmarshal(data, &vals)
	if len(vals) == 0 {
		vals = Values{}
	}
	return
}

// ReadValuesFile will parse a YAML file into a map of values.
func ReadValuesFile(filename string) (Values, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return map[string]interface{}{}, err
	}
	return ReadValues(data)
}

// coalesceGlobals copies the globals out of src and merges them into dest.
//
// For convenience, returns dest.
func coalesceGlobals(dest, src map[string]interface{}) map[string]interface{} {
	var dg, sg map[string]interface{}

	if destglob, ok := dest[GlobalKey]; !ok {
		dg = map[string]interface{}{}
	} else if dg, ok = destglob.(map[string]interface{}); !ok {
		klog.Infof("warning: skipping globals because destination %s is not a table.", GlobalKey)
		return dg
	}

	if srcglob, ok := src[GlobalKey]; !ok {
		sg = map[string]interface{}{}
	} else if sg, ok = srcglob.(map[string]interface{}); !ok {
		klog.Infof("warning: skipping globals because source %s is not a table.", GlobalKey)
		return dg
	}

	// EXPERIMENTAL: In the past, we have disallowed globals to test tables. This
	// reverses that decision. It may somehow be possible to introduce a loop
	// here, but I haven't found a way. So for the time being, let's allow
	// tables in globals.
	for key, val := range sg {
		if istable(val) {
			vv := copyMap(val.(map[string]interface{}))
			if destv, ok := dg[key]; ok {
				if destvmap, ok := destv.(map[string]interface{}); ok {
					// Basically, we reverse order of coalesce here to merge
					// top-down.
					coalesceTables(vv, destvmap)
					dg[key] = vv
					continue
				} else {
					klog.Infof("Conflict: cannot merge map onto non-map for %q. Skipping.", key)
				}
			} else {
				// Here there is no merge. We're just adding.
				dg[key] = vv
			}
		} else if dv, ok := dg[key]; ok && istable(dv) {
			// It's not clear if this condition can actually ever trigger.
			klog.Infof("key %s is table. Skipping", key)
			continue
		}
		// TODO: Do we need to do any additional checking on the value?
		dg[key] = val
	}
	dest[GlobalKey] = dg
	return dest
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	dest := make(map[string]interface{}, len(src))
	for k, v := range src {
		dest[k] = v
	}
	return dest
}

// coalesceTables merges a source map into a destination map.
//
// dest is considered authoritative.
func coalesceTables(dst, src map[string]interface{}) map[string]interface{} {
	// Because dest has higher precedence than src, dest values override src
	// values.
	for key, val := range src {
		if istable(val) {
			if innerdst, ok := dst[key]; !ok {
				dst[key] = val
			} else if istable(innerdst) {
				coalesceTables(innerdst.(map[string]interface{}), val.(map[string]interface{}))
			} else {
				klog.Infof("warning: cannot overwrite table with non table for %s (%v)", key, val)
			}
			continue
		} else if dv, ok := dst[key]; ok && istable(dv) {
			klog.Infof("warning: destination for %s is a table. Ignoring non-table value %v", key, val)
			continue
		} else if !ok { // <- ok is still in scope from preceding conditional.
			dst[key] = val
			continue
		}
	}
	return dst
}

// istable is a special-purpose function to see if the present thing matches the definition of a YAML table.
func istable(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

// PathValue takes a path that traverses a YAML structure and returns the value at the end of that path.
// The path starts at the root of the YAML structure and is comprised of YAML keys separated by periods.
// Given the following YAML data the value at path "chapter.one.title" is "Loomings".
//
//	chapter:
//	  one:
//	    title: "Loomings"
func (v Values) PathValue(ypath string) (interface{}, error) {
	if len(ypath) == 0 {
		return nil, errors.New("YAML path string cannot be zero length")
	}
	yps := strings.Split(ypath, ".")
	if len(yps) == 1 {
		// if exists must be root key not table
		vals := v.AsMap()
		k := yps[0]
		if val, ok := vals[k]; ok {
			// key found
			return val, nil
		}
		// key not found
		return nil, ErrNoValue(fmt.Errorf("%v is not a value", k))
	}
	// join all elements of YAML path except last to get string table path
	ypsLen := len(yps)
	table := yps[:ypsLen-1]
	st := strings.Join(table, ".")
	// get the last element as a string key
	key := yps[ypsLen-1:]
	sk := string(key[0])
	// get our table for table path
	t, err := v.Table(st)
	if err != nil {
		//no table
		return nil, ErrNoValue(fmt.Errorf("%v is not a value", sk))
	}
	// check table for key and ensure value is not a table
	if k, ok := t[sk]; ok {
		// key found
		return k, nil
	}

	// key not found
	return nil, ErrNoValue(fmt.Errorf("key not found: %s", sk))
}
