package configer

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/api"
	"github.com/yubo/golib/util"
)

// get config  ParseConfigFile(values, config.yml)
// merge baseconf, config

// templateFile defines the contents of a template to be stored in a file, for testing.
type templateFile struct {
	name     string
	contents string
}

func createTestDir(files []templateFile) string {
	dir, err := ioutil.TempDir("", "template")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		f, err := os.Create(filepath.Join(dir, file.name))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		_, err = io.WriteString(f, file.contents)
		if err != nil {
			log.Fatal(err)
		}
	}
	return dir
}

func TestConfig(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"conf.yml", `foo1: b_bar1
foo2: v_bar2
foo3: b_bar3
fooo:
  foo: bar
  foos: ["1", "2"]`},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	_, err := NewConfiger().Parse(WithValueFile("conf.yml"))
	assert.NoError(t, err)
}

func TestConfigWithConfig(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	type Bar struct {
		Foo  Foo `json:"foo"`
		Foo2 Foo `json:"foo2"`
	}
	v := Bar{Foo{2}, Foo{3}}

	{
		c, err := NewConfiger().Parse(WithDefault("foo", v.Foo))
		assert.NoError(t, err)

		var got Bar
		err = c.Read("foo", &got.Foo)
		assert.NoError(t, err)
		assert.Equalf(t, v.Foo, got.Foo, "configer read \"foo\"")
	}

	{
		c, err := NewConfiger().Parse(WithDefault("", v))
		assert.NoError(t, err)

		var got Bar
		err = c.Read("", &got)
		assert.NoError(t, err)
		assert.Equalf(t, v, got, "configer read \"\" configer %s", c)
	}
}

func TestConfigSet(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	type Bar struct {
		Foo  Foo `json:"foo"`
		Foo2 Foo `json:"foo2"`
	}
	v := Bar{Foo{2}, Foo{3}}

	{
		c, _ := NewConfiger().Parse(WithDefault("foo", v.Foo))
		c.Set("foo", Foo{20})

		var got Bar
		c.Read("foo", &got.Foo)
		assert.Equalf(t, 20, got.Foo.A, "configer read \"foo\"")

		c.Set("foo.a", 200)
		c.Read("foo", &got.Foo)
		assert.Equalf(t, 200, got.Foo.A, "configer read \"foo.a\"")
	}

	{
		c, _ := NewConfiger().Parse(WithDefault("a", v.Foo2))
		c.Set("a", Foo{30})

		var got Bar
		c.Read("a", &got.Foo2)
		assert.Equalf(t, 30, got.Foo2.A, "configer read \"a\"")
	}

}

func TestRaw(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"conf.yml", `foo1: b_bar1
foo2: v_bar2
foo3: b_bar3
fooo:
  foo: bar
  foos: ["1", "2"]`},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	config, _ := NewConfiger().Parse(WithValueFile("conf.yml"))

	var cases = []struct {
		path string
		want interface{}
	}{
		{"foo1", "b_bar1"},
		{"foo2", "v_bar2"},
		{"foo3", "b_bar3"},
		{"fooo.foo", "bar"},
		{"na", nil},
		{"na.na", nil},
	}

	for _, c := range cases {
		assert.Equalf(t, c.want, config.GetRaw(c.path), "configer.GetRaw(%s)", c.path)
	}
}

func TestRead(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"conf.yml", `foo1: b_bar1
foo2: v_bar2
foo3: b_bar3
fooo:
  foo: bar
  foos: ["1", "2"]`},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	config, _ := Parse(WithValueFile("conf.yml"))

	var (
		got  []string
		path = "fooo.foos"
		want = []string{"1", "2"}
	)

	if err := config.Read(path, &got); err != nil {
		t.Error(err)
	}

	assert.Equalf(t, want, got, "configer read %s", path)
}

func TestRawType(t *testing.T) {
	yml := `
ctrl:
  auth:
    google:
      client_id: "781171109477-10tu51e8bs1s677na46oct6hdefpntpu.apps.googleusercontent.com"
      client_secret: xpEoBFqkmI3KVN9pHt2VW-eN
      redirect_url: http://auth.dev.pt.example.com/v1.0/auth/callback/google
`

	var cases = []struct {
		path string
		want string
	}{
		{"", "map[string]interface {}"},
		{"ctrl", "map[string]interface {}"},
		{"ctrl.auth", "map[string]interface {}"},
		{"ctrl.auth.google", "map[string]interface {}"},
		{"ctrl.auth.google.client_id", "string"},
	}

	cfg, _ := NewConfiger().Parse(WithDefaultYaml("", yml))
	for _, c := range cases {
		if got := util.GetType(cfg.GetRaw(c.path)); got != c.want {
			assert.Equalf(t, c.want, got, "data %s", cfg)
		}
	}

	// test to configer
}

