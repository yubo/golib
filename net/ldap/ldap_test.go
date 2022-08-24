/*
 * Copyright 2016,2017 yubo. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */
package ldap

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestVerify(t *testing.T) {
	env := func(key, def string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return def
	}
	username := env("LDAP_USR", "")
	password := env("LDAP_PWD", "")

	l := &Ldap{
		Config: &Config{
			Addr:    env("LDAP_ADDR", "localhost:389"),
			BaseDN:  env("LDAP_BASE_DN", "cn=root,dc=example,dc=com"),
			BindDN:  env("LDAP_BIND_DN", ""),
			BindPwd: env("LDAP_BIND_PWD", ""),
			Filter:  env("LDAP_FILTER", "(&(objectClass=posixAccount)(cn=%s))"),
		},
	}

	t.Logf("%v\n", l.Config)
	t.Logf("user %s\n", username)
	t.Logf("pwd %s\n", password)

	entry, err := l.ldapUserAuthentication(username, password,
		"name",
		"title",
		"displayNamePrintable",
		"mailNickname",
		"mail",
		"mobile",
		"ipPhone",
		"uidNumber",
		"department",
		"company",
		"streetAddress",
		"uid",
	)
	if err != nil {
		if strings.HasPrefix(err.Error(), "dial ldap") {
			t.Logf("ignore err: %s", err)
			return
		}
		t.Fatalf("err: %s\n", err)
	}

	for _, v := range entry.Attributes {
		for i, vv := range v.Values {
			t.Logf("%s[%d] %s", v.Name, i, vv)
		}
	}

	// image user
	raw, err := l.GetUserRaw(username, "thumbnailPhoto")
	if err != nil {
		return
	}
	for i, v := range raw["thumbnailPhoto"] {
		t.Logf("thumbnailPhoto[%d] len %d", i, len(v))
		ioutil.WriteFile(fmt.Sprintf("/tmp/thumbnail-%d.jpg", i), v, 0644)
	}
}
