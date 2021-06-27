package configer_test

import (
	"fmt"

	"github.com/yubo/golib/configer"
)

func ExampleNew() {
	c, err := configer.New()
	fmt.Println(c, err)
}
