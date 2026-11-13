# Interop websearch contract

The [web-search service](../../../services/interop/crates/x-websearch/src/lib.rs) crawls and indexes content, serves paid search and fetch requests and answers xweb program requests. Payments and program fulfilments use EVM transactions and authenticated node receipts.

## cfg-xws-assets-asset-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-assets-asset-id</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config assets.&lt;SID&#124;PAX&#124;USDC&#124;USDL&gt;.asset_id: 32-byte hex; unknown fields refused by object.finish</code> |

## cfg-xws-assets-price

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-assets-price</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config assets.&lt;symbol&gt;.price: decimal string base units; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-interval-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-interval-seconds</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config crawl_interval_seconds: optional u64, default DEFAULT_CRAWL_INTERVAL_SECONDS; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-max-depth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-max-depth</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config crawl.max_depth: integer &lt;= MAX_DEPTH; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-pages-per-cycle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-pages-per-cycle</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config crawl.pages_per_cycle: positive u32; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-pages-per-host

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-pages-per-host</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config crawl.pages_per_host: positive u32 &lt;= pages_per_cycle; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-politeness-delay-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-politeness-delay-ms</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config crawl.politeness_delay_ms: u64 &lt;= MAX_POLITENESS_DELAY_MS; unknown fields refused by object.finish</code> |

## cfg-xws-data-dir

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-data-dir</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config data_dir: directory; unknown fields refused by object.finish</code> |

## cfg-xws-evm-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-evm-chain-id</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config evm.chain_id: u64; unknown fields refused by object.finish</code> |

## cfg-xws-evm-confirmations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-evm-confirmations</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config evm.confirmations: positive u32 &lt;= MAX_CONFIRMATIONS; unknown fields refused by object.finish</code> |

## cfg-xws-evm-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-evm-endpoint</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config evm.endpoint: URL; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-allow-loopback

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-allow-loopback</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config fetch.allow_loopback: bool; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-connect-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-connect-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config fetch.connect_timeout_ms: u64 bounded; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-max-body-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-max-body-bytes</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config fetch.max_body_bytes: u64 bounded; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-max-redirects

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-max-redirects</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config fetch.max_redirects: integer &lt;= REDIRECT_LIMIT; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-total-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-total-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config fetch.total_timeout_ms: u64 bounded; unknown fields refused by object.finish</code> |

## cfg-xws-gateway-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-gateway-endpoint</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config gateway.endpoint: URL; unknown fields refused by object.finish</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## cfg-xws-gateway-sequencer-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-gateway-sequencer-id</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config gateway.sequencer_id: 32-byte hex; unknown fields refused by object.finish</code> |

## cfg-xws-gateway-sequencer-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-gateway-sequencer-public-key</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config gateway.sequencer_public_key: 32-byte hex ed25519 key, weak keys refused; unknown fields refused by object.finish</code> |

## cfg-xws-kernel

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-kernel</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config kernel: optional object {endpoint, poll_interval_ms, topics, submitter_did, fee_limit}; unknown fields refused by object.finish</code> |

## cfg-xws-kernel-fee-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-kernel-fee-limit</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config kernel.fee_limit: decimal u128; unknown fields refused by object.finish</code> |

## cfg-xws-kernel-network-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-kernel-network-id</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config kernel_network_id: positive u32; unknown fields refused by object.finish</code> |

| Field | Rule |
|---|---|
| interface | x-websearch reads one JSON configuration file and refuses unknown fields. |
| storage | none |
| errors | ConfigError naming the refused field. |
| events | none |
| gas | none |
| activation | Read at start with --config. |
| invariants | Key material never appears in the configuration. |
| pause | none |

## cfg-xws-kernel-topics

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-kernel-topics</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config kernel.topics: array of strings; unknown fields refused by object.finish</code> |

## cfg-xws-listen

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-listen</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config listen: socket address; unknown fields refused by object.finish</code> |

## cfg-xws-payment-conformance-suite

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-payment-conformance-suite</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config payment.conformance_suite: optional; unknown fields refused by object.finish</code> |

## cfg-xws-payment-draw-fee-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-payment-draw-fee-limit</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config payment.draw_fee_limit: optional decimal u128; unknown fields refused by object.finish</code> |

## cfg-xws-peers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-peers</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config peers: array of peer endpoint URLs; unknown fields refused by object.finish</code> |

## cfg-xws-receipt-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-receipt-keys</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config receipt_keys: array of {keyId, publicKey (ed25519), firstHeight, lastHeight}; firstHeight &gt; lastHeight or weak key refused</code> |

| Field | Rule |
|---|---|
| interface | x-websearch reads one JSON configuration file and refuses unknown fields. |
| storage | none |
| errors | ConfigError naming the refused field. |
| events | none |
| gas | none |
| activation | Read at start with --config. |
| invariants | Key material never appears in the configuration. |
| pause | none |

## cfg-xws-seeds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-seeds</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config seeds: array of seed URLs (&lt;= MAX_SEEDS); unknown fields refused by object.finish</code> |

## cli-xws

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-xws</code> | <code>cli_command</code> | [services/interop/crates/x-websearch/src/main.rs](../../../services/interop/crates/x-websearch/src/main.rs) | <code>x-websearch --config PATH</code> |

## env-xws-paxeer-x-source-revision

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-xws-paxeer-x-source-revision</code> | <code>env_var</code> | [services/interop/crates/x-websearch/src/main.rs](../../../services/interop/crates/x-websearch/src/main.rs) | <code>PAXEER_X_SOURCE_REVISION</code> |

## env-xws-x-websearch-attestor-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-xws-x-websearch-attestor-key-file</code> | <code>env_var</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>X_WEBSEARCH_ATTESTOR_KEY_FILE, ATTESTOR_KEY_FILE</code> |

## env-xws-x-websearch-receiver-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-xws-x-websearch-receiver-key-file</code> | <code>env_var</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>X_WEBSEARCH_RECEIVER_KEY_FILE, RECEIVER_KEY_FILE</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## env-xws-x-websearch-submitter-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-xws-x-websearch-submitter-key-file</code> | <code>env_var</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>X_WEBSEARCH_SUBMITTER_KEY_FILE, SUBMITTER_KEY_FILE</code> |

## err-xws-answer-too-large

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-answer-too-large</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::TooLarge.code() = "answer_too_large"</code> |

## err-xws-attestation-not-found

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-attestation-not-found</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>HTTP 404 {"error":"attestation_not_found"}</code> |

## err-xws-bad-signature

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-bad-signature</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>Discard::BadSignature.code() = "bad_signature"</code> |

## err-xws-body-not-accepted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-body-not-accepted</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 400 {"error":"body_not_accepted"}</code> |

## err-xws-body-too-large

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-body-too-large</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::BodyTooLarge.code() = "body_too_large"</code> |

## err-xws-connect-failed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-connect-failed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::Connect.code() = "connect_failed"</code> |

## err-xws-connect-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-connect-timeout</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::ConnectTimeout.code() = "connect_timeout"</code> |

## err-xws-content-not-found

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-content-not-found</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>HTTP 404 {"error":"content_not_found"}</code> |

## err-xws-content-store-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-content-store-error</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>HTTP 500 {"error":"content_store_error"}</code> |

## err-xws-content-too-large

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-content-too-large</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::ContentTooLarge.code() = "content_too_large"</code> |

## err-xws-content-too-long

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-content-too-long</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>CanonicalError::TooLong.code() = "content_too_long"</code> |

## err-xws-delivery-failed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-delivery-failed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 500/502 {"error":"delivery_failed"}</code> |

## err-xws-delivery-result-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-delivery-result-invalid</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 500 {"error":"delivery_result_invalid"}</code> |

## err-xws-different-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-different-digest</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>Discard::DifferentDigest.code() = "different_digest"</code> |

## err-xws-duplicate-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-duplicate-header</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 400 {"error":"duplicate_header"}</code> |

## err-xws-duplicate-query

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-duplicate-query</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>HTTP 400 {"error":"duplicate_query"}</code> |

## err-xws-duplicate-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-duplicate-url</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>HTTP 400 {"error":"duplicate_url"}</code> |

## err-xws-empty-query

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-empty-query</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>SearchError::EmptyQuery.code() = "empty_query"</code> |

## err-xws-envelope-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-envelope-refused</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::Envelope.code() = "envelope_refused"</code> |

## err-xws-fetch-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-fetch-timeout</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::Timeout.code() = "fetch_timeout"</code> |

## err-xws-forbidden-destination

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-forbidden-destination</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::ForbiddenDestination.code() = "forbidden_destination"</code> |

## err-xws-headers-too-large

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-headers-too-large</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 431 {"error":"headers_too_large"}</code> |

## err-xws-index-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-index-error</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>IndexError::Tantivy.code() = "index_error"</code> |

## err-xws-index-io-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-index-io-error</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>IndexError::Io.code() = "index_io_error"</code> |

## err-xws-index-writer-poisoned

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-index-writer-poisoned</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>IndexError::WriterPoisoned.code() = "index_writer_poisoned"</code> |

## err-xws-internal-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-internal-error</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 500 {"error":"internal_error"}</code> |

## err-xws-invalid-api-payload

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-invalid-api-payload</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::Payload.code() = "invalid_api_payload"</code> |

## err-xws-invalid-credential

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-invalid-credential</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::Credential.code() = "invalid_credential"</code> |

## err-xws-invalid-encoding

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-invalid-encoding</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>CanonicalError::InvalidEncoding.code() = "invalid_encoding"</code> |

## err-xws-invalid-fetch-limits

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-invalid-fetch-limits</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::InvalidLimits.code() = "invalid_fetch_limits"</code> |

## err-xws-invalid-index-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-invalid-index-url</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>IndexError::InvalidUrl.code() = "invalid_index_url"</code> |

## err-xws-invalid-media-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-invalid-media-type</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>CanonicalError::InvalidMediaType.code() = "invalid_media_type"</code> |

## err-xws-invalid-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-invalid-response</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 500 {"error":"invalid_response"}</code> |

## err-xws-invalid-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-invalid-url</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::InvalidUrl.code() = "invalid_url"</code> |

## err-xws-malformed-content

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-content</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>CanonicalError::Malformed.code() = "malformed_content"</code> |

## err-xws-malformed-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-digest</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>HTTP 400 {"error":"malformed_digest"}</code> |

## err-xws-malformed-payer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-payer</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 400 {"error":"malformed_payer"}</code> |

## err-xws-malformed-program-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-program-request</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 400 {"error":"malformed_program_request"}</code> |

## err-xws-malformed-query

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-query</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>HTTP 400 {"error":"malformed_query"}</code> |

## err-xws-malformed-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-record</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>Discard::Malformed.code() = "malformed_record"</code> |

## err-xws-malformed-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-request</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 400 {"error":"malformed_request"}</code> |

