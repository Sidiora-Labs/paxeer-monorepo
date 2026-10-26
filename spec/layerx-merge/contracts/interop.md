# Interop contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## bridge-relayer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-relayer-attestation-allowed-origins-j48</code> | <code>config_key</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs</code> (planned) | <code>relayer config attestation_allowed_origins: list of exact origins answered with Access-Control-Allow-Origin</code> |
| <code>interface.11.cfg-relayer-attestation-listen-j48</code> | <code>config_key</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/config.rs</code> (planned) | <code>relayer config attestation_listen: optional socket address; absent, no attestation server runs</code> |
| <code>interface.11.http-relayer-v1-deposit-j48</code> | <code>http_route</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs</code> (planned) | <code>GET /v1/deposit/{chainId}/{txHash}/{logIndex} -&gt; 200 bundle {chainId, vault, txHash, logIndex, recipient, asset, amount, digest, signatures, status} &#124; 404 while unobserved</code> |
| <code>interface.11.http-relayer-v1-release-j48</code> | <code>http_route</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs</code> (planned) | <code>GET /v1/release/{chainId}/{paxeerTxHash}/{nonce} -&gt; 200 bundle {chainId, vault, paxeerTxHash, paxeerNonce, recipient, asset, amount, digest, signatures, status} &#124; 404 while unobserved</code> |
| <code>interface.11.rs-relayer-attestation-view-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs</code> (planned) | <code>pub struct AttestationView { items, signatures, thresholds }</code> |
| <code>interface.11.rs-relayer-deposit-answer-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs</code> (planned) | <code>pub fn deposit_answer(view: &amp;AttestationView, chain_id: u64, tx_hash: &amp;[u8; 32], log_index: u64) -&gt; Option&lt;serde_json::Value&gt;</code> |
| <code>interface.11.rs-relayer-paxeer-endpoint-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/transport.rs</code> (planned) | <code>pub struct PaxeerEndpoint { url, request_timeout, trust_anchor, local_emulator }</code> |
| <code>interface.11.rs-relayer-paxeer-transport-call-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/transport.rs</code> (planned) | <code>pub fn call(&amp;self, method: &amp;str, params: &amp;Value) -&gt; Result&lt;Value, TransportFault&gt;</code> |
| <code>interface.11.rs-relayer-paxeer-transport-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/transport.rs</code> (planned) | <code>pub struct PaxeerTransport</code> |
| <code>interface.11.rs-relayer-relayer-attestation-view-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/relayer.rs</code> (planned) | <code>pub fn attestation_view(&amp;self) -&gt; AttestationView</code> |
| <code>interface.11.rs-relayer-release-answer-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs</code> (planned) | <code>pub fn release_answer(view: &amp;AttestationView, chain_id: u64, paxeer_tx_hash: &amp;[u8; 32], nonce: u64) -&gt; Option&lt;serde_json::Value&gt;</code> |
| <code>interface.11.rs-relayer-sequencer-receipt-decode-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/receipt.rs</code> (planned) | <code>pub fn decode(value: &amp;serde_json::Value) -&gt; Result&lt;SequencerReceipt, RpcFault&gt;</code> |
| <code>interface.11.rs-relayer-sequencer-receipt-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/receipt.rs</code> (planned) | <code>pub struct SequencerReceipt { rung, tx_hash, nonce, view, block }</code> |
| <code>interface.11.rs-relayer-sequencer-receipt-verify-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/receipt.rs</code> (planned) | <code>pub fn verify(&amp;self, keys: &amp;[SequencerAuthorization]) -&gt; Result&lt;(), RpcFault&gt;</code> |
| <code>interface.11.rs-relayer-serve-attestations-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs</code> (planned) | <code>pub fn serve(listener: TcpListener, view: Arc&lt;RwLock&lt;AttestationView&gt;&gt;, origins: Vec&lt;String&gt;) -&gt; std::io::Result&lt;()&gt;</code> |
| <code>interface.11.rs-relayer-transport-fault-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-bridge-relayer/src/transport.rs</code> (planned) | <code>pub enum TransportFault { ChainMismatch, InsecureTransport, InvalidTrustAnchor, Timeout, Malformed }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-relayer-attestation-allowed-origins-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.cfg-relayer-attestation-listen-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.http-relayer-v1-deposit-j48</code> | notes | read by apps/bridge/src/lib/bridge/attestation-api.ts:132 |
| <code>interface.11.http-relayer-v1-release-j48</code> | notes | read by apps/bridge/src/lib/bridge/attestation-api.ts:158 |
| <code>interface.11.rs-relayer-attestation-view-j48</code> | notes | planned; no caller at HEAD; snapshot of journal items and collected cosign signatures |
| <code>interface.11.rs-relayer-deposit-answer-j48</code> | notes | planned; no caller at HEAD; the body of GET /v1/deposit/{chainId}/{txHash}/{logIndex} |
| <code>interface.11.rs-relayer-paxeer-endpoint-j48</code> | notes | planned; no caller at HEAD; replaces layerx_paxeer_verifier::EndpointConfig used at services/interop/crates/layerx-bridge-relayer/src/config.rs:15 |
| <code>interface.11.rs-relayer-paxeer-transport-call-j48</code> | notes | planned; no caller at HEAD; sends eth_chainId before the requested method |
| <code>interface.11.rs-relayer-paxeer-transport-j48</code> | notes | planned; no caller at HEAD; replaces layerx_paxeer_verifier::raw_call imported at services/interop/crates/layerx-bridge-relayer/src/rpc.rs:10 |
| <code>interface.11.rs-relayer-relayer-attestation-view-j48</code> | notes | planned; no caller at HEAD; built after each Relayer::tick |
| <code>interface.11.rs-relayer-release-answer-j48</code> | notes | planned; no caller at HEAD; the body of GET /v1/release/{chainId}/{paxeerTxHash}/{nonce} |
| <code>interface.11.rs-relayer-sequencer-receipt-decode-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-relayer-sequencer-receipt-j48</code> | notes | planned; no caller at HEAD; mirrors SequencerReceiptResult at rpc/sequencer_receipt.go:150-156 |
| <code>interface.11.rs-relayer-sequencer-receipt-verify-j48</code> | notes | planned; no caller at HEAD; calls verify_sequencer_signature at services/agent/crates/layerx-proof/src/receipt.rs:666 |
| <code>interface.11.rs-relayer-serve-attestations-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-relayer-transport-fault-j48</code> | notes | planned; no caller at HEAD |

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

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.241.1</code> | req | 11241.1 |
| <code>acceptance.11.241.1</code> | scenario | Given a loopback emulator endpoint that answers eth_chainId with the configured chain id and eth_blockNumber with 0x2a, when PaxeerTransport::call is made for eth_blockNumber, then the emulator receives eth_chainId then eth_blockNumber and the call returns the string 0x2a |
| <code>acceptance.11.241.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/paxeer_transport.rs::a_loopback_emulator_answers_after_the_chain_id_check (planned) |
| <code>acceptance.11.241.1</code> | qualification | UNRUN |
| <code>req.11241</code> | ac_1 | PaxeerTransport::call in services/interop/crates/layerx-bridge-relayer/src/transport.rs (planned) sends eth_chainId first on every request, compares it with the configured chain id and only then sends the requested method, returning the endpoint's result value. |
| <code>acceptance.11.241.2</code> | req | 11241.2 |
| <code>acceptance.11.241.2</code> | scenario | Given a loopback emulator that answers eth_chainId with a chain id one above the configured one, when PaxeerTransport::call is made for eth_getTransactionReceipt, then the call returns ChainMismatch naming expected and actual ids and the emulator saw only the eth_chainId request |
| <code>acceptance.11.241.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/paxeer_transport.rs::a_wrong_chain_id_is_refused_before_the_method_is_sent (planned) |
| <code>acceptance.11.241.2</code> | qualification | UNRUN |
| <code>req.11241</code> | ac_2 | An endpoint whose eth_chainId differs from the configured chain id makes PaxeerTransport::call return TransportFault::ChainMismatch with both ids, and the requested method is never sent, as raw_call does at services/agent/crates/layerx-paxeer-verifier/src/rpc.rs:176-196. |
| <code>acceptance.11.241.3</code> | req | 11241.3 |
| <code>acceptance.11.241.3</code> | scenario | Given an endpoint description with an http URL on a non-loopback host and local_emulator true, when PaxeerEndpoint::new is called, then it returns InsecureTransport and no socket is opened |
| <code>acceptance.11.241.3</code> | test | services/interop/crates/layerx-bridge-relayer/tests/paxeer_transport.rs::a_plaintext_url_off_loopback_is_refused (planned) |
| <code>acceptance.11.241.3</code> | qualification | UNRUN |
| <code>req.11241</code> | ac_3 | PaxeerEndpoint::new refuses an http URL whose host is not a loopback address, and refuses local_emulator for any https URL, returning TransportFault::InsecureTransport before any connection. |
| <code>acceptance.11.241.4</code> | req | 11241.4 |
| <code>acceptance.11.241.4</code> | scenario | Given an https endpoint description with an empty trust anchor and another with three random bytes, when PaxeerEndpoint::new is called for each, then both return InvalidTrustAnchor |
| <code>acceptance.11.241.4</code> | test | services/interop/crates/layerx-bridge-relayer/tests/paxeer_transport.rs::an_https_endpoint_without_a_trust_anchor_is_refused (planned) |
| <code>acceptance.11.241.4</code> | qualification | UNRUN |
| <code>req.11241</code> | ac_4 | PaxeerEndpoint::new refuses an https URL whose trust anchor bytes are empty or not one DER X.509 certificate, returning TransportFault::InvalidTrustAnchor. |
| <code>acceptance.11.241.5</code> | req | 11241.5 |
| <code>acceptance.11.241.5</code> | scenario | Given three loopback emulators where two answer eth_blockNumber with 0x10 and one with 0x11, when PaxeerRpc::call is made, then it returns 0x10; and given three different answers, then it returns Divergence |
| <code>acceptance.11.241.5</code> | test | services/interop/crates/layerx-bridge-relayer/tests/paxeer_rpc.rs::paxeer_reads_need_a_strict_majority_of_endpoints (planned) |
| <code>acceptance.11.241.5</code> | qualification | UNRUN |
| <code>req.11241</code> | ac_5 | PaxeerRpc::call (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:158) over three PaxeerEndpoint values returns a value only when at least two endpoints return it byte for byte, and returns RpcFault::Divergence when all three differ. |
| <code>acceptance.11.241.6</code> | req | 11241.6 |
| <code>acceptance.11.241.6</code> | scenario | Given endpoint lists of zero, nine, and two endpoints with chain ids 125 and 126, when PaxeerRpc::new is called on each, then each returns Configuration |
| <code>acceptance.11.241.6</code> | test | services/interop/crates/layerx-bridge-relayer/tests/paxeer_rpc.rs::paxeer_rpc_refuses_mixed_chain_ids_and_more_than_eight_endpoints (planned) |
| <code>acceptance.11.241.6</code> | qualification | UNRUN |
| <code>req.11241</code> | ac_6 | PaxeerRpc::new (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:113) refuses an empty list, nine endpoints, or endpoints with different expected chain ids with RpcFault::Configuration. |
| <code>acceptance.11.241.7</code> | req | 11241.7 |
| <code>acceptance.11.241.7</code> | scenario | Given loopback emulators that answer pax_sequencerReceipt with null, when sequencer_receipt is called for a 32-byte hash on PaxeerRpc, then the request names pax_sequencerReceipt with that hash and the result is None |
| <code>acceptance.11.241.7</code> | test | services/interop/crates/layerx-bridge-relayer/tests/paxeer_rpc.rs::a_sequencer_receipt_read_uses_pax_sequencer_receipt (planned) |
| <code>acceptance.11.241.7</code> | qualification | UNRUN |
| <code>req.11241</code> | ac_7 | JsonRpc::sequencer_receipt (services/interop/crates/layerx-bridge-relayer/src/rpc.rs:69) sends pax_sequencerReceipt with the 0x-prefixed transaction hash and returns None for a null result. |
| <code>acceptance.11.242.1</code> | req | 11242.1 |
| <code>acceptance.11.242.1</code> | scenario | Given a relayer config with one TLS endpoint naming a DER file and one loopback endpoint with local_emulator true, when paxeer_endpoints is called, then two PaxeerEndpoint values with chain id paxeer.chain_id are returned; and given a third endpoint without trust anchor or local_emulator, then the call returns a configuration error naming the endpoint url |
| <code>acceptance.11.242.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/operator_config.rs::paxeer_endpoints_are_built_without_the_verifier_crate (planned) |
| <code>acceptance.11.242.1</code> | qualification | UNRUN |
| <code>req.11242</code> | ac_1 | RelayerConfig::paxeer_endpoints (services/interop/crates/layerx-bridge-relayer/src/config.rs:427) is public and returns PaxeerEndpoint values carrying the chain id, the request timeout and the trust anchor bytes read from trust_anchor_der, and refuses an endpoint without a trust anchor unless local_emulator is true. |
| <code>acceptance.11.242.2</code> | req | 11242.2 |
| <code>acceptance.11.242.2</code> | scenario | Given config files with two adjacent receipt key windows, with an empty receipt_keys list, and with windows 1-10 and 5-20, when RelayerConfig::load is called, then the first loads two windows and the other two are refused naming paxeer.receipt_keys |
| <code>acceptance.11.242.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/operator_config.rs::receipt_keys_are_loaded_as_height_windows (planned) |
| <code>acceptance.11.242.2</code> | qualification | UNRUN |
| <code>req.11242</code> | ac_2 | RelayerConfig::load (services/interop/crates/layerx-bridge-relayer/src/config.rs:320) reads paxeer.receipt_keys as a list of {sequencer_id, public_key, first_height, last_height} hex entries and validate refuses an empty list, a first_height above last_height, and overlapping windows. |
| <code>acceptance.11.243.1</code> | req | 11243.1 |
| <code>acceptance.11.243.1</code> | scenario | Given a recorded committed pax_sequencerReceipt answer, when SequencerReceipt::decode runs, then rung is Committed, seq and height are non-zero, the logs keep their order and blockNumber and blockHash are present |
| <code>acceptance.11.243.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/sequencer_receipt.rs::a_committed_answer_decodes_every_field (planned) |
| <code>acceptance.11.243.1</code> | qualification | UNRUN |
| <code>req.11243</code> | ac_1 | SequencerReceipt::decode in services/interop/crates/layerx-bridge-relayer/src/receipt.rs (planned) reads rung, txHash, nonce, seq, height, status, logs, keyId, receipt bytes and, for committed, blockNumber and blockHash, the fields of SequencerReceiptResult at rpc/sequencer_receipt.go:150-156 and SequencerReceiptView at rpc/sequencer_receipt.go:125-144. |
| <code>acceptance.11.243.2</code> | req | 11243.2 |
| <code>acceptance.11.243.2</code> | scenario | Given a recorded submitted answer and the same answer with a receipt field added, when SequencerReceipt::decode runs on each, then the first yields Submitted with no receipt and the second returns Malformed |
| <code>acceptance.11.243.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/sequencer_receipt.rs::a_submitted_answer_carries_no_receipt (planned) |
| <code>acceptance.11.243.2</code> | qualification | UNRUN |
| <code>req.11243</code> | ac_2 | A submitted answer decodes to rung Submitted with txHash and nonce only, and a submitted answer that carries a receipt field is refused as malformed. |
| <code>acceptance.11.243.3</code> | req | 11243.3 |
| <code>acceptance.11.243.3</code> | scenario | Given a recorded receipted answer and the key window that covers its height, when verify runs, then it succeeds; and given a key window that ends one height earlier or the JSON status flipped, then verify returns a refusal |
| <code>acceptance.11.243.3</code> | test | services/interop/crates/layerx-bridge-relayer/tests/sequencer_receipt.rs::receipt_bytes_must_verify_under_the_key_window_for_their_height (planned) |
| <code>acceptance.11.243.3</code> | qualification | UNRUN |
| <code>req.11243</code> | ac_3 | SequencerReceipt::verify passes the receipt bytes to layerx_proof::receipt::verify_sequencer_signature (services/agent/crates/layerx-proof/src/receipt.rs:666) with ReceiptKeys::HeightRanges and refuses an answer whose JSON seq, height or status differ from the verified receipt. |
| <code>acceptance.11.244.1</code> | req | 11244.1 |
| <code>acceptance.11.244.1</code> | scenario | Given the relayer test fixture config with two loopback endpoints, when the test builds PaxeerRpc through RelayerConfig::paxeer_endpoints, then PaxeerRpc::new succeeds and a grep of the crate sources for layerx_paxeer_verifier finds nothing |
| <code>acceptance.11.244.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/operator_cosign_contract.rs::supplied_paxeer_endpoints_build_a_relayer_rpc (planned) |
| <code>acceptance.11.244.1</code> | qualification | UNRUN |
| <code>req.11244</code> | ac_1 | The operator contract test builds PaxeerRpc from RelayerConfig::paxeer_endpoints instead of constructing EndpointConfig at services/interop/crates/layerx-bridge-relayer/tests/operator_cosign_contract.rs:963-971, and the crate no longer names layerx_paxeer_verifier anywhere. |
| <code>acceptance.11.245.1</code> | req | 11245.1 |
| <code>acceptance.11.245.1</code> | scenario | Given a recorded Paxeer endpoint that answers pax_sequencerReceipt first receipted then committed for the bridgeIn hash, when inbound_step runs twice, then the journal holds Receipted with the receipt seq and height, then Completed Included with the committed blockNumber, and SubmissionStatus passes Pending, Receipted, Committed |
| <code>acceptance.11.245.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/loops.rs::an_inbound_bridge_in_is_receipted_then_committed_from_the_sequencer_receipt (planned) |
| <code>acceptance.11.245.1</code> | qualification | UNRUN |
| <code>req.11245</code> | ac_1 | Relayer::inbound_step (services/interop/crates/layerx-bridge-relayer/src/relayer.rs:623) resolving a Paxeer submission reads pax_sequencerReceipt, appends Entry::Receipted with seq and height (services/interop/crates/layerx-bridge-relayer/src/journal.rs:263-269) at the receipted rung after verifying the receipt, and completes the item as Included at the committed rung with the committed block number. |
| <code>acceptance.11.245.2</code> | req | 11245.2 |
| <code>acceptance.11.245.2</code> | scenario | Given a recorded pax_sequencerReceipt answer with status 0 and an unconsumed nullifier, when inbound_step resolves the submission, then the journal records Reverted and the item is free for a new transaction |
| <code>acceptance.11.245.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/loops.rs::a_receipt_with_status_zero_reverts_the_paxeer_submission (planned) |
| <code>acceptance.11.245.2</code> | qualification | UNRUN |
| <code>req.11245</code> | ac_2 | A verified receipted answer with status 0 for a Paxeer submission appends Entry::Reverted unless the nullifier read at services/interop/crates/layerx-bridge-relayer/src/relayer.rs:1048 reports it consumed, in which case the item completes AlreadyBridged. |
| <code>acceptance.11.245.3</code> | req | 11245.3 |
| <code>acceptance.11.245.3</code> | scenario | Given a recorded receipted answer signed outside every configured key window, when inbound_step resolves the submission, then no journal entry is appended and the step reports Malformed |
| <code>acceptance.11.245.3</code> | test | services/interop/crates/layerx-bridge-relayer/tests/loops.rs::an_unverifiable_sequencer_receipt_leaves_the_submission_pending (planned) |
| <code>acceptance.11.245.3</code> | qualification | UNRUN |
| <code>req.11245</code> | ac_3 | A receipted answer whose receipt bytes fail verification under the configured receipt keys appends nothing and returns RelayerError::Rpc(RpcFault::Malformed) without completing or dropping the item. |
| <code>acceptance.11.246.1</code> | req | 11246.1 |
| <code>acceptance.11.246.1</code> | scenario | Given the shared sequencer-receipt-v1 vector file, when each case is decoded and verified with its key windows, then every accepted case verifies with the vector's seq and height and every refused case is refused |
| <code>acceptance.11.246.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/sequencer_receipt.rs::the_shared_sequencer_receipt_vector_decodes_and_verifies (planned) |
| <code>acceptance.11.246.1</code> | qualification | UNRUN |
| <code>req.11246</code> | ac_1 | SequencerReceipt::decode and verify accept every receipted and committed case of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:1 with the key windows the vector names, and refuse every case the vector marks as refused. |
| <code>acceptance.11.421.1</code> | req | 11421.1 |
| <code>acceptance.11.421.1</code> | scenario | Given a journal holding one observed deposit and this relayer's signature over its digest, when deposit_answer runs for its chain id, transaction hash and log index, then every field equals the observation, digest equals InboundAttestation::digest of it and signatures holds the one signature |
| <code>acceptance.11.421.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_api.rs::an_observed_deposit_answers_the_bundle_fields_the_bridge_app_reads (planned) |
| <code>acceptance.11.421.1</code> | qualification | UNRUN |
| <code>req.11421</code> | ac_1 | deposit_answer in services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs (planned) answers an Observation::Inbound item (services/interop/crates/layerx-bridge-relayer/src/journal.rs:83-97) keyed by inbound_key (:52) with chainId, logIndex and amount as decimal strings, vault, txHash, recipient, asset and digest as lowercase 0x hex, signatures as 65-byte 0x hex and status, the fields read at apps/bridge/src/lib/bridge/attestation-api.ts:142-153. |
| <code>acceptance.11.421.2</code> | req | 11421.2 |
| <code>acceptance.11.421.2</code> | scenario | Given the same journal, when deposit_answer runs for another log index, then it returns None |
| <code>acceptance.11.421.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_api.rs::an_unobserved_deposit_answers_nothing (planned) |
| <code>acceptance.11.421.2</code> | qualification | UNRUN |
| <code>req.11421</code> | ac_2 | deposit_answer returns None for a key the journal does not hold, so the route answers 404, which the bridge app reads as null at apps/bridge/src/lib/bridge/attestation-api.ts:58. |
| <code>acceptance.11.421.3</code> | req | 11421.3 |
| <code>acceptance.11.421.3</code> | scenario | Given a deposit with threshold two, when deposit_answer runs with no signature, one signature and two signatures collected, then the statuses are observed, attesting and ready |
| <code>acceptance.11.421.3</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_api.rs::deposit_status_follows_the_collected_signatures (planned) |
| <code>acceptance.11.421.3</code> | qualification | UNRUN |
| <code>req.11421</code> | ac_3 | status is observed before this relayer signs, attesting while fewer than the destination threshold of distinct attestor signatures are collected through CosignDirectory::collect (services/interop/crates/layerx-bridge-relayer/src/cosign.rs:177), and ready at or above the threshold, three of the four values the bridge app accepts at apps/bridge/src/lib/bridge/attestation-api.ts:112-116. |
| <code>acceptance.11.422.1</code> | req | 11422.1 |
| <code>acceptance.11.422.1</code> | scenario | Given a journal holding one observed BridgeOut burn and its signature, when release_answer runs for its chain id, Paxeer transaction hash and nonce, then every field equals the observation and digest equals OutboundAttestation::digest of it |
| <code>acceptance.11.422.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_api.rs::an_observed_release_answers_the_bundle_fields_the_bridge_app_reads (planned) |
| <code>acceptance.11.422.1</code> | qualification | UNRUN |
| <code>req.11422</code> | ac_1 | release_answer answers an Observation::Outbound item (services/interop/crates/layerx-bridge-relayer/src/journal.rs:98-111) keyed by outbound_key (:58) with chainId, paxeerNonce and amount as decimal strings, vault, paxeerTxHash, recipient, asset and digest as lowercase 0x hex, signatures and status, the fields read at apps/bridge/src/lib/bridge/attestation-api.ts:168-178. |
| <code>acceptance.11.422.2</code> | req | 11422.2 |
| <code>acceptance.11.422.2</code> | scenario | Given a journal holding one deposit and one release, when release_answer runs for an unknown nonce and for the deposit's coordinates, then both return None |
| <code>acceptance.11.422.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_api.rs::an_unobserved_release_answers_nothing (planned) |
| <code>acceptance.11.422.2</code> | qualification | UNRUN |
| <code>req.11422</code> | ac_2 | release_answer returns None for a key the journal does not hold and for a key naming an inbound item. |
| <code>acceptance.11.423.1</code> | req | 11423.1 |
| <code>acceptance.11.423.1</code> | scenario | Given the loop harness with one deposit observed and signed, when Relayer::tick runs and attestation_view is taken, then deposit_answer on the view returns the deposit with status matching the harness threshold |
| <code>acceptance.11.423.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/loops.rs::the_attestation_view_holds_every_item_after_a_tick (planned) |
| <code>acceptance.11.423.1</code> | qualification | UNRUN |
| <code>req.11423</code> | ac_1 | Relayer::attestation_view (services/interop/crates/layerx-bridge-relayer/src/relayer.rs:407) returns the journal items of Journal::state (services/interop/crates/layerx-bridge-relayer/src/journal.rs:822), the signatures collected per digest and the destination threshold per chain read at services/interop/crates/layerx-bridge-relayer/src/relayer.rs:1068-1089. |
| <code>acceptance.11.423.2</code> | req | 11423.2 |
| <code>acceptance.11.423.2</code> | scenario | Given a view taken before a tick that observes a second deposit, when the tick completes, then the old view answers None for the second deposit and a new view answers it |
| <code>acceptance.11.423.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/loops.rs::the_attestation_view_is_a_snapshot (planned) |
| <code>acceptance.11.423.2</code> | qualification | UNRUN |
| <code>req.11423</code> | ac_2 | A view taken before a tick is not changed by later ticks, so the server never reads the journal while the loop writes it. |
| <code>acceptance.11.424.1</code> | req | 11424.1 |
| <code>acceptance.11.424.1</code> | scenario | Given a server bound to a loopback port over a view with one deposit and one release, when both routes and an unknown deposit are requested, then the answers are 200, 200 and 404 and the bodies equal deposit_answer and release_answer |
| <code>acceptance.11.424.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_server.rs::the_attestation_routes_answer_over_a_socket (planned) |
| <code>acceptance.11.424.1</code> | qualification | UNRUN |
| <code>req.11424</code> | ac_1 | serve in services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs (planned) answers GET /v1/deposit/{chainId}/{txHash}/{logIndex} and GET /v1/release/{chainId}/{paxeerTxHash}/{nonce} with 200 application/json bodies from the shared view and 404 for an unobserved item, the routes requested at apps/bridge/src/lib/bridge/attestation-api.ts:137-139 and :163-165. |
| <code>acceptance.11.424.2</code> | req | 11424.2 |
| <code>acceptance.11.424.2</code> | scenario | Given the same server, when /v1/deposit/x/0x12/1, /v1/release/1/&lt;hash&gt;/-1, /v2/deposit and POST /v1/deposit/1/&lt;hash&gt;/0 are requested, then the answers are 400, 400, 404 and 405 |
| <code>acceptance.11.424.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_server.rs::malformed_attestation_paths_answer_bad_request (planned) |
| <code>acceptance.11.424.2</code> | qualification | UNRUN |
| <code>req.11424</code> | ac_2 | A chain id or index that is not a decimal u64, or a hash that is not 32 bytes of lowercase 0x hex, answers 400; any other path or method answers 404 or 405. |
| <code>acceptance.11.424.3</code> | req | 11424.3 |
| <code>acceptance.11.424.3</code> | scenario | Given the operator config with attestation_listen set to a loopback address with port 0, without it and with not-an-address, when RelayerConfig::load runs on each, then the first yields the address, the second None and the third a Configuration error naming attestation_listen |
| <code>acceptance.11.424.3</code> | test | services/interop/crates/layerx-bridge-relayer/tests/operator_config.rs::attestation_listen_is_an_optional_socket_address (planned) |
| <code>acceptance.11.424.3</code> | qualification | UNRUN |
| <code>req.11424</code> | ac_3 | RelayerConfig::load (services/interop/crates/layerx-bridge-relayer/src/config.rs:320) reads attestation_listen as an optional socket address beside the fields at services/interop/crates/layerx-bridge-relayer/src/config.rs:289-308 and refuses a value that does not parse, naming the field. |
| <code>acceptance.11.425.1</code> | req | 11425.1 |
| <code>acceptance.11.425.1</code> | scenario | Given a server with one allowed origin, when a deposit route is requested with that Origin and with another origin, then only the first answer carries Access-Control-Allow-Origin |
| <code>acceptance.11.425.1</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_server.rs::a_configured_origin_receives_the_allow_origin_header (planned) |
| <code>acceptance.11.425.1</code> | qualification | UNRUN |
| <code>req.11425</code> | ac_1 | A GET with an Origin header listed in attestation_allowed_origins is answered with Access-Control-Allow-Origin equal to that origin and Vary: Origin, since the bridge app fetches from its configured attestationApiUrl at apps/bridge/src/lib/bridge/attestation-api.ts:46-54. |
| <code>acceptance.11.425.2</code> | req | 11425.2 |
| <code>acceptance.11.425.2</code> | scenario | Given the same server, when OPTIONS /v1/deposit/1/&lt;hash&gt;/0 is sent with the allowed origin and with another origin, then the answers are 204 with the two headers and 403 |
| <code>acceptance.11.425.2</code> | test | services/interop/crates/layerx-bridge-relayer/tests/attestation_server.rs::a_preflight_from_a_configured_origin_is_answered (planned) |
| <code>acceptance.11.425.2</code> | qualification | UNRUN |
| <code>req.11425</code> | ac_2 | OPTIONS on an attestation route from an allowed origin answers 204 with Access-Control-Allow-Methods GET and Access-Control-Allow-Headers accept; from another origin it answers 403. |
| <code>capability.11.21.1</code> | title | Add the relayer's own pinned Paxeer JSON-RPC transport |
| <code>capability.11.21.1</code> | reqs | 11241.1<br>11241.2<br>11241.3<br>11241.4 |
| <code>capability.11.21.2</code> | title | Run PaxeerRpc majority reads over the relayer transport |
| <code>capability.11.21.2</code> | reqs | 11241.5<br>11241.6<br>11241.7 |
| <code>capability.11.21.3</code> | title | Build relayer Paxeer endpoints and receipt keys from the config file |
| <code>capability.11.21.3</code> | reqs | 11242.1<br>11242.2 |
| <code>capability.11.22.1</code> | title | Decode and verify pax_sequencerReceipt answers in the relayer |
| <code>capability.11.22.1</code> | reqs | 11243.1<br>11243.2<br>11243.3 |
| <code>capability.11.22.2</code> | title | Drop layerx-paxeer-verifier from the bridge relayer |
| <code>capability.11.22.2</code> | reqs | 11244.1 |
| <code>capability.11.22.3</code> | title | Resolve Paxeer submissions from pax_sequencerReceipt rungs |
| <code>capability.11.22.3</code> | reqs | 11245.1<br>11245.2<br>11245.3 |
| <code>capability.11.22.4</code> | title | Check the relayer receipt decoder against the shared receipt vector |
| <code>capability.11.22.4</code> | reqs | 11246.1 |
| <code>capability.11.38.1</code> | title | Build the deposit attestation bundle from journal items and cosign signatures |
| <code>capability.11.38.1</code> | reqs | 11421.1<br>11421.2<br>11421.3 |
| <code>capability.11.38.2</code> | title | Build the release attestation bundle for Paxeer BridgeOut burns |
| <code>capability.11.38.2</code> | reqs | 11422.1<br>11422.2 |
| <code>capability.11.38.3</code> | title | Publish an attestation view from the relay loop after each tick |
| <code>capability.11.38.3</code> | reqs | 11423.1<br>11423.2 |
| <code>capability.11.38.4</code> | title | Serve the deposit and release attestation routes over HTTP from the relayer binary |
| <code>capability.11.38.4</code> | reqs | 11424.1<br>11424.2<br>11424.3 |
| <code>capability.11.38.5</code> | title | Answer cross-origin attestation reads for the configured bridge app origins |
| <code>capability.11.38.5</code> | reqs | 11425.1<br>11425.2 |
| <code>resolution.17.887</code> | title | decision.1122 binds: layerx-migrate mapping v2 already reads at one finalized height |
| <code>resolution.17.887</code> | applies_to | decision.1122<br>task.11.21.1<br>task.11.21.2<br>task.11.21.3<br>task.11.22.1<br>task.11.22.3<br>task.11.38.1<br>task.11.38.2<br>task.11.38.3<br>task.11.38.4<br>task.11.38.5 |
| <code>resolution.17.887</code> | source | decision.1122 in fragments/J48/decision.kvx; tasks from tasks naming the affected blocks |
| <code>resolution.17.893</code> | title | decision.1190 binds: wiring.J48.3 is fulfilled by task 16.3.1, not 11.30.1 |
| <code>resolution.17.893</code> | applies_to | decision.1190<br>task.11.22.2<br>task.11.30.1<br>task.16.3.1 |
| <code>resolution.17.893</code> | source | decision.1190 in fragments/J79/decision.kvx; tasks from affects |

