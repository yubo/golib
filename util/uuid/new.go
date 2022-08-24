package uuid

import (
	"github.com/yubo/golib/types"
)

func NewUUID() types.UID {
	return types.UID(New().String())
}