func TestToConfiger(t *testing.T) {
	yml := `
ctrl:
  auth:
    google:
      client_id: "781171109477-10tu51e8bs1s677na46oct6hdefpntpu.apps.googleusercontent.com"
      client_secret: xpEoBFqkmI3KVN9pHt2VW-eN
      redirect_url: http://auth.dev.pt.example.com/v1.0/auth/callback/google
`
	var cases = []struct {
		path1 string
		path2 string
		want  string
	}{
		{"ctrl.auth", "google", "map[string]interface {}"},
		{"ctrl.auth", "google.client_secret", "string"},
		{"ctrl.auth.google", "client_id", "string"},
	}

	conf, _ := NewConfiger().Parse(WithDefaultYaml("", yml))
	for _, c := range cases {
		cf := conf.GetConfiger(c.path1)
		if cf == nil {
			t.Fatalf("get %s error", c.path1)
		}

		got := util.GetType(cf.GetRaw(c.path2))
		assert.Equal(t, c.want, got)

	}
}

func TestGetConfiger(t *testing.T) {
	yml := `
ctrl:
  auth:
    google:
      client_id: "781171109477-10tu51e8bs1s677na46oct6hdefpntpu.apps.googleusercontent.com"
      client_secret: xpEoBFqkmI3KVN9pHt2VW-eN
      redirect_url: http://auth.dev.pt.example.com/v1.0/auth/callback/google
`

	type auth struct {
		ClientId     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RedirectUrl  string `json:"redirect_url"`
	}

	want := auth{
		ClientId:     "781171109477-10tu51e8bs1s677na46oct6hdefpntpu.apps.googleusercontent.com",
		ClientSecret: "xpEoBFqkmI3KVN9pHt2VW-eN",
		RedirectUrl:  "http://auth.dev.pt.example.com/v1.0/auth/callback/google",
	}

	var got auth

	conf, _ := NewConfiger().Parse(WithDefaultYaml("", yml))
	conf2 := conf.GetConfiger("ctrl.auth")
	err := conf2.Read("google", &got)
	if err != nil {
		t.Fatalf("error %s", err)
	}
	assert.Equalf(t, want, got, "configer read %s", conf2)
}

func TestWithValueFile(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"base.yml", "foo1: base1\nfoo3: base3"},
		{"conf.yml", "foo1: conf1\nfoo2: conf2"},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	cf, _ := NewConfiger().Parse(WithValueFile("base.yml", "conf.yml"))

	var cases = []struct {
		path string
		want interface{}
	}{
		{"foo1", "conf1"},
		{"foo2", "conf2"},
		{"foo3", "base3"},
	}

	for _, c := range cases {
		got := cf.GetRaw(c.path)
		assert.Equal(t, c.want, got)
	}
}

func TestWithDefaultYaml(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"conf.yml", "foo:\n  foo1: conf1\n  foo2: conf2"},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	cf, _ := NewConfiger().Parse(
		WithValueFile("conf.yml"),
		WithDefaultYaml("foo", "foo1: base1"),
		WithDefaultYaml("foo", "foo3: base3"))

	var cases = []struct {
		path string
		want interface{}
	}{
		{"foo.foo1", "conf1"},
		{"foo.foo2", "conf2"},
		{"foo.foo3", "base3"},
	}

	for _, c := range cases {
		got := cf.GetRaw(c.path)
		assert.Equal(t, c.want, got)
	}
}

func TestWithOverride(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"conf.yml", "foo:\n  foo1: conf1\n  foo2: conf2"},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	cf, _ := NewConfiger().Parse(
		WithValueFile("conf.yml"),
		WithOverrideYaml("foo", "foo1: override1"),
	)

	var cases = []struct {
		path string
		want interface{}
	}{
		{"foo.foo1", "override1"},
		{"foo.foo2", "conf2"},
	}

	for _, c := range cases {
		got := cf.GetRaw(c.path)
		assert.Equal(t, c.want, got)
	}

}

