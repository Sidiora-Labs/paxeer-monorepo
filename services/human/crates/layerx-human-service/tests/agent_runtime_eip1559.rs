use std::fmt::Debug;
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agent_api::error::RequestId;
use layerx_agent_api::idempotency::{BodyDigest, IdempotentMutation, Key};
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_agent_api::prepare::{
    IdempotencyRef, PayloadBytes, PreparationRef, PrepareRequest, TimestampBound,
};
use layerx_agent_api::submit::{SignatureBytes, SubmitRequest};
use layerx_agent_api::{Amount, Sequence, TimestampSeconds};
use layerx_agentd::human::{decode_request, encode_response, HumanRequest, HumanResponse};
use layerx_agentd::prepare::{
    prepare_bound, AbiArgument, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest,
    PrecompileCall, PrecompileTarget,
};
use layerx_crypto::disclosure::call::{bind_call, ledger_selector};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::custody::{KeyId, Operation};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::journeys::{
    AgentBoundary, AgentBoundaryError, AgentPreparation, JourneyEngine, JourneyError, JourneyKind,
    JourneyLeg, JourneyPlan,
};
use layerx_human_service::notify::JourneyId;
use layerx_human_service::server::agent_runtime::{
    decode_unsigned, AgentRuntime, TransactionType2,
};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_intents::{compile, BudgetDefund, Intent, IntentKind};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount as ProtocolAmount;
use layerx_types::ids::{AssetId, Did, IdempotencyKey};
use layerx_types::intent::{BudgetId, Sequence as ProtocolSequence};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;

const NONCE: u64 = 19;
const FEE: u128 = 100_000;
const ACTOR: &str = "did:layerx:alice";
const AUTHORITY: &str = "custody-human-primary";
const ACTION: [u8; 32] = [0x31; 32];
const MAX_FRAME: usize = 8192;

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("EIP-1559 peer: {error:?}"))
}

fn registry() -> ModuleRegistry {
    let activity = checked(ActivityType::new(ModuleId::Budget, 8));
    checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Budget,
        &[activity],
    ))]))
}

fn intent() -> Intent {
    Intent::v1(IntentKind::BudgetDefund(checked(BudgetDefund::new(
        BudgetId::new([0x21; 32]),
        checked(AccountId::parse("agent:did:layerx:alice:budget:travel")),
        checked(AccountId::parse("agent:did:layerx:alice:main")),
        AssetId::new([0x22; 32]),
        ProtocolAmount::from_u128(37),
        ProtocolSequence::from_u64(0),
        IdempotencyKey::new(ACTION),
    ))))
}

fn request() -> PrepareRequest {
    let compiled = checked(compile(&intent(), &registry()));
    PrepareRequest {
        protocol_activity_type: compiled.activity_type().value(),
        actor: checked(AgentDid::new(ACTOR)),
        authority: checked(AuthorityRef::new(AUTHORITY)),
        account_sequence: Sequence(NONCE),
        timestamp_bound: TimestampBound {
            not_before: TimestampSeconds(100),
            not_after: TimestampSeconds(200),
        },
        idempotency_key: checked(IdempotencyRef::new("31".repeat(32))),
        fee_limit: Amount(FEE),
        payload: checked(PayloadBytes::new(compiled.payload().as_bytes().to_vec())),
        payload_hash: compiled.payload_hash(),
    }
}

fn mutation<T>(operation: T) -> IdempotentMutation<T> {
    IdempotentMutation {
        request_id: RequestId(9),
        key: checked(Key::new(ACTION)),
        body_digest: BodyDigest([0x41; 32]),
        operation,
    }
}

fn client() -> layerx_sdk::Client {
    checked(layerx_sdk::Client::daemon(
        "/run/layerx-agentd.sock",
        layerx_agent_api::agent_api_schema_v1().version,
    ))
}

fn transaction(sender: [u8; 20], nonce: u64) -> TransactionType2 {
    let mut amount = [0; 32];
    amount[16..].copy_from_slice(&37_u128.to_be_bytes());
    checked(prepare_bound(
        EvmPreparationState {
            chain_id: 125,
            pending_nonce: nonce,
            base_fee_per_gas: 2,
            head_height: 88,
        },
        EvmPreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 5,
            maximum_payload_bytes: MAX_FRAME,
        },
        EvmPrepareRequest {
            from: sender,
            call: PrecompileCall {
                target: PrecompileTarget::Pay,
                selector: ledger_selector("budgetDefund(bytes32,uint128)"),
                arguments: vec![AbiArgument::Word([0x21; 32]), AbiArgument::Word(amount)],
            },
            expected_nonce: Some(nonce),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: MAX_FRAME,
        },
        20_000,
    ))
    .call
}

fn bytes(out: &mut Vec<u8>, value: &[u8]) {
    out.extend_from_slice(&checked(u32::try_from(value.len())).to_be_bytes());
    out.extend_from_slice(value);
}

