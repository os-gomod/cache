// Package unsafeopt provides zero-allocation conversions between []byte and
// string using unsafe. These operations are safe only when the source data is
// not modified for the lifetime of the returned value (or when the returned
// value is used only for read operations).
//
// Use with caution: these functions bypass the garbage collector's ability to
// track mutations. They are intended exclusively for the codec layer where
// cached byte slices are read-only after storage.
package unsafeopt

import (
	"unsafe"
)

// BytesToString converts a []byte to a string without allocating. The caller
// must ensure the byte slice is not modified for the lifetime of the returned
// string. If b is empty, an empty string is returned (no unsafe operation).
func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToBytes converts a string to a []byte without allocating. The caller
// must ensure the returned byte slice is never modified. If s is empty, an
// empty (non-nil) byte slice is returned (no unsafe operation).
func StringToBytes(s string) []byte {
	if s == "" {
		return []byte{}
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
