package util

import "testing"

func TestGetRemoteIP(t *testing.T) {
	cases := []string{
		"127.0.0.1:80",
		"[::1]:61827",
	}
	for _, c := range cases {
		if got, err := GetRemoteIP(c); err != nil {
			t.Fatalf("GetRemoteIP(%s) error %v", c, err)
		} else {
			t.Logf("GetRemoteIP(%s) got %v", c, got)
		}
	}
}

func TestIPContains(t *testing.T) {
	cases := []struct {
		ip      string
		network string
		want    bool
	}{
		{"[::1]:61827", "::1/128", true},
	}
	for _, c := range cases {
		ip, err := GetRemoteIP(c.ip)
		if err != nil {
			t.Fatalf("GetRemoteIP(%s) error %v", c.ip, err)
		}

		got := IPContains(ip, c.network)
		if got != c.want {
			t.Fatalf("IPContains(%s, %s) got %v want %v", ip, c.network, got, c.want)
		}
		t.Logf("IPContains(%s, %s) got %v want %v", ip, c.network, got, c.want)
	}
}
