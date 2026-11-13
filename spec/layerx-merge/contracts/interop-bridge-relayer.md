# Interop bridge relayer contract

The [bridge relayer](../../../services/interop/crates/layerx-bridge-relayer/src/lib.rs) observes bridge events, collects attestations and submits releases. It verifies Paxeer receipt signatures against configured key windows and reads EVM JSON-RPC from the node root.

## cfg-relayer-attestor-set-config-members

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-attestor-set-config-members</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer attestor set members: Vec&lt;String&gt; (required)</code> |

## cfg-relayer-attestor-set-config-threshold

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-attestor-set-config-threshold</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer attestor set threshold: u8 (required)</code> |

## cfg-relayer-chain-config-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer chain chain_id: u64 (required)</code> |

## cfg-relayer-chain-config-finality-depth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-finality-depth</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer chain finality_depth: u64 (required)</code> |

## cfg-relayer-chain-config-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-gas</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer chain gas: GasConfig (required)</code> |

## cfg-relayer-chain-config-max-block-range

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-max-block-range</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer chain max_block_range: u64 (required)</code> |

## cfg-relayer-chain-config-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-rpc</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer chain rpc: RpcQuorumConfig (required)</code> |

## cfg-relayer-chain-config-start-block

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-start-block</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer chain start_block: u64 (required)</code> |

## cfg-relayer-chain-config-submitter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-submitter</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer chain submitter: KeyHandleConfig (required)</code> |

## cfg-relayer-chain-config-vault

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-vault</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer chain vault: [u8; 20] (required)</code> |

## cfg-relayer-destination-policy-attestors

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-attestors</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator destination policy attestors: Vec&lt;String&gt; (required)</code> |

## cfg-relayer-destination-policy-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator destination policy chain_id: u64 (required)</code> |

## cfg-relayer-destination-policy-observed-block-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-observed-block-hash</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator destination policy observed_block_hash: String (required)</code> |

## cfg-relayer-destination-policy-threshold

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-threshold</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator destination policy threshold: usize (required)</code> |

## cfg-relayer-destination-policy-vault

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-vault</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator destination policy vault: String (required)</code> |

## cfg-relayer-gas-config-gas-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-gas-limit</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer gas gas_limit: u64 (required)</code> |

## cfg-relayer-gas-config-max-fee-per-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-max-fee-per-gas</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer gas max_fee_per_gas: u128 (required)</code> |

## cfg-relayer-gas-config-max-priority-fee-per-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-max-priority-fee-per-gas</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer gas max_priority_fee_per_gas: u128 (required)</code> |

## cfg-relayer-key-handle-config-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-key-handle-config-handle</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer key handle handle: String (required)</code> |

## cfg-relayer-key-handle-config-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-key-handle-config-public-key</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer key handle public_key: Vec&lt;u8&gt; (required)</code> |

## cfg-relayer-membership-attestors

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-membership-attestors</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator membership attestors: Vec&lt;String&gt; (required)</code> |

## cfg-relayer-membership-threshold

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-membership-threshold</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator membership threshold: usize (required)</code> |

## cfg-relayer-operator-fee-payer-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator fee payer chain_id: u64 (required)</code> |

## cfg-relayer-operator-fee-payer-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-handle</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator fee payer handle: String (required)</code> |

## cfg-relayer-operator-fee-payer-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-public-key</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator fee payer public_key: String (required)</code> |

## cfg-relayer-operator-inventory-approval

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-approval</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator inventory approval: String (required)</code> |

## cfg-relayer-operator-inventory-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-authority</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator inventory authority: String (required)</code> |

## cfg-relayer-operator-inventory-destinations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-destinations</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator inventory destinations: Vec&lt;DestinationPolicy&gt; (required)</code> |

## cfg-relayer-operator-inventory-membership

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-membership</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator inventory membership: PathBuf (required)</code> |

## cfg-relayer-operator-inventory-operators

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-operators</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator inventory operators: Vec&lt;OperatorRecord&gt; (required)</code> |

## cfg-relayer-operator-inventory-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-version</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator inventory version: u16 (required)</code> |

## cfg-relayer-operator-record-attestor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-attestor</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record attestor: String (required)</code> |

## cfg-relayer-operator-record-cosign-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-cosign-directory</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record cosign_directory: PathBuf (required)</code> |

## cfg-relayer-operator-record-delivery-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-delivery-directory</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record delivery_directory: PathBuf (required)</code> |

## cfg-relayer-operator-record-fee-payers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-fee-payers</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record fee_payers: Vec&lt;OperatorFeePayer&gt; (required)</code> |

## cfg-relayer-operator-record-instance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-instance</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record instance: String (required)</code> |

## cfg-relayer-operator-record-journal-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-journal-path</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record journal_path: PathBuf (required)</code> |

## cfg-relayer-operator-record-signer-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-signer-handle</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record signer_handle: String (required)</code> |

## cfg-relayer-operator-record-signer-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-signer-public-key</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record signer_public_key: String (required)</code> |

## cfg-relayer-operator-record-spki-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-spki-sha256</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record spki_sha256: String (required)</code> |

## cfg-relayer-operator-record-storage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-storage</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>bridge operator record storage: String (required)</code> |

## cfg-relayer-paxeer-config-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer chain_id: u64 (required)</code> |

## cfg-relayer-paxeer-config-endpoints

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-endpoints</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer endpoints: Vec&lt;PaxeerEndpointConfig&gt; (required)</code> |

## cfg-relayer-paxeer-config-finality-depth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-finality-depth</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer finality_depth: u64 (required)</code> |

## cfg-relayer-paxeer-config-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-gas</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer gas: GasConfig (required)</code> |

## cfg-relayer-paxeer-config-max-block-range

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-max-block-range</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer max_block_range: u64 (required)</code> |

## cfg-relayer-paxeer-config-start-block

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-start-block</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer start_block: u64 (required)</code> |

## cfg-relayer-paxeer-config-submitter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-submitter</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer submitter: KeyHandleConfig (required)</code> |

## cfg-relayer-paxeer-endpoint-config-local-emulator

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-local-emulator</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer endpoint local_emulator: bool (optional)</code> |

## cfg-relayer-paxeer-endpoint-config-request-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-request-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer endpoint request_timeout_ms: u64 (required)</code> |

## cfg-relayer-paxeer-endpoint-config-trust-anchor-der

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-trust-anchor-der</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer endpoint trust_anchor_der: Option&lt;PathBuf&gt; (optional)</code> |

## cfg-relayer-paxeer-endpoint-config-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-url</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer paxeer endpoint url: String (required)</code> |

## cfg-relayer-peer-config-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-address</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport peer address: SocketAddr (required)</code> |

## cfg-relayer-peer-config-attestor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-attestor</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport peer attestor: String (required)</code> |

