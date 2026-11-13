# Interop migrate contract

The [migration service](../../../services/interop/crates/layerx-migrate/src/lib.rs) verifies external ownership and history evidence and journals migration progress. Its versioned HTTP operations are exposed by the interop service.

## cfg-migrate-ethereum-config-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config chain_id: u64 (required)</code> |

## cfg-migrate-ethereum-config-custody

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config custody: EthereumCustodySchema (required)</code> |

## cfg-migrate-ethereum-config-custody-code-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-code-hash</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config custody_code_hash: [u8; 32] (required)</code> |

## cfg-migrate-ethereum-config-custody-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-contract</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config custody_contract: [u8; 20] (required)</code> |

## cfg-migrate-ethereum-config-custody-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-custody-identity</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config custody_identity: EthereumContractIdentity (required)</code> |

## cfg-migrate-ethereum-config-finality-tag

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-finality-tag</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config finality_tag: EthereumFinalityTag (required)</code> |

## cfg-migrate-ethereum-config-genesis-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-genesis-hash</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config genesis_hash: [u8; 32] (required)</code> |

## cfg-migrate-ethereum-config-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-journal</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config journal: JournalConfig (required)</code> |

## cfg-migrate-ethereum-config-maximum-ancestry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-ancestry</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config maximum_ancestry: u64 (required)</code> |

## cfg-migrate-ethereum-config-maximum-history-blocks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-history-blocks</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config maximum_history_blocks: u64 (required)</code> |

## cfg-migrate-ethereum-config-maximum-ownership-ttl-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-maximum-ownership-ttl-seconds</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config maximum_ownership_ttl_seconds: u64 (required)</code> |

## cfg-migrate-ethereum-config-minimum-confirmations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-minimum-confirmations</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config minimum_confirmations: u64 (required)</code> |

## cfg-migrate-ethereum-config-native-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-native-asset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config native_asset: [u8; 32] (required)</code> |

## cfg-migrate-ethereum-config-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-config-rpc</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum config rpc: RpcQuorumConfig (required)</code> |

## cfg-migrate-ethereum-contract-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-contract-identity</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum contract identity: one of "Immutable" &#124; "Proxy"</code> |

## cfg-migrate-ethereum-custody-schema-custody-reference

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-custody-reference</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema custody_reference: EthereumWord (required)</code> |

## cfg-migrate-ethereum-custody-schema-destination

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-destination</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema destination: EthereumWord (required)</code> |

## cfg-migrate-ethereum-custody-schema-event-topic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-event-topic</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema event_topic: [u8; 32] (required)</code> |

## cfg-migrate-ethereum-custody-schema-function-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-function-selector</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema function_selector: [u8; 4] (required)</code> |

## cfg-migrate-ethereum-custody-schema-layerx-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-layerx-amount</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema layerx_amount: EthereumWord (required)</code> |

## cfg-migrate-ethereum-custody-schema-layerx-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-layerx-asset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema layerx_asset: EthereumWord (required)</code> |

## cfg-migrate-ethereum-custody-schema-source

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema source: EthereumWord (required)</code> |

## cfg-migrate-ethereum-custody-schema-source-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source-amount</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema source_amount: EthereumWord (required)</code> |

## cfg-migrate-ethereum-custody-schema-source-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-custody-schema-source-asset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum custody_schema source_asset: EthereumWord (required)</code> |

## cfg-migrate-ethereum-finality-tag

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-finality-tag</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum finality: one of "Safe" &#124; "Finalized"</code> |

## cfg-migrate-ethereum-word

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-ethereum-word</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>migration ethereum word: one of "Topic" &#124; "Data"</code> |

## cfg-migrate-gateway-client-config-ca-certificate-der

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-ca-certificate-der</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>migrate gateway-config ca_certificate_der: PathBuf (required)</code> |

## cfg-migrate-gateway-client-config-connect-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-connect-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>migrate gateway-config connect_timeout_ms: u64 (required)</code> |

## cfg-migrate-gateway-client-config-customer-authorization-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-customer-authorization-file</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>migrate gateway-config customer_authorization_file: PathBuf (required)</code> |

## cfg-migrate-gateway-client-config-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-endpoint</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>migrate gateway-config endpoint: String (required)</code> |

## cfg-migrate-gateway-client-config-maximum-response-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-maximum-response-bytes</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>migrate gateway-config maximum_response_bytes: usize (required)</code> |

## cfg-migrate-gateway-client-config-request-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-gateway-client-config-request-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>migrate gateway-config request_timeout_ms: u64 (required)</code> |

## cfg-migrate-journal-config-authentication-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-authentication-key-file</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>migration journal config authentication_key_file: PathBuf (required)</code> |

