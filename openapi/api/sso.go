package api

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc/codes"
)

var (
	ErrInvalClient      = errors.New("Invalid client")
	ErrChkClient        = errors.New("client check failed")
	ErrInvalAccessToken = errors.New("Invalid access token")
	ErrInvalApiKey      = errors.New("Invalid api key")
	ErrInvalRedirect    = errors.New("bad client redriect uri")
	ErrExpToken         = errors.New("token has expired")
	ErrNoPrivateKey     = errors.New("no private key found")
)

// authModule interface
type AuthModule interface {
	ForbiddenAccess(req *restful.Request, resp *restful.Response, chain *restful.FilterChain)
	LoginFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain)
	WsScopeAccess(scope string, req *restful.Request, resp *restful.Response, chain *restful.FilterChain, timeout int64)
	WsAccess(req *restful.Request, resp *restful.Response, chain *restful.FilterChain, timeout int64, handles ...func(*AuthToken) error)
	ScopeAccess(scope string, req *restful.Request, resp *restful.Response, chain *restful.FilterChain)
	ScopeAccessWithOtp(scope string, req *restful.Request, resp *restful.Response, chain *restful.FilterChain)
	Access(req *restful.Request, resp *restful.Response, chain *restful.FilterChain, handles ...func(*AuthToken) error)
	AccessWithOtp(req *restful.Request, resp *restful.Response, chain *restful.FilterChain, handles ...func(*AuthToken) error)
	SsoClient() *Client
	GetAndVerifyTokenInfoByApiKey(code *string, peerAddr string) (*AuthToken, error)
	GetAndVerifyTokenInfoByBearer(code *string, otpVerify bool, otpCode *string) (*AuthToken, error)

	// apidoc
	AddScope(scope, description string)
}

type RshDataToken struct {
	Bearer *string `json:"bearer,omitempty"`
	ApiKey *string `json:"apiKey,omitempty"`
}

/// account  /account/* {{{
type AccountTplInput struct {
	URL      *string
	Redirect *string
	User     *User
	Photo    *string
	Err      error
}

func (p AccountTplInput) String() string {
	return util.Prettify(p)
}

// GetLoginInput  GET /account
type GetLoginInput struct {
	Redirect *string `param:"query" description:"302 redirect url"`
}

func (p GetLoginInput) String() string {
	return util.Prettify(p)
}

// GetLoginInput  POST /account
type PostLoginInput struct {
	Username      *string `param:"data" description:"name of the login user"`
	Password      *string `param:"data,password" description:"password of the login user"`
	Redirect      *string `param:"data" description:"302 redirect url"`
	Quiet         *bool   `param:"data" description:"Do not display the page"`
	SAMLRequest   *string `param:"query" name:"SAMLRequest" description:"saml request"`
	RelayState    *string `param:"query" name:"RelayState" description:"saml relay state"`
	RequestBuffer []byte  `param:"-"`
}

func (p PostLoginInput) String() string {
	return util.Prettify(p)
}

// GetLogoutInput GET /account/logout
type GetLogoutInput struct {
	Redirect *string `param:"query" description:"302 location url if logout success"`
}

func (p GetLogoutInput) String() string {
	return util.Prettify(p)
}

// }}}

// OAuth /oauth/* {{{

// AuthorizatioinBasicInput
type AuthorizatioinBasicInput struct {
	Authorization *string `param:"header" name:"Authorization" description:"Basic Authentication with the provided username(client_id) and password(client_secret)(http://www.ietf.org/rfc/rfc2617.txt)"`
}

func (p AuthorizatioinBasicInput) String() string {
	return util.Prettify(p)
}

// OAuthAuthorizeInput GET /oauth/authorize input (https://tools.ietf.org/html/rfc6749#section-4.1.1)
type OAuthAuthorizeInput struct {
	ResponseType        *string `param:"query" name:"response_type" description:"code/token"`
	ClientId            *string `param:"query" name:"client_id" description:"The client identifier"`
	RedirectUri         *string `param:"query" name:"redirect_uri" description:"The redirection endpoint URI MUST be an absolute URI"`
	State               *string `param:"query" name:"state" description:"state"`
	Scope               *string `param:"query" name:"scope" description:"scope"`
	CodeChallenge       *string `param:"query" name:"code_challenge" description:"code challenge(https://tools.ietf.org/html/rfc7636)"`
	CodeChallengeMethod *string `param:"query" name:"code_challenge_method" description:"code challenge(https://tools.ietf.org/html/rfc7636#section-4.3)"`
}

// OAuthAuthorizeOutput GET /oauth/authorize output (https://tools.ietf.org/html/rfc6749#section-4.1.2)
type OAuthAuthorizeOutput struct {
	Code  *string `param:"query" description:"The authorization code received from the authorization server"`
	State *string `param:"query" description:"state"`
}

func (p OAuthAuthorizeOutput) String() string {
	return util.Prettify(p)
}

// GetOAuthTokenInput GET /oauth/token input (https://tools.ietf.org/html/rfc6749#section-4.1.3)
type GetOAuthTokenInput struct {
	AuthorizatioinBasicInput
	GrantType    *string `param:"query" name:"grant_type" description:"authorization_code/password/client_credentials/refresh_token"`
	Code         *string `param:"query" name:"code" description:"The authorization code received from the authorization server"`
	RefreshToken *string `param:"query" name:"refresh_token" description:"The refresh token received from the authorization server"`
	RedirectUri  *string `param:"query" name:"redirect_uri" description:"if the \"redirect_uri\" parameter was included in the authorization request as described in Section 4.1.1, and their values MUST be identical.(grant_type:code)"`
}

func (p GetOAuthTokenInput) String() string {
	return util.Prettify(p)
}

// PostOAuthTokenInput POST /oauth/token input (https://tools.ietf.org/html/rfc6749#section-4.1.3)
type PostOAuthTokenInput struct {
	AuthorizatioinBasicInput `json:"-"`
	GrantType                *string `param:"data" json:"grant_type,omitempty" name:"grant_type" description:"authorization_code/password/client_credentials/refresh_token"`
	Code                     *string `param:"data" json:"code,omitempty" name:"code" description:"The authorization code received from the authorization server"`
	RefreshToken             *string `param:"data" json:"refresh_token,omitempty" name:"refresh_token" description:"The refresh token received from the authorization server"`
	RedirectUri              *string `param:"data" json:"redirect_uri,omitempty" name:"redirect_uri" description:"if the \"redirect_uri\" parameter was included in the authorization request as described in Section 4.1.1, and their values MUST be identical."`
}

func (p PostOAuthTokenInput) String() string {
	return util.Prettify(p)
}