## cfg-relayer-peer-config-server-name

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-server-name</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport peer server_name: String (required)</code> |

## cfg-relayer-peer-config-spki-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-spki-sha256</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport peer spki_sha256: String (required)</code> |

## cfg-relayer-receipt-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-receipt-keys</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config receipt_keys: [{sequencer_id, public_key, first_height, last_height}] (public keys only)</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## cfg-relayer-relayer-config-attestor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-attestor</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config attestor: KeyHandleConfig (required)</code> |

## cfg-relayer-relayer-config-attestor-set

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-attestor-set</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config attestor_set: Option&lt;AttestorSetConfig&gt; (optional)</code> |

## cfg-relayer-relayer-config-chains

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-chains</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config chains: Vec&lt;ChainConfig&gt; (required)</code> |

## cfg-relayer-relayer-config-cosign-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-cosign-directory</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config cosign_directory: Option&lt;PathBuf&gt; (optional)</code> |

## cfg-relayer-relayer-config-journal-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-journal-path</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config journal_path: PathBuf (required)</code> |

## cfg-relayer-relayer-config-max-submissions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-max-submissions</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config max_submissions: u32 (required)</code> |

## cfg-relayer-relayer-config-paxeer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-paxeer</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config paxeer: PaxeerConfig (required)</code> |

## cfg-relayer-relayer-config-poll-interval-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-poll-interval-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config poll_interval_ms: u64 (required)</code> |

## cfg-relayer-relayer-config-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-signer</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config signer: SignerConfig (required)</code> |

## cfg-relayer-relayer-config-solana

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-solana</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config solana: Option&lt;SolanaConfig&gt; (optional)</code> |

## cfg-relayer-signer-config-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-signer-config-endpoint</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer signer endpoint: SignerTransportConfig (required)</code> |

## cfg-relayer-signer-config-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-signer-config-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer signer timeout_ms: u64 (required)</code> |

## cfg-relayer-signer-transport-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-signer-transport-config</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer signer endpoint: one of "uds" &#124; "mutual_tls" (tag transport)</code> |

## cfg-relayer-solana-config-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana chain_id: u64 (required)</code> |

## cfg-relayer-solana-config-commitment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-commitment</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana commitment: Commitment (required)</code> |

## cfg-relayer-solana-config-fee-payer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-fee-payer</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana fee_payer: KeyHandleConfig (required)</code> |

## cfg-relayer-solana-config-finality-depth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-finality-depth</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana finality_depth: u64 (required)</code> |

## cfg-relayer-solana-config-max-slot-range

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-max-slot-range</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana max_slot_range: u64 (required)</code> |

## cfg-relayer-solana-config-program-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-program-id</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana program_id: String (required)</code> |

## cfg-relayer-solana-config-release-mints

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-release-mints</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana release_mints: Vec&lt;String&gt; (optional)</code> |

## cfg-relayer-solana-config-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-rpc</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana rpc: RpcQuorumConfig (required)</code> |

## cfg-relayer-solana-config-start-slot

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-start-slot</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana start_slot: u64 (required)</code> |

## cfg-relayer-solana-config-vault

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-vault</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer solana vault: [u8; 20] (required)</code> |

## cfg-relayer-transport-config-attestor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-attestor</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config attestor: String (required)</code> |

## cfg-relayer-transport-config-certificate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-certificate</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config certificate: PathBuf (required)</code> |

## cfg-relayer-transport-config-cosign-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-cosign-directory</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config cosign_directory: PathBuf (required)</code> |

## cfg-relayer-transport-config-delivery-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-delivery-directory</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config delivery_directory: PathBuf (required)</code> |

## cfg-relayer-transport-config-listen

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-listen</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config listen: SocketAddr (required)</code> |

## cfg-relayer-transport-config-max-connections

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-max-connections</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config max_connections: usize (required)</code> |

## cfg-relayer-transport-config-peers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-peers</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config peers: Vec&lt;PeerConfig&gt; (required)</code> |

## cfg-relayer-transport-config-private-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-private-key</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config private_key: PathBuf (required)</code> |

## cfg-relayer-transport-config-server-name

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-server-name</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config server_name: String (required)</code> |

## cfg-relayer-transport-config-sweep-interval-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-sweep-interval-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config sweep_interval_ms: u64 (required)</code> |

## cfg-relayer-transport-config-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-timeout-ms</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config timeout_ms: u64 (required)</code> |

## cfg-relayer-transport-config-trust-anchor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-trust-anchor</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign transport config trust_anchor: PathBuf (required)</code> |

## cli-relayer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-relayer</code> | <code>cli_command</code> | [services/interop/crates/layerx-bridge-relayer/src/main.rs](../../../services/interop/crates/layerx-bridge-relayer/src/main.rs) | <code>layerx-bridge-relayer --config PATH</code> |

## cli-relayer-cosign

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-relayer-cosign</code> | <code>cli_command</code> | [services/interop/crates/layerx-bridge-relayer/src/bin/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/bin/cosign.rs) | <code>layerx-bridge-cosign (no arguments; reads LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG)</code> |

## env-relayer-layerx-bridge-cosign-transport-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-relayer-layerx-bridge-cosign-transport-config</code> | <code>env_var</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG</code> |

## env-relayer-layerx-bridge-operator-inventory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-relayer-layerx-bridge-operator-inventory</code> | <code>env_var</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>LAYERX_BRIDGE_OPERATOR_INVENTORY</code> |

## env-relayer-layerx-bridge-relayer-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-relayer-layerx-bridge-relayer-config</code> | <code>env_var</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>LAYERX_BRIDGE_RELAYER_CONFIG</code> |

## fmt-relayer-bridge-in-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-bridge-in-domain</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>attestation preimage prefix "PAXEERX_BRIDGE_IN_V1" (20 bytes) for inbound bridge attestations</code> |

## fmt-relayer-bridge-out-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-bridge-out-domain</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>attestation preimage prefix "PAXEERX_BRIDGE_OUT_V1" (21 bytes) for outbound bridge releases</code> |

## fmt-relayer-commitment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-commitment</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>Commitment = confirmed &#124; finalized</code> |

## fmt-relayer-commitment-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-commitment-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>Commitment::as_str() -&gt; "confirmed" &#124; "finalized"</code> |

## fmt-relayer-completion

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-completion</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>Completion = included &#124; already_bridged &#124; released (tag outcome)</code> |

## fmt-relayer-cosign-frame

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-cosign-frame</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>cosign share frame: u32be length 143 &#124; "LXSH" &#124; version:u16be=1 &#124; recipient:20 &#124; digest:32 &#124; signer:20 &#124; signature:65; reply byte 1 stored, 2 already held, 3 conflict</code> |

