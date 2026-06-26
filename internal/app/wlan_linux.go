//go:build linux

package app

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbobo/kiage/internal/log"
)

const (
	wlanConnectWait  = 90 * time.Second
	wlanPollInterval = 500 * time.Millisecond
	wlanRTCSettle    = 8 * time.Second
)

var (
	wlanMu            sync.Mutex
	wlanOpenedByKiage bool
	wlanEnabledAt     time.Time
)

func wlanConnected() bool {
	if lipcEquals("com.lab126.wifid", "cmState", "CONNECTED") {
		return true
	}
	if lipcContains("com.lab126.wifid", "cmState", "connected") {
		return true
	}
	if lipcContains("com.lab126.wifid", "state", "connected") {
		return true
	}
	return lipcContains("com.lab126.wan", "state", "connected")
}

func wlanConnectedReady() bool {
	if _, err := os.Stat("/sys/class/net/wlan0"); err != nil {
		return false
	}
	if !wlanOperstateUp() {
		return false
	}
	ip := ifaceIPv4("wlan0")
	if ip == "" {
		return false
	}
	cm := wlanCmState()
	if cm == "CONNECTED" || cm == "" || wlanConnected() {
		return true
	}
	// 深度休眠 RTC 唤醒后 wifid 的 cmState 常滞留在 "NA"。此时即便 wpa 已完成关联、
	// 接口已有 IP，也可能尚未建立默认路由（外网不可达），仅凭 cmState/wpa 会误判为
	// 就绪，导致后台同步全部失败。对 NA 状态改用实测：wpa 完成 + 存在默认路由 +
	// 网关可达，才认为链路真正可用。
	if !wlanWpaCompleted() {
		return false
	}
	return wlanGatewayReachable()
}

// defaultRouteGateway 解析 /proc/net/route，返回 wlan0 默认路由(0.0.0.0)的网关 IP；无则空。
func defaultRouteGateway() string {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) < 4 || f[0] != "wlan0" {
			continue
		}
		if f[1] != "00000000" { // 目的地 0.0.0.0 即默认路由
			continue
		}
		if gw := parseHexIPLE(f[2]); gw != "" && gw != "0.0.0.0" {
			return gw
		}
	}
	return ""
}

// parseHexIPLE 将 /proc/net/route 中的小端十六进制 IP（如 0101A8C0）转为点分十进制。
func parseHexIPLE(h string) string {
	if len(h) != 8 {
		return ""
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[3], b[2], b[1], b[0])
}

// wlanGatewayReachable 实测链路是否真正可路由：要求存在默认路由且网关 TCP 可达。
// cmState=NA 但已拿到 IP 时，借此区分「仅有 IP 无路由」与「真正可上网」。
func wlanGatewayReachable() bool {
	gw := defaultRouteGateway()
	if gw == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(gw, "53"), 1500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true
	}
	// 连接被拒绝(RST) 说明网关主机可达、路由正常，只是 53 端口未监听。
	return strings.Contains(strings.ToLower(err.Error()), "refused")
}

func wlanNetworkReady() bool {
	if ifaceIPv4("wlan0") == "" {
		return false
	}
	cm := wlanCmState()
	if cm == "CONNECTED" {
		return true
	}
	return wlanConnectedReady()
}

func wlanOperstateUp() bool {
	b, err := os.ReadFile("/sys/class/net/wlan0/operstate")
	if err != nil {
		return false
	}
	switch strings.TrimSpace(string(b)) {
	case "up", "unknown":
		return true
	default:
		return false
	}
}

func wlanCmState() string {
	out, err := exec.Command("lipc-get-prop", "com.lab126.wifid", "cmState").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func lipcEquals(service, prop, want string) bool {
	out, err := exec.Command("lipc-get-prop", service, prop).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == want
}

func lipcContains(service, prop, want string) bool {
	out, err := exec.Command("lipc-get-prop", service, prop).Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(string(out))), want)
}

