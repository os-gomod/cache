package serialization

import (
	"encoding/binary"
	"fmt"
	"sync"

	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

const (
	// VersionHeaderSize is the number of bytes prepended to encoded data
	// to store the schema version (2 bytes: major + minor).
	versionHeaderSize = 2
)

// VersionedCodec wraps a base Codec with schema versioning. When encoding,
// a 2-byte version header is prepended to the payload. When decoding, the
// header is checked and, if the version differs from the codec's current
// version, registered migration functions are applied to transform the data
// before final deserialization.
//
// This enables forward-compatible cache entries: when the data schema
// changes (e.g. a struct field is renamed), old entries can be transparently
// migrated to the new format on read.
//
// VersionedCodec is safe for concurrent use.
type VersionedCodec[T any] struct {
	codec     Codec[T]
	version   uint16
	mu        sync.RWMutex
	migrators map[uint16]func([]byte) ([]byte, error)
}

// NewVersionedCodec creates a versioned wrapper around codec. All encoded
// payloads will be tagged with the given version number.
func NewVersionedCodec[T any](codec Codec[T], version uint16) *VersionedCodec[T] {
	return &VersionedCodec[T]{
		codec:     codec,
		version:   version,
		migrators: make(map[uint16]func([]byte) ([]byte, error)),
	}
}

// WithMigration registers a migration function that can transform data
// encoded with fromVersion into the current version's format. Multiple
// migration paths can be registered for different source versions.
//
// The migrate function receives the raw payload (without the version
// header) and must return the payload in the current version's format.
func (vc *VersionedCodec[T]) WithMigration(
	fromVersion uint16,
	migrate func([]byte) ([]byte, error),
) *VersionedCodec[T] {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.migrators[fromVersion] = migrate
	return vc
}

// Encode serializes value using the base codec and prepends a 2-byte
// version header. The scratch buffer is forwarded to the base codec.
func (vc *VersionedCodec[T]) Encode(value T, scratch []byte) ([]byte, error) {
	payload, err := vc.codec.Encode(value, scratch)
	if err != nil {
		return nil, err
	}

	// Prepend 2-byte version header.
	header := make([]byte, versionHeaderSize)
	binary.BigEndian.PutUint16(header, vc.version)

	// Concatenate header + payload.
	result := make([]byte, 0, versionHeaderSize+len(payload))
	result = append(result, header...)
	result = append(result, payload...)
	return result, nil
}

// Decode reads the 2-byte version header from data. If the version
// matches the codec's current version, the payload is decoded directly.
// If the version differs, the registered migration function is applied
// before decoding.
func (vc *VersionedCodec[T]) Decode(data []byte) (T, error) {
	var zero T

	if len(data) < versionHeaderSize {
		return zero, cacheerrors.Factory.New(
			cacheerrors.CodeDeserialize,
			"VersionedCodec.Decode",
			"",
			fmt.Sprintf(
				"data too short for version header: got %d bytes, need at least %d",
				len(data),
				versionHeaderSize,
			),
			nil,
		)
	}

	readVersion := binary.BigEndian.Uint16(data[:versionHeaderSize])
	payload := data[versionHeaderSize:]

	// If version matches, decode directly.
	if readVersion == vc.version {
		return vc.codec.Decode(payload)
	}

	// Look up a migration function for the source version.
	vc.mu.RLock()
	migrate, ok := vc.migrators[readVersion]
	vc.mu.RUnlock()

	if !ok {
		return zero, cacheerrors.Factory.New(
			cacheerrors.CodeDeserialize,
			"VersionedCodec.Decode",
			"",
			fmt.Sprintf(
				"no migration registered from version %d to %d",
				readVersion,
				vc.version,
			),
			nil,
		)
	}

	// Apply migration.
	migrated, err := migrate(payload)
	if err != nil {
		return zero, cacheerrors.Factory.New(
			cacheerrors.CodeDeserialize,
			"VersionedCodec.Decode",
			"",
			fmt.Sprintf("migration from version %d failed: %s", readVersion, err.Error()),
			err,
		)
	}

	return vc.codec.Decode(migrated)
}

// Name returns the base codec's name with a version suffix (e.g. "json:v3").
func (vc *VersionedCodec[T]) Name() string {
	return fmt.Sprintf("%s:v%d", vc.codec.Name(), vc.version)
}

// Version returns the current schema version.
func (vc *VersionedCodec[T]) Version() uint16 {
	return vc.version
}
