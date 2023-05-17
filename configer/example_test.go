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
	Users     []User `json:"users"`
	GroupName string `json:"groupName" flag:"group-name" default:"dev" description:"group name"`
}

// go test -run ExampleParse
func ExampleParse() {
	config := config{}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	cff := configer.New()
	if err := cff.Var(fs, "", &config); err != nil {
		panic(err)
	}

	cff.AddFlags(fs)

	if err := fs.Parse([]string{
		"--group-name", "devops",
		"--set", "users[0].name=tom,users[1].name=steve",
	}); err != nil {
		panic(err)
	}
	c, err := cff.Parse()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s", c.String())
	// Output:
	// groupName: devops
	// users:
	// - name: tom
	// - name: steve
}