## gas-station

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-full-sequencer-receipt-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs</code> (planned) | <code>pub struct FullSequencerReceipt</code> |
| <code>interface.11.rs-gas-sequencer-endpoint-full-receipt-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs</code> (planned) | <code>pub fn full_receipt(&amp;self, hash: &amp;Word) -&gt; Result&lt;Option&lt;FullSequencerReceipt&gt;, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-full-sequencer-receipt-j48</code> | notes | planned; no caller at HEAD; beside SequencerReceipt at services/interop/crates/layerx-gas-station/src/rpc.rs:255 |
| <code>interface.11.rs-gas-sequencer-endpoint-full-receipt-j48</code> | notes | planned; no caller at HEAD; beside SequencerEndpoint::receipt at services/interop/crates/layerx-gas-station/src/rpc.rs:277 |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.261.1</code> | req | 11261.1 |
| <code>acceptance.11.261.1</code> | scenario | Given a harness chain whose DID binding for the account is unavailable, when a quote is requested, then the station returns Quote(Policy(DidUnavailable)), no signature is made, the journal is empty and no transaction is sent |
| <code>acceptance.11.261.1</code> | test | services/interop/crates/layerx-gas-station/tests/did_budget.rs::did_lookup_failure_refuses (existing) |
| <code>acceptance.11.261.1</code> | qualification | UNRUN |
| <code>req.11261</code> | ac_1 | A quote whose 0x1004 getLayerXDid read fails is refused with PolicyRefusal::DidUnavailable (services/interop/crates/layerx-gas-station/src/policy.rs:6-13 gains the variant) instead of BalanceFloor, signs nothing, journals nothing and sends nothing, as the existing test at services/interop/crates/layerx-gas-station/tests/did_budget.rs:485-500 asserts for the other effects. |
| <code>acceptance.11.261.2</code> | req | 11261.2 |
| <code>acceptance.11.261.2</code> | scenario | Given a running service whose chain cannot answer getLayerXDid for the account, when POST /quote is sent over a socket, then the response is 503 with body {"error":"unavailable"} |
| <code>acceptance.11.261.2</code> | test | services/interop/crates/layerx-gas-station/tests/service.rs::a_failed_did_read_answers_503_unavailable (planned) |
| <code>acceptance.11.261.2</code> | qualification | UNRUN |
| <code>req.11261</code> | ac_2 | POST /quote for an account whose DID read fails answers HTTP 503 with {"error":"unavailable"}, the ServiceError::Unavailable status of services/interop/crates/layerx-gas-station/src/service.rs:123, and not the balance_floor code. |
| <code>acceptance.11.262.1</code> | req | 11262.1 |
| <code>acceptance.11.262.1</code> | scenario | Given GAS_STATION_DID_GAS_LIMIT equal to one quote's gas and a policy interval_seconds of 3600, when a DID is quoted, quoted again one hour later and again one day later, then the second quote is refused PerAccount and the third is signed |
| <code>acceptance.11.262.1</code> | test | services/interop/crates/layerx-gas-station/tests/did_budget.rs::did_gas_budget_resets_at_the_day_boundary_not_the_policy_interval (planned) |
| <code>acceptance.11.262.1</code> | qualification | UNRUN |
| <code>req.11262</code> | ac_1 | DidBudget (services/interop/crates/layerx-gas-station/src/policy.rs:21-26) charges gas per DID over interval now / 86400, so with a token policy interval of 3600 seconds a DID that spent its gas limit stays refused across hourly boundaries and is admitted again after the next UTC day starts. |
| <code>acceptance.11.262.2</code> | req | 11262.2 |
| <code>acceptance.11.262.2</code> | scenario | Given a station that charged a DID's whole gas limit, when the station is reopened two policy intervals later on the same UTC day and the DID requests a quote, then the quote is refused PerAccount |
| <code>acceptance.11.262.2</code> | test | services/interop/crates/layerx-gas-station/tests/did_budget.rs::journal_replay_restores_did_gas_within_the_same_day (planned) |
| <code>acceptance.11.262.2</code> | qualification | UNRUN |
| <code>req.11262</code> | ac_2 | Replaying the journal at station open (services/interop/crates/layerx-gas-station/src/policy.rs:164-172 restore_did_usage) restores DID gas charged earlier the same UTC day even when the token policy interval has rolled over. |
| <code>acceptance.11.263.1</code> | req | 11263.1 |
| <code>acceptance.11.263.1</code> | scenario | Given a recorded committed pax_sequencerReceipt answer, when decode_full_sequencer_receipt runs, then every field equals the recorded value and the receipt bytes equal the hex-decoded receipt field |
| <code>acceptance.11.263.1</code> | test | services/interop/crates/layerx-gas-station/tests/sequencer_receipt.rs::a_committed_answer_decodes_logs_receipt_bytes_and_block (planned) |
| <code>acceptance.11.263.1</code> | qualification | UNRUN |
| <code>req.11263</code> | ac_1 | decode_full_sequencer_receipt in services/interop/crates/layerx-gas-station/src/rpc.rs:300 returns FullSequencerReceipt with rung, txHash, nonce, seq, height, status, gasUsed, sponsor, from, to, keyId, logs in order, the canonical receipt bytes and, at committed, blockNumber and blockHash, the fields of rpc/sequencer_receipt.go:125-156. |
| <code>acceptance.11.263.2</code> | req | 11263.2 |
| <code>acceptance.11.263.2</code> | scenario | Given a recorded receipted answer with a blockNumber added and a committed answer with blockHash removed, when decode_full_sequencer_receipt runs on each, then both return Malformed |
| <code>acceptance.11.263.2</code> | test | services/interop/crates/layerx-gas-station/tests/sequencer_receipt.rs::an_answer_with_a_block_below_committed_is_refused (planned) |
| <code>acceptance.11.263.2</code> | qualification | UNRUN |
| <code>req.11263</code> | ac_2 | An answer at rung submitted or receipted that carries blockNumber or blockHash, or a committed answer without them, is refused with RpcFault::Malformed. |
| <code>acceptance.11.263.3</code> | req | 11263.3 |
| <code>acceptance.11.263.3</code> | scenario | Given a configured endpoint over a recorded exchange answering null, when full_receipt is called for a hash, then the request names pax_sequencerReceipt with that hash and the result is None |
| <code>acceptance.11.263.3</code> | test | services/interop/crates/layerx-gas-station/tests/sequencer_receipt.rs::full_receipt_reads_pax_sequencer_receipt_by_hash (planned) |
| <code>acceptance.11.263.3</code> | qualification | UNRUN |
| <code>req.11263</code> | ac_3 | SequencerEndpoint::full_receipt calls pax_sequencerReceipt with the 0x-prefixed hash through the endpoint's JsonRpc and returns None for a null answer, leaving SequencerEndpoint::receipt at services/interop/crates/layerx-gas-station/src/rpc.rs:277-279 unchanged. |
| <code>acceptance.11.264.1</code> | req | 11264.1 |
| <code>acceptance.11.264.1</code> | scenario | Given a running service whose recorded sequencer answers committed for the submitted hash, when POST /submit is sent, then the 200 body carries rung committed with seq, height, logs, keyId, receipt, blockNumber and blockHash equal to the recorded answer |
| <code>acceptance.11.264.1</code> | test | services/interop/crates/layerx-gas-station/tests/service.rs::a_committed_submission_answers_the_full_sequencer_receipt (planned) |
| <code>acceptance.11.264.1</code> | qualification | UNRUN |
| <code>req.11264</code> | ac_1 | POST /submit for a committed sponsored transaction answers transactionHash, rung, seq, height, status, logs, keyId, receipt and blockNumber and blockHash, extending the three fields submission_rung writes at services/interop/crates/layerx-gas-station/src/service.rs:571-575. |
| <code>acceptance.11.264.2</code> | req | 11264.2 |
| <code>acceptance.11.264.2</code> | scenario | Given a recorded sequencer answering receipted for the submitted hash, when POST /submit is sent, then the body has rung receipted, a receipt field and no blockNumber or blockHash |
| <code>acceptance.11.264.2</code> | test | services/interop/crates/layerx-gas-station/tests/service.rs::a_receipted_submission_answers_without_block_fields (planned) |
| <code>acceptance.11.264.2</code> | qualification | UNRUN |
| <code>req.11264</code> | ac_2 | POST /submit for a receipted transaction answers the receipt fields without blockNumber and blockHash. |
| <code>acceptance.11.265.1</code> | req | 11265.1 |
| <code>acceptance.11.265.1</code> | scenario | Given a DID with a gas limit of 300000 that was sponsored 100000 gas today, when POST /status is sent for one of its quotes, then budget.gasLimit is 300000, usedGas 100000, remainingGas 200000 and dayEnds is the next multiple of 86400 |
| <code>acceptance.11.265.1</code> | test | services/interop/crates/layerx-gas-station/tests/service.rs::status_reports_the_did_gas_budget (planned) |
| <code>acceptance.11.265.1</code> | qualification | UNRUN |
| <code>req.11265</code> | ac_1 | POST /status answers a budget object {key, gasLimit, usedGas, remainingGas, dayEnds} for the budget key of the quote's account, read from DidBudget::used (services/interop/crates/layerx-gas-station/src/policy.rs:38) at the current UTC day, beside the fields report writes at services/interop/crates/layerx-gas-station/src/service.rs:602-664. |
| <code>acceptance.11.266.1</code> | req | 11266.1 |
| <code>acceptance.11.266.1</code> | scenario | Given the shared sequencer-receipt-v1 vector file, when each answer case is decoded, then every decoded field equals the vector's expectation |
| <code>acceptance.11.266.1</code> | test | services/interop/crates/layerx-gas-station/tests/sequencer_receipt.rs::the_shared_sequencer_receipt_vector_decodes_in_the_gas_station (planned) |
| <code>acceptance.11.266.1</code> | qualification | UNRUN |
| <code>req.11266</code> | ac_1 | decode_full_sequencer_receipt accepts every answer case of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:1 and returns the vector's seq, height, status and keyId. |
| <code>capability.11.23.1</code> | title | Refuse a failed DID read as DidUnavailable and answer it 503 |
| <code>capability.11.23.1</code> | reqs | 11261.1<br>11261.2 |
| <code>capability.11.23.2</code> | title | Charge GAS_STATION_DID_GAS_LIMIT per UTC day, apart from the token interval |
| <code>capability.11.23.2</code> | reqs | 11262.1<br>11262.2 |
| <code>capability.11.23.3</code> | title | Report the per-DID gas budget on POST /status |
| <code>capability.11.23.3</code> | reqs | 11265.1 |
| <code>capability.11.24.1</code> | title | Decode the full pax_sequencerReceipt answer in the gas station |
| <code>capability.11.24.1</code> | reqs | 11263.1<br>11263.2<br>11263.3 |
| <code>capability.11.24.2</code> | title | Answer POST /submit with the full sequencer receipt |
| <code>capability.11.24.2</code> | reqs | 11264.1<br>11264.2 |
| <code>capability.11.24.3</code> | title | Check the gas station decoder against the shared receipt vector |
| <code>capability.11.24.3</code> | reqs | 11266.1 |