// OAuthTokenOutput GET/POST /oauth/token output
type OAuthTokenOutput struct {
	AccessToken  *string `json:"access_token,omitempty" description:"REQUIRED. The access token issued by the authorization server."`
	TokenType    *string `json:"token_type,omitempty" description:"REQUIRED. The type of the token issued as described in Section 7.1.  Value is case insensitive."`
	ExpiresIn    *int32  `json:"expires_in,omitempty" description:"RECOMMENDED.  The lifetime in seconds of the access token.  For example, the value \"3600\" denotes that the access token will expire in one hour from the time the response was generated.  If omitted, the authorization server SHOULD provide the expiration time via other means or document the default value."`
	RefreshToken *string `json:"refresh_token,omitempty" description:"OPTIONAL.  The refresh token, which can be used to obtain new access tokens using the same authorization grant as described in Section 6."`
	Scope        *string `json:"scope,omitempty" description:"OPTIONAL, if identical to the scope requested by the client; otherwise, REQUIRED.  The scope of the access token as described by Section 3.3."`
	Err          *string `json:"error,omitempty"`
	ErrMsg       *string `json:"error_description,omitempty"`
}

func (p OAuthTokenOutput) String() string {
	return util.Prettify(p)
}

// OAuthUserInfoOutput GET /oauth/userinfo output
// The ID Token represents a JWT passed to the client as part of the token response.
// https://openid.net/specs/openid-connect-core-1_0.html#OAuthUserInfoOutput
type OAuthUserInfoOutput struct {
	Issuer    *string `json:"iss,omitempty" description:"Issuer Identifier for the Issuer of the response. The iss value is a case sensitive URL using the https scheme that contains scheme, host, and optionally, port number and path components and no query or fragment components."`
	UserID    *string `json:"sub,omitempty" description:"Subject Identifier. A locally unique and never reassigned identifier within the Issuer for the End-User, which is intended to be consumed by the Client, e.g., 24400320 or AItOawmwtWwcT0k51BayewNvutrJUqsvl6qs7A4. It MUST NOT exceed 255 ASCII characters in length. The sub value is a case sensitive string."`
	ClientID  *string `json:"aud,omitempty" description:"Audience(s) that this ID Token is intended for. It MUST contain the OAuth 2.0 client_id of the Relying Party as an audience value. It MAY also contain identifiers for other audiences. In the general case, the aud value is an array of case sensitive strings. In the common special case when there is one audience, the aud value MAY be a single case sensitive string."`
	ExpiresAt *int64  `json:"exp,omitempty" description:"Expiration time on or after which the ID Token MUST NOT be accepted for processing. The processing of this parameter requires that the current date/time MUST be before the expiration date/time listed in the value. Implementers MAY provide for some small leeway, usually no more than a few minutes, to account for clock skew. Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular."`
	IssuedAt  *int64  `json:"iat,omitempty" description:"Time at which the JWT was issued. Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time."`
	Nonce     *string `json:"nonce,omitempty" description:"String value used to associate a Client session with an ID Token, and to mitigate replay attacks. The value is passed through unmodified from the Authentication Request to the ID Token. If present in the ID Token, Clients MUST verify that the nonce Claim Value is equal to the value of the nonce parameter sent in the Authentication Request. If present in the Authentication Request, Authorization Servers MUST include a nonce Claim in the ID Token with the Claim Value being the nonce value sent in the Authentication Request. Authorization Servers SHOULD perform no other processing on nonce values used. The nonce value is a case sensitive string."` // Non-manditory fields MUST be "omitempty"

	// Custom claims supported by this server.
	// See: https://openid.net/specs/openid-connect-core-1_0.html#StandardClaims
	Email         *string `json:"email,omitempty"`
	EmailVerified *bool   `json:"email_verified,omitempty" description:"RFC5332"`
	Name          *string `json:"name,omitempty"`

	Title       *string `json:"title,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	NickName    *string `json:"nick_name,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	IpPhone     *string `json:"ip_phone,omitempty"`
	Uid         *string `json:"uid,omitempty"`
	Department  *string `json:"department,omitempty"`
	Company     *string `json:"company,omitempty"`
	Address     *string `json:"address,omitempty"`
	Extra       *string `json:"extra,omitempty"`
}

func (p OAuthUserInfoOutput) String() string {
	return util.Prettify(p)
}

func (p *OAuthUserInfoOutput) User() *User {
	return &User{
		Name:        p.Name,
		Title:       p.Title,
		DisplayName: p.DisplayName,
		NickName:    p.NickName,
		Email:       p.Email,
		Phone:       p.Phone,
		IpPhone:     p.IpPhone,
		Uid:         p.Uid,
		Department:  p.Department,
		Company:     p.Company,
		Address:     p.Address,
		Extra:       p.Extra,
	}
}

// OAuthInfoInput POST /oauth/info input
type OAuthInfoInput struct {
	AuthorizatioinBasicInput
	Code *string `param:"query" name:"code" description:"access token"`
	Typ  *int    `param:"query" name:"code_type" description:"Oauth:1 ApiKey:2"`
}

func (p OAuthInfoInput) String() string {
	return util.Prettify(p)
}

// AuthOutput is output format for 'grab get auth'
type AuthOutput struct {
	UserName     *string  `json:"userName,omitempty"`
	Name         *string  `json:"name,omitempty"`
	Scope        *string  `json:"scope,omitempty"`
	Code         *string  `json:"code,omitempty"`
	OtpAt        *string  `json:"otpAt,omitempty"`
	ExpiresAt    *string  `json:"expiresAt,omitempty"`
	RemoteAddrs  []string `json:"remoteAddrs,omitempty" out:"AclRemoteAddrs"`
	ServiceNames []string `json:"serviceNames,omitempty" out:"AclServiceNames"`
}

func (p AuthOutput) String() string {
	return util.Prettify(p)
}

func (p AuthToken) AuthOutput(verbose bool) *AuthOutput {
	code := util.StringValue(p.Code)
	if !verbose {
		code = util.SubStr3(code, 5, -5)
	}

	extra := p.Extra
	if extra == nil {
		extra = &TokenExtra{}
	}

	return &AuthOutput{
		UserName:     p.UserName,
		Name:         p.Name,
		Scope:        util.ToScopeStr(p.Scope),
		Code:         util.String(fmt.Sprintf("%s (%s)", code, TokenType(util.IntValue(p.Typ)).String())),
		OtpAt:        util.String(util.FromNow(util.Int64Value(p.OtpAt))),
		ExpiresAt:    util.String(util.FromNow(util.Int64Value(p.ExpiresAt))),
		RemoteAddrs:  extra.RemoteAddrs,
		ServiceNames: extra.ServiceNames,
	}
}

// }}}

// OAuth client  /api/v1/auth/* {{{

