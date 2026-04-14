// Package security provides utilities for detecting and redacting sensitive keys in cache operations, as well as
// auditing access to sensitive data. It uses regex patterns to identify sensitive keys and hashes them for redaction in
// logs.
package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

var sensitiveKeyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|secret|token|key|auth|credential|private|api[_-]?key|jwt)`),
}

func IsSensitiveKey(key string) bool {
	if key == "" {
		return false
	}
	lower := strings.ToLower(key)
	for _, re := range sensitiveKeyPatterns {
		if re.MatchString(lower) {
			return true
		}
	}
	return false
}

func RedactKey(key string) string {
	if !IsSensitiveKey(key) {
		return key
	}
	h := sha256.Sum256([]byte(key))
	return "REDACTED-" + hex.EncodeToString(h[:8])
}

func Audit(operation, key string, extra ...any) {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	redactedKey := RedactKey(key)

	msg := fmt.Sprintf("[AUDIT] %s | key=%s | ts=%s", operation, redactedKey, timestamp)
	if len(extra) > 0 {
		msg += fmt.Sprintf(" | extra=%+v", extra)
	}
	log.Printf("%s", msg) // replace with your structured logger (zap/slog) if desired
}
