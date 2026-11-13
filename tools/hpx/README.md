# Node distribution and peer registry (`hpx`)

HPX is the public installer, node manager, artifact publisher and peer registry
for Paxeer mainnet-beta (consensus chain ID `hyperpax_125-1`, EVM chain ID
`125`, or `0x7d`). Nodes run a native `paxd` binary under systemd. The kernel is
part of this BFT network and proposes blocks; a receipt is final when its block
is committed. Generated executables, native libraries, live chain configuration,
registry data and TLS material are published outside Git.

`HPX_MIRROR` selects the artifact and library mirror, which distributes node
software and peer information. Public JSON-RPC uses the root HTTPS paths listed
in the [public RPC reference](../../docs/site/docs/reference/public-rpc.md).

## Install a node

Use a Linux host with systemd and an x86-64 or AArch64 CPU. Installation and
node lifecycle changes require root privileges.

```bash
: "${HPX_MIRROR:?set the trusted artifact mirror URL}"
curl -fsSL "$HPX_MIRROR/get-hpx.sh" | sudo env HPX_MIRROR="$HPX_MIRROR" bash
```

The [web installer](get-hpx.sh) verifies the HPX CLI against `checksums.txt`.
The [setup flow](hpx) verifies downloads against that manifest in stages:
`chain-info.json`, then `paxd` and the three libwasmvm runtimes for the host
architecture, then genesis and the selected fullnode or validator configuration.
The executable and libraries are installed before the chain files. These checks establish
integrity against the selected mirror's manifest; use `HPX_MIRROR` only for an
explicitly trusted alternate mirror.

Run `hpx` with no arguments for an interactive menu, or use the subcommands:

```bash
HPX_TYPE=fullnode hpx setup
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

The default node home is `/root/.paxeer` (`HPX_HOME` overrides it), and the
systemd service is `paxd.service`. If the registry requires registration
authentication, set the CLI's `HPX_TOKEN` to the registry's `HPX_REGISTER_TOKEN`.

`hpx statesync` stops the service and applies trust parameters from the mirror.
It invokes `paxd tendermint unsafe-reset-all --home` with the node home, trying
`paxd comet unsafe-reset-all --home` if needed, then starts the service.

## Publish artifacts

The mirror deployment is described by [tools/hpx/hosting/fly.toml](hosting/fly.toml)
for the Fly app `paxeer-hpx-registry`. Its volume at `/srv/hpx` holds release
artifacts under `/srv/hpx/artifacts` and registry data under `/srv/hpx/data`.
All repository commands below run from the monorepo root.

The [publisher](publish.sh) takes these inputs:

| Input | Default or requirement |
|---|---|
| `SRC_BIN` | `build/paxd` |
| `WASM_ROOT` | monorepo root; libraries under `wasm-runtime/internal/api` and `wasm/x/wasm/artifacts/{v152,v155}/api` |
| `SRC_CFG` | `HPX_RUNTIME_CONFIG_DIR`, otherwise `/root/.paxeer/config` |
| `HPX_ARTIFACTS_ROOT` | `/srv/hpx/artifacts` |
| `HPX_PAXD_VERSION` | Defaults to `tools/release/version.json`; an override must equal that version |
| `HPX_BUILD_PROVENANCE` | Required build provenance JSON; defaults to `${SRC_BIN}.provenance.json` |
| `HPX_RELEASE_ID` | UTC timestamp followed by the first 12 hex characters of the supplied binary's SHA-256 |
| `HPX_SEED_PEERS` | Required: at least three distinct `40-hex-node-id@host:port` entries, comma-separated |
| `HPX_CHAIN_ID` / `HPX_EVM_CHAIN_ID` | `hyperpax_125-1` / `125` |
| `HPX_P2P_PORT` / `HPX_RPC_PORT` | `26656` / `26657` (CometBFT ports) |
| `HPX_RELEASE_REPO` | `Sidiora-Labs/Paxeer-X-Network`; used in the generated binary-download URL |

### Release identity

The publisher reads `tools/release/version.json` and checks the staged executable's
`version --long --output json` identity before writing checksums or switching
`current`. Its `version` must equal the manifest and any `HPX_PAXD_VERSION`
override, and its `commit` must equal the checkout's full `git rev-parse HEAD`.
The `server_name` must be `paxd`.

Supply a build provenance JSON file through `HPX_BUILD_PROVENANCE`
(default `${SRC_BIN}.provenance.json`).
The `version` value is read from `tools/release/version.json`.
It must contain these three fields:

```json
{
  "version": "v7.0.0",
  "source_revision": "<full 40-character build revision>",
  "paxd_sha256": "<64-character SHA-256 of the build artifact>"
}
```

The provenance version and source revision must match the manifest, executable
and checkout, and its digest must match the exact staged executable.
Missing or inconsistent identity data aborts publication. The release includes
`binary-info.json`, `build-provenance.json` and `version.json` in its checksums.
`chain-info.json` records the validated version, commit, source revision and
artifact digest; `release_id` independently names the artifact directory.

After supplying seed peers from the private deployment configuration:

```bash
sudo env \
  HPX_BUILD_PROVENANCE="${HPX_BUILD_PROVENANCE:?set the build provenance JSON}" \
  HPX_SEED_PEERS="${HPX_SEED_PEERS:?set at least three distinct seed peers}" \
  tools/hpx/publish.sh