## fmt-relayer-entry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-entry</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>Entry = cursor &#124; observed &#124; signed &#124; submitted &#124; receipted &#124; reverted &#124; dropped &#124; completed &#124; refused &#124; recipient_pending &#124; recipient_resolved &#124; release_submitted &#124; release_expired &#124; release_failed (tag kind)</code> |

## fmt-relayer-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-observation</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>Observation = inbound &#124; outbound (tag direction)</code> |

## fmt-relayer-position

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-relayer-position</code> | <code>file_format</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct Position { pub block_number: u64, #[serde(with = "hex::fixed_serde")] pub block_hash: [u8; 32], }</code> |

## relayer-abi-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-abi-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub enum AbiError</code> |

## relayer-account-data

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-account-data</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub struct AccountData</code> |

## relayer-address-word

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-address-word</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn address_word(address: &amp;[u8; 20]) -&gt; [u8; 32]</code> |

## relayer-admission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-admission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub enum Admission</code> |

## relayer-algorithm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-algorithm</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const ALGORITHM: SigningAlgorithm</code> |

## relayer-assemble

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-assemble</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/tx.rs](../../../services/interop/crates/layerx-bridge-relayer/src/tx.rs) | <code>pub fn assemble(call: &amp;Eip1559Call, signature: &amp;[u8; 65]) -&gt; SignedTransaction</code> |

## relayer-assemble-signatures

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-assemble-signatures</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn assemble_signatures&lt;I&gt;(digest: &amp;[u8; 32], candidates: I, attestors: &amp;[[u8; 20]], threshold: usize) -&gt; Result&lt;Vec&lt;[u8; 65]&gt;, SignatureError&gt; where I: IntoIterator&lt;Item = [u8; 65]&gt;,</code> |

## relayer-asset-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-asset-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn asset_address(program_id: &amp;[u8; 32], mint: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

## relayer-asset-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-asset-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const ASSET_BYTES: usize</code> |

## relayer-asset-magic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-asset-magic</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const ASSET_MAGIC: &amp;[u8; 8]</code> |

## relayer-asset-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-asset-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub struct AssetRecord</code> |

## relayer-asset-record-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-asset-record-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Option&lt;Self&gt;</code> |

## relayer-asset-seed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-asset-seed</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const ASSET_SEED: &amp;[u8]</code> |

## relayer-associated-token-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-associated-token-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn associated_token_address(wallet: &amp;[u8; 32], mint: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

## relayer-associated-token-program

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-associated-token-program</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const ASSOCIATED_TOKEN_PROGRAM: [u8; 32]</code> |

## relayer-attest-inbound-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attest-inbound-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const ATTEST_INBOUND_DOMAIN: &amp;[u8]</code> |

## relayer-attest-outbound-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attest-outbound-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const ATTEST_OUTBOUND_DOMAIN: &amp;[u8]</code> |

## relayer-attestor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub struct Attestor</code> |

## relayer-attestor-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const fn address(&amp;self) -&gt; [u8; 20]</code> |

## relayer-attestor-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn new(signer: RemoteChainSigner) -&gt; Result&lt;Self, SignerError&gt;</code> |

## relayer-attestor-set

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-set</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub struct AttestorSet</code> |

## relayer-attestor-set-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct AttestorSetConfig</code> |

## relayer-attestor-set-config-addresses

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-config-addresses</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub fn addresses(&amp;self) -&gt; Result&lt;Vec&lt;[u8; 20]&gt;, RelayerError&gt;</code> |

## relayer-attestor-set-size

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-size</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub const ATTESTOR_SET_SIZE: usize</code> |

## relayer-attestor-sign-inbound

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-sign-inbound</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn sign_inbound(&amp;self, attestation: &amp;InboundAttestation) -&gt; Result&lt;[u8; 65], KeyError&gt;</code> |

## relayer-attestor-sign-outbound

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-sign-outbound</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn sign_outbound(&amp;self, attestation: &amp;OutboundAttestation) -&gt; Result&lt;[u8; 65], KeyError&gt;</code> |

## relayer-attestor-threshold

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-threshold</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub const ATTESTOR_THRESHOLD: u8</code> |

## relayer-attestors-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-attestors-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const ATTESTORS_SELECTOR: [u8; 4]</code> |

