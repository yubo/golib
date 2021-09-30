package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenCountQuery(t *testing.T) {
	cases := []struct {
		query      string
		countQuery string
		err        bool
	}{{
		"select * from user c left join b ",
		"select count(*) from user c left join b ",
		false,
	}, {
		"select a, b, c from (select a.name from a)",
		"select count(*) from (select a.name from a)",
		false,
	}, {
		"SELECT a.a1, b.*, c.name_c FROM user",
		"select count(*) from user",
		false,
	}, {
		"a SELECT a.a1, b.*, c.name_c FROM user",
		"",
		true,
	}}

	for _, c := range cases {
		got, err := genCountQuery(c.query)
		assert.Equal(t, c.countQuery, got, c.query)
		assert.Equal(t, c.err, err != nil, "err != nil")
	}
}