## cfg-migrate-journal-config-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-directory</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>migration journal config directory: PathBuf (required)</code> |

## cfg-migrate-journal-config-namespace

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-namespace</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>migration journal config namespace: String (required)</code> |

## cfg-migrate-journal-config-rollback-anchor-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-rollback-anchor-id</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>migration journal config rollback_anchor_id: String (required)</code> |

## cfg-migrate-journal-config-rollback-anchor-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-journal-config-rollback-anchor-rpc</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>migration journal config rollback_anchor_rpc: RpcQuorumConfig (required)</code> |

## cfg-migrate-paxeer-binding-config-v2-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>migration paxeer_binding config chain_id: u64 (required)</code> |

## cfg-migrate-paxeer-binding-config-v2-genesis-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-genesis-hash</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>migration paxeer_binding config genesis_hash: [u8; 32] (required)</code> |

## cfg-migrate-paxeer-binding-config-v2-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-paxeer-binding-config-v2-rpc</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>migration paxeer_binding config rpc: RpcQuorumConfig (required)</code> |

## cfg-migrate-rpc-endpoint-config-bearer-token-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-bearer-token-file</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc endpoint bearer_token_file: PathBuf (required)</code> |

## cfg-migrate-rpc-endpoint-config-ca-certificate-der

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-ca-certificate-der</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc endpoint ca_certificate_der: PathBuf (required)</code> |

## cfg-migrate-rpc-endpoint-config-independent-backend

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-independent-backend</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc endpoint independent_backend: String (required)</code> |

## cfg-migrate-rpc-endpoint-config-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-endpoint-config-url</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc endpoint url: String (required)</code> |

## cfg-migrate-rpc-quorum-config-connect-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-connect-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc quorum connect_timeout_ms: u64 (required)</code> |

## cfg-migrate-rpc-quorum-config-endpoints

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-endpoints</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc quorum endpoints: Vec&lt;RpcEndpointConfig&gt; (required)</code> |

## cfg-migrate-rpc-quorum-config-maximum-response-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-maximum-response-bytes</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc quorum maximum_response_bytes: usize (required)</code> |

## cfg-migrate-rpc-quorum-config-quorum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-quorum</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc quorum quorum: usize (required)</code> |

## cfg-migrate-rpc-quorum-config-request-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-rpc-quorum-config-request-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>migration rpc quorum request_timeout_ms: u64 (required)</code> |

## cfg-migrate-solana-amount-endian

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-amount-endian</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana amount endian: one of "Big" &#124; "Little"</code> |

## cfg-migrate-solana-config-custody

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody: SolanaInstructionSchema (required)</code> |

## cfg-migrate-solana-config-custody-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-account</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody_account: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-custody-account-owner

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-account-owner</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody_account_owner: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-custody-program

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody_program: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-custody-program-data-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-account</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody_program_data_account: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-custody-program-data-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-hash</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody_program_data_hash: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-custody-program-data-reference-offset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-data-reference-offset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody_program_data_reference_offset: u16 (required)</code> |

## cfg-migrate-solana-config-custody-program-owner

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-program-owner</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody_program_owner: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-custody-token-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-custody-token-authority</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config custody_token_authority: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-genesis-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-genesis-hash</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config genesis_hash: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-journal</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config journal: JournalConfig (required)</code> |

## cfg-migrate-solana-config-maximum-ancestry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-ancestry</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config maximum_ancestry: u64 (required)</code> |

## cfg-migrate-solana-config-maximum-history-slots

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-history-slots</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config maximum_history_slots: u64 (required)</code> |

## cfg-migrate-solana-config-maximum-ownership-ttl-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-maximum-ownership-ttl-seconds</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config maximum_ownership_ttl_seconds: u64 (required)</code> |

## cfg-migrate-solana-config-minimum-rooted-slots

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-minimum-rooted-slots</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config minimum_rooted_slots: u64 (required)</code> |

## cfg-migrate-solana-config-native-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-native-asset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config native_asset: [u8; 32] (required)</code> |

## cfg-migrate-solana-config-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-rpc</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config rpc: RpcQuorumConfig (required)</code> |

## cfg-migrate-solana-config-source-asset-account-owner

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-config-source-asset-account-owner</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana config source_asset_account_owner: [u8; 32] (required)</code> |

## cfg-migrate-solana-instruction-schema-amount-endian

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-amount-endian</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema amount_endian: SolanaAmountEndian (required)</code> |

## cfg-migrate-solana-instruction-schema-custody-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-custody-account</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema custody_account: u16 (required)</code> |

## cfg-migrate-solana-instruction-schema-custody-reference-offset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-custody-reference-offset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema custody_reference_offset: u16 (required)</code> |

