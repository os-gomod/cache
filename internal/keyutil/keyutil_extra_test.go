package keyutil

import "testing"

func TestStampedeLockKey_WithPrefix(t *testing.T) {
	got := StampedeLockKey("cache:", "user:1")
	want := "cache:user:1:__lock__"
	if got != want {
		t.Errorf("StampedeLockKey = %q, want %q", got, want)
	}
}

func TestStampedeLockKey_EmptyPrefix(t *testing.T) {
	got := StampedeLockKey("", "user:1")
	want := "user:1:__lock__"
	if got != want {
		t.Errorf("StampedeLockKey = %q, want %q", got, want)
	}
}

func TestStampedeLockKey_EmptyKey(t *testing.T) {
	got := StampedeLockKey("prefix:", "")
	want := "prefix::__lock__"
	if got != want {
		t.Errorf("StampedeLockKey = %q, want %q", got, want)
	}
}

func TestValidateKey_EmptyKey(t *testing.T) {
	err := ValidateKey("test", "")
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestValidateKey_NonEmptyKey(t *testing.T) {
	err := ValidateKey("test", "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildKey_OnlyKey(t *testing.T) {
	got := BuildKey("", "user:1")
	if got != "user:1" {
		t.Errorf("BuildKey = %q, want %q", got, "user:1")
	}
}

func TestStripPrefix_KeyShorterThanPrefix(t *testing.T) {
	got := StripPrefix("long_prefix", "short")
	if got != "short" {
		t.Errorf("StripPrefix = %q, want %q", got, "short")
	}
}

func TestStripPrefix_KeySameLength(t *testing.T) {
	got := StripPrefix("abc", "abc")
	if got != "abc" {
		t.Errorf("StripPrefix = %q, want %q", got, "abc")
	}
}

func TestStripPrefix_EmptyPrefix(t *testing.T) {
	got := StripPrefix("", "user:1")
	if got != "user:1" {
		t.Errorf("StripPrefix = %q, want %q", got, "user:1")
	}
}
