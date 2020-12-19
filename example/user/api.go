// this is a sample echo rest api module
package user

import "github.com/yubo/golib/openapi"

type User struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type CreateUserInput struct {
	Name  string `param:"data" name:"name"`
	Phone string `param:"data" name:"phone"`
}

type CreateUserOutput struct {
	openapi.RespStatus
	Data openapi.RespID `json:"data"`
}

type CreateUsersOutput struct {
	openapi.RespStatus
	Data openapi.RespTotal `json:"data"`
}

type GetUsersInput struct {
	Query *string `param:"query" name:"query" description:"query user"`
	Count bool    `param:"query" name:"count" description:"just response total count"`
}

func (p *GetUsersInput) Validate() error {
	return nil
}

type GetUsersOutput struct {
	openapi.RespStatus
	Data struct {
		Total int     `json:"total"`
		List  []*User `json:"list"`
	} `json:'data'`
}

type GetUserInput struct {
	Name string `param:"path" name:"user-name"`
}

func (p *GetUserInput) Validate() error {
	return nil
}

type GetUserOutput struct {
	openapi.RespStatus
	Data *User `json:"data"`
}

type UpdateUserInput struct {
	Name  string `param:"path" name:"user-name"`
	Phone string `param:"data" json:"phone"`
}

type UpdateUserOutput struct {
	openapi.RespStatus
	Data *User `json:"data"`
}

type DeleteUserInput struct {
	Name string `param:"path" name:"user-name"`
}

type DeleteUserOutput struct {
	openapi.RespStatus
	Data *User `json:"data"`
}
