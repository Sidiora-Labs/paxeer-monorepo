# Releasing paxd

A Paxeer X Network release is one git tag, one [`tools/release/version.json`](tools/release/version.json)
and a set of files built from that tag and recorded in `SHA256SUMS`. The merged network runs the
kernel inside `paxd`, including block proposal and execution. A receipt is final when its block is
committed. The EVM chain ID is `125` (`0x7d`); public node JSON-RPC uses the root URLs listed in
[Public RPC endpoints](docs/site/docs/reference/public-rpc.md).

Applying a release to a running network requires the matching governance upgrade plan and node
handler. The release tooling lives in [`tools/release`](tools/release). Its checked-in identity and
fleet scripts still target `v6.12.0`; they are version-specific tooling, not a general merged-network
activation procedure. The merge handler is registered separately as `MergeUpgrade` in
[`node/upgrades.go`](node/upgrades.go).

The v6.12.0 migration includes the SID holder migration and SID mint-authority move
as steps of the upgrade handler in [`node/upgrades_v6120.go`](node/upgrades_v6120.go).
These steps migrate the legacy SID balances to usid and move its mint authority to the
bridge module account; they ship together in the v6.12.0 release.

## Release identity

- `tools/release/version.json` currently holds `version` and `upgrade` both set to `v6.12.0`.
  `version` identifies the package (`vX.Y.Z`); `upgrade` identifies the on-chain upgrade plan
  name the release carries. The plan name must equal the constant the node registers for it (for
  `v6.12.0`, `V6120Upgrade` in [`node/upgrades_v6120.go`](node/upgrades_v6120.go), registered in
  [`node/upgrades.go`](node/upgrades.go)).
- The release tag is `paxeer-network/` + `version`. `tools/release/build-paxd.sh --print-tag` prints
  it.
- The `Paxeer / Release Check` workflow ([`.github/workflows/paxeer-release-check.yml`](.github/workflows/paxeer-release-check.yml))
  runs on pull requests that touch the node, the release tooling or the price sidecar. It refuses a
  version that is not semantic, an `upgrade` name that does not equal the declared `V6120Upgrade`,
  a tag that already exists, or a version that is not newer than the latest `paxeer-network/v*` tag,
  and then runs the
  reproducibility check. On pull requests it also checks consensus-changing commits against
  [the consensus commit policy](docs/release/consensus-commits.md). The identity check is currently
  tied to `node/upgrades_v6120.go`; changing `version.json` alone does not retarget it to another plan.

## Building a release

`tools/release/build-paxd.sh` builds the release from the current checkout. Its only flag is
`--print-tag`; the merged package uses `PAXD_DIST_DIR` (output directory), `PAXD_COMMIT` (version
metadata and commit-date source) and `SOURCE_DATE_EPOCH` from the environment. `PAXD_COMMIT` does
not check out that revision: build from the intended tag's checkout.

The separate `layerxd` binary is retired; the kernel and program runtime are linked into `paxd`.
The script retains the pre-merge `PAXD_RELEASE_LAYERXD` opt-in branch. Leave it unset when building
the merged package, including when running the reproducibility check:

```sh
env -u PAXD_RELEASE_LAYERXD tools/release/build-paxd.sh
env -u PAXD_RELEASE_LAYERXD tools/release/reproducible-check.sh
```

- Toolchains are pinned and checked before anything builds: Go must equal the `go` line of `go.mod`
  (`GOTOOLCHAIN=local`), `rustc` the channel of `rust-toolchain.toml`, and the C compiler the
  `CC_VERSION` of `tools/release/toolchain.env`.
- The kernel and programs native libraries are built with `make paxd-native-libs` and their hashes
  recorded in `native-libs.sha256`.
- `paxd` is built from `./daemon/paxd` with `-trimpath -buildvcs=false`, tags `netgo ledger`, an empty
  build id and the version variables (`Name`, `AppName`, `Version`, `Commit`, `BuildTags`), with
  `CGO_ENABLED=1`, fixed `CGO_*FLAGS`, `LC_ALL=C`, `TZ=UTC` and `SOURCE_DATE_EPOCH` defaulting to the
  commit date.
