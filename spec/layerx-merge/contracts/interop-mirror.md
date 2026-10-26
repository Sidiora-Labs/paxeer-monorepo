# Interop mirror contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cfg-mirror-config-first-batch-number

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-config-first-batch-number</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:28</code> | <code>mirror-verify config first_batch_number: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-config-first-batch-number</code> | notes | field Config.first_batch_number; struct denies unknown fields |

## cfg-mirror-config-last-batch-number

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-config-last-batch-number</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:29</code> | <code>mirror-verify config last_batch_number: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-config-last-batch-number</code> | notes | field Config.last_batch_number; struct denies unknown fields |

## cfg-mirror-config-layerx-network-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-config-layerx-network-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:25</code> | <code>mirror-verify config layerx_network_id: u32 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-config-layerx-network-id</code> | notes | field Config.layerx_network_id; struct denies unknown fields |

## cfg-mirror-config-sequencer-id-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-config-sequencer-id-hex</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:26</code> | <code>mirror-verify config sequencer_id_hex: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-config-sequencer-id-hex</code> | notes | field Config.sequencer_id_hex; struct denies unknown fields |

## cfg-mirror-config-sequencer-public-key-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-config-sequencer-public-key-hex</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:27</code> | <code>mirror-verify config sequencer_public_key_hex: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-config-sequencer-public-key-hex</code> | notes | field Config.sequencer_public_key_hex; struct denies unknown fields |

## cfg-mirror-config-sources

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-config-sources</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:30</code> | <code>mirror-verify config sources: Vec&lt;SourceConfig&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-config-sources</code> | notes | field Config.sources; struct denies unknown fields; 432 reference lines in 119 files; callers lists the first reference in each file |

## cfg-mirror-ethereum-file-config-archive-code-hash-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-archive-code-hash-hex</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:68</code> | <code>mirror runtime ethereum archive_code_hash_hex: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-archive-code-hash-hex</code> | notes | field EthereumFileConfig.archive_code_hash_hex; struct denies unknown fields |

## cfg-mirror-ethereum-file-config-archive-contract-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-archive-contract-hex</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:67</code> | <code>mirror runtime ethereum archive_contract_hex: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-archive-contract-hex</code> | notes | field EthereumFileConfig.archive_contract_hex; struct denies unknown fields |

## cfg-mirror-ethereum-file-config-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:65</code> | <code>mirror runtime ethereum chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-chain-id</code> | notes | field EthereumFileConfig.chain_id; struct denies unknown fields; 312 reference lines in 102 files; callers lists the first reference in each file |

## cfg-mirror-ethereum-file-config-chunk-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-chunk-bytes</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:71</code> | <code>mirror runtime ethereum chunk_bytes: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-chunk-bytes</code> | notes | field EthereumFileConfig.chunk_bytes; struct denies unknown fields |

## cfg-mirror-ethereum-file-config-genesis-hash-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-genesis-hash-hex</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:66</code> | <code>mirror runtime ethereum genesis_hash_hex: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-genesis-hash-hex</code> | notes | field EthereumFileConfig.genesis_hash_hex; struct denies unknown fields |

## cfg-mirror-ethereum-file-config-maximum-fee-per-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-fee-per-gas</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:73</code> | <code>mirror runtime ethereum maximum_fee_per_gas: u128 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-fee-per-gas</code> | notes | field EthereumFileConfig.maximum_fee_per_gas; struct denies unknown fields |

## cfg-mirror-ethereum-file-config-maximum-priority-fee-per-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-priority-fee-per-gas</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:74</code> | <code>mirror runtime ethereum maximum_priority_fee_per_gas: u128 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-priority-fee-per-gas</code> | notes | field EthereumFileConfig.maximum_priority_fee_per_gas; struct denies unknown fields |

## cfg-mirror-ethereum-file-config-maximum-reorg-depth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-reorg-depth</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:70</code> | <code>mirror runtime ethereum maximum_reorg_depth: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-maximum-reorg-depth</code> | notes | field EthereumFileConfig.maximum_reorg_depth; struct denies unknown fields |

## cfg-mirror-ethereum-file-config-required-confirmations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-required-confirmations</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:69</code> | <code>mirror runtime ethereum required_confirmations: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-required-confirmations</code> | notes | field EthereumFileConfig.required_confirmations; struct denies unknown fields |

## cfg-mirror-ethereum-file-config-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:64</code> | <code>mirror runtime ethereum rpc: RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-rpc</code> | notes | field EthereumFileConfig.rpc; struct denies unknown fields; 1208 reference lines in 212 files; callers lists the first reference in each file |

## cfg-mirror-ethereum-file-config-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-signer</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:75</code> | <code>mirror runtime ethereum signer: SignerFileConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-signer</code> | notes | field EthereumFileConfig.signer; struct denies unknown fields; 600 reference lines in 100 files; callers lists the first reference in each file |

## cfg-mirror-ethereum-file-config-transaction-gas-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-transaction-gas-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:72</code> | <code>mirror runtime ethereum transaction_gas_limit: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-ethereum-file-config-transaction-gas-limit</code> | notes | field EthereumFileConfig.transaction_gas_limit; struct denies unknown fields |

## cfg-mirror-native-checkpoint-policy-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-chain-id</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:507</code> | <code>mirror node checkpoint policy chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-chain-id</code> | notes | field NativeCheckpointPolicy.chain_id; struct denies unknown fields; 312 reference lines in 102 files; callers lists the first reference in each file; stale tasks 7.7.5 do_1 and 11.5.119 do_7 replace the anchored checkpoint with committed header and app hash reads; retention keys on block commit |

## cfg-mirror-native-checkpoint-policy-confirmations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-confirmations</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:510</code> | <code>mirror node checkpoint policy confirmations: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-confirmations</code> | notes | field NativeCheckpointPolicy.confirmations; struct denies unknown fields; stale tasks 7.7.5 do_1 and 11.5.119 do_7 replace the anchored checkpoint with committed header and app hash reads; retention keys on block commit |

## cfg-mirror-native-checkpoint-policy-genesis-hash-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-genesis-hash-hex</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:508</code> | <code>mirror node checkpoint policy genesis_hash_hex: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-genesis-hash-hex</code> | notes | field NativeCheckpointPolicy.genesis_hash_hex; struct denies unknown fields; stale tasks 7.7.5 do_1 and 11.5.119 do_7 replace the anchored checkpoint with committed header and app hash reads; retention keys on block commit |

## cfg-mirror-native-checkpoint-policy-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-rpc</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:506</code> | <code>mirror node checkpoint policy rpc: crate::rpc::RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-rpc</code> | notes | field NativeCheckpointPolicy.rpc; struct denies unknown fields; 1208 reference lines in 212 files; callers lists the first reference in each file; stale tasks 7.7.5 do_1 and 11.5.119 do_7 replace the anchored checkpoint with committed header and app hash reads; retention keys on block commit |

## cfg-mirror-native-checkpoint-policy-sequencer-public-key-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-sequencer-public-key-hex</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:509</code> | <code>mirror node checkpoint policy sequencer_public_key_hex: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-native-checkpoint-policy-sequencer-public-key-hex</code> | notes | field NativeCheckpointPolicy.sequencer_public_key_hex; struct denies unknown fields; stale tasks 7.7.5 do_1 and 11.5.119 do_7 replace the anchored checkpoint with committed header and app hash reads; retention keys on block commit |

## cfg-mirror-node-file-config-checkpoint-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-checkpoint-policy</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:58</code> | <code>mirror runtime node checkpoint_policy: crate::node::NativeCheckpointPolicy (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-checkpoint-policy</code> | notes | field NodeFileConfig.checkpoint_policy; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-deadline-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-deadline-ms</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:55</code> | <code>mirror runtime node deadline_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-deadline-ms</code> | notes | field NodeFileConfig.deadline_ms; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-expected-network-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-expected-network-id</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:50</code> | <code>mirror runtime node expected_network_id: u32 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-expected-network-id</code> | notes | field NodeFileConfig.expected_network_id; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-expected-protocol-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-expected-protocol-version</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:49</code> | <code>mirror runtime node expected_protocol_version: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-expected-protocol-version</code> | notes | field NodeFileConfig.expected_protocol_version; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-maximum-archive-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-archive-bytes</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:56</code> | <code>mirror runtime node maximum_archive_bytes: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-archive-bytes</code> | notes | field NodeFileConfig.maximum_archive_bytes; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-maximum-archive-chunks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-archive-chunks</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:57</code> | <code>mirror runtime node maximum_archive_chunks: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-archive-chunks</code> | notes | field NodeFileConfig.maximum_archive_chunks; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-maximum-connections

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-connections</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:52</code> | <code>mirror runtime node maximum_connections: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-connections</code> | notes | field NodeFileConfig.maximum_connections; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-maximum-frame-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-frame-bytes</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:51</code> | <code>mirror runtime node maximum_frame_bytes: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-frame-bytes</code> | notes | field NodeFileConfig.maximum_frame_bytes; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-maximum-queued-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-queued-bytes</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:54</code> | <code>mirror runtime node maximum_queued_bytes: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-queued-bytes</code> | notes | field NodeFileConfig.maximum_queued_bytes; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-maximum-streams

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-streams</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:53</code> | <code>mirror runtime node maximum_streams: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-maximum-streams</code> | notes | field NodeFileConfig.maximum_streams; struct denies unknown fields; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-node-file-config-socket

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-socket</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:48</code> | <code>mirror runtime node socket: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-node-file-config-socket</code> | notes | field NodeFileConfig.socket; struct denies unknown fields; 575 reference lines in 110 files; callers lists the first reference in each file; stale task 11.5.119 do_2 moves the mirror node input off the LNI socket transport to eth_call and gRPC queries at a height |

## cfg-mirror-rpc-endpoint-config-bearer-token-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-bearer-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:31</code> | <code>mirror rpc endpoint bearer_token_file: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-bearer-token-file</code> | notes | field RpcEndpointConfig.bearer_token_file; struct denies unknown fields |

## cfg-mirror-rpc-endpoint-config-ca-certificate-der

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-ca-certificate-der</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:30</code> | <code>mirror rpc endpoint ca_certificate_der: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-ca-certificate-der</code> | notes | field RpcEndpointConfig.ca_certificate_der; struct denies unknown fields |

## cfg-mirror-rpc-endpoint-config-independent-backend

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-independent-backend</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:34</code> | <code>mirror rpc endpoint independent_backend: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-independent-backend</code> | notes | field RpcEndpointConfig.independent_backend; struct denies unknown fields |

## cfg-mirror-rpc-endpoint-config-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-url</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:29</code> | <code>mirror rpc endpoint url: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-endpoint-config-url</code> | notes | field RpcEndpointConfig.url; struct denies unknown fields; 1052 reference lines in 163 files; callers lists the first reference in each file |

## cfg-mirror-rpc-quorum-config-connect-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-connect-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:43</code> | <code>mirror rpc quorum connect_timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-connect-timeout-ms</code> | notes | field RpcQuorumConfig.connect_timeout_ms; struct denies unknown fields |