func TestConfigWithFlagSets(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"base.yml", `
a: base_a
b: base_b
c: base_c
d: base_d
e: base_e
`},
		{"conf.yml", `
b: conf_b
c: conf_c
d: conf_d
e: conf_e
`},
		{"v1.yml", `
c: v1_c 
d: v1_d
e: v1_e
`},
		{"v2.yml", `
d: v2_d
e: v2_e
`},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	cff := newConfiger()
	cff.valueFiles = []string{"base.yml", "conf.yml", "v1.yml", "v2.yml"}
	cff.values = []string{"e=v2_e", "f1=f1,f2=f2"}
	cff.stringValues = []string{"sv1=sv1,sv2=sv2"}

	cf, err := cff.Parse()
	assert.NoError(t, err)

	var cases = []struct {
		path string
		want interface{}
	}{
		{"a", "base_a"},
		{"b", "conf_b"},
		{"c", "v1_c"},
		{"d", "v2_d"},
		{"e", "v2_e"},
		{"f1", "f1"},
		{"f2", "f2"},
		{"sv1", "sv1"},
		{"sv2", "sv2"},
	}

	for _, c := range cases {
		assert.Equalf(t, c.want, cf.GetRaw(c.path), "getRaw(%s)", c.path)
	}
}

func TestConfigerWithTagOptsGetter(t *testing.T) {
	type Foo struct {
		A string `json:"a" flag:"test-a" env:"TEST_A" default:"default-a"`
	}

	opts2 := &FieldTag{
		name:    "A",
		json:    "a",
		Flag:    []string{"test-a"},
		Env:     "TEST_A",
		Default: "default-a",
	}
	getter := func(path string) *FieldTag {
		switch path {
		case "a":
			return opts2
		default:
			return nil
		}
	}

	var cases = []struct {
		tagOptsGetter func(string) *FieldTag
		want          *FieldTag
	}{
		{nil, &FieldTag{
			name:    "A",
			json:    "a",
			Flag:    []string{"test-a"},
			Env:     "TEST_A",
			Default: "default-a",
		}},
		{getter, opts2},
	}

	for _, c := range cases {
		cff := NewConfiger()
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		err := cff.Var(fs, "", &Foo{})
		assert.NoError(t, err)

		found := false
		fs.VisitAll(func(flag *pflag.Flag) {
			if flag.Name == c.want.Flag[0] {
				found = true
			}
		})
		if !found {
			t.Errorf("can't found flag --%s", c.want.Flag[0])
		}
	}
}

// a=1,b=2
func setFlags(fs *pflag.FlagSet, flags string) {
	if flags == "" {
		return
	}

	if a1 := strings.Split(flags, ","); len(a1) > 0 {
		flags := map[string]string{}
		for _, v := range a1 {
			s := strings.Split(v, "=")
			flags[s[0]] = s[1]
		}
		setFlagSet(fs, flags)
	}
}

func setFlagSet(fs *pflag.FlagSet, values map[string]string) {
	fs.VisitAll(func(flag *pflag.Flag) {
		if v, ok := values[flag.Name]; ok && len(v) > 0 {
			flag.Value.Set(v)
			flag.Changed = true
		}
	})
}

