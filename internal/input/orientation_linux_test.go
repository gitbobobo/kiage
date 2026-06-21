//go:build linux

package input

import "testing"

const sampleInputDevices = `
I: Bus=0019 Vendor=0001 Product=0001 Version=0001
N: Name="cyttsp5m"
H: Handlers=kbd event0 
B: EV=b
B: KEY=1000000000000 0 0 0 0

I: Bus=0018 Vendor=0000 Product=0000 Version=0000
N: Name="gpio-keys"
H: Handlers=event4 
B: EV=10000d
B: ABS=1000000000000

I: Bus=0019 Vendor=0000 Product=0000 Version=0000
N: Name="bma2x2"
H: Handlers=event6 
B: EV=10000d
B: ABS=1000000000000

I: Bus=0019 Vendor=0000 Product=0000 Version=0000
N: Name="bma2x2"
H: Handlers=event7 
B: EV=10000d
B: ABS=1000000000000
`

func TestParseInputDevices(t *testing.T) {
	blocks := parseInputDevices(sampleInputDevices)
	if len(blocks) != 4 {
		t.Fatalf("blocks=%d want 4", len(blocks))
	}
	if blocks[0].name != "cyttsp5m" {
		t.Fatalf("touch name=%q", blocks[0].name)
	}
	if blocks[1].name != "bma2x2" {
		t.Fatalf("accel name=%q", blocks[1].name)
	}
}

func TestFindAccelDevicePathFromSample(t *testing.T) {
	blocks := parseInputDevices(sampleInputDevices)
	var bestNamed struct {
		path string
		name string
		num  int
	}
	for _, b := range blocks {
		if !blockHasAbsEV(b) {
			continue
		}
		path := eventPathFromHandlers(b.handlers)
		if path == "" || isExcludedInputName(b.name) {
			continue
		}
		if isNamedAccelDevice(b.name) {
			num := inputEventNum(path)
			if num > bestNamed.num {
				bestNamed = struct {
					path string
					name string
					num  int
				}{path, b.name, num}
			}
		}
	}
	if bestNamed.path != "/dev/input/event7" || bestNamed.name != "bma2x2" {
		t.Fatalf("picked=%q name=%q want event7 bma2x2", bestNamed.path, bestNamed.name)
	}
}

func TestMapAbsGyroValueOasis23(t *testing.T) {
	cases := []struct {
		val  int
		rota int
		ok   bool
	}{
		{15, 0, true},
		{16, 2, true},
		{17, 0, false},
		{18, 0, false},
	}
	for _, c := range cases {
		rota, ok := mapAbsGyroValue(gyroGenOasis23, c.val)
		if ok != c.ok || (ok && rota != c.rota) {
			t.Fatalf("val=%d got rota=%d ok=%v want rota=%d ok=%v", c.val, rota, ok, c.rota, c.ok)
		}
	}
}

func TestMapAbsGyroValueOasis1(t *testing.T) {
	rota, ok := mapAbsGyroValue(gyroGenOasis1, 17)
	if !ok || rota != 0 {
		t.Fatalf("Oasis1 val=17 got rota=%d ok=%v want 0 true", rota, ok)
	}
	_, ok = mapAbsGyroValue(gyroGenOasis23, 17)
	if ok {
		t.Fatal("Oasis23 val=17 should be ignored")
	}
	rota, ok = mapAbsGyroValue(gyroGenOasis1, 20)
	if !ok || rota != 2 {
		t.Fatalf("Oasis1 val=20 got rota=%d ok=%v", rota, ok)
	}
}

func TestOrientationListenerParseEvent(t *testing.T) {
	l := &OrientationListener{gen: gyroGenOasis23}
	buf := make([]byte, inputEventSize)
	// EV_ABS, ABS_PRESSURE, value 16
	binaryPutEvent(buf, evAbs, absPressure, 16)
	rota, ok := l.parseEvent(buf)
	if !ok || rota != 2 {
		t.Fatalf("parse got rota=%d ok=%v want 2 true", rota, ok)
	}
}

func binaryPutEvent(buf []byte, typ, code uint16, val int32) {
	buf[8] = byte(typ)
	buf[9] = 0
	buf[10] = byte(code)
	buf[11] = 0
	buf[12] = byte(val)
	buf[13] = byte(val >> 8)
	buf[14] = byte(val >> 16)
	buf[15] = byte(val >> 24)
}
