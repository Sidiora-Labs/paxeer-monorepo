# Interop mirror contract

The [mirror library](../../../services/interop/crates/layerx-mirror/src/lib.rs) acquires committed Paxeer blocks, publishes retrievable archives and verifies mirrored evidence. Committed coordinates identify an execution block by height and block hash. `MirrorVerifier::from_committed_source(observed: ObservedArchive)` admits a committed archive and returns `Result<Self, MirrorVerifyError>`; receipt verification takes independently supplied receipt-key pins.

## cfg-mirror-config-layerx-network-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-config-layerx-network-id</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>mirror-verify config layerx_network_id: u32 (required)</code> |

## cfg-mirror-config-sequencer-id-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-config-sequencer-id-hex</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>mirror-verify config sequencer_id_hex: String (required)</code> |

## cfg-mirror-config-sources

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-config-sources</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>mirror-verify config sources: Vec&lt;SourceConfig&gt; (required)</code> |

## cfg-mirror-ethereum-file-config-archive-code-hash-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-archive-code-hash-hex</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum archive_code_hash_hex: String (required)</code> |

## cfg-mirror-ethereum-file-config-archive-contract-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-archive-contract-hex</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum archive_contract_hex: String (required)</code> |

## cfg-mirror-ethereum-file-config-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum chain_id: u64 (required)</code> |

## cfg-mirror-ethereum-file-config-chunk-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-chunk-bytes</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum chunk_bytes: usize (required)</code> |

## cfg-mirror-ethereum-file-config-genesis-hash-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-genesis-hash-hex</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum genesis_hash_hex: String (required)</code> |

## cfg-mirror-ethereum-file-config-maximum-fee-per-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-fee-per-gas</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum maximum_fee_per_gas: u128 (required)</code> |

## cfg-mirror-ethereum-file-config-maximum-priority-fee-per-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-priority-fee-per-gas</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum maximum_priority_fee_per_gas: u128 (required)</code> |

## cfg-mirror-ethereum-file-config-maximum-reorg-depth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-reorg-depth</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum maximum_reorg_depth: u64 (required)</code> |

## cfg-mirror-ethereum-file-config-required-confirmations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-required-confirmations</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum required_confirmations: u64 (required)</code> |

## cfg-mirror-ethereum-file-config-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-rpc</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum rpc: RpcQuorumConfig (required)</code> |

## cfg-mirror-ethereum-file-config-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-signer</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum signer: SignerFileConfig (required)</code> |

## cfg-mirror-ethereum-file-config-transaction-gas-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-transaction-gas-limit</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime ethereum transaction_gas_limit: u64 (required)</code> |

## cfg-mirror-node-file-config-checkpoint-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-checkpoint-policy</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node checkpoint_policy: crate::node::NativeCheckpointPolicy (required)</code> |

## cfg-mirror-node-file-config-deadline-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-deadline-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node deadline_ms: u64 (required)</code> |

## cfg-mirror-node-file-config-expected-network-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-expected-network-id</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node expected_network_id: u32 (required)</code> |

## cfg-mirror-node-file-config-expected-protocol-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-expected-protocol-version</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node expected_protocol_version: u16 (required)</code> |

## cfg-mirror-node-file-config-maximum-archive-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-archive-bytes</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node maximum_archive_bytes: usize (required)</code> |

## cfg-mirror-node-file-config-maximum-archive-chunks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-archive-chunks</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node maximum_archive_chunks: usize (required)</code> |

## cfg-mirror-node-file-config-maximum-connections

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-connections</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node maximum_connections: usize (required)</code> |

## cfg-mirror-node-file-config-maximum-frame-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-frame-bytes</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node maximum_frame_bytes: usize (required)</code> |

## cfg-mirror-node-file-config-maximum-queued-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-queued-bytes</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node maximum_queued_bytes: usize (required)</code> |

## cfg-mirror-node-file-config-maximum-streams

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-streams</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node maximum_streams: usize (required)</code> |

## cfg-mirror-node-file-config-socket

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-socket</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime node socket: PathBuf (required)</code> |

## cfg-mirror-rpc-endpoint-config-bearer-token-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-bearer-token-file</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc endpoint bearer_token_file: PathBuf (required)</code> |

## cfg-mirror-rpc-endpoint-config-ca-certificate-der

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-ca-certificate-der</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc endpoint ca_certificate_der: PathBuf (required)</code> |

## cfg-mirror-rpc-endpoint-config-independent-backend

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-independent-backend</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc endpoint independent_backend: String (required)</code> |

