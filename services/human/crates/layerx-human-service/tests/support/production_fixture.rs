#[path = "recorded_node.rs"]
pub(crate) mod recorded_node;

#[allow(dead_code)]
#[path = "../../../layerx-human-kms/tests/support/attestor_cluster.rs"]
pub(crate) mod cluster;

#[path = "../../src/time.rs"]
pub(crate) mod time;

// The included runtime-clock/src/server.rs is shared with another test module.
#[allow(clippy::duplicate_mod)]
#[path = "../../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use std::collections::BTreeMap;
use std::fs;
use std::io::{Read, Write};
use std::os::unix::fs::PermissionsExt;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_service::server::{
    HttpConfig, PrincipalLimits, ProductionComponents, ProductionComponentsConfig, Router,
};
use layerx_types::clock::{Clock, Deadline};
use serde_json::{json, Value};
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread::JoinHandle;

pub(crate) static ENVIRONMENT: Mutex<()> = Mutex::new(());
pub(crate) type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

pub(crate) struct SerialClock(Mutex<Arc<RuntimeClock>>);

impl Clock for SerialClock {
    fn sample(
        &self,
        transport_budget: Duration,
    ) -> std::result::Result<layerx_types::clock::ClockReading, layerx_types::clock::ClockError>
    {
        self.0
            .lock()
            .map_err(|_| layerx_types::clock::ClockError::Unavailable)?
            .sample(transport_budget)
    }
}

pub(crate) struct Fixture {
    pub(crate) root: PathBuf,
    pub(crate) custody_cluster: Option<cluster::Cluster>,
    pub(crate) kms: Option<layerx_human_service::server::production_components::AttestorKms>,
    pub(crate) original_roots: Vec<(&'static str, Option<std::ffi::OsString>)>,
    pub(crate) assertion: String,
    pub(crate) browser_access: String,
    browser_csrf: String,
    pub(crate) peer_state: Arc<Mutex<PeerState>>,
    pub(crate) rpc_server: Option<RpcServer>,
    pub(crate) clock: Arc<SerialClock>,
    pub(crate) clock_server: Option<ClockServer>,
    pub(crate) agent_server: Option<RealAgentLayer>,
    pub(crate) identity_server: Option<IdentityServer>,
    pub(crate) router: Option<Arc<Router<ProductionComponents>>>,
    pub(crate) document: Value,
}

pub(crate) struct ClockServer {
    socket: PathBuf,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}

#[derive(Debug)]
pub(crate) struct IdentityTenancy(pub(crate) layerx_identity_binding::Client);

impl layerx_human_service::store::PrincipalTenancyAuthority for IdentityTenancy {
    fn tenant_for(
        &self,
        principal: &layerx_human_service::store::PrincipalId,
    ) -> std::result::Result<
        layerx_human_service::store::AgentTenantId,
        layerx_human_service::store::StoreError,
    > {
        let binding = self.0.lookup(principal.as_str())?;
        if binding.principal() != principal.as_str() {
            return Err(layerx_human_service::store::StoreError::InvalidPrincipal);
        }
        layerx_human_service::store::AgentTenantId::new(binding.agent_tenant())
    }
}

pub(crate) struct IdentityServer {
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}

impl IdentityServer {
    fn start(
        root: &std::path::Path,
        clock: Arc<SerialClock>,
        state: layerx_human_identity_provider::State,
    ) -> Result<Self> {
        use layerx_human_identity_provider::Server;
        let server = Server::bind(
            &root.join("identity.sock"),
            state,
            rustix::process::getuid().as_raw(),
            Duration::from_secs(5),
            clock,
        )?
        .with_binding_reader(
            &root.join("identity-binding.sock"),
            "tenant-a",
            &[rustix::process::getuid().as_raw()],
        )?;
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = std::thread::spawn(move || server.run(&stop));
        Ok(Self {
            shutdown,
            worker: Some(worker),
        })
    }
}

impl Drop for IdentityServer {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            match worker.join() {
                Ok(Ok(())) => {}
                result => eprintln!("identity server stopped with {result:?}"),
            }
        }
    }
}

// Bank asset identifiers are the SHA-256 digest of the ASCII denom "alxp".
pub(crate) const ASSET: [u8; 32] = [
    0x80, 0xfa, 0xce, 0x2b, 0x4a, 0x14, 0x30, 0x8e, 0x32, 0x93, 0x26, 0xee, 0x2f, 0x81, 0x41, 0xd2,
    0x49, 0x07, 0xae, 0x84, 0x75, 0x54, 0xcb, 0xba, 0xb6, 0x48, 0x54, 0xc6, 0xec, 0xc9, 0x11, 0xf7,
];
pub(crate) const HEAD: u64 = 88;
pub(crate) const NONCE: u64 = 19;
pub(crate) const MAX_FRAME: usize = 1_048_576;

#[derive(Default)]
pub(crate) struct PeerState {
    pub(crate) served: Vec<String>,
    pub(crate) rpc_served: Vec<String>,
    pub(crate) gateway_served: Vec<String>,
    pub(crate) gateway_last_answer: Option<Value>,
    pub(crate) identities: BTreeMap<String, ([u8; 32], [u8; 20])>,
    pub(crate) submitted: BTreeMap<[u8; 32], Vec<u8>>,
    pub(crate) pending_nonces: Vec<([u8; 20], u64)>,
    pub(crate) receipts: BTreeMap<[u8; 32], Value>,
    pub(crate) blocks: BTreeMap<u64, Value>,
    pub(crate) receipt_rung: Option<layerx_types::rung::Rung>,
    pub(crate) node_recording: Option<recorded_node::NodeRecording>,
    pub(crate) fail_next_receipt_lookup: bool,
    pub(crate) failed_receipt_lookups: usize,
    pub(crate) reject_repeated_send: bool,
    pub(crate) receipt_lookups: Vec<([u8; 32], [u8; 32])>,
}

pub(crate) fn bytes(out: &mut Vec<u8>, value: &[u8]) {
    out.extend_from_slice(
        &u32::try_from(value.len())
            .or_fail("frame length")
            .to_be_bytes(),
    );
    out.extend_from_slice(value);
}

impl PeerState {
    fn identity_response(&self, agent: &str) -> Vec<u8> {
        let mut out = Vec::new();
        let (key, _) = self.identities.get(agent).or_fail("registered identity");
        let did = layerx_types::ids::Did::new(agent.as_bytes()).or_fail("DID");
        let mut canonical = vec![0; 223];
        canonical[..5].copy_from_slice(b"LXGI1");
        canonical[5..37]
            .copy_from_slice(&layerx_wire::hash::did_id_for_protocol(&did, 3).or_fail("DID id"));
        canonical[37..69].copy_from_slice(key);
        canonical[69..77].copy_from_slice(&1_u64.to_be_bytes());
        canonical[77..109].copy_from_slice(&[0x63; 32]);
        canonical[109..111].copy_from_slice(&1_u16.to_be_bytes());
        canonical[215..].copy_from_slice(&HEAD.to_be_bytes());
        out.extend_from_slice(&HEAD.to_be_bytes());
        out.extend_from_slice(&1_u64.to_be_bytes());
        out.extend_from_slice(&[4, 0]);
        out.extend_from_slice(&1_u16.to_be_bytes());
        out.push(1);
        out.extend_from_slice(key);
        bytes(&mut out, &canonical);
        out
    }

    fn answer(
        &mut self,
        request: layerx_agentd::human::HumanRequest,
        account: Option<String>,
    ) -> Vec<u8> {
        use layerx_agentd::human::HumanRequest;
        use layerx_types::rung::Rung;
        self.served.push(request_name(&request).to_owned());
        let mut out = Vec::new();
        match request {
            HumanRequest::Subject {
                principal,
                owner,
                account,
                asset,
                request,
            } => {
                assert!(!principal.is_empty());
                assert!(account.starts_with(&format!("agent:{owner}:")));
                assert_eq!(asset, ASSET);
                return self.answer(*request, Some(account));
            }
            HumanRequest::Registry => {
                out.extend_from_slice(&1_u16.to_be_bytes());
                out.extend_from_slice(
                    &(layerx_types::payload::ModuleId::Asset as u16).to_be_bytes(),
                );
                out.extend_from_slice(&1_u16.to_be_bytes());
                out.extend_from_slice(
                    &layerx_types::payload::ActivityType::new(
                        layerx_types::payload::ModuleId::Asset,
                        5,
                    )
                    .or_fail("send activity")
                    .value()
                    .to_be_bytes(),
                );
            }
            HumanRequest::NativeFeePolicy => {
                out.push(1);
                out.extend_from_slice(&ASSET);
                bytes(&mut out, b"LXP");
                out.push(0);
                out.extend_from_slice(&HEAD.to_be_bytes());
                out.extend_from_slice(&[0x32; 32]);
            }
            HumanRequest::Head => {
                out.extend_from_slice(&HEAD.to_be_bytes());
                out.extend_from_slice(&1_u64.to_be_bytes());
                out.extend_from_slice(&[0x32; 32]);
            }
            HumanRequest::AccountSequence { actor, authority } => {
                let (key, wallet) = self.identities.get(&actor).or_fail("registered actor");
                assert_eq!(authority, cluster::hex(key));
                let next = self
                    .submitted
                    .values()
                    .filter_map(|raw| {
                        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(raw)
                            .or_fail("signed leg");
                        (layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                            .or_fail("sender")
                            == *wallet)
                            .then_some(call.nonce + 1)
                    })
                    .max()
                    .unwrap_or(NONCE);
                out.extend_from_slice(&next.to_be_bytes());
                out.push(Rung::Committed.wire_rank());
            }
            HumanRequest::IdentityResolve { agent } => return self.identity_response(&agent),
            HumanRequest::Balance => {
                let named =
                    layerx_types::account::AccountId::parse(&account.or_fail("subject account"))
                        .or_fail("account");
                let id = layerx_intents::canonical::account_id_for_protocol(&named, 3)
                    .or_fail("account id");
                bytes(&mut out, &serde_json::to_vec(&json!({
                    "achieved_verification_level": "committed",
                    "value": {"account": cluster::hex(&id), "asset": cluster::hex(&ASSET),
                        "amount": "1000000000000000000", "canonical_state": "01", "height": HEAD.to_string()},
                    "freshness": {"chain_head": HEAD.to_string(), "value_height": HEAD.to_string(), "relative_to": HEAD.to_string()}
                })).or_fail("balance JSON"));
            }
            HumanRequest::ReceiptLookup {
                expected_activity_id,
                ..
            } => {
                assert!(
                    self.submitted.contains_key(&expected_activity_id),
                    "receipt requested for an unsubmitted leg"
                );
                // The peer has accepted this signed transaction; no committed receipt exists yet.
                out.push(0);
            }
            HumanRequest::Evidence { tx_hash, .. } => {
                assert!(self.submitted.contains_key(&tx_hash));
                out.push(0);
            }
            other => panic!("unexpected agent request: {}", request_name(&other)),
        }
        out
    }
}