- The price sidecar is built from the `layerx-oracle-feeder` package of `services/platform`.

Output goes to `dist/paxeer-network/vX.Y.Z/` (or `PAXD_DIST_DIR`):

- `paxd-vX.Y.Z-<os>-<arch>` and `price-sidecar-vX.Y.Z-<os>-<arch>`;
- on linux/amd64, `libwasmvm.x86_64.so`, `libwasmvm152.x86_64.so` and `libwasmvm155.x86_64.so`,
  each checked against `wasm-runtime/libwasmvm-linux.sha256`;
- `native-libs.sha256`, recording `build/liblayerx.a` and
  `programs/target/release/liblayerx_programs_sandbox.a` (the archives are linked into the node,
  not copied into the release directory);
- `upgrade-info.json`, written by `tools/release/upgrade_info.py release`: the plan name and the
  download URL of the linux/amd64 `paxd` asset with its sha256, the document a governance upgrade
  proposal carries;
- `SHA256SUMS` over every other file.

`tools/release/reproducible-check.sh` builds twice into separate directories, removes the kernel
archive, generated build directory and programs target directory between builds, and uses an empty
Go build cache for the second build. It prints both `SHA256SUMS` and fails unless the native library
hashes and every file hash match. The builder recreates its output directory, so use a dedicated
`PAXD_DIST_DIR`. The release metadata generator requires the linux/amd64 node asset; use a
linux/amd64 build host for the complete release package.

## Publishing

1. Before publishing a merged-network release, align the release workflows with the package above.
   The checked-in check and publish workflows still set `PAXD_RELEASE_LAYERXD=1`, a pre-merge
   setting for the retired daemon. Their current configuration is not the merged package recipe.
   A passing run with that setting does not establish that only the merged assets were built.
2. Open a pull request that sets `version` and `upgrade` in `tools/release/version.json`, with the
   upgrade handler registered under that name. `Paxeer / Release Check` validates the identity and
   runs the reproducibility check.
3. Merge it to `main` after the workflow configuration and plan checks agree with the release.
4. Tag the merge commit and push the tag:

   ```sh
   git tag -s paxeer-network/vX.Y.Z -m "Paxeer X Network vX.Y.Z" <commit>
   git push origin paxeer-network/vX.Y.Z
   ```

5. `Paxeer / Release Publish` ([`.github/workflows/paxeer-release-publish.yml`](.github/workflows/paxeer-release-publish.yml))
   runs on the tag, or by hand with an existing tag as input. It checks the tag against
   `version.json`, invokes `build-paxd.sh`, generates release notes with git-cliff from
   [`.github/cliff.toml`](.github/cliff.toml), and creates the GitHub release with every file of the
   release directory attached. The run summary lists `SHA256SUMS` and the release URL.
6. On the build host, compare the published release with a local build of the same tag:

   ```sh
   tools/release/verify-release.sh --tag paxeer-network/vX.Y.Z --dist dist/paxeer-network/vX.Y.Z
   ```

   It requires the asset names to equal the files of the local release directory, every published
   `SHA256SUMS` line to equal the local line, and every file on both sides to match its own
   `SHA256SUMS`. It needs an authenticated `gh`.

## Verifying a published binary

Anyone can rebuild a tag and compare the merged package. For a historical tag, inspect its build
script and asset list first; its package may include retired components.

```sh
git clone https://github.com/Sidiora-Labs/Paxeer-X-Network.git
cd Paxeer-X-Network
git checkout paxeer-network/vX.Y.Z
env -u PAXD_RELEASE_LAYERXD tools/release/build-paxd.sh
gh release download paxeer-network/vX.Y.Z --pattern SHA256SUMS --dir published
diff published/SHA256SUMS dist/paxeer-network/vX.Y.Z/SHA256SUMS
```

