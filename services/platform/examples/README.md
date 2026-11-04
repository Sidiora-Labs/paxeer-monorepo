# Reference applications

Node.js reference applications for Paxeer X Network. The network contains the
kernel and uses EVM chain ID **125** (`0x7d`). Transactions execute in the node;
receipts progress through `submitted`, `receipted` and `committed`. Block commit
is finality. Tenant domains are database namespaces within this network.

## Endpoints and prerequisites

Beta profiles use public node JSON-RPC at
`https://api-mainnet-beta.paxeer.network/`. The other public names are
`https://api1.mainnet-beta.paxeer.network/` through
`https://api14.mainnet-beta.paxeer.network/`, also at the root path. Nodes and
hosted application services have different APIs. `px_*` methods belong to the
[hosted gateway](../hosted/gateway/README.md); human payments and merchant
settlement additionally require operator-deployed services and credentials.
Mainnet-beta carries real value; an environment label does not imply free funds
or a reset schedule.

Each `layerx.example.json` declares public configuration and the environment
variable names for account-specific inputs. These are server-side Node.js
applications. Keep credentials and signing material outside profile JSON.

Install the locked Node workspace from the repository root with `npm ci` and
compile the SDK and middleware with `npm run build`. The four projects declared
in [reference-apps.json](reference-apps.json) provide these launch commands:

| Application | Emulator profile | Beta profile |
| --- | --- | --- |
| [Buyer agent](buyer-agent/) | `npm run start:emulator --workspace @sidiora/layerx-example-buyer-agent` | `npm run start:beta --workspace @sidiora/layerx-example-buyer-agent` |
| [Paid API](paid-api/) | `npm run start:emulator --workspace @sidiora/layerx-example-paid-api` | `npm run start:beta --workspace @sidiora/layerx-example-paid-api` |
| [Merchant shop](merchant-shop/) | `npm run start:emulator --workspace @sidiora/layerx-example-merchant-shop` | `npm run start:beta --workspace @sidiora/layerx-example-merchant-shop` |
| [Programs marketplace](marketplace/) | `npm run start:emulator --workspace @sidiora/layerx-example-marketplace` | `npm run start:beta --workspace @sidiora/layerx-example-marketplace` |

`merchant-checkout` remains a compatibility workspace for the merchant service.
The marketplace has deploy, list and buy commands and a no-std guest under
[marketplace/program](marketplace/program/). Its client checks deployment and
call receipts before reporting success; preserve the actual receipt digest
passed to the purchase step. Build the guest for the network's ABI and chain
domain and keep the full deployment envelope within the payload limit.

## Paid API configuration and verification

[paid-api/index.mjs](paid-api/index.mjs) serves `GET /paid` on loopback port
8081. Its x402 v2 offer uses `scheme: exact`, `network: eip155:125`, the configured
integer amount, asset and `payTo`, and `extra.layerx` fields `commitment:
receipted`, `account` and `currency`. The released body comes from
[paid-api/resource.json](paid-api/resource.json).

For the beta profile, set these inputs to authentic values from the deployment:

| Variable | Meaning |
| --- | --- |
| `LAYERX_TESTNET_ASSET` | Asset identifier accepted by the payment implementation |
| `LAYERX_TESTNET_SELLER` | Advertised payee identifier |
| `LAYERX_TESTNET_SELLER_ACCOUNT` | Human-service destination account reference |
| `LAYERX_TESTNET_CURRENCY` | Currency advertised with that account |
| `LAYERX_TESTNET_PRICE` | Exact integer amount, expressed as a string |
| `LAYERX_RECEIPT_KEYS_JSON` | Nonempty receipt-key array pinned independently of the payment response |

The emulator paid-API profile uses the corresponding `LAYERX_EMULATOR_*`
variables for money fields, but still reads **`LAYERX_RECEIPT_KEYS_JSON`**. It
currently selects JSON-RPC at `http://127.0.0.1:8545/`; the other applications'
emulator profiles select port 9402. Choose compatible service coordinates in an
external profile using `LAYERX_EXAMPLE_CONFIG`; starting the CLI emulator alone
does not supply a native chain RPC at port 8545.

Receipt-key JSON is an array of objects with `keyId`, `publicKey`, `firstHeight`
and `lastHeight`. Identifiers and public keys encode 32 bytes in hexadecimal
(with an optional `0x` prefix); heights accept safe nonnegative JSON integers or
canonical decimal/hexadecimal strings, with `firstHeight <= lastHeight`. Use the
operator's authentic keys and authorised ranges, never keys copied from a
payment's untrusted response.

