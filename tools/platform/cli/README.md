# `layerx` developer CLI

`tools/platform/cli` builds the `layerx` binary (package `layerx-platform-cli` in the
[`tools/platform`](../Cargo.toml) Cargo workspace). It provides developer tools for
Paxeer X Network, including native wallets, tokens, programs and agent transports.
The kernel is part of the network and proposes blocks on the same BFT chain;
a transaction is final once its block is committed. Mainnet-beta uses EVM chain
ID **125** (`0x7d`). Sub-chains are tenant domains in the database.

The top-level commands are `chains`, `wallet`, `token`, `new`, `workspace`,
`environment`, `key`, `auth`, `account`, `register`, `faucet`, `payment`, `receipt`,
`program`, `emulator`, `install`, `mcp` and `a2a`
([command declarations](src/main.rs)). The
[payment walkthrough](../../../docs/wiki/PaymentsQuickstart.md) provides related
context; hosted documentation lives at [docs.paxeer.app](https://docs.paxeer.app/).
The command behavior and limitations below follow this checkout's implementation.

`--json` emits a JSON result. The global `--rpc` override and
`--gateway-credential` alias apply to wallet, chains, token, program and
`receipt get` commands. `a2a serve` has its own required
`--gateway-credential` argument. Gateway keys and bearer identity sessions are
different credentials; `auth set` stores the latter for an environment.

## Build

From the repository root:

```bash
cargo build --manifest-path tools/platform/cli/Cargo.toml
```

The default executable is `tools/platform/target/debug/layerx`; put that directory
on your PATH. A configured `CARGO_TARGET_DIR` changes the output location.

## Network connections and current limitations

The public JSON-RPC endpoints are HTTPS URLs at the **root path**, including
`https://api-mainnet-beta.paxeer.network/` and the numbered hosts
`https://api1.mainnet-beta.paxeer.network/` through
`https://api14.mainnet-beta.paxeer.network/`. For example, a direct node read is:

```bash
export RPC_URL=https://api1.mainnet-beta.paxeer.network/
curl --fail-with-body --silent --show-error "$RPC_URL" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The chain ID result is `0x7d`. Node namespaces are `debug`, `echo`, `eth`, `net`,
`pax`, `test` and `txpool`. Hosted `px_` methods belong to
[`services/platform/hosted/gateway`](../../../services/platform/hosted/gateway/openrpc.json);
they are not implied by a public node URL. `node.hyperpaxeer.com` is only the library mirror used by build scripts.

**This checkout still has CLI transport gaps.** The old public `/rpc` and `/rpc/ws` routes are retired; they are not public node endpoints.
[`src/rpc.rs`](src/rpc.rs) still requires that obsolete HTTP suffix, the wallet
write handler repeats that assumption, and SDK-backed wallet reads use the
SDK's gateway connection rather than its direct-node connection. Consequently,
setting `--rpc` to a correct public root URL does not make these commands work
against a public node. The CLI's bind and program paths also request methods
that its local RPC allowlist does not admit. Changing the public URL to satisfy
the old client validation does not fix the network connection.

The RPC examples below document parsed command inputs for an operator-provided
compatible deployment. `CLI_RPC_URL` denotes that deployment's explicitly
configured transport, and `beta` denotes an already provisioned gateway-key
alias. These examples are not a working public-node onboarding sequence.
Wallet reads and writes additionally require compatible hosted account snapshots;
the CLI requires a DID-bound `state_proven` account list, which is not the
hosted gateway's `account`/`height`/`balances` response shape. Such mismatches
produce errors, rather than a verified balance or successful write.

## Wallet quickstart

Local wallet creation uses the emulator. Imports, listing and offline mnemonic
derivation do not register or fund a public account. Public wallet creation
returns `wallet_registration_unavailable` before generating a key; `layerx
register` creates a hosted identity principal, not a native main account.
DID history returns `wallet_history_unavailable`.

### Create a local wallet

Use the OS keyring or configure the [encrypted file store](#headless-credential-storage)
before creating keys. Create a private profile and provision the emulator:

```bash
umask 077
export LAYERX_CONFIG="$HOME/.config/layerx/config.json"
mkdir -p "$(dirname "$LAYERX_CONFIG")"
layerx emulator provision
layerx emulator up --sequencer-seed-file "$HOME/.config/layerx/emulator/sequencer.seed"
```

The emulator listens on loopback port 9402 with local network ID 402 by default
and refuses non-loopback listen addresses. It is a local development harness;
402 is not the mainnet-beta chain ID. Its `sequencer`-named flags and files
configure the harness's signing identity.

In another terminal with the same configuration and credential-store settings:

```bash
layerx environment use emulator --endpoint http://localhost:9402 \
  --network-id 402 \
  --sequencer-trust-anchor-file "$HOME/.config/layerx/emulator/sequencer.anchor"
layerx wallet create alice
layerx wallet list
layerx wallet balance
```

`--endpoint`, `--network-id` and a trust anchor must be supplied together when
configuring an environment. The emulator selection checks its signing identity
against the supplied anchor. Non-loopback HTTP endpoints require HTTPS.

Creation registers the local DID and opens its main account with zero units.
If registration fails after key creation, the key is retained; retry using the
same wallet name. `layerx wallet import NAME` reads a 32-byte hexadecimal
Ed25519 seed from stdin. It does not register or fund an identity.
`account create/get` and `payment test` use the environment's REST service;
`payment test` quotes at `/v1/moves/quote` and commits at `/v1/moves` with the
caller's idempotency key. These are not Ethereum node JSON-RPC methods.

### Beta funds

`layerx faucet [--key NAME]` requests the hosted `px_requestFunds` method with
the selected key's DID and Ed25519 public key. It requires a bearer identity
session stored with `layerx auth set`, which reads the token from stdin. Missing
sessions return `identity_session_required`. Global `--rpc` and
`--gateway-credential` overrides do not apply to this command.

The faucet handler still constructs the retired HTTP route described above;
it does not provide a public-node funding path. Availability and eligibility
are determined by the hosted service, not by the existence of the CLI command.
The decoder requires `funded: true`, a nonempty hexadecimal `funding_id`, a
nonempty `transaction_id`, a positive canonical decimal `amount` and a nonempty
`network`. It exposes `transaction_id` as `txHash`; a refusal or incomplete
response exits nonzero ([implementation](src/faucet.rs)).

### Prepare signing keys and receipt trust

Wallet and token writes require both the selected wallet's Ed25519 seed and a
matching EVM private-key export. Offline derivation reads a BIP-39 phrase from
stdin or `--mnemonic-file`; an optional BIP-39 passphrase uses
`--passphrase-file`. It prints public metadata and writes secrets only when
`--export-private-keys` is supplied. The export must be a new file; Unix mode
0600 is used.

For a new wallet alias, derive and import the matching identity without printing
its secret to the terminal:

```bash
umask 077
export EVM_KEY_FILE="$HOME/.config/layerx/payment-keys.json"
layerx wallet derive --mnemonic-file "$MNEMONIC_FILE" \
  --export-private-keys "$EVM_KEY_FILE"
WALLET_DID="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["did"])' "$EVM_KEY_FILE")"
python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["layerx_seed"])' "$EVM_KEY_FILE" \
  | layerx wallet import payments --did "$WALLET_DID"
