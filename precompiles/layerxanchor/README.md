# LayerX anchor precompile

This directory holds the Go executor, embedded ABI, Solidity interface and tests for the `layerxAnchor` EVM precompile. It connects checkpoint, availability, guarantor bond and challenge calls to the native anchor keeper. On the merged Paxeer network, receipts are final when their block is committed. The `v7.0` activation disables anchor writes; reads remain available while the anchor store is retained. From `v7.0-anchor-store-delete`, every selector reverts with `LXP_MODULE_DISABLED`. An executor without an anchor keeper also returns this refusal.

## Layout

- `layerxanchor.go` — executor construction, method dispatch, retirement checks, gas calculation, payments and event encoding.
- `LayerXAnchor.sol` — Solidity interface, checkpoint and guarantor tuples, events and precompile constants.
- `abi.json` — ABI embedded by the Go executor.
- `setup.go` — generated versioned precompile constructor.
- `versions` — precompile version metadata.
- `layerxanchor_test.go` — shared test harness and checks for authority, call restrictions, gas, wiring and revert reasons.
- `retire_gate_test.go` — write retirement, behavior before merge activation and payable-call balance checks.
- `retire_read_gate_test.go` — reads from the retained store and refusal after store deletion activation.
- `retired_surface_test.go` — registered EVM calls across every ABI selector, receipt status and event checks.
- `retired_wrapper_test.go` — retirement responses through the common wrapper, including malformed write calldata.
- `unwired_test.go` — disabled-module responses when no anchor keeper is supplied.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no separate build manifest or Makefile. Run these targets from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [Makefile](../../Makefile) defines both targets. `paxd-build` prepares native libraries and builds the node through [chain.mk](../../chain.mk). `paxeer-test` invokes the chain test target, whose package discovery includes this directory's Go tests.

## Configuration

The executor reads no environment variables or runtime configuration files. Its constructor receives the anchor, bank and EVM keepers from the node. Anchor parameters, including `Authority`, `BondDenom`, `MinBond` and `Threshold`, come from keeper state; retirement checks read applied upgrade state. `abi.json` is embedded at compile time.

## Documentation

- [Precompile ABI catalogue and availability](../../docs/system/precompiles.md).
- [Native chain modules](../../docs/system/chain-modules.md).
- [Documentation site: checkpoints and block commitment](../../docs/site/paxeer-docs/src/app/checkpoints/page.tsx).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
