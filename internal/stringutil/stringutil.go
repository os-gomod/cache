// Package stringutil provides shared string manipulation utilities used
// across the cache library.
package stringutil

// TruncateKey truncates a key to maxLen characters, appending "..." if truncated.
// If maxLen <= 3, the key is simply sliced to that length without an ellipsis.
func TruncateKey(key string, maxLen int) string {
	if len(key) <= maxLen {
		return key
	}
	if maxLen <= 3 {
		return key[:maxLen]
	}
	return key[:maxLen-3] + "..."
}

// IsReadOp reports whether the operation name corresponds to a read operation.
// This is used by observability interceptors to determine whether to record
// hit/miss metrics.
func IsReadOp(name string) bool {
	switch name {
	case "get", "get_multi", "get_or_set", "getset", "exists":
		return true
	}
	return false
}
