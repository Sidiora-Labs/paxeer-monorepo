# x-websearch

The Paxeer web search sidecar serves `GET /health` and `GET /readyz` for free,
`GET /search` and `GET /fetch` behind the x402 payment gate, and
`GET /content/<digest>` unpaid. It loads one JSON file passed as
`--config <path>`. The loader refuses unknown fields, missing required fields,
placeholders and embedded key material, naming the refused field.

Paxeer is one BFT network: `paxd` includes the kernel that proposes blocks.
Mainnet-beta uses EVM chain id 125 (`0x7d`). Public node JSON-RPC uses the root
path, for example `https://api1.mainnet-beta.paxeer.network/`; choose a node
that provides the methods required below.

## EVM program relay and settlement

The program relay starts when an attestor key is present and requires a
submitter key. It watches `XWebProgramRequested` logs from the xweb precompile
at `0x0000000000000000000000000000000000001019` with `eth_getLogs`, waits for
`evm.confirmations`, answers requests, and collects peer attestor signatures.
Each relay round refreshes the registered attestor set and threshold from
`evm.endpoint`. With a quorum, the submitter signs an EIP-155 transaction
calling `fulfilProgram(bytes)` on that precompile.

Paid HTTP requests advertise only the `exact` scheme, with one offer per
configured asset. The buyer supplies `PAYMENT-SIGNATURE`; an unpaid request
receives HTTP 402 and `PAYMENT-REQUIRED`. Exact settlement identifies a payment
to the ledger precompile at `0x000000000000000000000000000000000000101a`.
The scheme payload carries `txHash`, `receiptDigest` and
`verificationLevel: "receipted"`, with an optional hexadecimal
`idempotencyKey`. The buyer submits the `payExact` transaction; the sidecar
checks its evidence rather than signing a new receiver draw. The gate queries
`pax_sequencerReceipt` through `gateway.endpoint` and verifies the
signed receipt, authorized signer and height range, logs, payee, asset, amount
and payment reference before releasing content. The receipt method's historical
name does not imply another consensus component.

Payer grants and metered draws are retired at the ledger. The gate refuses
unoffered metered payments, unverified or mismatched receipts, and payments
reused for a different request without releasing content. It continues to read
historical payment journals for recovery; restoring retired kernel activity
types is not a supported way to enable payments.

A verified receipt authenticates execution. The websearch gate reports that
evidence as `receipted`; RPC block fields alone do not establish a commit proof.
On the network, a receipt is final once its block is committed. See the
[x402 conformance notes](../../specs/conformance/x402/README.md) for the
distinction between wire validation and authenticated settlement evidence.

| Key | Meaning |
| --- | --- |
| `gateway.endpoint` | Settlement authority shared by the payment gate and readiness. Must serve `eth_chainId`, ledger/account `eth_call` lookups and `pax_sequencerReceipt` for exact payments. |
| `gateway.receipt_keys` | Nonempty authorized receipt signer list. Each entry has a 32-byte hexadecimal `key_id`, an Ed25519 `public_key`, and inclusive integer `first_height` and `last_height` bounds. Ranges for the same key id must not overlap. |
| `payment.payer_address` | Optional `0x`-prefixed, 20-byte EVM payer address. If absent, a request can identify its payer with `LAYERX-PAYER-ADDRESS`. |
| `evm.endpoint` | Node JSON-RPC for network-id lookup, request watching, attestor refresh and fulfilment submission. Payment receipts are read from `gateway.endpoint`. |
| `evm.chain_id` | Chain id bound to transactions and payment offers; 125 for Paxeer mainnet-beta. |
| `evm.confirmations` | Confirmation depth used by the watcher and fulfilment checks. |
| `kernel_network_id` | Optional nonzero kernel network id. Startup reads the xweb precompile and rejects a configured value that differs; this is distinct from the EVM chain id. |
| `peers` | Sidecar base URLs used for content retrieval and attestor exchange. |

