package configer_test

import (
	"fmt"

	"github.com/yubo/golib/configer"
)

type User struct {
	Name string `flag:"name"`
}

func ExampleNewConfiger() {

	c, err := configer.NewFactory().NewConfiger()
	fmt.Printf("%s", c)
	fmt.Printf("%v", err)
	// Output:
	// {}
	// <nil>
}
