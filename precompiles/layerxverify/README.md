# LayerX Verify precompile

This directory holds the Go implementation, ABI, Solidity interface and tests for the `layerxVerify` EVM precompile. It remains registered in Paxeer at `0x0000000000000000000000000000000000001012`; once the `v7.0` merge upgrade is active, every method reverts with `LXP_MODULE_DISABLED: module disabled; layerxVerify retired`. Paxeer is one BFT network with the LayerX kernel embedded in the node, and a receipt is final when its block is committed.

## Layout

- [layerxverify.go](layerxverify.go): executor, ABI loading, gas calculation, verification handlers and merge activation check.
- [abi.json](abi.json): embedded ABI declaring `verifyEd25519`, `verifyReceipt`, `verifyReceiptInclusion`, `verifyStateProof` and `verifyDiscoveryProof`.
- [LayerXVerify.sol](LayerXVerify.sol): Solidity interface, address constant and result structs.
- [setup.go](setup.go): generated versioned precompile constructor.
- [versions](versions): precompile version metadata.
- [layerxverify_test.go](layerxverify_test.go): ABI, verification vectors, call restrictions and gas formula tests.
- [retire_gate_test.go](retire_gate_test.go): merge height behavior, refusal responses and EVM gas charging tests.
- [verifier_methods_retired_test.go](verifier_methods_retired_test.go): receipt inclusion, discovery proof and Ed25519 domain refusal tests.

## Build and test

This package belongs to the root [Go module](../../go.mod) and has no separate build manifest or Makefile. From the repository root, the targets in [chain.mk](../../chain.mk) cover it:

```sh
make -f chain.mk build
make -f chain.mk test
```

The build target compiles `paxd`, including its registered precompiles, and prepares native library dependencies. The test target runs the root Go test packages in groups with race detection and coverage, including this package.

## Configuration

The package reads no environment variables or runtime configuration files. It embeds `abi.json` at build time and receives the upgrade activation reader through the EVM keeper supplied to `NewPrecompile`. The applied `v7.0` upgrade height and the call's block height determine the merge refusal behavior.

## Documentation

- [Precompile ABIs and availability](../../docs/system/precompiles.md).
- [Settlement evidence and native verification](../../docs/wiki/SettlementEvidence.md).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Documentation site: Markdown precompile reference](../../docs/site/paxeer-docs/public/markdown/precompiles.md).
