package openapi

import (
	"fmt"
	"strings"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/go-openapi/spec"
	"k8s.io/klog"
)

const (
	reqToken              = "req-openapi-token"
	SecurityDefinitionKey = "OAPI_SECURITY_DEFINITION"
	NativeClientID        = "native-client-id"
	NativeClientSecret    = "native-client-secret"

	// scope
	OauthScopeNil           = "nil"
	OauthScopeRead          = "read"
	OauthScopeWrite         = "write"
	OauthScopeExec          = "exec"
	OauthScopeWork          = "work"
	OauthScopeRoot          = "root"
	OauthScopeUpload        = "upload"
	OauthScopeOverwrite     = "overwrite"
	OauthScopeEdit          = "edit"
	OauthScopeAdmin         = "admin"
	OauthScopeReadSecret    = "read:secret"
	OauthScopeWriteSecret   = "write:secret"
	OauthScopeWriteRegistry = "write:registry"
	OauthScopeReadSso       = "read:sso"
	OauthScopeWriteSso      = "write:sso"
)

type SecurityType string

const (
	SecurityTypeBase        SecurityType = "base"
	SecurityTypeApiKey      SecurityType = "apiKey"
	SecurityTypeOAuth2      SecurityType = "oauth2"
	SecurityTypeImplicit    SecurityType = "implicity"
	SecurityTypePassword    SecurityType = "password"
	SecurityTypeApplication SecurityType = "application"
	SecurityTypeAccessCode  SecurityType = "accessCode"
)

var (
	Scopes          = map[string]string{}
	securitySchemes = map[string]*spec.SecurityScheme{}
	swaggerTags     = []spec.Tag{}
)

func ScopeRegister(scope, description string) {
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

func SecurityScheme(ssoAddr string) *spec.SecurityScheme {
	return spec.OAuth2AccessToken(ssoAddr+"/o/oauth2/authorize", ssoAddr+"/o/oauth2/token")
}

func SecuritySchemeRegister(sType SecurityType, s *spec.SecurityScheme) error {
	if securitySchemes[string(sType)] != nil {
		return fmt.Errorf("SecuritySchemeRegister %s exists", sType)
	}

	for scope, desc := range Scopes {
		s.AddScope(scope, desc)
	}

	securitySchemes[string(sType)] = s
	return nil
}

func SwaggerTagsRegister(tags ...spec.Tag) {
	swaggerTags = append(swaggerTags, tags...)
}

func SwaggerTagRegister(name, desc string) {
	for _, v := range swaggerTags {
		if v.Name == name {
			klog.Warningf("SwaggerTagRegister %s has been added", name)
			return
		}
	}

	swaggerTags = append(swaggerTags, spec.Tag{TagProps: spec.TagProps{
		Name:        name,
		Description: desc,
	}})
}

type httpServer interface {
	RegisteredWebServices() []*restful.WebService
	Add(service *restful.WebService) *restful.Container
}

func InstallApiDocs(http httpServer, infoProps spec.InfoProps) {
	wss := http.RegisteredWebServices()
	ws := restfulspec.NewOpenAPIService(restfulspec.Config{
		// you control what services are visible
		WebServices:                   wss,
		APIPath:                       "/apidocs.json",
		PostBuildSwaggerObjectHandler: getSwaggerHandler(wss, infoProps),
	})
	http.Add(ws)
}

func getSwaggerHandler(wss []*restful.WebService, infoProps spec.InfoProps) func(*spec.Swagger) {
	return func(s *spec.Swagger) {
		s.Info = &spec.Info{InfoProps: infoProps}
		s.Tags = swaggerTags
		s.SecurityDefinitions = securitySchemes

		if len(s.SecurityDefinitions) == 0 {
			return
		}

		// loop through all registerd web services
		for _, ws := range wss {
			for _, route := range ws.Routes() {

				// route metadata for a SecurityDefinition
				secdefn, ok := route.Metadata[SecurityDefinitionKey]
				if !ok {
					continue
				}

				scope, ok := secdefn.(string)
				if !ok {
					continue
				}

				// grab path and path item in openapi spec
				path, err := s.Paths.JSONLookup(strings.TrimRight(route.Path, "/"))
				if err != nil {
					klog.Warningf("skipping Security openapi spec for %s:%s, %s", route.Method, route.Path, err.Error())
					path, err = s.Paths.JSONLookup(route.Path[:len(route.Path)-1])
					if err != nil {
						klog.Warningf("skipping Security openapi spec for %s:%s, %s", route.Method, route.Path[:len(route.Path)-1], err.Error())
						continue
					}
				}
				pItem := path.(*spec.PathItem)

				// Update respective path Option based on method
				var pOption *spec.Operation
				switch method := strings.ToLower(route.Method); method {
				case "get":
					pOption = pItem.Get
				case "post":
					pOption = pItem.Post
				case "patch":
					pOption = pItem.Patch
				case "delete":
					pOption = pItem.Delete
				case "put":
					pOption = pItem.Put
				case "head":
					pOption = pItem.Head
				case "options":
					pOption = pItem.Options
				default:
					// unsupported method
					klog.Warningf("skipping Security openapi spec for %s:%s, unsupported method '%s'", route.Method, route.Path, route.Method)
					continue
				}

				// update the pOption with security entry
				for key, _ := range s.SecurityDefinitions {
					pOption.SecuredWith(key, scope)
				}
			}
		}

	}

}
