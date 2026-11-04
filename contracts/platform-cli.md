# Platform CLI contract

The developer binary is **`layerx`**, declared by [tools/platform/cli](../tools/platform/cli/Cargo.toml). Its name and historical receipt types remain in the tree while LayerX functionality is merged into one Paxeer BFT network. Mainnet-beta has EVM chain ID **125** (`0x7d`); the kernel is part of the network and proposes blocks, and a receipt becomes final when its block is committed. Sub-chains are tenant domains in the database, not independently deployed chains.

Public node JSON-RPC uses the HTTPS root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, and `api-mainnet-beta.paxeer.network`. The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test`, and `txpool`. Hosted `px_` methods have their own [OpenRPC contract](../services/platform/hosted/gateway/openrpc.json). `node.hyperpaxeer.com` is only the build library mirror, not an RPC endpoint.

See [explorer HTTP](explorer-http-api.md), [socket updates](explorer-ws.md), [metrics](explorer-metrics.md), and [historical header verification](kernel.md#format-batch-header). These describe different application transports on the same network.

## CLI CLI commands

[main.rs](../tools/platform/cli/src/main.rs) declares the command tree and dispatches to the owning modules:

| Command family | Purpose |
| --- | --- |
| `new <name> --directory <path>` | Scaffold a deterministic Rust program project |
| `workspace` | Install, build, and inspect repository workspace modules |
| `environment list`, `current`, `use` | Inspect/select configured endpoint profiles |
| `key create`, `import`, `list`, `show`, `default`, `delete` | Manage Ed25519 signing keys and public metadata |
| `auth set`, `status`, `delete` | Store and inspect hosted API tokens |
| `account create`, `get` | Developer account operations |
| `register`, `faucet` | Identity registration and beta faucet requests |
| `payment test` | Quote and submit a payment with a stable idempotency key |
| `receipt get`, `verify` | Fetch receipt material or verify supplied receipt evidence locally |
| `program` | Discover, build, bind, publish, deploy, and call program interfaces |
| `wallet`, `token` | Native wallet, transfer, account, and token operations |
| `chains list`, `show`, `create` | Registry operations for tenant domains |
| `emulator provision`, `up` | Provision a local signing identity and run the development emulator |
| `install`, `mcp`, `a2a` | Install/manage agent integrations and serve transports |

Command registration is not proof that the selected service implements every operation. Consult each subcommand's declared arguments and backend handler before using it in automation. In particular, the current emulator route table is narrower than the full CLI command tree.

`--json` is global. Ordinary successful commands emit one object on stdout with `ok: true`, `kind`, `message`, and `data`. Failures emit `ok: false` and `error: {code, detail}` on stderr and exit unsuccessfully. Long-running MCP/A2A servers use their own protocol streams rather than ordinary command output. See [output.rs](../tools/platform/cli/src/output.rs).

### Endpoint selection and current transport limits

Global `--rpc` and `--gateway-credential` are accepted only for wallet, chains, token, program, and `receipt get`; dispatch rejects them for other command families. An alias selects a stored gateway credential, not a literal secret to print in shell history.

The pre-merge `/rpc` suffix assumption remains in CLI program/chain override validation and some wallet/chain transports; that retired path is not the public node endpoint. The implementations strip or append that suffix depending on the operation. Do not append it to a public node root to make the CLI appear configured. These code paths require transport alignment before they can be presented as working public-node integrations. Other commands use the configured environment endpoint and service-specific HTTP routes; changing a profile cannot make a node implement hosted application REST endpoints.

The shared [HTTP client](../tools/platform/cli/src/http.rs) accepts HTTP only for loopback authorities (`localhost`, `127.0.0.1`, or `[::1]`); non-loopback endpoints require HTTPS. It trims trailing slashes and joins validated absolute resource paths. System root certificates and optional configured client identity are used for TLS.

## CLI config keys

[config.rs](../tools/platform/cli/src/config.rs) stores JSON version 1 with:

| Key | Meaning |
| --- | --- |
| `version` | Must equal 1 |
| `current_environment` | Active profile name |
| `default_key` | Optional local key alias |
| `environments` | Map of names to endpoint profiles |
| `environments.<name>.endpoint` | Application endpoint base |
| `environments.<name>.network_id` | u32 protocol network identifier |
| `environments.<name>.sequencer_trust_anchor` | Optional independently supplied receipt-signing anchor |
| `keys.<name>.did`, `keys.<name>.public_key` | Public key metadata; private seeds are stored separately |

Supported profile names are `emulator`, `beta`, and `production`; legacy `testnet` is canonicalized to `beta`. Loading an old `testnet` record adopts it only if a `beta` record does not already exist. New configuration defaults to the sole `emulator` profile, `http://127.0.0.1:9402`, network ID 402, and no key or anchor. That development network ID is not Paxeer mainnet's EVM chain ID.

Loading fails for malformed JSON, unsupported versions, or a missing active environment. Saving creates an owner-only temporary file on Unix, writes/syncs it, then renames it over the configuration. There are no implicit beta/production network endpoints or trust anchors in this default.