type AuthorizeInput struct {
	ResponseType *string `param:"query" name:"response_type" description:"authorize response type(code|token|openid)"`
	Redirect     *string `param:"query" name:"redirect" description:"redirect url"`
	State        *string `param:"query" name:"state" description:"client state"`
	Scope        *string `param:"query" name:"scope" description:"scope"`
}

func (p AuthorizeInput) String() string {
	return util.Prettify(p)
}

type CallbackInput struct {
	Code         *string `param:"query" name:"code" description:"code authorize response"`
	State        *string `param:"query" name:"state" description:"client state"`
	Output       *string `param:"query" name:"output" description:"token output format json/js(default)"`
	AccessToken  *string `param:"query" name:"access_token" description:"access token"`
	RefreshToken *string `param:"query" name:"refresh_token" description:"refresh token"`
	ExpiresIn    *int32  `param:"query" name:"expires_in" description:"RECOMMENDED.  The lifetime in seconds of the access token.  For example, the value \"3600\" denotes that the access token will expire in one hour from the time the response was generated.  If omitted, the authorization server SHOULD provide the expiration time via other means or document the default value."`
	Scope        *string `param:"query" name:"scope" description:"OPTIONAL, if identical to the scope requested by the client; otherwise, REQUIRED.  The scope of the access token as described by Section 3.3."`
	TokenType    *string `param:"query" name:"token_type" description:"REQUIRED. The type of the token issued as described in Section 7.1.  Value is case insensitive."`
}

func (p CallbackInput) String() string {
	return util.Prettify(p)
}

func (p CallbackInput) TokenResp() *OAuthTokenOutput {
	return &OAuthTokenOutput{
		AccessToken:  p.AccessToken,
		TokenType:    p.TokenType,
		ExpiresIn:    p.ExpiresIn,
		RefreshToken: p.RefreshToken,
		Scope:        p.Scope,
	}
}

type RefreshInput struct {
	RefreshToken *string `param:"query" name:"refresh_token" description:"refresh token"`
}

func (p RefreshInput) String() string {
	return util.Prettify(p)
}

type CallbackOutput struct {
	AccessToken  string `json:"accessToken,omitempty" description:"token code, from sso"`
	RefreshToken string `json:"refreshToken,omitempty" description:"The refresh token received from the authorization server"`
	UserName     string `json:"userName,omitempty" description:"user name, from sso, "`
	TokenName    string `json:"tokenName,omitempty" description:"access name, from sso"`
	Scope        string `json:"scope,omitempty" description:"Scope is a mechanism in OAuth 2.0 to limit an application's access to a user's account, from sso"`
	ExpiresAt    int64  `json:"expiresAt,omitempty" description:"expiration time on. Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular."`
	Redirect     string `json:"redirect,omitempty"`
	Status       string `json:"status,omitempty"`
	Err          string `json:"error,omitempty"`
}

func (p CallbackOutput) String() string {
	return util.Prettify(p)
}

// }}} AuthState db.auth_state {{{
type AuthState struct {
	State     *string
	Typ       *string
	Redirect  *string
	ExpiresAt *int64
}

func (p AuthState) String() string {
	return util.Prettify(p)
}

// }}}

type UserBearerToken struct {
	AccessToken  *string         `json:"accessToken,omitempty" description:"token code, from sso"`
	RefreshToken *string         `json:"refreshToken,omitempty" description:"The refresh token received from the authorization server"`
	UserName     *string         `json:"userName,omitempty" description:"user name, from sso, "`
	UserId       *int64          `json:"userId,omitempty" description:"user id, from custom system"`
	Scope        map[string]bool `json:"scope,omitempty" sql:"-" description:"Scope is a mechanism in OAuth 2.0 to limit an application's access to a user's account, from sso"`
	OtpAt        *int64          `json:"otpAt,omitempty" description:"otp time, Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular."`
	ExpiresAt    *int64          `json:"expiresAt,omitempty" description:"expiration time on. Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular."`
	CachedAt     *int64          `json:"cachedAt,omitempty" description:"cached time"`
	Avatar       *string         `json:"avatar,omitempty" description:"avatar url"`
}

func (p UserBearerToken) String() string {
	return util.Prettify(p)
}

// AuthToken db.auth_token {{{
// TODO: remove me
type AuthTokenTmp struct {
	Code          *string         `json:"code,omitempty" description:"token code, from sso"`
	Typ           *int            `json:"typ,omitempty" description:"1:OAuth, 2:apiKey, from sso"`
	RefreshToken  *string         `json:"refreshToken,omitempty" description:"The refresh token received from the authorization server"`
	RefreshToken2 *string         `json:"refresh_token,omitempty" description:"The refresh token received from the authorization server (deprecated)"`
	Name          *string         `json:"name,omitempty" description:"access name, from sso"`
	UserName      *string         `json:"userName,omitempty" description:"user name, from sso, "`
	UserName2     *string         `json:"user_name,omitempty" description:"user name, from sso, (deprecated)"`
	UserId        *int64          `json:"userId,omitempty" description:"user id, from custom system"`
	UserId2       *int64          `json:"user_id,omitempty" description:"user id, from custom system (deprecated)"`
	Scope         map[string]bool `json:"scope,omitempty" sql:"-" description:"Scope is a mechanism in OAuth 2.0 to limit an application's access to a user's account, from sso"`
	OtpAt         *int64          `json:"otpAt,omitempty" description:"otp time, Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular."`
	OtpAt2        *int64          `json:"otp_at,omitempty" description:"otp time, Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular. (deprecated)"`
	ExpiresAt     *int64          `json:"expiresAt,omitempty" description:"expiration time on. Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular."`
	ExpiresAt2    *int64          `json:"expires_at,omitempty" description:"expiration time on. Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular. (deprecated)"`
	CachedAt      *int64          `json:"cachedAt,omitempty" description:"cached time"`
	CachedAt2     *int64          `json:"cached_at,omitempty" description:"cached time (deprecated)"`
	Extra         *TokenExtra     `json:"extra,omitempty" description:"Extra information for token code"`
}

func (p *AuthToken) AuthTokenTmp() *AuthTokenTmp {
	if p == nil {
		return nil
	}
	return &AuthTokenTmp{
		Code:          p.Code,
		Typ:           p.Typ,
		RefreshToken:  p.RefreshToken,
		RefreshToken2: p.RefreshToken,
		Name:          p.Name,
		UserName:      p.UserName,
		UserName2:     p.UserName,
		UserId:        p.UserId,
		UserId2:       p.UserId,
		Scope:         p.Scope,
		OtpAt:         p.OtpAt,
		OtpAt2:        p.OtpAt,
		ExpiresAt:     p.ExpiresAt,
		ExpiresAt2:    p.ExpiresAt,
		CachedAt:      p.CachedAt,
		CachedAt2:     p.CachedAt,
		Extra:         p.Extra,
	}
}

