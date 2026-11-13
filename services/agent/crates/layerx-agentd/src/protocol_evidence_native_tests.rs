use super::{RawReceiptEvidence, VerifiedReceiptEvidence};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{AuthorizedBatch, MaintainedOutcomeEvidence};

fn checked<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("native terminal evidence: {error:?}"))
}

fn custody_fixture(directory: &str) -> std::path::PathBuf {
    std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/custody")
        .join(directory)
}

#[test]
fn daemon_terminal_fixture_carries_the_credit_and_maintenance_the_platform_writes() {
    let root = custody_fixture("daemon-light-credit-receipt");
    let vector = custody_fixture("paxeer-light-v1");
    let read = |path: &std::path::Path, name: &str| checked(std::fs::read(path.join(name)));
    let credit = read(&root, "credit");
    let profile = read(&root, "profile");
    assert_eq!(credit, read(&vector, "custody.credit"));
    assert_eq!(profile, read(&vector, "custody.profile"));
    assert_eq!(&credit[..5], b"LXDC3");
    assert_eq!(&profile[..5], b"LXBC3");
    let kind = checked(layerx_types::payload::ActivityType::new(
        layerx_types::payload::ModuleId::Bridge,
        1,
    ));
    let registration = checked(layerx_types::payload::ModuleRegistration::new(
        layerx_types::payload::ModuleId::Bridge,
        &[kind],
    ));
    let registry = checked(layerx_types::payload::ModuleRegistry::new(&[registration]));
    let activity = checked(layerx_wire::historical::activity::decode_signed(
        &read(&root, "activity"),
        &registry,
    ));
    assert_eq!(activity.payload(), credit.as_slice());
    let (prepared, _, _, _, _) = native_fixture();
    checked(crate::prepare::verify_call_binding(&prepared.transaction));
    assert_eq!(
        prepared.transaction.disclosure,
        checked(layerx_crypto::disclosure::call::bind_call(
            &prepared.transaction.call,
            prepared.actor_address,
        )),
    );
    assert!(matches!(
        checked(layerx_wire::batch_maintenance::decode_maintenance(&read(
            &root,
            "maintenance.receipt"
        ))),
        layerx_wire::batch_maintenance::MaintenanceReceipt::Batch(_)
    ));
}

#[test]
fn actual_daemon_terminal_requires_the_complete_maintained_transition() {
    let root = custody_fixture("daemon-light-credit-receipt");
    let read = |name: &str| checked(std::fs::read(root.join(name)));
    let header_bytes = read("header");
    let header = checked(layerx_wire::receipt::decode_batch_header(&header_bytes));
    let receipt = read("credit.receipt");
    let decoded = checked(layerx_wire::receipt::decode(&receipt));
    let protocol = decoded
        .protocol()
        .unwrap_or_else(|| panic!("protocol credit"));
    let public = checked(read("sequencer.public").try_into());
    let authority = AuthorizedBatch::new(
        protocol.batch_id(),
        [0; 32],
        header.previous_state_root(),
        protocol.resulting_state_root(),
        public,
    );
    let raw = RawReceiptEvidence::new(
        receipt,
        checked(layerx_proof::merkle::decode_proof(&read("receipt.proof"))),
        header_bytes,
        checked(read("header.signature").try_into()),
    );
    let maintenance = read("maintenance.receipt");
    let proof = checked(layerx_proof::merkle::decode_proof(&read(
        "maintenance.proof",
    )));
    let authorization = SequencerAuthorization::new(header.sequencer_id(), public, 1, 1);
    let signature = raw.header_signature();
    let evidence = MaintainedOutcomeEvidence {
        header: raw.canonical_header(),
        header_signature: &signature,
        activity_proof: raw.proof(),
        maintenance: &maintenance,
        maintenance_proof: &proof,
        authorization: &authorization,
    };
    let terminal = checked(VerifiedReceiptEvidence::verify_authorized_maintained(
        &raw,
        &authority,
        &evidence,
        &[raw.canonical_receipt().to_vec()],
        header.protocol_version(),
        header.network_id(),
    ));
    let unsigned = checked(layerx_wire::receipt::encode_unsigned(&decoded));
    let mut preimage = b"LXP/v1/receipt\0".to_vec();
    preimage.extend_from_slice(&unsigned);
    let expected_reference: [u8; 32] = <sha2::Sha256 as sha2::Digest>::digest(&preimage).into();
    assert_eq!(terminal.receipt_ref(), expected_reference);
    assert_eq!(terminal.activity_id(), protocol.activity_id());
    assert_eq!(terminal.result_code(), 0);
    prove_terminal_recovery();
    assert_eq!(terminal.global_sequence(), 1);
    assert_eq!(
        terminal.level(),
        layerx_types::verify::VerificationLevel::BATCH_INCLUDED
    );
    assert_terminal_refusals(&raw, authority, evidence, &header, &maintenance, public);
}
fn assert_terminal_refusals(
    raw: &RawReceiptEvidence,
    authority: AuthorizedBatch,
    evidence: MaintainedOutcomeEvidence<'_>,
    header: &layerx_wire::receipt::BatchHeader,
    maintenance: &[u8],
    public: [u8; 32],
) {
    // Exercise the public ingress without maintained-transition evidence; the
    // terminal receipt alone cannot authenticate the complete batch transition.
    assert!(checked(
        crate::protocol_evidence::EvidenceAuthority::pinned_to_handshake(
            header.protocol_version(),
            header.network_id(),
            public,
        )
    )
    .verify_receipt(raw)
    .is_err());
    let wrong = replace_terminal_root(authority, header.resulting_state_root());
    assert!(VerifiedReceiptEvidence::verify_authorized_maintained(
        raw,
        &wrong,
        &evidence,
        &[raw.canonical_receipt().to_vec()],
        header.protocol_version(),
        header.network_id()
    )
    .is_err());
    assert!(VerifiedReceiptEvidence::verify_authorized_maintained(
        raw,
        &authority,
        &evidence,
        &[raw.canonical_receipt().to_vec()],
        header.protocol_version(),
        header.network_id() + 1
    )
    .is_err());
    let mut corrupted = maintenance.to_vec();
    corrupted[50] ^= 1;
    let bad_evidence = MaintainedOutcomeEvidence {
        maintenance: &corrupted,
        ..evidence
    };
    assert!(VerifiedReceiptEvidence::verify_authorized_maintained(
        raw,
        &authority,
        &bad_evidence,
        &[raw.canonical_receipt().to_vec()],
        header.protocol_version(),
        header.network_id()
    )
    .is_err());
}

