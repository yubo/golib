package net

import (
	net "github.com/yubo/golib/util/net/sloopy"
)

// ParseIPSloppy is identical to Go's standard net.ParseIP, except that it allows
// leading '0' characters on numbers.  Go used to allow this and then changed
// the behavior in 1.17.  We're choosing to keep it for compat with potential
// stored values.
var ParseIPSloppy = net.ParseIP

// ParseCIDRSloppy is identical to Go's standard net.ParseCIDR, except that it allows
// leading '0' characters on numbers.  Go used to allow this and then changed
// the behavior in 1.17.  We're choosing to keep it for compat with potential
// stored values.
var ParseCIDRSloppy = net.ParseCIDR
