package util

import (
	"os/user"
	"testing"
)

func TestGetUGroups(t *testing.T) {
	u, _ := user.Current()
	if gs, err := GetUGroups(u.Username); err != nil {
		t.Fatalf("GetUGroups: %v", err)
	} else {
		t.Logf("username:%s groups:%v\n", u.Username, gs)
	}
}
