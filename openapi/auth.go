package openapi

import "github.com/emicklei/go-restful"

const (
	reqToken = "req-openapi-token"
)

var (
	Scopes = map[string]string{}
)

func AddScope(scope, description string) {
	Scopes[scope] = description
}

type Token interface {
	GetTokenName() string
	GetUserName() string
	HasScope(scope string) bool
}

func TokenFrom(r *restful.Request) (Token, bool) {
	token, ok := r.Attribute(reqToken).(Token)
	return token, ok
}

func WithToken(r *restful.Request, token Token) *restful.Request {
	r.SetAttribute(reqToken, token)
	return r
}

type AnonymousToken struct{}

func (p AnonymousToken) GetTokenName() string       { return "null" }
func (p AnonymousToken) GetUserName() string        { return "anonymous" }
func (p AnonymousToken) HasScope(scope string) bool { return false }
