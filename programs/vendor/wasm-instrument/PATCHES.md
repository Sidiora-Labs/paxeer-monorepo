# Local vendoring changes

`Cargo.toml.orig` retains the upstream source manifest. Cargo's published
`Cargo.toml` also contains normalization changes, including expanded dependency
tables and package metadata.

The vendored manifest keeps `parity-wasm` at `=0.45.0` through the sibling
`../parity-wasm` path. The programs workspace uses this local dependency, as
recorded without a registry source in `programs/Cargo.lock`; switching back to
the upstream registry dependency would change the locked dependency graph.

The vendored manifest pins the development dependencies and omits upstream
`rand` and `binaryen`. Commit `b022d25a24` removed `binaryen` while qualifying
the vendored crate build. Retain that removal: the current programs lockfile
records `wasm-instrument` without `binaryen`, and restoring it would add the
native Binaryen toolchain to the crate's development dependency graph. These
development dependencies are not required by the programs runtime.

Commit `3a508e8102` also changed `src/gas_metering/mod.rs` and
`src/gas_metering/validation.rs` without refreshing their checksums. Retain
these changes: memory growth metering works without the `bulk` feature, and
the validation graph accounts for conditional branches, loop costs and prepaid
continuations using explicit expected instruction costs.

`.cargo-checksum.json` records the SHA-256 of the retained `Cargo.toml`, both
gas metering files and this patch record. Its upstream package checksum and
all other file digests remain unchanged.