## cfg-mirror-rpc-endpoint-config-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-url</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc endpoint url: String (required)</code> |

## cfg-mirror-rpc-quorum-config-connect-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-connect-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc quorum connect_timeout_ms: u64 (required)</code> |

## cfg-mirror-rpc-quorum-config-endpoints

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-endpoints</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc quorum endpoints: Vec&lt;RpcEndpointConfig&gt; (required)</code> |

## cfg-mirror-rpc-quorum-config-maximum-response-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-maximum-response-bytes</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc quorum maximum_response_bytes: usize (required)</code> |

## cfg-mirror-rpc-quorum-config-quorum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-quorum</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc quorum quorum: usize (required)</code> |

## cfg-mirror-rpc-quorum-config-request-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-request-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>mirror rpc quorum request_timeout_ms: u64 (required)</code> |

## cfg-mirror-runtime-config-checkpoint-freshness-budget-batches

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-checkpoint-freshness-budget-batches</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config checkpoint_freshness_budget_batches: u64 (required)</code> |

## cfg-mirror-runtime-config-ethereum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-ethereum</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config ethereum: EthereumFileConfig (required)</code> |

## cfg-mirror-runtime-config-first-batch-number

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-first-batch-number</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config first_batch_number: u64 (required)</code> |

## cfg-mirror-runtime-config-node

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-node</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config node: NodeFileConfig (required)</code> |

## cfg-mirror-runtime-config-poll-interval-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-poll-interval-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config poll_interval_ms: u64 (required)</code> |

## cfg-mirror-runtime-config-solana

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-solana</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config solana: Option&lt;SolanaFileConfig&gt; (optional)</code> |

## cfg-mirror-runtime-config-state-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-state-directory</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config state_directory: PathBuf (required)</code> |

## cfg-mirror-runtime-config-status-listen

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-status-listen</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config status_listen: SocketAddr (required)</code> |

## cfg-mirror-signer-file-config-key-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-key-handle</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime signer key_handle: String (required)</code> |

## cfg-mirror-signer-file-config-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-public-key</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime signer public_key: String (required)</code> |

## cfg-mirror-signer-file-config-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime signer timeout_ms: u64 (required)</code> |

## cfg-mirror-signer-file-config-transport

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-transport</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime signer transport: SignerTransportFileConfig (required)</code> |

## cfg-mirror-signer-transport-file-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-transport-file-config</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime signer transport: one of "uds" &#124; "mutual_tls" (tag kind)</code> |

## cfg-mirror-solana-file-config-archive-program-base58

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-archive-program-base58</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana archive_program_base58: String (required)</code> |

## cfg-mirror-solana-file-config-chunk-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-chunk-bytes</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana chunk_bytes: usize (required)</code> |

## cfg-mirror-solana-file-config-genesis-hash-base58

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-genesis-hash-base58</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana genesis_hash_base58: String (required)</code> |

## cfg-mirror-solana-file-config-maximum-ancestry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-maximum-ancestry</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana maximum_ancestry: u64 (required)</code> |

## cfg-mirror-solana-file-config-program-code-hash-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-program-code-hash-hex</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana program_code_hash_hex: String (required)</code> |

## cfg-mirror-solana-file-config-program-data-account-base58

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-program-data-account-base58</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana program_data_account_base58: String (required)</code> |

## cfg-mirror-solana-file-config-required-rooted-slots

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-required-rooted-slots</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana required_rooted_slots: u64 (required)</code> |

## cfg-mirror-solana-file-config-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-rpc</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana rpc: RpcQuorumConfig (required)</code> |

## cfg-mirror-solana-file-config-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-signer</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana signer: SignerFileConfig (required)</code> |

## cfg-mirror-solana-file-config-upgradeable-loader-base58

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-upgradeable-loader-base58</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime solana upgradeable_loader_base58: String (required)</code> |

## cfg-mirror-source-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-source-config</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>mirror-verify source: one of "ethereum" &#124; "solana" (tag kind)</code> |

## cli-mirror-publisher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-mirror-publisher</code> | <code>cli_command</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-publisher.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-publisher.rs) | <code>layerx-mirror-publisher &lt;config.json&gt; &#124; --probe</code> |

## cli-mirror-verify

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-mirror-verify</code> | <code>cli_command</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>layerx-mirror-verify &lt;request.json&gt;</code> |