## CLI env vars

Configuration path resolution is `LAYERX_CONFIG`, then `XDG_CONFIG_HOME/layerx/config.json`, then `HOME/.config/layerx/config.json`. A relative explicit `LAYERX_CONFIG` is resolved against the current directory.

| Variable | Consumer |
| --- | --- |
| `LAYERX_CREDENTIAL_STORE` | `os` (default) or `file` credential backend |
| `LAYERX_CREDENTIAL_PASSPHRASE` | Required by encrypted-file backend, 12–16384 bytes |
| `LAYERX_CLIENT_CERTIFICATE_FILE`, `LAYERX_CLIENT_PRIVATE_KEY_FILE` | Paired optional TLS client identity; requires HTTPS |
| `LAYERX_EVM_KEY_FILE` | `chains create` key-file alternative to `--evm-key-file` |
| `LAYERX_GATEWAY_KEY_ID` | Optional expected gateway identity used by the agent tool runtime |
| `LAYERX_BETA_REGISTRY_URL` | Program interface registry publication URL |
| `LAYERX_BETA_REGISTRY_PUBLICATION_TOKEN_FILE`, `LAYERX_BETA_REGISTRY_PUBLICATION_KEY_FILE` | Private publication credentials for the program registry |
| `CARGO` | Cargo executable override used by program builds |

Keys and tokens enter through the command's stdin/file mechanisms. OS credential storage is the default; the explicit file backend is for environments without that facility. Neither the explorer's user session nor an explorer rate-limit key is a CLI wallet signing credential.

## CLI file formats

The encrypted credential file in [file_store.rs](../tools/platform/cli/src/file_store.rs) is stored under `credentials/vault` beside the CLI configuration. Its layout is `LXVAULT1` (8 bytes), random salt (16), random nonce (12), then AES-256-GCM ciphertext with its authentication tag. The header is authenticated additional data. PBKDF2-HMAC-SHA256 derives the key with 600,000 iterations. Plaintext is a JSON string map.

The total vault limit is 1 MiB. Unix owner-only directory/file checks and locking protect access; symlink-following is refused on protected opens. Wrong passphrases or damaged authentication fail the read. The JSON configuration contains public metadata, not this plaintext map. Do not copy the encrypted-file bytes into ordinary configuration or logs.

## CLI HTTP routes

[A2A](../tools/platform/cli/src/a2a.rs) binds a fixed, nonzero loopback socket port. `GET /.well-known/agent-card.json` and `GET /.well-known/agent.json` publish its card. `POST /` dispatches JSON-RPC and requires a bearer token from the configured authorization material. Unauthorized POSTs return HTTP 401, unknown GET/POST routes 404, and other verbs 405.

The card routes are discovery reads, not an authorization bypass for task execution. The runtime uses bounded task tracking (128 entries) and bounded admission; its task map is process-local.

## CLI JSONRPC methods

A2A dispatch implements `message/send`, `tasks/get`, and `tasks/cancel`. It explicitly refuses `message/stream`, `tasks/resubscribe`, and `tasks/pushNotificationConfig/set`. MCP uses its own tool protocol over stdin/stdout. Neither transport is the network's public Ethereum JSON-RPC namespace.

## CLI error codes

For A2A JSON-RPC, the dispatcher uses:

| Code | Meaning |
| --- | --- |
| `-32700` | Invalid JSON |
| `-32600` | No method in the message |
| `-32601` | Method/tool not implemented |
| `-32602` | Invalid or missing parameters |
| `-32603` | Internal/task-ledger failure |
| `-32001` | Task not found |
| `-32002` | Task already terminal; cancellation refused |
| `-32004` | Unsupported deployment operation |

Ordinary CLI errors have string codes. The output formatter preserves a lowercase/digit/underscore prefix before `: ` only if it contains an underscore; otherwise it emits `command_failed`. Bootstrap codes include `environment_input_missing`, `network_id_mismatch`, and `sequencer_trust_anchor_mismatch`. A successful command wrapper is not necessarily a committed transaction: stateful operations can return `submitted` or `unknown`, requiring evidence reconciliation rather than a new idempotency key.

## CLI Rust pub items

The executable modules implement configuration, credentials, programs, account/payment operations, receipt verification, and the agent runtime. [lib.rs](../tools/platform/cli/src/lib.rs) exports library modules including chains and its HTTP client. Public Rust items are implementation APIs; CLI command output and the remote service contract define the automation boundary. Use the modules' real exported types rather than inferring a new API from a registry row with no current caller.

## Emulator CLI commands

[emulator.rs](../tools/platform/cli/src/emulator.rs) provisions a random local identity under the profile's `emulator` directory as `sequencer.seed` and `sequencer.anchor`. Existing identity material is refused unless the caller deliberately selects replacement. Environment selection binds endpoint, network ID, and independently supplied signing anchor; a fetched identity is not automatically trusted.

