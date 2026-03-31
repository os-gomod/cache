// Package strutil_test provides tests for allocation-free string conversion helpers.
package strutil_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/os-gomod/cache/internal/strutil"
)

func TestItoa(t *testing.T) {
	tests := []struct {
		name string
		i    int64
		want string
	}{
		{"zero", 0, "0"},
		{"positive single digit", 5, "5"},
		{"positive two digits", 42, "42"},
		{"positive three digits", 123, "123"},
		{"positive four digits", 1234, "1234"},
		{"positive five digits", 12345, "12345"},
		{"positive six digits", 123456, "123456"},
		{"positive seven digits", 1234567, "1234567"},
		{"positive eight digits", 12345678, "12345678"},
		{"positive nine digits", 123456789, "123456789"},
		{"positive ten digits", 1234567890, "1234567890"},
		{"positive eleven digits", 12345678901, "12345678901"},
		{"positive twelve digits", 123456789012, "123456789012"},
		{"positive thirteen digits", 1234567890123, "1234567890123"},
		{"positive fourteen digits", 12345678901234, "12345678901234"},
		{"positive fifteen digits", 123456789012345, "123456789012345"},
		{"positive sixteen digits", 1234567890123456, "1234567890123456"},
		{"positive seventeen digits", 12345678901234567, "12345678901234567"},
		{"positive eighteen digits", 123456789012345678, "123456789012345678"},
		{"positive max int64", math.MaxInt64, "9223372036854775807"},
		{"negative single digit", -5, "-5"},
		{"negative two digits", -42, "-42"},
		{"negative three digits", -123, "-123"},
		{"negative four digits", -1234, "-1234"},
		{"negative five digits", -12345, "-12345"},
		{"negative six digits", -123456, "-123456"},
		{"negative seven digits", -1234567, "-1234567"},
		{"negative eight digits", -12345678, "-12345678"},
		{"negative nine digits", -123456789, "-123456789"},
		{"negative ten digits", -1234567890, "-1234567890"},
		{"negative eleven digits", -12345678901, "-12345678901"},
		{"negative twelve digits", -123456789012, "-123456789012"},
		{"negative thirteen digits", -1234567890123, "-1234567890123"},
		{"negative fourteen digits", -12345678901234, "-12345678901234"},
		{"negative fifteen digits", -123456789012345, "-123456789012345"},
		{"negative sixteen digits", -1234567890123456, "-1234567890123456"},
		{"negative seventeen digits", -12345678901234567, "-12345678901234567"},
		{"negative eighteen digits", -123456789012345678, "-123456789012345678"},
		{"negative min int64", math.MinInt64, "-9223372036854775808"},
		{"one", 1, "1"},
		{"negative one", -1, "-1"},
		{"ten", 10, "10"},
		{"negative ten", -10, "-10"},
		{"hundred", 100, "100"},
		{"negative hundred", -100, "-100"},
		{"thousand", 1000, "1000"},
		{"negative thousand", -1000, "-1000"},
		{"million", 1000000, "1000000"},
		{"negative million", -1000000, "-1000000"},
		{"billion", 1000000000, "1000000000"},
		{"negative billion", -1000000000, "-1000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strutil.Itoa(tt.i); got != tt.want {
				t.Errorf("Itoa(%d) = %v, want %v", tt.i, got, tt.want)
			}
		})
	}
}

func TestItoa_ComparisonWithStrconv(t *testing.T) {
	tests := []int64{
		0,
		1,
		-1,
		10,
		-10,
		100,
		-100,
		1000,
		-1000,
		12345,
		-12345,
		123456789,
		-123456789,
		math.MaxInt32,
		math.MinInt32,
		math.MaxInt64,
		math.MinInt64,
	}

	for _, i := range tests {
		t.Run(strconv.FormatInt(i, 10), func(t *testing.T) {
			expected := strconv.FormatInt(i, 10)
			if got := strutil.Itoa(i); got != expected {
				t.Errorf("Itoa(%d) = %v, want %v", i, got, expected)
			}
		})
	}
}

func TestItoa_AllDigits(t *testing.T) {
	// Test all single digits
	for i := int64(0); i <= 9; i++ {
		expected := strconv.FormatInt(i, 10)
		if got := strutil.Itoa(i); got != expected {
			t.Errorf("Itoa(%d) = %v, want %v", i, got, expected)
		}
	}

	// Test all negative single digits
	for i := int64(-9); i < 0; i++ {
		expected := strconv.FormatInt(i, 10)
		if got := strutil.Itoa(i); got != expected {
			t.Errorf("Itoa(%d) = %v, want %v", i, got, expected)
		}
	}
}

