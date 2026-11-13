# Test fixtures

This directory holds shared test data for Paxeer's native kernel, SDKs and chain execution tests: encoded activities, signed receipts, inclusion proofs, genesis artifacts, trading payloads and example contracts. These fixtures let consumers check the byte formats and execution evidence used by the kernel within the Paxeer network.

## Layout

- [asset/](asset/) — asset withdrawal and supply receipts, proofs and a supply receipt generator.
- [authority/](authority/) — provider consent, fee grant and session activity and receipt vectors.
- [custody/](custody/) — credit, profile, state and receipt evidence for custody tests.
- [example/](example/) — Solidity sources, ABI files, bytecode and Go bindings, plus CosmWasm contract crates and WebAssembly artifacts.
- [governance/](governance/) — owner rotation activities and receipts, and handover maintenance evidence.
- [programs/](programs/) — program execution responses, call graphs, terminal outcomes, activities and receipts.
- [public-testnet-genesis/](public-testnet-genesis/) — genesis request, manifest, snapshot, registration request, deployment descriptor and expected hashes.
- [public-testnet-genesis-v3/](public-testnet-genesis-v3/) — a second genesis artifact set with its own request and expected hashes.
- [public-testnet-modules/](public-testnet-modules/) — the asset and module registry in `registry.json`.
- [trading-payloads/](trading-payloads/) — `vectors.json` pairs perpetual and spot activity fields with their encoded payload bytes.
- [web/](web/) — the encoded observation activity in `observation-activity.hex`.
- [fee-params-v3.bin](fee-params-v3.bin) — encoded fee parameters used by genesis request producers.
- [fee-params-v4.bin](fee-params-v4.bin) — encoded fee and module parameters used by genesis request producers.
- [pay1-retired-genesis.manifest](pay1-retired-genesis.manifest) — genesis manifest input for issuance migration tests.
- [pay1-retired-issuance.lxs](pay1-retired-issuance.lxs) — issuance snapshot input for migration tests.
- [receipt-supply-v2.bin](receipt-supply-v2.bin) — native supply receipt bytes consumed by receipt readers and the supply receipt generator.
- [regen-retired-genesis.sh](regen-retired-genesis.sh) — rebuilds the issuance migration genesis manifest with the genesis builder.
- [regenerate_public_genesis.py](regenerate_public_genesis.py) — rebuilds both public testnet genesis artifact sets and updates their expected hashes and the kernel test's pinned manifest hashes.

## Build and test

The shared vectors are data files and have no standalone build. Run the following targets from the repository root; their definitions are in the root [Makefile](../../Makefile).

```sh
make test-kernel
make test-genesis-bootstrap
```

`test-kernel` builds and runs the kernel suite, including checks of the public testnet genesis fixtures. `test-genesis-bootstrap` builds and runs the genesis manifest, module table and builder CLI tests, including the issuance migration inputs.

The CosmWasm crates under `example/` are included in the root manifest targets, which also cover other repository packages:

```sh
make paxeer-manifest-install
make paxeer-manifest-build
make paxeer-manifest-test
```

The build and test targets use locked, offline Cargo dependencies. The install target fetches those dependencies.

To regenerate supply receipts from the repository root, use the supplied script:

```sh
sh tests/fixtures/asset/generate_supply_receipts.sh
```

It compiles [generate_supply_receipts.c](asset/generate_supply_receipts.c) against the native receipt encoder and writes the supply receipt fixtures. Solidity artifact regeneration commands are listed in [example/contracts/README](example/contracts/README).

## Configuration

`asset/generate_supply_receipts.sh` reads the `CC` environment variable to select the C compiler.

`regenerate_public_genesis.py` takes the kernel suite executable through its required `--kernel-suite` argument. It reads `expected.json` and `genesis-request.lxgb` in each public testnet genesis directory, and [lxp_test_audit_m966.h](../protocol/lxp_test_audit_m966.h) for the pinned manifest hashes. It also requires OpenSSL.

Each CosmWasm example declares its dependencies and features in `Cargo.toml` and pins dependencies in `Cargo.lock`.

## Documentation

- [Protocol](../../docs/wiki/Protocol.md) — activity and receipt formats.
- [Assets](../../docs/wiki/Assets.md) — native asset behavior.
- [Custody](../../docs/wiki/Custody.md) — custody evidence and credit handling.
- [Programs](../../docs/wiki/Programs.md) — native program execution and terminal evidence.
- [Receipts on the documentation site](../../docs/site/paxeer-docs/public/markdown/receipts.md).
- [Assets on the documentation site](../../docs/site/paxeer-docs/public/markdown/assets.md).
- [Custody on the documentation site](../../docs/site/paxeer-docs/public/markdown/custody.md).
- [Programs on the documentation site](../../docs/site/paxeer-docs/public/markdown/programs.md).
