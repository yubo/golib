module github.com/yubo/golib/net/ldap

go 1.18

replace github.com/yubo/golib => ../..

require (
	github.com/go-ldap/ldap v3.0.3+incompatible
	github.com/yubo/golib v0.0.0-00010101000000-000000000000
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/google/go-cmp v0.5.5 // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	k8s.io/klog/v2 v2.60.1 // indirect
)