## cfg-migrate-solana-instruction-schema-destination-offset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-destination-offset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema destination_offset: u16 (required)</code> |

## cfg-migrate-solana-instruction-schema-discriminator

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-discriminator</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema discriminator: Vec&lt;u8&gt; (required)</code> |

## cfg-migrate-solana-instruction-schema-layerx-amount-offset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-layerx-amount-offset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema layerx_amount_offset: u16 (required)</code> |

## cfg-migrate-solana-instruction-schema-layerx-asset-offset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-layerx-asset-offset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema layerx_asset_offset: u16 (required)</code> |

## cfg-migrate-solana-instruction-schema-source-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-account</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema source_account: u16 (required)</code> |

## cfg-migrate-solana-instruction-schema-source-amount-offset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-amount-offset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema source_amount_offset: u16 (required)</code> |

## cfg-migrate-solana-instruction-schema-source-asset-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-asset-account</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema source_asset_account: u16 (required)</code> |

## cfg-migrate-solana-instruction-schema-source-asset-offset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-migrate-solana-instruction-schema-source-asset-offset</code> | <code>config_key</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>migration solana instruction schema source_asset_offset: u16 (required)</code> |

## cli-migrate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-migrate</code> | <code>cli_command</code> | [services/interop/crates/layerx-migrate/src/main.rs](../../../services/interop/crates/layerx-migrate/src/main.rs) | <code>layerx-migrate &lt;migrate-asset&#124;confirm-mapping&#124;verify-ownership&#124;verify-asset&#124;import-history&#124;read-history&gt; [--chain --config --evidence --store-config --principal --trace --cursor --limit --binding-config --mapping-config --identity --gateway-config --order-digest --idempotency]</code> |

## env-migrate-layerx-migration-secret-dir

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-migrate-layerx-migration-secret-dir</code> | <code>env_var</code> | [services/interop/crates/layerx-migrate/src/main.rs](../../../services/interop/crates/layerx-migrate/src/main.rs) | <code>LAYERX_MIGRATION_SECRET_DIR</code> |

## err-migrate-address-chain-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-address-chain-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::AddressChainMismatch.code() = "address_chain_mismatch"</code> |

## err-migrate-checkpoint-conflict

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-checkpoint-conflict</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::CheckpointConflict.code() = "checkpoint_conflict"</code> |

## err-migrate-checkpoint-integrity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-checkpoint-integrity</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::CheckpointIntegrity.code() = "checkpoint_integrity"</code> |

## err-migrate-configuration-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-configuration-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::Configuration.code() = "configuration_invalid"</code> |

## err-migrate-custody-event-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-custody-event-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::CustodyEventMismatch.code() = "custody_event_mismatch"</code> |

## err-migrate-custody-program-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-custody-program-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::CustodyProgramMismatch.code() = "custody_program_mismatch"</code> |

## err-migrate-evidence-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-evidence-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::EvidenceMismatch.code() = "evidence_mismatch"</code> |

## err-migrate-finality-window-exceeded

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-finality-window-exceeded</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::FinalityWindowExceeded.code() = "finality_window_exceeded"</code> |

## err-migrate-gateway-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-gateway-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::Gateway.code() = "gateway_refused"</code> |

## err-migrate-gateway-submission-unknown

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-gateway-submission-unknown</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::GatewaySubmissionUnknown.code() = "gateway_submission_unknown"</code> |

## err-migrate-invalid-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-invalid-address</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::InvalidAddress.code() = "invalid_address"</code> |

## err-migrate-invalid-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-invalid-evidence</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::InvalidEvidence.code() = "invalid_evidence"</code> |

## err-migrate-invalid-history

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-invalid-history</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::InvalidHistory.code() = "invalid_history"</code> |

## err-migrate-invalid-network

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-invalid-network</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::InvalidNetwork.code() = "invalid_network"</code> |

## err-migrate-invalid-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-invalid-transaction</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::InvalidTransaction.code() = "invalid_transaction"</code> |

## err-migrate-ownership-signature-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-ownership-signature-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::OwnershipSignatureMismatch.code() = "ownership_signature_mismatch"</code> |

## err-migrate-plane-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-plane-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::PlaneRefused.code() = "plane_refused"</code> |

## err-migrate-receipt-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-receipt-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::ReceiptMismatch.code() = "receipt_mismatch"</code> |

## err-migrate-receipt-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-receipt-required</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::ReceiptRequired.code() = "receipt_required"</code> |

## err-migrate-rpc-divergence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-rpc-divergence</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::RpcDivergence.code() = "rpc_divergence"</code> |

