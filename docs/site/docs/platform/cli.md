# LayerX CLI

The developer binary is `layerx`, from the `layerx-platform-cli` crate in
`tools/platform/cli`. Its command parser is `src/main.rs`; wallet and token
commands are declared in `src/wallet.rs`, and chain commands in
`src/commands.rs`. The binary name remains LayerX after the network merge.

Paxeer mainnet-beta is one BFT network, with EVM chain ID **125** (`0x7d`).
The kernel participates in block proposal inside the network. A transaction's
receipt is final when its block is committed. Sub-chains are tenant domains
in the database; creating a domain does not require another node deployment.

Public node JSON-RPC uses the root URL of
`https://api-mainnet-beta.paxeer.network` or `https://api1.mainnet-beta.paxeer.network`
through `https://api14.mainnet-beta.paxeer.network`. See
[Public RPC endpoints](../reference/public-rpc.md) for node selection and
WebSocket access. Hosted `px_` methods belong to the
[hosted gateway contract](gateway-rpc.md), declared in
`services/platform/hosted/gateway/openrpc.json`; they are not promised by
public nodes.

**Transport:** `src/rpc.rs::RpcClient::new` and `src/chains.rs::node_endpoint`
accept only an HTTP(S) origin at the root path and refuse any other path,
including the retired `/rpc` route. `src/programs.rs::rpc_call`, `src/chains.rs`
and `src/a2a.rs` post JSON-RPC to `/`. Registration and faucet take an explicit
hosted service URL and post to exactly that URL.

Local key, derivation, scaffolding, build, and configuration operations remain
useful independently of those transports. The command inventory below
separates local operations, hosted features, and emulator compatibility.
For application workflows, see [Payments](../overview/payments.md),
[Programs](../programs/index.md), and
[Portable receipt verification](../interop/portable-receipts.md).

## Commands

Prefix the commands in the tables with `layerx`.

### Global options and output

| Option | Current behavior |
| --- | --- |
| `--json` | Emit the command result as JSON. |
| `--rpc <url>` | Override the RPC target for `wallet`, `token`, `chains`, `program`, or `receipt get`. Must be an origin at the root path. |
| `--gateway-credential <alias>` | Load a stored gateway credential for those same command groups. The alias is not the credential itself. |

The latter two options are rejected on other command groups, including
`register` and `faucet`. Those commands take an explicit hosted service URL.

`src/output.rs` emits successful command results on stdout as
`{ok: true, kind, message, data}` and runtime errors on stderr as
`{ok: false, error: {code, detail}}`. A leading `snake_case: ` error prefix
becomes `error.code`; prose errors use `command_failed`. Parser errors are
handled by clap before command execution. Wallet writes can print disclosure
and submission JSON to stderr before the final result; long-running servers
use their transport protocol rather than a single command-result object.

### Wallet and token commands

These commands are present in the parser. Presence does not guarantee that
the configured remote endpoint supplies their backend. Native account reads
require authenticated SDK snapshots; token reads use the SDK's paginated
asset contract and do not independently prove its state. An ordinary EVM
balance response is not a substitute for account evidence.

