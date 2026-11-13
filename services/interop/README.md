# Paxeer X Network interoperability workspace

This Rust workspace holds adapters, receipt verifiers, mirror publishers, bridge
relayers and payment services at the edge of Paxeer X Network. The adapters
translate external protocols into evidence consumed by the kernel inside
`paxd`; native ledger and payment operations execute on the merged chain
(EVM chain ID `125`), where BFT block commit establishes finality. Interop
services publish mirror archives, verify external attestations and consume
signed network receipts.

## Layout

| Path | Contents |
| --- | --- |
| [`Cargo.toml`](Cargo.toml) | Workspace members, shared dependencies, Rust version and lint settings. |
| [`Cargo.lock`](Cargo.lock) | Locked Rust dependency versions. |
| [`clippy.toml`](clippy.toml) | Clippy minimum supported Rust version and API lint settings. |
| [`deny.toml`](deny.toml) | Dependency advisory, license, source and package policies. |
| [`.gitignore`](.gitignore) | Cargo build output exclusions. |
| [`contracts/`](contracts/) | Ethereum and Solana mirror archive contracts and their tests. |
| [`crates/`](crates/) | Protocol adapters, gateway, executable services, receipt proofs and mirror libraries listed in the workspace manifest. |
| [`deploy/`](deploy/) | Gateway, bridge, mirror, gas station and search deployment templates, renderers and signer protocol. |
| [`examples/`](examples/) | Portable receipt verification example in `portable_v2.rs`. |
| [`specs/`](specs/) | Vendored protocol specifications and conformance vectors for adapters and transports. |
| [`tests/`](tests/) | Portable receipt example and guide coverage against receipt vectors. |

### Components

| Surface | What it is | Where it lives |
| --- | --- | --- |
| Gateway | Transport-neutral routes, redaction, adapter host | `crates/layerx-interop-gateway`, executable composition in `crates/layerx-interop-service` |
| MCP / A2A transports | Ingress labels `mcp` and `a2a` next to `http` | Gateway `IngressTransport`; x402 `TransportKind` |
| x402 v2 | Buyer, seller, facilitator over HTTP, MCP, and A2A | `crates/layerx-x402` - [COMPATIBILITY.md](crates/layerx-x402/COMPATIBILITY.md). Payment profiles and settlement evidence: [`docs/wiki/X402Transport.md`](../../docs/wiki/X402Transport.md) |
| Ethereum / Solana mirrors | Batch archive publication and verification. Archives of batch commitments and verification material | `crates/layerx-mirror`; contracts in `contracts/ethereum-mirror/` and `contracts/solana-mirror/`; deploy notes in `deploy/mirror/` |
| Ethereum / Solana migration | Source-chain verifiers and the `migration-v2` adapter | `crates/layerx-migrate` - [OPERATIONS.md](crates/layerx-migrate/OPERATIONS.md) |
| Committed proofs | Receipt authentication, batch inclusion and validator commit verification | `crates/layerx-committed-proof` |
| Portable receipts | Verify signed `ProtocolReceipt` v2 bytes under independently pinned, height-ranged receipt keys without a live node | `crates/layerx-portable` - [PORTABILITY.md](../../docs/interop/PORTABILITY.md) |
| Bridge relayer | Carries `PaxeerXVault` deposits into the bridge precompile and Paxeer X burns into vault releases, plus the `layerx-bridge-cosign` share transport | `crates/layerx-bridge-relayer`; deploy config in `deploy/bridge-relayer/`; operator runbook in [`contracts/bridge/README.md`](../../contracts/bridge/README.md) |
| Remote signer | Reference signer for the mirror publisher and the bridge relayer | `crates/layerx-mirror-signer`; protocol in [`deploy/mirror/signer-protocol.md`](deploy/mirror/signer-protocol.md) |
| Gas station | Sponsored submission (`paxeer-gas-station` binary) | `crates/layerx-gas-station`; deploy config in `deploy/gas-station/` |
| Web search sidecar | `x-websearch`: paid `/search` and `/fetch`, free `/health` | `crates/x-websearch`; deploy files in `deploy/x-websearch/` and `deploy/search-front/` |
| Other adapters | AP2, UCP, Visa TAP, fiat rails | `crates/layerx-ap2`, `crates/layerx-ucp`, `crates/layerx-visa-tap`, `crates/layerx-fiat` |

