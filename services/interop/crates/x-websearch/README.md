# x-websearch

The Paxeer X Network web search sidecar. It serves `GET /health` and `GET /readyz` for free, `GET /search` and `GET /fetch` behind the 402LXP payment gate, and `GET /content/<digest>` unpaid. Its configuration is one JSON file passed as `--config <path>`. The loader refuses unknown fields, missing fields, placeholders and key material, and names the field it refused.

## EVM program relay and settlement

The program relay starts when an attestor key is present and requires a submitter key. It follows `XWebProgramRequested` logs from the `0x1019` xweb precompile through `eth_getLogs`, waits for the configured confirmations, answers each request, and collects signatures from peer attestors. It refreshes the registered attestor set and threshold from the EVM endpoint on every step. Once the threshold agrees, the submitter sends an EIP-155 transaction calling `fulfilProgram(bytes)` on `0x1019`.

Paid HTTP requests settle on the `0x101A` ledger precompile. The receiver signs metered draws as EIP-155 transactions and broadcasts them with `eth_sendRawTransaction`. The payment gate checks settlement through `pax_sequencerReceipt`, verifying the receipt against the configured receipt keys and their authorized height ranges.

| Key | Meaning |
| --- | --- |
| `gateway.endpoint` | The settlement JSON-RPC URL, supporting `eth_*` and `pax_sequencerReceipt`. Plain http is accepted only for loopback or `localhost`. |
| `gateway.receipt_keys` | A nonempty array of authorized receipt signers. Each entry has a 32-byte hexadecimal `key_id`, an Ed25519 `public_key`, and inclusive integer `first_height` and `last_height` bounds. Ranges for the same key id must not overlap. |
| `payment.payer_address` | The payer's `0x`-prefixed, 20-byte EVM address. When absent, each request names its payer with `LAYERX-PAYER-ADDRESS`. |
| `evm.endpoint` | The chain JSON-RPC URL used to watch requests, refresh attestors, and submit fulfilments. |
| `evm.chain_id` | The EVM chain id bound to signed transactions; 125 for Paxeer. |
| `evm.confirmations` | The number of confirmations required for watched requests and fulfilments. |
| `kernel_network_id` | The optional nonzero network id used to bind observations and payment accounts. Startup resolves it from the xweb precompile and checks a configured value against it. |
| `peers` | The peer attestor endpoints whose program signatures are collected. |

Before loading `config.example.json`, replace every `${NAME}` string with a valid value; the loader does not expand environment variables. `X_WEBSEARCH_SEED_URL` is the crawl seed URL. `X_WEBSEARCH_<SID|PAX|USDC|USDL>_ASSET_ID` is the asset's 32-byte hexadecimal id, and `X_WEBSEARCH_<SID|PAX|USDC|USDL>_PRICE` is its positive decimal price in base units per request. Set `X_WEBSEARCH_GATEWAY_ENDPOINT` and `X_WEBSEARCH_EVM_ENDPOINT` to the settlement and chain RPC URLs, `X_WEBSEARCH_RECEIPT_KEY_ID` and `X_WEBSEARCH_RECEIPT_PUBLIC_KEY` to an authorized receipt signer's id and public key, and `X_WEBSEARCH_PAYER_ADDRESS` to the payer's EVM address. Replace the example receipt height bounds with that signer's actual authorization range. The example contains only fields accepted by the loader.

Keys are read from the files named by these environment variables:

- `X_WEBSEARCH_ATTESTOR_KEY_FILE`: the secp256k1 attestor key that signs answers.
- `X_WEBSEARCH_SUBMITTER_KEY_FILE`: the secp256k1 key that signs EVM fulfilment transactions.
- `X_WEBSEARCH_RECEIVER_KEY_FILE`: the secp256k1 receiver key that signs metered draws to `0x101A`.

### State and lag

The relay keeps its watcher state under `<data_dir>/program-watch`, peer answers under `<data_dir>/program-attest`, and submission journals under `<data_dir>/program-submit`. Signed transactions are journalled before broadcast; a restart resumes pending submissions and checks their outcomes before taking further action.

`GET /readyz` carries `relay_lag` when the program relay is running: `role` is `program_relay`, `head` is the last observed block height, `next_block` is the next block to scan, `blocks_behind` measures lag behind the confirmed head, and `pending` counts unfinished program requests.