pub(crate) fn request_name(request: &layerx_agentd::human::HumanRequest) -> &'static str {
    use layerx_agentd::human::HumanRequest;
    match request {
        HumanRequest::NativeOwnerContextV1(..) => "NativeOwnerContextV1",
        HumanRequest::NativeSendPreviewV1(..) => "NativeSendPreviewV1",
        HumanRequest::NativeSendOwnerPreviewV1(..) => "NativeSendOwnerPreviewV1",
        HumanRequest::NativeJourneyEnvelopeV1(..) => "NativeJourneyEnvelopeV1",
        HumanRequest::NativeSendEnvelopeV1(..) => "NativeSendEnvelopeV1",
        HumanRequest::NativeProgramApprovalListV5 { .. } => "NativeProgramApprovalListV5",
        HumanRequest::NativeProgramApprovalGetV5 { .. } => "NativeProgramApprovalGetV5",
        HumanRequest::NativeProgramApprovalMaterialV5 { .. } => "NativeProgramApprovalMaterialV5",
        HumanRequest::NativeProgramApprovalBudgetV5 { .. } => "NativeProgramApprovalBudgetV5",
        HumanRequest::NativeProgramApprovalDecideV5 { .. } => "NativeProgramApprovalDecideV5",
        HumanRequest::AgentBudgetProofV4 { .. } => "AgentBudgetProofV4",
        HumanRequest::Subject { .. } => "Subject",
        HumanRequest::Prepare(..) => "Prepare",
        HumanRequest::Submit(..) => "Submit",
        HumanRequest::Track { .. } => "Track",
        HumanRequest::ReceiptLookup { .. } => "ReceiptLookup",
        HumanRequest::Registry => "Registry",
        HumanRequest::ApprovalList { .. } => "ApprovalList",
        HumanRequest::ApprovalGet { .. } => "ApprovalGet",
        HumanRequest::ApprovalListFactsV2 { .. } => "ApprovalListFactsV2",
        HumanRequest::ApprovalGetFactsV2 { .. } => "ApprovalGetFactsV2",
        HumanRequest::ApprovalBudgetAfterV2 { .. } => "ApprovalBudgetAfterV2",
        HumanRequest::ManagedEvidenceByDigestV2 { .. } => "ManagedEvidenceByDigestV2",
        HumanRequest::NativeApprovalListFactsV2 { .. } => "NativeApprovalListFactsV2",
        HumanRequest::NativeApprovalGetFactsV2 { .. } => "NativeApprovalGetFactsV2",
        HumanRequest::NativeEffectApprovalListV3 { .. } => "NativeEffectApprovalListV3",
        HumanRequest::NativeEffectApprovalGetV3 { .. } => "NativeEffectApprovalGetV3",
        HumanRequest::NativeEffectApprovalDecideV3 { .. } => "NativeEffectApprovalDecideV3",
        HumanRequest::NativeEffectApprovalMaterialV4 { .. } => "NativeEffectApprovalMaterialV4",
        HumanRequest::NativeEffectApprovalBudgetV4 { .. } => "NativeEffectApprovalBudgetV4",
        HumanRequest::ApprovalApprove { .. } => "ApprovalApprove",
        HumanRequest::ApprovalReject { .. } => "ApprovalReject",
        HumanRequest::Balance => "Balance",
        HumanRequest::NativeFeePolicy => "NativeFeePolicy",
        HumanRequest::AccountState { .. } => "AccountState",
        HumanRequest::SessionFeeState { .. } => "SessionFeeState",
        HumanRequest::SessionSeedPrepare { .. } => "SessionSeedPrepare",
        HumanRequest::Head => "Head",
        HumanRequest::Evidence { .. } => "Evidence",
        HumanRequest::IdentityResolve { .. } => "IdentityResolve",
        HumanRequest::LeaseMap { .. } => "LeaseMap",
        HumanRequest::OwnerValidate(..) => "OwnerValidate",
        HumanRequest::OwnerInstall(..) => "OwnerInstall",
        HumanRequest::AccountSequence { .. } => "AccountSequence",
        HumanRequest::AgentList { .. } => "AgentList",
        HumanRequest::AgentGet { .. } => "AgentGet",
        HumanRequest::AgentControl { .. } => "AgentControl",
        HumanRequest::AgentLimit { .. } => "AgentLimit",
        HumanRequest::AgentJourney { .. } => "AgentJourney",
        HumanRequest::AgentArchive { .. } => "AgentArchive",
        HumanRequest::CapabilityInstall(..) => "CapabilityInstall",
        HumanRequest::AgentContext { .. } => "AgentContext",
        HumanRequest::AgentLifecyclePublish(..) => "AgentLifecyclePublish",
        HumanRequest::AgentBudgetState { .. } => "AgentBudgetState",
        HumanRequest::AgentKeyPolicy { .. } => "AgentKeyPolicy",
        HumanRequest::AgentSessionSnapshot { .. } => "AgentSessionSnapshot",
        HumanRequest::AgentSessionSuspend { .. } => "AgentSessionSuspend",
        HumanRequest::AgentSessionBind { .. } => "AgentSessionBind",
        HumanRequest::AgentSessionRestrict { .. } => "AgentSessionRestrict",
        HumanRequest::Operator { .. } => "Operator",
    }
}

fn decode_agent_frame(frame: Vec<u8>) -> (Option<layerx_human_service::trace::TraceId>, Vec<u8>) {
    if frame.starts_with(b"LXHAGT02") {
        assert!(frame.len() >= 48, "truncated traced agent request");
        let trace = layerx_human_service::trace::TraceId::parse(
            std::str::from_utf8(&frame[8..44]).or_fail("request trace UTF-8"),
        )
        .or_fail("request trace");
        let length = u32::from_be_bytes(frame[44..48].try_into().or_fail("frame length")) as usize;
        assert_eq!(frame.len(), 48 + length, "traced request length");
        assert!(frame[48..].starts_with(b"LXHAGT01"));
        (Some(trace), frame[48..].to_vec())
    } else {
        (None, frame)
    }
}

pub(crate) struct RealAgentLayer {
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}

impl RealAgentLayer {
    fn start(
        root: &std::path::Path,
        state: Arc<Mutex<PeerState>>,
        rpc_url: String,
    ) -> Result<Self> {
        use layerx_agentd::human::{decode_request, encode_response, HumanResponse};
        use layerx_human_service::frame::{read_frame, write_frame};
        let directory = root.join("agent-socket");
        fs::create_dir(&directory)?;
        fs::set_permissions(&directory, fs::Permissions::from_mode(0o700))?;
        let socket = directory.join("agent.sock");
        let listener = UnixListener::bind(&socket)?;
        fs::set_permissions(&socket, fs::Permissions::from_mode(0o600))?;
        listener.set_nonblocking(true)?;
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = std::thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => return Err(error),
                };
                stream.set_read_timeout(Some(Duration::from_secs(5)))?;
                let frame = read_frame(&mut stream, MAX_FRAME)
                    .map_err(|error| std::io::Error::other(format!("agent frame: {error:?}")))?;
                let (trace, frame) = decode_agent_frame(frame);
                let request = decode_request(frame)
                    .unwrap_or_else(|error| panic!("decode agent request: {error:?}"));
                let mut inner = &request;
                while let layerx_agentd::human::HumanRequest::Subject { request, .. } = inner {
                    inner = request;
                }
                let payload = if let layerx_agentd::human::HumanRequest::ReceiptLookup {
                    idempotency_key,
                    expected_activity_id,
                } = inner
                {
                    let raw = {
                        let mut state = state.lock().or_fail("peer state");
                        state.served.push("ReceiptLookup".to_owned());
                        state
                            .receipt_lookups
                            .push((*idempotency_key, *expected_activity_id));
                        if std::mem::take(&mut state.fail_next_receipt_lookup) {
                            state.failed_receipt_lookups += 1;
                            continue;
                        }
                        let raw = state
                            .submitted
                            .get(expected_activity_id)
                            .or_fail("accepted leg");
                        let (call, signature) = required(
                            layerx_crypto::evm_transaction::decode_signed(raw),
                            "signed leg",
                        );
                        let sender = required(
                            layerx_crypto::evm_transaction::recover_sender(&call, &signature),
                            "sender",
                        );
                        assert_eq!(
                            required(
                                layerx_crypto::disclosure::call::bind_call(&call, sender),
                                "disclosure"
                            )
                            .idempotency_key,
                            *idempotency_key
                        );
                        raw.clone()
                    };
                    receipt_lookup_answer(&rpc_url, *expected_activity_id, &raw)
                } else {
                    state.lock().or_fail("peer state").answer(request, None)
                };
                let response =
                    encode_response(Ok(HumanResponse::new(payload).or_fail("agent response")));
                let response = if let Some(trace) = trace {
                    let mut envelope = b"LXHAGT02".to_vec();
                    envelope.extend_from_slice(trace.as_str().as_bytes());
                    bytes(&mut envelope, &response);
                    envelope
                } else {
                    response
                };
                write_frame(&mut stream, &response, MAX_FRAME).map_err(|error| {
                    std::io::Error::other(format!("agent response frame: {error:?}"))
                })?;
            }
            Ok(())
        });
        Ok(Self {
            shutdown,
            worker: Some(worker),
        })
    }
}

