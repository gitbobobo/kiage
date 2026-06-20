//go:build !linux

package app

func kindleFirewallOpen(port int) {}

func kindleFirewallClose(port int) {}