## err-migrate-rpc-rate-limited

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-rpc-rate-limited</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::RpcRateLimited.code() = "rpc_rate_limited"</code> |

## err-migrate-rpc-response-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-rpc-response-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::RpcResponseMismatch.code() = "rpc_response_mismatch"</code> |

## err-migrate-rpc-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-rpc-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::RpcUnavailable.code() = "rpc_unavailable"</code> |

## err-migrate-source-displaced

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-source-displaced</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::SourceDisplaced.code() = "source_displaced"</code> |

## err-migrate-source-pending

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-source-pending</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::SourcePending.code() = "source_pending"</code> |

## err-migrate-source-reverted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-source-reverted</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::SourceReverted.code() = "source_reverted"</code> |

## err-migrate-storage-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-migrate-storage-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>MigrationError::StorageRefused.code() = "storage_refused"</code> |

## fmt-migrate-account-mapping-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-account-mapping-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: pub struct AccountMappingV2 { version: &amp;'static str, #[serde(flatten)] checkpoint: MappingCheckpointV2, provenance: &amp;'static str, layerx_receipt: bool, }</code> |

## fmt-migrate-chain-checkpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-chain-checkpoint</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: pub(crate) struct ChainCheckpoint { pub(crate) height: u64, pub(crate) hash: [u8; 32], pub(crate) parent_hash: [u8; 32], pub(crate) previous_height: u64, pub(crate) previous_hash: [u8; 32], }</code> |

## fmt-migrate-claim-checkpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-claim-checkpoint</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: struct ClaimCheckpoint { height: u64, block_hash: [u8; 32], evidence_digest: [u8; 32], }</code> |

## fmt-migrate-funded-migration-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-funded-migration-observation</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>Rust model: pub struct FundedMigrationObservation { state: &amp;'static str, producer_state: String, order_digest: [u8; 32], operation: String, source_evidence_digest: [u8; 32], source_claim_id: Option&lt;[u8; 32]&gt;, provenance: &amp;'static str, layerx_receipt: bool, layerx_credit_verified: bool, }</code> |

## fmt-migrate-head-body

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-head-body</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: struct HeadBody { sequence: u64, digest: [u8; 32], }</code> |

## fmt-migrate-head-seal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-head-seal</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: struct HeadSeal { body: HeadBody, mac: [u8; 32], }</code> |

## fmt-migrate-history-checkpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-history-checkpoint</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: struct HistoryCheckpoint { previous_cursor: Option&lt;[u8; 32]&gt;, previous_anchor_hash: Option&lt;[u8; 32]&gt;, from: u64, to: u64, anchor_hash: [u8; 32], evidence_digest: [u8; 32], next_cursor: [u8; 32], committed: bool, }</code> |

## fmt-migrate-mapping-checkpoint-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-mapping-checkpoint-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct MappingCheckpointV2 { principal: String, source_network: String, source_address: String, layerx_identity: [u8; 32], evm_address: [u8; 20], paxeer_block_number: u64, paxeer_block_hash: [u8; 32], evidence_digest: [u8; 32], }</code> |

## fmt-migrate-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-record</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: struct Record { body: RecordBody, mac: [u8; 32], }</code> |

## fmt-migrate-record-body

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-record-body</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Rust model: struct RecordBody { sequence: u64, previous: [u8; 32], update: Update, }</code> |

## fmt-migrate-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-settlement</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct Settlement { #[serde(rename = "source_settlement")] source: SourceSettlementResponseV2, provenance: String, custody_label: String, layerx_receipt: bool, }</code> |

## fmt-migrate-source-settlement-request-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-source-settlement-request-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/ramp_v2.rs](../../../services/interop/crates/layerx-migrate/src/ramp_v2.rs) | <code>Rust model: pub struct SourceSettlementRequestV2 { version: &amp;'static str, order_digest: [u8; 32], chain: String, source_evidence: String, }</code> |

## fmt-migrate-source-settlement-response-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-source-settlement-response-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/ramp_v2.rs](../../../services/interop/crates/layerx-migrate/src/ramp_v2.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct SourceSettlementResponseV2 { pub version: String, pub order_digest: [u8; 32], pub state: String, pub source_evidence_digest: [u8; 32], pub source_claim_id: Option&lt;[u8; 32]&gt;, }</code> |

## fmt-migrate-success

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-success</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct Success { ok: bool, operation: String, result: Settlement, trace: String, }</code> |

## fmt-migrate-update

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-migrate-update</code> | <code>file_format</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>Update = chain &#124; claim &#124; history-prepared &#124; history-committed &#124; ownership &#124; account-mapping-v2 &#124; custody-reference &#124; external-history (tag kind)</code> |

## migrate-account-mapping-store-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>pub struct AccountMappingStoreV2</code> |

