package config

import (
	"fmt"
	"strings"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// Validator is the contract all configuration types must satisfy.
type Validator interface {
	SetDefaults()
	Validate() error
}

// ----------------------------------------------------------------------------
// Base — shared validation helpers embedded in every config struct
// ----------------------------------------------------------------------------

// Base embeds into each config struct and provides uniform validation helpers.
type Base struct {
	opPrefix string
}

// NewBase constructs a Base with an optional operation prefix.
func NewBase(opPrefix ...string) *Base {
	if len(opPrefix) > 0 && opPrefix[0] != "" {
		return &Base{opPrefix: opPrefix[0]}
	}
	return &Base{}
}

func (b *Base) op(name string) string {
	if b.opPrefix != "" {
		return b.opPrefix + "." + name
	}
	return name
}

func (b *Base) validateNonNegative(field string, value int, op string) error {
	if value < 0 {
		return _errors.InvalidConfig(b.op(op),
			fmt.Sprintf("%s cannot be negative (got: %d)", field, value))
	}
	return nil
}

func (b *Base) validatePositive(field string, value int64, op string) error {
	if value <= 0 {
		return _errors.InvalidConfig(b.op(op),
			fmt.Sprintf("%s must be positive (got: %d)", field, value))
	}
	return nil
}

func (b *Base) validatePositiveInt(field string, value int, op string) error {
	return b.validatePositive(field, int64(value), op)
}

func (b *Base) validateDuration(field string, value time.Duration, op string) error {
	if value < 0 {
		return _errors.InvalidConfig(b.op(op),
			fmt.Sprintf("%s cannot be negative (got: %v)", field, value))
	}
	return nil
}

func (b *Base) validateDurationPositive(field string, value time.Duration, op string) error {
	if value <= 0 {
		return _errors.InvalidConfig(b.op(op),
			fmt.Sprintf("%s must be positive (got: %v)", field, value))
	}
	return nil
}

func (b *Base) validateRequired(field, value, op string) error {
	if value == "" {
		return _errors.InvalidConfig(b.op(op), fmt.Sprintf("%s is required", field))
	}
	return nil
}

func (b *Base) validateMin(field string, value, minValue int64, op string) error {
	if value < minValue {
		return _errors.InvalidConfig(b.op(op),
			fmt.Sprintf("%s must be at least %d (got: %d)", field, minValue, value))
	}
	return nil
}

func (b *Base) validateMax(field string, value, maxValue int64, op string) error {
	if value > maxValue {
		return _errors.InvalidConfig(b.op(op),
			fmt.Sprintf("%s must be at most %d (got: %d)", field, maxValue, value))
	}
	return nil
}

func (b *Base) validateOneOf(field string, value any, allowed []any, op string) error {
	for _, v := range allowed {
		if value == v {
			return nil
		}
	}
	return _errors.InvalidConfig(b.op(op),
		fmt.Sprintf("%s must be one of %v (got: %v)", field, allowed, value))
}

// ----------------------------------------------------------------------------
// ValidationErrors — collect multiple errors
// ----------------------------------------------------------------------------

// ValidationErrors aggregates multiple validation _errors.
type ValidationErrors struct {
	errs []error
}

// Add appends err if non-nil.
func (ve *ValidationErrors) Add(err error) {
	if err != nil {
		ve.errs = append(ve.errs, err)
	}
}

// HasErrors reports whether any errors were collected.
func (ve *ValidationErrors) HasErrors() bool { return len(ve.errs) > 0 }

// Error implements the error interface by joining messages with "; ".
func (ve *ValidationErrors) Error() string {
	msgs := make([]string, len(ve.errs))
	for i, e := range ve.errs {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// Errors returns the collected _errors.
func (ve *ValidationErrors) Errors() []error { return ve.errs }

// Unwrap returns the collected errors for _errors.Is / _errors.As chain walking.
func (ve *ValidationErrors) Unwrap() []error { return ve.errs }

// ToError returns nil when empty, otherwise the *ValidationErrors itself.
func (ve *ValidationErrors) ToError() error {
	if ve.HasErrors() {
		return ve
	}
	return nil
}