## err-xws-malformed-request-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-request-id</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 400 {"error":"malformed_request_id"}</code> |

## err-xws-malformed-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-malformed-response</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::MalformedResponse.code() = "malformed_response"</code> |

## err-xws-method-not-allowed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-method-not-allowed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 405 {"error":"method_not_allowed"}</code> |

## err-xws-missing-host

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-missing-host</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 400 {"error":"missing_host"}</code> |

## err-xws-missing-media-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-missing-media-type</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/extract.rs](../../../services/interop/crates/x-websearch/src/extract.rs) | <code>ExtractError::MissingMediaType.code() = "missing_media_type"</code> |

## err-xws-missing-query

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-missing-query</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 400 {"error":"missing_query"}</code> |

## err-xws-missing-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-missing-url</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>HTTP 400 {"error":"missing_url"}</code> |

## err-xws-no-envelope-for-attestor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-no-envelope-for-attestor</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::NoEnvelope.code() = "no_envelope_for_attestor"</code> |

## err-xws-not-found

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-not-found</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 404 {"error":"not_found"}</code> |

## err-xws-payer-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-payer-mismatch</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 400 {"error":"payer_mismatch"}</code> |

## err-xws-payment-pending

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-payment-pending</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 503 {"error":"payment_pending"}</code> |

## err-xws-payment-store-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-payment-store-unavailable</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 503 {"error":"payment_store_unavailable"}</code> |

## err-xws-payment-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-payment-unavailable</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 503 {"error":"payment_unavailable"}</code> |

## err-xws-persistence-failed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-persistence-failed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>Discard::Persistence.code() = "persistence_failed"</code> |

## err-xws-pointer-unresolved

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-pointer-unresolved</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::Pointer.code() = "pointer_unresolved"</code> |

## err-xws-query-too-long

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-query-too-long</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>SearchError::QueryTooLong.code() = "query_too_long"</code> |

## err-xws-receipt-consumed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-receipt-consumed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>HTTP 409 {"error":"receipt_consumed"}</code> |

## err-xws-request-line-too-long

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-request-line-too-long</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 414 {"error":"request_line_too_long"}</code> |

## err-xws-request-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-request-timeout</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 408 {"error":"request_timeout"}</code> |

## err-xws-resolve-failed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-resolve-failed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::Resolve.code() = "resolve_failed"</code> |

## err-xws-response-not-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-response-not-json</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::NotJson.code() = "response_not_json"</code> |

## err-xws-robots-disallowed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-robots-disallowed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::RobotsDisallowed.code() = "robots_disallowed"</code> |

## err-xws-robots-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-robots-unavailable</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::RobotsUnavailable.code() = "robots_unavailable"</code> |

## err-xws-route-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-route-unavailable</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 503 {"error":"route_unavailable"}</code> |

## err-xws-server-busy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-server-busy</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 503 {"error":"server_busy"}</code> |

## err-xws-service-not-ready

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-service-not-ready</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 503 {"error":"service_not_ready"}</code> |

## err-xws-title-too-long

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-title-too-long</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>IndexError::TitleTooLong.code() = "title_too_long"</code> |

## err-xws-tls-failed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-tls-failed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::Tls.code() = "tls_failed"</code> |

## err-xws-too-many-redirects

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-too-many-redirects</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::TooManyRedirects.code() = "too_many_redirects"</code> |

## err-xws-too-many-terms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-too-many-terms</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>SearchError::TooManyTerms.code() = "too_many_terms"</code> |

## err-xws-transport-failed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-transport-failed</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::Transport.code() = "transport_failed"</code> |

## err-xws-unexpected-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-unexpected-bytes</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>HTTP 400 {"error":"unexpected_bytes"}</code> |

## err-xws-unknown-query-parameter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-unknown-query-parameter</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>HTTP 400 {"error":"unknown_query_parameter"}</code> |

## err-xws-unknown-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-unknown-signer</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>Discard::UnknownSigner.code() = "unknown_signer"</code> |

## err-xws-unsupported-charset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-unsupported-charset</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>CanonicalError::UnsupportedCharset.code() = "unsupported_charset"</code> |

## err-xws-unsupported-encoding

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-unsupported-encoding</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::UnsupportedEncoding.code() = "unsupported_encoding"</code> |

## err-xws-unsupported-media-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-unsupported-media-type</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::MediaType.code() = "unsupported_media_type"</code> |

## err-xws-unsupported-scheme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-unsupported-scheme</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>FetchError::UnsupportedScheme.code() = "unsupported_scheme"</code> |

## err-xws-upstream-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-upstream-status</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>ApiError::Status.code() = "upstream_status"</code> |

## err-xws-wrong-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-xws-wrong-request</code> | <code>error_code</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>Discard::WrongRequest.code() = "wrong_request"</code> |

## fmt-xws-api-envelope-info

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-api-envelope-info</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>HKDF info "PAXEERX_WEB_API_ENVELOPE_V1" for the api request envelope (AES-GCM under ECDH secp256k1); payload: version u8=1 &#124; method u8 (1 GET, 2 POST) &#124; level u8 (0 majority, 1 single) &#124; attestor address 20 &#124; ...</code> |

## fmt-xws-asset-symbol-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-asset-symbol-labels</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>AssetSymbol::code() -&gt; "SID" &#124; "PAX" &#124; "USDC" &#124; "USDL"</code> |

## fmt-xws-attestation-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-attestation-domain</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>attestation digest prefix "PAXEERX_WEB_V1" over the answered request</code> |

## fmt-xws-content-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-content-domain</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>canonical content = "PAXEERX_WEB_CONTENT_V1" &#124; kind u8 &#124; payload len u32 &#124; payload &#124; media type len u32 &#124; media type &#124; text len u64 &#124; text</code> |

## fmt-xws-delivery-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-delivery-record</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct DeliveryRecord { version: u8, principal: String, request_digest: String, resource: String, receipt_digest: String, state: DeliveryState, response: Option&lt;DeliveryResponse&gt;, #[serde(default)] failures: u32, #[serde(default)] retry_after_ms: u64, }</code> |

## fmt-xws-delivery-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-delivery-response</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct DeliveryResponse { status: u16, content_type: String, headers: Vec&lt;(String, String)&gt;, body: Vec&lt;u8&gt;, body_digest: String, canonical_digest: Option&lt;String&gt;, canonical_content: Option&lt;Vec&lt;u8&gt;&gt;, }</code> |

## fmt-xws-delivery-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-delivery-state</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>DeliveryState = pending &#124; computing &#124; failed &#124; ready</code> |

## fmt-xws-dependency-readiness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-dependency-readiness</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/main.rs](../../../services/interop/crates/x-websearch/src/main.rs) | <code>Rust model: struct DependencyReadiness { dependency: &amp;'static str, state: DependencyState, critical: bool, last_success_unix_ms: Option&lt;u64&gt;, }</code> |

## fmt-xws-dependency-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-dependency-state</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/main.rs](../../../services/interop/crates/x-websearch/src/main.rs) | <code>DependencyState = starting &#124; ready &#124; unavailable &#124; stale</code> |

## fmt-xws-http-reason-phrases

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-http-reason-phrases</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>fn reason(status: u16) -&gt; &amp;'static str: OK &#124; Forbidden &#124; Conflict</code> |

## fmt-xws-journal-state-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-journal-state-labels</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>JournalState::code() -&gt; "signed" &#124; "fulfilled" &#124; "already_fulfilled" &#124; "refunded" &#124; "reorged"</code> |

