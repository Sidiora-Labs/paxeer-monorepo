use ed25519_dalek::{Signer, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
};
use layerx_indexer::ai_market::{BlockCommitProof, FinalityAuthority};
pub use layerx_platform_webhooks::{
    ai_market, chain, deliveries, endpoints, error, events, http, scheme, trusted,
};
#[path = "../src/boundary.rs"]
mod boundary;
#[path = "../src/encoding.rs"]
pub mod encoding;
// Compile the exact hosted engine and route source into this integration target
// so its private dependency wiring can point at committed in-process peers.
#[path = "../src/repair_22_33_1328_authority.rs"]
mod authority_path_support;
#[path = "../src/hosted.rs"]
pub mod hosted;
#[path = "../src/routes.rs"]
mod routes;
use ai_market::*;
use chain::*;
use encoding::hex_encode;
use error::WebhookError;
use events::{EndpointId, Principal, ProtocolEvent, Verification};
use hosted::repair_support::{must, Harness, Peer};
use http::{Reply, Request};
use layerx_programs_ai_market::{
    codec, dispatch,
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{self, CaptureFacts, ReadProof},
    registry, registry_ops, state, *,
};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    fs,
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    path::PathBuf,
    sync::{
        atomic::{AtomicU64, AtomicUsize, Ordering},
        Arc, Mutex,
    },
    thread,
    time::Duration,
};
const CHAIN: [u8; 32] = [10; 32];
const PROGRAM: [u8; 32] = [11; 32];
const CHAIN_ID: &str = "webhook-repair-chain";
fn checked<T, E: std::fmt::Debug>(r: Result<T, E>) -> T {
    must(r)
}
fn principal() -> Principal {
    must(Principal::new("tenant:repair"))
}
fn endpoint() -> EndpointId {
    must(EndpointId::new("whep_repair"))
}
fn digest(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
struct Scratch(PathBuf);
impl Scratch {
    fn new(name: &str) -> Self {
        let mut nonce = [0; 16];
        must(getrandom::fill(&mut nonce));
        let root = std::env::temp_dir().join(format!("repair-1328-{name}-{}", hex_encode(&nonce)));
        must(fs::create_dir_all(&root));
        Self(root)
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn validators() -> Vec<Validator> {
    (1..=4)
        .map(|n| Validator {
            pub_key: SigningKey::from_bytes(&[n; 32]).verifying_key().to_bytes(),
            voting_power: 10,
        })
        .collect()
}

fn policy() -> CapturePolicy {
    CapturePolicy {
        chain_domain: CHAIN,
        consensus: checked(ChainPolicy::from_native(&FinalityAuthority {
            chain_id: CHAIN_ID.into(),
            validators: validators(),
        })),
    }
}

fn keys() -> CursorKeys {
    checked(CursorKeys::new((1, [1; 32]), &[]))
}

fn market() -> [u8; 32] {
    checked(codec::derive_market(
        checked(ChainDomain::new(CHAIN)),
        checked(ProgramId::new(PROGRAM)),
    ))
    .bytes()
}

fn created_state() -> Vec<u8> {
    let chain = checked(ChainDomain::new(CHAIN));
    let program = checked(ProgramId::new(PROGRAM));
    let actor = checked(PrincipalId::new([12; 32]));
    let asset = checked(AssetId::new([13; 32]));
    let policy = checked(TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: checked(Digest32::new([1; 32])),
            dataset_artifact: [2; 32],
            benchmark_suite: checked(Digest32::new([3; 32])),
            rubric: checked(RubricDigest::new([4; 32])),
            task_schema: checked(Digest32::new([5; 32])),
            result_schema: checked(Digest32::new([6; 32])),
            service_terms: checked(Digest32::new([7; 32])),
        },
        100,
        1,
    ));
    let mut payload = actor.bytes().to_vec();
    payload.extend(asset.bytes());
    payload.extend(checked(registry::derive_rewards_account(program, asset)).bytes());
    payload.extend([14; 32]);
    payload.push(0);
    let mut encoded = vec![0; TASK_POLICY_BYTES];
    checked(policy.encode(&mut encoded));
    payload.extend(encoded);
    payload.extend([16; 32]);
    let envelope = codec::Envelope {
        operation: dispatch::CREATE,
        chain,
        program,
        market: checked(codec::derive_market(chain, program)),
        actor,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        expiry: 10_000,
        request: checked(RequestId::new([17; 32])),
        payload: &payload,
        authentication: Authentication::Native,
    };
    let mut envelope_bytes = vec![0; 16_384];
    let n = checked(codec::encode_envelope(&envelope, &mut envelope_bytes));
    envelope_bytes.truncate(n);
    let decoded = checked(codec::decode_envelope(&envelope_bytes));
    let mut section = vec![0; registry::F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let outcome = checked(registry_ops::apply(
        &registry_ops::CallContext {
            chain,
            program,
            principal: actor,
            height: 1000,
        },
        None,
        &decoded,
        &mut section,
        &mut event,
    ));
    let registry_ops::Outcome::Applied { state, .. } = outcome else {
        panic!("CREATE must apply")
    };
    let mut out = vec![0; checked(state.encoded_len())];
    let mut scratch = vec![0; state::Section::Control.payload_cap()];
    checked(state::encode_shared_state(&state, &mut out, &mut scratch));
    out
}

fn header(root: [u8; 32], height: u64, data_hash: [u8; 32]) -> Header {
    Header {
        chain_id: CHAIN_ID.into(),
        height: must(i64::try_from(height)),
        app_hash: root.to_vec(),
        version: ConsensusVersion { block: 11, app: 1 },
        time: Timestamp {
            seconds: 1000,
            nanos: 0,
        },
        last_block_id: BlockId::default(),
        last_commit_hash: vec![1; 32],
        data_hash: data_hash.to_vec(),
        validators_hash: validators_hash(&validators()),
        next_validators_hash: validators_hash(&validators()).to_vec(),
        consensus_hash: vec![3; 32],
        last_results_hash: vec![4; 32],
        evidence_hash: vec![5; 32],
        proposer_address: vec![6; 20],
    }
}

fn signed(header: &Header) -> Commit {
    let mut commit = Commit {
        height: must(u64::try_from(header.height)),
        round: 0,
        block_id: BlockId {
            hash: header.hash().to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0xa5; 32],
            },
        },
        signatures: Vec::new(),
    };
    for n in 1..=4u8 {
        if n == 4 {
            commit.signatures.push(CommitSig::absent());
            continue;
        }
        let key = SigningKey::from_bytes(&[n; 32]);
        let timestamp = Timestamp {
            seconds: 1000,
            nanos: 123,
        };
        let mut vote = Vec::new();
        encode_varint_field(1, 2, &mut vote);
        vote.push(0x11);
        vote.extend(commit.height.to_le_bytes());
        let mut parts = Vec::new();
        encode_varint_field(1, 1, &mut parts);
        encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
        let mut block = Vec::new();
        encode_length_delimited(1, &commit.block_id.hash, &mut block);
        encode_length_delimited(2, &parts, &mut block);
        encode_length_delimited(4, &block, &mut vote);
        let mut time = Vec::new();
        encode_varint_field(1, 1000, &mut time);
        encode_varint_field(2, 123, &mut time);
        encode_length_delimited(5, &time, &mut vote);
        encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
        let mut message = Vec::new();
        encode_varint(vote.len() as u64, &mut message);
        message.extend(vote);
        commit.signatures.push(CommitSig {
            block_id_flag: BlockIdFlag::Commit,
            validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
            timestamp,
            signature: Some(key.sign(&message).to_bytes()),
        });
    }
    commit
}

