package template

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"text/template"
)

func TestFuncs(t *testing.T) {
	os.Setenv("FOO", "bar")
	cases := map[string]string{
		`{{hello}}`:                                                  "hello!",
		`{{env "FOO"}}`:                                              "bar",
		`{{expandenv "hello ${FOO}"}}`:                               "hello bar",
		`{{expandenv "hello $FOO"}}`:                                 "hello bar",
		`{{base "foo/bar"}}`:                                         "bar",
		`{{dir "foo/bar/baz"}}`:                                      "foo/bar",
		`{{clean "/foo/../foo/../bar"}}`:                             "/bar",
		`{{clean "//foo///bar/"}}`:                                   "/foo/bar",
		`{{ext "/foo/bar/baz.txt"}}`:                                 ".txt",
		`{{isAbs "/foo/bar/baz"}}`:                                   "true",
		`{{isAbs "foo/bar/baz"}}`:                                    "false",
		`{{quote "a" "b" "c"}}`:                                      `"a" "b" "c"`,
		`{{quote "\"a\"" "b" "c"}}`:                                  `"\"a\"" "b" "c"`,
		`{{quote 1 2 3 }}`:                                           `"1" "2" "3"`,
		`{{squote "a" "b" "c"}}`:                                     `'a' 'b' 'c'`,
		`{{squote 1 2 3 }}`:                                          `'1' '2' '3'`,
		`{{if contains "cat" "fair catch"}}1{{end}}`:                 "1",
		`{{if hasPrefix "cat" "catch"}}1{{end}}`:                     "1",
		`{{if hasSuffix "cat" "ducat"}}1{{end}}`:                     "1",
		`{{trim "   5.00   "}}`:                                      "5.00",
		`{{trimAll "$" "$5.00$"}}`:                                   "5.00",
		`{{trimPrefix "$" "$5.00"}}`:                                 "5.00",
		`{{trimSuffix "$" "5.00$"}}`:                                 "5.00",
		`{{$v := "foo$bar$baz" | split "$"}}{{$v._0}}`:               "foo",
		`{{toString 1 | kindOf }}`:                                   "string",
		`{{$s := list 1 2 3 | toStrings }}{{ index $s 1 | kindOf }}`: "string",
		`{{list "a" "b" "c" | join "-" }}`:                           "a-b-c",
		`{{list 1 2 3 | join "-" }}`:                                 "1-2-3",
		`{{list "c" "a" "b" | sortAlpha | join "" }}`:                "abc",
		`{{list 2 1 4 3 | sortAlpha | join "" }}`:                    "1234",
		`{{b64enc "hello,world"}}`:                                   "aGVsbG8sd29ybGQ=",
		`{{b64dec "aGVsbG8sd29ybGQ="}}`:                              "hello,world",
		`{{$b := "b"}}{{"c" | cat "a" $b}}`:                          "a b c",
		`{{indent 4 "a\nb\nc"}}`:                                     "    a\n    b\n    c",
		`{{nindent 4 "a\nb\nc"}}`:                                    "\n    a\n    b\n    c",
		`{{"a b c" | replace " " "-"}}`:                              "a-b-c",
		`{{ timeOf "3600" }}`:                                        "3600s",
		`{{ timeOf "3600s" }}`:                                       "3600s",
		`{{ timeOf "60m" }}`:                                         "3600s",
		`{{ timeOf "1h" }}`:                                          "3600s",
		`{{ timeOf "1d" }}`:                                          "86400s",
		`{{ sizeOf "1k" }}`:                                          "1024",
		`{{ sizeOf "1m" }}`:                                          "1048576",
		`{{if env "FOO" | eq "bar"}}1{{end}}`:                        "1",
		`{{repeat 3 "FOO" | join ","}}`:                              `FOO,FOO,FOO`,
		`{{$s := list 1 2 3 }}{{ if last 2 $s}}1{{end}}`:             "1",
	}
	for tpl, want := range cases {
		if err := runt(tpl, want); err != nil {
			t.Error(err)
		}
	}
}

// runt runs a template and checks that the output exactly matches the expected string.
func runt(tpl, expect string) error {
	return runtv(tpl, expect, map[string]string{})
}

// runtv takes a template, and expected return, and values for substitution.
//
// It runs the template and verifies that the output is an exact match.
func runtv(tpl, expect string, vars interface{}) error {
	t := template.Must(template.New("test").Funcs(FuncMap).Parse(tpl))
	var b bytes.Buffer
	err := t.Execute(&b, vars)
	if err != nil {
		return err
	}
	if expect != b.String() {
		return fmt.Errorf("%s Expected '%s', got '%s'", tpl, expect, b.String())
	}
	return nil
}

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

func TestReadFileWithInclude(t *testing.T) {
	// Here we create a temporary directory and populate it with our sample
	// template definition files; usually the template files would already
	// exist in some location known to the program.
	dir := createTestDir([]templateFile{
		{"1.conf", `include "2.conf"`},
		{"2.conf", "2\n  include 3.*.conf"},
		{"3.1.conf", "3.1"},
		{"3.2.conf", "3.2\n"},
		{"3.3.conf", "3.3\n  include 4.conf\n"},
		{"4.conf", "4"},
	})
	// Clean up after the test; another quirk of running as an example.
	defer os.RemoveAll(dir)

	expect := "2\n  3.1\n  3.2\n  3.3\n    4\n"

	os.Chdir(dir)

	b, e := ReadFileWithInclude("1.conf")
	if e != nil {
		t.Error(e)
	}

	if expect != string(b) {
		t.Errorf("readFileWithInclude Expected '%s', got '%s'",
			expect, string(b))
	}
}
