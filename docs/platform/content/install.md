# Install the tools

Install the `layerx` CLI and the SDK your application uses, then configure the
service it actually calls. Paxeer mainnet-beta is one BFT network: `paxd`
contains the kernel, which executes transactions and proposes blocks. EVM chain
ID is **125** (`0x7d`); a receipt is final when its block commits. Tenant domains
are database namespaces and do not require another chain deployment.

This page describes source-checkout tooling. Package publication and hosted
service availability must be established by the release or deployment you use;
installing the CLI does not provision a funded mainnet account.

## Install the command line tool

From the repository root:

```sh
cargo install --path tools/platform/cli
```

The binary is `layerx`. The global `--json` flag emits command results as JSON;
use `layerx --help` for the current command catalogue.

## Pick an environment

| Environment | Endpoint and purpose | Money |
| --- | --- | --- |
| Local emulator | Loopback development service, default port 9402 and network ID 402 | Disposable local balances |
| Mainnet-beta node | `https://api-mainnet-beta.paxeer.network/` or `https://api1.mainnet-beta.paxeer.network/` through `https://api14.mainnet-beta.paxeer.network/` | Real value, chain ID 125 |
| Hosted application services | Operator-provided authenticated origins for human payments, agent tools, or settlement | Determined by the configured network |

Public node JSON-RPC uses the root path. Hosted gateway `px_*` methods are
additional service APIs; a node URL does not supply human-payment credentials
or every hosted method. See the [gateway source guide](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/5c65ff650b571b5294048762896e896aa35d11ae/services/platform/hosted/gateway/README.md)
and [reference applications](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/5c65ff650b571b5294048762896e896aa35d11ae/services/platform/examples/README.md)
for the distinct service bindings.

The emulator exercises local protocol transitions. A successful emulator
payment does not establish native transaction submission, BFT commitment,
mainnet funding, or hosted service availability.

### Local bootstrap

`layerx emulator provision` generates an owner-only receipt-signing seed and
trust anchor under the profile directory. It prints their paths and the anchor,
never the seed. Keep the bootstrap sequence in this order:

<!-- layerx:bootstrap-sequence -->
```bash
layerx emulator provision
layerx emulator up --sequencer-seed-file "$HOME/.config/layerx/emulator/sequencer.seed" &
layerx environment use emulator --endpoint http://127.0.0.1:9402 --network-id 402 \
  --sequencer-trust-anchor-file "$HOME/.config/layerx/emulator/sequencer.anchor"
```

Provisioning supports Linux, Android and Apple hosts with atomic owner-only
profile publication. If `XDG_CONFIG_HOME` or `LAYERX_CONFIG` changes the profile
location, use the printed paths. Provisioning refuses to replace existing seed
material without `--force`. Environment selection checks the anchor against
the running emulator before saving the profile.

`layerx environment current` and `layerx environment list` show saved profiles.
Configure a hosted profile with the operator's endpoint and network identity;
use 125 for a profile that submits mainnet EVM transactions.

## Get credentials

Hosted provisioning uses a short-lived identity session read from standard
input and retained in the configured credential store:

```text
layerx auth set --environment beta
layerx auth status --environment beta
```

For local development, these commands create an Ed25519 CLI key and an
emulator account:

```text
layerx key create dev
layerx account create --key dev --initial-amount 1000000
```

That CLI key is distinct from a secp256k1 EVM transaction signer and from the
network's pinned Ed25519 receipt-verification keys.

## Confirm it works before you write code

Against a configured human-payment service, supply its account references,
currency, amount and idempotency key:

```text
layerx payment test --from "$LAYERX_SOURCE" --to "$LAYERX_DESTINATION" \
  --currency "$LAYERX_CURRENCY" --amount "$LAYERX_AMOUNT" \
  --idempotency-key "$LAYERX_PAYMENT_KEY" --json
```

This runs the quote/commit journey exposed by that service. Treat pending,
unknown and refused results distinctly; a submission acknowledgement alone is
not a committed payment.