## relayer-base58-data

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-base58-data</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub fn base58_data(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, CodecError&gt;</code> |

## relayer-base58-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-base58-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub fn base58_decode(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, CodecError&gt;</code> |

## relayer-base58-encode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-base58-encode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub fn base58_encode(bytes: &amp;[u8]) -&gt; String</code> |

## relayer-base58-fixed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-base58-fixed</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub fn base58_fixed&lt;const N: usize&gt;(text: &amp;str) -&gt; Result&lt;[u8; N], CodecError&gt;</code> |

## relayer-base64-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-base64-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub fn base64_decode(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, CodecError&gt;</code> |

## relayer-base64-encode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-base64-encode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub fn base64_encode(bytes: &amp;[u8]) -&gt; String</code> |

## relayer-blockhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-blockhash</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub struct Blockhash</code> |

## relayer-bridge-deposit-topic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-bridge-deposit-topic</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const BRIDGE_DEPOSIT_TOPIC: [u8; 32]</code> |

## relayer-bridge-in-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-bridge-in-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const BRIDGE_IN_SELECTOR: [u8; 4]</code> |

## relayer-bridge-out-topic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-bridge-out-topic</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const BRIDGE_OUT_TOPIC: [u8; 32]</code> |

## relayer-build-release-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-build-release-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn build_release_transaction(release: &amp;Release, recent_blockhash: &amp;[u8; 32]) -&gt; Result&lt;Vec&lt;u8&gt;, ReleaseError&gt;</code> |

## relayer-burn-log

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-burn-log</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub struct BurnLog</code> |

## relayer-burn-log-attestation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-burn-log-attestation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const fn attestation(&amp;self, vault: [u8; 20]) -&gt; OutboundAttestation</code> |

## relayer-chain-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-chain-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct ChainConfig</code> |

## relayer-chain-link

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-chain-link</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct ChainLink</code> |

## relayer-chain-registration

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-chain-registration</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub struct ChainRegistration</code> |

## relayer-chain-settings

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-chain-settings</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct ChainSettings</code> |

## relayer-codec-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-codec-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub struct CodecError</code> |

## relayer-commitment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-commitment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub enum Commitment</code> |

## relayer-commitment-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-commitment-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub const fn as_str(self) -&gt; &amp;'static str</code> |

## relayer-completion

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-completion</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub enum Completion</code> |

## relayer-config-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-config-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn config_address(program_id: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

## relayer-config-attestors-offset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-config-attestors-offset</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const CONFIG_ATTESTORS_OFFSET: usize</code> |

## relayer-config-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-config-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const CONFIG_BYTES: usize</code> |

## relayer-config-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-config-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct ConfigError(pub String)</code> |

## relayer-config-magic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-config-magic</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const CONFIG_MAGIC: &amp;[u8; 8]</code> |

## relayer-config-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-config-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub struct ConfigRecord</code> |

## relayer-config-record-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-config-record-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Option&lt;Self&gt;</code> |

## relayer-config-seed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-config-seed</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const CONFIG_SEED: &amp;[u8]</code> |

## relayer-confirmation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-confirmation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub enum Confirmation</code> |

## relayer-confirmation-reaches

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-confirmation-reaches</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub const fn reaches(self, commitment: Commitment) -&gt; bool</code> |

## relayer-cosign-directory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct CosignDirectory</code> |

## relayer-cosign-directory-admit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-admit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn admit(&amp;self, digest: &amp;[u8; 32], signer: &amp;[u8; 20], signature: &amp;[u8; 65]) -&gt; Result&lt;Admission, JournalError&gt;</code> |

## relayer-cosign-directory-collect

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-collect</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn collect(&amp;self, digest: &amp;[u8; 32]) -&gt; Vec&lt;[u8; 65]&gt;</code> |

## relayer-cosign-directory-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const fn new(root: PathBuf) -&gt; Self</code> |

## relayer-cosign-directory-own-shares

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-own-shares</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn own_shares(&amp;self, signer: &amp;[u8; 20]) -&gt; Vec&lt;([u8; 32], [u8; 65])&gt;</code> |

## relayer-cosign-directory-publish

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-publish</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn publish(&amp;self, digest: &amp;[u8; 32], signer: &amp;[u8; 20], signature: &amp;[u8; 65]) -&gt; Result&lt;(), JournalError&gt;</code> |

## relayer-create-program-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-create-program-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn create_program_address(seeds: &amp;[&amp;[u8]], program_id: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

## relayer-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn decode(value: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, HexError&gt;</code> |

## relayer-decode-address-list

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode-address-list</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn decode_address_list(data: &amp;[u8]) -&gt; Result&lt;Vec&lt;[u8; 20]&gt;, AbiError&gt;</code> |

## relayer-decode-bool

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode-bool</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn decode_bool(data: &amp;[u8]) -&gt; Result&lt;bool, AbiError&gt;</code> |

## relayer-decode-burn-log

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode-burn-log</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn decode_burn_log(value: &amp;Value) -&gt; Result&lt;BurnLog, AbiError&gt;</code> |

## relayer-decode-deposit-log

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode-deposit-log</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn decode_deposit_log(value: &amp;Value, vault: &amp;[u8; 20]) -&gt; Result&lt;DepositLog, AbiError&gt;</code> |

## relayer-decode-frame

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode-frame</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn decode_frame(frame: &amp;[u8; FRAME_LENGTH]) -&gt; Result&lt;Share, String&gt;</code> |

## relayer-decode-get-attestors

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode-get-attestors</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn decode_get_attestors(data: &amp;[u8]) -&gt; Result&lt;AttestorSet, AbiError&gt;</code> |

## relayer-decode-get-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode-get-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn decode_get_chain(data: &amp;[u8]) -&gt; Result&lt;ChainRegistration, AbiError&gt;</code> |

## relayer-decode-threshold

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-decode-threshold</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn decode_threshold(data: &amp;[u8]) -&gt; Result&lt;u64, AbiError&gt;</code> |

## relayer-delivery-workers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-delivery-workers</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const DELIVERY_WORKERS: usize</code> |

## relayer-deposit-data-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-data-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const DEPOSIT_DATA_BYTES: usize</code> |

## relayer-deposit-instruction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-instruction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub fn deposit_instruction(data: &amp;[u8]) -&gt; Option&lt;(u64, [u8; 32])&gt;</code> |

## relayer-deposit-log

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-log</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub struct DepositLog</code> |

## relayer-deposit-log-attestation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-log-attestation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const fn attestation(&amp;self, chain_id: u64, vault: [u8; 20]) -&gt; InboundAttestation</code> |

## relayer-deposit-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-receipt</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub struct DepositReceipt</code> |

## relayer-deposit-receipt-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-receipt-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Option&lt;Self&gt;</code> |

## relayer-deposit-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const DEPOSIT_RECORD: &amp;str</code> |

## relayer-deposit-record-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-record-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub fn parse(line: &amp;str) -&gt; Option&lt;Self&gt;</code> |

## relayer-deposit-records

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-records</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub fn deposit_records(logs: &amp;[String], program: &amp;[u8; 32]) -&gt; Result&lt;Vec&lt;DepositRecord&gt;, RelayerError&gt;</code> |

## relayer-deserialize

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-deserialize</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn deserialize&lt;'de, D: Deserializer&lt;'de&gt;, const N: usize&gt;(deserializer: D) -&gt; Result&lt;[u8; N], D::Error&gt;</code> |

## relayer-destination-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-destination-policy</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct DestinationPolicy</code> |

## relayer-disagreement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-disagreement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const DISAGREEMENT: &amp;str</code> |

## relayer-domain-in

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-domain-in</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub const DOMAIN_IN: &amp;[u8; 20]</code> |

## relayer-domain-out

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-domain-out</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub const DOMAIN_OUT: &amp;[u8; 21]</code> |

## relayer-enable-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-enable-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const ENABLE_VARIABLE: &amp;str</code> |

## relayer-encode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-encode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn encode(bytes: &amp;[u8]) -&gt; String</code> |

## relayer-encode-bridge-in

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-encode-bridge-in</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn encode_bridge_in(attestation: &amp;InboundAttestation, signatures: &amp;[[u8; 65]]) -&gt; Vec&lt;u8&gt;</code> |

## relayer-encode-get-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-encode-get-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn encode_get_chain(chain_id: u64) -&gt; Vec&lt;u8&gt;</code> |

## relayer-encode-is-nullified

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-encode-is-nullified</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn encode_is_nullified(chain_id: u64, tx_hash: &amp;[u8; 32], log_index: u64) -&gt; Vec&lt;u8&gt;</code> |

## relayer-encode-nullified

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-encode-nullified</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn encode_nullified(nullifier: &amp;[u8; 32]) -&gt; Vec&lt;u8&gt;</code> |

## relayer-encode-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-encode-release</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub fn encode_release(attestation: &amp;OutboundAttestation, signatures: &amp;[[u8; 65]]) -&gt; Vec&lt;u8&gt;</code> |

## relayer-encode-wire

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-encode-wire</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn encode_wire(recipient: &amp;[u8; 20], digest: &amp;[u8; 32], signer: &amp;[u8; 20], signature: &amp;[u8; 65]) -&gt; Vec&lt;u8&gt;</code> |

## relayer-entry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-entry</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub enum Entry</code> |

## relayer-ethereum-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-ethereum-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn ethereum_address(key: &amp;VerifyingKey) -&gt; [u8; 20]</code> |

## relayer-ethereum-transaction-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-ethereum-transaction-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const ETHEREUM_TRANSACTION_DOMAIN: &amp;[u8]</code> |

## relayer-fee-payer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub struct FeePayer</code> |

## relayer-fee-payer-algorithm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-algorithm</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const FEE_PAYER_ALGORITHM: SigningAlgorithm</code> |

## relayer-fee-payer-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn new(signer: RemoteChainSigner) -&gt; Result&lt;Self, SignerError&gt;</code> |

## relayer-fee-payer-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-public-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const fn public_key(&amp;self) -&gt; [u8; 32]</code> |

## relayer-fee-payer-sign-message

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-sign-message</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn sign_message(&amp;self, message: &amp;[u8]) -&gt; Result&lt;[u8; 64], KeyError&gt;</code> |

## relayer-find-program-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-find-program-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn find_program_address(seeds: &amp;[&amp;[u8]], program_id: &amp;[u8; 32]) -&gt; Option&lt;([u8; 32], u8)&gt;</code> |

## relayer-finding

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-finding</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub enum Finding</code> |

## relayer-fixed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-fixed</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn fixed&lt;const N: usize&gt;(value: &amp;str) -&gt; Result&lt;[u8; N], HexError&gt;</code> |

## relayer-frame-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-frame-length</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const FRAME_LENGTH: usize</code> |

## relayer-gas-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-gas-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct GasConfig</code> |

## relayer-gas-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-gas-policy</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct GasPolicy</code> |

## relayer-get-attestors-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-get-attestors-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const GET_ATTESTORS_SELECTOR: [u8; 4]</code> |

## relayer-get-chain-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-get-chain-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const GET_CHAIN_SELECTOR: [u8; 4]</code> |

## relayer-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-handle</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub fn handle(key: &amp;[u8; 32]) -&gt; [u8; HANDLE_BYTES]</code> |

## relayer-handle-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-handle-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub const HANDLE_BYTES: usize</code> |

## relayer-hex-bytes-serde-deserialize

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-hex-bytes-serde-deserialize</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn deserialize&lt;'de, D: Deserializer&lt;'de&gt;&gt;(deserializer: D) -&gt; Result&lt;Vec&lt;u8&gt;, D::Error&gt;</code> |

## relayer-hex-bytes-serde-serialize

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-hex-bytes-serde-serialize</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn serialize&lt;S: Serializer&gt;(value: &amp;[u8], serializer: S) -&gt; Result&lt;S::Ok, S::Error&gt;</code> |

## relayer-hex-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-hex-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub struct HexError</code> |

## relayer-in-preimage-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-in-preimage-length</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub const IN_PREIMAGE_LENGTH: usize</code> |

## relayer-inbound-attestation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub struct InboundAttestation</code> |

## relayer-inbound-attestation-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn digest(&amp;self) -&gt; [u8; 32]</code> |

## relayer-inbound-attestation-paxeer-recipient

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-paxeer-recipient</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn paxeer_recipient(&amp;self) -&gt; Option&lt;[u8; 20]&gt;</code> |

## relayer-inbound-attestation-preimage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-preimage</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn preimage(&amp;self) -&gt; [u8; IN_PREIMAGE_LENGTH]</code> |

## relayer-inbound-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn inbound_key(chain_id: u64, tx_hash: &amp;[u8; 32], log_index: u64) -&gt; String</code> |

## relayer-inbound-stream

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-stream</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub fn inbound_stream(chain_id: u64) -&gt; String</code> |

## relayer-inbound-tx-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-tx-hash</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub fn inbound_tx_hash(signature: &amp;[u8; 64]) -&gt; [u8; 32]</code> |

## relayer-instruction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-instruction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub struct Instruction</code> |

## relayer-instruction-magic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-instruction-magic</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const INSTRUCTION_MAGIC: &amp;[u8; 4]</code> |

## relayer-instruction-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-instruction-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const INSTRUCTION_VERSION: u16</code> |

## relayer-instructions-sysvar

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-instructions-sysvar</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const INSTRUCTIONS_SYSVAR: [u8; 32]</code> |

## relayer-inventory-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-inventory-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const INVENTORY_VARIABLE: &amp;str</code> |

## relayer-is-nullified-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-is-nullified-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const IS_NULLIFIED_SELECTOR: [u8; 4]</code> |

## relayer-is-token-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-is-token-account</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn is_token_account(data: &amp;[u8], mint: &amp;[u8; 32], owner: &amp;[u8; 32]) -&gt; bool</code> |

## relayer-item

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-item</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub struct Item</code> |

## relayer-item-failure

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-item-failure</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct ItemFailure</code> |

## relayer-item-is-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-item-is-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub const fn is_open(&amp;self) -&gt; bool</code> |

## relayer-item-pending

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-item-pending</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn pending(&amp;self) -&gt; Option&lt;&amp;Submission&gt;</code> |

## relayer-item-pending-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-item-pending-release</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn pending_release(&amp;self) -&gt; Option&lt;&amp;ReleaseSubmission&gt;</code> |

## relayer-item-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-item-transactions</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn transactions(&amp;self) -&gt; usize</code> |

## relayer-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub struct Journal</code> |

## relayer-journal-append

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-journal-append</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn append(&amp;mut self, entry: &amp;Entry) -&gt; Result&lt;(), JournalError&gt;</code> |

## relayer-journal-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-journal-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub enum JournalError</code> |

## relayer-journal-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-journal-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn open(path: &amp;Path) -&gt; Result&lt;Self, JournalError&gt;</code> |

## relayer-journal-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-journal-path</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn path(&amp;self) -&gt; &amp;Path</code> |

## relayer-journal-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-journal-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub const fn state(&amp;self) -&gt; &amp;State</code> |

## relayer-json-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-json-rpc</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/rpc.rs) | <code>pub trait JsonRpc</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-key-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-key-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub enum KeyError</code> |

## relayer-key-handle-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-key-handle-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct KeyHandleConfig</code> |

## relayer-layerx-bridge-precompile

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-layerx-bridge-precompile</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const LAYERX_BRIDGE_PRECOMPILE: [u8; 20]</code> |

## relayer-layout-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-layout-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const LAYOUT_VERSION: u16</code> |

## relayer-log-position

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-log-position</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub struct LogPosition</code> |

## relayer-magic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-magic</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const MAGIC: [u8; 4]</code> |

## relayer-max-attestors

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-attestors</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const MAX_ATTESTORS: usize</code> |

## relayer-max-connections

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-connections</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const MAX_CONNECTIONS: usize</code> |

## relayer-max-interval-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-interval-ms</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const MAX_INTERVAL_MS: u64</code> |

## relayer-max-peers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-peers</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const MAX_PEERS: usize</code> |

## relayer-max-processing-age

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-processing-age</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const MAX_PROCESSING_AGE: u64</code> |

## relayer-max-signature-page

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-signature-page</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub const MAX_SIGNATURE_PAGE: usize</code> |

## relayer-max-sweep-entries

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-sweep-entries</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const MAX_SWEEP_ENTRIES: usize</code> |

## relayer-max-timeout-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-timeout-ms</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const MAX_TIMEOUT_MS: u64</code> |

## relayer-max-transaction-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-max-transaction-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub const MAX_TRANSACTION_BYTES: usize</code> |

## relayer-nullified-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-nullified-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const NULLIFIED_SELECTOR: [u8; 4]</code> |

## relayer-nullifier-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-nullifier-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn nullifier_address(program_id: &amp;[u8; 32], nullifier: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

## relayer-nullifier-seed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-nullifier-seed</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const NULLIFIER_SEED: &amp;[u8]</code> |

## relayer-observation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-observation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub enum Observation</code> |

## relayer-observation-inbound

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-observation-inbound</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn inbound(attestation: &amp;InboundAttestation, position: Position) -&gt; Self</code> |

## relayer-observation-inbound-attestation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-observation-inbound-attestation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub const fn inbound_attestation(&amp;self) -&gt; Option&lt;InboundAttestation&gt;</code> |

## relayer-observation-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-observation-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn key(&amp;self) -&gt; String</code> |

## relayer-observation-outbound

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-observation-outbound</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn outbound(attestation: &amp;OutboundAttestation, position: Position) -&gt; Self</code> |

## relayer-observation-outbound-attestation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-observation-outbound-attestation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub const fn outbound_attestation(&amp;self) -&gt; Option&lt;OutboundAttestation&gt;</code> |

## relayer-observe-deposits

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-observe-deposits</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub fn observe_deposits(rpc: &amp;SolanaRpc, settings: &amp;SolanaSettings, from: u64, to: u64) -&gt; Result&lt;Vec&lt;Finding&gt;, RelayerError&gt;</code> |

## relayer-op-deposit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-op-deposit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const OP_DEPOSIT: u8</code> |

## relayer-op-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-op-release</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const OP_RELEASE: u8</code> |

## relayer-operator-fee-payer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-operator-fee-payer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct OperatorFeePayer</code> |

## relayer-operator-inventory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct OperatorInventory</code> |

## relayer-operator-inventory-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory-load</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## relayer-operator-inventory-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ConfigError&gt;</code> |

## relayer-operator-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-operator-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct OperatorRecord</code> |

## relayer-out-preimage-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-out-preimage-length</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub const OUT_PREIMAGE_LENGTH: usize</code> |

## relayer-outbound-attestation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub struct OutboundAttestation</code> |

## relayer-outbound-attestation-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn digest(&amp;self) -&gt; [u8; 32]</code> |

## relayer-outbound-attestation-nullifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-nullifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn nullifier(&amp;self) -&gt; [u8; 32]</code> |

## relayer-outbound-attestation-preimage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-preimage</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn preimage(&amp;self) -&gt; [u8; OUT_PREIMAGE_LENGTH]</code> |

## relayer-outbound-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub fn outbound_key(chain_id: u64, paxeer_tx_hash: &amp;[u8; 32], nonce: u64) -&gt; String</code> |

## relayer-outbound-preimage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-preimage</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn outbound_preimage(attestation: &amp;OutboundAttestation) -&gt; [u8; OUTBOUND_PREIMAGE_BYTES]</code> |

## relayer-outbound-preimage-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-preimage-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const OUTBOUND_PREIMAGE_BYTES: usize</code> |

## relayer-outbound-stream

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-stream</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub const OUTBOUND_STREAM: &amp;str</code> |

## relayer-parse-quantity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-parse-quantity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn parse_quantity(value: &amp;str) -&gt; Result&lt;u64, HexError&gt;</code> |

## relayer-parse-quantity-u128

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-parse-quantity-u128</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn parse_quantity_u128(value: &amp;str) -&gt; Result&lt;u128, HexError&gt;</code> |

## relayer-paxeer-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct PaxeerConfig</code> |

## relayer-paxeer-endpoint-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-endpoint-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct PaxeerEndpointConfig</code> |

## relayer-paxeer-link

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-link</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct PaxeerLink</code> |

## relayer-paxeer-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-rpc</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/rpc.rs) | <code>pub struct PaxeerRpc</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-paxeer-rpc-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-rpc-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/rpc.rs) | <code>pub fn new(endpoints: Vec&lt;PaxeerEndpoint&gt;) -&gt; Result&lt;Self, RpcFault&gt;</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-paxeer-settings

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-settings</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct PaxeerSettings</code> |

## relayer-paxeer-transaction-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-transaction-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const PAXEER_TRANSACTION_DOMAIN: &amp;[u8]</code> |

## relayer-peer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-peer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct Peer</code> |

## relayer-peer-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-peer-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct PeerConfig</code> |

## relayer-position

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-position</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub struct Position</code> |

## relayer-prefixed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-prefixed</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn prefixed(bytes: &amp;[u8]) -&gt; String</code> |

## relayer-program-log-lines

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-program-log-lines</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub fn program_log_lines&lt;'a&gt;(logs: &amp;'a [String], program: &amp;[u8; 32]) -&gt; Result&lt;Vec&lt;&amp;'a str&gt;, RelayerError&gt;</code> |