/// The node recorder uses this public test key and emits both the type-2 call
/// and its sequencer receipt. Reproduce the transaction through preparation and
/// signing, then require byte equality with that committed producer transcript.
pub(super) fn native_fixture() -> (
    crate::sign::SigningPreparation,
    layerx_types::payload::ModuleRegistry,
    Vec<u8>,
    serde_json::Value,
    Vec<layerx_client::sequencer::ReceiptKeyEntry>,
) {
    use crate::prepare::{
        evm::{AbiArgument, PrecompileCall, PrecompileTarget},
        prepare_bound, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest, EvmPrepared,
    };
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    let document: serde_json::Value = checked(serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))));
    let Some(cases) = document["cases"].as_array() else {
        panic!("recorded cases");
    };
    let Some(case) = cases
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
    else {
        panic!("native call recording");
    };
    let raw = decode_hex(checked(
        case["rawTransaction"].as_str().ok_or("raw transaction"),
    ));
    let (call, signature) = checked(decode_signed(&raw));
    let actor_address = checked(recover_sender(&call, &signature));
    assert_eq!(
        &call.data[..4],
        &layerx_crypto::disclosure::call::ledger_selector(
            "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)"
        )
    );
    let transaction: EvmPrepared = checked(prepare_bound(
        EvmPreparationState {
            chain_id: 125,
            pending_nonce: 2,
            base_fee_per_gas: 1_000_000_000,
            head_height: 1001,
        },
        EvmPreparationDefaults {
            max_priority_fee_per_gas: 1_000_000_000,
            max_fee_per_gas: 3_000_000_000,
            maximum_payload_bytes: 2048,
        },
        EvmPrepareRequest {
            from: actor_address,
            call: PrecompileCall {
                target: PrecompileTarget::Ledger,
                selector: checked(call.data[..4].try_into()),
                arguments: call.data[4..]
                    .chunks_exact(32)
                    .map(|word| AbiArgument::Word(checked(word.try_into())))
                    .collect(),
            },
            expected_nonce: Some(2),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 2048,
        },
        call.gas_limit,
    ));
    assert_eq!(transaction.call, call);
    let prepared = crate::sign::SigningPreparation {
        transaction,
        actor_address,
    };
    let seed: [u8; 32] = checked(
        decode_hex("8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63").try_into(),
    );
    let signed = checked(prepared.transaction.call.sign(&seed));
    assert_eq!(
        signed, raw,
        "producer must reproduce the node recorder's exact bytes"
    );
    let kind = checked(ActivityType::new(ModuleId::Asset, 5));
    let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Asset,
        &[kind],
    ))]));
    let pin = &case["pinnedKey"];
    let keys = vec![layerx_client::sequencer::ReceiptKeyEntry {
        key_id: checked(decode_hex(checked(pin["keyId"].as_str().ok_or("key id"))).try_into()),
        public_key: checked(
            decode_hex(checked(pin["publicKey"].as_str().ok_or("public key"))).try_into(),
        ),
        first_height: checked(pin["firstHeight"].as_u64().ok_or("first height")),
        last_height: checked(pin["lastHeight"].as_u64().ok_or("last height")),
    }];
    (
        prepared,
        registry,
        signed,
        case["receiptJson"].clone(),
        keys,
    )
}