func TestItoa_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		i    int64
	}{
		{"max int64", math.MaxInt64},
		{"min int64", math.MinInt64},
		{"max int64 - 1", math.MaxInt64 - 1},
		{"min int64 + 1", math.MinInt64 + 1},
		{"max int32", math.MaxInt32},
		{"min int32", math.MinInt32},
		{"max int16", math.MaxInt16},
		{"min int16", math.MinInt16},
		{"max int8", math.MaxInt8},
		{"min int8", math.MinInt8},
		{"zero", 0},
		{"one", 1},
		{"negative one", -1},
		{"ten", 10},
		{"negative ten", -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := strconv.FormatInt(tt.i, 10)
			if got := strutil.Itoa(tt.i); got != expected {
				t.Errorf("Itoa(%d) = %v, want %v", tt.i, got, expected)
			}
		})
	}
}

func TestItoa_NoAllocations(t *testing.T) {
	// This test verifies that the function doesn't allocate on the heap
	allocs := testing.AllocsPerRun(100, func() {
		_ = strutil.Itoa(12345)
	})

	if allocs > 0 {
		t.Errorf("Itoa allocated %f heap objects, expected 0", allocs)
	}
}

func TestItoa_NoAllocationsNegative(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_ = strutil.Itoa(-12345)
	})

	if allocs > 0 {
		t.Errorf("Itoa allocated %f heap objects, expected 0", allocs)
	}
}

func TestItoa_NoAllocationsMaxInt64(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_ = strutil.Itoa(math.MaxInt64)
	})

	if allocs > 0 {
		t.Errorf("Itoa allocated %f heap objects, expected 0", allocs)
	}
}

func TestItoa_NoAllocationsMinInt64(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_ = strutil.Itoa(math.MinInt64)
	})

	if allocs > 0 {
		t.Errorf("Itoa allocated %f heap objects, expected 0", allocs)
	}
}

func TestItoa_NoAllocationsZero(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_ = strutil.Itoa(0)
	})

	if allocs > 0 {
		t.Errorf("Itoa allocated %f heap objects, expected 0", allocs)
	}
}

func TestItoa_BufferSize(t *testing.T) {
	// Test that the buffer size (20 bytes) is sufficient for all int64 values
	// Max int64: 9223372036854775807 (19 digits + possible sign)
	// Min int64: -9223372036854775808 (20 chars including sign)

	maxStr := strutil.Itoa(math.MaxInt64)
	if len(maxStr) != 19 {
		t.Errorf("MaxInt64 string length = %d, want 19", len(maxStr))
	}

	minStr := strutil.Itoa(math.MinInt64)
	if len(minStr) != 20 {
		t.Errorf("MinInt64 string length = %d, want 20", len(minStr))
	}
}

func TestItoa_StringContent(t *testing.T) {
	// Verify that the string contains only digits and optional leading minus
	testCases := []int64{
		0, 1, -1, 123, -123, 999, -999, 1000, -1000, math.MaxInt64, math.MinInt64,
	}

	for _, i := range testCases {
		t.Run(strconv.FormatInt(i, 10), func(t *testing.T) {
			s := strutil.Itoa(i)

			// Check first character
			if i < 0 {
				if s[0] != '-' {
					t.Errorf("negative number should start with '-', got %c", s[0])
				}
				// Check remaining characters are digits
				for j := 1; j < len(s); j++ {
					if s[j] < '0' || s[j] > '9' {
						t.Errorf("non-digit character %c at position %d", s[j], j)
					}
				}
			} else {
				// Check all characters are digits
				for j := 0; j < len(s); j++ {
					if s[j] < '0' || s[j] > '9' {
						t.Errorf("non-digit character %c at position %d", s[j], j)
					}
				}
			}
		})
	}
}

func TestItoa_NoLeadingZeros(t *testing.T) {
	testCases := []int64{
		0, 1, 10, 100, 1000, 123, 4567, math.MaxInt64, math.MinInt64,
	}

	for _, i := range testCases {
		t.Run(strconv.FormatInt(i, 10), func(t *testing.T) {
			s := strutil.Itoa(i)

			if i == 0 {
				if s != "0" {
					t.Errorf("zero should be '0', got %q", s)
				}
				return
			}

			// Check for leading zeros
			if i > 0 && s[0] == '0' {
				t.Errorf("positive number has leading zero: %q", s)
			}
			if i < 0 && len(s) > 1 && s[1] == '0' {
				t.Errorf("negative number has leading zero: %q", s)
			}
		})
	}
}