impl Drop for RealAgentLayer {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker
                .join()
                .or_fail("agent peer panicked")
                .or_fail("agent peer IO");
        }
    }
}

fn receipt_lookup_answer(rpc_url: &str, hash: [u8; 32], raw: &[u8]) -> Vec<u8> {
    use layerx_agentd::human_receipt_frame::{ReceiptLookupAnswer, ReceiptLookupFrame};
    use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
    use layerx_client::sequencer::SequencerEndpoint;
    use layerx_proof::receipt::AuthorizedBatch;
    use sha2::{Digest as _, Sha256};

    let endpoint = required(SequencerEndpoint::new(rpc_url, &[]), "receipt endpoint");
    let keys = required(endpoint.receipt_keys(), "receipt keys");
    let lookup = required(
        lookup_authenticated(
            &endpoint,
            hash,
            LookupContext {
                receipt_keys: &keys,
            },
        ),
        "authenticated receipt",
    );
    if let Some(answer) = layerx_agentd::human_runtime::receipt_lookup_not_ready_bytes(&lookup) {
        return answer;
    }
    let Lookup::Verified(receipt) = lookup else {
        unreachable!("not-ready answer handled")
    };
    let protocol = receipt.receipt().protocol().or_fail("protocol receipt");
    let (call, signature) = required(
        layerx_crypto::evm_transaction::decode_signed(raw),
        "submitted call",
    );
    let sender = required(
        layerx_crypto::evm_transaction::recover_sender(&call, &signature),
        "submitted sender",
    );
    let disclosure = required(
        layerx_crypto::disclosure::call::bind_call(&call, sender),
        "submitted disclosure",
    );
    let batch = protocol.batch_id();
    eprintln!(
        "receipt binding: activity_id=0x{} retained_hash=0x{} batch_height={} submission_height={} timestamp={} fee_charged={} fee_limit={} result_code={} module_id={} expected_module=12 protocol_version={} expected_protocol={} module_version={} expiry={}..={}",
        cluster::hex(&protocol.activity_id()),
        cluster::hex(&hash),
        u64::from_be_bytes(batch[24..].try_into().or_fail("batch height")),
        receipt.height(),
        protocol.timestamp(),
        protocol.fee_charged(),
        disclosure.fee_limit,
        protocol.result_code(),
        protocol.module_id(),
        protocol.protocol_version(),
        layerx_wire::limits::PROTOCOL_VERSION,
        protocol.module_version(),
        disclosure.expiry.not_before,
        disclosure.expiry.not_after,
    );
    let key = keys
        .iter()
        .find(|key| key.key_id == receipt.execution().key_id)
        .or_fail("published receipt key");
    required(
        ReceiptLookupAnswer::Found(ReceiptLookupFrame {
            canonical_bytes: receipt.canonical_bytes().to_vec(),
            authorised_batch: AuthorizedBatch::new(
                protocol.batch_id(),
                protocol.asset(),
                protocol.previous_state_root(),
                protocol.resulting_state_root(),
                key.public_key,
            ),
            verification_level: layerx_types::verify::VerificationLevel::SEQUENCER_SIGNED,
            tx_hash: receipt.activity_id(),
            receipt_hash: Sha256::digest(receipt.canonical_bytes()).into(),
            block_height: receipt.height(),
            rung: receipt.rung(),
            signed_receipt: receipt.canonical_bytes().to_vec(),
            receipt_key_id: key.key_id,
            public_key: key.public_key,
            first_height: key.first_height,
            last_height: key.last_height,
            inclusion_height: receipt.height(),
        })
        .encode(),
        "owner receipt answer",
    )
}

pub(crate) struct RpcServer {
    pub(crate) url: String,
    pub(crate) shutdown: Arc<AtomicBool>,
    pub(crate) worker: Option<JoinHandle<std::io::Result<()>>>,
}

pub(crate) fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").or_fail("hex prefix");
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).or_fail("hex text"), 16)
                .or_fail("hex byte")
        })
        .collect()
}

pub(crate) fn receipt_key() -> layerx_proof::inclusion::SequencerAuthorization {
    layerx_proof::inclusion::SequencerAuthorization::new(
        [0x92; 32],
        ed25519_dalek::SigningKey::from_bytes(&[0x93; 32])
            .verifying_key()
            .to_bytes(),
        HEAD,
        HEAD + 100,
    )
}

// ProtocolReceipt exposes decoded fields only; bind this pre-encoded fixture before signing.
const SEQUENCER_RECEIPT_TEMPLATE: &str = concat!(
    "0x",
    "0002", // envelope
    "5201", // receipt tag
    "0002", // protocol version
    "000000200000000000000000000000000000000000000000000000000000000000000000", // transaction hash
    "0000000000000001", // sequence
    "000000200404040404040404040404040404040404040404040404040404040404040404", // previous root
    "000000200000000000000000000000000000000000000000000000000000000000000000", // sequence root
    "00000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // return hash
    "00000000", // status
    "00000001", // effect count
    "000c", // effect module
    "0000", // ordinal
    "0000", // event type
    "01", // summary effect
    "00", // monetary
    "000000200000000000000000000000000000000000000000000000000000000000000000", // effect transfer root
    "000000710000000000005208000000000000520800000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855000000000000000004616c7870", // summary
    "00000000000000000000000000005208", // fee
    "000000200000000000000000000000000000000000000000000000000000000000000058", // height
    "000c", // module
    "00000001", // module version
    "00000000", // parameter version
    "02", // operation
    "0000002080face2b4a14308e329326ee2f8141d24907ae847554cbbab64854c6ecc911f7", // asset
    "00000000000000000000000000000001", // amount
    "000000200000000000000000000000000000000000000000000000000000000000000000", // from
    "0000000000000000000000000000520a", // from before
    "00000000000000000000000000000001", // from after
    "0000000000000013", // nonce
    "000000200000000000000000000000000000000000000000000000000000000000000000", // to
    "00000000000000000000000000000000", // to before
    "00000000000000000000000000000001", // to after
    "000000200000000000000000000000000000000000000000000000000000000000000000", // transfer root
    "000000200000000000000000000000000000000000000000000000000000000000000000", // authorization hash
    "000000209292929292929292929292929292929292929292929292929292929292929292", // key id
    "0000000000000001", // timestamp
    "01", // signature option
    "0000004000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", // signature
);

fn receipted_answer(raw: &[u8], timestamp: u64) -> Value {
    use ed25519_dalek::Signer as _;
    use sha2::{Digest as _, Sha256};
    let (call, signature) =
        layerx_crypto::evm_transaction::decode_signed(raw).or_fail("signed leg");
    let sender =
        layerx_crypto::evm_transaction::recover_sender(&call, &signature).or_fail("sender");
    let hash = layerx_crypto::evm_transaction::transaction_hash(raw);
    let sequence = call.nonce - NONCE + 1;
    let gas = 21_000_u64;
    let fee = u128::from(gas);
    let previous = [4_u8; 32];
    let mut sequence_preimage = b"PAXEER/v1/sequence\0".to_vec();
    sequence_preimage.extend_from_slice(&previous);
    sequence_preimage.extend_from_slice(&sequence.to_be_bytes());
    sequence_preimage.extend_from_slice(&hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEAD.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    let key = receipt_key();
    let mut signed = unhex(SEQUENCER_RECEIPT_TEMPLATE);
    signed[10..42].copy_from_slice(&hash);
    signed[42..50].copy_from_slice(&sequence.to_be_bytes());
    signed[90..122].copy_from_slice(&Sha256::digest(&sequence_preimage));
    signed[347..379].copy_from_slice(&height);
    signed[394..426].copy_from_slice(&ASSET);
    signed[446..478].copy_from_slice(&from);
    signed[510..518].copy_from_slice(&call.nonce.to_be_bytes());
    signed[522..554].copy_from_slice(&to);
    signed[662..694].copy_from_slice(&key.sequencer_id());
    signed[694..702].copy_from_slice(&timestamp.to_be_bytes());
    let receipt = required(layerx_wire::receipt::decode(&signed), "receipt fixture");
    assert_eq!(
        receipt
            .protocol()
            .or_fail("protocol receipt")
            .protocol_version(),
        layerx_wire::limits::PROTOCOL_VERSION
    );
    let unsigned = required(
        layerx_wire::receipt::encode_unsigned(&receipt),
        "unsigned receipt",
    );
    let digest = required(
        layerx_wire::hash::receipt_digest(&unsigned),
        "receipt digest",
    );
    let receipt_key = ed25519_dalek::SigningKey::from_bytes(&[0x93; 32]);
    signed[707..771].copy_from_slice(&receipt_key.sign(&digest).to_bytes());
    let signed = required(
        layerx_wire::receipt::encode(&required(
            layerx_wire::receipt::decode(&signed),
            "signed receipt fixture",
        )),
        "canonical signed receipt",
    );
    let answer = json!({
        "rung": "receipted", "txHash": format!("0x{}", cluster::hex(&hash)),
        "nonce": format!("0x{:x}", call.nonce), "seq": format!("0x{sequence:x}"),
        "height": format!("0x{HEAD:x}"), "index": "0x0", "timestamp": format!("0x{timestamp:x}"),
        "status": "0x1", "returnData": "0x", "logs": [], "gasUsed": format!("0x{gas:x}"),
        "cumulativeGasUsed": format!("0x{gas:x}"), "effectiveGasPrice": "0x1",
        "feeDenom": "alxp", "feeAmount": format!("0x{fee:x}"),
        "sponsor": format!("0x{}", cluster::hex(&[0; 20])),
        "from": format!("0x{}", cluster::hex(&sender)), "to": format!("0x{}", cluster::hex(&call.to)),
        "contractAddress": format!("0x{}", cluster::hex(&[0; 20])), "vmError": "",
        "keyId": format!("0x{}", cluster::hex(&key.sequencer_id())),
        "signingKey": format!("0x{}", cluster::hex(&key.public_key())),
        "receipt": format!("0x{}", cluster::hex(&signed))
    });
    answer
}

fn read_rpc_request(stream: &mut std::net::TcpStream) -> std::io::Result<(String, Value)> {
    let mut request = Vec::new();
    let header_end = loop {
        let mut byte = [0];
        stream.read_exact(&mut byte)?;
        request.push(byte[0]);
        assert!(request.len() < MAX_FRAME);
        if request.ends_with(b"\r\n\r\n") {
            break request.len();
        }
    };
    let headers = std::str::from_utf8(&request).or_fail("HTTP headers");
    let path = headers
        .lines()
        .next()
        .or_fail("HTTP request line")
        .split_whitespace()
        .nth(1)
        .or_fail("HTTP request path")
        .to_owned();
    let length: usize = headers
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            name.eq_ignore_ascii_case("content-length")
                .then(|| value.trim().parse().or_fail("content length"))
        })
        .or_fail("content length header");
    request.resize(header_end + length, 0);
    stream.read_exact(&mut request[header_end..])?;
    let request: Value = serde_json::from_slice(&request[header_end..]).or_fail("RPC request");
    Ok((path, request))
}

