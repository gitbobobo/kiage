package app

import "strings"

func wlanSanitizeSSID(ssid string) string {
	ssid = strings.TrimSpace(ssid)
	if len(ssid) >= 2 && ssid[0] == '[' && ssid[len(ssid)-1] == ']' {
		ssid = ssid[1 : len(ssid)-1]
	}
	return strings.TrimSpace(ssid)
}