`layerx emulator up` accepts `--listen`, `--network-id`, `--protocol-version`, `--time-ms`, `--sequencer-seed-file`, and repeatable `--prefund`. The underlying [emulator](../tools/platform/emulator/src/main.rs) defaults to `127.0.0.1:9402`, network ID 402, and accepts protocol versions 2 or 3. Listening outside loopback is refused. The runtime requires an explicit nonzero 32-byte signing seed (raw bytes or 64 hexadecimal characters); it has no compiled-in signing authority. This is a local development process, not a second production chain.

## Emulator HTTP routes

The current `route` function, rather than the historical registry inventory, determines supported emulator operations:

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/healthz` | Process/core health |
| GET | `/v1/sequencer` | Local signing identity |
| GET | `/v1/protocol/account-state/head` | Core head evidence |
| GET | `/v1/state` | Core state inspection |
| GET | `/v1/dids/:did/sequence` | Identity sequence read |
| GET | `/v1/evidence/:id`, `/v1/journeys/:id`, `/v1/receipts/:id` | Retained evidence/journey/receipt reads |
| POST | `/__emulator/accounts/prefund` | Development prefunding |
| POST | `/__emulator/time/set`, `/__emulator/time/advance` | Controlled development time |
| POST | `/__emulator/faults` | Development fault injection |
| GET / PUT | `/__emulator/snapshot` | Export/import recovery bytes |

The former `/v1/settle` operation is retired and returns HTTP 410 for any method. Other unknown paths return 404. Do not assume older program or payment route inventories are still mounted by this emulator.

## Emulator error codes

Refusals have `ok: false`, `error: {code, retry: "never", detail}`, and a trace ID formatted `emu-` plus 16 hexadecimal digits. Declared codes include `invalid_argument`, `invalid_did`, `invalid_snapshot`, `method_not_allowed`, `not_found`, `snapshot_encoding_failed`, and `trace_exhausted`. Core errors are translated through `core_error`; status depends on the failing path. A missing retained receipt is 404 and does not prove a transaction never existed elsewhere.

## Emulator file formats

Recovery snapshots begin with `LXEMR001`, followed by a big-endian u32 version **4**, a core-byte length, counts for retained receipt/program/account/quote/move indexes, the encoded sections, and a 32-byte SHA-256 digest over the domain `layerx-emulator-recovery-snapshot-v1` plus a zero byte and the preceding snapshot bytes. Decode enforces version, digest, lengths, and index relationships before recovery. The digest detects corruption; it is not an external consensus signature.

The emulator holds its ordinary state in process memory. Export a snapshot and import it into the intended development instance to recover retained indexes; merely restarting the process is not persistence. The core snapshot bound is 24 MiB, receipt retention is bounded at 4096, and emulator accounts at 512. See the source for the remaining section bounds and recovery validation.

## Emulator env vars

The emulator's Rust build script reads `CC`, `AR`, and `CARGO_MANIFEST_DIR` to compile/link its native core. Runtime network identity, time, listener, seed, and prefunds come from arguments, not an implicit public endpoint variable.

## Emulator Rust pub items

`layerx_platform_emulator::run(arguments)` is the library entry used by the CLI/workspace tooling. It parses the same argument list and returns `Result<(), String>`; it does not spawn an independent network finality service.

## Release CLI commands

[layerx-platform-release](../tools/platform/release/src/main.rs) is a separate release-declaration tool. Its default/check mode validates registry and workflow declarations; `--plan` prints the plan. `manifest` accepts `--release-dir`, `--output`, `--rollback-version`, and optional `--registries`/`--workflow`. `verify` accepts `--manifest`, optional `--source-revision`, and exactly one of `--from` or `--fetch` to select local artifacts or fetch registry artifacts. Errors print `platform-release: ...` and exit 1. These operations do not run as a chain node.

## Release config keys

Defaults point to [registries.kvx](../tools/platform/release/registries.kvx) and [.github/workflows/platform.yml](../.github/workflows/platform.yml). Artifact records include `name`, `version`, `file`, `digest`, `digest_of`, `signature`, `sbom`, `attestation`, and `location`; image records additionally bind a registry digest, platform, and SBOM/attestation digests. The validator checks the declared pipeline and publication evidence rather than assuming a file's presence proves publication.

## Release env vars

Release workflow declarations use `LAYERX_RELEASE_REGISTRY`, `LAYERX_RELEASE_DISTRIBUTION`, and `LAYERX_RELEASE_PACKAGES`. These are release-job bindings recognized by the pipeline checker, not public node configuration or substitutes for manifest source binding.

## Release file formats

The JSON artifact manifest schema is `layerx/artifact-manifest/1`. Top-level fields are `schema`, `version`, `tag`, `source_revision`, `source_digest`, `rollback_version`, `artifacts`, and `images`. Entries carry source revision, location/reference, digest, signatures, SBOM, attestations, and publication/install or pull-check state. Verification binds artifact bytes to the declared digests and revision; it does not weaken a source-archive digest into proof of built bytes.