layerx key default payments
```

`MNEMONIC_FILE` must name your existing private phrase file. Derivation and import
alone do not create an on-chain binding or asset account. The optional `--bind`
path needs RPC and is subject to the transport and allowlist limits above.
The write handler rejects an export that is not an owner-only regular file or
whose DID and derived Ed25519 public key do not match the selected wallet
([derivation](src/wallet_derive.rs), [key validation](src/wallet_signing.rs)).

Obtain a receipt policy from an independently trusted operator; never copy trust
pins from the response being verified. The wallet transaction-policy parser
requires these JSON fields:

| Field | Required value |
| --- | --- |
| `protocol_version` | `2` |
| `network_id` | `125` |
| `sequencer_id` | Trusted key ID, 64 hexadecimal characters |
| `sequencer_key` | Trusted signing public key, 64 hexadecimal characters |
| `first_batch` | Positive first authorized height |
| `last_batch` | Inclusive last authorized height, at least `first_batch` |
| `checkpoint_context_digest` | `null`, or omitted |

The `sequencer` and `batch` names are the parser's field names for receipt-key
authorization and height bounds. They do not introduce another network or
settlement step. Unknown fields, protocol 3 policies and non-null checkpoint
digests are rejected by the transaction-policy parser.
Set `RECEIPT_POLICY` to this file and `FEE_LIMIT` to the maximum transaction fee
in integer base units.

### Send and create a token

The following are command-shape examples under the connection prerequisites
above. `ASSET_ID` is a 64-character hexadecimal asset ID. `RECIPIENT_ACCOUNT`
is an existing 64-character hexadecimal native account ID, not an EVM address.
A recipient DID is also accepted, but requires successful hosted account
resolution for that asset. Amounts are integer base units.

```bash
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta token create \
  --key payments --symbol PAY --name 'Payment Token' \
  --decimals 6 --supply-cap 1000000000 --salt "$(openssl rand -hex 32)" \
  --evm-key-file "$EVM_KEY_FILE" --receipt-policy "$RECEIPT_POLICY" \
  --fee-limit "$FEE_LIMIT" --wait batched
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta wallet open-account \
  --key payments --asset "$ASSET_ID" --evm-key-file "$EVM_KEY_FILE" \
  --receipt-policy "$RECEIPT_POLICY" --fee-limit "$FEE_LIMIT" --wait batched
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta token mint \
  --key payments --asset "$ASSET_ID" --to "$RECIPIENT_ACCOUNT" --amount 100 \
  --evm-key-file "$EVM_KEY_FILE" --receipt-policy "$RECEIPT_POLICY" \
  --fee-limit "$FEE_LIMIT" --wait batched
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta token burn \
  --key payments --asset "$ASSET_ID" --amount 1 \
  --evm-key-file "$EVM_KEY_FILE" --receipt-policy "$RECEIPT_POLICY" \
  --fee-limit "$FEE_LIMIT" --wait batched
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta wallet send \
  --key payments --to "$RECIPIENT_ACCOUNT" --asset "$ASSET_ID" --amount 100 \
  --evm-key-file "$EVM_KEY_FILE" --receipt-policy "$RECEIPT_POLICY" \
  --fee-limit "$FEE_LIMIT" --wait batched
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta token transfer \
  --key payments --to "$RECIPIENT_ACCOUNT" --asset "$ASSET_ID" --amount 10 \
  --evm-key-file "$EVM_KEY_FILE" --receipt-policy "$RECEIPT_POLICY" \
  --fee-limit "$FEE_LIMIT" --wait batched