## intent-ingester-retired

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.341.1</code> | req | 11341.1 |
| <code>acceptance.11.341.1</code> | scenario | Given the interop workspace, when cargo metadata --no-deps runs, then no package is named layerx-intent-ingester |
| <code>acceptance.11.341.1</code> | test | services/interop/crates/layerx-portable/tests/retired_crates.rs::the_interop_workspace_has_no_intent_ingester_package (planned) |
| <code>acceptance.11.341.1</code> | qualification | UNRUN |
| <code>req.11341</code> | ac_1 | cargo metadata for services/interop lists no package named layerx-intent-ingester once services/interop/crates/layerx-intent-ingester/Cargo.toml:1 is deleted and the workspace member at services/interop/Cargo.toml:8 is removed. |
| <code>acceptance.11.341.2</code> | req | 11341.2 |
| <code>acceptance.11.341.2</code> | scenario | Given the interop workspace, when every package's dependencies are listed with cargo metadata, then none is named layerx-intent-ingester |
| <code>acceptance.11.341.2</code> | test | services/interop/crates/layerx-portable/tests/retired_crates.rs::no_interop_crate_depends_on_the_intent_ingester (planned) |
| <code>acceptance.11.341.2</code> | qualification | UNRUN |
| <code>req.11341</code> | ac_2 | No package in the interop workspace names layerx-intent-ingester as a dependency, so deleting the crate breaks no build. |
| <code>capability.11.30.1</code> | title | Delete the layerx-intent-ingester crate |
| <code>capability.11.30.1</code> | reqs | 11341.1<br>11341.2 |
| <code>resolution.17.892</code> | title | decision.1127 binds: The ingester removal check lives in layerx-portable |
| <code>resolution.17.892</code> | applies_to | decision.1127<br>task.11.30.1 |
| <code>resolution.17.892</code> | source | decision.1127 in fragments/J48/decision.kvx; tasks from affects |