## migrate-account-mapping-store-v2-confirm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2-confirm</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>pub fn confirm(&amp;self, principal: &amp;PrincipalId, authenticated_identity: [u8; 32], evidence: &amp;crate::SourceEvidence, verifier: &amp;impl crate::SourceVerifier, bindings: &amp;crate::mapping_v2::PaxeerBindingVerifierV2, trace: &amp;layerx_interop_gateway::trace::TraceId) -&gt; Result&lt;AccountMappingV2, MigrationError&gt;</code> |

## migrate-account-mapping-store-v2-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-store-v2-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>pub fn new(config: &amp;JournalConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-account-mapping-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-account-mapping-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>pub struct AccountMappingV2</code> |

## migrate-binding-execution

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-binding-execution</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct BindingExecution&lt;'a&gt;</code> |

## migrate-binding-execution-idempotency-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-binding-execution-idempotency-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn idempotency_key(&amp;self) -&gt; [u8; 32]</code> |

## migrate-binding-execution-ownership

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-binding-execution-ownership</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn ownership(&amp;self) -&gt; &amp;VerifiedOwnership</code> |

## migrate-binding-receipt-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct BindingReceiptPolicy</code> |

## migrate-binding-receipt-policy-context-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy-context-hash</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn context_hash(ownership: &amp;VerifiedOwnership) -&gt; [u8; 32]</code> |

## migrate-binding-receipt-policy-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-binding-receipt-policy-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn new(sequencer_public_key: [u8; 32], authority: [u8; 32], module_id: u16, module_version: u32, parameter_version: u32, operation: u8, asset: [u8; 32], amount: u128) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-confirmed-paxeer-binding-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub struct ConfirmedPaxeerBindingV2</code> |

## migrate-confirmed-paxeer-binding-v2-block-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-block-hash</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub const fn block_hash(&amp;self) -&gt; [u8; 32]</code> |

## migrate-confirmed-paxeer-binding-v2-block-number

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-block-number</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub const fn block_number(&amp;self) -&gt; u64</code> |

## migrate-confirmed-paxeer-binding-v2-evm-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-evm-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub const fn evm_address(&amp;self) -&gt; [u8; 20]</code> |

## migrate-confirmed-paxeer-binding-v2-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-confirmed-paxeer-binding-v2-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub const fn identity(&amp;self) -&gt; [u8; 32]</code> |

## migrate-custody-execution

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-custody-execution</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct CustodyExecution&lt;'a&gt;</code> |

## migrate-custody-execution-finality

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-custody-execution-finality</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn finality(&amp;self) -&gt; &amp;VerifiedAssetFinality</code> |

## migrate-custody-execution-idempotency-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-custody-execution-idempotency-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn idempotency_key(&amp;self) -&gt; [u8; 32]</code> |

## migrate-custody-receipt-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct CustodyReceiptPolicy</code> |

## migrate-custody-receipt-policy-context-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy-context-hash</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn context_hash(finality: &amp;VerifiedAssetFinality) -&gt; [u8; 32]</code> |

## migrate-custody-receipt-policy-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-custody-receipt-policy-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn new(sequencer_public_key: [u8; 32], custody_authority: [u8; 32], module_id: u16, module_version: u32, parameter_version: u32, operation: u8) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-durable-external-history

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/history.rs](../../../services/interop/crates/layerx-migrate/src/history.rs) | <code>pub struct DurableExternalHistory</code> |

## migrate-durable-external-history-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/history.rs](../../../services/interop/crates/layerx-migrate/src/history.rs) | <code>pub fn new(config: &amp;JournalConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-durable-external-history-read

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-durable-external-history-read</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/history.rs](../../../services/interop/crates/layerx-migrate/src/history.rs) | <code>pub fn read(&amp;self, principal: &amp;PrincipalId, after: Option&lt;[u8; 32]&gt;, limit: usize) -&gt; Result&lt;ExternalHistoryPage, MigrationError&gt;</code> |

## migrate-ethereum-asset-claim

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-asset-claim</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub struct EthereumAssetClaim</code> |

## migrate-ethereum-asset-claim-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-asset-claim-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub fn evidence(self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

## migrate-ethereum-claim-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-claim-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub fn ethereum_claim_digest(claim: EthereumAssetClaim) -&gt; [u8; 32]</code> |

## migrate-ethereum-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub struct EthereumConfig</code> |

## migrate-ethereum-contract-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-contract-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub enum EthereumContractIdentity</code> |

## migrate-ethereum-custody-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-custody-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub struct EthereumCustodySchema</code> |

## migrate-ethereum-finality-tag

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-finality-tag</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub enum EthereumFinalityTag</code> |