```

Token creation discloses the derived asset ID; use it as `ASSET_ID` in subsequent
commands. Each recipient must open its own asset account before receiving units.
Writes build chain-125 EIP-1559 transactions. The sender nonce comes from
`eth_getTransactionCount` at `pending`. Send/transfer separately obtain the
native source account's `next_sequence`, disclose and sign the Ed25519 debit,
then disclose and sign the EVM transaction. Source selection validates the
DID, asset, canonical account name and recomputed account ID from the hosted
snapshot. Missing, incompatible or ambiguous account evidence is an error.

The handler reads `eth_gasPrice`, computes a gas bound from `--fee-limit` and
calls `eth_estimateGas` at `pending`. It rejects fees above the limit, submits
with `eth_sendRawTransaction` and checks the returned hash against the locally
computed hash. Disclosures go to stderr. A submission acknowledgement is not
execution evidence. The returned identifier is a `0x`-prefixed transaction hash.

| CLI `--wait` value | Current behavior |
| --- | --- |
| `executed` (default) | Verifies the signed receipt and bound execution/log fields; returns rung `receipted` |
| `batched` | Also matches the committed block hash, number, status and transaction index to `eth_getTransactionReceipt`; returns rung `committed` |
| `finalised` | Returns `MissingFinalityTrust`; this legacy option is unsupported |

Use `batched` to request committed-block evidence. Commitment is finality on
this BFT network; it does not require another certificate. Receipt reads use
`pax_sequencerReceipt`. Signature, transaction hash, nonce, log or execution
mismatches fail verification; a reverted transaction exits nonzero.
`--timeout-seconds` on writes accepts 1–300 and defaults to 60. A timeout returns
`transaction_pending` with the transaction hash. Reconcile that hash before
retrying: a fresh send invocation generates a new native idempotency key.

Retrieve and verify the result later:

```bash
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta wallet receipt "$TX_HASH" \
  --receipt-policy "$RECEIPT_POLICY" --wait batched
