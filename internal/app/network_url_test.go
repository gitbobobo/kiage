package app

import (
	"net"
	"testing"
)

func TestFirstIPv4(t *testing.T) {
	ip := firstIPv4([]net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.31.116"), Mask: net.CIDRMask(24, 32)},
	})
	if ip != "192.168.31.116" {
		t.Fatalf("got %q", ip)
	}
}