## fmt-xws-payment-domains

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-payment-domains</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>payment digests: purpose = sha256("LayerX/x-websearch/v1/purpose\0" &#124; receiver public key), principal = sha256("LayerX/x-websearch/v1/principal\0" &#124; DID), trace = sha256("LayerX/x-websearch/v1/trace\0" &#124; header)</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## fmt-xws-payment-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-payment-record</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct PaymentRecord { pub principal: String, pub request_digest: String, pub resource: String, pub offer: Value, pub payer: Option&lt;String&gt;, pub receive_key: Option&lt;String&gt;, pub tx_hash: Option&lt;String&gt;, pub raw_transaction: Option&lt;String&gt;, pub attempted: bool, pub receipt: Option&lt;String&gt;, pub receipt_digest: Option&lt;String&gt;, pub released: bool, }</code> |

## fmt-xws-pending

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-pending</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>Rust model: pub struct Pending { pub request: WebRequest, pub attempts: u32, pub refused: Option&lt;String&gt;, #[serde(default)] pub source: Option&lt;SourceIdentity&gt;, #[serde(default)] pub stage: WorkStage, #[serde(default)] pub next_retry_block: u64, #[serde(default)] pub last_error: Option&lt;String&gt;, }</code> |

## fmt-xws-progress

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-progress</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct Progress { version: u32, next_block: Option&lt;u64&gt;, chain_id: Option&lt;u64&gt;, entries: Vec&lt;Pending&gt;, #[serde(default)] program_entries: Vec&lt;PendingProgram&gt;, }</code> |

## fmt-xws-role-readiness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-role-readiness</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/main.rs](../../../services/interop/crates/x-websearch/src/main.rs) | <code>Rust model: struct RoleReadiness { role: &amp;'static str, state: DependencyState, dependencies: Vec&lt;DependencyReadiness&gt;, freshness_budget_ms: u64, first_use_deadline_unix_ms: u64, }</code> |

## fmt-xws-search-result

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-search-result</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>Rust model: pub struct SearchResult { pub url: String, pub title: String, pub snippet: String, }</code> |

## fmt-xws-source-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-source-identity</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct SourceIdentity { pub chain_id: u64, pub block_hash: [u8; 32], pub transaction_hash: [u8; 32], pub log_index: u64, }</code> |

## fmt-xws-web-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-web-request</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>Rust model: pub struct WebRequest { pub request_id: u64, pub requester: [u8; 20], pub kind: u8, pub payload: Vec&lt;u8&gt;, pub callback_gas: u64, pub paid: [u8; 32], pub timeout_height: u64, pub block_number: u64, }</code> |

## fmt-xws-work-stage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-xws-work-stage</code> | <code>file_format</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>WorkStage = queued &#124; attesting &#124; attested &#124; quorum &#124; signed &#124; retry &#124; refused &#124; reorged</code> |

## http-xws-attestations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-attestations</code> | <code>http_route</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>GET /attestations/&lt;decimal request id&gt;</code> |

## http-xws-content

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-content</code> | <code>http_route</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>GET /content/&lt;64 lowercase hex digest&gt;</code> |

| Field | Rule |
|---|---|
| interface | x-websearch serves /health, /readyz, /search, /fetch, /content and /program-attestations. |
| storage | none |
| errors | HTTP error bodies with stable codes. |
| events | none |
| gas | none |
| activation | At start. |
| invariants | /content is served before serving eligibility; paid routes wait for it. |
| pause | none |

## http-xws-fetch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-fetch</code> | <code>http_route</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>GET /fetch?url=&lt;absolute url&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## http-xws-health

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-health</code> | <code>http_route</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>GET /health &#124; GET /xweb/health</code> |

## http-xws-program-attestations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-program-attestations</code> | <code>http_route</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>GET /program-attestations/&lt;64 lowercase hex program id&gt;/&lt;decimal request id&gt;</code> |

| Field | Rule |
|---|---|
| interface | x-websearch serves /health, /readyz, /search, /fetch, /content and /program-attestations. |
| storage | none |
| errors | HTTP error bodies with stable codes. |
| events | none |
| gas | none |
| activation | At start. |
| invariants | /content is served before serving eligibility; paid routes wait for it. |
| pause | none |

## http-xws-readyz

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-readyz</code> | <code>http_route</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>GET /readyz</code> |

## http-xws-search

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-xws-search</code> | <code>http_route</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>GET /search?q=&lt;query&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## rs-xws-draw-calldata

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-draw-calldata</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn draw_calldata( grant_id: &amp;[u8; 32], recipient: &amp;[u8; 32], amount: u128, idempotency_key: &amp;[u8; 32], receiver_sequence: u64, purpose: &amp;[u8; 32], trace: &amp;[u8; 32], ) -&gt; Vec&lt;u8&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## rs-xws-fulfil-program-calldata

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fulfil-program-calldata</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn fulfil_program_calldata(observation: &amp;[u8]) -&gt; Vec&lt;u8&gt;</code> |

| Field | Rule |
|---|---|
| interface | x-websearch watches xweb program requests on 0x1019 and answers them with fulfilProgram. |
| storage | Program exchange records under the data directory. |
| errors | Program refusals recorded per request. |
| events | XWebRequested and XWebProgramRequested of 0x1019. |
| gas | fulfilProgram transactions paid by the submitter account. |
| activation | When program requests are configured. |
| invariants | One fulfilment per request id. |
| pause | none |

## xws-accept

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accept</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub const ACCEPT: &amp;str</code> |

## xws-accepted-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accepted-asset</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/assets.rs](../../../services/interop/crates/x-websearch/src/assets.rs) | <code>pub struct AcceptedAsset</code> |

## xws-accepted-asset-id-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accepted-asset-id-hex</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/assets.rs](../../../services/interop/crates/x-websearch/src/assets.rs) | <code>pub fn id_hex(&amp;self) -&gt; String</code> |

## xws-accepted-assets

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/assets.rs](../../../services/interop/crates/x-websearch/src/assets.rs) | <code>pub struct AcceptedAssets</code> |

## xws-accepted-assets-all

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-all</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/assets.rs](../../../services/interop/crates/x-websearch/src/assets.rs) | <code>pub const fn all(&amp;self) -&gt; &amp;[AcceptedAsset; 4]</code> |

## xws-accepted-assets-by-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-by-id</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/assets.rs](../../../services/interop/crates/x-websearch/src/assets.rs) | <code>pub fn by_id(&amp;self, asset_id: &amp;[u8; 32]) -&gt; Option&lt;&amp;AcceptedAsset&gt;</code> |

## xws-accepted-assets-get

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-get</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/assets.rs](../../../services/interop/crates/x-websearch/src/assets.rs) | <code>pub fn get(&amp;self, symbol: AssetSymbol) -&gt; &amp;AcceptedAsset</code> |

## xws-accepted-assets-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/assets.rs](../../../services/interop/crates/x-websearch/src/assets.rs) | <code>pub fn new(configured: &amp;[AssetConfig]) -&gt; Result&lt;Self, AssetRefusal&gt;</code> |

## xws-accepted-media-types

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-accepted-media-types</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/extract.rs](../../../services/interop/crates/x-websearch/src/extract.rs) | <code>pub const ACCEPTED_MEDIA_TYPES: [&amp;str; 3]</code> |

## xws-address-of

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-address-of</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn address_of(key: &amp;VerifyingKey) -&gt; [u8; 20]</code> |

## xws-answer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-answer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn answer(client: &amp;ApiClient, key: &amp;SigningKey, raw_payload: &amp;[u8], payload: &amp;ApiPayload) -&gt; Result&lt;ApiAnswer, ApiError&gt;</code> |

## xws-answer-canonical-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-answer-canonical-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn answer_canonical_bytes(payload: &amp;[u8], media_type: &amp;str, answer: &amp;[u8]) -&gt; Result&lt;Vec&lt;u8&gt;, CanonicalError&gt;</code> |

## xws-answer-media-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-answer-media-type</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const ANSWER_MEDIA_TYPE: &amp;str</code> |

## xws-answer-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-answer-record</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn record(&amp;self) -&gt; Value</code> |

## xws-answer-request-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-answer-request-id</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const fn request_id(&amp;self) -&gt; u64</code> |

## xws-api-answer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-answer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub struct ApiAnswer</code> |

## xws-api-canonical-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-canonical-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn api_canonical_bytes(payload: &amp;[u8], media_type: &amp;str, answer: &amp;[u8]) -&gt; Result&lt;Vec&lt;u8&gt;, ApiError&gt;</code> |

## xws-api-client

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-client</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub struct ApiClient</code> |

## xws-api-client-body-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-client-body-limit</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn body_limit(&amp;self) -&gt; usize</code> |

## xws-api-client-call

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-client-call</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn call(&amp;self, payload: &amp;ApiPayload, credential: Option&lt;&amp;Credential&gt;) -&gt; Result&lt;ApiResponse, ApiError&gt;</code> |

## xws-api-client-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-client-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn new(fetcher: Arc&lt;Fetcher&gt;, roots: &amp;[native_tls::Certificate]) -&gt; Result&lt;Self, ApiError&gt;</code> |

## xws-api-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub enum ApiError</code> |

## xws-api-error-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-error-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const fn code(&amp;self) -&gt; &amp;'static str</code> |

## xws-api-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-header</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub struct ApiHeader</code> |

## xws-api-header-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-header-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn new(name: &amp;str, value: &amp;str) -&gt; Self</code> |

## xws-api-payload

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-payload</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub struct ApiPayload</code> |

## xws-api-payload-attestation-level

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-attestation-level</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const fn attestation_level(&amp;self) -&gt; Level</code> |

## xws-api-payload-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn decode(raw: &amp;[u8]) -&gt; Result&lt;Self, ApiError&gt;</code> |

## xws-api-payload-encode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-encode</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn encode(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, ApiError&gt;</code> |

## xws-api-payload-envelope-for

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-envelope-for</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn envelope_for(&amp;self, attestor: &amp;[u8; 20]) -&gt; Option&lt;&amp;Envelope&gt;</code> |

## xws-api-payload-method-name

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-method-name</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn method_name(&amp;self) -&gt; Result&lt;&amp;'static str, ApiError&gt;</code> |

## xws-api-payload-origin

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-origin</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn origin(&amp;self) -&gt; Result&lt;String, ApiError&gt;</code> |

## xws-api-payload-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ApiError&gt;</code> |

## xws-api-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-response</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub struct ApiResponse</code> |

## xws-api-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-api-version</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const API_VERSION: u8</code> |

## xws-asset-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-asset-config</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub struct AssetConfig</code> |

## xws-asset-refusal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-asset-refusal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/assets.rs](../../../services/interop/crates/x-websearch/src/assets.rs) | <code>pub enum AssetRefusal</code> |

## xws-asset-symbol

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-asset-symbol</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub enum AssetSymbol</code> |

## xws-asset-symbol-all

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-asset-symbol-all</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const ALL: [Self; 4]</code> |

## xws-asset-symbol-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-asset-symbol-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

## xws-attest-answer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attest-answer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub struct Answer</code> |

## xws-attest-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attest-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub enum AttestError</code> |

## xws-attest-error-is-terminal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attest-error-is-terminal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const fn is_terminal(&amp;self) -&gt; bool</code> |

## xws-attestation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestation</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub struct Attestation</code> |

## xws-attestation-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestation-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn digest(&amp;self) -&gt; [u8; 32]</code> |

## xws-attestation-evm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestation-evm</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn evm(chain_id: u64, request: &amp;WebRequest, content_digest: [u8; 32], response: &amp;[u8], full_length: u32) -&gt; Self</code> |

## xws-attestation-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestation-path</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub const ATTESTATION_PATH: &amp;str</code> |

## xws-attestation-preimage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestation-preimage</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn preimage(&amp;self) -&gt; [u8; PREIMAGE_LENGTH]</code> |

## xws-attestor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub struct Attestor</code> |

## xws-attestor-attest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor-attest</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn attest(&amp;self, request: &amp;WebRequest) -&gt; Result&lt;Answer, AttestError&gt;</code> |

## xws-attestor-binds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor-binds</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn binds(&amp;self, request: &amp;WebRequest, answer: &amp;Answer) -&gt; bool</code> |

## xws-attestor-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor-key-file</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub const ATTESTOR_KEY_FILE: &amp;str</code> |

## xws-attestor-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn new(key: SigningKey, chain_id: u64, fetcher: Arc&lt;Fetcher&gt;, index: Arc&lt;WebIndex&gt;, store: Arc&lt;ContentStore&gt;) -&gt; Self</code> |

## xws-attestor-set

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor-set</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub struct AttestorSet</code> |

## xws-attestor-set-contains

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor-set-contains</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn contains(&amp;self, signer: &amp;[u8; 20]) -&gt; bool</code> |

## xws-attestor-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor-signer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const fn signer(&amp;self) -&gt; [u8; 20]</code> |

## xws-attestor-with-api-roots

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-attestor-with-api-roots</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn with_api_roots(mut self, roots: Vec&lt;native_tls::Certificate&gt;) -&gt; Self</code> |

## xws-body-limit-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-body-limit-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const BODY_LIMIT_BYTES: u64</code> |

## xws-cache-lifetime

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-cache-lifetime</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub const CACHE_LIFETIME: Duration</code> |

## xws-canonical

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-canonical</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub enum Canonical</code> |

## xws-canonical-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-canonical-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn canonical_bytes(kind: ContentKind, payload: &amp;[u8], media_type: &amp;str, text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, CanonicalError&gt;</code> |

## xws-canonical-content

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-canonical-content</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub struct CanonicalContent</code> |

## xws-canonical-content-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-canonical-content-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn parse(bytes: &amp;[u8]) -&gt; Result&lt;Self, CanonicalError&gt;</code> |

## xws-canonical-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-canonical-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub enum CanonicalError</code> |

## xws-canonical-error-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-canonical-error-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

