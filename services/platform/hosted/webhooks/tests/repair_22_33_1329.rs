use ed25519_dalek::{Signer, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
};
use layerx_indexer::ai_market::FinalityAuthority;
use layerx_platform_webhooks::{
    ai_market::{
        Admission, CapturedEvent, Consumed, CursorKeys, Evidence, FinalityPolicy, MarketConsumer,
        MarketEventError, MarketOutbox, ProducerGrant, SignedDelivery,
    },
    encoding::hex_encode,
    EndpointId, Principal, ProtocolEvent, Verification, WebhookError,
};
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    fmt::Debug,
    fs,
    path::{Path, PathBuf},
    time::{SystemTime, UNIX_EPOCH},
};

const CHAIN: [u8; 32] = [0x11; 32];
const PROGRAM: [u8; 32] = [0x22; 32];
const CHAIN_ID: &str = "ai-market-finality";
const SIGNED: [(u8, u8); 3] = [(1, 1), (2, 2), (3, 3)];
const KEY_ID: &str = "whk_paxai";

fn ok<T, E: Debug>(value: Result<T, E>, context: &str) -> T {
    value.unwrap_or_else(|error| panic!("{context}: {error:?}"))
}

struct Scratch(PathBuf);

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn scratch(label: &str) -> Scratch {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(0, |duration| duration.as_nanos());
    let directory = std::env::temp_dir().join(format!(
        "paxai-webhooks-{label}-{}-{nanos}",
        std::process::id()
    ));
    ok(fs::create_dir_all(&directory), "scratch directory");
    Scratch(directory)
}

fn now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(0, |duration| duration.as_secs())
}

fn validators() -> Vec<Validator> {
    (1..=4)
        .map(|value| Validator {
            pub_key: SigningKey::from_bytes(&[value; 32])
                .verifying_key()
                .to_bytes(),
            voting_power: 10,
        })
        .collect()
}

fn commit_header(chain_id: &str, observed: u64, app_hash: [u8; 32]) -> Header {
    Header {
        chain_id: chain_id.to_owned(),
        height: (observed + 100) as i64,
        app_hash: app_hash.to_vec(),
        version: ConsensusVersion { block: 11, app: 1 },
        time: Timestamp {
            seconds: 1_000,
            nanos: 0,
        },
        last_block_id: BlockId::default(),
        last_commit_hash: vec![1; 32],
        data_hash: vec![2; 32],
        validators_hash: validators_hash(&validators()),
        next_validators_hash: validators_hash(&validators()).to_vec(),
        consensus_hash: vec![3; 32],
        last_results_hash: vec![4; 32],
        evidence_hash: vec![5; 32],
        proposer_address: vec![6; 20],
    }
}

