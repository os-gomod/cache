package config

import (
	"fmt"
	"strings"

	gvalidator "github.com/go-playground/validator/v10"

	cacheerrors "github.com/os-gomod/cache/errors"
)

// Validator is the interface for configuration types that support defaults and validation.
type Validator interface {
	SetDefaults()
	Validate() error
	ValidateWithContext(opPrefix string) error
}

// Base provides shared validation infrastructure for configuration types.
type Base struct {
	opPrefix string
	validate *gvalidator.Validate
}

// NewBase creates a new Base with an optional operation prefix for error messages.
func NewBase(opPrefix ...string) *Base {
	v := gvalidator.New()
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

// Apply applies defaults and then validates the configuration.
func Apply(c Validator) error {
	c.SetDefaults()
	return c.Validate()
}

// ApplyWithEnv binds environment variables to the config and then validates it.
func ApplyWithEnv(c any) error {
	if err := ApplyEnv(c); err != nil {
		return err
	}
	if v, ok := c.(Validator); ok {
		return v.Validate()
	}
	return nil
}

func translateValidationErrors(err error) error {
	if err == nil {
		return nil
	}
	validationErrs, ok := err.(gvalidator.ValidationErrors)
	if !ok {
		return cacheerrors.New("config.validate", "", err)
	}
	var fieldErrors []string
	for _, e := range validationErrs {
		fieldErrors = append(fieldErrors, fmt.Sprintf(
			"field %s: %s (value=%v)",
			e.Field(), e.Tag(), e.Value(),
		))
	}
	msg := "validation failed: " + strings.Join(fieldErrors, "; ")
	return cacheerrors.New("config.validate", "", fmt.Errorf("%s", msg))
}
