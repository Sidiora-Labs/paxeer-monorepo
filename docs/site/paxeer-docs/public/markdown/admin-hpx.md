<!-- Source: https://docs.paxeer.app/admin-hpx/ -->

# Admin & HPX

Runtime administration, the HPX node distribution, and operational controls for Paxeer X.

**Admin:** `admin/`
 **HPX:** `tools/hpx/`

## Operating the unified network

The HPX distribution runs the Paxeer X chain node, including EVM, Cosmos, custody, anchor, exchange, bridge, launchpad, and XWeb modules. HyperPax identifiers remain in binary configuration and historical chain identity. Paxeer X is the unified product name.

Runtime logging administration is separate from economic and security administration. Chain governance controls packaged LayerX parameter and admin messages through `modules/layerxgov/`. Custody authority, checkpoint thresholds, guarantor bonds, bridge attestors and caps, launchpad fee configuration, and XWeb attestors each belong to their module’s authorization model.

Monitor consensus health together with checkpoint finality, availability attestations, custody claims and exits, exchange intents, bridge pause and cap state, and XWeb request delivery. The [parameter reference](https://docs.paxeer.app/network-parameters), [settlement lifecycle](https://docs.paxeer.app/settlement), and [kernel architecture](https://docs.paxeer.app/layerx-kernel) explain those responsibilities. Application operators integrate through the [platform API](https://docs.paxeer.app/platform-api).

## Admin gRPC Service

The Admin service provides runtime log level control for `paxd` without restarting the node. It runs on a dedicated loopback-only gRPC server for security.

### Configuration

Enable in `app.toml`:

```
[admin_server]
admin_enabled = true
admin_address = "127.0.0.1:9095"
```

The address **must** be a loopback address (`127.0.0.1` or `::1`). Binding to external interfaces is rejected at startup.

### gRPC Methods

| Method | Request | Response | Purpose |
| --- | --- | --- | --- |
| `SetLogLevel` | `pattern`, `level` | `affected` count | Change log level for loggers matching pattern |
| `GetLogLevel` | `logger` | `level` | Get current log level for a logger |
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

### Example: Using grpcurl

```
# List all loggers
grpcurl -plaintext localhost:9095 \
  paxprotocol.paxchain.admin.v0.AdminService/ListLoggers

# Set EVM logger to debug
grpcurl -plaintext -d '{"pattern":"evm","level":"debug"}' \
  localhost:9095 \
  paxprotocol.paxchain.admin.v0.AdminService/SetLogLevel

# Get log level for a specific logger
grpcurl -plaintext -d '{"logger":"evm"}' \
  localhost:9095 \
  paxprotocol.paxchain.admin.v0.AdminService/GetLogLevel
```

### Security

The Admin service is loopback-only by design:

- No authentication required (loopback is trusted)

- Cannot be bound to external interfaces

- Must access from the same machine as `paxd`

For remote access, use SSH port forwarding:

```
ssh -L 9095:127.0.0.1:9095 user@node-ip
```

**Proto:** `api/pax/admin/v0/admin.proto`
 **Implementation:** `admin/server.go`, `service.go`, `config.go`

## HPX: HyperPax Node Distribution

HPX is the public installer, node manager, and peer registry for HyperPax (`hyperpax_125-1`). Nodes run a native `paxd` binary under systemd. All artifacts are published outside Git.

### Chain Identifier

`hyperpax_125-1` (Cosmos-style, EVM chain ID `125`)

### Install a Node

```
curl -sSL https://node.hyperpaxeer.com/get-hpx.sh | sudo bash
```

The installer:

1. Downloads and verifies the HPX CLI against `checksums.txt`
2. Places `hpx` in `/usr/local/bin/`

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
3. Installs artifacts to `/root/.paxeer/`
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
| `hpx register` | Announce node to public registry |
| `hpx statesync` | Enable state-sync for fast bootstrap |
| `hpx remove` | Uninstall node and clean up |

### Node Types

| Type | Purpose | Config |
| --- | --- | --- |
| `fullnode` | Non-validating full node (RPC, indexing) | `/config/fullnode/` |
| `validator` | Validating node (must register validator key) | `/config/validator/` |

### Public Registry

The HPX registry at `https://node.hyperpaxeer.com` provides:

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Registry liveness, chain and source revision |
| `GET /checksums.txt` | SHA-256 checksums for all artifacts |
| `GET /chain-info.json` | Chain metadata (chain ID, genesis hash) |
| `GET /paxd` | Native `paxd` binary |
| `GET /lib/*.so` | Architecture-specific `libwasmvm` runtimes |
| `GET /genesis.json` | Chain genesis file |
| `GET /config/<type>/<file>` | Node configuration files (`config.toml`, `app.toml`) |
| `GET /api/myip` | Caller's public IP address |
| `POST /api/register` | Announce node's public peer address |
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

1. Collects `paxd` binary from `build/paxd`
2. Collects native libraries from `wasm-runtime/` and `wasm/x/wasm/artifacts/`
3. Collects live chain configuration from `/root/.paxeer/config/` (or `$SRC_CFG`)
4. Stages artifacts in `/srv/hpx/artifacts/releases/<release-id>/`
5. Generates `checksums.txt`
6. Atomically moves `current` symlink to new release

Failed staging runs never change the served release.

### Registry Runtime Deployment

Changes to the registry service under `tools/hpx/registry/` trigger the GitHub workflow `Paxeer / HPX Registry`, which:

1. Builds Linux executables (x86-64, AArch64)
2. Publishes them as GitHub release assets
3. Publishes a multi-architecture GHCR image

Deploy the registry on the public origin host:

```
sudo tools/hpx/hosting/deploy.sh
```

This script:

1. Downloads and verifies the latest registry executable
2. Installs it as a systemd service (loopback-only)
3. Obtains a Let's Encrypt certificate for `node.hyperpaxeer.com`
4. Configures Nginx reverse proxy with rate-limiting

Registry state is persisted at `/srv/hpx/data/registry.json`.

### Registry Authentication

Registration is public by default. To require a token:

```
export HPX_REGISTER_TOKEN=your-secret-token
sudo tools/hpx/hosting/deploy.sh
```

The registry will then require `X-HPX-Token: your-secret-token` header on `POST /api/register`.

### Using an Alternate Mirror

Set `HPX_MIRROR` only when operating an explicitly trusted alternate mirror:

```
export HPX_MIRROR=https://your-mirror.example.com
hpx setup
```

**Warning:** The mirror must serve the same artifact structure and checksums. Untrusted mirrors can serve malicious binaries.

### Native Libraries

HPX distributes architecture-specific `libwasmvm` runtimes:

- `libwasmvm.x86_64.so` — x86-64 Linux

- `libwasmvm.aarch64.so` — AArch64 Linux

- `libwasmvm_muslc.x86_64.so` — x86-64 musl (Alpine)

- `libwasmvm_muslc.aarch64.so` — AArch64 musl

The installer detects the host architecture and installs the correct library.

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

This stops `paxd`, removes the systemd service, and deletes `/root/.paxeer/`.

**Source:** `tools/hpx/README.md`, `tools/hpx/hosting/`
