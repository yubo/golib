package util

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestDiff2Map(t *testing.T) {
	nestedMap := map[string]interface{}{
		"foo": "bar",
		"baz": map[string]string{
			"cool": "stuff",
		},
	}
	anotherNestedMap := map[string]interface{}{
		"foo": "bar",
		"baz": map[string]string{
			"cool":    "things",
			"awesome": "stuff",
		},
	}
	flatMap := map[string]interface{}{
		"foo": "bar",
		"baz": "stuff",
	}
	anotherFlatMap := map[string]interface{}{
		"testing": "fun",
	}

	testMap := Diff2Map(flatMap, nestedMap)
	want := map[string]interface{}{
		"baz": map[string]interface{}{
			"cool": "stuff",
		},
	}
	require.Equalf(t, objStr(want), objStr(testMap),
		"Expected a nested map to overwrite a flat value")

	testMap = Diff2Map(nestedMap, flatMap)
	want = map[string]interface{}{
		"baz": "stuff",
	}
	require.Equalf(t, objStr(want), objStr(testMap),
		"Expected a flat value to overwrite a map.")

	testMap = Diff2Map(nestedMap, anotherNestedMap)
	want = map[string]interface{}{
		"baz": map[string]string{
			"cool":    "things",
			"awesome": "stuff",
		},
	}
	require.Equalf(t, objStr(want), objStr(testMap),
		"Expected a nested map to overwrite another nested map.")

	testMap = Diff2Map(anotherFlatMap, anotherNestedMap)
	require.Equalf(t, objStr(anotherNestedMap), objStr(testMap),
		"Expected a map with different keys to merge properly with another map")
}

func objStr(in interface{}) string {
	buf, err := yaml.Marshal(in)
	if err != nil {
		panic(err)
	}
	return string(buf)
}

func TestDiff(t *testing.T) {
	cases := []struct {
		name string
		src  []string
		dst  []string
		add  []string
		del  []string
	}{{
		"1",
		[]string{"1", "2"},
		nil,
		nil,
		[]string{"1", "2"},
	}, {
		"2",
		nil,
		[]string{"1", "2"},
		[]string{"1", "2"},
		nil,
	}, {
		"3",
		[]string{"1", "2"},
		[]string{"3", "4"},
		[]string{"3", "4"},
		[]string{"1", "2"},
	}, {
		"4",
		[]string{"1", "2"},
		[]string{"2", "3"},
		[]string{"3"},
		[]string{"1"},
	}}

	for _, c := range cases {
		add, del := Diff(c.src, c.dst)
		require.Equal(t, _ss2map(c.add), _ss2map(add), c.name)
		require.Equal(t, _ss2map(c.del), _ss2map(del), c.name)
	}
}