fn membership(store: &[u8], key: &[u8], value: &[u8]) -> (Membership, [u8; 32]) {
    fn leaf(key: &[u8], value: &[u8], prefix: &[u8]) -> (Vec<u8>, [u8; 32]) {
        let mut op = Vec::new();
        number(1, 1, &mut op);
        number(3, 1, &mut op);
        number(4, 1, &mut op);
        bytes(5, prefix, &mut op);
        let mut proof = Vec::new();
        bytes(1, key, &mut proof);
        bytes(2, value, &mut proof);
        bytes(3, &op, &mut proof);
        let mut preimage = prefix.to_vec();
        varint(key.len() as u64, &mut preimage);
        preimage.extend(key);
        varint(32, &mut preimage);
        preimage.extend(digest(value));
        (proof, digest(&preimage))
    }
    let (proof, root) = leaf(key, value, &[0, 2, 2]);
    let (outer, app) = leaf(store, &root, &[0]);
    let mut multistore = Vec::new();
    bytes(1, &outer, &mut multistore);
    (Membership { proof, multistore }, app)
}

fn capture(observed: u64) -> CaptureDocument {
    let state = created_state();
    let mut manifest = vec![0, 1];
    manifest.extend(1u32.to_be_bytes());
    manifest.extend(must(u16::try_from(SHARED_STATE_KEY.len())).to_be_bytes());
    manifest.extend(SHARED_STATE_KEY);
    manifest.extend(digest(&state));
    manifest.extend(must(u32::try_from(state.len())).to_be_bytes());
    let mut value = vec![0, 1];
    value.extend(1u32.to_be_bytes());
    value.extend(digest(&manifest));
    let mut head_key = b"\x01\x00\x09progstor".to_vec();
    head_key.extend(PROGRAM);
    head_key.push(1);
    let (membership, root) = membership(b"lxphost", &head_key, &value);
    let proof = ProgramStateProof {
        membership,
        manifest,
    };
    let store_proof = checked(serde_json::to_vec(&proof));
    let header = header(root, 1000, [2; 32]);
    let commit = signed(&header);
    let native = BlockCommitProof {
        header: &header,
        commit: &commit,
        store_proof: &store_proof,
    };
    let verified = checked(layerx_indexer::ai_market::verify_block_commit(
        &native,
        &FinalityAuthority {
            chain_id: CHAIN_ID.into(),
            validators: validators(),
        },
    ));
    let facts = CaptureFacts {
        proof: ReadProof {
            chain: checked(ChainDomain::new(CHAIN)),
            program: checked(ProgramId::new(PROGRAM)),
            native_state_root: checked(Digest32::new(root)),
            observed_sequence: observed,
            execution_height: 1000,
            block_hash: checked(Digest32::new(header.hash())),
        },
        revision: 1,
        digest: checked(codec::state_digest(&state)),
        total_bytes: must(u32::try_from(state.len())),
        chunks: 1,
    };
    let binding = checked(queries::bind_snapshot(
        &state,
        &facts,
        &checked(verified.finality_evidence(checked(Digest32::new(root)))),
        1_700_000_000_000,
    ));
    let mut activity = [31; 32];
    activity[..8].copy_from_slice(&observed.to_be_bytes());
    let row = CapturedEvent {
        snapshot_id: checked(binding.snapshot_id()).bytes(),
        binding,
        event_id: checked(market_event_id(
            &market(),
            &activity,
            0,
            dispatch::CREATE.selector(),
        ))
        .as_str()
        .into(),
        decoder_version: 1,
        action: "CREATE_MARKET".into(),
        ai_event_tag: dispatch::CREATE.selector(),
        source_activity_id: activity,
        effect_ordinal: 0,
        receipt_digest: [32; 32],
        manifest: UpstreamManifest::default(),
    };
    let evidence = Evidence {
        state: &state,
        proof: Some(native),
    };
    checked(CaptureDocument::from_capture(&row, &evidence))
}