fn pending_nonce_answer(state: &mut PeerState, request: &Value) -> Value {
    assert_eq!(request["params"][1], "pending");
    let wallet: [u8; 20] = unhex(request["params"][0].as_str().or_fail("nonce wallet"))
        .try_into()
        .or_fail("nonce wallet length");
    assert!(state.identities.values().any(|(_, known)| *known == wallet));
    let nonce = state
        .submitted
        .values()
        .filter_map(|raw| {
            let (call, signature) = layerx_crypto::evm_transaction::decode_signed(raw)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "signed transaction"));
            (layerx_crypto::evm_transaction::recover_sender(&call, &signature).or_fail("sender")
                == wallet)
                .then_some(call.nonce + 1)
        })
        .max()
        .unwrap_or(NONCE);
    state.pending_nonces.push((wallet, nonce));
    json!(format!("0x{nonce:x}"))
}

fn send_transaction_answer(
    state: &mut PeerState,
    request: &Value,
    rpc_error: &mut Option<Value>,
) -> Value {
    let raw = unhex(request["params"][0].as_str().or_fail("raw transaction"));
    let (call, signature) =
        layerx_crypto::evm_transaction::decode_signed(&raw).or_fail("signed type-2 leg");
    let sender =
        layerx_crypto::evm_transaction::recover_sender(&call, &signature).or_fail("leg sender");
    assert!(state
        .identities
        .values()
        .any(|(_, wallet)| *wallet == sender));
    assert_eq!(call.chain_id, 125);
    layerx_crypto::disclosure::call::bind_call(&call, sender).or_fail("disclosed call");
    let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
    if state.reject_repeated_send && state.submitted.contains_key(&hash) {
        *rpc_error = Some(json!({"code": -32000,
                                "message": format!("same nonce already receipted or held: sender 0x{} nonce {} receipted",
                                    cluster::hex(&sender), call.nonce)}));
    } else if let Some(previous) = state.submitted.insert(hash, raw.clone()) {
        assert_eq!(previous, raw);
    } else {
        assert_eq!(layerx_crypto::evm_transaction::transaction_hash(&raw), hash);
    }
    json!(format!("0x{}", cluster::hex(&hash)))
}

fn sequencer_receipt_answer(
    state: &mut PeerState,
    request: &Value,
    clock: Option<&Arc<SerialClock>>,
) -> Value {
    let hash: [u8; 32] = unhex(request["params"][0].as_str().or_fail("hash"))
        .try_into()
        .or_fail("hash length");
    match state.submitted.get(&hash) {
        Some(raw) => {
            let (call, _) = layerx_crypto::evm_transaction::decode_signed(raw)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "signed transaction"));
            match state.receipt_rung {
                Some(layerx_types::rung::Rung::Receipted) => {
                    let timestamp = clock
                        .unwrap_or_else(|| panic!("{}", "receipted peer clock"))
                        .sample(Duration::from_secs(1))
                        .unwrap_or_else(|error| panic!("{}: {error:?}", "receipt clock"))
                        .unix_seconds();
                    receipted_answer(raw, timestamp)
                }
                _ => {
                    json!({"rung": "submitted", "txHash": format!("0x{}", cluster::hex(&hash)), "nonce": format!("0x{:x}", call.nonce)})
                }
            }
        }
        None => Value::Null,
    }
}

fn balance_answer(state: &PeerState, request: &Value) -> Value {
    let address = request["params"][0].as_str().or_fail("balance wallet");
    let wallet: [u8; 20] = unhex(address)
        .try_into()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "balance wallet length"));
    let identity = state
        .identities
        .values()
        .find(|(_, known_wallet)| *known_wallet == wallet)
        .map(|(key, _)| cluster::hex(key));
    let did = identity.as_ref().map(|key| format!("did:layerx:{key}"));
    json!({
        "account": {
            "evm_address": address, "pax_address": null,
            "layerx_did": did, "layerx_account": identity,
            "bound": identity.is_some()
        },
        "height": HEAD.to_string(),
        "balances": [{
            "denom": "alxp", "amount": "1000000000000000000"
        }]
    })
}

fn answer_rpc(
    state: &mut PeerState,
    method: &str,
    request: &Value,
    clock: Option<&Arc<SerialClock>>,
) -> (Value, Option<Value>) {
    let mut rpc_error = None;
    let recording = state.node_recording.clone().or_else(|| {
        (state.receipt_rung == Some(layerx_types::rung::Rung::Committed)
            && matches!(
                method,
                "px_getReceiptKeys"
                    | "pax_sequencerReceipt"
                    | "eth_getTransactionReceipt"
                    | "eth_getBlockByNumber"
                    | "eth_getRawTransactionByHash"
                    | "block"
                    | "commit"
                    | "validators"
                    | "tx"
            ))
        .then(recorded_node::NodeRecording::open)
    });
    let result = if let Some(recording) = recording {
        match recording.answer(method, &request["params"]) {
            Ok(answer) => answer,
            Err(error) => {
                rpc_error = Some(error);
                Value::Null
            }
        }
    } else {
        match method {
            "eth_chainId" => json!("0x7d"),
            "eth_getTransactionCount" => pending_nonce_answer(state, request),
            "px_listAssets" => {
                assert_eq!(request["params"], json!([]));
                json!({
                    "height": HEAD.to_string(),
                    "joined_limit": 16,
                    "assets": [{
                        "asset_id": "denom:alxp",
                        "denom": "alxp", "kind": "bank",
                        "name": null, "symbol": null, "decimals": null, "supply": null
                    }]
                })
            }
            "eth_call" => {
                let data = unhex(request["params"][0]["data"].as_str().or_fail("call data"));
                let key: [u8; 32] = data[4..].try_into().or_fail("DID public key");
                let wallet = state
                    .identities
                    .values()
                    .find(|(public, _)| *public == key)
                    .or_fail("bound identity")
                    .1;
                json!(format!("0x{}{}", "00".repeat(12), cluster::hex(&wallet)))
            }
            "px_getBalances" => balance_answer(state, request),
            "eth_sendRawTransaction" => send_transaction_answer(state, request, &mut rpc_error),
            "eth_getTransactionReceipt" => {
                let hash: [u8; 32] = unhex(request["params"][0].as_str().or_fail("hash"))
                    .try_into()
                    .or_fail("hash length");
                state.receipts.get(&hash).cloned().unwrap_or(Value::Null)
            }
            "eth_getBlockByNumber" => {
                let height = request["params"][0].as_str().or_fail("block height");
                let height = u64::from_str_radix(
                    height.strip_prefix("0x").or_fail("block height prefix"),
                    16,
                )
                .unwrap_or_else(|error| panic!("{}: {error:?}", "block height quantity"));
                state.blocks.get(&height).cloned().unwrap_or(Value::Null)
            }

            "px_getReceiptKeys" => {
                assert_eq!(request["params"], json!([]));
                let key = receipt_key();
                json!({"keys": [{"keyId": format!("0x{}", cluster::hex(&key.sequencer_id())),
                            "publicKey": format!("0x{}", cluster::hex(&key.public_key())),
                            "firstHeight": key.first_batch_number(),
                            "lastHeight": key.last_batch_number()}]})
            }
            "pax_sequencerReceipt" => sequencer_receipt_answer(state, request, clock),
            method => panic!("unexpected node request: {method}"),
        }
    };
    if method.starts_with("px_") {
        state.gateway_last_answer = Some(result.clone());
    }
    (result, rpc_error)
}

impl RpcServer {
    fn start(state: Arc<Mutex<PeerState>>, clock: Arc<SerialClock>) -> Result<Self> {
        Self::serve(state, Some(clock))
    }

    pub(crate) fn serve(
        state: Arc<Mutex<PeerState>>,
        clock: Option<Arc<SerialClock>>,
    ) -> Result<Self> {
        let listener = std::net::TcpListener::bind("127.0.0.1:0")?;
        let url = format!("http://{}", listener.local_addr()?);
        listener.set_nonblocking(true)?;
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = std::thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => return Err(error),
                };
                stream.set_read_timeout(Some(Duration::from_secs(5)))?;
                let (path, request) = read_rpc_request(&mut stream)?;
                let mut state = state.lock().or_fail("peer state");
                let method = request["method"].as_str().or_fail("RPC method");
                state.rpc_served.push(method.to_owned());
                if method.starts_with("px_") {
                    state.gateway_served.push(format!("{path} {method}"));
                }
                let (result, rpc_error) = answer_rpc(&mut state, method, &request, clock.as_ref());
                drop(state);
                let answer = match rpc_error {
                    Some(error) => json!({"jsonrpc": "2.0", "id": request["id"], "error": error}),
                    None => json!({"jsonrpc": "2.0", "id": request["id"], "result": result}),
                };
                let body = serde_json::to_vec(&answer).or_fail("RPC response");
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())?;
                stream.write_all(&body)?;
            }
            Ok(())
        });
        Ok(Self {
            url,
            shutdown,
            worker: Some(worker),
        })
    }
}

impl Drop for RpcServer {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker
                .join()
                .or_fail("node peer panicked")
                .or_fail("node peer IO");
        }
    }
}