pub(super) fn decode_hex(value: &str) -> Vec<u8> {
    checked(hex::decode(value.strip_prefix("0x").unwrap_or(value)))
}

#[test]
fn native_type2_bytes_and_matching_node_receipt_survive_restart() {
    prove_terminal_recovery();
}

#[test]
fn receipt_reference_survives_terminal_recovery() {
    actual_daemon_terminal_requires_the_complete_maintained_transition();
}

fn track_terminal(
    hash: [u8; 32],
    keys: &[layerx_client::sequencer::ReceiptKeyEntry],
    status: &crate::outbox::SubmissionStatus,
    canonical: &[u8],
) -> (serde_json::Value, layerx_types::rung::Rung) {
    let verified = checked(crate::finality::VerifiedReceipt::verify(
        hash, canonical, keys,
    ));
    let response = checked(crate::human_runtime::write_submission(
        status,
        Some(&verified),
    ));
    checked(crate::agent_rpc_dispatch::decode_write_answer(
        layerx_agent_api::error::RequestId(1684),
        response,
    ))
}

fn spawn_terminal_server(
    listener: std::net::TcpListener,
    expected_signed: Vec<u8>,
    receipt_view: serde_json::Value,
    hash: [u8; 32],
) -> std::thread::JoinHandle<()> {
    use std::io::{BufRead, BufReader, Read, Write};
    std::thread::spawn(move || {
        for expected_method in [
            "eth_sendRawTransaction",
            "pax_sequencerReceipt",
            "eth_getTransactionReceipt",
        ] {
            let (mut stream, _) = checked(listener.accept());
            checked(stream.set_read_timeout(Some(std::time::Duration::from_secs(5))));
            let mut reader = BufReader::new(&mut stream);
            let mut line = String::new();
            checked(reader.read_line(&mut line));
            assert!(line.starts_with("POST / HTTP/1.1"));
            let mut length = None;
            loop {
                line.clear();
                checked(reader.read_line(&mut line));
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        length = Some(checked(value.trim().parse::<usize>()));
                    }
                }
            }
            let mut body = vec![0; checked(length.ok_or("request length"))];
            checked(reader.read_exact(&mut body));
            let request: serde_json::Value = checked(serde_json::from_slice(&body));
            assert_eq!(request["method"], expected_method);
            let result = if expected_method == "eth_sendRawTransaction" {
                assert_eq!(
                    request["params"][0],
                    format!("0x{}", hex::encode(&expected_signed))
                );
                serde_json::json!(format!("0x{}", hex::encode(hash)))
            } else {
                assert_eq!(request["params"][0], format!("0x{}", hex::encode(hash)));
                if expected_method == "pax_sequencerReceipt" {
                    receipt_view.clone()
                } else {
                    serde_json::Value::Null
                }
            };
            let body = checked(serde_json::to_vec(
                &serde_json::json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
            ));
            checked(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()));
            checked(stream.write_all(&body));
        }
    })
}

fn mark_submission_unknown(
    outbox: &mut crate::outbox::Outbox,
    store: &mut crate::store::Store,
    id: [u8; 32],
) {
    use crate::outbox::SubmissionState;
    checked(outbox.transition(
        store,
        id,
        SubmissionState::Submitted,
        "sent exact native transaction",
        None,
    ));
    checked(outbox.transition(
        store,
        id,
        SubmissionState::Unknown,
        "restart before receipt resolution",
        None,
    ));
}