```

This command has a fixed 60-second wait budget. `receipt get "$TX_HASH"` fetches
receipt material; use `wallet receipt` with independently pinned trust for the
verification described above.

### Fee estimates and live notifications

`wallet estimate-fee --from "$FROM_ADDRESS" "$UNSIGNED_TX_HEX"` decodes an
unsigned type-2 transaction with nine RLP fields, estimates gas at `pending`
and multiplies it by `eth_gasPrice`. It is subject to the CLI transport limits.
`FROM_ADDRESS` is a `0x`-prefixed 20-byte EVM address.

The implemented watch selector is `wallet watch account --account-id
"$WATCH_ADDRESS"`, where `WATCH_ADDRESS` is an EVM log-emitting address, not a
native account ID. It maps to `eth_subscribe` with a `logs` address filter and
requires a stored gateway credential. Although clap still lists `receipts` and
`checkpoints`, the subscription handler rejects those selectors. Its WebSocket
URL construction also retains the retired route assumption; it is not a
public-node WebSocket configuration guide.

A watch returns one notification with `verified: false` and exits. Its timeout
accepts 1–300 seconds and defaults to 60. A notification establishes no
commitment. After timeout, closure or feed loss, reconcile receipt and chain
reads before reconnecting ([subscription implementation](src/rpc_subscription.rs)).

### Public JSON-RPC reads

Use direct root-node requests for standard Ethereum reads. The CLI's native
wallet and token reads require hosted methods and the compatible transport
and response shapes described above:

```bash
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta wallet balance --did "$WALLET_DID"
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta wallet balance \
  --did "$WALLET_DID" --asset "$ASSET_ID"
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta token info "$ASSET_ID"
layerx --rpc "$CLI_RPC_URL" --gateway-credential beta token list --limit 64
```

Balance uses `px_getBalances`; token listing uses `px_listAssets`. Token info
searches asset-list pages for the requested asset. `token list` accepts an
optional `--cursor` and a `--limit` of 1–256; the default page size is 64.
Asset metadata is unverified read data. Unknown methods, malformed responses
and unavailable account evidence exit nonzero. Retired `lx_` methods are
explicitly refused by the CLI RPC allowlist.

## MCP payment surface

The [MCP catalogue](../../../services/agent/crates/layerx-mcp/src/catalogue.rs)
includes `wallet.send`, `token.create`, `token.mint` and `token.transfer`.
`layerx install mcp --daemon-binding FILE` registers an agent-bound MCP client;
`layerx mcp serve --daemon-binding FILE` serves it over stdin/stdout by delegating
to the enrolled agent's authenticated session. This is an off-chain agent
service, not another chain executor. The binding and deployment mode determine
the served catalogue; `--read-only` narrows it. CLI burn, open-account, token info
and token list are not exposed under those names by this catalogue.

A2A uses a different tool surface. Its `activity.submit` accepts a signed
EIP-1559 `raw_transaction` and returns a transaction hash with rung `submitted`;
it does not accept the CLI wallet transfer arguments or establish commitment.
Use a subsequent receipt lookup to reconcile the result
([A2A tool implementation](src/toolset.rs)).

## Credential integration tests

The real OS credential integration tests require `dbus-daemon`, `dbus-send` and
`gnome-keyring-daemon`. When `dbus-daemon` is absent they print
`skipped: dbus-daemon is not installed` and return early. Set
`LAYERX_CLI_TESTS_REQUIRE_DBUS` to any value, including an empty value, to require
the prerequisite and fail when it is missing.

## TLS integration test input

`public_http_uses_system_trust_and_checks_server_identity` requires the real
`layerx-paxeer-boundary` binary from `services/platform/Cargo.toml`. Supply its
path through `LAYERX_PAXEER_BOUNDARY_BIN`, or place it in the Cargo target's
`debug/` directory beside `deps/` (normally `tools/platform/target/debug/`).
The [shared test support](../../../services/platform/tests/support/tls_boundary.rs)
uses OpenSSL-generated certificates to check system trust and server identity.

## Headless credential storage

The OS keyring is the default. On a headless Linux server, container or CI
runner, explicitly select the encrypted file store before creating a key:

```bash
export LAYERX_CREDENTIAL_STORE=file
read -r -s -p 'Credential passphrase: ' LAYERX_CREDENTIAL_PASSPHRASE
export LAYERX_CREDENTIAL_PASSPHRASE
layerx --json key create quickstart
layerx --json key list
layerx --json auth status
```

Use a passphrase of 12–16384 bytes. In CI, supply
`LAYERX_CREDENTIAL_PASSPHRASE` through the CI secret environment. Keep it
available for subsequent CLI and MCP/A2A processes, and unset it when finished.
Secret imports still read stdin; the passphrase does not consume that input.
`auth status` reports whether an API token is stored, independently of keys;
store one with `layerx auth set` using the token on stdin.

The store writes `credentials/vault` beside the resolved CLI config file
(`LAYERX_CONFIG`, otherwise `$XDG_CONFIG_HOME/layerx/config.json`, otherwise
`$HOME/.config/layerx/config.json`). It encrypts its contents using AES-256-GCM
with a key from PBKDF2-HMAC-SHA256 at 600,000 iterations, a fresh 16-byte salt
and a fresh 12-byte nonce on every update. Vault and lock files use mode 0600
inside a mode 0700 directory. Updates hold a file lock and replace the vault
atomically. This backend requires Unix file permissions.

There is no automatic fallback or migration between stores. Unset
`LAYERX_CREDENTIAL_STORE` (or set it to `os`) to use the OS keyring. Retain the
passphrase and encrypted vault together in your backup procedure: a lost
passphrase cannot be recovered. Changing the environment passphrase does not
rotate the vault password; it makes authentication fail. The explicit EVM
private-key export is a separate plaintext secret file, not part of this vault.
