package configer_test

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/yubo/golib/configer"
)

type User struct {
	Name string `json:"name" flag:"name" default:"tom" description:"user name"`
}

type config struct {
	Users []User `json:"users"`
}

// go test -run ExampleParse
func ExampleParse() {
	config := &config{
		Users: []User{
			{Name: "json"},
		},
	}
	cfger := configer.New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfger.Var(fs, "", &config)

	c, err := cfger.Parse()

	fmt.Printf("%s", c)
	fmt.Printf("%v", err)
	// Output:
	// <nil>
}