type AuthToken struct {
	Code         *string         `json:"code,omitempty" description:"token code, from sso"`
	Typ          *int            `json:"typ,omitempty" description:"1:OAuth, 2:apiKey, from sso"`
	RefreshToken *string         `json:"refreshToken,omitempty" description:"The refresh token received from the authorization server"`
	Name         *string         `json:"name,omitempty" description:"access name, from sso"`
	UserName     *string         `json:"userName,omitempty" description:"user name, from sso, "`
	UserId       *int64          `json:"userId,omitempty" description:"user id, from custom system"`
	Scope        map[string]bool `json:"scope,omitempty" sql:"-" description:"Scope is a mechanism in OAuth 2.0 to limit an application's access to a user's account, from sso"`
	OtpAt        *int64          `json:"otpAt,omitempty" description:"otp time, Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular."`
	ExpiresAt    *int64          `json:"expiresAt,omitempty" description:"expiration time on. Its value is a JSON number representing the number of seconds from 1970-01-01T0:0:0Z as measured in UTC until the date/time. See RFC 3339 [RFC3339] for details regarding date/times in general and UTC in particular."`
	CachedAt     *int64          `json:"cachedAt,omitempty" description:"cached time"`
	Extra        *TokenExtra     `json:"extra,omitempty" description:"Extra information for token code"`
	ScopeStr     *string         `json:"-" sql:"scope"`
	// TokenExtra `json:"extra,omitempty" sql:"-" description:"Extra information for token code"`
}

func (p AuthToken) String() string {
	return util.Prettify(p)
}

// CreateAuthTokenInput: just for orm.Insert
type CreateAuthTokenInput struct {
	Code         *string
	Typ          *int
	RefreshToken *string
	UserName     *string
	Name         *string
	UserId       *int64
	Scope        *string
	Extra        *TokenExtra
	OtpAt        *int64
	ExpiresAt    *int64
	CachedAt     *int64
}

func (p CreateAuthTokenInput) String() string {
	return util.Prettify(p)
}

func (p AuthToken) CreateAuthTokenInput() *CreateAuthTokenInput {
	return &CreateAuthTokenInput{
		Code:         p.Code,
		Typ:          p.Typ,
		RefreshToken: p.RefreshToken,
		UserName:     p.UserName,
		Name:         p.Name,
		UserId:       p.UserId,
		Scope:        util.ToScopeStr(p.Scope),
		Extra:        p.Extra,
		OtpAt:        p.OtpAt,
		ExpiresAt:    p.ExpiresAt,
		CachedAt:     p.CachedAt,
	}
}

func (p AuthToken) Who() string {
	if name := util.StringValue(p.Name); name != "" {
		return fmt.Sprintf("%s(%s)", util.StringValue(p.UserName), name)
	}
	return util.StringValue(p.UserName)
}

func (p AuthToken) Tokens() (bearer, apiKey *string) {
	if *p.Typ == *(TokenType_ApiKey.Int()) {
		return nil, p.Code
	}
	return p.Code, nil
}

func (p *AuthToken) CallbackOutput(redirect *string, err error) *CallbackOutput {
	if err != nil {
		return &CallbackOutput{
			Err:    err.Error(),
			Status: "error",
		}
	}

	return &CallbackOutput{
		AccessToken:  util.StringValue(p.Code),
		RefreshToken: util.StringValue(p.RefreshToken),
		UserName:     util.StringValue(p.UserName),
		TokenName:    util.StringValue(p.Name),
		Scope:        util.StringValue(util.ToScopeStr(p.Scope)),
		ExpiresAt:    util.Int64Value(p.ExpiresAt),
		Redirect:     util.StringValue(redirect),
		Status:       "ok",
	}
}

func (p *AuthToken) UserBearerToken(sso string) *UserBearerToken {
	if p == nil {
		return &UserBearerToken{}
	}
	return &UserBearerToken{
		AccessToken:  p.Code,
		RefreshToken: p.RefreshToken,
		UserName:     p.UserName,
		UserId:       p.UserId,
		Scope:        p.Scope,
		OtpAt:        p.OtpAt,
		ExpiresAt:    p.ExpiresAt,
		CachedAt:     p.CachedAt,
		Avatar:       util.String(sso + "/account/photo?name=" + util.StringValue(p.UserName)),
	}
}

type TokenExtra struct {
	// Restrictions *string  `json:"restrictions,omitempty" flags:"-" description:"restrictions name"`
	RemoteAddrs  []string `json:"remoteAddrs,omitempty" flags:"addr,," description:"list of remote addresses allowd to access"`
	ServiceNames []string `json:"serviceNames,omitempty" flags:"service,," description:"list of service names to access"`
}

func (p TokenExtra) String() string {
	return util.Prettify(p)
}

// }}}

// ApiKey db.api_key {{{
type ApiKey struct {
	Id        *int64      `json:"id,omitempty"`
	Name      *string     `json:"name,omitempty" description:"apiKey name"`
	UserName  *string     `json:"userName,omitempty"`
	ApiKey    *string     `json:"apiKey,omitempty"`
	Scope     *string     `json:"scope,omitempty" description:"scope, split by ' '"`
	PreApiKey *string     `json:"preApiKey,omitempty"`
	ExpiresAt *int64      `json:"expiresAt,omitempty"`
	CreatedAt *int64      `json:"createdAt,omitempty" out:",date"`
	LastAt    *int64      `json:"lastAt,omitempty"`
	Extra     *TokenExtra `json:"extra,omitempty" description:"Extra information for API key"`
}

func (p ApiKey) String() string {
	return util.Prettify(p)
}

func (p ApiKey) AuthToken() *AuthToken {
	return &AuthToken{
		Code:      p.ApiKey,
		Typ:       util.Int(int(TokenType_ApiKey)),
		UserName:  p.UserName,
		Name:      p.Name,
		Scope:     util.ToScope(p.Scope),
		ExpiresAt: p.ExpiresAt,
		Extra:     p.Extra,
	}
}

type GetApiKeyCntInput struct {
	Query    *string `param:"query" flags:"user,u," description:"creator's name search(for admin)"`
	UserName *string `param:"-"`
}

func (p GetApiKeyCntInput) String() string {
	return util.Prettify(p)
}

type GetApiKeysInput struct {
	GetApiKeyCntInput `flags:",inline"`
	Pagination        `flags:",inline"`
}

func (p GetApiKeysInput) String() string {
	return util.Prettify(p)
}

type GetApiKeyInput struct {
	ApiKey *string `param:"path" flags:"code,," description:"api key"`
}

