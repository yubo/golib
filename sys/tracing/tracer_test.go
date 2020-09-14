package tracing

// go test -v -args -v 8 -logtostderr true

import (
	"testing"
)

func TestInfof(t *testing.T) {
	SetLevel(6)
	tracer := GetTracerWithSpan(nil)
	tracer.Infof("info %v %v", "aa", "bb")
	tracer.V(5).Infof("info %v %v", "ss", "ff")
	//tracer.Error("error")
	//tracer.Warningln("warning", "warning")
	//tracer.Fatal("fatal")
}