Native wallet and token commands are documented in the
[CLI wallet quickstart](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/tools/platform/cli/README.md#wallet-quickstart).
Use its signing, receipt-policy and fee-limit requirements for the selected
transport. Emulator and hosted registration paths have different prerequisites.

Wallet and token writes require the selected wallet's Ed25519 seed, a matching
secp256k1 key export supplied with `--evm-key-file`, an independently trusted
`--receipt-policy` and a `--fee-limit` in integer base units. They construct
chain-125 EIP-1559 transactions. Send/transfer first disclose and sign the native
Ed25519 debit, including the source account's `next_sequence`, then disclose
and sign the EVM transaction. Disclosures are written to stderr before signing.
Review the sender, recipient, asset, amount, calldata, chain ID, nonce, gas limit
and fee caps. The EVM nonce and native source-account sequence are distinct.

## Five-minute agent-runtime installation

MCP installation binds to the enrolled agent runtime's local binding document.
It registers a launch line for `layerx mcp serve`, opens the binding and refuses
if it is absent. Credentials belong to that enrolled runtime. Name one or more
supported hosts:

```text
layerx install mcp --host claude-code --json
```

The binding records `core_height`; the retired `core_sequence` field is refused
with `mcp.binding_retired_key`. Installation and serving require
`agent.readiness` to identify the authenticated HTTPS gateway and its protected
credential file. `--daemon-binding <path>` selects a binding explicitly.
`--read-only` narrows the live catalogue without rewriting the binding.
Supported hosts are `layerx`, `claude-code`, `claude-desktop`, `cursor` and `vscode`.

A2A installation selects a hosted environment, an existing CLI key and the
stored identity session used to provision its scoped gateway credential:

```text
layerx install a2a --environment beta --key agent-runtime \
  --listen 127.0.0.1:9433 --json
layerx a2a status --json
```

Create or import `agent-runtime` before installing. Optional `--source-account`
and `--asset` accept 32-byte hexadecimal identifiers only as a pair; these
compatibility bindings do not sign or construct a payment. They are rejected
in read-only mode. Payment submission requires the caller's **already signed
EIP-1559 transaction**, including the correct chain ID, nonce, native payment
calldata, gas limit and fee caps, signed by the authorised EVM account.

Before signing for A2A, review the same payment and EIP-1559 disclosures with
your signer. The A2A runtime receives signed bytes; it does not construct the
payment or sign it from the installed CLI key or compatibility bindings.

The `activity.submit` skill's arguments object contains exactly
`{"raw_transaction":"0x02..."}` (replace the ellipsis with complete signed
transaction bytes). The runtime validates the transaction and active network,
submits with `eth_sendRawTransaction`, and checks that the node's returned hash
matches the locally computed hash of those bytes. It returns `txHash` with
`submitted` rung; this acknowledges submission without proving execution or
commitment. After transport ambiguity it can return `txHash` with `unknown`
rung. Keep that hash and reconcile it before retrying; starting a fresh payment
can create a second payment. `receipt.get` accepts exactly
`{"activity_id":"0x<64 hex digits>"}` and fetches `pax_sequencerReceipt` material.
Set `activity_id` to the same `txHash` from the submission, including after an
`unknown` result. CLI write timeouts likewise report `transaction_pending` with
the hash; follow the maintained guide's `wallet receipt` flow before sending
again, because a fresh send invocation generates a new native idempotency key.
Receipt verification requires independently pinned keys with height ranges;
fetched bytes alone are not a verification or finality claim.

A receipt is final once its block is committed. For independent finality
verification, authenticate the block header and CometBFT commit, then verify
transaction or state inclusion at the relevant height. See
[Committed block evidence](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/abc81fd2cead6b2b9d5612c6e09a6ab56f13c1ed/docs/site/docs/concepts/checkpoints.md#committed-block-evidence).

The installation result names an owner-only `authorization.credential_file`.
Send its value as `Authorization: Bearer <value>` on every loopback JSON-RPC
POST. The Agent Card is readable without that bearer. Installation JSON exposes
credential aliases, identifiers and file paths; secrets remain in credential
storage. Reinstalling is idempotent; `--rotate` replaces the gateway credential
and local bearer before restarting. `--read-only` serves only read tools with
`receipt:read` scope.

## Install an SDK

Build the package matching the API you call from `sdk/platform/*`,
`services/agent/sdk/*` or `services/agent/crates/*` in this checkout until your
release publishes verified package coordinates. Human-payment clients and
native transaction/receipt clients have different configuration requirements;
choose the corresponding language quickstart and package README.

## What your application reads from the environment

Human-payment samples use `LAYERX_API_URL` for the deployed human service and
`LAYERX_API_TOKEN` for its bearer credential. These values do not configure a
native transaction signer or receipt trust policy. The reference applications
use per-profile environment bindings, including receipt keys; follow their
[configuration guide](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/5c65ff650b571b5294048762896e896aa35d11ae/services/platform/examples/README.md).

Mobile and browser clients use a broker for short-lived sessions. See
[iOS](framework/ios.md) and [Android](framework/android.md).

## Enforced by

| Capability | Layer | What that means here |
| --- | --- | --- |
| Native payment execution and block finality | `protocol` | Transactions execute in the network; finality follows BFT block commit. |
| A2A transaction validation and local bearer checks | `service` | The CLI runtime checks signed inputs and authenticates local requests. |
| MCP binding and catalogue access | `agent-layer` | The enrolled runtime supplies credentials and readiness evidence. |
| Hosted credentials and funding eligibility | `hosted-surface` | Deployment policy controls access and allocations; emulator balances confer no mainnet funds. |
