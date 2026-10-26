# Interop websearch contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cfg-xws-assets-asset-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-assets-asset-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:423</code> | <code>x-websearch config assets.&lt;SID&#124;PAX&#124;USDC&#124;USDL&gt;.asset_id: 32-byte hex; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-assets-asset-id</code> | notes | 54 reference lines in 13 files; callers lists the first reference in each file |

## cfg-xws-assets-price

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-assets-price</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:808</code> | <code>x-websearch config assets.&lt;symbol&gt;.price: decimal string base units; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-assets-price</code> | notes | 49 reference lines in 13 files; callers lists the first reference in each file |

## cfg-xws-crawl-interval-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-interval-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:770</code> | <code>x-websearch config crawl_interval_seconds: optional u64, default DEFAULT_CRAWL_INTERVAL_SECONDS; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-max-depth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-max-depth</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:754</code> | <code>x-websearch config crawl.max_depth: integer &lt;= MAX_DEPTH; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-pages-per-cycle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-pages-per-cycle</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:752</code> | <code>x-websearch config crawl.pages_per_cycle: positive u32; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-pages-per-host

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-pages-per-host</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:753</code> | <code>x-websearch config crawl.pages_per_host: positive u32 &lt;= pages_per_cycle; unknown fields refused by object.finish</code> |

## cfg-xws-crawl-politeness-delay-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-crawl-politeness-delay-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:759</code> | <code>x-websearch config crawl.politeness_delay_ms: u64 &lt;= MAX_POLITENESS_DELAY_MS; unknown fields refused by object.finish</code> |

## cfg-xws-data-dir

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-data-dir</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:576</code> | <code>x-websearch config data_dir: directory; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-data-dir</code> | notes | 45 reference lines in 21 files; callers lists the first reference in each file |

## cfg-xws-evm-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-evm-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:1004</code> | <code>x-websearch config evm.chain_id: u64; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-evm-chain-id</code> | notes | 48 reference lines in 25 files; callers lists the first reference in each file |

## cfg-xws-evm-confirmations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-evm-confirmations</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:1005</code> | <code>x-websearch config evm.confirmations: positive u32 &lt;= MAX_CONFIRMATIONS; unknown fields refused by object.finish</code> |

## cfg-xws-evm-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-evm-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:1001</code> | <code>x-websearch config evm.endpoint: URL; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-evm-endpoint</code> | notes | 61 reference lines in 21 files; callers lists the first reference in each file |

## cfg-xws-fetch-allow-loopback

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-allow-loopback</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:793</code> | <code>x-websearch config fetch.allow_loopback: bool; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-connect-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-connect-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:781</code> | <code>x-websearch config fetch.connect_timeout_ms: u64 bounded; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-max-body-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-max-body-bytes</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:787</code> | <code>x-websearch config fetch.max_body_bytes: u64 bounded; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-max-redirects

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-max-redirects</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:788</code> | <code>x-websearch config fetch.max_redirects: integer &lt;= REDIRECT_LIMIT; unknown fields refused by object.finish</code> |

## cfg-xws-fetch-total-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-fetch-total-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:783</code> | <code>x-websearch config fetch.total_timeout_ms: u64 bounded; unknown fields refused by object.finish</code> |

## cfg-xws-gateway-authorization-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-gateway-authorization-file</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:864</code> | <code>x-websearch config gateway.authorization_file: optional path; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-gateway-authorization-file</code> | notes | spec/layerx-merge/spec.kvx task 5.6.6 do_4 drops the authorization_file requirement that only the relay read |

## cfg-xws-gateway-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-gateway-endpoint</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/config.rs:863</code> | <code>x-websearch config gateway.endpoint: URL; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-gateway-endpoint</code> | notes | spec/layerx-merge/spec.kvx task 5.3.2 settles over eth_sendRawTransaction and pax_sequencerReceipt instead of the gateway lx_ methods; 61 reference lines in 21 files; callers lists the first reference in each file |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

| Decision | Title | Value | Basis | Affects |
|---|---|---|---|---|
| <code>decision.1124</code> | gateway.endpoint is kept as the settlement endpoint field | The config key gateway.endpoint keeps its name and now names the EVM JSON-RPC endpoint that answers eth_ and pax_sequencerReceipt; no new key is added. | services/interop/crates/x-websearch/src/config.rs:156 defines GatewayConfig with the endpoint the settlement client uses. | <code>interface.11.cfg-xws-gateway-endpoint</code><br><code>task.11.34.4</code> |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.889</code> | title | decision.1124 binds: gateway.endpoint is kept as the settlement endpoint field |
| <code>resolution.17.889</code> | applies_to | decision.1124<br>task.11.34.4 |
| <code>resolution.17.889</code> | source | decision.1124 in fragments/J48/decision.kvx; tasks from affects |

## cfg-xws-gateway-sequencer-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-gateway-sequencer-id</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:423</code> | <code>x-websearch config gateway.sequencer_id: 32-byte hex; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-gateway-sequencer-id</code> | notes | spec/layerx-merge/spec.kvx task 5.4.3 replaces the gateway key fields (services/interop/crates/x-websearch/src/config.rs:873-881) with receipt_keys |

## cfg-xws-gateway-sequencer-public-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-gateway-sequencer-public-key</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:875</code> | <code>x-websearch config gateway.sequencer_public_key: 32-byte hex ed25519 key, weak keys refused; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-gateway-sequencer-public-key</code> | notes | spec/layerx-merge/spec.kvx task 5.4.3 replaces it with receipt_keys entries; 27 reference lines in 16 files; callers lists the first reference in each file |

## cfg-xws-kernel

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-kernel</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:962</code> | <code>x-websearch config kernel: optional object {endpoint, poll_interval_ms, topics, submitter_did, fee_limit}; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-kernel</code> | notes | spec/layerx-merge/spec.kvx task 5.6.6 removes KernelConfig and refuses a kernel key as unknown; 27 reference lines in 5 files; callers lists the first reference in each file |

## cfg-xws-kernel-fee-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-kernel-fee-limit</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:977</code> | <code>x-websearch config kernel.fee_limit: decimal u128; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-kernel-fee-limit</code> | notes | spec/layerx-merge/spec.kvx task 5.6.6 |

## cfg-xws-kernel-network-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-kernel-network-id</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/config.rs:290</code> | <code>x-websearch config kernel_network_id: positive u32; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-kernel-network-id</code> | notes | spec/layerx-merge/spec.kvx task 11.5.121 do_7 makes kernel_network_id optional (absent parses; 0 still refused) and 11.5.120/11.5.126 remove it from the example configs |

Contract context <code>contract_context.11.x-websearch-config</code>:

| Field | Frozen rule |
|---|---|
| interface | x-websearch reads one JSON configuration file and refuses unknown fields. |
| storage | none |
| errors | ConfigError naming the refused field. |
| events | none |
| gas | none |
| activation | Read at start with --config. |
| invariants | Key material never appears in the configuration. |
| pause | none |

## cfg-xws-kernel-poll-interval-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-kernel-poll-interval-ms</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:970</code> | <code>x-websearch config kernel.poll_interval_ms: u64; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-kernel-poll-interval-ms</code> | notes | spec/layerx-merge/spec.kvx task 5.6.6 |

## cfg-xws-kernel-submitter-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-kernel-submitter-did</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:972</code> | <code>x-websearch config kernel.submitter_did: DID; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-kernel-submitter-did</code> | notes | spec/layerx-merge/spec.kvx task 5.6.6 |

## cfg-xws-kernel-topics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-kernel-topics</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:938</code> | <code>x-websearch config kernel.topics: array of strings; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-kernel-topics</code> | notes | spec/layerx-merge/spec.kvx task 5.6.6; 26 reference lines in 11 files; callers lists the first reference in each file |

## cfg-xws-listen

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-listen</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:562</code> | <code>x-websearch config listen: socket address; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-listen</code> | notes | 28 reference lines in 17 files; callers lists the first reference in each file |

## cfg-xws-payment-conformance-suite

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-payment-conformance-suite</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:916</code> | <code>x-websearch config payment.conformance_suite: optional; unknown fields refused by object.finish</code> |

## cfg-xws-payment-draw-fee-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-payment-draw-fee-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:908</code> | <code>x-websearch config payment.draw_fee_limit: optional decimal u128; unknown fields refused by object.finish</code> |

## cfg-xws-payment-payer-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-payment-payer-did</code> | <code>config_key</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:901</code> | <code>x-websearch config payment.payer_did: optional DID; unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-payment-payer-did</code> | notes | spec/layerx-merge/spec.kvx task 11.5.121 do_2 names the payer by EVM address via 0x1004 |

## cfg-xws-peers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-peers</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:741</code> | <code>x-websearch config peers: array of peer endpoint URLs; unknown fields refused by object.finish</code> |

## cfg-xws-receipt-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-receipt-keys</code> | <code>config_key</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/config.rs</code> (planned) | <code>x-websearch config receipt_keys: array of {keyId, publicKey (ed25519), firstHeight, lastHeight}; firstHeight &gt; lastHeight or weak key refused</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-receipt-keys</code> | notes | spec/layerx-merge/spec.kvx task 5.4.3 adds it in place of SequencerTrust (services/interop/crates/x-websearch/src/config.rs:150) |

Contract context <code>contract_context.11.x-websearch-config</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-seeds</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:733</code> | <code>x-websearch config seeds: array of seed URLs (&lt;= MAX_SEEDS); unknown fields refused by object.finish</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-seeds</code> | notes | 30 reference lines in 16 files; callers lists the first reference in each file |

## cli-xws

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-xws</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/main.rs:435</code> | <code>x-websearch --config PATH</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-xws</code> | notes | PATH is the sidecar config JSON (Config::load); 331 reference lines in 71 files; callers lists the first reference in each file |

## env-xws-paxeer-x-source-revision

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-xws-paxeer-x-source-revision</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/main.rs:512</code> | <code>PAXEER_X_SOURCE_REVISION: compile-time option_env! source revision reported in readiness JSON</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-xws-paxeer-x-source-revision</code> | notes | build-time variable, not read at run time |

## env-xws-x-websearch-attestor-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-xws-x-websearch-attestor-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:8</code> | <code>X_WEBSEARCH_ATTESTOR_KEY_FILE: attestor signing key file (ATTESTOR_KEY_FILE)</code> |

## env-xws-x-websearch-receiver-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-xws-x-websearch-receiver-key-file</code> | <code>env_var</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/keys.rs:10</code> | <code>X_WEBSEARCH_RECEIVER_KEY_FILE: payment receiver key file (RECEIVER_KEY_FILE)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-xws-x-websearch-receiver-key-file</code> | notes | spec/layerx-merge/spec.kvx task 5.3.2 do_2 holds the receiver key as a secp256k1 EVM key so the receiver address is the 0x101A draw sender |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## env-xws-x-websearch-submitter-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-xws-x-websearch-submitter-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:9</code> | <code>X_WEBSEARCH_SUBMITTER_KEY_FILE: EVM submitter key file for fulfil transactions (SUBMITTER_KEY_FILE)</code> |

## err-xws-answer-too-large

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-answer-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:152</code> | <code>ApiError::TooLarge.code() = "answer_too_large"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-answer-too-large</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141) |