func BenchmarkItoa(b *testing.B) {
	benchmarks := []struct {
		name string
		i    int64
	}{
		{"Zero", 0},
		{"Positive_SingleDigit", 5},
		{"Positive_TwoDigits", 42},
		{"Positive_ThreeDigits", 123},
		{"Positive_FourDigits", 1234},
		{"Positive_FiveDigits", 12345},
		{"Positive_SixDigits", 123456},
		{"Positive_SevenDigits", 1234567},
		{"Positive_EightDigits", 12345678},
		{"Positive_NineDigits", 123456789},
		{"Positive_TenDigits", 1234567890},
		{"Positive_ElevenDigits", 12345678901},
		{"Positive_TwelveDigits", 123456789012},
		{"Positive_ThirteenDigits", 1234567890123},
		{"Positive_FourteenDigits", 12345678901234},
		{"Positive_FifteenDigits", 123456789012345},
		{"Positive_SixteenDigits", 1234567890123456},
		{"Positive_SeventeenDigits", 12345678901234567},
		{"Positive_EighteenDigits", 123456789012345678},
		{"Positive_MaxInt64", math.MaxInt64},
		{"Negative_SingleDigit", -5},
		{"Negative_TwoDigits", -42},
		{"Negative_ThreeDigits", -123},
		{"Negative_FourDigits", -1234},
		{"Negative_FiveDigits", -12345},
		{"Negative_SixDigits", -123456},
		{"Negative_SevenDigits", -1234567},
		{"Negative_EightDigits", -12345678},
		{"Negative_NineDigits", -123456789},
		{"Negative_TenDigits", -1234567890},
		{"Negative_ElevenDigits", -12345678901},
		{"Negative_TwelveDigits", -123456789012},
		{"Negative_ThirteenDigits", -1234567890123},
		{"Negative_FourteenDigits", -12345678901234},
		{"Negative_FifteenDigits", -123456789012345},
		{"Negative_SixteenDigits", -1234567890123456},
		{"Negative_SeventeenDigits", -12345678901234567},
		{"Negative_EighteenDigits", -123456789012345678},
		{"Negative_MinInt64", math.MinInt64},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = strutil.Itoa(bm.i)
			}
		})
	}
}

func BenchmarkStrconvItoa(b *testing.B) {
	benchmarks := []struct {
		name string
		i    int64
	}{
		{"Zero", 0},
		{"Positive_SingleDigit", 5},
		{"Positive_TwoDigits", 42},
		{"Positive_ThreeDigits", 123},
		{"Positive_FourDigits", 1234},
		{"Positive_FiveDigits", 12345},
		{"Positive_SixDigits", 123456},
		{"Positive_SevenDigits", 1234567},
		{"Positive_EightDigits", 12345678},
		{"Positive_NineDigits", 123456789},
		{"Positive_TenDigits", 1234567890},
		{"Positive_ElevenDigits", 12345678901},
		{"Positive_TwelveDigits", 123456789012},
		{"Positive_ThirteenDigits", 1234567890123},
		{"Positive_FourteenDigits", 12345678901234},
		{"Positive_FifteenDigits", 123456789012345},
		{"Positive_SixteenDigits", 1234567890123456},
		{"Positive_SeventeenDigits", 12345678901234567},
		{"Positive_EighteenDigits", 123456789012345678},
		{"Positive_MaxInt64", math.MaxInt64},
		{"Negative_SingleDigit", -5},
		{"Negative_TwoDigits", -42},
		{"Negative_ThreeDigits", -123},
		{"Negative_FourDigits", -1234},
		{"Negative_FiveDigits", -12345},
		{"Negative_SixDigits", -123456},
		{"Negative_SevenDigits", -1234567},
		{"Negative_EightDigits", -12345678},
		{"Negative_NineDigits", -123456789},
		{"Negative_TenDigits", -1234567890},
		{"Negative_ElevenDigits", -12345678901},
		{"Negative_TwelveDigits", -123456789012},
		{"Negative_ThirteenDigits", -1234567890123},
		{"Negative_FourteenDigits", -12345678901234},
		{"Negative_FifteenDigits", -123456789012345},
		{"Negative_SixteenDigits", -1234567890123456},
		{"Negative_SeventeenDigits", -12345678901234567},
		{"Negative_EighteenDigits", -123456789012345678},
		{"Negative_MinInt64", math.MinInt64},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = strconv.FormatInt(bm.i, 10)
			}
		})
	}
}

func BenchmarkItoa_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := int64(0)
		for pb.Next() {
			_ = strutil.Itoa(i)
			i++
			if i > 1000000 {
				i = 0
			}
		}
	})
}

func BenchmarkStrconvItoa_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := int64(0)
		for pb.Next() {
			_ = strconv.FormatInt(i, 10)
			i++
			if i > 1000000 {
				i = 0
			}
		}
	})
}
