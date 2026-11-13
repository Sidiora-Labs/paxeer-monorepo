# LayerX agent precompiles

This directory holds the Go implementations, embedded ABIs and tests for the eight agent precompiles at `0x0901`–`0x0908`. They expose price-offset calculations, clearing, oracle observations, fill scoring, scheduled calls, payment streams, attestation envelopes and typed-data signatures through the Paxeer EVM. The node supplies their keepers and registers their constructors; calls use the merge upgrade activation height in the same network state as other native operations.

## Layout

- [clearing/](clearing/) — clearing offset, price and matched-volume calculation for buy and sell orders.
- [common/](common/) — activation checks, ABI validation, write guards, gas schedules and revert-code handling.
- [eip712/](eip712/) — typed-data hashing, domain separators and signer recovery at `0x0908`.
- [oracle/](oracle/) — confidence-weighted aggregation, signed observation intake and committed price reads at `0x0903`.
- [orob/](orob/) — conversion between oracle-relative offsets and absolute prices at `0x0901`.
- [pofq/](pofq/) — individual, batch and rolling fill-quality scores at `0x0904`.
- [scheduler/](scheduler/) — job scheduling, cancellation, rescheduling, queries and EVM events at `0x0905`.
- [served/](served/) — node integration tests for activation, registration and attestation revert codes.
- [streams/](streams/) — payment-stream creation, settlement, closure, rate updates and queries at `0x0906`.
- [teeattestor/](teeattestor/) — verification of signed `PXVE` development envelopes, attestor queries and peer bindings at `0x0907`.

Each implementation embeds its ABI in its Go source. The `merge.go` files register constructors and bind keepers and limits; `teeattestor/teeattestor.go` contains its registration. Tests live alongside the implementations.

## Build and test

These packages belong to the root [Go module](../../go.mod) and use the root [Makefile](../../Makefile) and [chain.mk](../../chain.mk). Run these commands from the repository root:

```sh
make paxd-build
make paxeer-test
```

`paxd-build` builds the native libraries and the node, including its agent precompile imports. `paxeer-test` runs the chain Go test groups, which discover packages with tests across the root module, including this directory. Both targets prepare native libraries through `paxd-native-libs`. The Go dependencies must be available locally because these targets use `GOPROXY=off`.

The EIP-712, scheduler, streams and attestation integration tests also invoke Node.js; the TypeScript consumers use `tsx`.

## Configuration

The precompile sources read no environment variables or configuration files directly. Constructors receive the upgrade keeper and plan name, application keepers and calculation limits. The node binds these through [precompile wiring](../../node/precompiles.go), the [merge address table](../setup.go) and [upgrade activation](../../node/upgrades.go).

The scheduler uses the agent keeper, streams use the pay keeper, the oracle uses committed host state, and the attestor reads the XWeb attestor set. Calculation and input limits are declared in the implementation constructors and `merge.go` files.

## Documentation

- [Precompile ABI catalogue](../../docs/system/precompiles.md) — agent methods, events and activation rules.
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx) — native EVM interfaces and agent-function addresses.
- [Documentation site: precompiles Markdown](../../docs/site/paxeer-docs/public/markdown/precompiles.md) — the site's text reference.