fn outbox(scratch: &Scratch) -> MarketOutbox {
    must(MarketOutbox::open_portable(
        &scratch.0,
        principal(),
        policy(),
        keys(),
    ))
}
fn producer() -> ProducerGrant {
    must(ProducerGrant::from_authenticated_role(
        trusted::IngressRole::Producer,
    ))
}
fn admit(outbox: &MarketOutbox, doc: &CaptureDocument) -> ProtocolEvent {
    let Admission::Finalized {
        event,
        duplicate: false,
    } = must(outbox.admit_document(&producer(), doc))
    else {
        panic!("new committed admission");
    };
    event
}
fn provisional(document: &CaptureDocument, observed: u64, ordinal: u16) -> CapturedEvent {
    let mut binding = must(queries::SnapshotBinding::decode(&document.binding));
    binding.rank = 3;
    binding.observed_sequence = observed;
    let mut activity = [31; 32];
    activity[..8].copy_from_slice(&observed.to_be_bytes());
    CapturedEvent {
        snapshot_id: must(binding.snapshot_id()).bytes(),
        binding,
        event_id: must(market_event_id(&market(), &activity, ordinal, 1))
            .as_str()
            .into(),
        decoder_version: 1,
        action: "CREATE_MARKET".into(),
        ai_event_tag: 1,
        source_activity_id: activity,
        effect_ordinal: ordinal,
        receipt_digest: [32; 32],
        manifest: UpstreamManifest::default(),
    }
}
fn state_json(scratch: &Scratch) -> Value {
    must(serde_json::from_slice(&must(fs::read(
        scratch.0.join("state.json"),
    ))))
}
fn policy_fast() -> endpoints::RetryPolicy {
    endpoints::RetryPolicy {
        base_delay_seconds: 1,
        maximum_delay_seconds: 1,
        spread_percent: 0,
        ..endpoints::RetryPolicy::hosted()
    }
}
fn register(
    service: &hosted::HostedService,
    principal: &Principal,
    url: &str,
    token: &str,
    now: u64,
) -> EndpointId {
    let registration =
        must(service.register(principal, url, &[], Verification::Unverified, token, now));
    must(EndpointId::new(registration.endpoint))
}
fn trusted_event(observed: u64) -> trusted::TrustedEvent {
    trusted_event_for(principal(), observed)
}
fn trusted_event_for(principal: Principal, observed: u64) -> trusted::TrustedEvent {
    let scratch = Scratch::new("trusted");
    let store = must(MarketOutbox::open_portable(
        &scratch.0,
        principal,
        policy(),
        keys(),
    ));
    must(store.grant(&market()));
    admit(&store, &capture(observed));
    must(store.pending_trusted(&market()))
        .pop()
        .unwrap_or_else(|| panic!("durable verified event"))
}
fn verify_request(request: &Request, now: u64) -> scheme::Verified {
    let keys = BTreeMap::from([(
        "whk_repair".into(),
        SigningKey::from_bytes(&[91; 32]).verifying_key().to_bytes(),
    )]);
    must(scheme::verify(
        &scheme::Presentation {
            id: request
                .header(scheme::ID_HEADER)
                .unwrap_or_else(|| panic!("event ID")),
            timestamp: request
                .header(scheme::TIMESTAMP_HEADER)
                .unwrap_or_else(|| panic!("timestamp")),
            key_id: request
                .header(scheme::KEY_HEADER)
                .unwrap_or_else(|| panic!("key ID")),
            signature: request
                .header(scheme::SIGNATURE_HEADER)
                .unwrap_or_else(|| panic!("signature")),
            payload: &request.body,
            now,
            tolerance_seconds: scheme::DEFAULT_TOLERANCE_SECONDS,
        },
        &keys,
    ))
}