/// A block commit with `(validator position, signing key)` slots, numbered from 1.
fn signed(header: &Header, slots: &[(u8, u8)]) -> Commit {
    let mut commit = Commit {
        height: header.height as u64,
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
    for position in 1..=4 {
        let Some((_, key_value)) = slots.iter().find(|(slot, _)| *slot == position) else {
            commit.signatures.push(CommitSig::absent());
            continue;
        };
        let key = SigningKey::from_bytes(&[*key_value; 32]);
        let validator_key = SigningKey::from_bytes(&[position; 32]);
        let timestamp = Timestamp {
            seconds: 1_000,
            nanos: 123,
        };
        let mut vote = Vec::new();
        encode_varint_field(1, 2, &mut vote);
        vote.push(0x11);
        vote.extend_from_slice(&commit.height.to_le_bytes());
        let mut parts = Vec::new();
        encode_varint_field(
            1,
            u64::from(commit.block_id.part_set_header.total),
            &mut parts,
        );
        encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
        let mut block = Vec::new();
        encode_length_delimited(1, &commit.block_id.hash, &mut block);
        encode_length_delimited(2, &parts, &mut block);
        encode_length_delimited(4, &block, &mut vote);
        let mut time = Vec::new();
        encode_varint_field(1, timestamp.seconds as u64, &mut time);
        encode_varint_field(2, u64::from(timestamp.nanos), &mut time);
        encode_length_delimited(5, &time, &mut vote);
        encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
        let mut sign_bytes = Vec::new();
        encode_varint(vote.len() as u64, &mut sign_bytes);
        sign_bytes.extend_from_slice(&vote);
        commit.signatures.push(CommitSig {
            block_id_flag: BlockIdFlag::Commit,
            validator_address: Sha256::digest(validator_key.verifying_key().as_bytes())[..20]
                .to_vec(),
            timestamp,
            signature: Some(key.sign(&sign_bytes).to_bytes()),
        });
    }
    commit
}

fn principal() -> Principal {
    ok(Principal::new("tenant:paxai-markets"), "principal")
}

fn endpoint() -> EndpointId {
    ok(EndpointId::new("whep_paxai"), "endpoint")
}

fn policy() -> FinalityPolicy {
    FinalityPolicy {
        chain_domain: CHAIN,
        authority: FinalityAuthority {
            chain_id: CHAIN_ID.to_owned(),
            validators: validators(),
        },
    }
}

fn open_with(directory: &Path, keys: CursorKeys) -> MarketOutbox {
    ok(
        MarketOutbox::open(directory, principal(), policy(), keys),
        "outbox opens",
    )
}

fn open(directory: &Path) -> MarketOutbox {
    open_with(directory, ok(CursorKeys::new((1, [1; 32]), &[]), "keys"))
}

fn signer() -> SigningKey {
    SigningKey::from_bytes(&[0x42; 32])
}

fn receiver_keys() -> BTreeMap<String, [u8; 32]> {
    BTreeMap::from([(KEY_ID.to_owned(), signer().verifying_key().to_bytes())])
}

fn consumer(directory: &Path) -> MarketConsumer {
    ok(MarketConsumer::open(directory, receiver_keys()), "consumer")
}

fn deliver(event: &ProtocolEvent, timestamp: u64) -> SignedDelivery {
    ok(
        SignedDelivery::sign(event, &endpoint(), KEY_ID, &signer(), timestamp),
        "signed delivery",
    )
}

mod repair_m1329 {
    use super::{consumer, deliver, hex_encode, now, scratch};
    use super::{Admission, CapturedEvent, Consumed, Evidence, MarketEventError, MarketOutbox};
    use super::{ProducerGrant, ProtocolEvent, SignedDelivery, Verification, WebhookError};
    use super::{CHAIN, CHAIN_ID, PROGRAM};
    use layerx_indexer::ai_market::{BlockCommitProof, FinalityAuthority};
    use layerx_platform_webhooks::{
        ai_market::{market_event_id, CaptureDocument, ConsumerTransaction, UpstreamManifest},
        chain::{bytes, number, varint, Membership, ProgramStateProof},
        http,
        trusted::IngressRole,
    };
    use layerx_programs_ai_market::{
        codec, dispatch,
        policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
        queries::{self, ReadProof, StateCapture},
        registry, registry_ops, state, *,
    };
    use serde_json::{json, Value};
    use sha2::{Digest, Sha256};
    use std::{
        fs,
        io::{Read, Write},
        net::{TcpListener, TcpStream},
        path::Path,
        process::{Command, Stdio},
        thread,
        time::{Duration, Instant},
    };

    fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
        result.unwrap_or_else(|error| panic!("{error:?}"))
    }

    fn digest(bytes: &[u8]) -> [u8; 32] {
        Sha256::digest(bytes).into()
    }

    fn validators() -> Vec<layerx_committed_proof::header::Validator> {
        super::validators()
    }

    fn market() -> [u8; 32] {
        checked(codec::derive_market(
            checked(ChainDomain::new(CHAIN)),
            checked(ProgramId::new(PROGRAM)),
        ))
        .bytes()
    }

    fn open(root: &Path) -> MarketOutbox {
        super::open(root)
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
    fn capture_document(membership_program: [u8; 32]) -> CaptureDocument {
        let state = created_state();
        let mut manifest = vec![0, 1];
        manifest.extend(1u32.to_be_bytes());
        manifest.extend((SHARED_STATE_KEY.len() as u16).to_be_bytes());
        manifest.extend(SHARED_STATE_KEY);
        manifest.extend(digest(&state));
        manifest.extend((state.len() as u32).to_be_bytes());
        let mut value = vec![0, 1];
        value.extend(1u32.to_be_bytes());
        value.extend(digest(&manifest));
        let mut head_key = b"\x01\x00\x09progstor".to_vec();
        head_key.extend(membership_program);
        head_key.push(1);
        let (membership, root) = membership(b"lxphost", &head_key, &value);
        let proof = ProgramStateProof {
            membership,
            manifest,
        };
        let store_proof = checked(serde_json::to_vec(&proof));
        let header = super::commit_header(CHAIN_ID, 900, root);
        let commit = super::signed(&header, &super::SIGNED);
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
        let read_proof = ReadProof {
            chain: checked(ChainDomain::new(CHAIN)),
            program: checked(ProgramId::new(PROGRAM)),
            native_state_root: checked(Digest32::new(root)),
            observed_sequence: 1,
            execution_height: 1000,
            block_hash: checked(Digest32::new(header.hash())),
        };
        let mut captured = vec![0; state.len()];
        let mut capture = StateCapture::new(&mut captured);
        for offset in (0..state.len()).step_by(MAX_CHUNK_BYTES) {
            let request = codec::ChunkRequest {
                revision: 1,
                digest: Presence::Present(checked(codec::state_digest(&state))),
                offset: offset as u32,
                requested: MAX_CHUNK_BYTES as u16,
            };
            let mut request_bytes = [0; 46];
            checked(codec::encode_chunk_request(&request, &mut request_bytes));
            let mut response = vec![0; MAX_CHUNK_BYTES + 128];
            let length = checked(queries::read_state_chunk(
                &state,
                &request_bytes,
                &mut response,
            ));
            checked(capture.accept(&read_proof, &response[..length]));
        }
        let (captured, facts) = checked(capture.finish());
        assert_eq!(captured, state);
        let binding = checked(queries::bind_snapshot(
            captured,
            &facts,
            &checked(verified.finality_evidence(checked(Digest32::new(root)))),
            1_700_000_000_000,
        ));
        let row = CapturedEvent {
            snapshot_id: checked(binding.snapshot_id()).bytes(),
            binding,
            event_id: checked(market_event_id(
                &market(),
                &[31; 32],
                0,
                dispatch::CREATE.selector(),
            ))
            .as_str()
            .into(),
            decoder_version: 1,
            action: "CREATE_MARKET".into(),
            ai_event_tag: dispatch::CREATE.selector(),
            source_activity_id: [31; 32],
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

    fn transport(root: &Path, document: &CaptureDocument) -> Result<Admission, MarketEventError> {
        let outbox = open(root);
        checked(outbox.grant(&market()));
        let listener = checked(TcpListener::bind("127.0.0.1:0"));
        let address = checked(listener.local_addr());
        let body = checked(serde_json::to_vec(document));
        let expected = body.clone();
        let peer = thread::spawn(move || {
            let (mut stream, _) = checked(listener.accept());
            checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
            checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
            let request = checked(http::read_request(&mut stream));
            assert_eq!(request.method, "POST");
            assert_eq!(
                request.path,
                "/internal/v1/ai-market/captures/tenant:paxai-markets"
            );
            assert_eq!(request.header("content-type"), Some("application/json"));
            assert_eq!(request.body, expected);
            let capture: CaptureDocument = checked(serde_json::from_slice(&request.body));
            let producer = checked(ProducerGrant::from_authenticated_role(
                IngressRole::Producer,
            ));
            let admission = outbox.admit_document(&producer, &capture);
            let reply = match &admission {
                Ok(admitted) => http::Reply::json(202, checked(serde_json::to_string(admitted))),
                Err(_) => http::Reply::refusal(422, "capture_rejected", None),
            };
            checked(http::write_reply(&mut stream, &reply));
            admission
        });
        let mut stream = checked(TcpStream::connect_timeout(&address, Duration::from_secs(5)));
        checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
        checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
        checked(write!(stream, "POST /internal/v1/ai-market/captures/tenant:paxai-markets HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n", body.len()));
        checked(stream.write_all(&body));
        let mut response = String::new();
        checked(stream.read_to_string(&mut response));
        let admission = checked(peer.join());
        match &admission {
            Ok(admitted) => {
                assert!(response.starts_with("HTTP/1.1 202 Accepted\r\n"));
                assert_eq!(
                    checked(response.split_once("\r\n\r\n").ok_or("HTTP body")).1,
                    checked(serde_json::to_string(admitted))
                );
            }
            Err(_) => assert!(response.starts_with("HTTP/1.1 422 Unprocessable Content\r\n")),
        }
        admission
    }

    fn admit_document(root: &Path, document: &CaptureDocument) -> ProtocolEvent {
        let Admission::Finalized { event, duplicate } = checked(transport(root, document)) else {
            panic!("guest capture must finalize");
        };
        assert!(!duplicate);
        event
    }

    fn credit(transaction: &mut ConsumerTransaction<'_>) -> String {
        let previous = transaction
            .get("balance")
            .map_or(0, |value| checked(value.parse::<u64>()));
        let next = (previous + 7).to_string();
        checked(transaction.set("balance", next.clone()));
        next
    }

    #[test]
    fn guest_capture_transport_restart_and_membership_refusal() {
        let root = scratch("repair-m1329-guest");
        let document = capture_document(PROGRAM);
        let admitted = admit_document(&root.0.join("outbox"), &document);
        let restarted = open(&root.0.join("outbox"));
        let pending = checked(restarted.pending(&market()));
        assert_eq!(pending, vec![admitted.clone()]);
        assert_eq!(admitted.verification(), Verification::BlockCommitted);
        assert_eq!(admitted.subject_sequence(), 1);
        drop(restarted);
        let Admission::Finalized {
            event: retried,
            duplicate,
        } = checked(transport(&root.0.join("outbox"), &document))
        else {
            panic!("idempotent source retry must retain finality");
        };
        assert!(duplicate);
        assert_eq!(retried, admitted);
        assert_eq!(
            checked(open(&root.0.join("outbox")).pending(&market())),
            pending
        );
        let facts: std::collections::BTreeMap<_, _> = admitted
            .facts()
            .iter()
            .map(|fact| (fact.name(), fact.value()))
            .collect();
        assert_eq!(facts.get("ai_action"), Some(&"CREATE_MARKET"));
        assert_eq!(
            facts.get("market_id").copied(),
            Some(hex_encode(&market()).as_str())
        );
        assert_eq!(
            facts.get("snapshot_id").copied(),
            Some(hex_encode(&document.snapshot_id).as_str())
        );
        let before = deliver(&admitted, now());
        let after = deliver(&pending[0], now());
        assert_eq!(before.body, after.body);
        assert_eq!(before.id, after.id);
        let delivered: Value = checked(serde_json::from_slice(&after.body));
        assert_eq!(delivered["event"], checked(serde_json::to_value(&admitted)));
        let receiver = consumer(&root.0.join("consumer"));
        assert_eq!(
            checked(receiver.receive(&after.presentation(now()), credit)),
            Consumed::Applied {
                sequence: 1,
                result: "7".into()
            }
        );
        drop(receiver);
        let receiver = consumer(&root.0.join("consumer"));
        assert_eq!(
            checked(receiver.effect(admitted.subject(), "balance")),
            Some("7".into())
        );
        assert_eq!(
            checked(receiver.receive(&after.presentation(now()), |_| panic!(
                "duplicate must not execute business effect"
            ))),
            Consumed::Duplicate {
                sequence: 1,
                result: "7".into()
            }
        );
        assert_eq!(checked(receiver.applied(admitted.subject())), 1);

        let forged = capture_document([0x99; 32]);
        // The valid certificate signs the wrong program's head. Only membership can refuse it.
        let policy = checked(layerx_platform_webhooks::chain::ChainPolicy::from_native(
            &super::policy().authority,
        ));
        checked(forged.commit.verify(&policy, &forged.store_proof));
        assert!(matches!(
            transport(&root.0.join("refused"), &forged),
            Err(MarketEventError::CommitRejected)
        ));
        let rejected = open(&root.0.join("refused"));
        assert!(checked(rejected.pending(&market())).is_empty());
        assert_eq!(checked(rejected.quarantine(&market())), None);
    }

    fn retained_delivery(root: &Path) -> SignedDelivery {
        let value: Value = checked(serde_json::from_slice(&checked(fs::read(
            root.join("delivery.json"),
        ))));
        let field = |name: &str| checked(value[name].as_str().ok_or("delivery field")).to_owned();
        SignedDelivery {
            id: field("id"),
            timestamp: field("timestamp"),
            key_id: field("key_id"),
            signature: field("signature"),
            body: checked(layerx_platform_webhooks::encoding::hex_decode(&field(
                "body",
            ))),
        }
    }

    #[test]
    fn receiver_process_failure_boundaries_and_durable_effect_replay() {
        const CHILD: &str = "WEBHOOK_REPAIR_M1329_CHILD";
        const BOUNDARY: &str = "WEBHOOK_REPAIR_M1329_BOUNDARY";
        if let Ok(root) = std::env::var(CHILD) {
            let root = Path::new(&root);
            let boundary = checked(std::env::var(BOUNDARY));
            let delivery = retained_delivery(root);
            let receiver = consumer(&root.join("consumer"));
            let result = receiver.receive(&delivery.presentation(now()), |transaction| {
                if boundary == "before_effect" {
                    std::process::exit(79);
                }
                let result = credit(transaction);
                if boundary == "after_effect" {
                    std::process::exit(79);
                }
                if boundary == "before_stage" {
                    checked(fs::create_dir(root.join("consumer/state.json.staged")));
                }
                if boundary == "before_rename" {
                    checked(fs::create_dir(root.join("consumer/state.json")));
                }
                result
            });
            if boundary == "before_stage" || boundary == "before_rename" {
                assert!(matches!(result, Err(WebhookError::Io(_))));
                std::process::exit(79);
            }
            assert_eq!(
                checked(result),
                Consumed::Applied {
                    sequence: 1,
                    result: "7".into()
                }
            );
            std::process::exit(80);
        }
        let scratch = scratch("repair-m1329-crashes");
        let document = capture_document(PROGRAM);
        let event = admit_document(&scratch.0.join("outbox"), &document);
        let delivery = deliver(&event, now());
        for boundary in [
            "before_effect",
            "after_effect",
            "before_stage",
            "before_rename",
            "after_commit",
        ] {
            let root = scratch.0.join(boundary);
            checked(fs::create_dir_all(&root));
            checked(fs::write(
                root.join("delivery.json"),
                checked(serde_json::to_vec(&json!({
                    "id": delivery.id, "timestamp": delivery.timestamp, "key_id": delivery.key_id,
                    "signature": delivery.signature, "body": hex_encode(&delivery.body),
                }))),
            ));
            let mut child = checked(
                Command::new(checked(std::env::current_exe()))
                    .args([
                        "--exact",
                        concat!(
                            module_path!(),
                            "::receiver_process_failure_boundaries_and_durable_effect_replay"
                        )
                        .split_once("::")
                        .expect("test crate prefix")
                        .1,
                        "--nocapture",
                    ])
                    .env(CHILD, &root)
                    .env(BOUNDARY, boundary)
                    .stdin(Stdio::null())
                    .stdout(Stdio::null())
                    .spawn(),
            );
            let deadline = Instant::now() + Duration::from_secs(10);
            let status = loop {
                if let Some(status) = checked(child.try_wait()) {
                    break status;
                }
                if Instant::now() >= deadline {
                    checked(child.kill());
                    checked(child.wait());
                    panic!("receiver child exceeded deadline at {boundary}");
                }
                thread::sleep(Duration::from_millis(10));
            };
            assert_eq!(
                status.code(),
                Some(if boundary == "after_commit" { 80 } else { 79 }),
                "{boundary}"
            );
            if boundary == "before_stage" {
                checked(fs::remove_dir(root.join("consumer/state.json.staged")));
            }
            if boundary == "before_rename" {
                let staged: Value = checked(serde_json::from_slice(&checked(fs::read(
                    root.join("consumer/state.json.staged"),
                ))));
                assert_eq!(
                    staged["subjects"][event.subject().as_str()]["effects"]["balance"],
                    "7"
                );
                assert_eq!(
                    staged["subjects"][event.subject().as_str()]["last_sequence"],
                    1
                );
                assert_eq!(
                    checked(
                        staged["subjects"][event.subject().as_str()]["applied"]
                            .as_object()
                            .ok_or("staged ledger")
                    )
                    .len(),
                    1
                );
                checked(fs::remove_dir(root.join("consumer/state.json")));
            }
            let receiver = consumer(&root.join("consumer"));
            let committed = boundary == "after_commit";
            assert_eq!(
                checked(receiver.effect(event.subject(), "balance")),
                if committed { Some("7".into()) } else { None },
                "{boundary}"
            );
            assert_eq!(
                checked(receiver.applied(event.subject())),
                usize::from(committed),
                "{boundary}"
            );
            assert_eq!(
                checked(receiver.last_sequence(event.subject())),
                u64::from(committed),
                "{boundary}"
            );
            let replay = checked(receiver.receive(&delivery.presentation(now()), credit));
            assert_eq!(
                replay,
                if committed {
                    Consumed::Duplicate {
                        sequence: 1,
                        result: "7".into(),
                    }
                } else {
                    Consumed::Applied {
                        sequence: 1,
                        result: "7".into(),
                    }
                },
                "{boundary}"
            );
            drop(receiver);
            let receiver = consumer(&root.join("consumer"));
            assert_eq!(
                checked(receiver.effect(event.subject(), "balance")),
                Some("7".into()),
                "{boundary}"
            );
            assert_eq!(
                checked(receiver.receive(&delivery.presentation(now()), |_| panic!(
                    "replay applied the effect twice at {boundary}"
                ))),
                Consumed::Duplicate {
                    sequence: 1,
                    result: "7".into()
                }
            );
            assert_eq!(checked(receiver.applied(event.subject())), 1, "{boundary}");
            assert_eq!(
                checked(receiver.last_sequence(event.subject())),
                1,
                "{boundary}"
            );
            let durable: Value = checked(serde_json::from_slice(&checked(fs::read(
                root.join("consumer/state.json"),
            ))));
            assert_eq!(
                durable["subjects"][event.subject().as_str()]["effects"]["balance"],
                "7"
            );
            let ledger = &durable["subjects"][event.subject().as_str()];
            assert_eq!(ledger["last_sequence"], 1, "{boundary}");
            let applied = checked(ledger["applied"].as_object().ok_or("durable ledger"));
            assert_eq!(applied.len(), 1, "{boundary}");
            let entry = &applied[event.id().as_str()];
            assert_eq!(entry["sequence"], 1, "{boundary}");
            assert_eq!(entry["result"], "7", "{boundary}");
            assert_eq!(
                entry["body_digest"],
                json!(digest(&delivery.body)),
                "{boundary}"
            );
        }
    }
}
