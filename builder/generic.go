// Package builder provides fluent, immutable builders for all cache backends.
package builder

import (
	"context"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/internal/cachectx"
)

// Cloner produces a deep copy of a config struct.
// Registered once at GenericBuilder construction time so WithConfig never
// needs the caller to supply it at the call site.
type Cloner[C any] func(*C) *C

// Factory constructs a cache from a fully-validated config.
type Factory[C any, T any] func(ctx context.Context, cfg *C) (T, error)

// Preparer normalizes a config before fluent options are applied.
// Builders use this to fill defaults on explicit WithConfig values without
// letting defaulting overwrite later user-supplied options.
type Preparer[C any] func(*C)

// GenericBuilder is the shared immutable builder core.
// Every method that modifies state returns a new GenericBuilder; the receiver
// is never mutated, so builders are safe to fork and reuse.
//
// Type parameters:
//   - C  the config struct type (e.g. config.Memory, config.Redis)
//   - T  the cache type produced by Build (e.g. *memory.Cache)
type GenericBuilder[C any, T any] struct {
	ctx      context.Context
	cfg      *C         // explicit config set via WithConfig; nil = use defaults
	opts     []func(*C) // functional options applied after cfg/defaults
	defaults func() *C
	prepare  Preparer[C]
	validate func(*C) error
	factory  Factory[C, T]
	cloner   Cloner[C]
}

// NewGenericBuilder creates a reusable builder.
//
//   - defaults  returns a fresh, fully-defaulted config (called in Build when
//     no explicit config has been set via WithConfig).
//   - prepare   normalizes a cloned config before fluent options are applied.
//   - validate  validates a finalized config before handing it to factory.
//   - factory   constructs the cache from a validated config.
//   - cloner    produces a deep copy of a config; used by WithConfig to
//     protect the caller's struct and by Build to isolate the internal copy
//     before applying options.
func NewGenericBuilder[C, T any](
	ctx context.Context,
	defaults func() *C,
	prepare Preparer[C],
	validate func(*C) error,
	factory Factory[C, T],
	cloner Cloner[C],
) *GenericBuilder[C, T] {
	return &GenericBuilder[C, T]{
		ctx:      cachectx.NormalizeContext(ctx),
		defaults: defaults,
		prepare:  prepare,
		validate: validate,
		factory:  factory,
		cloner:   cloner,
	}
}

// cloneBuilder returns a shallow copy of the builder with the opts slice
// defensively copied so appending to the clone never modifies the original's
// backing array.
func (b *GenericBuilder[C, T]) cloneBuilder() *GenericBuilder[C, T] {
	clone := *b
	clone.opts = append([]func(*C){}, b.opts...)
	return &clone
}

// WithOption returns a new builder with opt appended to the option chain.
// opt receives the config after defaults (or an explicit WithConfig value)
// have been applied, and may freely mutate it.
func (b *GenericBuilder[C, T]) WithOption(opt func(*C)) *GenericBuilder[C, T] {
	nb := b.cloneBuilder()
	nb.opts = append(nb.opts, opt)
	return nb
}

// WithConfig returns a new builder that uses cfg as the starting config
// instead of the defaults.  cfg is cloned immediately so the caller may
// safely discard their copy after this call.
//
// Passing nil resets to the defaults-based path.
func (b *GenericBuilder[C, T]) WithConfig(cfg *C) *GenericBuilder[C, T] {
	nb := b.cloneBuilder()
	if cfg == nil {
		nb.cfg = nil
		return nb
	}
	nb.cfg = b.cloner(cfg)
	return nb
}

// Build finalizes the config, validates it, and constructs the cache.
//
// Resolution order:
//  1. Start from an explicit config set via WithConfig, or call defaults().
//  2. Clone the result so later steps never mutate the stored cfg.
//  3. Prepare the config to fill derived/default values.
//  4. Apply all WithOption functions in registration order.
//  5. Validate the final config.
//  6. Call factory.
func (b *GenericBuilder[C, T]) Build() (T, error) {
	var zero T

	var base *C
	if b.cfg != nil {
		base = b.cfg
	} else {
		base = b.defaults()
	}

	cfg := b.cloner(base)
	if b.prepare != nil {
		b.prepare(cfg)
	}
	for _, opt := range b.opts {
		opt(cfg)
	}

	if err := b.validate(cfg); err != nil {
		return zero, err
	}
	return b.factory(b.ctx, cfg)
}

// MustBuild calls Build and panics on error.
// Intended for program initialisation where a misconfiguration is a
// programming error, not a recoverable condition.
func (b *GenericBuilder[C, T]) MustBuild() T {
	v, err := b.Build()
	if err != nil {
		panic("cache: MustBuild: " + err.Error())
	}
	return v
}

type configPointer[C any] interface {
	*C
	config.Validator
}

func newValidatedBuilder[C any, PC configPointer[C], T any](
	ctx context.Context,
	defaults func() *C,
	factory Factory[C, T],
	cloner Cloner[C],
) *GenericBuilder[C, T] {
	return NewGenericBuilder(
		ctx,
		defaults,
		func(cfg *C) { PC(cfg).SetDefaults() },
		func(cfg *C) error {
			return PC(cfg).Validate()
		},
		factory,
		cloner,
	)
}