func (p GetApiKeyInput) String() string {
	return util.Prettify(p)
}

type CreateApiKeyInput struct {
	Scope     *string `param:"data" json:"scope,omitempty" flags:"scope,," description:"scope, split by ' '"`
	Name      *string `param:"data" json:"name,omitempty" flags:"name,," description:"api key name"`
	ApiKey    *string `param:"-" json:"-"`
	UserName  *string `param:"-" json:"-"`
	CreatedAt *int64  `param:"-" json:"-"`
	LastAt    *int64  `param:"-" json:"-"`
}

func (p CreateApiKeyInput) Log() (action, target string, data interface{}) {
	return ActionCreate, "apiKey " + util.StringValue(p.UserName), nil
}

func (p CreateApiKeyInput) String() string {
	return util.Prettify(p)
}

type UpdateApiKeyInput struct {
	ApiKey *string     `param:"path" json:"-" sql:"api_key,where" flags:"code,," description:"api key"`
	Scope  *string     `param:"data" json:"scope,omitempty" flags:"scope,," description:"scope, split by ' '"`
	Name   *string     `param:"data" json:"name,omitempty" flags:"name,r," description:"access token commentary"`
	Extra  *TokenExtra `param:"data" json:"extra,omitempty" flags:",inline" description:"Extra information for token code"`
}

func (p UpdateApiKeyInput) Log() (action, target string, data interface{}) {
	return ActionUpdate, "apiKey " +
		util.SubStr3(util.StringValue(p.ApiKey), 3, -3), nil
}

func (p UpdateApiKeyInput) String() string {
	return util.Prettify(p)
}

type ResetApiKeyInput struct {
	ApiKey    *string `param:"path" description:"api key"`
	Id        *int64  `param:"-" sql:"id,where" description:"api key id"`
	PreApiKey *string `param:"-" description:"origin api key"`
	ExpiresAt *int64  `param:"-" description:"origin api key expires time"`
}

func (p ResetApiKeyInput) Log() (action, target string, data interface{}) {
	return ActionUpdate, "apiKey " +
		util.SubStr3(util.StringValue(p.ApiKey), 3, -3), nil
}

func (p ResetApiKeyInput) String() string {
	return util.Prettify(p)
}

type RevertApiKeyInput struct {
	ApiKey    *string `param:"path" description:"api key"`
	Id        *int64  `param:"-" sql:"id,where" description:"api key id"`
	PreApiKey *string `param:"-" description:"origin api key"`
	ExpiresAt *int64  `param:"-" description:"origin api key expires time"`
}

func (p RevertApiKeyInput) Log() (action, target string, data interface{}) {
	return ActionUpdate, "apiKey " +
		util.SubStr3(util.StringValue(p.ApiKey), 3, -3), nil
}

func (p RevertApiKeyInput) String() string {
	return util.Prettify(p)
}

type DeleteApiKeyInput struct {
	ApiKey *string `param:"path" json:"-" flags:"code,," description:"api key"`
}

func (p DeleteApiKeyInput) Log() (action, target string, data interface{}) {
	return ActionDelete, "apiKey " +
		util.SubStr3(util.StringValue(p.ApiKey), 3, -3), nil
}

func (p DeleteApiKeyInput) String() string {
	return util.Prettify(p)
}

// }}}

// Otp db.otp {{{
type Otp struct {
	Id        *int64  `json:"id,omitempty"`
	UserName  *string `json:"userName,omitempty"`
	Secret    *string `json:"secret,omitempty" description:"access token commentary"`
	TokenId   *int64  `json:"tokenId,omitempty"`
	AccessAt  *int64  `json:"accessAt,omitempty"`
	LastAt    *int64  `json:"lastAt,omitempty"`
	CreatedAt *int64  `json:"createdAt,omitempty"`
}

func (p Otp) String() string {
	return util.Prettify(p)
}

func (p Otp) UpdateOtpInput() *UpdateOtpInput {
	return &UpdateOtpInput{
		UserName: p.UserName,
		Secret:   p.Secret,
		TokenId:  p.TokenId,
		AccessAt: p.AccessAt,
		LastAt:   p.LastAt,
	}
}

func (p *Otp) Output(host string) *OtpOutput {
	if p == nil {
		return nil
	}

	ret := &OtpOutput{
		Id:        p.Id,
		UserName:  p.UserName,
		TokenId:   p.TokenId,
		AccessAt:  p.AccessAt,
		LastAt:    p.LastAt,
		CreatedAt: p.CreatedAt,
	}

	if host != "" {
		if i := strings.Index(host, "//"); i > 0 {
			host = host[i+2:]
		}

		v := url.Values{}
		v.Set("issuer", "Freewheel Web Services")
		v.Set("secret", util.StringValue(p.Secret))

		code := fmt.Sprintf("otpauth://totp/%s@%s?%v",
			util.StringValue(p.UserName), host, v.Encode())
		ret.OtpCode = &code
		ret.Host = host
	}

	return ret
}

type OtpOutput struct {
	Id        *int64  `json:"id,omitempty"`
	UserName  *string `json:"userName,omitempty"`
	Secret    *string `json:"secret,omitempty" description:"access token commentary"`
	TokenId   *int64  `json:"tokenId,omitempty"`
	AccessAt  *int64  `json:"accessAt,omitempty"`
	LastAt    *int64  `json:"lastAt,omitempty"`
	CreatedAt *int64  `json:"createdAt,omitempty"`
	OtpCode   *string `json:"otpCode,omitempty"`
	Image     *string `json:"image,omitempty" description:"png image Base64Encode"`
	Host      string  `json:"-"`
}

func (p OtpOutput) String() string {
	return util.Prettify(p)
}

type GetOtpInput struct {
	UserName *string `param:"-"`
	OtpCode  *string `param:"query" flags:"otp-code,,"`
}

type CreateOtpInput struct {
	SendMail  *bool   `param:"query" sql:"-" flags:"sendmail,,true" description:"send mail"`
	Image     *bool   `param:"query" sql:"-" flags:"image,,false" description:"response include png with Base64Encode"`
	UserName  *string `param:"-"`
	Secret    *string `param:"-"`
	TokenId   *int64  `param:"-"`
	AccessAt  *int64  `param:"-"`
	LastAt    *int64  `param:"-"`
	CreatedAt *int64  `param:"-"`
}

func (p CreateOtpInput) Log() (action, target string, data interface{}) {
	return ActionCreate, "otp " + util.StringValue(p.UserName), nil
}

func (p CreateOtpInput) String() string {
	return util.Prettify(p)
}