## relayer-quantity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-quantity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn quantity(value: u64) -&gt; String</code> |

## relayer-receipt-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-receipt-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const RECEIPT_BYTES: usize</code> |

## relayer-receipt-magic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-receipt-magic</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const RECEIPT_MAGIC: &amp;[u8; 8]</code> |

## relayer-recipient

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-recipient</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub enum Recipient</code> |

## relayer-recipient-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn recipient_address(program_id: &amp;[u8; 32], handle: &amp;[u8; HANDLE_BYTES]) -&gt; Option&lt;[u8; 32]&gt;</code> |

## relayer-recipient-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const RECIPIENT_BYTES: usize</code> |

## relayer-recipient-magic

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-magic</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const RECIPIENT_MAGIC: &amp;[u8; 8]</code> |

## relayer-recipient-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub struct RecipientRecord</code> |

## relayer-recipient-record-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-record-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Option&lt;Self&gt;</code> |

## relayer-recipient-seed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-seed</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const RECIPIENT_SEED: &amp;[u8]</code> |

## relayer-recover-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-recover-signer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn recover_signer(digest: &amp;[u8; 32], signature: &amp;[u8; 65]) -&gt; Result&lt;[u8; 20], SignatureError&gt;</code> |

## relayer-relayer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct Relayer</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-relayer-assembly

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-assembly</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct RelayerAssembly</code> |