func wlanApplyOn() {
	_ = exec.Command("lipc-set-prop", "com.lab126.cmd", "wirelessEnable", "1").Run()
	_ = exec.Command("lipc-set-prop", "-i", "com.lab126.wifid", "enable", "1").Run()
}

func wlanApplyOff() {
	_ = exec.Command("lipc-set-prop", "-i", "com.lab126.wifid", "enable", "0").Run()
	_ = exec.Command("lipc-set-prop", "com.lab126.cmd", "wirelessEnable", "0").Run()
}

func wlanEnsureOn() {
	wlanMu.Lock()
	defer wlanMu.Unlock()
	wlanApplyOn()
	wlanOpenedByKiage = true
	wlanEnabledAt = time.Now()
	log.Info("wlan enabled for sync")
}

// wlanEnsureOnAfterResume 深度休眠 RTC 唤醒后仅打开射频，不做先关再开（会致 wifid cmState=NA）。
func wlanEnsureOnAfterResume() {
	wlanMu.Lock()
	defer wlanMu.Unlock()
	wlanApplyOn()
	wlanOpenedByKiage = true
	wlanEnabledAt = time.Now()
	log.Info("wlan enabled for sync after resume")
}

func wlanEnsureOff() {
	wlanMu.Lock()
	defer wlanMu.Unlock()
	if !wlanOpenedByKiage {
		return
	}
	wlanApplyOff()
	wlanOpenedByKiage = false
	wlanEnabledAt = time.Time{}
	log.Info("wlan disabled after sync")
}

func wlanTriggerScan() {
	_ = exec.Command("lipc-set-prop", "-i", "com.lab126.wifid", "scan", "").Run()
	log.Info("wlan scan triggered")
}

func wlanSavedSSID() string {
	out, err := exec.Command("wpa_cli", "-i", "wlan0", "status").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ssid=") {
			continue
		}
		ssid := wlanSanitizeSSID(strings.TrimPrefix(line, "ssid="))
		if ssid != "" && ssid != "N/A" {
			return ssid
		}
	}
	return ""
}

func wlanPreferredSSID() string {
	if ssid := wlanSavedSSID(); ssid != "" {
		return ssid
	}
	out, err := exec.Command("wpa_cli", "-i", "wlan0", "list_networks").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "network id") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			fields = strings.Fields(line)
		}
		if len(fields) >= 2 && fields[1] != "" {
			return wlanSanitizeSSID(fields[1])
		}
	}
	return ""
}

func wlanEnsureConnection(ssid string) {
	if ssid == "" {
		return
	}
	target := "wifi:" + ssid
	_ = exec.Command("lipc-set-prop", "com.lab126.cmd", "ensureConnection", target).Run()
	log.Info("wlan ensureConnection %q", target)
}

func wlanCmConnect(ssid string) {
	if ssid == "" {
		return
	}
	_ = exec.Command("lipc-set-prop", "com.lab126.wifid", "cmConnect", ssid).Run()
	log.Info("wlan cmConnect %q", ssid)
}

func wlanCmCheckConnection() {
	_ = exec.Command("lipc-set-prop", "com.lab126.wifid", "cmCheckConnection", "").Run()
	log.Info("wlan cmCheckConnection")
}

func wlanCmDisconnect() {
	_ = exec.Command("lipc-set-prop", "com.lab126.wifid", "cmDisconnect", "").Run()
	log.Info("wlan cmDisconnect")
}

func wlanWpaReassociate() {
	out, err := exec.Command("wpa_cli", "-i", "wlan0", "reassociate").CombinedOutput()
	log.Info("wlan wpa_cli reassociate: %s err=%v", strings.TrimSpace(string(out)), err)
}

func wlanWpaReconnect() {
	out, err := exec.Command("wpa_cli", "-i", "wlan0", "reconnect").CombinedOutput()
	log.Info("wlan wpa_cli reconnect: %s err=%v", strings.TrimSpace(string(out)), err)
}