fn prove_terminal_recovery() {
    use crate::outbox::{resolve_unknown, Outbox, ResolutionObservation, SubmissionState};
    use crate::store::{Store, TenantId};
    use layerx_client::sequencer::SequencerEndpoint;
    use layerx_crypto::evm_transaction::transaction_hash;
    use sha2::{Digest, Sha256};
    static NEXT: std::sync::atomic::AtomicU64 = std::sync::atomic::AtomicU64::new(0);
    let (prepared, registry, signed, receipt_view, keys) = native_fixture();
    let verified = checked(crate::sign::verify_before_submit(
        &signed,
        &prepared,
        &prepared.actor_address,
        &registry,
    ));
    let hash = transaction_hash(&signed);
    assert_eq!(verified.activity_id(), hash);
    assert_eq!(format!("0x{}", hex::encode(hash)), receipt_view["txHash"]);
    let canonical_receipt = decode_hex(checked(
        receipt_view["receipt"].as_str().ok_or("node receipt"),
    ));
    let receipt_hash: [u8; 32] = Sha256::digest(&canonical_receipt).into();
    let decoded = checked(layerx_wire::receipt::decode(&canonical_receipt));
    let unsigned = checked(layerx_wire::receipt::encode_unsigned(&decoded));
    let mut preimage = b"LXP/v1/receipt\0".to_vec();
    preimage.extend_from_slice(&unsigned);
    let receipt_reference: [u8; 32] = Sha256::digest(&preimage).into();
    assert_eq!(
        checked(super::canonical_receipt_reference(&canonical_receipt)),
        receipt_reference
    );
    assert_ne!(receipt_reference, receipt_hash);
    let listener = checked(std::net::TcpListener::bind("127.0.0.1:0"));
    let mut endpoint = checked(SequencerEndpoint::new(
        &format!("http://{}", checked(listener.local_addr())),
        &[],
    ));
    let server = spawn_terminal_server(listener, signed.clone(), receipt_view, hash);
    let root = std::env::temp_dir().join(format!(
        "native-terminal-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, std::sync::atomic::Ordering::Relaxed)
    ));
    let tenant = checked(TenantId::new("native-terminal"));
    let mut store = checked(Store::open(&root));
    let mut outbox = Outbox::default();
    checked(prepared.retain(&mut store, tenant.clone(), hash));
    let id = checked(outbox.enqueue_verified(&mut store, tenant.clone(), verified));
    assert_eq!(id, hash);
    assert_eq!(checked(outbox.bytes_for_transmission(id)), signed);
    assert_eq!(
        checked(endpoint.send_raw_transaction(checked(outbox.bytes_for_transmission(id)))),
        hash
    );
    mark_submission_unknown(&mut outbox, &mut store, id);
    drop(outbox);
    drop(store);
    let mut store = checked(Store::open(&root));
    let restored_preparation = checked(crate::sign::SigningPreparation::restore(
        &store,
        tenant.clone(),
        hash,
    ));
    assert_eq!(restored_preparation, prepared);
    let mut outbox = Outbox::default();
    checked(outbox.restore(&store, tenant.clone(), hash));
    assert_eq!(checked(outbox.exact_signed_bytes(hash)), signed);
    let result = checked(resolve_unknown(
        &mut outbox,
        &mut store,
        hash,
        100,
        &keys,
        &mut endpoint,
    ));
    assert_eq!(result.observation, ResolutionObservation::ExecutedReceipt);
    assert_eq!(result.state, SubmissionState::Executed);
    checked(server.join());
    let before_reopen = track_terminal(
        hash,
        &keys,
        checked(outbox.status(hash).ok_or("terminal before reopen")),
        &canonical_receipt,
    );
    assert_eq!(
        before_reopen.0["receipt_ref"],
        hex::encode(receipt_reference)
    );
    assert_eq!(
        before_reopen.0["evidence"][0]["digest"],
        hex::encode(receipt_reference)
    );
    assert_recovered_terminal(
        (store, outbox, root, tenant),
        (hash, signed, receipt_hash, receipt_reference),
        &keys,
        &canonical_receipt,
        &before_reopen,
    );
}
fn assert_recovered_terminal(
    resources: (
        crate::store::Store,
        crate::outbox::Outbox,
        std::path::PathBuf,
        crate::store::TenantId,
    ),
    identity: ([u8; 32], Vec<u8>, [u8; 32], [u8; 32]),
    keys: &[layerx_client::sequencer::ReceiptKeyEntry],
    canonical_receipt: &[u8],
    before_reopen: &(serde_json::Value, layerx_types::rung::Rung),
) {
    use crate::outbox::{Outbox, SubmissionState};
    use crate::store::{ObjectKind, Store, TenantKey};
    let (store, outbox, root, tenant) = resources;
    let (hash, signed, receipt_hash, receipt_reference) = identity;
    drop(outbox);
    drop(store);
    let store = checked(Store::open(&root));
    let mut recovered = Outbox::default();
    checked(recovered.restore(&store, tenant.clone(), hash));
    let status = checked(recovered.status(hash).ok_or("recovered native execution"));
    assert_eq!(status.state, SubmissionState::Executed);
    let evidence = checked(status.evidence.ok_or("authenticated node receipt"));
    assert_eq!(evidence.receipt_ref(), receipt_hash);
    assert_eq!(evidence.block_height(), 1001);
    assert_eq!(evidence.transaction_index(), 1);
    let receipt_key = checked(TenantKey::new(
        tenant,
        ObjectKind::Receipt,
        receipt_hash.to_vec(),
    ));
    assert_eq!(
        checked(store.get(&receipt_key).ok_or("durable receipt")).bytes(),
        canonical_receipt
    );
    let retained = checked(store.get(&receipt_key).ok_or("retained signed receipt"));
    let after_reopen = track_terminal(hash, keys, status, retained.bytes());
    assert_eq!(&after_reopen, before_reopen);
    assert_eq!(
        after_reopen.0["receipt_ref"],
        hex::encode(receipt_reference)
    );
    assert_eq!(
        checked(super::canonical_receipt_reference(retained.bytes())),
        receipt_reference
    );
    assert_eq!(checked(recovered.exact_signed_bytes(hash)), signed);
    assert!(recovered.bytes_for_transmission(hash).is_err());
    drop(store);
    checked(std::fs::remove_dir_all(root));
}