## mirror-archive

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-first-height-j48</code> | <code>config_key</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs</code> (planned) | <code>mirror runtime config first_height: u64</code> |
| <code>interface.11.cfg-mirror-freshness-budget-blocks-j48</code> | <code>config_key</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs</code> (planned) | <code>mirror runtime config freshness_budget_blocks: u64</code> |
| <code>interface.11.cfg-mirror-paxeer-section-j48</code> | <code>config_key</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/runtime.rs</code> (planned) | <code>mirror runtime config paxeer: {endpoints, quorum, chain_id, genesis_hash, receipt_keys}</code> |
| <code>interface.11.err-mirror-committed-refusal-codes-j48</code> | <code>error_code</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>committed_unavailable &#124; committed_chain_mismatch &#124; committed_genesis_mismatch &#124; committed_parent_mismatch &#124; committed_receipt_unverified &#124; committed_malformed</code> |
| <code>interface.11.rs-mirror-archive-from-committed-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs</code> (planned) | <code>pub fn from_committed(block: &amp;CommittedBlock) -&gt; Result&lt;Archive, SourceError&gt;</code> |
| <code>interface.11.rs-mirror-archived-committed-block-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/publisher.rs</code> (planned) | <code>pub struct ArchivedCommittedBlock</code> |
| <code>interface.11.rs-mirror-committed-block-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub struct CommittedBlock { header, transactions, receipts }</code> |
| <code>interface.11.rs-mirror-committed-block-source-acquire-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub fn acquire(&amp;self, height: u64) -&gt; Result&lt;CommittedBlock, CommittedRefusal&gt;</code> |
| <code>interface.11.rs-mirror-committed-block-source-acquire-next-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub fn acquire_next(&amp;mut self) -&gt; Result&lt;Option&lt;CommittedBlock&gt;, CommittedRefusal&gt;</code> |
| <code>interface.11.rs-mirror-committed-block-source-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub struct CommittedBlockSource</code> |
| <code>interface.11.rs-mirror-committed-block-source-latest-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub fn latest_committed(&amp;self) -&gt; Result&lt;u64, CommittedRefusal&gt;</code> |
| <code>interface.11.rs-mirror-committed-coordinate-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub struct CommittedCoordinate { chain_id, genesis_hash, height, block_hash }</code> |
| <code>interface.11.rs-mirror-committed-header-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub struct CommittedHeader</code> |
| <code>interface.11.rs-mirror-committed-refusal-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub enum CommittedRefusal</code> |
| <code>interface.11.rs-mirror-committed-source-config-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/committed.rs</code> (planned) | <code>pub struct CommittedSourceConfig { endpoints, quorum, chain_id, genesis_hash, receipt_keys }</code> |
| <code>interface.11.rs-mirror-mirror-verifier-committed-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs</code> (planned) | <code>pub fn committed(&amp;self, height: u64) -&gt; Result&lt;CommittedCoordinate, MirrorVerifyError&gt;</code> |
| <code>interface.11.rs-mirror-mirror-verifier-committed-receipt-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-mirror/src/verify.rs</code> (planned) | <code>pub fn committed_receipt(&amp;self, tx_hash: &amp;[u8; 32]) -&gt; Result&lt;MirrorEvidence, MirrorVerifyError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-first-height-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.cfg-mirror-freshness-budget-blocks-j48</code> | notes | planned; no caller at HEAD; replaces checkpoint_freshness_budget_batches at services/interop/crates/layerx-mirror/src/runtime.rs:35 |
| <code>interface.11.cfg-mirror-paxeer-section-j48</code> | notes | planned; no caller at HEAD; replaces checkpoint_policy at services/interop/crates/layerx-mirror/src/runtime.rs:58 |
| <code>interface.11.err-mirror-committed-refusal-codes-j48</code> | notes | planned; no caller at HEAD; CommittedRefusal::code strings replacing the checkpoint_* codes at services/interop/crates/layerx-mirror/src/node.rs:118-130 |
| <code>interface.11.rs-mirror-archive-from-committed-j48</code> | notes | planned; no caller at HEAD; replaces Archive::from_node |
| <code>interface.11.rs-mirror-archived-committed-block-j48</code> | notes | planned; no caller at HEAD; replaces ArchivedCheckpoint at services/interop/crates/layerx-mirror/src/publisher.rs:168 |
| <code>interface.11.rs-mirror-committed-block-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-mirror-committed-block-source-acquire-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-mirror-committed-block-source-acquire-next-j48</code> | notes | planned; no caller at HEAD; enforces parent hash continuity |
| <code>interface.11.rs-mirror-committed-block-source-j48</code> | notes | planned; no caller at HEAD; replaces LniArchiveSource at services/interop/crates/layerx-mirror/src/node.rs:201 |
| <code>interface.11.rs-mirror-committed-block-source-latest-j48</code> | notes | planned; no caller at HEAD; reads eth_getBlockByNumber at the finalized tag |
| <code>interface.11.rs-mirror-committed-coordinate-j48</code> | notes | planned; no caller at HEAD; replaces CheckpointCoordinate at services/interop/crates/layerx-mirror/src/publisher.rs:44 |
| <code>interface.11.rs-mirror-committed-header-j48</code> | notes | planned; no caller at HEAD; built by CommittedHeader::from_block_json |
| <code>interface.11.rs-mirror-committed-refusal-j48</code> | notes | planned; no caller at HEAD; replaces CheckpointRefusal at services/interop/crates/layerx-mirror/src/node.rs:98 |
| <code>interface.11.rs-mirror-committed-source-config-j48</code> | notes | planned; no caller at HEAD; replaces NodeSourceConfig at services/interop/crates/layerx-mirror/src/node.rs:36 |
| <code>interface.11.rs-mirror-mirror-verifier-committed-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-mirror-mirror-verifier-committed-receipt-j48</code> | notes | planned; no caller at HEAD |

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

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.281.1</code> | req | 11281.1 |
| <code>acceptance.11.281.1</code> | scenario | Given every CommittedRefusal variant, when code is called on each, then the nine strings are returned in variant order and no two are equal |
| <code>acceptance.11.281.1</code> | test | services/interop/crates/layerx-mirror/tests/committed_source.rs::committed_refusal_codes_are_the_published_strings (planned) |
| <code>acceptance.11.281.1</code> | qualification | UNRUN |
| <code>req.11281</code> | ac_1 | CommittedRefusal::code in services/interop/crates/layerx-mirror/src/committed.rs (planned) returns committed_unavailable, committed_chain_mismatch, committed_genesis_mismatch, committed_divergence, committed_not_committed, committed_receipt_signature, committed_parent_mismatch, committed_height_regressed and committed_malformed, one per variant, replacing the checkpoint_* strings of services/interop/crates/layerx-mirror/src/node.rs:114-131. |
| <code>acceptance.11.281.2</code> | req | 11281.2 |
| <code>acceptance.11.281.2</code> | scenario | Given a recorded eth_getBlockByNumber answer with two transaction hashes and the same answer with stateRoot removed, when from_block_json runs on each, then the first yields the recorded height, hash, parent hash, app hash and two hashes in order and the second returns Malformed |
| <code>acceptance.11.281.2</code> | test | services/interop/crates/layerx-mirror/tests/committed_source.rs::a_recorded_block_answer_decodes_to_a_committed_header (planned) |
| <code>acceptance.11.281.2</code> | qualification | UNRUN |
| <code>req.11281</code> | ac_2 | CommittedHeader::from_block_json reads number, hash, parentHash, stateRoot as the app hash written at rpc/block.go:638, timestamp and the transactions hash list of an eth_getBlockByNumber answer with fullTx false, and refuses a missing field, a full transaction object or a non-32-byte hash with CommittedRefusal::Malformed. |
| <code>acceptance.11.282.1</code> | req | 11282.1 |
| <code>acceptance.11.282.1</code> | scenario | Given three loopback TLS emulators answering the recorded block at height 7 with two transactions and committed sequencer receipts signed by the configured key window, when acquire(7) is called, then the block hash, app hash and both receipt byte strings equal the recorded values |
| <code>acceptance.11.282.1</code> | test | services/interop/crates/layerx-mirror/tests/committed_source.rs::a_committed_block_is_acquired_with_every_receipt_verified (planned) |
| <code>acceptance.11.282.1</code> | qualification | UNRUN |
| <code>req.11282</code> | ac_1 | CommittedBlockSource::acquire(height) reads eth_getBlockByNumber with the hex height and fullTx false through RpcCluster::call (services/interop/crates/layerx-mirror/src/rpc.rs:130), reads pax_sequencerReceipt (rpc/sequencer_receipt.go:232) for every transaction hash, and returns CommittedBlock with the header and each receipt's bytes in transaction order after verify_sequencer_signature (services/agent/crates/layerx-proof/src/receipt.rs:666) accepts them under ReceiptKeys::HeightRanges. |
| <code>acceptance.11.282.2</code> | req | 11282.2 |
| <code>acceptance.11.282.2</code> | scenario | Given emulators where the second transaction of block 7 answers rung receipted, when acquire(7) is called, then it returns NotCommitted and no CommittedBlock is produced |
| <code>acceptance.11.282.2</code> | test | services/interop/crates/layerx-mirror/tests/committed_source.rs::a_transaction_not_yet_committed_refuses_the_block (planned) |
| <code>acceptance.11.282.2</code> | qualification | UNRUN |
| <code>req.11282</code> | ac_2 | acquire refuses the whole block with CommittedRefusal::NotCommitted when any pax_sequencerReceipt answer is not at rung committed (rpc/sequencer_receipt.go:37) or its blockNumber or blockHash differ from the block read, and archives nothing. |
| <code>acceptance.11.282.3</code> | req | 11282.3 |
| <code>acceptance.11.282.3</code> | scenario | Given emulators whose receipt for block 7 is signed by a key whose window ends at height 6, and separately three emulators answering three different block hashes, when acquire(7) is called, then the first returns ReceiptSignature and the second Divergence |
| <code>acceptance.11.282.3</code> | test | services/interop/crates/layerx-mirror/tests/committed_source.rs::a_receipt_outside_every_key_window_refuses_the_block (planned) |
| <code>acceptance.11.282.3</code> | qualification | UNRUN |
| <code>req.11282</code> | ac_3 | acquire returns CommittedRefusal::ReceiptSignature when a committed receipt does not verify under the configured key windows, and CommittedRefusal::Divergence when the quorum's answers differ, mapping RpcError::Divergence from services/interop/crates/layerx-mirror/src/rpc.rs:49-56. |
| <code>acceptance.11.283.1</code> | req | 11283.1 |
| <code>acceptance.11.283.1</code> | scenario | Given emulators answering finalized with block 9, when latest_committed is called with no previous coordinate and again with previous height 10, then the first returns height 9 with the recorded hash and the second returns HeightRegressed |
| <code>acceptance.11.283.1</code> | test | services/interop/crates/layerx-mirror/tests/committed_source.rs::the_finalized_tag_gives_the_latest_committed_coordinate (planned) |
| <code>acceptance.11.283.1</code> | qualification | UNRUN |
| <code>req.11283</code> | ac_1 | CommittedBlockSource::latest_committed reads eth_getBlockByNumber with the finalized tag, which rpc/block.go:355-357 resolves to the safe latest height, returns its CommittedCoordinate, and returns HeightRegressed when the answer is below the coordinate passed as the previous observation. |
| <code>acceptance.11.283.2</code> | req | 11283.2 |
| <code>acceptance.11.283.2</code> | scenario | Given emulators whose block 8 names a parent hash different from the recorded hash of block 7, when acquire_next is called with block 7's coordinate, then it returns ParentMismatch |
| <code>acceptance.11.283.2</code> | test | services/interop/crates/layerx-mirror/tests/committed_source.rs::a_parent_hash_break_is_refused_as_parent_mismatch (planned) |
| <code>acceptance.11.283.2</code> | qualification | UNRUN |
| <code>req.11283</code> | ac_2 | CommittedBlockSource::acquire_next(previous) acquires previous.height + 1 and refuses it with ParentMismatch when its parentHash is not previous.block_hash. |
| <code>acceptance.11.283.3</code> | req | 11283.3 |
| <code>acceptance.11.283.3</code> | scenario | Given emulators answering block 0 with a hash one byte different from the configured genesis hash, when open is called, then it returns GenesisMismatch and no source is returned |
| <code>acceptance.11.283.3</code> | test | services/interop/crates/layerx-mirror/tests/committed_source.rs::a_wrong_genesis_hash_is_refused_when_the_source_opens (planned) |
| <code>acceptance.11.283.3</code> | qualification | UNRUN |
| <code>req.11283</code> | ac_3 | CommittedBlockSource::open reads block 0 and refuses a hash different from genesis_hash_hex with GenesisMismatch, the check NativeCheckpointPolicy carried at services/interop/crates/layerx-mirror/src/node.rs:508. |
| <code>acceptance.11.284.1</code> | req | 11284.1 |
| <code>acceptance.11.284.1</code> | scenario | Given rendered configurations with a valid paxeer section, with chain_id 0 and with receipt_keys empty, when the configuration is parsed and validated, then the first is accepted and the other two return RuntimeError::Configuration |
| <code>acceptance.11.284.1</code> | test | services/interop/crates/layerx-mirror/src/runtime.rs::a_paxeer_section_builds_the_committed_source_configuration (planned) |
| <code>acceptance.11.284.1</code> | qualification | UNRUN |
| <code>req.11284</code> | ac_1 | RuntimeConfig (services/interop/crates/layerx-mirror/src/runtime.rs:29-43) reads a paxeer section deserialised into CommittedSourceConfig in place of node (services/interop/crates/layerx-mirror/src/runtime.rs:37), renames first_batch_number to first_height and checkpoint_freshness_budget_batches to freshness_budget_blocks, and validate_runtime refuses chain_id 0, a malformed genesis hash and an empty receipt_keys list. |
| <code>acceptance.11.284.2</code> | req | 11284.2 |
| <code>acceptance.11.284.2</code> | scenario | Given a scratch state directory and a committed source over recorded blocks 5 to 7, when the committed loop runs three steps and recover_next_batch reads the spool, then three archives are spooled in height order and the next height is 8 |
| <code>acceptance.11.284.2</code> | test | services/interop/crates/layerx-mirror/src/runtime.rs::the_runtime_spools_committed_archives_in_height_order (planned) |
| <code>acceptance.11.284.2</code> | qualification | UNRUN |
| <code>req.11284</code> | ac_2 | spawn_committed replaces spawn_node (services/interop/crates/layerx-mirror/src/runtime.rs:216-312), acquires first_height then each next height with CommittedBlockSource::acquire_next, builds Archive::from_committed and puts each archive in the ArchiveSpool so recover_next_batch (services/interop/crates/layerx-mirror/src/runtime.rs:442) resumes at the next height. |
| <code>acceptance.11.285.1</code> | req | 11285.1 |
| <code>acceptance.11.285.1</code> | scenario | Given a RuntimeStatus, when record_committed is applied with a ParentMismatch refusal and then with a coordinate at height 9, then the status first reports committed_parent_mismatch and not ready, then ready at height 9 with the block hash hex |
| <code>acceptance.11.285.1</code> | test | services/interop/crates/layerx-mirror/src/runtime.rs::runtime_status_reports_the_committed_source_refusal_code (planned) |
| <code>acceptance.11.285.1</code> | qualification | UNRUN |
| <code>req.11285</code> | ac_1 | record_committed replaces record_checkpoint (services/interop/crates/layerx-mirror/src/runtime.rs:706-731): a CommittedRefusal latches committed readiness off and reports its committed_* code, and a verified committed coordinate restores readiness with its height and block hash, so the status no longer names checkpoint_refusal. |
| <code>acceptance.11.286.1</code> | req | 11286.1 |
| <code>acceptance.11.286.1</code> | scenario | Given a CommittedBlock at height 7 with two receipts, when Archive::from_committed builds an archive and ArchiveData::decode reads its bytes, then committed holds height 7, the three hashes, the timestamp and both receipts in order |
| <code>acceptance.11.286.1</code> | test | services/interop/crates/layerx-mirror/src/publisher.rs::a_v4_archive_round_trips_a_committed_block (planned) |
| <code>acceptance.11.286.1</code> | qualification | UNRUN |
| <code>req.11286</code> | ac_1 | Archive::from_committed encodes LXMIRROR (services/interop/crates/layerx-mirror/src/publisher.rs:16) with version 4, the network id, the block height as batch_number, block hash, parent hash, app hash, timestamp and length-prefixed receipts, and ArchiveData::decode (services/interop/crates/layerx-mirror/src/publisher.rs:197) returns the same values in ArchiveData.committed. |
| <code>acceptance.11.286.2</code> | req | 11286.2 |
| <code>acceptance.11.286.2</code> | scenario | Given a valid v4 archive truncated by one byte and the same archive with one byte appended, when ArchiveData::decode runs on each, then the first returns Length and the second TrailingBytes |
| <code>acceptance.11.286.2</code> | test | services/interop/crates/layerx-mirror/src/publisher.rs::a_v4_archive_with_a_short_receipt_or_trailing_bytes_is_refused (planned) |
| <code>acceptance.11.286.2</code> | qualification | UNRUN |
| <code>req.11286</code> | ac_2 | ArchiveData::decode refuses a v4 archive whose last receipt length runs past the end with ArchiveError::Length and one with bytes after the last receipt with ArchiveError::TrailingBytes (services/interop/crates/layerx-mirror/src/publisher.rs:781-795). |
| <code>acceptance.11.286.3</code> | req | 11286.3 |
| <code>acceptance.11.286.3</code> | scenario | Given a CommittedBlock with a zero block hash and another with one empty receipt, when Archive::from_committed is called on each, then both return SourceError::Committed |
| <code>acceptance.11.286.3</code> | test | services/interop/crates/layerx-mirror/src/publisher.rs::from_committed_refuses_a_zero_block_hash_and_an_empty_receipt (planned) |
| <code>acceptance.11.286.3</code> | qualification | UNRUN |
| <code>req.11286</code> | ac_3 | Archive::from_committed refuses a CommittedBlock whose block hash is zero or whose receipt list holds an empty byte string with SourceError::Committed, a variant added to SourceError at services/interop/crates/layerx-mirror/src/publisher.rs:765. |
| <code>acceptance.11.287.1</code> | req | 11287.1 |
| <code>acceptance.11.287.1</code> | scenario | Given a node head committed at height 12 and cursors committed at 12 with the same hash, at 9, absent, and at 12 with a different hash, when publication reports freshness for each, then it reports Current, Behind 3, NotYetMirrored 12 and DifferentAtHeight 12 |
| <code>acceptance.11.287.1</code> | test | services/interop/crates/layerx-mirror/tests/committed_freshness.rs::freshness_reports_committed_lag_in_blocks (planned) |
| <code>acceptance.11.287.1</code> | qualification | UNRUN |
| <code>req.11287</code> | ac_1 | MirrorFreshness::new (services/interop/crates/layerx-mirror/src/publisher.rs:1175-1205) compares NodeHead.latest_committed with MirrorCursor.latest_committed and reports CommittedFreshness::Current, NotYetMirrored {target_height}, Behind {block_lag} or DifferentAtHeight {height}, the variants of CheckpointFreshness at services/interop/crates/layerx-mirror/src/publisher.rs:1156-1162 restated in heights. |
| <code>acceptance.11.287.2</code> | req | 11287.2 |
| <code>acceptance.11.287.2</code> | scenario | Given a v4 archive read through MirrorSource over the recorded Ethereum mirror answers, when the observation is returned, then committed equals the archive's height and block hash |
| <code>acceptance.11.287.2</code> | test | services/interop/crates/layerx-mirror/tests/committed_freshness.rs::a_source_observation_carries_the_committed_coordinate (planned) |
| <code>acceptance.11.287.2</code> | qualification | UNRUN |
| <code>req.11287</code> | ac_2 | MirrorObservation.committed and MirrorSourceFreshness.latest_committed (services/interop/crates/layerx-mirror/src/source.rs:84-105) carry the CommittedCoordinate decoded from a v4 archive at services/interop/crates/layerx-mirror/src/source.rs:305-316. |
| <code>acceptance.11.288.1</code> | req | 11288.1 |
| <code>acceptance.11.288.1</code> | scenario | Given a scratch state directory, when a journal records a confirmed publication with committed height 7 and is reopened, then PublicationJournal::cursor returns latest_committed height 7 with the same block hash |
| <code>acceptance.11.288.1</code> | test | services/interop/crates/layerx-mirror/tests/state_directory.rs::a_v3_journal_round_trips_a_committed_cursor (planned) |
| <code>acceptance.11.288.1</code> | qualification | UNRUN |
| <code>req.11288</code> | ac_1 | PublicationJournal::open (services/interop/crates/layerx-mirror/src/store.rs:229) writes records framed with LXMJNL03, LXMHED03 and LXMSPL03 and RECORD_VERSION 3 (services/interop/crates/layerx-mirror/src/store.rs:13-16), and encode_committed writes height and block hash where encode_checkpoint (services/interop/crates/layerx-mirror/src/store.rs:767-786) wrote batch number and checkpoint id. |
| <code>acceptance.11.288.2</code> | req | 11288.2 |
| <code>acceptance.11.288.2</code> | scenario | Given a state directory holding one LXMJNL02 journal record, when PublicationJournal::open is called, then it returns Corrupt and the file bytes are unchanged |
| <code>acceptance.11.288.2</code> | test | services/interop/crates/layerx-mirror/tests/state_directory.rs::a_v2_state_directory_is_refused_at_open (planned) |
| <code>acceptance.11.288.2</code> | qualification | UNRUN |
| <code>req.11288</code> | ac_2 | PublicationJournal::open and ArchiveSpool::open (services/interop/crates/layerx-mirror/src/store.rs:390) refuse a directory holding LXMJNL02 or LXMSPL02 records with StoreError::Corrupt and never rewrite them. |
| <code>acceptance.11.289.1</code> | req | 11289.1 |
| <code>acceptance.11.289.1</code> | scenario | Given recorded version 2 and version 3 archive bytes and a v4 archive, when ArchiveData::decode runs on each, then the first two return Version and the third decodes |
| <code>acceptance.11.289.1</code> | test | services/interop/crates/layerx-mirror/src/publisher.rs::batch_era_archive_versions_are_refused (planned) |
| <code>acceptance.11.289.1</code> | qualification | UNRUN |
| <code>req.11289</code> | ac_1 | ArchiveData::decode refuses archives of version 2 and 3 with ArchiveError::Version (services/interop/crates/layerx-mirror/src/publisher.rs:781-795) and ArchiveData keeps only the v4 committed fields, with NodeBatch, BatchAuthorization, NodeCheckpoint, ArchivedChunk, ArchivedRecords, ArchivedCheckpoint and Archive::from_node deleted from services/interop/crates/layerx-mirror/src/publisher.rs:60-510. |
| <code>acceptance.11.289.2</code> | req | 11289.2 |
| <code>acceptance.11.289.2</code> | scenario | Given the recording Ethereum and Solana clients of the publish suite and a v4 archive at height 7, when the publisher publishes until both confirm, then both cursors report latest_committed height 7 |
| <code>acceptance.11.289.2</code> | test | services/interop/crates/layerx-mirror/tests/committed_freshness.rs::the_publish_suite_archives_committed_blocks (planned) |
| <code>acceptance.11.289.2</code> | qualification | UNRUN |
| <code>req.11289</code> | ac_2 | The build_archive helper of services/interop/crates/layerx-mirror/tests/publish.rs:119-181 builds archives with Archive::from_committed, and a confirmed publication of one such archive on both test networks reports latest_committed equal to the archive coordinate. |
| <code>acceptance.11.301.1</code> | req | 11301.1 |
| <code>acceptance.11.301.1</code> | scenario | Given a v4 archive built from a recorded committed block and the key window that signed it, when from_committed_source then committed_receipt(0) run, then the receipt verifies and the verification reports level Committed and the archive height |
| <code>acceptance.11.301.1</code> | test | services/interop/crates/layerx-mirror/src/verify.rs::a_v4_archive_receipt_verifies_under_the_configured_keys (planned) |
| <code>acceptance.11.301.1</code> | qualification | UNRUN |
| <code>req.11301</code> | ac_1 | MirrorVerifier::from_committed_source admits a v4 archive observation and MirrorVerifier::committed_receipt(index, keys) returns the receipt decoded by verify_sequencer_signature (services/agent/crates/layerx-proof/src/receipt.rs:666) with MirrorEvidenceLevel::Committed, without the signed-header checks of admit at services/interop/crates/layerx-mirror/src/verify.rs:195-243. |
| <code>acceptance.11.301.2</code> | req | 11301.2 |
| <code>acceptance.11.301.2</code> | scenario | Given the same archive and a key window ending one height below the receipt, when committed_receipt(0) runs, then it returns Receipt with the failing check |
| <code>acceptance.11.301.2</code> | test | services/interop/crates/layerx-mirror/src/verify.rs::a_v4_receipt_signed_outside_the_key_windows_is_refused (planned) |
| <code>acceptance.11.301.2</code> | qualification | UNRUN |
| <code>req.11301</code> | ac_2 | committed_receipt returns MirrorVerifyError::Receipt with the ReceiptCheck of the failed signature check when the configured windows do not cover the receipt height. |
| <code>acceptance.11.301.3</code> | req | 11301.3 |
| <code>acceptance.11.301.3</code> | scenario | Given a v4 archive and a version 2 archive built by Archive::from_node, when from_committed_source runs on each, then the first yields committed() equal to the recorded height and hash and the second returns CommittedUnavailable |
| <code>acceptance.11.301.3</code> | test | services/interop/crates/layerx-mirror/src/verify.rs::the_committed_coordinate_is_read_from_a_v4_archive (planned) |
| <code>acceptance.11.301.3</code> | qualification | UNRUN |
| <code>req.11301</code> | ac_3 | MirrorVerifier::committed returns the archive's CommittedCoordinate {height, block_hash}, and from_committed_source refuses a batch-era archive with MirrorVerifyError::CommittedUnavailable. |
| <code>acceptance.11.302.1</code> | req | 11302.1 |
| <code>acceptance.11.302.1</code> | scenario | Given every ReconcileRefusal variant, when code is called, then the six committed_mirror strings are returned and none starts with checkpoint_ |
| <code>acceptance.11.302.1</code> | test | services/interop/crates/layerx-mirror/src/publisher.rs::reconcile_refusal_codes_name_the_committed_mirror (planned) |
| <code>acceptance.11.302.1</code> | qualification | UNRUN |
| <code>req.11302</code> | ac_1 | ReconcileRefusal::code (services/interop/crates/layerx-mirror/src/publisher.rs:1951-1962) returns committed_mirror_archive_unavailable, committed_mirror_chain_unavailable, committed_mirror_missing, committed_mirror_commitment_mismatch, committed_mirror_malformed and committed_mirror_mismatch, and the variant CheckpointMismatch becomes CommittedMismatch. |
| <code>acceptance.11.369.1</code> | req | 11369.1 |
| <code>acceptance.11.369.1</code> | scenario | Given the shared vector file, when each receipted and committed case is verified as a mirror receipt, then each verifies and reports the case's height |
| <code>acceptance.11.369.1</code> | test | services/interop/crates/layerx-mirror/tests/sequencer_vectors.rs::every_shared_receipted_case_verifies_in_the_mirror (planned) |
| <code>acceptance.11.369.1</code> | qualification | UNRUN |
| <code>req.11369</code> | ac_1 | MirrorVerifier::committed_receipt verifies the receipt of every receipted and committed answer case of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:1 under the keys the file pins, with verify_sequencer_signature (services/agent/crates/layerx-proof/src/receipt.rs:666). |
| <code>acceptance.11.369.2</code> | req | 11369.2 |
| <code>acceptance.11.369.2</code> | scenario | Given the V3 cases of the shared vector file, when each is verified as a mirror receipt, then each returns Receipt with the case's expected check |
| <code>acceptance.11.369.2</code> | test | services/interop/crates/layerx-mirror/tests/sequencer_vectors.rs::every_shared_refusal_case_is_refused_by_the_mirror (planned) |
| <code>acceptance.11.369.2</code> | qualification | UNRUN |
| <code>req.11369</code> | ac_2 | MirrorVerifier::committed_receipt refuses every V3 case of the shared vector file with MirrorVerifyError::Receipt carrying the failed check. |
| <code>capability.11.25.1</code> | title | Add the mirror committed module with coordinates, headers and refusal codes |
| <code>capability.11.25.1</code> | reqs | 11281.1<br>11281.2 |
| <code>capability.11.25.2</code> | title | Acquire committed blocks and their sequencer receipts over the RPC quorum |
| <code>capability.11.25.2</code> | reqs | 11282.1<br>11282.2<br>11282.3 |
| <code>capability.11.25.3</code> | title | Follow the finalized tag and enforce genesis and parent continuity |
| <code>capability.11.25.3</code> | reqs | 11283.1<br>11283.2<br>11283.3 |
| <code>capability.11.25.4</code> | title | Run the mirror runtime on the committed block source and its config section |
| <code>capability.11.25.4</code> | reqs | 11284.1<br>11284.2 |
| <code>capability.11.25.5</code> | title | Delete the mirror LNI source, native checkpoint policy and certificate helpers |
| <code>capability.11.25.5</code> | reqs | 11285.1 |
| <code>capability.11.26.1</code> | title | Encode and decode mirror archive v4 for one committed block |
| <code>capability.11.26.1</code> | reqs | 11286.1<br>11286.2<br>11286.3 |
| <code>capability.11.26.2</code> | title | Replace CheckpointCoordinate with CommittedCoordinate in heads, cursors and freshness |
| <code>capability.11.26.2</code> | reqs | 11287.1<br>11287.2 |
| <code>capability.11.26.3</code> | title | Move the mirror journal, head and spool records to version 3 |
| <code>capability.11.26.3</code> | reqs | 11288.1<br>11288.2 |
| <code>capability.11.26.4</code> | title | Delete batch-era archive types and the checkpoint vector test from the mirror |
| <code>capability.11.26.4</code> | reqs | 11289.1<br>11289.2 |
| <code>capability.11.37.1</code> | title | Verify v4 archive receipts and the committed coordinate in MirrorVerifier |
| <code>capability.11.37.1</code> | reqs | 11301.1<br>11301.2<br>11301.3 |
| <code>capability.11.37.2</code> | title | Rename mirror reconcile refusals to committed_mirror codes |
| <code>capability.11.37.2</code> | reqs | 11302.1 |
| <code>capability.11.37.3</code> | title | Check mirror receipt verification against the shared sequencer-receipt vectors |
| <code>capability.11.37.3</code> | reqs | 11369.1<br>11369.2 |