## cfg-mirror-rpc-quorum-config-endpoints

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-endpoints</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:41</code> | <code>mirror rpc quorum endpoints: Vec&lt;RpcEndpointConfig&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-endpoints</code> | notes | field RpcQuorumConfig.endpoints; struct denies unknown fields; 333 reference lines in 73 files; callers lists the first reference in each file |

## cfg-mirror-rpc-quorum-config-maximum-response-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-maximum-response-bytes</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:45</code> | <code>mirror rpc quorum maximum_response_bytes: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-maximum-response-bytes</code> | notes | field RpcQuorumConfig.maximum_response_bytes; struct denies unknown fields |

## cfg-mirror-rpc-quorum-config-quorum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-quorum</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:42</code> | <code>mirror rpc quorum quorum: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-quorum</code> | notes | field RpcQuorumConfig.quorum; struct denies unknown fields; 149 reference lines in 44 files; callers lists the first reference in each file |

## cfg-mirror-rpc-quorum-config-request-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-request-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:44</code> | <code>mirror rpc quorum request_timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-rpc-quorum-config-request-timeout-ms</code> | notes | field RpcQuorumConfig.request_timeout_ms; struct denies unknown fields |

## cfg-mirror-runtime-config-checkpoint-freshness-budget-batches

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-checkpoint-freshness-budget-batches</code> | <code>config_key</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:35</code> | <code>mirror runtime config checkpoint_freshness_budget_batches: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-checkpoint-freshness-budget-batches</code> | notes | field RuntimeConfig.checkpoint_freshness_budget_batches; struct denies unknown fields; stale tasks 7.7.5 do_1 and 11.5.119 do_7 replace the anchored checkpoint with committed header and app hash reads; retention keys on block commit |

## cfg-mirror-runtime-config-ethereum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-ethereum</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:38</code> | <code>mirror runtime config ethereum: EthereumFileConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-ethereum</code> | notes | field RuntimeConfig.ethereum; struct denies unknown fields; 490 reference lines in 140 files; callers lists the first reference in each file; callers truncated by 14 entries for line length |

## cfg-mirror-runtime-config-first-batch-number

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-first-batch-number</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:31</code> | <code>mirror runtime config first_batch_number: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-first-batch-number</code> | notes | field RuntimeConfig.first_batch_number; struct denies unknown fields |

## cfg-mirror-runtime-config-node

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-node</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:37</code> | <code>mirror runtime config node: NodeFileConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-node</code> | notes | field RuntimeConfig.node; struct denies unknown fields; 2572 reference lines in 431 files; callers lists the first reference in each file |

## cfg-mirror-runtime-config-poll-interval-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-poll-interval-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:32</code> | <code>mirror runtime config poll_interval_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-poll-interval-ms</code> | notes | field RuntimeConfig.poll_interval_ms; struct denies unknown fields |

## cfg-mirror-runtime-config-solana

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-solana</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:42</code> | <code>mirror runtime config solana: Option&lt;SolanaFileConfig&gt; (optional)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-solana</code> | notes | field RuntimeConfig.solana; struct denies unknown fields; 234 reference lines in 29 files; callers lists the first reference in each file |

## cfg-mirror-runtime-config-state-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-state-directory</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:30</code> | <code>mirror runtime config state_directory: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-state-directory</code> | notes | field RuntimeConfig.state_directory; struct denies unknown fields |