type ResetOtpInput struct {
	SendMail *bool   `param:"query" flags:"sendmail,,true" description:"send mail"`
	Image    *bool   `param:"query" flags:"image,,false" description:"response include png with Base64Encode"`
	UserName *string `param:"query" flags:"user,u," description:"update user's otp secret key(just for admin)"`
	OtpCode  *string `param:"query" flags:"otp-code,," description:"otp code"`
}

func (p ResetOtpInput) Log() (action, target string, data interface{}) {
	return ActionUpdate, "otp " + util.StringValue(p.UserName), nil
}

func (p ResetOtpInput) String() string {
	return util.Prettify(p)
}

// UpdateOtpInput just for sql update
type UpdateOtpInput struct {
	UserName *string `sql:"user_name,where"`
	Secret   *string `param:"-"`
	TokenId  *int64  `param:"-"`
	AccessAt *int64  `param:"-"`
	LastAt   *int64  `param:"-"`
}

func (p UpdateOtpInput) String() string {
	return util.Prettify(p)
}

type DeleteOtpInput struct {
	UserName *string `param:"query" flags:"user,u," description:"update user's otp secret key"`
}

func (p DeleteOtpInput) Log() (action, target string, data interface{}) {
	return ActionDelete, "otp " + util.StringValue(p.UserName), nil
}

func (p DeleteOtpInput) String() string {
	return util.Prettify(p)
}

type AuthenticateOtpInput struct {
	OtpCode *string `param:"data" json:"otpCode,omitempty" description:"otp code"`
}

func (p AuthenticateOtpInput) String() string {
	return util.Prettify(p)
}

// }}}

// Token db.access {{{
type Token struct {
	Id           *int64  `json:"id,omitempty"`
	Name         *string `json:"name,omitempty" description:"access token name"`
	AccessToken  *string `json:"accessToken,omitempty"`
	RefreshToken *string `json:"refreshToken,omitempty"`
	ClientId     *string `json:"clientId,omitempty"`
	Authorize    *string `json:"authorize,omitempty"`
	Previous     *string `json:"previous,omitempty"`
	ExpiresIn    *int64  `json:"expiresIn,omitempty" description:"max 3 year, default:0, max time"`
	Scope        *string `json:"scope,omitempty" description:"scope split by ' '"`
	RedirectUri  *string `json:"redirectUri,omitempty"`
	UserName     *string `json:"userName,omitempty"`
	CreatedAt    *int64  `json:"createdAt,omitempty" out:",date"`
	LastAt       *int64  `json:"lastAt,omitempty" out:",date"`
}

func (p Token) String() string {
	return util.Prettify(p)
}

func (p Token) AuthToken() *AuthToken {
	return &AuthToken{
		Typ:          util.Int(int(TokenType_Oauth)),
		Code:         p.AccessToken,
		RefreshToken: p.RefreshToken,
		UserName:     p.UserName,
		Name:         p.Name,
		Scope:        util.ToScope(p.Scope),
		ExpiresAt:    util.Int64(util.Int64Value(p.CreatedAt) + util.Int64Value(p.ExpiresIn)),
	}
}

type GetTokenCntInput struct {
	Query    *string `param:"query" flags:"user,u," description:"creator's name search(for admin)"`
	UserName *string `param:"-"`
}

func (p GetTokenCntInput) String() string {
	return util.Prettify(p)
}

type GetTokensInput struct {
	GetTokenCntInput `flags:",inline"`
	Pagination       `flags:",inline"`
}

func (p GetTokensInput) String() string {
	return util.Prettify(p)
}

type GetTokenInput struct {
	AccessToken *string `param:"path" name:"token" flags:"code,," description:"access token"`
}

func (p GetTokenInput) String() string {
	return util.Prettify(p)
}

type CreateTokenInput struct {
	// ExpiresIn   *int            `param:"data" json:"expiresIn,omitempty" flags:"-" description:"max 3 day, default:0, max time"`
	Name        *string `param:"data" json:"name,omitempty" flags:"name,," description:"access token commentary"`
	Scope       *string `param:"data" json:"scope,omitempty" flags:"scope,," description:"scope split by ' '"`
	ExpiresIn   *int64  `param:"data" json:"expiresIn,omitempty" flags:"-" description:"expiration seconds time(max 3d)."`
	AccessToken *string `param:"-" json:"-"`
	UserName    *string `param:"-" json:"-"`
	CreatedAt   *int64  `param:"-" json:"-"`
	LastAt      *int64  `param:"-" json:"-"`
}

func (p CreateTokenInput) Log() (action, target string, data interface{}) {
	return ActionCreate, "token " + util.StringValue(p.UserName), p
}

func (p CreateTokenInput) String() string {
	return util.Prettify(p)
}

type UpdateTokenInput struct {
	AccessToken *string `param:"path" json:"-" sql:"access_token,where" name:"token" flags:"code,," description:"access token"`
	Scope       *string `param:"data" json:"scope,omitempty" flags:"scope,," description:"scope split by ' '"`
	Name        *string `param:"data" json:"name,omitempty" flags:"name,," description:"access token commentary"`
}

func (p UpdateTokenInput) Log() (action, target string, data interface{}) {
	return ActionUpdate, "token " +
		util.SubStr3(util.StringValue(p.AccessToken), 3, -3), p
}

func (p UpdateTokenInput) String() string {
	return util.Prettify(p)
}

type DeleteTokenInput struct {
	AccessToken *string `param:"path" json:"-" name:"token" flags:"code,," description:"access token"`
}

func (p DeleteTokenInput) Log() (action, target string, data interface{}) {
	return ActionDelete, "token " +
		util.SubStr3(util.StringValue(p.AccessToken), 3, -3), p
}

func (p DeleteTokenInput) String() string {
	return util.Prettify(p)
}

// }}}

// User db.user {{{
type User struct {
	Id          *int64  `json:"id,omitempty" ldap:"-" description:"sso user id"`
	Name        *string `json:"name,omitempty" ldap:"name" description:"name of the user"`
	Title       *string `json:"title,omitempty" ldap:"title"`
	DisplayName *string `json:"displayName,omitempty" ldap:"displayNamePrintable"`
	NickName    *string `json:"nickName,omitempty" ldap:"mailNickname"`
	Email       *string `json:"email,omitempty" ldap:"mail"`
	Phone       *string `json:"phone,omitempty" ldap:"mobile"`
	IpPhone     *string `json:"ipPhone,omitempty" ldap:"ipPhone"`
	Uid         *string `json:"uid,omitempty" ldap:"uidNumber" description:"ldap uid number"`
	Department  *string `json:"department,omitempty" ldap:"department"`
	Company     *string `json:"company,omitempty" ldap:"company"`
	Address     *string `json:"address,omitempty" ldap:"streetAddress"`
	Extra       *string `json:"extra,omitempty" ldap:"-"`
	CreatedAt   *int64  `json:"createdAt,omitempty" ldap:"-"`
	LastAt      *int64  `json:"lastAt,omitempty" ldap:"-"`
	LoginAt     *int64  `json:"loginAt,omitempty" ldap:"-" sql:"-"`
}

