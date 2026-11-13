<!-- Source: https://docs.paxeer.app/cli/ -->

# Command-Line Workflows

Operate Paxeer X identities, native assets, payments, deterministic Programs and agent transports with the layerx CLI.

## The Paxeer X developer CLI

The binary is named `layerx`. Use `layerx <command> --help` for the command's current flags. Global `--json` emits one structured object: success contains `ok`, `kind`, `message` and `data`; failure contains `ok: false` and `error.code` with `error.detail`. Use these fields in automation instead of parsing presentation text.

## Bind an environment

```
layerx environment list
layerx environment current
layerx environment use beta   --endpoint https://api-mainnet-beta.paxeer.network   --network-id <native-network-id>   --sequencer-trust-anchor-file <trusted-anchor-file>
```

Profiles are `emulator`, `beta` and `production`. Configure endpoint, network ID and sequencer trust anchor together when binding a new profile. Reusing a configured profile needs only its name. Non-loopback HTTP endpoints are refused; use HTTPS. The native network ID is distinct from EVM chain ID 125.

### Configuration and credential storage

`LAYERX_CONFIG` selects a config file. Otherwise the CLI uses `$XDG_CONFIG_HOME/layerx/config.json` or `$HOME/.config/layerx/config.json`. Public metadata and endpoints live in config; keys, hosted tokens and gateway credentials live in credential storage. The OS keyring is the default.

```
# Headless Linux: select the encrypted file store before key creation.
export LAYERX_CREDENTIAL_STORE=file
read -r -s -p 'Credential passphrase: ' LAYERX_CREDENTIAL_PASSPHRASE
export LAYERX_CREDENTIAL_PASSPHRASE
layerx --json key create operator
layerx key default operator
layerx --json key list
# auth set reads the session token from standard input.
layerx auth set --environment beta
layerx --json auth status --environment beta
```

The file store requires a 12–16,384 byte passphrase and encrypts the vault beside the configuration using AES-256-GCM and PBKDF2-HMAC-SHA256. Preserve the vault and its passphrase securely. Store selection has no automatic fallback or migration. Seeds and tokens supplied to import commands are read from stdin and never belong in shell arguments or public configuration.

## Register and inspect an identity

```
layerx key show operator
layerx register --key operator
layerx account get
layerx wallet list
layerx wallet balance
```

Registration signs the tenant binding and accepts a response only when its tenant, subject and signer key match. An imported key establishes local custody; registration and funding are separate steps. `layerx faucet --key operator` requests an approved bounded beta allocation using the stored identity session.

## Virtual chains

```
layerx chains list
layerx chains show <id-or-name>
layerx chains create --name <name> --tier <tier> --fee-share-bps <0-5000> --evm-key-file <path>
```

`layerx chains` lists, inspects and creates virtual chains on the chain registry. Like wallet, token, program and `receipt get` commands it accepts the global `--rpc` and `--gateway-credential` flags; the RPC URL must end in `/rpc`. Creation requires an EVM key file (for example from `wallet derive --export-private-keys`) or `LAYERX_EVM_KEY_FILE`.

## Wallets and native assets

| Command | Workflow |
| --- | --- |
| `wallet create <name>`, `wallet import <name>` | Generate a key, register its identity and open its main account (optional `--did`), or import a 32-byte hexadecimal seed from stdin. |
| `wallet derive` | Derive the EVM account and native identity from one BIP-39 phrase read from stdin or a file. |
| `wallet balance`, `wallet list` | Inspect DID accounts and local public wallet metadata. |
| `wallet open-account` | Open a wallet's native account for a specified asset. |
| `wallet send`, `token transfer` | Prepare native transfers with recipient, asset, amount and signing policy. |
| `token create` | Register a native asset with symbol, name, decimals, salt and optional supply cap. |
| `token mint`, `token burn` | Issue into an existing asset account or return units to issuance. |
| `token info <asset_id>`, `token list` | Read metadata and the ascending native asset registry. |
| `wallet estimate-fee`, `wallet watch`, `wallet receipt` | Estimate fees from unsigned EIP-1559 transaction bytes at pending (requires `--from`), receive a notification, and verify receipt evidence for a transaction hash at the requested commitment. |

For public wallet, token and Program RPC operations, global `--rpc` identifies the gateway URL ending in `/rpc`; `--gateway-credential` selects a stored gateway alias. Use independently trusted receipt policy, explicit fee limits and current sequences for monetary writes. Native asset amounts are integer units rather than floating-point display amounts.