func TestPriority(t *testing.T) {
	type Foo struct {
		A string `json:"a" flag:"test-a" env:"TEST_A" default:"default-a"`
	}
	dir := createTestDir([]templateFile{{"base.yml", `a: base_a`}})
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	var cases = []struct {
		name         string
		override     string
		overrideYaml string
		flag         string
		setFile      string
		set          string
		setString    string
		file         string
		env          string
		def          string // WithDefault
		defYaml      string // WithDefaultYaml
		sample       string // Var
		tag          string // withTags
		want         interface{}
	}{{
		name: "override",
		flag: "override-a",
		want: "override-a",
	}, {
		name: "flag",
		flag: "flag-a",
		want: "flag-a",
	}, {
		name:    "--set-file",
		setFile: "set-file-a",
		want:    "set-file-a",
	}, {
		name: "--set",
		set:  "set-a",
		want: "set-a",
	}, {
		name:      "--set-string",
		setString: "set-string-a",
		want:      "set-string-a",
	}, {
		name: "--values",
		file: "file-a",
		want: "file-a",
	}, {
		name: "Var env",
		env:  "env-a",
		want: "env-a",
	}, {
		name: "WithDefault",
		def:  "def-a",
		want: "def-a",
	}, {
		name:    "WithDefaultYaml",
		defYaml: "def-yaml-a",
		want:    "def-yaml-a",
	}, {
		name:   "Var sample",
		sample: "sample-a",
		want:   "sample-a",
	}, {
		name: "Var tag",
		tag:  "tag-a",
		want: "tag-a",
	}, {
		name: "Var default",
		want: "default-a",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cff := NewConfiger()
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

			if c.env != "" {
				os.Setenv("TEST_A", c.env)
			} else {
				os.Unsetenv("TEST_A")
			}

			fopts := []ConfigFieldsOption{}
			if c.tag != "" {
				tags := map[string]*FieldTag{
					"a": {Default: c.tag},
				}
				fopts = append(fopts,
					WithTags(func() map[string]*FieldTag {
						return tags
					}))
			}

			err := cff.Var(fs, "", &Foo{A: c.sample}, fopts...)
			assert.NoError(t, err)

			args := []string{}

			if c.flag != "" {
				args = append(args, "--test-a="+c.flag)
			}
			if c.setFile != "" {
				ioutil.WriteFile(filepath.Join(dir, "set_file.yml"), []byte(c.setFile), 0666)
				args = append(args, "--set-file=a="+filepath.Join(dir, "set_file.yml"))
			}
			if c.set != "" {
				args = append(args, "--set=a="+c.set)
			}
			if c.setString != "" {
				args = append(args, "--set-string=a="+c.setString)
			}

			if c.file != "" {
				ioutil.WriteFile(filepath.Join(dir, "base.yml"), []byte("a: "+c.file), 0666)
				args = append(args, "--values="+filepath.Join(dir, "base.yml"))
			}
			cff.AddFlags(fs)

			err = fs.Parse(args)
			assert.NoError(t, err)

			opts := []ConfigerOption{}

			if c.override != "" {
				opts = append(opts, WithOverride("", &Foo{A: c.override}))
			}
			if c.overrideYaml != "" {
				opts = append(opts, WithOverrideYaml("", "a: "+c.overrideYaml))
			}
			if c.def != "" {
				opts = append(opts, WithDefault("", &Foo{A: c.def}))
			}
			if c.defYaml != "" {
				opts = append(opts, WithDefaultYaml("", "a: "+c.defYaml))
			}

			cfg, err := cff.Parse(opts...)
			assert.NoError(t, err)

			assert.Equalf(t, c.want, cfg.GetRaw("a"), "case %+v env [%s] cfg [%s]", c, os.Getenv("TEST_A"), cfg)
		})
	}
}

func TestConfigerDef(t *testing.T) {
	type Foo struct {
		A string `json:"a" default:"default-a"`
		B string `json:"b" default:""`
	}
	type Bar struct {
		Foo *Foo `json:"foo"`
	}

	cff := NewConfiger()

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	err := cff.Var(fs, "bar", &Bar{Foo: &Foo{B: "default-b"}})
	assert.NoError(t, err)

	cfg, err := cff.Parse()
	assert.NoError(t, err)

	assert.Equalf(t, "default-a", cfg.GetRaw("bar.foo.a"), "config [%s]", cfg)
	assert.Equalf(t, "default-b", cfg.GetRaw("bar.foo.b"), "config [%s]", cfg)
}