## err-mirror-checkpoint-mirror-archive-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-archive-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>ReconcileRefusal::ArchiveUnavailable.code() = "committed_mirror_archive_unavailable"</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-mirror-chain-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-chain-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>ReconcileRefusal::ChainUnavailable.code() = "committed_mirror_chain_unavailable"</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-mirror-commitment-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-commitment-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>ReconcileRefusal::CommitmentMismatch.code() = "committed_mirror_commitment_mismatch"</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-mirror-malformed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-malformed</code> | <code>error_code</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>ReconcileRefusal::Malformed.code() = "committed_mirror_malformed"</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-mirror-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>ReconcileRefusal::CommittedMismatch.code() = "committed_mirror_mismatch"</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-mirror-missing

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-missing</code> | <code>error_code</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>ReconcileRefusal::Missing.code() = "committed_mirror_missing"</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## fmt-mirror-archive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-archive</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>mirror archive = "LXMIRROR" &#124; archive_version u16be=4 &#124; network_id u32be &#124; height u64be &#124; block_hash:32 &#124; parent_hash:32 &#124; app_hash:32 &#124; timestamp u64be &#124; receipt records; at most 64 MiB and 1048576 records; commitment domain "LXP/mirror/archive/v4\0"</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## fmt-mirror-candidate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-candidate</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub(crate) struct Candidate { pub(crate) source: usize, pub(crate) commitment_hex: String, }</code> |

## fmt-mirror-component-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-component-status</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>Rust model: struct ComponentStatus { ready: bool, #[serde(rename = "latest_height_mirrored")] latest_batch_mirrored: Option&lt;u64&gt;, #[serde(rename = "latest_height_observed")] latest_batch_observed: Option&lt;u64&gt;, #[serde(rename = "latest_committed_height_mirrored")] latest_checkpoint_batch_mirrored: Option&lt;u64&gt;, #[serde(rename = "latest_committed_block_hash_mirrored")] latest_checkpoint_id_mirrored: Option&lt;String&gt;, #[serde(rename = "latest_height_verified")] latest_batch_verified: Option&lt;u64&gt;, #[serde(rename = "latest_committed_height_verified")] latest_checkpoint_batch_verified: Option&lt;u64&gt;, #[serde(rename = "committed_block_lag")] checkpoint_batch_lag: Option&lt;u64&gt;, #[serde(rename = "freshness_budget_blocks")] freshness_budget_batches: u64, freshness: Option&lt;&amp;'static str&gt;, #[serde(rename = "committed_within_budget")] checkpoint_within_budget: bool, phase: Option&lt;&amp;'static str&gt;, error_class: Option&lt;&amp;'static str&gt;, reorgs_observed: u64, }</code> |

## fmt-mirror-evidence-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-evidence-request</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>EvidenceRequest = receipt { index: usize } &#124; committed {} (tag kind)</code> |

## fmt-mirror-policy-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-policy-request</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>PolicyRequest = exact &#124; ordered-preference &#124; agreement (tag kind)</code> |

## fmt-mirror-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-request</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs](../../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub(crate) struct Request { pub(crate) batch_number: String, pub(crate) evidence: EvidenceRequest, pub(crate) policy: PolicyRequest, }</code> |

## fmt-mirror-runtime-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-runtime-status</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>Rust model: struct RuntimeStatus { source_chain_id: u64, status_revision: u16, native_network_id: Option&lt;u32&gt;, service_version: String, node: ComponentStatus, ethereum: ComponentStatus, solana: Option&lt;ComponentStatus&gt;, committed_ready: bool, latest_committed_height: Option&lt;u64&gt;, latest_committed_block_hash: Option&lt;String&gt;, committed_refusal: Option&lt;&amp;'static str&gt;, #[serde(rename = "committed_freshness_budget_blocks")] checkpoint_freshness_budget_batches: u64, }</code> |

## fmt-mirror-store-head

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-store-head</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>journal head file = "LXMHED03" &#124; sequence u64be &#124; offset u64be &#124; digest 32 (56 bytes, written atomically)</code> |

## fmt-mirror-store-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-store-journal</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>publication journal record = "LXMJNL03" &#124; version u16be=3 &#124; chain tag u8 &#124; height u64be &#124; commitment 32 &#124; optional committed coordinate (tag 0, or tag 1 + height u64be + block hash 32) &#124; stage u8 (1 manifest, 2 chunk + index u32be, 3 finalize) &#124; phase u8 &#124; stage payload digest 32 &#124; signed payload (u32be length, &lt;= 384 KiB) &#124; transaction &#124; position; record &lt;= 512 KiB</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## fmt-mirror-store-spool

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-store-spool</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>archive spool = "LXMSPL03" &#124; sealed height u64be &#124; optional committed coordinate (tag 0, or tag 1 + height u64be + block hash 32) &#124; archive bytes (u32be length, &lt;= 64 MiB) &#124; sha256 of the preceding bytes</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-archive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct Archive</code> |