## relayer-relayer-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct RelayerConfig</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-relayer-config-build

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-build</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub fn build(&amp;self) -&gt; Result&lt;RelayerSetup, RelayerError&gt;</code> |

## relayer-relayer-config-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-load</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, RelayerError&gt;</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-relayer-config-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), RelayerError&gt;</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-relayer-config-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const RELAYER_CONFIG_VARIABLE: &amp;str</code> |

## relayer-relayer-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub enum RelayerError</code> |

## relayer-relayer-error-is-item-scoped

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-error-is-item-scoped</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub const fn is_item_scoped(&amp;self) -&gt; bool</code> |

## relayer-relayer-failures

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-failures</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub fn failures(&amp;self) -&gt; &amp;[ItemFailure]</code> |

## relayer-relayer-inbound-step

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-inbound-step</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub fn inbound_step(&amp;mut self, index: usize) -&gt; Result&lt;StepReport, RelayerError&gt;</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-relayer-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

## relayer-relayer-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub fn new(setup: impl Into&lt;RelayerSetup&gt;) -&gt; Result&lt;Self, RelayerError&gt;</code> |

## relayer-relayer-outbound-step

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-outbound-step</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub fn outbound_step(&amp;mut self) -&gt; Result&lt;StepReport, RelayerError&gt;</code> |

