package unsafeopt

import (
	"testing"
)

func TestBytesToString(t *testing.T) {
	got := BytesToString([]byte("hello"))
	if got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestBytesToString_Empty(t *testing.T) {
	got := BytesToString([]byte{})
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestBytesToString_Unicode(t *testing.T) {
	got := BytesToString([]byte("café"))
	if got != "café" {
		t.Errorf("got %q, want café", got)
	}
}

func TestStringToBytes(t *testing.T) {
	got := StringToBytes("hello")
	if string(got) != "hello" {
		t.Errorf("got %q, want hello", string(got))
	}
}

func TestStringToBytes_Empty(t *testing.T) {
	got := StringToBytes("")
	if len(got) != 0 {
		t.Errorf("got %d bytes, want 0", len(got))
	}
}

func TestStringToBytes_Unicode(t *testing.T) {
	got := StringToBytes("café")
	if string(got) != "café" {
		t.Errorf("got %q, want café", string(got))
	}
}

func TestRoundtrip(t *testing.T) {
	original := "the quick brown fox jumps over the lazy dog"
	got := BytesToString(StringToBytes(original))
	if got != original {
		t.Errorf("roundtrip failed: got %q, want %q", got, original)
	}
}