## xws-canonical-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-canonical-json</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn canonical_json(value: &amp;Json) -&gt; String</code> |

## xws-canonicalise

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-canonicalise</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn canonicalise(text: &amp;str) -&gt; String</code> |

## xws-check

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-check</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn check(bytes: &amp;[u8]) -&gt; Result&lt;ContentKind, CanonicalError&gt;</code> |

## xws-clock

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-clock</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub type Clock = fn() -&gt; u64</code> |

## xws-collapse-blank-lines

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-collapse-blank-lines</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn collapse_blank_lines(text: &amp;str) -&gt; String</code> |

## xws-collapse-spaces

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-collapse-spaces</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn collapse_spaces(text: &amp;str) -&gt; String</code> |

## xws-commitment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-commitment</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const COMMITMENT: &amp;str</code> |

## xws-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-config</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub struct Config</code> |

| Field | Rule |
|---|---|
| interface | x-websearch reads one JSON configuration file and refuses unknown fields. |
| storage | none |
| errors | ConfigError naming the refused field. |
| events | none |
| gas | none |
| activation | Read at start with --config. |
| invariants | Key material never appears in the configuration. |
| pause | none |

## xws-config-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-config-asset</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub fn asset(&amp;self, symbol: AssetSymbol) -&gt; &amp;AssetConfig</code> |

## xws-config-crawl-interval

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-config-crawl-interval</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const fn crawl_interval(&amp;self) -&gt; Duration</code> |

## xws-config-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-config-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub struct ConfigError</code> |

## xws-config-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-config-load</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## xws-config-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-config-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Field | Rule |
|---|---|
| interface | x-websearch reads one JSON configuration file and refuses unknown fields. |
| storage | none |
| errors | ConfigError naming the refused field. |
| events | none |
| gas | none |
| activation | Read at start with --config. |
| invariants | Key material never appears in the configuration. |
| pause | none |

## xws-conformance-suite

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-conformance-suite</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/lib.rs](../../../services/interop/crates/x-websearch/src/lib.rs) | <code>pub const CONFORMANCE_SUITE: &amp;str</code> |

## xws-conformance-suite-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-conformance-suite-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/lib.rs](../../../services/interop/crates/x-websearch/src/lib.rs) | <code>pub const CONFORMANCE_SUITE_DIGEST: [u8; 32]</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-conformance-vector-count

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-conformance-vector-count</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/lib.rs](../../../services/interop/crates/x-websearch/src/lib.rs) | <code>pub const CONFORMANCE_VECTOR_COUNT: u64</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-connect-timeout-limit-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-connect-timeout-limit-ms</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const CONNECT_TIMEOUT_LIMIT_MS: u64</code> |

## xws-content-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn content_digest(canonical: &amp;[u8]) -&gt; [u8; 32]</code> |

## xws-content-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub const CONTENT_DOMAIN: &amp;[u8]</code> |

## xws-content-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub enum ContentKind</code> |

## xws-content-kind-byte

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-kind-byte</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub const fn byte(self) -&gt; u8</code> |

## xws-content-kind-from-byte

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-kind-from-byte</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub const fn from_byte(byte: u8) -&gt; Option&lt;Self&gt;</code> |

## xws-content-register

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-register</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub fn register(routes: &amp;mut RouteTable, gate: &amp;Arc&lt;PaymentGate&gt;, fetcher: &amp;Arc&lt;Fetcher&gt;, store: &amp;Arc&lt;ContentStore&gt;) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-content-store

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-store</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub struct ContentStore</code> |

## xws-content-store-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-store-directory</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub fn directory(&amp;self) -&gt; &amp;Path</code> |

## xws-content-store-get

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-store-get</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub fn get(&amp;self, digest: &amp;[u8; 32]) -&gt; io::Result&lt;Option&lt;Vec&lt;u8&gt;&gt;&gt;</code> |

## xws-content-store-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-store-handle</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub fn handle(&amp;self, request: &amp;Request) -&gt; Response</code> |

## xws-content-store-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-store-open</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub fn open(data_dir: &amp;Path, peers: &amp;[String]) -&gt; io::Result&lt;Self&gt;</code> |

## xws-content-store-put

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-store-put</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub fn put(&amp;self, bytes: &amp;[u8]) -&gt; io::Result&lt;[u8; 32]&gt;</code> |

## xws-content-store-retrieve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-content-store-retrieve</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub fn retrieve(&amp;self, digest: &amp;[u8; 32]) -&gt; io::Result&lt;Option&lt;Vec&lt;u8&gt;&gt;&gt;</code> |

## xws-crawl-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-budget</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub struct CrawlBudget</code> |

## xws-crawl-budget-from-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-budget-from-config</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub const fn from_config(config: &amp;CrawlConfig) -&gt; Self</code> |

## xws-crawl-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-config</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub struct CrawlConfig</code> |

## xws-crawl-config-politeness-delay

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-config-politeness-delay</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const fn politeness_delay(&amp;self) -&gt; Duration</code> |

## xws-crawl-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub enum CrawlError</code> |

## xws-crawl-report

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-report</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub struct CrawlReport</code> |

## xws-crawl-report-indexed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-report-indexed</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn indexed(&amp;self) -&gt; impl Iterator&lt;Item = &amp;CrawledPage&gt;</code> |

## xws-crawl-report-refusal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-report-refusal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn refusal(&amp;self, url: &amp;str) -&gt; Option&lt;FetchError&gt;</code> |

## xws-crawl-report-visited

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawl-report-visited</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn visited(&amp;self) -&gt; Vec&lt;&amp;str&gt;</code> |

## xws-crawled-page

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawled-page</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub struct CrawledPage</code> |

## xws-crawler

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawler</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub struct Crawler</code> |

## xws-crawler-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawler-budget</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub const fn budget(&amp;self) -&gt; &amp;CrawlBudget</code> |

## xws-crawler-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawler-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn new(budget: CrawlBudget, fetcher: Arc&lt;Fetcher&gt;, index: Arc&lt;WebIndex&gt;) -&gt; Result&lt;Self, CrawlError&gt;</code> |

## xws-crawler-run-cycle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawler-run-cycle</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn run_cycle(&amp;mut self, seeds: &amp;[String]) -&gt; Result&lt;CrawlReport, CrawlError&gt;</code> |

## xws-crawler-run-every

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-crawler-run-every</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn run_every(&amp;mut self, seeds: &amp;[String], interval: Duration, stop: &amp;StopSignal, mut report: impl FnMut(Result&lt;CrawlReport, CrawlError&gt;)) -&gt; u64</code> |

## xws-credential

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-credential</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub struct Credential</code> |

## xws-credential-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-credential-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn decode(plaintext: &amp;[u8], public: &amp;[ApiHeader]) -&gt; Result&lt;Self, ApiError&gt;</code> |

## xws-credential-encode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-credential-encode</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn encode(headers: &amp;[ApiHeader]) -&gt; Result&lt;Zeroizing&lt;Vec&lt;u8&gt;&gt;, ApiError&gt;</code> |

## xws-credential-names

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-credential-names</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn names(&amp;self) -&gt; Vec&lt;&amp;str&gt;</code> |

## xws-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn decode(bytes: &amp;[u8], charset: Option&lt;&amp;str&gt;) -&gt; Result&lt;String, CanonicalError&gt;</code> |

## xws-decode-grant

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-decode-grant</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn decode_grant(bytes: &amp;[u8], actor: &amp;str) -&gt; Option&lt;Grant&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-decode-request-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-decode-request-record</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub fn decode_request_record(program_id: [u8; 32], sequence: u64, data: &amp;[u8]) -&gt; Result&lt;ProgramRequest, KernelError&gt;</code> |

## xws-decode-request-view

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-decode-request-view</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn decode_request_view(answer: &amp;[u8], request_id: u64) -&gt; Option&lt;RequestView&gt;</code> |

## xws-decode-requested

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-decode-requested</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn decode_requested(log: &amp;Value) -&gt; Result&lt;WebRequest, EvmError&gt;</code> |

## xws-default-crawl-interval-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-default-crawl-interval-seconds</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const DEFAULT_CRAWL_INTERVAL_SECONDS: u64</code> |

## xws-default-draw-fee-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-default-draw-fee-limit</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const DEFAULT_DRAW_FEE_LIMIT: u128</code> |

## xws-destination-permitted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-destination-permitted</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn destination_permitted(address: IpAddr, allow_loopback: bool) -&gt; bool</code> |

## xws-digest-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-digest-hex</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn digest_hex(digest: &amp;[u8; 32]) -&gt; String</code> |

## xws-discard

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-discard</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub enum Discard</code> |

## xws-discard-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-discard-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

## xws-discarded

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-discarded</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub struct Discarded</code> |

## xws-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const DOMAIN: &amp;[u8]</code> |

## xws-draw-validity-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-draw-validity-ms</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const DRAW_VALIDITY_MS: u64</code> |

## xws-dynamic-fee-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-dynamic-fee-type</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const DYNAMIC_FEE_TYPE: u8</code> |

## xws-envelope

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub struct Envelope</code> |

## xws-envelope-aad

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-aad</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn envelope_aad(attestor: &amp;[u8; 20], origin: &amp;str) -&gt; Vec&lt;u8&gt;</code> |

## xws-envelope-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn bytes(&amp;self) -&gt; Vec&lt;u8&gt;</code> |

## xws-envelope-info

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-info</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const ENVELOPE_INFO: &amp;[u8]</code> |

## xws-envelope-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-key</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn envelope_key(shared_x: &amp;[u8], ephemeral_key: &amp;[u8]) -&gt; Result&lt;Zeroizing&lt;[u8; 32]&gt;, ApiError&gt;</code> |

## xws-envelope-key-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-key-length</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const ENVELOPE_KEY_LENGTH: usize</code> |

## xws-envelope-nonce-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-nonce-length</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const ENVELOPE_NONCE_LENGTH: usize</code> |

## xws-envelope-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-open</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn open(&amp;self, key: &amp;SigningKey, origin: &amp;str) -&gt; Result&lt;Zeroizing&lt;Vec&lt;u8&gt;&gt;, ApiError&gt;</code> |

## xws-envelope-overhead

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-overhead</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const ENVELOPE_OVERHEAD: usize</code> |

## xws-envelope-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn parse(raw: &amp;[u8]) -&gt; Result&lt;Self, String&gt;</code> |

## xws-envelope-shared-x

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-shared-x</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn envelope_shared_x(secret: &amp;NonZeroScalar, public: &amp;PublicKey) -&gt; Result&lt;Zeroizing&lt;[u8; 32]&gt;, ApiError&gt;</code> |

## xws-envelope-tag-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-envelope-tag-length</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const ENVELOPE_TAG_LENGTH: usize</code> |

