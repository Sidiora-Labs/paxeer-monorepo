# Interop bridge relayer contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cfg-relayer-attestor-set-config-members

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-attestor-set-config-members</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:234</code> | <code>relayer attestor set members: Vec&lt;String&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-attestor-set-config-members</code> | notes | field AttestorSetConfig.members; struct denies unknown fields; 166 reference lines in 37 files; callers lists the first reference in each file |

## cfg-relayer-attestor-set-config-threshold

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-attestor-set-config-threshold</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:235</code> | <code>relayer attestor set threshold: u8 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-attestor-set-config-threshold</code> | notes | field AttestorSetConfig.threshold; struct denies unknown fields; 214 reference lines in 62 files; callers lists the first reference in each file |

## cfg-relayer-chain-config-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:115</code> | <code>relayer chain chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-chain-id</code> | notes | field ChainConfig.chain_id; struct denies unknown fields; 354 reference lines in 107 files; callers lists the first reference in each file |

## cfg-relayer-chain-config-finality-depth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-finality-depth</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:118</code> | <code>relayer chain finality_depth: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-finality-depth</code> | notes | field ChainConfig.finality_depth; struct denies unknown fields |

## cfg-relayer-chain-config-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-gas</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:123</code> | <code>relayer chain gas: GasConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-gas</code> | notes | field ChainConfig.gas; struct denies unknown fields; 663 reference lines in 110 files; callers lists the first reference in each file; callers truncated by 13 entries for line length |

## cfg-relayer-chain-config-max-block-range

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-max-block-range</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:120</code> | <code>relayer chain max_block_range: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-max-block-range</code> | notes | field ChainConfig.max_block_range; struct denies unknown fields |

## cfg-relayer-chain-config-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:121</code> | <code>relayer chain rpc: RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-rpc</code> | notes | field ChainConfig.rpc; struct denies unknown fields; 1281 reference lines in 218 files; callers lists the first reference in each file |

## cfg-relayer-chain-config-start-block

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-start-block</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:119</code> | <code>relayer chain start_block: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-start-block</code> | notes | field ChainConfig.start_block; struct denies unknown fields |

## cfg-relayer-chain-config-submitter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-submitter</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:122</code> | <code>relayer chain submitter: KeyHandleConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-submitter</code> | notes | field ChainConfig.submitter; struct denies unknown fields; 178 reference lines in 31 files; callers lists the first reference in each file |

## cfg-relayer-chain-config-vault

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-vault</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:117</code> | <code>relayer chain vault: [u8; 20] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-chain-config-vault</code> | notes | field ChainConfig.vault; struct denies unknown fields; 109 reference lines in 21 files; callers lists the first reference in each file |

## cfg-relayer-destination-policy-attestors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-attestors</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:410</code> | <code>bridge operator destination policy attestors: Vec&lt;String&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-attestors</code> | notes | field DestinationPolicy.attestors; struct denies unknown fields; 138 reference lines in 27 files; callers lists the first reference in each file |

## cfg-relayer-destination-policy-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:408</code> | <code>bridge operator destination policy chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-chain-id</code> | notes | field DestinationPolicy.chain_id; struct denies unknown fields; 354 reference lines in 107 files; callers lists the first reference in each file |

## cfg-relayer-destination-policy-observed-block-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-observed-block-hash</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:412</code> | <code>bridge operator destination policy observed_block_hash: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-observed-block-hash</code> | notes | field DestinationPolicy.observed_block_hash; struct denies unknown fields |

## cfg-relayer-destination-policy-threshold

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-threshold</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:411</code> | <code>bridge operator destination policy threshold: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-threshold</code> | notes | field DestinationPolicy.threshold; struct denies unknown fields; 214 reference lines in 62 files; callers lists the first reference in each file |

## cfg-relayer-destination-policy-vault

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-vault</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:409</code> | <code>bridge operator destination policy vault: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-destination-policy-vault</code> | notes | field DestinationPolicy.vault; struct denies unknown fields; 109 reference lines in 21 files; callers lists the first reference in each file |

## cfg-relayer-gas-config-gas-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-gas-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:73</code> | <code>relayer gas gas_limit: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-gas-limit</code> | notes | field GasConfig.gas_limit; struct denies unknown fields; 39 reference lines in 14 files; callers lists the first reference in each file |

## cfg-relayer-gas-config-max-fee-per-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-max-fee-per-gas</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:74</code> | <code>relayer gas max_fee_per_gas: u128 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-max-fee-per-gas</code> | notes | field GasConfig.max_fee_per_gas; struct denies unknown fields |

## cfg-relayer-gas-config-max-priority-fee-per-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-max-priority-fee-per-gas</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:75</code> | <code>relayer gas max_priority_fee_per_gas: u128 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-gas-config-max-priority-fee-per-gas</code> | notes | field GasConfig.max_priority_fee_per_gas; struct denies unknown fields |

## cfg-relayer-key-handle-config-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-key-handle-config-handle</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:64</code> | <code>relayer key handle handle: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-key-handle-config-handle</code> | notes | field KeyHandleConfig.handle; struct denies unknown fields; 658 reference lines in 172 files; callers lists the first reference in each file |

## cfg-relayer-key-handle-config-public-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-key-handle-config-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:67</code> | <code>relayer key handle public_key: Vec&lt;u8&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-key-handle-config-public-key</code> | notes | field KeyHandleConfig.public_key; struct denies unknown fields; 275 reference lines in 72 files; callers lists the first reference in each file |

## cfg-relayer-membership-attestors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-membership-attestors</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:418</code> | <code>bridge operator membership attestors: Vec&lt;String&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-membership-attestors</code> | notes | field Membership.attestors; struct denies unknown fields; 138 reference lines in 27 files; callers lists the first reference in each file |

## cfg-relayer-membership-threshold

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-membership-threshold</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:419</code> | <code>bridge operator membership threshold: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-membership-threshold</code> | notes | field Membership.threshold; struct denies unknown fields; 214 reference lines in 62 files; callers lists the first reference in each file |

## cfg-relayer-operator-fee-payer-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:400</code> | <code>bridge operator fee payer chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-chain-id</code> | notes | field OperatorFeePayer.chain_id; struct denies unknown fields; 354 reference lines in 107 files; callers lists the first reference in each file |

## cfg-relayer-operator-fee-payer-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-handle</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:401</code> | <code>bridge operator fee payer handle: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-handle</code> | notes | field OperatorFeePayer.handle; struct denies unknown fields; 658 reference lines in 172 files; callers lists the first reference in each file |

## cfg-relayer-operator-fee-payer-public-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:402</code> | <code>bridge operator fee payer public_key: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-fee-payer-public-key</code> | notes | field OperatorFeePayer.public_key; struct denies unknown fields; 275 reference lines in 72 files; callers lists the first reference in each file |

## cfg-relayer-operator-inventory-approval

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-approval</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:376</code> | <code>bridge operator inventory approval: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-approval</code> | notes | field OperatorInventory.approval; struct denies unknown fields; 191 reference lines in 53 files; callers lists the first reference in each file |

## cfg-relayer-operator-inventory-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-authority</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:375</code> | <code>bridge operator inventory authority: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-authority</code> | notes | field OperatorInventory.authority; struct denies unknown fields; 1564 reference lines in 231 files; callers lists the first reference in each file; callers truncated by 1 entries for line length |

## cfg-relayer-operator-inventory-destinations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-destinations</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:379</code> | <code>bridge operator inventory destinations: Vec&lt;DestinationPolicy&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-destinations</code> | notes | field OperatorInventory.destinations; struct denies unknown fields; 60 reference lines in 17 files; callers lists the first reference in each file |

## cfg-relayer-operator-inventory-membership

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-membership</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:377</code> | <code>bridge operator inventory membership: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-membership</code> | notes | field OperatorInventory.membership; struct denies unknown fields; 44 reference lines in 22 files; callers lists the first reference in each file |

## cfg-relayer-operator-inventory-operators

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-operators</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:378</code> | <code>bridge operator inventory operators: Vec&lt;OperatorRecord&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-operators</code> | notes | field OperatorInventory.operators; struct denies unknown fields; 360 reference lines in 57 files; callers lists the first reference in each file |

## cfg-relayer-operator-inventory-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:374</code> | <code>bridge operator inventory version: u16 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-inventory-version</code> | notes | field OperatorInventory.version; struct denies unknown fields; 4872 reference lines in 550 files; callers lists the first reference in each file |

## cfg-relayer-operator-record-attestor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-attestor</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:387</code> | <code>bridge operator record attestor: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-attestor</code> | notes | field OperatorRecord.attestor; struct denies unknown fields; 477 reference lines in 72 files; callers lists the first reference in each file |

## cfg-relayer-operator-record-cosign-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-cosign-directory</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:391</code> | <code>bridge operator record cosign_directory: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-cosign-directory</code> | notes | field OperatorRecord.cosign_directory; struct denies unknown fields |

## cfg-relayer-operator-record-delivery-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-delivery-directory</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:392</code> | <code>bridge operator record delivery_directory: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-delivery-directory</code> | notes | field OperatorRecord.delivery_directory; struct denies unknown fields |

## cfg-relayer-operator-record-fee-payers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-fee-payers</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:394</code> | <code>bridge operator record fee_payers: Vec&lt;OperatorFeePayer&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-fee-payers</code> | notes | field OperatorRecord.fee_payers; struct denies unknown fields |

## cfg-relayer-operator-record-instance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-instance</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:385</code> | <code>bridge operator record instance: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-instance</code> | notes | field OperatorRecord.instance; struct denies unknown fields; 4572 reference lines in 669 files; callers lists the first reference in each file; callers truncated by 8 entries for line length |

## cfg-relayer-operator-record-journal-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-journal-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:390</code> | <code>bridge operator record journal_path: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-journal-path</code> | notes | field OperatorRecord.journal_path; struct denies unknown fields |

## cfg-relayer-operator-record-signer-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-signer-handle</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:388</code> | <code>bridge operator record signer_handle: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-signer-handle</code> | notes | field OperatorRecord.signer_handle; struct denies unknown fields |

## cfg-relayer-operator-record-signer-public-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-signer-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:389</code> | <code>bridge operator record signer_public_key: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-signer-public-key</code> | notes | field OperatorRecord.signer_public_key; struct denies unknown fields; 52 reference lines in 13 files; callers lists the first reference in each file |

## cfg-relayer-operator-record-spki-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-spki-sha256</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:393</code> | <code>bridge operator record spki_sha256: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-spki-sha256</code> | notes | field OperatorRecord.spki_sha256; struct denies unknown fields |

## cfg-relayer-operator-record-storage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-storage</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:386</code> | <code>bridge operator record storage: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-operator-record-storage</code> | notes | field OperatorRecord.storage; struct denies unknown fields; 959 reference lines in 212 files; callers lists the first reference in each file |

## cfg-relayer-paxeer-config-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:103</code> | <code>relayer paxeer chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-chain-id</code> | notes | field PaxeerConfig.chain_id; struct denies unknown fields; 354 reference lines in 107 files; callers lists the first reference in each file |

## cfg-relayer-paxeer-config-endpoints

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-endpoints</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:104</code> | <code>relayer paxeer endpoints: Vec&lt;PaxeerEndpointConfig&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-endpoints</code> | notes | field PaxeerConfig.endpoints; struct denies unknown fields; 360 reference lines in 84 files; callers lists the first reference in each file |

## cfg-relayer-paxeer-config-finality-depth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-finality-depth</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:105</code> | <code>relayer paxeer finality_depth: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-finality-depth</code> | notes | field PaxeerConfig.finality_depth; struct denies unknown fields |

## cfg-relayer-paxeer-config-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-gas</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:109</code> | <code>relayer paxeer gas: GasConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-gas</code> | notes | field PaxeerConfig.gas; struct denies unknown fields; 663 reference lines in 110 files; callers lists the first reference in each file; callers truncated by 13 entries for line length |

## cfg-relayer-paxeer-config-max-block-range

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-max-block-range</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:107</code> | <code>relayer paxeer max_block_range: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-max-block-range</code> | notes | field PaxeerConfig.max_block_range; struct denies unknown fields |

## cfg-relayer-paxeer-config-start-block

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-start-block</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:106</code> | <code>relayer paxeer start_block: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-start-block</code> | notes | field PaxeerConfig.start_block; struct denies unknown fields |

## cfg-relayer-paxeer-config-submitter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-submitter</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:108</code> | <code>relayer paxeer submitter: KeyHandleConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-config-submitter</code> | notes | field PaxeerConfig.submitter; struct denies unknown fields; 178 reference lines in 31 files; callers lists the first reference in each file |

## cfg-relayer-paxeer-endpoint-config-local-emulator

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-local-emulator</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:96</code> | <code>relayer paxeer endpoint local_emulator: bool (optional)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-local-emulator</code> | notes | field PaxeerEndpointConfig.local_emulator; struct denies unknown fields |

## cfg-relayer-paxeer-endpoint-config-request-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-request-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:97</code> | <code>relayer paxeer endpoint request_timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-request-timeout-ms</code> | notes | field PaxeerEndpointConfig.request_timeout_ms; struct denies unknown fields |