## migrate-ethereum-history-claim

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-history-claim</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub struct EthereumHistoryClaim</code> |

## migrate-ethereum-history-claim-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-history-claim-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub fn evidence(self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

## migrate-ethereum-ownership-claim

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub struct EthereumOwnershipClaim</code> |

## migrate-ethereum-ownership-claim-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub fn evidence(&amp;self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

## migrate-ethereum-ownership-claim-signing-message

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-ownership-claim-signing-message</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub fn signing_message(&amp;self) -&gt; Vec&lt;u8&gt;</code> |

## migrate-ethereum-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub struct EthereumVerifier</code> |

## migrate-ethereum-verifier-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub fn new(config: EthereumConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-ethereum-verifier-verify-owned-history

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-verifier-verify-owned-history</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub fn verify_owned_history(&amp;self, ownership_evidence: &amp;SourceEvidence, history_evidence: &amp;SourceEvidence, authenticated_identity: [u8; 32], trace: &amp;TraceId) -&gt; Result&lt;VerifiedHistoryPage, MigrationError&gt;</code> |

## migrate-ethereum-word

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-ethereum-word</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ethereum.rs](../../../services/interop/crates/layerx-migrate/src/ethereum.rs) | <code>pub enum EthereumWord</code> |

## migrate-external-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub enum ExternalAddress</code> |

## migrate-external-history-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub enum ExternalHistoryKind</code> |

## migrate-external-history-page

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-page</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/history.rs](../../../services/interop/crates/layerx-migrate/src/history.rs) | <code>pub struct ExternalHistoryPage</code> |

## migrate-external-history-page-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-json</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/history.rs](../../../services/interop/crates/layerx-migrate/src/history.rs) | <code>pub fn json(&amp;self) -&gt; Value</code> |

## migrate-external-history-page-next-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-next-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/history.rs](../../../services/interop/crates/layerx-migrate/src/history.rs) | <code>pub const fn next_cursor(&amp;self) -&gt; Option&lt;[u8; 32]&gt;</code> |

## migrate-external-history-page-records

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-page-records</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/history.rs](../../../services/interop/crates/layerx-migrate/src/history.rs) | <code>pub fn records(&amp;self) -&gt; &amp;[ExternalHistoryRecord]</code> |

## migrate-external-history-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct ExternalHistoryRecord</code> |

## migrate-external-history-record-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn address(&amp;self) -&gt; ExternalAddress</code> |

## migrate-external-history-record-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn chain(&amp;self) -&gt; SourceChain</code> |

## migrate-external-history-record-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn kind(&amp;self) -&gt; ExternalHistoryKind</code> |

## migrate-external-history-record-provenance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-provenance</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn provenance(&amp;self) -&gt; ExternalProvenance</code> |

## migrate-external-history-record-source-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-source-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn source_amount(&amp;self) -&gt; u128</code> |

## migrate-external-history-record-source-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-source-asset</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn source_asset(&amp;self) -&gt; [u8; 32]</code> |

## migrate-external-history-record-timestamp

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-timestamp</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn timestamp(&amp;self) -&gt; u64</code> |

## migrate-external-history-record-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-record-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn transaction(&amp;self) -&gt; SourceTransaction</code> |

## migrate-external-history-sink

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-history-sink</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub trait ExternalHistorySink</code> |

## migrate-external-provenance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-external-provenance</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub enum ExternalProvenance</code> |

## migrate-funded-migration-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-funded-migration-observation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>pub struct FundedMigrationObservation</code> |

## migrate-gateway-client

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-gateway-client</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>pub struct GatewayClient</code> |

## migrate-gateway-client-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>pub struct GatewayClientConfig</code> |

## migrate-gateway-client-migrate-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-migrate-asset</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>pub fn migrate_asset(&amp;self, order_digest: [u8; 32], chain: &amp;str, evidence: &amp;SourceEvidence, idempotency: &amp;str, trace: &amp;TraceId) -&gt; Result&lt;FundedMigrationObservation, MigrationError&gt;</code> |

## migrate-gateway-client-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-gateway-client-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/gateway_client.rs](../../../services/interop/crates/layerx-migrate/src/gateway_client.rs) | <code>pub fn new(config: &amp;GatewayClientConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-interop-migrate-ethereum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-interop-migrate-ethereum</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn interop_migrate_ethereum() -&gt; &amp;'static str</code> |

## migrate-interop-migrate-solana

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-interop-migrate-solana</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn interop_migrate_solana() -&gt; &amp;'static str</code> |

## migrate-journal-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-journal-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/journal.rs](../../../services/interop/crates/layerx-migrate/src/journal.rs) | <code>pub struct JournalConfig</code> |