## mirror-contracts

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.291.1</code> | req | 11291.1 |
| <code>acceptance.11.291.1</code> | scenario | Given a v4 archive at height 7 with a known block hash, when stages builds the manifest call, then the calldata starts with the begin selector and its third and fourth words are 7 and the block hash |
| <code>acceptance.11.291.1</code> | test | services/interop/crates/layerx-mirror/src/ethereum.rs::begin_carries_the_committed_height_and_block_hash (planned) |
| <code>acceptance.11.291.1</code> | qualification | UNRUN |
| <code>req.11291</code> | ac_1 | stages (services/interop/crates/layerx-mirror/src/ethereum.rs:856) encodes begin(bytes32,uint32,uint64,bytes32,uint64,uint32,bytes32,bytes32) with the archive commitment, network id, committed height and committed block hash, matching the parameter order of LayerXMirrorArchive.begin at services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:81-90. |
| <code>acceptance.11.291.2</code> | req | 11291.2 |
| <code>acceptance.11.291.2</code> | scenario | Given an archive decoded without a committed block, when stages is called, then it returns Transaction and no stage payload |
| <code>acceptance.11.291.2</code> | test | services/interop/crates/layerx-mirror/src/ethereum.rs::a_v4_archive_without_a_committed_block_has_no_manifest (planned) |
| <code>acceptance.11.291.2</code> | qualification | UNRUN |
| <code>req.11291</code> | ac_2 | stages refuses an archive whose committed block is absent with EthereumError::Transaction instead of writing the zero checkpoint id that services/interop/crates/layerx-mirror/src/ethereum.rs:860-864 writes today. |
| <code>acceptance.11.292.1</code> | req | 11292.1 |
| <code>acceptance.11.292.1</code> | scenario | Given a v4 archive at height 7, when stages builds the begin instruction, then bytes 4-5 are version 4, the height follows the network id and the next 32 bytes equal the block hash |
| <code>acceptance.11.292.1</code> | test | services/interop/crates/layerx-mirror/src/solana.rs::the_begin_instruction_is_version_4_with_the_committed_block_hash (planned) |
| <code>acceptance.11.292.1</code> | qualification | UNRUN |
| <code>req.11292</code> | ac_1 | stages (services/interop/crates/layerx-mirror/src/solana.rs:944) writes INSTRUCTION_VERSION 4 (services/interop/crates/layerx-mirror/src/solana.rs:25) and the committed block hash in the 32-byte slot where services/interop/crates/layerx-mirror/src/solana.rs:949-961 writes the checkpoint id, and MANIFEST_MAGIC (services/interop/crates/layerx-mirror/src/solana.rs:22) reads LXMMAN04. |
| <code>acceptance.11.292.2</code> | req | 11292.2 |
| <code>acceptance.11.292.2</code> | scenario | Given a recorded LXMMAN03 manifest account answer, when the mirror reads it for a v4 commitment, then the read returns the manifest refusal |
| <code>acceptance.11.292.2</code> | test | services/interop/crates/layerx-mirror/src/solana.rs::a_version_3_manifest_account_is_not_read_as_a_v4_archive (planned) |
| <code>acceptance.11.292.2</code> | qualification | UNRUN |
| <code>req.11292</code> | ac_2 | The Solana read path refuses a manifest account whose magic is LXMMAN03 with SolanaError::Manifest, so a version 3 account is never confirmed as a v4 publication. |
| <code>acceptance.11.311.1</code> | req | 11311.1 |
| <code>acceptance.11.311.1</code> | scenario | Given a publisher-owned archive, when begin is called with height 7 and a non-zero block hash, then ManifestOpened is emitted with height 7 and that block hash and a second identical begin returns without a second event |
| <code>acceptance.11.311.1</code> | test | services/interop/contracts/ethereum-mirror/test/LayerXMirrorArchive.t.sol::testBeginRecordsCommittedHeightAndBlockHash (planned) |
| <code>acceptance.11.311.1</code> | qualification | UNRUN |
| <code>req.11311</code> | ac_1 | begin (services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:81) keeps its selector and argument types, names its third and fourth arguments height and blockHash, stores them in ArchiveManifest (services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:29-42) and emits ManifestOpened with blockHash after height. |
| <code>acceptance.11.311.2</code> | req | 11311.2 |
| <code>acceptance.11.311.2</code> | scenario | Given a publisher-owned archive, when begin is called with a zero block hash, then it reverts InvalidManifest and no manifest is stored |
| <code>acceptance.11.311.2</code> | test | services/interop/contracts/ethereum-mirror/test/LayerXMirrorArchive.t.sol::testBeginRejectsZeroBlockHash (planned) |
| <code>acceptance.11.311.2</code> | qualification | UNRUN |
| <code>req.11311</code> | ac_2 | begin reverts InvalidManifest (services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:68) when blockHash is zero, in addition to the zero and bound checks at services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:90-95. |
| <code>acceptance.11.311.3</code> | req | 11311.3 |
| <code>acceptance.11.311.3</code> | scenario | Given a manifest opened at height 7 with block hash A, when begin is called again with block hash B, then it reverts ManifestConflict, and an append to an unopened commitment reverts InvalidManifest |
| <code>acceptance.11.311.3</code> | test | services/interop/contracts/ethereum-mirror/test/LayerXMirrorArchive.t.sol::testBeginRejectsDifferentBlockHashOnReopen (planned) |
| <code>acceptance.11.311.3</code> | qualification | UNRUN |
| <code>req.11311</code> | ac_3 | A second begin for the same commitment with a different blockHash reverts ManifestConflict (services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:69), and append (services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:126) accepts chunks only after the committed manifest is open. |
| <code>acceptance.11.312.1</code> | req | 11312.1 |
| <code>acceptance.11.312.1</code> | scenario | Given a program test context, when initialize is sent with version 4, height 7 and a non-zero block hash, then the manifest account starts with LXMMAN04 and bytes 84-116 equal the block hash |
| <code>acceptance.11.312.1</code> | test | services/interop/contracts/solana-mirror/tests/runtime.rs::version_4_manifest_binds_the_committed_block_hash (planned) |
| <code>acceptance.11.312.1</code> | qualification | UNRUN |
| <code>req.11312</code> | ac_1 | process_instruction (services/interop/contracts/solana-mirror/src/lib.rs:47) accepts instruction version 4 (services/interop/contracts/solana-mirror/src/lib.rs:20), writes LXMMAN04 at [0..8] and the committed block hash at [84..116] of the 334-byte manifest (services/interop/contracts/solana-mirror/src/lib.rs:103-116) and LXMCHK04 chunk accounts. |
| <code>acceptance.11.312.2</code> | req | 11312.2 |
| <code>acceptance.11.312.2</code> | scenario | Given a program test context, when initialize is sent once with a zero block hash and once with version 3, then the first fails with the bounds error, the second with the instruction error and no manifest account is created |
| <code>acceptance.11.312.2</code> | test | services/interop/contracts/solana-mirror/tests/runtime.rs::zero_block_hash_and_version_3_instructions_are_refused (planned) |
| <code>acceptance.11.312.2</code> | qualification | UNRUN |
| <code>req.11312</code> | ac_2 | initialize refuses a zero block hash with MirrorError::Bounds and any instruction carrying version 3 with MirrorError::Instruction (services/interop/contracts/solana-mirror/src/lib.rs:31-36). |
| <code>capability.11.27.1</code> | title | Encode the Ethereum begin call with the committed height and block hash |
| <code>capability.11.27.1</code> | reqs | 11291.1<br>11291.2 |
| <code>capability.11.27.2</code> | title | Encode Solana mirror instructions at version 4 with the committed block hash |
| <code>capability.11.27.2</code> | reqs | 11292.1<br>11292.2 |
| <code>capability.11.28.1</code> | title | Store the committed height and block hash in the Ethereum archive manifest |
| <code>capability.11.28.1</code> | reqs | 11311.1<br>11311.2<br>11311.3 |
| <code>capability.11.28.2</code> | title | Move the Solana mirror program to instruction version 4 with the committed block hash |
| <code>capability.11.28.2</code> | reqs | 11312.1<br>11312.2 |

