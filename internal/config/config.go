// Package config provides type-safe configuration for all cache backends
// with automatic defaults, validation via go-playground/validator tags,
// and immutable cloning for safe configuration sharing.
package config

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator defines the contract for configuration types that support
// validation and default-value initialisation. All config structs
// (Memory, Redis, Layered) implement this interface.
type Validator interface {
	// Validate checks all constraints and returns an error describing
	// any violations.
	Validate() error

	// SetDefaults populates zero-value fields with sensible defaults
	// so that callers can create a working config with zero config.
	SetDefaults()
}

// Base provides shared validation infrastructure for configuration types.
// It wraps a go-playground/validator instance and exposes convenience
// methods for struct validation.
type Base struct {
	validate *validator.Validate
}

// NewBase creates a Base with a freshly initialized validator. The
// validator is configured with English error messages.
func NewBase() *Base {
	return &Base{
		validate: validator.New(),
	}
}

// ValidateStruct runs go-playground/validator against the given struct
// and translates any validation errors into human-readable messages.
func (b *Base) ValidateStruct(s any) error {
	if b.validate == nil {
		b.validate = validator.New()
	}
	err := b.validate.Struct(s)
	if err == nil {
		return nil
	}
	return translateErrors(err)
}

// translateErrors converts validator.ValidationErrors into a single
// error with a friendly, human-readable message.
func translateErrors(err error) error {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err
	}

	if len(ve) == 0 {
		return nil
	}

	return cacheErrors("config.Validate", formatValidationErrors(ve), nil)
}

// formatValidationErrors formats validation errors into a human-readable string.
func formatValidationErrors(ve validator.ValidationErrors) string {
	var b strings.Builder
	for i, fe := range ve {
		if i > 0 {
			b.WriteString("; ")
		}
		field := fe.Field()
		switch fe.Tag() {
		case "required":
			b.WriteString(field)
			b.WriteString(" is required")
		case "gte":
			b.WriteString(field)
			b.WriteString(" must be >= ")
			b.WriteString(paramToString(fe.Param()))
		case "lte":
			b.WriteString(field)
			b.WriteString(" must be <= ")
			b.WriteString(paramToString(fe.Param()))
		case "gt":
			b.WriteString(field)
			b.WriteString(" must be > ")
			b.WriteString(paramToString(fe.Param()))
		case "lt":
			b.WriteString(field)
			b.WriteString(" must be < ")
			b.WriteString(paramToString(fe.Param()))
		case "oneof":
			b.WriteString(field)
			b.WriteString(" must be one of: ")
			b.WriteString(fe.Param())
		case "omitempty":
			// omitempty alone is not a failure; skip.
			continue
		default:
			b.WriteString(field)
			b.WriteString(" failed ")
			b.WriteString(fe.Tag())
			b.WriteString(" validation")
		}
	}

	return b.String()
}

// paramToString returns a string representation of a validator parameter,
// handling both numeric and string parameters.
func paramToString(param string) string {
	if param == "" {
		return "0"
	}
	return param
}

// cacheErrors creates a config validation error. We use a simple
// approach here to avoid circular dependencies with the errors package.
type configError struct {
	op, message string
	cause       error
}

func (e *configError) Error() string {
	if e.cause != nil {
		return "config." + e.op + ": " + e.message + ": " + e.cause.Error()
	}
	return "config." + e.op + ": " + e.message
}

func (e *configError) Unwrap() error { return e.cause }

func cacheErrors(op, message string, cause error) error {
	return &configError{op: op, message: message, cause: cause}
}

// setDefaultString sets *field to value if the current value is empty.
func setDefaultString(field *string, value string) {
	if *field == "" {
		*field = value
	}
}

// setDefaultInt sets *field to value if the current value is zero.
func setDefaultInt(field *int, value int) {
	if *field == 0 {
		*field = value
	}
}

// setDefaultInt64 sets *field to value if the current value is zero.
func setDefaultInt64(field *int64, value int64) {
	if *field == 0 {
		*field = value
	}
}
