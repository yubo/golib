module github.com/yubo/golib

go 1.15

require (
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96
	github.com/elazarl/goproxy v0.0.0-20180725130230-947c36da3153
	github.com/emicklei/go-restful v2.15.0+incompatible
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/go-ldap/ldap v3.0.3+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.1.2
	github.com/hashicorp/golang-lru v0.5.1
	github.com/json-iterator/go v1.1.11
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.0
	github.com/mitchellh/go-wordwrap v1.0.1
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635
	github.com/modern-go/reflect2 v1.0.1
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.15.0
	github.com/spf13/cast v1.3.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/uber-go/tally v3.3.17+incompatible
	go.uber.org/atomic v1.6.0 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.13.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	golang.org/x/sys v0.0.0-20210426230700-d19ff857e887
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	golang.org/x/tools v0.0.0-20210106214847-113979e3529a // indirect
	google.golang.org/genproto v0.0.0-20210204154452-deb828366460 // indirect
	google.golang.org/grpc v1.27.1
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	gotest.tools/v3 v3.0.3 // indirect
	honnef.co/go/tools v0.0.1-2020.1.6 // indirect
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/elazarl/goproxy => github.com/elazarl/goproxy v0.0.0-20180725130230-947c36da3153 // 947c36da3153 is the SHA for git tag v1.11
