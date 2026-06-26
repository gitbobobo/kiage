//go:build linux

package app

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/godbobo/kiage/internal/log"
)

const (
	rtcWakeSlackSec  = 300 // 允许闹钟略早/略晚；窗口约 [RTC-300, RTC+600]s
	rtcWakeDefer     = 800 * time.Millisecond
	rtcInputSuppress = 15 * time.Second // 仅抑制 RTC 背景同步期间的误触单键
)

// 默认关闭休眠期间 RTC 后台同步，仅保留用户唤醒(outOfScreenSaver)后的自动同步。
// 如需恢复旧行为，可设置 KIAGE_ENABLE_RTC_SYNC=1。
func rtcSyncEnabled() bool {
	return os.Getenv("KIAGE_ENABLE_RTC_SYNC") == "1"
}

// kindleRTCSec 为 RTC 周期唤醒间隔（秒），默认 3600。可用环境变量 KIAGE_RTC_SEC
// 覆盖（用于快速验证休眠联网：设短间隔后放置设备勿动即可抓到完整周期）。
var kindleRTCSec = rtcSecFromEnv()

func rtcSecFromEnv() int {
	if v := os.Getenv("KIAGE_RTC_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 60 {
			return n
		}
	}
	return 3600
}

type kindleUIState struct {
	awesomeStopped bool
	pillowDisabled bool
}

func (a *App) initKindleUIState() {
	a.kindleUI.awesomeStopped = os.Getenv("KIAGE_AWESOME_STOPPED") == "yes"
	a.kindleUI.pillowDisabled = os.Getenv("KIAGE_PILLOW_DISABLED") == "yes"
}