fn preparation_response(call: &TransactionType2, request: &PrepareRequest) -> Vec<u8> {
    let mut out = Vec::new();
    bytes(&mut out, b"type2-preparation");
    out.push(2);
    out.extend_from_slice(&call.chain_id.to_be_bytes());
    out.extend_from_slice(&call.nonce.to_be_bytes());
    out.extend_from_slice(&call.max_priority_fee_per_gas.to_be_bytes());
    out.extend_from_slice(&call.max_fee_per_gas.to_be_bytes());
    out.extend_from_slice(&call.gas_limit.to_be_bytes());
    out.extend_from_slice(&call.to);
    bytes(&mut out, &call.data);
    out.extend_from_slice(&request.protocol_activity_type.to_be_bytes());
    bytes(&mut out, request.actor.as_str().as_bytes());
    bytes(&mut out, request.authority.as_str().as_bytes());
    for coordinate in [request.account_sequence.0, 100, 200] {
        out.extend_from_slice(&coordinate.to_be_bytes());
    }
    out.extend_from_slice(&FEE.to_be_bytes());
    bytes(&mut out, request.payload.as_bytes());
    out.extend_from_slice(&request.payload_hash);
    out.extend_from_slice(&ACTION);
    encode_response(Ok(checked(HumanResponse::new(out))))
}