func TestVar(t *testing.T) {
	var cases = []struct {
		fn   func(string) interface{}
		data [][4]string
	}{{
		func(val string) interface{} {
			type Foo struct {
				Duration *api.Duration `json:"duration" flag:"timeout" default:"5s"`
			}
			if val == "" {
				return &Foo{}
			}
			v, _ := strconv.Atoi(val)
			return &Foo{&api.Duration{
				Duration: time.Duration(v) * time.Second,
			}}
		}, [][4]string{
			// name, flag, yaml, expected
			{"duration default", "", "", "5"},
			{"duration flag", "timeout=10s", "", "10"},
			{"duration file", "", "duration: 20s", "20"},
		},
	}, {
		func(val string) interface{} {
			type Foo struct {
				IP net.IP `json:"ip" flag:"ip" default:"1.1.1.1"`
			}
			if val == "" {
				return &Foo{}
			}
			return &Foo{IP: net.ParseIP(val)}
		}, [][4]string{
			// name, flag, yaml, expected
			{"ip default", "", "", "1.1.1.1"},
			{"ip flag", "ip=2.2.2.2", "", "2.2.2.2"},
			{"ip file", "", "ip: 3.3.3.3", "3.3.3.3"},
		},
	}}
	for _, c1 := range cases {
		fn := c1.fn
		for _, c := range c1.data {
			name, args, yaml, want, got := c[0], c[1], c[2], fn(c[3]), fn("")

			t.Run(name, func(t *testing.T) {
				dir := createTestDir([]templateFile{
					{"base.yml", yaml},
				})
				defer os.RemoveAll(dir)
				os.Chdir(dir)

				cff := NewConfiger()

				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

				err := cff.Var(fs, "", got)
				assert.NoError(t, err)

				setFlags(fs, args)

				cfg, err := cff.Parse(
					WithValueFile("base.yml"),
				)
				assert.NoError(t, err)

				cfg.Read("", got)
				assert.Equal(t, want, got)
			})
		}
	}
}

func TestAddFlags(t *testing.T) {
	type Bar struct {
		A string  `json:"a" flag:"bar-a" env:"bar_a" default:"def-bar-a"`
		B *string `json:"b" flag:"bar-b" env:"bar_b" default:"def-bar-b"`
		C *string `json:"c" flag:"bar-c" env:"bar_c"`
	}
	type Foo struct {
		A string  `json:"a" flag:"foo-a" env:"foo_a" default:"def-foo-a"`
		B *string `json:"b" flag:"foo-b" env:"foo-b" default:"def-foo-b"`
		C *Bar    `json:"c"`
	}

	cases := []struct {
		args []string
		want Foo
	}{{
		[]string{},
		Foo{
			A: "def-foo-a",
			B: util.String("def-foo-b"),
			C: &Bar{
				A: "def-bar-a",
				B: util.String("def-bar-b"),
				C: util.String(""),
			},
		},
	}, {
		[]string{
			"--bar-a=bar-a",
			"--bar-b=bar-b",
			"--bar-c=bar-c",
			"--foo-a=foo-a",
			"--foo-b=foo-b",
		},
		Foo{
			A: "foo-a",
			B: util.String("foo-b"),
			C: &Bar{
				A: "bar-a",
				B: util.String("bar-b"),
				C: util.String("bar-c"),
			},
		},
	}}

	for _, c := range cases {

		cff := NewConfiger()
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

		var got Foo
		err := cff.Var(fs, "foo", &got)
		assert.NoError(t, err)

		err = fs.Parse(c.args)
		assert.NoError(t, err)

		assert.Equalf(t, util.JsonStr(c.want), util.JsonStr(got), "args %v", c.args)
	}
}

