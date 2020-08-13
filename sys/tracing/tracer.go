package tracing

import (
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	"k8s.io/klog/v2"
)

var (
	level = 1
)

func SetLevel(l int) {
	level = l
}

// Tracer will output trace to Jaeger and standard output
type Tracer struct {
	span  opentracing.Span
	level int
}

// GetTracerWithSpan returns a tracer instance with opentracing span
// If span is nil, tracer will only print to standard output
func GetTracerWithSpan(span opentracing.Span) *Tracer {
	if span == nil {
		return &Tracer{}
	}
	return &Tracer{span: span}
}

func (t *Tracer) V(level int) *Tracer {
	return &Tracer{span: t.span, level: level}
}

// GetRequestIDWithSpan returns trace ID as request ID
func GetRequestIDWithSpan(span opentracing.Span) string {
	if sc, ok := span.Context().(jaeger.SpanContext); ok {
		reqID := sc.TraceID().String()
		return reqID
	}
	return ""
}

// Infof is the same as klog.Infof and will try to log to span additionally
func (t *Tracer) Infof(format string, args ...interface{}) {
	if level >= t.level {
		klog.InfoDepth(1, fmt.Sprintf(format, args...))
	}
	if t.span != nil {
		t.span.LogKV(
			fmt.Sprintf("Info(%v)", t.level),
			fmt.Sprintf(format, args...),
		)
	}
}

// Info is the same as klog.Info and will try to log to span additionally
func (t *Tracer) Info(args ...interface{}) {
	if level >= t.level {
		klog.InfoDepth(1, fmt.Sprint(args...))
	}
	if t.span != nil {
		t.span.LogKV(
			fmt.Sprintf("Info(%v)", t.level),
			fmt.Sprint(args...),
		)
	}
}

// Infoln is the same as klog.Infoln and will try to log to span additionally
func (t *Tracer) Infoln(args ...interface{}) {
	if level >= t.level {
		klog.InfoDepth(1, fmt.Sprintln(args...))
	}
	if t.span != nil {
		t.span.LogKV(
			fmt.Sprintf("Info(%v)", t.level),
			fmt.Sprintln(args...),
		)
	}
}

// Error is the same as klog.Info and will try to log to span additionally
func (t *Tracer) Error(args ...interface{}) {
	klog.ErrorDepth(1, fmt.Sprint(args...))
	if t.span != nil {
		t.span.LogKV("Error", fmt.Sprint(args...))
		ext.Error.Set(t.span, true)
	}
}

// Errorf is the same as klog.Infof and will try to log to span additionally
func (t *Tracer) Errorf(format string, args ...interface{}) {
	klog.ErrorDepth(1, fmt.Sprintf(format, args...))
	if t.span != nil {
		t.span.LogKV("Error", fmt.Sprintf(format, args...))
		ext.Error.Set(t.span, true)
	}
}

// Errorln is the same as klog.Infoln and will try to log to span additionally
func (t *Tracer) Errorln(args ...interface{}) {
	klog.ErrorDepth(1, fmt.Sprintln(args...))
	if t.span != nil {
		t.span.LogKV("Error", fmt.Sprintln(args...))
		ext.Error.Set(t.span, true)
	}
}

// Warning is the same as klog.Info and will try to log to span additionally
func (t *Tracer) Warning(args ...interface{}) {
	klog.InfoDepth(1, fmt.Sprint(args...))
	if t.span != nil {
		t.span.LogKV("Warning", fmt.Sprint(args...))
	}
}

// Warningf is the same as klog.Infof
// and will try to log to span additionally
func (t *Tracer) Warningf(format string, args ...interface{}) {
	klog.ErrorDepth(1, fmt.Sprintf(format, args...))
	if t.span != nil {
		t.span.LogKV("Warning", fmt.Sprintf(format, args...))
	}
}

// Warningln is the same as klog.Infoln
// and will try to log to span additionally
func (t *Tracer) Warningln(args ...interface{}) {
	klog.ErrorDepth(1, fmt.Sprintln(args...))
	if t.span != nil {
		t.span.LogKV("Warning", fmt.Sprintln(args...))
	}
}

// Fatal is the same as klog.Info and will try to log to span additionally
func (t *Tracer) Fatal(args ...interface{}) {
	if t.span != nil {
		t.span.LogKV("Fatal", fmt.Sprint(args...))
		ext.Error.Set(t.span, true)
	}
	klog.ErrorDepth(1, fmt.Sprint(args...))
}

// Fatalf is the same as klog.Infof and will try to log to span additionally
func (t *Tracer) Fatalf(format string, args ...interface{}) {
	if t.span != nil {
		t.span.LogKV("Fatal", fmt.Sprintf(format, args...))
		ext.Error.Set(t.span, true)
	}
	klog.ErrorDepth(1, fmt.Sprintf(format, args...))
}

// Fatalln is the same as klog.Infoln and will try to log to span additionally
func (t *Tracer) Fatalln(args ...interface{}) {
	if t.span != nil {
		t.span.LogKV("Fatal", fmt.Sprintln(args...))
		ext.Error.Set(t.span, true)
	}
	klog.ErrorDepth(1, fmt.Sprintln(args...))
}

// LogKV will output as <key>:<value> format
func (t *Tracer) LogKV(key interface{}, value interface{}) {
	if level >= t.level {
		klog.InfoDepth(1, fmt.Sprintf("%v: %v", key, value))
	}
	if t.span != nil {
		t.span.LogKV(key, value)
	}
}
