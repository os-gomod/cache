// Package serialization provides a flexible, type-safe codec system for
// serializing and deserializing cache values. It includes a codec registry
// for named lookup, built-in implementations for JSON, MessagePack, and
// zero-allocation primitive types, schema versioning support, and a
// pooled buffer allocator to minimize heap pressure during encoding.
package serialization

import (
	"fmt"
	"sort"
	"sync"

	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// Codec defines the serialization contract for cache values. Implementations
// must be safe for concurrent use. The generic type parameter T describes
// the Go type that the codec handles (e.g. []byte, string, map[string]any).
//
// Encode serializes value into a byte slice. When scratch is non-nil and has
// sufficient capacity, the codec SHOULD append into scratch and return it,
// avoiding an allocation. Callers that require zero-alloc behaviour should
// pre-allocate a scratch buffer from BufPool.
//
// Decode deserialises data back into a value of type T.
//
// Name returns a unique string identifier used for registry lookup and
// metrics labels.
type Codec[T any] interface {
	// Encode serializes value into a byte slice, optionally reusing scratch.
	Encode(value T, scratch []byte) ([]byte, error)

	// Decode deserialises data into a value of type T.
	Decode(data []byte) (T, error)

	// Name returns the codec's unique identifier (e.g. "json", "msgpack").
	Name() string
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry manages named codecs for schema versioning and fallback. It is
// safe for concurrent use and supports a "default" codec that is returned
// when callers do not specify an explicit codec name.
//
// Use Registry.Register to add codecs, Registry.SetDefault to choose a
// fallback, and Registry.Get to look up a specific codec by name.
type Registry struct {
	mu       sync.RWMutex
	codecs   map[string]Codec[any]
	defaults string // name of the default codec
}

// NewRegistry creates an empty codec Registry. No codecs are registered
// by default; callers must register codecs explicitly.
func NewRegistry() *Registry {
	return &Registry{
		codecs: make(map[string]Codec[any]),
	}
}

// Register adds a codec to the registry under the given name. If a codec
// with the same name already exists, Register returns an error.
func (r *Registry) Register(name string, codec Codec[any]) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.codecs[name]; exists {
		return cacheerrors.Factory.New(cacheerrors.CodeInvalid, "Registry.Register", "",
			fmt.Sprintf("codec %q is already registered", name), nil)
	}

	r.codecs[name] = codec

	// If this is the first codec, make it the default.
	if r.defaults == "" {
		r.defaults = name
	}

	return nil
}

// Get returns the codec registered under the given name. If no codec is
// found, the boolean is false.
func (r *Registry) Get(name string) (Codec[any], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.codecs[name]
	return c, ok
}

// MustGet returns the codec registered under the given name. If the codec
// does not exist, MustGet panics with a descriptive message.
func (r *Registry) MustGet(name string) Codec[any] {
	c, ok := r.Get(name)
	if !ok {
		panic(fmt.Sprintf("serialization: codec %q not found in registry", name))
	}
	return c
}

// Default returns the default codec and true, or nil and false if no
// default has been set and no codecs have been registered.
func (r *Registry) Default() (Codec[any], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaults == "" {
		return nil, false
	}
	c, ok := r.codecs[r.defaults]
	return c, ok
}

// SetDefault marks the named codec as the default. If the name does not
// correspond to a registered codec, SetDefault returns an error.
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.codecs[name]; !ok {
		return cacheerrors.Factory.New(cacheerrors.CodeInvalid, "Registry.SetDefault", "",
			fmt.Sprintf("codec %q not found in registry", name), nil)
	}

	r.defaults = name
	return nil
}

// List returns the names of all registered codecs in lexicographic order.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.codecs))
	for n := range r.codecs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Len returns the number of codecs registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.codecs)
}
