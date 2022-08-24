module examples/stream

go 1.18

replace github.com/yubo/golib => ../..

require (
	github.com/yubo/golib v0.0.0-00010101000000-000000000000
	k8s.io/klog/v2 v2.70.1
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	golang.org/x/sys v0.0.0-20211013075003-97ac67df715c // indirect
)