## xws-es-number

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-es-number</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn es_number(value: f64) -&gt; String</code> |

## xws-evm-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-config</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub struct EvmConfig</code> |

## xws-evm-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub enum EvmError</code> |

## xws-evm-requester

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-requester</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn evm_requester(address: [u8; 20]) -&gt; [u8; 32]</code> |

## xws-evm-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub struct EvmRpc</code> |

## xws-evm-rpc-block-number

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-block-number</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn block_number(&amp;self) -&gt; Result&lt;u64, EvmError&gt;</code> |

## xws-evm-rpc-call

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-call</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn call(&amp;self, method: &amp;str, params: &amp;Value) -&gt; Result&lt;Value, EvmError&gt;</code> |

## xws-evm-rpc-eth-call

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-eth-call</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn eth_call(&amp;self, to: [u8; 20], data: &amp;[u8]) -&gt; Result&lt;Vec&lt;u8&gt;, EvmError&gt;</code> |

## xws-evm-rpc-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn new(endpoint: &amp;str) -&gt; Result&lt;Self, EvmError&gt;</code> |

## xws-evm-rpc-quantity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-quantity</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn quantity(&amp;self, method: &amp;str, params: &amp;Value) -&gt; Result&lt;u128, EvmError&gt;</code> |

## xws-exact

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-exact</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const EXACT: &amp;str</code> |

## xws-extract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-extract</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/extract.rs](../../../services/interop/crates/x-websearch/src/extract.rs) | <code>pub fn extract(content_type: Option&lt;&amp;str&gt;, body: &amp;[u8]) -&gt; Result&lt;Extracted, ExtractError&gt;</code> |

## xws-extract-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-extract-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/extract.rs](../../../services/interop/crates/x-websearch/src/extract.rs) | <code>pub enum ExtractError</code> |

## xws-extract-error-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-extract-error-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/extract.rs](../../../services/interop/crates/x-websearch/src/extract.rs) | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

## xws-extracted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-extracted</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/extract.rs](../../../services/interop/crates/x-websearch/src/extract.rs) | <code>pub struct Extracted</code> |

## xws-fees

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fees</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub struct Fees</code> |

## xws-fetch-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetch-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub enum FetchError</code> |

## xws-fetch-error-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetch-error-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

## xws-fetch-error-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetch-error-status</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub const fn status(self) -&gt; u16</code> |

## xws-fetch-limits

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetch-limits</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub struct FetchLimits</code> |

## xws-fetch-limits-connect-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetch-limits-connect-timeout</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const fn connect_timeout(&amp;self) -&gt; Duration</code> |

## xws-fetch-limits-total-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetch-limits-total-timeout</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const fn total_timeout(&amp;self) -&gt; Duration</code> |

## xws-fetch-route

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetch-route</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn fetch_route(fetcher: &amp;Fetcher, store: &amp;ContentStore, request: &amp;Request) -&gt; Response</code> |

## xws-fetched-page

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetched-page</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub struct FetchedPage</code> |

## xws-fetcher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetcher</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub struct Fetcher</code> |

## xws-fetcher-destination

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetcher-destination</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn destination(&amp;self, url: &amp;Url) -&gt; Result&lt;SocketAddr, FetchError&gt;</code> |

## xws-fetcher-fetch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetcher-fetch</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn fetch(&amp;self, url: &amp;str) -&gt; Result&lt;FetchedPage, FetchError&gt;</code> |

## xws-fetcher-limits

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetcher-limits</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub const fn limits(&amp;self) -&gt; &amp;FetchLimits</code> |

## xws-fetcher-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fetcher-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn new(limits: FetchLimits) -&gt; Result&lt;Self, FetchError&gt;</code> |

## xws-fields

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fields</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub struct Fields</code> |

## xws-fulfil-calldata

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fulfil-calldata</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn fulfil_calldata(ready: &amp;Ready) -&gt; Vec&lt;u8&gt;</code> |

## xws-fulfil-gas-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fulfil-gas-limit</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn fulfil_gas_limit(calldata: &amp;[u8], signatures: usize, callback_gas: u64) -&gt; u64</code> |

## xws-fulfil-signature

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-fulfil-signature</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const FULFIL_SIGNATURE: &amp;str</code> |

## xws-gate-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-gate-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub enum GateError</code> |

## xws-gateway-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-gateway-config</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub struct GatewayConfig</code> |

| Field | Rule |
|---|---|
| interface | x-websearch reads one JSON configuration file and refuses unknown fields. |
| storage | none |
| errors | ConfigError naming the refused field. |
| events | none |
| gas | none |
| activation | Read at start with --config. |
| invariants | Key material never appears in the configuration. |
| pause | none |

## xws-gateway-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub struct GatewayRpc</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-gateway-rpc-call

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc-call</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn call(&amp;self, method: &amp;str, params: &amp;Value) -&gt; Option&lt;RpcAnswer&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-gateway-rpc-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn new(endpoint: &amp;str) -&gt; Result&lt;Self, GateError&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-get-attestors-signature

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-get-attestors-signature</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const GET_ATTESTORS_SIGNATURE: &amp;str</code> |

## xws-get-request-signature

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-get-request-signature</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const GET_REQUEST_SIGNATURE: &amp;str</code> |

## xws-grant-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-grant-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const GRANT_BYTES: usize</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-hex</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String</code> |

## xws-hex0x

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-hex0x</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn hex0x(bytes: &amp;[u8]) -&gt; String</code> |

## xws-html-text

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-html-text</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/extract.rs](../../../services/interop/crates/x-websearch/src/extract.rs) | <code>pub fn html_text(source: &amp;str) -&gt; String</code> |

## xws-http-client

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-http-client</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub struct HttpClient</code> |

## xws-http-client-get

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-http-client-get</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn get(&amp;self, url: &amp;Url, address: SocketAddr, deadline: Instant, body_limit: usize, headers: &amp;[(&amp;str, &amp;str)]) -&gt; Result&lt;HttpResponse, FetchError&gt;</code> |

## xws-http-client-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-http-client-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn new(connect_timeout: Duration) -&gt; Result&lt;Self, FetchError&gt;</code> |

## xws-http-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-http-response</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub struct HttpResponse</code> |

## xws-http-response-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-http-response-header</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn header(&amp;self, name: &amp;str) -&gt; Option&lt;&amp;str&gt;</code> |

## xws-index-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-index-directory</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub const INDEX_DIRECTORY: &amp;str</code> |

## xws-index-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-index-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub enum IndexError</code> |

## xws-index-error-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-index-error-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub const fn code(&amp;self) -&gt; &amp;'static str</code> |

## xws-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub struct Journal</code> |

## xws-journal-entry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-entry</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub struct JournalEntry</code> |

## xws-journal-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-load</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn load(&amp;self, key: impl Into&lt;JournalKey&gt;) -&gt; Result&lt;Option&lt;JournalEntry&gt;, SubmitError&gt;</code> |

## xws-journal-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-open</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn open(directory: &amp;Path) -&gt; io::Result&lt;Self&gt;</code> |

## xws-journal-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-path</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn path(&amp;self, key: impl Into&lt;JournalKey&gt;) -&gt; PathBuf</code> |

## xws-journal-requests

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-requests</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn requests(&amp;self) -&gt; Result&lt;Vec&lt;u64&gt;, SubmitError&gt;</code> |

## xws-journal-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-state</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub enum JournalState</code> |

## xws-journal-state-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-state-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

## xws-journal-state-completed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-state-completed</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const fn completed(self) -&gt; bool</code> |

## xws-journal-store

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-journal-store</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn store(&amp;self, entry: &amp;JournalEntry) -&gt; Result&lt;(), SubmitError&gt;</code> |

## xws-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-json</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub enum Json</code> |

## xws-keccak

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-keccak</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn keccak(bytes: &amp;[u8]) -&gt; [u8; 32]</code> |

## xws-kernel-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-kernel-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub enum KernelError</code> |

## xws-kernel-request-topic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-kernel-request-topic</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub fn request_topic(topic: &amp;[u8]) -&gt; bool</code> |

## xws-key-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-key-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub struct KeyError</code> |

## xws-key-files

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-key-files</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub struct KeyFiles</code> |

## xws-key-files-from-env

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-key-files-from-env</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub fn from_env() -&gt; Result&lt;Self, KeyError&gt;</code> |

## xws-key-files-from-lookup

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-key-files-from-lookup</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub fn from_lookup(lookup: impl Fn(&amp;str) -&gt; Option&lt;OsString&gt;) -&gt; Result&lt;Self, KeyError&gt;</code> |

## xws-key-files-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-key-files-load</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub fn load(&amp;self) -&gt; Result&lt;Keys, KeyError&gt;</code> |

## xws-key-refusal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-key-refusal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub enum KeyRefusal</code> |

## xws-key-role

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-key-role</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub enum KeyRole</code> |

## xws-key-role-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-key-role-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub const fn variable(self) -&gt; &amp;'static str</code> |

## xws-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-keys</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub struct Keys</code> |

## xws-keys-attestor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-keys-attestor</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub const fn attestor(&amp;self) -&gt; Option&lt;&amp;SecpKey&gt;</code> |

## xws-keys-receiver

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-keys-receiver</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub const fn receiver(&amp;self) -&gt; &amp;SecpKey</code> |

## xws-keys-submitter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-keys-submitter</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub const fn submitter(&amp;self) -&gt; Option&lt;&amp;SecpKey&gt;</code> |

## xws-kind-api

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-kind-api</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const KIND_API: u8</code> |

## xws-level

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-level</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub enum Level</code> |

## xws-level-majority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-level-majority</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const LEVEL_MAJORITY: u8</code> |

## xws-level-single

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-level-single</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const LEVEL_SINGLE: u8</code> |

## xws-limits

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-limits</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub struct Limits</code> |

## xws-links

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-links</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn links(html: &amp;str) -&gt; Vec&lt;String&gt;</code> |

## xws-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-load</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Config, ConfigError&gt;</code> |

## xws-max-api-body-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-api-body-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_API_BODY_BYTES: usize</code> |

## xws-max-api-header-name-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-api-header-name-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_API_HEADER_NAME_BYTES: usize</code> |

## xws-max-api-header-value-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-api-header-value-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_API_HEADER_VALUE_BYTES: usize</code> |

## xws-max-api-headers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-api-headers</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_API_HEADERS: usize</code> |

## xws-max-api-pointer-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-api-pointer-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_API_POINTER_BYTES: usize</code> |

## xws-max-api-pointers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-api-pointers</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_API_POINTERS: usize</code> |

## xws-max-api-url-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-api-url-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_API_URL_BYTES: usize</code> |

## xws-max-attempts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-attempts</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub const MAX_ATTEMPTS: u32</code> |