impl ClockServer {
    fn start(root: &std::path::Path) -> Result<Self> {
        let mut server = runtime_clock_server::Server::bind(root)?;
        let socket = server.socket().to_owned();
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = std::thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                server.poll()?;
                std::thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok(Self {
            socket,
            shutdown,
            worker: Some(worker),
        })
    }
}

impl Drop for ClockServer {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            match worker.join() {
                Ok(Ok(())) => {}
                result => eprintln!("runtime clock server stopped with {result:?}"),
            }
        }
    }
}

fn bind_owner_browser_session(
    root: &std::path::Path,
    scope: &mut layerx_human_service::store::PrincipalScope<'_>,
    principal: &layerx_human_service::store::PrincipalId,
    now: u64,
    browser_access: &mut String,
    browser_csrf: &mut String,
) -> Result<()> {
    use layerx_human_service::auth::{AccountIdentity, AuthConfig, Device, Passkeys, RateLimit};
    use layerx_human_service::server::production_auth::{
        AuthDiscoveryIndex, IndexAuthenticationKey,
    };
    let passkeys = Passkeys::new(AuthConfig {
        rp_id: "paxportwallet.com".to_owned(),
        rp_name: "Paxport".to_owned(),
        origin: "https://paxportwallet.com".to_owned(),
        ceremony_ttl_secs: 300,
        assertion_ttl_secs: 300,
        session_ttl_secs: 900,
        refresh_ttl_secs: 86400,
        step_up_ttl_secs: 300,
        rate_limit: RateLimit {
            attempts: 10,
            window_secs: 60,
        },
    })?;
    let mut authenticator = SoftwareAuthenticator::new();
    let identity = AccountIdentity::new("intent-owner@example.test", "Intent owner")?;
    let registration = passkeys.begin_registration(scope, &identity, "Intent passkey", now)?;
    let credential = authenticator.register(&registration.ceremony);
    passkeys.finish_registration(scope, &registration.registration_id, &credential, now)?;
    let challenge = passkeys.begin_assertion(scope, now)?;
    let credential = authenticator.assert(&challenge.ceremony);
    passkeys.finish_assertion(scope, &challenge.assertion_id, &credential, now)?;
    let grant = passkeys.open_session(
        scope,
        &challenge.assertion_id,
        Device::mint("Intent browser", "web")?,
        now,
    )?;
    let index = AuthDiscoveryIndex::open(
        root.join("auth-index"),
        IndexAuthenticationKey::new([0x61; 32])
            .map_err(|error| format!("auth index key: {error:?}"))?,
    )
    .map_err(|error| format!("auth index: {error:?}"))?;
    index
        .bind_session(&grant, principal)
        .map_err(|error| format!("browser session binding: {error:?}"))?;
    grant.access_token().expose().clone_into(browser_access);
    grant.csrf_token().expose().clone_into(browser_csrf);
    Ok(())
}

fn assert_wallet_binding(
    identities: &layerx_human_service::server::RemoteIdentityProvider,
    producer: &p256::ecdsa::SigningKey,
    subject: &str,
    did: &str,
    assertion: &str,
) -> Result<()> {
    use p256::ecdsa::{signature::Signer as _, Signature};
    let header = json!({"alg": "ES256", "typ": "layerx-wallet-binding+jwt"});
    let claims = json!({"iss": cluster::ISSUER, "sub": subject, "did": did, "tenant": "tenant-a"});
    let input = format!(
        "{}.{}",
        URL_SAFE_NO_PAD.encode(header.to_string()),
        URL_SAFE_NO_PAD.encode(claims.to_string())
    );
    let signature: Signature = producer.sign(input.as_bytes());
    let binding = format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()));
    let bound = identities
        .resolve_assertion_with_binding(assertion, Some(&binding))
        .map_err(|error| format!("wallet binding: {error:?}"))?;
    assert_eq!(bound.did.as_deref(), Some(did));
    Ok(())
}

impl Fixture {
    pub(crate) fn open() -> Result<Self> {
        let root = layerx_human_test_support::directory("intent-plan-evm");
        fs::create_dir(&root)?;
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
        let clock_server = ClockServer::start(&root)?;
        let clock = Arc::new(SerialClock(Mutex::new(RuntimeClock::connect(
            ClockBinding {
                socket: clock_server.socket.clone(),
                process: std::process::id(),
                user: rustix::process::getuid().as_raw(),
            },
        )?)));
        let mut fixture = Self {
            root,
            custody_cluster: None,
            kms: None,
            original_roots: Vec::new(),
            assertion: String::new(),
            browser_access: String::new(),
            browser_csrf: String::new(),
            peer_state: Arc::new(Mutex::new(PeerState::default())),
            rpc_server: None,
            clock,
            clock_server: Some(clock_server),
            agent_server: None,
            identity_server: None,
            router: None,
            document: json!({}),
        };
        for (variable, value) in [
            (
                "LAYERX_RUNTIME_CLOCK_SOCKET",
                fixture
                    .clock_server
                    .as_ref()
                    .ok_or("clock server")?
                    .socket
                    .clone()
                    .into_os_string(),
            ),
            (
                "LAYERX_RUNTIME_CLOCK_PID",
                std::process::id().to_string().into(),
            ),
            (
                "LAYERX_RUNTIME_CLOCK_UID",
                rustix::process::getuid().as_raw().to_string().into(),
            ),
        ] {
            fixture
                .original_roots
                .push((variable, std::env::var_os(variable)));
            std::env::set_var(variable, value);
        }
        for (variable, name) in [
            ("LAYERX_HUMAN_STORE_ROOT", "store"),
            ("LAYERX_HUMAN_CUSTODY_ROOT", "custody"),
            ("LAYERX_HUMAN_AUTH_INDEX_ROOT", "auth-index"),
        ] {
            let destination = fixture.root.join(name);
            fs::create_dir(&destination)?;
            fs::set_permissions(&destination, fs::Permissions::from_mode(0o700))?;
            fixture
                .original_roots
                .push((variable, std::env::var_os(variable)));
            std::env::set_var(variable, destination);
        }
        let map = layerx_human_service::store::TenancyMap::new([])?;
        let digest = map.install(&fixture.root.join("store"))?;
        let variable = "LAYERX_HUMAN_TENANCY_DIGEST";
        fixture
            .original_roots
            .push((variable, std::env::var_os(variable)));
        std::env::set_var(variable, URL_SAFE_NO_PAD.encode(digest.bytes()));
        fixture.configure_services(digest.bytes())?;
        fixture.bootstrap()?;
        fixture.agent_server = Some(RealAgentLayer::start(
            &fixture.root,
            fixture.peer_state.clone(),
            fixture.rpc_server.as_ref().ok_or("RPC peer")?.url.clone(),
        )?);
        fixture.reopen()?;
        Ok(fixture)
    }

    pub(crate) fn configure_services(&mut self, tenancy_digest: [u8; 32]) -> Result<()> {
        let cluster = cluster::Cluster::start_with_kernel_policy(
            &[
                layerx_types::payload::ActivityType::new(layerx_types::payload::ModuleId::Asset, 5)
                    .map_err(|error| format!("send activity: {error:?}"))?
                    .value()
                    .to_string(),
            ],
            &json!({"version": 1, "defaults": {
                "modules": {"asset": [5]},
                "caps": {"native": {"per_operation": "1000000000000000000"}}
            }}),
        )?;
        self.rpc_server = Some(RpcServer::start(
            self.peer_state.clone(),
            self.clock.clone(),
        )?);
        let values = loader_environment(
            &self.root,
            &cluster,
            tenancy_digest,
            &self.rpc_server.as_ref().ok_or("RPC peer")?.url,
        )?;
        self.custody_cluster = Some(cluster);
        for (variable, value) in values {
            if !self
                .original_roots
                .iter()
                .any(|(name, _)| *name == variable)
            {
                self.original_roots
                    .push((variable, std::env::var_os(variable)));
            }
            std::env::set_var(variable, value);
        }
        write_private(
            &self.root.join("purpose-catalog.json"),
            &serde_json::to_vec(&purpose_catalog())?,
        )?;
        Ok(())
    }