## cfg-relayer-paxeer-endpoint-config-trust-anchor-der

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-trust-anchor-der</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:94</code> | <code>relayer paxeer endpoint trust_anchor_der: Option&lt;PathBuf&gt; (optional)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-trust-anchor-der</code> | notes | field PaxeerEndpointConfig.trust_anchor_der; struct denies unknown fields |

## cfg-relayer-paxeer-endpoint-config-url

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-url</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:91</code> | <code>relayer paxeer endpoint url: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-paxeer-endpoint-config-url</code> | notes | field PaxeerEndpointConfig.url; struct denies unknown fields; 1062 reference lines in 167 files; callers lists the first reference in each file |

## cfg-relayer-peer-config-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-address</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:300</code> | <code>cosign transport peer address: SocketAddr (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-address</code> | notes | field PeerConfig.address; struct denies unknown fields; 5720 reference lines in 532 files; callers lists the first reference in each file |

## cfg-relayer-peer-config-attestor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-attestor</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:299</code> | <code>cosign transport peer attestor: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-attestor</code> | notes | field PeerConfig.attestor; struct denies unknown fields; 477 reference lines in 72 files; callers lists the first reference in each file |

## cfg-relayer-peer-config-server-name

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-server-name</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:301</code> | <code>cosign transport peer server_name: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-server-name</code> | notes | field PeerConfig.server_name; struct denies unknown fields; 32 reference lines in 16 files; callers lists the first reference in each file |

## cfg-relayer-peer-config-spki-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-spki-sha256</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:302</code> | <code>cosign transport peer spki_sha256: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-peer-config-spki-sha256</code> | notes | field PeerConfig.spki_sha256; struct denies unknown fields |

## cfg-relayer-receipt-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-receipt-keys</code> | <code>config_key</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs</code> (planned) | <code>relayer config receipt_keys: [{key_id, public_key, first_height, last_height}] (public keys only)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-receipt-keys</code> | notes | stale task 7.5.16 do_7: confirm_by_receipt verifies canonical receipt bytes against the key valid at the height |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-attestor</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:298</code> | <code>relayer config attestor: KeyHandleConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-attestor</code> | notes | field RelayerConfig.attestor; struct denies unknown fields; 477 reference lines in 72 files; callers lists the first reference in each file |

## cfg-relayer-relayer-config-attestor-set

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-attestor-set</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:307</code> | <code>relayer config attestor_set: Option&lt;AttestorSetConfig&gt; (optional)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-attestor-set</code> | notes | field RelayerConfig.attestor_set; struct denies unknown fields |

## cfg-relayer-relayer-config-chains

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-chains</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:300</code> | <code>relayer config chains: Vec&lt;ChainConfig&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-chains</code> | notes | field RelayerConfig.chains; struct denies unknown fields; 98 reference lines in 40 files; callers lists the first reference in each file |

## cfg-relayer-relayer-config-cosign-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-cosign-directory</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:294</code> | <code>relayer config cosign_directory: Option&lt;PathBuf&gt; (optional)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-cosign-directory</code> | notes | field RelayerConfig.cosign_directory; struct denies unknown fields |

## cfg-relayer-relayer-config-journal-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-journal-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:290</code> | <code>relayer config journal_path: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-journal-path</code> | notes | field RelayerConfig.journal_path; struct denies unknown fields |

## cfg-relayer-relayer-config-max-submissions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-max-submissions</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:296</code> | <code>relayer config max_submissions: u32 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-max-submissions</code> | notes | field RelayerConfig.max_submissions; struct denies unknown fields |

## cfg-relayer-relayer-config-paxeer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-paxeer</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:299</code> | <code>relayer config paxeer: PaxeerConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-paxeer</code> | notes | field RelayerConfig.paxeer; struct denies unknown fields; 1822 reference lines in 440 files; callers lists the first reference in each file |

## cfg-relayer-relayer-config-poll-interval-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-poll-interval-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:295</code> | <code>relayer config poll_interval_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-poll-interval-ms</code> | notes | field RelayerConfig.poll_interval_ms; struct denies unknown fields |

## cfg-relayer-relayer-config-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-signer</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:297</code> | <code>relayer config signer: SignerConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-signer</code> | notes | field RelayerConfig.signer; struct denies unknown fields; 726 reference lines in 107 files; callers lists the first reference in each file |

## cfg-relayer-relayer-config-solana

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-solana</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:304</code> | <code>relayer config solana: Option&lt;SolanaConfig&gt; (optional)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-relayer-config-solana</code> | notes | field RelayerConfig.solana; struct denies unknown fields; 291 reference lines in 35 files; callers lists the first reference in each file |

## cfg-relayer-signer-config-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-signer-config-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:56</code> | <code>relayer signer endpoint: SignerTransportConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-signer-config-endpoint</code> | notes | field SignerConfig.endpoint; struct denies unknown fields; 1036 reference lines in 182 files; callers lists the first reference in each file |

## cfg-relayer-signer-config-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-signer-config-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:57</code> | <code>relayer signer timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-signer-config-timeout-ms</code> | notes | field SignerConfig.timeout_ms; struct denies unknown fields |

## cfg-relayer-signer-transport-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-signer-transport-config</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:40</code> | <code>relayer signer endpoint: one of "uds" &#124; "mutual_tls" (tag transport)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-signer-transport-config</code> | notes | serde enum SignerTransportConfig, deny_unknown_fields |

## cfg-relayer-solana-config-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:132</code> | <code>relayer solana chain_id: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-chain-id</code> | notes | field SolanaConfig.chain_id; struct denies unknown fields; 354 reference lines in 107 files; callers lists the first reference in each file |

## cfg-relayer-solana-config-commitment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-commitment</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:146</code> | <code>relayer solana commitment: Commitment (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-commitment</code> | notes | field SolanaConfig.commitment; struct denies unknown fields; 100 reference lines in 21 files; callers lists the first reference in each file |

## cfg-relayer-solana-config-fee-payer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-fee-payer</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:142</code> | <code>relayer solana fee_payer: KeyHandleConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-fee-payer</code> | notes | field SolanaConfig.fee_payer; struct denies unknown fields; 36 reference lines in 4 files; callers lists the first reference in each file |

## cfg-relayer-solana-config-finality-depth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-finality-depth</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:136</code> | <code>relayer solana finality_depth: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-finality-depth</code> | notes | field SolanaConfig.finality_depth; struct denies unknown fields |

## cfg-relayer-solana-config-max-slot-range

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-max-slot-range</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:138</code> | <code>relayer solana max_slot_range: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-max-slot-range</code> | notes | field SolanaConfig.max_slot_range; struct denies unknown fields |

## cfg-relayer-solana-config-program-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-program-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:144</code> | <code>relayer solana program_id: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-program-id</code> | notes | field SolanaConfig.program_id; struct denies unknown fields; 267 reference lines in 29 files; callers lists the first reference in each file |

## cfg-relayer-solana-config-release-mints

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-release-mints</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:151</code> | <code>relayer solana release_mints: Vec&lt;String&gt; (optional)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-release-mints</code> | notes | field SolanaConfig.release_mints; struct denies unknown fields |

## cfg-relayer-solana-config-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:139</code> | <code>relayer solana rpc: RpcQuorumConfig (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-rpc</code> | notes | field SolanaConfig.rpc; struct denies unknown fields; 1281 reference lines in 218 files; callers lists the first reference in each file |

## cfg-relayer-solana-config-start-slot

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-start-slot</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:137</code> | <code>relayer solana start_slot: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-start-slot</code> | notes | field SolanaConfig.start_slot; struct denies unknown fields |