## xws-max-block-range

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-block-range</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub const MAX_BLOCK_RANGE: u64</code> |

## xws-max-config-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-config-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_CONFIG_BYTES: usize</code> |

## xws-max-confirmations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-confirmations</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_CONFIRMATIONS: u32</code> |

## xws-max-content-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-content-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub const MAX_CONTENT_BYTES: usize</code> |

## xws-max-crawl-interval-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-crawl-interval-seconds</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_CRAWL_INTERVAL_SECONDS: u64</code> |

## xws-max-credential-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-credential-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_CREDENTIAL_BYTES: usize</code> |

## xws-max-credential-headers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-credential-headers</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_CREDENTIAL_HEADERS: usize</code> |

## xws-max-depth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-depth</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_DEPTH: u32</code> |

## xws-max-envelope-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-envelope-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_ENVELOPE_BYTES: usize</code> |

## xws-max-envelopes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-envelopes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_ENVELOPES: usize</code> |

## xws-max-json-depth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-json-depth</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const MAX_JSON_DEPTH: usize</code> |

## xws-max-key-file-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-key-file-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub const MAX_KEY_FILE_BYTES: usize</code> |

## xws-max-links-per-page

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-links-per-page</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub const MAX_LINKS_PER_PAGE: usize</code> |

## xws-max-observation-signatures

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-observation-signatures</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub const MAX_OBSERVATION_SIGNATURES: usize = 16</code> |

## xws-max-pages-per-cycle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-pages-per-cycle</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_PAGES_PER_CYCLE: u32</code> |

## xws-max-payload-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-payload-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub const MAX_PAYLOAD_BYTES: usize</code> |

## xws-max-peers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-peers</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_PEERS: usize</code> |

## xws-max-politeness-delay-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-politeness-delay-ms</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_POLITENESS_DELAY_MS: u64</code> |

## xws-max-query-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-query-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub const MAX_QUERY_BYTES: usize</code> |

## xws-max-query-terms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-query-terms</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub const MAX_QUERY_TERMS: usize</code> |

## xws-max-record-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-record-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const MAX_RECORD_BYTES: usize</code> |

## xws-max-response-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-response-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const MAX_RESPONSE_BYTES: usize</code> |

## xws-max-results

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-results</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub const MAX_RESULTS: usize</code> |

## xws-max-seeds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-seeds</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_SEEDS: usize</code> |

## xws-max-title-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-title-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub const MAX_TITLE_BYTES: usize</code> |

## xws-max-url-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-max-url-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const MAX_URL_BYTES: usize</code> |

## xws-media-type-essence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-media-type-essence</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn media_type_essence(media_type: &amp;str) -&gt; Result&lt;String, CanonicalError&gt;</code> |

## xws-metered

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-metered</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const METERED: &amp;str</code> |

## xws-method-get

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-method-get</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const METHOD_GET: u8</code> |

## xws-method-post

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-method-post</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub const METHOD_POST: u8</code> |

## xws-network-word

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-network-word</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn network_word(id: u64) -&gt; [u8; 32]</code> |

## xws-observation-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-observation-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub fn observation_bytes(network_id: u32, request: &amp;ProgramRequest, ready: &amp;Ready) -&gt; Result&lt;Vec&lt;u8&gt;, KernelError&gt;</code> |

## xws-observation-header-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-observation-header-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub const OBSERVATION_HEADER_BYTES: usize = 146</code> |

## xws-offer-timeout-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-offer-timeout-seconds</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const OFFER_TIMEOUT_SECONDS: u32</code> |

## xws-open-envelope

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-open-envelope</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn open_envelope(raw: &amp;[u8], key: &amp;SigningKey, origin: &amp;str) -&gt; Result&lt;Zeroizing&lt;Vec&lt;u8&gt;&gt;, ApiError&gt;</code> |

## xws-origin-evm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-origin-evm</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const ORIGIN_EVM: u8</code> |

## xws-origin-program

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-origin-program</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const ORIGIN_PROGRAM: u8</code> |

## xws-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub enum Outcome</code> |

## xws-outcome-completed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-outcome-completed</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const fn completed(self) -&gt; bool</code> |

## xws-page-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-page-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub enum PageOutcome</code> |

## xws-parse-content-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-parse-content-type</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/extract.rs](../../../services/interop/crates/x-websearch/src/extract.rs) | <code>pub fn parse_content_type(value: &amp;str) -&gt; Result&lt;(String, Option&lt;String&gt;), ExtractError&gt;</code> |

## xws-parse-hex32

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-parse-hex32</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub fn parse_hex32(text: &amp;str) -&gt; Option&lt;[u8; 32]&gt;</code> |

## xws-parse-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-parse-json</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn parse_json(text: &amp;str) -&gt; Result&lt;Json, ApiError&gt;</code> |

## xws-parse-quantity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-parse-quantity</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn parse_quantity(text: &amp;str) -&gt; Option&lt;u128&gt;</code> |

## xws-payment-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-config</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub struct PaymentConfig</code> |

## xws-payment-gate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub struct PaymentGate</code> |

## xws-payment-gate-assets

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-assets</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const fn assets(&amp;self) -&gt; &amp;AcceptedAssets</code> |

## xws-payment-gate-draw-fee-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-draw-fee-limit</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const fn draw_fee_limit(&amp;self) -&gt; u128</code> |

