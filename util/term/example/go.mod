module example

go 1.18

replace github.com/yubo/golib => ../../..

replace github.com/yubo/golib/util/term => ../

require (
	github.com/yubo/golib v0.0.0-00010101000000-000000000000
	github.com/yubo/golib/util/term v0.0.0-00010101000000-000000000000
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	golang.org/x/sys v0.0.0-20220823224334-20c2bfdbfe24 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.60.1 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)
