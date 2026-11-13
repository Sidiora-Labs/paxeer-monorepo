# Interop contract

Interop services use node JSON-RPC and authenticated receipts on Paxeer mainnet-beta, with EVM chain ID `125` (`0x7d`). The kernel participates in the network and proposes blocks; receipts are final once their blocks are committed. The services below adapt external protocols and publish evidence for that network.

## bridge-relayer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-relayer-attestation-allowed-origins-j48</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config attestation_allowed_origins: list of exact origins answered with Access-Control-Allow-Origin</code> |
| <code>interface.11.cfg-relayer-attestation-listen-j48</code> | <code>config_key</code> | [services/interop/crates/layerx-bridge-relayer/src/config.rs](../../../services/interop/crates/layerx-bridge-relayer/src/config.rs) | <code>relayer config attestation_listen: optional socket address; absent, no attestation server runs</code> |
| <code>interface.11.http-relayer-v1-deposit-j48</code> | <code>http_route</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs) | <code>GET /v1/deposit/{chainId}/{txHash}/{logIndex} -&gt; 200 bundle {chainId, vault, txHash, logIndex, recipient, asset, amount, digest, signatures, status} &#124; 404 while unobserved</code> |
| <code>interface.11.http-relayer-v1-release-j48</code> | <code>http_route</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs) | <code>GET /v1/release/{chainId}/{paxeerTxHash}/{nonce} -&gt; 200 bundle {chainId, vault, paxeerTxHash, paxeerNonce, recipient, asset, amount, digest, signatures, status} &#124; 404 while unobserved</code> |
| <code>interface.11.rs-relayer-attestation-view-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs) | <code>pub struct AttestationView { pub items: BTreeMap&lt;String, Item&gt;, pub signatures: BTreeMap&lt;[u8; 32], Vec&lt;[u8; 65]&gt;&gt;, pub thresholds: BTreeMap&lt;u64, usize&gt;, }</code> |
| <code>interface.11.rs-relayer-deposit-answer-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs) | <code>pub fn deposit_answer&lt;S: AttestationSource + ?Sized&gt;( view: &amp;S, chain_id: u64, tx_hash: &amp;[u8; 32], log_index: u64, ) -&gt; Option&lt;Value&gt;</code> |
| <code>interface.11.rs-relayer-paxeer-endpoint-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/transport.rs](../../../services/interop/crates/layerx-bridge-relayer/src/transport.rs) | <code>pub struct PaxeerEndpoint { pub url: String, pub request_timeout: Duration, pub trust_anchor_der: Option&lt;Vec&lt;u8&gt;&gt;, pub expected_chain_id: u64, }</code> |
| <code>interface.11.rs-relayer-paxeer-transport-call-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/transport.rs](../../../services/interop/crates/layerx-bridge-relayer/src/transport.rs) | <code>pub fn call(endpoint: &amp;PaxeerEndpoint, method: &amp;str, params: &amp;Value) -&gt; Result&lt;Value, TransportFault&gt;</code> |
| <code>interface.11.rs-relayer-paxeer-transport-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/transport.rs](../../../services/interop/crates/layerx-bridge-relayer/src/transport.rs) | <code>pub struct PaxeerTransport</code> |
| <code>interface.11.rs-relayer-relayer-attestation-view-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/relayer.rs](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) | <code>pub fn attestation_view(&amp;self) -&gt; AttestationView</code> |
| <code>interface.11.rs-relayer-release-answer-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs) | <code>pub fn release_answer&lt;S: AttestationSource + ?Sized&gt;( view: &amp;S, chain_id: u64, tx_hash: &amp;[u8; 32], nonce: u64, ) -&gt; Option&lt;Value&gt;</code> |
| <code>interface.11.rs-relayer-sequencer-receipt-decode-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/receipt.rs](../../../services/interop/crates/layerx-bridge-relayer/src/receipt.rs) | <code>pub fn decode(value: &amp;Value) -&gt; Result&lt;Self, ReceiptError&gt;</code> |
| <code>interface.11.rs-relayer-sequencer-receipt-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/receipt.rs](../../../services/interop/crates/layerx-bridge-relayer/src/receipt.rs) | <code>pub struct SequencerReceipt { pub rung: Rung, pub tx_hash: [u8; 32], pub nonce: u64, pub view: Option&lt;ReceiptView&gt;, pub block: Option&lt;(u64, [u8; 32])&gt;, }</code> |
| <code>interface.11.rs-relayer-sequencer-receipt-verify-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/receipt.rs](../../../services/interop/crates/layerx-bridge-relayer/src/receipt.rs) | <code>pub fn verify(&amp;self, keys: &amp;[SequencerAuthorization]) -&gt; Result&lt;(), ReceiptError&gt;</code> |
| <code>interface.11.rs-relayer-serve-attestations-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs](../../../services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs) | <code>pub fn serve&lt;S: AttestationSource&gt;( listener: &amp;TcpListener, view: &amp;RwLock&lt;S&gt;, origins: &amp;[String], ) -&gt; io::Result&lt;()&gt;</code> |
| <code>interface.11.rs-relayer-transport-fault-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-bridge-relayer/src/transport.rs](../../../services/interop/crates/layerx-bridge-relayer/src/transport.rs) | <code>pub enum TransportFault { UnsupportedUrl, InsecureTransport, InvalidTrustAnchor, Authentication { detail: String }, Connect { detail: String }, Transport { detail: String }, Http { status: u16 }, ResponseTooLarge, AmbiguousFraming, MalformedResponse, Rpc { code: i64, message: String }, ChainMismatch { expected: u64, actual: u64 }, InconsistentObservation, UnexpectedValue { detail: String } }</code> |

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

