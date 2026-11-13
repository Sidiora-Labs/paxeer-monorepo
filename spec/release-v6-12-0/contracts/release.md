# Release artifacts and activation

The release tools build and deploy the merged `paxd` binary, its native libraries and the validator price sidecar. Release identity comes from `tools/release/version.json`: the current binary version is `v7.0.0` and the plan is `v7.0`. The `v6.12.0` activation handler remains registered in `node/upgrades_v6120.go`; its name is distinct from the current release identity.

## Build and plan

1. `bash tools/release/build-paxd.sh` builds the checked-out revision; `--print-tag` prints `paxeer-network/<version>`. The script requires a clean tree and, when `PAXD_COMMIT` is supplied, requires it to equal HEAD.
2. `tools/release/version.json` supplies the semantic version and upgrade plan to the build, release workflows, upgrade document, staging, fleet checks and recovery scripts.
3. The build requires the Go version in `go.mod` with `GOTOOLCHAIN=local`, the channel in `rust-toolchain.toml`, and the C and C++ compiler versions in `tools/release/toolchain.env`.
4. `make paxd-native-libs` builds `build/liblayerx.a` and `programs/target/release/liblayerx_programs_sandbox.a`. The release records both hashes and checks the combined native archive digest embedded in `paxd`.
5. Linker flags stamp the version, commit and `netgo,ledger` build tags. ABCI info reports the application version. `SOURCE_DATE_EPOCH` defaults to the source commit time; Go uses `-trimpath -buildvcs=false` and an empty build id, and C and Rust paths are remapped.

## Release artifacts

The directory `dist/paxeer-network/<version>/` contains, for linux/amd64:

- `paxd-<version>-linux-amd64` and `price-sidecar-<version>-linux-amd64`;
- the three pinned libwasmvm shared libraries checked against `wasm-runtime/libwasmvm-linux.sha256`;
- `source-revision.txt` and `native-libs.sha256`;
- `upgrade-info.json` and `SHA256SUMS`.

`upgrade-info.json` names the manifest plan and maps `linux/amd64` to the binary asset URL with its SHA-256 checksum. `SHA256SUMS` covers the other files. The builder regenerates checksums after writing the upgrade document and applies `SOURCE_DATE_EPOCH` after the final writes.

`tools/release/reproducible-check.sh` compares independent builds. After publication, `tools/release/verify-release.sh --tag <tag> --dist <directory>` compares every remote asset and checksum with the local directory. The publish workflow checks the tag against the manifest, builds the release and publishes its directory.

On a node, `<home>/data/upgrade-info.json` contains `name` and `height`. The upgrade keeper applies a pending registered plan at that exact height (`node/upgrades.go`). Staging and swapping use the current manifest identity rather than a hard-coded activation name.

## Upgrade rehearsal

`tools/release/rehearse-v6120.sh` rehearses the upgrade with local validator homes, the source state, the old and new binaries and the price-sidecar configuration. Inputs are declared in `tools/release/env/rehearsal.env.example`; plan v7.0 also requires `--plan-info <file>`. The rehearsal checks plan application, application version, matching block hashes, precompile interfaces, pool migration, venue economics, prices and feed keys through `tools/release/rehearsal/programme.py`.

Snapshot restoration is supported only for an isolated local fork with a distinct consensus chain identity, fresh consensus keys and loopback-only networking. It does not restore a live network's signing state to an earlier height.

## Rollout

The [rollout runbook](../docs/rollout-runbook.md) describes operator inputs and deployment commands. Validator and full-node homes halt at H before replacement; the upgrade file uses H1 = H + 1. A shared binary is replaced only after every affected home has stopped. `stage.sh` inspects process ownership by home, checks checksums and shared libraries and retains the previous binary. `snapshot.sh` archives stopped homes, including signing state. `swap.sh` performs an atomic binary replacement and starts the selected units.

The deployment order is preflight, staging, automation pause, halt, snapshots, full-node and validator replacement, feed-key registration, sidecar start, data services, remaining nodes, verification and automation resumption. The archive node retains its data.

## Rollback

`tools/release/rollback.sh` provides `--readiness`, `--execute` and `--verify`. For a live network, use `--forward` with `RECOVERY_BIN` and `RECOVERY_SHA256` naming a recovery binary compatible with the current state. Forward recovery retains the live data and signing state, journals recovery material, replaces the binary atomically and verifies that height advances.

Snapshot restoration requires `ROLLBACK_ISOLATED_FORK=true`, `ROLLBACK_SOURCE_GENESIS`, a different consensus chain identity, fresh consensus identities and loopback-only networking. Its archives are `<home>-pre-<version>.tar.zst`; restored data is set aside as `data.<version>-<RELEASE_ID>`. This mode is for isolated rehearsals.

## Post-rollout verification

`python3 tools/release/fleet_check.py --env-file <file> --phase <phase>` checks the configured inventory. Phases are `inputs`, `preflight`, `frozen`, `validators`, `oracle`, `markets`, `precompiles`, `services`, `full` and `post`.

Checks cover the manifest version and applied plan, height and block-hash agreement, served precompile calls, launchpad pool count, eighteen venue markets, fresh prices, one feed key per bonded validator, native token pointers and USDL decimals. Service checks cover `/healthz/ready` and the application's `/api/build-id`. `--receipt <path>` writes a JSON result with counts and pass flags.

## Price sidecar

The `price-sidecar` binary is built with `cargo build --locked --release` from the `layerx-oracle-feeder` package in `services/platform/Cargo.toml`. Each validator registers its own compressed secp256k1 feed public key at 0x1020 after associating the operator EVM address. `tools/release/feed_key.py new` creates the feed key and `payload` encodes `registerFeedKey(bytes)`.

`tools/price-sidecar/price-sidecar.service` loads feed and Chainflow keys as systemd credentials. Each instance needs separate credentials, configuration, state and health-listener settings. `/readyz` succeeds only after successful committed submission execution and expires after three cadences. See [pricefeed](pricefeed.md#install).

## Cut-over

The data services start after fresh venue prices are available: `perps-data-indexer` with `PERPS_START_HEIGHT=H1`, then `perps-gateway`, `perps-explorer` and `perps-portal`, checking readiness between services. Chainflow supplies charts and market data; venue logs supply fills, book and account state.

The Hyperpax application deployment uses `Hyperpax:deploy/scripts/deploy.sh`. Its `/api/build-id` identifies the build. The spot route, navigation entry and markets mode use `NEXT_PUBLIC_RELEASE_SPOT_OPEN`; the spot-enabled build is deployed after post-rollout verification.

## Operator inputs

`tools/release/env/rehearsal.env.example` and `tools/release/env/rollout.env.example` declare deployment inputs. Filled copies hold the inventory, endpoints, homes, service units, hashes and halt height in access-controlled storage. Quote space-separated list values when sourcing an env file. The scripts require the inputs they consume.

The `v6.12.0` handler runs migrations and the SID authority and holder hooks before the pointer, launchpad, pricefeed, perpetuals and spot activation hooks. These hooks remain part of the registered activation behavior; the release manifest governs the version and plan selected by current tooling.