## err-xws-attestation-not-found

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-attestation-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1154</code> | <code>HTTP 404 {"error":"attestation_not_found"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-attestation-not-found</code> | notes | emitted at services/interop/crates/x-websearch/src/attest.rs:1154 (404), services/interop/crates/x-websearch/src/kernel.rs:1247 (404) |

## err-xws-bad-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-bad-signature</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:655</code> | <code>Discard::BadSignature.code() = "bad_signature"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-bad-signature</code> | notes | wire code returned by Discard::code (services/interop/crates/x-websearch/src/attest.rs:650) |

## err-xws-body-not-accepted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-body-not-accepted</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:818</code> | <code>HTTP 400 {"error":"body_not_accepted"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-body-not-accepted</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:818 (400) |

## err-xws-body-too-large

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-body-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:66</code> | <code>FetchError::BodyTooLarge.code() = "body_too_large"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-body-too-large</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-connect-failed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-connect-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:57</code> | <code>FetchError::Connect.code() = "connect_failed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-connect-failed</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-connect-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-connect-timeout</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:58</code> | <code>FetchError::ConnectTimeout.code() = "connect_timeout"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-connect-timeout</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-content-not-found

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-content-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:207</code> | <code>HTTP 404 {"error":"content_not_found"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-content-not-found</code> | notes | emitted at services/interop/crates/x-websearch/src/content.rs:207 (404) |

## err-xws-content-store-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-content-store-error</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:208</code> | <code>HTTP 500 {"error":"content_store_error"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-content-store-error</code> | notes | emitted at services/interop/crates/x-websearch/src/content.rs:208 (500), services/interop/crates/x-websearch/src/fetch.rs:945 (500), services/interop/crates/x-websearch/src/search.rs:311 (500) |

## err-xws-content-too-large

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-content-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:67</code> | <code>FetchError::ContentTooLarge.code() = "content_too_large"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-content-too-large</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-content-too-long

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-content-too-long</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:89</code> | <code>CanonicalError::TooLong.code() = "content_too_long"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-content-too-long</code> | notes | wire code returned by CanonicalError::code (services/interop/crates/x-websearch/src/canonical.rs:84) |

## err-xws-delivery-failed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-delivery-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1444</code> | <code>HTTP 500/502 {"error":"delivery_failed"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-delivery-failed</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1444 (500), services/interop/crates/x-websearch/src/payment.rs:1446 (502) |

## err-xws-delivery-result-invalid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-delivery-result-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1452</code> | <code>HTTP 500 {"error":"delivery_result_invalid"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-delivery-result-invalid</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1452 (500) |

## err-xws-different-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-different-digest</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:654</code> | <code>Discard::DifferentDigest.code() = "different_digest"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-different-digest</code> | notes | wire code returned by Discard::code (services/interop/crates/x-websearch/src/attest.rs:650) |

## err-xws-duplicate-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-duplicate-header</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:799</code> | <code>HTTP 400 {"error":"duplicate_header"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-duplicate-header</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:799 (400) |

## err-xws-duplicate-query

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-duplicate-query</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:279</code> | <code>HTTP 400 {"error":"duplicate_query"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-duplicate-query</code> | notes | emitted at services/interop/crates/x-websearch/src/search.rs:279 (400) |

## err-xws-duplicate-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-duplicate-url</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:242</code> | <code>HTTP 400 {"error":"duplicate_url"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-duplicate-url</code> | notes | emitted at services/interop/crates/x-websearch/src/content.rs:242 (400), services/interop/crates/x-websearch/src/fetch.rs:937 (400) |

## err-xws-empty-query

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-empty-query</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:46</code> | <code>SearchError::EmptyQuery.code() = "empty_query"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-empty-query</code> | notes | wire code returned by SearchError::code (services/interop/crates/x-websearch/src/search.rs:44) |

## err-xws-envelope-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-envelope-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:144</code> | <code>ApiError::Envelope.code() = "envelope_refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-envelope-refused</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141) |

## err-xws-fetch-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-fetch-timeout</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:59</code> | <code>FetchError::Timeout.code() = "fetch_timeout"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-fetch-timeout</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-forbidden-destination

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-forbidden-destination</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:54</code> | <code>FetchError::ForbiddenDestination.code() = "forbidden_destination"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-forbidden-destination</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-headers-too-large

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-headers-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:613</code> | <code>HTTP 431 {"error":"headers_too_large"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-headers-too-large</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:613 (431), services/interop/crates/x-websearch/src/server.rs:802 (431) |

## err-xws-index-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-index-error</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:35</code> | <code>IndexError::Tantivy.code() = "index_error"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-index-error</code> | notes | wire code returned by IndexError::code (services/interop/crates/x-websearch/src/index.rs:32) |

## err-xws-index-io-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-index-io-error</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:34</code> | <code>IndexError::Io.code() = "index_io_error"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-index-io-error</code> | notes | wire code returned by IndexError::code (services/interop/crates/x-websearch/src/index.rs:32) |

## err-xws-index-writer-poisoned

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-index-writer-poisoned</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:38</code> | <code>IndexError::WriterPoisoned.code() = "index_writer_poisoned"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-index-writer-poisoned</code> | notes | wire code returned by IndexError::code (services/interop/crates/x-websearch/src/index.rs:32) |

## err-xws-internal-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-internal-error</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:349</code> | <code>HTTP 500 {"error":"internal_error"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-internal-error</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:349 (500), services/interop/crates/x-websearch/src/server.rs:360 (500), services/interop/crates/x-websearch/src/server.rs:370 (500), services/interop/crates/x-websearch/src/server.rs:375 (500), services/interop/crates/x-websearch/src/server.rs:386 (500) |

## err-xws-invalid-api-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-invalid-api-payload</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:143</code> | <code>ApiError::Payload.code() = "invalid_api_payload"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-invalid-api-payload</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141) |

## err-xws-invalid-credential

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-invalid-credential</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:145</code> | <code>ApiError::Credential.code() = "invalid_credential"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-invalid-credential</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141) |

## err-xws-invalid-encoding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-invalid-encoding</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:87</code> | <code>CanonicalError::InvalidEncoding.code() = "invalid_encoding"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-invalid-encoding</code> | notes | wire code returned by CanonicalError::code (services/interop/crates/x-websearch/src/canonical.rs:84) |

## err-xws-invalid-fetch-limits

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-invalid-fetch-limits</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:50</code> | <code>FetchError::InvalidLimits.code() = "invalid_fetch_limits"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-invalid-fetch-limits</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-invalid-index-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-invalid-index-url</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:36</code> | <code>IndexError::InvalidUrl.code() = "invalid_index_url"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-invalid-index-url</code> | notes | wire code returned by IndexError::code (services/interop/crates/x-websearch/src/index.rs:32) |

## err-xws-invalid-media-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-invalid-media-type</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:88</code> | <code>CanonicalError::InvalidMediaType.code() = "invalid_media_type"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-invalid-media-type</code> | notes | wire code returned by CanonicalError::code (services/interop/crates/x-websearch/src/canonical.rs:84) |

## err-xws-invalid-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-invalid-response</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:911</code> | <code>HTTP 500 {"error":"invalid_response"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-invalid-response</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:911 (500); 42 reference lines in 3 files; callers lists the first reference in each file |

## err-xws-invalid-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-invalid-url</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:51</code> | <code>FetchError::InvalidUrl.code() = "invalid_url"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-invalid-url</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48); HTTP status 400 at services/interop/crates/x-websearch/src/content.rs:263 (400), services/interop/crates/x-websearch/src/content.rs:266 (400) |

## err-xws-malformed-content

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-content</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:90</code> | <code>CanonicalError::Malformed.code() = "malformed_content"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-content</code> | notes | wire code returned by CanonicalError::code (services/interop/crates/x-websearch/src/canonical.rs:84) |

## err-xws-malformed-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-digest</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:198</code> | <code>HTTP 400 {"error":"malformed_digest"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-digest</code> | notes | emitted at services/interop/crates/x-websearch/src/content.rs:198 (400), services/interop/crates/x-websearch/src/payment.rs:1804 (400), services/interop/crates/x-websearch/src/server.rs:720 (400) |

## err-xws-malformed-payer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-payer</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1736</code> | <code>HTTP 400 {"error":"malformed_payer"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-payer</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1736 (400) |

## err-xws-malformed-program-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-program-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:763</code> | <code>HTTP 400 {"error":"malformed_program_request"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-program-request</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:763 (400) |

## err-xws-malformed-query

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-query</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:243</code> | <code>HTTP 400 {"error":"malformed_query"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-query</code> | notes | emitted at services/interop/crates/x-websearch/src/content.rs:243 (400), services/interop/crates/x-websearch/src/content.rs:247 (400), services/interop/crates/x-websearch/src/content.rs:254 (400), services/interop/crates/x-websearch/src/fetch.rs:938 (400), services/interop/crates/x-websearch/src/payment.rs:1814 (400), services/interop/crates/x-websearch/src/search.rs:251 (400), services/interop/crates/x-websearch/src/search.rs:259 (400), services/interop/crates/x-websearch/src/search.rs:272 (400) |

## err-xws-malformed-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-record</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:652</code> | <code>Discard::Malformed.code() = "malformed_record"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-record</code> | notes | wire code returned by Discard::code (services/interop/crates/x-websearch/src/attest.rs:650) |

## err-xws-malformed-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:614</code> | <code>HTTP 400 {"error":"malformed_request"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-request</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:614 (400), services/interop/crates/x-websearch/src/server.rs:767 (400) |

## err-xws-malformed-request-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-request-id</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:751</code> | <code>HTTP 400 {"error":"malformed_request_id"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-request-id</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:751 (400) |

## err-xws-malformed-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-malformed-response</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:62</code> | <code>FetchError::MalformedResponse.code() = "malformed_response"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-malformed-response</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-method-not-allowed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-method-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:607</code> | <code>HTTP 405 {"error":"method_not_allowed"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-method-not-allowed</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:607 (405) |

## err-xws-missing-host

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-missing-host</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1264</code> | <code>HTTP 400 {"error":"missing_host"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-missing-host</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1264 (400), services/interop/crates/x-websearch/src/server.rs:813 (400) |

## err-xws-missing-media-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-missing-media-type</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/extract.rs:185</code> | <code>ExtractError::MissingMediaType.code() = "missing_media_type"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-missing-media-type</code> | notes | wire code returned by ExtractError::code (services/interop/crates/x-websearch/src/extract.rs:183) |

## err-xws-missing-query

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-missing-query</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1816</code> | <code>HTTP 400 {"error":"missing_query"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-missing-query</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1816 (400), services/interop/crates/x-websearch/src/search.rs:267 (400), services/interop/crates/x-websearch/src/search.rs:284 (400) |

## err-xws-missing-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-missing-url</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:241</code> | <code>HTTP 400 {"error":"missing_url"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-missing-url</code> | notes | emitted at services/interop/crates/x-websearch/src/content.rs:241 (400), services/interop/crates/x-websearch/src/fetch.rs:936 (400) |

## err-xws-no-envelope-for-attestor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-no-envelope-for-attestor</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:146</code> | <code>ApiError::NoEnvelope.code() = "no_envelope_for_attestor"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-no-envelope-for-attestor</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141) |

## err-xws-not-found

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:346</code> | <code>HTTP 404 {"error":"not_found"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-not-found</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:346 (404), services/interop/crates/x-websearch/src/server.rs:357 (404), services/interop/crates/x-websearch/src/server.rs:715 (404) |

## err-xws-payer-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-payer-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1740</code> | <code>HTTP 400 {"error":"payer_mismatch"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-payer-mismatch</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1740 (400) |

## err-xws-payment-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-payment-pending</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1361</code> | <code>HTTP 503 {"error":"payment_pending"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-payment-pending</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1361 (503) |

## err-xws-payment-store-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-payment-store-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1377</code> | <code>HTTP 503 {"error":"payment_store_unavailable"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-payment-store-unavailable</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1377 (503), services/interop/crates/x-websearch/src/payment.rs:1382 (503) |

## err-xws-payment-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-payment-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1297</code> | <code>HTTP 503 {"error":"payment_unavailable"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-payment-unavailable</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1297 (503), services/interop/crates/x-websearch/src/payment.rs:1331 (503) |

## err-xws-persistence-failed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-persistence-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:657</code> | <code>Discard::Persistence.code() = "persistence_failed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-persistence-failed</code> | notes | wire code returned by Discard::code (services/interop/crates/x-websearch/src/attest.rs:650) |

## err-xws-pointer-unresolved

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-pointer-unresolved</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:150</code> | <code>ApiError::Pointer.code() = "pointer_unresolved"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-pointer-unresolved</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141) |

## err-xws-query-too-long

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-query-too-long</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:47</code> | <code>SearchError::QueryTooLong.code() = "query_too_long"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-query-too-long</code> | notes | wire code returned by SearchError::code (services/interop/crates/x-websearch/src/search.rs:44) |

## err-xws-receipt-consumed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-receipt-consumed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1408</code> | <code>HTTP 409 {"error":"receipt_consumed"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-receipt-consumed</code> | notes | emitted at services/interop/crates/x-websearch/src/payment.rs:1408 (409) |

## err-xws-request-line-too-long

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-request-line-too-long</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:612</code> | <code>HTTP 414 {"error":"request_line_too_long"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-request-line-too-long</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:612 (414) |

## err-xws-request-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-request-timeout</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:611</code> | <code>HTTP 408 {"error":"request_timeout"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-request-timeout</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:611 (408) |

## err-xws-resolve-failed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-resolve-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:53</code> | <code>FetchError::Resolve.code() = "resolve_failed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-resolve-failed</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-response-not-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-response-not-json</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:149</code> | <code>ApiError::NotJson.code() = "response_not_json"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-response-not-json</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141) |

## err-xws-robots-disallowed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-robots-disallowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:55</code> | <code>FetchError::RobotsDisallowed.code() = "robots_disallowed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-robots-disallowed</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-robots-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-robots-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:56</code> | <code>FetchError::RobotsUnavailable.code() = "robots_unavailable"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-robots-unavailable</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-route-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-route-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:383</code> | <code>HTTP 503 {"error":"route_unavailable"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-route-unavailable</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:383 (503) |

## err-xws-server-busy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-server-busy</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:556</code> | <code>HTTP 503 {"error":"server_busy"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-server-busy</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:556 (503) |

## err-xws-service-not-ready

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-service-not-ready</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:343</code> | <code>HTTP 503 {"error":"service_not_ready"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-service-not-ready</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:343 (503), services/interop/crates/x-websearch/src/server.rs:354 (503), services/interop/crates/x-websearch/src/server.rs:367 (503), services/interop/crates/x-websearch/src/server.rs:380 (503) |

## err-xws-title-too-long

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-title-too-long</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:37</code> | <code>IndexError::TitleTooLong.code() = "title_too_long"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-title-too-long</code> | notes | wire code returned by IndexError::code (services/interop/crates/x-websearch/src/index.rs:32) |

## err-xws-tls-failed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-tls-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:60</code> | <code>FetchError::Tls.code() = "tls_failed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-tls-failed</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-too-many-redirects

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-too-many-redirects</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:65</code> | <code>FetchError::TooManyRedirects.code() = "too_many_redirects"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-too-many-redirects</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-too-many-terms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-too-many-terms</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:48</code> | <code>SearchError::TooManyTerms.code() = "too_many_terms"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-too-many-terms</code> | notes | wire code returned by SearchError::code (services/interop/crates/x-websearch/src/search.rs:44) |

## err-xws-transport-failed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-transport-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:61</code> | <code>FetchError::Transport.code() = "transport_failed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-transport-failed</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-unexpected-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-unexpected-bytes</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:597</code> | <code>HTTP 400 {"error":"unexpected_bytes"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-unexpected-bytes</code> | notes | emitted at services/interop/crates/x-websearch/src/server.rs:597 (400) |

## err-xws-unknown-query-parameter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-unknown-query-parameter</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:253</code> | <code>HTTP 400 {"error":"unknown_query_parameter"}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-unknown-query-parameter</code> | notes | emitted at services/interop/crates/x-websearch/src/content.rs:253 (400), services/interop/crates/x-websearch/src/search.rs:276 (400) |

## err-xws-unknown-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-unknown-signer</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:656</code> | <code>Discard::UnknownSigner.code() = "unknown_signer"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-unknown-signer</code> | notes | wire code returned by Discard::code (services/interop/crates/x-websearch/src/attest.rs:650) |

## err-xws-unsupported-charset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-unsupported-charset</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:86</code> | <code>CanonicalError::UnsupportedCharset.code() = "unsupported_charset"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-unsupported-charset</code> | notes | wire code returned by CanonicalError::code (services/interop/crates/x-websearch/src/canonical.rs:84) |

## err-xws-unsupported-encoding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-unsupported-encoding</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:63</code> | <code>FetchError::UnsupportedEncoding.code() = "unsupported_encoding"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-unsupported-encoding</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-unsupported-media-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-unsupported-media-type</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:151</code> | <code>ApiError::MediaType.code() = "unsupported_media_type"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-unsupported-media-type</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141); also ExtractError::UnsupportedMediaType at services/interop/crates/x-websearch/src/extract.rs:186 |

## err-xws-unsupported-scheme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-unsupported-scheme</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:52</code> | <code>FetchError::UnsupportedScheme.code() = "unsupported_scheme"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-unsupported-scheme</code> | notes | wire code returned by FetchError::code (services/interop/crates/x-websearch/src/fetch.rs:48) |

## err-xws-upstream-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-upstream-status</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:148</code> | <code>ApiError::Status.code() = "upstream_status"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-upstream-status</code> | notes | wire code returned by ApiError::code (services/interop/crates/x-websearch/src/api.rs:141); also FetchError::Status at services/interop/crates/x-websearch/src/fetch.rs:64 |

## err-xws-wrong-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-xws-wrong-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:653</code> | <code>Discard::WrongRequest.code() = "wrong_request"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-xws-wrong-request</code> | notes | wire code returned by Discard::code (services/interop/crates/x-websearch/src/attest.rs:650) |

## fmt-xws-api-envelope-info

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-api-envelope-info</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:80</code> | <code>HKDF info "PAXEERX_WEB_API_ENVELOPE_V1" for the api request envelope (AES-GCM under ECDH secp256k1); payload: version u8=1 &#124; method u8 (1 GET, 2 POST) &#124; level u8 (0 majority, 1 single) &#124; attestor address 20 &#124; ...</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-api-envelope-info</code> | notes | mirrors the xweb module api codec byte for byte (services/interop/crates/x-websearch/src/api.rs:22-79), pinned by its api vectors |

## fmt-xws-asset-symbol-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-asset-symbol-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:96</code> | <code>AssetSymbol::code() -&gt; "SID" &#124; "PAX" &#124; "USDC" &#124; "USDL"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-asset-symbol-labels</code> | notes | stable label vocabulary of AssetSymbol |

## fmt-xws-attestation-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-attestation-domain</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:21</code> | <code>attestation digest prefix "PAXEERX_WEB_V1" over the answered request</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-attestation-domain</code> | notes | signed by each attestor at the majority or single level; verified by the xweb precompile path |

## fmt-xws-content-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-content-domain</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:5</code> | <code>canonical content = "PAXEERX_WEB_CONTENT_V1" &#124; kind u8 &#124; payload len u32 &#124; payload &#124; media type len u32 &#124; media type &#124; text len u64 &#124; text</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-content-domain</code> | notes | digest addressed by GET /content/&lt;digest&gt; (services/interop/crates/x-websearch/src/canonical.rs:283-290) |

## fmt-xws-delivery-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-delivery-record</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:779</code> | <code>DeliveryRecord { version: u8, principal: String, request_digest: String, resource: String, receipt_digest: String, state: DeliveryState, response?: Option&lt;DeliveryResponse&gt;, failures?: u32, retry_after_ms?: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-delivery-record</code> | notes | x-websearch wire/journal JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize, deny_unknown_fields |

## fmt-xws-delivery-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-delivery-response</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:682</code> | <code>DeliveryResponse { status: u16, content_type: String, headers: Vec&lt;(String, String)&gt;, body: Vec&lt;u8&gt;, body_digest: String, canonical_digest?: Option&lt;String&gt;, canonical_content?: Option&lt;Vec&lt;u8&gt;&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-delivery-response</code> | notes | x-websearch wire/journal JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize, deny_unknown_fields |

## fmt-xws-delivery-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-delivery-state</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:673</code> | <code>DeliveryState = pending &#124; computing &#124; failed &#124; ready</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-delivery-state</code> | notes | x-websearch wire/journal JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize, rename_all snake_case |

## fmt-xws-dependency-readiness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-dependency-readiness</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/main.rs:34</code> | <code>DependencyReadiness { dependency: &amp;'static str, state: DependencyState, critical: bool, last_success_unix_ms?: Option&lt;u64&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-dependency-readiness</code> | notes | x-websearch wire/journal JSON via serde Clone, Serialize |

## fmt-xws-dependency-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-dependency-state</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/main.rs:26</code> | <code>DependencyState = starting &#124; ready &#124; unavailable &#124; stale</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-dependency-state</code> | notes | x-websearch wire/journal JSON via serde Clone, Copy, Eq, PartialEq, Serialize, rename_all snake_case |

## fmt-xws-http-reason-phrases

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-http-reason-phrases</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:850</code> | <code>fn reason(status: u16) -&gt; &amp;'static str: OK &#124; Forbidden &#124; Conflict</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-http-reason-phrases</code> | notes | HTTP/1.1 status line reason phrases written by the hand-rolled HTTP server |

## fmt-xws-journal-state-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-journal-state-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:412</code> | <code>JournalState::code() -&gt; "signed" &#124; "fulfilled" &#124; "already_fulfilled" &#124; "refunded" &#124; "reorged"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-journal-state-labels</code> | notes | stable label vocabulary of JournalState |

## fmt-xws-payment-domains

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-payment-domains</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:75</code> | <code>payment digests: purpose = sha256("LayerX/x-websearch/v1/purpose\0" &#124; receiver public key), principal = sha256("LayerX/x-websearch/v1/principal\0" &#124; DID), trace = sha256("LayerX/x-websearch/v1/trace\0" &#124; header)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-payment-domains</code> | notes | stale task 5.4.4 drops DID account names from the web path, so the principal digest over a DID changes |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## fmt-xws-payment-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-payment-record</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:645</code> | <code>PaymentRecord { principal: String, request_digest: String, resource: String, offer: Value, payer?: Option&lt;String&gt;, receive_key?: Option&lt;String&gt;, activity?: Option&lt;String&gt;, activity_id?: Option&lt;String&gt;, attempted: bool, receipt?: Option&lt;String&gt;, receipt_digest?: Option&lt;String&gt;, released: bool }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-payment-record</code> | notes | x-websearch wire/journal JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize, deny_unknown_fields |

## fmt-xws-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-pending</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:313</code> | <code>Pending { request: WebRequest, attempts: u32, refused?: Option&lt;String&gt;, source?: Option&lt;SourceIdentity&gt;, stage?: WorkStage, next_retry_block?: u64, last_error?: Option&lt;String&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-pending</code> | notes | x-websearch wire/journal JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize |

## fmt-xws-progress

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-progress</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:354</code> | <code>Progress { version: u32, next_block?: Option&lt;u64&gt;, chain_id?: Option&lt;u64&gt;, entries: Vec&lt;Pending&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-progress</code> | notes | x-websearch wire/journal JSON via serde Serialize, Deserialize, deny_unknown_fields |

## fmt-xws-relay-journal-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-relay-journal-version</code> | <code>file_format</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:90</code> | <code>kernel relay journal version "PAXEERX_KERNEL_RELAY_V1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-relay-journal-version</code> | notes | stale task 5.5.6 retires the kernel relay journal |

## fmt-xws-request-topic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-request-topic</code> | <code>file_format</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:43</code> | <code>kernel request topic "PAXEERX_WEB_REQUEST_V1"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-request-topic</code> | notes | stale task 5.5.6 retires the kernel relay; origin-2 requests are watched from EVM logs (5.4.2) |

## fmt-xws-role-readiness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-role-readiness</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/main.rs:42</code> | <code>RoleReadiness { role: &amp;'static str, state: DependencyState, dependencies: Vec&lt;DependencyReadiness&gt;, freshness_budget_ms: u64, first_use_deadline_unix_ms: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-role-readiness</code> | notes | x-websearch wire/journal JSON via serde Clone, Serialize |

## fmt-xws-search-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-search-result</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:88</code> | <code>SearchResult { url: String, title: String, snippet: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-search-result</code> | notes | x-websearch wire/journal JSON via serde Clone, Debug, Eq, PartialEq, Serialize |

## fmt-xws-source-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-source-identity</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:331</code> | <code>SourceIdentity { chain_id: u64, block_hash: [u8; 32], transaction_hash: [u8; 32], log_index: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-source-identity</code> | notes | x-websearch wire/journal JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize, deny_unknown_fields |

## fmt-xws-stage-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-stage-labels</code> | <code>file_format</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1433</code> | <code>Stage::code() -&gt; "awaiting_quorum" &#124; "submitting" &#124; "unknown" &#124; "completed" &#124; "refused" &#124; "rejected"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-stage-labels</code> | notes | stable label vocabulary of Stage; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs |

## fmt-xws-web-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-web-request</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:187</code> | <code>WebRequest { request_id: u64, requester: [u8; 20], kind: u8, payload: Vec&lt;u8&gt;, callback_gas: u64, paid: [u8; 32], timeout_height: u64, block_number: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-web-request</code> | notes | x-websearch wire/journal JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize |

## fmt-xws-work-stage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-xws-work-stage</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:340</code> | <code>WorkStage = queued &#124; attesting &#124; attested &#124; quorum &#124; signed &#124; retry &#124; refused &#124; reorged</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-xws-work-stage</code> | notes | x-websearch wire/journal JSON via serde Clone, Copy, Debug, Default, Eq, PartialEq, Serialize, Deserialize, rename_all snake_case |

## http-xws-attestations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-attestations</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:211</code> | <code>GET /attestations/&lt;decimal request id&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-attestations</code> | notes | x-websearch sidecar; GET only (services/interop/crates/x-websearch/src/server.rs:598-607, other methods 405 method_not_allowed with Allow: GET); errors {"error":code}; signature exchange between attestors; handler set_attestations (services/interop/crates/x-websearch/src/attest.rs:1286); 400 malformed_request_id for a non-canonical id, 404 not_found without a handler |

## http-xws-content

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-content</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/server.rs:714</code> | <code>GET /content/&lt;64 lowercase hex digest&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-content</code> | notes | x-websearch sidecar; GET only (services/interop/crates/x-websearch/src/server.rs:598-607, other methods 405 method_not_allowed with Allow: GET); errors {"error":code}; handler set at services/interop/crates/x-websearch/src/content.rs:233; 400 malformed_digest for a non-hex or 0x-prefixed digest; stale task 5.0.8 do_5 exempts Route::Content from the serving-eligibility gate, but at HEAD dispatch (services/interop/crates/x-websearch/src/server.rs:377-379) still answers 503 service_not_ready before the handler |

Contract context <code>contract_context.11.x-websearch-server</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-fetch</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/server.rs:712</code> | <code>GET /fetch?url=&lt;absolute url&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-fetch</code> | notes | x-websearch sidecar; GET only (services/interop/crates/x-websearch/src/server.rs:598-607, other methods 405 method_not_allowed with Allow: GET); errors {"error":code}; handler set at services/interop/crates/x-websearch/src/content.rs:228; paid route like /search; stale task 5.3.2 and 5.4.4 change payment settlement and payer naming |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## http-xws-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:709</code> | <code>GET /health &#124; GET /xweb/health</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-health</code> | notes | x-websearch sidecar; GET only (services/interop/crates/x-websearch/src/server.rs:598-607, other methods 405 method_not_allowed with Allow: GET); errors {"error":code}; answers the set_health handler (services/interop/crates/x-websearch/src/main.rs:524) or {"status":"ok"}; never payment-gated |

## http-xws-program-attestations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-program-attestations</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/server.rs:216</code> | <code>GET /program-attestations/&lt;64 lowercase hex program id&gt;/&lt;decimal request id&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-program-attestations</code> | notes | x-websearch sidecar; GET only (services/interop/crates/x-websearch/src/server.rs:598-607, other methods 405 method_not_allowed with Allow: GET); errors {"error":code}; handler set_program_attestations is registered only from services/interop/crates/x-websearch/src/kernel.rs:1372, which is retired; stale task 5.2.3 moves the program signature exchange onto the origin-2 EVM path; 400 malformed_program_request |

Contract context <code>contract_context.11.x-websearch-server</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:710</code> | <code>GET /readyz</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-readyz</code> | notes | x-websearch sidecar; GET only (services/interop/crates/x-websearch/src/server.rs:598-607, other methods 405 method_not_allowed with Allow: GET); errors {"error":code}; answers the set_readiness handler (services/interop/crates/x-websearch/src/main.rs:496) or 503 service_not_ready (services/interop/crates/x-websearch/src/server.rs:365-371) |

## http-xws-search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-search</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/server.rs:711</code> | <code>GET /search?q=&lt;query&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-search</code> | notes | x-websearch sidecar; GET only (services/interop/crates/x-websearch/src/server.rs:598-607, other methods 405 method_not_allowed with Allow: GET); errors {"error":code}; handler set at services/interop/crates/x-websearch/src/search.rs:341; 503 service_not_ready until serving-eligible, 503 route_unavailable without a handler; paid route: 402 with PAYMENT-REQUIRED, settled from PAYMENT-SIGNATURE, answered with PAYMENT-RESPONSE (services/interop/crates/x-websearch/src/payment.rs:60-62); stale task 5.3.2 replaces the kernel settlement with the ledger draw and payExact and receipt verification by ReceiptKeys; stale task 5.4.4 drops the LAYERX-PAYER-DID account name |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## rs-xws-decode-program-requested

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-decode-program-requested</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/watch.rs</code> (planned) | <code>program request decoder beside decode_requested</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-decode-program-requested</code> | notes | stale task 5.4.2 do_1: origin-2 program requests watched from EVM logs of the xweb precompile |

Contract context <code>contract_context.11.x-websearch-program-requests</code>:

| Field | Frozen rule |
|---|---|
| interface | x-websearch watches xweb program requests on 0x1019 and answers them with fulfilProgram. |
| storage | Program exchange records under the data directory. |
| errors | Program refusals recorded per request. |
| events | XWebRequested and XWebProgramRequested of 0x1019. |
| gas | fulfilProgram transactions paid by the submitter account. |
| activation | When program requests are configured. |
| invariants | One fulfilment per request id. |
| pause | none |

## rs-xws-draw-calldata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-draw-calldata</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/payment.rs</code> (planned) | <code>pub fn draw_calldata(...) -&gt; Vec&lt;u8&gt;: ABI encoding of the ledger draw method</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-draw-calldata</code> | notes | stale task 5.3.2 do_1 replaces sign_draw and receive_authorization; the target precompile is 0x101A in 5.3.2 and 0x101B in 11.5.122 do_5 (unresolved) |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## rs-xws-fulfil-program-calldata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fulfil-program-calldata</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/submit.rs</code> (planned) | <code>pub fn fulfil_program_calldata(...) -&gt; Vec&lt;u8&gt; (beside fulfil_calldata)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fulfil-program-calldata</code> | notes | stale task 5.2.4 do_1: origin-2 program answers submitted as EVM transactions |

Contract context <code>contract_context.11.x-websearch-program-requests</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accept</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:16</code> | <code>pub const ACCEPT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accept</code> | notes | const x_websearch::fetch::ACCEPT; no reference outside the defining file found by name match |

## xws-accepted-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accepted-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/assets.rs:47</code> | <code>pub struct AcceptedAsset</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accepted-asset</code> | notes | struct x_websearch::assets::AcceptedAsset |

## xws-accepted-asset-id-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accepted-asset-id-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/assets.rs:56</code> | <code>pub fn id_hex(&amp;self) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accepted-asset-id-hex</code> | notes | method x_websearch::assets::AcceptedAsset::id_hex |

## xws-accepted-assets

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/assets.rs:63</code> | <code>pub struct AcceptedAssets</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accepted-assets</code> | notes | struct x_websearch::assets::AcceptedAssets |

## xws-accepted-assets-all

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-all</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/assets.rs:113</code> | <code>pub const fn all(&amp;self) -&gt; &amp;[AcceptedAsset; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-all</code> | notes | method x_websearch::assets::AcceptedAssets::all |

## xws-accepted-assets-by-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-by-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/assets.rs:127</code> | <code>pub fn by_id(&amp;self, asset_id: &amp;[u8; 32]) -&gt; Option&lt;&amp;AcceptedAsset&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-by-id</code> | notes | method x_websearch::assets::AcceptedAssets::by_id |

## xws-accepted-assets-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-get</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/assets.rs:118</code> | <code>pub fn get(&amp;self, symbol: AssetSymbol) -&gt; &amp;AcceptedAsset</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-get</code> | notes | method x_websearch::assets::AcceptedAssets::get; no reference outside the defining file found by name match |

## xws-accepted-assets-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/assets.rs:71</code> | <code>pub fn new(configured: &amp;[AssetConfig]) -&gt; Result&lt;Self, AssetRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accepted-assets-new</code> | notes | method x_websearch::assets::AcceptedAssets::new |

## xws-accepted-media-types

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-accepted-media-types</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/extract.rs:4</code> | <code>pub const ACCEPTED_MEDIA_TYPES: [&amp;str; 3]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-accepted-media-types</code> | notes | const x_websearch::extract::ACCEPTED_MEDIA_TYPES; no reference outside the defining file found by name match |

## xws-account-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-account-id</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:173</code> | <code>pub fn account_id(account: &amp;str) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-account-id</code> | notes | fn x_websearch::payment::account_id; spec/layerx-merge/spec.kvx task 11.5.122 do_4 retires kernel account naming (LAYERX-PAYER-DID, agent:{did}:main, :asset:{) for web payments; payer named by EVM address via 0x1004 |

## xws-activity-options

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-activity-options</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:666</code> | <code>pub struct ActivityOptions&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-activity-options</code> | notes | struct x_websearch::kernel::ActivityOptions; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-activity-validity-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-activity-validity-ms</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:73</code> | <code>pub const ACTIVITY_VALIDITY_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-activity-validity-ms</code> | notes | const x_websearch::kernel::ACTIVITY_VALIDITY_MS; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-address-of

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-address-of</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:159</code> | <code>pub fn address_of(key: &amp;VerifyingKey) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-address-of</code> | notes | fn x_websearch::attest::address_of |

## xws-answer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-answer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1898</code> | <code>pub fn answer(client: &amp;ApiClient, key: &amp;SigningKey, raw_payload: &amp;[u8], payload: &amp;ApiPayload) -&gt; Result&lt;ApiAnswer, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-answer</code> | notes | fn x_websearch::api::answer; 362 reference lines in 15 files; callers lists the first reference in each file |

## xws-answer-canonical-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-answer-canonical-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:266</code> | <code>pub fn answer_canonical_bytes(payload: &amp;[u8], media_type: &amp;str, answer: &amp;[u8]) -&gt; Result&lt;Vec&lt;u8&gt;, CanonicalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-answer-canonical-bytes</code> | notes | fn x_websearch::canonical::answer_canonical_bytes |

## xws-answer-media-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-answer-media-type</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:94</code> | <code>pub const ANSWER_MEDIA_TYPE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-answer-media-type</code> | notes | const x_websearch::api::ANSWER_MEDIA_TYPE |

## xws-answer-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-answer-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:386</code> | <code>pub fn record(&amp;self) -&gt; Value</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-answer-record</code> | notes | method x_websearch::attest::Answer::record; no reference outside the defining file found by name match |

## xws-answer-request-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-answer-request-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:380</code> | <code>pub const fn request_id(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-answer-request-id</code> | notes | method x_websearch::attest::Answer::request_id |

## xws-api-answer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-answer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1882</code> | <code>pub struct ApiAnswer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-answer</code> | notes | struct x_websearch::api::ApiAnswer; no reference outside the defining file found by name match |

## xws-api-canonical-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-canonical-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1501</code> | <code>pub fn api_canonical_bytes(payload: &amp;[u8], media_type: &amp;str, answer: &amp;[u8]) -&gt; Result&lt;Vec&lt;u8&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-canonical-bytes</code> | notes | fn x_websearch::api::api_canonical_bytes |

## xws-api-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1689</code> | <code>pub struct ApiClient</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-client</code> | notes | struct x_websearch::api::ApiClient |

## xws-api-client-body-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-client-body-limit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1710</code> | <code>pub fn body_limit(&amp;self) -&gt; usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-client-body-limit</code> | notes | method x_websearch::api::ApiClient::body_limit; no reference outside the defining file found by name match |

## xws-api-client-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-client-call</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1809</code> | <code>pub fn call(&amp;self, payload: &amp;ApiPayload, credential: Option&lt;&amp;Credential&gt;) -&gt; Result&lt;ApiResponse, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-client-call</code> | notes | method x_websearch::api::ApiClient::call; no reference outside the defining file found by name match |

## xws-api-client-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-client-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1699</code> | <code>pub fn new(fetcher: Arc&lt;Fetcher&gt;, roots: &amp;[native_tls::Certificate]) -&gt; Result&lt;Self, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-client-new</code> | notes | method x_websearch::api::ApiClient::new |

## xws-api-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:116</code> | <code>pub enum ApiError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-error</code> | notes | enum x_websearch::api::ApiError |

## xws-api-error-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-error-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:141</code> | <code>pub const fn code(&amp;self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-error-code</code> | notes | method x_websearch::api::ApiError::code; no reference outside the defining file found by name match |

## xws-api-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:178</code> | <code>pub struct ApiHeader</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-header</code> | notes | struct x_websearch::api::ApiHeader |

## xws-api-header-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-header-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:185</code> | <code>pub fn new(name: &amp;str, value: &amp;str) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-header-new</code> | notes | method x_websearch::api::ApiHeader::new |

## xws-api-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-payload</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:760</code> | <code>pub struct ApiPayload</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-payload</code> | notes | struct x_websearch::api::ApiPayload |

## xws-api-payload-attestation-level

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-attestation-level</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:816</code> | <code>pub const fn attestation_level(&amp;self) -&gt; Level</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-payload-attestation-level</code> | notes | method x_websearch::api::ApiPayload::attestation_level |

## xws-api-payload-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:869</code> | <code>pub fn decode(raw: &amp;[u8]) -&gt; Result&lt;Self, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-payload-decode</code> | notes | method x_websearch::api::ApiPayload::decode |

## xws-api-payload-encode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-encode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:836</code> | <code>pub fn encode(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-payload-encode</code> | notes | method x_websearch::api::ApiPayload::encode; no reference outside the defining file found by name match |

## xws-api-payload-envelope-for

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-envelope-for</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:826</code> | <code>pub fn envelope_for(&amp;self, attestor: &amp;[u8; 20]) -&gt; Option&lt;&amp;Envelope&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-payload-envelope-for</code> | notes | method x_websearch::api::ApiPayload::envelope_for; no reference outside the defining file found by name match |

## xws-api-payload-method-name

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-method-name</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:810</code> | <code>pub fn method_name(&amp;self) -&gt; Result&lt;&amp;'static str, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-payload-method-name</code> | notes | method x_websearch::api::ApiPayload::method_name; no reference outside the defining file found by name match |

## xws-api-payload-origin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-origin</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:802</code> | <code>pub fn origin(&amp;self) -&gt; Result&lt;String, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-payload-origin</code> | notes | method x_websearch::api::ApiPayload::origin |

## xws-api-payload-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-payload-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:793</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-payload-validate</code> | notes | method x_websearch::api::ApiPayload::validate; no reference outside the defining file found by name match |

## xws-api-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1514</code> | <code>pub struct ApiResponse</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-response</code> | notes | struct x_websearch::api::ApiResponse; no reference outside the defining file found by name match |

## xws-api-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-api-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:56</code> | <code>pub const API_VERSION: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-api-version</code> | notes | const x_websearch::api::API_VERSION; no reference outside the defining file found by name match |

## xws-asset-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-asset-account</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:155</code> | <code>pub fn asset_account(did: &amp;str, asset_id: &amp;[u8; 32]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-asset-account</code> | notes | fn x_websearch::payment::asset_account; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 11.5.122 do_4 retires kernel account naming (LAYERX-PAYER-DID, agent:{did}:main, :asset:{) for web payments; payer named by EVM address via 0x1004 |

## xws-asset-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-asset-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:107</code> | <code>pub struct AssetConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-asset-config</code> | notes | struct x_websearch::config::AssetConfig |

## xws-asset-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-asset-refusal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/assets.rs:9</code> | <code>pub enum AssetRefusal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-asset-refusal</code> | notes | enum x_websearch::assets::AssetRefusal |

## xws-asset-symbol

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-asset-symbol</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:85</code> | <code>pub enum AssetSymbol</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-asset-symbol</code> | notes | enum x_websearch::config::AssetSymbol |

## xws-asset-symbol-all

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-asset-symbol-all</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:93</code> | <code>pub const ALL: [Self; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-asset-symbol-all</code> | notes | associated const x_websearch::config::AssetSymbol::ALL |

## xws-asset-symbol-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-asset-symbol-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:96</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-asset-symbol-code</code> | notes | method x_websearch::config::AssetSymbol::code; no reference outside the defining file found by name match |

## xws-attest-answer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attest-answer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:367</code> | <code>pub struct Answer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attest-answer</code> | notes | struct x_websearch::attest::Answer |

## xws-attest-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attest-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:224</code> | <code>pub enum AttestError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attest-error</code> | notes | enum x_websearch::attest::AttestError |

## xws-attest-error-is-terminal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attest-error-is-terminal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:249</code> | <code>pub const fn is_terminal(&amp;self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attest-error-is-terminal</code> | notes | method x_websearch::attest::AttestError::is_terminal; no reference outside the defining file found by name match |

## xws-attestation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:53</code> | <code>pub struct Attestation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestation</code> | notes | struct x_websearch::attest::Attestation |

## xws-attestation-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestation-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:114</code> | <code>pub fn digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestation-digest</code> | notes | method x_websearch::attest::Attestation::digest; no reference outside the defining file found by name match |

## xws-attestation-evm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestation-evm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:68</code> | <code>pub fn evm(chain_id: u64, request: &amp;WebRequest, content_digest: [u8; 32], response: &amp;[u8], full_length: u32) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestation-evm</code> | notes | method x_websearch::attest::Attestation::evm |

## xws-attestation-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestation-path</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:211</code> | <code>pub const ATTESTATION_PATH: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestation-path</code> | notes | const x_websearch::server::ATTESTATION_PATH; re-exported at services/interop/crates/x-websearch/src/attest.rs:38 |

## xws-attestation-preimage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestation-preimage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:90</code> | <code>pub fn preimage(&amp;self) -&gt; [u8; PREIMAGE_LENGTH]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestation-preimage</code> | notes | method x_websearch::attest::Attestation::preimage |

## xws-attestor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:401</code> | <code>pub struct Attestor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor</code> | notes | struct x_websearch::attest::Attestor |

## xws-attestor-attest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor-attest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:524</code> | <code>pub fn attest(&amp;self, request: &amp;WebRequest) -&gt; Result&lt;Answer, AttestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor-attest</code> | notes | method x_websearch::attest::Attestor::attest |

## xws-attestor-binds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor-binds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:496</code> | <code>pub fn binds(&amp;self, request: &amp;WebRequest, answer: &amp;Answer) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor-binds</code> | notes | method x_websearch::attest::Attestor::binds |

## xws-attestor-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor-key-file</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:8</code> | <code>pub const ATTESTOR_KEY_FILE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor-key-file</code> | notes | const x_websearch::keys::ATTESTOR_KEY_FILE |

## xws-attestor-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:413</code> | <code>pub fn new(key: SigningKey, chain_id: u64, fetcher: Arc&lt;Fetcher&gt;, index: Arc&lt;WebIndex&gt;, store: Arc&lt;ContentStore&gt;) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor-new</code> | notes | method x_websearch::attest::Attestor::new |

## xws-attestor-set

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor-set</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:620</code> | <code>pub struct AttestorSet</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor-set</code> | notes | struct x_websearch::attest::AttestorSet |

## xws-attestor-set-contains

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor-set-contains</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:627</code> | <code>pub fn contains(&amp;self, signer: &amp;[u8; 20]) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor-set-contains</code> | notes | method x_websearch::attest::AttestorSet::contains; no reference outside the defining file found by name match |

## xws-attestor-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor-signer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:440</code> | <code>pub const fn signer(&amp;self) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor-signer</code> | notes | method x_websearch::attest::Attestor::signer |

## xws-attestor-with-api-roots

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-attestor-with-api-roots</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:433</code> | <code>pub fn with_api_roots(mut self, roots: Vec&lt;native_tls::Certificate&gt;) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-attestor-with-api-roots</code> | notes | method x_websearch::attest::Attestor::with_api_roots |

## xws-body-limit-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-body-limit-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:10</code> | <code>pub const BODY_LIMIT_BYTES: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-body-limit-bytes</code> | notes | const x_websearch::config::BODY_LIMIT_BYTES |

## xws-cache-lifetime

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-cache-lifetime</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:9</code> | <code>pub const CACHE_LIFETIME: Duration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-cache-lifetime</code> | notes | const x_websearch::robots::CACHE_LIFETIME; no reference outside the defining file found by name match |

## xws-canonical

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-canonical</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:176</code> | <code>pub enum Canonical</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-canonical</code> | notes | enum x_websearch::watch::Canonical |

## xws-canonical-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-canonical-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:250</code> | <code>pub fn canonical_bytes(kind: ContentKind, payload: &amp;[u8], media_type: &amp;str, text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, CanonicalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-canonical-bytes</code> | notes | fn x_websearch::canonical::canonical_bytes |

## xws-canonical-content

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-canonical-content</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:318</code> | <code>pub struct CanonicalContent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-canonical-content</code> | notes | struct x_websearch::canonical::CanonicalContent |

## xws-canonical-content-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-canonical-content-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:330</code> | <code>pub fn parse(bytes: &amp;[u8]) -&gt; Result&lt;Self, CanonicalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-canonical-content-parse</code> | notes | method x_websearch::canonical::CanonicalContent::parse |

## xws-canonical-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-canonical-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:74</code> | <code>pub enum CanonicalError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-canonical-error</code> | notes | enum x_websearch::canonical::CanonicalError |

## xws-canonical-error-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-canonical-error-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:84</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-canonical-error-code</code> | notes | method x_websearch::canonical::CanonicalError::code; no reference outside the defining file found by name match |

## xws-canonical-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-canonical-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1461</code> | <code>pub fn canonical_json(value: &amp;Json) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-canonical-json</code> | notes | fn x_websearch::api::canonical_json; no reference outside the defining file found by name match |

## xws-canonicalise

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-canonicalise</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:204</code> | <code>pub fn canonicalise(text: &amp;str) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-canonicalise</code> | notes | fn x_websearch::canonical::canonicalise |

## xws-check

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-check</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:351</code> | <code>pub fn check(bytes: &amp;[u8]) -&gt; Result&lt;ContentKind, CanonicalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-check</code> | notes | fn x_websearch::canonical::check |

## xws-clock

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-clock</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:89</code> | <code>pub type Clock = fn() -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-clock</code> | notes | type alias x_websearch::payment::Clock; no reference outside the defining file found by name match |

## xws-collapse-blank-lines

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-collapse-blank-lines</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:185</code> | <code>pub fn collapse_blank_lines(text: &amp;str) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-collapse-blank-lines</code> | notes | fn x_websearch::canonical::collapse_blank_lines |

## xws-collapse-spaces

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-collapse-spaces</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:157</code> | <code>pub fn collapse_spaces(text: &amp;str) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-collapse-spaces</code> | notes | fn x_websearch::canonical::collapse_spaces |

## xws-commitment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-commitment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:65</code> | <code>pub const COMMITMENT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-commitment</code> | notes | const x_websearch::payment::COMMITMENT |

## xws-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-config</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/config.rs:249</code> | <code>pub struct Config</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-config</code> | notes | struct x_websearch::config::Config; 41 reference lines in 7 files; callers lists the first reference in each file; re-exported at services/interop/crates/x-websearch/src/lib.rs:29; spec/layerx-merge/spec.kvx task 5.6.6 drops the kernel section and 11.5.121 drops kernel_network_id from the parsed config |

Contract context <code>contract_context.11.x-websearch-config</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-config-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:324</code> | <code>pub fn asset(&amp;self, symbol: AssetSymbol) -&gt; &amp;AssetConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-config-asset</code> | notes | method x_websearch::config::Config::asset |

## xws-config-crawl-interval

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-config-crawl-interval</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:319</code> | <code>pub const fn crawl_interval(&amp;self) -&gt; Duration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-config-crawl-interval</code> | notes | method x_websearch::config::Config::crawl_interval |

## xws-config-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-config-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:57</code> | <code>pub struct ConfigError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-config-error</code> | notes | struct x_websearch::config::ConfigError; re-exported at services/interop/crates/x-websearch/src/lib.rs:29 |

## xws-config-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-config-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:313</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-config-load</code> | notes | method x_websearch::config::Config::load |

## xws-config-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-config-parse</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/config.rs:272</code> | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-config-parse</code> | notes | method x_websearch::config::Config::parse; spec/layerx-merge/spec.kvx task 5.4.3/5.6.6: receipt_keys parsed, kernel key refused as unknown field |

Contract context <code>contract_context.11.x-websearch-config</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-conformance-suite</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/lib.rs:37</code> | <code>pub const CONFORMANCE_SUITE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-conformance-suite</code> | notes | const x_websearch::CONFORMANCE_SUITE |

## xws-conformance-suite-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-conformance-suite-digest</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/lib.rs:43</code> | <code>pub const CONFORMANCE_SUITE_DIGEST: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-conformance-suite-digest</code> | notes | const x_websearch::CONFORMANCE_SUITE_DIGEST; spec/layerx-merge/spec.kvx task 5.6.11 and 12.7.1 re-record the digest and count after the kernel recordings are removed; 13.6.5 do_1 says keep it, which conflicts |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-conformance-vector-count

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-conformance-vector-count</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/lib.rs:40</code> | <code>pub const CONFORMANCE_VECTOR_COUNT: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-conformance-vector-count</code> | notes | const x_websearch::CONFORMANCE_VECTOR_COUNT; spec/layerx-merge/spec.kvx task 5.6.11 and 12.7.1 re-record the digest and count after the kernel recordings are removed; 13.6.5 do_1 says keep it, which conflicts |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-connect-timeout-limit-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-connect-timeout-limit-ms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:8</code> | <code>pub const CONNECT_TIMEOUT_LIMIT_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-connect-timeout-limit-ms</code> | notes | const x_websearch::config::CONNECT_TIMEOUT_LIMIT_MS |

## xws-content-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:300</code> | <code>pub fn content_digest(canonical: &amp;[u8]) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-digest</code> | notes | fn x_websearch::canonical::content_digest; 99 reference lines in 16 files; callers lists the first reference in each file |

## xws-content-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:5</code> | <code>pub const CONTENT_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-domain</code> | notes | const x_websearch::canonical::CONTENT_DOMAIN |

## xws-content-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:46</code> | <code>pub enum ContentKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-kind</code> | notes | enum x_websearch::canonical::ContentKind; 42 reference lines in 11 files; callers lists the first reference in each file |

## xws-content-kind-byte

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-kind-byte</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:54</code> | <code>pub const fn byte(self) -&gt; u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-kind-byte</code> | notes | method x_websearch::canonical::ContentKind::byte |

## xws-content-kind-from-byte

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-kind-from-byte</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:63</code> | <code>pub const fn from_byte(byte: u8) -&gt; Option&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-kind-from-byte</code> | notes | method x_websearch::canonical::ContentKind::from_byte |

## xws-content-register

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-register</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:218</code> | <code>pub fn register(routes: &amp;mut RouteTable, gate: &amp;Arc&lt;PaymentGate&gt;, fetcher: &amp;Arc&lt;Fetcher&gt;, store: &amp;Arc&lt;ContentStore&gt;) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-register</code> | notes | fn x_websearch::content::register |

## xws-content-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-store</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:24</code> | <code>pub struct ContentStore</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-store</code> | notes | struct x_websearch::content::ContentStore; 42 reference lines in 12 files; callers lists the first reference in each file |

## xws-content-store-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-store-directory</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:65</code> | <code>pub fn directory(&amp;self) -&gt; &amp;Path</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-store-directory</code> | notes | method x_websearch::content::ContentStore::directory |

## xws-content-store-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-store-get</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:131</code> | <code>pub fn get(&amp;self, digest: &amp;[u8; 32]) -&gt; io::Result&lt;Option&lt;Vec&lt;u8&gt;&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-store-get</code> | notes | method x_websearch::content::ContentStore::get; no reference outside the defining file found by name match |

## xws-content-store-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-store-handle</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:196</code> | <code>pub fn handle(&amp;self, request: &amp;Request) -&gt; Response</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-store-handle</code> | notes | method x_websearch::content::ContentStore::handle; no reference outside the defining file found by name match |

## xws-content-store-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-store-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:41</code> | <code>pub fn open(data_dir: &amp;Path, peers: &amp;[String]) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-store-open</code> | notes | method x_websearch::content::ContentStore::open |

## xws-content-store-put

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-store-put</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:79</code> | <code>pub fn put(&amp;self, bytes: &amp;[u8]) -&gt; io::Result&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-store-put</code> | notes | method x_websearch::content::ContentStore::put |

## xws-content-store-retrieve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-content-store-retrieve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:152</code> | <code>pub fn retrieve(&amp;self, digest: &amp;[u8; 32]) -&gt; io::Result&lt;Option&lt;Vec&lt;u8&gt;&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-content-store-retrieve</code> | notes | method x_websearch::content::ContentStore::retrieve |

## xws-crawl-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-budget</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:20</code> | <code>pub struct CrawlBudget</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-budget</code> | notes | struct x_websearch::crawl::CrawlBudget |

## xws-crawl-budget-from-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-budget-from-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:29</code> | <code>pub const fn from_config(config: &amp;CrawlConfig) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-budget-from-config</code> | notes | method x_websearch::crawl::CrawlBudget::from_config |

## xws-crawl-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:114</code> | <code>pub struct CrawlConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-config</code> | notes | struct x_websearch::config::CrawlConfig |

## xws-crawl-config-politeness-delay

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-config-politeness-delay</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:123</code> | <code>pub const fn politeness_delay(&amp;self) -&gt; Duration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-config-politeness-delay</code> | notes | method x_websearch::config::CrawlConfig::politeness_delay |

## xws-crawl-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:54</code> | <code>pub enum CrawlError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-error</code> | notes | enum x_websearch::crawl::CrawlError |

## xws-crawl-report

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-report</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:136</code> | <code>pub struct CrawlReport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-report</code> | notes | struct x_websearch::crawl::CrawlReport |

## xws-crawl-report-indexed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-report-indexed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:149</code> | <code>pub fn indexed(&amp;self) -&gt; impl Iterator&lt;Item = &amp;CrawledPage&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-report-indexed</code> | notes | method x_websearch::crawl::CrawlReport::indexed |

## xws-crawl-report-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-report-refusal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:160</code> | <code>pub fn refusal(&amp;self, url: &amp;str) -&gt; Option&lt;FetchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-report-refusal</code> | notes | method x_websearch::crawl::CrawlReport::refusal; no reference outside the defining file found by name match |

## xws-crawl-report-visited

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawl-report-visited</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:144</code> | <code>pub fn visited(&amp;self) -&gt; Vec&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawl-report-visited</code> | notes | method x_websearch::crawl::CrawlReport::visited; no reference outside the defining file found by name match |

## xws-crawled-page

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawled-page</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:127</code> | <code>pub struct CrawledPage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawled-page</code> | notes | struct x_websearch::crawl::CrawledPage; no reference outside the defining file found by name match |

## xws-crawler

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawler</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:174</code> | <code>pub struct Crawler</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawler</code> | notes | struct x_websearch::crawl::Crawler |

## xws-crawler-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawler-budget</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:337</code> | <code>pub const fn budget(&amp;self) -&gt; &amp;CrawlBudget</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawler-budget</code> | notes | method x_websearch::crawl::Crawler::budget |

## xws-crawler-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawler-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:320</code> | <code>pub fn new(budget: CrawlBudget, fetcher: Arc&lt;Fetcher&gt;, index: Arc&lt;WebIndex&gt;) -&gt; Result&lt;Self, CrawlError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawler-new</code> | notes | method x_websearch::crawl::Crawler::new |

## xws-crawler-run-cycle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawler-run-cycle</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:350</code> | <code>pub fn run_cycle(&amp;mut self, seeds: &amp;[String]) -&gt; Result&lt;CrawlReport, CrawlError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawler-run-cycle</code> | notes | method x_websearch::crawl::Crawler::run_cycle |

## xws-crawler-run-every

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-crawler-run-every</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:360</code> | <code>pub fn run_every(&amp;mut self, seeds: &amp;[String], interval: Duration, stop: &amp;StopSignal, mut report: impl FnMut(Result&lt;CrawlReport, CrawlError&gt;)) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-crawler-run-every</code> | notes | method x_websearch::crawl::Crawler::run_every |

## xws-credential

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-credential</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:941</code> | <code>pub struct Credential</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-credential</code> | notes | struct x_websearch::api::Credential |

## xws-credential-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-credential-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:994</code> | <code>pub fn decode(plaintext: &amp;[u8], public: &amp;[ApiHeader]) -&gt; Result&lt;Self, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-credential-decode</code> | notes | method x_websearch::api::Credential::decode |

## xws-credential-encode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-credential-encode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:952</code> | <code>pub fn encode(headers: &amp;[ApiHeader]) -&gt; Result&lt;Zeroizing&lt;Vec&lt;u8&gt;&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-credential-encode</code> | notes | method x_websearch::api::Credential::encode |

## xws-credential-names

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-credential-names</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1037</code> | <code>pub fn names(&amp;self) -&gt; Vec&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-credential-names</code> | notes | method x_websearch::api::Credential::names |

## xws-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:109</code> | <code>pub fn decode(bytes: &amp;[u8], charset: Option&lt;&amp;str&gt;) -&gt; Result&lt;String, CanonicalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-decode</code> | notes | fn x_websearch::canonical::decode |

## xws-decode-grant

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-decode-grant</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:185</code> | <code>pub fn decode_grant(bytes: &amp;[u8], actor: &amp;str) -&gt; Option&lt;Grant&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-decode-grant</code> | notes | fn x_websearch::payment::decode_grant; spec/layerx-merge/spec.kvx task 5.3.2 do_10 reads the grant with eth_call getPayerGrant(grantId) on 0x101A; kernel grant ordinals deleted |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-decode-request-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-decode-request-record</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:198</code> | <code>pub fn decode_request_record(program_id: [u8; 32], sequence: u64, data: &amp;[u8]) -&gt; Result&lt;ProgramRequest, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-decode-request-record</code> | notes | fn x_websearch::kernel::decode_request_record; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-decode-request-view

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-decode-request-view</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:306</code> | <code>pub fn decode_request_view(answer: &amp;[u8], request_id: u64) -&gt; Option&lt;RequestView&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-decode-request-view</code> | notes | fn x_websearch::submit::decode_request_view; no reference outside the defining file found by name match |

## xws-decode-requested

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-decode-requested</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:219</code> | <code>pub fn decode_requested(log: &amp;Value) -&gt; Result&lt;WebRequest, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-decode-requested</code> | notes | fn x_websearch::watch::decode_requested |

## xws-default-crawl-interval-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-default-crawl-interval-seconds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:21</code> | <code>pub const DEFAULT_CRAWL_INTERVAL_SECONDS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-default-crawl-interval-seconds</code> | notes | const x_websearch::config::DEFAULT_CRAWL_INTERVAL_SECONDS |

## xws-default-draw-fee-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-default-draw-fee-limit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:174</code> | <code>pub const DEFAULT_DRAW_FEE_LIMIT: u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-default-draw-fee-limit</code> | notes | const x_websearch::config::DEFAULT_DRAW_FEE_LIMIT |

## xws-destination-permitted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-destination-permitted</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:317</code> | <code>pub fn destination_permitted(address: IpAddr, allow_loopback: bool) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-destination-permitted</code> | notes | fn x_websearch::fetch::destination_permitted |

## xws-digest-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-digest-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:306</code> | <code>pub fn digest_hex(digest: &amp;[u8; 32]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-digest-hex</code> | notes | fn x_websearch::canonical::digest_hex |

## xws-discard

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-discard</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:634</code> | <code>pub enum Discard</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-discard</code> | notes | enum x_websearch::attest::Discard |

## xws-discard-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-discard-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:650</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-discard-code</code> | notes | method x_websearch::attest::Discard::code; no reference outside the defining file found by name match |

## xws-discarded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-discarded</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:664</code> | <code>pub struct Discarded</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-discarded</code> | notes | struct x_websearch::attest::Discarded; no reference outside the defining file found by name match |

## xws-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:21</code> | <code>pub const DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-domain</code> | notes | const x_websearch::attest::DOMAIN |

## xws-draw-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-draw-error</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:217</code> | <code>pub enum DrawError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-draw-error</code> | notes | enum x_websearch::payment::DrawError; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.3.2 do_1 replaces sign_draw and receive_authorization with draw_calldata on the 0x101A draw method; 11.5.122 do_5 names 0x101B for the draw, which disagrees with 5.3.2 (0x101A) |

## xws-draw-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-draw-request</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:197</code> | <code>pub struct DrawRequest&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-draw-request</code> | notes | struct x_websearch::payment::DrawRequest; spec/layerx-merge/spec.kvx task 5.3.2 do_1 replaces sign_draw and receive_authorization with draw_calldata on the 0x101A draw method; 11.5.122 do_5 names 0x101B for the draw, which disagrees with 5.3.2 (0x101A) |

## xws-draw-validity-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-draw-validity-ms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:72</code> | <code>pub const DRAW_VALIDITY_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-draw-validity-ms</code> | notes | const x_websearch::payment::DRAW_VALIDITY_MS; no reference outside the defining file found by name match |

## xws-dynamic-fee-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-dynamic-fee-type</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:18</code> | <code>pub const DYNAMIC_FEE_TYPE: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-dynamic-fee-type</code> | notes | const x_websearch::submit::DYNAMIC_FEE_TYPE; no reference outside the defining file found by name match |

## xws-encode-activity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-encode-activity</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:727</code> | <code>pub fn encode_activity(observation: &amp;[u8], submitter: &amp;SubmitterKey, options: &amp;ActivityOptions&lt;'_&gt;) -&gt; Result&lt;Vec&lt;u8&gt;, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-encode-activity</code> | notes | fn x_websearch::kernel::encode_activity; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-envelope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:195</code> | <code>pub struct Envelope</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope</code> | notes | struct x_websearch::api::Envelope |

## xws-envelope-aad

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-aad</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:307</code> | <code>pub fn envelope_aad(attestor: &amp;[u8; 20], origin: &amp;str) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-aad</code> | notes | fn x_websearch::api::envelope_aad |

## xws-envelope-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:205</code> | <code>pub fn bytes(&amp;self) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-bytes</code> | notes | method x_websearch::api::Envelope::bytes; no reference outside the defining file found by name match |

## xws-envelope-info

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-info</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:80</code> | <code>pub const ENVELOPE_INFO: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-info</code> | notes | const x_websearch::api::ENVELOPE_INFO; no reference outside the defining file found by name match |

## xws-envelope-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:337</code> | <code>pub fn envelope_key(shared_x: &amp;[u8], ephemeral_key: &amp;[u8]) -&gt; Result&lt;Zeroizing&lt;[u8; 32]&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-key</code> | notes | fn x_websearch::api::envelope_key |

## xws-envelope-key-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-key-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:82</code> | <code>pub const ENVELOPE_KEY_LENGTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-key-length</code> | notes | const x_websearch::api::ENVELOPE_KEY_LENGTH; no reference outside the defining file found by name match |

## xws-envelope-nonce-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-nonce-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:83</code> | <code>pub const ENVELOPE_NONCE_LENGTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-nonce-length</code> | notes | const x_websearch::api::ENVELOPE_NONCE_LENGTH; no reference outside the defining file found by name match |

## xws-envelope-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:259</code> | <code>pub fn open(&amp;self, key: &amp;SigningKey, origin: &amp;str) -&gt; Result&lt;Zeroizing&lt;Vec&lt;u8&gt;&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-open</code> | notes | method x_websearch::api::Envelope::open; no reference outside the defining file found by name match |

## xws-envelope-overhead

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-overhead</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:88</code> | <code>pub const ENVELOPE_OVERHEAD: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-overhead</code> | notes | const x_websearch::api::ENVELOPE_OVERHEAD; no reference outside the defining file found by name match |

## xws-envelope-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:232</code> | <code>pub fn parse(raw: &amp;[u8]) -&gt; Result&lt;Self, String&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-parse</code> | notes | method x_websearch::api::Envelope::parse |

## xws-envelope-shared-x

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-shared-x</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:318</code> | <code>pub fn envelope_shared_x(secret: &amp;NonZeroScalar, public: &amp;PublicKey) -&gt; Result&lt;Zeroizing&lt;[u8; 32]&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-shared-x</code> | notes | fn x_websearch::api::envelope_shared_x |

## xws-envelope-tag-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-envelope-tag-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:84</code> | <code>pub const ENVELOPE_TAG_LENGTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-envelope-tag-length</code> | notes | const x_websearch::api::ENVELOPE_TAG_LENGTH; no reference outside the defining file found by name match |

## xws-es-number

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-es-number</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1360</code> | <code>pub fn es_number(value: f64) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-es-number</code> | notes | fn x_websearch::api::es_number |

## xws-events-method

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-events-method</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:55</code> | <code>pub const EVENTS_METHOD: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-events-method</code> | notes | const x_websearch::kernel::EVENTS_METHOD; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-events-per-poll

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-events-per-poll</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:58</code> | <code>pub const EVENTS_PER_POLL: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-events-per-poll</code> | notes | const x_websearch::kernel::EVENTS_PER_POLL; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-evm-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:163</code> | <code>pub struct EvmConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-config</code> | notes | struct x_websearch::config::EvmConfig; no reference outside the defining file found by name match |

## xws-evm-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:79</code> | <code>pub enum EvmError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-error</code> | notes | enum x_websearch::watch::EvmError |

## xws-evm-requester

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-requester</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:121</code> | <code>pub fn evm_requester(address: [u8; 20]) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-requester</code> | notes | fn x_websearch::attest::evm_requester |

## xws-evm-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:115</code> | <code>pub struct EvmRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-rpc</code> | notes | struct x_websearch::watch::EvmRpc |

## xws-evm-rpc-block-number

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-block-number</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:155</code> | <code>pub fn block_number(&amp;self) -&gt; Result&lt;u64, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-block-number</code> | notes | method x_websearch::watch::EvmRpc::block_number; chain calls: eth_blockNumber at :156 |

## xws-evm-rpc-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-call</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:132</code> | <code>pub fn call(&amp;self, method: &amp;str, params: &amp;Value) -&gt; Result&lt;Value, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-call</code> | notes | method x_websearch::watch::EvmRpc::call; no reference outside the defining file found by name match |

## xws-evm-rpc-eth-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-eth-call</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:164</code> | <code>pub fn eth_call(&amp;self, to: [u8; 20], data: &amp;[u8]) -&gt; Result&lt;Vec&lt;u8&gt;, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-eth-call</code> | notes | method x_websearch::watch::EvmRpc::eth_call; chain calls: eth_call at :166 |

## xws-evm-rpc-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:122</code> | <code>pub fn new(endpoint: &amp;str) -&gt; Result&lt;Self, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-new</code> | notes | method x_websearch::watch::EvmRpc::new |

## xws-evm-rpc-quantity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-quantity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:144</code> | <code>pub fn quantity(&amp;self, method: &amp;str, params: &amp;Value) -&gt; Result&lt;u128, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-evm-rpc-quantity</code> | notes | method x_websearch::watch::EvmRpc::quantity |

## xws-exact

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-exact</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:67</code> | <code>pub const EXACT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-exact</code> | notes | const x_websearch::payment::EXACT |

## xws-extract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-extract</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/extract.rs:233</code> | <code>pub fn extract(content_type: Option&lt;&amp;str&gt;, body: &amp;[u8]) -&gt; Result&lt;Extracted, ExtractError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-extract</code> | notes | fn x_websearch::extract::extract |

## xws-extract-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-extract-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/extract.rs:175</code> | <code>pub enum ExtractError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-extract-error</code> | notes | enum x_websearch::extract::ExtractError |

## xws-extract-error-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-extract-error-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/extract.rs:183</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-extract-error-code</code> | notes | method x_websearch::extract::ExtractError::code; no reference outside the defining file found by name match |

## xws-extracted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-extracted</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/extract.rs:208</code> | <code>pub struct Extracted</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-extracted</code> | notes | struct x_websearch::extract::Extracted |

## xws-fees

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fees</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:103</code> | <code>pub struct Fees</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fees</code> | notes | struct x_websearch::submit::Fees |

## xws-fetch-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetch-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:23</code> | <code>pub enum FetchError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetch-error</code> | notes | enum x_websearch::fetch::FetchError; 99 reference lines in 6 files; callers lists the first reference in each file |

## xws-fetch-error-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetch-error-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:48</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetch-error-code</code> | notes | method x_websearch::fetch::FetchError::code; no reference outside the defining file found by name match |

## xws-fetch-error-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetch-error-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:75</code> | <code>pub const fn status(self) -&gt; u16</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetch-error-status</code> | notes | method x_websearch::fetch::FetchError::status; no reference outside the defining file found by name match |

## xws-fetch-limits

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetch-limits</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:129</code> | <code>pub struct FetchLimits</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetch-limits</code> | notes | struct x_websearch::config::FetchLimits |

## xws-fetch-limits-connect-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetch-limits-connect-timeout</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:139</code> | <code>pub const fn connect_timeout(&amp;self) -&gt; Duration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetch-limits-connect-timeout</code> | notes | method x_websearch::config::FetchLimits::connect_timeout |

## xws-fetch-limits-total-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetch-limits-total-timeout</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:144</code> | <code>pub const fn total_timeout(&amp;self) -&gt; Duration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetch-limits-total-timeout</code> | notes | method x_websearch::config::FetchLimits::total_timeout |

## xws-fetch-route

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetch-route</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:933</code> | <code>pub fn fetch_route(fetcher: &amp;Fetcher, store: &amp;ContentStore, request: &amp;Request) -&gt; Response</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetch-route</code> | notes | fn x_websearch::fetch::fetch_route |

## xws-fetched-page

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetched-page</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:722</code> | <code>pub struct FetchedPage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetched-page</code> | notes | struct x_websearch::fetch::FetchedPage |

## xws-fetcher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetcher</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:735</code> | <code>pub struct Fetcher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetcher</code> | notes | struct x_websearch::fetch::Fetcher |

## xws-fetcher-destination

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetcher-destination</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:779</code> | <code>pub fn destination(&amp;self, url: &amp;Url) -&gt; Result&lt;SocketAddr, FetchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetcher-destination</code> | notes | method x_websearch::fetch::Fetcher::destination |

## xws-fetcher-fetch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetcher-fetch</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:855</code> | <code>pub fn fetch(&amp;self, url: &amp;str) -&gt; Result&lt;FetchedPage, FetchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetcher-fetch</code> | notes | method x_websearch::fetch::Fetcher::fetch |

## xws-fetcher-limits

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetcher-limits</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:766</code> | <code>pub const fn limits(&amp;self) -&gt; &amp;FetchLimits</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetcher-limits</code> | notes | method x_websearch::fetch::Fetcher::limits |

## xws-fetcher-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fetcher-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:749</code> | <code>pub fn new(limits: FetchLimits) -&gt; Result&lt;Self, FetchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fetcher-new</code> | notes | method x_websearch::fetch::Fetcher::new |

## xws-fields

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fields</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:76</code> | <code>pub struct Fields</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fields</code> | notes | struct x_websearch::index::Fields |

## xws-fulfil-calldata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fulfil-calldata</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:65</code> | <code>pub fn fulfil_calldata(ready: &amp;Ready) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fulfil-calldata</code> | notes | fn x_websearch::submit::fulfil_calldata; spec/layerx-merge/spec.kvx task 5.2.4 adds fulfil_program_calldata beside this function; this one is kept |

## xws-fulfil-gas-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fulfil-gas-limit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:90</code> | <code>pub fn fulfil_gas_limit(calldata: &amp;[u8], signatures: usize, callback_gas: u64) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fulfil-gas-limit</code> | notes | fn x_websearch::submit::fulfil_gas_limit |

## xws-fulfil-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-fulfil-signature</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:11</code> | <code>pub const FULFIL_SIGNATURE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-fulfil-signature</code> | notes | const x_websearch::submit::FULFIL_SIGNATURE |

## xws-gate-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-gate-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1035</code> | <code>pub enum GateError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-gate-error</code> | notes | enum x_websearch::payment::GateError |

## xws-gateway-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-gateway-config</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/config.rs:156</code> | <code>pub struct GatewayConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-gateway-config</code> | notes | struct x_websearch::config::GatewayConfig; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.4.3 parses receipt_keys in place of the gateway key fields (services/interop/crates/x-websearch/src/config.rs:873-881) |

Contract context <code>contract_context.11.x-websearch-config</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:373</code> | <code>pub struct GatewayRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc</code> | notes | struct x_websearch::payment::GatewayRpc; spec/layerx-merge/spec.kvx task 5.3.2 do_2/do_16 replace lx_sendActivity, lx_getActivityStatus, lx_getReceipt with eth_sendRawTransaction and pax_sequencerReceipt; 5.6.6 do_4 drops the authorization_file only the relay read |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-gateway-rpc-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc-call</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:448</code> | <code>pub fn call(&amp;self, method: &amp;str, params: &amp;Value) -&gt; Option&lt;RpcAnswer&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc-call</code> | notes | method x_websearch::payment::GatewayRpc::call; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.3.2 do_2/do_16 replace lx_sendActivity, lx_getActivityStatus, lx_getReceipt with eth_sendRawTransaction and pax_sequencerReceipt; 5.6.6 do_4 drops the authorization_file only the relay read |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-gateway-rpc-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc-new</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:385</code> | <code>pub fn new(endpoint: &amp;str) -&gt; Result&lt;Self, GateError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc-new</code> | notes | method x_websearch::payment::GatewayRpc::new; spec/layerx-merge/spec.kvx task 5.3.2 do_2/do_16 replace lx_sendActivity, lx_getActivityStatus, lx_getReceipt with eth_sendRawTransaction and pax_sequencerReceipt; 5.6.6 do_4 drops the authorization_file only the relay read |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-gateway-rpc-with-authorization-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc-with-authorization-file</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:428</code> | <code>pub fn with_authorization_file(mut self, path: &amp;Path) -&gt; Result&lt;Self, GateError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-gateway-rpc-with-authorization-file</code> | notes | method x_websearch::payment::GatewayRpc::with_authorization_file; spec/layerx-merge/spec.kvx task 5.3.2 do_2/do_16 replace lx_sendActivity, lx_getActivityStatus, lx_getReceipt with eth_sendRawTransaction and pax_sequencerReceipt; 5.6.6 do_4 drops the authorization_file only the relay read |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-get-attestors-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-get-attestors-signature</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:15</code> | <code>pub const GET_ATTESTORS_SIGNATURE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-get-attestors-signature</code> | notes | const x_websearch::submit::GET_ATTESTORS_SIGNATURE; no reference outside the defining file found by name match |

## xws-get-request-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-get-request-signature</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:14</code> | <code>pub const GET_REQUEST_SIGNATURE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-get-request-signature</code> | notes | const x_websearch::submit::GET_REQUEST_SIGNATURE; no reference outside the defining file found by name match |

## xws-grant-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-grant-bytes</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:73</code> | <code>pub const GRANT_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-grant-bytes</code> | notes | const x_websearch::payment::GRANT_BYTES; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.3.2 do_10 reads the grant with eth_call getPayerGrant(grantId) on 0x101A; kernel grant ordinals deleted |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:102</code> | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-hex</code> | notes | fn x_websearch::payment::hex; 85 reference lines in 15 files; callers lists the first reference in each file |

## xws-hex0x

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-hex0x</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:47</code> | <code>pub fn hex0x(bytes: &amp;[u8]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-hex0x</code> | notes | fn x_websearch::watch::hex0x; 78 reference lines in 7 files; callers lists the first reference in each file |

## xws-html-text

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-html-text</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/extract.rs:267</code> | <code>pub fn html_text(source: &amp;str) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-html-text</code> | notes | fn x_websearch::extract::html_text |

## xws-http-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-http-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:578</code> | <code>pub struct HttpClient</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-http-client</code> | notes | struct x_websearch::fetch::HttpClient |

## xws-http-client-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-http-client-get</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:640</code> | <code>pub fn get(&amp;self, url: &amp;Url, address: SocketAddr, deadline: Instant, body_limit: usize, headers: &amp;[(&amp;str, &amp;str)]) -&gt; Result&lt;HttpResponse, FetchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-http-client-get</code> | notes | method x_websearch::fetch::HttpClient::get; no reference outside the defining file found by name match |

## xws-http-client-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-http-client-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:587</code> | <code>pub fn new(connect_timeout: Duration) -&gt; Result&lt;Self, FetchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-http-client-new</code> | notes | method x_websearch::fetch::HttpClient::new |

## xws-http-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-http-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:519</code> | <code>pub struct HttpResponse</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-http-response</code> | notes | struct x_websearch::fetch::HttpResponse; no reference outside the defining file found by name match |

## xws-http-response-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-http-response-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:527</code> | <code>pub fn header(&amp;self, name: &amp;str) -&gt; Option&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-http-response-header</code> | notes | method x_websearch::fetch::HttpResponse::header; no reference outside the defining file found by name match |

## xws-index-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-index-directory</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:13</code> | <code>pub const INDEX_DIRECTORY: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-index-directory</code> | notes | const x_websearch::index::INDEX_DIRECTORY; no reference outside the defining file found by name match |

## xws-index-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-index-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:22</code> | <code>pub enum IndexError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-index-error</code> | notes | enum x_websearch::index::IndexError |

## xws-index-error-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-index-error-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:32</code> | <code>pub const fn code(&amp;self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-index-error-code</code> | notes | method x_websearch::index::IndexError::code; no reference outside the defining file found by name match |

## xws-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:496</code> | <code>pub struct Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal</code> | notes | struct x_websearch::submit::Journal; 48 reference lines in 2 files; callers lists the first reference in each file |

## xws-journal-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-entry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:444</code> | <code>pub struct JournalEntry</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-entry</code> | notes | struct x_websearch::submit::JournalEntry |

## xws-journal-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:522</code> | <code>pub fn load(&amp;self, request_id: u64) -&gt; Result&lt;Option&lt;JournalEntry&gt;, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-load</code> | notes | method x_websearch::submit::Journal::load; no reference outside the defining file found by name match |

## xws-journal-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:503</code> | <code>pub fn open(directory: &amp;Path) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-open</code> | notes | method x_websearch::submit::Journal::open; no reference outside the defining file found by name match |

## xws-journal-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-path</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:512</code> | <code>pub fn path(&amp;self, request_id: u64) -&gt; PathBuf</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-path</code> | notes | method x_websearch::submit::Journal::path; no reference outside the defining file found by name match |

## xws-journal-requests

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-requests</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:562</code> | <code>pub fn requests(&amp;self) -&gt; Result&lt;Vec&lt;u64&gt;, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-requests</code> | notes | method x_websearch::submit::Journal::requests; no reference outside the defining file found by name match |

## xws-journal-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:398</code> | <code>pub enum JournalState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-state</code> | notes | enum x_websearch::submit::JournalState |

## xws-journal-state-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-state-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:412</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-state-code</code> | notes | method x_websearch::submit::JournalState::code; no reference outside the defining file found by name match |

## xws-journal-state-completed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-state-completed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:436</code> | <code>pub const fn completed(self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-state-completed</code> | notes | method x_websearch::submit::JournalState::completed |

## xws-journal-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-journal-store</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:542</code> | <code>pub fn store(&amp;self, entry: &amp;JournalEntry) -&gt; Result&lt;(), SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-journal-store</code> | notes | method x_websearch::submit::Journal::store |

## xws-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1044</code> | <code>pub enum Json</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-json</code> | notes | enum x_websearch::api::Json |

## xws-keccak

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-keccak</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:35</code> | <code>pub fn keccak(bytes: &amp;[u8]) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-keccak</code> | notes | fn x_websearch::watch::keccak |

## xws-kernel-attestor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-attestor</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:475</code> | <code>pub struct KernelAttestor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-attestor</code> | notes | struct x_websearch::kernel::KernelAttestor; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-attestor-attest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-attestor-attest</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:539</code> | <code>pub fn attest(&amp;self, request: &amp;ProgramRequest) -&gt; Result&lt;Answer, AttestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-attestor-attest</code> | notes | method x_websearch::kernel::KernelAttestor::attest; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-attestor-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-attestor-new</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:486</code> | <code>pub fn new(key: SigningKey, network_id: u32, fetcher: Arc&lt;Fetcher&gt;, index: Arc&lt;WebIndex&gt;, store: Arc&lt;ContentStore&gt;) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-attestor-new</code> | notes | method x_websearch::kernel::KernelAttestor::new; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-attestor-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-attestor-signer</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:505</code> | <code>pub const fn signer(&amp;self) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-attestor-signer</code> | notes | method x_websearch::kernel::KernelAttestor::signer; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-config</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:222</code> | <code>pub struct KernelConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-config</code> | notes | struct x_websearch::config::KernelConfig; spec/layerx-merge/spec.kvx task 5.6.6 removes KernelConfig and its parser (services/interop/crates/x-websearch/src/config.rs:222, :961); still present at HEAD |

## xws-kernel-config-poll-interval

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-config-poll-interval</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:243</code> | <code>pub const fn poll_interval(&amp;self) -&gt; Duration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-config-poll-interval</code> | notes | method x_websearch::config::KernelConfig::poll_interval; spec/layerx-merge/spec.kvx task 5.6.6 removes KernelConfig and its parser (services/interop/crates/x-websearch/src/config.rs:222, :961); still present at HEAD |

## xws-kernel-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-error</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:135</code> | <code>pub enum KernelError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-error</code> | notes | enum x_websearch::kernel::KernelError; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-register

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-register</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1367</code> | <code>pub fn register(routes: &amp;mut RouteTable, exchange: &amp;Arc&lt;ProgramExchange&gt;) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-register</code> | notes | fn x_websearch::kernel::register; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1655</code> | <code>pub struct KernelRelay</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay</code> | notes | struct x_websearch::kernel::KernelRelay; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay-entries

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-entries</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1857</code> | <code>pub fn entries(&amp;self) -&gt; &amp;[RelayEntry]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-entries</code> | notes | method x_websearch::kernel::KernelRelay::entries; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay-journal-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-journal-path</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1863</code> | <code>pub fn journal_path(&amp;self) -&gt; Option&lt;&amp;Path&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-journal-path</code> | notes | method x_websearch::kernel::KernelRelay::journal_path; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay-lag

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-lag</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1711</code> | <code>pub fn lag(&amp;self, now_ms: u64) -&gt; RelayLag</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-lag</code> | notes | method x_websearch::kernel::KernelRelay::lag; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-new</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1675</code> | <code>pub fn new(watcher: KernelWatcher, attestor: KernelAttestor, exchange: Arc&lt;ProgramExchange&gt;, set: AttestorSet, submitter: ObservationSubmitter) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-new</code> | notes | method x_websearch::kernel::KernelRelay::new; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-open</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1745</code> | <code>pub fn open(watcher: KernelWatcher, attestor: KernelAttestor, exchange: Arc&lt;ProgramExchange&gt;, set: AttestorSet, submitter: ObservationSubmitter, state_dir: &amp;Path) -&gt; Result&lt;Self, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-open</code> | notes | method x_websearch::kernel::KernelRelay::open; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay-queued

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-queued</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1851</code> | <code>pub fn queued(&amp;self) -&gt; &amp;[ProgramRequest]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-queued</code> | notes | method x_websearch::kernel::KernelRelay::queued; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay-step

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-step</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1909</code> | <code>pub fn step(&amp;mut self, now_ms: u64) -&gt; Result&lt;Vec&lt;Step&gt;, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-step</code> | notes | method x_websearch::kernel::KernelRelay::step; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-relay-with-backoff

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-with-backoff</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1703</code> | <code>pub fn with_backoff(mut self, base_ms: u64, max_ms: u64) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-relay-with-backoff</code> | notes | method x_websearch::kernel::KernelRelay::with_backoff; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-request-topic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-request-topic</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:50</code> | <code>pub fn request_topic(topic: &amp;[u8]) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-request-topic</code> | notes | fn x_websearch::kernel::request_topic; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD; chain calls: lx_getProgramEvents at :55, 0x1001 at :75 |

## xws-kernel-watcher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:293</code> | <code>pub struct KernelWatcher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher</code> | notes | struct x_websearch::kernel::KernelWatcher; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-watcher-fetch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-fetch</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:435</code> | <code>pub fn fetch(&amp;self) -&gt; Result&lt;(u64, Vec&lt;(Vec&lt;u8&gt;, ProgramRequest)&gt;), KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-fetch</code> | notes | method x_websearch::kernel::KernelWatcher::fetch; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-watcher-next-sequence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-next-sequence</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:368</code> | <code>pub const fn next_sequence(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-next-sequence</code> | notes | method x_websearch::kernel::KernelWatcher::next_sequence; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-watcher-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-open</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:308</code> | <code>pub fn open(endpoint: &amp;str, state_dir: &amp;Path, start: u64) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-open</code> | notes | method x_websearch::kernel::KernelWatcher::open; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-watcher-poll

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-poll</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:412</code> | <code>pub fn poll(&amp;mut self) -&gt; Result&lt;Vec&lt;ProgramRequest&gt;, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-poll</code> | notes | method x_websearch::kernel::KernelWatcher::poll; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-watcher-resume-at

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-resume-at</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:424</code> | <code>pub fn resume_at(&amp;mut self, next: u64) -&gt; Result&lt;(), KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-resume-at</code> | notes | method x_websearch::kernel::KernelWatcher::resume_at; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-watcher-topics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-topics</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:362</code> | <code>pub fn topics(&amp;self) -&gt; &amp;[Vec&lt;u8&gt;]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-topics</code> | notes | method x_websearch::kernel::KernelWatcher::topics; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-watcher-with-authorization-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-with-authorization-file</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:328</code> | <code>pub fn with_authorization_file(mut self, path: &amp;Path) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-with-authorization-file</code> | notes | method x_websearch::kernel::KernelWatcher::with_authorization_file; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-kernel-watcher-with-topics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-with-topics</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:343</code> | <code>pub fn with_topics(mut self, topics: &amp;[&amp;[u8]]) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kernel-watcher-with-topics</code> | notes | method x_websearch::kernel::KernelWatcher::with_topics; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-key-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-key-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:43</code> | <code>pub struct KeyError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-key-error</code> | notes | struct x_websearch::keys::KeyError; re-exported at services/interop/crates/x-websearch/src/lib.rs:30 |

## xws-key-files

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-key-files</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:87</code> | <code>pub struct KeyFiles</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-key-files</code> | notes | struct x_websearch::keys::KeyFiles; re-exported at services/interop/crates/x-websearch/src/lib.rs:30 |

## xws-key-files-from-env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-key-files-from-env</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:110</code> | <code>pub fn from_env() -&gt; Result&lt;Self, KeyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-key-files-from-env</code> | notes | method x_websearch::keys::KeyFiles::from_env |

## xws-key-files-from-lookup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-key-files-from-lookup</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:116</code> | <code>pub fn from_lookup(lookup: impl Fn(&amp;str) -&gt; Option&lt;OsString&gt;) -&gt; Result&lt;Self, KeyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-key-files-from-lookup</code> | notes | method x_websearch::keys::KeyFiles::from_lookup |

## xws-key-files-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-key-files-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:133</code> | <code>pub fn load(&amp;self) -&gt; Result&lt;Keys, KeyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-key-files-load</code> | notes | method x_websearch::keys::KeyFiles::load; no reference outside the defining file found by name match |

## xws-key-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-key-refusal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:32</code> | <code>pub enum KeyRefusal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-key-refusal</code> | notes | enum x_websearch::keys::KeyRefusal |

## xws-key-role

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-key-role</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:14</code> | <code>pub enum KeyRole</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-key-role</code> | notes | enum x_websearch::keys::KeyRole |

## xws-key-role-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-key-role-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:22</code> | <code>pub const fn variable(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-key-role-variable</code> | notes | method x_websearch::keys::KeyRole::variable; no reference outside the defining file found by name match |

## xws-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-keys</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:241</code> | <code>pub struct Keys</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-keys</code> | notes | struct x_websearch::keys::Keys; re-exported at services/interop/crates/x-websearch/src/lib.rs:30 |

## xws-keys-attestor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-keys-attestor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:249</code> | <code>pub const fn attestor(&amp;self) -&gt; Option&lt;&amp;SecpKey&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-keys-attestor</code> | notes | method x_websearch::keys::Keys::attestor |

## xws-keys-receiver

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-keys-receiver</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:259</code> | <code>pub const fn receiver(&amp;self) -&gt; &amp;ReceiverKey</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-keys-receiver</code> | notes | method x_websearch::keys::Keys::receiver |

## xws-keys-submitter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-keys-submitter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:254</code> | <code>pub const fn submitter(&amp;self) -&gt; Option&lt;&amp;SecpKey&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-keys-submitter</code> | notes | method x_websearch::keys::Keys::submitter |

## xws-kind-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-kind-api</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:53</code> | <code>pub const KIND_API: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-kind-api</code> | notes | const x_websearch::api::KIND_API |

## xws-level

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-level</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:360</code> | <code>pub enum Level</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-level</code> | notes | enum x_websearch::attest::Level |

## xws-level-majority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-level-majority</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:62</code> | <code>pub const LEVEL_MAJORITY: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-level-majority</code> | notes | const x_websearch::api::LEVEL_MAJORITY |

## xws-level-single

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-level-single</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:63</code> | <code>pub const LEVEL_SINGLE: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-level-single</code> | notes | const x_websearch::api::LEVEL_SINGLE |

## xws-limits

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-limits</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:17</code> | <code>pub struct Limits</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-limits</code> | notes | struct x_websearch::server::Limits; re-exported at services/interop/crates/x-websearch/src/lib.rs:31 |

## xws-links

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-links</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:288</code> | <code>pub fn links(html: &amp;str) -&gt; Vec&lt;String&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-links</code> | notes | fn x_websearch::crawl::links |

## xws-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:337</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Config, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-load</code> | notes | fn x_websearch::config::load; 50 reference lines in 14 files; callers lists the first reference in each file; re-exported at services/interop/crates/x-websearch/src/lib.rs:29 |

## xws-max-api-body-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-api-body-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:69</code> | <code>pub const MAX_API_BODY_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-api-body-bytes</code> | notes | const x_websearch::api::MAX_API_BODY_BYTES; no reference outside the defining file found by name match |

## xws-max-api-header-name-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-api-header-name-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:67</code> | <code>pub const MAX_API_HEADER_NAME_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-api-header-name-bytes</code> | notes | const x_websearch::api::MAX_API_HEADER_NAME_BYTES; no reference outside the defining file found by name match |

## xws-max-api-header-value-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-api-header-value-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:68</code> | <code>pub const MAX_API_HEADER_VALUE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-api-header-value-bytes</code> | notes | const x_websearch::api::MAX_API_HEADER_VALUE_BYTES; no reference outside the defining file found by name match |

## xws-max-api-headers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-api-headers</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:66</code> | <code>pub const MAX_API_HEADERS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-api-headers</code> | notes | const x_websearch::api::MAX_API_HEADERS; no reference outside the defining file found by name match |

## xws-max-api-pointer-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-api-pointer-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:71</code> | <code>pub const MAX_API_POINTER_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-api-pointer-bytes</code> | notes | const x_websearch::api::MAX_API_POINTER_BYTES; no reference outside the defining file found by name match |

## xws-max-api-pointers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-api-pointers</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:70</code> | <code>pub const MAX_API_POINTERS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-api-pointers</code> | notes | const x_websearch::api::MAX_API_POINTERS; no reference outside the defining file found by name match |

## xws-max-api-url-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-api-url-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:65</code> | <code>pub const MAX_API_URL_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-api-url-bytes</code> | notes | const x_websearch::api::MAX_API_URL_BYTES; no reference outside the defining file found by name match |

## xws-max-attempts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-attempts</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:31</code> | <code>pub const MAX_ATTEMPTS: u32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-attempts</code> | notes | const x_websearch::watch::MAX_ATTEMPTS |

## xws-max-block-range

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-block-range</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:21</code> | <code>pub const MAX_BLOCK_RANGE: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-block-range</code> | notes | const x_websearch::watch::MAX_BLOCK_RANGE; no reference outside the defining file found by name match |

## xws-max-config-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-config-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:7</code> | <code>pub const MAX_CONFIG_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-config-bytes</code> | notes | const x_websearch::config::MAX_CONFIG_BYTES |

## xws-max-confirmations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-confirmations</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:18</code> | <code>pub const MAX_CONFIRMATIONS: u32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-confirmations</code> | notes | const x_websearch::config::MAX_CONFIRMATIONS; no reference outside the defining file found by name match |

## xws-max-content-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-content-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:14</code> | <code>pub const MAX_CONTENT_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-content-bytes</code> | notes | const x_websearch::content::MAX_CONTENT_BYTES |

## xws-max-crawl-interval-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-crawl-interval-seconds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:23</code> | <code>pub const MAX_CRAWL_INTERVAL_SECONDS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-crawl-interval-seconds</code> | notes | const x_websearch::config::MAX_CRAWL_INTERVAL_SECONDS |

## xws-max-credential-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-credential-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:77</code> | <code>pub const MAX_CREDENTIAL_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-credential-bytes</code> | notes | const x_websearch::api::MAX_CREDENTIAL_BYTES; no reference outside the defining file found by name match |

## xws-max-credential-headers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-credential-headers</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:76</code> | <code>pub const MAX_CREDENTIAL_HEADERS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-credential-headers</code> | notes | const x_websearch::api::MAX_CREDENTIAL_HEADERS; no reference outside the defining file found by name match |

## xws-max-depth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-depth</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:16</code> | <code>pub const MAX_DEPTH: u32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-depth</code> | notes | const x_websearch::config::MAX_DEPTH |

## xws-max-did-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-did-bytes</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/config.rs:25</code> | <code>pub const MAX_DID_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-did-bytes</code> | notes | const x_websearch::config::MAX_DID_BYTES; spec/layerx-merge/spec.kvx task 11.5.121 do_2 names the payer by its EVM address bound through addr 0x1004 instead of LAYERX-PAYER-DID |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-max-envelope-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-envelope-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:91</code> | <code>pub const MAX_ENVELOPE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-envelope-bytes</code> | notes | const x_websearch::api::MAX_ENVELOPE_BYTES; no reference outside the defining file found by name match |

## xws-max-envelopes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-envelopes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:74</code> | <code>pub const MAX_ENVELOPES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-envelopes</code> | notes | const x_websearch::api::MAX_ENVELOPES; no reference outside the defining file found by name match |

## xws-max-json-depth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-json-depth</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:97</code> | <code>pub const MAX_JSON_DEPTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-json-depth</code> | notes | const x_websearch::api::MAX_JSON_DEPTH; no reference outside the defining file found by name match |

## xws-max-kernel-poll-interval-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-kernel-poll-interval-ms</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:27</code> | <code>pub const MAX_KERNEL_POLL_INTERVAL_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-kernel-poll-interval-ms</code> | notes | const x_websearch::config::MAX_KERNEL_POLL_INTERVAL_MS; spec/layerx-merge/spec.kvx task 5.6.6 removes KernelConfig and its parser (services/interop/crates/x-websearch/src/config.rs:222, :961); still present at HEAD |

## xws-max-key-file-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-key-file-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:11</code> | <code>pub const MAX_KEY_FILE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-key-file-bytes</code> | notes | const x_websearch::keys::MAX_KEY_FILE_BYTES |

## xws-max-links-per-page

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-links-per-page</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:12</code> | <code>pub const MAX_LINKS_PER_PAGE: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-links-per-page</code> | notes | const x_websearch::crawl::MAX_LINKS_PER_PAGE; no reference outside the defining file found by name match |

## xws-max-observation-signatures

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-observation-signatures</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:70</code> | <code>pub const MAX_OBSERVATION_SIGNATURES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-observation-signatures</code> | notes | const x_websearch::kernel::MAX_OBSERVATION_SIGNATURES; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-max-pages-per-cycle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-pages-per-cycle</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:15</code> | <code>pub const MAX_PAGES_PER_CYCLE: u32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-pages-per-cycle</code> | notes | const x_websearch::config::MAX_PAGES_PER_CYCLE |

## xws-max-payload-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-payload-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:24</code> | <code>pub const MAX_PAYLOAD_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-payload-bytes</code> | notes | const x_websearch::watch::MAX_PAYLOAD_BYTES; no reference outside the defining file found by name match |

## xws-max-peers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-peers</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:13</code> | <code>pub const MAX_PEERS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-peers</code> | notes | const x_websearch::config::MAX_PEERS; no reference outside the defining file found by name match |

## xws-max-politeness-delay-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-politeness-delay-ms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:17</code> | <code>pub const MAX_POLITENESS_DELAY_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-politeness-delay-ms</code> | notes | const x_websearch::config::MAX_POLITENESS_DELAY_MS |

## xws-max-query-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-query-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:23</code> | <code>pub const MAX_QUERY_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-query-bytes</code> | notes | const x_websearch::search::MAX_QUERY_BYTES |

## xws-max-query-terms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-query-terms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:26</code> | <code>pub const MAX_QUERY_TERMS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-query-terms</code> | notes | const x_websearch::search::MAX_QUERY_TERMS; no reference outside the defining file found by name match |

## xws-max-record-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-record-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:41</code> | <code>pub const MAX_RECORD_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-record-bytes</code> | notes | const x_websearch::attest::MAX_RECORD_BYTES |

## xws-max-response-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-response-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:33</code> | <code>pub const MAX_RESPONSE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-response-bytes</code> | notes | const x_websearch::attest::MAX_RESPONSE_BYTES |

## xws-max-results

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-results</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:20</code> | <code>pub const MAX_RESULTS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-results</code> | notes | const x_websearch::search::MAX_RESULTS |

## xws-max-retry-backoff-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-retry-backoff-ms</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:93</code> | <code>pub const MAX_RETRY_BACKOFF_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-retry-backoff-ms</code> | notes | const x_websearch::kernel::MAX_RETRY_BACKOFF_MS; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-max-seeds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-seeds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:12</code> | <code>pub const MAX_SEEDS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-seeds</code> | notes | const x_websearch::config::MAX_SEEDS; no reference outside the defining file found by name match |

## xws-max-title-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-title-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:16</code> | <code>pub const MAX_TITLE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-title-bytes</code> | notes | const x_websearch::index::MAX_TITLE_BYTES |

## xws-max-url-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-max-url-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:14</code> | <code>pub const MAX_URL_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-max-url-bytes</code> | notes | const x_websearch::config::MAX_URL_BYTES |

## xws-media-type-essence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-media-type-essence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:217</code> | <code>pub fn media_type_essence(media_type: &amp;str) -&gt; Result&lt;String, CanonicalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-media-type-essence</code> | notes | fn x_websearch::canonical::media_type_essence |

## xws-metered

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-metered</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:66</code> | <code>pub const METERED: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-metered</code> | notes | const x_websearch::payment::METERED |

## xws-method-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-method-get</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:58</code> | <code>pub const METHOD_GET: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-method-get</code> | notes | const x_websearch::api::METHOD_GET |

## xws-method-post

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-method-post</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:59</code> | <code>pub const METHOD_POST: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-method-post</code> | notes | const x_websearch::api::METHOD_POST |

## xws-network-word

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-network-word</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:129</code> | <code>pub fn network_word(id: u64) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-network-word</code> | notes | fn x_websearch::attest::network_word |

## xws-observation-activity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-activity</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:61</code> | <code>pub const OBSERVATION_ACTIVITY: u32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-activity</code> | notes | const x_websearch::kernel::OBSERVATION_ACTIVITY; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-bytes</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:567</code> | <code>pub fn observation_bytes(network_id: u32, request: &amp;ProgramRequest, ready: &amp;Ready) -&gt; Result&lt;Vec&lt;u8&gt;, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-bytes</code> | notes | fn x_websearch::kernel::observation_bytes; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-header-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-header-bytes</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:67</code> | <code>pub const OBSERVATION_HEADER_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-header-bytes</code> | notes | const x_websearch::kernel::OBSERVATION_HEADER_BYTES; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-submitter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:763</code> | <code>pub struct ObservationSubmitter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter</code> | notes | struct x_websearch::kernel::ObservationSubmitter; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-submitter-consumed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-consumed</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:940</code> | <code>pub fn consumed(&amp;self, account_sequence: u64) -&gt; Result&lt;bool, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-consumed</code> | notes | method x_websearch::kernel::ObservationSubmitter::consumed; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-submitter-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-did</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:871</code> | <code>pub fn did(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-did</code> | notes | method x_websearch::kernel::ObservationSubmitter::did; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-submitter-lookup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-lookup</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:918</code> | <code>pub fn lookup(&amp;self, activity_id: &amp;[u8; 32]) -&gt; Option&lt;RpcAnswer&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-lookup</code> | notes | method x_websearch::kernel::ObservationSubmitter::lookup; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD; chain calls: lx_getActivityStatus at :920, lx_getReceipt at :928 |

## xws-observation-submitter-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-new</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:775</code> | <code>pub fn new(endpoint: &amp;str, key: SubmitterKey, did: String, network_id: u32, fee_limit: u128) -&gt; Result&lt;Self, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-new</code> | notes | method x_websearch::kernel::ObservationSubmitter::new; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-submitter-send

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-send</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:909</code> | <code>pub fn send(&amp;self, activity: &amp;[u8]) -&gt; Option&lt;RpcAnswer&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-send</code> | notes | method x_websearch::kernel::ObservationSubmitter::send; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD; chain calls: lx_sendActivity at :911 |

## xws-observation-submitter-sign

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-sign</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:882</code> | <code>pub fn sign(&amp;self, observation: &amp;[u8], now_ms: u64) -&gt; Result&lt;Signed, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-sign</code> | notes | method x_websearch::kernel::ObservationSubmitter::sign; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-submitter-submit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-submit</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:860</code> | <code>pub fn submit(&amp;self, observation: &amp;[u8], now_ms: u64) -&gt; Result&lt;Value, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-submit</code> | notes | method x_websearch::kernel::ObservationSubmitter::submit; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-submitter-with-authorization-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-with-authorization-file</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:792</code> | <code>pub fn with_authorization_file(mut self, path: &amp;Path) -&gt; Result&lt;Self, KernelError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-with-authorization-file</code> | notes | method x_websearch::kernel::ObservationSubmitter::with_authorization_file; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-observation-submitter-with-receipt-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-with-receipt-key</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:801</code> | <code>pub fn with_receipt_key(mut self, key: [u8; 32]) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-observation-submitter-with-receipt-key</code> | notes | method x_websearch::kernel::ObservationSubmitter::with_receipt_key; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD; chain calls: lx_getSequence at :839 via private fn next_sequence |

## xws-offer-timeout-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-offer-timeout-seconds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:71</code> | <code>pub const OFFER_TIMEOUT_SECONDS: u32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-offer-timeout-seconds</code> | notes | const x_websearch::payment::OFFER_TIMEOUT_SECONDS; no reference outside the defining file found by name match |

## xws-open-envelope

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-open-envelope</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:402</code> | <code>pub fn open_envelope(raw: &amp;[u8], key: &amp;SigningKey, origin: &amp;str) -&gt; Result&lt;Zeroizing&lt;Vec&lt;u8&gt;&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-open-envelope</code> | notes | fn x_websearch::api::open_envelope |

## xws-origin-evm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-origin-evm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:27</code> | <code>pub const ORIGIN_EVM: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-origin-evm</code> | notes | const x_websearch::attest::ORIGIN_EVM |

## xws-origin-program

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-origin-program</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:30</code> | <code>pub const ORIGIN_PROGRAM: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-origin-program</code> | notes | const x_websearch::attest::ORIGIN_PROGRAM |

## xws-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:582</code> | <code>pub enum Outcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-outcome</code> | notes | enum x_websearch::submit::Outcome; 147 reference lines in 6 files; callers lists the first reference in each file |

## xws-outcome-completed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-outcome-completed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:602</code> | <code>pub const fn completed(self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-outcome-completed</code> | notes | method x_websearch::submit::Outcome::completed |

## xws-page-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-page-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:116</code> | <code>pub enum PageOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-page-outcome</code> | notes | enum x_websearch::crawl::PageOutcome |

## xws-parse-content-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-parse-content-type</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/extract.rs:217</code> | <code>pub fn parse_content_type(value: &amp;str) -&gt; Result&lt;(String, Option&lt;String&gt;), ExtractError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-parse-content-type</code> | notes | fn x_websearch::extract::parse_content_type |

## xws-parse-hex32

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-parse-hex32</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:519</code> | <code>pub fn parse_hex32(text: &amp;str) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-parse-hex32</code> | notes | fn x_websearch::config::parse_hex32 |

## xws-parse-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-parse-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1293</code> | <code>pub fn parse_json(text: &amp;str) -&gt; Result&lt;Json, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-parse-json</code> | notes | fn x_websearch::api::parse_json |

## xws-parse-quantity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-parse-quantity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:66</code> | <code>pub fn parse_quantity(text: &amp;str) -&gt; Option&lt;u128&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-parse-quantity</code> | notes | fn x_websearch::watch::parse_quantity |

## xws-payer-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payer-did</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:64</code> | <code>pub const PAYER_DID: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payer-did</code> | notes | const x_websearch::payment::PAYER_DID; spec/layerx-merge/spec.kvx task 11.5.122 do_4 retires kernel account naming (LAYERX-PAYER-DID, agent:{did}:main, :asset:{) for web payments; payer named by EVM address via 0x1004 |

## xws-payer-did-valid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payer-did-valid</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/config.rs:207</code> | <code>pub fn payer_did_valid(did: &amp;str) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payer-did-valid</code> | notes | fn x_websearch::config::payer_did_valid; spec/layerx-merge/spec.kvx task 11.5.121 do_2 names the payer by its EVM address bound through addr 0x1004 instead of LAYERX-PAYER-DID |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:177</code> | <code>pub struct PaymentConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-config</code> | notes | struct x_websearch::config::PaymentConfig |

## xws-payment-gate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1071</code> | <code>pub struct PaymentGate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate</code> | notes | struct x_websearch::payment::PaymentGate |

## xws-payment-gate-assets

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-assets</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1175</code> | <code>pub const fn assets(&amp;self) -&gt; &amp;AcceptedAssets</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-assets</code> | notes | method x_websearch::payment::PaymentGate::assets; no reference outside the defining file found by name match |

## xws-payment-gate-draw-fee-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-draw-fee-limit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1197</code> | <code>pub const fn draw_fee_limit(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-draw-fee-limit</code> | notes | method x_websearch::payment::PaymentGate::draw_fee_limit |

## xws-payment-gate-install

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-install</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1707</code> | <code>pub fn install(gate: &amp;Arc&lt;Self&gt;, routes: &amp;mut RouteTable, route: Route, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-install</code> | notes | method x_websearch::payment::PaymentGate::install; chain calls: lx_getSequence at :2011 via private fn sequence, lx_sendActivity at :2041 via private fn submit, lx_getActivityStatus at :2063 via private fn recover, lx_getReceipt at :2071 via private fn recover, lx_getReceipt at :2254 via private fn exact |

## xws-payment-gate-install-validated

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-install-validated</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1668</code> | <code>pub fn install_validated(gate: &amp;Arc&lt;Self&gt;, routes: &amp;mut RouteTable, route: Route, validate: impl Fn(&amp;Request) -&gt; Result&lt;(), Response&gt; + Send + Sync + 'static, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-install-validated</code> | notes | method x_websearch::payment::PaymentGate::install_validated |

## xws-payment-gate-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-new</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1097</code> | <code>pub fn new(config: &amp;Config, receiver: &amp;SigningKey, conformance: ConformanceSuite, clock: Clock) -&gt; Result&lt;Self, GateError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-new</code> | notes | method x_websearch::payment::PaymentGate::new; spec/layerx-merge/spec.kvx task 5.3.2 settles paid HTTP by 0x101A draw/payExact and verifies receipts by ReceiptKeys; 6.5.51 one EVM nonce per draw |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-paid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-paid</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1689</code> | <code>pub fn paid(gate: &amp;Arc&lt;Self&gt;, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-paid</code> | notes | method x_websearch::payment::PaymentGate::paid; no reference outside the defining file found by name match |

## xws-payment-gate-payee

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payee</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1203</code> | <code>pub fn payee(&amp;self, asset: &amp;AcceptedAsset) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payee</code> | notes | method x_websearch::payment::PaymentGate::payee; no reference outside the defining file found by name match |

## xws-payment-gate-payer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payer</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1191</code> | <code>pub fn payer(&amp;self) -&gt; Option&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payer</code> | notes | method x_websearch::payment::PaymentGate::payer; spec/layerx-merge/spec.kvx task 5.3.2 settles paid HTTP by 0x101A draw/payExact and verifies receipts by ReceiptKeys; 6.5.51 one EVM nonce per draw |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-payment-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payment-required</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1250</code> | <code>pub fn payment_required(&amp;self, request: &amp;Request, payer: Option&lt;&amp;str&gt;) -&gt; Result&lt;PaymentRequired, Response&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-payment-required</code> | notes | method x_websearch::payment::PaymentGate::payment_required; no reference outside the defining file found by name match |

## xws-payment-gate-purpose

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-purpose</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1185</code> | <code>pub const fn purpose(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-purpose</code> | notes | method x_websearch::payment::PaymentGate::purpose; no reference outside the defining file found by name match |

## xws-payment-gate-receiver-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-receiver-did</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1180</code> | <code>pub fn receiver_did(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-receiver-did</code> | notes | method x_websearch::payment::PaymentGate::receiver_did; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.3.2 settles paid HTTP by 0x101A draw/payExact and verifies receipts by ReceiptKeys; 6.5.51 one EVM nonce per draw |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-recover-pending-deliveries

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-recover-pending-deliveries</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1556</code> | <code>pub fn recover_pending_deliveries(&amp;self) -&gt; io::Result&lt;()&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-recover-pending-deliveries</code> | notes | method x_websearch::payment::PaymentGate::recover_pending_deliveries; spec/layerx-merge/spec.kvx task 5.3.2 settles paid HTTP by 0x101A draw/payExact and verifies receipts by ReceiptKeys; 6.5.51 one EVM nonce per draw |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-settle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-settle</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1304</code> | <code>pub fn settle(&amp;self, request: &amp;Request, release: &amp;dyn Fn(&amp;Request) -&gt; Response) -&gt; Response</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-settle</code> | notes | method x_websearch::payment::PaymentGate::settle; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.3.2 settles paid HTTP by 0x101A draw/payExact and verifies receipts by ReceiptKeys; 6.5.51 one EVM nonce per draw |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-gate-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-gate-store</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1214</code> | <code>pub const fn store(&amp;self) -&gt; &amp;PaymentStore</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-gate-store</code> | notes | method x_websearch::payment::PaymentGate::store |

## xws-payment-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-record</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:645</code> | <code>pub struct PaymentRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-record</code> | notes | struct x_websearch::payment::PaymentRecord; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.3.2 do_4 adds tx_hash to PaymentRecord |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-required</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:60</code> | <code>pub const PAYMENT_REQUIRED: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-required</code> | notes | const x_websearch::payment::PAYMENT_REQUIRED |

## xws-payment-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:62</code> | <code>pub const PAYMENT_RESPONSE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-response</code> | notes | const x_websearch::payment::PAYMENT_RESPONSE |

## xws-payment-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-signature</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:61</code> | <code>pub const PAYMENT_SIGNATURE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-signature</code> | notes | const x_websearch::payment::PAYMENT_SIGNATURE |

## xws-payment-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-store</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:662</code> | <code>pub struct PaymentStore</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-store</code> | notes | struct x_websearch::payment::PaymentStore; no reference outside the defining file found by name match |

## xws-payment-store-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-store-load</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:933</code> | <code>pub fn load(&amp;self, key: &amp;[u8; 32]) -&gt; io::Result&lt;Option&lt;PaymentRecord&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-store-load</code> | notes | method x_websearch::payment::PaymentStore::load; no reference outside the defining file found by name match |
| <code>interface.11.rs-xws-payment-store-load</code> | overridden_by | override.J48.48 |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-store-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-store-open</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:914</code> | <code>pub fn open(data_dir: &amp;Path) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-store-open</code> | notes | method x_websearch::payment::PaymentStore::open; no reference outside the defining file found by name match |
| <code>interface.11.rs-xws-payment-store-open</code> | overridden_by | override.J48.47 |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-payment-store-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-store-release</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:1018</code> | <code>pub fn release(&amp;self, key: &amp;[u8; 32], receipt_digest: &amp;[u8; 32]) -&gt; io::Result&lt;bool&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-store-release</code> | notes | method x_websearch::payment::PaymentStore::release; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.3.2 settles paid HTTP by 0x101A draw/payExact and verifies receipts by ReceiptKeys; 6.5.51 one EVM nonce per draw |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-peer-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-peer-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/content.rs:18</code> | <code>pub const PEER_HEADER: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-peer-header</code> | notes | const x_websearch::content::PEER_HEADER |

## xws-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-pending</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:313</code> | <code>pub struct Pending</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-pending</code> | notes | struct x_websearch::watch::Pending |

## xws-preimage-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-preimage-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:24</code> | <code>pub const PREIMAGE_LENGTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-preimage-length</code> | notes | const x_websearch::attest::PREIMAGE_LENGTH |

## xws-program-attest-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-attest-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:302</code> | <code>pub enum ProgramAttestError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-attest-error</code> | notes | enum x_websearch::attest::ProgramAttestError |

## xws-program-attestation-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-attestation-path</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:216</code> | <code>pub const PROGRAM_ATTESTATION_PATH: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-attestation-path</code> | notes | const x_websearch::server::PROGRAM_ATTESTATION_PATH; re-exported at services/interop/crates/x-websearch/src/kernel.rs:40 |

## xws-program-discarded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-discarded</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:960</code> | <code>pub struct ProgramDiscarded</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-discarded</code> | notes | struct x_websearch::kernel::ProgramDiscarded; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1062</code> | <code>pub struct ProgramExchange</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange</code> | notes | struct x_websearch::kernel::ProgramExchange; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-accept

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-accept</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1258</code> | <code>pub fn accept(&amp;self, peer: &amp;str, key: ProgramKey, record: &amp;Value, set: &amp;AttestorSet) -&gt; Result&lt;Option&lt;[u8; 20]&gt;, Discard&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-accept</code> | notes | method x_websearch::kernel::ProgramExchange::accept; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-answer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-answer</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1196</code> | <code>pub fn answer(&amp;self, key: ProgramKey) -&gt; Option&lt;Answer&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-answer</code> | notes | method x_websearch::kernel::ProgramExchange::answer; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-collect

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-collect</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1336</code> | <code>pub fn collect(&amp;self, key: ProgramKey, set: &amp;AttestorSet) -&gt; usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-collect</code> | notes | method x_websearch::kernel::ProgramExchange::collect; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-discarded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-discarded</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1213</code> | <code>pub fn discarded(&amp;self) -&gt; Vec&lt;ProgramDiscarded&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-discarded</code> | notes | method x_websearch::kernel::ProgramExchange::discarded; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-forget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-forget</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1207</code> | <code>pub fn forget(&amp;self, key: ProgramKey)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-forget</code> | notes | method x_websearch::kernel::ProgramExchange::forget; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-handle</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1244</code> | <code>pub fn handle(&amp;self, key: ProgramKey) -&gt; Response</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-handle</code> | notes | method x_websearch::kernel::ProgramExchange::handle; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-log-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-log-path</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1174</code> | <code>pub fn log_path(&amp;self) -&gt; &amp;Path</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-log-path</code> | notes | method x_websearch::kernel::ProgramExchange::log_path; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-open</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1076</code> | <code>pub fn open(state_dir: &amp;Path, peers: &amp;[String]) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-open</code> | notes | method x_websearch::kernel::ProgramExchange::open; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-pending</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1202</code> | <code>pub fn pending(&amp;self) -&gt; Vec&lt;ProgramKey&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-pending</code> | notes | method x_websearch::kernel::ProgramExchange::pending; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-ready

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-ready</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1355</code> | <code>pub fn ready(&amp;self, key: ProgramKey, set: &amp;AttestorSet) -&gt; Option&lt;Ready&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-ready</code> | notes | method x_websearch::kernel::ProgramExchange::ready; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-exchange-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-exchange-record</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1185</code> | <code>pub fn record(&amp;self, answer: Answer)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-exchange-record</code> | notes | method x_websearch::kernel::ProgramExchange::record; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-key</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:956</code> | <code>pub type ProgramKey = ([u8; 32], u64)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-key</code> | notes | type alias x_websearch::kernel::ProgramKey; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-record</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:987</code> | <code>pub fn program_record(answer: &amp;Answer) -&gt; Value</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-record</code> | notes | fn x_websearch::kernel::program_record; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-request</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:184</code> | <code>pub struct ProgramRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-request</code> | notes | struct x_websearch::kernel::ProgramRequest; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-program-request-attestation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-request-attestation</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:228</code> | <code>pub fn attestation(&amp;self, network_id: u32, content_digest: [u8; 32], response: &amp;[u8], full_length: u32) -&gt; Attestation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-request-attestation</code> | notes | method x_websearch::kernel::ProgramRequest::attestation; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-protocol-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-protocol-version</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:70</code> | <code>pub const PROTOCOL_VERSION: u16</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-protocol-version</code> | notes | const x_websearch::payment::PROTOCOL_VERSION; spec/layerx-merge/spec.kvx task 11.5.122 do_2 drops the layerx_wire activity codec the draw protocol version comes from |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-purpose-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-purpose-hash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:179</code> | <code>pub fn purpose_hash(receiver_public_key: &amp;[u8; 32]) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-purpose-hash</code> | notes | fn x_websearch::payment::purpose_hash |

## xws-quantity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-quantity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:60</code> | <code>pub fn quantity(value: u128) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-quantity</code> | notes | fn x_websearch::watch::quantity |

## xws-query-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-query-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:76</code> | <code>pub enum QueryError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-query-error</code> | notes | enum x_websearch::server::QueryError |

## xws-query-terms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-query-terms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:107</code> | <code>pub fn query_terms(index: &amp;WebIndex, query: &amp;str) -&gt; Result&lt;Vec&lt;String&gt;, SearchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-query-terms</code> | notes | fn x_websearch::search::query_terms; no reference outside the defining file found by name match |

## xws-ready

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-ready</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:688</code> | <code>pub struct Ready</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-ready</code> | notes | struct x_websearch::attest::Ready |

## xws-ready-from-signatures

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-ready-from-signatures</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:701</code> | <code>pub fn from_signatures(answer: &amp;Answer, set: &amp;AttestorSet, signatures: &amp;BTreeMap&lt;[u8; 20], [u8; SIGNATURE_LENGTH]&gt;) -&gt; Option&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-ready-from-signatures</code> | notes | method x_websearch::attest::Ready::from_signatures |

## xws-receiver-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-receiver-did</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:149</code> | <code>pub fn receiver_did(public_key: &amp;[u8; 32]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-receiver-did</code> | notes | fn x_websearch::payment::receiver_did; spec/layerx-merge/spec.kvx task 11.5.122 do_4 retires kernel account naming (LAYERX-PAYER-DID, agent:{did}:main, :asset:{) for web payments; payer named by EVM address via 0x1004 |

## xws-receiver-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-receiver-key-file</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:10</code> | <code>pub const RECEIVER_KEY_FILE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-receiver-key-file</code> | notes | const x_websearch::keys::RECEIVER_KEY_FILE |

## xws-record-header-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-record-header-bytes</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:64</code> | <code>pub const RECORD_HEADER_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-record-header-bytes</code> | notes | const x_websearch::kernel::RECORD_HEADER_BYTES; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-recover-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-recover-signer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:177</code> | <code>pub fn recover_signer(digest: &amp;[u8; 32], signature: &amp;[u8; SIGNATURE_LENGTH]) -&gt; Result&lt;[u8; 20], SignatureError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-recover-signer</code> | notes | fn x_websearch::attest::recover_signer |

## xws-redirect-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-redirect-limit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:11</code> | <code>pub const REDIRECT_LIMIT: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-redirect-limit</code> | notes | const x_websearch::config::REDIRECT_LIMIT |

## xws-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-refusal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:30</code> | <code>pub enum Refusal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-refusal</code> | notes | enum x_websearch::config::Refusal; 94 reference lines in 1 files; callers lists the first reference in each file |

## xws-register

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-register</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1281</code> | <code>pub fn register(routes: &amp;mut RouteTable, exchange: &amp;Arc&lt;SignatureExchange&gt;) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-register</code> | notes | fn x_websearch::attest::register |

## xws-relay-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-relay-entry</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1467</code> | <code>pub struct RelayEntry</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-relay-entry</code> | notes | struct x_websearch::kernel::RelayEntry; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-relay-journal-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-relay-journal-file</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:87</code> | <code>pub const RELAY_JOURNAL_FILE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-relay-journal-file</code> | notes | const x_websearch::kernel::RELAY_JOURNAL_FILE; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-relay-journal-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-relay-journal-version</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:90</code> | <code>pub const RELAY_JOURNAL_VERSION: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-relay-journal-version</code> | notes | const x_websearch::kernel::RELAY_JOURNAL_VERSION; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-relay-lag

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-relay-lag</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:108</code> | <code>pub struct RelayLag</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-relay-lag</code> | notes | struct x_websearch::kernel::RelayLag; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-relay-lag-value

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-relay-lag-value</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:118</code> | <code>pub fn value(&amp;self) -&gt; Value</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-relay-lag-value</code> | notes | method x_websearch::kernel::RelayLag::value; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-remove-controls

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-remove-controls</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:149</code> | <code>pub fn remove_controls(text: &amp;str) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-remove-controls</code> | notes | fn x_websearch::canonical::remove_controls |

## xws-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:82</code> | <code>pub struct Request</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request</code> | notes | struct x_websearch::server::Request; 44 reference lines in 10 files; callers lists the first reference in each file; re-exported at services/interop/crates/x-websearch/src/lib.rs:31 |

## xws-request-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:95</code> | <code>pub fn header(&amp;self, name: &amp;str) -&gt; Option&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-header</code> | notes | method x_websearch::server::Request::header; no reference outside the defining file found by name match |

## xws-request-query-param

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-query-param</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:107</code> | <code>pub fn query_param(&amp;self, name: &amp;str) -&gt; Result&lt;Option&lt;String&gt;, QueryError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-query-param</code> | notes | method x_websearch::server::Request::query_param |

## xws-request-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:324</code> | <code>pub fn request_status(rpc: &amp;EvmRpc, request_id: u64) -&gt; Result&lt;u8, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-status</code> | notes | fn x_websearch::submit::request_status |

## xws-request-status-at-depth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-status-at-depth</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:333</code> | <code>pub fn request_status_at_depth(rpc: &amp;EvmRpc, request_id: u64, confirmations: u64) -&gt; Result&lt;u8, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-status-at-depth</code> | notes | fn x_websearch::submit::request_status_at_depth; chain calls: eth_call at :348 |

## xws-request-topic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-topic</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:43</code> | <code>pub const REQUEST_TOPIC: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-topic</code> | notes | const x_websearch::kernel::REQUEST_TOPIC; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-request-topics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-topics</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:46</code> | <code>pub const REQUEST_TOPICS: [&amp;[u8]; 1]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-topics</code> | notes | const x_websearch::kernel::REQUEST_TOPICS; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-request-view

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-view</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:281</code> | <code>pub struct RequestView</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-view</code> | notes | struct x_websearch::submit::RequestView; no reference outside the defining file found by name match |

## xws-request-view-words

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-view-words</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:270</code> | <code>pub const REQUEST_VIEW_WORDS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-view-words</code> | notes | const x_websearch::submit::REQUEST_VIEW_WORDS; no reference outside the defining file found by name match |

## xws-request-watcher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:299</code> | <code>pub struct RequestWatcher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher</code> | notes | struct x_websearch::watch::RequestWatcher |

## xws-request-watcher-canonical

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-canonical</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:574</code> | <code>pub fn canonical(&amp;mut self, request: &amp;WebRequest) -&gt; Result&lt;Canonical, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-canonical</code> | notes | method x_websearch::watch::RequestWatcher::canonical; chain calls: eth_getLogs at :581, eth_chainId at :592 |

## xws-request-watcher-confirmations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-confirmations</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:459</code> | <code>pub const fn confirmations(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-confirmations</code> | notes | method x_websearch::watch::RequestWatcher::confirmations |

## xws-request-watcher-eligible

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-eligible</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:461</code> | <code>pub fn eligible(&amp;self, request_id: u64) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-eligible</code> | notes | method x_websearch::watch::RequestWatcher::eligible |

## xws-request-watcher-fail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-fail</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:520</code> | <code>pub fn fail(&amp;mut self, request_id: u64, terminal: bool, reason: &amp;str) -&gt; Result&lt;bool, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-fail</code> | notes | method x_websearch::watch::RequestWatcher::fail |

## xws-request-watcher-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:501</code> | <code>pub fn journal(&amp;self) -&gt; Vec&lt;Pending&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-journal</code> | notes | method x_websearch::watch::RequestWatcher::journal |

## xws-request-watcher-last-head

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-last-head</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:495</code> | <code>pub const fn last_head(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-last-head</code> | notes | method x_websearch::watch::RequestWatcher::last_head |

## xws-request-watcher-next-block

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-next-block</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:489</code> | <code>pub const fn next_block(&amp;self) -&gt; Option&lt;u64&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-next-block</code> | notes | method x_websearch::watch::RequestWatcher::next_block |

## xws-request-watcher-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:383</code> | <code>pub fn open(rpc: EvmRpc, confirmations: u32, state_dir: &amp;Path, start: Option&lt;u64&gt;) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-open</code> | notes | method x_websearch::watch::RequestWatcher::open |

## xws-request-watcher-poll

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-poll</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:656</code> | <code>pub fn poll(&amp;mut self) -&gt; Result&lt;Vec&lt;WebRequest&gt;, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-poll</code> | notes | method x_websearch::watch::RequestWatcher::poll; chain calls: eth_chainId at :660, eth_getLogs at :673 |

## xws-request-watcher-reorged

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-reorged</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:478</code> | <code>pub fn reorged(&amp;mut self, request_id: u64) -&gt; Result&lt;(), EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-reorged</code> | notes | method x_websearch::watch::RequestWatcher::reorged |

## xws-request-watcher-retire

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-retire</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:549</code> | <code>pub fn retire(&amp;mut self, request_id: u64) -&gt; Result&lt;(), EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-retire</code> | notes | method x_websearch::watch::RequestWatcher::retire |

## xws-request-watcher-transition

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-transition</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:466</code> | <code>pub fn transition(&amp;mut self, request_id: u64, stage: WorkStage) -&gt; Result&lt;(), EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-transition</code> | notes | method x_websearch::watch::RequestWatcher::transition |

## xws-request-watcher-with-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-with-chain-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:445</code> | <code>pub fn with_chain_id(mut self, chain_id: u64) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-with-chain-id</code> | notes | method x_websearch::watch::RequestWatcher::with_chain_id |

## xws-request-watcher-work

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-request-watcher-work</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:507</code> | <code>pub fn work(&amp;self) -&gt; Vec&lt;WebRequest&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-request-watcher-work</code> | notes | method x_websearch::watch::RequestWatcher::work |

## xws-requested-event

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-requested-event</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:18</code> | <code>pub const REQUESTED_EVENT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-requested-event</code> | notes | const x_websearch::watch::REQUESTED_EVENT; no reference outside the defining file found by name match |

## xws-requested-topic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-requested-topic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:41</code> | <code>pub fn requested_topic() -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-requested-topic</code> | notes | fn x_websearch::watch::requested_topic |

## xws-resolve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-resolve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1306</code> | <code>pub fn resolve&lt;'a&gt;(document: &amp;'a Json, pointer: &amp;str) -&gt; Option&lt;&amp;'a Json&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-resolve</code> | notes | fn x_websearch::api::resolve; no reference outside the defining file found by name match |

## xws-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:163</code> | <code>pub struct Response</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-response</code> | notes | struct x_websearch::server::Response; 98 reference lines in 10 files; callers lists the first reference in each file; re-exported at services/interop/crates/x-websearch/src/lib.rs:31 |

## xws-response-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-response-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:188</code> | <code>pub fn error(status: u16, code: &amp;str) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-response-error</code> | notes | method x_websearch::server::Response::error; 56 reference lines in 8 files; callers lists the first reference in each file |

## xws-response-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-response-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:182</code> | <code>pub fn json(status: u16, body: Vec&lt;u8&gt;) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-response-json</code> | notes | method x_websearch::server::Response::json |

## xws-response-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-response-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:172</code> | <code>pub fn new(status: u16, content_type: &amp;str, body: Vec&lt;u8&gt;) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-response-new</code> | notes | method x_websearch::server::Response::new |

## xws-response-with-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-response-with-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:198</code> | <code>pub fn with_header(mut self, name: &amp;str, value: &amp;str) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-response-with-header</code> | notes | method x_websearch::server::Response::with_header |

## xws-resumed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-resumed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:608</code> | <code>pub type Resumed = Vec&lt;(u64, Result&lt;Outcome, SubmitError&gt;)&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-resumed</code> | notes | type alias x_websearch::submit::Resumed; no reference outside the defining file found by name match |

## xws-retry-delay-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-retry-delay-ms</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:98</code> | <code>pub fn retry_delay_ms(base_ms: u64, max_ms: u64, attempts: u32) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-retry-delay-ms</code> | notes | fn x_websearch::kernel::retry_delay_ms; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-robots

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:21</code> | <code>pub struct Robots</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots</code> | notes | struct x_websearch::robots::Robots |

## xws-robots-allow-all

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots-allow-all</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:35</code> | <code>pub const fn allow_all() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots-allow-all</code> | notes | method x_websearch::robots::Robots::allow_all |

## xws-robots-allows

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots-allows</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:112</code> | <code>pub fn allows(&amp;self, path: &amp;str) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots-allows</code> | notes | method x_websearch::robots::Robots::allows |

## xws-robots-cache

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots-cache</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:168</code> | <code>pub struct RobotsCache</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots-cache</code> | notes | struct x_websearch::robots::RobotsCache |

## xws-robots-cache-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots-cache-get</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:180</code> | <code>pub fn get(&amp;self, origin: &amp;str) -&gt; Option&lt;Robots&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots-cache-get</code> | notes | method x_websearch::robots::RobotsCache::get; no reference outside the defining file found by name match |

## xws-robots-cache-insert

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots-cache-insert</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:188</code> | <code>pub fn insert(&amp;self, origin: &amp;str, robots: Robots)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots-cache-insert</code> | notes | method x_websearch::robots::RobotsCache::insert; no reference outside the defining file found by name match |

## xws-robots-cache-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots-cache-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:174</code> | <code>pub fn new() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots-cache-new</code> | notes | method x_websearch::robots::RobotsCache::new |

## xws-robots-disallow-all

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots-disallow-all</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:41</code> | <code>pub fn disallow_all() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots-disallow-all</code> | notes | method x_websearch::robots::Robots::disallow_all |

## xws-robots-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-robots-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:53</code> | <code>pub fn parse(text: &amp;str) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-robots-parse</code> | notes | method x_websearch::robots::Robots::parse |

## xws-root-conformance-suite

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-root-conformance-suite</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/lib.rs:52</code> | <code>pub fn conformance_suite() -&gt; Result&lt;ConformanceSuite, AdapterError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-root-conformance-suite</code> | notes | fn x_websearch::conformance_suite |

## xws-route

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:61</code> | <code>pub enum Route</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route</code> | notes | enum x_websearch::server::Route; re-exported at services/interop/crates/x-websearch/src/lib.rs:31 |

## xws-route-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:219</code> | <code>pub enum RouteError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-error</code> | notes | enum x_websearch::server::RouteError |

## xws-route-is-paid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-is-paid</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:70</code> | <code>pub const fn is_paid(self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-is-paid</code> | notes | method x_websearch::server::Route::is_paid |

## xws-route-table

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:239</code> | <code>pub struct RouteTable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table</code> | notes | struct x_websearch::server::RouteTable; re-exported at services/interop/crates/x-websearch/src/lib.rs:31 |

## xws-route-table-dispatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table-dispatch</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/server.rs:364</code> | <code>pub fn dispatch(&amp;self, request: &amp;Request) -&gt; Response</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table-dispatch</code> | notes | method x_websearch::server::RouteTable::dispatch; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.0.8 do_5 (in_progress) exempts Route::Content from the serving-eligibility gate; at HEAD services/interop/crates/x-websearch/src/server.rs:379 still gates every non-health route |

Contract context <code>contract_context.11.x-websearch-server</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:250</code> | <code>pub fn new() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table-new</code> | notes | method x_websearch::server::RouteTable::new |

## xws-route-table-set

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:256</code> | <code>pub fn set(&amp;mut self, route: Route, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table-set</code> | notes | method x_websearch::server::RouteTable::set; no reference outside the defining file found by name match |

## xws-route-table-set-attestations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-attestations</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:314</code> | <code>pub fn set_attestations(&amp;mut self, handler: impl Fn(u64) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table-set-attestations</code> | notes | method x_websearch::server::RouteTable::set_attestations |

## xws-route-table-set-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-health</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:271</code> | <code>pub fn set_health(&amp;mut self, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table-set-health</code> | notes | method x_websearch::server::RouteTable::set_health |

## xws-route-table-set-program-attestations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-program-attestations</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/server.rs:330</code> | <code>pub fn set_program_attestations(&amp;mut self, handler: impl Fn([u8; 32], u64) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table-set-program-attestations</code> | notes | method x_websearch::server::RouteTable::set_program_attestations; spec/layerx-merge/spec.kvx task 5.2.3 backs the program handler with the attestor store, not ProgramExchange (services/interop/crates/x-websearch/src/kernel.rs:1011) |

Contract context <code>contract_context.11.x-websearch-server</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-readiness</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:282</code> | <code>pub fn set_readiness(&amp;mut self, handler: impl Fn(&amp;Request) -&gt; Response + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table-set-readiness</code> | notes | method x_websearch::server::RouteTable::set_readiness |

## xws-route-table-set-serving-eligibility

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-route-table-set-serving-eligibility</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:293</code> | <code>pub fn set_serving_eligibility(&amp;mut self, eligibility: impl Fn() -&gt; bool + Send + Sync + 'static) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-route-table-set-serving-eligibility</code> | notes | method x_websearch::server::RouteTable::set_serving_eligibility |

## xws-rpc-answer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-rpc-answer</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/x-websearch/src/payment.rs:353</code> | <code>pub enum RpcAnswer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-rpc-answer</code> | notes | enum x_websearch::payment::RpcAnswer; spec/layerx-merge/spec.kvx task 5.3.2 do_2/do_16 replace lx_sendActivity, lx_getActivityStatus, lx_getReceipt with eth_sendRawTransaction and pax_sequencerReceipt; 5.6.6 do_4 drops the authorization_file only the relay read |

Contract context <code>contract_context.11.x-websearch-payment</code>:

| Field | Frozen rule |
|---|---|
| interface | Paid routes settle on the 0x101A ledger precompile: metered draws and payExact transactions concluded from pax_sequencerReceipt. |
| storage | Payment journal under &lt;data_dir&gt;/payments. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Draw transactions are paid by the receiver account. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## xws-running-server

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-running-server</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:457</code> | <code>pub struct RunningServer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-running-server</code> | notes | struct x_websearch::server::RunningServer; re-exported at services/interop/crates/x-websearch/src/lib.rs:31 |

## xws-running-server-local-addr

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-running-server-local-addr</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:491</code> | <code>pub const fn local_addr(&amp;self) -&gt; SocketAddr</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-running-server-local-addr</code> | notes | method x_websearch::server::RunningServer::local_addr |

## xws-running-server-shutdown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-running-server-shutdown</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:510</code> | <code>pub fn shutdown(self) -&gt; io::Result&lt;()&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-running-server-shutdown</code> | notes | method x_websearch::server::RunningServer::shutdown; no reference outside the defining file found by name match |

## xws-running-server-stopper

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-running-server-stopper</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:498</code> | <code>pub fn stopper(&amp;self) -&gt; Stopper</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-running-server-stopper</code> | notes | method x_websearch::server::RunningServer::stopper; no reference outside the defining file found by name match |

## xws-running-server-wait

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-running-server-wait</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:519</code> | <code>pub fn wait(self) -&gt; io::Result&lt;()&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-running-server-wait</code> | notes | method x_websearch::server::RunningServer::wait; no reference outside the defining file found by name match |

## xws-scored-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-scored-result</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:97</code> | <code>pub struct ScoredResult</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-scored-result</code> | notes | struct x_websearch::search::ScoredResult; no reference outside the defining file found by name match |

## xws-seal-envelope-with

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-seal-envelope-with</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:361</code> | <code>pub fn seal_envelope_with(recipient: &amp;PublicKey, ephemeral: &amp;SigningKey, nonce: [u8; ENVELOPE_NONCE_LENGTH], origin: &amp;str, plaintext: &amp;[u8]) -&gt; Result&lt;Vec&lt;u8&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-seal-envelope-with</code> | notes | fn x_websearch::api::seal_envelope_with |

## xws-search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:172</code> | <code>pub fn search(index: &amp;WebIndex, query: &amp;str) -&gt; Result&lt;Vec&lt;ScoredResult&gt;, SearchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search</code> | notes | fn x_websearch::search::search; 95 reference lines in 14 files; callers lists the first reference in each file |

## xws-search-canonical-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-canonical-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:229</code> | <code>pub fn search_canonical_bytes(query: &amp;str, results: &amp;[SearchResult]) -&gt; Result&lt;Vec&lt;u8&gt;, CanonicalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-canonical-bytes</code> | notes | fn x_websearch::search::search_canonical_bytes |

## xws-search-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:35</code> | <code>pub enum SearchError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-error</code> | notes | enum x_websearch::search::SearchError |

## xws-search-error-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-error-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:44</code> | <code>pub const fn code(&amp;self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-error-code</code> | notes | method x_websearch::search::SearchError::code; no reference outside the defining file found by name match |

## xws-search-error-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-error-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:54</code> | <code>pub const fn status(&amp;self) -&gt; u16</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-error-status</code> | notes | method x_websearch::search::SearchError::status; no reference outside the defining file found by name match |

## xws-search-media-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-media-type</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:32</code> | <code>pub const SEARCH_MEDIA_TYPE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-media-type</code> | notes | const x_websearch::search::SEARCH_MEDIA_TYPE |

## xws-search-register

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-register</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:330</code> | <code>pub fn register(routes: &amp;mut RouteTable, gate: &amp;Arc&lt;PaymentGate&gt;, index: &amp;Arc&lt;WebIndex&gt;, store: &amp;Arc&lt;ContentStore&gt;) -&gt; Result&lt;(), RouteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-register</code> | notes | fn x_websearch::search::register |

## xws-search-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-result</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:88</code> | <code>pub struct SearchResult</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-result</code> | notes | struct x_websearch::search::SearchResult |

## xws-search-route

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-route</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:297</code> | <code>pub fn search_route(index: &amp;WebIndex, store: &amp;ContentStore, request: &amp;Request) -&gt; Response</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-route</code> | notes | fn x_websearch::search::search_route |

## xws-search-text

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-search-text</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:219</code> | <code>pub fn search_text(results: &amp;[SearchResult]) -&gt; Result&lt;String, serde_json::Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-search-text</code> | notes | fn x_websearch::search::search_text |

## xws-select-answer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-select-answer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/api.rs:1474</code> | <code>pub fn select_answer(body: &amp;[u8], pointers: &amp;[String]) -&gt; Result&lt;Vec&lt;u8&gt;, ApiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-select-answer</code> | notes | fn x_websearch::api::select_answer |

## xws-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:40</code> | <code>pub fn selector(signature: &amp;str) -&gt; [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-selector</code> | notes | fn x_websearch::submit::selector |

## xws-sequencer-trust

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-sequencer-trust</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/config.rs:150</code> | <code>pub struct SequencerTrust</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-sequencer-trust</code> | notes | struct x_websearch::config::SequencerTrust; spec/layerx-merge/spec.kvx task 5.4.3 replaces SequencerTrust (services/interop/crates/x-websearch/src/config.rs:150) with a ReceiptKeys list of key id, ed25519 key and height window; SequencerTrust still present at HEAD |

## xws-server

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-server</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:390</code> | <code>pub struct Server</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-server</code> | notes | struct x_websearch::server::Server; re-exported at services/interop/crates/x-websearch/src/lib.rs:31 |

## xws-server-bind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-server-bind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:399</code> | <code>pub fn bind(address: SocketAddr, limits: Limits, routes: RouteTable) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-server-bind</code> | notes | method x_websearch::server::Server::bind |

## xws-server-local-addr

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-server-local-addr</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:410</code> | <code>pub fn local_addr(&amp;self) -&gt; io::Result&lt;SocketAddr&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-server-local-addr</code> | notes | method x_websearch::server::Server::local_addr |

## xws-server-run

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-server-run</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:418</code> | <code>pub fn run(self) -&gt; io::Result&lt;()&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-server-run</code> | notes | method x_websearch::server::Server::run; no reference outside the defining file found by name match |

## xws-server-spawn

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-server-spawn</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:426</code> | <code>pub fn spawn(self) -&gt; io::Result&lt;RunningServer&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-server-spawn</code> | notes | method x_websearch::server::Server::spawn |

## xws-sign-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-sign-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:200</code> | <code>pub fn sign_digest(key: &amp;SigningKey, digest: &amp;[u8; 32]) -&gt; Result&lt;[u8; SIGNATURE_LENGTH], SignatureError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-sign-digest</code> | notes | fn x_websearch::attest::sign_digest |

## xws-sign-draw

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-sign-draw</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:273</code> | <code>pub fn sign_draw(receiver: &amp;SigningKey, draw: &amp;DrawRequest&lt;'_&gt;) -&gt; Result&lt;SignedDraw, DrawError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-sign-draw</code> | notes | fn x_websearch::payment::sign_draw; spec/layerx-merge/spec.kvx task 5.3.2 do_1 replaces sign_draw and receive_authorization with draw_calldata on the 0x101A draw method; 11.5.122 do_5 names 0x101B for the draw, which disagrees with 5.3.2 (0x101A) |

## xws-signature-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:136</code> | <code>pub enum SignatureError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-error</code> | notes | enum x_websearch::attest::SignatureError |

## xws-signature-exchange

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:953</code> | <code>pub struct SignatureExchange</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange</code> | notes | struct x_websearch::attest::SignatureExchange |

## xws-signature-exchange-accept

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-accept</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1165</code> | <code>pub fn accept(&amp;self, peer: &amp;str, request_id: u64, record: &amp;Value, set: &amp;AttestorSet) -&gt; Result&lt;Option&lt;[u8; 20]&gt;, Discard&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-accept</code> | notes | method x_websearch::attest::SignatureExchange::accept; no reference outside the defining file found by name match |

## xws-signature-exchange-answer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-answer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1084</code> | <code>pub fn answer(&amp;self, request_id: u64) -&gt; Option&lt;Answer&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-answer</code> | notes | method x_websearch::attest::SignatureExchange::answer |

## xws-signature-exchange-collect

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-collect</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1250</code> | <code>pub fn collect(&amp;self, request_id: u64, set: &amp;AttestorSet) -&gt; usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-collect</code> | notes | method x_websearch::attest::SignatureExchange::collect |

## xws-signature-exchange-discarded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-discarded</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1120</code> | <code>pub fn discarded(&amp;self) -&gt; Vec&lt;Discarded&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-discarded</code> | notes | method x_websearch::attest::SignatureExchange::discarded |

## xws-signature-exchange-expire

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-expire</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1104</code> | <code>pub fn expire(&amp;self, height: u64)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-expire</code> | notes | method x_websearch::attest::SignatureExchange::expire |

## xws-signature-exchange-forget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-forget</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1097</code> | <code>pub fn forget(&amp;self, request_id: u64)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-forget</code> | notes | method x_websearch::attest::SignatureExchange::forget |

## xws-signature-exchange-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-handle</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1151</code> | <code>pub fn handle(&amp;self, request_id: u64) -&gt; Response</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-handle</code> | notes | method x_websearch::attest::SignatureExchange::handle; no reference outside the defining file found by name match |

## xws-signature-exchange-log-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-log-path</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1056</code> | <code>pub fn log_path(&amp;self) -&gt; &amp;Path</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-log-path</code> | notes | method x_websearch::attest::SignatureExchange::log_path |

## xws-signature-exchange-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:971</code> | <code>pub fn open(state_dir: &amp;Path, peers: &amp;[String]) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-open</code> | notes | method x_websearch::attest::SignatureExchange::open |

## xws-signature-exchange-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-pending</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1092</code> | <code>pub fn pending(&amp;self) -&gt; Vec&lt;u64&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-pending</code> | notes | method x_websearch::attest::SignatureExchange::pending |

## xws-signature-exchange-ready

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-ready</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1270</code> | <code>pub fn ready(&amp;self, request_id: u64, set: &amp;AttestorSet) -&gt; Option&lt;Ready&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-ready</code> | notes | method x_websearch::attest::SignatureExchange::ready |

## xws-signature-exchange-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:1066</code> | <code>pub fn record(&amp;self, answer: Answer) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-exchange-record</code> | notes | method x_websearch::attest::SignatureExchange::record; no reference outside the defining file found by name match |

## xws-signature-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signature-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:36</code> | <code>pub const SIGNATURE_LENGTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signature-length</code> | notes | const x_websearch::attest::SIGNATURE_LENGTH |

## xws-signed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signed</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:947</code> | <code>pub struct Signed</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signed</code> | notes | struct x_websearch::kernel::Signed; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-signed-draw

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signed-draw</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:211</code> | <code>pub struct SignedDraw</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signed-draw</code> | notes | struct x_websearch::payment::SignedDraw; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.3.2 do_1 replaces sign_draw and receive_authorization with draw_calldata on the 0x101A draw method; 11.5.122 do_5 names 0x101B for the draw, which disagrees with 5.3.2 (0x101A) |

## xws-signed-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signed-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:121</code> | <code>pub struct SignedTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signed-transaction</code> | notes | struct x_websearch::submit::SignedTransaction; no reference outside the defining file found by name match |

## xws-signer-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-signer-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:168</code> | <code>pub fn signer_address(key: &amp;SigningKey) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-signer-address</code> | notes | fn x_websearch::attest::signer_address |

## xws-snippet-chars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-snippet-chars</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/search.rs:29</code> | <code>pub const SNIPPET_CHARS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-snippet-chars</code> | notes | const x_websearch::search::SNIPPET_CHARS; no reference outside the defining file found by name match |

## xws-source-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-source-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:331</code> | <code>pub struct SourceIdentity</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-source-identity</code> | notes | struct x_websearch::watch::SourceIdentity |

## xws-stage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stage</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1415</code> | <code>pub enum Stage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stage</code> | notes | enum x_websearch::kernel::Stage; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-stage-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stage-code</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1433</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stage-code</code> | notes | method x_websearch::kernel::Stage::code; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-stage-terminal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stage-terminal</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1459</code> | <code>pub const fn terminal(self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stage-terminal</code> | notes | method x_websearch::kernel::Stage::terminal; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-status-fulfilled

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-status-fulfilled</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:22</code> | <code>pub const STATUS_FULFILLED: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-status-fulfilled</code> | notes | const x_websearch::submit::STATUS_FULFILLED |

## xws-status-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-status-pending</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:21</code> | <code>pub const STATUS_PENDING: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-status-pending</code> | notes | const x_websearch::submit::STATUS_PENDING |

## xws-status-refunded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-status-refunded</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:23</code> | <code>pub const STATUS_REFUNDED: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-status-refunded</code> | notes | const x_websearch::submit::STATUS_REFUNDED; no reference outside the defining file found by name match |

## xws-step

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-step</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/kernel.rs:1379</code> | <code>pub enum Step</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-step</code> | notes | enum x_websearch::kernel::Step; spec/layerx-merge/spec.kvx task 5.5.6 deletes kernel.rs with KernelWatcher, KernelAttestor, ObservationSubmitter, ProgramExchange and KernelRelay; 11.5.121 moves origin-2 to eth_getLogs and eth_sendRawTransaction; file present at HEAD |

## xws-stop-signal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:79</code> | <code>pub struct StopSignal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stop-signal</code> | notes | struct x_websearch::crawl::StopSignal |

## xws-stop-signal-is-raised

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal-is-raised</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:97</code> | <code>pub fn is_raised(&amp;self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stop-signal-is-raised</code> | notes | method x_websearch::crawl::StopSignal::is_raised |

## xws-stop-signal-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:85</code> | <code>pub fn new() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stop-signal-new</code> | notes | method x_websearch::crawl::StopSignal::new |

## xws-stop-signal-raise

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal-raise</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:90</code> | <code>pub fn raise(&amp;self)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stop-signal-raise</code> | notes | method x_websearch::crawl::StopSignal::raise |

## xws-stop-signal-wait-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stop-signal-wait-timeout</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:104</code> | <code>pub fn wait_timeout(&amp;self, timeout: Duration) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stop-signal-wait-timeout</code> | notes | method x_websearch::crawl::StopSignal::wait_timeout |

## xws-stopper

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stopper</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:466</code> | <code>pub struct Stopper</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stopper</code> | notes | struct x_websearch::server::Stopper |

## xws-stopper-stop

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stopper-stop</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/server.rs:474</code> | <code>pub fn stop(&amp;self)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stopper-stop</code> | notes | method x_websearch::server::Stopper::stop; no reference outside the defining file found by name match |

## xws-stored-answer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stored-answer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:346</code> | <code>pub fn stored_answer(answer: &amp;[u8]) -&gt; Result&lt;(Vec&lt;u8&gt;, u32), AttestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stored-answer</code> | notes | fn x_websearch::attest::stored_answer; no reference outside the defining file found by name match |

## xws-stored-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-stored-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/attest.rs:331</code> | <code>pub fn stored_response(text: &amp;str) -&gt; Result&lt;(Vec&lt;u8&gt;, u32), AttestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-stored-response</code> | notes | fn x_websearch::attest::stored_response |

## xws-submit-attestor-set

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submit-attestor-set</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:366</code> | <code>pub fn attestor_set(rpc: &amp;EvmRpc) -&gt; Result&lt;AttestorSet, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submit-attestor-set</code> | notes | fn x_websearch::submit::attestor_set |

## xws-submit-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submit-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:207</code> | <code>pub enum SubmitError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submit-error</code> | notes | enum x_websearch::submit::SubmitError |

## xws-submitter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:611</code> | <code>pub struct Submitter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter</code> | notes | struct x_websearch::submit::Submitter |

## xws-submitter-abandon

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-abandon</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:745</code> | <code>pub fn abandon(&amp;self, request_id: u64) -&gt; Result&lt;(), SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-abandon</code> | notes | method x_websearch::submit::Submitter::abandon; chain calls: eth_sendRawTransaction at :806 via private fn broadcast |

## xws-submitter-acknowledge-closed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-acknowledge-closed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:734</code> | <code>pub fn acknowledge_closed(&amp;self, request_id: u64) -&gt; Result&lt;bool, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-acknowledge-closed</code> | notes | method x_websearch::submit::Submitter::acknowledge_closed |

## xws-submitter-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:649</code> | <code>pub const fn address(&amp;self) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-address</code> | notes | method x_websearch::submit::Submitter::address; no reference outside the defining file found by name match |

## xws-submitter-bind-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-bind-source</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:658</code> | <code>pub fn bind_source(&amp;self, request: &amp;crate::watch::WebRequest, source: &amp;crate::watch::SourceIdentity) -&gt; Result&lt;(), SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-bind-source</code> | notes | method x_websearch::submit::Submitter::bind_source; chain calls: eth_chainId at :708 via private fn canonical_source, eth_getLogs at :716 via private fn canonical_source |

## xws-submitter-confirm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-confirm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:948</code> | <code>pub fn confirm(&amp;self, request_id: u64) -&gt; Result&lt;Option&lt;Outcome&gt;, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-confirm</code> | notes | method x_websearch::submit::Submitter::confirm; chain calls: eth_getTransactionReceipt at :978, eth_getBlockByNumber at :1003 |

## xws-submitter-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:654</code> | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-journal</code> | notes | method x_websearch::submit::Submitter::journal |

## xws-submitter-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-key-file</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/keys.rs:9</code> | <code>pub const SUBMITTER_KEY_FILE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-key-file</code> | notes | const x_websearch::keys::SUBMITTER_KEY_FILE |

## xws-submitter-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:623</code> | <code>pub fn open(rpc: EvmRpc, key: SigningKey, chain_id: u64, journal_dir: &amp;Path) -&gt; io::Result&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-open</code> | notes | method x_websearch::submit::Submitter::open |

## xws-submitter-resume

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-resume</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:847</code> | <code>pub fn resume(&amp;self) -&gt; Result&lt;Resumed, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-resume</code> | notes | method x_websearch::submit::Submitter::resume; chain calls: eth_maxPriorityFeePerGas at :864 via private fn fees, eth_getBlockByNumber at :867 via private fn fees |

## xws-submitter-resume-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-resume-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:832</code> | <code>pub fn resume_request(&amp;self, request_id: u64) -&gt; Result&lt;Option&lt;Outcome&gt;, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-resume-request</code> | notes | method x_websearch::submit::Submitter::resume_request |

## xws-submitter-submit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-submit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:892</code> | <code>pub fn submit(&amp;self, ready: &amp;Ready) -&gt; Result&lt;Outcome, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-submit</code> | notes | method x_websearch::submit::Submitter::submit; no reference outside the defining file found by name match; chain calls: eth_getTransactionCount at :920 |

## xws-submitter-with-confirmations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-submitter-with-confirmations</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:642</code> | <code>pub const fn with_confirmations(mut self, confirmations: u64) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-submitter-with-confirmations</code> | notes | method x_websearch::submit::Submitter::with_confirmations |

## xws-system-clock

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-system-clock</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:92</code> | <code>pub fn system_clock() -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-system-clock</code> | notes | fn x_websearch::payment::system_clock |

## xws-title-of

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-title-of</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/crawl.rs:210</code> | <code>pub fn title_of(text: &amp;str) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-title-of</code> | notes | fn x_websearch::crawl::title_of |

## xws-total-timeout-limit-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-total-timeout-limit-ms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/config.rs:9</code> | <code>pub const TOTAL_TIMEOUT_LIMIT_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-total-timeout-limit-ms</code> | notes | const x_websearch::config::TOTAL_TIMEOUT_LIMIT_MS |

## xws-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:110</code> | <code>pub struct Transaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-transaction</code> | notes | struct x_websearch::submit::Transaction |

## xws-transaction-sign

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-transaction-sign</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/submit.rs:184</code> | <code>pub fn sign(&amp;self, key: &amp;SigningKey) -&gt; Result&lt;SignedTransaction, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-transaction-sign</code> | notes | method x_websearch::submit::Transaction::sign; no reference outside the defining file found by name match |

## xws-trim-line-ends

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-trim-line-ends</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:176</code> | <code>pub fn trim_line_ends(text: &amp;str) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-trim-line-ends</code> | notes | fn x_websearch::canonical::trim_line_ends |

## xws-unhex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-unhex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/payment.rs:114</code> | <code>pub fn unhex(text: &amp;str) -&gt; Option&lt;Vec&lt;u8&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-unhex</code> | notes | fn x_websearch::payment::unhex |

## xws-unhex0x

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-unhex0x</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:53</code> | <code>pub fn unhex0x(text: &amp;str) -&gt; Option&lt;Vec&lt;u8&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-unhex0x</code> | notes | fn x_websearch::watch::unhex0x |

## xws-unify-line-breaks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-unify-line-breaks</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/canonical.rs:143</code> | <code>pub fn unify_line_breaks(text: &amp;str) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-unify-line-breaks</code> | notes | fn x_websearch::canonical::unify_line_breaks |

## xws-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-url</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:113</code> | <code>pub struct Url</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-url</code> | notes | struct x_websearch::fetch::Url |

## xws-url-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-url-authority</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:240</code> | <code>pub fn authority(&amp;self) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-url-authority</code> | notes | method x_websearch::fetch::Url::authority |

## xws-url-bare-host

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-url-bare-host</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:256</code> | <code>pub fn bare_host(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-url-bare-host</code> | notes | method x_websearch::fetch::Url::bare_host |

## xws-url-join

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-url-join</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:265</code> | <code>pub fn join(&amp;self, reference: &amp;str) -&gt; Result&lt;Self, FetchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-url-join</code> | notes | method x_websearch::fetch::Url::join; 42 reference lines in 6 files; callers lists the first reference in each file |

## xws-url-origin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-url-origin</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:250</code> | <code>pub fn origin(&amp;self) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-url-origin</code> | notes | method x_websearch::fetch::Url::origin |

## xws-url-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-url-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:169</code> | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, FetchError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-url-parse</code> | notes | method x_websearch::fetch::Url::parse |

## xws-url-scheme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-url-scheme</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/fetch.rs:226</code> | <code>pub const fn scheme(&amp;self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-url-scheme</code> | notes | method x_websearch::fetch::Url::scheme; no reference outside the defining file found by name match |

## xws-user-agent-token

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-user-agent-token</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/robots.rs:6</code> | <code>pub const USER_AGENT_TOKEN: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-user-agent-token</code> | notes | const x_websearch::robots::USER_AGENT_TOKEN |

## xws-wallet-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-wallet-account</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/x-websearch/src/payment.rs:163</code> | <code>pub fn wallet_account(did: &amp;str, asset: &amp;AcceptedAsset) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-wallet-account</code> | notes | fn x_websearch::payment::wallet_account; spec/layerx-merge/spec.kvx task 11.5.122 do_4 retires kernel account naming (LAYERX-PAYER-DID, agent:{did}:main, :asset:{) for web payments; payer named by EVM address via 0x1004 |

## xws-web-index

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:92</code> | <code>pub struct WebIndex</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index</code> | notes | struct x_websearch::index::WebIndex |

## xws-web-index-commit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-commit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:166</code> | <code>pub fn commit(&amp;self) -&gt; Result&lt;(), IndexError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-commit</code> | notes | method x_websearch::index::WebIndex::commit |

## xws-web-index-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-directory</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:126</code> | <code>pub fn directory(&amp;self) -&gt; &amp;Path</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-directory</code> | notes | method x_websearch::index::WebIndex::directory |

## xws-web-index-documents-for

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-documents-for</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:192</code> | <code>pub fn documents_for(&amp;self, url: &amp;str) -&gt; Result&lt;usize, IndexError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-documents-for</code> | notes | method x_websearch::index::WebIndex::documents_for |

## xws-web-index-fields

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-fields</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:136</code> | <code>pub const fn fields(&amp;self) -&gt; Fields</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-fields</code> | notes | method x_websearch::index::WebIndex::fields |

## xws-web-index-index

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-index</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:131</code> | <code>pub const fn index(&amp;self) -&gt; &amp;Index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-index</code> | notes | method x_websearch::index::WebIndex::index |

## xws-web-index-num-docs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-num-docs</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:183</code> | <code>pub fn num_docs(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-num-docs</code> | notes | method x_websearch::index::WebIndex::num_docs |

## xws-web-index-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:106</code> | <code>pub fn open(data_dir: &amp;Path) -&gt; Result&lt;Self, IndexError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-open</code> | notes | method x_websearch::index::WebIndex::open |

## xws-web-index-put

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-put</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:145</code> | <code>pub fn put(&amp;self, url: &amp;str, title: &amp;str, body: &amp;str) -&gt; Result&lt;(), IndexError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-put</code> | notes | method x_websearch::index::WebIndex::put |

## xws-web-index-searcher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-index-searcher</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/index.rs:177</code> | <code>pub fn searcher(&amp;self) -&gt; Searcher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-index-searcher</code> | notes | method x_websearch::index::WebIndex::searcher |

## xws-web-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-web-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:187</code> | <code>pub struct WebRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-web-request</code> | notes | struct x_websearch::watch::WebRequest |

## xws-work-stage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-work-stage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:340</code> | <code>pub enum WorkStage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-work-stage</code> | notes | enum x_websearch::watch::WorkStage |

## xws-xweb-precompile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-xweb-precompile</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/x-websearch/src/watch.rs:13</code> | <code>pub const XWEB_PRECOMPILE: [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-xweb-precompile</code> | notes | const x_websearch::watch::XWEB_PRECOMPILE; address 0x1019 at HEAD; spec/layerx-merge/spec.kvx task 5.4.2 do_2 matches address 0x101D for origin-2 requests (precompile address disagrees); chain calls: 0x1019 at :14 |
| <code>interface.11.rs-xws-xweb-precompile</code> | overridden_by | override.J48.1 |

| Decision | Title | Value | Basis | Affects |
|---|---|---|---|---|
| <code>decision.1121</code> | The xweb precompile address is 0x1019 | x-websearch watches XWebRequested and calls fulfilProgram on 0x0000000000000000000000000000000000001019; the earlier plan's 0x101D is not used. | services/interop/crates/x-websearch/src/watch.rs:13 and precompiles/xweb/abi.json:14 at HEAD. | <code>interface.11.rs-xws-xweb-precompile</code><br><code>task.11.32.1</code><br><code>task.11.32.3</code> |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.886</code> | title | decision.1121 binds: The xweb precompile address is 0x1019 |
| <code>resolution.17.886</code> | applies_to | decision.1121<br>task.11.32.1<br>task.11.32.3 |
| <code>resolution.17.886</code> | source | decision.1121 in fragments/J48/decision.kvx; tasks from affects |
