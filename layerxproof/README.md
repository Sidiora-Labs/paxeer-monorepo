# layerxproof

`layerxproof` holds the Go codecs, verification helpers and shared test vectors
for Paxeer execution receipts and native state witnesses. These libraries let
node and client code decode canonical evidence, authenticate receipts under a
caller-supplied key and verify state against a trusted composite root. The
kernel runs inside `paxd` as part of the BFT network; a receipt becomes final
when its block commits. Receipt authentication and state-witness verification
each establish their own facts, while block commitment requires chain evidence.

## Layout

- [codec/](codec/) — canonical receipt, batch-header, discovery-attestation,
  Merkle-path, state-witness and account-value decoding, hashing and validation;
  includes native C fixtures for Go tests.
- [exportcheck/](exportcheck/) — reads compiled Go package exports and tests the
  available proof API surface.
- [retired/](retired/) — Python checks for proof-asset references and root build
  rules.
- [testdata/](testdata/) — shared receipt, signature, state and account-binding
  JSON vectors, with Python and Rust state-vector recorders.
- [testvectors/](testvectors/) — Go loaders for the shared JSON vectors and
  helpers for decoding their byte and integer fields.
- [verify/](verify/) — strict signature, receipt inclusion, state, account and
  withdrawal-outcome verification, with fixture-based tests.

## Build and test

The packages belong to the root [go.mod](../go.mod); this directory has no
separate module or Makefile. Run these targets from the repository root:

```sh
make paxd-native-libs
make paxeer-build
make paxeer-test
```

The root [Makefile](../Makefile) defines these targets. `paxd-native-libs`
builds the native archives used by the cgo host and codec fixtures.
`paxeer-build` delegates to [chain.mk](../chain.mk) to build `paxd`, including
its proof-library dependencies. `paxeer-test` delegates to the root Go test
suite, whose package discovery includes the Go tests in this directory.
The native fixtures require cgo, a C compiler and the OpenSSL development
libraries alongside the programs sandbox archive.

## Configuration

The codec and verification libraries read no runtime environment variables or
configuration files. Callers supply receipt keys, expected state roots and
account or asset identities directly to the verification functions.

The vector loaders read [testdata/vectors.json](testdata/vectors.json) and
[testdata/paxeer_bind_vectors.json](testdata/paxeer_bind_vectors.json).
Codec tests also read the native proof vectors in
[contracts/config/native-state-proofs.json](../contracts/config/native-state-proofs.json)
and [tests/vectors/program_account_state_v2.vec](../tests/vectors/program_account_state_v2.vec).
These files supply test evidence rather than runtime trust configuration.

## Documentation

- [Settlement evidence](../docs/wiki/SettlementEvidence.md) — receipt authority,
  block commitment and the native state-witness format.
- [SDK terminal verification](../docs/wiki/SdkTerminalVerification.md) — receipt
  authentication and its bindings to transaction and chain evidence.
- [Receipts & Proofs on the documentation site](../docs/site/paxeer-docs/src/app/proofs/page.tsx)
  — receipt, native state, custody and archive evidence.
- [Receipts & Proofs Markdown](../docs/site/paxeer-docs/public/markdown/proofs.md)
  — the site's Markdown page.