func (p User) String() string {
	return util.Prettify(p)
}

// TODO:
func (p User) Groups() []string {
	return []string{}
}

func (p User) CreateUserInput() *CreateUserInput {
	now := util.Int64(util.Now())
	return &CreateUserInput{
		Name:        p.Name,
		Title:       p.Title,
		DisplayName: p.DisplayName,
		NickName:    p.NickName,
		Email:       p.Email,
		Phone:       p.Phone,
		IpPhone:     p.IpPhone,
		Uid:         p.Uid,
		Department:  p.Department,
		Company:     p.Company,
		Address:     p.Address,
		Extra:       p.Extra,
		CreatedAt:   now,
		LastAt:      now,
	}
}

func (p User) UpdateUserInput() *UpdateUserInput {
	return &UpdateUserInput{
		Name:        p.Name,
		Title:       p.Title,
		DisplayName: p.DisplayName,
		NickName:    p.NickName,
		Email:       p.Email,
		Phone:       p.Phone,
		IpPhone:     p.IpPhone,
		Uid:         p.Uid,
		Department:  p.Department,
		Company:     p.Company,
		Address:     p.Address,
		Extra:       p.Extra,
		LastAt:      util.Int64(util.Now()),
	}
}

func (p User) LdapAttributes() (ret []string) {
	rv := reflect.Indirect(reflect.ValueOf(p))
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		if v, ok := rt.Field(i).Tag.Lookup("ldap"); ok {
			ret = append(ret, v)
		}
	}
	return ret
}

// merge userInfo
func (p User) UserInfo(in ...*OAuthUserInfoOutput) *OAuthUserInfoOutput {
	ret := &OAuthUserInfoOutput{}
	if len(in) > 0 {
		ret = in[0]
	}

	ret.Name = p.Name
	ret.Title = p.Title
	ret.DisplayName = p.DisplayName
	ret.NickName = p.NickName
	ret.Email = p.Email
	ret.Phone = p.Phone
	ret.IpPhone = p.IpPhone
	ret.Uid = p.Uid
	ret.Department = p.Department
	ret.Company = p.Company
	ret.Address = p.Address
	ret.Extra = p.Extra

	return ret
}

type GetUserCntInput struct {
	Query *string `param:"query" description:"user name search"`
}

func (p GetUserCntInput) String() string {
	return util.Prettify(p)
}

type GetUsersInput struct {
	GetUserCntInput
	Pagination
}

func (p GetUsersInput) String() string {
	return util.Prettify(p)
}

type GetUserInput struct {
	Name *string `param:"path" description:"identifier of the user"`
	Sync *bool   `param:"query" description:"sync user from provider(ldap) when user does not exist"`
}

func (p GetUserInput) String() string {
	return util.Prettify(p)
}

type CreateUserInput struct {
	Name        *string `param:"data" json:"name,omitempty" description:"name of the user"`
	Title       *string `param:"data" json:"title,omitempty"`
	DisplayName *string `param:"data" json:"displayName,omitempty"`
	NickName    *string `param:"data" json:"nickName,omitempty"`
	Email       *string `param:"data" json:"email,omitempty"`
	Phone       *string `param:"data" json:"phone,omitempty"`
	IpPhone     *string `param:"data" json:"ipPhone,omitempty"`
	Uid         *string `param:"data" json:"uid,omitempty"`
	Department  *string `param:"data" json:"department,omitempty"`
	Company     *string `param:"data" json:"company,omitempty"`
	Address     *string `param:"data" json:"address,omitempty"`
	Extra       *string `param:"data" json:"extra,omitempty"`
	CreatedAt   *int64  `param:"data" json:"createdAt,omitempty"`
	LastAt      *int64  `param:"data" json:"lastAt,omitempty"`
}

func (p CreateUserInput) String() string {
	return util.Prettify(p)
}

type UpdateUserInput struct {
	Name        *string `param:"path" json:"-" sql:"name,where"`
	Title       *string `param:"data" json:"title,omitempty"`
	DisplayName *string `param:"data" json:"displayName,omitempty"`
	NickName    *string `param:"data" json:"nickName,omitempty"`
	Email       *string `param:"data" json:"email,omitempty"`
	Phone       *string `param:"data" json:"phone,omitempty"`
	IpPhone     *string `param:"data" json:"ipPhone,omitempty"`
	Uid         *string `param:"data" json:"uid,omitempty"`
	Department  *string `param:"data" json:"department,omitempty"`
	Company     *string `param:"data" json:"company,omitempty"`
	Address     *string `param:"data" json:"address,omitempty"`
	Extra       *string `param:"data" json:"extra,omitempty"`
	LastAt      *int64  `param:"data" json:"lastAt,omitempty"`
}

func (p UpdateUserInput) Log() (action, target string, data interface{}) {
	return ActionUpdate, "user " + util.StringValue(p.Name), p
}

func (p UpdateUserInput) String() string {
	return util.Prettify(p)
}

type DeleteUserInput struct {
	Name *string `param:"path" description:"name of the user"`
}

func (p DeleteUserInput) Log() (action, target string, data interface{}) {
	return ActionDelete, "user " + util.StringValue(p.Name), nil
}

func (p DeleteUserInput) String() string {
	return util.Prettify(p)
}

type GetUserPhotoInput struct {
	Name   *string `param:"query" description:"name of the user"`
	Update *bool   `param:"query" description:"update user's photo from provider(ldap)"`
}

func (p GetUserPhotoInput) String() string {
	return util.Prettify(p)
}

// }}}

// UserPhoto db.user_photo {{{
type UserPhoto struct {
	Id        *int64
	UserId    *int64
	UserName  *string
	Data      []byte
	LastAt    *int64
	CreatedAt *int64
}

func (p UserPhoto) String() string {
	return util.Prettify(p)
}

// }}}

type ClientTmp struct {
	Id            *int64  `json:"id,omitempty"`
	ClientId      *string `json:"clientId,omitempty"`
	ClientId2     *string `json:"client_id,omitempty"`
	ClientSecret  *string `json:"clientSecret,omitempty"`
	ClientSecret2 *string `json:"client_secret,omitempty"`
	RedirectUri   *string `json:"redirectUri,omitempty"`
	RedirectUri2  *string `json:"redirect_uri,omitempty"`
	UserName      *string `json:"userName,omitempty"`
	UserName2     *string `json:"user_name,omitempty"`
	CreatedAt     *int64  `json:"createdAt,omitempty"`
	CreatedAt2    *int64  `json:"created_at,omitempty"`
}

