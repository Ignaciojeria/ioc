# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- **Type-Based Dependency Inference**: `ioc.Register` now takes a single constructor. The framework automatically builds the dependency graph by matching the parameter types of a constructor to the return types of other registered constructors.
- **Fail-Fast Ambiguity Detection**: If a constructor requires an interface, and multiple registered providers implement that interface, `LoadDependencies()` will immediately return a clear error detailing the multiple matching providers instead of silently selecting one.
- **IDE-Clickable Error Traces**: All framework errors (e.g., missing dependencies, ambiguous providers, initialization failures) now include the exact `file:line` where the failing constructor was registered. This allows developers to `Ctrl+Click` (or `Cmd+Click`) in their IDE terminal to jump directly to the code that caused the container issue.
- **Support for Side-Effect (Void) Constructors**: Constructors that return no values can now be registered. The framework will invoke them in the correct topological order based on their parameters without trying to expose them as a dependency to others.

### Changed
- The public API has been radically simplified to just three functions:
  - `ioc.Register(ctor)`
  - `ioc.RegisterAtEnd(ctor)`
  - `ioc.LoadDependencies()`
- Previous functions (`ioc.New()`, `ioc.Reset()`) and the `ioc.Container` type have been made unexported. The framework uses a clean, global state internally by default, enforcing best practices.
- The `dag` structure is now exclusively used for topological ordering and cycle detection; the registry is now primarily indexed by `reflect.Type`.

### Removed
- Removed the variadic `deps` parameter from `Register`. Dependencies are no longer hardcoded strings or function pointers.
