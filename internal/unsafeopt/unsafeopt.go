// Package unsafeopt provides zero-allocation byte/string conversion utilities using unsafe.
package unsafeopt

import (
	"unsafe"
)

func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func StringToBytes(s string) []byte {
	if s == "" {
		return []byte{}
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