## mirror-archive-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn bytes(&amp;self) -&gt; &amp;[u8]</code> |

## mirror-archive-commitment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct ArchiveCommitment([u8; 32])</code> |

## mirror-archive-commitment-as-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment-as-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn as_bytes(&amp;self) -&gt; &amp;[u8; 32]</code> |

## mirror-archive-commitment-from-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment-from-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn from_bytes(bytes: [u8; 32]) -&gt; Self</code> |

## mirror-archive-data

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-data</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct ArchiveData</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-archive-data-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-data-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Result&lt;Self, ArchiveError&gt;</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-archive-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum ArchiveError</code> |

## mirror-archive-node-head

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-node-head</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn node_head(&amp;self) -&gt; NodeHead</code> |

## mirror-archive-spool

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub struct ArchiveSpool</code> |

## mirror-archive-spool-commitments

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-commitments</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn commitments(&amp;self) -&gt; Result&lt;Vec&lt;ArchiveCommitment&gt;, StoreError&gt;</code> |

## mirror-archive-spool-get

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-get</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn get(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;SpooledArchive, StoreError&gt;</code> |

## mirror-archive-spool-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn open(directory: impl Into&lt;PathBuf&gt;) -&gt; Result&lt;Self, StoreError&gt;</code> |

## mirror-archive-spool-put

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-put</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn put(&amp;self, commitment: ArchiveCommitment, bytes: &amp;[u8], node_head: NodeHead) -&gt; Result&lt;(), StoreError&gt;</code> |

## mirror-broadcast-result

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-broadcast-result</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>pub enum BroadcastResult</code> |

## mirror-chain-failure

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-chain-failure</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum ChainFailure</code> |

## mirror-chain-position

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-chain-position</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum ChainPosition</code> |

## mirror-chain-progress

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-chain-progress</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct ChainProgress</code> |

## mirror-chain-signature

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-chain-signature</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub enum ChainSignature</code> |

## mirror-config-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-config-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum ConfigError</code> |

## mirror-durable-publication-report

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-durable-publication-report</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct DurablePublicationReport</code> |

## mirror-ethereum-archive-client

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub struct EthereumArchiveClient</code> |

## mirror-ethereum-archive-client-advance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-advance</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn advance(&amp;mut self, archive: &amp;Archive) -&gt; Result&lt;EthereumProgress, EthereumError&gt;</code> |

## mirror-ethereum-archive-client-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn cursor(&amp;self) -&gt; MirrorCursor</code> |

## mirror-ethereum-archive-client-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn open(config: EthereumProductionConfig, signer: RemoteChainSigner) -&gt; Result&lt;Self, EthereumError&gt;</code> |

## mirror-ethereum-archive-client-retrieve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-retrieve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, EthereumError&gt;</code> |

## mirror-ethereum-archive-client-retrieve-finalized

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-retrieve-finalized</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn retrieve_finalized(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, EthereumError&gt;</code> |

## mirror-ethereum-archive-write

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-write</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct EthereumArchiveWrite&lt;'a&gt;</code> |

## mirror-ethereum-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct EthereumConfig</code> |

## mirror-ethereum-config-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-config-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn validate(self) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## mirror-ethereum-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub enum EthereumError</code> |

## mirror-ethereum-file-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-file-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>pub struct EthereumFileConfig</code> |

## mirror-ethereum-lane-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-lane-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum EthereumLaneState</code> |

## mirror-ethereum-mirror-read

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-read</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub struct EthereumMirrorRead</code> |

## mirror-ethereum-mirror-read-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-read-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub struct EthereumMirrorReadConfig</code> |

## mirror-ethereum-mirror-reader

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub struct EthereumMirrorReader</code> |

## mirror-ethereum-mirror-reader-is-canonical

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-is-canonical</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn is_canonical(&amp;self, observation: &amp;EthereumMirrorRead) -&gt; Result&lt;bool, EthereumError&gt;</code> |

## mirror-ethereum-mirror-reader-is-coordinate-canonical

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-is-coordinate-canonical</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn is_coordinate_canonical(&amp;self, block_number: u64, block_hash: [u8; 32]) -&gt; Result&lt;bool, EthereumError&gt;</code> |

## mirror-ethereum-mirror-reader-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn open(config: EthereumMirrorReadConfig) -&gt; Result&lt;Self, EthereumError&gt;</code> |

## mirror-ethereum-mirror-reader-retrieve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-retrieve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;EthereumMirrorRead&gt;, EthereumError&gt;</code> |

## mirror-ethereum-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-observation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum EthereumObservation</code> |

## mirror-ethereum-production-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-production-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub struct EthereumProductionConfig</code> |

## mirror-ethereum-progress

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-progress</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/ethereum.rs](../../../services/interop/crates/layerx-mirror/src/ethereum.rs) | <code>pub struct EthereumProgress</code> |

## mirror-ethereum-submission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-submission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct EthereumSubmission</code> |

## mirror-finality-position

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-finality-position</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub enum FinalityPosition</code> |

## mirror-generic-publisher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct GenericPublisher&lt;E, S&gt;</code> |

## mirror-generic-publisher-ethereum-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-ethereum-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn ethereum_cursor(&amp;self) -&gt; MirrorCursor</code> |

## mirror-generic-publisher-into-clients

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-into-clients</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn into_clients(self) -&gt; (E, S)</code> |

## mirror-generic-publisher-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn new(ethereum_config: EthereumConfig, ethereum_client: E, solana_config: SolanaConfig, solana_client: S) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## mirror-generic-publisher-publish

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-publish</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn publish(&amp;mut self, archive: &amp;Archive) -&gt; PublicationReport</code> |

## mirror-generic-publisher-retrieve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-retrieve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn retrieve(&amp;mut self, commitment: ArchiveCommitment) -&gt; RetrievalReport</code> |

## mirror-generic-publisher-solana-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-solana-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn solana_cursor(&amp;self) -&gt; MirrorCursor</code> |

## mirror-max-mirror-sources

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-max-mirror-sources</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub const MAX_MIRROR_SOURCES: usize</code> |

## mirror-max-source-id-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-max-source-id-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub const MAX_SOURCE_ID_BYTES: usize</code> |

## mirror-mirror-canonical-position

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-canonical-position</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub enum MirrorCanonicalPosition</code> |

## mirror-mirror-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub enum MirrorChain</code> |

## mirror-mirror-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct MirrorCursor</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-mirror-degradation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-degradation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum MirrorDegradation</code> |

## mirror-mirror-evidence-level

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-evidence-level</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub enum MirrorEvidenceLevel</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-mirror-freshness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-freshness</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct MirrorFreshness</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-mirror-lag

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-lag</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub enum MirrorLag</code> |

## mirror-mirror-locator

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-locator</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub struct MirrorLocator</code> |

## mirror-mirror-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-observation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub struct MirrorObservation</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-mirror-provenance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-provenance</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub enum MirrorProvenance</code> |

## mirror-mirror-read-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-read-policy</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub enum MirrorReadPolicy</code> |

## mirror-mirror-retrieval

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-retrieval</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub type MirrorRetrieval&lt;E&gt; = Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, E&gt;</code> |

## mirror-mirror-source

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub struct MirrorSource</code> |

## mirror-mirror-source-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub enum MirrorSourceError</code> |

## mirror-mirror-source-ethereum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-ethereum</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn ethereum(id: MirrorSourceId, config: EthereumMirrorReadConfig) -&gt; Result&lt;Self, MirrorSourceError&gt;</code> |

## mirror-mirror-source-freshness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-freshness</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub struct MirrorSourceFreshness</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-mirror-source-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub struct MirrorSourceId(String)</code> |

## mirror-mirror-source-id-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## mirror-mirror-source-id-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, MirrorSourceError&gt;</code> |

## mirror-mirror-source-solana

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-solana</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn solana(id: MirrorSourceId, config: SolanaMirrorReadConfig) -&gt; Result&lt;Self, MirrorSourceError&gt;</code> |

## mirror-mirror-sources

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub struct MirrorSources</code> |

## mirror-mirror-sources-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn new(layerx_network_id: u32, sources: Vec&lt;MirrorSource&gt;) -&gt; Result&lt;Self, MirrorSourceError&gt;</code> |

## mirror-mirror-sources-read

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-read</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn read(&amp;self, batch_number: u64, policy: &amp;MirrorReadPolicy) -&gt; Result&lt;ObservedArchive, MirrorSourceError&gt;</code> |

## mirror-mirror-sources-recheck

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-recheck</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn recheck(&amp;self, mut archive: ObservedArchive) -&gt; Result&lt;ObservedArchive, MirrorSourceError&gt;</code> |

## mirror-mirror-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum MirrorState</code> |

## mirror-mirror-state-freshness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-state-freshness</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn freshness(&amp;self) -&gt; MirrorFreshness</code> |

## mirror-mirror-target-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-target-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub enum MirrorTargetIdentity</code> |

## mirror-mirror-verification

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub struct MirrorVerification&lt;T&gt;</code> |

## mirror-mirror-verification-batch-number

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-batch-number</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub const fn batch_number(&amp;self) -&gt; u64</code> |

## mirror-mirror-verification-freshness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub struct MirrorVerificationFreshness</code> |

## mirror-mirror-verification-freshness-from-source

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-from-source</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub const fn from_source(value: MirrorSourceFreshness) -&gt; Self</code> |

## mirror-mirror-verification-freshness-offline

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-offline</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub const fn offline( latest_batch_mirrored: u64, latest_committed_mirrored: Option&lt;CommittedCoordinate&gt;, ) -&gt; Self</code> |

## mirror-mirror-verification-freshness-relative

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-relative</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub fn relative(value: MirrorFreshness) -&gt; Self</code> |

## mirror-mirror-verification-level

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-level</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub const fn level(&amp;self) -&gt; MirrorEvidenceLevel</code> |

## mirror-mirror-verification-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-observation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub fn observation(&amp;self) -&gt; Option&lt;&amp;MirrorObservation&gt;</code> |

## mirror-mirror-verification-value

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-value</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub const fn value(&amp;self) -&gt; &amp;T</code> |

## mirror-mirror-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub struct MirrorVerifier</code> |

## mirror-mirror-verify-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verify-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub enum MirrorVerifyError</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-node-head

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-node-head</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct NodeHead</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-observed-archive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-observed-archive</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub struct ObservedArchive</code> |

## mirror-observed-archive-archive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-observed-archive-archive</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn archive(&amp;self) -&gt; &amp;[u8]</code> |

## mirror-observed-archive-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-observed-archive-observation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/source.rs](../../../services/interop/crates/layerx-mirror/src/source.rs) | <code>pub fn observation(&amp;self) -&gt; &amp;MirrorObservation</code> |

## mirror-progress-gate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-progress-gate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct ProgressGate</code> |

## mirror-progress-gate-acknowledge

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-progress-gate-acknowledge</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn acknowledge&lt;E&gt;(&amp;mut self, journal: MirrorCursor, archives: &amp;[Archive], retrieve: impl FnOnce(ArchiveCommitment) -&gt; MirrorRetrieval&lt;E&gt;) -&gt; ChainProgress</code> |

## mirror-progress-gate-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-progress-gate-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn new() -&gt; Self</code> |

## mirror-publication-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum PublicationId</code> |

## mirror-publication-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub struct PublicationJournal</code> |

## mirror-publication-journal-append

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-append</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn append(&amp;mut self, record: PublicationRecord) -&gt; Result&lt;(), StoreError&gt;</code> |

## mirror-publication-journal-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn cursor(&amp;self) -&gt; MirrorCursor</code> |

## mirror-publication-journal-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn open(directory: impl Into&lt;PathBuf&gt;, chain: MirrorChain, first_batch_number: u64) -&gt; Result&lt;Self, StoreError&gt;</code> |

## mirror-publication-journal-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn record(&amp;self, commitment: ArchiveCommitment, stage: PublicationStage) -&gt; Option&lt;&amp;PublicationRecord&gt;</code> |

## mirror-publication-journal-records

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-records</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn records(&amp;self) -&gt; impl Iterator&lt;Item = &amp;PublicationRecord&gt;</code> |

## mirror-publication-phase

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-phase</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub enum PublicationPhase</code> |

## mirror-publication-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub struct PublicationRecord</code> |

## mirror-publication-record-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-record-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), StoreError&gt;</code> |

## mirror-publication-report

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-report</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct PublicationReport</code> |

## mirror-publication-stage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publication-stage</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub enum PublicationStage</code> |

## mirror-publisher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct Publisher</code> |

## mirror-publisher-archive-commitment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-commitment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn commitment(&amp;self) -&gt; ArchiveCommitment</code> |

## mirror-publisher-archive-commitment-2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-commitment-2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn archive_commitment(bytes: &amp;[u8]) -&gt; ArchiveCommitment</code> |

## mirror-publisher-archive-data

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-data</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn data(&amp;self) -&gt; &amp;ArchiveData</code> |

## mirror-publisher-ethereum-archive-client

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-ethereum-archive-client</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub trait EthereumArchiveClient</code> |

## mirror-publisher-ethereum-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-ethereum-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn ethereum_cursor(&amp;self) -&gt; MirrorCursor</code> |

## mirror-publisher-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn new(ethereum: crate::ethereum::EthereumArchiveClient, solana: crate::solana::SolanaArchiveClient) -&gt; Self</code> |

## mirror-publisher-progress

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-progress</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn progress(&amp;mut self, archives: &amp;[Archive]) -&gt; (ChainProgress, ChainProgress)</code> |

## mirror-publisher-publish

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-publish</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn publish(&amp;mut self, archive: &amp;Archive) -&gt; DurablePublicationReport</code> |

## mirror-publisher-retrieve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-retrieve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; (MirrorRetrieval&lt;crate::ethereum::EthereumError&gt;, MirrorRetrieval&lt;crate::solana::SolanaError&gt;)</code> |

## mirror-publisher-solana-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-solana-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn solana_cursor(&amp;self) -&gt; MirrorCursor</code> |

## mirror-reconcile-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-reconcile-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn reconcile_journal&lt;E&gt;(journal: MirrorCursor, archives: &amp;[Archive], retrieve: impl FnOnce(ArchiveCommitment) -&gt; MirrorRetrieval&lt;E&gt;) -&gt; Result&lt;(), ReconcileRefusal&gt;</code> |

## mirror-reconcile-refusal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-reconcile-refusal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum ReconcileRefusal</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-reconcile-refusal-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-reconcile-refusal-code</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

## mirror-remote-chain-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub struct RemoteChainSigner</code> |

## mirror-remote-chain-signer-ethereum-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-ethereum-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub fn ethereum_address(&amp;self) -&gt; Result&lt;[u8; 20], SignerError&gt;</code> |

## mirror-remote-chain-signer-key-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-key-handle</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub fn key_handle(&amp;self) -&gt; &amp;str</code> |

## mirror-remote-chain-signer-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub fn new(config: RemoteSignerConfig) -&gt; Result&lt;Self, SignerError&gt;</code> |

## mirror-remote-chain-signer-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-public-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub fn public_key(&amp;self) -&gt; &amp;[u8]</code> |

## mirror-remote-chain-signer-sign-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-sign-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub fn sign_digest(&amp;self, policy_domain: &amp;[u8], digest: [u8; 32]) -&gt; Result&lt;ChainSignature, SignerError&gt;</code> |

## mirror-remote-chain-signer-sign-message

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-sign-message</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub fn sign_message(&amp;self, policy_domain: &amp;[u8], message: &amp;[u8]) -&gt; Result&lt;ChainSignature, SignerError&gt;</code> |

## mirror-remote-signer-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-remote-signer-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub struct RemoteSignerConfig</code> |

## mirror-retrieval-report

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-retrieval-report</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct RetrievalReport</code> |

## mirror-retrieval-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-retrieval-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum RetrievalState</code> |

## mirror-rpc-cluster

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>pub struct RpcCluster</code> |

## mirror-rpc-cluster-broadcast

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-broadcast</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>pub fn broadcast(&amp;self, method: &amp;str, parameters: Value, expected_identity: &amp;str) -&gt; Result&lt;BroadcastResult, RpcError&gt;</code> |

## mirror-rpc-cluster-call

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-call</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>pub fn call(&amp;self, method: &amp;str, parameters: Value) -&gt; Result&lt;Value, RpcError&gt;</code> |

## mirror-rpc-cluster-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>pub fn new(config: &amp;RpcQuorumConfig) -&gt; Result&lt;Self, RpcError&gt;</code> |

## mirror-rpc-endpoint-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-endpoint-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>pub struct RpcEndpointConfig</code> |

## mirror-rpc-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>pub enum RpcError</code> |

## mirror-rpc-quorum-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-quorum-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/rpc.rs](../../../services/interop/crates/layerx-mirror/src/rpc.rs) | <code>pub struct RpcQuorumConfig</code> |

## mirror-run

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-run</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>pub fn run(config_path: &amp;Path) -&gt; Result&lt;(), RuntimeError&gt;</code> |

## mirror-runtime-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-runtime-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>pub struct RuntimeConfig</code> |

| Field | Rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-runtime-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-runtime-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>pub enum RuntimeError</code> |

## mirror-signer-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-signer-endpoint</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub enum SignerEndpoint</code> |

## mirror-signer-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-signer-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub enum SignerError</code> |

## mirror-signer-file-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-signer-file-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>pub struct SignerFileConfig</code> |

## mirror-signer-transport-file-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-signer-transport-file-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>pub enum SignerTransportFileConfig</code> |

## mirror-signing-algorithm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-signing-algorithm</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub enum SigningAlgorithm</code> |

## mirror-solana-archive-client

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub trait SolanaArchiveClient</code> |

## mirror-solana-archive-client-advance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-advance</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn advance(&amp;mut self, archive: &amp;Archive) -&gt; Result&lt;SolanaProgress, SolanaError&gt;</code> |

## mirror-solana-archive-client-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn cursor(&amp;self) -&gt; MirrorCursor</code> |

## mirror-solana-archive-client-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn open(config: SolanaProductionConfig, signer: RemoteChainSigner) -&gt; Result&lt;Self, SolanaError&gt;</code> |

## mirror-solana-archive-client-retrieve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-retrieve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, SolanaError&gt;</code> |

## mirror-solana-archive-client-retrieve-finalized

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-retrieve-finalized</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn retrieve_finalized(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, SolanaError&gt;</code> |

## mirror-solana-archive-write

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-write</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct SolanaArchiveWrite&lt;'a&gt;</code> |

## mirror-solana-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct SolanaConfig</code> |

## mirror-solana-config-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-config-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn validate(self) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## mirror-solana-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub enum SolanaError</code> |

## mirror-solana-file-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-file-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>pub struct SolanaFileConfig</code> |

## mirror-solana-lane-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-lane-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum SolanaLaneState</code> |

## mirror-solana-mirror-read

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-read</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub struct SolanaMirrorRead</code> |

## mirror-solana-mirror-read-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-read-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub struct SolanaMirrorReadConfig</code> |

## mirror-solana-mirror-reader

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub struct SolanaMirrorReader</code> |

## mirror-solana-mirror-reader-is-canonical

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-is-canonical</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn is_canonical(&amp;self, observation: &amp;SolanaMirrorRead) -&gt; Result&lt;bool, SolanaError&gt;</code> |

## mirror-solana-mirror-reader-is-coordinate-canonical

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-is-coordinate-canonical</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn is_coordinate_canonical(&amp;self, rooted_slot: u64, rooted_blockhash: [u8; 32]) -&gt; Result&lt;bool, SolanaError&gt;</code> |

## mirror-solana-mirror-reader-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn open(config: SolanaMirrorReadConfig) -&gt; Result&lt;Self, SolanaError&gt;</code> |

## mirror-solana-mirror-reader-retrieve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-retrieve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;SolanaMirrorRead&gt;, SolanaError&gt;</code> |

## mirror-solana-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-observation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum SolanaObservation</code> |

## mirror-solana-production-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-production-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub struct SolanaProductionConfig</code> |

## mirror-solana-progress

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-progress</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub struct SolanaProgress</code> |

## mirror-solana-solana-archive-client

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-solana-archive-client</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/solana.rs](../../../services/interop/crates/layerx-mirror/src/solana.rs) | <code>pub struct SolanaArchiveClient</code> |

## mirror-solana-submission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-solana-submission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct SolanaSubmission</code> |

## mirror-source-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-source-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub enum SourceError</code> |

## mirror-spooled-archive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-spooled-archive</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub struct SpooledArchive</code> |

## mirror-store-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-store-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub enum StoreError</code> |

## mirror-tls-connector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-tls-connector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/signer.rs](../../../services/interop/crates/layerx-mirror/src/signer.rs) | <code>pub fn tls_connector(server_name: &amp;str, trust_anchor: &amp;Path, client_certificate: &amp;Path, client_private_key: &amp;Path) -&gt; Result&lt;SslConnector, SignerError&gt;</code> |

## mirror-transaction-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-transaction-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/store.rs](../../../services/interop/crates/layerx-mirror/src/store.rs) | <code>pub enum TransactionIdentity</code> |

## mirror-verified-cursor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-verified-cursor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn verified_cursor(archives: &amp;[Archive]) -&gt; MirrorCursor</code> |

## mirror-verify-mirror-verification-freshness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-mirror-verify-mirror-verification-freshness</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub const fn freshness(&amp;self) -&gt; MirrorVerificationFreshness</code> |