```

Pass any input overrides explicitly through `sudo env` as well. The publisher
requires the binary, all six x86-64 and AArch64 native libraries, `genesis.json`,
`config.toml`, `app.toml` and the lifecycle scripts. It checks the genesis chain
ID, generates fullnode and validator configuration variants, and clears their
published peer and external-address settings. It stages a release, writes a
sorted SHA-256 manifest, makes the release read-only, then atomically moves the
local `current` symlink to `releases/<release-id>`. Existing release IDs are
rejected; a staging failure leaves `current` unchanged.

Stream the assembled release into the app's volume, verify it inside the
machine, then atomically move the served `current` symlink:

```bash
set -euo pipefail
root="${HPX_ARTIFACTS_ROOT:-/srv/hpx/artifacts}"
rel=$(readlink "$root/current")
tar -C "$root" -cf - "$rel" | \
  flyctl ssh console --app paxeer-hpx-registry --command "tar -C /srv/hpx/artifacts -xf -"
flyctl ssh console --app paxeer-hpx-registry --command \
  "sh -c 'cd /srv/hpx/artifacts/$rel && sha256sum -c checksums.txt && ln -s $rel /srv/hpx/artifacts/.current.new && mv -Tf /srv/hpx/artifacts/.current.new /srv/hpx/artifacts/current'"
```

A release whose checksums fail verification does not move the served `current`.

## Publish the registry runtime

Pushes to `main` changing `tools/hpx/registry` or
[.github/workflows/paxeer-hpx-registry.yml](../../.github/workflows/paxeer-hpx-registry.yml)
trigger `Paxeer / HPX Registry`; it also supports manual dispatch. The workflow
publishes Linux amd64 and arm64 executables and checksum files under GitHub
release tag `hpx-registry-<source-revision>`. It also publishes a multi-architecture
GHCR image containing the Go registry service. Generated executables are never
committed.

The Fly deployment uses the separate
[docker/hpx-registry/Dockerfile](../../docker/hpx-registry/Dockerfile) image,
which includes nginx, the landing page and the registry. Build this image from
the repository root and deploy it to the app's single machine:

```bash
: "${HPX_REGISTRY_IMAGE:?set the container registry image tag to publish}"
docker build --build-arg SOURCE_REVISION="$(git rev-parse HEAD)" \
  -f docker/hpx-registry/Dockerfile -t "$HPX_REGISTRY_IMAGE" .
docker push "$HPX_REGISTRY_IMAGE"
flyctl deploy -c tools/hpx/hosting/fly.toml --app paxeer-hpx-registry \
  --image "$HPX_REGISTRY_IMAGE" --ha=false -y
```

The container requires `PORT` (use `8080` to match the Fly internal port),
`HPX_ADDR` (the configured loopback listener `127.0.0.1:8099`) and
`LAYERX_TRUSTED_PROXIES` (the trusted proxy addresses or CIDRs). Supply `PORT`
and the proxy list in the deployment environment; the checked-in Fly
configuration does not set them. The [entrypoint](../../docker/hpx-registry/entrypoint.sh)
starts nginx, runs the registry as user `hpx`, and exits if either stops.

Nginx uses [hosting/nginx-registry.conf.template](hosting/nginx-registry.conf.template).
It derives the client address from `X-Forwarded-For` using
`LAYERX_TRUSTED_PROXIES`, overwrites the headers forwarded to the loopback
registry, and rate-limits registration. The registry persists state at
`/srv/hpx/data/registry.json`; Fly checks `/healthz`.

Set `HPX_SEED_PEERS`, `HPX_STATESYNC_RPC` and `HPX_STATESYNC_RPC_SERVERS` for
peer discovery and state-sync trust queries against the intended CometBFT
nodes. If the app's `HPX_REGISTER_TOKEN` secret is set, registration requires
`X-HPX-Token`. The edge mapping is managed by
[tools/bringup/edge.sh](../bringup/edge.sh);
`tools/bringup/check-live.sh hpx` checks the mirror and its Fly app, including
checksums and whether the public name is served by the app.

## Public surface

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

## Fleet collection and funding recovery

The [fleet key collector](fleet-keygen.sh) installs the local HPX CLI on selected
hosts before collecting their public operator identities; set `HPX_PUSH=0` when
the intended CLI is already installed. The [staking tool](stake-fleet.sh) funds
operator wallets and prints or runs validator staking commands. Set `HPX_RPC`
and `HPX_EVM_RPC` to the intended network before using the staking tool.

Run `tools/hpx/fleet-keygen.sh` or `tools/hpx/stake-fleet.sh keygen` from the
repository root. Both use the same durable state under `tools/hpx/.fleet`
(`HPX_WORKDIR` overrides it). The first collection persists the intended host
roster before contacting hosts. Later collections merge hosts, retain established
identities, and mark unreachable or changed identities unresolved. An existing
`operators.tsv` is imported without granting verification. Collection exits
nonzero until every intended host verifies; `fund`, `stake` and `stake-cmds`
check that state before proceeding. Preserve `state.json` with the roster.

`stake-fleet.sh` loads the repository-root `.env`, or the explicit `HPX_ENV_FILE`,
before deriving settings. Explicit shell values override file values, which
override defaults. A missing explicit file is an error; an absent default file
is allowed. Only the initial founder import needs `MNEMONIC_PHRASE`.
`HPX_KEYRING_BACKEND` defaults to the protected `file` backend. Prepare/import
keys in that keyring before a dry-run. The founder preview reads public identity
and balances only; association uses `paxd tx evm associate-address --from` with
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
