package app

import (
	"fmt"
	"net"
)

func localLANURL(port int) string {
	if ip := ifaceIPv4("wlan0"); ip != "" {
		return fmt.Sprintf("http://%s:%d", ip, port)
	}
	if ip := firstLANIPv4("lo", "usb0"); ip != "" {
		return fmt.Sprintf("http://%s:%d", ip, port)
	}
	if ip := ifaceIPv4("usb0"); ip != "" {
		return fmt.Sprintf("http://%s:%d", ip, port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func ifaceIPv4(name string) string {
	iface, err := net.InterfaceByName(name)
	if err != nil || iface.Flags&net.FlagUp == 0 {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	return firstIPv4(addrs)
}

func firstLANIPv4(skip ...string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if ifaceSkipped(iface.Name, skip) {
			continue
		}
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		if ip := firstIPv4(addrs); ip != "" {
			return ip
		}
	}
	return ""
}

func ifaceSkipped(name string, skip []string) bool {
	for _, s := range skip {
		if name == s {
			return true
		}
	}
	return false
}

func firstIPv4(addrs []net.Addr) string {
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return ""
}
