package orm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithSelect(t *testing.T) {
	cases := []struct {
		selector []string
		expect   string
	}{
		{[]string{"x=y"}, "x=y"},
		{[]string{"", "x=y"}, "x=y"},
		{[]string{"x=y", "z=w"}, "x=y,z=w"},
	}

	for _, c := range cases {
		opts, err := NewOptions(WithSelector(c.selector...))
		require.NoError(t, err)
		require.Equal(t, c.expect, opts._selector.String())
	}

	for _, c := range cases {
		o := []QueryOption{}
		for _, v := range c.selector {
			o = append(o, WithSelector(v))
		}
		opts, err := NewOptions(o...)
		require.NoError(t, err)
		require.Equal(t, c.expect, opts._selector.String())
	}
}

func TestWithSelectf(t *testing.T) {
	cases := []struct {
		format string
		args   []interface{}
		expect string
	}{{
		"x=%s,z>%d",
		[]interface{}{"y", 2},
		"x=y,z>2",
	}, {
		"x in (%s), y in (%s)",
		[]interface{}{[]int{1, 2, 3}, []string{"b", "c", "a"}},
		"x in (1,2,3),y in (a,b,c)",
	}}

	for _, c := range cases {
		opts, err := NewOptions(WithSelectorf(c.format, c.args...))
		require.NoError(t, err)
		require.Equal(t, c.expect, opts._selector.String())
	}
}