## cfg-relayer-solana-config-vault

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-vault</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:135</code> | <code>relayer solana vault: [u8; 20] (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-solana-config-vault</code> | notes | field SolanaConfig.vault; struct denies unknown fields; 109 reference lines in 21 files; callers lists the first reference in each file |

## cfg-relayer-transport-config-attestor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-attestor</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:308</code> | <code>cosign transport config attestor: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-attestor</code> | notes | field TransportConfig.attestor; struct denies unknown fields; 477 reference lines in 72 files; callers lists the first reference in each file |

## cfg-relayer-transport-config-certificate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-certificate</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:312</code> | <code>cosign transport config certificate: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-certificate</code> | notes | field TransportConfig.certificate; struct denies unknown fields; 235 reference lines in 55 files; callers lists the first reference in each file |

## cfg-relayer-transport-config-cosign-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-cosign-directory</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:314</code> | <code>cosign transport config cosign_directory: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-cosign-directory</code> | notes | field TransportConfig.cosign_directory; struct denies unknown fields |

## cfg-relayer-transport-config-delivery-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-delivery-directory</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:315</code> | <code>cosign transport config delivery_directory: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-delivery-directory</code> | notes | field TransportConfig.delivery_directory; struct denies unknown fields |

## cfg-relayer-transport-config-listen

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-listen</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:309</code> | <code>cosign transport config listen: SocketAddr (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-listen</code> | notes | field TransportConfig.listen; struct denies unknown fields; 257 reference lines in 64 files; callers lists the first reference in each file |

## cfg-relayer-transport-config-max-connections

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-max-connections</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:319</code> | <code>cosign transport config max_connections: usize (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-max-connections</code> | notes | field TransportConfig.max_connections; struct denies unknown fields |

## cfg-relayer-transport-config-peers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-peers</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:316</code> | <code>cosign transport config peers: Vec&lt;PeerConfig&gt; (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-peers</code> | notes | field TransportConfig.peers; struct denies unknown fields; 322 reference lines in 65 files; callers lists the first reference in each file |

## cfg-relayer-transport-config-private-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-private-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:313</code> | <code>cosign transport config private_key: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-private-key</code> | notes | field TransportConfig.private_key; struct denies unknown fields |

## cfg-relayer-transport-config-server-name

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-server-name</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:310</code> | <code>cosign transport config server_name: String (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-server-name</code> | notes | field TransportConfig.server_name; struct denies unknown fields; 32 reference lines in 16 files; callers lists the first reference in each file |

## cfg-relayer-transport-config-sweep-interval-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-sweep-interval-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:318</code> | <code>cosign transport config sweep_interval_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-sweep-interval-ms</code> | notes | field TransportConfig.sweep_interval_ms; struct denies unknown fields |

## cfg-relayer-transport-config-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:317</code> | <code>cosign transport config timeout_ms: u64 (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-timeout-ms</code> | notes | field TransportConfig.timeout_ms; struct denies unknown fields |

## cfg-relayer-transport-config-trust-anchor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-trust-anchor</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:311</code> | <code>cosign transport config trust_anchor: PathBuf (required)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-transport-config-trust-anchor</code> | notes | field TransportConfig.trust_anchor; struct denies unknown fields |

## cli-relayer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-relayer</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/main.rs:1</code> | <code>layerx-bridge-relayer --config PATH</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-relayer</code> | notes | PATH is RelayerConfig JSON |

## cli-relayer-cosign

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-relayer-cosign</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/bin/cosign.rs:9</code> | <code>layerx-bridge-cosign (no arguments; reads LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-relayer-cosign</code> | notes | bin name from services/interop/crates/layerx-bridge-relayer/Cargo.toml:16 |

## env-relayer-layerx-bridge-cosign-transport-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-relayer-layerx-bridge-cosign-transport-config</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:280</code> | <code>LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG: path to the cosign transport config (TransportConfig); its presence enables layerx-bridge-cosign (ENABLE_VARIABLE)</code> |

## env-relayer-layerx-bridge-operator-inventory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-relayer-layerx-bridge-operator-inventory</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:368</code> | <code>LAYERX_BRIDGE_OPERATOR_INVENTORY: path to the bridge operator inventory (OperatorInventory) (INVENTORY_VARIABLE)</code> |

## env-relayer-layerx-bridge-relayer-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-relayer-layerx-bridge-relayer-config</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:369</code> | <code>LAYERX_BRIDGE_RELAYER_CONFIG: path to the relayer config read alongside the inventory (RELAYER_CONFIG_VARIABLE)</code> |

## fmt-relayer-bridge-in-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-bridge-in-domain</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:13</code> | <code>attestation preimage prefix "PAXEERX_BRIDGE_IN_V1" (20 bytes) for inbound bridge attestations</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-bridge-in-domain</code> | notes | signed by each attestor and recovered by recover_signer; shared with the bridge precompile 0x1016 verifier |

## fmt-relayer-bridge-out-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-bridge-out-domain</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:15</code> | <code>attestation preimage prefix "PAXEERX_BRIDGE_OUT_V1" (21 bytes) for outbound bridge releases</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-bridge-out-domain</code> | notes | signed by attestors for releases on the destination chain |

## fmt-relayer-commitment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-commitment</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:23</code> | <code>Commitment = confirmed &#124; finalized</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-commitment</code> | notes | bridge relayer journal JSONL via serde Clone, Copy, Debug, Deserialize, Eq, Ord, PartialEq, PartialOrd, rename_all snake_case |

## fmt-relayer-commitment-labels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-commitment-labels</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:30</code> | <code>Commitment::as_str() -&gt; "confirmed" &#124; "finalized"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-commitment-labels</code> | notes | stable label vocabulary of Commitment |

## fmt-relayer-completion

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-completion</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:217</code> | <code>Completion = included &#124; already_bridged &#124; released (tag outcome)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-completion</code> | notes | bridge relayer journal JSONL via serde Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize, deny_unknown_fields, rename_all snake_case; 32 reference lines in 5 files; callers lists the first reference in each file |

## fmt-relayer-cosign-frame

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-cosign-frame</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:282</code> | <code>cosign share frame: u32be length 143 &#124; "LXSH" &#124; version:u16be=1 &#124; recipient:20 &#124; digest:32 &#124; signer:20 &#124; signature:65; reply byte 1 stored, 2 already held, 3 conflict</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-cosign-frame</code> | notes | peer-to-peer attestor share transport enabled by LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG (services/interop/crates/layerx-bridge-relayer/src/cosign.rs:279); encoder services/interop/crates/layerx-bridge-relayer/src/cosign.rs:1089, decoder services/interop/crates/layerx-bridge-relayer/src/cosign.rs:1105 |

## fmt-relayer-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-entry</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:239</code> | <code>Entry = cursor &#124; observed &#124; signed &#124; submitted &#124; receipted &#124; reverted &#124; dropped &#124; completed &#124; refused &#124; recipient_pending &#124; recipient_resolved &#124; release_submitted &#124; release_expired &#124; release_failed (tag kind)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-entry</code> | notes | bridge relayer journal JSONL via serde Clone, Debug, Deserialize, Eq, PartialEq, Serialize, deny_unknown_fields, rename_all snake_case; 72 reference lines in 3 files; callers lists the first reference in each file |

## fmt-relayer-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-observation</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:82</code> | <code>Observation = inbound &#124; outbound (tag direction)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-observation</code> | notes | bridge relayer journal JSONL via serde Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize, deny_unknown_fields, rename_all snake_case; 36 reference lines in 5 files; callers lists the first reference in each file |

## fmt-relayer-position

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-relayer-position</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:64</code> | <code>Position { block_number: u64, block_hash: [u8; 32] }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-relayer-position</code> | notes | bridge relayer journal JSONL via serde Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize, deny_unknown_fields |

## relayer-abi-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-abi-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:49</code> | <code>pub enum AbiError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-abi-error</code> | notes | enum layerx_bridge_relayer::abi::AbiError |

## relayer-account-data

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-account-data</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:101</code> | <code>pub struct AccountData</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-account-data</code> | notes | struct layerx_bridge_relayer::solana::rpc::AccountData |

## relayer-address-word

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-address-word</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:75</code> | <code>pub fn address_word(address: &amp;[u8; 20]) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-address-word</code> | notes | fn layerx_bridge_relayer::abi::address_word |

## relayer-admission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-admission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:212</code> | <code>pub enum Admission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-admission</code> | notes | enum layerx_bridge_relayer::cosign::Admission; no reference outside the defining file found by name match |

## relayer-algorithm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-algorithm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:212</code> | <code>pub const ALGORITHM: SigningAlgorithm</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-algorithm</code> | notes | const layerx_bridge_relayer::signer::ALGORITHM |

## relayer-assemble

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-assemble</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/tx.rs:59</code> | <code>pub fn assemble(call: &amp;Eip1559Call, signature: &amp;[u8; 65]) -&gt; SignedTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-assemble</code> | notes | fn layerx_bridge_relayer::tx::assemble |

## relayer-assemble-signatures

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-assemble-signatures</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:223</code> | <code>pub fn assemble_signatures&lt;I&gt;(digest: &amp;[u8; 32], candidates: I, attestors: &amp;[[u8; 20]], threshold: usize) -&gt; Result&lt;Vec&lt;[u8; 65]&gt;, SignatureError&gt; where I: IntoIterator&lt;Item = [u8; 65]&gt;,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-assemble-signatures</code> | notes | fn layerx_bridge_relayer::attestation::assemble_signatures |

## relayer-asset-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-asset-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:163</code> | <code>pub fn asset_address(program_id: &amp;[u8; 32], mint: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-asset-address</code> | notes | fn layerx_bridge_relayer::solana::release::asset_address |

## relayer-asset-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-asset-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:39</code> | <code>pub const ASSET_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-asset-bytes</code> | notes | const layerx_bridge_relayer::solana::observe::ASSET_BYTES; no reference outside the defining file found by name match |

## relayer-asset-magic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-asset-magic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:38</code> | <code>pub const ASSET_MAGIC: &amp;[u8; 8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-asset-magic</code> | notes | const layerx_bridge_relayer::solana::observe::ASSET_MAGIC; no reference outside the defining file found by name match |

## relayer-asset-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-asset-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:91</code> | <code>pub struct AssetRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-asset-record</code> | notes | struct layerx_bridge_relayer::solana::observe::AssetRecord |

## relayer-asset-record-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-asset-record-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:151</code> | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Option&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-asset-record-decode</code> | notes | method layerx_bridge_relayer::solana::observe::AssetRecord::decode |

## relayer-asset-seed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-asset-seed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:40</code> | <code>pub const ASSET_SEED: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-asset-seed</code> | notes | const layerx_bridge_relayer::solana::release::ASSET_SEED; no reference outside the defining file found by name match |

## relayer-associated-token-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-associated-token-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:144</code> | <code>pub fn associated_token_address(wallet: &amp;[u8; 32], mint: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-associated-token-address</code> | notes | fn layerx_bridge_relayer::solana::release::associated_token_address |

## relayer-associated-token-program

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-associated-token-program</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:73</code> | <code>pub const ASSOCIATED_TOKEN_PROGRAM: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-associated-token-program</code> | notes | const layerx_bridge_relayer::solana::release::ASSOCIATED_TOKEN_PROGRAM; no reference outside the defining file found by name match |

## relayer-attest-inbound-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attest-inbound-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:15</code> | <code>pub const ATTEST_INBOUND_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attest-inbound-domain</code> | notes | const layerx_bridge_relayer::signer::ATTEST_INBOUND_DOMAIN |

## relayer-attest-outbound-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attest-outbound-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:17</code> | <code>pub const ATTEST_OUTBOUND_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attest-outbound-domain</code> | notes | const layerx_bridge_relayer::signer::ATTEST_OUTBOUND_DOMAIN |

## relayer-attestor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:71</code> | <code>pub struct Attestor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor</code> | notes | struct layerx_bridge_relayer::signer::Attestor |

## relayer-attestor-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:86</code> | <code>pub const fn address(&amp;self) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-address</code> | notes | method layerx_bridge_relayer::signer::Attestor::address; no reference outside the defining file found by name match |

## relayer-attestor-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:80</code> | <code>pub fn new(signer: RemoteChainSigner) -&gt; Result&lt;Self, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-new</code> | notes | method layerx_bridge_relayer::signer::Attestor::new |

## relayer-attestor-set

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-set</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:248</code> | <code>pub struct AttestorSet</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-set</code> | notes | struct layerx_bridge_relayer::abi::AttestorSet; no reference outside the defining file found by name match |

## relayer-attestor-set-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:232</code> | <code>pub struct AttestorSetConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-config</code> | notes | struct layerx_bridge_relayer::config::AttestorSetConfig; no reference outside the defining file found by name match |

## relayer-attestor-set-config-addresses

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-config-addresses</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:244</code> | <code>pub fn addresses(&amp;self) -&gt; Result&lt;Vec&lt;[u8; 20]&gt;, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-config-addresses</code> | notes | method layerx_bridge_relayer::config::AttestorSetConfig::addresses; no reference outside the defining file found by name match |

## relayer-attestor-set-size

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-size</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:34</code> | <code>pub const ATTESTOR_SET_SIZE: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-set-size</code> | notes | const layerx_bridge_relayer::config::ATTESTOR_SET_SIZE; no reference outside the defining file found by name match |

## relayer-attestor-sign-inbound

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-sign-inbound</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:95</code> | <code>pub fn sign_inbound(&amp;self, attestation: &amp;InboundAttestation) -&gt; Result&lt;[u8; 65], KeyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-sign-inbound</code> | notes | method layerx_bridge_relayer::signer::Attestor::sign_inbound |

## relayer-attestor-sign-outbound

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-sign-outbound</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:105</code> | <code>pub fn sign_outbound(&amp;self, attestation: &amp;OutboundAttestation) -&gt; Result&lt;[u8; 65], KeyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-sign-outbound</code> | notes | method layerx_bridge_relayer::signer::Attestor::sign_outbound |

## relayer-attestor-threshold

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestor-threshold</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:36</code> | <code>pub const ATTESTOR_THRESHOLD: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestor-threshold</code> | notes | const layerx_bridge_relayer::config::ATTESTOR_THRESHOLD; no reference outside the defining file found by name match |

## relayer-attestors-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-attestors-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:42</code> | <code>pub const ATTESTORS_SELECTOR: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-attestors-selector</code> | notes | const layerx_bridge_relayer::abi::ATTESTORS_SELECTOR |

## relayer-base58-data

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-base58-data</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:120</code> | <code>pub fn base58_data(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, CodecError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-base58-data</code> | notes | fn layerx_bridge_relayer::solana::base58_data |

## relayer-base58-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-base58-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:107</code> | <code>pub fn base58_decode(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, CodecError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-base58-decode</code> | notes | fn layerx_bridge_relayer::solana::base58_decode; no reference outside the defining file found by name match |

## relayer-base58-encode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-base58-encode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:79</code> | <code>pub fn base58_encode(bytes: &amp;[u8]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-base58-encode</code> | notes | fn layerx_bridge_relayer::solana::base58_encode |

## relayer-base58-fixed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-base58-fixed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:155</code> | <code>pub fn base58_fixed&lt;const N: usize&gt;(text: &amp;str) -&gt; Result&lt;[u8; N], CodecError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-base58-fixed</code> | notes | fn layerx_bridge_relayer::solana::base58_fixed |

## relayer-base64-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-base64-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:185</code> | <code>pub fn base64_decode(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, CodecError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-base64-decode</code> | notes | fn layerx_bridge_relayer::solana::base64_decode |

## relayer-base64-encode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-base64-encode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:161</code> | <code>pub fn base64_encode(bytes: &amp;[u8]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-base64-encode</code> | notes | fn layerx_bridge_relayer::solana::base64_encode |

## relayer-blockhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-blockhash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:111</code> | <code>pub struct Blockhash</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-blockhash</code> | notes | struct layerx_bridge_relayer::solana::rpc::Blockhash; no reference outside the defining file found by name match |

## relayer-bridge-deposit-topic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-bridge-deposit-topic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:19</code> | <code>pub const BRIDGE_DEPOSIT_TOPIC: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-bridge-deposit-topic</code> | notes | const layerx_bridge_relayer::abi::BRIDGE_DEPOSIT_TOPIC |

## relayer-bridge-in-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-bridge-in-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:30</code> | <code>pub const BRIDGE_IN_SELECTOR: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-bridge-in-selector</code> | notes | const layerx_bridge_relayer::abi::BRIDGE_IN_SELECTOR; no reference outside the defining file found by name match |

## relayer-bridge-out-topic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-bridge-out-topic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:24</code> | <code>pub const BRIDGE_OUT_TOPIC: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-bridge-out-topic</code> | notes | const layerx_bridge_relayer::abi::BRIDGE_OUT_TOPIC |

## relayer-build-release-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-build-release-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:499</code> | <code>pub fn build_release_transaction(release: &amp;Release, recent_blockhash: &amp;[u8; 32]) -&gt; Result&lt;Vec&lt;u8&gt;, ReleaseError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-build-release-transaction</code> | notes | fn layerx_bridge_relayer::solana::release::build_release_transaction |

## relayer-burn-log

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-burn-log</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:397</code> | <code>pub struct BurnLog</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-burn-log</code> | notes | struct layerx_bridge_relayer::abi::BurnLog; no reference outside the defining file found by name match |

## relayer-burn-log-attestation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-burn-log-attestation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:408</code> | <code>pub const fn attestation(&amp;self, vault: [u8; 20]) -&gt; OutboundAttestation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-burn-log-attestation</code> | notes | method layerx_bridge_relayer::abi::BurnLog::attestation; no reference outside the defining file found by name match |

## relayer-chain-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-chain-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:114</code> | <code>pub struct ChainConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-chain-config</code> | notes | struct layerx_bridge_relayer::config::ChainConfig; no reference outside the defining file found by name match |

## relayer-chain-link

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-chain-link</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:218</code> | <code>pub struct ChainLink</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-chain-link</code> | notes | struct layerx_bridge_relayer::relayer::ChainLink |

## relayer-chain-registration

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-chain-registration</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:269</code> | <code>pub struct ChainRegistration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-chain-registration</code> | notes | struct layerx_bridge_relayer::abi::ChainRegistration; no reference outside the defining file found by name match |

## relayer-chain-settings

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-chain-settings</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:198</code> | <code>pub struct ChainSettings</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-chain-settings</code> | notes | struct layerx_bridge_relayer::relayer::ChainSettings |

## relayer-codec-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-codec-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:51</code> | <code>pub struct CodecError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-codec-error</code> | notes | struct layerx_bridge_relayer::solana::CodecError; no reference outside the defining file found by name match |

## relayer-commitment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-commitment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:23</code> | <code>pub enum Commitment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-commitment</code> | notes | enum layerx_bridge_relayer::solana::rpc::Commitment |

## relayer-commitment-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-commitment-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:30</code> | <code>pub const fn as_str(self) -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-commitment-as-str</code> | notes | method layerx_bridge_relayer::solana::rpc::Commitment::as_str; no reference outside the defining file found by name match |

## relayer-completion

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-completion</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:217</code> | <code>pub enum Completion</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-completion</code> | notes | enum layerx_bridge_relayer::journal::Completion |

## relayer-config-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-config-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:169</code> | <code>pub fn config_address(program_id: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-config-address</code> | notes | fn layerx_bridge_relayer::solana::release::config_address |

## relayer-config-attestors-offset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-config-attestors-offset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:49</code> | <code>pub const CONFIG_ATTESTORS_OFFSET: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-config-attestors-offset</code> | notes | const layerx_bridge_relayer::solana::release::CONFIG_ATTESTORS_OFFSET; no reference outside the defining file found by name match |

## relayer-config-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-config-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:50</code> | <code>pub const CONFIG_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-config-bytes</code> | notes | const layerx_bridge_relayer::solana::release::CONFIG_BYTES; no reference outside the defining file found by name match |

## relayer-config-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-config-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:346</code> | <code>pub struct ConfigError(pub String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-config-error</code> | notes | struct layerx_bridge_relayer::cosign::transport::ConfigError; no reference outside the defining file found by name match |

## relayer-config-magic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-config-magic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:44</code> | <code>pub const CONFIG_MAGIC: &amp;[u8; 8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-config-magic</code> | notes | const layerx_bridge_relayer::solana::release::CONFIG_MAGIC; no reference outside the defining file found by name match |

## relayer-config-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-config-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:189</code> | <code>pub struct ConfigRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-config-record</code> | notes | struct layerx_bridge_relayer::solana::release::ConfigRecord |

## relayer-config-record-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-config-record-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:200</code> | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Option&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-config-record-decode</code> | notes | method layerx_bridge_relayer::solana::release::ConfigRecord::decode |

## relayer-config-seed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-config-seed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:38</code> | <code>pub const CONFIG_SEED: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-config-seed</code> | notes | const layerx_bridge_relayer::solana::release::CONFIG_SEED; no reference outside the defining file found by name match |

## relayer-confirmation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-confirmation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:40</code> | <code>pub enum Confirmation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-confirmation</code> | notes | enum layerx_bridge_relayer::solana::rpc::Confirmation |

## relayer-confirmation-reaches

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-confirmation-reaches</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:58</code> | <code>pub const fn reaches(self, commitment: Commitment) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-confirmation-reaches</code> | notes | method layerx_bridge_relayer::solana::rpc::Confirmation::reaches |

## relayer-cosign-directory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:25</code> | <code>pub struct CosignDirectory</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory</code> | notes | struct layerx_bridge_relayer::cosign::CosignDirectory |

## relayer-cosign-directory-admit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-admit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:59</code> | <code>pub fn admit(&amp;self, digest: &amp;[u8; 32], signer: &amp;[u8; 20], signature: &amp;[u8; 65]) -&gt; Result&lt;Admission, JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-admit</code> | notes | method layerx_bridge_relayer::cosign::CosignDirectory::admit |

## relayer-cosign-directory-collect

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-collect</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:177</code> | <code>pub fn collect(&amp;self, digest: &amp;[u8; 32]) -&gt; Vec&lt;[u8; 65]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-collect</code> | notes | method layerx_bridge_relayer::cosign::CosignDirectory::collect; 53 reference lines in 5 files; callers lists the first reference in each file |

## relayer-cosign-directory-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:31</code> | <code>pub const fn new(root: PathBuf) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-new</code> | notes | method layerx_bridge_relayer::cosign::CosignDirectory::new |

## relayer-cosign-directory-own-shares

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-own-shares</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:142</code> | <code>pub fn own_shares(&amp;self, signer: &amp;[u8; 20]) -&gt; Vec&lt;([u8; 32], [u8; 65])&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-own-shares</code> | notes | method layerx_bridge_relayer::cosign::CosignDirectory::own_shares; no reference outside the defining file found by name match |

## relayer-cosign-directory-publish

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-publish</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:40</code> | <code>pub fn publish(&amp;self, digest: &amp;[u8; 32], signer: &amp;[u8; 20], signature: &amp;[u8; 65]) -&gt; Result&lt;(), JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-cosign-directory-publish</code> | notes | method layerx_bridge_relayer::cosign::CosignDirectory::publish |

## relayer-create-program-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-create-program-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:116</code> | <code>pub fn create_program_address(seeds: &amp;[&amp;[u8]], program_id: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-create-program-address</code> | notes | fn layerx_bridge_relayer::solana::release::create_program_address; no reference outside the defining file found by name match |

## relayer-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:49</code> | <code>pub fn decode(value: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, HexError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode</code> | notes | fn layerx_bridge_relayer::hex::decode |

## relayer-decode-address-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode-address-list</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:241</code> | <code>pub fn decode_address_list(data: &amp;[u8]) -&gt; Result&lt;Vec&lt;[u8; 20]&gt;, AbiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode-address-list</code> | notes | fn layerx_bridge_relayer::abi::decode_address_list |

## relayer-decode-bool

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode-bool</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:217</code> | <code>pub fn decode_bool(data: &amp;[u8]) -&gt; Result&lt;bool, AbiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode-bool</code> | notes | fn layerx_bridge_relayer::abi::decode_bool |

## relayer-decode-burn-log

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode-burn-log</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:428</code> | <code>pub fn decode_burn_log(value: &amp;Value) -&gt; Result&lt;BurnLog, AbiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode-burn-log</code> | notes | fn layerx_bridge_relayer::abi::decode_burn_log |

## relayer-decode-deposit-log

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode-deposit-log</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:383</code> | <code>pub fn decode_deposit_log(value: &amp;Value, vault: &amp;[u8; 20]) -&gt; Result&lt;DepositLog, AbiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode-deposit-log</code> | notes | fn layerx_bridge_relayer::abi::decode_deposit_log |

## relayer-decode-frame

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode-frame</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:1105</code> | <code>pub fn decode_frame(frame: &amp;[u8; FRAME_LENGTH]) -&gt; Result&lt;Share, String&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode-frame</code> | notes | fn layerx_bridge_relayer::cosign::transport::decode_frame; no reference outside the defining file found by name match |

## relayer-decode-get-attestors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode-get-attestors</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:258</code> | <code>pub fn decode_get_attestors(data: &amp;[u8]) -&gt; Result&lt;AttestorSet, AbiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode-get-attestors</code> | notes | fn layerx_bridge_relayer::abi::decode_get_attestors |

## relayer-decode-get-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode-get-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:281</code> | <code>pub fn decode_get_chain(data: &amp;[u8]) -&gt; Result&lt;ChainRegistration, AbiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode-get-chain</code> | notes | fn layerx_bridge_relayer::abi::decode_get_chain |

## relayer-decode-threshold

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-decode-threshold</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:229</code> | <code>pub fn decode_threshold(data: &amp;[u8]) -&gt; Result&lt;u64, AbiError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-decode-threshold</code> | notes | fn layerx_bridge_relayer::abi::decode_threshold |

## relayer-delivery-workers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-delivery-workers</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:285</code> | <code>pub const DELIVERY_WORKERS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-delivery-workers</code> | notes | const layerx_bridge_relayer::cosign::transport::DELIVERY_WORKERS; no reference outside the defining file found by name match |

## relayer-deposit-data-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-data-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:28</code> | <code>pub const DEPOSIT_DATA_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-data-bytes</code> | notes | const layerx_bridge_relayer::solana::observe::DEPOSIT_DATA_BYTES; no reference outside the defining file found by name match |

## relayer-deposit-instruction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-instruction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:280</code> | <code>pub fn deposit_instruction(data: &amp;[u8]) -&gt; Option&lt;(u64, [u8; 32])&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-instruction</code> | notes | fn layerx_bridge_relayer::solana::observe::deposit_instruction; no reference outside the defining file found by name match |

## relayer-deposit-log

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-log</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:352</code> | <code>pub struct DepositLog</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-log</code> | notes | struct layerx_bridge_relayer::abi::DepositLog; no reference outside the defining file found by name match |

## relayer-deposit-log-attestation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-log-attestation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:363</code> | <code>pub const fn attestation(&amp;self, chain_id: u64, vault: [u8; 20]) -&gt; InboundAttestation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-log-attestation</code> | notes | method layerx_bridge_relayer::abi::DepositLog::attestation; no reference outside the defining file found by name match |

## relayer-deposit-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-receipt</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:80</code> | <code>pub struct DepositReceipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-receipt</code> | notes | struct layerx_bridge_relayer::solana::observe::DepositReceipt; no reference outside the defining file found by name match |

## relayer-deposit-receipt-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-receipt-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:129</code> | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Option&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-receipt-decode</code> | notes | method layerx_bridge_relayer::solana::observe::DepositReceipt::decode; no reference outside the defining file found by name match |

## relayer-deposit-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:42</code> | <code>pub const DEPOSIT_RECORD: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-record</code> | notes | const layerx_bridge_relayer::solana::observe::DEPOSIT_RECORD; no reference outside the defining file found by name match |

## relayer-deposit-record-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-record-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:182</code> | <code>pub fn parse(line: &amp;str) -&gt; Option&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-record-parse</code> | notes | method layerx_bridge_relayer::solana::observe::DepositRecord::parse; no reference outside the defining file found by name match |

## relayer-deposit-records

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deposit-records</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:266</code> | <code>pub fn deposit_records(logs: &amp;[String], program: &amp;[u8; 32]) -&gt; Result&lt;Vec&lt;DepositRecord&gt;, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deposit-records</code> | notes | fn layerx_bridge_relayer::solana::observe::deposit_records; no reference outside the defining file found by name match |

## relayer-deserialize

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-deserialize</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:118</code> | <code>pub fn deserialize&lt;'de, D: Deserializer&lt;'de&gt;, const N: usize&gt;(deserializer: D) -&gt; Result&lt;[u8; N], D::Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-deserialize</code> | notes | fn layerx_bridge_relayer::hex::fixed_serde::deserialize; no reference outside the defining file found by name match |

## relayer-destination-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-destination-policy</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:407</code> | <code>pub struct DestinationPolicy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-destination-policy</code> | notes | struct layerx_bridge_relayer::cosign::transport::DestinationPolicy |

## relayer-disagreement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-disagreement</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:49</code> | <code>pub const DISAGREEMENT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-disagreement</code> | notes | const layerx_bridge_relayer::solana::observe::DISAGREEMENT |

## relayer-domain-in

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-domain-in</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:13</code> | <code>pub const DOMAIN_IN: &amp;[u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-domain-in</code> | notes | const layerx_bridge_relayer::attestation::DOMAIN_IN; no reference outside the defining file found by name match |

## relayer-domain-out

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-domain-out</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:15</code> | <code>pub const DOMAIN_OUT: &amp;[u8; 21]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-domain-out</code> | notes | const layerx_bridge_relayer::attestation::DOMAIN_OUT; no reference outside the defining file found by name match |

## relayer-enable-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-enable-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:280</code> | <code>pub const ENABLE_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-enable-variable</code> | notes | const layerx_bridge_relayer::cosign::transport::ENABLE_VARIABLE |

## relayer-encode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-encode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:20</code> | <code>pub fn encode(bytes: &amp;[u8]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-encode</code> | notes | fn layerx_bridge_relayer::hex::encode |

## relayer-encode-bridge-in

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-encode-bridge-in</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:101</code> | <code>pub fn encode_bridge_in(attestation: &amp;InboundAttestation, signatures: &amp;[[u8; 65]]) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-encode-bridge-in</code> | notes | fn layerx_bridge_relayer::abi::encode_bridge_in |

## relayer-encode-get-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-encode-get-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:144</code> | <code>pub fn encode_get_chain(chain_id: u64) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-encode-get-chain</code> | notes | fn layerx_bridge_relayer::abi::encode_get_chain |

## relayer-encode-is-nullified

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-encode-is-nullified</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:133</code> | <code>pub fn encode_is_nullified(chain_id: u64, tx_hash: &amp;[u8; 32], log_index: u64) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-encode-is-nullified</code> | notes | fn layerx_bridge_relayer::abi::encode_is_nullified |

## relayer-encode-nullified

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-encode-nullified</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:152</code> | <code>pub fn encode_nullified(nullifier: &amp;[u8; 32]) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-encode-nullified</code> | notes | fn layerx_bridge_relayer::abi::encode_nullified |

## relayer-encode-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-encode-release</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:118</code> | <code>pub fn encode_release(attestation: &amp;OutboundAttestation, signatures: &amp;[[u8; 65]]) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-encode-release</code> | notes | fn layerx_bridge_relayer::abi::encode_release |

## relayer-encode-wire

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-encode-wire</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:1083</code> | <code>pub fn encode_wire(recipient: &amp;[u8; 20], digest: &amp;[u8; 32], signer: &amp;[u8; 20], signature: &amp;[u8; 65]) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-encode-wire</code> | notes | fn layerx_bridge_relayer::cosign::transport::encode_wire |

## relayer-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-entry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:239</code> | <code>pub enum Entry</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-entry</code> | notes | enum layerx_bridge_relayer::journal::Entry; 68 reference lines in 3 files; callers lists the first reference in each file |

## relayer-ethereum-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-ethereum-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:164</code> | <code>pub fn ethereum_address(key: &amp;VerifyingKey) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-ethereum-address</code> | notes | fn layerx_bridge_relayer::attestation::ethereum_address |

## relayer-ethereum-transaction-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-ethereum-transaction-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:21</code> | <code>pub const ETHEREUM_TRANSACTION_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-ethereum-transaction-domain</code> | notes | const layerx_bridge_relayer::signer::ETHEREUM_TRANSACTION_DOMAIN |

## relayer-fee-payer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:167</code> | <code>pub struct FeePayer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-fee-payer</code> | notes | struct layerx_bridge_relayer::signer::FeePayer |

## relayer-fee-payer-algorithm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-algorithm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:215</code> | <code>pub const FEE_PAYER_ALGORITHM: SigningAlgorithm</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-algorithm</code> | notes | const layerx_bridge_relayer::signer::FEE_PAYER_ALGORITHM |

## relayer-fee-payer-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:177</code> | <code>pub fn new(signer: RemoteChainSigner) -&gt; Result&lt;Self, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-new</code> | notes | method layerx_bridge_relayer::signer::FeePayer::new |

## relayer-fee-payer-public-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-public-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:190</code> | <code>pub const fn public_key(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-public-key</code> | notes | method layerx_bridge_relayer::signer::FeePayer::public_key |

## relayer-fee-payer-sign-message

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-sign-message</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:200</code> | <code>pub fn sign_message(&amp;self, message: &amp;[u8]) -&gt; Result&lt;[u8; 64], KeyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-fee-payer-sign-message</code> | notes | method layerx_bridge_relayer::signer::FeePayer::sign_message |

## relayer-find-program-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-find-program-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:133</code> | <code>pub fn find_program_address(seeds: &amp;[&amp;[u8]], program_id: &amp;[u8; 32]) -&gt; Option&lt;([u8; 32], u8)&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-find-program-address</code> | notes | fn layerx_bridge_relayer::solana::release::find_program_address |

## relayer-finding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-finding</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:99</code> | <code>pub enum Finding</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-finding</code> | notes | enum layerx_bridge_relayer::solana::observe::Finding |

## relayer-fixed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-fixed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:69</code> | <code>pub fn fixed&lt;const N: usize&gt;(value: &amp;str) -&gt; Result&lt;[u8; N], HexError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-fixed</code> | notes | fn layerx_bridge_relayer::hex::fixed; 61 reference lines in 8 files; callers lists the first reference in each file |

## relayer-frame-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-frame-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:281</code> | <code>pub const FRAME_LENGTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-frame-length</code> | notes | const layerx_bridge_relayer::cosign::transport::FRAME_LENGTH; no reference outside the defining file found by name match |

## relayer-gas-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-gas-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:72</code> | <code>pub struct GasConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-gas-config</code> | notes | struct layerx_bridge_relayer::config::GasConfig; no reference outside the defining file found by name match |

## relayer-gas-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-gas-policy</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:190</code> | <code>pub struct GasPolicy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-gas-policy</code> | notes | struct layerx_bridge_relayer::relayer::GasPolicy |

## relayer-get-attestors-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-get-attestors-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:34</code> | <code>pub const GET_ATTESTORS_SELECTOR: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-get-attestors-selector</code> | notes | const layerx_bridge_relayer::abi::GET_ATTESTORS_SELECTOR |

## relayer-get-chain-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-get-chain-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:36</code> | <code>pub const GET_CHAIN_SELECTOR: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-get-chain-selector</code> | notes | const layerx_bridge_relayer::abi::GET_CHAIN_SELECTOR; no reference outside the defining file found by name match |

## relayer-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-handle</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:63</code> | <code>pub fn handle(key: &amp;[u8; 32]) -&gt; [u8; HANDLE_BYTES]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-handle</code> | notes | fn layerx_bridge_relayer::solana::handle; 83 reference lines in 11 files; callers lists the first reference in each file |

## relayer-handle-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-handle-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:35</code> | <code>pub const HANDLE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-handle-bytes</code> | notes | const layerx_bridge_relayer::solana::HANDLE_BYTES |

## relayer-hex-bytes-serde-deserialize

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-hex-bytes-serde-deserialize</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:140</code> | <code>pub fn deserialize&lt;'de, D: Deserializer&lt;'de&gt;&gt;(deserializer: D) -&gt; Result&lt;Vec&lt;u8&gt;, D::Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-hex-bytes-serde-deserialize</code> | notes | fn layerx_bridge_relayer::hex::bytes_serde::deserialize; no reference outside the defining file found by name match |

## relayer-hex-bytes-serde-serialize

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-hex-bytes-serde-serialize</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:133</code> | <code>pub fn serialize&lt;S: Serializer&gt;(value: &amp;[u8], serializer: S) -&gt; Result&lt;S::Ok, S::Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-hex-bytes-serde-serialize</code> | notes | fn layerx_bridge_relayer::hex::bytes_serde::serialize; no reference outside the defining file found by name match |

## relayer-hex-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-hex-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:6</code> | <code>pub struct HexError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-hex-error</code> | notes | struct layerx_bridge_relayer::hex::HexError; no reference outside the defining file found by name match |

## relayer-in-preimage-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-in-preimage-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:16</code> | <code>pub const IN_PREIMAGE_LENGTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-in-preimage-length</code> | notes | const layerx_bridge_relayer::attestation::IN_PREIMAGE_LENGTH; no reference outside the defining file found by name match |

## relayer-inbound-attestation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:36</code> | <code>pub struct InboundAttestation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation</code> | notes | struct layerx_bridge_relayer::attestation::InboundAttestation |

## relayer-inbound-attestation-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:65</code> | <code>pub fn digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-digest</code> | notes | method layerx_bridge_relayer::attestation::InboundAttestation::digest; no reference outside the defining file found by name match |

## relayer-inbound-attestation-paxeer-recipient

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-paxeer-recipient</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:73</code> | <code>pub fn paxeer_recipient(&amp;self) -&gt; Option&lt;[u8; 20]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-paxeer-recipient</code> | notes | method layerx_bridge_relayer::attestation::InboundAttestation::paxeer_recipient; no reference outside the defining file found by name match |

## relayer-inbound-attestation-preimage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-preimage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:50</code> | <code>pub fn preimage(&amp;self) -&gt; [u8; IN_PREIMAGE_LENGTH]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-inbound-attestation-preimage</code> | notes | method layerx_bridge_relayer::attestation::InboundAttestation::preimage |

## relayer-inbound-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:52</code> | <code>pub fn inbound_key(chain_id: u64, tx_hash: &amp;[u8; 32], log_index: u64) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-inbound-key</code> | notes | fn layerx_bridge_relayer::journal::inbound_key |

## relayer-inbound-stream

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-stream</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:328</code> | <code>pub fn inbound_stream(chain_id: u64) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-inbound-stream</code> | notes | fn layerx_bridge_relayer::relayer::inbound_stream |

## relayer-inbound-tx-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-inbound-tx-hash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:73</code> | <code>pub fn inbound_tx_hash(signature: &amp;[u8; 64]) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-inbound-tx-hash</code> | notes | fn layerx_bridge_relayer::solana::inbound_tx_hash |

## relayer-instruction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-instruction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:77</code> | <code>pub struct Instruction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-instruction</code> | notes | struct layerx_bridge_relayer::solana::rpc::Instruction |

## relayer-instruction-magic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-instruction-magic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:23</code> | <code>pub const INSTRUCTION_MAGIC: &amp;[u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-instruction-magic</code> | notes | const layerx_bridge_relayer::solana::observe::INSTRUCTION_MAGIC |

## relayer-instruction-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-instruction-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:24</code> | <code>pub const INSTRUCTION_VERSION: u16</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-instruction-version</code> | notes | const layerx_bridge_relayer::solana::observe::INSTRUCTION_VERSION |

## relayer-instructions-sysvar

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-instructions-sysvar</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:63</code> | <code>pub const INSTRUCTIONS_SYSVAR: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-instructions-sysvar</code> | notes | const layerx_bridge_relayer::solana::release::INSTRUCTIONS_SYSVAR |

## relayer-inventory-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-inventory-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:368</code> | <code>pub const INVENTORY_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-inventory-variable</code> | notes | const layerx_bridge_relayer::cosign::transport::INVENTORY_VARIABLE |

## relayer-is-nullified-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-is-nullified-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:32</code> | <code>pub const IS_NULLIFIED_SELECTOR: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-is-nullified-selector</code> | notes | const layerx_bridge_relayer::abi::IS_NULLIFIED_SELECTOR; no reference outside the defining file found by name match |

## relayer-is-token-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-is-token-account</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:260</code> | <code>pub fn is_token_account(data: &amp;[u8], mint: &amp;[u8; 32], owner: &amp;[u8; 32]) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-is-token-account</code> | notes | fn layerx_bridge_relayer::solana::release::is_token_account |

## relayer-item

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-item</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:370</code> | <code>pub struct Item</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-item</code> | notes | struct layerx_bridge_relayer::journal::Item |

## relayer-item-failure

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-item-failure</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:182</code> | <code>pub struct ItemFailure</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-item-failure</code> | notes | struct layerx_bridge_relayer::relayer::ItemFailure; no reference outside the defining file found by name match |

## relayer-item-is-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-item-is-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:395</code> | <code>pub const fn is_open(&amp;self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-item-is-open</code> | notes | method layerx_bridge_relayer::journal::Item::is_open; no reference outside the defining file found by name match |

## relayer-item-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-item-pending</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:401</code> | <code>pub fn pending(&amp;self) -&gt; Option&lt;&amp;Submission&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-item-pending</code> | notes | method layerx_bridge_relayer::journal::Item::pending; no reference outside the defining file found by name match |

## relayer-item-pending-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-item-pending-release</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:412</code> | <code>pub fn pending_release(&amp;self) -&gt; Option&lt;&amp;ReleaseSubmission&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-item-pending-release</code> | notes | method layerx_bridge_relayer::journal::Item::pending_release; no reference outside the defining file found by name match |

## relayer-item-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-item-transactions</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:420</code> | <code>pub fn transactions(&amp;self) -&gt; usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-item-transactions</code> | notes | method layerx_bridge_relayer::journal::Item::transactions; no reference outside the defining file found by name match |

## relayer-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:752</code> | <code>pub struct Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-journal</code> | notes | struct layerx_bridge_relayer::journal::Journal |

## relayer-journal-append

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-journal-append</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:831</code> | <code>pub fn append(&amp;mut self, entry: &amp;Entry) -&gt; Result&lt;(), JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-journal-append</code> | notes | method layerx_bridge_relayer::journal::Journal::append |

## relayer-journal-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-journal-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:28</code> | <code>pub enum JournalError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-journal-error</code> | notes | enum layerx_bridge_relayer::journal::JournalError |

## relayer-journal-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-journal-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:767</code> | <code>pub fn open(path: &amp;Path) -&gt; Result&lt;Self, JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-journal-open</code> | notes | method layerx_bridge_relayer::journal::Journal::open |

## relayer-journal-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-journal-path</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:817</code> | <code>pub fn path(&amp;self) -&gt; &amp;Path</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-journal-path</code> | notes | method layerx_bridge_relayer::journal::Journal::path; no reference outside the defining file found by name match |

## relayer-journal-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-journal-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:822</code> | <code>pub const fn state(&amp;self) -&gt; &amp;State</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-journal-state</code> | notes | method layerx_bridge_relayer::journal::Journal::state; 61 reference lines in 6 files; callers lists the first reference in each file |

## relayer-json-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-json-rpc</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/rpc.rs:69</code> | <code>pub trait JsonRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-json-rpc</code> | notes | trait layerx_bridge_relayer::rpc::JsonRpc; spec/layerx-merge/spec.kvx task 7.5.16 and 11.5.117 move the raw_call transport off layerx-paxeer-verifier (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:3, :10, :100, :137); 8.5.32 send_raw_transaction (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:186) returns the receipt; chain calls: eth_sendRawTransaction at :89 via private fn send_raw_transaction |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-key-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:27</code> | <code>pub enum KeyError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-key-error</code> | notes | enum layerx_bridge_relayer::signer::KeyError |

## relayer-key-handle-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-key-handle-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:63</code> | <code>pub struct KeyHandleConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-key-handle-config</code> | notes | struct layerx_bridge_relayer::config::KeyHandleConfig; no reference outside the defining file found by name match |

## relayer-layerx-bridge-precompile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-layerx-bridge-precompile</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:14</code> | <code>pub const LAYERX_BRIDGE_PRECOMPILE: [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-layerx-bridge-precompile</code> | notes | const layerx_bridge_relayer::abi::LAYERX_BRIDGE_PRECOMPILE; chain calls: 0x1016 at :15 |

## relayer-layout-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-layout-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:35</code> | <code>pub const LAYOUT_VERSION: u16</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-layout-version</code> | notes | const layerx_bridge_relayer::solana::observe::LAYOUT_VERSION |

## relayer-log-position

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-log-position</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:295</code> | <code>pub struct LogPosition</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-log-position</code> | notes | struct layerx_bridge_relayer::abi::LogPosition |

## relayer-magic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-magic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:282</code> | <code>pub const MAGIC: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-magic</code> | notes | const layerx_bridge_relayer::cosign::transport::MAGIC; no reference outside the defining file found by name match |

## relayer-max-attestors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-attestors</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:47</code> | <code>pub const MAX_ATTESTORS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-attestors</code> | notes | const layerx_bridge_relayer::solana::release::MAX_ATTESTORS |

## relayer-max-connections

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-connections</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:287</code> | <code>pub const MAX_CONNECTIONS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-connections</code> | notes | const layerx_bridge_relayer::cosign::transport::MAX_CONNECTIONS; no reference outside the defining file found by name match |

## relayer-max-interval-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-interval-ms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:289</code> | <code>pub const MAX_INTERVAL_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-interval-ms</code> | notes | const layerx_bridge_relayer::cosign::transport::MAX_INTERVAL_MS; no reference outside the defining file found by name match |

## relayer-max-peers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-peers</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:286</code> | <code>pub const MAX_PEERS: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-peers</code> | notes | const layerx_bridge_relayer::cosign::transport::MAX_PEERS; no reference outside the defining file found by name match |

## relayer-max-processing-age

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-processing-age</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:36</code> | <code>pub const MAX_PROCESSING_AGE: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-processing-age</code> | notes | const layerx_bridge_relayer::solana::release::MAX_PROCESSING_AGE |

## relayer-max-signature-page

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-signature-page</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:14</code> | <code>pub const MAX_SIGNATURE_PAGE: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-signature-page</code> | notes | const layerx_bridge_relayer::solana::rpc::MAX_SIGNATURE_PAGE; no reference outside the defining file found by name match |

## relayer-max-sweep-entries

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-sweep-entries</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:284</code> | <code>pub const MAX_SWEEP_ENTRIES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-sweep-entries</code> | notes | const layerx_bridge_relayer::cosign::transport::MAX_SWEEP_ENTRIES; no reference outside the defining file found by name match |

## relayer-max-timeout-ms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-timeout-ms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:288</code> | <code>pub const MAX_TIMEOUT_MS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-timeout-ms</code> | notes | const layerx_bridge_relayer::cosign::transport::MAX_TIMEOUT_MS; no reference outside the defining file found by name match |

## relayer-max-transaction-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-max-transaction-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:17</code> | <code>pub const MAX_TRANSACTION_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-max-transaction-bytes</code> | notes | const layerx_bridge_relayer::solana::rpc::MAX_TRANSACTION_BYTES |

## relayer-nullified-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-nullified-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:44</code> | <code>pub const NULLIFIED_SELECTOR: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-nullified-selector</code> | notes | const layerx_bridge_relayer::abi::NULLIFIED_SELECTOR; no reference outside the defining file found by name match |

## relayer-nullifier-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-nullifier-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:151</code> | <code>pub fn nullifier_address(program_id: &amp;[u8; 32], nullifier: &amp;[u8; 32]) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-nullifier-address</code> | notes | fn layerx_bridge_relayer::solana::release::nullifier_address |

## relayer-nullifier-seed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-nullifier-seed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:42</code> | <code>pub const NULLIFIER_SEED: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-nullifier-seed</code> | notes | const layerx_bridge_relayer::solana::release::NULLIFIER_SEED; no reference outside the defining file found by name match |

## relayer-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-observation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:82</code> | <code>pub enum Observation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-observation</code> | notes | enum layerx_bridge_relayer::journal::Observation |

## relayer-observation-inbound

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-observation-inbound</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:117</code> | <code>pub fn inbound(attestation: &amp;InboundAttestation, position: Position) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-observation-inbound</code> | notes | method layerx_bridge_relayer::journal::Observation::inbound |

## relayer-observation-inbound-attestation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-observation-inbound-attestation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:164</code> | <code>pub const fn inbound_attestation(&amp;self) -&gt; Option&lt;InboundAttestation&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-observation-inbound-attestation</code> | notes | method layerx_bridge_relayer::journal::Observation::inbound_attestation |

## relayer-observation-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-observation-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:146</code> | <code>pub fn key(&amp;self) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-observation-key</code> | notes | method layerx_bridge_relayer::journal::Observation::key; no reference outside the defining file found by name match |

## relayer-observation-outbound

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-observation-outbound</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:131</code> | <code>pub fn outbound(attestation: &amp;OutboundAttestation, position: Position) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-observation-outbound</code> | notes | method layerx_bridge_relayer::journal::Observation::outbound |

## relayer-observation-outbound-attestation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-observation-outbound-attestation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:189</code> | <code>pub const fn outbound_attestation(&amp;self) -&gt; Option&lt;OutboundAttestation&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-observation-outbound-attestation</code> | notes | method layerx_bridge_relayer::journal::Observation::outbound_attestation |

## relayer-observe-deposits

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-observe-deposits</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:482</code> | <code>pub fn observe_deposits(rpc: &amp;SolanaRpc, settings: &amp;SolanaSettings, from: u64, to: u64) -&gt; Result&lt;Vec&lt;Finding&gt;, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-observe-deposits</code> | notes | fn layerx_bridge_relayer::solana::observe::observe_deposits |

## relayer-op-deposit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-op-deposit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:26</code> | <code>pub const OP_DEPOSIT: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-op-deposit</code> | notes | const layerx_bridge_relayer::solana::observe::OP_DEPOSIT; no reference outside the defining file found by name match |

## relayer-op-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-op-release</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:26</code> | <code>pub const OP_RELEASE: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-op-release</code> | notes | const layerx_bridge_relayer::solana::release::OP_RELEASE |

## relayer-operator-fee-payer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-operator-fee-payer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:399</code> | <code>pub struct OperatorFeePayer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-operator-fee-payer</code> | notes | struct layerx_bridge_relayer::cosign::transport::OperatorFeePayer; no reference outside the defining file found by name match |

## relayer-operator-inventory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:373</code> | <code>pub struct OperatorInventory</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory</code> | notes | struct layerx_bridge_relayer::cosign::transport::OperatorInventory |

## relayer-operator-inventory-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:472</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory-load</code> | notes | method layerx_bridge_relayer::cosign::OperatorInventory::load |

## relayer-operator-inventory-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:478</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-operator-inventory-validate</code> | notes | method layerx_bridge_relayer::cosign::OperatorInventory::validate; no reference outside the defining file found by name match |

## relayer-operator-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-operator-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:384</code> | <code>pub struct OperatorRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-operator-record</code> | notes | struct layerx_bridge_relayer::cosign::transport::OperatorRecord; no reference outside the defining file found by name match |

## relayer-out-preimage-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-out-preimage-length</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:17</code> | <code>pub const OUT_PREIMAGE_LENGTH: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-out-preimage-length</code> | notes | const layerx_bridge_relayer::attestation::OUT_PREIMAGE_LENGTH |

## relayer-outbound-attestation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:85</code> | <code>pub struct OutboundAttestation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation</code> | notes | struct layerx_bridge_relayer::attestation::OutboundAttestation |

## relayer-outbound-attestation-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:114</code> | <code>pub fn digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-digest</code> | notes | method layerx_bridge_relayer::attestation::OutboundAttestation::digest; no reference outside the defining file found by name match |

## relayer-outbound-attestation-nullifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-nullifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:120</code> | <code>pub fn nullifier(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-nullifier</code> | notes | method layerx_bridge_relayer::attestation::OutboundAttestation::nullifier |

## relayer-outbound-attestation-preimage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-preimage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:99</code> | <code>pub fn preimage(&amp;self) -&gt; [u8; OUT_PREIMAGE_LENGTH]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-outbound-attestation-preimage</code> | notes | method layerx_bridge_relayer::attestation::OutboundAttestation::preimage |

## relayer-outbound-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:58</code> | <code>pub fn outbound_key(chain_id: u64, paxeer_tx_hash: &amp;[u8; 32], nonce: u64) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-outbound-key</code> | notes | fn layerx_bridge_relayer::journal::outbound_key |

## relayer-outbound-preimage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-preimage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:294</code> | <code>pub fn outbound_preimage(attestation: &amp;OutboundAttestation) -&gt; [u8; OUTBOUND_PREIMAGE_BYTES]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-outbound-preimage</code> | notes | fn layerx_bridge_relayer::solana::release::outbound_preimage; no reference outside the defining file found by name match |

## relayer-outbound-preimage-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-preimage-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:30</code> | <code>pub const OUTBOUND_PREIMAGE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-outbound-preimage-bytes</code> | notes | const layerx_bridge_relayer::solana::release::OUTBOUND_PREIMAGE_BYTES; no reference outside the defining file found by name match |

## relayer-outbound-stream

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-outbound-stream</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:333</code> | <code>pub const OUTBOUND_STREAM: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-outbound-stream</code> | notes | const layerx_bridge_relayer::relayer::OUTBOUND_STREAM; chain calls: eth_call at :358 via private fn eth_call, eth_blockNumber at :368 via private fn block_number, eth_chainId at :372 via private fn chain_id, eth_getBlockByNumber at :377 via private fn canonical_hash |

## relayer-parse-quantity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-parse-quantity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:97</code> | <code>pub fn parse_quantity(value: &amp;str) -&gt; Result&lt;u64, HexError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-parse-quantity</code> | notes | fn layerx_bridge_relayer::hex::parse_quantity |

## relayer-parse-quantity-u128

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-parse-quantity-u128</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:84</code> | <code>pub fn parse_quantity_u128(value: &amp;str) -&gt; Result&lt;u128, HexError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-parse-quantity-u128</code> | notes | fn layerx_bridge_relayer::hex::parse_quantity_u128 |

## relayer-paxeer-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:102</code> | <code>pub struct PaxeerConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-paxeer-config</code> | notes | struct layerx_bridge_relayer::config::PaxeerConfig; no reference outside the defining file found by name match |

## relayer-paxeer-endpoint-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-endpoint-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:90</code> | <code>pub struct PaxeerEndpointConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-paxeer-endpoint-config</code> | notes | struct layerx_bridge_relayer::config::PaxeerEndpointConfig; no reference outside the defining file found by name match |

## relayer-paxeer-link

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-link</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:224</code> | <code>pub struct PaxeerLink</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-paxeer-link</code> | notes | struct layerx_bridge_relayer::relayer::PaxeerLink |

## relayer-paxeer-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-rpc</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/rpc.rs:104</code> | <code>pub struct PaxeerRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-paxeer-rpc</code> | notes | struct layerx_bridge_relayer::rpc::PaxeerRpc; spec/layerx-merge/spec.kvx task 7.5.16 and 11.5.117 move the raw_call transport off layerx-paxeer-verifier (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:3, :10, :100, :137); 8.5.32 send_raw_transaction (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:186) returns the receipt |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-rpc-new</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/rpc.rs:113</code> | <code>pub fn new(endpoints: Vec&lt;EndpointConfig&gt;) -&gt; Result&lt;Self, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-paxeer-rpc-new</code> | notes | method layerx_bridge_relayer::rpc::PaxeerRpc::new; spec/layerx-merge/spec.kvx task 7.5.16 and 11.5.117 move the raw_call transport off layerx-paxeer-verifier (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:3, :10, :100, :137); 8.5.32 send_raw_transaction (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:186) returns the receipt; chain calls: eth_sendRawTransaction at :189 via private fn send_raw_transaction |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-settings</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:209</code> | <code>pub struct PaxeerSettings</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-paxeer-settings</code> | notes | struct layerx_bridge_relayer::relayer::PaxeerSettings |

## relayer-paxeer-transaction-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-paxeer-transaction-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:19</code> | <code>pub const PAXEER_TRANSACTION_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-paxeer-transaction-domain</code> | notes | const layerx_bridge_relayer::signer::PAXEER_TRANSACTION_DOMAIN |

## relayer-peer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-peer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:322</code> | <code>pub struct Peer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-peer</code> | notes | struct layerx_bridge_relayer::cosign::transport::Peer; no reference outside the defining file found by name match |

## relayer-peer-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-peer-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:298</code> | <code>pub struct PeerConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-peer-config</code> | notes | struct layerx_bridge_relayer::cosign::transport::PeerConfig; no reference outside the defining file found by name match |

## relayer-position

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-position</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:64</code> | <code>pub struct Position</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-position</code> | notes | struct layerx_bridge_relayer::journal::Position |

## relayer-prefixed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-prefixed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:31</code> | <code>pub fn prefixed(bytes: &amp;[u8]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-prefixed</code> | notes | fn layerx_bridge_relayer::hex::prefixed; 62 reference lines in 14 files; callers lists the first reference in each file |

## relayer-program-log-lines

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-program-log-lines</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:225</code> | <code>pub fn program_log_lines&lt;'a&gt;(logs: &amp;'a [String], program: &amp;[u8; 32]) -&gt; Result&lt;Vec&lt;&amp;'a str&gt;, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-program-log-lines</code> | notes | fn layerx_bridge_relayer::solana::observe::program_log_lines; no reference outside the defining file found by name match |

## relayer-quantity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-quantity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:75</code> | <code>pub fn quantity(value: u64) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-quantity</code> | notes | fn layerx_bridge_relayer::hex::quantity |

## relayer-receipt-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-receipt-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:37</code> | <code>pub const RECEIPT_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-receipt-bytes</code> | notes | const layerx_bridge_relayer::solana::observe::RECEIPT_BYTES; no reference outside the defining file found by name match |

## relayer-receipt-magic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-receipt-magic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:36</code> | <code>pub const RECEIPT_MAGIC: &amp;[u8; 8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-receipt-magic</code> | notes | const layerx_bridge_relayer::solana::observe::RECEIPT_MAGIC; no reference outside the defining file found by name match |

## relayer-recipient

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-recipient</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:362</code> | <code>pub enum Recipient</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-recipient</code> | notes | enum layerx_bridge_relayer::journal::Recipient |

## relayer-recipient-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:157</code> | <code>pub fn recipient_address(program_id: &amp;[u8; 32], handle: &amp;[u8; HANDLE_BYTES]) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-recipient-address</code> | notes | fn layerx_bridge_relayer::solana::release::recipient_address |

## relayer-recipient-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:51</code> | <code>pub const RECIPIENT_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-recipient-bytes</code> | notes | const layerx_bridge_relayer::solana::release::RECIPIENT_BYTES; no reference outside the defining file found by name match |

## relayer-recipient-magic

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-magic</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:45</code> | <code>pub const RECIPIENT_MAGIC: &amp;[u8; 8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-recipient-magic</code> | notes | const layerx_bridge_relayer::solana::release::RECIPIENT_MAGIC; no reference outside the defining file found by name match |

## relayer-recipient-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:234</code> | <code>pub struct RecipientRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-recipient-record</code> | notes | struct layerx_bridge_relayer::solana::release::RecipientRecord |

## relayer-recipient-record-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-record-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:243</code> | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Option&lt;Self&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-recipient-record-decode</code> | notes | method layerx_bridge_relayer::solana::release::RecipientRecord::decode |

## relayer-recipient-seed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-recipient-seed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:41</code> | <code>pub const RECIPIENT_SEED: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-recipient-seed</code> | notes | const layerx_bridge_relayer::solana::release::RECIPIENT_SEED; no reference outside the defining file found by name match |

## relayer-recover-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-recover-signer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:196</code> | <code>pub fn recover_signer(digest: &amp;[u8; 32], signature: &amp;[u8; 65]) -&gt; Result&lt;[u8; 20], SignatureError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-recover-signer</code> | notes | fn layerx_bridge_relayer::attestation::recover_signer |

## relayer-relayer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:407</code> | <code>pub struct Relayer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer</code> | notes | struct layerx_bridge_relayer::relayer::Relayer |
| <code>interface.11.rs-relayer-relayer</code> | overridden_by | override.J48.45 |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-assembly</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:249</code> | <code>pub struct RelayerAssembly</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-assembly</code> | notes | struct layerx_bridge_relayer::relayer::RelayerAssembly |

## relayer-relayer-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:289</code> | <code>pub struct RelayerConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-config</code> | notes | struct layerx_bridge_relayer::config::RelayerConfig; spec/layerx-merge/spec.kvx task 7.5.16 drops the verifier client from config load (services/interop/crates/layerx-bridge-relayer/src/config.rs:15, :320) and adds receipt_keys [{key_id, public_key, first_height, last_height}] |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-build</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:456</code> | <code>pub fn build(&amp;self) -&gt; Result&lt;RelayerSetup, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-build</code> | notes | method layerx_bridge_relayer::config::RelayerConfig::build; no reference outside the defining file found by name match |

## relayer-relayer-config-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-load</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:320</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-load</code> | notes | method layerx_bridge_relayer::config::RelayerConfig::load; spec/layerx-merge/spec.kvx task 7.5.16 drops the verifier client from config load (services/interop/crates/layerx-bridge-relayer/src/config.rs:15, :320) and adds receipt_keys [{key_id, public_key, first_height, last_height}] |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-validate</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:337</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-validate</code> | notes | method layerx_bridge_relayer::config::RelayerConfig::validate; no reference outside the defining file found by name match |
| <code>interface.11.rs-relayer-relayer-config-validate</code> | overridden_by | override.J48.46 |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:369</code> | <code>pub const RELAYER_CONFIG_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-config-variable</code> | notes | const layerx_bridge_relayer::cosign::transport::RELAYER_CONFIG_VARIABLE |

## relayer-relayer-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:77</code> | <code>pub enum RelayerError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-error</code> | notes | enum layerx_bridge_relayer::relayer::RelayerError |

## relayer-relayer-error-is-item-scoped

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-error-is-item-scoped</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:165</code> | <code>pub const fn is_item_scoped(&amp;self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-error-is-item-scoped</code> | notes | method layerx_bridge_relayer::relayer::RelayerError::is_item_scoped; no reference outside the defining file found by name match |

## relayer-relayer-failures

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-failures</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:574</code> | <code>pub fn failures(&amp;self) -&gt; &amp;[ItemFailure]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-failures</code> | notes | method layerx_bridge_relayer::relayer::Relayer::failures |

## relayer-relayer-inbound-step

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-inbound-step</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:623</code> | <code>pub fn inbound_step(&amp;mut self, index: usize) -&gt; Result&lt;StepReport, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-inbound-step</code> | notes | method layerx_bridge_relayer::relayer::Relayer::inbound_step; spec/layerx-merge/spec.kvx task 8.5.32 confirms bridgeIn from the sequencer receipt (services/interop/crates/layerx-bridge-relayer/src/relayer.rs:623) |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:567</code> | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-journal</code> | notes | method layerx_bridge_relayer::relayer::Relayer::journal |

## relayer-relayer-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:463</code> | <code>pub fn new(setup: impl Into&lt;RelayerSetup&gt;) -&gt; Result&lt;Self, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-new</code> | notes | method layerx_bridge_relayer::relayer::Relayer::new |

## relayer-relayer-outbound-step

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-outbound-step</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:749</code> | <code>pub fn outbound_step(&amp;mut self) -&gt; Result&lt;StepReport, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-outbound-step</code> | notes | method layerx_bridge_relayer::relayer::Relayer::outbound_step; spec/layerx-merge/spec.kvx task 8.5.32 do_3 keeps outbound_step; chain calls: eth_getLogs at :874 via private fn scan_inbound, eth_getLogs at :950 via private fn scan_outbound, eth_getTransactionCount at :1222 via private fn next_nonce, eth_estimateGas at :1251 via private fn prepare_transaction, eth_maxPriorityFeePerGas at :1272 via private fn prepare_transaction, eth_gasPrice at :1274 via private fn prepare_transaction, eth_getTransactionReceipt at :1310 via private fn resolve, eth_getTransactionByHash at :1312 via private fn resolve |

## relayer-relayer-parts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-parts</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:230</code> | <code>pub struct RelayerParts</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-parts</code> | notes | struct layerx_bridge_relayer::relayer::RelayerParts |

## relayer-relayer-setup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-setup</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:272</code> | <code>pub struct RelayerSetup</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-setup</code> | notes | struct layerx_bridge_relayer::relayer::RelayerSetup |

## relayer-relayer-solana-step

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-solana-step</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:662</code> | <code>pub fn solana_step(&amp;mut self) -&gt; Result&lt;StepReport, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-solana-step</code> | notes | method layerx_bridge_relayer::relayer::Relayer::solana_step |

## relayer-relayer-tick

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-relayer-tick</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:580</code> | <code>pub fn tick(&amp;mut self) -&gt; Vec&lt;(String, Result&lt;StepReport, RelayerError&gt;)&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-relayer-tick</code> | notes | method layerx_bridge_relayer::relayer::Relayer::tick |

## relayer-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-release</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:285</code> | <code>pub struct Release</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-release</code> | notes | struct layerx_bridge_relayer::solana::release::Release |

## relayer-release-accounts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-release-accounts</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:269</code> | <code>pub struct ReleaseAccounts</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-release-accounts</code> | notes | struct layerx_bridge_relayer::solana::release::ReleaseAccounts |

## relayer-release-data-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-release-data-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:28</code> | <code>pub const RELEASE_DATA_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-release-data-bytes</code> | notes | const layerx_bridge_relayer::solana::release::RELEASE_DATA_BYTES; no reference outside the defining file found by name match |

## relayer-release-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-release-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:82</code> | <code>pub enum ReleaseError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-release-error</code> | notes | enum layerx_bridge_relayer::solana::release::ReleaseError |

## relayer-release-instruction-data

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-release-instruction-data</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:356</code> | <code>pub fn release_instruction_data(attestation: &amp;OutboundAttestation, recipient: &amp;[u8; 32]) -&gt; Result&lt;Vec&lt;u8&gt;, ReleaseError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-release-instruction-data</code> | notes | fn layerx_bridge_relayer::solana::release::release_instruction_data; no reference outside the defining file found by name match |

## relayer-release-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-release-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:38</code> | <code>pub const RELEASE_SELECTOR: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-release-selector</code> | notes | const layerx_bridge_relayer::abi::RELEASE_SELECTOR; no reference outside the defining file found by name match |

## relayer-release-submission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-release-submission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:352</code> | <code>pub struct ReleaseSubmission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-release-submission</code> | notes | struct layerx_bridge_relayer::journal::ReleaseSubmission |

## relayer-reply-already-held

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-reply-already-held</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:293</code> | <code>pub const REPLY_ALREADY_HELD: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-reply-already-held</code> | notes | const layerx_bridge_relayer::cosign::transport::REPLY_ALREADY_HELD |

## relayer-reply-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-reply-conflict</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:294</code> | <code>pub const REPLY_CONFLICT: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-reply-conflict</code> | notes | const layerx_bridge_relayer::cosign::transport::REPLY_CONFLICT; no reference outside the defining file found by name match |

## relayer-reply-stored

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-reply-stored</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:292</code> | <code>pub const REPLY_STORED: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-reply-stored</code> | notes | const layerx_bridge_relayer::cosign::transport::REPLY_STORED |

## relayer-rpc-fault

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-rpc-fault</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/rpc.rs:16</code> | <code>pub enum RpcFault</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-rpc-fault</code> | notes | enum layerx_bridge_relayer::rpc::RpcFault; 96 reference lines in 6 files; callers lists the first reference in each file |

## relayer-secp256-k1-offsets-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-offsets-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:32</code> | <code>pub const SECP256K1_OFFSETS_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-offsets-bytes</code> | notes | const layerx_bridge_relayer::solana::release::SECP256K1_OFFSETS_BYTES; no reference outside the defining file found by name match |

## relayer-secp256-k1-program

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-program</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:58</code> | <code>pub const SECP256K1_PROGRAM: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-program</code> | notes | const layerx_bridge_relayer::solana::release::SECP256K1_PROGRAM |

## relayer-secp256-k1-signature-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-signature-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:34</code> | <code>pub const SECP256K1_SIGNATURE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-secp256-k1-signature-bytes</code> | notes | const layerx_bridge_relayer::solana::release::SECP256K1_SIGNATURE_BYTES; no reference outside the defining file found by name match |

## relayer-secp256k1-instruction-data

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-secp256k1-instruction-data</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:313</code> | <code>pub fn secp256k1_instruction_data(signatures: &amp;[([u8; HANDLE_BYTES], [u8; 65])], preimage: &amp;[u8; OUTBOUND_PREIMAGE_BYTES], own: u8) -&gt; Result&lt;Vec&lt;u8&gt;, ReleaseError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-secp256k1-instruction-data</code> | notes | fn layerx_bridge_relayer::solana::release::secp256k1_instruction_data; no reference outside the defining file found by name match |

## relayer-send-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-send-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/rpc.rs:62</code> | <code>pub enum SendOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-send-outcome</code> | notes | enum layerx_bridge_relayer::rpc::SendOutcome |

## relayer-serialize

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-serialize</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/hex.rs:108</code> | <code>pub fn serialize&lt;S: Serializer, const N: usize&gt;(value: &amp;[u8; N], serializer: S) -&gt; Result&lt;S::Ok, S::Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-serialize</code> | notes | fn layerx_bridge_relayer::hex::fixed_serde::serialize; no reference outside the defining file found by name match |

## relayer-share

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-share</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:1074</code> | <code>pub struct Share</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-share</code> | notes | struct layerx_bridge_relayer::cosign::transport::Share; no reference outside the defining file found by name match |

## relayer-shortvec

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-shortvec</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:394</code> | <code>pub fn shortvec(out: &amp;mut Vec&lt;u8&gt;, value: usize)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-shortvec</code> | notes | fn layerx_bridge_relayer::solana::release::shortvec; no reference outside the defining file found by name match |

## relayer-sign

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-sign</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/tx.rs:76</code> | <code>pub fn sign(call: &amp;Eip1559Call, submitter: &amp;Submitter) -&gt; Result&lt;SignedTransaction, KeyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-sign</code> | notes | fn layerx_bridge_relayer::tx::sign |

## relayer-signature-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-signature-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:129</code> | <code>pub enum SignatureError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-signature-error</code> | notes | enum layerx_bridge_relayer::attestation::SignatureError |

## relayer-signature-info

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-signature-info</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:68</code> | <code>pub struct SignatureInfo</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-signature-info</code> | notes | struct layerx_bridge_relayer::solana::rpc::SignatureInfo |

## relayer-signature-page

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-signature-page</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:46</code> | <code>pub const SIGNATURE_PAGE: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-signature-page</code> | notes | const layerx_bridge_relayer::solana::observe::SIGNATURE_PAGE; no reference outside the defining file found by name match |

## relayer-signature-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-signature-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:118</code> | <code>pub struct SignatureStatus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-signature-status</code> | notes | struct layerx_bridge_relayer::solana::rpc::SignatureStatus; no reference outside the defining file found by name match |

## relayer-signed-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-signed-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/tx.rs:13</code> | <code>pub struct SignedTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-signed-transaction</code> | notes | struct layerx_bridge_relayer::tx::SignedTransaction |

## relayer-signer-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-signer-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:55</code> | <code>pub struct SignerConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-signer-config</code> | notes | struct layerx_bridge_relayer::config::SignerConfig; no reference outside the defining file found by name match |

## relayer-signer-transport-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-signer-transport-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:40</code> | <code>pub enum SignerTransportConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-signer-transport-config</code> | notes | enum layerx_bridge_relayer::config::SignerTransportConfig |

## relayer-solana-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-chain-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs:32</code> | <code>pub const SOLANA_CHAIN_ID: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-chain-id</code> | notes | const layerx_bridge_relayer::solana::SOLANA_CHAIN_ID |

## relayer-solana-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:130</code> | <code>pub struct SolanaConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-config</code> | notes | struct layerx_bridge_relayer::config::SolanaConfig; no reference outside the defining file found by name match |

## relayer-solana-config-mints

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-config-mints</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:217</code> | <code>pub fn mints(&amp;self) -&gt; Result&lt;Vec&lt;[u8; 32]&gt;, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-config-mints</code> | notes | method layerx_bridge_relayer::config::SolanaConfig::mints; no reference outside the defining file found by name match |

## relayer-solana-config-settings

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-config-settings</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs:160</code> | <code>pub fn settings(&amp;self) -&gt; Result&lt;SolanaSettings, RelayerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-config-settings</code> | notes | method layerx_bridge_relayer::config::SolanaConfig::settings; no reference outside the defining file found by name match |

## relayer-solana-link

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-link</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:242</code> | <code>pub struct SolanaLink</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-link</code> | notes | struct layerx_bridge_relayer::relayer::SolanaLink |

## relayer-solana-observe-deposit-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-observe-deposit-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:68</code> | <code>pub struct DepositRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-observe-deposit-record</code> | notes | struct layerx_bridge_relayer::solana::observe::DepositRecord; no reference outside the defining file found by name match |

## relayer-solana-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-release</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:266</code> | <code>pub struct SolanaRelease</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-release</code> | notes | struct layerx_bridge_relayer::relayer::SolanaRelease |

## relayer-solana-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:125</code> | <code>pub struct SolanaRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc</code> | notes | struct layerx_bridge_relayer::solana::rpc::SolanaRpc |

## relayer-solana-rpc-get-account-info

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-account-info</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:347</code> | <code>pub fn get_account_info(&amp;self, address: &amp;[u8; 32], commitment: Commitment) -&gt; Result&lt;Option&lt;AccountData&gt;, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-account-info</code> | notes | method layerx_bridge_relayer::solana::rpc::SolanaRpc::get_account_info; chain calls: getAccountInfo at :353 |

## relayer-solana-rpc-get-latest-blockhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-latest-blockhash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:388</code> | <code>pub fn get_latest_blockhash(&amp;self, commitment: Commitment) -&gt; Result&lt;Blockhash, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-latest-blockhash</code> | notes | method layerx_bridge_relayer::solana::rpc::SolanaRpc::get_latest_blockhash; chain calls: getLatestBlockhash at :390 |

## relayer-solana-rpc-get-signature-statuses

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-signature-statuses</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:436</code> | <code>pub fn get_signature_statuses(&amp;self, signatures: &amp;[[u8; 64]]) -&gt; Result&lt;Vec&lt;Option&lt;SignatureStatus&gt;&gt;, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-signature-statuses</code> | notes | method layerx_bridge_relayer::solana::rpc::SolanaRpc::get_signature_statuses; chain calls: getSignatureStatuses at :448 |

## relayer-solana-rpc-get-signatures-for-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-signatures-for-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:276</code> | <code>pub fn get_signatures_for_address(&amp;self, address: &amp;[u8; 32], before: Option&lt;&amp;[u8; 64]&gt;, limit: usize, commitment: Commitment) -&gt; Result&lt;Vec&lt;SignatureInfo&gt;, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-signatures-for-address</code> | notes | method layerx_bridge_relayer::solana::rpc::SolanaRpc::get_signatures_for_address; chain calls: getSignaturesForAddress at :293 |

## relayer-solana-rpc-get-slot

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-slot</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:263</code> | <code>pub fn get_slot(&amp;self, commitment: Commitment) -&gt; Result&lt;u64, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-slot</code> | notes | method layerx_bridge_relayer::solana::rpc::SolanaRpc::get_slot; chain calls: getSlot at :264 |

## relayer-solana-rpc-get-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:319</code> | <code>pub fn get_transaction(&amp;self, signature: &amp;[u8; 64], commitment: Commitment) -&gt; Result&lt;Option&lt;SolanaTransaction&gt;, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-get-transaction</code> | notes | method layerx_bridge_relayer::solana::rpc::SolanaRpc::get_transaction; chain calls: getTransaction at :325 |

## relayer-solana-rpc-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:250</code> | <code>pub fn new(transport: Box&lt;dyn JsonRpc&gt;) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-new</code> | notes | method layerx_bridge_relayer::solana::rpc::SolanaRpc::new |

## relayer-solana-rpc-send-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-send-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:409</code> | <code>pub fn send_transaction(&amp;self, raw: &amp;[u8], signature: &amp;[u8; 64]) -&gt; Result&lt;SendOutcome, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-rpc-send-transaction</code> | notes | method layerx_bridge_relayer::solana::rpc::SolanaRpc::send_transaction; chain calls: sendTransaction at :418 |

## relayer-solana-settings

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-settings</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/observe.rs:53</code> | <code>pub struct SolanaSettings</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-settings</code> | notes | struct layerx_bridge_relayer::solana::observe::SolanaSettings |

## relayer-solana-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/rpc.rs:85</code> | <code>pub struct SolanaTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-transaction</code> | notes | struct layerx_bridge_relayer::solana::rpc::SolanaTransaction |

## relayer-solana-transaction-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-solana-transaction-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:24</code> | <code>pub const SOLANA_TRANSACTION_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-solana-transaction-domain</code> | notes | const layerx_bridge_relayer::signer::SOLANA_TRANSACTION_DOMAIN |

## relayer-spki-pin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-spki-pin</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:363</code> | <code>pub fn spki_pin(certificate: &amp;X509) -&gt; Result&lt;[u8; 32], openssl::error::ErrorStack&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-spki-pin</code> | notes | fn layerx_bridge_relayer::cosign::transport::spki_pin |

## relayer-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:484</code> | <code>pub struct State</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-state</code> | notes | struct layerx_bridge_relayer::journal::State |

## relayer-step-report

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-step-report</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs:294</code> | <code>pub struct StepReport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-step-report</code> | notes | struct layerx_bridge_relayer::relayer::StepReport |

## relayer-submission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-submission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:342</code> | <code>pub struct Submission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-submission</code> | notes | struct layerx_bridge_relayer::journal::Submission |

## relayer-submission-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-submission-status</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-bridge-relayer/src/journal.rs:325</code> | <code>pub enum SubmissionStatus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-submission-status</code> | notes | enum layerx_bridge_relayer::journal::SubmissionStatus; spec/layerx-merge/spec.kvx task 8.5.32 do_4 records receipted and committed (services/interop/crates/layerx-bridge-relayer/src/journal.rs:318) |

Contract context <code>contract_context.11.bridge-relayer</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-submitter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:112</code> | <code>pub struct Submitter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-submitter</code> | notes | struct layerx_bridge_relayer::signer::Submitter |

## relayer-submitter-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:150</code> | <code>pub const fn address(&amp;self) -&gt; [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-submitter-address</code> | notes | method layerx_bridge_relayer::signer::Submitter::address; no reference outside the defining file found by name match |

## relayer-submitter-ethereum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-ethereum</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:145</code> | <code>pub fn ethereum(signer: RemoteChainSigner) -&gt; Result&lt;Self, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-submitter-ethereum</code> | notes | method layerx_bridge_relayer::signer::Submitter::ethereum |

## relayer-submitter-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:122</code> | <code>pub fn new(signer: RemoteChainSigner, domain: &amp;'static [u8]) -&gt; Result&lt;Self, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-submitter-new</code> | notes | method layerx_bridge_relayer::signer::Submitter::new; no reference outside the defining file found by name match |

## relayer-submitter-paxeer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-paxeer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:136</code> | <code>pub fn paxeer(signer: RemoteChainSigner) -&gt; Result&lt;Self, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-submitter-paxeer</code> | notes | method layerx_bridge_relayer::signer::Submitter::paxeer |

## relayer-submitter-sign-transaction-hash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-submitter-sign-transaction-hash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/signer.rs:159</code> | <code>pub fn sign_transaction_hash(&amp;self, digest: [u8; 32]) -&gt; Result&lt;[u8; 65], KeyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-submitter-sign-transaction-hash</code> | notes | method layerx_bridge_relayer::signer::Submitter::sign_transaction_hash |

## relayer-system-program

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-system-program</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:78</code> | <code>pub const SYSTEM_PROGRAM: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-system-program</code> | notes | const layerx_bridge_relayer::solana::release::SYSTEM_PROGRAM |

## relayer-threshold-selector

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-threshold-selector</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/abi.rs:40</code> | <code>pub const THRESHOLD_SELECTOR: [u8; 4]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-threshold-selector</code> | notes | const layerx_bridge_relayer::abi::THRESHOLD_SELECTOR |

## relayer-to-attestor-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-to-attestor-signature</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:178</code> | <code>pub fn to_attestor_signature(recoverable: [u8; 65]) -&gt; Result&lt;[u8; 65], SignatureError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-to-attestor-signature</code> | notes | fn layerx_bridge_relayer::attestation::to_attestor_signature |

## relayer-token-account-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-token-account-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:53</code> | <code>pub const TOKEN_ACCOUNT_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-token-account-bytes</code> | notes | const layerx_bridge_relayer::solana::release::TOKEN_ACCOUNT_BYTES; no reference outside the defining file found by name match |

## relayer-token-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-token-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:374</code> | <code>pub fn token_amount(amount: &amp;[u8; 32]) -&gt; Option&lt;u64&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-token-amount</code> | notes | fn layerx_bridge_relayer::solana::release::token_amount |

## relayer-token-program

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-token-program</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:68</code> | <code>pub const TOKEN_PROGRAM: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-token-program</code> | notes | const layerx_bridge_relayer::solana::release::TOKEN_PROGRAM |

## relayer-transport

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-transport</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:329</code> | <code>pub struct Transport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-transport</code> | notes | struct layerx_bridge_relayer::cosign::transport::Transport |

## relayer-transport-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-transport-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:307</code> | <code>pub struct TransportConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-transport-config</code> | notes | struct layerx_bridge_relayer::cosign::transport::TransportConfig; no reference outside the defining file found by name match |

## relayer-transport-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-transport-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:617</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-transport-load</code> | notes | method layerx_bridge_relayer::cosign::Transport::load |

## relayer-transport-record-stem

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-transport-record-stem</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:990</code> | <code>pub fn record_stem(&amp;self, peer: &amp;Peer, digest: &amp;[u8; 32], signature: &amp;[u8; 65]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-transport-record-stem</code> | notes | method layerx_bridge_relayer::cosign::Transport::record_stem; no reference outside the defining file found by name match |

## relayer-transport-serve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-transport-serve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:838</code> | <code>pub fn serve(self: &amp;std::sync::Arc&lt;Self&gt;) -&gt; Result&lt;(), String&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-transport-serve</code> | notes | method layerx_bridge_relayer::cosign::Transport::serve; no reference outside the defining file found by name match |

## relayer-transport-sweep

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-transport-sweep</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:934</code> | <code>pub fn sweep(&amp;self)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-transport-sweep</code> | notes | method layerx_bridge_relayer::cosign::Transport::sweep |

## relayer-transport-validate-operator

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-transport-validate-operator</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:713</code> | <code>pub fn validate_operator(&amp;self, inventory: &amp;OperatorInventory, relayer: &amp;crate::config::RelayerConfig) -&gt; Result&lt;(), ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-transport-validate-operator</code> | notes | method layerx_bridge_relayer::cosign::Transport::validate_operator |

## relayer-uint256-from-u64

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-uint256-from-u64</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation.rs:27</code> | <code>pub fn uint256_from_u64(value: u64) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-uint256-from-u64</code> | notes | fn layerx_bridge_relayer::attestation::uint256_from_u64 |

## relayer-vault-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-vault-authority</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:175</code> | <code>pub fn vault_authority(program_id: &amp;[u8; 32], bump: u8) -&gt; Option&lt;[u8; 32]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-vault-authority</code> | notes | fn layerx_bridge_relayer::solana::release::vault_authority |

## relayer-vault-seed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-vault-seed</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:39</code> | <code>pub const VAULT_SEED: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-vault-seed</code> | notes | const layerx_bridge_relayer::solana::release::VAULT_SEED |

## relayer-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/cosign.rs:283</code> | <code>pub const VERSION: u16</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-version</code> | notes | const layerx_bridge_relayer::cosign::transport::VERSION; no reference outside the defining file found by name match |

## relayer-wire-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-relayer-wire-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-bridge-relayer/src/solana/release.rs:482</code> | <code>pub fn wire_transaction(signature: &amp;[u8; 64], message: &amp;[u8]) -&gt; Vec&lt;u8&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-relayer-wire-transaction</code> | notes | fn layerx_bridge_relayer::solana::release::wire_transaction |
