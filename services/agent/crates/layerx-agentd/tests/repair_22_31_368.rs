//! The in-scope repair record maps audit findings to these production-path cases.
#[path = "support/send_authorization.rs"]
mod send_authorization;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::finality::{
    self, BlockCommit, CommitSignature, FinalityError, SubmissionProgress, Validator,
    VerificationProgress, VerifiedBlock, VerifiedReceipt,
};
use layerx_agentd::outbox::Outbox;
use layerx_agentd::prepare::{
    prepare_bound, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest, PrecompileCall,
};
use layerx_agentd::read::{validator_set_hash, CheckpointHeader, CommitTimestamp, CommitValidator};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::shutdown::{graceful, DaemonLifecycle};
use layerx_agentd::sign::{
    attach_external_signature, self_sign, verify_before_submit, SigningError, SigningPreparation,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerBlock, SequencerEndpoint};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::session::{
    issue_session_key, IssuedSessionKey, SessionKeyRequest, SessionPurpose,
};
use layerx_programs::chain_proof::merkle::hash_from_byte_slices;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use layerx_wire::encode::Encoder;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::future::Future;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{MetadataExt, PermissionsExt};
use std::path::PathBuf;
use std::pin::pin;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll, Wake, Waker};

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}
static NEXT: AtomicU64 = AtomicU64::new(1);
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-368-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&path));
        must(fs::set_permissions(
            &path,
            fs::Permissions::from_mode(0o700),
        ));
        Self(path)
    }
    fn registry(&self) -> SessionKeyRegistry {
        must(SessionKeyRegistry::open(
            self.0.clone(),
            vec![0x5a; 32],
            17,
            must(fs::metadata(&self.0)).uid(),
        ))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}
fn tenant() -> TenantId {
    must(TenantId::new("repair-368"))
}
struct ReadyWake;
impl Wake for ReadyWake {
    fn wake(self: Arc<Self>) {}
}
fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(ReadyWake));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local signer blocked"),
    }
}
fn send_type() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 5).unwrap_or_else(|error| panic!("activity: {error:?}"))
}

fn registry() -> ModuleRegistry {
    ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &[send_type()])
        .unwrap_or_else(|error| panic!("registration: {error:?}"))])
    .unwrap_or_else(|error| panic!("registry: {error:?}"))
}

