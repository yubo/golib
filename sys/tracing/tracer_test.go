package tracing

// go test -v -args -v 8 -logtostderr true

import (
	"flag"
	"testing"

	"k8s.io/klog/v2"
)

func TestInfof(t *testing.T) {
	flag.Set("v", "3")
	flag.Parse()
	defer klog.Flush()
	tracer := GetTracerWithSpan(nil)
	tracer.Infof("info %v %v", "aa", "bb")
	tracer.V(5).Infof("info %v %v", "ss", "ff")
	//tracer.Error("error")
	//tracer.Warningln("warning", "warning")
	//tracer.Fatal("fatal")
}