A2A has a gateway ingress label and an x402 JSON binding. The developer CLI in
[`tools/platform/cli/`](../../tools/platform/cli/) also provides an A2A installer and runtime.

### MCP integrations

| Surface | Role |
| --- | --- |
| `services/agent/crates/layerx-mcp` | Tenant- and scope-bound MCP server backed by `layerx-agentd` |
| [`tools/platform/cli/`](../../tools/platform/cli/) | `layerx install mcp` installs a daemon binding; `layerx mcp serve --daemon-binding <binding-file>` exposes that daemon's catalogue over stdio |
| Interop gateway | HTTP routes under `/v1/mcp/x402/` select the x402 MCP JSON binding |

MCP tool inventory: [`services/agent/crates/layerx-mcp/README.md`](../../services/agent/crates/layerx-mcp/README.md). CLI install copy: [`docs/platform/content/install.md`](../../docs/platform/content/install.md) and [`docs/platform/content/guide/interop.md`](../../docs/platform/content/guide/interop.md).

### Mirrors are archives

`layerx-mirror-publisher` and `layerx-mirror-verify` publish and check batch
commitments on Ethereum and Solana. The mirror archive carries verification
material; its contracts hold no vault or custody and do not establish Paxeer
finality. Bridge vaults are separate contracts with their own attestor policy.
Settlement on Paxeer is the committed chain state, whose node lives in the
chain directories at the [Paxeer monorepo root](../../README.md).

Remote signer framing: [`deploy/mirror/signer-protocol.md`](deploy/mirror/signer-protocol.md).

### Attestation bindings and network receipts