## portable-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-export-with-transaction-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs</code> (planned) | <code>pub fn export_with_transaction(..., raw_transaction: &amp;[u8]) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |
| <code>interface.11.rs-portable-portable-receipt-v2-from-sequencer-receipt-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs</code> (planned) | <code>pub fn from_sequencer_receipt(answer: &amp;serde_json::Value, receipt_keys: &amp;[ReceiptKeyEntry], raw_transaction: Option&lt;&amp;[u8]&gt;) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |
| <code>interface.11.rs-portable-portable-verified-receipt-v2-raw-transaction-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs</code> (planned) | <code>pub fn raw_transaction(&amp;self) -&gt; Option&lt;&amp;[u8]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-export-with-transaction-j48</code> | notes | planned; no caller at HEAD; beside PortableReceiptV2::export at services/interop/crates/layerx-portable/src/receipt.rs:374 |
| <code>interface.11.rs-portable-portable-receipt-v2-from-sequencer-receipt-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-portable-portable-verified-receipt-v2-raw-transaction-j48</code> | notes | planned; no caller at HEAD |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.331.1</code> | req | 11331.1 |
| <code>acceptance.11.331.1</code> | scenario | Given the V1-pax-transfer receipt bytes, its pinned key and its rawTransaction, when export_with_transaction then verify_v2 run, then the verified receipt returns the same raw bytes from raw_transaction and its activity id equals the case txHash |
| <code>acceptance.11.331.1</code> | test | services/interop/crates/layerx-portable/tests/sequencer_vectors.rs::a_pax_transfer_vector_exports_with_its_raw_transaction (planned) |
| <code>acceptance.11.331.1</code> | qualification | UNRUN |
| <code>req.11331</code> | ac_1 | PortableReceiptV2::export_with_transaction exports the V1-pax-transfer case of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:1 with rawTransaction as lowercase 0x hex, and verify_v2 (services/interop/crates/layerx-portable/src/receipt.rs:434) accepts it only when keccak256 of the raw bytes equals the receipt activity id, which rpc/sequencer_receipt.go:198 checks against txHash. |
| <code>acceptance.11.331.2</code> | req | 11331.2 |
| <code>acceptance.11.331.2</code> | scenario | Given the V1-pax-transfer proof with the last byte of rawTransaction flipped, when verify_v2 runs, then it returns RawTransactionMismatch |
| <code>acceptance.11.331.2</code> | test | services/interop/crates/layerx-portable/tests/sequencer_vectors.rs::a_raw_transaction_that_does_not_hash_to_the_activity_id_is_refused (planned) |
| <code>acceptance.11.331.2</code> | qualification | UNRUN |
| <code>req.11331</code> | ac_2 | verify_v2 refuses a proof whose rawTransaction does not hash to the activity id with PortableReceiptError::RawTransactionMismatch, the case V3-activity-id-not-raw-hash of the shared vector. |
| <code>acceptance.11.331.3</code> | req | 11331.3 |
| <code>acceptance.11.331.3</code> | scenario | Given the v2 signed bytes of the existing vector exported by PortableReceiptV2::export, when verify_v2 runs, then it verifies and raw_transaction is None |
| <code>acceptance.11.331.3</code> | test | services/interop/crates/layerx-portable/tests/receipt_vectors.rs::a_v2_proof_without_raw_transaction_still_verifies (planned) |
| <code>acceptance.11.331.3</code> | qualification | UNRUN |
| <code>req.11331</code> | ac_3 | A v2 proof without the rawTransaction field keeps verifying exactly as the existing v2_receipt_verifies_offline test at services/interop/crates/layerx-portable/tests/receipt_vectors.rs:87 asserts, and raw_transaction returns None. |
| <code>acceptance.11.332.1</code> | req | 11332.1 |
| <code>acceptance.11.332.1</code> | scenario | Given the V1 and V2 cases of the shared sequencer receipt vector, when from_sequencer_receipt runs on each receiptJson with its rawTransaction, then each proof verifies through PortableReceiptV2::verify_json under the case keys |
| <code>acceptance.11.332.1</code> | test | services/interop/crates/layerx-portable/tests/sequencer_vectors.rs::every_valid_shared_case_exports_from_its_receipt_answer (planned) |
| <code>acceptance.11.332.1</code> | qualification | UNRUN |
| <code>req.11332</code> | ac_1 | PortableReceiptV2::from_sequencer_receipt(answer, receipt_keys, raw_transaction) takes the receipt hex of a receipted or committed pax_sequencerReceipt answer (rpc/sequencer_receipt.go:150-156), uses blockNumber as the inclusion height at committed and height otherwise, selects the key by keyId, and exports a proof that verify_json (services/interop/crates/layerx-portable/src/receipt.rs:479) accepts. |
| <code>acceptance.11.332.2</code> | req | 11332.2 |
| <code>acceptance.11.332.2</code> | scenario | Given each V3 case and a submitted answer carrying only rung, txHash and nonce, when from_sequencer_receipt runs, then each returns an error and no proof is produced |
| <code>acceptance.11.332.2</code> | test | services/interop/crates/layerx-portable/tests/sequencer_vectors.rs::the_shared_refusal_cases_are_refused_at_export (planned) |
| <code>acceptance.11.332.2</code> | qualification | UNRUN |
| <code>req.11332</code> | ac_2 | from_sequencer_receipt refuses the V3 cases of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:1 (key not pinned, height outside the key range, activity id not the raw hash, tampered bytes) with the PortableReceiptError of the failed check, and refuses a submitted answer with PortableReceiptError::NotReceipted. |
| <code>acceptance.11.333.1</code> | req | 11333.1 |
| <code>acceptance.11.333.1</code> | scenario | Given a v2 proof exported from the V1-erc20-approve case, when the independent verifier runs on it and on each single-field substitution, then the original verifies and every substitution is refused |
| <code>acceptance.11.333.1</code> | test | services/interop/crates/layerx-portable/tests/independent_verifier.rs::independent_verifier_accepts_a_v2_proof_and_refuses_substitutions (planned) |
| <code>acceptance.11.333.1</code> | qualification | UNRUN |
| <code>req.11333</code> | ac_1 | The independent verifier harness of services/interop/crates/layerx-portable/tests/independent_verifier.rs:44-98 verifies v2 proofs exported from the shared sequencer receipt vector against the pinned key list only, and refuses each proof with one byte of signedReceipt, receiptDigest, receiptKey or rawTransaction altered. |
| <code>acceptance.11.333.2</code> | req | 11333.2 |
| <code>acceptance.11.333.2</code> | scenario | Given the v2 proof exported from the V1-erc20-approve case written to the Python verifier's stdin with the shared fixture path, when the Python verifier runs, then it exits 0, and with rawTransaction altered it exits non-zero |
| <code>acceptance.11.333.2</code> | test | services/interop/crates/layerx-portable/tests/independent_verifier.rs::python_verifier_accepts_the_same_v2_proof (planned) |
| <code>acceptance.11.333.2</code> | qualification | UNRUN |
| <code>req.11333</code> | ac_2 | services/interop/crates/layerx-portable/tests/python_portable_verifier.py:59-90 verifies a v2 proof from FORMAT.md alone: it recomputes sha256("LXP/v1/receipt\0" &#124;&#124; unsigned preimage), checks the Ed25519 signature with the pinned key whose window covers the signed height and checks keccak256 of rawTransaction against the activity id. |
| <code>acceptance.11.334.1</code> | req | 11334.1 |
| <code>acceptance.11.334.1</code> | scenario | Given the v1 golden JSON and a v2 proof, when verify_portable_json runs on each with the pinned keys, then the first returns UnsupportedFormat and the second returns the verified v2 receipt |
| <code>acceptance.11.334.1</code> | test | services/interop/crates/layerx-portable/tests/receipt_vectors.rs::a_v1_document_is_refused_as_unsupported (planned) |
| <code>acceptance.11.334.1</code> | qualification | UNRUN |
| <code>req.11334</code> | ac_1 | verify_portable_json (services/interop/crates/layerx-portable/src/receipt.rs:609) takes only the pinned receipt keys and refuses a layerx-receipt-proof-v1 document with PortableReceiptError::UnsupportedFormat, and PORTABLE_RECEIPT_FORMAT (services/interop/crates/layerx-portable/src/receipt.rs:14) equals layerx-receipt-proof-v2. |
| <code>acceptance.11.334.2</code> | req | 11334.2 |
| <code>acceptance.11.334.2</code> | scenario | Given the crate, when interop_portable_verification is called, then it returns the v2 anchor string |
| <code>acceptance.11.334.2</code> | test | services/interop/crates/layerx-portable/tests/receipt_vectors.rs::the_portable_anchor_names_the_v2_format (planned) |
| <code>acceptance.11.334.2</code> | qualification | UNRUN |
| <code>req.11334</code> | ac_2 | interop_portable_verification (services/interop/crates/layerx-portable/src/lib.rs:29) returns layerx-receipt-proof-v2-and-pinned-external-evidence-v1 and the crate re-exports no PortableReceipt, PortableVerifiedReceipt or AuthorizedBatch name at services/interop/crates/layerx-portable/src/lib.rs:21-25. |
| <code>capability.11.29.1</code> | title | Carry an optional rawTransaction in v2 portable receipts bound to the activity id |
| <code>capability.11.29.1</code> | reqs | 11331.1<br>11331.2<br>11331.3 |
| <code>capability.11.29.2</code> | title | Export a v2 portable receipt from a pax_sequencerReceipt answer |
| <code>capability.11.29.2</code> | reqs | 11332.1<br>11332.2 |
| <code>capability.11.29.3</code> | title | Move the independent Rust and Python portable verifiers to v2 proofs |
| <code>capability.11.29.3</code> | reqs | 11333.1<br>11333.2 |
| <code>capability.11.29.4</code> | title | Retire the v1 portable receipt and the batch-authority path |
| <code>capability.11.29.4</code> | reqs | 11334.1<br>11334.2 |

## x-websearch-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-xws-payment-payer-address-j48</code> | <code>config_key</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/config.rs</code> (planned) | <code>x-websearch config payment.payer_address: 0x-prefixed 20-byte EVM address</code> |
| <code>interface.11.rs-xws-kernel-network-id-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/watch.rs</code> (planned) | <code>pub fn kernel_network_id(rpc: &amp;EvmRpc) -&gt; Result&lt;u32, EvmError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-xws-payment-payer-address-j48</code> | notes | planned; no caller at HEAD; replaces payment.payer_did read at services/interop/crates/x-websearch/src/config.rs:901 |
| <code>interface.11.rs-xws-kernel-network-id-j48</code> | notes | planned; no caller at HEAD; eth_call of getKernelNetworkId() declared at precompiles/xweb/abi.json:13 |

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

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.361.1</code> | req | 11361.1 |
| <code>acceptance.11.361.1</code> | scenario | Given the committed configuration with gateway.receipt_keys holding two windows, when Config::parse runs, then GatewayConfig.receipt_keys holds both windows in order with their key ids, keys and heights |
| <code>acceptance.11.361.1</code> | test | services/interop/crates/x-websearch/tests/config.rs::receipt_keys_are_read_from_the_gateway_object (planned) |
| <code>acceptance.11.361.1</code> | qualification | UNRUN |
| <code>req.11361</code> | ac_1 | Config::parse (services/interop/crates/x-websearch/src/config.rs:272) reads gateway.receipt_keys as a non-empty list of {key_id, public_key, first_height, last_height} into GatewayConfig.receipt_keys: Vec&lt;SequencerAuthorization&gt;, in place of the sequencer_id and sequencer_public_key fields read at services/interop/crates/x-websearch/src/config.rs:873-881. |
| <code>acceptance.11.361.2</code> | req | 11361.2 |
| <code>acceptance.11.361.2</code> | scenario | Given configurations with each of those defects, when Config::parse runs on each, then each returns ConfigError naming gateway.receipt_keys or the unknown field |
| <code>acceptance.11.361.2</code> | test | services/interop/crates/x-websearch/tests/config.rs::malformed_receipt_keys_are_refused_naming_the_field (planned) |
| <code>acceptance.11.361.2</code> | qualification | UNRUN |
| <code>req.11361</code> | ac_2 | gateway.receipt_keys is refused naming the field for an empty list, a weak or malformed Ed25519 key, last_height below first_height, overlapping windows of one key id, and a left-over sequencer_id or sequencer_public_key field as unknown. |
| <code>acceptance.11.361.3</code> | req | 11361.3 |
| <code>acceptance.11.361.3</code> | scenario | Given the gateway recording and a configuration whose only key window ends below the receipt height, when a metered payment is settled, then the route answers the receipt_unverified challenge and the response is not released |
| <code>acceptance.11.361.3</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_receipt_outside_every_configured_key_window_is_refused (planned) |
| <code>acceptance.11.361.3</code> | qualification | UNRUN |
| <code>req.11361</code> | ac_3 | PaymentGate verify_receipt (services/interop/crates/x-websearch/src/payment.rs:2158) calls verify_sequencer_signature with ReceiptKeys::HeightRanges of the configured windows, so a receipt signed at a height no window covers is refused receipt_unverified and releases nothing. |
| <code>acceptance.11.362.1</code> | req | 11362.1 |
| <code>acceptance.11.362.1</code> | scenario | Given the committed configuration without kernel_network_id and with kernel_network_id 0, when Config::parse runs, then the first yields None and the second returns ConfigError naming kernel_network_id |
| <code>acceptance.11.362.1</code> | test | services/interop/crates/x-websearch/tests/config.rs::an_absent_kernel_network_id_parses_and_zero_is_refused (planned) |
| <code>acceptance.11.362.1</code> | qualification | UNRUN |
| <code>req.11362</code> | ac_1 | Config.kernel_network_id (services/interop/crates/x-websearch/src/config.rs:263) becomes Option&lt;u32&gt;: an absent field parses to None and 0 is still refused naming kernel_network_id, as positive_u32 refuses it at services/interop/crates/x-websearch/src/config.rs:290. |
| <code>acceptance.11.362.2</code> | req | 11362.2 |
| <code>acceptance.11.362.2</code> | scenario | Given a recorded EVM endpoint answering getKernelNetworkId with 7 and another answering 33 bytes, when kernel_network_id runs on each, then the first returns 7 and the second Malformed |
| <code>acceptance.11.362.2</code> | test | services/interop/crates/x-websearch/tests/submit.rs::the_kernel_network_id_is_read_from_the_xweb_precompile (planned) |
| <code>acceptance.11.362.2</code> | qualification | UNRUN |
| <code>req.11362</code> | ac_2 | kernel_network_id(rpc) in services/interop/crates/x-websearch/src/watch.rs calls eth_call of getKernelNetworkId() on 0x1019 through EvmRpc::eth_call (services/interop/crates/x-websearch/src/watch.rs:164) and returns the uint32 word, refusing a non-32-byte answer or a value above u32::MAX with EvmError::Malformed. |
| <code>acceptance.11.402.1</code> | req | 11402.1 |
| <code>acceptance.11.402.1</code> | scenario | Given the committed configuration with a kernel section added, when Config::parse runs, then it returns ConfigError naming kernel |
| <code>acceptance.11.402.1</code> | test | services/interop/crates/x-websearch/tests/config.rs::a_kernel_section_is_refused_as_an_unknown_field (planned) |
| <code>acceptance.11.402.1</code> | qualification | UNRUN |
| <code>req.11402</code> | ac_1 | Config::parse refuses a top-level kernel section as an unknown field naming kernel, once KernelConfig (services/interop/crates/x-websearch/src/config.rs:222) and its parse at services/interop/crates/x-websearch/src/config.rs:961-977 are deleted. |
| <code>acceptance.11.402.2</code> | req | 11402.2 |
| <code>acceptance.11.402.2</code> | scenario | Given the committed configuration with gateway.authorization_file set, when Config::parse runs, then it returns ConfigError naming gateway.authorization_file |
| <code>acceptance.11.402.2</code> | test | services/interop/crates/x-websearch/tests/config.rs::a_gateway_authorization_file_is_refused_as_an_unknown_field (planned) |
| <code>acceptance.11.402.2</code> | qualification | UNRUN |
| <code>req.11402</code> | ac_2 | Config::parse refuses gateway.authorization_file as an unknown field, since GatewayConfig (services/interop/crates/x-websearch/src/config.rs:156) keeps only the settlement endpoint and sequencer trust. |
| <code>acceptance.11.405.1</code> | req | 11405.1 |
| <code>acceptance.11.405.1</code> | scenario | Given the committed example configuration with each placeholder replaced by a valid value, when Config::parse runs, then it succeeds and the parsed gateway endpoint is the replaced value |
| <code>acceptance.11.405.1</code> | test | services/interop/crates/x-websearch/tests/example_config.rs::the_committed_example_configuration_parses (planned) |
| <code>acceptance.11.405.1</code> | qualification | UNRUN |
| <code>req.11405</code> | ac_1 | Config::parse accepts services/interop/crates/x-websearch/config.example.json:1 after its environment placeholders are replaced with valid values, with payment.payer_address, receipt keys and no kernel section. |
| <code>acceptance.11.405.2</code> | req | 11405.2 |
| <code>acceptance.11.405.2</code> | scenario | Given the committed example configuration, when its keys are listed, then none of the retired field names appears |
| <code>acceptance.11.405.2</code> | test | services/interop/crates/x-websearch/tests/example_config.rs::the_example_configuration_names_no_retired_field (planned) |
| <code>acceptance.11.405.2</code> | qualification | UNRUN |
| <code>req.11405</code> | ac_2 | The example configuration names none of kernel, gateway.authorization_file, payment.payer_did or sequencer_id, the fields the loader refuses after tasks 11.36.2 and 11.34.7. |
| <code>capability.11.31.1</code> | title | Replace the gateway sequencer key with a receipt_keys list in x-websearch |
| <code>capability.11.31.1</code> | reqs | 11361.1<br>11361.2<br>11361.3 |
| <code>capability.11.31.2</code> | title | Make kernel_network_id optional and read it from getKernelNetworkId |
| <code>capability.11.31.2</code> | reqs | 11362.1<br>11362.2 |
| <code>capability.11.36.2</code> | title | Remove the kernel config section and gateway.authorization_file from x-websearch |
| <code>capability.11.36.2</code> | reqs | 11402.1<br>11402.2 |
| <code>capability.11.36.5</code> | title | Keep the x-websearch example configuration and README in step with the loader |
| <code>capability.11.36.5</code> | reqs | 11405.1<br>11405.2 |

## x-websearch-payment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-payment-exact-log-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/payment.rs</code> (planned) | <code>pub fn decode_payment_exact(log: &amp;SettlementLog) -&gt; Option&lt;PaymentExactLog&gt;</code> |
| <code>interface.11.rs-xws-read-payer-grant-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/payment.rs</code> (planned) | <code>pub fn read_payer_grant(rpc: &amp;SettlementRpc, grant_id: &amp;[u8; 32]) -&gt; Result&lt;Option&lt;Grant&gt;, SettlementError&gt;</code> |
| <code>interface.11.rs-xws-settlement-error-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/settlement.rs</code> (planned) | <code>pub enum SettlementError { Transport, Rpc, Malformed, ChainMismatch }</code> |
| <code>interface.11.rs-xws-settlement-receipt-decode-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/settlement.rs</code> (planned) | <code>pub fn decode(value: &amp;Value) -&gt; Result&lt;SettlementReceipt, SettlementError&gt;</code> |
| <code>interface.11.rs-xws-settlement-receipt-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/settlement.rs</code> (planned) | <code>pub struct SettlementReceipt { rung, tx_hash, nonce, seq, height, status, logs, key_id, receipt }</code> |
| <code>interface.11.rs-xws-settlement-rpc-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/settlement.rs</code> (planned) | <code>pub struct SettlementRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-payment-exact-log-j48</code> | notes | planned; no caller at HEAD; PaymentExact(bytes32 indexed,bytes32 indexed,bytes32 indexed,bytes32,uint128) emitted by 0x101A |
| <code>interface.11.rs-xws-read-payer-grant-j48</code> | notes | planned; no caller at HEAD; eth_call of getPayerGrant(bytes32) on 0x101A |
| <code>interface.11.rs-xws-settlement-error-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-xws-settlement-receipt-decode-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-xws-settlement-receipt-j48</code> | notes | planned; no caller at HEAD; decoded from pax_sequencerReceipt answers (rpc/sequencer_receipt.go:150-156) |
| <code>interface.11.rs-xws-settlement-rpc-j48</code> | notes | planned; no caller at HEAD; replaces GatewayRpc at services/interop/crates/x-websearch/src/payment.rs:373 |

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

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.391.1</code> | req | 11391.1 |
| <code>acceptance.11.391.1</code> | scenario | Given a loopback endpoint answering eth_chainId with the configured chain id plus one, when SettlementRpc::call is made for eth_getTransactionCount, then it returns ChainMismatch and the endpoint saw only eth_chainId |
| <code>acceptance.11.391.1</code> | test | services/interop/crates/x-websearch/tests/settlement.rs::the_settlement_client_checks_the_chain_id_before_each_call (planned) |
| <code>acceptance.11.391.1</code> | qualification | UNRUN |
| <code>req.11391</code> | ac_1 | SettlementRpc::call in services/interop/crates/x-websearch/src/settlement.rs (planned) sends eth_chainId first and refuses an endpoint whose chain id differs from the configured evm.chain_id (services/interop/crates/x-websearch/src/config.rs:162-166) with SettlementError::ChainMismatch before sending the requested method. |
| <code>acceptance.11.391.2</code> | req | 11391.2 |
| <code>acceptance.11.391.2</code> | scenario | Given a recorded committed pax_sequencerReceipt answer, when settlement::decode runs, then every field equals the recording and the receipt bytes equal the decoded hex |
| <code>acceptance.11.391.2</code> | test | services/interop/crates/x-websearch/tests/settlement.rs::a_committed_sequencer_receipt_answer_decodes_every_field (planned) |
| <code>acceptance.11.391.2</code> | qualification | UNRUN |
| <code>req.11391</code> | ac_2 | settlement::decode reads rung, txHash, nonce, seq, height, status, logs {address, topics, data, logIndex}, keyId and receipt hex from a pax_sequencerReceipt answer, plus blockNumber and blockHash at committed, the fields of rpc/sequencer_receipt.go:125-156. |
| <code>acceptance.11.391.3</code> | req | 11391.3 |
| <code>acceptance.11.391.3</code> | scenario | Given three recorded answers with those defects, when settlement::decode runs on each, then each returns Malformed |
| <code>acceptance.11.391.3</code> | test | services/interop/crates/x-websearch/tests/settlement.rs::a_submitted_answer_with_receipt_fields_is_refused (planned) |
| <code>acceptance.11.391.3</code> | qualification | UNRUN |
| <code>req.11391</code> | ac_3 | settlement::decode returns SettlementError::Malformed for a submitted answer carrying receipt, seq or height, for an unknown rung string, and for a receipted answer carrying blockNumber. |
| <code>acceptance.11.391.4</code> | req | 11391.4 |
| <code>acceptance.11.391.4</code> | scenario | Given a loopback endpoint answering eth_getTransactionCount with 0x2a, when pending_nonce runs for the receiver address, then it returns 42 and the call carried the pending tag |
| <code>acceptance.11.391.4</code> | test | services/interop/crates/x-websearch/tests/settlement.rs::pending_nonce_reads_the_pending_transaction_count (planned) |
| <code>acceptance.11.391.4</code> | qualification | UNRUN |
| <code>req.11391</code> | ac_4 | SettlementRpc::pending_nonce(address) calls eth_getTransactionCount with the 0x-prefixed address and the pending tag and returns the quantity, refusing a non-canonical quantity with SettlementError::Malformed. |
| <code>acceptance.11.392.1</code> | req | 11392.1 |
| <code>acceptance.11.392.1</code> | scenario | Given a grant id, recipient, amount 1000 and fixed key, deadline, purpose and trace words, when draw_calldata runs, then the bytes start with 0x2e89490f and equal the independently recorded encoding in services/interop/crates/x-websearch/tests/fixtures/gateway/metered.json |
| <code>acceptance.11.392.1</code> | test | services/interop/crates/x-websearch/tests/payment.rs::draw_calldata_matches_the_ledger_draw_selector_and_layout (planned) |
| <code>acceptance.11.392.1</code> | qualification | UNRUN |
| <code>req.11392</code> | ac_1 | draw_calldata in services/interop/crates/x-websearch/src/payment.rs encodes draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32) with selector 0x2e89490f, the grant id, recipient account, amount, idempotency key, deadline in milliseconds, purpose hash and trace word, in place of the kernel activity sign_draw builds at services/interop/crates/x-websearch/src/payment.rs:273-352. |
| <code>acceptance.11.392.2</code> | req | 11392.2 |
| <code>acceptance.11.392.2</code> | scenario | Given a loopback endpoint answering getPayerGrant with a recorded grant and another answering zero words, when read_payer_grant runs on each, then the first returns the grant with its payer, recipient, asset, purpose and per-draw maximum and the second returns None |
| <code>acceptance.11.392.2</code> | test | services/interop/crates/x-websearch/tests/payment.rs::the_payer_grant_is_read_from_the_ledger_precompile (planned) |
| <code>acceptance.11.392.2</code> | qualification | UNRUN |
| <code>req.11392</code> | ac_2 | read_payer_grant calls eth_call of getPayerGrant(bytes32) on 0x000000000000000000000000000000000000101A through SettlementRpc and decodes the answer into Grant, returning None for an all-zero grant; check_grant (services/interop/crates/x-websearch/src/payment.rs:2271-2299) then runs on the decoded grant with the five refusals unchanged. |
| <code>acceptance.11.392.3</code> | req | 11392.3 |
| <code>acceptance.11.392.3</code> | scenario | Given the metered recording with getPayerGrant answering zero words, when a metered payment is settled, then the route answers the invalid_grant challenge and the endpoint saw no eth_sendRawTransaction |
| <code>acceptance.11.392.3</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_grant_the_ledger_does_not_hold_is_refused_before_any_transaction (planned) |
| <code>acceptance.11.392.3</code> | qualification | UNRUN |
| <code>req.11392</code> | ac_3 | Plane::draw (services/interop/crates/x-websearch/src/payment.rs:1948) refuses a metered payload whose grant id the ledger answers as absent with invalid_grant and sends no eth_sendRawTransaction. |
| <code>acceptance.11.393.1</code> | req | 11393.1 |
| <code>acceptance.11.393.1</code> | scenario | Given the metered recording, when a metered payment is settled, then the endpoint receives one eth_sendRawTransaction whose recovered sender is the receiver address and whose input is the draw calldata, and the payment record holds the transaction hash |
| <code>acceptance.11.393.1</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_metered_draw_is_broadcast_as_a_receiver_signed_ledger_transaction (planned) |
| <code>acceptance.11.393.1</code> | qualification | UNRUN |
| <code>req.11393</code> | ac_1 | Plane::draw signs a legacy EIP-155 transaction to 0x101A carrying draw_calldata with the receiver secp256k1 key at the evm.chain_id, sends it with eth_sendRawTransaction and saves tx_hash and raw_transaction in PaymentRecord before the broadcast, replacing the lx_sendActivity submit at services/interop/crates/x-websearch/src/payment.rs:2026-2043. |
| <code>acceptance.11.393.2</code> | req | 11393.2 |
| <code>acceptance.11.393.2</code> | scenario | Given the metered recording answering a pending count of 7, when two metered payments are settled in turn, then the broadcast transactions carry nonces 7 and 8 and no lx_getSequence request is sent |
| <code>acceptance.11.393.2</code> | test | services/interop/crates/x-websearch/tests/payment.rs::draw_nonces_are_sequential_for_one_receiver (planned) |
| <code>acceptance.11.393.2</code> | qualification | UNRUN |
| <code>req.11393</code> | ac_2 | The draw nonce is read with eth_getTransactionCount at the pending tag under the payment store lock, so two draws settled back to back carry consecutive nonces and lx_getSequence (services/interop/crates/x-websearch/src/payment.rs:2010-2024) is never called. |
| <code>acceptance.11.393.3</code> | req | 11393.3 |
| <code>acceptance.11.393.3</code> | scenario | Given a receiver key file holding a valid secp256k1 scalar and another holding the group order, when KeyFiles::load runs on each, then the first yields the key whose address matches the recorded receiver address and the second is refused naming X_WEBSEARCH_RECEIVER_KEY_FILE |
| <code>acceptance.11.393.3</code> | test | services/interop/crates/x-websearch/tests/keys.rs::the_receiver_key_file_holds_a_secp256k1_key (planned) |
| <code>acceptance.11.393.3</code> | qualification | UNRUN |
| <code>req.11393</code> | ac_3 | Keys::receiver (services/interop/crates/x-websearch/src/keys.rs:259) returns a k256 SigningKey read from X_WEBSEARCH_RECEIVER_KEY_FILE (services/interop/crates/x-websearch/src/keys.rs:10) and refuses a zero or out-of-range scalar naming the variable, as the attestor and submitter keys are read at services/interop/crates/x-websearch/src/keys.rs:161-175. |
| <code>acceptance.11.394.1</code> | req | 11394.1 |
| <code>acceptance.11.394.1</code> | scenario | Given the binary started against a loopback settlement endpoint answering eth_chainId with another chain id, when GET /readyz is requested, then it answers 503 naming the settlement role and the endpoint saw no lx_getNodeInfo request |
| <code>acceptance.11.394.1</code> | test | services/interop/crates/x-websearch/tests/binary.rs::the_binary_refuses_a_settlement_endpoint_on_another_chain (planned) |
| <code>acceptance.11.394.1</code> | qualification | UNRUN |
| <code>req.11394</code> | ac_1 | settlement_usable (services/interop/crates/x-websearch/src/main.rs:211-238) builds SettlementRpc for gateway.endpoint, reports the paid routes not ready when eth_chainId differs from evm.chain_id, and no longer calls lx_getNodeInfo or compares authorised_sequencer_key and the layerx_wire handover sequencer id. |
| <code>acceptance.11.394.2</code> | req | 11394.2 |
| <code>acceptance.11.394.2</code> | scenario | Given the binary started against a loopback settlement endpoint answering the configured chain id and null for the probe, when GET /readyz is requested, then the settlement role is listed as ready |
| <code>acceptance.11.394.2</code> | test | services/interop/crates/x-websearch/tests/binary.rs::the_binary_reports_settlement_ready_on_the_configured_chain (planned) |
| <code>acceptance.11.394.2</code> | qualification | UNRUN |
| <code>req.11394</code> | ac_2 | With an endpoint answering eth_chainId equal to evm.chain_id and pax_sequencerReceipt with null for the probe hash, settlement_usable reports the settlement role ready. |
| <code>acceptance.11.395.1</code> | req | 11395.1 |
| <code>acceptance.11.395.1</code> | scenario | Given the pending recording that answers null, then submitted, then receipted for the draw hash, when the same metered payment is retried three times, then the raw transaction is rebroadcast once, the second retry stays pending and the third releases the response, and a fourth retry is answered from the store without a second release |
| <code>acceptance.11.395.1</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_pending_draw_recovers_from_the_sequencer_receipt_and_releases_once (planned) |
| <code>acceptance.11.395.1</code> | qualification | UNRUN |
| <code>req.11395</code> | ac_1 | Plane::recover (services/interop/crates/x-websearch/src/payment.rs:2045-2076) reads SettlementRpc::sequencer_receipt for the recorded tx_hash: null rebroadcasts the recorded raw_transaction, submitted stays pending, and a receipted or committed answer whose receipt verifies under the configured receipt keys and whose PaymentDrawn log of 0x101A matches the grant, recipient, amount and idempotency key releases the response once. |
| <code>acceptance.11.395.2</code> | req | 11395.2 |
| <code>acceptance.11.395.2</code> | scenario | Given the pending recording answering a receipted status 0 receipt for the draw hash, when the metered payment is retried, then the route answers the payment_failed challenge and no response is released |
| <code>acceptance.11.395.2</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_reverted_draw_releases_nothing (planned) |
| <code>acceptance.11.395.2</code> | qualification | UNRUN |
| <code>req.11395</code> | ac_2 | A verified receipted answer with status 0 for the draw hash makes conclude return payment_failed and releases nothing, as the completed-state check at services/interop/crates/x-websearch/src/payment.rs:2098-2103 does today. |
| <code>acceptance.11.395.3</code> | req | 11395.3 |
| <code>acceptance.11.395.3</code> | scenario | Given recordings with a missing log, a log with another amount and a receipt signed by an unconfigured key, when the metered payment is retried on each, then the answers are receipt_mismatch, receipt_mismatch and receipt_unverified |
| <code>acceptance.11.395.3</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_draw_receipt_without_a_matching_payment_drawn_log_is_refused (planned) |
| <code>acceptance.11.395.3</code> | qualification | UNRUN |
| <code>req.11395</code> | ac_3 | A verified receipt whose logs carry no PaymentDrawn event of 0x101A, or one whose amount, recipient or idempotency key differ from the record, is refused receipt_mismatch, and a receipt signed outside every receipt key window is refused receipt_unverified. |
| <code>acceptance.11.396.1</code> | req | 11396.1 |
| <code>acceptance.11.396.1</code> | scenario | Given the exact recording with a receipted payExact answer paying the price to the payee, when an exact payment names that transaction hash, then the response is released and the receipt digest is claimed |
| <code>acceptance.11.396.1</code> | test | services/interop/crates/x-websearch/tests/payment.rs::an_exact_payment_settles_from_its_pay_exact_transaction (planned) |
| <code>acceptance.11.396.1</code> | qualification | UNRUN |
| <code>req.11396</code> | ac_1 | Plane::exact (services/interop/crates/x-websearch/src/payment.rs:2198-2269) reads the payExact transaction hash from the exact payload, fetches pax_sequencerReceipt, verifies it under the receipt keys and releases when the PaymentExact log of 0x101A names this payee, asset and price, in place of the receipt bytes and lx_getReceipt call at :2254. |
| <code>acceptance.11.396.2</code> | req | 11396.2 |
| <code>acceptance.11.396.2</code> | scenario | Given one exact payment settled with a transaction hash, when a second request with another idempotency key names the same hash, then the second is answered receipt_consumed |
| <code>acceptance.11.396.2</code> | test | services/interop/crates/x-websearch/tests/payment.rs::an_exact_transaction_already_consumed_is_refused (planned) |
| <code>acceptance.11.396.2</code> | qualification | UNRUN |
| <code>req.11396</code> | ac_2 | A payExact transaction whose receipt digest is already held by another idempotency key is refused receipt_consumed, as the holder check at services/interop/crates/x-websearch/src/payment.rs:2228-2232 refuses it today. |
| <code>acceptance.11.396.3</code> | req | 11396.3 |
| <code>acceptance.11.396.3</code> | scenario | Given a ready delivery persisted with a receipted settlement, when the store is reopened and recover_pending_deliveries runs, then the delivery is served unchanged, and the same record with a transaction that is neither a 0x transaction hash nor lxp:&lt;digest&gt; is refused as invalid data |
| <code>acceptance.11.396.3</code> | test | services/interop/crates/x-websearch/tests/payment.rs::delivery_recovers_the_recorded_receipted_settlement (planned) |
| <code>acceptance.11.396.3</code> | qualification | UNRUN |
| <code>req.11396</code> | ac_3 | DeliveryRecord::validate (services/interop/crates/x-websearch/src/payment.rs:808-905) accepts a settlement whose transaction is the 0x-prefixed payment transaction hash and whose verificationLevel is receipted, beside the lxp:&lt;digest&gt; form at :851 and sequencer-signed at :862-863 that task 11.34.8 keeps for records written before the upgrade, so recover_pending_deliveries (:1556) serves it after a restart. |
| <code>acceptance.11.397.1</code> | req | 11397.1 |
| <code>acceptance.11.397.1</code> | scenario | Given the committed configuration with payment.payer_address and another with payment.payer_did, when Config::parse runs on each, then the first yields the address and the second returns ConfigError naming payment.payer_did |
| <code>acceptance.11.397.1</code> | test | services/interop/crates/x-websearch/tests/config.rs::payer_address_is_read_and_payer_did_is_refused (planned) |
| <code>acceptance.11.397.1</code> | qualification | UNRUN |
| <code>req.11397</code> | ac_1 | Config::parse reads payment.payer_address as a 0x-prefixed 20-byte address into PaymentConfig.payer_address and refuses a left-over payment.payer_did as an unknown field, in place of the payer_did read at services/interop/crates/x-websearch/src/config.rs:901-906. |
| <code>acceptance.11.397.2</code> | req | 11397.2 |
| <code>acceptance.11.397.2</code> | scenario | Given a gate configured with a payer address and a request whose header names another address, when the metered payment is settled, then the route answers 400 payer_mismatch, and with the same address it settles |
| <code>acceptance.11.397.2</code> | test | services/interop/crates/x-websearch/tests/payment.rs::the_configured_payer_address_is_offered_and_drawn_from (planned) |
| <code>acceptance.11.397.2</code> | qualification | UNRUN |
| <code>req.11397</code> | ac_2 | PaymentGate::payer (services/interop/crates/x-websearch/src/payment.rs:1191) returns the configured address; a LAYERX-PAYER-ADDRESS header may repeat it but a different address is refused payer_mismatch, and the grant payer is compared with the ledger account of that address. |
| <code>acceptance.11.397.3</code> | req | 11397.3 |
| <code>acceptance.11.397.3</code> | scenario | Given requests carrying only LAYERX-PAYER-DID and one carrying LAYERX-PAYER-ADDRESS 0x12, when each is settled, then the first is treated as having no named payer and the second answers 400 malformed_payer |
| <code>acceptance.11.397.3</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_payer_did_header_is_ignored_and_a_malformed_address_is_refused (planned) |
| <code>acceptance.11.397.3</code> | qualification | UNRUN |
| <code>req.11397</code> | ac_3 | The LAYERX-PAYER-DID header (services/interop/crates/x-websearch/src/payment.rs:64) is no longer read, and a LAYERX-PAYER-ADDRESS header that is not a 0x-prefixed 20-byte address is refused 400 malformed_payer. |
| <code>acceptance.11.398.1</code> | req | 11398.1 |
| <code>acceptance.11.398.1</code> | scenario | Given a payment directory holding a released record written before the upgrade, when the same request is retried, then the stored response is answered and the settlement endpoint sees no request |
| <code>acceptance.11.398.1</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_released_kernel_era_record_is_still_replayed (planned) |
| <code>acceptance.11.398.1</code> | qualification | UNRUN |
| <code>req.11398</code> | ac_1 | PaymentStore::load (services/interop/crates/x-websearch/src/payment.rs:933) reads a released record that carries the retired activity and activity_id fields of services/interop/crates/x-websearch/src/payment.rs:652-653, which deny_unknown_fields at :644 would otherwise refuse, and the gate replays its stored response for the same idempotency key without settling again. |
| <code>acceptance.11.398.2</code> | req | 11398.2 |
| <code>acceptance.11.398.2</code> | scenario | Given a payment directory holding one attempted, unreleased record written before the upgrade, when PaymentStore::open runs, then it returns an error naming that record file |
| <code>acceptance.11.398.2</code> | test | services/interop/crates/x-websearch/tests/payment.rs::an_unreleased_kernel_era_record_stops_startup (planned) |
| <code>acceptance.11.398.2</code> | qualification | UNRUN |
| <code>req.11398</code> | ac_2 | PaymentStore::open (services/interop/crates/x-websearch/src/payment.rs:914) refuses a directory holding an unreleased record with activity fields, naming the file, because its kernel activity may still settle and must not be drawn again on 0x101A. |
| <code>acceptance.11.398.3</code> | req | 11398.3 |
| <code>acceptance.11.398.3</code> | scenario | Given a ready delivery persisted before the upgrade with an lxp settlement, when the store is reopened and recover_pending_deliveries runs, then the delivery is served unchanged |
| <code>acceptance.11.398.3</code> | test | services/interop/crates/x-websearch/tests/payment.rs::a_kernel_era_delivery_is_still_served (planned) |
| <code>acceptance.11.398.3</code> | qualification | UNRUN |
| <code>req.11398</code> | ac_3 | DeliveryRecord::validate (services/interop/crates/x-websearch/src/payment.rs:808-905) keeps accepting a delivery whose settlement transaction is lxp:&lt;digest&gt; with sequencer-signed, so recover_pending_deliveries serves deliveries paid before the upgrade. |
| <code>acceptance.11.403.1</code> | req | 11403.1 |
| <code>acceptance.11.403.1</code> | scenario | Given the re-recorded gateway fixtures, when the pinned suite is computed, then its digest equals CONFORMANCE_SUITE_DIGEST |
| <code>acceptance.11.403.1</code> | test | services/interop/crates/x-websearch/tests/binary.rs::the_pinned_conformance_suite_is_the_recorded_gateway_exchange (existing) |
| <code>acceptance.11.403.1</code> | qualification | UNRUN |
| <code>req.11403</code> | ac_1 | CONFORMANCE_SUITE_DIGEST (services/interop/crates/x-websearch/src/lib.rs:43-46) equals SHA-256 over the recordings under tests/fixtures/gateway in file name order after the settlement re-recordings, as the test at services/interop/crates/x-websearch/tests/binary.rs:564 computes. |
| <code>acceptance.11.403.2</code> | req | 11403.2 |
| <code>acceptance.11.403.2</code> | scenario | Given the re-recorded gateway fixtures, when their answers are counted, then the count equals CONFORMANCE_VECTOR_COUNT |
| <code>acceptance.11.403.2</code> | test | services/interop/crates/x-websearch/tests/binary.rs::the_conformance_vector_count_is_the_number_of_recorded_answers (planned) |
| <code>acceptance.11.403.2</code> | qualification | UNRUN |
| <code>req.11403</code> | ac_2 | CONFORMANCE_VECTOR_COUNT (services/interop/crates/x-websearch/src/lib.rs:40) equals the number of recorded settlement answers across the files of tests/fixtures/gateway. |
| <code>acceptance.11.404.1</code> | req | 11404.1 |
| <code>acceptance.11.404.1</code> | scenario | Given the shared sequencer-receipt-v1 vector file, when each answer case is decoded, then every decoded field equals the vector's expectation |
| <code>acceptance.11.404.1</code> | test | services/interop/crates/x-websearch/tests/sequencer_vectors.rs::every_shared_answer_case_decodes_through_the_settlement_client (planned) |
| <code>acceptance.11.404.1</code> | qualification | UNRUN |
| <code>req.11404</code> | ac_1 | settlement::decode accepts every answer case of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:1 and returns the case's rung, seq, height, status and keyId. |
| <code>acceptance.11.404.2</code> | req | 11404.2 |
| <code>acceptance.11.404.2</code> | scenario | Given the V3 cases of the shared vector file and the receipt keys the file pins, when each is checked as a draw receipt, then each is refused receipt_unverified |
| <code>acceptance.11.404.2</code> | test | services/interop/crates/x-websearch/tests/sequencer_vectors.rs::shared_refusal_cases_do_not_release_a_payment (planned) |
| <code>acceptance.11.404.2</code> | qualification | UNRUN |
| <code>req.11404</code> | ac_2 | The payment receipt check refuses every V3 case of the shared vector file (key not pinned, height outside the key range, tampered bytes) with receipt_unverified, the refusal Plane::recover returns for an unverifiable receipt. |
| <code>capability.11.34.1</code> | title | Add the x-websearch settlement client for eth_ and pax_sequencerReceipt calls |
| <code>capability.11.34.1</code> | reqs | 11391.1<br>11391.2<br>11391.3<br>11391.4 |
| <code>capability.11.34.2</code> | title | Encode the 0x101A draw call and read getPayerGrant before a metered draw |
| <code>capability.11.34.2</code> | reqs | 11392.1<br>11392.2<br>11392.3 |
| <code>capability.11.34.3</code> | title | Broadcast metered draws as receiver-signed 0x101A transactions |
| <code>capability.11.34.3</code> | reqs | 11393.1<br>11393.2<br>11393.3 |
| <code>capability.11.34.4</code> | title | Check the settlement endpoint with eth_chainId and pax_ support at startup |
| <code>capability.11.34.4</code> | reqs | 11394.1<br>11394.2 |
| <code>capability.11.34.5</code> | title | Recover metered draws from pax_sequencerReceipt and retire the gateway lx_ client |
| <code>capability.11.34.5</code> | reqs | 11395.1<br>11395.2<br>11395.3 |
| <code>capability.11.34.6</code> | title | Settle exact payments from the payExact transaction hash and its PaymentExact log |
| <code>capability.11.34.6</code> | reqs | 11396.1<br>11396.2<br>11396.3 |
| <code>capability.11.34.7</code> | title | Name the payer by EVM address in config and the LAYERX-PAYER-ADDRESS header |
| <code>capability.11.34.7</code> | reqs | 11397.1<br>11397.2<br>11397.3 |
| <code>capability.11.34.8</code> | title | Load kernel-era payment and delivery records after the settlement upgrade |
| <code>capability.11.34.8</code> | reqs | 11398.1<br>11398.2<br>11398.3 |
| <code>capability.11.36.3</code> | title | Re-record the x-websearch conformance suite digest and vector count |
| <code>capability.11.36.3</code> | reqs | 11403.1<br>11403.2 |
| <code>capability.11.36.4</code> | title | Check x-websearch settlement decoding against the shared sequencer-receipt vectors |
| <code>capability.11.36.4</code> | reqs | 11404.1<br>11404.2 |
| <code>resolution.17.888</code> | title | decision.1123 binds: x-websearch settles paid routes on the 0x101A ledger precompile with receiver-signed EVM transactions |
| <code>resolution.17.888</code> | applies_to | decision.1123<br>task.11.34.1<br>task.11.34.2<br>task.11.34.3<br>task.11.34.5<br>task.11.34.6 |
| <code>resolution.17.888</code> | source | decision.1123 in fragments/J48/decision.kvx; tasks from affects |
| <code>resolution.17.891</code> | title | decision.1126 binds: The x-websearch conformance suite is re-recorded once after the settlement fixtures settle |
| <code>resolution.17.891</code> | applies_to | decision.1126<br>task.11.34.2<br>task.11.34.5<br>task.11.34.6<br>task.11.36.3 |
| <code>resolution.17.891</code> | source | decision.1126 in fragments/J48/decision.kvx; tasks from affects |