#[test]
fn finding_15_retention_reopens_compacted_identities_and_protects_unpublished_rows() {
    let scratch = Scratch::new("retention");
    let store = outbox(&scratch);
    must(store.grant(&market()));
    let first = capture(1);
    let endpoint = endpoint();
    let ResumePage::Events {
        next_cursor: old_cursor,
        ..
    } = must(store.resume(&principal(), &endpoint, &market(), None, 1))
    else {
        panic!("cursor");
    };
    let event = admit(&store, &first);
    assert!(matches!(
        store.prune(&market(), 1),
        Err(MarketEventError::InvalidRequest)
    ));
    assert_eq!(must(store.pending(&market())), std::slice::from_ref(&event));
    let delayed = provisional(&first, 1, 40);
    assert_eq!(
        must(store.admit(
            &producer(),
            &delayed,
            &Evidence {
                state: &first.state,
                proof: None
            }
        )),
        Admission::Delayed
    );
    must(store.acknowledge(&market(), event.id()));
    assert_eq!(must(store.prune(&market(), 1)), 1);
    drop(store);
    let store = outbox(&scratch);
    assert!(matches!(
        store.admit_document(&producer(), &first),
        Err(MarketEventError::HistoryExpired)
    ));
    assert!(matches!(
        must(store.resume(&principal(), &endpoint, &market(), Some(&old_cursor), 1)),
        ResumePage::Resync {
            through_sequence: 1,
            ..
        }
    ));
    let json = state_json(&scratch);
    let stream = &json["streams"][hex_encode(&market())];
    assert_eq!(stream["replay_through"], 1);
    assert_eq!(
        stream["admitted"]
            .as_object()
            .unwrap_or_else(|| panic!("identities"))
            .len(),
        0
    );
    assert_eq!(
        stream["provisional"]
            .as_object()
            .unwrap_or_else(|| panic!("provisional"))
            .len(),
        0
    );
    for n in 2..=18 {
        let event = admit(&store, &capture(n));
        must(store.acknowledge(&market(), event.id()));
        assert_eq!(must(store.prune(&market(), n)), 1);
        assert_eq!(
            state_json(&scratch)["streams"][hex_encode(&market())]["admitted"]
                .as_object()
                .unwrap_or_else(|| panic!("identities"))
                .len(),
            0
        );
    }
    let last = admit(&store, &capture(19));
    drop(store);
    let reopened = outbox(&scratch);
    assert_eq!(
        must(reopened.pending(&market())),
        std::slice::from_ref(&last)
    );
    let Admission::Finalized { event, duplicate } =
        must(reopened.admit_document(&producer(), &capture(19)))
    else {
        panic!("duplicate");
    };
    assert!(duplicate);
    assert_eq!(event, last);
    assert!(matches!(
        reopened.rollback(&market(), 18),
        Err(MarketEventError::Quarantined(
            QuarantineCause::FinalityBoundary
        ))
    ));
}

#[test]
fn finding_15_bounds_provisional_and_market_admission_without_allocating_unknown_streams() {
    let scratch = Scratch::new("capacity");
    let store = outbox(&scratch);
    let doc = capture(1);
    assert!(matches!(
        store.admit_document(&producer(), &doc),
        Err(MarketEventError::Revoked)
    ));
    must(store.grant(&market()));
    for n in 0..MAXIMUM_PROVISIONAL {
        let row = provisional(&doc, 2, must(u16::try_from(n)));
        assert_eq!(
            must(store.admit(
                &producer(),
                &row,
                &Evidence {
                    state: &doc.state,
                    proof: None
                }
            )),
            Admission::Delayed
        );
    }
    let extra = provisional(&doc, 2, must(u16::try_from(MAXIMUM_PROVISIONAL)));
    assert!(matches!(
        store.admit(
            &producer(),
            &extra,
            &Evidence {
                state: &doc.state,
                proof: None
            }
        ),
        Err(MarketEventError::CaptureStopped)
    ));
    for n in 1..MAXIMUM_MARKETS {
        let mut id = [88; 32];
        id[..8].copy_from_slice(&(n as u64).to_be_bytes());
        must(store.grant(&id));
    }
    assert!(matches!(
        store.grant(&[99; 32]),
        Err(MarketEventError::CaptureStopped)
    ));
    drop(store);
    let reopened = outbox(&scratch);
    assert!(matches!(
        reopened.grant(&[99; 32]),
        Err(MarketEventError::CaptureStopped)
    ));
    assert_eq!(must(reopened.rollback(&market(), 0)), MAXIMUM_PROVISIONAL);
    let json = state_json(&scratch);
    assert_eq!(
        json["streams"]
            .as_object()
            .unwrap_or_else(|| panic!("streams"))
            .len(),
        MAXIMUM_MARKETS
    );
    assert!(must(reopened.alerts())
        .iter()
        .any(|alert| alert.category == "outbox_capacity"));
}

