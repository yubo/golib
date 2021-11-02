package configer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestMerge(t *testing.T) {
	cases := []struct {
		into string
		src  string
		want string
	}{
		{"", "", "{}\n"},
		{"a: 1", "", "a: 1\n"},
		{
			"a: [1, 2, 3]",
			"a: [2, 3, 4]",
			"a:\n- 2\n- 3\n- 4\n",
		},
		{
			`a:
- name: tom
  age: 16
- name: john
  age: 17
`, `a:
- age: 18
- name: john
  age: 19
`, `a:
- age: 18
- age: 19
  name: john
`,
		},
		{`certs:
- names: ["abc"]
  keyFile: "foo.key"
  certFile: "foo.crt"
`, `certs:
- keyFile: "bar.key"
  certFile: "bar.crt"
`, `certs:
- certFile: bar.crt
  keyFile: bar.key
`,
		},
	}
	for i, c := range cases {
		into, err := yamlToMap([]byte(c.into))
		assert.NoError(t, err, i)

		src, err := yamlToMap([]byte(c.src))
		assert.NoError(t, err, i)

		got, err := yaml.Marshal(mergeValues(into, src))
		assert.NoError(t, err, i)

		assert.Equal(t, c.want, string(got), i)
	}
}
