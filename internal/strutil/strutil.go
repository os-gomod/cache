// Package strutil provides allocation-free string conversion helpers.
package strutil

// Itoa converts an int64 to its decimal string representation without
// importing strconv or fmt, keeping the hot path allocation-free on the stack.
func Itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	for i != 0 {
		pos--
		d := i % 10
		if d < 0 {
			d = -d
		}
		buf[pos] = byte('0' + d)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