Both endpoint fields accept HTTPS; plain HTTP is accepted only for loopback
or `localhost`. Distinct URLs are supported: configure both for the same
network and `evm.chain_id`, with the methods assigned above. The watcher URL
need not serve `pax_sequencerReceipt`; the settlement URL must. An unavailable
receipt method or a wrong-chain settlement authority makes readiness and paid
routes unavailable.

The [interop gateway deployment guide](../../deploy/gateway/README.md)
describes the hosted protocol service's separate configuration schema.

Before loading [config.example.json](config.example.json), replace every
`${NAME}` string with a valid value; the loader does not expand environment
variables. `X_WEBSEARCH_SEED_URL` is the crawl seed URL.
`X_WEBSEARCH_<SID|PAX|USDC|USDL>_ASSET_ID` is a 32-byte hexadecimal asset id,
and `X_WEBSEARCH_<SID|PAX|USDC|USDL>_PRICE` is its positive decimal price in
base units per request. Replace
`X_WEBSEARCH_EVM_ENDPOINT` with the watcher/relay URL;
`X_WEBSEARCH_GATEWAY_ENDPOINT` must select the receipt authority even when
these URLs differ. Replace
`X_WEBSEARCH_RECEIPT_KEY_ID`, `X_WEBSEARCH_RECEIPT_PUBLIC_KEY` and the example
height bounds with an authorized signer's actual values.
`X_WEBSEARCH_PAYER_ADDRESS` supplies the example's payer address. These names
are substitution markers in the example, not automatic environment overrides.

Keys are read from files named by these environment variables:

- `X_WEBSEARCH_ATTESTOR_KEY_FILE`: optional secp256k1 key signing answers.
- `X_WEBSEARCH_SUBMITTER_KEY_FILE`: secp256k1 key signing EVM fulfilment transactions; required when the program relay is enabled.
- `X_WEBSEARCH_RECEIVER_KEY_FILE`: required secp256k1 key identifying the receiver whose DID and ledger accounts startup resolves. Exact payment verification still requires this identity; the key does not enable retired payer grants or metered draws.

Each key file holds 32 hexadecimal bytes, must be unreadable by group and
others, and must hold a different key from the other configured roles.

### State and lag

Keep `data_dir` on durable storage. The relay keeps watcher state under
`<data_dir>/program-watch`, program answers and collected peer signatures in
`<data_dir>/program-attest/answers.json`, and signed fulfilment journals under
`<data_dir>/program-submit`. These are separate recovery stages:

- The watcher persists unfinished requests in `program-watch/pending.json`.
- The exchange syncs `program-attest/answers.json` before returning a local
  answer or accepting a peer signature, including before quorum. Each record
  retains the attestation, response bytes, local signature and collected signer
  signatures. `program-attest/identity.json` binds the store to the chain,
  network id and local signer; reopening under a different identity is refused.
  Restarting after signing reuses the identical answer and collected signatures
  even if the index changes. See the
  [pre-quorum restart proof](tests/proof_x_websearch_program_requests.rs) and
  [documentation recovery/retirement test](tests/repair_22_33_120.rs).
- Pending submission or quorum alone does not retire an answer. After completed
  submission (fulfilled, already fulfilled, refunded or reorged), or a durably
  recorded terminal request refusal, the relay retires watcher work and calls
  `ProgramExchange::forget`.
  This persists a retirement tombstone in `answers.json`; it hides the answer
  from peer HTTP, pending work and quorum lookup and refuses signing it again.
  The original answer and signatures remain on disk to prevent re-signing after
  restart; retirement does not delete those records. Peer-signature refusals
  are recorded separately in `program-attest/program-discarded.jsonl` and do
  not by themselves retire a request.
- The submitter syncs signed transactions before broadcast and resumes pending
  submissions after restart. HTTP payment and delivery recovery uses
  `<data_dir>/payments`.

`GET /readyz` carries `relay_lag` when the program relay is running: `role` is
`program_relay`, `head` is the last observed block height, `next_block` is the
next block to scan, `blocks_behind` measures lag behind the confirmed head,
and `pending` counts unfinished program requests. Readiness also checks the
payment journal, index, content storage and configured receipt endpoint.