| Command | Inputs and behavior |
| --- | --- |
| `wallet create <name>` | Optional `--did`. Creates or reuses an Ed25519 key and registers its main account on the emulator only, without an RPC override. Other targets return `wallet_registration_unavailable` before generating a key. |
| `wallet import <name>` | Read a 32-byte hexadecimal Ed25519 seed from stdin; optional `--did`. Imports a local key without registering or funding it. |
| `wallet derive` | Read a BIP-39 phrase from stdin or `--mnemonic-file`; optional `--passphrase-file`, `--index` (default `0`), `--export-private-keys <new-file>`, and `--bind`. Local derivation returns EVM address and DID metadata. Binding requires RPC and remains subject to transport and method availability. |
| `wallet list` | List local public key metadata. |
| `wallet balance` | Optional `--did` and `--asset`. Select the DID explicitly or from the default key; emulator reads local state, other targets require an authenticated SDK account snapshot. |
| `wallet send` | `--to`, `--asset`, `--amount`, plus the write inputs below. Signs and submits a native transfer when all required account, sequence, signing, and RPC inputs are available. |
| `wallet open-account` | `--asset`, plus the write inputs below. Opens the selected wallet's account for that native asset. |
| `wallet estimate-fee <unsigned_tx_hex>` | Required `--from <0x-address>`. Validate unsigned EIP-1559 bytes, call `eth_estimateGas` with `pending`, then `eth_gasPrice`; return decimal strings `gas`, `gas_price`, and `fee`. Uses the CLI RPC client with the transport limitation above. |
| `wallet watch <topic>` | Parser accepts `receipts`, `checkpoints`, or `account`; optional `--account-id` and `--timeout-seconds` (default `60`, range `1–300`). The transport supports `account --account-id <EVM-address>`, translating it to an `eth_subscribe` logs filter. Other topics are refused. Requires RPC and a gateway credential; retains retired routing rather than using the public root WebSocket endpoint. |
| `wallet receipt <tx_hash>` | A `0x`-prefixed 32-byte transaction hash, `--receipt-policy <file>`, and optional `--wait`. Fetch and verify transaction evidence; see receipt limitations below. |
| `wallet history` | Hidden compatibility command; optional `--did`. Returns `wallet_history_unavailable`. |
| `token create` | `--symbol`, `--name`, `--decimals`, `--salt`, and write inputs. Optional `--supply-cap` defaults to `0`. Encodes native token registration. |
| `token mint` | `--asset`, `--to`, `--amount`, and write inputs. Mint to an existing asset account. |
| `token burn` | `--asset`, `--amount`, and write inputs. Burn from the selected wallet's asset account. |
| `token transfer` | Same inputs and implementation as `wallet send`. |
| `token info <asset>` | Read native token metadata through the SDK. Asset identifiers are 32-byte hex values. Backend or method errors are reported rather than replaced with fabricated metadata. |
| `token list` | Optional `--cursor <asset>` and `--limit <1–256>`; backend default page size is `64`. Follow `next_cursor` to continue the asset list. |

Native writes require `--evm-key-file`, `--receipt-policy`, and `--fee-limit`;
`--key` selects the local Ed25519 key, otherwise the configured default is used.
The EVM key file must match the selected wallet identity. `wallet send` and
`token transfer` enforce these inputs at runtime; the other native write
commands require them in the parser. The former claim that transfers always
refuse because a signing API is missing is obsolete: `src/wallet.rs::execute_write`
now prepares and signs the EIP-1559 transaction, checks the fee estimate,
checks the returned transaction hash, and waits for evidence.