## x-websearch-program-requests

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-xws-program-attestor-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/program.rs</code> (planned) | <code>pub struct ProgramAttestor</code> |
| <code>interface.11.rs-xws-program-exchange-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/program.rs</code> (planned) | <code>pub struct ProgramExchange</code> |
| <code>interface.11.rs-xws-program-relay-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/program.rs</code> (planned) | <code>pub struct ProgramRelay</code> |
| <code>interface.11.rs-xws-request-watcher-program-work-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/watch.rs</code> (planned) | <code>pub fn program_work(&amp;self) -&gt; Vec&lt;ProgramRequest&gt;</code> |
| <code>interface.11.rs-xws-submitter-submit-program-j48</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/x-websearch/src/submit.rs</code> (planned) | <code>pub fn submit_program(&amp;mut self, key: ProgramKey, observation: &amp;[u8], signatures: usize) -&gt; Result&lt;Submission, SubmitError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-xws-program-attestor-j48</code> | notes | planned; no caller at HEAD; the former KernelAttestor at services/interop/crates/x-websearch/src/kernel.rs:475 |
| <code>interface.11.rs-xws-program-exchange-j48</code> | notes | planned; no caller at HEAD; moved from services/interop/crates/x-websearch/src/kernel.rs:1062 |
| <code>interface.11.rs-xws-program-relay-j48</code> | notes | planned; no caller at HEAD; replaces KernelRelay at services/interop/crates/x-websearch/src/kernel.rs:1655 |
| <code>interface.11.rs-xws-request-watcher-program-work-j48</code> | notes | planned; no caller at HEAD |
| <code>interface.11.rs-xws-submitter-submit-program-j48</code> | notes | planned; no caller at HEAD |

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

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.366.1</code> | req | 11366.1 |
| <code>acceptance.11.366.1</code> | scenario | Given a recorded eth_getLogs answer holding one XWebProgramRequested log for program P request 4 of kind fetch, when decode_program_requested runs, then the request carries P, 4, the fetch kind, the payload bytes and the log's block number |
| <code>acceptance.11.366.1</code> | test | services/interop/crates/x-websearch/tests/program_watch.rs::a_program_request_log_decodes_to_a_program_request (planned) |
| <code>acceptance.11.366.1</code> | qualification | UNRUN |
| <code>req.11366</code> | ac_1 | decode_program_requested in services/interop/crates/x-websearch/src/watch.rs decodes an XWebProgramRequested(bytes32 programId, uint64 requestId, uint8 kind, bytes payload, uint64 sequence) log of XWEB_PRECOMPILE (services/interop/crates/x-websearch/src/watch.rs:13) into program::ProgramRequest with its block number and source identity, applying MAX_PAYLOAD_BYTES (services/interop/crates/x-websearch/src/watch.rs:24) and the fetch or search kind check of modules/xweb/keeper/program.go:160. |
| <code>acceptance.11.366.2</code> | req | 11366.2 |
| <code>acceptance.11.366.2</code> | scenario | Given the same log from address 0x1018, with removed true, and with a truncated payload, when decode_program_requested runs on each, then the first two return ForeignLog and the third Malformed |
| <code>acceptance.11.366.2</code> | test | services/interop/crates/x-websearch/tests/program_watch.rs::a_foreign_or_removed_program_log_is_not_a_request (planned) |
| <code>acceptance.11.366.2</code> | qualification | UNRUN |
| <code>req.11366</code> | ac_2 | decode_program_requested returns EvmError::ForeignLog for a log of another address, another first topic or removed true, and EvmError::Malformed for a payload length that runs past the data, as decode_requested does at services/interop/crates/x-websearch/src/watch.rs:219-235. |
| <code>acceptance.11.366.3</code> | req | 11366.3 |
| <code>acceptance.11.366.3</code> | scenario | Given a recorded endpoint answering one web request log and one program request log, when the watcher polls and is reopened from its state directory, then poll yields the web request and program_work yields the program request |
| <code>acceptance.11.366.3</code> | test | services/interop/crates/x-websearch/tests/program_watch.rs::the_watcher_journals_program_requests_beside_web_requests (planned) |
| <code>acceptance.11.366.3</code> | qualification | UNRUN |
| <code>req.11366</code> | ac_3 | RequestWatcher::poll (services/interop/crates/x-websearch/src/watch.rs:656) queries eth_getLogs with both the XWebRequested and XWebProgramRequested topics and journals program requests in pending.json keyed by program id and request id, surviving a reopen. |
| <code>acceptance.11.367.1</code> | req | 11367.1 |
| <code>acceptance.11.367.1</code> | scenario | Given the recorded observation bytes and the submitter key, when fulfil_program_calldata and Transaction::sign run, then the calldata and the raw transaction equal the recorded independent encoder's |
| <code>acceptance.11.367.1</code> | test | services/interop/crates/x-websearch/tests/submit.rs::fulfil_program_calldata_matches_an_independent_encoder (planned) |
| <code>acceptance.11.367.1</code> | qualification | UNRUN |
| <code>req.11367</code> | ac_1 | fulfil_program_calldata(observation) in services/interop/crates/x-websearch/src/submit.rs encodes fulfilProgram(bytes) of precompiles/xweb/abi.json:1 with selector("fulfilProgram(bytes)") from services/interop/crates/x-websearch/src/submit.rs:40 and one dynamic bytes argument, and the signed dynamic-fee transaction to 0x1019 matches the recorded independent encoding. |
| <code>acceptance.11.367.2</code> | req | 11367.2 |
| <code>acceptance.11.367.2</code> | scenario | Given a recorded endpoint and a submitter over a scratch journal, when submit_program is called, the submitter is reopened and resume runs, then one transaction hash is broadcast twice with identical bytes and the entry ends completed |
| <code>acceptance.11.367.2</code> | test | services/interop/crates/x-websearch/tests/submit.rs::a_program_fulfil_is_journalled_by_program_and_request (planned) |
| <code>acceptance.11.367.2</code> | qualification | UNRUN |
| <code>req.11367</code> | ac_2 | Submitter::submit_program journals the signed bytes under (program id, request id) before eth_sendRawTransaction, rebroadcasts the same bytes after a restart and records completion from the transaction receipt, as Submitter::submit and confirm do for web requests at services/interop/crates/x-websearch/src/submit.rs:892-990. |
| <code>acceptance.11.368.1</code> | req | 11368.1 |
| <code>acceptance.11.368.1</code> | scenario | Given a recorded endpoint with one program request log, an attestor set of threshold 1 and a fetch site, when the relay steps until idle, then one fulfilProgram transaction is broadcast and the program request is retired from the watcher |
| <code>acceptance.11.368.1</code> | test | services/interop/crates/x-websearch/tests/program.rs::a_program_request_is_attested_exchanged_and_fulfilled (planned) |
| <code>acceptance.11.368.1</code> | qualification | UNRUN |
| <code>req.11368</code> | ac_1 | program::ProgramRelay::step takes program work from RequestWatcher, attests it with ProgramAttestor, records it in ProgramExchange, and once the registered threshold agrees submits fulfil_program_calldata through Submitter::submit_program, the flow KernelRelay::step runs over the gateway at services/interop/crates/x-websearch/src/kernel.rs:1909. |
| <code>acceptance.11.368.2</code> | req | 11368.2 |
| <code>acceptance.11.368.2</code> | scenario | Given a recorded program request whose URL is refused by robots.txt, when the relay steps until idle, then no transaction is broadcast and the discarded list names the request with its refusal code |
| <code>acceptance.11.368.2</code> | test | services/interop/crates/x-websearch/tests/program.rs::a_program_request_refused_by_the_attestor_is_retired_with_its_code (planned) |
| <code>acceptance.11.368.2</code> | qualification | UNRUN |
| <code>req.11368</code> | ac_2 | A program request whose fetch is refused is recorded as discarded with the attestor's refusal code, retired from the watcher and never submitted. |
| <code>acceptance.11.371.1</code> | req | 11371.1 |
| <code>acceptance.11.371.1</code> | scenario | Given the recorded program request, its answer and two attestor signatures, when program::observation_bytes runs, then the bytes equal the recorded observation |
| <code>acceptance.11.371.1</code> | test | services/interop/crates/x-websearch/tests/program.rs::observation_bytes_are_unchanged_by_the_move (planned) |
| <code>acceptance.11.371.1</code> | qualification | UNRUN |
| <code>req.11371</code> | ac_1 | program::observation_bytes in services/interop/crates/x-websearch/src/program.rs (planned) produces the same bytes as kernel::observation_bytes at services/interop/crates/x-websearch/src/kernel.rs:567 for the recorded program request under services/interop/crates/x-websearch/tests/fixtures/kernel. |
| <code>acceptance.11.371.2</code> | req | 11371.2 |
| <code>acceptance.11.371.2</code> | scenario | Given an exchange with two peers and an attestor set of threshold 2, when the local answer and one peer signature for program P request 4 are recorded, then ready returns the two signatures, and a signature for request 5 is not counted |
| <code>acceptance.11.371.2</code> | test | services/interop/crates/x-websearch/tests/program.rs::the_program_exchange_collects_peer_signatures_by_program_and_request (planned) |
| <code>acceptance.11.371.2</code> | qualification | UNRUN |
| <code>req.11371</code> | ac_2 | program::ProgramExchange keeps the behaviour of ProgramExchange at services/interop/crates/x-websearch/src/kernel.rs:1062-1365: it records one answer per (program id, request id), accepts a peer signature only for that key from a registered attestor, and ready returns the threshold set. |
| <code>acceptance.11.401.1</code> | req | 11401.1 |
| <code>acceptance.11.401.1</code> | scenario | Given the binary started with the committed configuration and a loopback EVM endpoint, when GET /readyz is requested, then the program relay role is listed and no kernel role is listed |
| <code>acceptance.11.401.1</code> | test | services/interop/crates/x-websearch/tests/binary.rs::the_binary_runs_the_evm_program_relay_without_a_kernel_section (planned) |
| <code>acceptance.11.401.1</code> | qualification | UNRUN |
| <code>req.11401</code> | ac_1 | main starts the EVM program relay of services/interop/crates/x-websearch/src/program.rs (planned) whenever program requests are configured and no longer reads the kernel section or starts the kernel thread of services/interop/crates/x-websearch/src/main.rs:1130-1146. |
| <code>acceptance.11.401.2</code> | req | 11401.2 |
| <code>acceptance.11.401.2</code> | scenario | Given the binary started with the committed configuration, when GET /readyz is requested, then the body names no role containing kernel |
| <code>acceptance.11.401.2</code> | test | services/interop/crates/x-websearch/tests/binary.rs::readiness_lists_no_kernel_roles (planned) |
| <code>acceptance.11.401.2</code> | qualification | UNRUN |
| <code>req.11401</code> | ac_2 | The readiness roles of services/interop/crates/x-websearch/src/main.rs:57-118 no longer include the kernel watcher, kernel submitter and kernel relay entries, and the readiness body of services/interop/crates/x-websearch/src/main.rs:386-408 is built without them. |
| <code>capability.11.32.1</code> | title | Decode XWebProgramRequested logs of 0x1019 in the request watcher |
| <code>capability.11.32.1</code> | reqs | 11366.1<br>11366.2<br>11366.3 |
| <code>capability.11.32.2</code> | title | Encode and submit fulfilProgram calldata from the EVM submitter |
| <code>capability.11.32.2</code> | reqs | 11367.1<br>11367.2 |
| <code>capability.11.32.3</code> | title | Run the program relay from EVM program requests to fulfilProgram |
| <code>capability.11.32.3</code> | reqs | 11368.1<br>11368.2 |
| <code>capability.11.33.1</code> | title | Move program attestation and the peer exchange into a program module |
| <code>capability.11.33.1</code> | reqs | 11371.1<br>11371.2 |
| <code>capability.11.36.1</code> | title | Delete the x-websearch kernel module, its binary wiring and the kernel tests |
| <code>capability.11.36.1</code> | reqs | 11401.1<br>11401.2 |

