# Consensus

Within the `consensus` subdirectory

- `consensus` is the embedded consensus-engine tree, but it is not the root of the repo. When refactoring, check references across the whole repo.
- use consensus/libs/utils.Option for optional values, passing nil as function value is not allowed,
  unless explicitly documented (except for maps and slices, for which nil is a valid empty value).
- use consensus/libs/utils/scope.Run for structured concurrency (instead of waitgroup)
- The root [`go.mod`](../go.mod) selects Go 1.25.8. Its language and testing APIs include:
  - modern range iteration ("for range N", to iterate N times)
  - avoid capturing loop iterators ("tc := tc" is not needed)
  - use slices, maps libraries whenever they do not harm readability
  - use t.Context() in tests instead of context.Background().
- use public api of types in tests, never malform internal state of types. For read-only access,
  you can use private fields directly in case it is not accessible via public api.
- when writing tests, focus on asserting the publically visible properties, especially the API contract. Avoid asserting implementation details.
- [`libs/utils/mutex.go`](libs/utils/mutex.go) provides `Watch` and the `AtomicSend`/`AtomicRecv` pair for waiting for updates.
- Go's `errors.Is` and `errors.As` match error values and types.
- After introducing changes, you may Use `go test --count=0` to quickly check if tests compile. You may run `go test` to actually run the tests
  afterwards. Prefer running modified tests selectively and run the whole test suite only if requested or looking for failures.
- Avoid removing comments and logs which are not obviously obsolete. Keep the original wording, only fixing mistakes or obsolete parts.
- [`utils.TestRng()`](libs/utils/testonly.go) returns a deterministic `Rng` value. `Rng.Split()` returns a separate generator for an independent goroutine.