## gas-station

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-full-sequencer-receipt-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub struct FullSequencerReceipt</code> |
| <code>interface.11.rs-gas-sequencer-endpoint-full-receipt-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn full_receipt(&amp;self, hash: &amp;Word) -&gt; Result&lt;Option&lt;FullSequencerReceipt&gt;, RpcFault&gt;</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## mirror-archive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-first-height-j48</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config first_height: u64</code> |
| <code>interface.11.cfg-mirror-freshness-budget-blocks-j48</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config freshness_budget_blocks: u64</code> |
| <code>interface.11.cfg-mirror-paxeer-section-j48</code> | <code>config_key</code> | [services/interop/crates/layerx-mirror/src/runtime.rs](../../../services/interop/crates/layerx-mirror/src/runtime.rs) | <code>mirror runtime config paxeer: {rpc, chain_id, genesis_hash_hex, receipt_keys}; rpc contains endpoints and quorum</code> |
| <code>interface.11.err-mirror-committed-refusal-codes-j48</code> | <code>error_code</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>committed_unavailable &#124; committed_chain_mismatch &#124; committed_genesis_mismatch &#124; committed_parent_mismatch &#124; committed_receipt_unverified &#124; committed_malformed</code> |
| <code>interface.11.rs-mirror-archive-from-committed-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub fn from_committed( block: &amp;CommittedBlock, network_id: u32, node_head: NodeHead, ) -&gt; Result&lt;Self, SourceError&gt;</code> |
| <code>interface.11.rs-mirror-archived-committed-block-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/publisher.rs](../../../services/interop/crates/layerx-mirror/src/publisher.rs) | <code>pub struct ArchivedCommittedBlock</code> |
| <code>interface.11.rs-mirror-committed-block-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub struct CommittedBlock { pub header: CommittedHeader, pub receipts: Vec&lt;Vec&lt;u8&gt;&gt;, }</code> |
| <code>interface.11.rs-mirror-committed-block-source-acquire-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub fn acquire(&amp;self, height: u64) -&gt; Result&lt;CommittedBlock, CommittedRefusal&gt;</code> |
| <code>interface.11.rs-mirror-committed-block-source-acquire-next-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub fn acquire_next( &amp;self, previous: CommittedCoordinate, ) -&gt; Result&lt;CommittedBlock, CommittedRefusal&gt;</code> |
| <code>interface.11.rs-mirror-committed-block-source-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub struct CommittedBlockSource</code> |
| <code>interface.11.rs-mirror-committed-block-source-latest-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub fn latest_committed( &amp;self, previous: Option&lt;CommittedCoordinate&gt;, ) -&gt; Result&lt;CommittedCoordinate, CommittedRefusal&gt;</code> |
| <code>interface.11.rs-mirror-committed-coordinate-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub struct CommittedCoordinate { pub height: u64, pub block_hash: [u8; 32], }</code> |
| <code>interface.11.rs-mirror-committed-header-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub struct CommittedHeader</code> |
| <code>interface.11.rs-mirror-committed-refusal-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub enum CommittedRefusal</code> |
| <code>interface.11.rs-mirror-committed-source-config-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/committed.rs](../../../services/interop/crates/layerx-mirror/src/committed.rs) | <code>pub struct CommittedSourceConfig { pub rpc: RpcQuorumConfig, pub chain_id: u64, pub genesis_hash_hex: String, pub receipt_keys: Vec&lt;ReceiptKeyConfig&gt;, }</code> |
| <code>interface.11.rs-mirror-mirror-verifier-committed-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub fn committed(&amp;self) -&gt; Result&lt;CommittedCoordinate, MirrorVerifyError&gt;</code> |
| <code>interface.11.rs-mirror-mirror-verifier-committed-receipt-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror/src/verify.rs](../../../services/interop/crates/layerx-mirror/src/verify.rs) | <code>pub fn committed_receipt( &amp;self, index: usize, keys: &amp;[SequencerAuthorization], ) -&gt; Result&lt;MirrorVerification&lt;Receipt&gt;, MirrorVerifyError&gt;</code> |

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

