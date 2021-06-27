package configer

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
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

	config, err := New("conf.yml")
	if err != nil {
		t.Error(t)
	}

	err = config.Prepare()
	if err != nil {
		t.Error(t)
	}

	klog.V(3).Infof("%s", config)
}

func TestTimeDuration(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"conf.yml", `t1: 1s
t2: 1m
t3: 1h
t4: 24h30m30s`},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	config, _ := New("conf.yml")
	config.Prepare()

	var cases = []struct {
		path string
		want time.Duration
	}{
		{"t1", time.Second},
		{"t2", time.Minute},
		{"t3", time.Hour},
		{"t4", 24*time.Hour + 30*time.Minute + 30*time.Second},
	}
	var got time.Duration

	for _, c := range cases {
		err := config.ReadYaml(c.path, &got)
		require.Emptyf(t, err, "config.Readyaml(%s)", c.path)
		require.Equalf(t, c.want, got, "configer read %s", c.path)
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

	config, _ := New("conf.yml")
	config.Prepare()

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
		if got := config.GetRaw(c.path); got != c.want {
			t.Errorf("config.GetRaw(%s) expected %#v got %#v", c.path, c.want, got)
		}
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

	config, _ := New("conf.yml")
	config.Prepare()

	var (
		got  []string
		path = "fooo.foos"
		want = []string{"1", "2"}
	)

	if err := config.Read(path, &got); err != nil {
		t.Error(err)
	}

	require.Equalf(t, want, got, "configer read %s", path)
}

func TestRawType(t *testing.T) {
	yml := `
ctrl:
  auth:
    google:
      client_id: "781171109477-10tu51e8bs1s677na46oct6hdefpntpu.apps.googleusercontent.com"
      client_secret: xpEoBFqkmI3KVN9pHt2VW-eN
      redirect_url: http://auth.dev.pt.xiaomi.com/v1.0/auth/callback/google
`

	var cases = []struct {
		path string
		want string
	}{
		{"ctrl", "map[string]interface {}"},
		{"ctrl.auth", "map[string]interface {}"},
		{"ctrl.auth.google", "map[string]interface {}"},
		{"ctrl.auth.google.client_id", "string"},
	}

	conf, _ := newConfiger([]byte(yml))

	for _, c := range cases {
		if got := util.GetType(conf.GetRaw(c.path)); got != c.want {
			t.Fatalf("GetType(conf.GetRaw(%s)) got %s want %s",
				c.path, got, c.want)
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
      redirect_url: http://auth.dev.pt.xiaomi.com/v1.0/auth/callback/google
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

	conf, _ := newConfiger([]byte(yml))
	for _, c := range cases {
		cf := ToConfiger(conf.GetRaw(c.path1))
		if cf == nil {
			t.Fatalf("get %s error", c.path1)
		}

		if got := util.GetType(cf.GetRaw(c.path2)); got != c.want {
			t.Fatalf("GetType(ToConfiger(conf.GetRaw(%s)).GetRaw(%s)) got %s want %s",
				c.path1, c.path2, got, c.want)
		} else {
			klog.V(3).Infof("%s %s got value %v", c.path1, c.path2, cf.GetRaw(c.path2))
		}
	}
}

func TestToConfigerAsYaml(t *testing.T) {
	yml := `
ctrl:
  auth:
    google:
      client_id: "781171109477-10tu51e8bs1s677na46oct6hdefpntpu.apps.googleusercontent.com"
      client_secret: xpEoBFqkmI3KVN9pHt2VW-eN
      redirect_url: http://auth.dev.pt.xiaomi.com/v1.0/auth/callback/google
`

	type auth struct {
		ClientId     string `yaml:"client_id"`
		ClientSecret string `yaml:"client_secret"`
		RedirectUrl  string `yaml:"redirect_url"`
	}

	want := auth{
		ClientId:     "781171109477-10tu51e8bs1s677na46oct6hdefpntpu.apps.googleusercontent.com",
		ClientSecret: "xpEoBFqkmI3KVN9pHt2VW-eN",
		RedirectUrl:  "http://auth.dev.pt.xiaomi.com/v1.0/auth/callback/google",
	}

	var got auth

	conf, _ := newConfiger([]byte(yml))
	cf := ToConfiger(conf.GetRaw("ctrl.auth"))
	err := cf.ReadYaml("google", &got)
	if err != nil {
		t.Fatalf("error %s", err)
	}
	require.Equalf(t, want, got, "configer read yaml")
}

func TestConfigWithBase(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"base.yml", "foo1: base1\nfoo3: base3"},
		{"conf.yml", "foo1: conf1\nfoo2: conf2"},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	cf, _ := New("conf.yml", WithBaseFile("base.yml"))
	cf.Prepare()

	var cases = []struct {
		path string
		want interface{}
	}{
		{"foo1", "conf1"},
		{"foo2", "conf2"},
		{"foo3", "base3"},
	}

	for _, c := range cases {
		if got := cf.GetRaw(c.path); got != c.want {
			t.Errorf("config.GetRaw(%s) expected %#v got %#v", c.path, c.want, got)
		}
	}
}

func TestConfigWithBase2(t *testing.T) {
	dir := createTestDir([]templateFile{
		{"conf.yml", "foo:\n  foo1: conf1\n  foo2: conf2"},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)
	os.Chdir(dir)

	cf, _ := New("conf.yml", WithBaseBytes2("foo", "foo1: base1"), WithBaseBytes2("foo", "foo3: base3"))
	cf.Prepare()

	var cases = []struct {
		path string
		want interface{}
	}{
		{"foo.foo1", "conf1"},
		{"foo.foo2", "conf2"},
		{"foo.foo3", "base3"},
	}

	for _, c := range cases {
		if got := cf.GetRaw(c.path); got != c.want {
			t.Errorf("config.GetRaw(%s) expected %#v got %#v", c.path, c.want, got)
		}
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

	cf, _ := New("conf.yml",
		WithBaseFile("base.yml"),
		WithValueFile("v1.yml"),
		WithValueFile("v2.yml"),
		WithValues("e=v2_e"),
		WithValues("f1=f1,f2=f2"),
		WithStringValues("sv1=sv1,sv2=sv2"),
	)
	err := cf.Prepare()
	if err != nil {
		t.Errorf("prepare err %s", err)
	}

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
		if got := cf.GetRaw(c.path); got != c.want {
			t.Errorf("config.GetRaw(%s) expected %#v got %#v",
				c.path, c.want, got)
		}
	}
}
