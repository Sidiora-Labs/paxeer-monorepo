# Interop intent ingester contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cli-ingester

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-ingester</code> | <code>cli_command</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:209</code> | <code>layerx-intent-ingester (no arguments; env configured)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-ingester</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-ingester-interval-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-ingester-interval-ms</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:89</code> | <code>INGESTER_INTERVAL_MS: poll interval ms; default 1000</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-ingester-interval-ms</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-ingester-listen

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-ingester-listen</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:90</code> | <code>INGESTER_LISTEN: readiness listen address; default loopback port 8490</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-ingester-listen</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-ingester-max-range

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-ingester-max-range</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:88</code> | <code>INGESTER_MAX_RANGE: max eth_getLogs block range; default DEFAULT_MAX_RANGE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-ingester-max-range</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-ingester-retention-window

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-ingester-retention-window</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:87</code> | <code>INGESTER_RETENTION_WINDOW: retention window in blocks; default DEFAULT_RETENTION_WINDOW</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-ingester-retention-window</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-ingester-rpc-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-ingester-rpc-timeout-ms</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:76</code> | <code>INGESTER_RPC_TIMEOUT_MS: RPC timeout ms; default 10000</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-ingester-rpc-timeout-ms</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-ingester-rpc-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-ingester-rpc-url</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:75</code> | <code>INGESTER_RPC_URL: chain JSON-RPC URL (eth_blockNumber, eth_getLogs); required</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-ingester-rpc-url</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-ingester-start-block

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-ingester-start-block</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:80</code> | <code>INGESTER_START_BLOCK: first block to tail; optional</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-ingester-start-block</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-ingester-state-dir

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-ingester-state-dir</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:79</code> | <code>INGESTER_STATE_DIR: state directory; required</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-ingester-state-dir</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-ca-bundle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-ca-bundle</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:70</code> | <code>LAYERX_INGESTER_CA_BUNDLE: path to a PEM CA bundle for outbound TLS; optional</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-ca-bundle</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-fee-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-fee-limit</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:113</code> | <code>LAYERX_INGESTER_FEE_LIMIT: activity fee limit; default 0</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-fee-limit</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-gateway-api-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-gateway-api-key</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:95</code> | <code>LAYERX_INGESTER_GATEWAY_API_KEY: gateway API key; required</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-gateway-api-key</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-gateway-key-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-gateway-key-id</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:94</code> | <code>LAYERX_INGESTER_GATEWAY_KEY_ID: gateway API key id; required</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-gateway-key-id</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-gateway-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-gateway-url</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:93</code> | <code>LAYERX_INGESTER_GATEWAY_URL: gateway URL for lx_sendActivity submissions; required</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-gateway-url</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-market-map

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-market-map</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:101</code> | <code>LAYERX_INGESTER_MARKET_MAP: chain-to-kernel market id map</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-market-map</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-network-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-network-id</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:109</code> | <code>LAYERX_INGESTER_NETWORK_ID: kernel network id; default 0</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-network-id</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-protocol-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-protocol-version</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:105</code> | <code>LAYERX_INGESTER_PROTOCOL_VERSION: activity protocol version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-protocol-version</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## env-ingester-layerx-ingester-signer-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-signer-key-file</code> | <code>env_var</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:99</code> | <code>LAYERX_INGESTER_SIGNER_KEY_FILE: operator activity signer key file; required</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-ingester-layerx-ingester-signer-key-file</code> | notes | spec/layerx-merge/spec.kvx task 3.6.12, 3.6.14 and 11.7.35 retire the intent ingester (crate, image and rollout entry) |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## http-ingester-readyz

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-ingester-readyz</code> | <code>http_route</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/main.rs:43</code> | <code>GET /readyz</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-ingester-readyz</code> | notes | 200 or 503 with the readyz JSON body, any other target 404 {}; stale task 3.6.14 and 11.7.35 retire the intent ingester |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-config</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:556</code> | <code>pub struct Config</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-config</code> | notes | struct layerx_intent_ingester::Config; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-config-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-config-new</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:567</code> | <code>pub fn new(state_dir: PathBuf) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-config-new</code> | notes | method layerx_intent_ingester::Config::new; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-default-max-range

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-default-max-range</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:30</code> | <code>pub const DEFAULT_MAX_RANGE: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-default-max-range</code> | notes | const layerx_intent_ingester::DEFAULT_MAX_RANGE; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-default-retention-window

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-default-retention-window</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:27</code> | <code>pub const DEFAULT_RETENTION_WINDOW: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-default-retention-window</code> | notes | const layerx_intent_ingester::DEFAULT_RETENTION_WINDOW; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-hex</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:764</code> | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-hex</code> | notes | fn layerx_intent_ingester::hex; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-http-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-http-endpoint</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:89</code> | <code>pub struct HttpEndpoint</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-http-endpoint</code> | notes | struct layerx_intent_ingester::HttpEndpoint; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-http-endpoint-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-http-endpoint-new</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:102</code> | <code>pub fn new(url: &amp;str, timeout: Duration, roots: &amp;[native_tls::Certificate]) -&gt; Result&lt;Self, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-http-endpoint-new</code> | notes | method layerx_intent_ingester::HttpEndpoint::new; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-http-endpoint-post

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-http-endpoint-post</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:150</code> | <code>pub fn post(&amp;self, headers: &amp;str, body: &amp;str, limit: u64) -&gt; Result&lt;(String, Vec&lt;u8&gt;), IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-http-endpoint-post</code> | notes | method layerx_intent_ingester::HttpEndpoint::post; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-http-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-http-rpc</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:260</code> | <code>pub struct HttpRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-http-rpc</code> | notes | struct layerx_intent_ingester::HttpRpc; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-http-rpc-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-http-rpc-new</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:268</code> | <code>pub fn new(url: &amp;str, timeout: Duration, roots: &amp;[native_tls::Certificate]) -&gt; Result&lt;Self, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-http-rpc-new</code> | notes | method layerx_intent_ingester::HttpRpc::new; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-ingest-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-ingest-error</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:35</code> | <code>pub enum IngestError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-ingest-error</code> | notes | enum layerx_intent_ingester::IngestError; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-ingested-intent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-ingested-intent</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:320</code> | <code>pub struct IngestedIntent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-ingested-intent</code> | notes | struct layerx_intent_ingester::IngestedIntent; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-ingester

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-ingester</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:588</code> | <code>pub struct Ingester&lt;R&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-ingester</code> | notes | struct layerx_intent_ingester::Ingester; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-ingester-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-ingester-journal</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:636</code> | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-ingester-journal</code> | notes | method layerx_intent_ingester::Ingester::journal; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-ingester-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-ingester-open</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:606</code> | <code>pub fn open(rpc: R, config: &amp;Config) -&gt; Result&lt;Self, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-ingester-open</code> | notes | method layerx_intent_ingester::Ingester::open; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-ingester-poll

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-ingester-poll</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:657</code> | <code>pub fn poll(&amp;mut self) -&gt; Result&lt;Vec&lt;IngestedIntent&gt;, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-ingester-poll</code> | notes | method layerx_intent_ingester::Ingester::poll; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD; chain calls: eth_blockNumber at :658, eth_getLogs at :683 |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-ingester-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-ingester-status</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:641</code> | <code>pub fn status(&amp;self) -&gt; Status</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-ingester-status</code> | notes | method layerx_intent_ingester::Ingester::status; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-intent-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-intent-id</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:427</code> | <code>pub const fn intent_id(event: &amp;PrecompileEvent) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-intent-id</code> | notes | fn layerx_intent_ingester::intent_id; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-journal</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:446</code> | <code>pub struct Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-journal</code> | notes | struct layerx_intent_ingester::Journal; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-journal-append

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-journal-append</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:515</code> | <code>pub fn append(&amp;mut self, intents: Vec&lt;IngestedIntent&gt;) -&gt; Result&lt;Vec&lt;IngestedIntent&gt;, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-journal-append</code> | notes | method layerx_intent_ingester::Journal::append; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-journal-contains

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-journal-contains</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:495</code> | <code>pub fn contains(&amp;self, id: &amp;[u8; 32]) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-journal-contains</code> | notes | method layerx_intent_ingester::Journal::contains; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-journal-is-empty

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-journal-is-empty</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:505</code> | <code>pub const fn is_empty(&amp;self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-journal-is-empty</code> | notes | method layerx_intent_ingester::Journal::is_empty; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-journal-len

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-journal-len</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:500</code> | <code>pub const fn len(&amp;self) -&gt; usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-journal-len</code> | notes | method layerx_intent_ingester::Journal::len; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-journal-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-journal-open</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:458</code> | <code>pub fn open(path: &amp;Path) -&gt; Result&lt;Self, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-journal-open</code> | notes | method layerx_intent_ingester::Journal::open; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-journal-read-all

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-journal-read-all</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:483</code> | <code>pub fn read_all(path: &amp;Path) -&gt; Result&lt;Vec&lt;IngestedIntent&gt;, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-journal-read-all</code> | notes | method layerx_intent_ingester::Journal::read_all; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-json-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-json-rpc</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:80</code> | <code>pub trait JsonRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-json-rpc</code> | notes | trait layerx_intent_ingester::JsonRpc; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-load-roots

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-load-roots</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:246</code> | <code>pub fn load_roots(path: &amp;Path) -&gt; Result&lt;Vec&lt;native_tls::Certificate&gt;, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-load-roots</code> | notes | fn layerx_intent_ingester::load_roots; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-parse-quantity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-parse-quantity</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:754</code> | <code>pub fn parse_quantity(value: &amp;Value) -&gt; Result&lt;u64, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-parse-quantity</code> | notes | fn layerx_intent_ingester::parse_quantity; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-quantity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-quantity</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:745</code> | <code>pub fn quantity(value: u64) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-quantity</code> | notes | fn layerx_intent_ingester::quantity; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-readyz-body

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-readyz-body</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:805</code> | <code>pub fn readyz_body(status: &amp;Status, last_error: Option&lt;&amp;str&gt;) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-readyz-body</code> | notes | fn layerx_intent_ingester::readyz_body; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-rpc-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-rpc-result</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:296</code> | <code>pub fn rpc_result(reply: &amp;[u8]) -&gt; Result&lt;Value, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-rpc-result</code> | notes | fn layerx_intent_ingester::rpc_result; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-status</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:579</code> | <code>pub struct Status</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-status</code> | notes | struct layerx_intent_ingester::Status; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-tailed-precompiles

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-tailed-precompiles</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:23</code> | <code>pub const TAILED_PRECOMPILES: [[u8; 20]; 3] = [EXCHANGE_PRECOMPILE, BRIDGE_PRECOMPILE, LAUNCHPAD_PRECOMPILE]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-tailed-precompiles</code> | notes | const layerx_intent_ingester::TAILED_PRECOMPILES; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-unhex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-unhex</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:778</code> | <code>pub fn unhex(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, IngestError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-unhex</code> | notes | fn layerx_intent_ingester::unhex; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |

## ingester-window-alert

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ingester-window-alert</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-intent-ingester/src/lib.rs:549</code> | <code>pub struct WindowAlert</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ingester-window-alert</code> | notes | struct layerx_intent_ingester::WindowAlert; spec/layerx-merge/spec.kvx task 3.6.14 and 11.7.35 delete the crate (exchange intents copied into kernel perps/spot activities); 0x1015 depositMargin and per-caller margin replace it; crate still present at HEAD |

Contract context <code>contract_context.11.intent-ingester-retired</code>:

| Field | Frozen rule |
|---|---|
| interface | The intent ingester is removed. |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | No interop package names layerx-intent-ingester. |
| pause | none |