fn deliver_facts(
    consumer: &Arc<MarketConsumer>,
    signed: &SignedDelivery,
    expected: &BTreeMap<&'static str, String>,
) {
    let expected = expected.clone();
    let tls = hosted::repair_support::tls_material();
    let receiver = Arc::clone(consumer);
    let peer = Peer::http(&tls, move |request| {
        let result = must(
            receiver.receive(
                &scheme::Presentation {
                    id: request
                        .header(scheme::ID_HEADER)
                        .unwrap_or_else(|| panic!("id")),
                    timestamp: request
                        .header(scheme::TIMESTAMP_HEADER)
                        .unwrap_or_else(|| panic!("timestamp")),
                    key_id: request
                        .header(scheme::KEY_HEADER)
                        .unwrap_or_else(|| panic!("key")),
                    signature: request
                        .header(scheme::SIGNATURE_HEADER)
                        .unwrap_or_else(|| panic!("signature")),
                    payload: &request.body,
                    now: 1000,
                    tolerance_seconds: scheme::DEFAULT_TOLERANCE_SECONDS,
                },
                |tx| {
                    for (name, value) in &expected {
                        assert_eq!(
                            tx.facts()
                                .iter()
                                .find(|fact| fact.name() == *name)
                                .unwrap_or_else(|| panic!("v2 fact"))
                                .value(),
                            value.as_str()
                        );
                    }
                    must(tx.set("schema", MARKET_FACTS_SCHEMA.into()));
                    "committed".into()
                },
            ),
        );
        assert!(matches!(result, Consumed::Applied { sequence: 1, .. }));
        Reply::json(204, String::new())
    });
    let client = boundary::Client::trusted(boundary::ClientIdentity::new(
        must(native_tls::Certificate::from_der(&tls.ca)),
        Some(tls.identity.clone()),
    ));
    let target = must(boundary::Endpoint::parse(&peer.url));
    let headers = vec![
        (scheme::ID_HEADER.into(), signed.id.clone()),
        (scheme::TIMESTAMP_HEADER.into(), signed.timestamp.clone()),
        (scheme::KEY_HEADER.into(), signed.key_id.clone()),
        (scheme::SIGNATURE_HEADER.into(), signed.signature.clone()),
    ];
    assert_eq!(
        must(client.request(&boundary::OutboundRequest {
            endpoint: &target,
            method: "POST",
            path: "",
            bearer: None,
            idempotency: None,
            headers: &headers,
            body: &signed.body
        }))
        .status,
        204
    );
    drop(peer);
}

#[test]
fn finding_16_v2_facts_survive_signed_transport_and_consumer_restart() {
    let scratch = Scratch::new("facts");
    let store = outbox(&scratch);
    must(store.grant(&market()));
    let doc = capture(1);
    let event = admit(&store, &doc);
    let binding = must(queries::SnapshotBinding::decode(&doc.binding));
    let expected = BTreeMap::from([
        ("facts_schema", MARKET_FACTS_SCHEMA.to_owned()),
        (
            "store_proof_digest",
            hex_encode(binding.store_proof_digest.as_bytes()),
        ),
        ("block_height", binding.block_height.to_string()),
        ("block_hash", hex_encode(binding.block_hash.as_bytes())),
        ("app_hash", hex_encode(binding.app_hash.as_bytes())),
    ]);
    let facts: BTreeMap<_, _> = event
        .facts()
        .iter()
        .map(|f| (f.name(), f.value().to_owned()))
        .collect();
    for (name, value) in &expected {
        assert_eq!(facts.get(name), Some(value));
    }
    assert!(!facts.contains_key("checkpoint_id"));
    let signing = SigningKey::from_bytes(&[91; 32]);
    let signed = must(SignedDelivery::sign(
        &event,
        &endpoint(),
        "whk_repair",
        &signing,
        1000,
    ));
    drop(store);
    let reopened = outbox(&scratch);
    let Admission::Finalized {
        event: recovered,
        duplicate: true,
    } = must(reopened.admit_document(&producer(), &doc))
    else {
        panic!("recovered projection");
    };
    assert_eq!(
        must(SignedDelivery::sign(
            &recovered,
            &endpoint(),
            "whk_repair",
            &signing,
            1001
        ))
        .body,
        signed.body
    );
    let consumer_dir = scratch.0.join("consumer");
    let keys = BTreeMap::from([("whk_repair".into(), signing.verifying_key().to_bytes())]);
    let consumer = Arc::new(must(MarketConsumer::open(&consumer_dir, keys.clone())));
    deliver_facts(&consumer, &signed, &expected);
    drop(consumer);
    let recovered = must(MarketConsumer::open(&consumer_dir, keys));
    assert_eq!(
        must(recovered.effect(event.subject(), "schema")).as_deref(),
        Some(MARKET_FACTS_SCHEMA)
    );
    assert_eq!(must(recovered.last_sequence(event.subject())), 1);
    assert!(matches!(
        must(recovered.receive(&signed.presentation(1001), |_| panic!(
            "effect already committed"
        ))),
        Consumed::Duplicate { sequence: 1, .. }
    ));
}

