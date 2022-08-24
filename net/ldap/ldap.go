package ldap

import (
	"errors"
	"fmt"

	"github.com/go-ldap/ldap"
	liberrors "github.com/yubo/golib/api/errors"
	"github.com/yubo/golib/config/configtls"
)

type Config struct {
	Addr    string                     `json:"addr"`
	BaseDN  string                     `json:"baseDn"`
	BindDN  string                     `json:"bindDn"`
	BindPwd string                     `json:"bindPwd"`
	Filter  string                     `json:"filter"`
	DialTLS bool                       `json:"dialTLS"` // true: DailTLS(), false: StartTLS()
	TLS     configtls.TLSClientSetting `json:"tls"`
}

type Ldap struct {
	*Config
}

func New(conf *Config) (*Ldap, error) {
	if conf == nil {
		return nil, nil
	}
	cli := &Ldap{Config: conf}

	conn, err := cli.connection()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if cli.BindDN != "" {
		err = conn.Bind(cli.BindDN, cli.BindPwd)
		if err != nil {
			return cli, fmt.Errorf("ldap: bind ldap: %s", err.Error())
		}
	}

	return cli, nil
}

func getAtrributes(e *ldap.Entry) map[string]string {
	ret := map[string]string{}
	for _, v := range e.Attributes {
		ret[v.Name] = v.Values[0]
	}
	return ret
}

func getAtrributesRaw(e *ldap.Entry) map[string][][]byte {
	ret := map[string][][]byte{}
	for _, v := range e.Attributes {
		ret[v.Name] = v.ByteValues
	}
	return ret
}

func (p *Ldap) Login(username, password string, attributes ...string) (map[string]string, error) {
	// bugfix: empty password
	if len(password) == 0 {
		return nil, errors.New("password is empty")
	}

	entry, err := p.ldapUserAuthentication(username, password, attributes...)
	if err != nil {
		return map[string]string{}, err
	}
	return getAtrributes(entry), nil
}

func (p *Ldap) GetUser(username string, attributes ...string) (map[string]string, error) {
	entry, err := p.ldapUserAuthentication(username, "", attributes...)
	if err != nil {
		return map[string]string{}, err
	}
	return getAtrributes(entry), nil
}

func (p *Ldap) GetUserRaw(username string, attributes ...string) (map[string][][]byte, error) {
	entry, err := p.ldapUserAuthentication(username, "", attributes...)
	if err != nil {
		return map[string][][]byte{}, err
	}
	return getAtrributesRaw(entry), nil
}

func (p *Ldap) connection() (conn *ldap.Conn, err error) {
	tlsconfig, err := p.TLS.LoadTLSConfig()
	if err != nil {
		return nil, err
	}

	if p.DialTLS && tlsconfig != nil {
		conn, err = ldap.DialTLS("tcp", p.Addr, tlsconfig)
		if err != nil {
			return nil, fmt.Errorf("dial ldaps://%s: %s", p.Addr, err.Error())
		}

		return conn, nil
	}

	// ldap
	conn, err = ldap.Dial("tcp", p.Addr)
	if err != nil {
		return nil, fmt.Errorf("dial ldap://%s: %s", p.Addr, err.Error())
	}

	if tlsconfig != nil {
		// with tls
		if err = conn.StartTLS(tlsconfig); err != nil {
			return nil, fmt.Errorf("start tls: %s", err.Error())
		}
	}

	return conn, nil
}

func (p *Ldap) ldapUserAuthentication(username, password string, attributes ...string) (*ldap.Entry, error) {
	conn, err := p.connection()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// First bind with a read only user
	err = conn.Bind(p.BindDN, p.BindPwd)
	if err != nil {
		return nil, fmt.Errorf("bind ldap: %s", err.Error())
	}

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		p.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(p.Filter, username),
		attributes,
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("ldap search: %s", err.Error())
	}

	if len(sr.Entries) != 1 {
		return nil, liberrors.NewUnauthorized("ldap")
	}

	if password == "" {
		return sr.Entries[0], nil
	}

	// Bind as the user to verify their password
	err = conn.Bind(sr.Entries[0].DN, password)
	if err != nil {
		return nil, liberrors.NewUnauthorized(fmt.Sprintf("ldap.Bind() %s error %s",
			sr.Entries[0].DN, err.Error()))
	}
	return sr.Entries[0], nil
}
