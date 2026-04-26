# Contributing Guide

Thank you for your interest in contributing to `os-gomod/cache v2`! This document outlines the development workflow, coding standards, and review process.

---

## Development Setup

### Prerequisites

- **Go 1.22+** (required for generics)
- **Git** with commit signing recommended
- **golangci-lint** v1.55+ for linting

### Clone and Build

```bash
git clone https://github.com/os-gomod/cache/v2.git
cd cache/v2
go mod download
go build ./...
```

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Specific package
go test ./cache/...

# With verbose output
go test -v ./memory/...
```

### Running Benchmarks

```bash
# All benchmarks
go test -bench=. -benchmem ./benchmarks/

# Specific benchmark
go test -bench=BenchmarkMemoryGet -benchmem ./benchmarks/

# Compare before/after
go test -bench=. -benchmem ./benchmarks/ > bench_before.txt
# ... make changes ...
go test -bench=. -benchmem ./benchmarks/ > bench_after.txt
benchstat bench_before.txt bench_after.txt
```

### Running Linter

```bash
go vet ./...
golangci-lint run ./...
```

---

## Code Standards

### Formatting

- Run `gofmt -w .` before committing.
- Use `goimports` for automatic import management.
- Maximum line length: 120 characters.
- Use tabs for indentation, spaces for alignment.

### Naming Conventions

- **Packages**: lowercase, single word, no underscores (`cache`, `memory`, `middleware`).
- **Exported types**: PascalCase (`TypedCache`, `HotKeyDetector`).
- **Exported functions**: PascalCase (`NewMemory`, `WithRetry`).
- **Unexported fields**: camelCase (`bufPool`, `mu`).
- **Interfaces**: noun or adjective describing behavior (`Compressor`, `StatsProvider`).
- **Error variables**: `Err` prefix for sentinel errors (`ErrNotFound`).

### Documentation

Every exported symbol **must** have a godoc comment:

```go
// NewMemory creates a new in-process memory cache with the given options.
// The memory backend uses sharding for high concurrent throughput and
// supports pluggable eviction policies (LRU, LFU, FIFO, TinyLFU).
//
// Example:
//
//	c, err := cache.NewMemory(WithMaxEntries(10000))
//	if err != nil { /* handle error */ }
//	defer c.Close(context.Background())
func NewMemory(opts ...Option) (Backend, error) {
```

Comments should:
- Start with the symbol name.
- Use full sentences.
- Include an `Example:` section for non-trivial functions.
- Document parameters, return values, and error conditions.

### Error Handling

- **Never** use `fmt.Errorf` in public packages. Always use `errors.Factory`.
- Errors should carry context: what operation failed, which key, which backend.
- Use `errors.Is()` and `errors.As()` for error checking.
- Wrap underlying errors with `%w` verb.

```go
// Correct
return nil, errors.Factory.DecodeFailed(key, codec.Name(), err)

// Wrong
return nil, fmt.Errorf("decode failed for key %s: %w", key, err)
```

### Concurrency

- Document thread-safety guarantees in godoc.
- Prefer `sync.RWMutex` over `sync.Mutex` for read-heavy paths.
- Use `atomic` operations for simple counters.
- Use `sync.Map` for append-only maps with infrequent deletion.
- Always protect shared state, even if it seems "obviously safe."

### Generics

- Use generics for type-safe wrappers (`TypedCache[T]`, `Codec[T]`).
- Keep generic parameters at the type level, not the method level.
- Provide concrete type aliases for common instantiations (`NewMemoryString`, `NewMemoryInt64`).

---

## Testing Requirements

### Coverage Target

All packages must maintain **90%+ test coverage**. Run `go test -cover ./...` to verify.

### What to Test

- **Happy path**: Correct behavior with valid inputs.
- **Error path**: Correct error handling for invalid inputs, missing keys, backend failures.
- **Edge cases**: Empty inputs, zero values, maximum sizes, concurrent access.
- **Interface compliance**: Verify type assertions compile (`var _ Interface = (*Impl)(nil)`).
- **Middleware**: Test each middleware independently with mock backends.

### Test Structure

```go
func TestFeature_Scenario(t *testing.T) {
    // Setup
    backend, err := NewMemory(WithMaxEntries(100))
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    defer backend.Close(context.Background())

    // Execute
    err = backend.Set(ctx, "key", []byte("value"), time.Minute)
    if err != nil {
        t.Fatalf("set failed: %v", err)
    }

    // Verify
    val, err := backend.Get(ctx, "key")
    if err != nil {
        t.Fatalf("get failed: %v", err)
    }
    if string(val) != "value" {
        t.Errorf("got %q, want %q", string(val), "value")
    }
}
```

### Table-Driven Tests

Use table-driven tests for multiple scenarios:

```go
func TestEvictionPolicy(t *testing.T) {
    tests := []struct {
        name     string
        policy   string
        entries  int
        max      int
        wantSize int
    }{
        {"LRU eviction", "lru", 200, 100, 100},
        {"LFU eviction", "lfu", 200, 100, 100},
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... })
    }
}
```

### Benchmarks

Add benchmarks for any new backend, codec, or middleware. Follow the existing naming convention: `Benchmark<Feature>_<Scenario>`.

---

## Pull Request Process

### Before Submitting

1. Run `go vet ./...` — must pass.
2. Run `golangci-lint run ./...` — must pass.
3. Run `go test ./...` — all tests must pass.
4. Run `go test -cover ./...` — coverage must be ≥ 90%.
5. Run `go test -race ./...` — no race conditions.
6. Update godoc comments for any changed/added exported symbols.
7. Add tests for new functionality.

### PR Description Template

```markdown
## Summary
Brief description of the change.

## Motivation
Why is this change needed?

## Changes
- List of files/packages changed
- New APIs introduced

## Testing
- Unit tests added: Y/N
- Benchmarks added: Y/N
- Coverage before: X% / after: Y%

## Breaking Changes
List any breaking changes (if applicable).
```

### Code Review Checklist

Reviewers should verify:

- [ ] Code follows project conventions (naming, formatting, docs)
- [ ] Error handling uses `errors.Factory` (no raw `fmt.Errorf`)
- [ ] All exported symbols have godoc comments
- [ ] Tests cover happy path, error path, and edge cases
- [ ] No race conditions (`go test -race`)
- [ ] Coverage ≥ 90%
- [ ] No unnecessary dependencies added
- [ ] Backward compatibility maintained (or breaking changes documented)

### Merge Policy

- **Squash merge** for small changes (< 5 commits).
- **Rebase merge** for larger changes to maintain clean history.
- All PRs require at least **1 approval** from a maintainer.
- CI must be green before merging.

---

## Release Process

Releases follow semantic versioning (semver):

- **Patch** (x.x.Z): Bug fixes, no API changes.
- **Minor** (x.Y.0): New features, backward-compatible API additions.
- **Major** (X.0.0): Breaking API changes.

Release checklist:
1. Update `CHANGELOG.md`.
2. Bump version in `go.mod`.
3. Tag with `vX.Y.Z`.
4. Push tag to trigger CI/CD.
5. Verify `pkg.go.dev` documentation updates.
