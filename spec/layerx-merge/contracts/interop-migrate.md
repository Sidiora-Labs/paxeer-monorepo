# Interop migrate contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cfg-migrate-ethereum-config-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:87</code> | <code>migration ethereum config chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-chain-id</code> | notes | field EthereumConfig.chain_id; 311 reference lines in 102 files; callers lists the first reference in each file |

## cfg-migrate-ethereum-config-custody

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:93</code> | <code>migration ethereum config custody: EthereumCustodySchema (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody</code> | notes | field EthereumConfig.custody; 474 reference lines in 88 files; callers lists the first reference in each file |

## cfg-migrate-ethereum-config-custody-code-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-code-hash</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:90</code> | <code>migration ethereum config custody_code_hash: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-code-hash</code> | notes | field EthereumConfig.custody_code_hash |

## cfg-migrate-ethereum-config-custody-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-contract</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:89</code> | <code>migration ethereum config custody_contract: [u8; 20] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-contract</code> | notes | field EthereumConfig.custody_contract |

## cfg-migrate-ethereum-config-custody-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-identity</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:91</code> | <code>migration ethereum config custody_identity: EthereumContractIdentity (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-identity</code> | notes | field EthereumConfig.custody_identity |

## cfg-migrate-ethereum-config-finality-tag

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-finality-tag</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:94</code> | <code>migration ethereum config finality_tag: EthereumFinalityTag (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-finality-tag</code> | notes | field EthereumConfig.finality_tag |

## cfg-migrate-ethereum-config-genesis-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-genesis-hash</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:88</code> | <code>migration ethereum config genesis_hash: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-genesis-hash</code> | notes | field EthereumConfig.genesis_hash |

## cfg-migrate-ethereum-config-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-journal</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:100</code> | <code>migration ethereum config journal: JournalConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-journal</code> | notes | field EthereumConfig.journal; 451 reference lines in 44 files; callers lists the first reference in each file |

## cfg-migrate-ethereum-config-maximum-ancestry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-ancestry</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:96</code> | <code>migration ethereum config maximum_ancestry: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-ancestry</code> | notes | field EthereumConfig.maximum_ancestry |

## cfg-migrate-ethereum-config-maximum-history-blocks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-history-blocks</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:97</code> | <code>migration ethereum config maximum_history_blocks: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-history-blocks</code> | notes | field EthereumConfig.maximum_history_blocks |

## cfg-migrate-ethereum-config-maximum-ownership-ttl-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-ownership-ttl-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:98</code> | <code>migration ethereum config maximum_ownership_ttl_seconds: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-ownership-ttl-seconds</code> | notes | field EthereumConfig.maximum_ownership_ttl_seconds |

## cfg-migrate-ethereum-config-minimum-confirmations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-minimum-confirmations</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:95</code> | <code>migration ethereum config minimum_confirmations: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-minimum-confirmations</code> | notes | field EthereumConfig.minimum_confirmations |

## cfg-migrate-ethereum-config-native-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-native-asset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:92</code> | <code>migration ethereum config native_asset: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-native-asset</code> | notes | field EthereumConfig.native_asset |

## cfg-migrate-ethereum-config-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:99</code> | <code>migration ethereum config rpc: RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-rpc</code> | notes | field EthereumConfig.rpc; 1207 reference lines in 211 files; callers lists the first reference in each file |

## cfg-migrate-ethereum-contract-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-contract-identity</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:64</code> | <code>migration ethereum contract identity: one of "Immutable" &#124; "Proxy"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-contract-identity</code> | notes | serde enum EthereumContractIdentity |

## cfg-migrate-ethereum-custody-schema-custody-reference

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-custody-reference</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:48</code> | <code>migration ethereum custody_schema custody_reference: EthereumWord (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-custody-reference</code> | notes | field EthereumCustodySchema.custody_reference |

## cfg-migrate-ethereum-custody-schema-destination

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-destination</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:51</code> | <code>migration ethereum custody_schema destination: EthereumWord (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-destination</code> | notes | field EthereumCustodySchema.destination; 568 reference lines in 110 files; callers lists the first reference in each file |

## cfg-migrate-ethereum-custody-schema-event-topic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-event-topic</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:44</code> | <code>migration ethereum custody_schema event_topic: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-event-topic</code> | notes | field EthereumCustodySchema.event_topic |

## cfg-migrate-ethereum-custody-schema-function-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-function-selector</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:43</code> | <code>migration ethereum custody_schema function_selector: [u8; 4] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-function-selector</code> | notes | field EthereumCustodySchema.function_selector |

## cfg-migrate-ethereum-custody-schema-layerx-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-layerx-amount</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:50</code> | <code>migration ethereum custody_schema layerx_amount: EthereumWord (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-layerx-amount</code> | notes | field EthereumCustodySchema.layerx_amount |

## cfg-migrate-ethereum-custody-schema-layerx-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-layerx-asset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:49</code> | <code>migration ethereum custody_schema layerx_asset: EthereumWord (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-layerx-asset</code> | notes | field EthereumCustodySchema.layerx_asset |

## cfg-migrate-ethereum-custody-schema-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:45</code> | <code>migration ethereum custody_schema source: EthereumWord (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source</code> | notes | field EthereumCustodySchema.source; 6200 reference lines in 573 files; callers lists the first reference in each file |

## cfg-migrate-ethereum-custody-schema-source-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source-amount</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:47</code> | <code>migration ethereum custody_schema source_amount: EthereumWord (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source-amount</code> | notes | field EthereumCustodySchema.source_amount |

## cfg-migrate-ethereum-custody-schema-source-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source-asset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:46</code> | <code>migration ethereum custody_schema source_asset: EthereumWord (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source-asset</code> | notes | field EthereumCustodySchema.source_asset |

## cfg-migrate-ethereum-finality-tag

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-finality-tag</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:56</code> | <code>migration ethereum finality: one of "Safe" &#124; "Finalized"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-finality-tag</code> | notes | serde enum EthereumFinalityTag |

## cfg-migrate-ethereum-word

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-word</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:34</code> | <code>migration ethereum word: one of "Topic" &#124; "Data"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-word</code> | notes | serde enum EthereumWord |

## cfg-migrate-gateway-client-config-ca-certificate-der

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-ca-certificate-der</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:26</code> | <code>migrate gateway-config ca_certificate_der: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-ca-certificate-der</code> | notes | field GatewayClientConfig.ca_certificate_der; struct denies unknown fields |

## cfg-migrate-gateway-client-config-connect-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-connect-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:28</code> | <code>migrate gateway-config connect_timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-connect-timeout-ms</code> | notes | field GatewayClientConfig.connect_timeout_ms; struct denies unknown fields |

## cfg-migrate-gateway-client-config-customer-authorization-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-customer-authorization-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:27</code> | <code>migrate gateway-config customer_authorization_file: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-customer-authorization-file</code> | notes | field GatewayClientConfig.customer_authorization_file; struct denies unknown fields |

## cfg-migrate-gateway-client-config-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:25</code> | <code>migrate gateway-config endpoint: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-endpoint</code> | notes | field GatewayClientConfig.endpoint; struct denies unknown fields; 985 reference lines in 175 files; callers lists the first reference in each file |

## cfg-migrate-gateway-client-config-maximum-response-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-maximum-response-bytes</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:30</code> | <code>migrate gateway-config maximum_response_bytes: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-maximum-response-bytes</code> | notes | field GatewayClientConfig.maximum_response_bytes; struct denies unknown fields |

## cfg-migrate-gateway-client-config-request-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-request-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:29</code> | <code>migrate gateway-config request_timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-request-timeout-ms</code> | notes | field GatewayClientConfig.request_timeout_ms; struct denies unknown fields |

## cfg-migrate-journal-config-authentication-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-authentication-key-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:35</code> | <code>migration journal config authentication_key_file: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-authentication-key-file</code> | notes | field JournalConfig.authentication_key_file |

## cfg-migrate-journal-config-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-directory</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:34</code> | <code>migration journal config directory: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-directory</code> | notes | field JournalConfig.directory; 3260 reference lines in 480 files; callers lists the first reference in each file |

## cfg-migrate-journal-config-namespace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-namespace</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:36</code> | <code>migration journal config namespace: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-namespace</code> | notes | field JournalConfig.namespace; 702 reference lines in 152 files; callers lists the first reference in each file |

## cfg-migrate-journal-config-rollback-anchor-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-rollback-anchor-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:37</code> | <code>migration journal config rollback_anchor_id: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-rollback-anchor-id</code> | notes | field JournalConfig.rollback_anchor_id |

## cfg-migrate-journal-config-rollback-anchor-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-rollback-anchor-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:38</code> | <code>migration journal config rollback_anchor_rpc: RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-rollback-anchor-rpc</code> | notes | field JournalConfig.rollback_anchor_rpc |

## cfg-migrate-paxeer-binding-config-v2-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:19</code> | <code>migration paxeer_binding config chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-chain-id</code> | notes | field PaxeerBindingConfigV2.chain_id; struct denies unknown fields; 311 reference lines in 102 files; callers lists the first reference in each file |

## cfg-migrate-paxeer-binding-config-v2-genesis-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-genesis-hash</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:20</code> | <code>migration paxeer_binding config genesis_hash: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-genesis-hash</code> | notes | field PaxeerBindingConfigV2.genesis_hash; struct denies unknown fields |

## cfg-migrate-paxeer-binding-config-v2-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:21</code> | <code>migration paxeer_binding config rpc: RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-rpc</code> | notes | field PaxeerBindingConfigV2.rpc; struct denies unknown fields; 1207 reference lines in 211 files; callers lists the first reference in each file |

## cfg-migrate-rpc-endpoint-config-bearer-token-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-bearer-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:31</code> | <code>migration rpc endpoint bearer_token_file: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-bearer-token-file</code> | notes | field RpcEndpointConfig.bearer_token_file; struct denies unknown fields |

## cfg-migrate-rpc-endpoint-config-ca-certificate-der

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-ca-certificate-der</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:30</code> | <code>migration rpc endpoint ca_certificate_der: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-ca-certificate-der</code> | notes | field RpcEndpointConfig.ca_certificate_der; struct denies unknown fields |

## cfg-migrate-rpc-endpoint-config-independent-backend

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-independent-backend</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:34</code> | <code>migration rpc endpoint independent_backend: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-independent-backend</code> | notes | field RpcEndpointConfig.independent_backend; struct denies unknown fields |

## cfg-migrate-rpc-endpoint-config-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-url</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:29</code> | <code>migration rpc endpoint url: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-url</code> | notes | field RpcEndpointConfig.url; struct denies unknown fields; 1052 reference lines in 163 files; callers lists the first reference in each file |

## cfg-migrate-rpc-quorum-config-connect-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-connect-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:43</code> | <code>migration rpc quorum connect_timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-connect-timeout-ms</code> | notes | field RpcQuorumConfig.connect_timeout_ms; struct denies unknown fields |

## cfg-migrate-rpc-quorum-config-endpoints

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-endpoints</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:41</code> | <code>migration rpc quorum endpoints: Vec&lt;RpcEndpointConfig&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-endpoints</code> | notes | field RpcQuorumConfig.endpoints; struct denies unknown fields; 333 reference lines in 73 files; callers lists the first reference in each file |

## cfg-migrate-rpc-quorum-config-maximum-response-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-maximum-response-bytes</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:45</code> | <code>migration rpc quorum maximum_response_bytes: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-maximum-response-bytes</code> | notes | field RpcQuorumConfig.maximum_response_bytes; struct denies unknown fields |

## cfg-migrate-rpc-quorum-config-quorum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-quorum</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:42</code> | <code>migration rpc quorum quorum: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-quorum</code> | notes | field RpcQuorumConfig.quorum; struct denies unknown fields; 149 reference lines in 44 files; callers lists the first reference in each file |

## cfg-migrate-rpc-quorum-config-request-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-request-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:44</code> | <code>migration rpc quorum request_timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-request-timeout-ms</code> | notes | field RpcQuorumConfig.request_timeout_ms; struct denies unknown fields |

## cfg-migrate-solana-amount-endian

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-amount-endian</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:32</code> | <code>migration solana amount endian: one of "Big" &#124; "Little"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-amount-endian</code> | notes | serde enum SolanaAmountEndian |

## cfg-migrate-solana-config-custody

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:69</code> | <code>migration solana config custody: SolanaInstructionSchema (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody</code> | notes | field SolanaConfig.custody; 474 reference lines in 88 files; callers lists the first reference in each file |

## cfg-migrate-solana-config-custody-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:64</code> | <code>migration solana config custody_account: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-account</code> | notes | field SolanaConfig.custody_account |

## cfg-migrate-solana-config-custody-account-owner

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-account-owner</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:65</code> | <code>migration solana config custody_account_owner: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-account-owner</code> | notes | field SolanaConfig.custody_account_owner |

## cfg-migrate-solana-config-custody-program

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:59</code> | <code>migration solana config custody_program: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program</code> | notes | field SolanaConfig.custody_program |

## cfg-migrate-solana-config-custody-program-data-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:61</code> | <code>migration solana config custody_program_data_account: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-account</code> | notes | field SolanaConfig.custody_program_data_account |

## cfg-migrate-solana-config-custody-program-data-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-hash</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:63</code> | <code>migration solana config custody_program_data_hash: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-hash</code> | notes | field SolanaConfig.custody_program_data_hash |

## cfg-migrate-solana-config-custody-program-data-reference-offset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-reference-offset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:62</code> | <code>migration solana config custody_program_data_reference_offset: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-reference-offset</code> | notes | field SolanaConfig.custody_program_data_reference_offset |

## cfg-migrate-solana-config-custody-program-owner

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-owner</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:60</code> | <code>migration solana config custody_program_owner: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-owner</code> | notes | field SolanaConfig.custody_program_owner |

## cfg-migrate-solana-config-custody-token-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-token-authority</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:66</code> | <code>migration solana config custody_token_authority: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-token-authority</code> | notes | field SolanaConfig.custody_token_authority |

## cfg-migrate-solana-config-genesis-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-genesis-hash</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:58</code> | <code>migration solana config genesis_hash: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-genesis-hash</code> | notes | field SolanaConfig.genesis_hash |

## cfg-migrate-solana-config-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-journal</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:75</code> | <code>migration solana config journal: JournalConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-journal</code> | notes | field SolanaConfig.journal; 451 reference lines in 44 files; callers lists the first reference in each file |

## cfg-migrate-solana-config-maximum-ancestry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-ancestry</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:71</code> | <code>migration solana config maximum_ancestry: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-ancestry</code> | notes | field SolanaConfig.maximum_ancestry |

## cfg-migrate-solana-config-maximum-history-slots

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-history-slots</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:72</code> | <code>migration solana config maximum_history_slots: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-history-slots</code> | notes | field SolanaConfig.maximum_history_slots |

## cfg-migrate-solana-config-maximum-ownership-ttl-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-ownership-ttl-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:73</code> | <code>migration solana config maximum_ownership_ttl_seconds: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-ownership-ttl-seconds</code> | notes | field SolanaConfig.maximum_ownership_ttl_seconds |

## cfg-migrate-solana-config-minimum-rooted-slots

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-minimum-rooted-slots</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:70</code> | <code>migration solana config minimum_rooted_slots: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-minimum-rooted-slots</code> | notes | field SolanaConfig.minimum_rooted_slots |

## cfg-migrate-solana-config-native-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-native-asset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:68</code> | <code>migration solana config native_asset: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-native-asset</code> | notes | field SolanaConfig.native_asset |

## cfg-migrate-solana-config-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:74</code> | <code>migration solana config rpc: RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-rpc</code> | notes | field SolanaConfig.rpc; 1207 reference lines in 211 files; callers lists the first reference in each file |

## cfg-migrate-solana-config-source-asset-account-owner

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-source-asset-account-owner</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:67</code> | <code>migration solana config source_asset_account_owner: [u8; 32] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-source-asset-account-owner</code> | notes | field SolanaConfig.source_asset_account_owner |

## cfg-migrate-solana-instruction-schema-amount-endian

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-amount-endian</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:51</code> | <code>migration solana instruction schema amount_endian: SolanaAmountEndian (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-amount-endian</code> | notes | field SolanaInstructionSchema.amount_endian |

## cfg-migrate-solana-instruction-schema-custody-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-custody-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:43</code> | <code>migration solana instruction schema custody_account: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-custody-account</code> | notes | field SolanaInstructionSchema.custody_account |

## cfg-migrate-solana-instruction-schema-custody-reference-offset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-custody-reference-offset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:47</code> | <code>migration solana instruction schema custody_reference_offset: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-custody-reference-offset</code> | notes | field SolanaInstructionSchema.custody_reference_offset |

## cfg-migrate-solana-instruction-schema-destination-offset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-destination-offset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:50</code> | <code>migration solana instruction schema destination_offset: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-destination-offset</code> | notes | field SolanaInstructionSchema.destination_offset |

## cfg-migrate-solana-instruction-schema-discriminator

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-discriminator</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:41</code> | <code>migration solana instruction schema discriminator: Vec&lt;u8&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-discriminator</code> | notes | field SolanaInstructionSchema.discriminator |

## cfg-migrate-solana-instruction-schema-layerx-amount-offset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-layerx-amount-offset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:49</code> | <code>migration solana instruction schema layerx_amount_offset: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-layerx-amount-offset</code> | notes | field SolanaInstructionSchema.layerx_amount_offset |

## cfg-migrate-solana-instruction-schema-layerx-asset-offset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-layerx-asset-offset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:48</code> | <code>migration solana instruction schema layerx_asset_offset: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-layerx-asset-offset</code> | notes | field SolanaInstructionSchema.layerx_asset_offset |

## cfg-migrate-solana-instruction-schema-source-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:42</code> | <code>migration solana instruction schema source_account: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-account</code> | notes | field SolanaInstructionSchema.source_account |

## cfg-migrate-solana-instruction-schema-source-amount-offset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-amount-offset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:46</code> | <code>migration solana instruction schema source_amount_offset: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-amount-offset</code> | notes | field SolanaInstructionSchema.source_amount_offset |

## cfg-migrate-solana-instruction-schema-source-asset-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-asset-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:44</code> | <code>migration solana instruction schema source_asset_account: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-asset-account</code> | notes | field SolanaInstructionSchema.source_asset_account |

## cfg-migrate-solana-instruction-schema-source-asset-offset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-asset-offset</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:45</code> | <code>migration solana instruction schema source_asset_offset: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-asset-offset</code> | notes | field SolanaInstructionSchema.source_asset_offset |

## cli-migrate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-migrate</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/main.rs:113</code> | <code>layerx-migrate &lt;migrate-asset&#124;confirm-mapping&#124;verify-ownership&#124;verify-asset&#124;import-history&#124;read-history&gt; [--chain --config --evidence --store-config --principal --trace --cursor --limit --binding-config --mapping-config --identity --gateway-config --order-digest --idempotency]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-migrate</code> | notes | each subcommand allows a fixed flag subset (services/interop/crates/layerx-migrate/src/main.rs:143-175); duplicate or unknown flags refuse with MigrationError::Configuration; prints one JSON value |

## env-migrate-layerx-migration-secret-dir

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-migrate-layerx-migration-secret-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/main.rs:70</code> | <code>LAYERX_MIGRATION_SECRET_DIR: directory holding migration CLI secrets; required</code> |

## err-migrate-address-chain-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-address-chain-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1231</code> | <code>MigrationError::AddressChainMismatch.code() = "address_chain_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-address-chain-mismatch</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-checkpoint-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-checkpoint-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1248</code> | <code>MigrationError::CheckpointConflict.code() = "checkpoint_conflict"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-checkpoint-conflict</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-checkpoint-integrity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-checkpoint-integrity</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1247</code> | <code>MigrationError::CheckpointIntegrity.code() = "checkpoint_integrity"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-checkpoint-integrity</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-configuration-invalid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-configuration-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1228</code> | <code>MigrationError::Configuration.code() = "configuration_invalid"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-configuration-invalid</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-custody-event-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-custody-event-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1244</code> | <code>MigrationError::CustodyEventMismatch.code() = "custody_event_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-custody-event-mismatch</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-custody-program-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-custody-program-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1245</code> | <code>MigrationError::CustodyProgramMismatch.code() = "custody_program_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-custody-program-mismatch</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-evidence-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-evidence-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1234</code> | <code>MigrationError::EvidenceMismatch.code() = "evidence_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-evidence-mismatch</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-finality-window-exceeded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-finality-window-exceeded</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1238</code> | <code>MigrationError::FinalityWindowExceeded.code() = "finality_window_exceeded"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-finality-window-exceeded</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-gateway-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-gateway-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1254</code> | <code>MigrationError::Gateway.code() = "gateway_refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-gateway-refused</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-gateway-submission-unknown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-gateway-submission-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1240</code> | <code>MigrationError::GatewaySubmissionUnknown.code() = "gateway_submission_unknown"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-gateway-submission-unknown</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-invalid-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-invalid-address</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1230</code> | <code>MigrationError::InvalidAddress.code() = "invalid_address"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-invalid-address</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-invalid-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-invalid-evidence</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1233</code> | <code>MigrationError::InvalidEvidence.code() = "invalid_evidence"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-invalid-evidence</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-invalid-history

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-invalid-history</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1249</code> | <code>MigrationError::InvalidHistory.code() = "invalid_history"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-invalid-history</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-invalid-network

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-invalid-network</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1229</code> | <code>MigrationError::InvalidNetwork.code() = "invalid_network"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-invalid-network</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-invalid-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-invalid-transaction</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1232</code> | <code>MigrationError::InvalidTransaction.code() = "invalid_transaction"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-invalid-transaction</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-ownership-signature-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-ownership-signature-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1246</code> | <code>MigrationError::OwnershipSignatureMismatch.code() = "ownership_signature_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-ownership-signature-mismatch</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-plane-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-plane-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1251</code> | <code>MigrationError::PlaneRefused.code() = "plane_refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-plane-refused</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-receipt-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-receipt-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1253</code> | <code>MigrationError::ReceiptMismatch.code() = "receipt_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-receipt-mismatch</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-receipt-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-receipt-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1252</code> | <code>MigrationError::ReceiptRequired.code() = "receipt_required"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-receipt-required</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-rpc-divergence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-rpc-divergence</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1242</code> | <code>MigrationError::RpcDivergence.code() = "rpc_divergence"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-rpc-divergence</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-rpc-rate-limited

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-rpc-rate-limited</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1241</code> | <code>MigrationError::RpcRateLimited.code() = "rpc_rate_limited"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-rpc-rate-limited</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-rpc-response-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-rpc-response-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1243</code> | <code>MigrationError::RpcResponseMismatch.code() = "rpc_response_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-rpc-response-mismatch</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-rpc-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-rpc-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1239</code> | <code>MigrationError::RpcUnavailable.code() = "rpc_unavailable"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-rpc-unavailable</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-source-displaced

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-source-displaced</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1237</code> | <code>MigrationError::SourceDisplaced.code() = "source_displaced"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-source-displaced</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-source-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-source-pending</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1235</code> | <code>MigrationError::SourcePending.code() = "source_pending"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-source-pending</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-source-reverted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-source-reverted</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1236</code> | <code>MigrationError::SourceReverted.code() = "source_reverted"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-source-reverted</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## err-migrate-storage-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-migrate-storage-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1250</code> | <code>MigrationError::StorageRefused.code() = "storage_refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-migrate-storage-refused</code> | notes | wire code returned by MigrationError::code (services/interop/crates/layerx-migrate/src/lib.rs:1226) |

## fmt-migrate-account-mapping-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-account-mapping-v2</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:71</code> | <code>AccountMappingV2 { version: &amp;'static str, checkpoint: MappingCheckpointV2, provenance: &amp;'static str, layerx_receipt: bool }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-account-mapping-v2</code> | notes | migration journal/wire JSON via serde Clone, Debug, Serialize |

## fmt-migrate-chain-checkpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-chain-checkpoint</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:42</code> | <code>ChainCheckpoint { height: u64, hash: [u8; 32], parent_hash: [u8; 32], previous_height: u64, previous_hash: [u8; 32] }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-chain-checkpoint</code> | notes | migration journal/wire JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize |

## fmt-migrate-claim-checkpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-claim-checkpoint</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:51</code> | <code>ClaimCheckpoint { height: u64, block_hash: [u8; 32], evidence_digest: [u8; 32] }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-claim-checkpoint</code> | notes | migration journal/wire JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize |

## fmt-migrate-funded-migration-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-funded-migration-observation</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:44</code> | <code>FundedMigrationObservation { state: &amp;'static str, producer_state: String, order_digest: [u8; 32], operation: String, source_evidence_digest: [u8; 32], source_claim_id?: Option&lt;[u8; 32]&gt;, provenance: &amp;'static str, layerx_receipt: bool, layerx_credit_verified: bool }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-funded-migration-observation</code> | notes | migration journal/wire JSON via serde Serialize |

## fmt-migrate-head-body

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-head-body</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:214</code> | <code>HeadBody { sequence: u64, digest: [u8; 32] }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-head-body</code> | notes | migration journal/wire JSON via serde Serialize, Deserialize |

## fmt-migrate-head-seal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-head-seal</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:220</code> | <code>HeadSeal { body: HeadBody, mac: [u8; 32] }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-head-seal</code> | notes | migration journal/wire JSON via serde Serialize, Deserialize |

## fmt-migrate-history-checkpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-history-checkpoint</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:153</code> | <code>HistoryCheckpoint { previous_cursor?: Option&lt;[u8; 32]&gt;, previous_anchor_hash?: Option&lt;[u8; 32]&gt;, from: u64, to: u64, anchor_hash: [u8; 32], evidence_digest: [u8; 32], next_cursor: [u8; 32], committed: bool }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-history-checkpoint</code> | notes | migration journal/wire JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize |

## fmt-migrate-mapping-checkpoint-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-mapping-checkpoint-v2</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:59</code> | <code>MappingCheckpointV2 { principal: String, source_network: String, source_address: String, layerx_identity: [u8; 32], evm_address: [u8; 20], paxeer_block_number: u64, paxeer_block_hash: [u8; 32], evidence_digest: [u8; 32] }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-mapping-checkpoint-v2</code> | notes | migration journal/wire JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize, deny_unknown_fields |

## fmt-migrate-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-record</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:208</code> | <code>Record { body: RecordBody, mac: [u8; 32] }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-record</code> | notes | migration journal/wire JSON via serde Serialize, Deserialize |

## fmt-migrate-record-body

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-record-body</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:201</code> | <code>RecordBody { sequence: u64, previous: [u8; 32], update: Update }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-record-body</code> | notes | migration journal/wire JSON via serde Serialize, Deserialize |

## fmt-migrate-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-settlement</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:66</code> | <code>Settlement { source_settlement: SourceSettlementResponseV2, provenance: String, custody_label: String, layerx_receipt: bool }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-settlement</code> | notes | migration journal/wire JSON via serde Deserialize, deny_unknown_fields |

## fmt-migrate-source-settlement-request-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-source-settlement-request-v2</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ramp_v2.rs:10</code> | <code>SourceSettlementRequestV2 { version: &amp;'static str, order_digest: [u8; 32], chain: String, source_evidence: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-source-settlement-request-v2</code> | notes | migration journal/wire JSON via serde Serialize |

## fmt-migrate-source-settlement-response-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-source-settlement-response-v2</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ramp_v2.rs:37</code> | <code>SourceSettlementResponseV2 { version: String, order_digest: [u8; 32], state: String, source_evidence_digest: [u8; 32], source_claim_id?: Option&lt;[u8; 32]&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-source-settlement-response-v2</code> | notes | migration journal/wire JSON via serde Deserialize, Serialize, deny_unknown_fields |

## fmt-migrate-success

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-success</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:58</code> | <code>Success { ok: bool, operation: String, result: Settlement, trace: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-success</code> | notes | migration journal/wire JSON via serde Deserialize, deny_unknown_fields |

## fmt-migrate-update

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-migrate-update</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:166</code> | <code>Update = chain &#124; claim &#124; history-prepared &#124; history-committed &#124; ownership &#124; account-mapping-v2 &#124; custody-reference &#124; external-history (tag kind)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-migrate-update</code> | notes | migration journal/wire JSON via serde Clone, Debug, Eq, PartialEq, Serialize, Deserialize, rename_all kebab-case |

## migrate-account-mapping-store-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:79</code> | <code>pub struct AccountMappingStoreV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2</code> | notes | struct layerx_migrate::journal::AccountMappingStoreV2; re-exported at services/interop/crates/layerx-migrate/src/lib.rs:15 |

## migrate-account-mapping-store-v2-confirm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2-confirm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:88</code> | <code>pub fn confirm(&amp;self, principal: &amp;PrincipalId, authenticated_identity: [u8; 32], evidence: &amp;crate::SourceEvidence, verifier: &amp;impl crate::SourceVerifier, bindings: &amp;crate::mapping_v2::PaxeerBindingVerifierV2, trace: &amp;layerx_interop_gateway::trace::TraceId) -&gt; Result&lt;AccountMappingV2, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2-confirm</code> | notes | method layerx_migrate::journal::AccountMappingStoreV2::confirm |

## migrate-account-mapping-store-v2-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:84</code> | <code>pub fn new(config: &amp;JournalConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2-new</code> | notes | method layerx_migrate::journal::AccountMappingStoreV2::new |

## migrate-account-mapping-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-v2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:71</code> | <code>pub struct AccountMappingV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-v2</code> | notes | struct layerx_migrate::journal::AccountMappingV2; re-exported at services/interop/crates/layerx-migrate/src/lib.rs:15; no reference outside the defining file found by name match |

## migrate-binding-execution

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-binding-execution</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:547</code> | <code>pub struct BindingExecution&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-binding-execution</code> | notes | struct layerx_migrate::BindingExecution |

## migrate-binding-execution-idempotency-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-binding-execution-idempotency-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:559</code> | <code>pub const fn idempotency_key(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-binding-execution-idempotency-key</code> | notes | method layerx_migrate::BindingExecution::idempotency_key |

## migrate-binding-execution-ownership

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-binding-execution-ownership</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:554</code> | <code>pub const fn ownership(&amp;self) -&gt; &amp;VerifiedOwnership</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-binding-execution-ownership</code> | notes | method layerx_migrate::BindingExecution::ownership |

## migrate-binding-receipt-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:623</code> | <code>pub struct BindingReceiptPolicy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy</code> | notes | struct layerx_migrate::BindingReceiptPolicy |

## migrate-binding-receipt-policy-context-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy-context-hash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:676</code> | <code>pub fn context_hash(ownership: &amp;VerifiedOwnership) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy-context-hash</code> | notes | method layerx_migrate::BindingReceiptPolicy::context_hash |

## migrate-binding-receipt-policy-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:641</code> | <code>pub fn new(sequencer_public_key: [u8; 32], authority: [u8; 32], module_id: u16, module_version: u32, parameter_version: u32, operation: u8, asset: [u8; 32], amount: u128) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy-new</code> | notes | method layerx_migrate::BindingReceiptPolicy::new |

## migrate-confirmed-paxeer-binding-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:30</code> | <code>pub struct ConfirmedPaxeerBindingV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2</code> | notes | struct layerx_migrate::mapping_v2::ConfirmedPaxeerBindingV2; no reference outside the defining file found by name match |

## migrate-confirmed-paxeer-binding-v2-block-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-block-hash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:54</code> | <code>pub const fn block_hash(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-block-hash</code> | notes | method layerx_migrate::mapping_v2::ConfirmedPaxeerBindingV2::block_hash; no reference outside the defining file found by name match |

## migrate-confirmed-paxeer-binding-v2-block-number

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-block-number</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:49</code> | <code>pub const fn block_number(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-block-number</code> | notes | method layerx_migrate::mapping_v2::ConfirmedPaxeerBindingV2::block_number; no reference outside the defining file found by name match |

## migrate-confirmed-paxeer-binding-v2-evm-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-evm-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:44</code> | <code>pub const fn evm_address(&amp;self) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-evm-address</code> | notes | method layerx_migrate::mapping_v2::ConfirmedPaxeerBindingV2::evm_address; no reference outside the defining file found by name match |

## migrate-confirmed-paxeer-binding-v2-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:39</code> | <code>pub const fn identity(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-identity</code> | notes | method layerx_migrate::mapping_v2::ConfirmedPaxeerBindingV2::identity; no reference outside the defining file found by name match |

## migrate-custody-execution

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-custody-execution</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:566</code> | <code>pub struct CustodyExecution&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-custody-execution</code> | notes | struct layerx_migrate::CustodyExecution |

## migrate-custody-execution-finality

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-custody-execution-finality</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:573</code> | <code>pub const fn finality(&amp;self) -&gt; &amp;VerifiedAssetFinality</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-custody-execution-finality</code> | notes | method layerx_migrate::CustodyExecution::finality |

## migrate-custody-execution-idempotency-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-custody-execution-idempotency-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:578</code> | <code>pub const fn idempotency_key(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-custody-execution-idempotency-key</code> | notes | method layerx_migrate::CustodyExecution::idempotency_key |

## migrate-custody-receipt-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:723</code> | <code>pub struct CustodyReceiptPolicy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy</code> | notes | struct layerx_migrate::CustodyReceiptPolicy |

## migrate-custody-receipt-policy-context-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy-context-hash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:767</code> | <code>pub fn context_hash(finality: &amp;VerifiedAssetFinality) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy-context-hash</code> | notes | method layerx_migrate::CustodyReceiptPolicy::context_hash |

## migrate-custody-receipt-policy-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:738</code> | <code>pub fn new(sequencer_public_key: [u8; 32], custody_authority: [u8; 32], module_id: u16, module_version: u32, parameter_version: u32, operation: u8) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy-new</code> | notes | method layerx_migrate::CustodyReceiptPolicy::new |

## migrate-durable-external-history

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/history.rs:18</code> | <code>pub struct DurableExternalHistory</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history</code> | notes | struct layerx_migrate::history::DurableExternalHistory |

## migrate-durable-external-history-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/history.rs:23</code> | <code>pub fn new(config: &amp;JournalConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history-new</code> | notes | method layerx_migrate::history::DurableExternalHistory::new |

## migrate-durable-external-history-read

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history-read</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/history.rs:29</code> | <code>pub fn read(&amp;self, principal: &amp;PrincipalId, after: Option&lt;[u8; 32]&gt;, limit: usize) -&gt; Result&lt;ExternalHistoryPage, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history-read</code> | notes | method layerx_migrate::history::DurableExternalHistory::read; no reference outside the defining file found by name match |

## migrate-ethereum-asset-claim

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-asset-claim</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:159</code> | <code>pub struct EthereumAssetClaim</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-asset-claim</code> | notes | struct layerx_migrate::ethereum::EthereumAssetClaim; no reference outside the defining file found by name match |

## migrate-ethereum-asset-claim-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-asset-claim-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:177</code> | <code>pub fn evidence(self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-asset-claim-evidence</code> | notes | method layerx_migrate::ethereum::EthereumAssetClaim::evidence; no reference outside the defining file found by name match |

## migrate-ethereum-claim-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-claim-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:1337</code> | <code>pub fn ethereum_claim_digest(claim: EthereumAssetClaim) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-claim-digest</code> | notes | fn layerx_migrate::ethereum::ethereum_claim_digest; no reference outside the defining file found by name match |

## migrate-ethereum-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:86</code> | <code>pub struct EthereumConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-config</code> | notes | struct layerx_migrate::ethereum::EthereumConfig |

## migrate-ethereum-contract-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-contract-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:64</code> | <code>pub enum EthereumContractIdentity</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-contract-identity</code> | notes | enum layerx_migrate::ethereum::EthereumContractIdentity; no reference outside the defining file found by name match |

## migrate-ethereum-custody-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-custody-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:42</code> | <code>pub struct EthereumCustodySchema</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-custody-schema</code> | notes | struct layerx_migrate::ethereum::EthereumCustodySchema; no reference outside the defining file found by name match |

## migrate-ethereum-finality-tag

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-finality-tag</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:56</code> | <code>pub enum EthereumFinalityTag</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-finality-tag</code> | notes | enum layerx_migrate::ethereum::EthereumFinalityTag; no reference outside the defining file found by name match |

## migrate-ethereum-history-claim

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-history-claim</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:206</code> | <code>pub struct EthereumHistoryClaim</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-history-claim</code> | notes | struct layerx_migrate::ethereum::EthereumHistoryClaim; no reference outside the defining file found by name match |

## migrate-ethereum-history-claim-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-history-claim-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:220</code> | <code>pub fn evidence(self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-history-claim-evidence</code> | notes | method layerx_migrate::ethereum::EthereumHistoryClaim::evidence; no reference outside the defining file found by name match |

## migrate-ethereum-ownership-claim

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:106</code> | <code>pub struct EthereumOwnershipClaim</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim</code> | notes | struct layerx_migrate::ethereum::EthereumOwnershipClaim; no reference outside the defining file found by name match |

## migrate-ethereum-ownership-claim-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:135</code> | <code>pub fn evidence(&amp;self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim-evidence</code> | notes | method layerx_migrate::ethereum::EthereumOwnershipClaim::evidence; no reference outside the defining file found by name match |

## migrate-ethereum-ownership-claim-signing-message

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim-signing-message</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:119</code> | <code>pub fn signing_message(&amp;self) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim-signing-message</code> | notes | method layerx_migrate::ethereum::EthereumOwnershipClaim::signing_message; no reference outside the defining file found by name match |

## migrate-ethereum-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:246</code> | <code>pub struct EthereumVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier</code> | notes | struct layerx_migrate::ethereum::EthereumVerifier |

## migrate-ethereum-verifier-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:288</code> | <code>pub fn new(config: EthereumConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier-new</code> | notes | method layerx_migrate::ethereum::EthereumVerifier::new; chain calls: eth_chainId at :300 via private fn verify_network, eth_getCode at :318 via private fn verify_contract, eth_getStorageAt at :337 via private fn verify_contract, eth_getCode at :353 via private fn verify_contract, eth_getBlockByNumber at :457 via private fn block_by_tag, eth_getBlockByNumber at :464 via private fn block_by_number, eth_getBlockByHash at :472 via private fn block_by_hash, eth_getTransactionReceipt at :478 via private fn receipt, eth_getTransactionByHash at :489 via private fn transaction, eth_getLogs at :648 via private fn history, eth_getLogs at :662 via private fn history, eth_getLogs at :670 via private fn history, eth_getBlockByNumber at :747 via private fn history_transactions |

## migrate-ethereum-verifier-verify-owned-history

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier-verify-owned-history</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:255</code> | <code>pub fn verify_owned_history(&amp;self, ownership_evidence: &amp;SourceEvidence, history_evidence: &amp;SourceEvidence, authenticated_identity: [u8; 32], trace: &amp;TraceId) -&gt; Result&lt;VerifiedHistoryPage, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier-verify-owned-history</code> | notes | method layerx_migrate::ethereum::EthereumVerifier::verify_owned_history; no reference outside the defining file found by name match |

## migrate-ethereum-word

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-word</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ethereum.rs:34</code> | <code>pub enum EthereumWord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-ethereum-word</code> | notes | enum layerx_migrate::ethereum::EthereumWord; no reference outside the defining file found by name match |

## migrate-external-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:75</code> | <code>pub enum ExternalAddress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-address</code> | notes | enum layerx_migrate::ExternalAddress |

## migrate-external-history-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:402</code> | <code>pub enum ExternalHistoryKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-kind</code> | notes | enum layerx_migrate::ExternalHistoryKind |

## migrate-external-history-page

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-page</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/history.rs:58</code> | <code>pub struct ExternalHistoryPage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-page</code> | notes | struct layerx_migrate::history::ExternalHistoryPage |

## migrate-external-history-page-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/history.rs:65</code> | <code>pub fn json(&amp;self) -&gt; Value</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-json</code> | notes | method layerx_migrate::history::ExternalHistoryPage::json; no reference outside the defining file found by name match |

## migrate-external-history-page-next-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-next-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/history.rs:95</code> | <code>pub const fn next_cursor(&amp;self) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-next-cursor</code> | notes | method layerx_migrate::history::ExternalHistoryPage::next_cursor; no reference outside the defining file found by name match |

## migrate-external-history-page-records

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-records</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/history.rs:90</code> | <code>pub fn records(&amp;self) -&gt; &amp;[ExternalHistoryRecord]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-records</code> | notes | method layerx_migrate::history::ExternalHistoryPage::records; no reference outside the defining file found by name match |

## migrate-external-history-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:411</code> | <code>pub struct ExternalHistoryRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record</code> | notes | struct layerx_migrate::ExternalHistoryRecord |

## migrate-external-history-record-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:441</code> | <code>pub const fn address(&amp;self) -&gt; ExternalAddress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-address</code> | notes | method layerx_migrate::ExternalHistoryRecord::address; no reference outside the defining file found by name match |

## migrate-external-history-record-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:431</code> | <code>pub const fn chain(&amp;self) -&gt; SourceChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-chain</code> | notes | method layerx_migrate::ExternalHistoryRecord::chain |

## migrate-external-history-record-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:446</code> | <code>pub const fn kind(&amp;self) -&gt; ExternalHistoryKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-kind</code> | notes | method layerx_migrate::ExternalHistoryRecord::kind; no reference outside the defining file found by name match |

## migrate-external-history-record-provenance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-provenance</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:466</code> | <code>pub const fn provenance(&amp;self) -&gt; ExternalProvenance</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-provenance</code> | notes | method layerx_migrate::ExternalHistoryRecord::provenance |

## migrate-external-history-record-source-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-source-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:461</code> | <code>pub const fn source_amount(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-source-amount</code> | notes | method layerx_migrate::ExternalHistoryRecord::source_amount |

## migrate-external-history-record-source-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-source-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:456</code> | <code>pub const fn source_asset(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-source-asset</code> | notes | method layerx_migrate::ExternalHistoryRecord::source_asset |

## migrate-external-history-record-timestamp

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-timestamp</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:451</code> | <code>pub const fn timestamp(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-timestamp</code> | notes | method layerx_migrate::ExternalHistoryRecord::timestamp |

## migrate-external-history-record-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:436</code> | <code>pub const fn transaction(&amp;self) -&gt; SourceTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-transaction</code> | notes | method layerx_migrate::ExternalHistoryRecord::transaction |

## migrate-external-history-sink

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-sink</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:526</code> | <code>pub trait ExternalHistorySink</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-history-sink</code> | notes | trait layerx_migrate::ExternalHistorySink |

## migrate-external-provenance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-external-provenance</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:424</code> | <code>pub enum ExternalProvenance</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-external-provenance</code> | notes | enum layerx_migrate::ExternalProvenance |

## migrate-funded-migration-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-funded-migration-observation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:44</code> | <code>pub struct FundedMigrationObservation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-funded-migration-observation</code> | notes | struct layerx_migrate::gateway_client::FundedMigrationObservation; no reference outside the defining file found by name match |

## migrate-gateway-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-gateway-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:33</code> | <code>pub struct GatewayClient</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-gateway-client</code> | notes | struct layerx_migrate::gateway_client::GatewayClient |

## migrate-gateway-client-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:24</code> | <code>pub struct GatewayClientConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-config</code> | notes | struct layerx_migrate::gateway_client::GatewayClientConfig |

## migrate-gateway-client-migrate-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-migrate-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:117</code> | <code>pub fn migrate_asset(&amp;self, order_digest: [u8; 32], chain: &amp;str, evidence: &amp;SourceEvidence, idempotency: &amp;str, trace: &amp;TraceId) -&gt; Result&lt;FundedMigrationObservation, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-migrate-asset</code> | notes | method layerx_migrate::gateway_client::GatewayClient::migrate_asset |

## migrate-gateway-client-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/gateway_client.rs:74</code> | <code>pub fn new(config: &amp;GatewayClientConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-new</code> | notes | method layerx_migrate::gateway_client::GatewayClient::new |

## migrate-interop-migrate-ethereum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-interop-migrate-ethereum</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1322</code> | <code>pub const fn interop_migrate_ethereum() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-interop-migrate-ethereum</code> | notes | fn layerx_migrate::interop_migrate_ethereum |

## migrate-interop-migrate-solana

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-interop-migrate-solana</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1328</code> | <code>pub const fn interop_migrate_solana() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-interop-migrate-solana</code> | notes | fn layerx_migrate::interop_migrate_solana |

## migrate-journal-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-journal-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/journal.rs:33</code> | <code>pub struct JournalConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-journal-config</code> | notes | struct layerx_migrate::journal::JournalConfig; re-exported at services/interop/crates/layerx-migrate/src/lib.rs:15 |

## migrate-migration-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:824</code> | <code>pub struct MigrationAdapter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter</code> | notes | struct layerx_migrate::MigrationAdapter |

## migrate-migration-adapter-descriptor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-descriptor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1100</code> | <code>pub fn migration_adapter_descriptor(spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-descriptor</code> | notes | fn layerx_migrate::migration_adapter_descriptor |

## migrate-migration-adapter-import-history

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-import-history</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:941</code> | <code>pub fn import_history(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, evidence: &amp;SourceEvidence, verifier: &amp;impl SourceVerifier, sink: &amp;mut impl ExternalHistorySink, trace: &amp;TraceId, now: u64) -&gt; Result&lt;MigrationState, Traced&lt;MigrationError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-import-history</code> | notes | method layerx_migrate::MigrationAdapter::import_history |

## migrate-migration-adapter-map-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-map-account</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:833</code> | <code>pub fn map_account(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, evidence: &amp;SourceEvidence, verifier: &amp;impl SourceVerifier, plane: &amp;mut impl MigrationPlane, binding_policy: &amp;BindingReceiptPolicy, trace: &amp;TraceId, now: u64) -&gt; Result&lt;MigrationState, Traced&lt;MigrationError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-map-account</code> | notes | method layerx_migrate::MigrationAdapter::map_account |

## migrate-migration-adapter-migrate-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-migrate-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:886</code> | <code>pub fn migrate_asset(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, evidence: &amp;SourceEvidence, verifier: &amp;impl SourceVerifier, plane: &amp;mut impl MigrationPlane, receipt_policy: &amp;CustodyReceiptPolicy, trace: &amp;TraceId, now: u64) -&gt; Result&lt;MigrationState, Traced&lt;MigrationError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-migrate-asset</code> | notes | method layerx_migrate::MigrationAdapter::migrate_asset |

## migrate-migration-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1194</code> | <code>pub enum MigrationError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-error</code> | notes | enum layerx_migrate::MigrationError; 787 reference lines in 14 files; callers lists the first reference in each file |

## migrate-migration-error-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-error-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:1226</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-error-code</code> | notes | method layerx_migrate::MigrationError::code; no reference outside the defining file found by name match |

## migrate-migration-plane

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-plane</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:596</code> | <code>pub trait MigrationPlane</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-plane</code> | notes | trait layerx_migrate::MigrationPlane |

## migrate-migration-plane-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-plane-result</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:586</code> | <code>pub enum MigrationPlaneResult</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-plane-result</code> | notes | enum layerx_migrate::MigrationPlaneResult |

## migrate-migration-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-migration-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:813</code> | <code>pub enum MigrationState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-migration-state</code> | notes | enum layerx_migrate::MigrationState |

## migrate-paxeer-binding-config-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-config-v2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:18</code> | <code>pub struct PaxeerBindingConfigV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-config-v2</code> | notes | struct layerx_migrate::mapping_v2::PaxeerBindingConfigV2 |

## migrate-paxeer-binding-verifier-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:24</code> | <code>pub struct PaxeerBindingVerifierV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2</code> | notes | struct layerx_migrate::mapping_v2::PaxeerBindingVerifierV2 |

## migrate-paxeer-binding-verifier-v2-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:60</code> | <code>pub fn new(config: PaxeerBindingConfigV2) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2-new</code> | notes | method layerx_migrate::mapping_v2::PaxeerBindingVerifierV2::new |

## migrate-paxeer-binding-verifier-v2-verify-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2-verify-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/mapping_v2.rs:68</code> | <code>pub fn verify_identity(&amp;self, expected_did: [u8; 32]) -&gt; Result&lt;ConfirmedPaxeerBindingV2, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2-verify-identity</code> | notes | method layerx_migrate::mapping_v2::PaxeerBindingVerifierV2::verify_identity; spec/layerx-merge/spec.kvx task 7.6.2 (in_progress): HEAD already reads 0x1004 at one finalized block (services/interop/crates/layerx-migrate/src/mapping_v2.rs:76-95); chain calls: eth_chainId at :114 via private fn verify_network, eth_getBlockByNumber at :121 via private fn verify_network, eth_getBlockByNumber at :129 via private fn finalized_block, eth_getBlockByNumber at :141 via private fn block_by_number, eth_call at :156 via private fn call_precompile |

## migrate-rpc-endpoint-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-rpc-endpoint-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:28</code> | <code>pub struct RpcEndpointConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-rpc-endpoint-config</code> | notes | struct layerx_migrate::rpc::RpcEndpointConfig; re-exported at services/interop/crates/layerx-migrate/src/lib.rs:16; no reference outside the defining file found by name match |

## migrate-rpc-quorum-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-rpc-quorum-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/rpc.rs:40</code> | <code>pub struct RpcQuorumConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-rpc-quorum-config</code> | notes | struct layerx_migrate::rpc::RpcQuorumConfig; re-exported at services/interop/crates/layerx-migrate/src/lib.rs:16 |

## migrate-solana-amount-endian

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-amount-endian</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:32</code> | <code>pub enum SolanaAmountEndian</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-amount-endian</code> | notes | enum layerx_migrate::solana::SolanaAmountEndian; no reference outside the defining file found by name match |

## migrate-solana-asset-claim

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-asset-claim</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:125</code> | <code>pub struct SolanaAssetClaim</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-asset-claim</code> | notes | struct layerx_migrate::solana::SolanaAssetClaim; no reference outside the defining file found by name match |

## migrate-solana-asset-claim-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-asset-claim-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:144</code> | <code>pub fn evidence(self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-asset-claim-evidence</code> | notes | method layerx_migrate::solana::SolanaAssetClaim::evidence; no reference outside the defining file found by name match |

## migrate-solana-claim-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-claim-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:1618</code> | <code>pub fn solana_claim_digest(claim: &amp;SolanaAssetClaim) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-claim-digest</code> | notes | fn layerx_migrate::solana::solana_claim_digest; no reference outside the defining file found by name match |

## migrate-solana-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:57</code> | <code>pub struct SolanaConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-config</code> | notes | struct layerx_migrate::solana::SolanaConfig |

## migrate-solana-history-claim

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-history-claim</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:163</code> | <code>pub struct SolanaHistoryClaim</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-history-claim</code> | notes | struct layerx_migrate::solana::SolanaHistoryClaim; no reference outside the defining file found by name match |

## migrate-solana-history-claim-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-history-claim-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:177</code> | <code>pub fn evidence(self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-history-claim-evidence</code> | notes | method layerx_migrate::solana::SolanaHistoryClaim::evidence; no reference outside the defining file found by name match |

## migrate-solana-instruction-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-instruction-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:40</code> | <code>pub struct SolanaInstructionSchema</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-instruction-schema</code> | notes | struct layerx_migrate::solana::SolanaInstructionSchema; no reference outside the defining file found by name match |

## migrate-solana-ownership-claim

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:80</code> | <code>pub struct SolanaOwnershipClaim</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim</code> | notes | struct layerx_migrate::solana::SolanaOwnershipClaim; no reference outside the defining file found by name match |

## migrate-solana-ownership-claim-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:109</code> | <code>pub fn evidence(&amp;self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim-evidence</code> | notes | method layerx_migrate::solana::SolanaOwnershipClaim::evidence; no reference outside the defining file found by name match |

## migrate-solana-ownership-claim-signing-message

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim-signing-message</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:93</code> | <code>pub fn signing_message(&amp;self) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim-signing-message</code> | notes | method layerx_migrate::solana::SolanaOwnershipClaim::signing_message; no reference outside the defining file found by name match |

## migrate-solana-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:196</code> | <code>pub struct SolanaVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier</code> | notes | struct layerx_migrate::solana::SolanaVerifier |

## migrate-solana-verifier-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:239</code> | <code>pub fn new(config: SolanaConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier-new</code> | notes | method layerx_migrate::solana::SolanaVerifier::new; chain calls: getGenesisHash at :251 via private fn verify_network, getAccountInfo at :296 via private fn account, getTransaction at :350 via private fn transaction, getBlock at :368 via private fn block, getSlot at :424 via private fn finality, getSignatureStatuses at :489 via private fn verify_status, getSlot at :672 via private fn history |

## migrate-solana-verifier-verify-owned-history

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier-verify-owned-history</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/solana.rs:205</code> | <code>pub fn verify_owned_history(&amp;self, ownership_evidence: &amp;SourceEvidence, history_evidence: &amp;SourceEvidence, authenticated_identity: [u8; 32], trace: &amp;TraceId) -&gt; Result&lt;VerifiedHistoryPage, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier-verify-owned-history</code> | notes | method layerx_migrate::solana::SolanaVerifier::verify_owned_history; no reference outside the defining file found by name match |

## migrate-source-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:45</code> | <code>pub enum SourceChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-chain</code> | notes | enum layerx_migrate::SourceChain |

## migrate-source-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:177</code> | <code>pub struct SourceEvidence</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-evidence</code> | notes | struct layerx_migrate::SourceEvidence; 67 reference lines in 10 files; callers lists the first reference in each file |

## migrate-source-evidence-canonical

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-canonical</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:197</code> | <code>pub fn canonical(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-canonical</code> | notes | method layerx_migrate::SourceEvidence::canonical |

## migrate-source-evidence-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:202</code> | <code>pub const fn digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-digest</code> | notes | method layerx_migrate::SourceEvidence::digest; no reference outside the defining file found by name match |

## migrate-source-evidence-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:188</code> | <code>pub fn new(canonical: Vec&lt;u8&gt;) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-new</code> | notes | method layerx_migrate::SourceEvidence::new |

## migrate-source-settlement-request-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-request-v2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ramp_v2.rs:10</code> | <code>pub struct SourceSettlementRequestV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-request-v2</code> | notes | struct layerx_migrate::ramp_v2::SourceSettlementRequestV2 |

## migrate-source-settlement-request-v2-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-request-v2-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ramp_v2.rs:18</code> | <code>pub fn new(order_digest: [u8; 32], chain: &amp;str, evidence: &amp;SourceEvidence) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-request-v2-new</code> | notes | method layerx_migrate::ramp_v2::SourceSettlementRequestV2::new |

## migrate-source-settlement-response-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-response-v2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ramp_v2.rs:37</code> | <code>pub struct SourceSettlementResponseV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-response-v2</code> | notes | struct layerx_migrate::ramp_v2::SourceSettlementResponseV2 |

## migrate-source-settlement-response-v2-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-response-v2-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/ramp_v2.rs:46</code> | <code>pub fn validate(&amp;self, request: &amp;SourceSettlementRequestV2, evidence: &amp;SourceEvidence) -&gt; Result&lt;(), MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-response-v2-validate</code> | notes | method layerx_migrate::ramp_v2::SourceSettlementResponseV2::validate; no reference outside the defining file found by name match |

## migrate-source-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:109</code> | <code>pub enum SourceTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-transaction</code> | notes | enum layerx_migrate::SourceTransaction |

## migrate-source-transaction-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:142</code> | <code>pub const fn bytes(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-bytes</code> | notes | method layerx_migrate::SourceTransaction::bytes; no reference outside the defining file found by name match |

## migrate-source-transaction-ethereum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-ethereum</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:120</code> | <code>pub fn ethereum(value: [u8; 32]) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-ethereum</code> | notes | method layerx_migrate::SourceTransaction::ethereum |

## migrate-source-transaction-solana

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-solana</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:133</code> | <code>pub fn solana(value: [u8; 64]) -&gt; Result&lt;Self, MigrationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-solana</code> | notes | method layerx_migrate::SourceTransaction::solana |

## migrate-source-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-source-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:351</code> | <code>pub trait SourceVerifier: sealed::SourceVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-source-verifier</code> | notes | trait layerx_migrate::SourceVerifier |

## migrate-verified-asset-finality

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:260</code> | <code>pub struct VerifiedAssetFinality</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality</code> | notes | struct layerx_migrate::VerifiedAssetFinality |

## migrate-verified-asset-finality-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:276</code> | <code>pub const fn chain(&amp;self) -&gt; SourceChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-chain</code> | notes | method layerx_migrate::VerifiedAssetFinality::chain |

## migrate-verified-asset-finality-custody-reference

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-custody-reference</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:301</code> | <code>pub const fn custody_reference(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-custody-reference</code> | notes | method layerx_migrate::VerifiedAssetFinality::custody_reference; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-destination

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-destination</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:316</code> | <code>pub const fn destination(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-destination</code> | notes | method layerx_migrate::VerifiedAssetFinality::destination; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-evidence-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-evidence-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:326</code> | <code>pub const fn evidence_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-evidence-digest</code> | notes | method layerx_migrate::VerifiedAssetFinality::evidence_digest; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-finality-height

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-finality-height</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:321</code> | <code>pub const fn finality_height(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-finality-height</code> | notes | method layerx_migrate::VerifiedAssetFinality::finality_height; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-layerx-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-layerx-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:311</code> | <code>pub const fn layerx_amount(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-layerx-amount</code> | notes | method layerx_migrate::VerifiedAssetFinality::layerx_amount; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-layerx-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-layerx-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:306</code> | <code>pub const fn layerx_asset(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-layerx-asset</code> | notes | method layerx_migrate::VerifiedAssetFinality::layerx_asset; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:286</code> | <code>pub const fn source(&amp;self) -&gt; ExternalAddress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source</code> | notes | method layerx_migrate::VerifiedAssetFinality::source; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-source-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:296</code> | <code>pub const fn source_amount(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source-amount</code> | notes | method layerx_migrate::VerifiedAssetFinality::source_amount; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-source-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:291</code> | <code>pub const fn source_asset(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source-asset</code> | notes | method layerx_migrate::VerifiedAssetFinality::source_asset; no reference outside the defining file found by name match |

## migrate-verified-asset-finality-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:281</code> | <code>pub const fn transaction(&amp;self) -&gt; SourceTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-transaction</code> | notes | method layerx_migrate::VerifiedAssetFinality::transaction |

## migrate-verified-history-page

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:492</code> | <code>pub struct VerifiedHistoryPage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page</code> | notes | struct layerx_migrate::VerifiedHistoryPage |

## migrate-verified-history-page-evidence-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-evidence-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:510</code> | <code>pub const fn evidence_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-evidence-digest</code> | notes | method layerx_migrate::VerifiedHistoryPage::evidence_digest |

## migrate-verified-history-page-next-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-next-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:505</code> | <code>pub const fn next_cursor(&amp;self) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-next-cursor</code> | notes | method layerx_migrate::VerifiedHistoryPage::next_cursor |

## migrate-verified-history-page-records

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-records</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:500</code> | <code>pub fn records(&amp;self) -&gt; &amp;[ExternalHistoryRecord]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-records</code> | notes | method layerx_migrate::VerifiedHistoryPage::records |

## migrate-verified-ownership

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:219</code> | <code>pub struct VerifiedOwnership</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership</code> | notes | struct layerx_migrate::VerifiedOwnership |

## migrate-verified-ownership-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:233</code> | <code>pub const fn address(&amp;self) -&gt; ExternalAddress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-address</code> | notes | method layerx_migrate::VerifiedOwnership::address; no reference outside the defining file found by name match |

## migrate-verified-ownership-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:228</code> | <code>pub const fn chain(&amp;self) -&gt; SourceChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-chain</code> | notes | method layerx_migrate::VerifiedOwnership::chain |

## migrate-verified-ownership-evidence-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-evidence-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:243</code> | <code>pub const fn evidence_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-evidence-digest</code> | notes | method layerx_migrate::VerifiedOwnership::evidence_digest; no reference outside the defining file found by name match |

## migrate-verified-ownership-layerx-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-layerx-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-migrate/src/lib.rs:238</code> | <code>pub const fn layerx_identity(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-layerx-identity</code> | notes | method layerx_migrate::VerifiedOwnership::layerx_identity |
