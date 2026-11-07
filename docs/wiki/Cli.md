# LayerX CLI

`layerx` is the developer CLI for the Paxeer X Network. Its crate is
`layerx-platform-cli`, declared in
[Cargo.toml](../../tools/platform/cli/Cargo.toml); command declarations and
dispatch live in [main.rs](../../tools/platform/cli/src/main.rs).
The stable graph anchor returned by `platform_cli()` is `layerx-cli-v1`.

Paxeer mainnet-beta is one BFT network, running `paxd`, with EVM chain ID
`125` (`0x7d`). The kernel is part of the network and proposes blocks. A
transaction receipt is final once its block is committed. Sub-chains are
tenant domains in the database, without separate node infrastructure.
See [Public RPC](PublicRpc.md) for node access and
[Programs](Programs.md) for program concepts.

This reference describes the commands present in the tree, including their
explicit refusals and remaining transport limitations. A command's presence
in the parser does not establish that a public deployment supports its
backend. The public node API and the hosted gateway API have distinct
contracts; the CLI still contains legacy transport assumptions described
[below](#emulator-hosted-gateway-and-node).

## Commands

### Global options and output

`--json` selects machine presentation. Ordinary successful commands write one
JSON object to stdout with `{ok: true, kind, message, data}`. Command failures
write `{ok: false, error: {code, detail}}` to stderr and exit `1`. Invalid
arguments with `--json` use `invalid_arguments` and exit `2`. The output code
preserves a leading lowercase `snake_case` token followed by `": "` as
`error.code`; other errors use `command_failed`. Remote JSON-RPC errors can
therefore appear as JSON text inside `error.detail`, with a generic outer
code. See [output.rs](../../tools/platform/cli/src/output.rs).

Wallet writes also print transaction disclosures and the submitted hash to
stderr. Long-running `emulator up`, `mcp serve`, and `a2a serve` run their
service or transport loop rather than returning an ordinary command result.

`--rpc <url>` and `--gateway-credential <alias>` are global parser options
accepted for `wallet`, `token`, `chains`, `program`, and `receipt get`.
`validate_wallet_transport` rejects them for other command groups. The alias
selects a credential already held in the credential store; it is not a secret
passed on the command line. `a2a serve` separately declares its required
`--gateway-credential` option.

Required inputs below follow the current parser or a stated runtime check.
Use the selected command's `--help` for its complete option list.

### Wallet and token commands

The definitions and handlers are in
[wallet.rs](../../tools/platform/cli/src/wallet.rs). These groups are
implemented; they are not a proposed command surface.

| Command | Behavior and inputs |
| --- | --- |
| `wallet create <name>` | Generate an Ed25519 key, register its identity, and open its main account on the emulator. Optional `--did`. Outside emulator mode, refuse with `wallet_registration_unavailable` before generating a key. |
| `wallet import <name>` | Import a 32-byte hexadecimal Ed25519 seed from stdin; optional `--did`. Does not register or fund an account. |
| `wallet derive` | Derive an EVM address and native DID from a BIP-39 phrase on stdin, or `--mnemonic-file`. Optional `--passphrase-file`, `--index` (default `0`), `--export-private-keys <new-file>`, and `--bind`. |
| `wallet list` | List local public key metadata without opening private seed material. |
| `wallet balance` | Read the selected DID's accounts; optional `--did` and `--asset`. The emulator reads local state; the hosted path requires a state-proven account snapshot. |
| `wallet send` | Sign and submit a native transfer in an EIP-1559 transaction. Requires `--to`, `--asset`, `--amount`, and the wallet write inputs below. |
| `wallet open-account` | Open the selected wallet's account for `--asset`; requires wallet write inputs. |
| `wallet estimate-fee <transaction_hex> --from <address>` | Decode unsigned EIP-1559 bytes and call `eth_estimateGas` at `pending`, then `eth_gasPrice`. Return decimal strings for `gas`, `gas_price`, and their product `fee`. |
| `wallet watch <topic>` | Read one unverified notification. The parser accepts `receipts`, `checkpoints`, and `account`; the current transport supports only `account --account-id <EVM-address>` as an Ethereum logs subscription. Requires explicit RPC transport and a gateway credential. Optional `--timeout-seconds`, default `60`, range `1–300`. |
| `wallet receipt <tx_hash>` | Retrieve and verify evidence for a `0x`-prefixed 32-byte transaction hash. Requires `--receipt-policy` at runtime; optional `--wait` defaults to `executed`. |
| `wallet history` | Hidden compatibility command; always refuses with `wallet_history_unavailable` after resolving the optional `--did` or default wallet DID. |
| `token create` | Register a native asset. Requires `--symbol`, `--name`, `--decimals`, `--salt`, and wallet write inputs; `--supply-cap` defaults to `0`. |
| `token mint` | Mint to an existing asset account. Requires `--asset`, `--to`, `--amount`, and wallet write inputs. |
| `token burn` | Burn units from the selected wallet's asset account. Requires `--asset`, `--amount`, and wallet write inputs. |
| `token transfer` | Same transfer implementation and inputs as `wallet send`. |
| `token info <asset>` | Read asset metadata through the SDK's current RPC implementation. Asset identifiers are 32 bytes expressed as 64 hex digits. |
| `token list` | Read one page of registered assets. Optional `--cursor <asset-id>` and `--limit` (`1–256`); the documented gateway default is `64`. Follow the returned next cursor. |

Wallet write inputs are `--evm-key-file`, `--receipt-policy`, and
`--fee-limit`; `--key` selects local Ed25519 metadata, otherwise the configured
default is used. `--wait` accepts `executed`, `batched`, or `finalised`, with
`executed` the default. Write timeouts use `--timeout-seconds`, default `60`,
range `1–300`. Transfer options are parser-optional but the handler requires
all three write inputs before submission.

The EVM key file comes from `wallet derive --export-private-keys` and must
match the selected native identity. Derivation returns public metadata and
does not import the derived Ed25519 seed into the CLI credential store. Use
`wallet import` or `key import` when that local signing key is needed. Private
key export creates a new owner-only file and refuses an existing path; secrets
are not printed in the normal derivation output. `--bind` requires an RPC
argument, but is subject to the transport and method limitations below.
See [wallet_derive.rs](../../tools/platform/cli/src/wallet_derive.rs) and
[wallet_signing.rs](../../tools/platform/cli/src/wallet_signing.rs).

Writes fetch the identity nonce, quote gas, check the fee limit, disclose the
prepared transaction, sign it, and compare the returned transaction hash with
the locally computed hash. Transfers also authenticate the source account and
its independent debit sequence before signing the native debit. They are no
longer unconditional pre-signing refusals. Missing authenticated evidence or
credentials still causes a refusal.

### Project, workspace, environment, and credentials

| Command | Behavior and inputs |
| --- | --- |
| `new <name>` | Scaffold a deterministic Rust program project. `--directory` defaults to `.`. Names are lowercase Cargo package names, length `1–64`, containing lowercase letters, digits, or hyphens and not beginning with a hyphen. |
| `workspace` | Open the interactive workspace on a TTY. Machine mode or non-TTY stdin refuses interactive use. |
| `workspace modules` | List the modules controlled by the workspace CLI. |
| `workspace doctor` | Inspect tools and readiness without changing modules. Selection: repeatable/comma-separated `--module`, `--all`, and optional `--environment`. |
| `workspace install`, `workspace build`, `workspace test`, `workspace all` | Run the selected steps; `all` installs, builds, then tests. Accept selection flags plus `--dry-run`, `--fail-fast`, `-y`/`--yes`, `--ci`, and `--allow-production`. Production tests require `--allow-production`. |
| `environment list`, `environment current` | Inspect saved endpoint profiles or the active profile. |
| `environment use <name>` | Select `emulator`, `beta`, or `production`; legacy `testnet` is normalized to `beta`. To configure a profile, supply `--endpoint`, `--network-id`, and exactly one of `--sequencer-trust-anchor` or `--sequencer-trust-anchor-file` together. Otherwise use an existing profile. Emulator selection verifies its advertised identity. |
| `key create <name>` | Generate and store an OS-random Ed25519 seed; optional `--did`. |
| `key import <name>` | Read a 32-byte hexadecimal Ed25519 seed from stdin; optional `--did`. |
| `key list`, `key show <name>` | Read public metadata without exposing private keys. |
| `key default <name>`, `key delete <name>` | Select the default signing key or permanently delete a stored key. |
| `auth set`, `auth status`, `auth delete` | Store a token from stdin, report whether one is held, or delete it. Optional `--environment`, otherwise the active profile. |
| `register` | Sign a tenant binding with `--key` or the default key and call hosted `px_register` at the required `--gateway-endpoint` URL. The current handler fixes the tenant to `beta` and checks the returned tenant, subject, and signer. This creates an identity principal, not a funded main account. |
| `faucet` | Call hosted `px_requestFunds` at the required `--service-endpoint` URL using a stored bearer identity session and `--key` or the default key. Accept only a confirmed funded claim carrying funding ID, transaction ID, positive amount, and network. |
| `account create` | Emulator: use a local key and optional `--initial-amount` (default `0`). Hosted: require `--email`, `--display-name`, `--idempotency-key`; initial amount must be `0`. |
| `account get` | Read the hosted profile, or an emulator account selected by required `--did`. |
| `payment test` | Quote then commit a payment with `--from`, `--to`, `--currency` (alias `--asset`), positive `--amount`, and `--idempotency-key`. This is the REST payment journey. |
| `receipt get <id>` | Fetch `pax_sequencerReceipt` by transaction hash. Retrieval alone does not verify commitment. |
| `receipt verify` | Locally verify canonical receipt bytes with `--receipt`, `--batch-id`, `--asset`, `--previous-state-root`, `--resulting-state-root`, and `--sequencer-public-key`. |

Command dispatch is in [main.rs](../../tools/platform/cli/src/main.rs).
Workspace selection and execution are in
[workspace.rs](../../tools/platform/cli/src/workspace.rs), scaffolding in
[scaffold.rs](../../tools/platform/cli/src/scaffold.rs), and the hosted
identity/funding handlers in [register.rs](../../tools/platform/cli/src/register.rs)
and [faucet.rs](../../tools/platform/cli/src/faucet.rs).

### Programs and tenant domains

The program command handlers use the native `layerxprograms` precompile at
`0x000000000000000000000000000000000000101d`. Reads use Ethereum calls and
logs; writes submit signed EIP-1559 transactions. See
[programs.rs](../../tools/platform/cli/src/programs.rs).

| Command | Behavior and inputs |
| --- | --- |
| `program discover <program_id>` | Read the registry record and refuse an inactive program. |
| `program interface get <program_id>` | Read the code-bound interface from the precompile. |
| `program interface publish <program_id>` | Republish the deployed artifact with an interface through upgrade. Requires `--interface`, `--artifact`, `--evm-key-file`, `--idempotency-key`; optional `--fee-limit`, default `0`. The artifact hash must match deployed code. |
| `program build` | Build and inspect WASM locally. `--manifest-path` defaults to `Cargo.toml`; optional `--artifact`. A project toolchain descriptor selects its build and any declared determinism lint; otherwise use Cargo. |
| `program bindings` | Verify a deployment proof and generate interface bindings. Requires `--interface`, `--digest`, `--code-hash`, `--deployment-proof`, and `--trust-history`. `--output` defaults to `bindings`. `--historical` verifies historical evidence without claiming current-head freshness. |
| `program deploy <artifact>` | Inspect the artifact and submit deployment. Requires lifecycle inputs below; optional `--upgrade-authority` and `--interface`. |
| `program upgrade <artifact>` | Require `--old-hash` and lifecycle inputs; optional `--migration-hook`, `--interface`, `--clear-interface`, `--allow-breaking-interface`. An interface and clearing it conflict; allowing a breaking interface requires `--interface`. |
| `program wind-down route` | Require `--account`, `--asset`, `--destination`, and lifecycle inputs; optional `--seed`, default empty. |
| `program wind-down deprecate` | Require `--exit-program`, `--deadline-batch`, and lifecycle inputs. |
| `program wind-down tombstone` | Require lifecycle inputs. |
| `program wind-down exit` | Require `--account` and lifecycle inputs. |
| `program call <program_id>` | Require `--fuel` and, at runtime, `--evm-key-file`. Optional `--calldata`, `--fee-limit` (default `0`), repeatable `--capability`, and native ABI/resource options. Return the submitted transaction hash and rung. |
| `program simulate <program_id>` | Encode the same call and run `eth_call` at `pending`; require `--fuel`, optional `--from <EVM-address>`. Decode an outcome without submitting a transaction. |
| `program registry list` | Scan deployment event logs in bounded windows and report the resulting history coverage. |
| `program registry get <program_id>` | Read the program's precompile registry record. |
| `program registry mirror-source` | Require `--program-id`, `--source-uri`, `--plan`, `--archive`, plus the registry mirror environment inputs below. Mirror source material without claiming on-chain source verification. |
| `program registry verify-source <program_id>` | Accept `--source-uri`, `--source-digest`, `--idempotency-key`, then refuse: the contract record does not expose a source digest. |
| `chains list`, `chains show <id_or_name>` | Read the chain registry's tenant-domain records. |
| `chains create` | Require `--name`, `--tier` (`starter`, `growth`, `pro`), `--fee-share-bps` (`0–5000`), and `--evm-key-file` or `LAYERX_EVM_KEY_FILE`. Submit registry creation and check its transaction receipt/event. |

Shared program lifecycle inputs are `--program-id` and `--evm-key-file`, with
`--fee-limit` defaulting to `0`. The old account-sequence, validity-window,
and idempotency flags are not lifecycle inputs in the current parser.
Native call defaults include ABI `2`, entrypoint `layerx_call`, and
`--memory-bytes 16777216`; the guest ABI must match the deployed program.
Tenant-domain CLI definitions and registry operations are in
[commands.rs](../../tools/platform/cli/src/commands.rs) and
[chains.rs](../../tools/platform/cli/src/chains.rs).

### Emulator and agent transports

| Command | Behavior and inputs |
| --- | --- |
| `emulator provision` | Generate the local emulator signing seed and publish its trust anchor under the profile directory. `--force` explicitly replaces existing material. |
| `emulator up` | Run the local real-transition emulator. Require `--sequencer-seed-file`; optional `--listen`, `--network-id`, `--protocol-version`, `--time-ms`, repeatable `--prefund`. |
| `install mcp` | Select at least one `--host`: `layerx`, `claude-code`, `claude-desktop`, `cursor`, `vscode`. Use `--daemon-binding` or the default `mcp/binding.json` beside CLI config; optional `--read-only`. Open and authenticate the enrolled agent daemon's binding before installing its tool catalogue. |
| `mcp serve` | Serve the enrolled agent daemon's tools on stdin/stdout; require `--daemon-binding`, optional `--read-only`. |
| `install a2a` | Install a hosted payment-capable A2A runtime; optional `--environment`, `--key`, `--listen` (default `127.0.0.1:9433`), `--well-known`, `--read-only`, `--token-stdin`, `--rotate`, `--source-account`, `--asset`. |
| `a2a serve` | Require `--gateway-credential` and `--authorization-file`; optional `--environment`, `--key`, `--source-account`, `--asset`, `--listen` (same loopback default), `--read-only`. |
| `a2a start`, `a2a stop`, `a2a status` | Manage the installed runtime recorded in `a2a/runtime.json`. |

The agent daemon used by MCP is an off-chain agent runtime. MCP installation
and serving use its authenticated local binding, hold no signing seed or
gateway credential themselves, and require enrollment to have published the
binding document. See [MCP](Mcp.md),
[install/mcp.rs](../../tools/platform/cli/src/install/mcp.rs), and
[mcp.rs](../../tools/platform/cli/src/mcp.rs). A2A uses the hosted gateway and
its own authorization file; installation refuses an emulator environment.

## Credentials

The OS credential store is the default. Keyring service `dev.layerx.cli`
uses entry names `key:<name>`, `token:<environment>`, and `gateway:<alias>`.
Config JSON stores public key/DID metadata and endpoint profiles, not these
secrets. Key creation uses OS randomness; key import and `auth set` read
secrets from stdin with a 16 KiB bound. Tokens must contain visible ASCII.
Gateway credentials use an ID followed by `:lxp_live_` and 64 hexadecimal
digits. See [credential.rs](../../tools/platform/cli/src/credential.rs).

`key create` generates key material without asking you to type a seed;
`key import` deliberately accepts an existing seed on stdin. These are
different operations. `auth status` reports token presence independently of
whether any local signing keys exist.

### Headless credential storage

On a headless Unix host, explicitly select encrypted file storage before
creating keys or storing tokens:

```bash
export LAYERX_CREDENTIAL_STORE=file
read -r -s -p 'Credential passphrase: ' LAYERX_CREDENTIAL_PASSPHRASE
export LAYERX_CREDENTIAL_PASSPHRASE
layerx --json key create quickstart
layerx --json key list
layerx --json auth status
```

Use a strong passphrase of `12–16384` bytes. CI should supply
`LAYERX_CREDENTIAL_PASSPHRASE` through its secret environment. Keep the
passphrase available for subsequent processes using this store and unset it
when finished. Seed/token imports still read stdin independently.

The vault is `credentials/vault` beside the resolved CLI configuration file:
`LAYERX_CONFIG`, otherwise `$XDG_CONFIG_HOME/layerx/config.json`, otherwise
`$HOME/.config/layerx/config.json`. It encrypts keys, tokens, and gateway
credentials with AES-256-GCM and PBKDF2-HMAC-SHA256 using 600,000 iterations,
a fresh 16-byte salt, and a fresh 12-byte nonce on each update. Vault and lock
files use mode `0600` in a `0700` directory. Unsafe permissions, symlinks,
incorrect ownership, bad passphrases, and damaged vaults are refused. Updates
use a process lock and atomic replacement. This backend requires Unix
owner-only permissions; see [file_store.rs](../../tools/platform/cli/src/file_store.rs).

There is no automatic migration between stores. Unset
`LAYERX_CREDENTIAL_STORE` or set it to `os` to select the OS store. Back up the
passphrase and encrypted vault through your normal secret backup procedure;
a lost passphrase cannot be recovered. Changing the environment passphrase
does not rotate the vault password.

The `mock` credential store is available only in binaries built with
`test-credential-store`. A production binary refuses that override. This is
independent of the explicitly supported `file` and `os` choices.

## Environment variables

| Variable | Effect |
| --- | --- |
| `LAYERX_CONFIG` | Absolute or CWD-relative CLI configuration JSON path. |
| `XDG_CONFIG_HOME`, `HOME` | Default configuration location as described above. |
| `LAYERX_CREDENTIAL_STORE` | Select `file` or `os`; `mock` requires the test feature. |
| `LAYERX_CREDENTIAL_PASSPHRASE` | Required passphrase for the encrypted file store. |
| `LAYERX_GATEWAY_KEY_ID` | Non-secret stored gateway ID checked by the A2A runtime. |
| `LAYERX_INSTALL_ROOT` | Override the home root used to discover agent host configuration. |
| `LAYERX_REPO_ROOT` | Repository root for workspace commands. |
| `LAYERX_PROGRAM_SDK` | Rust program SDK path used by scaffolding. |
| `LAYERX_EVM_KEY_FILE` | Fallback key file for `chains create`. |
| `LAYERX_BETA_REGISTRY_URL` | Source mirror endpoint for `program registry mirror-source`. |
| `LAYERX_BETA_REGISTRY_PUBLICATION_TOKEN_FILE` | File containing the source mirror bearer token. |
| `LAYERX_BETA_REGISTRY_PUBLICATION_KEY_FILE` | File containing the source mirror publication key. |
| `LAYERX_CLIENT_CERTIFICATE_FILE`, `LAYERX_CLIENT_PRIVATE_KEY_FILE` | Paired client identity files for HTTPS mutual TLS. |
| `CARGO` | Cargo executable used by the default Rust program build path. |

Workspace children receive `LAYERX_ENVIRONMENT`, `LAYERX_ENDPOINT`, and
`LAYERX_NETWORK_ID`. Managed A2A startup allows only `LAYERX_CONFIG` and
`LAYERX_GATEWAY_KEY_ID` in its installed environment map and removes
`LAYERX_TOKEN`, `LAYERX_API_TOKEN`, and `LAYERX_AUTH_TOKEN` from the child.
Those token variables do not replace `layerx auth set`.

With no config file, [config.rs](../../tools/platform/cli/src/config.rs)
selects `emulator`, endpoint `http://127.0.0.1:9402`, network ID `402`, and no
trust anchor. Config version is `1`. This local default is not the public
network's chain ID. The retained `sequencer_trust_anchor` config field and
similarly named environment flags identify signing trust; they do not
configure another production network service.

## Emulator, hosted gateway, and node

The public JSON-RPC endpoint is the root URL of a mainnet-beta node:
`https://api-mainnet-beta.paxeer.network` or
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. For example, this request should
return chain ID `0x7d`:

```bash
curl -sS https://api1.mainnet-beta.paxeer.network \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Node root-path handling is in [rpcstack.go](../../rpc/rpcstack.go).
The public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test`,
and `txpool`. The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in [the gateway OpenRPC contract](../../services/platform/hosted/gateway/openrpc.json).
The retired router and `/rpc`, `/rpc/ws`, and `/rpc/schema` paths are not public node endpoints.

[rpc.rs](../../tools/platform/cli/src/rpc.rs) requires a node URL at the root
path and refuses any other path. The program and chain helpers and A2A
submission post to `/`, and
[the subscription transport](../../tools/platform/cli/src/rpc_subscription.rs)
upgrades the configured URL itself. Registration and faucet take an explicit
hosted service URL (`--gateway-endpoint`, `--service-endpoint`) and post to
exactly that URL; a public node URL does not supply those hosted services.

The active environment remains a REST base URL for `account` and
`payment test`. Emulator account creation uses
`/__emulator/accounts/prefund`; hosted creation uses `/v1/accounts` and
profile lookup uses `/v1/profile`. Payments use `/v1/moves/quote` then `/v1/moves`.
These routes do not turn a node's JSON-RPC endpoint into a hosted REST API.
See [account.rs](../../tools/platform/cli/src/account.rs) and
[payment.rs](../../tools/platform/cli/src/payment.rs).

The HTTP client permits plaintext HTTP only on loopback; remote environments
must use HTTPS. Stored identity sessions use `Authorization: Bearer`, while
gateway credentials use `Authorization: LayerX-Key`. A missing optional token
can result in an unauthenticated request; faucet explicitly requires a bearer
session. See [http.rs](../../tools/platform/cli/src/http.rs).

The emulator runs in-process through `layerx_platform_emulator::run` and
executes real local transitions. Profile binding checks its advertised
network ID and signing key through `GET /v1/sequencer`. Emulator provisioned
keys, its default network `402`, and prefunding are local development
material, not public network configuration.

## Refusals

These describe current handler behavior; backend availability must also be
checked against the selected endpoint's contract.

| Refusal | Meaning |
| --- | --- |
| `wallet_registration_unavailable` | `wallet create` outside emulator mode; no new key was generated. |
| `wallet_history_unavailable` | No authenticated DID activity-history backend is published for the compatibility command. |
| `wallet_accounts_unavailable` | Account enumeration lacks the required DID binding, bounded records, canonical values, or state proof material. |
| `identity_session_required` | Faucet requires a stored bearer session. |
| `rpc_transport_required` | A subscription needs explicit RPC transport; HTTP calls cannot perform a WebSocket subscription. |
| `gateway_credential_required` | The subscription client requires a stored gateway credential. |
| `rpc_method_unavailable` | The CLI method validator refuses the method, including switched-off `lx_` calls. Asset reads also use this prefix for non-remote SDK read errors. |
| `fee_limit_exceeded` | The allowed fee cannot cover gas or the estimate exceeds the bound. |
| `transaction_hash_mismatch` | A wallet submission returned a hash different from the signed transaction's hash. |
| `transaction_pending` | The wallet timeout expired before the requested rung. Retain the hash and query it again. |
| `receipt_verification_failed` | Signed execution material, hash/nonce, logs, or committed block evidence disagrees. |
| `transaction_reverted` | A verified transaction receipt records failed execution. |
| `network_id_mismatch`, `sequencer_trust_anchor_mismatch` | Emulator identity disagrees with the explicitly bound profile. |
| `environment_input_missing`, `network_id_reserved` | Partial environment binding inputs, or reserved network ID `0`. |
| `sequencer_seed_exists`, `sequencer_trust_anchor_exists` | Emulator provisioning refuses to overwrite existing material without `--force`. |
| `command_failed` | Prose errors, including missing keys, unusable credential storage, rejected endpoint syntax, absent MCP binding, source verification unavailability, or unsupported finality trust. Read `error.detail`. |

The generic CLI RPC validator is intentionally bounded to the methods in
its match table; it is not a general-purpose node RPC client. For example,
`wallet derive --bind` currently attempts `eth_chainId`, which that validator
does not admit. The watch parser's `receipts` and `checkpoints` choices also
have no supported transport mapping. Neither should be presented as a
working public-network journey.

## Receipt verification before success

The node's transaction progression is `submitted` → `receipted` → `committed`.
The signed receipt records execution; committed block inclusion establishes
network finality. `pax_sequencerReceipt` is the retained method name for this
network evidence, not evidence of a separate execution or settlement chain.
See [sequencer_receipt.go](../../rpc/sequencer_receipt.go).

The wallet keeps compatibility option names: `--wait executed` requests the
`receipted` rung, while `--wait batched` requests `committed`. Its default is
`executed`, so a successful default wallet command does not claim committed
block inclusion. `--wait finalised` is parsed but refused by the EVM wallet
path with missing finality trust. It is not an extra network finality stage.

Wallet writes and `wallet receipt` require an independently pinned policy
with `protocol_version: 2`, `network_id: 125`, 32-byte hex `sequencer_id` and
`sequencer_key`, and positive bounded `first_batch`/`last_batch` heights.
`checkpoint_context_digest` must be absent or null in this EVM policy. The
field names are retained wire names; the bounds scope the trusted receipt
signing key. Obtain actual trust material independently of the receipt
being checked; do not copy untrusted response values into a policy.

The wallet checks the receipt signature and signed logs, transaction hash,
nonce, execution fields, and return-data commitments. For `batched`, it also
checks the reported committed block against `eth_getTransactionReceipt`.
Notifications are explicitly unverified. A `submitted` hash or a live event
is not a committed receipt. After timeout, look up the existing hash rather
than submit another transfer.

`receipt get` fetches material without independently verifying it. Program
calls return `{tx_hash, rung: "submitted"}` after checking the endpoint's hash
against the locally signed transaction. Lifecycle writes additionally fetch
transaction status and check its hash and rung; that handler does not run the
old local program-outcome proof verifier. Simulation decodes a pending
`eth_call` outcome and submits no transaction. These outputs must not be
read as automatic cryptographic proof of committed success.

`receipt verify` is a separate local verifier for canonical receipt bytes
and caller-supplied batch facts. It accepts raw bytes or unprefixed hex text,
constructs `AuthorizedBatch`, and calls `layerx_proof::receipt::verify_outcome`.
It returns `verified`, verification level, digest, activity/batch IDs, result
code, and canonical byte length. A verified failure receipt still carries
its nonzero result code. This helper does not obtain consensus trust or
block inclusion from the network, and its old batch-fact inputs are not a
replacement for current transaction commitment verification.
See [receipt.rs](../../tools/platform/cli/src/receipt.rs) and
[Portable receipt verifier](PortableVerifier.md).

`payment test` reports its REST quote/commit journey and does not invoke
that local verifier. Generated program bindings have their own deployment
proof verification against supplied chain trust history; historical mode
does not claim a current head.

## Test suite

The suite lives under [tools/platform/cli/tests](../../tools/platform/cli/tests).
Its shared `Cli` fixture launches the real binary with isolated config and
repository paths. The emulator fixture starts a local emulator process.
Credential fixtures use private XDG directories, `dbus-daemon`, and
`gnome-keyring-daemon`; credential-dependent tests check D-Bus availability.
`LAYERX_CLI_TESTS_REQUIRE_DBUS` makes a missing D-Bus installation mandatory
rather than an optional test prerequisite. See
[the harness](../../tools/platform/cli/tests/common/mod.rs) and
[credential environment](../../tools/platform/cli/tests/common/credential_environment.rs).

| Test files or repository target | Coverage |
| --- | --- |
| `commands.rs`, `credential.rs`, `file_store.rs` | Command envelopes, key/token handling, and encrypted vault behavior. |
| `emulator.rs`, `faucet_route.rs` | Local emulator journeys and the faucet RPC method contract. |
| `wallet_commands.rs`, `wallet_derive.rs`, `wallet_encoding.rs`, `wallet_send.rs` | Wallet commands, derivation, native encoding, and debit signing. |
| `wallet_evm_writes.rs`, `wallet_px_reads.rs` | EVM write/receipt handling and current hosted account/asset read requests. |
| `wallet_estimate_fee.rs`, `wallet_watch_logs.rs` | Fee estimation and account-log subscriptions. |
| `program_evm_calls.rs`, `program_evm_lifecycle.rs`, `program_evm_registry.rs`, `chains_commands.rs` | Program EVM dispatch, lifecycle, registry, and tenant-domain CLI operations. |
| `mcp_daemon_bound.rs`, `install.rs`, `workspace.rs` | Agent binding, installation refusals, and workspace selection. |
| `make platform-test-cli-production-credential-refusal` | Production-shaped build refuses the test-only credential override. |
| `make platform-test-tooling` | CLI/emulator tests, supporting tooling checks, CLI build, and clean bootstrap. |
| `make platform-test` | Platform, platform tools, and SDK generator workspace suites. |
| `make platform-test-agent-install` | Hosted agent installation journey with its external prerequisites. |

The make targets are defined in
[services/platform/Makefile.inc](../../services/platform/Makefile.inc), including
runtime-clock and public-TLS prerequisites where needed. This inventory is
not a claim that these suites ran for the documentation rewrite or that
legacy transport assumptions have been repaired.

[Home](Home.md)