    fn provision_account(
        &mut self,
        subject: &str,
        identities: &layerx_human_service::server::RemoteIdentityProvider,
        keystore: &layerx_human_service::custody::Keystore,
        key: &layerx_human_service::custody::KeyId,
        producer: &p256::ecdsa::SigningKey,
        store: &mut layerx_human_service::store::PrincipalStore,
    ) -> Result<(layerx_human_service::store::PrincipalId, String, String)> {
        use layerx_human_service::custody::KeyClass;
        use layerx_human_service::store::PrincipalId;
        let cluster = self.custody_cluster.as_mut().ok_or("attestor cluster")?;
        let kms = self.kms.as_ref().ok_or("attestor custody")?;
        let assertion = cluster.tokens.mint(subject)?;
        let pending = identities
            .resolve_assertion(&assertion)
            .map_err(|error| format!("assertion identity: {error:?}"))?;
        let principal = PrincipalId::new(pending.principal.as_str())?;
        let wallet_public = cluster
            .generate_key(&format!("wallet-{subject}"), "secp256k1", subject, "")
            .map_err(|error| {
                format!(
                    "wallet generation: {error}; refusal: {:?}; attestors: {}",
                    kms.last_refusal_code(),
                    cluster.logs()
                )
            })?;
        let wallet_address = layerx_crypto::secp256k1::evm_address(&wallet_public)
            .map_err(|error| format!("wallet address: {error:?}"))?;
        let account = format!("0x{}", cluster::hex(&wallet_address));
        let digest = keystore
            .binding(&principal, key, KeyClass::HumanPrimary)?
            .digest();
        let key_id = format!("lx-{}", cluster::hex(&digest[..16]));
        cluster.approve_generate(&key_id, "ed25519", subject)?;
        let public = kms
            .create_owned_key(keystore, &principal, key, subject, &account)
            .map_err(|error| {
                format!(
                    "primary key creation: {error:?}; refusal: {:?}; attestors: {}",
                    kms.last_refusal_code(),
                    cluster.logs()
                )
            })?;
        cluster.approve_sign(&key_id, &public)?;
        let did = format!("did:layerx:{}", cluster::hex(&public));
        assert_wallet_binding(identities, producer, subject, &did, &assertion)?;
        let now = self.clock.sample(Duration::from_secs(1))?.unix_seconds();
        let mut scope = store.principal(&principal)?;
        let start = layerx_human_service::onboarding::OnboardingStart::new(
            public,
            layerx_types::ids::Did::new(did.as_bytes())
                .map_err(|error| format!("onboarding DID: {error:?}"))?,
            layerx_human_service::onboarding::RecoveryPolicy::new(
                layerx_types::intent::RecoveryRoot::new([0x63; 32]),
                layerx_types::intent::ApprovalThreshold::new(1)
                    .map_err(|error| format!("recovery threshold: {error:?}"))?,
                86400,
            )?,
        )?;
        let mut onboarding =
            layerx_human_service::onboarding::OnboardingJourney::start(&mut scope, &start, now)?;
        onboarding.resume_local(&mut scope, keystore, now)?;
        if subject == "intent-owner" {
            bind_owner_browser_session(
                &self.root,
                &mut scope,
                &principal,
                now,
                &mut self.browser_access,
                &mut self.browser_csrf,
            )?;
        }
        drop(scope);
        kms.admit_assertion(subject, &assertion).map_err(|error| {
            format!(
                "assertion admission: {error:?}; refusal: {:?}; attestors: {}",
                kms.last_refusal_code(),
                cluster.logs()
            )
        })?;
        let wallet = keystore.evm_wallet(&principal, key).map_err(|error| {
            format!(
                "wallet lookup: {error:?}; refusal: {:?}; attestors: {}",
                kms.last_refusal_code(),
                cluster.logs()
            )
        })?;
        assert_eq!(
            wallet,
            wallet_address,
            "refusal: {:?}; attestors: {}",
            kms.last_refusal_code(),
            cluster.logs()
        );
        self.peer_state
            .lock()
            .map_err(|_| "peer state")?
            .identities
            .insert(did.clone(), (public, wallet));
        Ok((principal, did, assertion))
    }

    fn start_identity_provider(
        &mut self,
        key_set_url: String,
        producer: &p256::ecdsa::SigningKey,
    ) -> Result<layerx_human_service::server::RemoteIdentityProvider> {
        use layerx_human_identity_provider::{AssertionConfig, AssertionVerifier, Policy, State};
        use layerx_human_service::server::{IdentityProviderConfig, RemoteIdentityProvider};
        let mut state = State::open(
            &self.root.join("identity-state"),
            Policy {
                root: [0x63; 32],
                threshold: 1,
                delay_seconds: 86400,
            },
        )?;
        state.enable_assertion(
            AssertionVerifier::new(AssertionConfig {
                jwks_url: key_set_url,
                issuer: cluster::ISSUER.to_owned(),
                audience: cluster::AUDIENCE.to_owned(),
                clock_skew_seconds: 60,
                refresh_interval_seconds: 300,
            })?
            .with_binding_producer_key(&cluster::hex(
                producer.verifying_key().to_encoded_point(false).as_bytes(),
            ))?,
        )?;
        self.identity_server = Some(IdentityServer::start(
            &self.root,
            self.clock.clone(),
            state,
        )?);
        let identities = RemoteIdentityProvider::new(IdentityProviderConfig {
            socket: self.root.join("identity.sock"),
            deadline: Duration::from_secs(5),
            maximum_frame_bytes: MAX_FRAME,
            peer_uid: rustix::process::getuid().as_raw(),
            peer_gid: rustix::process::getgid().as_raw(),
        })
        .map_err(|error| format!("identity provider: {error:?}"))?;
        Ok(identities)
    }

    pub(crate) fn bootstrap(&mut self) -> Result<()> {
        use layerx_human_service::custody::{KeyId, Keystore};
        use layerx_human_service::server::production_components::AttestorKms;
        use p256::ecdsa::SigningKey;

        let key_set_url = self
            .custody_cluster
            .as_ref()
            .ok_or("attestor cluster")?
            .key_set_url();
        let producer = SigningKey::from_slice(&[0x45; 32])?;
        let identities = self.start_identity_provider(key_set_url, &producer)?;
        let config = ProductionComponentsConfig::from_environment()?;
        let kms = AttestorKms::connect(
            config
                .attestor_custody()
                .cloned()
                .ok_or("attestor config")?,
        )?;
        self.kms = Some(kms.clone());
        let keystore = Keystore::open_production(self.root.join("custody"), 125, kms.clone())?;
        let key = KeyId::new("human-primary")?;
        let binding = layerx_identity_binding::Client::new(
            layerx_identity_binding::Config {
                socket: self.root.join("identity-binding.sock"),
                tenant: "tenant-a".to_owned(),
                peer_uid: rustix::process::getuid().as_raw(),
                peer_gid: rustix::process::getgid().as_raw(),
                deadline: Duration::from_secs(5),
            },
            self.clock.clone(),
        )?;
        let tenancy_digest: [u8; 32] = URL_SAFE_NO_PAD
            .decode(std::env::var("LAYERX_HUMAN_TENANCY_DIGEST")?)?
            .try_into()
            .map_err(|_| "tenancy digest")?;
        let mut store = layerx_human_service::store::PrincipalStore::open_with_authority(
            self.root.join("store"),
            layerx_human_test_support::retention_uniform(86400),
            layerx_human_service::store::TenancyDigest::new(tenancy_digest),
            Arc::new(IdentityTenancy(binding)),
        )?;
        let mut accounts = Vec::new();
        for subject in ["intent-owner", "intent-recipient"] {
            accounts.push(self.provision_account(
                subject,
                &identities,
                &keystore,
                &key,
                &producer,
                &mut store,
            )?);
        }
        let (principal, actor, assertion) = &accounts[0];
        self.assertion.clone_from(assertion);
        let now = self.clock.sample(Duration::from_secs(1))?.unix_seconds();
        self.document = json!({"principal": principal.as_str(), "actor": actor, "not_after": now + 300,
            "intent": {"source": {"kind": "human", "account": format!("agent:{actor}:main")},
                "destination": {"kind": "human", "account": format!("agent:{}:main", accounts[1].1)},
                "asset_id": cluster::hex(&ASSET), "money": {"amount": "1", "currency": "LXP"},
                "constraints": {"deadline": time::rfc3339(now + 300),
                    "max_fee": {"amount": "1000000000000000000", "currency": "LXP"}, "allow_top_up": false}}});
        Ok(())
    }

    pub(crate) fn reopen(&mut self) -> Result<()> {
        self.close_router()?;
        let components = ProductionComponents::open(
            ProductionComponentsConfig::from_environment()?,
            self.clock.clone(),
        )?;
        components
            .attestor_custody()
            .ok_or("attestor custody")?
            .admit_assertion("intent-owner", &self.assertion)?;
        self.router = Some(Arc::new(
            Router::new(
                Arc::new(components),
                PrincipalLimits::new(100, 60, 100).map_err(|e| format!("limits: {e:?}"))?,
                HttpConfig {
                    maximum_header_bytes: 32768,
                    maximum_body_bytes: 1_048_576,
                    allowed_origin: "https://paxportwallet.com".to_owned(),
                    service_version: "intent-plan-evm".to_owned(),
                },
                self.clock.clone(),
            )
            .map_err(|e| format!("router: {e:?}"))?,
        ));
        Ok(())
    }

    pub(crate) fn close_router(&mut self) -> Result<()> {
        if self.router.take().is_none() {
            return Ok(());
        }
        let lock = fs::OpenOptions::new()
            .read(true)
            .write(true)
            .open(self.root.join("store/provider.lock"))?;
        let mut expires = Deadline::start(self.clock.as_ref(), Duration::from_secs(5))?;
        loop {
            match rustix::fs::flock(&lock, rustix::fs::FlockOperation::NonBlockingLockExclusive) {
                Ok(()) => return Ok(()),
                Err(rustix::io::Errno::WOULDBLOCK)
                    if !expires.remaining(self.clock.as_ref())?.is_zero() =>
                {
                    std::thread::sleep(Duration::from_millis(1));
                }
                Err(error) => return Err(std::io::Error::from(error).into()),
            }
        }
    }

    pub(crate) fn request(
        &self,
        method: &str,
        path: &str,
        body: &Value,
        key: &str,
    ) -> Result<Value> {
        let (mut client, mut server) = UnixStream::pair()?;
        client.set_read_timeout(Some(Duration::from_secs(30)))?;
        let router = self.router.as_ref().ok_or("router closed")?.clone();
        let worker = std::thread::spawn(move || router.serve_one(&mut server, "intent-plan-evm"));
        let encoded = serde_json::to_vec(body)?;
        let csrf = if path.starts_with("/v1/moves") {
            format!(
                "; __Host-layerx_csrf={}\r\nX-LayerX-CSRF: {}",
                self.browser_csrf, self.browser_csrf
            )
        } else {
            String::new()
        };
        write!(client, "{method} {path} HTTP/1.1\r\nHost: paxportwallet.com\r\nOrigin: https://paxportwallet.com\r\nAuthorization: Bearer {}\r\nCookie: __Host-layerx_access={}{csrf}\r\nIdempotency-Key: {key}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", self.assertion, self.browser_access, encoded.len())?;
        client.write_all(&encoded)?;
        let mut response = String::new();
        client.read_to_string(&mut response)?;
        worker.join().map_err(|_| "router worker panicked")??;
        let (headers, body) = response.split_once("\r\n\r\n").ok_or("HTTP response")?;
        let (agent_calls, node_calls, gateway_calls, gateway_response) = {
            let peer = self
                .peer_state
                .lock()
                .unwrap_or_else(std::sync::PoisonError::into_inner);
            (
                peer.served.clone(),
                peer.rpc_served.clone(),
                peer.gateway_served.clone(),
                peer.gateway_last_answer.clone(),
            )
        };
        assert!(
            headers.starts_with("HTTP/1.1 200 ")
                || headers.starts_with("HTTP/1.1 201 ")
                || headers.starts_with("HTTP/1.1 202 "),
            "{method} {path}: {response}; agent: {:?}; node: {:?}; gateway: {:?}; last gateway answer: {:?}; document: {}; attestor refusal: {:?}; attestors: {}",
            agent_calls,
            node_calls,
            gateway_calls,
            gateway_response,
            self.document,
            self.kms.as_ref().and_then(layerx_human_service::server::production_components::AttestorKms::last_refusal_code),
            self.custody_cluster.as_ref().map(cluster::Cluster::logs).unwrap_or_default()
        );
        let response: Value = serde_json::from_str(body)?;
        Ok(response["result"].clone())
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        {
            let peer = self
                .peer_state
                .lock()
                .unwrap_or_else(std::sync::PoisonError::into_inner);
            eprintln!(
                "agent: {:?}; node: {:?}; gateway: {:?}; document: {}",
                peer.served, peer.rpc_served, peer.gateway_served, self.document
            );
        }
        if let Some(cluster) = &self.custody_cluster {
            eprintln!(
                "attestor refusal: {:?}; attestor logs: {}",
                self.kms.as_ref().and_then(layerx_human_service::server::production_components::AttestorKms::last_refusal_code),
                cluster.logs()
            );
        }
        self.router.take();
        self.agent_server.take();
        self.identity_server.take();
        self.rpc_server.take();
        self.custody_cluster.take();
        for (variable, original) in self.original_roots.iter().rev() {
            match original {
                Some(value) => std::env::set_var(variable, value),
                None => std::env::remove_var(variable),
            }
        }
        self.clock_server.take();
        let _ = fs::remove_dir_all(&self.root);
    }
}

