# LayerX Programs precompile

This directory holds the Go implementation, Solidity interface and ABI for the LayerX Programs precompile at `0x101D`. It connects EVM contracts to the native Programs module through `modules/lxphost`, supporting program deployment and upgrades, calls and simulation, account transfers, sandbox leases and program state queries. Programs execute inside `paxd` on the Paxeer BFT network; their state changes, transfers and receipts become final when the containing block commits.

## Layout

- [testdata/](testdata/) — encoded sandbox lease, execution result and usage receipt fixtures.
- [types/](types/) — generated Go protobuf bindings for genesis, queries and transactions.
- [LayerXPrograms.sol](LayerXPrograms.sol) — Solidity interface, precompile address and event declarations.
- [abi.json](abi.json) — ABI embedded by the Go implementation.
- [layerxprograms.go](layerxprograms.go) — precompile construction, keeper wiring and constructor registration.
- [submit.go](submit.go) — method dispatch, call restrictions and native activity submission.
- [principal.go](principal.go) — resolves the EVM caller's bound native identity and authority.
- [idempotency.go](idempotency.go) — derives activity keys from the transaction hash, call ordinal and selector.
- [args.go](args.go) — validates and decodes ABI arguments.
- [gas.go](gas.go) — embedded ABI and method gas table.
- [gate.go](gate.go) — activation and call validation helpers.
- [errors.go](errors.go) — native refusal codes and error conversion.
- [descriptors.go](descriptors.go) — imports protobuf descriptor registration.
- [method_deploy.go](method_deploy.go) — deployment handler.
- [method_upgrade.go](method_upgrade.go) — upgrade handler.
- [method_call.go](method_call.go) — program call handler.
- [method_call_receipts.go](method_call_receipts.go) — receipt authority decoding and call handler.
- [method_simulate.go](method_simulate.go) — program simulation without committed state changes.
- [method_registry.go](method_registry.go) — receipt-bound registry read handler.
- [method_transfer.go](method_transfer.go) — atomic transfer set handler.
- [method_account.go](method_account.go) — program account registration.
- [method_quote.go](method_quote.go) — call fee quotation.
- [method_winddown.go](method_winddown.go) — deprecation and exit route handlers.
- [method_tombstone.go](method_tombstone.go) — program tombstone handler.
- [method_exit.go](method_exit.go) — value exit handlers.
- [method_retire.go](method_retire.go) — storage retirement handler.
- [method_sandbox_fund.go](method_sandbox_fund.go) — sandbox funding handler.
- [method_sandbox_activate.go](method_sandbox_activate.go) — sandbox activation handler.
- [method_sandbox_execute.go](method_sandbox_execute.go) — sandbox execution handler.
- [payload_lifecycle.go](payload_lifecycle.go) — deployment and upgrade payload encoders.
- [payload_call.go](payload_call.go) — call payload encoder.
- [payload_accounts.go](payload_accounts.go) — transfer and account payload encoders.
- [payload_winddown.go](payload_winddown.go) — wind-down and exit payload encoders.
- [payload_sandbox.go](payload_sandbox.go) — sandbox payload encoders.
- [sandbox_lease.go](sandbox_lease.go) — lease request encoding.
- [view_registry.go](view_registry.go) — program, interface, owner and state head queries.
- [view_accounts.go](view_accounts.go) — program account and value account queries.
- [view_fees.go](view_fees.go) — active, versioned and pending fee schedule queries.
- [view_metering.go](view_metering.go) — metering schedule queries.
- [view_winddown.go](view_winddown.go) — wind-down and exit route queries.
- [view_sandbox.go](view_sandbox.go) — sandbox and retirement control queries.
- [view_web.go](view_web.go) — stored web request queries.
- [xweb.go](xweb.go) — native bank transfer and xweb request integration.
- [events.go](events.go) — lifecycle, transfer and account event encoding.
- [events_call.go](events_call.go) — guest event and call outcome encoding.
- [events_sandbox.go](events_sandbox.go) — sandbox usage receipt event encoding.
- [response.go](response.go) — canonical execution response decoder retaining terminal and transfer evidence.
- [response.md](response.md) — call and simulation response format and receipt verification requirements.
- Go test files alongside the implementation cover ABI agreement, dispatch, native execution, event encoding, gas charging, replay, conservation, simulation and sandbox settlement.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no separate Makefile. From the repository root, use the node targets in [chain.mk](../../chain.mk):

```sh
make -f chain.mk build
make -f chain.mk test
```

The build compiles `paxd`, including the precompile. The test target runs the root Go packages in groups, including this directory. Both targets prepare native libraries through `paxd-native-libs` in the root [Makefile](../../Makefile). Building requires Go, a C/C++ toolchain and the Rust Programs runtime toolchain. The tests also exercise guest builds and SDK consumers using Rust, Python and Node.js.

## Configuration

The precompile receives its native host, EVM keeper, upgrade keeper, merge plan name and xweb keeper from node construction. Fee denomination bindings and allowed denominations come from xweb genesis and governed state. The package reads no runtime environment variables or configuration files; `abi.json` is embedded at build time.

The cross-language proof test reads `LAYERX_PROOF_PYTHON` to select its Python executable. The web reader test sets `CARGO_TARGET_DIR` for its guest build.

## Documentation

- [Programs](../../docs/wiki/Programs.md) — native activity kinds, runtime behavior and precompile methods.
- [Sandbox](../../docs/wiki/Sandbox.md) — lease lifecycle and resource accounting.
- [Programs workspace](../../docs/wiki/ProgramsWorkspace.md) — guest development tools and workspace layout.
- [Precompile reference](../../docs/system/precompiles.md) — ABI inventory and call behavior.
- [Documentation site: Programs](../../docs/site/paxeer-docs/src/app/programs/page.tsx) — application capabilities and execution model.
- [Documentation site: Precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx) — EVM access to native constructs.
