package api

import (
	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/util"
)

type Version struct {
	Version   string `json:"version,omitempty"`
	Release   string `json:"release,omitempty"`
	Git       string `json:"git,omitempty"`
	Go        string `json:"go,omitempty"`
	Os        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Builder   string `json:"builder,omitempty"`
	BuildTime int64  `json:"buildTime,omitempty" out:",date"`
}

func (p Version) String() string {
	return util.Prettify(p)
}

func WithTokenInfo(r *restful.Request, token *AuthToken) *restful.Request {
	r.SetAttribute(AuthTokenKey, token)
	return r
}

func TokenInfoFrom(r *restful.Request) (*AuthToken, bool) {
	token, ok := r.Attribute(AuthTokenKey).(*AuthToken)
	return token, ok
}

func WithReqEntityConn(r *restful.Request, entity interface{}) *restful.Request {
	r.SetAttribute(ReqEntityKey, entity)
	return r
}

func ReqEntityFrom(r *restful.Request) (interface{}, bool) {
	entity := r.Attribute(ReqEntityKey)
	return entity, entity != nil
}