## portable-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-export-with-transaction-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn export_with_transaction( signed_receipt: &amp;[u8], receipt_key: &amp;ReceiptKeyEntry, inclusion_height: u64, raw_transaction: &amp;[u8], ) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |
| <code>interface.11.rs-portable-portable-receipt-v2-from-sequencer-receipt-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn from_sequencer_receipt(answer: &amp;serde_json::Value, receipt_keys: &amp;[ReceiptKeyEntry], raw_transaction: Option&lt;&amp;[u8]&gt;) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |
| <code>interface.11.rs-portable-portable-verified-receipt-v2-raw-transaction-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn raw_transaction(&amp;self) -&gt; Option&lt;&amp;[u8]&gt;</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## x-websearch-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-xws-payment-payer-address-j48</code> | <code>config_key</code> | [services/interop/crates/x-websearch/src/config.rs](../../../services/interop/crates/x-websearch/src/config.rs) | <code>x-websearch config payment.payer_address: 0x-prefixed 20-byte EVM address</code> |
| <code>interface.11.rs-xws-kernel-network-id-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn kernel_network_id(rpc: &amp;EvmRpc) -&gt; Result&lt;u32, EvmError&gt;</code> |

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

| Field | Rule |
|---|---|
| interface | Paid routes offer exact payments on the 0x101A ledger precompile and verify payExact evidence from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Exact payment transactions are signed and funded by the payer. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## x-websearch-payment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-payment-exact-log-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn decode_payment_exact(log: &amp;SettlementLog) -&gt; Option&lt;PaymentExactLog&gt;</code> |
| <code>interface.11.rs-xws-read-payer-grant-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/payment.rs](../../../services/interop/crates/x-websearch/src/payment.rs) | <code>pub fn read_payer_grant(rpc: &amp;SettlementRpc, grant_id: &amp;[u8; 32]) -&gt; Result&lt;Option&lt;Grant&gt;, SettlementError&gt;</code> |
| <code>interface.11.rs-xws-settlement-error-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/settlement.rs](../../../services/interop/crates/x-websearch/src/settlement.rs) | <code>pub enum SettlementError { Transport, Rpc { code: i64, message: String }, Malformed, ChainMismatch { expected: u64, actual: u64 }, }</code> |
| <code>interface.11.rs-xws-settlement-receipt-decode-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/settlement.rs](../../../services/interop/crates/x-websearch/src/settlement.rs) | <code>pub fn decode(value: &amp;Value) -&gt; Result&lt;SettlementReceipt, SettlementError&gt;</code> |
| <code>interface.11.rs-xws-settlement-receipt-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/settlement.rs](../../../services/interop/crates/x-websearch/src/settlement.rs) | <code>pub struct SettlementReceipt { pub rung: SettlementRung, pub tx_hash: [u8; 32], pub nonce: u64, pub seq: Option&lt;u64&gt;, pub height: Option&lt;u64&gt;, pub status: Option&lt;u64&gt;, pub logs: Vec&lt;SettlementLog&gt;, pub key_id: Option&lt;[u8; 32]&gt;, pub receipt: Option&lt;Vec&lt;u8&gt;&gt;, }</code> |
| <code>interface.11.rs-xws-settlement-rpc-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/settlement.rs](../../../services/interop/crates/x-websearch/src/settlement.rs) | <code>pub struct SettlementRpc</code> |

| Field | Rule |
|---|---|
| interface | Paid routes offer exact payments on the 0x101A ledger precompile and verify payExact evidence from pax_sequencerReceipt. |
| storage | Payment journal under `<data_dir>/payments`. |
| errors | 402 challenges with invalid_grant, payment_failed, receipt_mismatch, receipt_unverified, receipt_consumed and payer_mismatch. |
| events | PaymentDrawn and PaymentExact logs of 0x101A. |
| gas | Exact payment transactions are signed and funded by the payer. |
| activation | When payment is configured. |
| invariants | A response is released at most once per verified receipt. |
| pause | none |

## x-websearch-program-requests

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-xws-program-attestor-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub struct ProgramAttestor</code> |
| <code>interface.11.rs-xws-program-exchange-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub struct ProgramExchange</code> |
| <code>interface.11.rs-xws-program-relay-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/program.rs](../../../services/interop/crates/x-websearch/src/program.rs) | <code>pub struct ProgramRelay</code> |
| <code>interface.11.rs-xws-request-watcher-program-work-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/watch.rs](../../../services/interop/crates/x-websearch/src/watch.rs) | <code>pub fn program_work(&amp;self) -&gt; Vec&lt;ProgramRequest&gt;</code> |
| <code>interface.11.rs-xws-submitter-submit-program-j48</code> | <code>rust_pub_item</code> | [services/interop/crates/x-websearch/src/submit.rs](../../../services/interop/crates/x-websearch/src/submit.rs) | <code>pub fn submit_program(&amp;mut self, key: ProgramKey, observation: &amp;[u8], signatures: usize) -&gt; Result&lt;Submission, SubmitError&gt;</code> |

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
