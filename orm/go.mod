module github.com/yubo/golib/orm

go 1.18

replace github.com/yubo/golib => ../

require (
	github.com/go-sql-driver/mysql v1.6.0
	github.com/mattn/go-sqlite3 v1.14.15
	github.com/stretchr/testify v1.8.0
	github.com/yubo/golib v0.0.0-00010101000000-000000000000
	k8s.io/klog/v2 v2.70.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)