func wlanRadioToggle() {
	wlanApplyOff()
	time.Sleep(500 * time.Millisecond)
	wlanApplyOn()
	wlanMu.Lock()
	wlanEnabledAt = time.Now()
	wlanMu.Unlock()
	log.Info("wlan radio toggled")
}

func wlanWpaCompleted() bool {
	out, err := exec.Command("wpa_cli", "-i", "wlan0", "status").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "wpa_state=") {
			return strings.TrimPrefix(line, "wpa_state=") == "COMPLETED"
		}
	}
	return false
}

func wlanRenewDHCP() {
	if path, err := exec.LookPath("udhcpc"); err == nil {
		out, err := exec.Command(path, "-i", "wlan0", "-n", "-q", "-t", "5").CombinedOutput()
		log.Info("wlan udhcpc: %s err=%v", strings.TrimSpace(string(out)), err)
		wlanEnsureDefaultRoute(parseDHCPServerIP(string(out)))
		return
	}
	out, err := exec.Command("wpa_cli", "-i", "wlan0", "dhcp", "renew").CombinedOutput()
	log.Info("wlan dhcp renew: %s err=%v", strings.TrimSpace(string(out)), err)
}

// parseDHCPServerIP 从 udhcpc 输出解析 DHCP 服务器(家用环境通常即默认网关) IP。
func parseDHCPServerIP(out string) string {
	for _, kw := range []string{"obtained from ", "from server "} {
		i := strings.Index(out, kw)
		if i < 0 {
			continue
		}
		fields := strings.FieldsFunc(out[i+len(kw):], func(r rune) bool {
			return !((r >= '0' && r <= '9') || r == '.')
		})
		if len(fields) > 0 && net.ParseIP(fields[0]) != nil {
			return fields[0]
		}
	}
	return ""
}

// wlanEnsureDefaultRoute 在缺少默认路由时用 DHCP 网关补一条 default 路由。
// 深度休眠 RTC 唤醒后 wifid 停在 NA，手动 udhcpc 只配了 DNS、未建默认路由，
// 导致接口有 IP 但外网不可达（gw=""），后台同步全部失败。
func wlanEnsureDefaultRoute(gw string) {
	if defaultRouteGateway() != "" {
		return
	}
	if gw == "" || net.ParseIP(gw) == nil {
		return
	}
	if out, err := exec.Command("ip", "route", "replace", "default", "via", gw, "dev", "wlan0").CombinedOutput(); err == nil {
		log.Info("wlan default route set gw=%s", gw)
		return
	} else {
		out2, err2 := exec.Command("route", "add", "default", "gw", gw, "dev", "wlan0").CombinedOutput()
		if err2 == nil {
			log.Info("wlan default route added gw=%s", gw)
			return
		}
		log.Warn("wlan default route failed gw=%s ip_err=%v(%s) route_err=%v(%s)", gw,
			err, strings.TrimSpace(string(out)), err2, strings.TrimSpace(string(out2)))
	}
}

func powerDeferSuspend(sec int) {
	if sec <= 0 {
		return
	}
	_ = exec.Command("lipc-set-prop", "-i", "com.lab126.powerd", "deferSuspend", strconv.Itoa(sec)).Run()
	log.Info("power deferSuspend sec=%d", sec)
}

