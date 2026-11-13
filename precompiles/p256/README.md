# P-256 verification precompile

This directory holds the Go implementation, Solidity interface, ABI, and tests for P-256 (secp256r1) signature verification in Paxeer's EVM. The precompile exposes `verify(bytes)` at `0x0000000000000000000000000000000000001011`, allowing contracts to verify signatures within the network's execution. Its payload is exactly 160 bytes: a 32-byte digest followed by 32-byte unsigned big-endian values for `r`, `s`, public-key `x`, and public-key `y`. A valid signature returns ABI-encoded `bytes` containing a 32-byte word with value one; an invalid signature returns empty low-level call data. Incorrect payload lengths, nonzero call value, and delegatecalls are rejected.

## Layout

- [P256VERIFY.sol](P256VERIFY.sol): Solidity address constant and `IP256VERIFY` interface.
- [abi.json](abi.json): embedded ABI declaring the `verify(bytes)` view method and its bytes response.
- [p256.go](p256.go): precompile constructor, dispatch, input validation, result encoding, and gas calculation.
- [verifier.go](verifier.go): public-key validation and signature verification using Go's `crypto/ecdsa` and `crypto/elliptic` packages.
- [setup.go](setup.go): generated `GetVersioned` registration for the latest upgrade and versioned implementations.
- [versions](versions): version list used by the precompile versioning tooling.
- [legacy/](legacy/): versioned implementations and embedded ABIs.
- [p256_test.go](p256_test.go): precompile execution tests for valid and invalid signatures.
- [verifier_test.go](verifier_test.go): cryptographic verification tests, including invalid coordinates and nil inputs.
- [abi_freeze_test.go](abi_freeze_test.go): ABI, selector, and address checks.
- [gas_golden_test.go](gas_golden_test.go): gas and execution checks across the merge activation height.

## Build and test

The package belongs to the repository's root [Go module](../../go.mod) and uses the chain build and test targets. Run these commands from the repository root:

```sh
make paxd-build
make -f chain.mk test
```

The [root Makefile](../../Makefile) defines `paxd-build`, which builds the native libraries and the node containing the precompile. The test target in [chain.mk](../../chain.mk) discovers Go packages with tests, including this package, and runs them in groups with race detection and coverage.

The executor charges 300 gas per ABI argument byte, excluding the selector. The 160-byte payload occupies 224 encoded argument bytes, giving 67,200 precompile execution gas before other call or transaction costs.

## Configuration

The package reads no environment variables or runtime configuration files. It embeds [abi.json](abi.json) into the executable. `GetVersioned` receives the latest upgrade name from its caller and returns the corresponding implementations.

## Documentation

- [Precompile ABI reference](../../docs/system/precompiles.md): ABI catalogue and version selection.
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx): P-256 payload, return data, and gas semantics.
- [Precompiles Markdown page](../../docs/site/paxeer-docs/public/markdown/precompiles.md): the site's Markdown reference.
