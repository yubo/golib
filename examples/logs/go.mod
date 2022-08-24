module github.com/yubo/golib/examples/logs

go 1.16

replace github.com/yubo/golib => ../..

replace github.com/yubo/golib/logs/json => ../../logs/json

require (
	github.com/spf13/cobra v1.4.0
	github.com/yubo/golib v0.0.1
	github.com/yubo/golib/logs/json v0.0.0-00010101000000-000000000000
	k8s.io/klog/v2 v2.70.1
)
