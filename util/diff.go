package util

import (
	"reflect"

	yaml "gopkg.in/yaml.v2"
)

func Diff2Map(pre, now interface{}) map[string]interface{} {
	a, err := obj2map(pre)
	if err != nil {
		panic(err)
	}

	b, err := obj2map(now)
	if err != nil {
		panic(err)
	}

	return diffmap(a, b)
}

func diffmap(pre, now map[string]interface{}) map[string]interface{} {
	diff := map[string]interface{}{}
	for k, v := range now {
		// If the key doesn't exist already, then just set the key to that value
		if _, ok := pre[k]; !ok {
			diff[k] = v
			continue
		}
		defMap, defIsMap := pre[k].(map[string]interface{})
		curMap, curIsMap := v.(map[string]interface{})

		if defIsMap && curIsMap {
			// If we got to this point, it is a map in both, so changed them
			diff[k] = diffmap(defMap, curMap)
			continue
		}

		// If it isn't equal, overwrite the value
		if !reflect.DeepEqual(v, pre[k]) {
			diff[k] = v
		}
	}

	return diff
}

// just for yaml
func obj2map(in interface{}) (map[string]interface{}, error) {
	if in == nil {
		return map[string]interface{}{}, nil
	}

	buf, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}

	out := map[string]interface{}{}
	if err := yaml.Unmarshal(buf, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func Diff(src, dst []string) (add, del []string) {
	s := map[string]bool{}
	d := map[string]bool{}

	for _, v := range src {
		s[v] = true
	}

	for _, v := range dst {
		d[v] = true
	}

	for _, v := range dst {
		if !s[v] {
			add = append(add, v)
		}
	}

	for _, v := range src {
		if !d[v] {
			del = append(del, v)
		}
	}

	return
}

func Diff3(src, dst []string) (add, del, eq []string) {
	s := map[string]bool{}
	d := map[string]bool{}

	for _, v := range src {
		s[v] = true
	}

	for _, v := range dst {
		d[v] = true
		if !s[v] {
			add = append(add, v)
		} else {
			eq = append(eq, v)
		}
	}

	for _, v := range src {
		if !d[v] {
			del = append(del, v)
		}
	}

	return
}

type DiffEntity interface {
	Key() string
}

func Diff4(src, dst []DiffEntity) (add, del, eq []DiffEntity) {
	s := map[string]bool{}
	d := map[string]bool{}

	for _, v := range src {
		s[v.Key()] = true
	}

	for _, v := range dst {
		d[v.Key()] = true
		if !s[v.Key()] {
			add = append(add, v)
		} else {
			eq = append(eq, v)
		}
	}

	for _, v := range src {
		if !d[v.Key()] {
			del = append(del, v)
		}
	}

	return

}