## migrate-migration-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct MigrationAdapter</code> |

## migrate-migration-adapter-descriptor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-descriptor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn migration_adapter_descriptor(spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, MigrationError&gt;</code> |

## migrate-migration-adapter-import-history

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-import-history</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn import_history(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, evidence: &amp;SourceEvidence, verifier: &amp;impl SourceVerifier, sink: &amp;mut impl ExternalHistorySink, trace: &amp;TraceId, now: u64) -&gt; Result&lt;MigrationState, Traced&lt;MigrationError&gt;&gt;</code> |

## migrate-migration-adapter-map-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-map-account</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn map_account(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, evidence: &amp;SourceEvidence, verifier: &amp;impl SourceVerifier, plane: &amp;mut impl MigrationPlane, binding_policy: &amp;BindingReceiptPolicy, trace: &amp;TraceId, now: u64) -&gt; Result&lt;MigrationState, Traced&lt;MigrationError&gt;&gt;</code> |

## migrate-migration-adapter-migrate-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-adapter-migrate-asset</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn migrate_asset(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, evidence: &amp;SourceEvidence, verifier: &amp;impl SourceVerifier, plane: &amp;mut impl MigrationPlane, receipt_policy: &amp;CustodyReceiptPolicy, trace: &amp;TraceId, now: u64) -&gt; Result&lt;MigrationState, Traced&lt;MigrationError&gt;&gt;</code> |

## migrate-migration-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub enum MigrationError</code> |

## migrate-migration-error-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-error-code</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

## migrate-migration-plane

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-plane</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub trait MigrationPlane</code> |

## migrate-migration-plane-result

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-plane-result</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub enum MigrationPlaneResult</code> |

## migrate-migration-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-migration-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub enum MigrationState</code> |

## migrate-paxeer-binding-config-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-config-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub struct PaxeerBindingConfigV2</code> |

## migrate-paxeer-binding-verifier-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub struct PaxeerBindingVerifierV2</code> |

## migrate-paxeer-binding-verifier-v2-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub fn new(config: PaxeerBindingConfigV2) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-paxeer-binding-verifier-v2-verify-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-paxeer-binding-verifier-v2-verify-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/mapping_v2.rs](../../../services/interop/crates/layerx-migrate/src/mapping_v2.rs) | <code>pub fn verify_identity(&amp;self, expected_did: [u8; 32]) -&gt; Result&lt;ConfirmedPaxeerBindingV2, MigrationError&gt;</code> |

## migrate-rpc-endpoint-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-rpc-endpoint-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>pub struct RpcEndpointConfig</code> |

## migrate-rpc-quorum-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-rpc-quorum-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/rpc.rs](../../../services/interop/crates/layerx-migrate/src/rpc.rs) | <code>pub struct RpcQuorumConfig</code> |

## migrate-solana-amount-endian

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-amount-endian</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub enum SolanaAmountEndian</code> |

## migrate-solana-asset-claim

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-asset-claim</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub struct SolanaAssetClaim</code> |

## migrate-solana-asset-claim-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-asset-claim-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub fn evidence(self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

## migrate-solana-claim-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-claim-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub fn solana_claim_digest(claim: &amp;SolanaAssetClaim) -&gt; [u8; 32]</code> |

## migrate-solana-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub struct SolanaConfig</code> |

## migrate-solana-history-claim

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-history-claim</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub struct SolanaHistoryClaim</code> |

## migrate-solana-history-claim-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-history-claim-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub fn evidence(self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

## migrate-solana-instruction-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-instruction-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub struct SolanaInstructionSchema</code> |

## migrate-solana-ownership-claim

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub struct SolanaOwnershipClaim</code> |

## migrate-solana-ownership-claim-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub fn evidence(&amp;self) -&gt; Result&lt;SourceEvidence, MigrationError&gt;</code> |

## migrate-solana-ownership-claim-signing-message

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-ownership-claim-signing-message</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub fn signing_message(&amp;self) -&gt; Vec&lt;u8&gt;</code> |

## migrate-solana-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub struct SolanaVerifier</code> |

## migrate-solana-verifier-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub fn new(config: SolanaConfig) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-solana-verifier-verify-owned-history

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-solana-verifier-verify-owned-history</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/solana.rs](../../../services/interop/crates/layerx-migrate/src/solana.rs) | <code>pub fn verify_owned_history(&amp;self, ownership_evidence: &amp;SourceEvidence, history_evidence: &amp;SourceEvidence, authenticated_identity: [u8; 32], trace: &amp;TraceId) -&gt; Result&lt;VerifiedHistoryPage, MigrationError&gt;</code> |