struct Peer {
    root: PathBuf,
    server: Option<JoinHandle<()>>,
}
impl Peer {
    fn serve(sender: [u8; 20], nonce: u64) -> (Self, AgentRuntime) {
        let root = directory("eip1559-peer");
        checked(std::fs::create_dir_all(&root));
        let socket = root.join("peer.sock");
        let listener = checked(UnixListener::bind(&socket));
        checked(listener.set_nonblocking(true));
        let server = thread::spawn(move || {
            let mut prepared = None;
            for _ in 0..2 {
                let deadline = Instant::now() + Duration::from_secs(5);
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(Instant::now() < deadline, "peer request deadline");
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("peer accept: {error}"),
                    }
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
                let response =
                    match checked(decode_request(checked(read_frame(&mut stream, MAX_FRAME)))) {
                        HumanRequest::Prepare(value) => {
                            assert_eq!(value.operation.nonce, nonce);
                            assert_eq!(value.operation.payload, request().payload.as_bytes());
                            let call = transaction(sender, nonce);
                            let response = preparation_response(&call, &request());
                            prepared = Some(call);
                            response
                        }
                        HumanRequest::Submit(value) => {
                            assert_eq!(value.operation.preparation_ref, "type2-preparation");
                            assert_eq!(value.operation.signature.len(), 65);
                            let call = prepared.as_ref().expect("prepared transaction");
                            let raw = checked(layerx_intents::owner_activity::attach_signature(
                                call,
                                checked(value.operation.signature.as_slice().try_into()),
                                sender,
                            ));
                            let (decoded, signature) = checked(decode_signed(&raw));
                            assert_eq!(&decoded, call);
                            assert_eq!(checked(recover_sender(&decoded, &signature)), sender);
                            let mut out = transaction_hash(&raw).to_vec();
                            bytes(&mut out, b"type2-submission");
                            out.extend_from_slice(&[3, Rung::Submitted.wire_rank(), 0, 0, 0]);
                            encode_response(Ok(checked(HumanResponse::new(out))))
                        }
                        _ => panic!("unexpected peer request"),
                    };
                checked(write_frame(&mut stream, &response, MAX_FRAME));
            }
        });
        let runtime = checked(AgentRuntime::new(
            &socket,
            Limits {
                maximum_frame_bytes: MAX_FRAME,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: MAX_FRAME,
                deadline: Duration::from_secs(5),
            },
            registry(),
        ));
        (
            Self {
                root,
                server: Some(server),
            },
            runtime,
        )
    }
    fn finish(mut self) {
        checked(self.server.take().expect("peer thread").join());
    }
}
impl Drop for Peer {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

fn engine(root: &std::path::Path) -> JourneyEngine {
    let (mut store, _) = install_and_open(
        root,
        &tenancy(&[("alice", "tenant-a")]),
        retention_uniform(10_000),
    );
    let mut scope = checked(store.principal(&principal("alice")));
    let leg = checked(JourneyLeg::new(
        intent(),
        ACTION,
        checked(AgentDid::new(ACTOR)),
        checked(AuthorityRef::new(AUTHORITY)),
        NONCE,
        100,
        200,
        FEE,
    ));
    let plan = checked(JourneyPlan::new(
        checked(JourneyId::new("jrn_type2")),
        JourneyKind::Move,
        [0x51; 32],
        checked(KeyId::new("human-primary")),
        Operation::ProtocolMutation,
        vec![leg],
    ));
    checked(JourneyEngine::start(&mut scope, &plan, &registry(), 100))
}

#[test]
fn runtime_prepares_an_unsigned_type2_transaction() {
    let signed = checked(transaction([1; 20], NONCE).sign(&[0x53; 32]));
    let (call, signature) = checked(decode_signed(&signed));
    let sender = checked(recover_sender(&call, &signature));
    let (peer, mut runtime) = Peer::serve(sender, NONCE);
    checked(runtime.bind_transaction_sender(sender));
    let client = client();
    let prepared = checked(runtime.prepare(&client.prepare(mutation(request()))));
    assert_eq!(prepared.unsigned_canonical_bytes[0], 2);
    let decoded = checked(decode_unsigned(&prepared.unsigned_canonical_bytes));
    assert_eq!(decoded.nonce, NONCE);
    assert_eq!(decoded, transaction(sender, NONCE));
    assert_eq!(prepared.signing_preimage, decoded.signing_hash());
    assert_eq!(prepared.disclosure, checked(bind_call(&decoded, sender)));
    let root = directory("type2-engine");
    let engine = engine(&root);
    checked(engine.validate_preparation(0, &prepared, &registry()));
    for changed in 0..5 {
        let mut altered = prepared.clone();
        let mut call = decoded.clone();
        match changed {
            0 => call.nonce += 1,
            1 => call.to[0] = 1,
            2 => call.data[35] ^= 1,
            3 => call.max_fee_per_gas += 1,
            _ => call.max_priority_fee_per_gas = 6,
        }
        altered.unsigned_canonical_bytes =
            layerx_human_kms::attestor::unsigned_evm_transaction(&call);
        assert!(matches!(
            engine.validate_preparation(0, &altered, &registry()),
            Err(JourneyError::PreparationMismatch)
        ));
    }
    let raw = checked(decoded.sign(&[0x53; 32]));
    let (_, signature) = checked(decode_signed(&raw));
    let submitted = checked(runtime.submit(
        &client.submit(mutation(SubmitRequest {
            preparation_ref: prepared.preparation_ref,
            signature: checked(SignatureBytes::new(signature.as_bytes().to_vec())),
            approval_release_ref: None,
        })),
        [0x61; 32],
    ));
    assert_eq!(submitted.activity_id, transaction_hash(&raw));
    assert_eq!(submitted.submission.rung, Rung::Submitted);
    peer.finish();
    checked(std::fs::remove_dir_all(root));
}

#[test]
fn engine_refuses_a_kernel_activity_envelope_leg() {
    let compiled = checked(compile(&intent(), &registry()));
    let mut builder = layerx_types::activity::EnvelopeBuilder::new();
    checked(builder.protocol_version(3));
    checked(builder.network_id(125));
    checked(builder.activity_type(compiled.activity_type()));
    checked(builder.actor_did(checked(Did::new(ACTOR.as_bytes()))));
    checked(
        builder.authority(checked(layerx_types::activity::Authority::owner(
            &[0x61; 32],
        ))),
    );
    checked(builder.account_sequence(NONCE));
    checked(
        builder.timestamp_bound(checked(layerx_types::activity::TimestampBound::new(
            100, 200,
        ))),
    );
    checked(builder.idempotency_key(IdempotencyKey::new(ACTION)));
    checked(builder.fee_limit(ProtocolAmount::from_u128(FEE)));
    checked(builder.payload_hash(compiled.payload_hash()));
    checked(builder.payload(compiled.payload().clone()));
    let envelope = checked(builder.build());
    let canonical = checked(layerx_wire::activity::encode_unsigned_envelope(&envelope));
    let account_id = |name: &str| {
        checked(layerx_intents::canonical::account_id_for_protocol(
            &checked(AccountId::parse(name)),
            3,
        ))
    };
    let owner = account_id("agent:did:layerx:alice:main");
    let context = layerx_crypto::disclosure::BudgetStateContext {
        budget_id: [0x21; 32],
        owner,
        budget_account: account_id(&format!(
            "agent:did:layerx:alice:budget:{}",
            "21".repeat(32)
        )),
        asset: [0x22; 32],
        source_account: owner,
        native_source: false,
        revocation_sequence: 0,
        balance: 100,
        state_digest: [0x71; 32],
        observed_head_sequence: 88,
        purpose_hash: [0x72; 32],
    };
    let disclosure = checked(layerx_crypto::disclosure::bind_budget_mutation(
        &canonical,
        &registry(),
        &context,
    ));
    let request = request();
    let activity = checked(layerx_wire::activity::decode_unsigned(
        &canonical,
        &registry(),
    ));
    let preimage = checked(layerx_wire::sign::preimage(&activity))
        .as_bytes()
        .to_vec();
    let prepared = AgentPreparation {
        preparation_ref: checked(PreparationRef::new("kernel-preparation")),
        unsigned_canonical_bytes: canonical,
        signing_preimage: preimage,
        disclosure,
        actor: request.actor,
        authority: request.authority,
        account_sequence: NONCE,
        not_before: 100,
        not_after: 200,
        fee_limit: FEE,
        activity_type: compiled.activity_type(),
        payload: compiled.payload().as_bytes().to_vec(),
        payload_hash: compiled.payload_hash(),
        idempotency_key: ACTION,
    };
    let root = directory("kernel-refusal-engine");
    let engine = engine(&root);
    assert!(matches!(
        engine.validate_preparation(0, &prepared, &registry()),
        Err(JourneyError::PreparationMismatch)
    ));
    assert_eq!(
        decode_unsigned(&prepared.unsigned_canonical_bytes),
        Err(AgentBoundaryError::CorruptResponse)
    );
    checked(std::fs::remove_dir_all(root));
}
