package security

import (
	"strings"
	"testing"
)

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"", false},
		{"user:name", false},
		{"password", true},
		{"PASSWORD", true},
		{"my_password", true},
		{"secret_key", true},
		{"api_key", true},
		{"api-key", true},
		{"apikey", true},
		{"auth_token", true},
		{"credential", true},
		{"private_key", true},
		{"jwt_token", true},
		{"session_id", false},
		{"cache:item:123", false},
	}
	for _, tt := range tests {
		got := IsSensitiveKey(tt.key)
		if got != tt.want {
			t.Errorf("IsSensitiveKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestRedactKey_NonSensitive(t *testing.T) {
	key := "user:profile:123"
	got := RedactKey(key)
	if got != key {
		t.Errorf("expected %q unchanged, got %q", key, got)
	}
}

func TestRedactKey_Sensitive(t *testing.T) {
	key := "my_password"
	got := RedactKey(key)
	if got == key {
		t.Error("expected sensitive key to be redacted")
	}
	if !strings.HasPrefix(got, "REDACTED-") {
		t.Errorf("expected REDACTED- prefix, got %q", got)
	}
	// Should be a fixed hash, not the original key.
	if strings.Contains(got, "my_password") {
		t.Error("redacted key should not contain original value")
	}
}

func TestRedactKey_Sensitive_Consistent(t *testing.T) {
	key := "secret_value"
	r1 := RedactKey(key)
	r2 := RedactKey(key)
	if r1 != r2 {
		t.Errorf("expected consistent redaction: %q != %q", r1, r2)
	}
}

func TestAudit(t *testing.T) {
	// Audit writes to log; just verify it doesn't panic.
	Audit("SET", "user:name", "extra-info")
	Audit("GET", "password", map[string]string{"note": "sensitive"})
	Audit("DELETE", "", "no-key")
}
