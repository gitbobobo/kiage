//go:build linux

package input

import "unsafe"

// EVIOCGABS / EVIOCGRAB 在 golang.org/x/sys/unix 中未导出，按 linux/input.h 自行构造。
const (
	iocNRBits   = 8
	iocTypeBits = 8
	iocSizeBits = 14

	iocNRShift   = 0
	iocTypeShift = iocNRBits
	iocSizeShift = iocNRShift + iocTypeBits
	iocDirShift  = iocSizeShift + iocSizeBits

	iocRead = 2
	iocWrite = 1
)

func ioctlIOR(typ, nr, size uintptr) uintptr {
	dir := uintptr(iocRead)
	return (dir << iocDirShift) | (typ << iocTypeShift) | (nr << iocNRShift) | (size << iocSizeShift)
}

func ioctlIOW(typ, nr, size uintptr) uintptr {
	dir := uintptr(iocWrite)
	return (dir << iocDirShift) | (typ << iocTypeShift) | (nr << iocNRShift) | (size << iocSizeShift)
}

func eviocgabs(abs int) uintptr {
	return ioctlIOR(uintptr('E'), 0x40+uintptr(abs), unsafe.Sizeof(inputAbsinfo{}))
}

func eviocgrab() uintptr {
	return ioctlIOW(uintptr('E'), 0x90, unsafe.Sizeof(int32(0)))
}