The bridge relayer binds deposit attestations to the source chain ID, vault,
transaction hash and log index, recipient, asset and amount. Release bundles
bind the destination chain and vault, Paxeer transaction hash and nonce,
recipient, asset and amount. Signatures cover the corresponding digest and
must meet the destination's distinct-attestor threshold. The read-only
`GET /v1/deposit/{chainId}/{txHash}/{logIndex}` and
`GET /v1/release/{chainId}/{paxeerTxHash}/{nonce}` routes expose observed,
attesting and ready bundles; an unobserved item answers 404. See the
[interop contract](../../spec/layerx-merge/contracts/interop.md#bridge-relayer).

External evidence import preserves protocol presentation bytes and binds the
adapter's typed verification result to their digest and protocol metadata.
Consumers must authenticate the verifier or repeat verification; a digest alone
does not authenticate an attestation. See
[external evidence import](../../docs/interop/PORTABILITY.md#external-evidence-import).

Read `pax_sequencerReceipt` at the node root for the `submitted`, `receipted`
and `committed` rungs. Verify canonical signed `ProtocolReceipt` v2 bytes
against independently pinned receipt keys and their inclusive height windows,
and bind the transaction hash, result and logs to the intended operation.
`submitted` carries no signed execution receipt; `receipted` authenticates
execution but does not prove BFT commit. Finality requires authenticated
block inclusion and validator commit evidence at the `committed` rung.
Portable export uses `layerx-receipt-proof-v2` and retains verification level
`receipted`, even when exported from a committed answer; it cannot establish
block finality on its own. See the
[portable verification guide](../../docs/interop/PORTABILITY.md).

### Migration boundaries

| Kind | Location |
| --- | --- |
| ETH / Solana source migration into Paxeer | this workspace, `crates/layerx-migrate`; wiki page [`docs/wiki/Migration.md`](../../docs/wiki/Migration.md) |
| SQLite projection and genesis import bookkeeping schemas; launchpad PostgreSQL migrations | [`migrations/`](../../migrations/README.md) |
| Paxeer X chain EVM store migrations | [`modules/evm/migrations/`](../../modules/evm/migrations/) (chain-internal) |

## Build and test

The root [`Makefile`](../../Makefile) provides the workspace build, tests and
lint targets. Run these from the monorepo root:

```sh
make interop-build
make interop-test
make interop-lint
make interop-test-x402
make interop-test-migration
make interop-test-mandates
make interop-test-ucp
make interop-test-portable
make interop-test-visa-tap
make interop-test-mirrors
```

`interop-test` runs the workspace tests through
[`tests/ci/interop_workspace_test.py`](../../tests/ci/interop_workspace_test.py).
`interop-test-mirrors` also runs the Solana contract tests and the Ethereum
contract tests with Forge. `interop-lint` runs Clippy, the interop dependency
policy script and Cargo Deny.

Mirror publisher / verifier binaries, as built by the root mirror targets:

```sh
cargo build --locked --release --manifest-path services/interop/Cargo.toml --package layerx-mirror --bin layerx-mirror-publisher
cargo build --locked --release --manifest-path services/interop/Cargo.toml --package layerx-mirror --bin layerx-mirror-verify
```

`make mirror-live` and `make mirror-verify-live` build those binaries and run them against live networks through `scripts/qualify-mirror-live.sh` and `scripts/qualify-mirror-verification-live.sh`; they require operator configuration. Example configurations are [the publisher template](deploy/mirror/config.example.json) and [the verifier example](crates/layerx-mirror/examples/verifier.json).

## Configuration

### Interop gateway

The `layerx-interop-service` package builds the `layerx-interop-gateway` binary.
Its [configuration loader](crates/layerx-interop-service/src/config.rs) reads the
JSON manifest selected by `LAYERX_INTEROP_CONFIG`. The binary accepts no
command-line arguments. Configuration examples and deployment instructions are
in [`deploy/gateway/README.md`](deploy/gateway/README.md),
[`config.example.json`](deploy/gateway/config.example.json),
[`runtime.env.example`](deploy/gateway/runtime.env.example) and
[`railway.env.example`](deploy/gateway/railway.env.example).

| Purpose | Environment variable names |
| --- | --- |
| Listener | `LAYERX_INTEROP_LISTEN`, `LAYERX_INTEROP_LISTENER`, `LAYERX_INTEROP_TLS_CERT_DER`, `LAYERX_INTEROP_TLS_KEY_DER` |
| Node connection | `LAYERX_INTEROP_SEQUENCER_URL`, `LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK` |
| Network and protocol scope | `LAYERX_INTEROP_NETWORK_ID`, `LAYERX_INTEROP_CHAIN_ID`, `LAYERX_INTEROP_PROTOCOL_NETWORK_ID`, `LAYERX_INTEROP_WIRE_VERSION` |
| Readiness, retry retention and TAP clock | `LAYERX_INTEROP_READINESS_MAX_AGE_MS`, `LAYERX_INTEROP_IDEMPOTENCY_SECONDS`, `LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS` |
| Redis connection | `LAYERX_INTEROP_REDIS_URL`, `LAYERX_INTEROP_REDIS_USERNAME_FILE`, `LAYERX_INTEROP_REDIS_PASSWORD_FILE` |
| Outbound trust and client identity | `LAYERX_INTEROP_OUTBOUND_CA_DER`, `LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12`, `LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE` |
| Receipt and module trust files | `LAYERX_INTEROP_RECEIPT_KEYS_FILE`, `LAYERX_INTEROP_MODULE_REGISTRY_FILE`, `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE` |
| Migration configuration | `LAYERX_INTEROP_MIGRATION_V2_CONFIG` |

The gateway uses an authoritative module registry, server-owned TAP clock skew,
explicit trusted-agent status and a canonical TAP target for each principal.
The manifest includes `visa_agents` and `visa_targets`. The receipt-key file
pins signing keys and their height windows; examples of these file formats are
[`receipt-keys.example.json`](deploy/gateway/receipt-keys.example.json),
[`module-registry.example.json`](deploy/gateway/module-registry.example.json) and
[`visa-trust.example.json`](deploy/gateway/visa-trust.example.json).

### Mirrors and remote signer

`layerx-mirror-publisher` and `layerx-mirror-verify` read a JSON configuration
file passed as their positional argument. Templates are
[`deploy/mirror/config.example.json`](deploy/mirror/config.example.json),
[`deploy/mirror/verify-config.example.json`](deploy/mirror/verify-config.example.json)
and [`crates/layerx-mirror/examples/verifier.json`](crates/layerx-mirror/examples/verifier.json).

The remote signer reads `LAYERX_MIRROR_SIGNER_SOCKET`,
`LAYERX_MIRROR_SIGNER_ETHEREUM_KEY_FILE`,
`LAYERX_MIRROR_SIGNER_SOLANA_KEY_FILE`,
`LAYERX_MIRROR_SIGNER_ETHEREUM_KEY_HANDLE` and
`LAYERX_MIRROR_SIGNER_SOLANA_KEY_HANDLE`. Its bridge mode reads
`LAYERX_BRIDGE_SIGNER_SOCKET` and `LAYERX_BRIDGE_SIGNER_KEYS_FILE`.
Command-line options can select the socket, key files and handles. See the
[signer protocol](deploy/mirror/signer-protocol.md).

### Bridge relayer

`layerx-bridge-relayer` reads the TOML file selected by `--config`.
[`deploy/bridge-relayer/`](deploy/bridge-relayer/) contains relayer templates
and the signer key manifest `signer-keys.json`.
`layerx-bridge-cosign` reads its transport configuration through
`LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG`; operator inventory binding uses
`LAYERX_BRIDGE_OPERATOR_INVENTORY` and `LAYERX_BRIDGE_RELAYER_CONFIG`.
See the [bridge deployment page](deploy/bridge-relayer/README.md).

### Migration verifier

`layerx-migrate` reads JSON files selected by `--config`, `--store-config`,
`--binding-config`, `--mapping-config` and `--gateway-config` for the selected
operation. It expands `LAYERX_MIGRATION_SECRET_DIR` in configuration inputs.
Source endpoints, trust files, custody policies and durable journal settings
are described in the [migration operations guide](crates/layerx-migrate/OPERATIONS.md).

### Gas station

`paxeer-gas-station` reads JSON configuration through `--config` and durable
state through `--journal`. Its rate publisher also reads `--rate-file`, and
its configuration check reads `--rate-config`. The service reads
`GAS_STATION_PAYMASTER`, `GAS_STATION_DID_GAS_LIMIT` and
`GAS_STATION_BROWSER_ORIGINS`; `relayer_key_env` names the signing-key
variable and `rate_owner_key_env` names the rate publisher's signing-key
variable. The deployment inputs are listed in
[`deploy/gas-station/railway.env.example`](deploy/gas-station/railway.env.example).

### Web search

`x-websearch` reads a JSON file selected by `--config`. Configuration examples
are [`crates/x-websearch/config.example.json`](crates/x-websearch/config.example.json)
and [`deploy/x-websearch/config.example.json`](deploy/x-websearch/config.example.json).
It reads `X_WEBSEARCH_ATTESTOR_KEY_FILE`, `X_WEBSEARCH_SUBMITTER_KEY_FILE` and
`X_WEBSEARCH_RECEIVER_KEY_FILE` for role-specific key files.
`PAXEER_X_SOURCE_REVISION` supplies optional build-time source metadata.
See the [sidecar page](crates/x-websearch/README.md) for payment, program relay
and settlement settings.

## Documentation

- [Interop overview and gateway routes](../../docs/wiki/Interop.md).
- [Platform interop guide, MCP and A2A](../../docs/platform/content/guide/interop.md)
  and [CLI installation](../../docs/platform/content/install.md).
- [x402 transport and settlement](../../docs/wiki/X402Transport.md)
  and [compatibility notes](crates/layerx-x402/COMPATIBILITY.md).
- [Portable receipt and external evidence verification](../../docs/interop/PORTABILITY.md)
  and [portable verifier overview](../../docs/wiki/PortableVerifier.md).
- [Source migration](../../docs/wiki/Migration.md),
  [fiat rails](../../docs/wiki/FiatRamps.md) and
  [bridge overview](../../docs/wiki/Bridge.md).
- [Bridge contract runbook](../../contracts/bridge/README.md)
  and [MCP server](../agent/crates/layerx-mcp/README.md).
- Documentation site source pages: [interoperability](../../docs/site/paxeer-docs/src/app/interop/page.tsx),
  [mirrors](../../docs/site/paxeer-docs/src/app/mirrors/page.tsx) and
  [bridge](../../docs/site/paxeer-docs/src/app/bridge/page.tsx).