## x-websearch-server

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.381.1</code> | req | 11381.1 |
| <code>acceptance.11.381.1</code> | scenario | Given a route table with search, fetch and content handlers that is not serving-eligible, when GET /content/&lt;digest&gt;, GET /search and GET /fetch are dispatched, then content answers its handler's 200 and both paid routes answer 503 service_not_ready |
| <code>acceptance.11.381.1</code> | test | services/interop/crates/x-websearch/tests/server.rs::content_is_served_before_the_sidecar_is_serving_eligible (planned) |
| <code>acceptance.11.381.1</code> | qualification | UNRUN |
| <code>req.11381</code> | ac_1 | RouteTable::dispatch (services/interop/crates/x-websearch/src/server.rs:364) dispatches Route::Content to its handler while is_serving_eligible is false, and keeps answering 503 service_not_ready for /search and /fetch, the gate at services/interop/crates/x-websearch/src/server.rs:380-382. |
| <code>acceptance.11.381.2</code> | req | 11381.2 |
| <code>acceptance.11.381.2</code> | scenario | Given a route table with no content handler, when GET /content/&lt;digest&gt; and GET /content/0xabc are sent over a socket, then the first answers 503 route_unavailable and the second 400 malformed_digest |
| <code>acceptance.11.381.2</code> | test | services/interop/crates/x-websearch/tests/server.rs::a_content_route_without_a_handler_answers_route_unavailable (planned) |
| <code>acceptance.11.381.2</code> | qualification | UNRUN |
| <code>req.11381</code> | ac_2 | A Route::Content request with no handler answers 503 route_unavailable whatever the eligibility, and a malformed digest still answers 400 malformed_digest from route_of at services/interop/crates/x-websearch/src/server.rs:712-718. |
| <code>acceptance.11.382.1</code> | req | 11382.1 |
| <code>acceptance.11.382.1</code> | scenario | Given a running server whose ProgramExchange recorded an attested EVM program request, when a peer requests its program attestation route, then the answer is 200 with the observation hex and a signature that recovers to the attestor address |
| <code>acceptance.11.382.1</code> | test | services/interop/crates/x-websearch/tests/program_server.rs::a_peer_reads_the_attestation_of_an_evm_program_request (planned) |
| <code>acceptance.11.382.1</code> | qualification | UNRUN |
| <code>req.11382</code> | ac_1 | program::register registers RouteTable::set_program_attestations (services/interop/crates/x-websearch/src/server.rs:330-339) with a handler that answers GET /program-attestations/&lt;program id&gt;/&lt;request id&gt; with the local ProgramExchange record of an EVM program request: the observation bytes and the attestor signature. |
| <code>acceptance.11.382.2</code> | req | 11382.2 |
| <code>acceptance.11.382.2</code> | scenario | Given the same server, when the route is requested for an unrecorded request and for a 63-digit program id, then the answers are 404 not_found and 400 malformed_program_request |
| <code>acceptance.11.382.2</code> | test | services/interop/crates/x-websearch/tests/program_server.rs::an_unknown_program_request_answers_not_found (planned) |
| <code>acceptance.11.382.2</code> | qualification | UNRUN |
| <code>req.11382</code> | ac_2 | The handler answers 404 not_found for a program id and request id the exchange has not recorded, and the route answers 400 malformed_program_request for a program id that is not 64 lowercase hex digits (services/interop/crates/x-websearch/src/server.rs:754-770). |
| <code>capability.11.35.1</code> | title | Exempt GET /content from the serving-eligibility gate |
| <code>capability.11.35.1</code> | reqs | 11381.1<br>11381.2 |
| <code>capability.11.35.2</code> | title | Serve /program-attestations from the program exchange of the EVM relay |
| <code>capability.11.35.2</code> | reqs | 11382.1<br>11382.2 |