func (p *Client) ClientTmp() *ClientTmp {
	if p == nil {
		return nil
	}
	return &ClientTmp{
		Id:            p.Id,
		ClientId:      p.ClientId,
		ClientId2:     p.ClientId,
		ClientSecret:  p.ClientSecret,
		ClientSecret2: p.ClientSecret,
		RedirectUri:   p.RedirectUri,
		RedirectUri2:  p.RedirectUri,
		UserName:      p.UserName,
		UserName2:     p.UserName,
		CreatedAt:     p.CreatedAt,
		CreatedAt2:    p.CreatedAt,
	}
}

type Client struct {
	Id           *int64  `json:"id,omitempty"`
	Name         *string `json:"name,omitempty"`
	ClientId     *string `json:"clientId,omitempty"`
	ClientSecret *string `json:"clientSecret,omitempty"`
	RedirectUri  *string `json:"redirectUri,omitempty"`
	UserName     *string `json:"userName,omitempty"`
	CreatedAt    *int64  `json:"createdAt,omitempty" out:",date"`
}

func (p Client) String() string {
	return util.Prettify(p)
}

func (d *Client) GetId() string {
	return util.StringValue(d.ClientId)
}

func (d *Client) GetSecret() string {
	return util.StringValue(d.ClientSecret)
}

func (d *Client) GetRedirectUri() string {
	return util.StringValue(d.RedirectUri)
}

func (d *Client) GetUserName() string {
	return util.StringValue(d.UserName)
}

// Implement the ClientSecretMatcher interface
func (d *Client) ClientSecretMatches(secret string) bool {
	return util.StringValue(d.ClientSecret) == secret
}

/*
func (d *Client) CopyFrom(client Client) {
	d.ClientId = client.ClientId
	d.ClientSecret = client.ClientSecret
	d.RedirectUri = client.RedirectUri
	d.UserName = client.UserName
}
*/

func (p Client) CreateClientInput() *CreateClientInput {
	return &CreateClientInput{
		Name:         p.Name,
		RedirectUri:  p.RedirectUri,
		ClientId:     p.ClientId,
		ClientSecret: p.ClientSecret,
		UserName:     p.UserName,
		CreatedAt:    util.Int64(util.Now()),
	}
}

func (p Client) UpdateClientInput() *UpdateClientInput {
	return &UpdateClientInput{
		Name:         p.Name,
		ClientId:     p.ClientId,
		ClientSecret: p.ClientSecret,
		RedirectUri:  p.RedirectUri,
	}
}

type GetClientCntInput struct {
	Query *string `param:"query" description:"redirect_uri/user_name search"`
}

func (p GetClientCntInput) String() string {
	return util.Prettify(p)
}

type GetClientsInput struct {
	GetClientCntInput `flags:",inline"`
	Pagination        `flags:",inline"`
}

func (p GetClientsInput) String() string {
	return util.Prettify(p)
}

type GetClientInput struct {
	ClientId *string `param:"path" description:"client id"`
}

func (p GetClientInput) String() string {
	return util.Prettify(p)
}

type CreateClientInputTmp struct {
	RedirectUri  *string `param:"data" json:"redirectUri,omitempty" flags:"redirect-uri,," description:"redirct callback uri e.g. http://example.com/auth/callback"`
	RedirectUri2 *string `param:"data" json:"redirect_uri,omitempty" description:"same as redirectUri (deprecated)"`
	ClientId     *string `param:"-" json:"-"`
	ClientSecret *string `param:"-" json:"-"`
	UserName     *string `param:"-" json:"-"`
	CreatedAt    *int64  `param:"-" json:"-"`
}

func (p *CreateClientInputTmp) Validate() error {
	if p.RedirectUri == nil {
		p.RedirectUri = p.RedirectUri2
	}
	if p.RedirectUri == nil {
		return status.Errorf(codes.InvalidArgument, "redirectUri must be set")
	}
	return nil
}

type CreateClientInput struct {
	Name         *string `param:"data" json:"name,omitempty" flags:"name,," description:"client name"`
	RedirectUri  *string `param:"data" json:"redirectUri,omitempty" flags:"redirect-uri,," description:"redirct callback uri e.g. http://example.com/auth/callback"`
	ClientId     *string `param:"-" json:"-"`
	ClientSecret *string `param:"-" json:"-"`
	UserName     *string `param:"-" json:"-"`
	CreatedAt    *int64  `param:"-" json:"-"`
}

func (p *CreateClientInput) Validate() error {
	if p.RedirectUri == nil {
		return status.Errorf(codes.InvalidArgument, "redirectUri must be set")
	}
	return nil
}

func (p CreateClientInput) Log() (action, target string, data interface{}) {
	return ActionCreate, "client " + util.StringValue(p.RedirectUri), p
}

func (p CreateClientInput) String() string {
	return util.Prettify(p)
}

type UpdateClientInput struct {
	Name         *string `param:"data" json:"name,omitempty" flags:"name,r," description:"access token commentary"`
	ClientId     *string `param:"path" json:"-" sql:"client_id,where" description:"client id"`
	ClientSecret *string `param:"data" json:"clientSecret,omitempty"`
	RedirectUri  *string `param:"data" json:"redirectUri,omitempty"`
}

func (p UpdateClientInput) Log() (action, target string, data interface{}) {
	p2 := p
	p2.ClientSecret = nil
	return ActionUpdate, "client " + util.StringValue(p2.RedirectUri), p2
}

func (p UpdateClientInput) String() string {
	return util.Prettify(p)
}

type DeleteClientInput struct {
	ClientId *string `param:"path" description:"client id"`
}

func (p DeleteClientInput) Log() (action, target string, data interface{}) {
	return ActionDelete, "client " + util.StringValue(p.ClientId), p
}

func (p DeleteClientInput) String() string {
	return util.Prettify(p)
}

func (x TokenType) Int() *int {
	n := int(x)
	return &n
}

// GetSsoInput  GET /saml/sso
type GetSsoInput struct {
	SAMLRequest *string `param:"query" name:"SAMLRequest" description:"saml request"`
	RelayState  *string `param:"query" name:"RelayState" description:"saml relay state"`
}

func (p GetSsoInput) String() string {
	return util.Prettify(p)
}

// PostSsoInput  POST /saml/sso
type PostSsoInput struct {
	Username      *string `param:"data" description:"name of the login user"`
	Password      *string `param:"data,password" description:"password of the login user"`
	SAMLRequest   *string `param:"query" name:"SAMLRequest" description:"saml request"`
	RelayState    *string `param:"query" name:"RelayState" description:"saml relay state"`
	RequestBuffer []byte  `param:"-"`
}

func (p PostSsoInput) String() string {
	return util.Prettify(p)
}