Write commands accept `--wait executed|batched|finalised` (default `executed`)
and `--timeout-seconds` (`1–300`, default `60`). These are compatibility names
with the behavior described under [Receipt verification](#receipt-verification-before-success).
They do not define different network consensus stages.

### Projects, programs, and tenant domains

Program network commands encode calls to the `layerxprograms` precompile at
`0x000000000000000000000000000000000000101d`. Mutations use signed EIP-1559
transactions and `eth_sendRawTransaction`; reads use `eth_call` or bounded
`eth_getLogs`. They do not use the retired REST program-mutation ingress.
Their HTTP transport still has the limitation stated at the top of this page.

| Command | Inputs and behavior |
| --- | --- |
| `new <name>` | Scaffold a Rust program; optional `--directory` defaults to `.`. Name must be a lowercase Cargo package name of 1–64 characters, using letters, digits, and hyphens, and must not start with a hyphen. |
| `program build` | Local compile and runtime-policy validation; `--manifest-path` defaults to `Cargo.toml`, optional `--artifact`. |
| `program bindings` | Local generation requires `--interface`, `--digest`, `--code-hash`, `--deployment-proof`, and `--trust-history`. Optional `--historical`; `--output` defaults to `bindings`. Verification binds generated code to the supplied deployment evidence. |
| `program discover <program_id>` | Read active program metadata from the precompile. |
| `program interface get <program_id>` | Read the deployed, code-bound interface. |
| `program interface publish <program_id>` | `--interface`, `--artifact`, `--evm-key-file`, `--idempotency-key`; optional `--fee-limit` defaults to `0`. Republishes the artifact and interface through upgrade. |
| `program deploy <artifact>` | Lifecycle inputs below; optional `--upgrade-authority` and canonical encoded `--interface`. |
| `program upgrade <artifact>` | Lifecycle inputs and `--old-hash`; optional `--migration-hook`, `--interface`, `--clear-interface`, and `--allow-breaking-interface`. Interface clearing conflicts with an interface file; allowing a breaking interface requires that file. |
| `program wind-down route` | Lifecycle inputs plus `--account`, `--asset`, `--destination`; optional `--seed` defaults to empty. |
| `program wind-down deprecate` | Lifecycle inputs plus `--exit-program` and `--deadline-batch`. The latter is the parser's retained field name. |
| `program wind-down tombstone` | Lifecycle inputs. |
| `program wind-down exit` | Lifecycle inputs plus `--account`. |
| `program call <program_id>` | `--fuel` and runtime-required `--evm-key-file`; optional `--calldata`, `--fee-limit` (default `0`), repeated `--capability`, and native guest options. Returns the transaction hash at `rung: submitted`, not verified execution success. |
| `program simulate <program_id>` | `--fuel`, optional `--from`, calldata, fee, capabilities, and native guest options. Uses `eth_call` at `pending` without submitting a transaction. |
| `program registry list` | Enumerate deployment events with bounded log queries. This command is implemented. |
| `program registry get <program_id>` | Read the deployed program record. |
| `program registry mirror-source` | `--program-id`, `--source-uri`, `--plan`, `--archive`; requires the source-registry publication configuration described below. Mirroring does not establish on-chain source verification. |
| `program registry verify-source <program_id>` | `--source-uri`, `--source-digest`, `--idempotency-key`. Reads the deployed record, then reports source verification unavailable because that record exposes no source digest. |
| `chains list` | Enumerate tenant-domain registry entries. |
| `chains show <id_or_name>` | Read a registry entry by numeric ID or name. |
| `chains create` | `--name`, `--tier starter|growth|pro`, `--fee-share-bps` (`0–5000`), and runtime-required `--evm-key-file`. Signs a registry transaction; does not provision another network. |

Shared lifecycle inputs are `--program-id` and `--evm-key-file`, with optional
`--fee-limit` defaulting to `0`. The old account-sequence, validity-window,
previous-root, and idempotency flags are not lifecycle signing inputs now.

Native call options in `src/programs.rs::NativeCallOptions` include
`--abi-version` (default `2`), `--entrypoint` (default `layerx_call`),
`--access-declaration`, `--response-capacity`, `--memory-bytes`,
`--storage-read-bytes`, `--storage-write-bytes`, `--output-values`,
`--output-bytes`, and `--table-elements`. Match the deployed program's ABI and
resource requirements; the command validates canonical interface bytes and
artifact compatibility rather than accepting an arbitrary interface document.

### Profiles, credentials, and hosted operations

| Command | Inputs and behavior |
| --- | --- |
| `environment list`, `environment current` | Inspect configured profiles and the active one. |
| `environment use <name>` | Select `emulator`, `beta`, or `production`; `testnet` is a compatibility alias for `beta`. Configuration inputs are described below. |
| `key create <name>`, `key import <name>` | Generate an Ed25519 key from OS randomness or import a 32-byte hexadecimal seed from stdin. Optional `--did`. |
| `key list`, `key show <name>` | Read public key metadata. |
| `key default <name>`, `key delete <name>` | Select the default key or permanently remove its stored secret and metadata. |
| `auth set`, `auth status`, `auth delete` | Optional `--environment`, otherwise active profile. Set reads a token from stdin; status reports presence without printing it. |
| `register` | Optional `--key`, otherwise default. Sign the hosted `px_register` tenant binding, fixed to tenant `beta`, and validate the returned subject and signer. It does not create a native main account. |
| `faucet` | Required `--service-endpoint`; optional `--key`, otherwise default; requires a stored bearer identity session. Calls hosted `px_requestFunds`, validates the funded result, and preserves the funding transaction ID. |
| `account create` | Emulator uses a local key (`--key` or default) and optional `--initial-amount` (default `0`). Hosted creation requires `--email`, `--display-name`, `--idempotency-key`, and zero initial amount. |
| `account get` | Emulator requires `--did`; hosted mode reads the active account profile. |
| `payment test` | `--from`, `--to`, `--currency` (alias `--asset`), positive `--amount`, and `--idempotency-key`. Calls the active service's quote and commit routes. Its result is not independently verified receipt evidence. |
| `receipt get <tx_hash>` | Fetch `pax_sequencerReceipt` through the program client; hash must be `0x` plus 64 hex digits. This is a fetch, not a proof-verification command. |
| `receipt verify` | Local compatibility verifier; requires `--receipt`, `--batch-id`, `--asset`, `--previous-state-root`, `--resulting-state-root`, and `--sequencer-public-key`. Its scope is explained below. |

Registration and faucet post to the hosted service URL given with `--gateway-endpoint` and `--service-endpoint`; that URL is not a public node API.
Use a service deployment's declared contract and authentication requirements.
Do not configure hosted account or payment REST operations against a public
node and assume it supplies those routes.

### Workspace and agent transports

| Command | Inputs and behavior |
| --- | --- |
| `workspace` | Interactive visual workspace; requires a TTY and refuses `--json` interactive mode. |
| `workspace modules` | List controlled repository modules. |
| `workspace doctor` | Inspect readiness; optional repeated/comma-separated `--module`, `--all`, and `--environment`. |
| `workspace install`, `workspace build`, `workspace test`, `workspace all` | Same selection options, plus `--dry-run`, `--fail-fast`, `-y`/`--yes`, `--ci`, and `--allow-production`. `all` performs install, build, then test. Production tests require `--allow-production`. |
| `install mcp` | Select one or more `--host` values: `layerx`, `claude-code`, `claude-desktop`, `cursor`, or `vscode`. Optional `--daemon-binding` and `--read-only`. Opens the agent enrollment binding and checks the authenticated daemon's tool catalogue before installing host configuration. |
| `mcp serve` | Required `--daemon-binding <file>`, optional `--read-only`; serve the bound agent tools over stdin/stdout. |
| `install a2a` | Install a hosted, payment-capable A2A runtime; optional `--environment`, `--key`, `--source-account`, `--asset`, `--listen`, `--well-known`, `--read-only`, `--token-stdin`, and `--rotate`. Emulator installation is refused. |
| `a2a serve` | Requires `--gateway-credential` and `--authorization-file`; optional `--environment`, `--key`, `--source-account`, `--asset`, `--listen`, and `--read-only`. Loopback listener defaults to `127.0.0.1:9433`. |
| `a2a start`, `a2a stop`, `a2a status` | Manage the installed runtime using its `a2a/runtime.json` manifest. |

MCP's agent daemon is an application runtime, not a separate consensus node.
Its default binding document is `mcp/binding.json` beside the CLI config file.
Enrollment must write that document first; installation refuses a missing or
invalid binding. This path holds no CLI signing seed or gateway credential.
A2A uses stored gateway credentials; its `activity.submit` accepts signed
EIP-1559 transaction bytes and preserves a hash for indeterminate submissions.
Its current submission transport also retains retired routing.

## Credentials

The default credential store is the OS keyring: Keychain on macOS, Credential
Manager on Windows, and Secret Service on Unix. `src/credential.rs` uses
service `dev.layerx.cli` and entries `key:<name>`, `token:<environment>`, and
`gateway:<alias>`. The configuration JSON stores public key metadata and
profile endpoints, not those secrets.

Keys and API tokens imported by the CLI arrive on stdin, with a 16 KiB input
limit. Creating a key needs no seed input. `auth status` reports token
presence separately from key presence. There is no automatic fallback or
migration between credential stores.

### Headless credential storage

On a headless Unix host, explicitly select the encrypted file backend:

```bash
export LAYERX_CREDENTIAL_STORE=file
read -r -s -p 'Credential passphrase: ' LAYERX_CREDENTIAL_PASSPHRASE
export LAYERX_CREDENTIAL_PASSPHRASE
layerx --json key create quickstart
layerx --json key list
layerx --json auth status
```

The passphrase must contain 12–16384 bytes. Supply it through secret
environment configuration in CI and keep it available to later processes
using the vault. Secret imports still read stdin independently. Unset
`LAYERX_CREDENTIAL_STORE`, or set it to `os`, to select the OS keyring.

`src/file_store.rs` writes `credentials/vault` beside the resolved config.
Encryption uses AES-256-GCM and PBKDF2-HMAC-SHA256 with 600,000 iterations,
a fresh 16-byte salt, and a fresh 12-byte nonce on each update. The directory
is mode `0700`; vault and lock files are mode `0600`. Ownership, symlinks,
permissions, passphrase authentication, and vault integrity are checked.
Updates use a process lock and atomic replacement. The backend requires Unix
owner-only permissions.

Back up both the encrypted vault and its passphrase. Changing the environment
passphrase does not rotate an existing vault; it causes authentication to
fail. The `mock` backend is available only in binaries built with
`test-credential-store`; production binaries refuse that selection.

## Environment variables

| Variable | Effect |
| --- | --- |
| `LAYERX_CONFIG` | Explicit configuration JSON path, absolute or relative to the working directory. |
| `XDG_CONFIG_HOME`, `HOME` | Config fallback: `$XDG_CONFIG_HOME/layerx/config.json`, otherwise `$HOME/.config/layerx/config.json`. |
| `LAYERX_CREDENTIAL_STORE` | Select `os` or `file`; `mock` requires the test feature. |
| `LAYERX_CREDENTIAL_PASSPHRASE` | Passphrase for the encrypted file backend. |
| `LAYERX_GATEWAY_KEY_ID` | Non-secret ID checked against the A2A runtime's stored gateway credential. |
| `LAYERX_INSTALL_ROOT` | Override the home directory used to find agent host configurations. |
| `LAYERX_REPO_ROOT` | Repository root used by workspace commands. |
| `LAYERX_PROGRAM_SDK` | SDK path written into scaffolded program projects. |
| `CARGO` | Cargo executable used by program builds. |

Source mirroring additionally reads `LAYERX_BETA_REGISTRY_URL`,
`LAYERX_BETA_REGISTRY_PUBLICATION_TOKEN_FILE`, and
`LAYERX_BETA_REGISTRY_PUBLICATION_KEY_FILE` (`src/programs.rs`). The latter
two variables name files containing the publication secrets, not literal
tokens. Workspace child processes receive
`LAYERX_ENVIRONMENT`, `LAYERX_ENDPOINT`, and `LAYERX_NETWORK_ID`.
`LAYERX_API_TOKEN`, `LAYERX_TOKEN`, and `LAYERX_AUTH_TOKEN` do not replace
`auth set`; managed A2A startup removes those token variables from the child.

## Emulator, hosted gateway, and node

A missing config produces version `1`, environment `emulator`, endpoint
`http://127.0.0.1:9402`, network ID `402`, and no bound emulator key.
These are local compatibility defaults, not the public chain's identity.
Non-loopback HTTP endpoints are refused; remote connections require HTTPS.

`environment use` accepts endpoint, network ID, and one trust-anchor input
(`--sequencer-trust-anchor` or `--sequencer-trust-anchor-file`) as a complete
set, or omits the whole set to select an existing profile. Only the emulator
selection verifies the advertised identity via `GET /v1/sequencer`.
Selecting `beta` or `production` stores the supplied values without that
handshake. A stored anchor in such a profile does not prove public-chain
finality or make a node serve hosted APIs.

The retained local emulator commands are:

| Command | Compatibility behavior |
| --- | --- |
| `emulator provision` | Create a local signing seed and trust-anchor file under the profile directory. Existing files are refused unless `--force` is supplied. |
| `emulator up` | Requires `--sequencer-seed-file`; optional `--listen`, `--network-id`, `--protocol-version`, `--time-ms`, and repeated `--prefund`. Starts the local transition gateway in-process. |

These local identity and prefunding operations are emulator tooling, not
instructions for starting another mainnet service. Hosted account creation
uses `/v1/accounts`; emulator prefunding uses `/__emulator/accounts/prefund`.
`payment test` uses `/v1/moves/quote` followed by `/v1/moves`. Those service
routes and their bearer tokens are distinct from public node JSON-RPC.

To check public chain identity directly, use a root-path JSON-RPC client:

```bash
curl --max-time 15 -sS https://api1.mainnet-beta.paxeer.network \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected chain ID is `"0x7d"`. For the method inventory and other nodes,
see [Public RPC endpoints](../reference/public-rpc.md). The public namespaces
are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test`, and `txpool`.
`node.hyperpaxeer.com` is the hpx library mirror, never an RPC endpoint.

## Refusals

| Error or condition | Meaning and next step |
| --- | --- |
| `command_failed` with a URL-suffix error | The CLI transport still enforces a retired route. Use a client supporting the documented root node endpoint; changing the public URL does not repair the CLI. |
| `wallet_registration_unavailable` | Native main-account registration is unavailable on that target; no new key was generated. Hosted `register` creates a tenant principal instead. |
| `wallet_history_unavailable` | No DID activity-history backend is published for this command. |
| `identity_session_required` | Faucet needs a stored bearer identity session. Set it with `auth set`. |
| `identity_sequence_unavailable`, `wallet_accounts_unavailable` | Required authenticated account or sequence evidence is absent or malformed. The command must obtain valid evidence before signing. |
| `rpc_method_unavailable` | Method absent from the CLI contract or switched off. In `src/rpc.rs`, all methods beginning with `lx_` are explicitly refused. |
| `fee_limit_exceeded` | The declared fee limit cannot cover the estimated gas. Review the quote and inputs before resubmitting. |
| `transaction_hash_mismatch`, `receipt_verification_failed` | Returned transaction or receipt evidence does not match the operation. Do not treat it as success. |
| `transaction_pending` | Timeout before the requested evidence rung. Retain the transaction hash and query it again rather than creating another payment. |
| `network_id_mismatch`, `sequencer_trust_anchor_mismatch` | Local emulator identity disagrees with its bound profile. These checks are not mainnet finality checks. |
| `environment_input_missing` | Only part of the endpoint/network/anchor set was supplied. |
| `network_id_reserved` | Local network ID `0` is refused. |
| `sequencer_seed_exists`, `sequencer_trust_anchor_exists` | Emulator provisioning would replace an existing identity file. |
| Credential store unavailable (`command_failed`) | The selected OS backend, file passphrase, or vault is unavailable, invalid, or unsupported. There is no silent fallback. |
| MCP binding unavailable (`command_failed`) | Enrollment has not produced a usable binding document, or the daemon has not admitted its tool catalogue. |
| Source verification unavailable (`command_failed`) | The deployed program record does not expose a source digest. A successful source mirror does not resolve this. |

Remote JSON-RPC failures can retain their numeric code, message, and data
inside the CLI error detail. That remote numeric code is distinct from the
CLI's string `error.code`.

## Receipt verification before success

On the merged network, block commitment establishes finality. Submission,
execution output, a live notification, or a successful JSON envelope alone
does not prove that a transaction's block has committed. See
[Finality](../protocol/finality.md) and [Checkpoints](../concepts/checkpoints.md).

Current CLI behavior has several distinct evidence scopes:

- `receipt get` fetches `pax_sequencerReceipt` status without independently
  verifying it. The method name remains part of the node contract; it does
  not imply a separate block-producing service.
- `program call` checks that the returned transaction hash matches the signed
  bytes and returns `rung: submitted`. Program lifecycle operations additionally
  fetch status and check the hash and recognized rung; they do not run the old
  terminal receipt verifier on this path. A submission acknowledgement is not
  verified program execution or committed success.
- `program simulate` decodes the precompile's ABI result from `eth_call` at
  `pending`. It submits nothing and establishes no committed receipt.
- Wallet write and receipt paths require an independently pinned receipt
  policy. `src/wallet.rs::read_transaction_policy` requires protocol `2`,
  network `125`, a nonzero ordered height range (`first_batch`, `last_batch`),
  `sequencer_id`, `sequencer_key`, and no checkpoint context digest. These are
  retained policy field names, not instructions to trust a gateway's own keys.
- Wallet `--wait executed` verifies receipted evidence. Compatibility value
  `batched` requests committed evidence and cross-checks the chain receipt's
  hash, block, status, and transaction index. `finalised` currently refuses
  because that CLI path has no finality trust configured. This limitation does
  not change when the network itself considers a block final.
- `receipt verify` reads raw bytes or hex from a local file and calls
  `layerx_proof::receipt::verify_outcome` with caller-supplied `AuthorizedBatch`
  facts. It is a compatibility verifier for that receipt format, not a public
  EVM transaction inclusion or BFT commitment verifier.
- `payment test` returns the hosted quote/commit journey result; it does not
  call the local proof verifier. Wallet notifications are also unverified.

For portable transaction and commitment evidence, use the
[portable verifier](../interop/portable-receipts.md) with independently
configured chain trust. After a timeout or indeterminate response, preserve
the original transaction hash and reconcile its status before retrying.

## Test suite

The CLI tests live in `tools/platform/cli/tests`. Their source is useful for
checking command behavior; an emulator test does not establish deployment or
public-node support.

| Source or target | Coverage |
| --- | --- |
| `tests/common/mod.rs`, `tests/common/credential_environment.rs` | Real CLI child processes, isolated configuration, and a private D-Bus/Secret Service using `dbus-daemon` and `gnome-keyring-daemon`. The helper can decline credential tests when D-Bus is absent; `LAYERX_CLI_TESTS_REQUIRE_DBUS` requires that dependency instead. |
| `tests/credential.rs`, `tests/file_store.rs` | Key/token handling, secret protection, and encrypted file storage. |
| `tests/commands.rs`, `tests/emulator.rs` | Command parsing/results, invalid inputs, local profile binding, account prefunding, and payment operations. |
| `tests/wallet_commands.rs`, `tests/wallet_encoding.rs`, `tests/wallet_send.rs` | Wallet operations and native payload/authorization encodings. |
| `tests/workspace.rs`, `tests/install.rs` | Workspace inventory and agent installation behavior/refusals. |
| `tests/production-credential-refusal.sh` | A production-shaped binary refuses the test-only credential backend. |
| `tests/clean-bootstrap.sh`, `tests/install-journey.sh` | Bootstrap and hosted agent-install journeys. |
| `make platform-test-tooling` | CLI and tooling tests with runtime-clock and public-TLS prerequisites, plus bootstrap and script checks; declared in `services/platform/Makefile.inc`. |

[Home](../index.md)
