package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStructMd5(t *testing.T) {
	type foo struct {
		A int
		B *int
		C string
		D *string
		E []string
	}
	s, err := StructMd5(foo{
		A: 1,
		B: Int(2),
		C: "3",
		D: String("4"),
		E: []string{"5"},
	})
	t.Logf("md5Struct(a) %s %v", s, err)
}

func TestHashPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"1234", "81/dc/1234"},
		{"12/34", "83/54/12/34"},
		{"///etc/nginx/conf.d///", "a6/a7/etc/nginx/conf.d"},
	}

	for _, c := range cases {
		assert.Equal(t, c.want, HashPath(c.path, 2))
	}
}