## relayer-relayer-parts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-parts</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct RelayerParts</code> |

## relayer-relayer-setup

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-setup</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct RelayerSetup</code> |

## relayer-relayer-solana-step

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-solana-step</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub fn solana_step(&amp;mut self) -&gt; Result&lt;StepReport, RelayerError&gt;</code> |

## relayer-relayer-tick

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-tick</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub fn tick(&amp;mut self) -&gt; Vec&lt;(String, Result&lt;StepReport, RelayerError&gt;)&gt;</code> |

## relayer-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-release</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub struct Release</code> |

## relayer-release-accounts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-release-accounts</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub struct ReleaseAccounts</code> |

## relayer-release-data-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-release-data-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const RELEASE_DATA_BYTES: usize</code> |

## relayer-release-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-release-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub enum ReleaseError</code> |

## relayer-release-instruction-data

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-release-instruction-data</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn release_instruction_data(attestation: &amp;OutboundAttestation, recipient: &amp;[u8; 32]) -&gt; Result&lt;Vec&lt;u8&gt;, ReleaseError&gt;</code> |

## relayer-release-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-release-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const RELEASE_SELECTOR: [u8; 4]</code> |

## relayer-release-submission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-release-submission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub struct ReleaseSubmission</code> |

## relayer-reply-already-held

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-reply-already-held</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const REPLY_ALREADY_HELD: u8</code> |

## relayer-reply-conflict

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-reply-conflict</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const REPLY_CONFLICT: u8</code> |

## relayer-reply-stored

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-reply-stored</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const REPLY_STORED: u8</code> |

## relayer-rpc-fault

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-rpc-fault</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/rpc.rs) | <code>pub enum RpcFault</code> |

## relayer-secp256-k1-offsets-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-offsets-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const SECP256K1_OFFSETS_BYTES: usize</code> |

## relayer-secp256-k1-program

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-program</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const SECP256K1_PROGRAM: [u8; 32]</code> |

## relayer-secp256-k1-signature-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-signature-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const SECP256K1_SIGNATURE_BYTES: usize</code> |

## relayer-secp256k1-instruction-data

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-secp256k1-instruction-data</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn secp256k1_instruction_data(signatures: &amp;[([u8; HANDLE_BYTES], [u8; 65])], preimage: &amp;[u8; OUTBOUND_PREIMAGE_BYTES], own: u8) -&gt; Result&lt;Vec&lt;u8&gt;, ReleaseError&gt;</code> |

## relayer-send-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-send-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/rpc.rs) | <code>pub enum SendOutcome</code> |

## relayer-serialize

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-serialize</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/hex.rs](../../../services/interop/crates/layerx-bridge-relayer/src/hex.rs) | <code>pub fn serialize&lt;S: Serializer, const N: usize&gt;(value: &amp;[u8; N], serializer: S) -&gt; Result&lt;S::Ok, S::Error&gt;</code> |

## relayer-share

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-share</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct Share</code> |

## relayer-shortvec

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-shortvec</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn shortvec(out: &amp;mut Vec&lt;u8&gt;, value: usize)</code> |

## relayer-sign

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-sign</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/tx.rs](../../../services/interop/crates/layerx-bridge-relayer/src/tx.rs) | <code>pub fn sign(call: &amp;Eip1559Call, submitter: &amp;Submitter) -&gt; Result&lt;SignedTransaction, KeyError&gt;</code> |

## relayer-signature-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-signature-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub enum SignatureError</code> |

## relayer-signature-info

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-signature-info</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub struct SignatureInfo</code> |

## relayer-signature-page

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-signature-page</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub const SIGNATURE_PAGE: usize</code> |

## relayer-signature-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-signature-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub struct SignatureStatus</code> |

## relayer-signed-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-signed-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/tx.rs](../../../services/interop/crates/layerx-bridge-relayer/src/tx.rs) | <code>pub struct SignedTransaction</code> |

## relayer-signer-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-signer-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct SignerConfig</code> |

## relayer-signer-transport-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-signer-transport-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub enum SignerTransportConfig</code> |

## relayer-solana-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-chain-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs) | <code>pub const SOLANA_CHAIN_ID: u64</code> |

## relayer-solana-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub struct SolanaConfig</code> |

## relayer-solana-config-mints

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-config-mints</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub fn mints(&amp;self) -&gt; Result&lt;Vec&lt;[u8; 32]&gt;, RelayerError&gt;</code> |

## relayer-solana-config-settings

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-config-settings</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>pub fn settings(&amp;self) -&gt; Result&lt;SolanaSettings, RelayerError&gt;</code> |

## relayer-solana-link

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-link</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct SolanaLink</code> |

## relayer-solana-observe-deposit-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-observe-deposit-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub struct DepositRecord</code> |

## relayer-solana-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-release</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct SolanaRelease</code> |

## relayer-solana-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub struct SolanaRpc</code> |

## relayer-solana-rpc-get-account-info

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-account-info</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub fn get_account_info(&amp;self, address: &amp;[u8; 32], commitment: Commitment) -&gt; Result&lt;Option&lt;AccountData&gt;, RpcFault&gt;</code> |

