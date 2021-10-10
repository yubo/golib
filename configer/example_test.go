package configer_test

import (
	"fmt"
	"strings"

	"github.com/yubo/golib/configer"
)

type User struct {
	Name string `flag:"name"`
}

func ExampleNew() {
	configer.Reset()

	c, err := configer.New()
	fmt.Printf("%s, %v\n", strings.TrimSpace(c.String()), err)
	// Output:
	// {}, <nil>
}
