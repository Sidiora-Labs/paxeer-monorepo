# HPX node distribution and peer registry

This directory holds the HPX installer, node manager, artifact publisher, peer
registry and fleet tools for Paxeer mainnet-beta (consensus chain ID
`hyperpax_125-1`, EVM chain ID `125`, or `0x7d`). Nodes run a native `paxd`
binary under systemd. The kernel is part of this BFT network and proposes
blocks; a receipt is final when its block is committed. Generated executables,
native libraries, live chain configuration, registry data and TLS material are
published outside Git.

## Layout

- [hosting/](hosting/) — landing page, logo, font checksums, Nginx configurations, Fly configuration and deployment scripts, including the deployment preflight test.
- [registry/](registry/) — Go peer registry and artifact server, its module manifest and tests.
- [.ascii](.ascii) — text banner artwork.
- [hpx](hpx) — Bash node manager with setup, service lifecycle, peer discovery, state sync and validator commands.
- [get-hpx.sh](get-hpx.sh) — web installer that verifies and installs the CLI and offers interactive setup.
- [install.sh](install.sh) — installs the local CLI and Bash completion.
- [uninstall.sh](uninstall.sh) — removes the CLI, node binary and systemd units, with separate confirmation for deleting node data.
- [publish.sh](publish.sh) — assembles immutable node releases and switches the artifact mirror's current release.
- [release_identity.py](release_identity.py) — validates the release manifest, executable identity, provenance and artifact digest.
- [fleet-keygen.sh](fleet-keygen.sh) — distributes the CLI and collects public operator identities from selected nodes.
- [stake-fleet.sh](stake-fleet.sh) — collects identities, funds operator wallets and drives validator staking.
- [fleet_state.py](fleet_state.py) — persists the fleet roster, allocation plan and signed payment journal.
- [README.md](README.md) — component layout, commands, configuration and documentation.

## Build and test

Run repository commands from the monorepo root unless noted. The shell and
Python tools run directly; the registry is a separate Go module whose
[go.mod](registry/go.mod) declares its Go version and toolchain.

The root [Makefile](../../Makefile) includes `tools/hpx/registry` in
`PAXEER_NESTED_GO_DIRS`. Its `paxeer-manifest-build` and
`paxeer-manifest-test` targets run these commands inside that directory:

```bash
GOPROXY=off go build ./...
GOPROXY=off go test ./...
```

The registry tests cover registration, forwarding headers, ownership proofs,
persistence and recovery, peer bounds, artifact routing and state-sync queries.
The deployment preflight test is [hosting/deploy.test.sh](hosting/deploy.test.sh);
it requires root and the deployment environment variables read by
[hosting/deploy.sh](hosting/deploy.sh). Run it from the monorepo root with:

```bash
bash tools/hpx/hosting/deploy.test.sh
```

The [registry release workflow](../../.github/workflows/paxeer-hpx-registry.yml)
builds Linux amd64 and arm64 executables with the source identity embedded and
publishes their checksum files under the release tag derived from the source
revision. It also publishes a multi-architecture GHCR image of the Go service.
Changes on `main` to the registry or its workflow trigger publication; manual
dispatch is also supported. Generated executables stay outside Git.

The separate [container Dockerfile](../../docker/hpx-registry/Dockerfile)
builds the registry and packages Nginx, the landing page, logo and verified
fonts. It is selected by [hosting/fly.toml](hosting/fly.toml). The container
[entrypoint](../../docker/hpx-registry/entrypoint.sh) starts Nginx and runs the
registry as user `hpx`; it exits when either process stops. The Fly configuration
uses one machine and a volume containing the artifacts and registry data, and
checks `/healthz`.

The root target `make hpx-public-check` checks the deployed mirror selected by
`HPX_ORIGIN`: health, chain metadata, checksums, artifact availability, landing
page and rejection of undeclared paths. The script
`tools/bringup/check-live.sh hpx` checks the mirror and its Fly app, including
checksums and the public name's mapping to the app. Edge mapping is managed by
[tools/bringup/edge.sh](../bringup/edge.sh).

### Install and operate a node

Use Linux with systemd and an x86-64 or AArch64 CPU. Installation and node
lifecycle changes require root privileges. The web installer can run directly
from the checkout:

```bash
sudo bash tools/hpx/get-hpx.sh
```

The web installer verifies the HPX CLI against `checksums.txt`. Setup verifies
`chain-info.json`, then `paxd` and the three libwasmvm runtimes for the host
architecture, then genesis and the selected fullnode or validator configuration.
The executable and libraries are installed before the chain files. Integrity
checks use the selected mirror's manifest; select an explicitly trusted mirror
with `HPX_MIRROR`.