#[test]
fn finding_12_maximum_policy_completes_final_attempt_and_reopens_history() {
    let mut policy = policy_fast();
    policy.maximum_attempts = endpoints::MAXIMUM_ATTEMPTS;
    assert!(policy.validate().is_ok());
    for configured in [0, u64::from(endpoints::MAXIMUM_ATTEMPTS) + 1, u64::MAX] {
        assert!(hosted::configured_retry_policy(|name, default| Ok(
            if name == "LAYERX_WEBHOOKS_MAXIMUM_ATTEMPTS" {
                configured
            } else {
                default
            }
        ))
        .is_err());
    }
    assert_eq!(
        must(hosted::configured_retry_policy(|name, default| Ok(
            if name == "LAYERX_WEBHOOKS_MAXIMUM_ATTEMPTS" {
                32
            } else {
                default
            }
        )))
        .maximum_attempts,
        32
    );
    assert!(matches!(
        endpoints::RetryPolicy {
            maximum_attempts: endpoints::MAXIMUM_ATTEMPTS + 1,
            ..policy
        }
        .validate(),
        Err(WebhookError::InvalidRequest)
    ));
    let harness = Harness::new();
    let received = Arc::new(AtomicUsize::new(0));
    let count = Arc::clone(&received);
    let peer = Peer::http(&harness.tls, move |request| {
        let attempt = count.fetch_add(1, Ordering::AcqRel) + 1;
        let timestamp = must(
            request
                .header(scheme::TIMESTAMP_HEADER)
                .unwrap_or_else(|| panic!("timestamp"))
                .parse(),
        );
        verify_request(&request, timestamp);
        assert_eq!(
            request.header(scheme::ATTEMPT_HEADER),
            Some(attempt.to_string().as_str())
        );
        Reply::json(503, "{}".into())
    });
    let service = harness.service("maximum-attempts", policy);
    let endpoint = register(&service, &principal(), &peer.url, "max-attempts", 1000);
    let event = trusted_event(1);
    must(service.publish(&event, 1000));
    for attempt in 1..=endpoints::MAXIMUM_ATTEMPTS {
        let at = 999 + u64::from(attempt);
        let report = must(service.dispatch_with_clock(1, || at));
        assert_eq!(report.attempted, 1);
        assert_eq!(report.failure_count, 0);
        if attempt < endpoints::MAXIMUM_ATTEMPTS {
            assert_eq!(report.retrying, 1);
        } else {
            assert_eq!(report.dead_lettered, 1);
        }
    }
    assert_eq!(
        received.load(Ordering::Acquire),
        endpoints::MAXIMUM_ATTEMPTS as usize
    );
    drop(service);
    let reopened = harness.service("after-reopen", policy);
    let snapshot = must(reopened.snapshot(&principal(), 1032, 200));
    let delivery = snapshot
        .deliveries
        .iter()
        .find(|row| row.endpoint == endpoint.as_str())
        .unwrap_or_else(|| panic!("delivery"));
    assert_eq!(
        delivery.attempts.len(),
        endpoints::MAXIMUM_ATTEMPTS as usize
    );
    assert!(matches!(
        delivery.state,
        deliveries::DeliveryState::DeadLettered { attempts: 32, .. }
    ));
    assert_eq!(must(reopened.dispatch_with_clock(1, || 2000)).attempted, 0);
}

