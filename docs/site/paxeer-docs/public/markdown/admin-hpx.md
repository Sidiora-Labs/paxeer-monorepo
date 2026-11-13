<!-- Source: https://docs.paxeer.app/admin-hpx/ -->

# Admin & HPX

Runtime administration, the HPX node distribution, and operational controls for Paxeer X.

**Admin:** `admin/`
 **HPX:** `tools/hpx/`

## Operating the unified network

The HPX distribution installs and manages the Paxeer mainnet-beta BFT node, paxd. The node includes the EVM and native kernel on one network. EVM chain ID 125 identifies this network, and a receipt is final when its block is committed.

Runtime logging administration is separate from economic and security administration. Chain governance controls native parameter and admin messages through `modules/layerxgov/`. Module configuration follows the authorization model implemented by that module.

Monitor consensus health, block progress, transaction receipts, node connectivity, and the state of the modules used by your applications. The [parameter reference](https://docs.paxeer.app/network-parameters), [settlement lifecycle](https://docs.paxeer.app/settlement), and [kernel architecture](https://docs.paxeer.app/layerx-kernel) explain those responsibilities. Application operators integrate through the [platform API](https://docs.paxeer.app/platform-api).

## Admin gRPC Service

The Admin service provides runtime log level control for `paxd` without restarting the node. It runs on a dedicated loopback-only gRPC server for security.

### Configuration

Enable in `app.toml`:

```
[admin_server]
admin_enabled = true
# admin_address defaults to a loopback listener on port 9095.
```

The configured **admin_address** must contain a numeric loopback IP and port, such as an `IPv4` or `IPv6` loopback listener. The default port is 9095. The enabled server validates this at startup.

### gRPC Methods

| Method | Request | Response | Purpose |
| --- | --- | --- | --- |
| `SetLogLevel` | `pattern`, `level` | `pattern`, level and affected count | Change log level for loggers matching pattern |
| `GetLogLevel` | `logger` | `logger` and level | Get current log level for a logger |
| `ListLoggers` | `prefix` (optional) | List of loggers | List all loggers and their levels |

### SetLogLevel Pattern Matching

The `pattern` parameter supports:

- **Exact match:** `"evm"` (sets log level for the EVM logger only)

- **Glob:** `"evm*"` (sets log level for all loggers starting with "evm")

- **All loggers:** `"*"` (sets log level for every logger)

### Log Levels

| Level | Purpose |
| --- | --- |
| `debug` | Verbose debugging information |
| `info` | General operational information |
| `warn` | Warning messages (potential issues) |
| `error` | Error messages (failures) |

### Example: Using grpcurl with the protocol definition

```
# Run from the repository root; set ADMIN_ENDPOINT to the loopback listener.
# List all loggers
grpcurl -plaintext -import-path api -import-path sdk/third_party/proto \
  -proto pax/admin/v0/admin.proto "$ADMIN_ENDPOINT" \
  paxprotocol.paxchain.admin.v0.AdminService/ListLoggers

# Set EVM logger to debug
grpcurl -plaintext -import-path api -import-path sdk/third_party/proto \
  -proto pax/admin/v0/admin.proto -d '{"pattern":"evm","level":"debug"}' \
  "$ADMIN_ENDPOINT" \
  paxprotocol.paxchain.admin.v0.AdminService/SetLogLevel

# Get log level for a specific logger
grpcurl -plaintext -import-path api -import-path sdk/third_party/proto \
  -proto pax/admin/v0/admin.proto -d '{"logger":"evm"}' \
  "$ADMIN_ENDPOINT" \
  paxprotocol.paxchain.admin.v0.AdminService/GetLogLevel
```

### Security

The Admin service is loopback-only by design:

- The server applies no application authentication; access is bounded by loopback.

- Cannot be bound to external interfaces

- Must access from the same machine as `paxd`

To use the same client through SSH, set the example shell variables to your forwarding specification and SSH login:

```
ssh -N -L "$ADMIN_FORWARD" "$NODE_LOGIN"
```

**Proto:** `api/pax/admin/v0/admin.proto`
 **Implementation:** `admin/server.go`, `admin/service.go`, `admin/config.go`

## HPX: Paxeer Node Distribution

HPX provides the installer, node manager, and peer registry for Paxeer mainnet-beta (`hyperpax_125-1`). Nodes run a native `paxd` binary under systemd. Release artifacts are served by the HPX mirror.

### Chain Identifier

`hyperpax_125-1` (Cosmos-style, EVM chain ID `125`)

### Install a Node

```
# From the repository root on a supported Linux node
sudo bash tools/hpx/get-hpx.sh
```

The installer:

1. Downloads and verifies the HPX CLI against `checksums.txt`
2. Places `hpx` in `/usr/local/bin/` and offers to run setup

### Setup a Node

```
# Set node type (fullnode or validator)
export HPX_TYPE=fullnode

# Run setup
hpx setup
```

The setup flow:

1. Downloads `paxd`, `libwasmvm` runtimes, genesis, and configuration
2. Verifies all artifacts against `checksums.txt`
3. Installs artifacts under `/root/.paxeer/`, overridable with HPX_HOME
4. Configures systemd service
5. Starts `paxd`

### HPX CLI Commands

| Command | Purpose |
| --- | --- |
| `hpx status` | Show node sync status |
| `hpx info` | Show node configuration |
| `hpx logs` | Tail `paxd` logs |
| `hpx` | Interactive menu |
| `hpx start` / `hpx stop` / `hpx restart` | Control the `paxd` systemd service |
| `hpx validator keygen` | Generate this node's operator account |
| `hpx validator stake` | Submit create-validator from the funded operator account |
| `hpx validator status` | Show validator state |
| `hpx update` | Update `paxd` and libraries |
| `hpx peers show` | Show known peers |
| `hpx peers refresh` | Fetch latest peer list from registry |
| `hpx register` | Request registration with the peer registry |
| `hpx statesync` | Enable state-sync for fast bootstrap |
| `hpx remove` | Uninstall the node service and optionally remove its data |

### Node Types

| Type | Purpose | Config |
| --- | --- | --- |
| `fullnode` | Non-validating full node (RPC, indexing) | `/config/fullnode/` |
| `validator` | Validating node (must register validator key) | `/config/validator/` |

### Public Registry

The HPX registry in `tools/hpx/registry/` serves mirror artifacts and peer discovery through these routes:

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Registry liveness, chain identity and software identity |
| `GET /checksums.txt` | SHA-256 checksums for all artifacts |
| `GET /chain-info.json` | Chain IDs, binary version and digest, seed peers and release metadata |
| `GET /paxd` | Native `paxd` binary |
| `GET /lib/libwasmvm.<arch>.so` | Declared `libwasmvm` runtime artifacts; versioned names are listed below |
| `GET /genesis.json` | Chain genesis file |
| `GET /config/<type>/<file>` | Node configuration files (`config.toml`, `app.toml`) |
| `GET /api/myip` | Caller's public IP address |
| `POST /api/register` | Register a peer with a fresh challenge and signed node-ownership proof |
| `GET /api/peers` | JSON list of registered peers |
| `GET /api/peers.txt` | Text list of peer addresses (Tendermint format) |
| `GET /api/nodes` | Detailed node metadata |
| `GET /api/statesync` | Current state-sync trust parameters |

### Artifact Publishing

To publish a new `paxd` release or chain configuration:

```
# From the monorepo root
sudo tools/hpx/publish.sh
```

This script:

1. Collects `paxd` from `build/paxd` together with build provenance and the version manifest
2. Collects native libraries from `wasm-runtime/` and `wasm/x/wasm/artifacts/`
3. Collects genesis and chain configuration from `/root/.paxeer/config/` (or `$SRC_CFG`); HPX_SEED_PEERS supplies at least three distinct seeds
4. Stages artifacts in `/srv/hpx/artifacts/releases/<release-id>/`
5. Generates `checksums.txt`
6. Atomically moves `current` symlink to new release

The publisher validates the binary identity and all required inputs before updating the served release.

### Registry Runtime

The registry service under `tools/hpx/registry/` is distributed in these forms:

1. Linux executables for x86-64 and AArch64
2. GitHub release assets with SHA-256 checksum files
3. A GHCR image for Linux amd64 and arm64

After publishing artifacts and setting HPX_SEED_PEERS, HPX_STATESYNC_RPC and HPX_STATESYNC_RPC_SERVERS, deploy the registry:

```
sudo tools/hpx/hosting/deploy.sh
```

This script:

1. Downloads and verifies the registry executable selected by HPX_SOURCE_REVISION or the registry source identity in Git
2. Installs it as a systemd service (loopback-only)
3. Obtains a TLS certificate for the origin configured in `tools/hpx/hosting/deploy.sh`
4. Configures Nginx reverse proxy with rate-limiting

Registry state is persisted at `/srv/hpx/data/registry.json`.

### Registry Authentication

Registration requires a fresh challenge and an Ed25519 node-ownership proof. An optional registration token adds another requirement:

```
# Set HPX_REGISTER_TOKEN in the protected deployment environment.
# Set HPX_TOKEN for the node manager when the registry requires it.
sudo tools/hpx/hosting/deploy.sh
```

When configured, the registry also requires an `X-HPX-Token` header on `POST /api/register`. Obtain a challenge with GET /api/register/challenge before signing the node registration.

### Using an Alternate Mirror

Set `HPX_MIRROR` only when operating an explicitly trusted alternate mirror:

```
export HPX_MIRROR="<trusted HTTPS mirror origin>"
hpx setup
```

**Warning:** The mirror must serve the same artifact structure and checksums. Untrusted mirrors can serve malicious binaries.

### Native Libraries

HPX distributes architecture-specific `libwasmvm` runtimes:

- `libwasmvm.x86_64.so` — x86-64 Linux

- `libwasmvm.aarch64.so` — AArch64 Linux

- `libwasmvm152.<arch>.so` — version 1.5.2 for x86_64 and aarch64

- `libwasmvm155.<arch>.so` — version 1.5.5 for x86_64 and aarch64

The node manager detects the host architecture and installs all three runtime variants for that architecture.

### State Sync

New nodes can bootstrap from a recent state snapshot instead of replaying full history:

```
hpx statesync
```

This fetches trust parameters from `/api/statesync` and updates `config.toml` to enable state-sync.

### Uninstall

```
hpx remove
```

This stops `paxd` and removes the node service and peer-refresh units. A separate confirmation controls deletion of `/root/.paxeer/`, or the configured HPX_HOME.

**Source:** `tools/hpx/README.md`, `tools/hpx/hosting/`