## xws-payment-gate-install

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-install</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn install(gate: &amp;Arc&lt;Self&gt;, routes: &amp;mut RouteTable, route: Route, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-payment-gate-install-validated

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-install-validated</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn install_validated(gate: &amp;Arc&lt;Self&gt;, routes: &amp;mut RouteTable, route: Route, validate: impl Fn(&amp;Request) -&gt; Result&lt;(), Response&gt; + Send + Sync + 'static, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-payment-gate-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn new(config: &amp;Config, receiver: &amp;SigningKey, conformance: ConformanceSuite, clock: Clock) -&gt; Result&lt;Self, GateError&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-paid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-paid</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn paid(gate: &amp;Arc&lt;Self&gt;, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static</code> |

## xws-payment-gate-payee

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payee</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn payee(&amp;self, asset: &amp;AcceptedAsset) -&gt; [u8; 32]</code> |

## xws-payment-gate-payer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn payer(&amp;self) -&gt; Option&lt;[u8; 20]&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-payment-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payment-required</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn payment_required( &amp;self, request: &amp;Request, _payer: Option&lt;[u8; 20]&gt;, ) -&gt; Result&lt;PaymentRequired, Response&gt;</code> |

## xws-payment-gate-purpose

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-purpose</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const fn purpose(&amp;self) -&gt; [u8; 32]</code> |

## xws-payment-gate-recover-pending-deliveries

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-recover-pending-deliveries</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn recover_pending_deliveries(&amp;self) -&gt; io::Result&lt;()&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-settle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-settle</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn settle(&amp;self, request: &amp;Request, release: &amp;dyn Fn(&amp;Request) -&gt; Response) -&gt; Response</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-store

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-store</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const fn store(&amp;self) -&gt; &amp;PaymentStore</code> |

## xws-payment-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-record</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub struct PaymentRecord</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-required</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const PAYMENT_REQUIRED: &amp;str</code> |

## xws-payment-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-response</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const PAYMENT_RESPONSE: &amp;str</code> |

## xws-payment-signature

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-signature</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub const PAYMENT_SIGNATURE: &amp;str</code> |

## xws-payment-store

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-store</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub struct PaymentStore</code> |

## xws-payment-store-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-store-load</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn load(&amp;self, key: &amp;[u8; 32]) -&gt; io::Result&lt;Option&lt;PaymentRecord&gt;&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-store-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-store-open</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn open(data_dir: &amp;Path) -&gt; io::Result&lt;Self&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-store-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-store-release</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn release(&amp;self, key: &amp;[u8; 32], receipt_digest: &amp;[u8; 32]) -&gt; io::Result&lt;bool&gt;</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-peer-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-peer-header</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/content.rs](../../../services/interop/crates/x-websearch/src/content.rs) | <code>pub const PEER_HEADER: &amp;str</code> |

## xws-pending

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-pending</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub struct Pending</code> |

## xws-preimage-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-preimage-length</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const PREIMAGE_LENGTH: usize</code> |

## xws-program-attest-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-attest-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub enum ProgramAttestError</code> |

## xws-program-attestation-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-attestation-path</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub const PROGRAM_ATTESTATION_PATH: &amp;str</code> |

## xws-program-discarded

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-discarded</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub struct ProgramDiscarded</code> |

## xws-program-exchange

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub struct ProgramExchange</code> |

## xws-program-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-key</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub type ProgramKey = ([u8; 32], u64)</code> |

## xws-program-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-record</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub fn program_record(answer: &amp;Answer) -&gt; Value</code> |

## xws-program-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-request</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub struct ProgramRequest</code> |

## xws-program-request-attestation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-request-attestation</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub fn attestation(&amp;self, network_id: u32, content_digest: [u8; 32], response: &amp;[u8], full_length: u32) -&gt; Attestation</code> |

## xws-purpose-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-purpose-hash</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn purpose_hash(receiver_address: &amp;[u8; 20]) -&gt; [u8; 32]</code> |

## xws-quantity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-quantity</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn quantity(value: u128) -&gt; String</code> |

## xws-query-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-query-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub enum QueryError</code> |

## xws-query-terms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-query-terms</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub fn query_terms(index: &amp;WebIndex, query: &amp;str) -&gt; Result&lt;Vec&lt;String&gt;, SearchError&gt;</code> |

## xws-ready

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-ready</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub struct Ready</code> |

## xws-ready-from-signatures

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-ready-from-signatures</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn from_signatures(answer: &amp;Answer, set: &amp;AttestorSet, signatures: &amp;BTreeMap&lt;[u8; 20], [u8; SIGNATURE_LENGTH]&gt;) -&gt; Option&lt;Self&gt;</code> |

## xws-receiver-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-receiver-key-file</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub const RECEIVER_KEY_FILE: &amp;str</code> |

## xws-recover-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-recover-signer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn recover_signer(digest: &amp;[u8; 32], signature: &amp;[u8; SIGNATURE_LENGTH]) -&gt; Result&lt;[u8; 20], SignatureError&gt;</code> |

## xws-redirect-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-redirect-limit</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const REDIRECT_LIMIT: u8</code> |

## xws-refusal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-refusal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub enum Refusal</code> |

## xws-register

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-register</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn register(routes: &amp;mut RouteTable, exchange: &amp;Arc&lt;SignatureExchange&gt;) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-remove-controls

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-remove-controls</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn remove_controls(text: &amp;str) -&gt; String</code> |

## xws-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub struct Request</code> |

## xws-request-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-header</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn header(&amp;self, name: &amp;str) -&gt; Option&lt;&amp;str&gt;</code> |

## xws-request-query-param

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-query-param</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn query_param(&amp;self, name: &amp;str) -&gt; Result&lt;Option&lt;String&gt;, QueryError&gt;</code> |

## xws-request-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-status</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn request_status(rpc: &amp;EvmRpc, request_id: u64) -&gt; Result&lt;u8, EvmError&gt;</code> |

## xws-request-status-at-depth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-status-at-depth</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn request_status_at_depth(rpc: &amp;EvmRpc, request_id: u64, confirmations: u64) -&gt; Result&lt;u8, EvmError&gt;</code> |

## xws-request-topic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-topic</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const REQUEST_TOPIC: &amp;[u8] = b"PAXEERX_WEB_REQUEST_V1"</code> |

## xws-request-topics

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-topics</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const REQUEST_TOPICS: [&amp;[u8]; 1] = [REQUEST_TOPIC]</code> |

## xws-request-view

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-view</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub struct RequestView</code> |

## xws-request-view-words

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-view-words</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const REQUEST_VIEW_WORDS: usize</code> |

## xws-request-watcher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub struct RequestWatcher</code> |

## xws-request-watcher-canonical

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-canonical</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn canonical(&amp;mut self, request: &amp;WebRequest) -&gt; Result&lt;Canonical, EvmError&gt;</code> |

## xws-request-watcher-confirmations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-confirmations</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub const fn confirmations(&amp;self) -&gt; u64</code> |

## xws-request-watcher-eligible

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-eligible</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn eligible(&amp;self, request_id: u64) -&gt; bool</code> |

## xws-request-watcher-fail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-fail</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn fail(&amp;mut self, request_id: u64, terminal: bool, reason: &amp;str) -&gt; Result&lt;bool, EvmError&gt;</code> |

## xws-request-watcher-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn journal(&amp;self) -&gt; Vec&lt;Pending&gt;</code> |

## xws-request-watcher-last-head

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-last-head</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub const fn last_head(&amp;self) -&gt; u64</code> |

## xws-request-watcher-next-block

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-next-block</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub const fn next_block(&amp;self) -&gt; Option&lt;u64&gt;</code> |

## xws-request-watcher-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-open</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn open(rpc: EvmRpc, confirmations: u32, state_dir: &amp;Path, start: Option&lt;u64&gt;) -&gt; io::Result&lt;Self&gt;</code> |

## xws-request-watcher-poll

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-poll</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn poll(&amp;mut self) -&gt; Result&lt;Vec&lt;WebRequest&gt;, EvmError&gt;</code> |

## xws-request-watcher-reorged

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-reorged</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn reorged(&amp;mut self, request_id: u64) -&gt; Result&lt;(), EvmError&gt;</code> |

## xws-request-watcher-retire

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-retire</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn retire(&amp;mut self, request_id: u64) -&gt; Result&lt;(), EvmError&gt;</code> |

## xws-request-watcher-transition

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-transition</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn transition(&amp;mut self, request_id: u64, stage: WorkStage) -&gt; Result&lt;(), EvmError&gt;</code> |

## xws-request-watcher-with-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-with-chain-id</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn with_chain_id(mut self, chain_id: u64) -&gt; io::Result&lt;Self&gt;</code> |

## xws-request-watcher-work

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-work</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn work(&amp;self) -&gt; Vec&lt;WebRequest&gt;</code> |

## xws-requested-event

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-requested-event</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub const REQUESTED_EVENT: &amp;str</code> |

## xws-requested-topic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-requested-topic</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn requested_topic() -&gt; [u8; 32]</code> |

## xws-resolve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-resolve</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn resolve&lt;'a&gt;(document: &amp;'a Json, pointer: &amp;str) -&gt; Option&lt;&amp;'a Json&gt;</code> |

## xws-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-response</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub struct Response</code> |

## xws-response-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-response-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn error(status: u16, code: &amp;str) -&gt; Self</code> |

## xws-response-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-response-json</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn json(status: u16, body: Vec&lt;u8&gt;) -&gt; Self</code> |

## xws-response-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-response-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn new(status: u16, content_type: &amp;str, body: Vec&lt;u8&gt;) -&gt; Self</code> |

## xws-response-with-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-response-with-header</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn with_header(mut self, name: &amp;str, value: &amp;str) -&gt; Self</code> |

## xws-resumed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-resumed</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub type Resumed = Vec&lt;(u64, Result&lt;Outcome, SubmitError&gt;)&gt;</code> |

## xws-robots

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub struct Robots</code> |

## xws-robots-allow-all

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots-allow-all</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub const fn allow_all() -&gt; Self</code> |

## xws-robots-allows

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots-allows</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub fn allows(&amp;self, path: &amp;str) -&gt; bool</code> |

## xws-robots-cache

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots-cache</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub struct RobotsCache</code> |

## xws-robots-cache-get

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots-cache-get</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub fn get(&amp;self, origin: &amp;str) -&gt; Option&lt;Robots&gt;</code> |

## xws-robots-cache-insert

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots-cache-insert</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub fn insert(&amp;self, origin: &amp;str, robots: Robots)</code> |

## xws-robots-cache-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots-cache-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub fn new() -&gt; Self</code> |

## xws-robots-disallow-all

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots-disallow-all</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub fn disallow_all() -&gt; Self</code> |

## xws-robots-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-robots-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub fn parse(text: &amp;str) -&gt; Self</code> |

## xws-root-conformance-suite

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-root-conformance-suite</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/lib.rs](../../../services/interop/crates/x-websearch/src/lib.rs) | <code>pub fn conformance_suite() -&gt; Result&lt;ConformanceSuite, AdapterError&gt;</code> |

## xws-route

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub enum Route</code> |

## xws-route-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub enum RouteError</code> |

## xws-route-is-paid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-is-paid</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub const fn is_paid(self) -&gt; bool</code> |

## xws-route-table

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub struct RouteTable</code> |

## xws-route-table-dispatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table-dispatch</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn dispatch(&amp;self, request: &amp;Request) -&gt; Response</code> |

| Field | Rule |
|---|---|
| interface | x-websearch serves /health, /readyz, /search, /fetch, /content and /program-attestations. |
| storage | none |
| errors | HTTP error bodies with stable codes. |
| events | none |
| gas | none |
| activation | At start. |
| invariants | /content is served before serving eligibility; paid routes wait for it. |
| pause | none |

## xws-route-table-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn new() -&gt; Self</code> |

## xws-route-table-set

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn set(&amp;mut self, route: Route, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-route-table-set-attestations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-attestations</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn set_attestations(&amp;mut self, handler: impl Fn(u64) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-route-table-set-health

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-health</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn set_health(&amp;mut self, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-route-table-set-program-attestations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-program-attestations</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn set_program_attestations(&amp;mut self, handler: impl Fn([u8; 32], u64) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Field | Rule |
|---|---|
| interface | x-websearch serves /health, /readyz, /search, /fetch, /content and /program-attestations. |
| storage | none |
| errors | HTTP error bodies with stable codes. |
| events | none |
| gas | none |
| activation | At start. |
| invariants | /content is served before serving eligibility; paid routes wait for it. |
| pause | none |

## xws-route-table-set-readiness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-readiness</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn set_readiness(&amp;mut self, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-route-table-set-serving-eligibility

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-serving-eligibility</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn set_serving_eligibility(&amp;mut self, eligibility: impl Fn() -&gt; bool + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-rpc-answer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-rpc-answer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub enum RpcAnswer</code> |

| Field | Rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-running-server

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-running-server</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub struct RunningServer</code> |

## xws-running-server-local-addr

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-running-server-local-addr</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub const fn local_addr(&amp;self) -&gt; SocketAddr</code> |

## xws-running-server-shutdown

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-running-server-shutdown</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn shutdown(self) -&gt; io::Result&lt;()&gt;</code> |

## xws-running-server-stopper

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-running-server-stopper</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn stopper(&amp;self) -&gt; Stopper</code> |

## xws-running-server-wait

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-running-server-wait</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn wait(self) -&gt; io::Result&lt;()&gt;</code> |

## xws-scored-result

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-scored-result</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub struct ScoredResult</code> |

## xws-seal-envelope-with

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-seal-envelope-with</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn seal_envelope_with(recipient: &amp;PublicKey, ephemeral: &amp;SigningKey, nonce: [u8; ENVELOPE_NONCE_LENGTH], origin: &amp;str, plaintext: &amp;[u8]) -&gt; Result&lt;Vec&lt;u8&gt;, ApiError&gt;</code> |

## xws-search

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub fn search(index: &amp;WebIndex, query: &amp;str) -&gt; Result&lt;Vec&lt;ScoredResult&gt;, SearchError&gt;</code> |

## xws-search-canonical-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-canonical-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub fn search_canonical_bytes(query: &amp;str, results: &amp;[SearchResult]) -&gt; Result&lt;Vec&lt;u8&gt;, CanonicalError&gt;</code> |

## xws-search-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub enum SearchError</code> |

## xws-search-error-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-error-code</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub const fn code(&amp;self) -&gt; &amp;'static str</code> |

## xws-search-error-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-error-status</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub const fn status(&amp;self) -&gt; u16</code> |

## xws-search-media-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-media-type</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub const SEARCH_MEDIA_TYPE: &amp;str</code> |

## xws-search-register

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-register</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub fn register(routes: &amp;mut RouteTable, gate: &amp;Arc&lt;PaymentGate&gt;, index: &amp;Arc&lt;WebIndex&gt;, store: &amp;Arc&lt;ContentStore&gt;) -&gt; Result&lt;(), RouteError&gt;</code> |

## xws-search-result

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-result</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub struct SearchResult</code> |

## xws-search-route

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-route</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub fn search_route(index: &amp;WebIndex, store: &amp;ContentStore, request: &amp;Request) -&gt; Response</code> |

## xws-search-text

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-search-text</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub fn search_text(results: &amp;[SearchResult]) -&gt; Result&lt;String, serde_json::Error&gt;</code> |

## xws-select-answer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-select-answer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/api.rs](../../../services/interop/crates/x-websearch/src/api.rs) | <code>pub fn select_answer(body: &amp;[u8], pointers: &amp;[String]) -&gt; Result&lt;Vec&lt;u8&gt;, ApiError&gt;</code> |

## xws-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn selector(signature: &amp;str) -&gt; [u8; 4]</code> |

## xws-server

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-server</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub struct Server</code> |

## xws-server-bind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-server-bind</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn bind(address: SocketAddr, limits: Limits, routes: RouteTable) -&gt; io::Result&lt;Self&gt;</code> |

## xws-server-local-addr

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-server-local-addr</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn local_addr(&amp;self) -&gt; io::Result&lt;SocketAddr&gt;</code> |

## xws-server-run

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-server-run</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn run(self) -&gt; io::Result&lt;()&gt;</code> |

## xws-server-spawn

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-server-spawn</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn spawn(self) -&gt; io::Result&lt;RunningServer&gt;</code> |

## xws-sign-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-sign-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn sign_digest(key: &amp;SigningKey, digest: &amp;[u8; 32]) -&gt; Result&lt;[u8; SIGNATURE_LENGTH], SignatureError&gt;</code> |

## xws-signature-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub enum SignatureError</code> |

## xws-signature-exchange

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub struct SignatureExchange</code> |

## xws-signature-exchange-accept

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-accept</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn accept(&amp;self, peer: &amp;str, request_id: u64, record: &amp;Value, set: &amp;AttestorSet) -&gt; Result&lt;Option&lt;[u8; 20]&gt;, Discard&gt;</code> |

## xws-signature-exchange-answer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-answer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn answer(&amp;self, request_id: u64) -&gt; Option&lt;Answer&gt;</code> |

## xws-signature-exchange-collect

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-collect</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn collect(&amp;self, request_id: u64, set: &amp;AttestorSet) -&gt; usize</code> |

## xws-signature-exchange-discarded

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-discarded</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn discarded(&amp;self) -&gt; Vec&lt;Discarded&gt;</code> |

## xws-signature-exchange-expire

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-expire</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn expire(&amp;self, height: u64)</code> |

## xws-signature-exchange-forget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-forget</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn forget(&amp;self, request_id: u64)</code> |

## xws-signature-exchange-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-handle</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn handle(&amp;self, request_id: u64) -&gt; Response</code> |

## xws-signature-exchange-log-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-log-path</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn log_path(&amp;self) -&gt; &amp;Path</code> |

## xws-signature-exchange-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-open</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn open(state_dir: &amp;Path, peers: &amp;[String]) -&gt; io::Result&lt;Self&gt;</code> |

## xws-signature-exchange-pending

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-pending</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn pending(&amp;self) -&gt; Vec&lt;u64&gt;</code> |

## xws-signature-exchange-ready

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-ready</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn ready(&amp;self, request_id: u64, set: &amp;AttestorSet) -&gt; Option&lt;Ready&gt;</code> |

## xws-signature-exchange-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-record</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn record(&amp;self, answer: Answer) -&gt; bool</code> |

## xws-signature-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signature-length</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub const SIGNATURE_LENGTH: usize</code> |

## xws-signed-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signed-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub struct SignedTransaction</code> |

## xws-signer-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-signer-address</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn signer_address(key: &amp;SigningKey) -&gt; [u8; 20]</code> |

## xws-snippet-chars

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-snippet-chars</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/search.rs](../../../services/interop/crates/x-websearch/src/search.rs) | <code>pub const SNIPPET_CHARS: usize</code> |

## xws-source-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-source-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub struct SourceIdentity</code> |

## xws-status-fulfilled

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-status-fulfilled</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const STATUS_FULFILLED: u8</code> |

## xws-status-pending

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-status-pending</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const STATUS_PENDING: u8</code> |

## xws-status-refunded

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-status-refunded</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const STATUS_REFUNDED: u8</code> |

## xws-step

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-step</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub enum Step</code> |

## xws-stop-signal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub struct StopSignal</code> |

## xws-stop-signal-is-raised

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal-is-raised</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn is_raised(&amp;self) -&gt; bool</code> |

## xws-stop-signal-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal-new</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn new() -&gt; Self</code> |

## xws-stop-signal-raise

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal-raise</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn raise(&amp;self)</code> |

## xws-stop-signal-wait-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal-wait-timeout</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn wait_timeout(&amp;self, timeout: Duration) -&gt; bool</code> |

## xws-stopper

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stopper</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub struct Stopper</code> |

## xws-stopper-stop

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stopper-stop</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/server.rs](../../../services/interop/crates/x-websearch/src/server.rs) | <code>pub fn stop(&amp;self)</code> |

## xws-stored-answer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stored-answer</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn stored_answer(answer: &amp;[u8]) -&gt; Result&lt;(Vec&lt;u8&gt;, u32), AttestError&gt;</code> |

## xws-stored-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-stored-response</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/attest.rs](../../../services/interop/crates/x-websearch/src/attest.rs) | <code>pub fn stored_response(text: &amp;str) -&gt; Result&lt;(Vec&lt;u8&gt;, u32), AttestError&gt;</code> |

## xws-submit-attestor-set

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submit-attestor-set</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn attestor_set(rpc: &amp;EvmRpc) -&gt; Result&lt;AttestorSet, EvmError&gt;</code> |

## xws-submit-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submit-error</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub enum SubmitError</code> |

## xws-submitter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub struct Submitter</code> |

## xws-submitter-abandon

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-abandon</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn abandon(&amp;self, request_id: u64) -&gt; Result&lt;(), SubmitError&gt;</code> |

## xws-submitter-acknowledge-closed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-acknowledge-closed</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn acknowledge_closed(&amp;self, request_id: u64) -&gt; Result&lt;bool, SubmitError&gt;</code> |

## xws-submitter-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-address</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const fn address(&amp;self) -&gt; [u8; 20]</code> |

## xws-submitter-bind-source

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-bind-source</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn bind_source(&amp;self, request: &amp;crate::watch::WebRequest, source: &amp;crate::watch::SourceIdentity) -&gt; Result&lt;(), SubmitError&gt;</code> |

## xws-submitter-confirm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-confirm</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn confirm(&amp;self, request_id: u64) -&gt; Result&lt;Option&lt;Outcome&gt;, SubmitError&gt;</code> |

## xws-submitter-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

## xws-submitter-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-key-file</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/keys.rs](../../../services/interop/crates/x-websearch/src/keys.rs) | <code>pub const SUBMITTER_KEY_FILE: &amp;str</code> |

## xws-submitter-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-open</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn open(rpc: EvmRpc, key: SigningKey, chain_id: u64, journal_dir: &amp;Path) -&gt; io::Result&lt;Self&gt;</code> |

## xws-submitter-resume

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-resume</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn resume(&amp;self) -&gt; Result&lt;Resumed, SubmitError&gt;</code> |

## xws-submitter-resume-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-resume-request</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn resume_request(&amp;self, request_id: u64) -&gt; Result&lt;Option&lt;Outcome&gt;, SubmitError&gt;</code> |

## xws-submitter-submit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-submit</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn submit(&amp;self, ready: &amp;Ready) -&gt; Result&lt;Outcome, SubmitError&gt;</code> |

## xws-submitter-with-confirmations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-submitter-with-confirmations</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub const fn with_confirmations(mut self, confirmations: u64) -&gt; Self</code> |

## xws-system-clock

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-system-clock</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn system_clock() -&gt; u64</code> |

## xws-title-of

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-title-of</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/crawl.rs](../../../services/interop/crates/x-websearch/src/crawl.rs) | <code>pub fn title_of(text: &amp;str) -&gt; String</code> |

## xws-total-timeout-limit-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-total-timeout-limit-ms</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>pub const TOTAL_TIMEOUT_LIMIT_MS: u64</code> |

## xws-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub struct Transaction</code> |

## xws-transaction-sign

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-transaction-sign</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn sign(&amp;self, key: &amp;SigningKey) -&gt; Result&lt;SignedTransaction, SubmitError&gt;</code> |

## xws-trim-line-ends

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-trim-line-ends</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn trim_line_ends(text: &amp;str) -&gt; String</code> |

## xws-unhex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-unhex</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn unhex(text: &amp;str) -&gt; Option&lt;Vec&lt;u8&gt;&gt;</code> |

## xws-unhex0x

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-unhex0x</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn unhex0x(text: &amp;str) -&gt; Option&lt;Vec&lt;u8&gt;&gt;</code> |

## xws-unify-line-breaks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-unify-line-breaks</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/canonical.rs](../../../services/interop/crates/x-websearch/src/canonical.rs) | <code>pub fn unify_line_breaks(text: &amp;str) -&gt; String</code> |

## xws-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-url</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub struct Url</code> |

## xws-url-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-url-authority</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn authority(&amp;self) -&gt; String</code> |

## xws-url-bare-host

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-url-bare-host</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn bare_host(&amp;self) -&gt; &amp;str</code> |

## xws-url-join

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-url-join</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn join(&amp;self, reference: &amp;str) -&gt; Result&lt;Self, FetchError&gt;</code> |

## xws-url-origin

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-url-origin</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn origin(&amp;self) -&gt; String</code> |

## xws-url-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-url-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, FetchError&gt;</code> |

## xws-url-scheme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-url-scheme</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/fetch.rs](../../../services/interop/crates/x-websearch/src/fetch.rs) | <code>pub const fn scheme(&amp;self) -&gt; &amp;'static str</code> |

## xws-user-agent-token

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-user-agent-token</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/robots.rs](../../../services/interop/crates/x-websearch/src/robots.rs) | <code>pub const USER_AGENT_TOKEN: &amp;str</code> |

## xws-web-index

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub struct WebIndex</code> |

## xws-web-index-commit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-commit</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub fn commit(&amp;self) -&gt; Result&lt;(), IndexError&gt;</code> |

## xws-web-index-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-directory</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub fn directory(&amp;self) -&gt; &amp;Path</code> |

## xws-web-index-documents-for

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-documents-for</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub fn documents_for(&amp;self, url: &amp;str) -&gt; Result&lt;usize, IndexError&gt;</code> |

## xws-web-index-fields

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-fields</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub const fn fields(&amp;self) -&gt; Fields</code> |

## xws-web-index-index

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-index</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub const fn index(&amp;self) -&gt; &amp;Index</code> |

## xws-web-index-num-docs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-num-docs</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub fn num_docs(&amp;self) -&gt; u64</code> |

## xws-web-index-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-open</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub fn open(data_dir: &amp;Path) -&gt; Result&lt;Self, IndexError&gt;</code> |

## xws-web-index-put

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-put</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub fn put(&amp;self, url: &amp;str, title: &amp;str, body: &amp;str) -&gt; Result&lt;(), IndexError&gt;</code> |

## xws-web-index-searcher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-index-searcher</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/index.rs](../../../services/interop/crates/x-websearch/src/index.rs) | <code>pub fn searcher(&amp;self) -&gt; Searcher</code> |

## xws-web-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-web-request</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub struct WebRequest</code> |

## xws-work-stage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-work-stage</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub enum WorkStage</code> |

## xws-xweb-precompile

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-xweb-precompile</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub const XWEB_PRECOMPILE: [u8; 20]</code> |