## migrate-source-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub enum SourceChain</code> |

## migrate-source-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct SourceEvidence</code> |

## migrate-source-evidence-canonical

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-canonical</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn canonical(&amp;self) -&gt; &amp;[u8]</code> |

## migrate-source-evidence-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn digest(&amp;self) -&gt; [u8; 32]</code> |

## migrate-source-evidence-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-evidence-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn new(canonical: Vec&lt;u8&gt;) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-source-settlement-request-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-request-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ramp_v2.rs](../../../services/interop/crates/layerx-migrate/src/ramp_v2.rs) | <code>pub struct SourceSettlementRequestV2</code> |

## migrate-source-settlement-request-v2-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-request-v2-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ramp_v2.rs](../../../services/interop/crates/layerx-migrate/src/ramp_v2.rs) | <code>pub fn new(order_digest: [u8; 32], chain: &amp;str, evidence: &amp;SourceEvidence) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-source-settlement-response-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-response-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ramp_v2.rs](../../../services/interop/crates/layerx-migrate/src/ramp_v2.rs) | <code>pub struct SourceSettlementResponseV2</code> |

## migrate-source-settlement-response-v2-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-settlement-response-v2-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/ramp_v2.rs](../../../services/interop/crates/layerx-migrate/src/ramp_v2.rs) | <code>pub fn validate(&amp;self, request: &amp;SourceSettlementRequestV2, evidence: &amp;SourceEvidence) -&gt; Result&lt;(), MigrationError&gt;</code> |

## migrate-source-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub enum SourceTransaction</code> |

## migrate-source-transaction-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn bytes(&amp;self) -&gt; &amp;[u8]</code> |

## migrate-source-transaction-ethereum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-ethereum</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn ethereum(value: [u8; 32]) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-source-transaction-solana

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-transaction-solana</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn solana(value: [u8; 64]) -&gt; Result&lt;Self, MigrationError&gt;</code> |

## migrate-source-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-source-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub trait SourceVerifier: sealed::SourceVerifier</code> |

## migrate-verified-asset-finality

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct VerifiedAssetFinality</code> |

## migrate-verified-asset-finality-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn chain(&amp;self) -&gt; SourceChain</code> |

## migrate-verified-asset-finality-custody-reference

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-custody-reference</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn custody_reference(&amp;self) -&gt; [u8; 32]</code> |

## migrate-verified-asset-finality-destination

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-destination</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn destination(&amp;self) -&gt; [u8; 32]</code> |

## migrate-verified-asset-finality-evidence-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-evidence-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn evidence_digest(&amp;self) -&gt; [u8; 32]</code> |

## migrate-verified-asset-finality-finality-height

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-finality-height</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn finality_height(&amp;self) -&gt; u64</code> |

## migrate-verified-asset-finality-layerx-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-layerx-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn layerx_amount(&amp;self) -&gt; u128</code> |

## migrate-verified-asset-finality-layerx-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-layerx-asset</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn layerx_asset(&amp;self) -&gt; [u8; 32]</code> |

## migrate-verified-asset-finality-source

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn source(&amp;self) -&gt; ExternalAddress</code> |

## migrate-verified-asset-finality-source-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn source_amount(&amp;self) -&gt; u128</code> |

## migrate-verified-asset-finality-source-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-source-asset</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn source_asset(&amp;self) -&gt; [u8; 32]</code> |

## migrate-verified-asset-finality-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-asset-finality-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn transaction(&amp;self) -&gt; SourceTransaction</code> |

## migrate-verified-history-page

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct VerifiedHistoryPage</code> |

## migrate-verified-history-page-evidence-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-evidence-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn evidence_digest(&amp;self) -&gt; [u8; 32]</code> |

## migrate-verified-history-page-next-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-next-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn next_cursor(&amp;self) -&gt; Option&lt;[u8; 32]&gt;</code> |

## migrate-verified-history-page-records

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-history-page-records</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub fn records(&amp;self) -&gt; &amp;[ExternalHistoryRecord]</code> |

## migrate-verified-ownership

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub struct VerifiedOwnership</code> |

## migrate-verified-ownership-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn address(&amp;self) -&gt; ExternalAddress</code> |

## migrate-verified-ownership-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn chain(&amp;self) -&gt; SourceChain</code> |

## migrate-verified-ownership-evidence-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-evidence-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn evidence_digest(&amp;self) -&gt; [u8; 32]</code> |

## migrate-verified-ownership-layerx-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-migrate-verified-ownership-layerx-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-migrate/src/lib.rs](../../../services/interop/crates/layerx-migrate/src/lib.rs) | <code>pub const fn layerx_identity(&amp;self) -&gt; [u8; 32]</code> |
