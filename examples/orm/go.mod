module examples/orm

go 1.20

replace github.com/yubo/golib => ../..

require (
	github.com/yubo/golib v0.0.1
	k8s.io/klog/v2 v2.80.1
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/mattn/go-sqlite3 v1.14.15 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
