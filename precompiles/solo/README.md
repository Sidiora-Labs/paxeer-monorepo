# Solo precompile

This directory implements the Solo asset-claim precompile in the Paxeer node's EVM. It exposes `claim(bytes)` and `claimSpecific(bytes)` through the Solidity interface and embedded ABI. Both accept a signed Cosmos transaction payload authorizing a transfer to the native account resolved for the EVM caller. `claim` transfers the sender's native balances; `claimSpecific` transfers selected native assets, CW20 balances, or CW721 tokens. Calls must come directly from an EVM externally owned account, be nonpayable, and allow state changes. Signature validation checks the sender, claimer, and account sequence, then advances the sequence on success.

## Layout

- [solo.go](solo.go): dynamic-gas executor, signed-payload validation, asset transfers, and CosmWasm query helpers.
- [Solo.sol](Solo.sol): `ISolo` interface and precompile constants.
- [abi.json](abi.json): embedded ABI for the two claim methods.
- [setup.go](setup.go): generated `GetVersioned` registration of the current and upgrade-specific implementations.
- [versions](versions): version list used for precompile versioning.
- [legacy/](legacy/): version-specific Go implementations and ABIs.
- [solo_test.go](solo_test.go): execution restrictions, signatures, replay protection, and native, CW20, and CW721 claim tests.
- [abi_freeze_test.go](abi_freeze_test.go): ABI, selector, and event assertions.
- [gas_golden_test.go](gas_golden_test.go): dynamic-gas assertions across merge activation.
- [repair_m1284_test.go](repair_m1284_test.go): CW721 pagination bounds and claim atomicity tests.
- [repair_m1284.kvx](repair_m1284.kvx): CW721 enumeration limits and atomicity policy.
- [testdata/](testdata/): compressed CW721 pagination fixtures and their Python generator.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no separate build manifest or Makefile. From the repository root, use the targets in the root [Makefile](../../Makefile), which delegate to [chain.mk](../../chain.mk):

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` builds `build/paxd`, including the registered Solo precompile and native dependencies. `paxeer-test` runs the root Go package tests, including Solo, in package groups with race detection and coverage. Both targets use locally available Go dependencies.

The claim tests load [cw20_base.wasm](../../contracts/wasm/cw20_base.wasm), [cw721_base.wasm](../../contracts/wasm/cw721_base.wasm), and the committed fixtures in [testdata/](testdata/).

## Configuration

Solo reads no package-specific environment variables or runtime configuration files. The ABI in [abi.json](abi.json) is embedded at compile time. The node supplies keepers and transaction decoding through `NewPrecompile`; `GetVersioned` supplies implementations keyed by upgrade name.

CW721 enumeration uses the constants `MaxCW721ClaimPages` and `MaxCW721ClaimTokens` in [solo.go](solo.go): at most 32 queries, including the terminal empty page, and 256 tokens per asset. Token identifiers must be unique and strictly increasing. Larger collections can use `claimSpecific` with an explicit token denomination. A specific claim commits asset transfers, account sequence, and events together after all selected assets succeed.

## Documentation

- [Precompile reference](../../docs/system/precompiles.md): Solo ABI signatures and registration details.
- [Documentation site: Precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx): network precompile overview and Solo interface.
- [Documentation site: Markdown page](../../docs/site/paxeer-docs/public/markdown/precompiles.md): the precompile guide in Markdown.