Build on linux/amd64 with the pinned Go, Rust and C toolchains. An empty diff means the published
files are the ones the tag builds. To check downloaded files without rebuilding, run
`sha256sum --check SHA256SUMS` in the download directory.

## Rehearsing the upgrade

`tools/release/rehearse-v6120.sh` rehearses the v6.12.0 plan on a local four-validator network built
from real chain state. It exports a stopped full node's data with the previous
binary, rewrites the genesis to local validator keys, halts the network with `--halt-height`, archives
every home with `snapshot.sh`, writes the node upgrade file and restarts on the release candidate. It
then checks the applied plan, the version, the precompile set, launchpad pool parity, the markets,
the feed keys and fresh sidecar prices, the programme checks and the block hashes after the upgrade,
and finally runs `rollback.sh --execute` and requires the restored chain to resume on the previous
binary. Inputs come from flags or from a filled copy of
[`tools/release/env/rehearsal.env.example`](tools/release/env/rehearsal.env.example) passed with
`--env-file`; `--check` validates the inputs and touches nothing, and `--receipt` writes the receipt
that `fleet_check.py` reads later. Run it with `--help` for every flag. The rollback leg runs on an
isolated fork with a different consensus chain identity, fresh validator keys and loopback peers;
it does not authorize restoring a live network to a pre-upgrade snapshot. Price sidecars still need
their configured feed service. This rehearsal does not prove a different upgrade plan.

## Rolling out to a fleet

The fleet scripts read one operator input file, a filled copy of
[`tools/release/env/rollout.env.example`](tools/release/env/rollout.env.example) kept outside the
repository. It names the release id, tag and checksums, the halt height, the chain id, the binary
directory, the hosts, node homes, service units and operator key names, the snapshot and credential
directories, and the service endpoints. Required non-empty values depend on the script and phase;
role-specific lists may be empty. `CHAIN_ID` is the Cosmos consensus chain ID, distinct from the
EVM chain ID `125`.
Each script takes `--env-file`, most take `--receipt <file>` to write a JSON receipt of the step, and
`--service-manager systemd|foreground` where they start or stop nodes (default `systemd`). The
complete step-by-step procedure, with the operator inputs and the checks after each step, is the
[v6.12.0 rollout runbook](spec/release-v6-12-0/docs/rollout-runbook.md). These scripts carry the
`v6.12.0` file names and plan name; use them only with that plan and its matching candidate.

The order of a rollout:

1. **Inputs and preflight.** `fleet_check.py --env-file <file> --phase inputs` validates the operator
   inputs, the release binaries in the working directory and ssh access to every host;
   `--phase preflight` checks every node before the window. `verify-release.sh` confirms the published
   release.
2. **Stage.** On each host, with the release binaries in the working directory,
   `stage.sh --env-file <file> --local` checks the checksums and shared libraries, keeps the running
   binary as `paxd.pre-<RELEASE_ID>` and installs the new `paxd` and price sidecar beside it.
   `stage.sh --verify` checks every host over ssh, `--check` validates the inputs only and
   `--automation-status` reports the fleet automation and old feeder units that must be stopped for
   the window.
3. **Freeze.** Each validator sets `halt-height` in `app.toml` to the chosen halt height `H`, one at a time,
   and stops itself after committing that block. `fleet_check.py --phase frozen` requires every
   validator to report that height.
4. **Snapshot.** With the nodes stopped, `snapshot.sh --env-file <file>` archives the `data` directory
   of every validator and full-node home into the snapshot directory and checks each archive,
   including the validator signing state at the halt height. `snapshot.sh --verify` re-checks the
   archives, and `rollback.sh --readiness` confirms the rollback material is in place.
