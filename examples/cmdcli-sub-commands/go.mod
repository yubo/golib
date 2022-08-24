module examples/multi-cmds

go 1.16

replace github.com/yubo/golib => ../..

require (
	github.com/spf13/cobra v1.4.0
	github.com/yubo/golib v0.0.1
	k8s.io/klog/v2 v2.60.1
)
