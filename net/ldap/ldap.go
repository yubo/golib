package ldap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap"
	liberrors "github.com/yubo/golib/api/errors"
	"github.com/yubo/golib/util"
)

type LdapConfig struct {
	Addr               string `json:"addr"`
	BaseDN             string `json:"baseDn"`
	BindDN             string `json:"bindDn"`
	BindPwd            string `json:"bindPwd"`
	Filter             string `json:"filter"`
	SSL                bool   `json:"ssl"`
	TLS                bool   `json:"tls"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
}

func (p *LdapConfig) Validate() error {
	l, err := NewLdap(p, nil)
	if err != nil {
		return err
	}

	conn, err := l.connection()
	if err != nil {
		return err
	}
	defer conn.Close()

	if p.BindDN != "" {
		err = conn.Bind(p.BindDN, p.BindPwd)
		if err != nil {
			return fmt.Errorf("ldap: bind ldap: %s", err.Error())
		}
	}

	return nil
}

func (p LdapConfig) String() string {
	return util.Prettify(p)
}

type Ldap struct {
	*LdapConfig
	certificates []tls.Certificate
}

func NewLdap(conf *LdapConfig, certificates []tls.Certificate) (*Ldap, error) {
	if l, err := ldap.Dial("tcp", conf.Addr); err != nil {
		return nil, err
	} else {
		l.Close()
	}

	return &Ldap{LdapConfig: conf, certificates: certificates}, nil
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
	if p.SSL {
		// ldaps
		conf := &tls.Config{
			InsecureSkipVerify: p.InsecureSkipVerify,
			ServerName:         p.Addr[:strings.Index(p.Addr, ":")],
		}

		if p.certificates != nil {
			conf.Certificates = p.certificates
		}

		conn, err = ldap.DialTLS("tcp", p.Addr, conf)

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

	if p.TLS {
		// with tls
		if err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
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
