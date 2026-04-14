package config

import (
	"fmt"
	"strings"

	gvalidator "github.com/go-playground/validator/v10"

	_errors "github.com/os-gomod/cache/errors"
)

// Validator is an alias for Configurable for backward compatibility.
// New code should use Configurable directly.
type Validator interface {
	SetDefaults()
	Validate() error
	ValidateWithContext(opPrefix string) error
}

// Base provides shared validation helpers for config structs. Embed it in
// config types to use its validate* methods.
type Base struct {
	opPrefix string
	validate *gvalidator.Validate
}

// NewBase creates a Base with an optional operation prefix for error messages.
func NewBase(opPrefix ...string) *Base {
	v := gvalidator.New()

	// Register custom validators used in cache
	_ = v.RegisterValidation("power_of_two", func(fl gvalidator.FieldLevel) bool {
		n := fl.Field().Int()
		return n > 0 && (n&(n-1)) == 0
	})

	prefix := ""
	if len(opPrefix) > 0 && opPrefix[0] != "" {
		prefix = opPrefix[0]
	}

	return &Base{
		opPrefix: prefix,
		validate: v,
	}
}

// Apply applies defaults and validates the config. Returns the first validation error encountered.
func Apply(c Validator) error {
	c.SetDefaults()
	return c.Validate()
}

// ApplyWithEnv binds env vars then validates (recommended for production).
func ApplyWithEnv(c any) error {
	if err := ApplyEnv(c); err != nil {
		return err
	}
	if v, ok := c.(Validator); ok {
		return v.Validate()
	}
	return nil
}

// translateValidationErrors converts validator errors into CacheError format.
func translateValidationErrors(err error) error {
	if err == nil {
		return nil
	}

	validationErrs, ok := err.(gvalidator.ValidationErrors)
	if !ok {
		return _errors.New("config.validate", "", err)
	}

	var fieldErrors []string
	for _, e := range validationErrs {
		fieldErrors = append(fieldErrors, fmt.Sprintf(
			"field %s: %s (value=%v)",
			e.Field(), e.Tag(), e.Value(),
		))
	}

	msg := "validation failed: " + strings.Join(fieldErrors, "; ")
	return _errors.New("config.validate", "", fmt.Errorf("%s", msg))
}