// wlanConnectAfterResume 深度休眠唤醒后的联网（对齐 onlinescreensaver / KOReader：走 wifid LIPC，少碰 wpa_cli）。
func wlanConnectAfterResume(ctx context.Context) bool {
	powerDeferSuspend(120)

	ssid := wlanPreferredSSID()
	log.Info("wlan rtc connect begin ssid=%q cmState=%q", ssid, wlanCmState())

	select {
	case <-ctx.Done():
		return false
	case <-time.After(wlanRTCSettle):
	}

	if ssid != "" {
		wlanCmConnect(ssid)
		wlanEnsureConnection(ssid)
	}
	wlanCmCheckConnection()
	wlanTriggerScan()

	start := time.Now()
	deadline := start.Add(wlanConnectWait)
	var (
		escCheck10 bool
		escDisc25  bool
		escRadio45 bool
		escWpa65   bool
		dhcpTried  bool
	)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return false
		}
		if wlanNetworkReady() {
			log.Info("wlan ready ip=%s cmState=%q gw=%q after %s",
				ifaceIPv4("wlan0"), wlanCmState(), defaultRouteGateway(), time.Since(start).Round(time.Second))
			return true
		}

		elapsed := time.Since(start)
		sec := int(elapsed.Seconds())

		if wlanWpaCompleted() && !dhcpTried && (ifaceIPv4("wlan0") == "" || defaultRouteGateway() == "") {
			dhcpTried = true
			log.Info("wlan wpa completed but no ip/route, renew dhcp ip=%q gw=%q",
				ifaceIPv4("wlan0"), defaultRouteGateway())
			wlanRenewDHCP()
			wlanCmCheckConnection()
		}

		if sec >= 10 && !escCheck10 {
			escCheck10 = true
			log.Info("wlan recovery stage 10s cmCheckConnection")
			wlanCmCheckConnection()
			if ssid != "" {
				wlanEnsureConnection(ssid)
			}
		}
		if sec >= 25 && !escDisc25 {
			escDisc25 = true
			log.Info("wlan recovery stage 25s cmDisconnect/cmConnect")
			wlanCmDisconnect()
			time.Sleep(500 * time.Millisecond)
			if ssid != "" {
				wlanCmConnect(ssid)
				wlanEnsureConnection(ssid)
			}
			wlanCmCheckConnection()
			dhcpTried = false
		}
		if sec >= 45 && !escRadio45 {
			escRadio45 = true
			log.Info("wlan recovery stage 45s radio toggle")
			wlanRadioToggle()
			time.Sleep(wlanRTCSettle)
			if ssid != "" {
				wlanCmConnect(ssid)
				wlanEnsureConnection(ssid)
			}
			wlanCmCheckConnection()
			dhcpTried = false
		}
		if sec >= 65 && !escWpa65 {
			escWpa65 = true
			log.Info("wlan recovery stage 65s wpa_cli reconnect (last resort)")
			wlanWpaReconnect()
			wlanCmCheckConnection()
		}

		time.Sleep(wlanPollInterval)
	}

	log.Warn("wlan connect failed after %s cmState=%q ip=%s gw=%q wpa_completed=%v",
		wlanConnectWait, wlanCmState(), ifaceIPv4("wlan0"), defaultRouteGateway(), wlanWpaCompleted())
	if out, err := exec.Command("wpa_cli", "-i", "wlan0", "status").CombinedOutput(); err == nil {
		log.Warn("wlan wpa_cli status: %s", strings.TrimSpace(string(out)))
	}
	return false
}

func waitForWLAN(ctx context.Context, timeout time.Duration) bool {
	wlanMu.Lock()
	enabledAt := wlanEnabledAt
	wlanMu.Unlock()
	if !enabledAt.IsZero() {
		if settle := 10*time.Second - time.Since(enabledAt); settle > 0 {
			log.Info("wlan settle wait %dms", settle.Milliseconds())
			select {
			case <-ctx.Done():
				return false
			case <-time.After(settle):
			}
		}
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return false
		}
		if wlanNetworkReady() {
			log.Info("wlan ready ip=%s cmState=%q", ifaceIPv4("wlan0"), wlanCmState())
			return true
		}
		if wlanCmState() == "READY" && wlanPreferredSSID() != "" {
			wlanCmConnect(wlanPreferredSSID())
			wlanCmCheckConnection()
		}
		time.Sleep(wlanPollInterval)
	}
	log.Warn("wlan wait timeout after %s cmState=%q ip=%s", timeout, wlanCmState(), ifaceIPv4("wlan0"))
	return false
}
