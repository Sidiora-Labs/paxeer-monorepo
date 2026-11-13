# The px namespace at the node root

The node registers account, asset, discovery, history, consensus-evidence and inference APIs under `px` in [rpc/server.go](../../../../rpc/server.go). HTTP JSON-RPC requests use `POST /`. WebSocket registration uses the same method namespace. Configured namespace allowlists and method deny lists determine which methods a server exposes.

Node reads use the local EVM, keepers and consensus client. Indexed reads use one configured indexer. Hosted registration and funding remain service operations: their node methods validate parameters and return an explicit hosted-service refusal.

## Method inventory

Parameters below are positional; `?` marks an optional trailing parameter. Zero-argument examples use `[]`.

| Method | Parameters | Result or behavior | Implementation |
|---|---|---|---|
| `px_resolveAccount` | `selector` | Unified account binding; unavailable when no EVM binding can be resolved | [account](../../../../rpc/px_account.go) |
| `px_getAccount` | `selector` | `{account, paxeer}`; `paxeer` is null for an unbound DID | [account](../../../../rpc/px_account.go) |
| `px_getBalances` | `selector` | Account, pinned height and bank-denom balances | [account](../../../../rpc/px_account.go) |
| `px_listAssets` | `cursor?` | Registration-ordered asset page with metadata | [assets](../../../../rpc/px_assets.go) |
| `px_getNetwork` | none | Network ID, EVM head and optional consensus status | [account](../../../../rpc/px_account.go) |
| `px_getCapabilities` | none | Cached precompile probes and addresses | [discovery](../../../../rpc/px_discovery.go) |
| `px_getRouteCatalogue` | none | Route metadata and registration/readiness information | [discovery](../../../../rpc/px_discovery.go) |
| `px_getHistory` | `selector, cursor?, limit?, kind?` | EVM-side indexed history with freshness | [history](../../../../rpc/px_history.go) |
| `px_getUnifiedHistory` | `selector, cursor?, limit?, kind?` | Joined EVM and bound native-account history | [history](../../../../rpc/px_history.go) |
| `px_listHolderRecords` | `address, kind, cursor?, limit?` | Indexed budget or grant holder records | [history](../../../../rpc/px_history.go) |
| `px_getCommit` | `height` | Decimal height and consensus `signedHeader` | [kernel reads](../../../../rpc/px_kernel.go) |
| `px_getValidators` | `height` | Validators, decimal voting powers and decimal total count | [kernel reads](../../../../rpc/px_kernel.go) |
| `px_getReceiptKeys` | none | Governed receipt-key pins and height ranges | [receipt API](../../../../rpc/sequencer_receipt.go) |
| `px_getSequenceBatch` | `height` | Transaction-zero bytes, Merkle proof and ordered activity IDs | [kernel reads](../../../../rpc/px_kernel.go) |
| `px_register` | `signer, signature` | Hosted-service refusal after validation | [account](../../../../rpc/px_account.go) |
| `px_requestFunds` | `did, signer` | Hosted-service refusal after validation | [account](../../../../rpc/px_account.go) |
| `pax_sequencerReceipt` | `txHash` | Submitted, receipted or committed observation; null for unknown hash | [receipt API](../../../../rpc/sequencer_receipt.go) |

The receipt lookup belongs to the `pax` namespace. Its evidence is related to the `px` consensus reads, but method names retain their registered namespace.

## Account selectors and balances

An account selector accepts a `0x`-prefixed 20-byte EVM address or `did:layerx:` followed by a 32-byte hexadecimal identity. Account parsing also recognizes a bare native account ID, but reverse lookup of that form is unavailable. History accepts only the EVM/DID selector forms.

Resolution uses the address precompile at `0x1004`, including `getUnifiedAccount` and `getEvmAddrByLayerX`. The unified account fields are `evm_address`, `pax_address`, `layerx_did`, `layerx_account` and `bound`. Reads resolve one height through the node watermarks and keep dependent state reads at that height.

`px_getAccount` returns `{account, paxeer}`. The `paxeer` object contains address, balance, nonce and height when an EVM address is available. An unbound DID can return its account with `paxeer: null`; `px_resolveAccount` and `px_getBalances` require an EVM binding.

`px_getBalances` calls bank `all_balances` at `0x1001`. Each balance has `denom` and a decimal-string `amount`. These bank quantities are distinct from the hexadecimal quantities in EVM account reads.

## Assets and indexed history

The `evm.px_indexer_url` setting or `WithPxIndexerURL` server option selects one HTTP(S) indexer. An empty setting leaves indexed reads unavailable. Duplicate options, malformed URLs, embedded user information, query strings and fragments are rejected by [indexer configuration](../../../../rpc/px_assets.go).

Assets are returned in registration order with `joined_limit: 16`, `height`, `assets`, `next_cursor` and `order: "registration"`. The registration cursor is a positive canonical decimal value within signed 64-bit range. Omit the cursor on the first request; an explicit null cursor is rejected by [parameter handling](../../../../rpc/px_params.go). Asset reads have a ten-second deadline and enrich indexer records using block-pinned chain metadata.