Run `hpx` without arguments for an interactive menu. Its commands include:

```bash
hpx setup
hpx status
hpx info
hpx logs
hpx start
hpx stop
hpx restart
hpx update
hpx peers show
hpx peers refresh
hpx register
hpx statesync
hpx validator keygen
hpx validator stake
hpx validator status
hpx remove
```

The node runs as `paxd.service`; the peer-refresh timer periodically refreshes
peers and restarts the service. `hpx statesync` stops the service, applies trust
parameters from the mirror, invokes `paxd tendermint unsafe-reset-all --home`
with the node home (trying `paxd comet unsafe-reset-all --home` if needed), then
starts the service. `hpx remove` removes the node service and refresh units;
[uninstall.sh](uninstall.sh) also removes the CLI, completion and node binary.
Both request separate confirmation before deleting node data and keys.

### Publish node artifacts

The publisher reads the executable selected by `SRC_BIN`, native libraries
under `WASM_ROOT`, and genesis and configuration selected by `SRC_CFG` or
`HPX_RUNTIME_CONFIG_DIR`. Required libraries are under
[wasm-runtime/internal/api](../../wasm-runtime/internal/api/) and
[wasm/x/wasm/artifacts](../../wasm/x/wasm/artifacts/), including both x86-64 and
AArch64 variants of all three runtimes. Required configuration files are
`genesis.json`, `config.toml` and `app.toml`. `HPX_BUILD_PROVENANCE` selects the
build provenance JSON, and `HPX_RELEASE_REPO` identifies the repository used in
the generated binary-download URL. The lifecycle scripts are included
in the release. Supply at least three distinct seed peers through
`HPX_SEED_PEERS` before invoking:

```bash
sudo tools/hpx/publish.sh
```

Pass the intended inputs through `sudo env` when overriding settings. The
publisher checks the genesis chain ID, generates fullnode and validator
configuration variants, and clears published peer and external-address settings.
It stages a release under the configured artifacts root, writes a sorted
SHA-256 manifest, makes the release read-only and atomically switches `current`
to the release directory. Existing release IDs are rejected; a staging failure
leaves `current` unchanged. The generated release ID combines a UTC timestamp
with the first 12 hexadecimal characters of the executable's SHA-256 digest.

Before publication, [release_identity.py](release_identity.py) compares
[tools/release/version.json](../release/version.json), the staged executable's
`version --long --output json` output and the supplied build provenance.
Their versions must agree with `HPX_PAXD_VERSION` when supplied. The executable's
`commit` and provenance's `source_revision` must match the checkout's full
source identity, and `server_name` must identify `paxd`. Provenance contains
`version`, `source_revision` and `paxd_sha256`; its digest must match the exact
staged executable. Missing or inconsistent identity data aborts publication.

The release's checksums cover `binary-info.json`, `build-provenance.json` and
`version.json`. Generated `chain-info.json` records the validated version,
commit, source revision and digest; `release_id` separately names the artifact
directory. When copying a release to the served volume, verify its checksums
before switching that volume's `current` symlink.

### Registry HTTP surface