func TestConfigTypes(t *testing.T) {
	type Foo struct {
		Bool     *bool             `json:"bool" default:"true"`
		Byte     *byte             `json:"byte" default:"1"`
		Float32  *float32          `json:"float32" default:"1"`
		Float64  *float64          `json:"float64" default:"1"`
		Float64s []float64         `json:"float64s" default:"1,2"`
		Int      *int              `json:"int" default:"1"`
		Int8     *int8             `json:"int8" default:"1"`
		Int16    *int16            `json:"int16" default:"1"`
		Int32    *int32            `json:"int32" default:"1"`
		Int64    *int64            `json:"int64" default:"1"`
		Ints     []int             `json:"ints" default:"1,2"`
		IP       net.IP            `json:"ip" default:"1.1.1.1"`
		String   *string           `json:"string" default:"1"`
		Stringm  map[string]string `json:"stringm" default:"{\"1\":\"1\", \"2\":\"2\"}"`
		Strings  []string          `json:"strings" default:"[\"1\", \"2\"]"`
		Uint     *uint             `json:"uint" default:"1"`
		Uint8    *uint8            `json:"uint8" default:"1"`
		Uint16   *uint16           `json:"uint16" default:"1"`
		Uint32   *uint32           `json:"uint32" default:"1"`
		Uint64   *uint64           `json:"uint64" default:"1"`
	}
	cases := []struct {
		sample *Foo
		want   string
	}{{
		&Foo{},
		"bool: true\nbyte: 1\nfloat32: 1\nfloat64: 1\nfloat64s: []\nint: 1\nint8: 1\nint16: 1\nint32: 1\nint64: 1\nints: []\nip: 1.1.1.1\nstring: \"1\"\nstringm:\n  \"1\": \"1\"\n  \"2\": \"2\"\nstrings:\n- \"1\"\n- \"2\"\nuint: 1\nuint8: 1\nuint16: 1\nuint32: 1\nuint64: 1\n",
	}, {
		&Foo{
			Bool:     util.Bool(true),
			Byte:     util.Byte(3),
			Float32:  util.Float32(3),
			Float64:  util.Float64(3),
			Float64s: []float64{3, 4},
			Int:      util.Int(3),
			Int8:     util.Int8(3),
			Int16:    util.Int16(3),
			Int32:    util.Int32(3),
			Int64:    util.Int64(3),
			Ints:     []int{3, 4},
			IP:       net.IPv4(3, 3, 3, 3),
			String:   util.String("3"),
			Stringm:  map[string]string{"3": "3", "4": "4"},
			Strings:  []string{"3", "4"},
			Uint:     util.Uint(3),
			Uint8:    util.Uint8(3),
			Uint16:   util.Uint16(3),
			Uint32:   util.Uint32(3),
			Uint64:   util.Uint64(3),
		},
		"bool: true\nbyte: 3\nfloat32: 3\nfloat64: 3\nfloat64s:\n- 3\n- 4\nint: 3\nint8: 3\nint16: 3\nint32: 3\nint64: 3\nints:\n- 3\n- 4\nip: 3.3.3.3\nstring: \"3\"\nstringm:\n  \"3\": \"3\"\n  \"4\": \"4\"\nstrings:\n- \"3\"\n- \"4\"\nuint: 3\nuint8: 3\nuint16: 3\nuint32: 3\nuint64: 3\n",
	}}
	for _, c := range cases {
		cff := NewConfiger()
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

		err := cff.Var(fs, "", c.sample)
		assert.NoError(t, err)

		// obj ->  yaml
		cf, err := cff.Parse()
		assert.NoError(t, err)
		assert.Equalf(t, c.want, cf.String(), "sample %s", util.YamlStr(c.sample))

		// yaml -> obj
		o := Foo{}
		cf2, err := cff.Parse(WithOverrideYaml("", cf.String()))
		assert.NoError(t, err)
		err = cf2.Read("", &o)
		assert.NoError(t, err)
		assert.Equalf(t, c.want, util.YamlStr(o), "sample %s", util.YamlStr(c.sample))

	}

}

func TestConfigDefault(t *testing.T) {
	t.Run("str", func(t *testing.T) {
		type Foo struct {
			A string  `json:"a" default:"def"`
			B *string `json:"b" default:"def"`
		}
		cases := []struct {
			sample *Foo
			want   string
		}{{
			&Foo{A: "", B: nil},
			"a: def\nb: def\n",
		}, {
			&Foo{A: "a", B: util.String("b")},
			"a: a\nb: b\n",
		}}
		for _, c := range cases {
			cff := NewConfiger()
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

			err := cff.Var(fs, "", c.sample)
			assert.NoError(t, err)

			cf, err := cff.Parse()
			assert.NoError(t, err)

			assert.Equalf(t, c.want, cf.String(), "sample %s", util.YamlStr(c.sample))
		}
	})

	t.Run("int", func(t *testing.T) {
		type Foo struct {
			A int  `json:"a" default:"1"`
			B *int `json:"b" default:"1"` // best for zero and unset
			C int  `json:"c,omitempty" default:"1"`
		}
		cases := []struct {
			sample *Foo
			want   string
		}{{
			&Foo{},
			"a: 0\nb: 1\nc: 1\n",
		}, {
			&Foo{A: 0, B: util.Int(0), C: 0},
			"a: 0\nb: 0\nc: 1\n",
		}, {
			&Foo{A: 2, B: util.Int(2), C: 2},
			"a: 2\nb: 2\nc: 2\n",
		}}
		for _, c := range cases {
			cff := NewConfiger()
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

			err := cff.Var(fs, "", c.sample)
			assert.NoError(t, err)

			cf, err := cff.Parse()
			assert.NoError(t, err)

			assert.Equalf(t, c.want, cf.String(), "sample %s", util.YamlStr(c.sample))
		}
	})

}