## relayer-solana-rpc-get-latest-blockhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-latest-blockhash</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub fn get_latest_blockhash(&amp;self, commitment: Commitment) -&gt; Result&lt;Blockhash, RpcFault&gt;</code> |

## relayer-solana-rpc-get-signature-statuses

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-signature-statuses</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub fn get_signature_statuses(&amp;self, signatures: &amp;[[u8; 64]]) -&gt; Result&lt;Vec&lt;Option&lt;SignatureStatus&gt;&gt;, RpcFault&gt;</code> |

## relayer-solana-rpc-get-signatures-for-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-signatures-for-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub fn get_signatures_for_address(&amp;self, address: &amp;[u8; 32], before: Option&lt;&amp;[u8; 64]&gt;, limit: usize, commitment: Commitment) -&gt; Result&lt;Vec&lt;SignatureInfo&gt;, RpcFault&gt;</code> |

## relayer-solana-rpc-get-slot

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-slot</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub fn get_slot(&amp;self, commitment: Commitment) -&gt; Result&lt;u64, RpcFault&gt;</code> |

## relayer-solana-rpc-get-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub fn get_transaction(&amp;self, signature: &amp;[u8; 64], commitment: Commitment) -&gt; Result&lt;Option&lt;SolanaTransaction&gt;, RpcFault&gt;</code> |

## relayer-solana-rpc-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub fn new(transport: Box&lt;dyn JsonRpc&gt;) -&gt; Self</code> |

## relayer-solana-rpc-send-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-send-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub fn send_transaction(&amp;self, raw: &amp;[u8], signature: &amp;[u8; 64]) -&gt; Result&lt;SendOutcome, RpcFault&gt;</code> |

## relayer-solana-settings

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-settings</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs) | <code>pub struct SolanaSettings</code> |

## relayer-solana-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs) | <code>pub struct SolanaTransaction</code> |

## relayer-solana-transaction-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-solana-transaction-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const SOLANA_TRANSACTION_DOMAIN: &amp;[u8]</code> |

## relayer-spki-pin

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-spki-pin</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn spki_pin(certificate: &amp;X509) -&gt; Result&lt;[u8; 32], openssl::error::ErrorStack&gt;</code> |

## relayer-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub struct State</code> |

## relayer-step-report

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-step-report</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub struct StepReport</code> |

## relayer-submission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-submission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub struct Submission</code> |

## relayer-submission-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-submission-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/journal.rs](../../../services/interop/crates/layerx-bridge-relayer/src/journal.rs) | <code>pub enum SubmissionStatus</code> |

| Field | Rule |
|---|---|
| interface | The bridge relayer reads Paxeer deposits and releases over EVM JSON-RPC and pax_sequencerReceipt and submits co-signed releases. |
| storage | Relayer journal under the configured state directory; each submission records its status. |
| errors | RelayerError variants for transport, chain id mismatch, malformed receipt and unverified receipt. |
| events | Reads bridge precompile deposit and out events by topic. |
| gas | Release transactions carry the configured gas limit; no gas is spent on reads. |
| activation | Starts when the operator config names receipt_keys. |
| invariants | A release is submitted only after its source receipt verifies under a configured key window. |
| pause | none |

## relayer-submitter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-submitter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub struct Submitter</code> |

## relayer-submitter-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub const fn address(&amp;self) -&gt; [u8; 20]</code> |

## relayer-submitter-ethereum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-ethereum</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn ethereum(signer: RemoteChainSigner) -&gt; Result&lt;Self, SignerError&gt;</code> |

## relayer-submitter-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn new(signer: RemoteChainSigner, domain: &amp;'static [u8]) -&gt; Result&lt;Self, SignerError&gt;</code> |

## relayer-submitter-paxeer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-paxeer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn paxeer(signer: RemoteChainSigner) -&gt; Result&lt;Self, SignerError&gt;</code> |

## relayer-submitter-sign-transaction-hash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-sign-transaction-hash</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/signer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/signer.rs) | <code>pub fn sign_transaction_hash(&amp;self, digest: [u8; 32]) -&gt; Result&lt;[u8; 65], KeyError&gt;</code> |

## relayer-system-program

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-system-program</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const SYSTEM_PROGRAM: [u8; 32]</code> |

## relayer-threshold-selector

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-threshold-selector</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/abi.rs](../../../services/interop/crates/layerx-bridge-relayer/src/abi.rs) | <code>pub const THRESHOLD_SELECTOR: [u8; 4]</code> |

## relayer-to-attestor-signature

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-to-attestor-signature</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn to_attestor_signature(recoverable: [u8; 65]) -&gt; Result&lt;[u8; 65], SignatureError&gt;</code> |

## relayer-token-account-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-token-account-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const TOKEN_ACCOUNT_BYTES: usize</code> |

## relayer-token-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-token-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn token_amount(amount: &amp;[u8; 32]) -&gt; Option&lt;u64&gt;</code> |

## relayer-token-program

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-token-program</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const TOKEN_PROGRAM: [u8; 32]</code> |

## relayer-transport

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-transport</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct Transport</code> |

## relayer-transport-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-transport-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub struct TransportConfig</code> |

## relayer-transport-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-transport-load</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## relayer-transport-record-stem

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-transport-record-stem</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn record_stem(&amp;self, peer: &amp;Peer, digest: &amp;[u8; 32], signature: &amp;[u8; 65]) -&gt; String</code> |

## relayer-transport-serve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-transport-serve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn serve(self: &amp;std::sync::Arc&lt;Self&gt;) -&gt; Result&lt;(), String&gt;</code> |

## relayer-transport-sweep

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-transport-sweep</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn sweep(&amp;self)</code> |

## relayer-transport-validate-operator

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-transport-validate-operator</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub fn validate_operator(&amp;self, inventory: &amp;OperatorInventory, relayer: &amp;crate::config::RelayerConfig) -&gt; Result&lt;(), ConfigError&gt;</code> |

## relayer-uint256-from-u64

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-uint256-from-u64</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation.rs) | <code>pub fn uint256_from_u64(value: u64) -&gt; [u8; 32]</code> |

## relayer-vault-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-vault-authority</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn vault_authority(program_id: &amp;[u8; 32], bump: u8) -&gt; Option&lt;[u8; 32]&gt;</code> |

## relayer-vault-seed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-vault-seed</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub const VAULT_SEED: &amp;[u8]</code> |

## relayer-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/cosign.rs](../../../services/interop/crates/layerx-bridge-relayer/src/cosign.rs) | <code>pub const VERSION: u16</code> |

## relayer-wire-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-relayer-wire-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/solana/release.rs](../../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs) | <code>pub fn wire_transaction(signature: &amp;[u8; 64], message: &amp;[u8]) -&gt; Vec&lt;u8&gt;</code> |
