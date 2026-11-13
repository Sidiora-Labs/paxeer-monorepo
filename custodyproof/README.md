# Custodyproof

This directory holds the Go library for verifying native bank balances and historical custody records against Paxeer's committed application state. It checks signed block headers, validator transitions, chain identity and state membership proofs on the network with EVM chain ID `125`. Verification can replay headers from genesis or use a pinned checkpoint with persisted local trust.

## Layout

- `decode.go` — bounded JSON request decoding with duplicate and unknown field checks.
- `verify.go` — proof types, genesis and checkpoint verification, state membership checks and durable trust storage.
- `verify_test.go` — custody fixture checks and native bank proof production, transport and recovery tests.
- `repair_m856_test.go` — bank proof, checkpoint, trust persistence and historical custody compatibility tests.
- `repair_m856.kvx` — proof producer, trust and compatibility metadata.

## Build and test

The package uses the root [go.mod](../go.mod) and has no separate build target or module manifest. From the repository root, the [Makefile](../Makefile) target below compiles and runs the chain's Go tests, including this package:

```sh
make paxeer-test
```

This target delegates to [chain.mk](../chain.mk), which prepares native libraries and splits the Go test packages into groups. The custody tests also read the [state-credit fixture](../tests/fixtures/custody/paxeer-state-v3/state-credit.json).

## Configuration

The library reads no environment variables or configuration files. Callers supply `Expected` chain and proof identities and the verification time. `OpenVerifier` also accepts a dedicated database and an independently provisioned `PinnedCheckpoint`; `Close` closes that database. The checkpoint and expected identities come from the caller's trusted configuration.

Native bank requests use `BankVersion` and the `verify-bank` operation. Historical custody requests use `Version` and the `verify` operation. `TrustingPeriod`, `MaxClockDrift`, `MaxHistory` and the input and proof size bounds are constants in `verify.go`.

## Documentation

- [Chain modules and historical custody state](../docs/system/chain-modules.md).
- [Precompile lifecycle and custody declarations](../docs/system/precompiles.md).
- [Receipts and proofs](../docs/site/paxeer-docs/public/markdown/proofs.md), with the [documentation site page](../docs/site/paxeer-docs/src/app/proofs/page.tsx).