pub(crate) fn write_private(path: &std::path::Path, bytes: &[u8]) -> std::io::Result<()> {
    fs::write(path, bytes)?;
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))
}

pub(crate) fn purpose_catalog() -> Value {
    let counterparty = [1_u8; 32];
    let asset = [2_u8; 32];
    json!({"version": "production-unhooked-v1", "presets": [{
        "id": "bounded-send", "call_selectors": ["0xa9059cbb"],
        "counterparties": [counterparty], "assets": [asset],
        "amount_ceiling": 1000, "rate_maximum_uses": 10,
        "rate_window_sequences": 100, "purposes": ["send"],
        "expiry_sequence": 1000, "session_scopes": ["send"],
        "session_lifetime_seconds": 300, "budget_asset": asset,
        "budget_period_seconds": 60, "budget_expiry_seconds": 600,
        "initial_funding": 100
    }]})
}

fn events_loader_environment(
    root: &std::path::Path,
    events_url: String,
) -> Vec<(&'static str, String)> {
    let path = |name: &str| root.join(name).display().to_string();
    vec![
        ("LAYERX_EVENTS_JOURNEY_UPSTREAM_URL", events_url.clone()),
        ("LAYERX_EVENTS_APPROVAL_UPSTREAM_URL", events_url.clone()),
        ("LAYERX_EVENTS_WEBHOOKS_UPSTREAM_URL", events_url),
        (
            "LAYERX_EVENTS_JOURNEY_UPSTREAM_CA_DER",
            path("attestor-ca.der"),
        ),
        (
            "LAYERX_EVENTS_APPROVAL_UPSTREAM_CA_DER",
            path("attestor-ca.der"),
        ),
        (
            "LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CA_DER",
            path("attestor-ca.der"),
        ),
        (
            "LAYERX_EVENTS_JOURNEY_UPSTREAM_CLIENT_IDENTITY_PKCS12",
            path("events-client.p12"),
        ),
        (
            "LAYERX_EVENTS_APPROVAL_UPSTREAM_CLIENT_IDENTITY_PKCS12",
            path("events-client.p12"),
        ),
        (
            "LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CLIENT_IDENTITY_PKCS12",
            path("events-client.p12"),
        ),
        (
            "LAYERX_EVENTS_JOURNEY_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE",
            path("events-password"),
        ),
        (
            "LAYERX_EVENTS_APPROVAL_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE",
            path("events-password"),
        ),
        (
            "LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE",
            path("events-password"),
        ),
        (
            "LAYERX_EVENTS_JOURNEY_UPSTREAM_TOKEN_FILE",
            path("events-token"),
        ),
        (
            "LAYERX_EVENTS_APPROVAL_UPSTREAM_TOKEN_FILE",
            path("events-token"),
        ),
        (
            "LAYERX_EVENTS_WEBHOOKS_UPSTREAM_TOKEN_FILE",
            path("events-token"),
        ),
    ]
}

fn base_loader_environment(
    root: &std::path::Path,
    tenancy_digest: [u8; 32],
    rpc_url: &str,
) -> Vec<(&'static str, String)> {
    let path = |name: &str| root.join(name).display().to_string();
    let secret = |byte: u8| URL_SAFE_NO_PAD.encode([byte; 32]);
    vec![
        ("LAYERX_HUMAN_STORE_ROOT", path("store")),
        ("LAYERX_HUMAN_CUSTODY_ROOT", path("custody")),
        ("LAYERX_HUMAN_AUTH_INDEX_ROOT", path("auth-index")),
        (
            "LAYERX_HUMAN_TENANCY_DIGEST",
            URL_SAFE_NO_PAD.encode(tenancy_digest),
        ),
        ("LAYERX_HUMAN_AUTH_INDEX_KEY", secret(0x61)),
        ("LAYERX_HUMAN_STREAM_CURSOR_KEY", secret(0x62)),
        ("LAYERX_HUMAN_RP_ID", "paxportwallet.com".to_owned()),
        ("LAYERX_HUMAN_RP_NAME", "Paxport".to_owned()),
        (
            "LAYERX_HUMAN_ORIGIN",
            "https://paxportwallet.com".to_owned(),
        ),
        ("LAYERX_HUMAN_CEREMONY_TTL_SECONDS", "300".to_owned()),
        ("LAYERX_HUMAN_ASSERTION_TTL_SECONDS", "300".to_owned()),
        ("LAYERX_HUMAN_SESSION_TTL_SECONDS", "900".to_owned()),
        ("LAYERX_HUMAN_REFRESH_TTL_SECONDS", "86400".to_owned()),
        ("LAYERX_HUMAN_STEP_UP_TTL_SECONDS", "300".to_owned()),
        ("LAYERX_HUMAN_AUTH_RATE_ATTEMPTS", "10".to_owned()),
        ("LAYERX_HUMAN_AUTH_RATE_WINDOW_SECONDS", "60".to_owned()),
        (
            "LAYERX_HUMAN_RETENTION_JOURNEYS_SECONDS",
            "86400".to_owned(),
        ),
        (
            "LAYERX_HUMAN_RETENTION_NOTIFICATIONS_SECONDS",
            "86400".to_owned(),
        ),
        ("LAYERX_HUMAN_RETENTION_AUDIT_SECONDS", "86400".to_owned()),
        (
            "LAYERX_HUMAN_RETENTION_TELEMETRY_SECONDS",
            "86400".to_owned(),
        ),
        ("LAYERX_HUMAN_RETENTION_CACHE_SECONDS", "86400".to_owned()),
        ("LAYERX_HUMAN_CAPABILITY_TTL_SECONDS", "30".to_owned()),
        ("LAYERX_HUMAN_AGENT_SOCKET", path("agent-socket/agent.sock")),
        ("LAYERX_HUMAN_AGENT_MAX_FRAME_BYTES", "1048576".to_owned()),
        ("LAYERX_HUMAN_AGENT_MAX_CONNECTIONS", "4".to_owned()),
        ("LAYERX_HUMAN_AGENT_MAX_STREAMS", "4".to_owned()),
        ("LAYERX_HUMAN_AGENT_MAX_QUEUED_BYTES", "1048576".to_owned()),
        ("LAYERX_HUMAN_AGENT_DEADLINE_SECONDS", "5".to_owned()),
        ("LAYERX_AGENT_NODE_RPC_URL", rpc_url.to_owned()),
        ("LAYERX_AGENT_NODE_CHAIN_ID", "125".to_owned()),
        ("LAYERX_NETWORK_GATEWAY_ENDPOINT", rpc_url.to_owned()),
        ("LAYERX_HUMAN_SECURITY_SOCKET", path("security.sock")),
        ("LAYERX_HUMAN_SECURITY_DEADLINE_SECONDS", "5".to_owned()),
        (
            "LAYERX_HUMAN_SECURITY_MAX_FRAME_BYTES",
            "1048576".to_owned(),
        ),
    ]
}