func (a *App) runPowerManager(ctx context.Context) {
	a.initKindleUIState()
	for {
		if ctx.Err() != nil {
			return
		}
		if err := a.watchPowerd(ctx); err != nil && ctx.Err() == nil {
			log.Warn("powerd watch ended: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (a *App) watchPowerd(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lipc-wait-event", "-m", "com.lab126.powerd",
		"goingToScreenSaver,outOfScreenSaver,readyToSuspend,wakeupFromSuspend")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() { _ = cmd.Wait() }()

	log.Info("powerd watch started")
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		a.handlePowerdLine(sc.Text())
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return io.EOF
}

func (a *App) handlePowerdLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	log.Info("powerd event %q", line)
	kind := a.classifyPowerdLine(line)
	switch kind {
	case "suspend":
		a.onEnteringSuspend()
	case "resume":
		if a.cancelPendingRTCWake() {
			log.Info("kindle rtc wake cancelled user outOfScreenSaver")
		}
		a.clearInputSuppress()
		a.onResume(syncCauseWake, 0)
	case "rtc":
		slept, ok := parseWakeupSuspendSec(line)
		if !ok {
			return
		}
		if !rtcSyncEnabled() {
			log.Info("kindle rtc wake ignored disabled suspend=%ds", slept)
			return
		}
		// 注意：挂起后 powerd 会先有一次 ~90s 的"打盹"检查点唤醒，这并不会消耗 RTC 闹钟，
		// 设备随后会带着原闹钟进入深度休眠。切勿在此重设闹钟——每次重设都会让 powerd 从
		// 90s 检查点重新开始，导致设备永远进不了深睡（90s 唤醒死循环、狂耗电）。
		if !rtcWakeLooksScheduled(slept, kindleRTCSec, rtcWakeSlackSec) {
			if slept < kindleRTCSec-rtcWakeSlackSec {
				log.Info("kindle rtc wake ignored short suspend=%ds", slept)
				a.suppressInputFor(15 * time.Second)
			} else {
				log.Info("kindle rtc wake ignored off-schedule suspend=%ds window=%d-%ds",
					slept, kindleRTCSec-rtcWakeSlackSec, kindleRTCSec+600)
			}
			return
		}
		log.Info("kindle rtc wake armed defer=%s suspend=%ds", rtcWakeDefer, slept)
		a.armPendingRTCWake(slept)
		time.AfterFunc(rtcWakeDefer, func() {
			if slept, ok := a.takePendingRTCWake(); ok {
				a.onResume(syncCauseRTC, slept)
			}
		})
	case "ready":
		a.onReadyToSuspend(line)
	}
}

func (a *App) onEnteringSuspend() {
	log.Info("kindle suspend enter")
	a.rtcArmed.Store(false)
	a.stopSettingsServer()
}

func (a *App) onReadyToSuspend(line string) {
	delay := parseReadyToSuspendDelay(line)
	if delay > 1 {
		return
	}
	if !rtcSyncEnabled() {
		if a.rtcArmed.Load() {
			a.rtcArmed.Store(false)
		}
		clearRTCWakeup()
		log.Info("rtcWakeup disabled delay=%d", delay)
		return
	}
	// 注意：不再因 rtcMaintaining 跳过设闹钟。RTC 维护性同步期间 powerd 仍会发出
	// readyToSuspend 倒计时；若此时跳过，待同步结束设备直接挂起且不再产生新的
	// readyToSuspend，闹钟便永远设不上（设备深睡不再被周期唤醒）。设闹钟本身不会
	// 触发挂起（preventScreenSaver 仍生效），因此在挂起窗口内设置是安全且可靠的。
	if a.rtcArmed.Load() {
		return
	}
	clearRTCWakeup()
	if err := setRTCWakeup(kindleRTCSec); err != nil {
		log.Warn("rtcWakeup set failed: %v", err)
		if err2 := setRTCWakeup(kindleRTCSec); err2 != nil {
			log.Warn("rtcWakeup retry failed: %v", err2)
			return
		}
	}
	a.rtcArmed.Store(true)
	log.Info("rtcWakeup set sec=%d delay=%d maintaining=%v", kindleRTCSec, delay, a.rtcMaintaining.Load())
}

func (a *App) onResume(cause string, sleptSec int) {
	log.Info("kindle resume cause=%s slept=%d", cause, sleptSec)
	if cause == syncCauseWake {
		a.clearInputSuppress()
	}
	if cause == syncCauseRTC {
		a.rtcArmed.Store(false)
	}
	if cause == syncCauseWake && a.rtcMaintaining.Load() {
		log.Info("kindle user wake during rtc sync")
		a.clearInputSuppress()
		return
	}
	a.restoreKindleUI()
	if cause == syncCauseRTC {
		a.suppressInputFor(rtcInputSuppress)
		a.rtcMaintaining.Store(true)
		go func() {
			defer func() {
				a.rtcMaintaining.Store(false)
				a.clearInputSuppress()
			}()
			_ = a.syncAllProvidersBatch(context.Background(), syncCauseRTC)
		}()
		return
	}
	if cause == syncCauseWake && !a.globalSyncStale() {
		log.Info("kindle resume sync skipped fresh")
		go func() {
			time.Sleep(350 * time.Millisecond)
			a.refreshFrameOpts(false, false, true)
		}()
		return
	}
	go func() {
		_ = a.syncAllProvidersBatch(context.Background(), cause)
	}()
}

func (a *App) restoreKindleUI() {
	if a.kindleUI.awesomeStopped {
		_ = exec.Command("killall", "-STOP", "awesome").Run()
	}
	if a.kindleUI.pillowDisabled {
		_ = exec.Command("lipc-set-prop", "com.lab126.pillow", "disableEnablePillow", "disable").Run()
	}
}

func setRTCWakeup(sec int) error {
	clearRTCWakeup()
	return exec.Command("lipc-set-prop", "-i", "com.lab126.powerd", "rtcWakeup", strconv.Itoa(sec)).Run()
}

func clearRTCWakeup() {
	_ = exec.Command("lipc-set-prop", "-i", "com.lab126.powerd", "rtcWakeup", "0").Run()
}

func maybeKeepScreenAwake() {
	if os.Getenv("KIAGE_KEEP_AWAKE") == "1" {
		_ = exec.Command("lipc-set-prop", "com.lab126.powerd", "preventScreenSaver", "1").Run()
		log.Info("preventScreenSaver enabled (KIAGE_KEEP_AWAKE=1)")
	}
}

func releaseScreenAwake() {
	_ = exec.Command("lipc-set-prop", "com.lab126.powerd", "preventScreenSaver", "0").Run()
	clearRTCWakeup()
}

// rtcKeepAwake 在 RTC 同步期间阻止再次进入屏保/休眠，否则 wifid 无法从 NA 拉起。
func rtcKeepAwake(enable bool) {
	if os.Getenv("KIAGE_KEEP_AWAKE") == "1" {
		return
	}
	v := "0"
	if enable {
		v = "1"
		powerDeferSuspend(120)
	}
	_ = exec.Command("lipc-set-prop", "com.lab126.powerd", "preventScreenSaver", v).Run()
	if enable {
		log.Info("preventScreenSaver enabled for rtc sync")
	}
}
