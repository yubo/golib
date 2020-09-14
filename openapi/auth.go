package openapi

import "github.com/emicklei/go-restful"

const (
	reqToken = "req-openapi-token"
)

type Token interface {
	GetTokenName() string
	GetUserName() string
}

func TokenFrom(r *restful.Request) (Token, bool) {
	token, ok := r.Attribute(reqToken).(Token)
	return token, ok
}

func WithToken(r *restful.Request, token Token) *restful.Request {
	r.SetAttribute(reqToken, token)
	return r
}

type BaseToken struct{}

func (p BaseToken) GetTokenName() string { return "null" }
func (p BaseToken) GetUserName() string  { return "anonymous" }
