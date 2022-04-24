package orm

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/util"
)

func TestGetfields(t *testing.T) {
	driver := &mysql{DBOptions: NewDefaultDBOptions()}

	type test struct {
		Id int
	}
	field := GetFields(&test{}, driver)
	assert.Equal(t, StructFields{
		Fields: []*StructField{{
			Set:            map[string]string{},
			FieldName:      "Id",
			Name:           "id",
			DataType:       "int",
			DriverDataType: "bigint",
			Size:           util.Int64(64),
			Index:          []int{0},
			Type:           reflect.TypeOf(int(0)),
		}},
		nameIndex: map[string]int{
			"id": 0,
		},
	}, field)

}