5. **Swap.** `swap.sh --env-file <file> --role full`, then `--role validator`, writes
   `data/upgrade-info.json` into each home at upgrade height `H + 1` (through `upgrade_info.py node`),
   resets `halt-height` to `0`, swaps the binary after checksum and shared-library checks, starts
   the units and waits for each node to report ready. `--halt-height` and `--release-doc` select
   the halt height and release document; `--verify`
   re-checks a swapped host. `fleet_check.py --phase validators` follows.
6. **Price sidecars.** Each validator operator creates a feed key with
   `feed_key.py new --out <key file>`, registers it with the price feed precompile using the call data
   that `feed_key.py payload <public key>` prints, and starts its price sidecar from the unit and
   configuration template in [`tools/price-sidecar`](tools/price-sidecar).
   `fleet_check.py --phase oracle` requires fresh prices and one feed key per bonded validator.
7. **Services and verification.** The off-chain data services and frontends are cut over, then
   `fleet_check.py` runs the `markets`, `precompiles`, `services`, `full` and `post` phases. With no
   `--phase` it runs `post`. Expected values come from the rehearsal receipt (`--rehearsal <file>`);
   `--no-rehearsal` takes them from the input file for supported diagnostic phases, but is refused
   for `full` and `post`. Those phases require a successful receipt with `rollback_rehearsed=true`
   and the candidate's `new_bin_sha256`, supplied by `--rehearsal` or `RECEIPT_DIR/rehearsal.json`.

### Rollback

[`rollback.sh`](tools/release/rollback.sh) refuses a same-chain snapshot restore. Its snapshot
restore mode is limited to an explicitly configured isolated fork (`ROLLBACK_ISOLATED_FORK=true`
and `ROLLBACK_SOURCE_GENESIS`) with a different consensus chain ID, fresh validator keys and
loopback networking with peer discovery disabled. Retaining `paxd.pre-<RELEASE_ID>` and a snapshot
does not make a production rewind valid.

For the live network, stage a recovery `paxd` that understands the committed application state,
outside `BIN_DIR/paxd` and `BIN_DIR/paxd.new`. Set `RECOVERY_BIN` and `RECOVERY_SHA256` in the private
input file, then use `rollback.sh --env-file <file> --forward --execute`. Forward recovery keeps
the live data and upgrade metadata, journals the validator signing high-water mark, swaps the
executable, clears the configured halt and restarts. It refuses a regressed signing state or a
changed recovery checksum. Check the result with `--forward --verify`; `--forward --readiness`
checks recovery material but does not establish application-state compatibility.

From the operations host, the script drives the fleet over ssh, one phase across all hosts before
the next. `--local` runs on the current host only, where `--phase prepare|stop|restore|start` selects
a single phase. Keep recovery inputs, snapshots, credentials and raw receipts in access-controlled
storage.

## Testing the release tooling

`tools/release/tests/run-all.sh` runs every release tooling suite once (build, identity, reproducibility,
verify-release, rehearsal, stage, swap, snapshot, rollback, env templates and workflows) followed by
the operator-input and receipt checks.

## Sign-off checklist

- [ ] `version` and `upgrade` in `tools/release/version.json` are correct and the upgrade handler for
      `upgrade` is registered in `node/upgrades.go`.
- [ ] `Paxeer / Release Check` passed on the release commit.
- [ ] Release workflow configuration builds the merged package with the retired daemon option unset,
      and the plan validation matches the intended upgrade handler.
- [ ] `Paxeer / Release Publish` is green and the release carries `paxd`, the price sidecar,
      the three linux/amd64 libwasmvm libraries, `native-libs.sha256`, `upgrade-info.json` and
      `SHA256SUMS`.
- [ ] `verify-release.sh` passed on the build host, and at least one maintainer rebuilt the tag and got
      the same `SHA256SUMS`.
- [ ] The upgrade rehearsal passed on the release candidate and its receipt is kept for
      `fleet_check.py`.
- [ ] The governance upgrade proposal names the `upgrade` plan and carries `upgrade-info.json`, and
      validators had the release URL and hashes before the vote ended.
- [ ] The rollout pins the published `paxd` sha256.
