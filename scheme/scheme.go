package scheme

import (
	"github.com/yubo/golib/runtime"
	"github.com/yubo/golib/runtime/serializer"
)

var Codecs = serializer.NewCodecFactory()
var ParameterCodec = runtime.NewParameterCodec()