fn identity_loader_environment(root: &std::path::Path) -> Vec<(&'static str, String)> {
    let path = |name: &str| root.join(name).display().to_string();
    vec![
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_SOCKET",
            path("identity-binding.sock"),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_TENANT",
            "tenant-a".to_owned(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_PEER_UID",
            rustix::process::getuid().as_raw().to_string(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_PEER_GID",
            rustix::process::getgid().as_raw().to_string(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_DEADLINE_SECONDS",
            "10".to_owned(),
        ),
        ("LAYERX_HUMAN_IDENTITY_SOCKET", path("identity.sock")),
        ("LAYERX_HUMAN_IDENTITY_DEADLINE_SECONDS", "5".to_owned()),
        (
            "LAYERX_HUMAN_IDENTITY_MAX_FRAME_BYTES",
            "1048576".to_owned(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_PEER_UID",
            rustix::process::getuid().as_raw().to_string(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_PEER_GID",
            rustix::process::getgid().as_raw().to_string(),
        ),
    ]
}

fn attestor_loader_environment(
    root: &std::path::Path,
    nodes: String,
) -> Vec<(&'static str, String)> {
    let path = |name: &str| root.join(name).display().to_string();
    let secret = |byte: u8| URL_SAFE_NO_PAD.encode([byte; 32]);
    vec![
        ("LAYERX_HUMAN_ATTESTOR_NODES", nodes),
        ("LAYERX_HUMAN_ATTESTOR_SIGNERS", cluster::SIGNERS.join(",")),
        (
            "LAYERX_HUMAN_ATTESTOR_ROOT_CERTIFICATE_DER",
            path("attestor-ca.der"),
        ),
        (
            "LAYERX_HUMAN_ATTESTOR_CLIENT_CERTIFICATE_DER",
            path("attestor-client.der"),
        ),
        (
            "LAYERX_HUMAN_ATTESTOR_CLIENT_PRIVATE_KEY_DER",
            path("attestor-client-key.der"),
        ),
        ("LAYERX_HUMAN_ATTESTOR_DEADLINE_SECONDS", "120".to_owned()),
        ("LAYERX_HUMAN_NETWORK_ID", "125".to_owned()),
        ("LAYERX_HUMAN_SIGNING_RATE_MAXIMUM", "1000".to_owned()),
        (
            "LAYERX_HUMAN_SIGNING_RATE_WINDOW_SECONDS",
            "10000".to_owned(),
        ),
        (
            "LAYERX_HUMAN_AGENT_ACTOR",
            format!("did:layerx:{}", cluster::hex(&[0x77; 32])),
        ),
        (
            "LAYERX_HUMAN_AGENT_AUTHORITY",
            "custody-human-primary".to_owned(),
        ),
        (
            "LAYERX_HUMAN_AGENT_TIMESTAMP_SPAN_SECONDS",
            "300".to_owned(),
        ),
        ("LAYERX_HUMAN_AGENT_FEE_LIMIT", "1000000".to_owned()),
        (
            "LAYERX_HUMAN_ONBOARDING_SPONSOR_PRINCIPAL",
            "onboarding-sponsor".to_owned(),
        ),
        ("LAYERX_HUMAN_ONBOARDING_INITIAL_FUNDING", "1000".to_owned()),
        ("LAYERX_HUMAN_EVM_GAS_LIMIT", "300000".to_owned()),
        ("LAYERX_HUMAN_EVM_MAX_FEE_PER_GAS", "1000000000".to_owned()),
        (
            "LAYERX_HUMAN_EVM_MAX_PRIORITY_FEE_PER_GAS",
            "1000000000".to_owned(),
        ),
        (
            "LAYERX_HUMAN_AGENT_PURPOSE_CATALOG",
            path("purpose-catalog.json"),
        ),
        (
            "LAYERX_HUMAN_AGENT_OWNER_ACCOUNT",
            "0x00000000000000000000000000000000000000a1".to_owned(),
        ),
        ("LAYERX_HUMAN_AGENT_RECOVERY_ROOT", secret(0x63)),
        ("LAYERX_HUMAN_AGENT_RECOVERY_THRESHOLD", "1".to_owned()),
        ("LAYERX_HUMAN_ACTIVITY_FRESHNESS_SECONDS", "300".to_owned()),
        (
            "LAYERX_HUMAN_ACTIVITY_EXPORT_MAXIMUM_BYTES",
            "1048576".to_owned(),
        ),
        (
            "LAYERX_HUMAN_CONTINUATION_UNKNOWN_DEADLINE_SECONDS",
            "300".to_owned(),
        ),
    ]
}

pub(crate) fn loader_environment(
    root: &std::path::Path,
    cluster: &cluster::Cluster,
    tenancy_digest: [u8; 32],
    rpc_url: &str,
) -> std::result::Result<Vec<(&'static str, String)>, Box<dyn std::error::Error>> {
    write_private(
        &root.join("attestor-ca.der"),
        &cluster.pki.certificate_der("node-ca")?,
    )?;
    write_private(
        &root.join("attestor-client.der"),
        &cluster.pki.certificate_der("gateway")?,
    )?;
    write_private(
        &root.join("attestor-client-key.der"),
        &cluster.pki.key_der("gateway")?,
    )?;
    write_private(
        &root.join("events-token"),
        b"intent-integration-producer-token",
    )?;
    write_private(&root.join("events-password"), b"intent-integration-pkcs12")?;
    let status = std::process::Command::new("openssl")
        .args(["pkcs12", "-export", "-inkey"])
        .arg(cluster.pki.path("gateway.key"))
        .arg("-in")
        .arg(cluster.pki.path("gateway.pem"))
        .arg("-certfile")
        .arg(cluster.pki.path("gateway-ca.pem"))
        .arg("-passout")
        .arg(format!("file:{}", root.join("events-password").display()))
        .arg("-out")
        .arg(root.join("events-client.p12"))
        .status()?;
    if !status.success() {
        return Err("event client identity export failed".into());
    }
    fs::set_permissions(
        root.join("events-client.p12"),
        fs::Permissions::from_mode(0o600),
    )?;
    let nodes = cluster
        .nodes
        .iter()
        .map(|(id, address)| format!("{id}={address}"))
        .collect::<Vec<_>>()
        .join(",");
    let events_url = format!("https://localhost:{}", cluster.nodes[0].1.port());
    let mut environment = events_loader_environment(root, events_url);
    environment.extend(base_loader_environment(root, tenancy_digest, rpc_url));
    environment.extend(identity_loader_environment(root));
    environment.extend(attestor_loader_environment(root, nodes));
    Ok(environment)
}

pub(crate) fn required<T, E: std::fmt::Debug>(result: std::result::Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error:?}"))
}

pub(crate) struct SoftwareAuthenticator {
    signing_key: ed25519_dalek::SigningKey,
    credential_id: Vec<u8>,
    counter: u32,
    user_handle: Option<String>,
}

impl SoftwareAuthenticator {
    fn new() -> Self {
        let mut seed = [0_u8; 32];
        required(getrandom::fill(&mut seed), "authenticator entropy");
        let mut credential_id = vec![0_u8; 32];
        required(
            getrandom::fill(&mut credential_id),
            "credential identifier entropy",
        );
        Self {
            signing_key: ed25519_dalek::SigningKey::from_bytes(&seed),
            credential_id,
            counter: 0,
            user_handle: None,
        }
    }

    fn register(&mut self, ceremony: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.user_handle = Some(required_text(&options, "/user/id").to_owned());
        let client_data = client_data("webauthn.create", challenge);
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "transports": ["internal"],
            "attestationObject": URL_SAFE_NO_PAD.encode(self.attestation_object()),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
        }))
    }

    fn assert(&mut self, ceremony: &str) -> String {
        use ed25519_dalek::Signer as _;
        use sha2::Digest as _;
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.counter = self.counter.saturating_add(1);
        let authenticator_data = self.authenticator_data(self.counter, false);
        let client_data = client_data("webauthn.get", challenge);
        let client_hash = sha2::Sha256::digest(&client_data);
        let mut signed = Vec::with_capacity(authenticator_data.len() + client_hash.len());
        signed.extend_from_slice(&authenticator_data);
        signed.extend_from_slice(&client_hash);
        let signature = self.signing_key.sign(&signed).to_bytes();
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "authenticatorData": URL_SAFE_NO_PAD.encode(authenticator_data),
            "signature": URL_SAFE_NO_PAD.encode(signature),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
            "userHandle": self.user_handle,
        }))
    }

    fn attestation_object(&self) -> Vec<u8> {
        let map = ciborium::value::Value::Map(vec![
            (
                ciborium::value::Value::Text("fmt".to_owned()),
                ciborium::value::Value::Text("none".to_owned()),
            ),
            (
                ciborium::value::Value::Text("attStmt".to_owned()),
                ciborium::value::Value::Map(Vec::new()),
            ),
            (
                ciborium::value::Value::Text("authData".to_owned()),
                ciborium::value::Value::Bytes(self.authenticator_data(0, true)),
            ),
        ]);
        let mut bytes = Vec::new();
        required(
            ciborium::ser::into_writer(&map, &mut bytes),
            "encode attestation",
        );
        bytes
    }

    fn authenticator_data(&self, counter: u32, attested: bool) -> Vec<u8> {
        use sha2::Digest as _;
        let mut bytes = Vec::new();
        bytes.extend_from_slice(&sha2::Sha256::digest(b"paxportwallet.com"));
        bytes.push(1 | 4 | if attested { 64 } else { 0 });
        bytes.extend_from_slice(&counter.to_be_bytes());
        if attested {
            bytes.extend_from_slice(&[0_u8; 16]);
            let length = u16::try_from(self.credential_id.len())
                .unwrap_or_else(|_| panic!("credential identifier too long"));
            bytes.extend_from_slice(&length.to_be_bytes());
            bytes.extend_from_slice(&self.credential_id);
            bytes.extend_from_slice(&self.cose_public_key());
        }
        bytes
    }

    fn cose_public_key(&self) -> Vec<u8> {
        let map = ciborium::value::Value::Map(vec![
            (
                ciborium::value::Value::Integer(1.into()),
                ciborium::value::Value::Integer(1.into()),
            ),
            (
                ciborium::value::Value::Integer(3.into()),
                ciborium::value::Value::Integer((-8).into()),
            ),
            (
                ciborium::value::Value::Integer((-1).into()),
                ciborium::value::Value::Integer(6.into()),
            ),
            (
                ciborium::value::Value::Integer((-2).into()),
                ciborium::value::Value::Bytes(self.signing_key.verifying_key().to_bytes().to_vec()),
            ),
        ]);
        let mut bytes = Vec::new();
        required(
            ciborium::ser::into_writer(&map, &mut bytes),
            "encode public key",
        );
        bytes
    }
}

pub(crate) fn decode_ceremony(value: &str) -> Value {
    let bytes = required(URL_SAFE_NO_PAD.decode(value), "decode ceremony");
    required(serde_json::from_slice(&bytes), "parse ceremony")
}

pub(crate) fn required_text<'value>(value: &'value Value, pointer: &str) -> &'value str {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .unwrap_or_else(|| panic!("missing ceremony field {pointer}"))
}

pub(crate) fn encode_response(value: &Value) -> String {
    URL_SAFE_NO_PAD.encode(required(serde_json::to_vec(value), "serialize response"))
}

pub(crate) fn client_data(kind: &str, challenge: &str) -> Vec<u8> {
    required(
        serde_json::to_vec(&json!({
            "type": kind,
            "challenge": challenge,
            "origin": "https://paxportwallet.com",
            "crossOrigin": false,
        })),
        "encode client data",
    )
}

use crate::require_value::RequireValue as _;