History defaults to 50 items and accepts limits from 1 through 100. Cursors are opaque validated values; clients return them unchanged. Indexed history uses an eight-second deadline and returns `items`, `next_cursor`, `indexedHeight`, `chainHeight` and `freshness`.

Unified history enumerates the bound native accounts, includes the canonical native main account and joins at most sixteen accounts including the EVM side. The join preserves native rows and stable pagination. A missing indexer, unavailable binding or invalid indexer response remains an explicit error. Holder-record `kind` is `budget` or `grant`.

## Network and route discovery

`px_getNetwork` returns the local `network_id` and a `paxeer` object with hexadecimal `chain_id` and `latest_block`. Consensus information is optional: if the consensus status cannot be read, the response retains the EVM head, sets `consensus` to null and reports `reason: "consensus_unavailable"`.

`px_getCapabilities` probes seven precompiles through block-pinned calls and caches the result for fifteen seconds. The keys are `exchange`, `bridge`, `launchpad`, `ledger`, `pay`, `auth` and `programs`. The `exchange` probe checks the served perps address's `totalMarkets`; the field name does not select the historical exchange ABI.

`px_getRouteCatalogue` reads [the embedded route catalogue](../../../../rpc/px_route_catalogue.json). It reports `rpc_paths: ["/"]`. Node readiness describes registered methods. Hosted dependency readiness is `unknown` with reason `hosted_dependency_not_measured`; node registration does not measure remote service availability. Hosted gateway location is operator-provided.

## Consensus and receipt evidence

Commit, validator and sequence-batch heights are positive decimal unsigned-64-bit strings. A valid height greater than the consensus client's signed-64-bit range is unavailable rather than malformed.

`px_getCommit` checks the returned height and signed header's chain identity. `px_getValidators` reads all validator pages, validates keys and addresses and returns `height`, `validators` and `total`; each validator carries `address`, `pubKey` and decimal `votingPower`.

`px_getSequenceBatch` verifies the block's transaction root, decodes transaction zero, derives subsequent activity IDs and checks them against the batch. Its `proof` contains `total`, `index`, `leafHash` and `aunts`. A block without a sequence batch returns `no_sequence_batch` as an invalid-parameters error.

`px_getReceiptKeys` reads governed pins even when the node is not a receipt-signing primary. Each pin has `keyId`, `publicKey`, `firstHeight` and `lastHeight`. Clients must verify the key's height coverage; a public key carried inside a receipt bundle does not authorize itself. The method requires explicit empty parameters `[]` when governed keys are available.

`pax_sequencerReceipt` reads a local receipt source or forwards to the configured source. A signature authenticates execution; independent committed verification also needs inclusion, header and validator evidence. See [sequencer and consensus](sequencer-and-consensus.md) and [client verification](clients-and-sdks.md#receipt-observations-and-verification).

## Inference reads

[The inference API](../../../../rpc/px_inference.go) uses the deployed AI-market program selected by `WithPxInferenceProgram(program [32]byte, reader common.Address)`. The reader is a bound EVM identity used for read-only native program envelopes. IDs are nonzero, unprefixed lowercase 32-byte hex values.

| Method | Parameters | Read |
|---|---|---|
| `px_getInferenceRoom` | `roomID` | Current room and its payment-stream details |
| `px_getInferenceRoomAt` | `roomID, height` | Room at a retained committed height |
| `px_getInferenceRooms` | none | Rooms listed by program state |
| `px_getInferenceDeal` | `dealID` | Deal record |
| `px_getInferencePeer` | `peerKey` | Current peer record |
| `px_getInferencePeerAt` | `peerKey, height` | Peer at a retained committed height |
| `px_getInferenceReceipts` | `dealID` | Deal receipt records |

These methods validate fixed-width producer encodings and pin nested calls to one resolved height. Missing program configuration, unavailable historical state and noncanonical producer responses are explicit unavailable results.

## Hosted operations and errors

`px_register` requires an unprefixed lowercase 32-byte signer and 64-byte signature. `px_requestFunds` requires a syntactically valid DID of at most 512 bytes and a nonzero unprefixed lowercase 32-byte signer. Valid parameters receive `-32004`, message `Hosted gateway service operation`, with `data.code = "hosted_gateway_service_required"` and the method name. The node performs neither hosted action.

| Code | Meaning |
|---|---|
| `-32602` | Invalid parameters, including invalid cursors or a height with no sequence batch |
| `-32001` | Account, history, asset or inference read unavailable, with a specific data code |
| `-32003` | Consensus evidence or governed receipt keys unavailable |
| `-32004` | Hosted gateway operation required |
| `-32000` | Receipt lookup has neither a local source nor a forwarder |

A missing method remains subject to normal JSON-RPC method registration and server policy. Clients must not translate unavailable evidence into a successful empty result.

For example, this root-path request asks for network discovery:

```json
{"jsonrpc":"2.0","id":1,"method":"px_getNetwork","params":[]}
```
