// this is a sample custom auth module
package auth

import (
	"context"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/util"
	"k8s.io/klog"
)

const (
	moduleName = "auth"
)

type Config struct {
	Name        string `json:"name"`
	AuthSchemes string `json:"authSchemes"`
}

func (p Config) String() string {
	return util.Prettify(p)
}

type Module struct {
	*Config
	Name string
}

var (
	_module = &Module{Name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_POST_MODULE,
	}}
)

func (p *Module) preStartHook(ops *proc.HookOps, cf *proc.Configer) (err error) {
	popts := ops.Options()

	c := &Config{}
	if err := cf.Read(p.Name, c); err != nil {
		return err
	}
	p.Config = c

	popts = popts.SetAuth(p)

	ops.SetOptions(popts)

	klog.V(10).Infof("auth config: %s", c)
	return
}

func (p *Module) startHook(ops *proc.HookOps, cf *proc.Configer) (err error) {
	return nil
}

func (p *Module) GetFilter(acl string) (restful.FilterFunction, string, error) {
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		ctx := NewContext(req.Request.Context(), p)
		req.Request = req.Request.WithContext(ctx)

		klog.Infof("before %s filter", acl)
		chain.ProcessFilter(req, resp)
		klog.Infof("after %s filter", acl)
	}, acl, nil
}

func (p *Module) IsAdmin(token openapi.Token) bool {
	return false
}

type BaseClient struct{}

func (p BaseClient) GetId() string          { return "" }
func (p BaseClient) GetSecret() string      { return "" }
func (p BaseClient) GetRedirectUri() string { return "" }
func (p *Module) SsoClient() proc.Client    { return &BaseClient{} }
func (p *Module) Access(req *restful.Request, resp *restful.Response, mustLogin bool, chain *restful.FilterChain, handles ...func(openapi.Token) error) {
}
func (p *Module) WsAccess(req *restful.Request, resp *restful.Response, mustLogin bool, chain *restful.FilterChain, handles ...func(openapi.Token) error) {
}
func (p *Module) GetAndVerifyTokenInfoByApiKey(code *string, peerAddr string) (openapi.Token, error) {
	return &openapi.AnonymousToken{}, nil
}
func (p *Module) GetAndVerifyTokenInfoByBearer(code *string) (openapi.Token, error) {
	return &openapi.AnonymousToken{}, nil
}

type contextKeyT string

var contextKey = contextKeyT("auth")

// NewContext returns a copy of the parent context
// and associates it with an Auth.
func NewContext(ctx context.Context, auth proc.Auth) context.Context {
	return context.WithValue(ctx, contextKey, auth)
}

// FromContext returns the Auth bound to the context, if any.
func FromContext(ctx context.Context) (auth proc.Auth, ok bool) {
	auth, ok = ctx.Value(contextKey).(proc.Auth)
	return
}

func init() {
	proc.RegisterHooks(hookOps)
}
