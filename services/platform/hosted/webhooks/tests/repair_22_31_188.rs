use ed25519_dalek::{Signer, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
};
use layerx_indexer::ai_market::{BlockCommitProof, FinalityAuthority};
use layerx_platform_webhooks::{
    ai_market::*,
    chain::*,
    encoding::{fixed_hex, hex_decode, hex_encode},
    hosted::kms_idempotency_scope,
    trusted::{AuthorityReceiptVerifier, IngressRole},
    EndpointId, Principal, ProtocolEvent, Verification,
};
use layerx_programs_ai_market::{
    codec, dispatch,
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{self, CaptureFacts, ReadProof},
    registry, registry_ops, state, *,
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    fs,
    net::TcpListener,
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    thread,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

fn digest(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

const CHAIN: [u8; 32] = [10; 32];
const PROGRAM: [u8; 32] = [11; 32];
const CHAIN_ID: &str = "webhook-repair-chain";
const KEY: &str = "whk_repair";
const BINARY: &str = env!("CARGO_BIN_EXE_layerx-webhooks");
fn checked<T, E: std::fmt::Debug>(r: Result<T, E>) -> T {
    r.unwrap_or_else(|e| panic!("{e:?}"))
}
fn principal() -> Principal {
    checked(Principal::new("tenant:repair"))
}
fn endpoint() -> EndpointId {
    checked(EndpointId::new("whep_repair"))
}
fn now() -> u64 {
    checked(SystemTime::now().duration_since(UNIX_EPOCH)).as_secs()
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
struct Scratch(PathBuf);
impl Scratch {
    fn new(name: &str) -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-22-31-188-{name}-{}-{}",
            std::process::id(),
            checked(SystemTime::now().duration_since(UNIX_EPOCH)).as_nanos()
        ));
        checked(fs::create_dir_all(&path));
        Self(path)
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn run(args: &[&str]) {
    let result = checked(Command::new(args[0]).args(&args[1..]).output());
    assert!(
        result.status.success(),
        "{args:?}: {}",
        String::from_utf8_lossy(&result.stderr)
    );
}
fn text(path: &Path) -> &str {
    checked(path.to_str().ok_or("UTF-8 path"))
}
fn port() -> u16 {
    checked(checked(TcpListener::bind("127.0.0.1:0")).local_addr()).port()
}
struct Process(Child);
impl Drop for Process {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

// The actual CREATE evaluator, envelope codec and state encoder produce the state.
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
        height: height as i64,
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
// Canonical one-leaf ICS23 IAVL tree, linked through a one-store commitment.
// The separate keeper-vector test below checks compatibility with real multi-leaf proofs.
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
fn capture() -> (CaptureDocument, ProtocolEvent) {
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
            observed_sequence: 1,
            execution_height: 1000,
            block_hash: checked(Digest32::new(header.hash())),
        },
        revision: 1,
        digest: checked(codec::state_digest(&state)),
        total_bytes: state.len() as u32,
        chunks: 1,
    };
    let binding = checked(queries::bind_snapshot(
        &state,
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
    let document = checked(CaptureDocument::from_capture(&row, &evidence));
    let scratch = Scratch::new("capture");
    let outbox = checked(MarketOutbox::open_portable(
        &scratch.0,
        principal(),
        policy(),
        keys(),
    ));
    checked(outbox.grant(&market()));
    let Admission::Finalized { event, .. } = checked(outbox.admit(
        &checked(ProducerGrant::from_authenticated_role(
            IngressRole::Producer,
        )),
        &row,
        &evidence,
    )) else {
        panic!("finalized")
    };
    (document, event)
}

fn material(root: &Path) {
    let p = |name: &str| root.join(name).to_string_lossy().into_owned();
    run(&[
        "openssl",
        "req",
        "-x509",
        "-newkey",
        "ed25519",
        "-nodes",
        "-keyout",
        &p("ca-key.pem"),
        "-out",
        &p("ca.pem"),
        "-days",
        "1",
        "-subj",
        "/CN=repair-ca",
        "-addext",
        "basicConstraints=critical,CA:TRUE",
        "-addext",
        "keyUsage=critical,keyCertSign,cRLSign",
    ]);
    for (name, usage, san) in [
        ("server", "serverAuth", "DNS:localhost,IP:127.0.0.1"),
        (
            "producer",
            "clientAuth",
            "URI:urn:layerx:webhooks:role:producer",
        ),
        (
            "operator",
            "clientAuth",
            "URI:urn:layerx:webhooks:role:operator",
        ),
    ] {
        run(&[
            "openssl",
            "req",
            "-new",
            "-newkey",
            "ed25519",
            "-nodes",
            "-keyout",
            &p(&format!("{name}-key.pem")),
            "-out",
            &p(&format!("{name}.csr")),
            "-subj",
            &format!("/CN={name}"),
        ]);
        checked(fs::write(root.join(format!("{name}.ext")),format!("basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage={usage}\nsubjectAltName={san}\n")));
        run(&[
            "openssl",
            "x509",
            "-req",
            "-in",
            &p(&format!("{name}.csr")),
            "-CA",
            &p("ca.pem"),
            "-CAkey",
            &p("ca-key.pem"),
            "-CAcreateserial",
            "-out",
            &p(&format!("{name}.pem")),
            "-days",
            "1",
            "-extfile",
            &p(&format!("{name}.ext")),
        ]);
    }
    for name in ["ca", "server"] {
        run(&[
            "openssl",
            "x509",
            "-in",
            &p(&format!("{name}.pem")),
            "-outform",
            "DER",
            "-out",
            &p(&format!("{name}.der")),
        ]);
    }
    run(&[
        "openssl",
        "pkcs8",
        "-topk8",
        "-nocrypt",
        "-in",
        &p("server-key.pem"),
        "-outform",
        "DER",
        "-out",
        &p("server-key.der"),
    ]);
    run(&[
        "openssl",
        "pkcs12",
        "-export",
        "-inkey",
        &p("producer-key.pem"),
        "-in",
        &p("producer.pem"),
        "-certfile",
        &p("ca.pem"),
        "-out",
        &p("client.p12"),
        "-passout",
        "pass:repair-only",
    ]);
}

#[test]
fn finding_1_complete_authority_response_and_strict_refusals() {
    let scratch = Scratch::new("authority");
    material(&scratch.0);
    let fixture: Value = checked(serde_json::from_str(include_str!(
        "../../gateway/tests/fixtures/maintained-authority.json"
    )));
    let mut authority = fixture["authority"].clone();
    let bytes = checked(hex_decode(checked(
        authority["batch_evidence"]["header_hex"]
            .as_str()
            .ok_or("header"),
    )));
    let pin = checked(layerx_platform_gateway::configured_sequencer(
        checked(fixture["sequencer_id"].as_str().ok_or("id")),
        checked(fixture["sequencer_public_key"].as_str().ok_or("key")),
        checked(fixture["first_batch"].as_str().ok_or("first")),
        checked(fixture["last_batch"].as_str().ok_or("last")),
    ));
    let header_signature = checked(fixed_hex::<64>(checked(
        authority["batch_evidence"]["header_signature"]
            .as_str()
            .ok_or("signature"),
    )));
    let verified_header = checked(layerx_proof::inclusion::verify_header(
        &bytes,
        &header_signature,
        &pin,
    ));
    let network = verified_header.header().network_id();
    authority["receipt"] = fixture["receipt_hex"].clone();
    authority["protocol_network_id"] = json!(network);
    let activity = checked(authority["activity_id"].as_str().ok_or("activity"));
    let receipt = json!({"activity_id":activity,"receipt":fixture["receipt_hex"]});
    let receipt = checked(serde_json::to_vec(&receipt));

    let identity = checked(native_tls::Identity::from_pkcs12(
        &checked(fs::read(scratch.0.join("client.p12"))),
        "repair-only",
    ));
    let verifier = checked(AuthorityReceiptVerifier::new(
        "https://localhost:443",
        "https://localhost:443",
        "component".into(),
        "authority".into(),
        &checked(fs::read(scratch.0.join("ca.der"))),
        identity,
        pin,
        "layerx-authority-test".into(),
        "3".into(),
        network,
    ));
    let operation = checked(verifier.verify_documents(
        activity,
        &receipt,
        &checked(serde_json::to_vec(&authority)),
    ));
    assert_eq!(operation.activity_id(), checked(fixed_hex::<32>(activity)));
    for (field, value) in [
        ("unexpected", json!(true)),
        ("receipt", json!("00")),
        ("protocol_network_id", json!(network + 1)),
        ("network_id", json!("foreign")),
        ("wire_version", json!("9")),
        ("activity_id", json!("00".repeat(32))),
    ] {
        let mut changed = authority.clone();
        changed[field] = value;
        assert!(
            verifier
                .verify_documents(activity, &receipt, &checked(serde_json::to_vec(&changed)))
                .is_err(),
            "{field}"
        );
    }
    for field in ["receipt", "protocol_network_id"] {
        let mut changed = authority.clone();
        checked(changed.as_object_mut().ok_or("object")).remove(field);
        assert!(verifier
            .verify_documents(activity, &receipt, &checked(serde_json::to_vec(&changed)))
            .is_err());
    }
}

#[test]
fn finding_2_fixed_kms_scope_preserves_full_public_key_and_isolation() {
    let tenant = principal();
    let other = checked(Principal::new("tenant:other"));
    let endpoint = endpoint();
    let another = checked(EndpointId::new("whep_other"));
    for n in [64, 128] {
        let public = "a".repeat(n);
        let registration = checked(kms_idempotency_scope("register", &tenant, None, &public));
        let rotation = checked(kms_idempotency_scope(
            "rotate",
            &tenant,
            Some(&endpoint),
            &public,
        ));
        assert_eq!(registration.len(), 64);
        assert_eq!(rotation.len(), 64);
        assert!(registration.bytes().all(|b| b.is_ascii_hexdigit()));
        assert_eq!(
            registration,
            checked(kms_idempotency_scope("register", &tenant, None, &public))
        );
        assert_eq!(
            rotation,
            checked(kms_idempotency_scope(
                "rotate",
                &tenant,
                Some(&endpoint),
                &public
            ))
        );
        assert_ne!(registration, rotation);
        assert_ne!(
            registration,
            checked(kms_idempotency_scope("register", &other, None, &public))
        );
        assert_ne!(
            rotation,
            checked(kms_idempotency_scope(
                "rotate",
                &tenant,
                Some(&another),
                &public
            ))
        );
        assert_ne!(
            registration,
            checked(kms_idempotency_scope(
                "register",
                &tenant,
                None,
                &format!("{}b", &public[..n - 1])
            ))
        );
    }
    assert!(kms_idempotency_scope("register", &tenant, None, &"a".repeat(129)).is_err());
    assert!(kms_idempotency_scope("rotate", &tenant, None, "a").is_err());
}

#[test]
fn finding_17_committed_membership_binds_key_value_proof_and_market() {
    let (document, event) = capture();
    assert_eq!(event.verification(), Verification::BlockCommitted);
    let proof: ProgramStateProof = checked(serde_json::from_slice(&document.store_proof));
    let committed = checked(
        document
            .commit
            .verify(&policy().consensus, &document.store_proof),
    );
    checked(proof.verify(committed.app_hash, &PROGRAM, &document.state));
    assert!(proof
        .verify(committed.app_hash, &[99; 32], &document.state)
        .is_err());
    let mut state = document.state.clone();
    state[10] ^= 1;
    assert!(proof.verify(committed.app_hash, &PROGRAM, &state).is_err());
    let mut altered = proof.clone();
    let last = altered.membership.proof.len() - 1;
    altered.membership.proof[last] ^= 1;
    assert!(altered
        .verify(committed.app_hash, &PROGRAM, &document.state)
        .is_err());
    let mut forged = document.clone();
    forged.store_proof = vec![1, 2, 3];
    let mut forged_binding = checked(queries::SnapshotBinding::decode(&forged.binding));
    forged_binding.store_proof_digest = checked(Digest32::new(digest(&forged.store_proof)));
    forged.binding = vec![0; queries::BINDING_MAX_BYTES];
    let n = checked(forged_binding.encode(&mut forged.binding));
    forged.binding.truncate(n);
    forged.snapshot_id = checked(forged_binding.snapshot_id()).bytes();
    // Even a genuine consensus certificate cannot authorize self-declared state bytes.
    let scratch = Scratch::new("forgery");
    let outbox = checked(MarketOutbox::open_portable(
        &scratch.0,
        principal(),
        policy(),
        keys(),
    ));
    checked(outbox.grant(&market()));
    assert!(outbox
        .admit_document(
            &checked(ProducerGrant::from_authenticated_role(
                IngressRole::Producer
            )),
            &forged
        )
        .is_err());
    assert!(checked(outbox.pending(&market())).is_empty());
    let mut binding = checked(queries::SnapshotBinding::decode(&document.binding));
    binding.market = checked(MarketId::new([90; 32]));
    let mut wrong = document.clone();
    wrong.binding = vec![0; queries::BINDING_MAX_BYTES];
    let n = checked(binding.encode(&mut wrong.binding));
    wrong.binding.truncate(n);
    wrong.snapshot_id = checked(binding.snapshot_id()).bytes();
    wrong.event_id = checked(market_event_id(
        &[90; 32],
        &wrong.source_activity_id,
        wrong.effect_ordinal,
        wrong.ai_event_tag,
    ))
    .as_str()
    .into();
    checked(outbox.grant(&[90; 32]));
    assert!(outbox
        .admit_document(
            &checked(ProducerGrant::from_authenticated_role(
                IngressRole::Producer
            )),
            &wrong
        )
        .is_err());
}

#[test]
fn finding_17_real_keeper_membership_vectors_remain_interoperable() {
    let fixture: Value = checked(serde_json::from_str(include_str!(
        "../../../../../programs/tests/vectors/ics23-market-namespaces-app-hash.json"
    )));
    for entry in checked(fixture["entries"].as_array().ok_or("entries")) {
        for name in ["namespace", "chain_commit", "receipt"] {
            let e = &entry[name];
            let decode = |key: &str| checked(hex_decode(checked(e[key].as_str().ok_or("hex"))));
            let membership = Membership {
                proof: decode("proof"),
                multistore: decode("multistore_proof"),
            };
            let root = checked(fixed_hex(checked(e["app_hash"].as_str().ok_or("root"))));
            let store = if name == "chain_commit" {
                b"sequencer".as_slice()
            } else {
                b"lxphost".as_slice()
            };
            checked(membership.verify(root, store, &decode("key"), &decode("value")));
            let mut wrong = decode("key");
            wrong.push(0);
            assert!(membership
                .verify(root, store, &wrong, &decode("value"))
                .is_err());
        }
    }
}

fn node_evidence() -> (NodeReceiptEvidence, [u8; 32]) {
    let fixture: Value = checked(serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    let case = &fixture["cases"][0];
    let receipt = checked(hex_decode(
        checked(case["signedReceipt"].as_str().ok_or("receipt")).trim_start_matches("0x"),
    ));
    let activity = checked(fixed_hex(
        checked(case["receiptJson"]["txHash"].as_str().ok_or("hash")).trim_start_matches("0x"),
    ));
    let key = SigningKey::from_bytes(&[0x5e; 32]);
    let mut table = vec![0x6b; 32];
    table.extend(key.verifying_key().to_bytes());
    table.extend(1000u64.to_be_bytes());
    table.extend(1999u64.to_be_bytes());
    let (governed_keys, root) = membership(b"sequencer", &[3], &table);
    let mut batch = Vec::new();
    for n in [1000u64, 1, 1] {
        batch.extend(n.to_be_bytes());
    }
    batch.extend([0; 32]);
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend([0; 32]);
    sequence.extend(1u64.to_be_bytes());
    sequence.extend(activity);
    batch.extend(digest(&sequence));
    batch.extend(digest(&activity));
    let mut signing = b"PAXEER/v1/sequence-batch\0".to_vec();
    signing.extend(CHAIN_ID.as_bytes());
    signing.extend(&batch);
    let signature = key.sign(&digest(&signing)).to_bytes();
    batch.extend([0x6b; 32]);
    batch.extend(signature);
    let mut raw = b"PAXEER/v1/sequence-batch\0".to_vec();
    raw.extend(batch);
    let tx = checked(hex_decode(
        checked(case["rawTransaction"].as_str().ok_or("tx")).trim_start_matches("0x"),
    ));
    let transactions = vec![raw, tx];
    let data = merkle(
        &transactions
            .iter()
            .map(|tx| digest(tx).to_vec())
            .collect::<Vec<_>>(),
    );
    let h = header(root, 1000, data);
    let c = signed(&h);
    let commit = checked(CommitCertificate::from_proof(&BlockCommitProof {
        header: &h,
        commit: &c,
        store_proof: &governed_keys.proof,
    }));
    (
        NodeReceiptEvidence {
            receipt,
            commit,
            governed_keys,
            transactions,
            activity_ids: vec![activity],
        },
        activity,
    )
}

#[test]
fn finding_34_governed_node_receipt_committed_block_and_foreign_chain_refusal() {
    let (evidence, activity) = node_evidence();
    let operation = checked(evidence.verify(&policy().consensus, activity));
    assert_eq!(operation.activity_id(), activity);
    let mut foreign = policy().consensus;
    foreign.chain_id = "foreign-chain".into();
    assert!(evidence.verify(&foreign, activity).is_err());
    let mut changed = evidence.clone();
    changed.activity_ids[0][0] ^= 1;
    assert!(changed.verify(&policy().consensus, activity).is_err());
    let mut changed = evidence.clone();
    changed.transactions[1][0] ^= 1;
    assert!(changed.verify(&policy().consensus, activity).is_err());
    let mut changed = evidence.clone();
    changed.governed_keys.proof[3] ^= 1;
    assert!(changed.verify(&policy().consensus, activity).is_err());
    let scratch = Scratch::new("receipt");
    material(&scratch.0);
    let identity = checked(native_tls::Identity::from_pkcs12(
        &checked(fs::read(scratch.0.join("client.p12"))),
        "repair-only",
    ));
    let verifier = checked(layerx_platform_webhooks::trusted::ReceiptVerifier::new(
        "https://localhost:443",
        "https://localhost:443",
        "node".into(),
        &checked(fs::read(scratch.0.join("ca.der"))),
        identity,
        policy().consensus,
    ));
    let block = checked(
        evidence
            .commit
            .verify(&policy().consensus, &evidence.governed_keys.proof),
    )
    .block_hash;
    let verified = checked(verifier.verify_evidence(&evidence, activity, block));
    assert_eq!(verified.verification(), Verification::BlockCommitted);
    assert!(verifier
        .verify_evidence(&evidence, activity, [9; 32])
        .is_err());
    let (_, event) = capture();
    assert!(layerx_platform_webhooks::hosted::subscription_accepts(
        &[layerx_platform_webhooks::EventKind::Program],
        verified.verification(),
        &event
    ));
    let mut weaker = checked(serde_json::to_value(&event));
    for fact in checked(weaker["facts"].as_array_mut().ok_or("facts")) {
        fact["verification"] = json!("receipt-verified");
    }
    weaker["verification"] = json!("receipt-verified");
    let weaker: ProtocolEvent = checked(serde_json::from_value(weaker));
    checked(weaker.validate());
    assert!(!layerx_platform_webhooks::hosted::subscription_accepts(
        &[],
        verified.verification(),
        &weaker
    ));
    let weak_delivery = checked(SignedDelivery::sign(
        &weaker,
        &endpoint(),
        KEY,
        &SigningKey::from_bytes(&[42; 32]),
        now(),
    ));
    assert!(matches!(
        receiver(&scratch.0.join("weak-consumer"))
            .receive(&weak_delivery.presentation(now()), credit),
        Err(layerx_platform_webhooks::WebhookError::VerificationRequired)
    ));
    let path = scratch.0.join("node-evidence.json");
    checked(fs::write(&path, checked(serde_json::to_vec(&evidence))));
    drop(evidence);
    let recovered: NodeReceiptEvidence = checked(serde_json::from_slice(&checked(fs::read(path))));
    assert_eq!(
        checked(recovered.verify(&policy().consensus, activity)).receipt_digest(),
        operation.receipt_digest()
    );
}

fn receiver(root: &Path) -> MarketConsumer {
    checked(MarketConsumer::open(
        root,
        BTreeMap::from([(
            KEY.into(),
            SigningKey::from_bytes(&[42; 32]).verifying_key().to_bytes(),
        )]),
    ))
}
fn delivery() -> (SignedDelivery, ProtocolEvent) {
    let (_, event) = capture();
    let delivery = checked(SignedDelivery::sign(
        &event,
        &endpoint(),
        KEY,
        &SigningKey::from_bytes(&[42; 32]),
        now(),
    ));
    (delivery, event)
}
fn credit(tx: &mut ConsumerTransaction<'_>) -> String {
    let balance = tx.get("balance").map_or(0, |n| checked(n.parse::<u64>()));
    let balance = (balance + 7).to_string();
    checked(tx.set("balance", balance.clone()));
    balance
}

#[test]
fn finding_5_business_effect_and_ledger_survive_disk_failure_and_restart() {
    let scratch = Scratch::new("disk");
    let (delivery, event) = delivery();
    let consumer = receiver(&scratch.0);
    let staged = scratch.0.join("state.json.staged");
    checked(fs::create_dir(&staged));
    assert!(consumer
        .receive(&delivery.presentation(now()), credit)
        .is_err());
    drop(consumer);
    let consumer = receiver(&scratch.0);
    assert_eq!(checked(consumer.effect(event.subject(), "balance")), None);
    assert_eq!(checked(consumer.last_sequence(event.subject())), 0);
    checked(fs::remove_dir(staged));
    assert!(matches!(
        checked(consumer.receive(&delivery.presentation(now()), credit)),
        Consumed::Applied { sequence: 1, .. }
    ));
    drop(consumer);
    let consumer = receiver(&scratch.0);
    assert_eq!(
        checked(consumer.effect(event.subject(), "balance")),
        Some("7".into())
    );
    assert!(matches!(
        checked(consumer.receive(&delivery.presentation(now()), credit)),
        Consumed::Duplicate { sequence: 1, .. }
    ));
    assert_eq!(
        checked(consumer.effect(event.subject(), "balance")),
        Some("7".into())
    );
    assert_eq!(checked(consumer.applied(event.subject())), 1);
}

#[test]
fn finding_5_process_death_before_and_after_atomic_commit() {
    const CHILD: &str = "WEBHOOK_REPAIR_CONSUMER_BOUNDARY";
    if let Ok(root) = std::env::var(CHILD) {
        let value: Value = checked(serde_json::from_slice(&checked(fs::read(
            Path::new(&root).join("delivery.json"),
        ))));
        let delivery = SignedDelivery {
            id: checked(value["id"].as_str().ok_or("id")).into(),
            timestamp: checked(value["timestamp"].as_str().ok_or("timestamp")).into(),
            key_id: KEY.into(),
            signature: checked(value["signature"].as_str().ok_or("signature")).into(),
            body: checked(hex_decode(checked(value["body"].as_str().ok_or("body")))),
        };
        let consumer = receiver(&Path::new(&root).join("consumer"));
        let before = std::env::var("WEBHOOK_REPAIR_EXIT_BEFORE").is_ok();
        checked(consumer.receive(&delivery.presentation(now()), |tx| {
            let result = credit(tx);
            if before {
                std::process::exit(79);
            }
            result
        }));
        std::process::exit(80);
    }
    let scratch = Scratch::new("process");
    let (delivery, event) = delivery();
    checked(fs::write(
        scratch.0.join("delivery.json"),
        checked(serde_json::to_vec(
            &json!({"id":delivery.id,"timestamp":delivery.timestamp,"signature":delivery.signature,"body":hex_encode(&delivery.body)}),
        )),
    ));
    for before in [true, false] {
        let mut command = Command::new(checked(std::env::current_exe()));
        command
            .args([
                "--exact",
                "finding_5_process_death_before_and_after_atomic_commit",
                "--nocapture",
            ])
            .env(CHILD, &scratch.0)
            .stdout(Stdio::null());
        if before {
            command.env("WEBHOOK_REPAIR_EXIT_BEFORE", "1");
        } else {
            command.env_remove("WEBHOOK_REPAIR_EXIT_BEFORE");
        }
        let status = checked(command.status());
        assert_eq!(status.code(), Some(if before { 79 } else { 80 }));
        let consumer = receiver(&scratch.0.join("consumer"));
        assert_eq!(
            checked(consumer.effect(event.subject(), "balance")),
            if before { None } else { Some("7".into()) }
        );
        assert_eq!(
            checked(consumer.last_sequence(event.subject())),
            if before { 0 } else { 1 }
        );
    }
    let consumer = receiver(&scratch.0.join("consumer"));
    assert!(matches!(
        checked(consumer.receive(&delivery.presentation(now()), credit)),
        Consumed::Duplicate { .. }
    ));
    assert_eq!(
        checked(consumer.effect(event.subject(), "balance")),
        Some("7".into())
    );
}

fn service_environment(root: &Path, listen: u16, redis: u16) -> BTreeMap<String, String> {
    let path = |name: &str| root.join(name).to_string_lossy().into_owned();
    let secret = |name: &str, value: &str| {
        checked(fs::write(root.join(name), value));
        path(name)
    };
    let unused = format!("https://localhost:{}", port());
    let mut env: BTreeMap<String, String> = [
        ("LAYERX_WEBHOOKS_LISTEN", format!("127.0.0.1:{listen}")),
        ("LAYERX_WEBHOOKS_ROLE", "ingress".into()),
        ("LAYERX_WEBHOOKS_LISTENER", "tls".into()),
        ("LAYERX_WEBHOOKS_TLS_CERT_DER", path("server.der")),
        ("LAYERX_WEBHOOKS_TLS_KEY_DER", path("server-key.der")),
        ("LAYERX_WEBHOOKS_INGRESS_CLIENT_CA_DER", path("ca.der")),
        ("LAYERX_WEBHOOKS_INTERNAL_CA_DER", path("ca.der")),
        ("LAYERX_WEBHOOKS_PUBLIC_CA_DER", path("ca.der")),
        ("LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12", path("client.p12")),
        (
            "LAYERX_WEBHOOKS_CLIENT_IDENTITY_PASSWORD_FILE",
            secret("identity-password", "repair-only"),
        ),
        (
            "LAYERX_WEBHOOKS_REDIS_URL",
            format!("rediss://localhost:{redis}"),
        ),
        (
            "LAYERX_WEBHOOKS_REDIS_USERNAME_FILE",
            secret("redis-user", "webhooks"),
        ),
        (
            "LAYERX_WEBHOOKS_REDIS_PASSWORD_FILE",
            secret("redis-password", "repair-password"),
        ),
        (
            "LAYERX_WEBHOOKS_CURSOR_KEY_FILE",
            secret("cursor-key", &"01".repeat(32)),
        ),
        ("LAYERX_WEBHOOKS_KMS_URL", unused.clone()),
        (
            "LAYERX_WEBHOOKS_KMS_TOKEN_FILE",
            secret("kms-token", "kms-repair"),
        ),
        ("LAYERX_WEBHOOKS_INSTANCE_ID", "webhooks-repair".into()),
        ("LAYERX_WEBHOOKS_NODE_URL", unused.clone()),
        ("LAYERX_WEBHOOKS_COMET_URL", unused.clone()),
        (
            "LAYERX_WEBHOOKS_NODE_TOKEN_FILE",
            secret("node-token", "node-repair"),
        ),
        ("LAYERX_WEBHOOKS_CHAIN_POLICY_FILE", path("chain.json")),
        (
            "LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE",
            secret("source-token", "source-repair"),
        ),
        (
            "LAYERX_WEBHOOKS_OPERATOR_TOKEN_FILE",
            secret("operator-token", "operator-repair"),
        ),
        ("LAYERX_WEBHOOKS_MARKET_OUTBOX_DIR", path("outbox")),
        ("LAYERX_WEBHOOKS_MARKET_POLICY_FILE", path("market.json")),
        ("LAYERX_WEBHOOKS_DISPATCH_INTERVAL_SECONDS", "1".into()),
    ]
    .into_iter()
    .map(|(k, v)| (k.into(), v))
    .collect();
    for stem in ["JOURNEY", "PAYMENT", "APPROVAL", "PROGRAM"] {
        env.insert(format!("LAYERX_WEBHOOKS_{stem}_SOURCE_URL"), unused.clone());
        env.insert(
            format!("LAYERX_WEBHOOKS_{stem}_SOURCE_TOKEN_FILE"),
            secret("source-client-token", "source-client-repair"),
        );
    }
    checked(fs::write(
        root.join("chain.json"),
        checked(serde_json::to_vec(&policy().consensus)),
    ));
    checked(fs::write(
        root.join("market.json"),
        checked(serde_json::to_vec(&policy())),
    ));
    env
}
fn launch(root: &Path, env: &BTreeMap<String, String>) -> Process {
    let log = checked(
        fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(root.join("service.log")),
    );
    Process(checked(
        Command::new(BINARY)
            .env_clear()
            .envs(env)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(log)
            .spawn(),
    ))
}
fn capture_http(
    root: &Path,
    listen: u16,
    role: &str,
    token: &str,
    service: &mut Process,
) -> (u16, Value) {
    let p = |name: &str| root.join(name).to_string_lossy().into_owned();
    let url = format!("https://localhost:{listen}/internal/v1/ai-market/captures/tenant:repair");
    for _ in 0..100 {
        assert!(
            checked(service.0.try_wait()).is_none(),
            "service exited: {}",
            checked(fs::read_to_string(root.join("service.log")))
        );
        let result = checked(
            Command::new("curl")
                .args([
                    "-sS",
                    "--max-time",
                    "3",
                    "--cacert",
                    &p("ca.pem"),
                    "--cert",
                    &p(&format!("{role}.pem")),
                    "--key",
                    &p(&format!("{role}-key.pem")),
                    "-H",
                    "content-type: application/json",
                    "-H",
                    &format!("authorization: Bearer {token}"),
                    "--data-binary",
                    &format!("@{}", p("capture.json")),
                    "-w",
                    "\n%{http_code}",
                    &url,
                ])
                .output(),
        );
        if result.status.success() {
            let output = checked(String::from_utf8(result.stdout));
            let (body, status) = checked(output.rsplit_once('\n').ok_or("status"));
            return (checked(status.parse()), checked(serde_json::from_str(body)));
        }
        thread::sleep(Duration::from_millis(50));
    }
    panic!(
        "ingress did not listen: {}",
        checked(fs::read_to_string(root.join("service.log")))
    )
}
fn redis_json(root: &Path, redis: u16) -> Value {
    let output = checked(
        Command::new("redis-cli")
            .args([
                "--tls",
                "--cacert",
                text(&root.join("ca.pem")),
                "-h",
                "localhost",
                "-p",
                &redis.to_string(),
                "--user",
                "webhooks",
                "--no-auth-warning",
                "-a",
                "repair-password",
                "--raw",
                "HGET",
                &format!(
                    "webhooks:principal:{}",
                    hex_encode(&digest(principal().as_str().as_bytes()))
                ),
                "json",
            ])
            .output(),
    );
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    checked(serde_json::from_slice(&output.stdout))
}
#[test]
fn finding_4_authenticated_capture_service_restart_and_durable_handoff() {
    let scratch = Scratch::new("service");
    let root = &scratch.0;
    material(root);
    let (document, event) = capture();
    checked(fs::write(
        root.join("capture.json"),
        checked(serde_json::to_vec(&document)),
    ));
    let hub = checked(MarketHub::new(&root.join("outbox"), policy(), keys()));
    checked(hub.grant(&principal(), &market()));
    let listen = port();
    let redis = port();
    let environment = service_environment(root, listen, redis);
    let outbox_dir = root
        .join("outbox")
        .join(hex_encode(&digest(principal().as_str().as_bytes())));
    let outbox = || {
        checked(MarketOutbox::open_portable(
            &outbox_dir,
            principal(),
            policy(),
            keys(),
        ))
    };
    let mut service = launch(root, &environment);
    assert_eq!(
        capture_http(root, listen, "operator", "source-repair", &mut service).0,
        403
    );
    assert_eq!(
        capture_http(root, listen, "producer", "wrong-token", &mut service).0,
        401
    );
    let (status, response) = capture_http(root, listen, "producer", "source-repair", &mut service);
    assert_eq!(status, 202, "{response}");
    assert_eq!(response["Finalized"]["duplicate"], false);
    // Redis is stopped: successful admission must leave durable pending work.
    thread::sleep(Duration::from_millis(1200));
    assert_eq!(checked(outbox().pending(&market())), vec![event.clone()]);
    drop(service);
    assert_eq!(checked(outbox().pending(&market())), vec![event.clone()]);
    let redis_config=format!("bind 127.0.0.1\nport 0\ntls-port {redis}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\nuser default off\nuser webhooks on >repair-password ~webhooks:* +@all\nsave \"\"\nappendonly yes\ndir {}\n",text(&root.join("server.pem")),text(&root.join("server-key.pem")),text(&root.join("ca.pem")),text(root));
    checked(fs::write(root.join("redis.conf"), redis_config));
    let _redis = Process(checked(
        Command::new("redis-server")
            .arg(root.join("redis.conf"))
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn(),
    ));
    let mut service = launch(root, &environment);
    let (status, response) = capture_http(root, listen, "producer", "source-repair", &mut service);
    assert_eq!(status, 202, "{response}");
    assert_eq!(response["Finalized"]["duplicate"], true);
    let mut handed_off = false;
    for _ in 0..100 {
        if checked(outbox().pending(&market())).is_empty() {
            handed_off = true;
            break;
        }
        thread::sleep(Duration::from_millis(100));
    }
    assert!(
        handed_off,
        "durable capture must reach the real hosted Redis repository"
    );
    let shard = redis_json(root, redis);
    assert_eq!(shard["next_position"], 2);
    assert_eq!(shard["events"]["1"]["id"], event.id().as_str());
    drop(service);
    let mut service = launch(root, &environment);
    assert_eq!(
        capture_http(root, listen, "producer", "source-repair", &mut service).0,
        202
    );
    assert!(checked(outbox().pending(&market())).is_empty());
    assert_eq!(redis_json(root, redis)["next_position"], 2);
    let resumed = checked(hub.resume(&principal(), &endpoint(), &market(), None, 32));
    let ResumePage::Events {
        events,
        next_cursor,
        ..
    } = resumed
    else {
        panic!("resume events")
    };
    assert_eq!(events, vec![event]);
    assert!(outbox()
        .resume(
            &checked(Principal::new("tenant:other")),
            &endpoint(),
            &market(),
            None,
            32
        )
        .is_err());
    assert!(outbox()
        .resume(
            &principal(),
            &checked(EndpointId::new("whep_other")),
            &market(),
            Some(&next_cursor),
            32
        )
        .is_err());
    checked(hub.revoke(&principal(), &market()));
    assert!(hub
        .resume(&principal(), &endpoint(), &market(), None, 32)
        .is_err());
    assert_eq!(
        capture_http(root, listen, "producer", "source-repair", &mut service).0,
        403
    );
}