#[test]
fn finding_13_slow_dispatch_refreshes_signatures_leases_and_settlement() {
    let harness = Harness::new();
    let policy = policy_fast();
    let worker = Arc::new(harness.service("second-participant", policy));
    let clock = Arc::new(AtomicU64::new(1000));
    let seen = Arc::new(Mutex::new(Vec::<(String, u64)>::new()));
    let receiver_clock = Arc::clone(&clock);
    let receiver_seen = Arc::clone(&seen);
    let second_worker = Arc::clone(&worker);
    let receiver = Peer::http(&harness.tls, move |request| {
        let now = receiver_clock.load(Ordering::Acquire);
        let verified = verify_request(&request, now);
        let count = must(receiver_seen.lock()).len();
        must(receiver_seen.lock()).push((
            request
                .header(scheme::ENDPOINT_HEADER)
                .unwrap_or_else(|| panic!("endpoint"))
                .into(),
            verified.timestamp,
        ));
        if count == 0 {
            receiver_clock.store(1401, Ordering::Release);
            Reply::json(503, "{}".into())
        } else {
            assert_eq!(verified.timestamp, 1401);
            let competing = must(second_worker.dispatch_with_clock(8, || now));
            assert_eq!(competing.attempted, 0);
            receiver_clock.store(1405, Ordering::Release);
            Reply::json(204, String::new())
        }
    });
    let service = harness.service("first-participant", policy);
    register(
        &service,
        &principal(),
        &format!("{}/a", receiver.url),
        "a",
        1000,
    );
    register(
        &service,
        &principal(),
        &format!("{}/b", receiver.url),
        "b",
        1000,
    );
    let event = trusted_event(1);
    must(service.publish(&event, 1000));
    let report = must(service.dispatch_with_clock(2, || clock.load(Ordering::Acquire)));
    assert_eq!(
        (
            report.attempted,
            report.retrying,
            report.delivered,
            report.failure_count
        ),
        (2, 1, 1, 0)
    );
    let snapshot = must(service.snapshot(&principal(), 1405, 200));
    let seen_endpoints = must(seen.lock());
    let first = snapshot
        .deliveries
        .iter()
        .find(|d| d.endpoint == seen_endpoints[0].0)
        .unwrap_or_else(|| panic!("first"));
    let second = snapshot
        .deliveries
        .iter()
        .find(|d| d.endpoint == seen_endpoints[1].0)
        .unwrap_or_else(|| panic!("second"));
    assert!(matches!(
        first.state,
        deliveries::DeliveryState::Retrying {
            next_attempt_at: 1402,
            ..
        }
    ));
    assert!(matches!(
        second.state,
        deliveries::DeliveryState::Delivered { at: 1405, .. }
    ));
    assert_eq!(
        seen_endpoints.iter().map(|(_, at)| *at).collect::<Vec<_>>(),
        [1000, 1401]
    );
    assert_eq!(first.attempts[0].at, 1401);
    assert_eq!(second.attempts[0].at, 1405);
}

#[test]
fn finding_11_cursor_route_refuses_utf8_boundary_hex_errors_and_bad_tags() {
    let harness = Harness::new();
    let service = Arc::new(harness.service("cursor-route", policy_fast()));
    let receiver = Peer::http(&harness.tls, |_| Reply::json(204, String::new()));
    let endpoint = register(
        &service,
        &principal(),
        &receiver.url,
        "cursor-endpoint",
        1000,
    );
    must(service.publish(&trusted_event(1), 1000));
    let valid = must(service.events_since(&principal(), &endpoint, None, 1)).next_cursor;
    let mut forged = valid.clone().into_bytes();
    let last = forged.last_mut().unwrap_or_else(|| panic!("tag"));
    *last = if *last == b'0' { b'1' } else { b'0' };
    let malformed = vec![
        format!("lxwc2_{}é{}", "0".repeat(15), "0".repeat(31)),
        format!("lxwc2_{}g", "0".repeat(47)),
        String::from_utf8(forged).unwrap_or_else(|error| panic!("ASCII: {error:?}")),
        format!("lxwc2_{}", "0".repeat(47)),
    ];
    assert_eq!(
        malformed[0]
            .strip_prefix("lxwc2_")
            .unwrap_or_else(|| panic!("prefix"))
            .len(),
        48
    );
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let address = must(listener.local_addr());
    let target_endpoint = endpoint.clone();
    let worker = thread::spawn(move || {
        for _ in 0..5 {
            let (mut socket, _) = must(listener.accept());
            must(socket.set_read_timeout(Some(Duration::from_secs(5))));
            let request = must(http::read_request(&mut socket));
            let reply = routes::endpoint_route(
                &service,
                &request,
                &principal(),
                target_endpoint.as_str(),
                "events",
                1000,
            );
            must(http::write_reply(&mut socket, &reply));
        }
    });
    for (cursor, status) in malformed
        .into_iter()
        .map(|cursor| (cursor, 400))
        .chain(std::iter::once((valid, 200)))
    {
        let mut socket = must(TcpStream::connect(address));
        must(socket.set_read_timeout(Some(Duration::from_secs(5))));
        must(write!(socket, "GET /v1/webhooks/endpoints/{}/events?cursor={cursor} HTTP/1.1\r\nHost: localhost\r\n\r\n", endpoint.as_str()));
        let mut reply = String::new();
        must(socket.read_to_string(&mut reply));
        assert!(reply.starts_with(&format!("HTTP/1.1 {status}")), "{reply}");
        if status == 400 {
            assert!(reply.contains("invalid_cursor"));
        }
    }
    must(worker.join());
}