## cfg-mirror-runtime-config-status-listen

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-status-listen</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:36</code> | <code>mirror runtime config status_listen: SocketAddr (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-runtime-config-status-listen</code> | notes | field RuntimeConfig.status_listen; struct denies unknown fields |

## cfg-mirror-signer-file-config-key-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-key-handle</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:96</code> | <code>mirror runtime signer key_handle: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-key-handle</code> | notes | field SignerFileConfig.key_handle; struct denies unknown fields |

## cfg-mirror-signer-file-config-public-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:97</code> | <code>mirror runtime signer public_key: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-public-key</code> | notes | field SignerFileConfig.public_key; struct denies unknown fields; 260 reference lines in 67 files; callers lists the first reference in each file |

## cfg-mirror-signer-file-config-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:98</code> | <code>mirror runtime signer timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-timeout-ms</code> | notes | field SignerFileConfig.timeout_ms; struct denies unknown fields |

## cfg-mirror-signer-file-config-transport

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-transport</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:99</code> | <code>mirror runtime signer transport: SignerTransportFileConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-signer-file-config-transport</code> | notes | field SignerFileConfig.transport; struct denies unknown fields; 2371 reference lines in 85 files; callers lists the first reference in each file |

## cfg-mirror-signer-transport-file-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-signer-transport-file-config</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:104</code> | <code>mirror runtime signer transport: one of "uds" &#124; "mutual_tls" (tag kind)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-signer-transport-file-config</code> | notes | serde enum SignerTransportFileConfig, deny_unknown_fields |

## cfg-mirror-solana-file-config-archive-program-base58

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-archive-program-base58</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:83</code> | <code>mirror runtime solana archive_program_base58: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-archive-program-base58</code> | notes | field SolanaFileConfig.archive_program_base58; struct denies unknown fields |

## cfg-mirror-solana-file-config-chunk-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-chunk-bytes</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:89</code> | <code>mirror runtime solana chunk_bytes: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-chunk-bytes</code> | notes | field SolanaFileConfig.chunk_bytes; struct denies unknown fields |

## cfg-mirror-solana-file-config-genesis-hash-base58

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-genesis-hash-base58</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:82</code> | <code>mirror runtime solana genesis_hash_base58: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-genesis-hash-base58</code> | notes | field SolanaFileConfig.genesis_hash_base58; struct denies unknown fields |

## cfg-mirror-solana-file-config-maximum-ancestry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-maximum-ancestry</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:88</code> | <code>mirror runtime solana maximum_ancestry: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-maximum-ancestry</code> | notes | field SolanaFileConfig.maximum_ancestry; struct denies unknown fields |

## cfg-mirror-solana-file-config-program-code-hash-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-program-code-hash-hex</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:86</code> | <code>mirror runtime solana program_code_hash_hex: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-program-code-hash-hex</code> | notes | field SolanaFileConfig.program_code_hash_hex; struct denies unknown fields |

## cfg-mirror-solana-file-config-program-data-account-base58

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-program-data-account-base58</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:85</code> | <code>mirror runtime solana program_data_account_base58: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-program-data-account-base58</code> | notes | field SolanaFileConfig.program_data_account_base58; struct denies unknown fields |

## cfg-mirror-solana-file-config-required-rooted-slots

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-required-rooted-slots</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:87</code> | <code>mirror runtime solana required_rooted_slots: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-required-rooted-slots</code> | notes | field SolanaFileConfig.required_rooted_slots; struct denies unknown fields |

## cfg-mirror-solana-file-config-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:81</code> | <code>mirror runtime solana rpc: RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-rpc</code> | notes | field SolanaFileConfig.rpc; struct denies unknown fields; 1208 reference lines in 212 files; callers lists the first reference in each file |

## cfg-mirror-solana-file-config-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-signer</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:90</code> | <code>mirror runtime solana signer: SignerFileConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-signer</code> | notes | field SolanaFileConfig.signer; struct denies unknown fields; 600 reference lines in 100 files; callers lists the first reference in each file |

## cfg-mirror-solana-file-config-upgradeable-loader-base58

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-upgradeable-loader-base58</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:84</code> | <code>mirror runtime solana upgradeable_loader_base58: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-solana-file-config-upgradeable-loader-base58</code> | notes | field SolanaFileConfig.upgradeable_loader_base58; struct denies unknown fields |

## cfg-mirror-source-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-source-config</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:35</code> | <code>mirror-verify source: one of "ethereum" &#124; "solana" (tag kind)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-source-config</code> | notes | serde enum SourceConfig, deny_unknown_fields |

## cli-mirror-publisher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-mirror-publisher</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-publisher.rs:24</code> | <code>layerx-mirror-publisher &lt;config.json&gt; &#124; --probe</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-mirror-publisher</code> | notes | --probe checks readiness and exits |

## cli-mirror-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-mirror-verify</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:109</code> | <code>layerx-mirror-verify &lt;request.json&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-mirror-verify</code> | notes | reads a Request (evidence + policy) JSON and prints the verification |

## err-mirror-checkpoint-capability-missing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-capability-missing</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:118</code> | <code>CheckpointRefusal::CapabilityMissing.code() = "checkpoint_capability_missing"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-capability-missing</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-certificate-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-certificate-mismatch</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:125</code> | <code>CheckpointRefusal::CertificateMismatch.code() = "checkpoint_certificate_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-certificate-mismatch</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-core-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-core-refusal</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:129</code> | <code>CheckpointRefusal::CoreRefusal.code() = "checkpoint_core_refusal"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-core-refusal</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-domain-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-domain-mismatch</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:121</code> | <code>CheckpointRefusal::DomainMismatch.code() = "checkpoint_domain_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-domain-mismatch</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-head-behind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-head-behind</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:127</code> | <code>CheckpointRefusal::HeadBehindCheckpoint.code() = "checkpoint_head_behind"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-head-behind</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-malformed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-malformed</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:130</code> | <code>CheckpointRefusal::Malformed.code() = "checkpoint_malformed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-malformed</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-mirror-archive-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-archive-unavailable</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1955</code> | <code>ReconcileRefusal::ArchiveUnavailable.code() = "checkpoint_mirror_archive_unavailable"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-archive-unavailable</code> | notes | wire code returned by ReconcileRefusal::code (services/interop/crates/layerx-mirror/src/publisher.rs:1953); spec/layerx-merge/spec.kvx task 11.5.119 moves the publisher off layerx_proof::checkpoint; the reconcile compares a checkpoint commitment |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-chain-unavailable</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1956</code> | <code>ReconcileRefusal::ChainUnavailable.code() = "checkpoint_mirror_chain_unavailable"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-chain-unavailable</code> | notes | wire code returned by ReconcileRefusal::code (services/interop/crates/layerx-mirror/src/publisher.rs:1953); spec/layerx-merge/spec.kvx task 11.5.119 moves the publisher off layerx_proof::checkpoint; the reconcile compares a checkpoint commitment |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-commitment-mismatch</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1958</code> | <code>ReconcileRefusal::CommitmentMismatch.code() = "checkpoint_mirror_commitment_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-commitment-mismatch</code> | notes | wire code returned by ReconcileRefusal::code (services/interop/crates/layerx-mirror/src/publisher.rs:1953); spec/layerx-merge/spec.kvx task 11.5.119 moves the publisher off layerx_proof::checkpoint; the reconcile compares a checkpoint commitment |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-malformed</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1959</code> | <code>ReconcileRefusal::Malformed.code() = "checkpoint_mirror_malformed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-malformed</code> | notes | wire code returned by ReconcileRefusal::code (services/interop/crates/layerx-mirror/src/publisher.rs:1953); spec/layerx-merge/spec.kvx task 11.5.119 moves the publisher off layerx_proof::checkpoint; the reconcile compares a checkpoint commitment |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-mismatch</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1960</code> | <code>ReconcileRefusal::CheckpointMismatch.code() = "checkpoint_mirror_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-mismatch</code> | notes | wire code returned by ReconcileRefusal::code (services/interop/crates/layerx-mirror/src/publisher.rs:1953); spec/layerx-merge/spec.kvx task 11.5.119 moves the publisher off layerx_proof::checkpoint; the reconcile compares a checkpoint commitment |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-missing</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1957</code> | <code>ReconcileRefusal::Missing.code() = "checkpoint_mirror_missing"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-mirror-missing</code> | notes | wire code returned by ReconcileRefusal::code (services/interop/crates/layerx-mirror/src/publisher.rs:1953); spec/layerx-merge/spec.kvx task 11.5.119 moves the publisher off layerx_proof::checkpoint; the reconcile compares a checkpoint commitment |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-selector-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-selector-mismatch</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:120</code> | <code>CheckpointRefusal::SelectorMismatch.code() = "checkpoint_selector_mismatch"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-selector-mismatch</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-settlement</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:126</code> | <code>CheckpointRefusal::Settlement.code() = "checkpoint_settlement"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-settlement</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-signature</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:124</code> | <code>CheckpointRefusal::Signature.code() = "checkpoint_signature"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-signature</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-signer-membership

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-signer-membership</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:122</code> | <code>CheckpointRefusal::SignerMembership.code() = "checkpoint_signer_membership"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-signer-membership</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-threshold

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-threshold</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:123</code> | <code>CheckpointRefusal::Threshold.code() = "checkpoint_threshold"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-threshold</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-transport

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-transport</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:128</code> | <code>CheckpointRefusal::Transport.code() = "checkpoint_transport"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-transport</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## err-mirror-checkpoint-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-checkpoint-unavailable</code> | <code>error_code</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:119</code> | <code>CheckpointRefusal::Unavailable.code() = "checkpoint_unavailable"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-checkpoint-unavailable</code> | notes | wire code returned by CheckpointRefusal::code (services/interop/crates/layerx-mirror/src/node.rs:116); spec/layerx-merge/spec.kvx tasks 7.7.5 and 13.7.3 replace the 0x1014 checkpoint acquisition these refusals describe with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-archive</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:16</code> | <code>mirror archive = "LXMIRROR" &#124; archive_version u16be (2, or 3 native) &#124; protocol_version u16be &#124; network_id u32be &#124; batch_number u64be &#124; canonical batch header (length-prefixed) &#124; batch authorization (sequencer_id, sequencer_public_key, ...) &#124; checkpoint and certificate (native certificates start "LXCPAUTH") &#124; records; at most 64 MiB, 65536 chunks, 1048576 records; domain "LXP/mirror/archive/v2\0"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-archive</code> | notes | decoder services/interop/crates/layerx-mirror/src/publisher.rs:196-330; stale task 7.7.5 and 11.5.119 do_5 replace the checkpoint and certificate payload with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-candidate</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:94</code> | <code>Candidate { source: usize, commitment_hex: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-candidate</code> | notes | mirror status/request JSON via serde Deserialize, deny_unknown_fields |

## fmt-mirror-component-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-component-status</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:118</code> | <code>ComponentStatus { ready: bool, latest_batch_mirrored?: Option&lt;u64&gt;, latest_batch_observed?: Option&lt;u64&gt;, latest_checkpoint_batch_mirrored?: Option&lt;u64&gt;, latest_checkpoint_id_mirrored?: Option&lt;String&gt;, latest_batch_verified?: Option&lt;u64&gt;, latest_checkpoint_batch_verified?: Option&lt;u64&gt;, checkpoint_batch_lag?: Option&lt;u64&gt;, freshness_budget_batches: u64, freshness?: Option&lt;&amp;'static str&gt;, checkpoint_within_budget: bool, phase?: Option&lt;&amp;'static str&gt;, error_class?: Option&lt;&amp;'static str&gt;, reorgs_observed: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-component-status</code> | notes | mirror status/request JSON via serde Clone, Debug, Default, Serialize |

## fmt-mirror-evidence-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-evidence-request</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:67</code> | <code>EvidenceRequest = receipt &#124; state (tag kind)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-evidence-request</code> | notes | mirror status/request JSON via serde Deserialize, deny_unknown_fields, rename_all kebab-case |

## fmt-mirror-policy-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-policy-request</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:79</code> | <code>PolicyRequest = exact &#124; ordered-preference &#124; agreement (tag kind)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-policy-request</code> | notes | mirror status/request JSON via serde Deserialize, deny_unknown_fields, rename_all kebab-case |

## fmt-mirror-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-request</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs:59</code> | <code>Request { batch_number: String, evidence: EvidenceRequest, policy: PolicyRequest }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-request</code> | notes | mirror status/request JSON via serde Deserialize, deny_unknown_fields |

## fmt-mirror-runtime-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-runtime-status</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:136</code> | <code>RuntimeStatus { source_chain_id: u64, native_network_id: u32, native_protocol_version: u16, service_version: String, node: ComponentStatus, ethereum: ComponentStatus, solana?: Option&lt;ComponentStatus&gt;, checkpoint_proof_boundary_ready: bool, checkpoint_identifier_observed: bool, latest_verified_checkpoint_batch?: Option&lt;u64&gt;, latest_verified_checkpoint_id?: Option&lt;String&gt;, checkpoint_refusal?: Option&lt;&amp;'static str&gt;, checkpoint_freshness_budget_batches: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-runtime-status</code> | notes | mirror status/request JSON via serde Clone, Debug, Default, Serialize |

## fmt-mirror-store-head

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-store-head</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:14</code> | <code>journal head file = "LXMHED02" &#124; sequence u64be &#124; offset u64be &#124; digest 32 (56 bytes, written atomically)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-store-head</code> | notes | state-directory head pointer |

## fmt-mirror-store-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-store-journal</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:13</code> | <code>publication journal record = "LXMJNL02" &#124; version u16be=2 &#124; chain tag u8 &#124; batch_number u64be &#124; commitment 32 &#124; checkpoint &#124; stage u8 (1 manifest, 2 chunk + index u32be, 3 finalize) &#124; phase u8 &#124; stage payload digest 32 &#124; signed payload (length-prefixed, &lt;= 384 KiB) &#124; transaction &#124; position; record &lt;= 512 KiB</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-store-journal</code> | notes | state-directory journal of layerx-mirror-publisher; checkpoint field follows 7.7.5 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-store-spool</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:15</code> | <code>archive spool = "LXMSPL02" &#124; latest_sealed_batch u64be &#124; latest finalised checkpoint &#124; archive bytes (length-prefixed, &lt;= 64 MiB) &#124; sha256 of the preceding bytes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-store-spool</code> | notes | checkpoint field follows 7.7.5 and 11.5.119 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-acquired-batch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-acquired-batch</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:45</code> | <code>pub struct AcquiredBatch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-acquired-batch</code> | notes | struct layerx_mirror::node::AcquiredBatch; no reference outside the defining file found by name match |

## mirror-acquired-checkpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-acquired-checkpoint</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:55</code> | <code>pub struct AcquiredCheckpoint</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-acquired-checkpoint</code> | notes | struct layerx_mirror::node::AcquiredCheckpoint; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-acquired-checkpoint-coordinate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-acquired-checkpoint-coordinate</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:68</code> | <code>pub const fn coordinate(&amp;self) -&gt; CheckpointCoordinate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-acquired-checkpoint-coordinate</code> | notes | method layerx_mirror::node::AcquiredCheckpoint::coordinate; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-acquired-checkpoint-material

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-acquired-checkpoint-material</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:63</code> | <code>pub const fn material(&amp;self) -&gt; NodeCheckpoint&lt;'_&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-acquired-checkpoint-material</code> | notes | method layerx_mirror::node::AcquiredCheckpoint::material; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:385</code> | <code>pub struct Archive</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive</code> | notes | struct layerx_mirror::publisher::Archive |

## mirror-archive-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:511</code> | <code>pub fn bytes(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-bytes</code> | notes | method layerx_mirror::publisher::Archive::bytes; no reference outside the defining file found by name match |

## mirror-archive-commitment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:28</code> | <code>pub struct ArchiveCommitment([u8; 32])</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment</code> | notes | struct layerx_mirror::publisher::ArchiveCommitment; 44 reference lines in 7 files; callers lists the first reference in each file |

## mirror-archive-commitment-as-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment-as-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:37</code> | <code>pub const fn as_bytes(&amp;self) -&gt; &amp;[u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment-as-bytes</code> | notes | method layerx_mirror::publisher::ArchiveCommitment::as_bytes; no reference outside the defining file found by name match |

## mirror-archive-commitment-from-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment-from-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:32</code> | <code>pub const fn from_bytes(bytes: [u8; 32]) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-commitment-from-bytes</code> | notes | method layerx_mirror::publisher::ArchiveCommitment::from_bytes |

## mirror-archive-data

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-data</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:175</code> | <code>pub struct ArchiveData</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-data</code> | notes | struct layerx_mirror::publisher::ArchiveData |
| <code>interface.11.rs-mirror-archive-data</code> | overridden_by | override.J48.2 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-data-decode</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:197</code> | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Result&lt;Self, ArchiveError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-data-decode</code> | notes | method layerx_mirror::publisher::ArchiveData::decode |
| <code>interface.11.rs-mirror-archive-data-decode</code> | overridden_by | override.J48.3 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:781</code> | <code>pub enum ArchiveError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-error</code> | notes | enum layerx_mirror::publisher::ArchiveError |

## mirror-archive-from-node

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-from-node</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:420</code> | <code>pub fn from_node(batch: &amp;NodeBatch, availability: &amp;AvailabilityResult, checkpoint: Option&lt;NodeCheckpoint&lt;'_&gt;&gt;, node_head: NodeHead) -&gt; Result&lt;Self, SourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-from-node</code> | notes | method layerx_mirror::publisher::Archive::from_node |
| <code>interface.11.rs-mirror-archive-from-node</code> | overridden_by | override.J48.17 |

## mirror-archive-node-head

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-node-head</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:521</code> | <code>pub const fn node_head(&amp;self) -&gt; NodeHead</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-node-head</code> | notes | method layerx_mirror::publisher::Archive::node_head |

## mirror-archive-spool

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:381</code> | <code>pub struct ArchiveSpool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-spool</code> | notes | struct layerx_mirror::store::ArchiveSpool |

## mirror-archive-spool-commitments

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-commitments</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:462</code> | <code>pub fn commitments(&amp;self) -&gt; Result&lt;Vec&lt;ArchiveCommitment&gt;, StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-commitments</code> | notes | method layerx_mirror::store::ArchiveSpool::commitments |

## mirror-archive-spool-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-get</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:450</code> | <code>pub fn get(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;SpooledArchive, StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-get</code> | notes | method layerx_mirror::store::ArchiveSpool::get; no reference outside the defining file found by name match |

## mirror-archive-spool-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:390</code> | <code>pub fn open(directory: impl Into&lt;PathBuf&gt;) -&gt; Result&lt;Self, StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-open</code> | notes | method layerx_mirror::store::ArchiveSpool::open |

## mirror-archive-spool-put

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-put</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:412</code> | <code>pub fn put(&amp;self, commitment: ArchiveCommitment, bytes: &amp;[u8], node_head: NodeHead) -&gt; Result&lt;(), StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archive-spool-put</code> | notes | method layerx_mirror::store::ArchiveSpool::put |

## mirror-archived-checkpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archived-checkpoint</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:168</code> | <code>pub struct ArchivedCheckpoint</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archived-checkpoint</code> | notes | struct layerx_mirror::publisher::ArchivedCheckpoint; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 11.5.119 do_5 moves services/interop/crates/layerx-mirror/src/publisher.rs:8 off layerx_proof::checkpoint to block commit finality |

## mirror-archived-chunk

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archived-chunk</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:138</code> | <code>pub struct ArchivedChunk</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archived-chunk</code> | notes | struct layerx_mirror::publisher::ArchivedChunk; no reference outside the defining file found by name match |
| <code>interface.11.rs-mirror-archived-chunk</code> | overridden_by | override.J48.21 |

## mirror-archived-records

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-archived-records</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:148</code> | <code>pub struct ArchivedRecords</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-archived-records</code> | notes | struct layerx_mirror::publisher::ArchivedRecords; no reference outside the defining file found by name match |
| <code>interface.11.rs-mirror-archived-records</code> | overridden_by | override.J48.22 |

## mirror-batch-authorization

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-batch-authorization</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:60</code> | <code>pub struct BatchAuthorization</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-batch-authorization</code> | notes | struct layerx_mirror::publisher::BatchAuthorization |
| <code>interface.11.rs-mirror-batch-authorization</code> | overridden_by | override.J48.20 |

## mirror-broadcast-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-broadcast-result</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:59</code> | <code>pub enum BroadcastResult</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-broadcast-result</code> | notes | enum layerx_mirror::rpc::BroadcastResult |

## mirror-chain-failure

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-chain-failure</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1140</code> | <code>pub enum ChainFailure</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-chain-failure</code> | notes | enum layerx_mirror::publisher::ChainFailure |

## mirror-chain-position

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-chain-position</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1213</code> | <code>pub enum ChainPosition</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-chain-position</code> | notes | enum layerx_mirror::publisher::ChainPosition |

## mirror-chain-progress

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-chain-progress</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1934</code> | <code>pub struct ChainProgress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-chain-progress</code> | notes | struct layerx_mirror::publisher::ChainProgress |

## mirror-chain-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-chain-signature</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:66</code> | <code>pub enum ChainSignature</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-chain-signature</code> | notes | enum layerx_mirror::signer::ChainSignature |

## mirror-checkpoint-acquisition

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-acquisition</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:75</code> | <code>pub enum CheckpointAcquisition</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-acquisition</code> | notes | enum layerx_mirror::node::CheckpointAcquisition; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-checkpoint-acquisition-coordinate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-acquisition-coordinate</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:88</code> | <code>pub fn coordinate(&amp;self) -&gt; Option&lt;CheckpointCoordinate&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-acquisition-coordinate</code> | notes | method layerx_mirror::node::CheckpointAcquisition::coordinate; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-checkpoint-coordinate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-coordinate</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:44</code> | <code>pub struct CheckpointCoordinate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-coordinate</code> | notes | struct layerx_mirror::publisher::CheckpointCoordinate; spec/layerx-merge/spec.kvx task 11.5.119 do_5 moves services/interop/crates/layerx-mirror/src/publisher.rs:8 off layerx_proof::checkpoint to block commit finality |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-checkpoint-freshness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-freshness</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1156</code> | <code>pub enum CheckpointFreshness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-freshness</code> | notes | enum layerx_mirror::publisher::CheckpointFreshness |
| <code>interface.11.rs-mirror-checkpoint-freshness</code> | overridden_by | override.J48.12 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-checkpoint-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-refusal</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:98</code> | <code>pub enum CheckpointRefusal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-refusal</code> | notes | enum layerx_mirror::node::CheckpointRefusal; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-checkpoint-refusal-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-refusal-code</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:116</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-refusal-code</code> | notes | method layerx_mirror::node::CheckpointRefusal::code; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-checkpoint-refusal-from-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-refusal-from-evidence</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:136</code> | <code>pub const fn from_evidence(error: &amp;EvidenceError) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-checkpoint-refusal-from-evidence</code> | notes | method layerx_mirror::node::CheckpointRefusal::from_evidence; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-config-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-config-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1000</code> | <code>pub enum ConfigError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-config-error</code> | notes | enum layerx_mirror::publisher::ConfigError |

## mirror-durable-publication-report

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-durable-publication-report</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1793</code> | <code>pub struct DurablePublicationReport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-durable-publication-report</code> | notes | struct layerx_mirror::publisher::DurablePublicationReport; no reference outside the defining file found by name match |

## mirror-ethereum-archive-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:280</code> | <code>pub struct EthereumArchiveClient</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client</code> | notes | struct layerx_mirror::ethereum::EthereumArchiveClient |

## mirror-ethereum-archive-client-advance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-advance</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:320</code> | <code>pub fn advance(&amp;mut self, archive: &amp;Archive) -&gt; Result&lt;EthereumProgress, EthereumError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-advance</code> | notes | method layerx_mirror::ethereum::EthereumArchiveClient::advance |

## mirror-ethereum-archive-client-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:414</code> | <code>pub fn cursor(&amp;self) -&gt; MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-cursor</code> | notes | method layerx_mirror::ethereum::EthereumArchiveClient::cursor; chain calls: eth_sendRawTransaction at :468 via private fn prepare_and_broadcast, eth_sendRawTransaction at :501 via private fn observe_stage, eth_getTransactionReceipt at :532 via private fn observe_finality, eth_getTransactionByHash at :542 via private fn observe_finality, eth_getBlockByNumber at :577 via private fn observe_finality, eth_blockNumber at :590 via private fn observe_finality, eth_estimateGas at :630 via private fn sign_transaction, eth_getTransactionCount at :643 via private fn sign_transaction, eth_maxPriorityFeePerGas at :647 via private fn sign_transaction, eth_gasPrice at :650 via private fn sign_transaction, eth_chainId at :698 via private fn verify_target, eth_getBlockByNumber at :706 via private fn verify_target, eth_getCode at :711 via private fn verify_target, eth_getTransactionByHash at :751 via private fn verify_receipt, eth_call at :798 via private fn eth_call, eth_getCode at :809 via private fn verify_contract_at, eth_blockNumber at :825 via private fn reorg_monitoring_complete |

## mirror-ethereum-archive-client-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:292</code> | <code>pub fn open(config: EthereumProductionConfig, signer: RemoteChainSigner) -&gt; Result&lt;Self, EthereumError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-open</code> | notes | method layerx_mirror::ethereum::EthereumArchiveClient::open |

## mirror-ethereum-archive-client-retrieve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-retrieve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:354</code> | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, EthereumError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-retrieve</code> | notes | method layerx_mirror::ethereum::EthereumArchiveClient::retrieve |

## mirror-ethereum-archive-client-retrieve-finalized

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-retrieve-finalized</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:400</code> | <code>pub fn retrieve_finalized(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, EthereumError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-client-retrieve-finalized</code> | notes | method layerx_mirror::ethereum::EthereumArchiveClient::retrieve_finalized |

## mirror-ethereum-archive-write

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-write</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1007</code> | <code>pub struct EthereumArchiveWrite&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-archive-write</code> | notes | struct layerx_mirror::publisher::EthereumArchiveWrite |

## mirror-ethereum-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:946</code> | <code>pub struct EthereumConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-config</code> | notes | struct layerx_mirror::publisher::EthereumConfig |

## mirror-ethereum-config-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-config-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:958</code> | <code>pub fn validate(self) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-config-validate</code> | notes | method layerx_mirror::publisher::EthereumConfig::validate; no reference outside the defining file found by name match |

## mirror-ethereum-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:243</code> | <code>pub enum EthereumError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-error</code> | notes | enum layerx_mirror::ethereum::EthereumError |

## mirror-ethereum-file-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-file-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:63</code> | <code>pub struct EthereumFileConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-file-config</code> | notes | struct layerx_mirror::runtime::EthereumFileConfig; no reference outside the defining file found by name match |

## mirror-ethereum-lane-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-lane-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1761</code> | <code>pub enum EthereumLaneState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-lane-state</code> | notes | enum layerx_mirror::publisher::EthereumLaneState; no reference outside the defining file found by name match |

## mirror-ethereum-mirror-read

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-read</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:56</code> | <code>pub struct EthereumMirrorRead</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-read</code> | notes | struct layerx_mirror::ethereum::EthereumMirrorRead; no reference outside the defining file found by name match |

## mirror-ethereum-mirror-read-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-read-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:45</code> | <code>pub struct EthereumMirrorReadConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-read-config</code> | notes | struct layerx_mirror::ethereum::EthereumMirrorReadConfig |

## mirror-ethereum-mirror-reader

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:66</code> | <code>pub struct EthereumMirrorReader</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader</code> | notes | struct layerx_mirror::ethereum::EthereumMirrorReader |

## mirror-ethereum-mirror-reader-is-canonical

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-is-canonical</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:164</code> | <code>pub fn is_canonical(&amp;self, observation: &amp;EthereumMirrorRead) -&gt; Result&lt;bool, EthereumError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-is-canonical</code> | notes | method layerx_mirror::ethereum::EthereumMirrorReader::is_canonical; no reference outside the defining file found by name match |

## mirror-ethereum-mirror-reader-is-coordinate-canonical

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-is-coordinate-canonical</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:171</code> | <code>pub fn is_coordinate_canonical(&amp;self, block_number: u64, block_hash: [u8; 32]) -&gt; Result&lt;bool, EthereumError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-is-coordinate-canonical</code> | notes | method layerx_mirror::ethereum::EthereumMirrorReader::is_coordinate_canonical; chain calls: eth_getBlockByNumber at :177, eth_chainId at :184 via private fn verify_static_identity, eth_getBlockByNumber at :192 via private fn verify_static_identity, eth_getCode at :201 via private fn verify_target_at, eth_call at :222 via private fn eth_call_at |

## mirror-ethereum-mirror-reader-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:75</code> | <code>pub fn open(config: EthereumMirrorReadConfig) -&gt; Result&lt;Self, EthereumError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-open</code> | notes | method layerx_mirror::ethereum::EthereumMirrorReader::open |

## mirror-ethereum-mirror-reader-retrieve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-retrieve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:96</code> | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;EthereumMirrorRead&gt;, EthereumError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-mirror-reader-retrieve</code> | notes | method layerx_mirror::ethereum::EthereumMirrorReader::retrieve; chain calls: eth_getBlockByNumber at :103 |

## mirror-ethereum-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-observation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1044</code> | <code>pub enum EthereumObservation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-observation</code> | notes | enum layerx_mirror::publisher::EthereumObservation |

## mirror-ethereum-production-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-production-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:25</code> | <code>pub struct EthereumProductionConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-production-config</code> | notes | struct layerx_mirror::ethereum::EthereumProductionConfig |

## mirror-ethereum-progress

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-progress</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/ethereum.rs:233</code> | <code>pub struct EthereumProgress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-progress</code> | notes | struct layerx_mirror::ethereum::EthereumProgress |

## mirror-ethereum-submission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-ethereum-submission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1032</code> | <code>pub struct EthereumSubmission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-ethereum-submission</code> | notes | struct layerx_mirror::publisher::EthereumSubmission |

## mirror-finality-position

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-finality-position</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:113</code> | <code>pub enum FinalityPosition</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-finality-position</code> | notes | enum layerx_mirror::store::FinalityPosition |

## mirror-generic-publisher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1680</code> | <code>pub struct GenericPublisher&lt;E, S&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher</code> | notes | struct layerx_mirror::publisher::GenericPublisher |

## mirror-generic-publisher-ethereum-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-ethereum-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1742</code> | <code>pub const fn ethereum_cursor(&amp;self) -&gt; MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-ethereum-cursor</code> | notes | method layerx_mirror::publisher::GenericPublisher::ethereum_cursor |

## mirror-generic-publisher-into-clients

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-into-clients</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1754</code> | <code>pub fn into_clients(self) -&gt; (E, S)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-into-clients</code> | notes | method layerx_mirror::publisher::GenericPublisher::into_clients; no reference outside the defining file found by name match |

## mirror-generic-publisher-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1691</code> | <code>pub fn new(ethereum_config: EthereumConfig, ethereum_client: E, solana_config: SolanaConfig, solana_client: S) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-new</code> | notes | method layerx_mirror::publisher::GenericPublisher::new |

## mirror-generic-publisher-publish

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-publish</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1721</code> | <code>pub fn publish(&amp;mut self, archive: &amp;Archive) -&gt; PublicationReport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-publish</code> | notes | method layerx_mirror::publisher::GenericPublisher::publish |

## mirror-generic-publisher-retrieve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-retrieve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1734</code> | <code>pub fn retrieve(&amp;mut self, commitment: ArchiveCommitment) -&gt; RetrievalReport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-retrieve</code> | notes | method layerx_mirror::publisher::GenericPublisher::retrieve |

## mirror-generic-publisher-solana-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-solana-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1747</code> | <code>pub const fn solana_cursor(&amp;self) -&gt; MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-generic-publisher-solana-cursor</code> | notes | method layerx_mirror::publisher::GenericPublisher::solana_cursor |

## mirror-lni-archive-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-lni-archive-source</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:201</code> | <code>pub struct LniArchiveSource</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-lni-archive-source</code> | notes | struct layerx_mirror::node::LniArchiveSource; spec/layerx-merge/spec.kvx task 11.5.119 do_2 moves node.rs off the LNI transport to eth_sendRawTransaction/eth_call; 7.7.5 replaces acquire_checkpoint (services/interop/crates/layerx-mirror/src/node.rs:352) and verified_checkpoint (:382) with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-lni-archive-source-acquire

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-lni-archive-source-acquire</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:225</code> | <code>pub fn acquire(&amp;mut self, batch_number: u64) -&gt; Result&lt;AcquiredBatch, NodeSourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-lni-archive-source-acquire</code> | notes | method layerx_mirror::node::LniArchiveSource::acquire; spec/layerx-merge/spec.kvx task 11.5.119 do_2 moves node.rs off the LNI transport to eth_sendRawTransaction/eth_call; 7.7.5 replaces acquire_checkpoint (services/interop/crates/layerx-mirror/src/node.rs:352) and verified_checkpoint (:382) with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-lni-archive-source-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-lni-archive-source-new</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:209</code> | <code>pub fn new(config: NodeSourceConfig) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-lni-archive-source-new</code> | notes | method layerx_mirror::node::LniArchiveSource::new; spec/layerx-merge/spec.kvx task 11.5.119 do_2 moves node.rs off the LNI transport to eth_sendRawTransaction/eth_call; 7.7.5 replaces acquire_checkpoint (services/interop/crates/layerx-mirror/src/node.rs:352) and verified_checkpoint (:382) with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-lni-archive-source-observe-checkpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-lni-archive-source-observe-checkpoint</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:331</code> | <code>pub fn observe_checkpoint(&amp;mut self) -&gt; Result&lt;(NodeHead, CheckpointAcquisition), NodeSourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-lni-archive-source-observe-checkpoint</code> | notes | method layerx_mirror::node::LniArchiveSource::observe_checkpoint; spec/layerx-merge/spec.kvx task 11.5.119 do_2 moves node.rs off the LNI transport to eth_sendRawTransaction/eth_call; 7.7.5 replaces acquire_checkpoint (services/interop/crates/layerx-mirror/src/node.rs:352) and verified_checkpoint (:382) with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-max-mirror-sources

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-max-mirror-sources</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:10</code> | <code>pub const MAX_MIRROR_SOURCES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-max-mirror-sources</code> | notes | const layerx_mirror::source::MAX_MIRROR_SOURCES; no reference outside the defining file found by name match |

## mirror-max-source-id-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-max-source-id-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:11</code> | <code>pub const MAX_SOURCE_ID_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-max-source-id-bytes</code> | notes | const layerx_mirror::source::MAX_SOURCE_ID_BYTES; no reference outside the defining file found by name match |

## mirror-mirror-canonical-position

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-canonical-position</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:56</code> | <code>pub enum MirrorCanonicalPosition</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-canonical-position</code> | notes | enum layerx_mirror::source::MirrorCanonicalPosition |

## mirror-mirror-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:23</code> | <code>pub enum MirrorChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-chain</code> | notes | enum layerx_mirror::store::MirrorChain |

## mirror-mirror-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-cursor</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1149</code> | <code>pub struct MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-cursor</code> | notes | struct layerx_mirror::publisher::MirrorCursor |
| <code>interface.11.rs-mirror-mirror-cursor</code> | overridden_by | override.J48.14 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-degradation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1228</code> | <code>pub enum MirrorDegradation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-degradation</code> | notes | enum layerx_mirror::publisher::MirrorDegradation |

## mirror-mirror-evidence-level

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-evidence-level</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:117</code> | <code>pub enum MirrorEvidenceLevel</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-evidence-level</code> | notes | enum layerx_mirror::verify::MirrorEvidenceLevel |
| <code>interface.11.rs-mirror-mirror-evidence-level</code> | overridden_by | override.J48.6 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-freshness</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1166</code> | <code>pub struct MirrorFreshness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-freshness</code> | notes | struct layerx_mirror::publisher::MirrorFreshness |
| <code>interface.11.rs-mirror-mirror-freshness</code> | overridden_by | override.J48.13 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-lag</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:76</code> | <code>pub enum MirrorLag</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-lag</code> | notes | enum layerx_mirror::source::MirrorLag |

## mirror-mirror-locator

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-locator</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:129</code> | <code>pub struct MirrorLocator</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-locator</code> | notes | struct layerx_mirror::source::MirrorLocator |

## mirror-mirror-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-observation</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:92</code> | <code>pub struct MirrorObservation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-observation</code> | notes | struct layerx_mirror::source::MirrorObservation |
| <code>interface.11.rs-mirror-mirror-observation</code> | overridden_by | override.J48.16 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-provenance</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:70</code> | <code>pub enum MirrorProvenance</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-provenance</code> | notes | enum layerx_mirror::source::MirrorProvenance; no reference outside the defining file found by name match |

## mirror-mirror-read-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-read-policy</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:135</code> | <code>pub enum MirrorReadPolicy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-read-policy</code> | notes | enum layerx_mirror::source::MirrorReadPolicy |

## mirror-mirror-retrieval

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-retrieval</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1809</code> | <code>pub type MirrorRetrieval&lt;E&gt; = Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, E&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-retrieval</code> | notes | type alias layerx_mirror::publisher::MirrorRetrieval; no reference outside the defining file found by name match |

## mirror-mirror-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:203</code> | <code>pub struct MirrorSource</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-source</code> | notes | struct layerx_mirror::source::MirrorSource |

## mirror-mirror-source-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:145</code> | <code>pub enum MirrorSourceError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-error</code> | notes | enum layerx_mirror::source::MirrorSourceError |

## mirror-mirror-source-ethereum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-ethereum</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:213</code> | <code>pub fn ethereum(id: MirrorSourceId, config: EthereumMirrorReadConfig) -&gt; Result&lt;Self, MirrorSourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-ethereum</code> | notes | method layerx_mirror::source::MirrorSource::ethereum |

## mirror-mirror-source-freshness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-freshness</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:84</code> | <code>pub struct MirrorSourceFreshness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-freshness</code> | notes | struct layerx_mirror::source::MirrorSourceFreshness |
| <code>interface.11.rs-mirror-mirror-source-freshness</code> | overridden_by | override.J48.15 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:14</code> | <code>pub struct MirrorSourceId(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id</code> | notes | struct layerx_mirror::source::MirrorSourceId |

## mirror-mirror-source-id-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:32</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id-as-str</code> | notes | method layerx_mirror::source::MirrorSourceId::as_str; no reference outside the defining file found by name match |

## mirror-mirror-source-id-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:20</code> | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, MirrorSourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-id-new</code> | notes | method layerx_mirror::source::MirrorSourceId::new |

## mirror-mirror-source-solana

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-solana</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:242</code> | <code>pub fn solana(id: MirrorSourceId, config: SolanaMirrorReadConfig) -&gt; Result&lt;Self, MirrorSourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-source-solana</code> | notes | method layerx_mirror::source::MirrorSource::solana |

## mirror-mirror-sources

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:357</code> | <code>pub struct MirrorSources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources</code> | notes | struct layerx_mirror::source::MirrorSources |

## mirror-mirror-sources-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:366</code> | <code>pub fn new(layerx_network_id: u32, sources: Vec&lt;MirrorSource&gt;) -&gt; Result&lt;Self, MirrorSourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-new</code> | notes | method layerx_mirror::source::MirrorSources::new |

## mirror-mirror-sources-read

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-read</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:392</code> | <code>pub fn read(&amp;self, batch_number: u64, policy: &amp;MirrorReadPolicy) -&gt; Result&lt;ObservedArchive, MirrorSourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-read</code> | notes | method layerx_mirror::source::MirrorSources::read; no reference outside the defining file found by name match |

## mirror-mirror-sources-recheck

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-recheck</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:479</code> | <code>pub fn recheck(&amp;self, mut archive: ObservedArchive) -&gt; Result&lt;ObservedArchive, MirrorSourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-sources-recheck</code> | notes | method layerx_mirror::source::MirrorSources::recheck; no reference outside the defining file found by name match |

## mirror-mirror-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1238</code> | <code>pub enum MirrorState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-state</code> | notes | enum layerx_mirror::publisher::MirrorState |

## mirror-mirror-state-freshness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-state-freshness</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1265</code> | <code>pub const fn freshness(&amp;self) -&gt; MirrorFreshness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-state-freshness</code> | notes | method layerx_mirror::publisher::MirrorState::freshness |

## mirror-mirror-target-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-target-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:38</code> | <code>pub enum MirrorTargetIdentity</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-target-identity</code> | notes | enum layerx_mirror::source::MirrorTargetIdentity |

## mirror-mirror-verification

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:75</code> | <code>pub struct MirrorVerification&lt;T&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification</code> | notes | struct layerx_mirror::verify::MirrorVerification |

## mirror-mirror-verification-batch-number

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-batch-number</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:96</code> | <code>pub const fn batch_number(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-batch-number</code> | notes | method layerx_mirror::verify::MirrorVerification::batch_number; no reference outside the defining file found by name match |

## mirror-mirror-verification-freshness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:30</code> | <code>pub struct MirrorVerificationFreshness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness</code> | notes | struct layerx_mirror::verify::MirrorVerificationFreshness; no reference outside the defining file found by name match |

## mirror-mirror-verification-freshness-from-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-from-source</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:62</code> | <code>pub const fn from_source(value: MirrorSourceFreshness) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-from-source</code> | notes | method layerx_mirror::verify::MirrorVerificationFreshness::from_source; no reference outside the defining file found by name match |

## mirror-mirror-verification-freshness-offline

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-offline</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:39</code> | <code>pub const fn offline(latest_batch_mirrored: u64, latest_checkpoint_mirrored: Option&lt;CheckpointCoordinate&gt;) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-offline</code> | notes | method layerx_mirror::verify::MirrorVerificationFreshness::offline; no reference outside the defining file found by name match |

## mirror-mirror-verification-freshness-relative

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-relative</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:52</code> | <code>pub fn relative(value: MirrorFreshness) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-freshness-relative</code> | notes | method layerx_mirror::verify::MirrorVerificationFreshness::relative; no reference outside the defining file found by name match |

## mirror-mirror-verification-level

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-level</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:91</code> | <code>pub const fn level(&amp;self) -&gt; MirrorEvidenceLevel</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-level</code> | notes | method layerx_mirror::verify::MirrorVerification::level; no reference outside the defining file found by name match |

## mirror-mirror-verification-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-observation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:111</code> | <code>pub fn observation(&amp;self) -&gt; Option&lt;&amp;MirrorObservation&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-observation</code> | notes | method layerx_mirror::verify::MirrorVerification::observation; no reference outside the defining file found by name match |

## mirror-mirror-verification-signed-header-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-signed-header-digest</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:101</code> | <code>pub const fn signed_header_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-signed-header-digest</code> | notes | method layerx_mirror::verify::MirrorVerification::signed_header_digest; no reference outside the defining file found by name match |
| <code>interface.11.rs-mirror-mirror-verification-signed-header-digest</code> | overridden_by | override.J48.26 |

## mirror-mirror-verification-value

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-value</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:86</code> | <code>pub const fn value(&amp;self) -&gt; &amp;T</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verification-value</code> | notes | method layerx_mirror::verify::MirrorVerification::value; no reference outside the defining file found by name match |

## mirror-mirror-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:142</code> | <code>pub struct MirrorVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier</code> | notes | struct layerx_mirror::verify::MirrorVerifier |

## mirror-mirror-verifier-checkpoint-level

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-checkpoint-level</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:409</code> | <code>pub const fn checkpoint_level() -&gt; Result&lt;(), MirrorVerifyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-checkpoint-level</code> | notes | method layerx_mirror::verify::MirrorVerifier::checkpoint_level; no reference outside the defining file found by name match |
| <code>interface.11.rs-mirror-mirror-verifier-checkpoint-level</code> | overridden_by | override.J48.25 |

## mirror-mirror-verifier-checkpoint-with-native-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-checkpoint-with-native-authority</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:155</code> | <code>pub fn checkpoint_with_native_authority(&amp;self, policy: &amp;crate::node::NativeCheckpointPolicy) -&gt; Result&lt;CheckpointCoordinate, MirrorVerifyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-checkpoint-with-native-authority</code> | notes | method layerx_mirror::verify::MirrorVerifier::checkpoint_with_native_authority; no reference outside the defining file found by name match |
| <code>interface.11.rs-mirror-mirror-verifier-checkpoint-with-native-authority</code> | overridden_by | override.J48.10 |

## mirror-mirror-verifier-from-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-from-source</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:183</code> | <code>pub fn from_source(observed: ObservedArchive, trust: SignedHeaderTrust) -&gt; Result&lt;Self, MirrorVerifyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-from-source</code> | notes | method layerx_mirror::verify::MirrorVerifier::from_source |

## mirror-mirror-verifier-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-receipt</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:250</code> | <code>pub fn receipt(&amp;self, canonical_receipt: &amp;[u8]) -&gt; Result&lt;MirrorVerification&lt;VerifiedReceipt&gt;, MirrorVerifyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-receipt</code> | notes | method layerx_mirror::verify::MirrorVerifier::receipt |
| <code>interface.11.rs-mirror-mirror-verifier-receipt</code> | overridden_by | override.J48.4 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-mirror-verifier-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-state</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:366</code> | <code>pub fn state(&amp;self, canonical_state: &amp;[u8], proof: &amp;Proof) -&gt; Result&lt;MirrorVerification&lt;layerx_proof::inclusion::InclusionEvidence&gt;, MirrorVerifyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verifier-state</code> | notes | method layerx_mirror::verify::MirrorVerifier::state |
| <code>interface.11.rs-mirror-mirror-verifier-state</code> | overridden_by | override.J48.24 |

## mirror-mirror-verify-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-mirror-verify-error</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:123</code> | <code>pub enum MirrorVerifyError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-mirror-verify-error</code> | notes | enum layerx_mirror::verify::MirrorVerifyError |
| <code>interface.11.rs-mirror-mirror-verify-error</code> | overridden_by | override.J48.5 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-native-checkpoint-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-native-checkpoint-policy</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:505</code> | <code>pub struct NativeCheckpointPolicy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-native-checkpoint-policy</code> | notes | struct layerx_mirror::node::NativeCheckpointPolicy; spec/layerx-merge/spec.kvx task 7.7.5 and 13.7.3 replace the 0x1014 statusOf checkpoint and guarantor reads (services/interop/crates/layerx-mirror/src/node.rs:544, :591, :624, :704) with the committed header and app hash; chain calls: eth_call at :550 via private fn native_call, 0x1014 at :551 via private fn native_call, eth_chainId at :605 via private fn native_candidate_publication, eth_getBlockByNumber at :608 via private fn native_candidate_publication, eth_blockNumber at :612 via private fn native_candidate_publication, eth_getBlockByNumber at :616 via private fn native_candidate_publication, eth_getBlockByNumber at :650 via private fn native_candidate_publication, eth_getBlockByNumber at :651 via private fn native_candidate_publication, eth_getTransactionReceipt at :672 via private fn native_submission, eth_getTransactionByHash at :673 via private fn native_submission, eth_getBlockByNumber at :681 via private fn native_submission |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-node-batch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-node-batch</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:70</code> | <code>pub struct NodeBatch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-node-batch</code> | notes | struct layerx_mirror::publisher::NodeBatch |
| <code>interface.11.rs-mirror-node-batch</code> | overridden_by | override.J48.18 |

## mirror-node-batch-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-node-batch-verify</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:82</code> | <code>pub fn verify(canonical_header: Vec&lt;u8&gt;, authorization: BatchAuthorization, trust: &amp;crate::SignedHeaderTrust) -&gt; Result&lt;Self, SourceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-node-batch-verify</code> | notes | method layerx_mirror::publisher::NodeBatch::verify |
| <code>interface.11.rs-mirror-node-batch-verify</code> | overridden_by | override.J48.19 |

## mirror-node-checkpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-node-checkpoint</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:121</code> | <code>pub struct NodeCheckpoint&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-node-checkpoint</code> | notes | struct layerx_mirror::publisher::NodeCheckpoint; spec/layerx-merge/spec.kvx task 11.5.119 do_5 moves services/interop/crates/layerx-mirror/src/publisher.rs:8 off layerx_proof::checkpoint to block commit finality |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-node-checkpoint-verified

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-node-checkpoint-verified</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:128</code> | <code>pub const fn verified(read: &amp;'a VerifiedRead&lt;CheckpointValue&gt;, verification: &amp;'a ThresholdReport) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-node-checkpoint-verified</code> | notes | method layerx_mirror::publisher::NodeCheckpoint::verified; spec/layerx-merge/spec.kvx task 11.5.119 do_5 moves services/interop/crates/layerx-mirror/src/publisher.rs:8 off layerx_proof::checkpoint to block commit finality |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-node-file-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-node-file-config</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:47</code> | <code>pub struct NodeFileConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-node-file-config</code> | notes | struct layerx_mirror::runtime::NodeFileConfig; no reference outside the defining file found by name match |
| <code>interface.11.rs-mirror-node-file-config</code> | overridden_by | override.J48.8 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-node-head</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:51</code> | <code>pub struct NodeHead</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-node-head</code> | notes | struct layerx_mirror::publisher::NodeHead |
| <code>interface.11.rs-mirror-node-head</code> | overridden_by | override.J48.11 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-node-source-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-node-source-config</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:36</code> | <code>pub struct NodeSourceConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-node-source-config</code> | notes | struct layerx_mirror::node::NodeSourceConfig; spec/layerx-merge/spec.kvx task 11.5.119 do_2 moves node.rs off the LNI transport to eth_sendRawTransaction/eth_call; 7.7.5 replaces acquire_checkpoint (services/interop/crates/layerx-mirror/src/node.rs:352) and verified_checkpoint (:382) with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | The mirror archives committed Paxeer blocks as v4 archives and verifies their receipts by committed height. |
| storage | Mirror state directory holding the cursor and pending chunks. |
| errors | MirrorVerifyError and ReconcileRefusal with committed_mirror codes. |
| events | none |
| gas | Archive publication pays destination chain gas. |
| activation | Starts when the paxeer config section is present. |
| invariants | Every archived block is identified by height and block hash. |
| pause | none |

## mirror-node-source-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-node-source-error</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/node.rs:172</code> | <code>pub enum NodeSourceError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-node-source-error</code> | notes | enum layerx_mirror::node::NodeSourceError; spec/layerx-merge/spec.kvx task 11.5.119 do_2 moves node.rs off the LNI transport to eth_sendRawTransaction/eth_call; 7.7.5 replaces acquire_checkpoint (services/interop/crates/layerx-mirror/src/node.rs:352) and verified_checkpoint (:382) with committed header reads |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-observed-archive</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:107</code> | <code>pub struct ObservedArchive</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-observed-archive</code> | notes | struct layerx_mirror::source::ObservedArchive |

## mirror-observed-archive-archive

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-observed-archive-archive</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:114</code> | <code>pub fn archive(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-observed-archive-archive</code> | notes | method layerx_mirror::source::ObservedArchive::archive; no reference outside the defining file found by name match |

## mirror-observed-archive-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-observed-archive-observation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/source.rs:119</code> | <code>pub fn observation(&amp;self) -&gt; &amp;MirrorObservation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-observed-archive-observation</code> | notes | method layerx_mirror::source::ObservedArchive::observation; no reference outside the defining file found by name match |

## mirror-progress-gate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-progress-gate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1969</code> | <code>pub struct ProgressGate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-progress-gate</code> | notes | struct layerx_mirror::publisher::ProgressGate |

## mirror-progress-gate-acknowledge

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-progress-gate-acknowledge</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1982</code> | <code>pub fn acknowledge&lt;E&gt;(&amp;mut self, journal: MirrorCursor, archives: &amp;[Archive], retrieve: impl FnOnce(ArchiveCommitment) -&gt; MirrorRetrieval&lt;E&gt;) -&gt; ChainProgress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-progress-gate-acknowledge</code> | notes | method layerx_mirror::publisher::ProgressGate::acknowledge |

## mirror-progress-gate-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-progress-gate-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1975</code> | <code>pub const fn new() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-progress-gate-new</code> | notes | method layerx_mirror::publisher::ProgressGate::new |

## mirror-publication-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1207</code> | <code>pub enum PublicationId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-id</code> | notes | enum layerx_mirror::publisher::PublicationId |

## mirror-publication-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:199</code> | <code>pub struct PublicationJournal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-journal</code> | notes | struct layerx_mirror::store::PublicationJournal |

## mirror-publication-journal-append

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-append</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:277</code> | <code>pub fn append(&amp;mut self, record: PublicationRecord) -&gt; Result&lt;(), StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-append</code> | notes | method layerx_mirror::store::PublicationJournal::append |

## mirror-publication-journal-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:340</code> | <code>pub fn cursor(&amp;self) -&gt; MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-cursor</code> | notes | method layerx_mirror::store::PublicationJournal::cursor |

## mirror-publication-journal-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:229</code> | <code>pub fn open(directory: impl Into&lt;PathBuf&gt;, chain: MirrorChain, first_batch_number: u64) -&gt; Result&lt;Self, StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-open</code> | notes | method layerx_mirror::store::PublicationJournal::open |

## mirror-publication-journal-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:329</code> | <code>pub fn record(&amp;self, commitment: ArchiveCommitment, stage: PublicationStage) -&gt; Option&lt;&amp;PublicationRecord&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-record</code> | notes | method layerx_mirror::store::PublicationJournal::record; no reference outside the defining file found by name match |

## mirror-publication-journal-records

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-records</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:368</code> | <code>pub fn records(&amp;self) -&gt; impl Iterator&lt;Item = &amp;PublicationRecord&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-journal-records</code> | notes | method layerx_mirror::store::PublicationJournal::records; no reference outside the defining file found by name match |

## mirror-publication-phase

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-phase</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:56</code> | <code>pub enum PublicationPhase</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-phase</code> | notes | enum layerx_mirror::store::PublicationPhase; 79 reference lines in 4 files; callers lists the first reference in each file |

## mirror-publication-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:128</code> | <code>pub struct PublicationRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-record</code> | notes | struct layerx_mirror::store::PublicationRecord |

## mirror-publication-record-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-record-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:147</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), StoreError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-record-validate</code> | notes | method layerx_mirror::store::PublicationRecord::validate; no reference outside the defining file found by name match |

## mirror-publication-report

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-report</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1277</code> | <code>pub struct PublicationReport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-report</code> | notes | struct layerx_mirror::publisher::PublicationReport; no reference outside the defining file found by name match |

## mirror-publication-stage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publication-stage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:47</code> | <code>pub enum PublicationStage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publication-stage</code> | notes | enum layerx_mirror::store::PublicationStage |

## mirror-publisher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1802</code> | <code>pub struct Publisher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher</code> | notes | struct layerx_mirror::publisher::Publisher; no reference outside the defining file found by name match |

## mirror-publisher-archive-commitment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-commitment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:516</code> | <code>pub const fn commitment(&amp;self) -&gt; ArchiveCommitment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-commitment</code> | notes | method layerx_mirror::publisher::Archive::commitment |

## mirror-publisher-archive-commitment-2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-commitment-2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:745</code> | <code>pub fn archive_commitment(bytes: &amp;[u8]) -&gt; ArchiveCommitment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-commitment-2</code> | notes | fn layerx_mirror::publisher::archive_commitment |

## mirror-publisher-archive-data

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-data</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:506</code> | <code>pub const fn data(&amp;self) -&gt; &amp;ArchiveData</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-archive-data</code> | notes | method layerx_mirror::publisher::Archive::data |

## mirror-publisher-ethereum-archive-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-ethereum-archive-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1076</code> | <code>pub trait EthereumArchiveClient</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-ethereum-archive-client</code> | notes | trait layerx_mirror::publisher::EthereumArchiveClient |

## mirror-publisher-ethereum-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-ethereum-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1905</code> | <code>pub fn ethereum_cursor(&amp;self) -&gt; MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-ethereum-cursor</code> | notes | method layerx_mirror::publisher::Publisher::ethereum_cursor |

## mirror-publisher-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1813</code> | <code>pub const fn new(ethereum: crate::ethereum::EthereumArchiveClient, solana: crate::solana::SolanaArchiveClient) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-new</code> | notes | method layerx_mirror::publisher::Publisher::new; no reference outside the defining file found by name match |

## mirror-publisher-progress

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-progress</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1889</code> | <code>pub fn progress(&amp;mut self, archives: &amp;[Archive]) -&gt; (ChainProgress, ChainProgress)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-progress</code> | notes | method layerx_mirror::publisher::Publisher::progress; no reference outside the defining file found by name match |

## mirror-publisher-publish

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-publish</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1828</code> | <code>pub fn publish(&amp;mut self, archive: &amp;Archive) -&gt; DurablePublicationReport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-publish</code> | notes | method layerx_mirror::publisher::Publisher::publish |

## mirror-publisher-retrieve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-retrieve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1914</code> | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; (MirrorRetrieval&lt;crate::ethereum::EthereumError&gt;, MirrorRetrieval&lt;crate::solana::SolanaError&gt;)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-retrieve</code> | notes | method layerx_mirror::publisher::Publisher::retrieve |

## mirror-publisher-solana-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-publisher-solana-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1910</code> | <code>pub fn solana_cursor(&amp;self) -&gt; MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-publisher-solana-cursor</code> | notes | method layerx_mirror::publisher::Publisher::solana_cursor |

## mirror-reconcile-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-reconcile-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:2031</code> | <code>pub fn reconcile_journal&lt;E&gt;(journal: MirrorCursor, archives: &amp;[Archive], retrieve: impl FnOnce(ArchiveCommitment) -&gt; MirrorRetrieval&lt;E&gt;) -&gt; Result&lt;(), ReconcileRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-reconcile-journal</code> | notes | fn layerx_mirror::publisher::reconcile_journal; no reference outside the defining file found by name match |

## mirror-reconcile-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-reconcile-refusal</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1942</code> | <code>pub enum ReconcileRefusal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-reconcile-refusal</code> | notes | enum layerx_mirror::publisher::ReconcileRefusal; no reference outside the defining file found by name match |
| <code>interface.11.rs-mirror-reconcile-refusal</code> | overridden_by | override.J48.9 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-reconcile-refusal-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1953</code> | <code>pub const fn code(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-reconcile-refusal-code</code> | notes | method layerx_mirror::publisher::ReconcileRefusal::code; no reference outside the defining file found by name match |

## mirror-remote-chain-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:84</code> | <code>pub struct RemoteChainSigner</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer</code> | notes | struct layerx_mirror::signer::RemoteChainSigner |

## mirror-remote-chain-signer-ethereum-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-ethereum-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:288</code> | <code>pub fn ethereum_address(&amp;self) -&gt; Result&lt;[u8; 20], SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-ethereum-address</code> | notes | method layerx_mirror::signer::RemoteChainSigner::ethereum_address |

## mirror-remote-chain-signer-key-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-key-handle</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:148</code> | <code>pub fn key_handle(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-key-handle</code> | notes | method layerx_mirror::signer::RemoteChainSigner::key_handle |

## mirror-remote-chain-signer-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:95</code> | <code>pub fn new(config: RemoteSignerConfig) -&gt; Result&lt;Self, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-new</code> | notes | method layerx_mirror::signer::RemoteChainSigner::new |

## mirror-remote-chain-signer-public-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-public-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:153</code> | <code>pub fn public_key(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-public-key</code> | notes | method layerx_mirror::signer::RemoteChainSigner::public_key |

## mirror-remote-chain-signer-sign-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-sign-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:162</code> | <code>pub fn sign_digest(&amp;self, policy_domain: &amp;[u8], digest: [u8; 32]) -&gt; Result&lt;ChainSignature, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-sign-digest</code> | notes | method layerx_mirror::signer::RemoteChainSigner::sign_digest |

## mirror-remote-chain-signer-sign-message

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-sign-message</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:210</code> | <code>pub fn sign_message(&amp;self, policy_domain: &amp;[u8], message: &amp;[u8]) -&gt; Result&lt;ChainSignature, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-remote-chain-signer-sign-message</code> | notes | method layerx_mirror::signer::RemoteChainSigner::sign_message |

## mirror-remote-signer-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-remote-signer-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:56</code> | <code>pub struct RemoteSignerConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-remote-signer-config</code> | notes | struct layerx_mirror::signer::RemoteSignerConfig |

## mirror-retrieval-report

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-retrieval-report</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1292</code> | <code>pub struct RetrievalReport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-retrieval-report</code> | notes | struct layerx_mirror::publisher::RetrievalReport; no reference outside the defining file found by name match |

## mirror-retrieval-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-retrieval-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1285</code> | <code>pub enum RetrievalState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-retrieval-state</code> | notes | enum layerx_mirror::publisher::RetrievalState |

## mirror-rpc-cluster

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:75</code> | <code>pub struct RpcCluster</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster</code> | notes | struct layerx_mirror::rpc::RpcCluster |

## mirror-rpc-cluster-broadcast

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-broadcast</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:191</code> | <code>pub fn broadcast(&amp;self, method: &amp;str, parameters: Value, expected_identity: &amp;str) -&gt; Result&lt;BroadcastResult, RpcError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-broadcast</code> | notes | method layerx_mirror::rpc::RpcCluster::broadcast |

## mirror-rpc-cluster-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-call</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:130</code> | <code>pub fn call(&amp;self, method: &amp;str, parameters: Value) -&gt; Result&lt;Value, RpcError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-call</code> | notes | method layerx_mirror::rpc::RpcCluster::call |

## mirror-rpc-cluster-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:87</code> | <code>pub fn new(config: &amp;RpcQuorumConfig) -&gt; Result&lt;Self, RpcError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-rpc-cluster-new</code> | notes | method layerx_mirror::rpc::RpcCluster::new |

## mirror-rpc-endpoint-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-endpoint-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:28</code> | <code>pub struct RpcEndpointConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-rpc-endpoint-config</code> | notes | struct layerx_mirror::rpc::RpcEndpointConfig; no reference outside the defining file found by name match |

## mirror-rpc-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:49</code> | <code>pub enum RpcError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-rpc-error</code> | notes | enum layerx_mirror::rpc::RpcError |

## mirror-rpc-quorum-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-rpc-quorum-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/rpc.rs:40</code> | <code>pub struct RpcQuorumConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-rpc-quorum-config</code> | notes | struct layerx_mirror::rpc::RpcQuorumConfig |

## mirror-run

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-run</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:175</code> | <code>pub fn run(config_path: &amp;Path) -&gt; Result&lt;(), RuntimeError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-run</code> | notes | fn layerx_mirror::runtime::run |

## mirror-runtime-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-runtime-config</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:29</code> | <code>pub struct RuntimeConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-runtime-config</code> | notes | struct layerx_mirror::runtime::RuntimeConfig; no reference outside the defining file found by name match |
| <code>interface.11.rs-mirror-runtime-config</code> | overridden_by | override.J48.7 |

Contract context <code>contract_context.11.mirror-archive</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-runtime-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:153</code> | <code>pub enum RuntimeError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-runtime-error</code> | notes | enum layerx_mirror::runtime::RuntimeError; no reference outside the defining file found by name match |

## mirror-signed-header-trust

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-signed-header-trust</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:21</code> | <code>pub struct SignedHeaderTrust</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-signed-header-trust</code> | notes | struct layerx_mirror::verify::SignedHeaderTrust |
| <code>interface.11.rs-mirror-signed-header-trust</code> | overridden_by | override.J48.23 |

## mirror-signer-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-signer-endpoint</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:42</code> | <code>pub enum SignerEndpoint</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-signer-endpoint</code> | notes | enum layerx_mirror::signer::SignerEndpoint |

## mirror-signer-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-signer-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:72</code> | <code>pub enum SignerError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-signer-error</code> | notes | enum layerx_mirror::signer::SignerError |

## mirror-signer-file-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-signer-file-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:95</code> | <code>pub struct SignerFileConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-signer-file-config</code> | notes | struct layerx_mirror::runtime::SignerFileConfig; no reference outside the defining file found by name match |

## mirror-signer-transport-file-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-signer-transport-file-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:104</code> | <code>pub enum SignerTransportFileConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-signer-transport-file-config</code> | notes | enum layerx_mirror::runtime::SignerTransportFileConfig; no reference outside the defining file found by name match |

## mirror-signing-algorithm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-signing-algorithm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:25</code> | <code>pub enum SigningAlgorithm</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-signing-algorithm</code> | notes | enum layerx_mirror::signer::SigningAlgorithm |

## mirror-solana-archive-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1108</code> | <code>pub trait SolanaArchiveClient</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client</code> | notes | trait layerx_mirror::publisher::SolanaArchiveClient |

## mirror-solana-archive-client-advance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-advance</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:371</code> | <code>pub fn advance(&amp;mut self, archive: &amp;Archive) -&gt; Result&lt;SolanaProgress, SolanaError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-advance</code> | notes | method layerx_mirror::solana::SolanaArchiveClient::advance |

## mirror-solana-archive-client-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:455</code> | <code>pub fn cursor(&amp;self) -&gt; MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-cursor</code> | notes | method layerx_mirror::solana::SolanaArchiveClient::cursor; chain calls: sendTransaction at :510 via private fn prepare_and_broadcast, sendTransaction at :556 via private fn observe_stage, getSignatureStatuses at :582 via private fn observe_stage, getTransaction at :635 via private fn observe_finality, getBlock at :667 via private fn observe_finality, getLatestBlockhash at :717 via private fn sign_transaction, getGenesisHash at :742 via private fn verify_target, getSlot at :781 via private fn reorg_monitoring_complete, getSlot at :806 via private fn verify_rooted_ancestry, getBlock at :819 via private fn verify_rooted_ancestry, getAccountInfo at :868 via private fn account |

## mirror-solana-archive-client-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:342</code> | <code>pub fn open(config: SolanaProductionConfig, signer: RemoteChainSigner) -&gt; Result&lt;Self, SolanaError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-open</code> | notes | method layerx_mirror::solana::SolanaArchiveClient::open |

## mirror-solana-archive-client-retrieve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-retrieve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:405</code> | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, SolanaError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-retrieve</code> | notes | method layerx_mirror::solana::SolanaArchiveClient::retrieve |

## mirror-solana-archive-client-retrieve-finalized

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-retrieve-finalized</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:440</code> | <code>pub fn retrieve_finalized(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;Vec&lt;u8&gt;&gt;, SolanaError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-client-retrieve-finalized</code> | notes | method layerx_mirror::solana::SolanaArchiveClient::retrieve_finalized |

## mirror-solana-archive-write

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-write</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1019</code> | <code>pub struct SolanaArchiveWrite&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-archive-write</code> | notes | struct layerx_mirror::publisher::SolanaArchiveWrite |

## mirror-solana-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:972</code> | <code>pub struct SolanaConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-config</code> | notes | struct layerx_mirror::publisher::SolanaConfig |

## mirror-solana-config-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-config-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:986</code> | <code>pub fn validate(self) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-config-validate</code> | notes | method layerx_mirror::publisher::SolanaConfig::validate; no reference outside the defining file found by name match |

## mirror-solana-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:292</code> | <code>pub enum SolanaError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-error</code> | notes | enum layerx_mirror::solana::SolanaError |

## mirror-solana-file-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-file-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs:80</code> | <code>pub struct SolanaFileConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-file-config</code> | notes | struct layerx_mirror::runtime::SolanaFileConfig; no reference outside the defining file found by name match |

## mirror-solana-lane-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-lane-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1777</code> | <code>pub enum SolanaLaneState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-lane-state</code> | notes | enum layerx_mirror::publisher::SolanaLaneState; no reference outside the defining file found by name match |

## mirror-solana-mirror-read

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-read</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:59</code> | <code>pub struct SolanaMirrorRead</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-read</code> | notes | struct layerx_mirror::solana::SolanaMirrorRead; no reference outside the defining file found by name match |

## mirror-solana-mirror-read-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-read-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:48</code> | <code>pub struct SolanaMirrorReadConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-read-config</code> | notes | struct layerx_mirror::solana::SolanaMirrorReadConfig |

## mirror-solana-mirror-reader

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:67</code> | <code>pub struct SolanaMirrorReader</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader</code> | notes | struct layerx_mirror::solana::SolanaMirrorReader |

## mirror-solana-mirror-reader-is-canonical

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-is-canonical</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:175</code> | <code>pub fn is_canonical(&amp;self, observation: &amp;SolanaMirrorRead) -&gt; Result&lt;bool, SolanaError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-is-canonical</code> | notes | method layerx_mirror::solana::SolanaMirrorReader::is_canonical; no reference outside the defining file found by name match |

## mirror-solana-mirror-reader-is-coordinate-canonical

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-is-coordinate-canonical</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:182</code> | <code>pub fn is_coordinate_canonical(&amp;self, rooted_slot: u64, rooted_blockhash: [u8; 32]) -&gt; Result&lt;bool, SolanaError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-is-coordinate-canonical</code> | notes | method layerx_mirror::solana::SolanaMirrorReader::is_coordinate_canonical; chain calls: getBlock at :188, getGenesisHash at :201 via private fn verify_cluster, getAccountInfo at :239 via private fn account_at |

## mirror-solana-mirror-reader-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:76</code> | <code>pub fn open(config: SolanaMirrorReadConfig) -&gt; Result&lt;Self, SolanaError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-open</code> | notes | method layerx_mirror::solana::SolanaMirrorReader::open |

## mirror-solana-mirror-reader-retrieve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-retrieve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:95</code> | <code>pub fn retrieve(&amp;self, commitment: ArchiveCommitment) -&gt; Result&lt;Option&lt;SolanaMirrorRead&gt;, SolanaError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-mirror-reader-retrieve</code> | notes | method layerx_mirror::solana::SolanaMirrorReader::retrieve; chain calls: getSlot at :102, getBlock at :156 |

## mirror-solana-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-observation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1060</code> | <code>pub enum SolanaObservation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-observation</code> | notes | enum layerx_mirror::publisher::SolanaObservation |

## mirror-solana-production-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-production-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:31</code> | <code>pub struct SolanaProductionConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-production-config</code> | notes | struct layerx_mirror::solana::SolanaProductionConfig |

## mirror-solana-progress

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-progress</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:282</code> | <code>pub struct SolanaProgress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-progress</code> | notes | struct layerx_mirror::solana::SolanaProgress |

## mirror-solana-solana-archive-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-solana-archive-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/solana.rs:330</code> | <code>pub struct SolanaArchiveClient</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-solana-archive-client</code> | notes | struct layerx_mirror::solana::SolanaArchiveClient |

## mirror-solana-submission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-solana-submission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:1038</code> | <code>pub struct SolanaSubmission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-solana-submission</code> | notes | struct layerx_mirror::publisher::SolanaSubmission |

## mirror-source-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-source-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:765</code> | <code>pub enum SourceError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-source-error</code> | notes | enum layerx_mirror::publisher::SourceError |

## mirror-spooled-archive

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-spooled-archive</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:374</code> | <code>pub struct SpooledArchive</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-spooled-archive</code> | notes | struct layerx_mirror::store::SpooledArchive; no reference outside the defining file found by name match |

## mirror-store-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-store-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:492</code> | <code>pub enum StoreError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-store-error</code> | notes | enum layerx_mirror::store::StoreError |

## mirror-tls-connector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-tls-connector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/signer.rs:304</code> | <code>pub fn tls_connector(server_name: &amp;str, trust_anchor: &amp;Path, client_certificate: &amp;Path, client_private_key: &amp;Path) -&gt; Result&lt;SslConnector, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-tls-connector</code> | notes | fn layerx_mirror::signer::tls_connector |

## mirror-transaction-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-transaction-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/store.rs:104</code> | <code>pub enum TransactionIdentity</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-transaction-identity</code> | notes | enum layerx_mirror::store::TransactionIdentity |

## mirror-verified-cursor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-verified-cursor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs:2010</code> | <code>pub fn verified_cursor(archives: &amp;[Archive]) -&gt; MirrorCursor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-verified-cursor</code> | notes | fn layerx_mirror::publisher::verified_cursor; no reference outside the defining file found by name match |

## mirror-verify-mirror-verification-freshness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-mirror-verify-mirror-verification-freshness</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs:106</code> | <code>pub const fn freshness(&amp;self) -&gt; MirrorVerificationFreshness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-mirror-verify-mirror-verification-freshness</code> | notes | method layerx_mirror::verify::MirrorVerification::freshness; no reference outside the defining file found by name match |