The paid API computes the transaction hash from the x402 payload's
`rawTransaction` and calls `pax_sequencerReceipt` at its configured `rpcUrl`.
It verifies the signed receipt against the raw transaction and pinned keys,
then returns a `layerx-receipt-proof-v2` proof. It does not resolve a legacy
batch-authority object. Its resolver reports verification level `receipted`
even when the node returns a committed receipt; it does not prove BFT finality.

Fulfillment records bind the request digest and idempotency key and retain the
resource and commitment evidence. Pending returns 202; unknown or unreachable
evidence returns 503; refused evidence prevents release. Reusing an idempotency
key with a different request remains a conflict.

## Buyer, merchant and hosted bindings

The buyer uses its profile's `receiptKeysEnvironment`: beta reads
`LAYERX_TESTNET_RECEIPT_KEYS_JSON`, emulator reads
`LAYERX_EMULATOR_RECEIPT_KEYS_JSON`. Supply matching trust material to the paid
API's separately named variable. The buyer also needs its token, source and
idempotency inputs. Beta `LAYERX_TESTNET_HUMAN_URL` must identify an HTTPS human
service at the origin root, distinct from the node origin; a public node URL
is refused. If you override the beta endpoint, explicitly supply that human URL.

The merchant needs its profile's receipt keys, money fields, webhook public
keys and settlement token. Beta settlement uses
`LAYERX_TESTNET_SETTLEMENT_URL`, pointing at the deployed settlement service.
`LAYERX_EXAMPLE_ENDPOINT` rebases endpoint fields present in beta profiles; it
does not provision services or replace fields resolved through their own
environment variables. `LAYERX_EXAMPLE_STATE_ROOT` moves retained application
state outside the project directories.

See [PaymentsQuickstart](../../../docs/wiki/PaymentsQuickstart.md),
[CommitmentLevels](../../../docs/wiki/CommitmentLevels.md) and the
[receipt authority](../hosted/authority/README.md) for related contracts.

## Running the whole journey

`node services/platform/examples/run-reference-apps.mjs --check` checks project,
package and profile declarations. `--scenario beta` runs the applications using
the supplied `LAYERX_TESTNET_*` inputs and deployed services. The scenario
attempts buyer payment, merchant checkout, then marketplace deploy/list/buy;
only actual successful outcomes establish that journey.

`--scenario emulator` currently provisions a scratch CLI profile, four signing
keys and a local emulator. It resolves the CLI from `$LAYERX_BIN`,
`build/bin/layerx`, then `PATH`, waits for health, derives account and money
inputs from local state and a seeding transfer, and stops the process and
removes the scratch profile afterward. `LAYERX_EMULATOR_SEED_FILE` selects an
existing seed and its adjacent trust anchor. Credential storage defaults to a
per-run file store and passphrase unless overridden.

That bootstrap still uses local quote/commit, state and receipt routes. Its
provided receipt-key variable is `LAYERX_EMULATOR_RECEIPT_KEYS_JSON`, while the
paid API reads `LAYERX_RECEIPT_KEYS_JSON`; its chosen endpoint also differs from
the paid API's native RPC profile. Those prerequisites must agree before this
scenario can establish a native payment journey. The pre-merge `/rpc` paths retained in emulator profiles are retired network coordinates.
Do not infer native compatibility from declaration checks or emulator health.

## Building and checking against real receipts

On a clean checkout, install dependencies with `npm ci`. Set
`PAXEER_X_REFERENCE_APP_ARTIFACTS` to an absolute manifest filename in an
owner-only directory outside the checkout, then run:

```sh
node services/platform/examples/build-reference-apps.mjs
```

The script compiles the SDK and three middleware packages, builds the CLI and
marketplace guest, and records the source revision and artifact hashes. It
requires the source tree to remain clean.

Set `PAXEER_X_EVIDENCE_DIR` to an existing owner-only external directory, then run:

```sh
node services/platform/examples/qualify-reference-apps.mjs
```

This consumes the prebuilt artifacts and checks actual paid-resource, merchant
and marketplace outcomes, retaining private process logs and state. It defaults
to emulator; `PAXEER_X_REFERENCE_APPS_ENVIRONMENT=beta` selects deployed services.
The prerequisites above still apply. Missing artifacts, authentic receipt keys,
or successful receipts fail qualification; these commands do not create
production credentials or prove a higher commitment than their verifiers check.