The [registry implementation](registry/main.go) declares these routes:

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz` | Registry liveness, chain and source revision |
| GET | `/checksums.txt`, `/paxd.sha256`, `/chain-info.json` | Release integrity and chain metadata |
| GET | `/paxd`, `/lib/*.so`, `/genesis.json`, `/config/<type>/<file>` | Explicitly declared node artifacts |
| GET | `/install`, `/get-hpx.sh`, `/install.sh`, `/hpx`, `/uninstall`, `/uninstall.sh` | Lifecycle scripts and CLI |
| GET | `/api/myip` | Caller's observed public address |
| GET | `/api/register/challenge` | Obtain a source-bound node ownership challenge |
| POST | `/api/register` | Register a peer with its signed ownership proof |
| GET | `/api/peers`, `/api/peers.txt`, `/api/nodes` | Registry discovery |
| GET | `/api/statesync` | Current state-sync trust parameters |

Registration requires a fresh challenge, an Ed25519 public key and a signature
that binds the normalized node record to the chain and challenge. The public
key must derive the announced CometBFT node ID. Challenges expire after two
minutes and are consumed by a successful proof; the source address must match
the address observed when the challenge was issued. When configured, the
`X-HPX-Token` is required in addition to the ownership proof.

Artifact routes also accept HEAD. The table's library and configuration patterns
refer only to the explicit allowlist: three library versions for each supported
architecture and `app.toml` / `config.toml` for `fullnode` / `validator`.
Directory indexes and undeclared artifact paths return 404. Nginx serves the
landing page at `/` and its declared logo and font assets.

## Configuration

The tools and deployment entrypoints read these environment variables:

| Consumer | Environment variables |
| --- | --- |
| Node manager and installers | `HPX_MIRROR`, `HPX_CHAIN_ID`, `HPX_HOME`, `HPX_TYPE`, `HPX_TOKEN`, `HPX_UNIFIED_GATEWAY_URL`, `HPX_GATEWAY_TOKEN_FILE` |
| Validator commands | `HPX_OPERATOR_KEY`, `HPX_KEYRING_BACKEND`, `HPX_KEYRING_DIR`, `HPX_MIN_SELF_DELEGATION`, `HPX_COMMISSION_RATE`, `HPX_COMMISSION_MAX_RATE`, `HPX_COMMISSION_MAX_CHANGE_RATE`, `HPX_FEE_RESERVE_UHPX`, `HPX_GAS_PRICES`, `HPX_STAKE_UHPX`, `HPX_STAKE_PAX`, `HPX_YES` |
| Artifact publisher | `SRC_BIN`, `SRC_CFG`, `WASM_ROOT`, `HPX_RUNTIME_CONFIG_DIR`, `HPX_ARTIFACTS_ROOT`, `HPX_PAXD_VERSION`, `HPX_BUILD_PROVENANCE`, `HPX_RELEASE_ID`, `HPX_SEED_PEERS`, `HPX_CHAIN_ID`, `HPX_EVM_CHAIN_ID`, `HPX_P2P_PORT`, `HPX_RPC_PORT`, `HPX_RELEASE_REPO` |
| Registry | `HPX_ADDR`, `HPX_ARTIFACTS_DIR`, `HPX_DATA_DIR`, `HPX_REGISTER_TOKEN`, `HPX_CHAIN_ID`, `HPX_SEED_PEERS`, `HPX_STATESYNC_RPC`, `HPX_STATESYNC_RPC_SERVERS`, `HPX_STATESYNC_TRUST_OFFSET`, `HPX_STATESYNC_TRUST_PERIOD` |
| Host deployment | `HPX_SOURCE_REVISION`, `HPX_ARTIFACTS_ROOT`, `HPX_DATA_DIR`, `HPX_WEB_ROOT`, `HPX_REGISTRY_BIN`, `HPX_REGISTRY_ENV`, `HPX_CHAIN_ID`, `HPX_SEED_PEERS`, `HPX_STATESYNC_RPC`, `HPX_STATESYNC_RPC_SERVERS`, `HPX_REGISTER_TOKEN`, `LETSENCRYPT_EMAIL` |
| Container entrypoints | `PORT`, `HPX_ADDR`, `LAYERX_TRUSTED_PROXIES` |
| Fleet identity collection | `HPX_LOCAL`, `HPX_WORKDIR`, `HPX_MIRROR`, `HPX_SSH_USER`, `HPX_PUSH`, `HPX_SSH_OPTS`, `HPX_SCP_OPTS`, `SERVERS`, `SERVERS_FILE` |
| Fleet funding and staking | `HPX_ENV_FILE`, `PAXD`, `HPX_WORKDIR`, `HPX_KEYRING_DIR`, `HPX_CHAIN_ID`, `HPX_RPC`, `HPX_EVM_RPC`, `HPX_GAS_PRICES`, `HPX_KEYRING_BACKEND`, `HPX_BUFFER_PAX`, `HPX_SSH_USER`, `HPX_SSH_OPTS`, `HPX_MIRROR`, `HPX_STAKE_ROUNDS`, `HPX_STAKE_SETTLE`, `SERVERS`, `SERVERS_FILE`, `CONFIRM`, `MNEMONIC_PHRASE` |
| Public mirror check | `HPX_ORIGIN` |

`HPX_MIRROR` selects the artifact and library mirror. Public node JSON-RPC uses
the root HTTPS paths described in the public RPC documentation linked below.
`HPX_HOME` selects the node home. The manager reads `config/config.toml`,
`config/app.toml` and `.hpx-node.json` there, downloads `config/genesis.json`,
and writes `config/client.toml` and `.hpx-operator.json`. Node keys and validator
signing state remain in the node home.

Set the CLI's `HPX_TOKEN` when the registry requires `HPX_REGISTER_TOKEN`.
The optional hosted HPX API uses `HPX_UNIFIED_GATEWAY_URL` and the protected
file selected by `HPX_GATEWAY_TOKEN_FILE`.

The registry listens on the loopback address selected by `HPX_ADDR` behind
Nginx. It reads artifacts under `HPX_ARTIFACTS_DIR` and persists peer state as
`registry.json` under `HPX_DATA_DIR`. Configure seed peers and CometBFT RPC
sources for state-sync trust queries with `HPX_SEED_PEERS`, `HPX_STATESYNC_RPC`
and `HPX_STATESYNC_RPC_SERVERS`.

[hosting/deploy.sh](hosting/deploy.sh) writes the environment file selected by
`HPX_REGISTRY_ENV`, which the generated systemd service loads. It reads
[hosting/fonts.sha256](hosting/fonts.sha256), the landing page and logo, and
installs [hosting/nginx-http.conf](hosting/nginx-http.conf) followed by
[hosting/nginx.conf](hosting/nginx.conf) for HTTPS serving.

The Fly deployment reads [hosting/fly.toml](hosting/fly.toml). The container
renders [hosting/nginx-registry.conf.template](hosting/nginx-registry.conf.template)
using `PORT` and `HPX_ADDR`; supply `PORT` and `LAYERX_TRUSTED_PROXIES` in the
deployment environment. Nginx derives the client address from `X-Forwarded-For`
using the trusted proxies, overwrites forwarding headers sent to the registry,
and rate-limits registration.

### Fleet collection and funding recovery

The [fleet key collector](fleet-keygen.sh) installs the local HPX CLI on selected
hosts before collecting their public operator identities. `HPX_PUSH` controls
whether it installs the CLI before collection. The [staking tool](stake-fleet.sh) funds
operator wallets and prints or runs validator staking commands. Set `HPX_RPC`
and `HPX_EVM_RPC` to the intended network before using the staking tool.

Run `tools/hpx/fleet-keygen.sh` or `tools/hpx/stake-fleet.sh keygen` from the
repository root. Both use the same durable state in the work directory selected
by `HPX_WORKDIR`. The first collection persists the intended host roster before
contacting hosts. Later collections merge hosts, retain established
identities, and mark unreachable or changed identities unresolved. An existing
`operators.tsv` is imported without granting verification. Collection exits
nonzero until every intended host verifies; `fund`, `stake` and `stake-cmds`
check that state before proceeding. Preserve `state.json` with the roster.

`stake-fleet.sh` loads the repository-root `.env`, or the explicit `HPX_ENV_FILE`,
before deriving settings. Explicit shell values override file values, which
override defaults. A missing explicit file is an error; an absent default file
is allowed. Only the initial founder import needs `MNEMONIC_PHRASE`.
Select the funding keyring with `HPX_KEYRING_BACKEND` and `HPX_KEYRING_DIR`;
prepare/import keys in that keyring before a dry-run. The founder preview reads
public identity and balances only; association uses `paxd tx evm associate-address --from` with
that keyring and never exports a private key.

The `fund` phase durably freezes a per-recipient allocation and chain/sender
identity before broadcasting. Retries send only deficits against those original
targets, even when the funding wallet's remaining balance has fallen. The journal
persists exact signed transactions and their hashes before submission, then
requires committed code zero and the expected recipient balance before recording
a receipt. An interrupted payment is reconciled or rebroadcast with the same
signed bytes before any new signing. Pending, rejected, failed or underfunded
payments return nonzero. Keep the private work directory across restarts; never
delete its journal to retry a payment or reuse it for a different chain.

## Documentation

- [Paxeer network](../../docs/paxeer-network.md) — merged network and node distribution context.
- [Chain documentation index](../../docs/paxeer-chain-docs.md) — node tooling and related references.
- [Admin and HPX guide](../../docs/site/paxeer-docs/public/markdown/admin-hpx.md) — node installation, CLI commands, registry and publishing.
- [Admin and HPX site page](../../docs/site/paxeer-docs/src/app/admin-hpx/page.tsx) — documentation site page for this component.
- [Run a node](../../docs/site/paxeer-docs/public/markdown/run-node.md) — genesis, peers, synchronization and node roles.
- [Operators guide](../../docs/site/paxeer-docs/public/markdown/operators.md) — validator operations.
- [Public RPC reference](../../docs/site/paxeer-docs/public/markdown/public-rpc.md) — node JSON-RPC connection details.
