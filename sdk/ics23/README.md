# Historical ICS23 compatibility module

This independent `github.com/confio/ics23/go` module is retired. It no longer
serves as a standalone protobuf producer. Its checked-in `proofs.pb.go` is a historical wire-format
snapshot, retained with compatibility and malformed-input tests. The local
`protoc` and `install-proto-dep` Makefile targets explicitly refuse regeneration;
they no longer point at a missing schema or install an unpinned generator.

The repository root `go.mod` requires `github.com/confio/ics23/go v0.9.0` and
replaces it with the maintained published replacement
`github.com/cosmos/cosmos-sdk/ics23/go v0.8.0`. Root-module proof consumers use
that published module, not this directory. Schema and generator maintenance
belong to that published replacement.

The retained implementation is tested locally with `make test`. Its `Compress`
and `Decompress` APIs now return `(*CommitmentProof, error)` so callers must
handle malformed variants and compressed indexes explicitly. `CombineProofs`
and `Calculate` propagate those errors; boolean verifiers return false.

`repair_m1359_test.go` contains recorded producer Merkle-store vectors with
committed keys, values and roots, serialized by the real protobuf codec and consumed
by this implementation. It also covers decoded malformed indexes and entries,
partial verifier inputs, size errors, and non-membership neighbor paths.
