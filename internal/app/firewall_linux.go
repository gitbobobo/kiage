//go:build linux

package app

import (
	"os"
	"os/exec"
	"strconv"

	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/render"
)

func iptablesBin() string {
	for _, p := range []string{"/sbin/iptables", "/usr/sbin/iptables"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return "iptables"
}

func kindleFirewallHasRule(bin string, port int) bool {
	args := []string{"-C", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "ACCEPT"}
	return exec.Command(bin, args...).Run() == nil
}

func kindleFirewallPurgeStale(port int) {
	if port <= 0 || !render.KindleUI() {
		return
	}
	bin := iptablesBin()
	for kindleFirewallHasRule(bin, port) {
		_ = exec.Command(bin, "-D", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "ACCEPT").Run()
	}
}

func kindleFirewallOpen(port int) {
	if port <= 0 || !render.KindleUI() {
		return
	}
	kindleFirewallPurgeStale(port)
	bin := iptablesBin()
	args := []string{"-I", "INPUT", "1", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "ACCEPT"}
	if err := exec.Command(bin, args...).Run(); err != nil {
		log.Warn("kindle firewall apply failed port=%d: %v", port, err)
		return
	}
	log.Info("kindle firewall opened port=%d (%s)", port, bin)
}

func kindleFirewallClose(port int) {
	if port <= 0 || !render.KindleUI() {
		return
	}
	bin := iptablesBin()
	if !kindleFirewallHasRule(bin, port) {
		return
	}
	args := []string{"-D", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "ACCEPT"}
	if err := exec.Command(bin, args...).Run(); err != nil {
		log.Warn("kindle firewall remove failed port=%d: %v", port, err)
		return
	}
	log.Info("kindle firewall closed port=%d", port)
}