#[test]
fn finding_14_corrupt_principal_is_reported_while_healthy_delivery_and_retention_continue() {
    let harness = Harness::new();
    let service = harness.service("health", policy_fast());
    let peer = Peer::http(&harness.tls, |request| {
        verify_request(&request, 1000);
        Reply::json(204, String::new())
    });
    let corrupt = principal();
    register(&service, &corrupt, &peer.url, "bad", 1000);
    let saved = must(harness.store.lock()).corrupt(&corrupt);
    let healthy = must(Principal::new("tenant:healthy"));
    register(&service, &healthy, &peer.url, "good", 1000);
    // Use the real producer and its tenant principal, then route that healthy
    // shard ahead of neither the corrupt one nor a synthetic delivery.
    let scratch = Scratch::new("healthy-producer");
    let source = must(MarketOutbox::open_portable(
        &scratch.0,
        healthy.clone(),
        policy(),
        keys(),
    ));
    must(source.grant(&market()));
    admit(&source, &capture(1));
    admit(&source, &capture(2));
    for event in must(source.pending_trusted(&market())) {
        must(service.publish(&event, 1000));
    }
    let report = must(service.dispatch_with_clock(8, || 1000));
    assert_eq!((report.delivered, report.failure_count), (2, 1));
    assert_eq!(report.failures.len(), 1);
    assert_eq!(report.failures[0].category, "corrupt_store");
    assert_eq!(
        report.failures[0].principal_digest,
        hex_encode(&digest(corrupt.as_str().as_bytes()))
    );
    assert!(!service.ready());
    let retention = must(service.prune_all_report(1));
    assert_eq!(retention.failure_count, 1);
    assert_eq!(retention.released, 1);
    assert_eq!(retention.failures[0].category, "corrupt_store");
    assert_eq!(service.worker_health().failure_count, 2);
    must(harness.store.lock()).restore(&corrupt, saved);
    assert_eq!(
        must(service.dispatch_with_clock(8, || 1000)).failure_count,
        0
    );
    assert_eq!(must(service.prune_all_report(1)).failure_count, 0);
    assert_eq!(service.worker_health().failure_count, 0);
    assert!(service.ready());
}

#[test]
fn finding_20_authority_producer_https_consumer_maintained_and_historical_evidence() {
    authority_path_support::authority_path();
}

#[test]
fn finding_14_diagnostics_are_bounded_without_hiding_failure_count() {
    let harness = Harness::new();
    let service = harness.service("bounded-health", policy_fast());
    let mut store = must(harness.store.lock());
    // Corrupt durable participants are a store fault, not synthetic successful
    // deliveries. The production enumerator must inspect every affected shard.
    for n in 0..=hosted::MAXIMUM_REPORTED_FAILURES {
        let id = hex_encode(&digest(format!("tenant:corrupt-{n}").as_bytes()));
        store.members.insert(id.clone());
        store
            .shards
            .insert(format!("webhooks:principal:{id}"), (1, "{corrupt".into()));
    }
    drop(store);
    let report = must(service.dispatch_with_clock(1, || 1000));
    assert_eq!(report.attempted, 0);
    assert_eq!(report.failure_count, hosted::MAXIMUM_REPORTED_FAILURES + 1);
    assert_eq!(report.failures.len(), hosted::MAXIMUM_REPORTED_FAILURES);
    let health = service.worker_health();
    assert_eq!(health.failure_count, hosted::MAXIMUM_REPORTED_FAILURES + 1);
    assert_eq!(health.failures.len(), hosted::MAXIMUM_REPORTED_FAILURES);
    assert!(
        health
            .failures
            .iter()
            .all(|failure| failure.category == "corrupt_store"
                && failure.principal_digest.len() == 64)
    );
    assert!(!service.ready());
}

#[test]
fn finding_15_horizon_waits_for_older_unpublished_rows_and_migrates_legacy_identities() {
    let scratch = Scratch::new("out-of-order");
    let store = outbox(&scratch);
    must(store.grant(&market()));
    let high = admit(&store, &capture(20));
    let low = admit(&store, &capture(2));
    must(store.acknowledge(&market(), high.id()));
    assert_eq!(must(store.prune(&market(), 1)), 1);
    let json = state_json(&scratch);
    assert_eq!(json["streams"][hex_encode(&market())]["replay_through"], 1);
    assert_eq!(must(store.pending(&market())), std::slice::from_ref(&low));
    drop(store);
    // The prior durable codec emitted no observed_sequence on admitted
    // identities. Reopen that representation rather than inventing deliveries.
    let mut json = state_json(&scratch);
    for admitted in json["streams"][hex_encode(&market())]["admitted"]
        .as_object_mut()
        .unwrap_or_else(|| panic!("identities"))
        .values_mut()
    {
        admitted
            .as_object_mut()
            .unwrap_or_else(|| panic!("identity"))
            .remove("observed_sequence");
    }
    must(fs::write(
        scratch.0.join("state.json"),
        must(serde_json::to_vec(&json)),
    ));
    let store = outbox(&scratch);
    must(store.acknowledge(&market(), low.id()));
    assert_eq!(must(store.prune(&market(), 2)), 1);
    let json = state_json(&scratch);
    let stream = &json["streams"][hex_encode(&market())];
    assert_eq!(stream["replay_through"], 20);
    assert!(stream["admitted"]
        .as_object()
        .unwrap_or_else(|| panic!("compacted"))
        .is_empty());
    assert!(matches!(
        store.admit_document(&producer(), &capture(20)),
        Err(MarketEventError::HistoryExpired)
    ));
    assert_eq!(admit(&store, &capture(21)).subject_sequence(), 3);
}
