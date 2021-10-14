package configer

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
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

	_, err := New(WithValueFile("conf.yml"))
	assert.NoError(t, err)
}
func TestConfigWithConfig(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	type Bar struct {
		Foo  Foo `json:"foo"`
		Foo2 Foo `json:",inline"`
	}
	v := Bar{Foo{2}, Foo{3}}

	{
		c, err := New(WithConfig("foo", v.Foo))
		assert.NoError(t, err)

		var got Bar
		err = c.Read("foo", &got.Foo)
		assert.NoError(t, err)
		assert.Equalf(t, v.Foo, got.Foo, "configer read \"foo\"")
	}

	{
		c, err := New(WithConfig("", v))
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
		Foo2 Foo `json:",inline"`
	}
	v := Bar{Foo{2}, Foo{3}}

	{
		c, _ := New(WithConfig("foo", v.Foo))
		c.Set("foo", Foo{20})

		var got Bar
		c.Read("foo", &got.Foo)
		assert.Equalf(t, 20, got.Foo.A, "configer read \"foo\"")

		c.Set("foo.a", 200)
		c.Read("foo", &got.Foo)
		assert.Equalf(t, 200, got.Foo.A, "configer read \"foo.a\"")
	}

	{
		c, _ := New(WithConfig("a", v.Foo2))
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

	config, _ := New(WithValueFile("conf.yml"))

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

	config, _ := New(WithValueFile("conf.yml"))

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

	conf, _ := New(WithDefaultYaml("", yml))
	for _, c := range cases {
		if got := util.GetType(conf.GetRaw(c.path)); got != c.want {
			assert.Equalf(t, c.want, got, "data %+v", conf.data)
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

	conf, _ := New(WithDefaultYaml("", yml))
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

	conf, _ := New(WithDefaultYaml("", yml))
	conf2 := conf.GetConfiger("ctrl.auth")
	err := conf2.Read("google", &got)
	if err != nil {
		t.Fatalf("error %s", err)
	}
	assert.Equalf(t, want, got, "configer read %s", conf2)
}

func TestConfigWithBase(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"base.yml", "foo1: base1\nfoo3: base3"},
		{"conf.yml", "foo1: conf1\nfoo2: conf2"},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	cf, _ := New(WithValueFile("base.yml", "conf.yml"))

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

func TestConfigWithBase2(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"conf.yml", "foo:\n  foo1: conf1\n  foo2: conf2"},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	cf, _ := New(
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

func TestConfigWithValueFile(t *testing.T) {
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

	configerOptions.valueFiles = []string{"base.yml", "conf.yml", "v1.yml", "v2.yml"}
	configerOptions.values = []string{"e=v2_e", "f1=f1,f2=f2"}
	configerOptions.stringValues = []string{"sv1=sv1,sv2=sv2"}
	cf, err := New()
	defer func() { configerOptions = newOptions() }()

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

	opts2 := &TagOpts{
		Name:    "A",
		Json:    "a",
		Flag:    []string{"test-a"},
		Env:     "TEST_A",
		Default: "default-a",
	}
	getter := func(fieldName string) *TagOpts {
		switch fieldName {
		case "A":
			return opts2
		default:
			return nil
		}
	}

	var cases = []struct {
		tagOptsGetter func(string) *TagOpts
		want          *TagOpts
	}{
		{nil, &TagOpts{
			Name:    "A",
			Json:    "a",
			Flag:    []string{"test-a"},
			Env:     "TEST_A",
			Default: "default-a",
		}},
		{getter, opts2},
	}

	for _, c := range cases {
		teardown()

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		err := AddConfigs(fs, "", &Foo{})
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

func TestConfigerPriority(t *testing.T) {
	type Foo struct {
		A string `json:"a" flag:"test-a" env:"TEST_A" default:"default-a"`
	}
	dir := createTestDir([]templateFile{{"base.yml", `a: base_a`}})
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	var cases = []struct {
		flag string
		env  string
		file string
		want interface{}
	}{
		{"flag-a", "", "", "flag-a"},
		{"flag-a", "env-a", "", "flag-a"},
		{"flag-a", "env-a", "file-a", "flag-a"},
		{"", "env-a", "", "env-a"},
		{"", "env-a", "file-a", "file-a"},
		{"", "", "file-a", "file-a"},
		{"", "", "", "default-a"},
	}

	for _, c := range cases {
		teardown()

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		SetOptions(true, false, 5, fs)
		err := AddConfigs(fs, "", &Foo{})
		assert.NoError(t, err)

		if c.flag != "" {
			fs.VisitAll(func(flag *pflag.Flag) {
				if flag.Name == "test-a" {
					flag.Value.Set(c.flag)
					flag.Changed = true
				}
			})
		}
		if c.env != "" {
			os.Setenv("TEST_A", c.env)
		} else {
			os.Unsetenv("TEST_A")
		}

		if c.file != "" {
			ioutil.WriteFile(filepath.Join(dir, "base.yml"), []byte("a: "+c.file), 0666)
		} else {
			ioutil.WriteFile(filepath.Join(dir, "base.yml"), []byte("#"), 0666)
		}

		configerOptions.valueFiles = []string{"base.yml"}
		cf, err := New()
		assert.NoError(t, err)

		assert.Equalf(t, c.want, cf.GetRaw("a"), "flag [%s] env [%s] file [%s] config [%s]", c.flag, c.env, c.file, cf)
	}
}

func TestConfigerDef(t *testing.T) {
	type Foo struct {
		A string `json:"a" default:"default-a"`
	}
	type Bar struct {
		Foo *Foo `json:"foo"`
	}

	teardown()

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	err := AddConfigs(fs, "bar", &Bar{})
	cf, err := New()
	assert.NoError(t, err)

	assert.Equalf(t, "default-a", cf.GetRaw("bar.foo.a"), "config [%s]", cf)
}

func teardown() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	configerOptions = newOptions()
}