fn send_payload() -> Vec<u8> {
    let mut encoder = Encoder::new(512);
    for result in [encoder.u16(0x5301), encoder.u16(10)] {
        result.unwrap_or_else(|error| panic!("header: {error:?}"));
    }
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("from: {error:?}"));
    encoder
        .fixed(&[0x22; 32])
        .unwrap_or_else(|error| panic!("to: {error:?}"));
    encoder
        .fixed(&[0x33; 32])
        .unwrap_or_else(|error| panic!("asset: {error:?}"));
    encoder
        .u128(25)
        .unwrap_or_else(|error| panic!("amount: {error:?}"));
    encoder
        .u64(5)
        .unwrap_or_else(|error| panic!("sequence: {error:?}"));
    encoder
        .fixed(&[4; 32])
        .unwrap_or_else(|error| panic!("idempotency: {error:?}"));
    encoder
        .u64(1_010)
        .unwrap_or_else(|error| panic!("expiry: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("context: {error:?}"));
    encoder
        .u8(0)
        .unwrap_or_else(|error| panic!("conditions: {error:?}"));
    encoder
        .u8(1)
        .unwrap_or_else(|error| panic!("authority kind: {error:?}"));
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("controller: {error:?}"));
    encoder
        .fixed(&[0x66; 32])
        .unwrap_or_else(|error| panic!("payload key: {error:?}"));
    encoder
        .fixed(&[0x77; 64])
        .unwrap_or_else(|error| panic!("payload signature: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("signed context: {error:?}"));
    encoder
        .u32(17)
        .unwrap_or_else(|error| panic!("network: {error:?}"));
    encoder
        .u16(layerx_wire::limits::PROTOCOL_VERSION)
        .unwrap_or_else(|error| panic!("version: {error:?}"));
    send_authorization::sign(encoder.finish())
}

fn preparation(address: [u8; 20]) -> SigningPreparation {
    let transaction = must(prepare_bound(
        EvmPreparationState {
            chain_id: 17,
            pending_nonce: 5,
            base_fee_per_gas: 3,
            head_height: 88,
        },
        EvmPreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 7,
            maximum_payload_bytes: 1024,
        },
        EvmPrepareRequest {
            from: address,
            call: PrecompileCall::ledger_transfer(send_payload()),
            expected_nonce: Some(5),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1024,
        },
        100_000,
    ));
    SigningPreparation {
        transaction,
        actor_address: address,
    }
}
fn issue(address: [u8; 20], permitted: ActivityType) -> IssuedSessionKey {
    let mut word = [0; 32];
    word[12..].copy_from_slice(&address);
    must(issue_session_key(&SessionKeyRequest {
        grantor: [1; 32],
        session_public_key: word,
        not_before: 900,
        expires_at: Some(1100),
        permitted_activity_types: vec![permitted],
        revocation_sequence: Some(5),
        fee_budget: None,
        purpose: SessionPurpose::Activity,
    }))
}

#[test]
fn finding_9_provision_reopen_self_sign_scope_and_durable_revocation() {
    let directory = Directory::new();
    let registry_store = directory.registry();
    let seed = must(registry_store.prepare_seed(b"session", [1; 32]));
    let address = seed.evm_address();
    let issued = issue(address, send_type());
    must(registry_store.provision(issued.grant_id, seed.secret(), &issued));
    let mut changed_scope = issued.clone();
    changed_scope.permitted_activity_types = vec![must(ActivityType::new(ModuleId::Asset, 6))];
    assert!(registry_store.load(issued.grant_id, changed_scope).is_err());
    let mut changed_authority = issued.clone();
    changed_authority.authority = must(layerx_types::activity::Authority::owner(&[2; 32]));
    assert!(registry_store
        .load(issued.grant_id, changed_authority)
        .is_err());
    let mut changed_address = issued.clone();
    changed_address.session_public_key[31] ^= 1;
    assert!(registry_store
        .load(issued.grant_id, changed_address)
        .is_err());
    drop(seed);
    drop(registry_store);
    let reopened = directory.registry();
    let key = must(reopened.load(issued.grant_id, issued.clone()));
    let prepared = preparation(address);
    let signed = must(ready(self_sign(
        Some(&key),
        &prepared,
        &registry(),
        1000,
        5,
    )));
    let (call, signature) = must(decode_signed(&signed.signed_canonical_bytes));
    assert_eq!(signature.as_bytes().len(), 65);
    assert_eq!(must(recover_sender(&call, &signature)), address);
    assert_eq!(signed.audit.session_public_key, issued.session_public_key);
    assert_eq!(signed.audit.authority_bytes, issued.authority.as_bytes());
    assert_eq!(
        must(verify_before_submit(
            &signed.signed_canonical_bytes,
            &prepared,
            &address,
            &registry()
        ))
        .exact_bytes(),
        signed.signed_canonical_bytes
    );
    assert_eq!(
        ready(self_sign(Some(&key), &prepared, &registry(), 899, 5)),
        Err(SigningError::Expired)
    );
    assert_eq!(
        ready(self_sign(Some(&key), &prepared, &registry(), 1100, 5)),
        Err(SigningError::Expired)
    );
    assert_eq!(
        ready(self_sign(Some(&key), &prepared, &registry(), 1000, 6)),
        Err(SigningError::Revoked)
    );
    // A separate real, persisted grant cannot exercise an activity outside its exact scope.
    let other_seed = must(reopened.prepare_seed(b"other-scope", [2; 32]));
    let other = issue(
        other_seed.evm_address(),
        must(ActivityType::new(ModuleId::Asset, 6)),
    );
    must(reopened.provision(other.grant_id, other_seed.secret(), &other));
    let other_key = must(reopened.load(other.grant_id, other));
    assert_eq!(
        ready(self_sign(
            Some(&other_key),
            &preparation(other_key.evm_address()),
            &registry(),
            1000,
            5
        )),
        Err(SigningError::ScopeDenied)
    );
    must(reopened.revoke(issued.grant_id));
    assert_eq!(
        ready(self_sign(Some(&key), &prepared, &registry(), 1000, 5)),
        Err(SigningError::Revoked)
    );
    drop(reopened);
    assert!(directory.registry().load(issued.grant_id, issued).is_err());
    assert!(key.with_secret(|_| ()).is_err());
}

#[test]
fn finding_8_shutdown_batches_append_across_two_reopen_cycles() {
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let mut lifecycle = DaemonLifecycle::running();
    must(lifecycle.append_audit(b"first-a".to_vec()));
    must(lifecycle.append_audit(b"first-b".to_vec()));
    assert_eq!(
        must(graceful(
            &mut store,
            tenant(),
            &Outbox::default(),
            &mut lifecycle
        ))
        .audit_entries_flushed,
        2
    );
    drop(lifecycle);
    drop(store);
    let mut store = must(Store::open(&directory.0));
    let mut lifecycle = DaemonLifecycle::running();
    must(lifecycle.append_audit(b"second".to_vec()));
    assert_eq!(
        must(graceful(
            &mut store,
            tenant(),
            &Outbox::default(),
            &mut lifecycle
        ))
        .audit_entries_flushed,
        1
    );
    drop(lifecycle);
    drop(store);
    let store = must(Store::open(&directory.0));
    let ids = store.list_object_ids(&tenant(), ObjectKind::Audit);
    assert_eq!(ids.len(), 3);
    let records: Vec<_> = ids
        .iter()
        .enumerate()
        .map(|(index, id)| {
            let mut expected = b"shutdown-audit:".to_vec();
            expected.extend((index as u64).to_be_bytes());
            assert_eq!(id, &expected);
            store
                .get(&must(TenantKey::new(
                    tenant(),
                    ObjectKind::Audit,
                    id.clone(),
                )))
                .expect("durable audit")
                .bytes()
                .to_vec()
        })
        .collect();
    assert_eq!(
        records,
        vec![b"first-a".to_vec(), b"first-b".to_vec(), b"second".to_vec()]
    );
}

fn unhex(value: &Value) -> Vec<u8> {
    let text = value.as_str().expect("hex string");
    must(hex::decode(text.strip_prefix("0x").unwrap_or(text)))
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    must(unhex(value).try_into())
}
fn node_case() -> Value {
    let producer: Value = must(serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    producer["cases"]
        .as_array()
        .expect("node cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .expect("node-produced receipt")
        .clone()
}
fn receipt_key(case: &Value) -> ReceiptKeyEntry {
    let key = &case["pinnedKey"];
    ReceiptKeyEntry {
        key_id: fixed(&key["keyId"]),
        public_key: fixed(&key["publicKey"]),
        first_height: key["firstHeight"].as_u64().expect("height"),
        last_height: key["lastHeight"].as_u64().expect("height"),
    }
}
fn committed_block(
    transactions: Vec<Vec<u8>>,
    height: u64,
    salt: u8,
) -> (
    SequencerBlock,
    BlockCommit,
    CheckpointHeader,
    Vec<Validator>,
) {
    let signer = SigningKey::from_bytes(&[50; 32]);
    let public_key = signer.verifying_key().to_bytes();
    let validator = CommitValidator {
        address: finality::validator_address(&public_key),
        public_key,
        voting_power: 1,
    };
    let hashes: Vec<Vec<u8>> = transactions
        .iter()
        .map(|raw| Sha256::digest(raw).to_vec())
        .collect();
    let root = hash_from_byte_slices(&hashes.iter().map(Vec::as_slice).collect::<Vec<_>>());
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".to_owned(),
        height,
        time: CommitTimestamp {
            seconds: 1000,
            nanos: 0,
        },
        last_block_id: None,
        last_commit_hash: None,
        data_hash: if transactions.is_empty() {
            None
        } else {
            Some(root)
        },
        validators_hash: validator_set_hash(&[validator]),
        next_validators_hash: validator_set_hash(&[validator]),
        consensus_hash: [salt; 32],
        app_hash: [5; 32],
        last_results_hash: None,
        evidence_hash: None,
        proposer_address: validator.address,
    };
    let block = SequencerBlock {
        number: height,
        hash: header.block_hash(),
        parent_hash: [2; 32],
        timestamp: 1000,
        transactions_root: root,
        receipts_root: [3; 32],
        transactions,
    };
    let mut commit = BlockCommit {
        height,
        round: 0,
        block_hash: block.hash,
        part_set_total: 1,
        part_set_hash: [9; 32],
        signatures: Vec::new(),
    };
    let vote = must(commit.vote_sign_bytes("125", 1000, 0));
    commit.signatures.push(CommitSignature {
        validator_address: validator.address,
        timestamp_seconds: 1000,
        timestamp_nanos: 0,
        signature: signer.sign(&vote).to_bytes(),
    });
    (
        block,
        commit,
        header,
        vec![Validator {
            public_key,
            voting_power: 1,
        }],
    )
}

#[test]
fn finding_6_authenticated_transaction_binding_survives_restart_and_refuses_foreign_blocks() {
    let case = node_case();
    let raw = unhex(&case["rawTransaction"]);
    let receipt_bytes = unhex(&case["signedReceipt"]);
    let hash = transaction_hash(&raw);
    assert_eq!(hash, fixed(&case["receiptJson"]["txHash"]));
    let key = receipt_key(&case);
    let receipt = must(VerifiedReceipt::verify(hash, &receipt_bytes, &[key]));
    let height = u64::from_str_radix(
        case["receiptJson"]["height"]
            .as_str()
            .expect("height")
            .trim_start_matches("0x"),
        16,
    )
    .expect("height");
    let (block, commit, header, validators) = committed_block(vec![raw.clone()], height, 4);
    let verified = must(VerifiedBlock::verify_with_header(
        &block,
        &commit,
        &header,
        "125",
        &validators,
    ));
    let mut progress = SubmissionProgress::accepted([1; 32], hash);
    must(progress.observe_receipt(&receipt_bytes, &[key]));
    assert!(matches!(
        progress.observe_block(&block, &commit, "125", &validators),
        Err(FinalityError::InvalidCommit)
    ));
    assert_eq!(must(progress.rung_at([1; 32], 0)), Rung::Receipted);
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let vote_only = must(VerifiedBlock::verify(&block, &commit, "125", &validators));
    assert!(
        finality::augment_verified(&mut store, tenant(), [1; 32], &receipt, Some(&vote_only))
            .is_err()
    );
    let (empty, empty_commit, empty_header, _) = committed_block(Vec::new(), height, 4);
    assert!(VerifiedBlock::verify_with_header(
        &empty,
        &empty_commit,
        &empty_header,
        "125",
        &validators
    )
    .is_err());
    let (mut foreign_call, _) = must(decode_signed(&raw));
    foreign_call.nonce += 1;
    let foreign_raw = must(foreign_call.sign(&[0xa5; 32]));
    let (foreign, foreign_commit, foreign_header, _) =
        committed_block(vec![foreign_raw], height, 4);
    let foreign_verified = must(VerifiedBlock::verify_with_header(
        &foreign,
        &foreign_commit,
        &foreign_header,
        "125",
        &validators,
    ));
    assert!(finality::augment_verified(
        &mut store,
        tenant(),
        [1; 32],
        &receipt,
        Some(&foreign_verified)
    )
    .is_err());
    assert!(progress.observe_verified_block(&foreign_verified).is_err());
    let mut substituted = block.clone();
    substituted.transactions = foreign.transactions.clone();
    assert!(
        VerifiedBlock::verify_with_header(&substituted, &commit, &header, "125", &validators)
            .is_err()
    );
    let mut changed_header = header.clone();
    changed_header.data_hash = foreign_header.data_hash;
    assert!(VerifiedBlock::verify_with_header(
        &block,
        &commit,
        &changed_header,
        "125",
        &validators
    )
    .is_err());
    assert_eq!(must(progress.rung_at([1; 32], 0)), Rung::Receipted);
    must(progress.observe_block_with_header(&block, &commit, &header, "125", &validators));
    assert_eq!(must(progress.rung_at([1; 32], 0)), Rung::Committed);
    let record = must(finality::augment_verified(
        &mut store,
        tenant(),
        [1; 32],
        &receipt,
        Some(&verified),
    ));
    assert_eq!(record.receipt_bytes, receipt_bytes);
    drop(store);
    drop(progress);
    let mut store = must(Store::open(&directory.0));
    let recovered = must(finality::restore_finality(
        &store,
        tenant(),
        [1; 32],
        &key,
        "125",
        &validators,
    ));
    assert_eq!(recovered, record);
    assert_eq!(recovered.rung, Rung::Committed);
    let mut recovered_progress = must(SubmissionProgress::restore(
        &store,
        tenant(),
        [1; 32],
        &key,
        "125",
        &validators,
    ));
    assert_eq!(
        must(recovered_progress.rung_at([1; 32], 0)),
        Rung::Committed
    );
    let (conflicting, conflicting_commit, conflicting_header, _) =
        committed_block(vec![raw], height, 7);
    let conflicting = must(VerifiedBlock::verify_with_header(
        &conflicting,
        &conflicting_commit,
        &conflicting_header,
        "125",
        &validators,
    ));
    assert!(matches!(
        recovered_progress.observe_verified_block(&conflicting),
        Err(FinalityError::Corrupt)
    ));
    assert!(matches!(
        finality::augment_verified(&mut store, tenant(), [1; 32], &receipt, Some(&conflicting)),
        Err(FinalityError::Corrupt)
    ));
    assert_eq!(
        must(finality::restore_finality(
            &store,
            tenant(),
            [1; 32],
            &key,
            "125",
            &validators
        )),
        record
    );
    let mut bad_trust = validators.clone();
    bad_trust[0].voting_power += 1;
    assert!(
        finality::restore_finality(&store, tenant(), [1; 32], &key, "125", &bad_trust).is_err()
    );
}

#[test]
fn finding_10_retained_evm_producer_queues_transmits_and_restores_exact_bytes() {
    let keys = Directory::new();
    let registry_store = keys.registry();
    let seed = must(registry_store.prepare_seed(b"transmission", [8; 32]));
    let issued = issue(seed.evm_address(), send_type());
    must(registry_store.provision(issued.grant_id, seed.secret(), &issued));
    let prepared = preparation(seed.evm_address());
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let reference: [u8; 32] = Sha256::digest(prepared.transaction.call.encode_unsigned()).into();
    must(prepared.retain(&mut store, tenant(), reference));
    let key = must(registry_store.load(issued.grant_id, issued));
    let signed = must(ready(self_sign(
        Some(&key),
        &prepared,
        &registry(),
        1000,
        5,
    )));
    let (_, signature) = must(decode_signed(&signed.signed_canonical_bytes));
    let external = must(attach_external_signature(&prepared, *signature.as_bytes()));
    assert_eq!(external, signed.signed_canonical_bytes);
    let verified = must(verify_before_submit(
        &external,
        &prepared,
        &prepared.actor_address,
        &registry(),
    ));
    let exact = verified.exact_bytes().to_vec();
    let hash = verified.activity_id();
    let legacy = include_bytes!(
        "../../../../../tests/fixtures/authority/provider-subject/sponsored.activity"
    );
    assert!(verify_before_submit(legacy, &prepared, &prepared.actor_address, &registry()).is_err());
    assert!(Outbox::default()
        .enqueue(&mut store, tenant(), legacy.to_vec())
        .is_err());
    let mut outbox = Outbox::default();
    assert_eq!(
        must(outbox.enqueue_verified(&mut store, tenant(), verified)),
        hash
    );
    assert_eq!(must(outbox.bytes_for_transmission(hash)), exact);
    drop(outbox);
    drop(store);
    let store = must(Store::open(&directory.0));
    let recovered = must(SigningPreparation::restore(&store, tenant(), reference));
    assert_eq!(recovered, prepared);
    let mut outbox = Outbox::default();
    must(outbox.restore(&store, tenant(), hash));
    assert_eq!(must(outbox.bytes_for_transmission(hash)), exact);
    assert_eq!(
        must(verify_before_submit(
            must(outbox.bytes_for_transmission(hash)),
            &recovered,
            &prepared.actor_address,
            &registry()
        ))
        .exact_bytes(),
        exact
    );
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    let sent = Arc::new(Mutex::new(Vec::new()));
    let captured = Arc::clone(&sent);
    let worker = std::thread::spawn(move || {
        let (stream, _) = must(listener.accept());
        must(stream.set_read_timeout(Some(std::time::Duration::from_secs(5))));
        let mut reader = BufReader::new(stream);
        let mut length = None;
        loop {
            let mut line = String::new();
            assert_ne!(must(reader.read_line(&mut line)), 0);
            if line == "\r\n" {
                break;
            }
            if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                length = Some(must(value.trim().parse::<usize>()));
            }
        }
        let mut body = vec![0; length.expect("HTTP length")];
        must(reader.read_exact(&mut body));
        let request: Value = must(serde_json::from_slice(&body));
        assert_eq!(request["method"], "eth_sendRawTransaction");
        *must(captured.lock()) = unhex(&request["params"][0]);
        let response = json!({"jsonrpc":"2.0", "id": request["id"], "result": format!("0x{}", hex::encode(hash))}).to_string();
        must(write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()));
        must(reader.get_mut().flush());
    });
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    assert_eq!(
        must(endpoint.send_raw_transaction(must(outbox.bytes_for_transmission(hash)))),
        hash
    );
    must(worker.join());
    assert_eq!(*must(sent.lock()), exact);
    let mut changed = recovered.clone();
    changed.transaction.call.nonce += 1;
    assert!(verify_before_submit(&exact, &changed, &prepared.actor_address, &registry()).is_err());
}
