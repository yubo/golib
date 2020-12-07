package util

import "testing"

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