fn replace_terminal_root(authority: AuthorizedBatch, root: [u8; 32]) -> AuthorizedBatch {
    AuthorizedBatch::new(
        authority.batch_id(),
        authority.asset(),
        authority.previous_state_root(),
        root,
        authority.sequencer_public_key(),
    )
}

#[test]
fn real_native_multi_call_receipts_preserve_each_authenticated_transition() {
    use layerx_proof::merkle::build_proof;
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/programs/maintained-multicall");
    let read = |name: &str| checked(std::fs::read(root.join(name)));
    let header_bytes = read("header");
    let header = checked(layerx_wire::receipt::decode_batch_header(&header_bytes));
    let public = checked(read("sequencer.public").try_into());
    let signature = checked(read("header.signature").try_into());
    let receipts = vec![read("receipt-0"), read("receipt-1")];
    let maintenance = read("maintenance.receipt");
    let leaves = [receipts[0].as_slice(), receipts[1].as_slice(), &maintenance];
    let (maintenance_proof, committed_root) = checked(build_proof(&leaves, 2));
    assert_eq!(committed_root, header.receipt_merkle_root());
    assert_eq!(header.last_sequence() - header.first_sequence(), 2);
    let authorization = SequencerAuthorization::new(
        header.sequencer_id(),
        public,
        header.batch_number(),
        header.batch_number(),
    );
    for (index, bytes) in receipts.iter().enumerate() {
        let decoded = checked(layerx_wire::receipt::decode(bytes));
        let protocol = checked(decoded.protocol().ok_or("native Programs receipt"));
        assert_eq!(protocol.module_id(), 9);
        assert_eq!(protocol.operation(), 3);
        assert_eq!(protocol.result_code(), 0);
        let activity_authority = AuthorizedBatch::new(
            protocol.batch_id(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
            public,
        );
        let raw = RawReceiptEvidence::new(
            bytes.clone(),
            checked(build_proof(&leaves, index)).0,
            header_bytes.clone(),
            signature,
        );
        let evidence = MaintainedOutcomeEvidence {
            header: raw.canonical_header(),
            header_signature: &signature,
            activity_proof: raw.proof(),
            maintenance: &maintenance,
            maintenance_proof: &maintenance_proof,
            authorization: &authorization,
        };
        let verified = checked(VerifiedReceiptEvidence::verify_authorized_maintained(
            &raw,
            &activity_authority,
            &evidence,
            &receipts,
            header.protocol_version(),
            header.network_id(),
        ));
        assert_eq!(verified.activity_id(), protocol.activity_id());
        assert_eq!(verified.global_sequence(), protocol.global_sequence());
        assert_eq!(verified.result_code(), 0);
        if index == 1 {
            assert_ne!(
                activity_authority.previous_state_root(),
                header.previous_state_root()
            );
        }
        assert!(VerifiedReceiptEvidence::verify_authorized_maintained(
            &raw,
            &activity_authority,
            &evidence,
            &receipts[..1],
            header.protocol_version(),
            header.network_id(),
        )
        .is_err());
        let reversed = vec![receipts[1].clone(), receipts[0].clone()];
        assert!(VerifiedReceiptEvidence::verify_authorized_maintained(
            &raw,
            &activity_authority,
            &evidence,
            &reversed,
            header.protocol_version(),
            header.network_id(),
        )
        .is_err());
    }
}