## Quote and commit a payment

```
layerx --json payment test   --from <source-account>   --to <destination-account>   --currency <asset-id>   --amount 1000   --idempotency-key treasury-payment-0001
```

This workflow requests a quote through `/v1/moves/quote` and commits through `/v1/moves`. Keep the key stable for retries. The payment command returns the quote and commit journey; independently verify receipt bytes before treating the outcome as settled.

## Fetch and verify receipts

```
layerx --json receipt get <0x-transaction-hash>
layerx --json receipt verify   --receipt receipt.hex   --batch-id <batch-id>   --asset <asset-id>   --previous-state-root <previous-root>   --resulting-state-root <resulting-root>   --sequencer-public-key <trusted-public-key>
```

`receipt get` fetches a sequencer receipt by its 0x-prefixed transaction hash. Supply the receipt file as hex text or raw bytes and obtain batch facts through an independently trusted source. Verification is local and requires no node or gateway. The command reports `verified: true` only after checking the canonical outcome. A timeout or pending response retains the original transaction hash: look it up again rather than submitting a new payment.

## Build and operate Programs

```
layerx new treasury-program
layerx program build --manifest-path treasury-program/Cargo.toml
layerx program discover <program-id>
layerx program interface get <program-id>
layerx program registry get <program-id>
```

Build compiles WASM and enforces deterministic runtime policy locally. Discovery binds active code and interface to receipt-backed registry state. Bindings generation checks interface digest and code hash and produces typed SDK and guest bindings; supply verified deployment evidence and trust history according to `layerx program bindings --help`.

| Command family | Operation |
| --- | --- |
| `program deploy`, `program upgrade` | Validate and submit artifacts with lifecycle authorization and receipt verification. |
| `program interface publish` | Publish a canonical interface bound to program code. |
| `program simulate` | Execute against current state with sealed non-commit evidence. |
| `program call` | Sign and submit an EIP-1559 call to a deployed program; render a receipt-verified typed result. |
| `program wind-down route`, `deprecate`, `tombstone`, `exit` | Route owned accounts, define an exit deadline, retire code and exit account positions. |
| `program registry mirror-source`, `verify-source` | Publish build/source evidence and request source verification. |

Lifecycle writes share three arguments: `--program-id`, a required `--evm-key-file`, and `--fee-limit` (default `0`). An idempotency key is accepted only by `program interface publish` and `program registry verify-source`. Calls also bind fuel, ABI, memory and capability requirements. Receipt verification binds the returned activity, sequencer, state root, ABI and terminal payload before rendering typed success. Simulation explicitly establishes `committed: false`.

## Connect agent transports

```
layerx install mcp --host cursor --daemon-binding <binding.json>
layerx mcp serve --daemon-binding <binding.json>
layerx install a2a --environment beta --key operator --source-account <64-hex-source-account> --asset <64-hex-asset-id>
layerx a2a start
layerx a2a status
layerx a2a stop
```

MCP installation reads the binding document produced by agent-daemon enrollment and serves its tool catalogue over stdin/stdout. The transport carries no signing seed or gateway credential. Supported host names are `layerx`, `claude-code`, `claude-desktop`, `cursor` and `vscode`. Use `--read-only` for a read-only MCP installation.

A2A installation targets a hosted environment, provisions a scoped gateway credential in secure storage and serves the agent card and task interface on loopback. The default listener is `127.0.0.1:9433`. Managed start, status and stop use the installed runtime configuration.

## Local development and workspace operations

`layerx emulator provision` creates sequencer seed and trust-anchor files; `layerx emulator up --sequencer-seed-file <path>` runs the local gateway around real protocol transitions. Bind the emulator profile to its advertised identity before use. `layerx workspace` opens the visual workspace on a TTY. Its `modules`, `doctor`, `install`, `build`, `test` and `all` commands support repeatable module operations; use dry-run and explicit module selection for automation.

## Read failures before retrying

Check the JSON error code. Network or sequencer trust mismatches require correcting the profile. Credential failures require the selected store and session. Receipt verification and ABI mismatches require resolving the evidence or interface. Policy and capability refusals require appropriate authorization. Retry uncertain writes only with their original identity, and retain existing activity IDs for reconciliation.

Continue with the [platform API reference](https://docs.paxeer.app/platform-api), [identity model](https://docs.paxeer.app/identity) and [Programs runtime](https://docs.paxeer.app/wasm-runtime).
