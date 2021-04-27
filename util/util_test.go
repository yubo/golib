package util

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSrcAddrs(t *testing.T) {
	cases := []struct {
		addrs []net.IPAddr
	}{
		{
			addrs: []net.IPAddr{
				{IP: net.IPv4(127, 0, 0, 1)},
				{IP: net.IPv4(8, 8, 8, 8)},
			},
		},
	}

	for _, c := range cases {
		got := SrcAddrs(c.addrs)
		for i, _ := range c.addrs {
			t.Logf("%s -> %s", got[i], c.addrs[i].IP)
		}
	}

	for _, c := range cases {
		for _, addr := range c.addrs {
			got := SrcAddr(addr)
			t.Logf("%s -> %s", got, addr.IP)
		}
	}

}

func TestStrings(t *testing.T) {
	{
		cases := [][2]string{
			{"a\nb", "a"},
			{"a\n", "a"},
			{"\n", ""},
			{"", ""},
		}
		for i, c := range cases {
			if got := FirstLine(c[0]); got != c[1] {
				t.Errorf("FirstLine case%02d got %s want %s\n", i, got, c[1])
			}
		}
	}
	{
		cases := [][2]string{
			{"a\nb", "b"},
			{"a\n", "a"},
			{"\n", ""},
			{"", ""},
		}
		for i, c := range cases {
			if got := LastLine(c[0]); got != c[1] {
				t.Errorf("LastLine case%02d got %s want %s\n", i, got, c[1])
			}
		}
	}

}

func TestStructCopy(t *testing.T) {
	type S1 struct {
		X1 int
		X2 int
		X3 int
	}
	type S2 struct {
		X1 int
	}
	type S3 struct {
		X2 int
		X3 int
	}
	type S4 struct {
		S2
		S3
	}
	{
		foo := struct {
			X, Y int
		}{1, 1}
		bar := struct {
			Y, Z int
		}{2, 2}

		err := StructCopy(&foo, bar)
		if err != nil {
			t.Error(err)
		}
		if !(foo.X == 1 && foo.Y == 2) {
			t.Errorf("expected foo.x == 1 && foo.y == 2, got %v", foo)
		}
	}
	{
		foo := S4{}
		bar := S1{2, 2, 2}

		err := StructCopy(&foo, bar)
		if err != nil {
			t.Error(err)
		}
		if !(foo.X1 == 2 && foo.X2 == 2 && foo.X3 == 2) {
			t.Errorf("expected foo[2,2,2], got %v", foo)
		}
	}

}

func TestAtoi64(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"123", 123},
		{"0123", 123},
		{"a123", 0},
		{"123a", 0},
	}

	for _, c := range cases {
		if got := Atoi64(c.in); got != c.want {
			t.Errorf("Atoi64(%s) got %d expected %d",
				c.in, got, c.want)
		}
	}
}

func TestBash(t *testing.T) {
	buff := []byte("echo hello\necho world")
	out, err := Bash(buff)
	if err != nil {
		t.Error(err)
		return
	}

	require.Equal(t, "hello\nworld\n", string(out))
}

func TestStrings2MapString(t *testing.T) {
	cases := []struct {
		in   []string
		want map[string]string
	}{
		{
			[]string{"a=1", "a=2"},
			map[string]string{"a": "2"},
		},
		{
			[]string{"a=", "b=2", "A=3"},
			map[string]string{"a": "", "b": "2", "A": "3"},
		},
	}

	for _, c := range cases {
		got := Strings2MapString(c.in)
		require.Equal(t, c.want, got)
	}
}

func _ss2map(in []string) map[string]bool {
	ret := map[string]bool{}
	for _, v := range in {
		ret[v] = true
	}
	return ret
}

func TestTitleCasedName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"foo_bar", "FooBar"},
		{"foo-bar", "FooBar"},
	}

	for _, c := range cases {
		require.Equal(t, c.want, TitleCasedName(c.in, false))
	}

}

func TestSnakeCasedName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"FooBar", "foo_bar"},
		{"FooBar", "foo_bar"},
		{"FooBAR", "foo_bar"},
		{"fooBAR", "foo_bar"},
		{"fooBar", "foo_bar"},
	}

	for _, c := range cases {
		require.Equal(t, c.want, SnakeCasedName(c.in))
	}

}

func TestCheckName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"a", true},
		{"abc", true},
		{"a-b-c", true},
		{"a.com", true},
		{"a\\.com", false},
		{"a/com", false},
		{"a+com", false},
		{"_a", false},
		{"1a", false},
		{"-a", false},
		{"12341234", false},
	}

	for _, c := range cases {
		err := CheckName(c.name)
		require.Equalf(t, err == nil == c.valid,
			true, "name %s want %v got %s", c.name, c.valid, err)
	}
}

func TestCheckPath(t *testing.T) {
	cases := []struct {
		path  string
		valid bool
	}{
		{"a", true},
		{"a/-b-c", false},
		{"a/b/", false},
		{"a\\.com", false},
		{"abc", true},
		{"abc/a1/a-/a_", true},
		{"1a", false},
		{"_a", true},
	}

	for _, c := range cases {
		err := CheckPath(c.path)
		require.Equalf(t, err == nil == c.valid,
			true, "path %s want %v got %s", c.path, c.valid, err)
	}
}

func TestMakeSlice(t *testing.T) {
	type Foo struct{}
	cases := []struct {
		in   interface{}
		want interface{}
	}{
		{Foo{}, []Foo{}},
		{&Foo{}, []*Foo{}},
	}

	for i, c := range cases {
		got := MakeSlice(c.in)
		require.Equalf(t, c.want, got, "case-%d", i)
	}
}

func TestKvMask(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"a=", "a=***"},
		{"a=1", "a=***"},
		{"=c", ""},
		{"=", ""},
		{"", ""},
	}

	for i, c := range cases {
		got := KvMask(c.in)
		require.Equalf(t, c.want, got, "case-%d", i)
	}
}

func TestSubStr(t *testing.T) {
	cases := []struct {
		in    string
		begin int
		end   int
		want  string
	}{
		{"123456", 0, 2, "12"},
		{"123456", 0, 4, "1234"},
		{"1", 0, 4, "1"},
		{"", 0, 4, ""},
	}

	for i, c := range cases {
		got := SubStr(c.in, c.begin, c.end)
		require.Equalf(t, c.want, got, "%d - SubStr(%s, %d, %d)", i, c.in, c.begin, c.end)
	}

}

func TestSubStr3(t *testing.T) {
	cases := []struct {
		in    string
		begin int
		end   int
		want  string
	}{
		{"1234567890", 3, -3, "123...890"},
		{"", 3, -3, ""},
		{"1234567890", 8, -8, "1234567890"},
		{"1234567890", 20, 20, "1234567890"},
	}

	for i, c := range cases {
		got := SubStr3(c.in, c.begin, c.end)
		require.Equalf(t, c.want, got, "%d - SubStr(%s, %d, %d)", i, c.in, c.begin, c.end)
	}

}
