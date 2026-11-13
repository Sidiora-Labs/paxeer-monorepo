pub mod protocol_time {
    include!("support/protocol_time.rs");
}
use protocol_time::ProtocolTime;

use layerx_client::rpc::evidence::checkpoint;
use layerx_client::runtime_clock::RuntimeClock;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_types::clock::{Clock, Deadline};
use std::fmt::Debug;
use std::fs;
use std::future::Future;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::time::Duration;

use layerx_agent_api::error::RequestId;
use layerx_agent_api::idempotency::{BodyDigest, IdempotentMutation, Key};
use layerx_agent_api::identity::{
    AgentDid, AuthorityRef, NativeSendPurposeV2, SessionId, SignedNativeSendPurposeV2,
};
use layerx_agent_api::prepare::{IdempotencyRef, PayloadBytes, PrepareRequest, TimestampBound};
use layerx_human_service::audit::{AuditChain, AuditEvent, Decision};
use layerx_human_service::custody::{
    CustodySigner, KeyId, Keystore, Operation, RemoteKmsProvider, SigningLimits,
};
use layerx_human_service::frame::{Limits as FrameLimits, MutualTlsConfig};
use layerx_human_service::journeys::{
    AgentBoundary, JourneyEngine, JourneyKind, JourneyLeg, JourneyPhase, JourneyPlan, JourneyState,
};
use layerx_human_service::notify::JourneyId;
use layerx_human_service::server::agent_runtime::{AgentRuntime, NativeEffectApprovalFactState};
use layerx_human_service::store::{
    PrincipalId, PrincipalScope, PrincipalStore, RetentionPeriod, RetentionPolicy, RowKey, Table,
    TenancyDigest,
};
use layerx_human_service::trace::TraceId;
use layerx_intents::{Intent, IntentKind, LxpSend};
use layerx_sdk::{Call, Client as AgentClient};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, Did, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey, SendAuthorization,
    SendAuthorizationKind, Sequence, TimestampSeconds,
};
use native_tls::{Certificate, TlsConnector};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::RootCertStore;
use serde_json::Value;
use sha2::{Digest, Sha256};

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|_| panic!("genuine native journey boundary refused"))
}
fn text<'a>(value: &'a Value, key: &str) -> &'a str {
    value[key]
        .as_str()
        .or_fail("genuine protected fixture field")
}
fn number(value: &Value, key: &str) -> u64 {
    value[key]
        .as_u64()
        .or_fail("genuine bounded protocol value")
}
fn protected(path: &str, bound: u64) -> Vec<u8> {
    let info = checked(fs::symlink_metadata(path));
    assert!(
        info.is_file()
            && info.len() > 0
            && info.len() <= bound
            && info.permissions().mode().trailing_zeros() >= 6
    );
    checked(fs::read(path))
}
fn decode_hex(value: &str) -> Vec<u8> {
    assert!(
        value.len().is_multiple_of(2)
            && value
                .bytes()
                .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    );
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| checked(u8::from_str_radix(checked(std::str::from_utf8(pair)), 16)))
        .collect()
}
fn id(value: &str) -> [u8; 32] {
    let id = checked(decode_hex(value).try_into());
    assert_ne!(id, [0; 32]);
    id
}
fn frame_limits() -> FrameLimits {
    FrameLimits {
        maximum_frame_bytes: 1_212_416,
        maximum_connections: 1,
        maximum_streams: 1,
        maximum_queued_bytes: 1_212_416,
        deadline: Duration::from_secs(8),
    }
}
fn clock(fixture: &Value) -> ProtocolTime {
    let runtime_clock = checked(RuntimeClock::from_environment());
    let mut agent = runtime_with_clock(
        fixture,
        text(fixture, "principal"),
        text(fixture, "owner"),
        text(fixture, "account"),
        runtime_clock.clone(),
    );
    let head = checked(agent.head());
    clock_at(fixture, head.chain_sequence)
}

fn clock_at(fixture: &Value, height: u64) -> ProtocolTime {
    let runtime_clock = checked(RuntimeClock::from_environment());
    let listener = checked(TcpListener::bind("127.0.0.1:0"));
    let endpoint = checked(SequencerEndpoint::new(
        &format!("http://{}/", checked(listener.local_addr())),
        &[],
    ));
    checked(listener.set_nonblocking(true));
    let upstream = text(fixture, "rpc_url").to_owned();
    let server = std::thread::spawn(move || {
        let mut deadline = checked(Deadline::start(
            runtime_clock.as_ref(),
            Duration::from_secs(8),
        ));
        let stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(
                        !checked(deadline.remaining(runtime_clock.as_ref())).is_zero(),
                        "missing sequencer head read"
                    );
                    std::thread::sleep(Duration::from_millis(1));
                }
                Err(error) => panic!("sequencer head accept: {error}"),
            }
        };
        checked(stream.set_read_timeout(Some(Duration::from_secs(8))));
        checked(stream.set_write_timeout(Some(Duration::from_secs(8))));
        let mut reader = BufReader::new(stream);
        let mut length = None;
        loop {
            let mut line = String::new();
            assert_ne!(checked(reader.read_line(&mut line)), 0);
            if line == "\r\n" {
                break;
            }
            if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                length = Some(checked(value.trim().parse::<usize>()));
            }
        }
        let length = length.or_fail("RPC content length");
        assert!(length <= 65_536);
        let mut body = vec![0; length];
        checked(reader.read_exact(&mut body));
        let request: Value = checked(serde_json::from_slice(&body));
        assert_eq!(request["method"], "eth_getBlockByNumber");
        assert_eq!(
            request["params"],
            serde_json::json!([format!("{height:#x}"), true])
        );
        let mut response = checked(
            ureq::Agent::config_builder()
                .timeout_global(Some(Duration::from_secs(8)))
                .build()
                .new_agent()
                .post(&upstream)
                .header("Content-Type", "application/json")
                .send(&body),
        );
        let answer: Value = checked(serde_json::from_slice(&checked(
            response.body_mut().read_to_vec(),
        )));
        assert_eq!(answer["id"], request["id"]);
        let root = text(&answer["result"], "stateRoot")
            .strip_prefix("0x")
            .or_fail("served sequencer state root");
        assert_ne!(id(root), [0; 32]);
        let answer = checked(serde_json::to_vec(&answer));
        let mut stream = reader.into_inner();
        checked(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", answer.len()));
        checked(stream.write_all(&answer));
        checked(stream.flush());
    });
    let observed = checked(checkpoint(&endpoint, height));
    checked(server.join());
    assert_eq!(observed.number, height);
    ProtocolTime::from_block(&observed)
}

fn store(fixture: &Value) -> PrincipalStore {
    let retention = RetentionPeriod::new(number(fixture, "retention_seconds"));
    checked(PrincipalStore::open(
        text(fixture, "human_store"),
        RetentionPolicy {
            journeys: retention,
            notifications: retention,
            audit: retention,
            telemetry: retention,
            cache: retention,
        },
        TenancyDigest::new(id(text(fixture, "tenancy_digest"))),
    ))
}
fn runtime(fixture: &Value, principal: &str, owner: &str, account: &str) -> AgentRuntime {
    let clock = checked(RuntimeClock::from_environment());
    runtime_with_clock(fixture, principal, owner, account, clock)
}
fn runtime_with_clock(
    fixture: &Value,
    principal: &str,
    owner: &str,
    account: &str,
    clock: Arc<dyn Clock>,
) -> AgentRuntime {
    checked(
        checked(AgentRuntime::connect_with_clock(
            text(fixture, "agent_socket"),
            frame_limits(),
            clock,
        ))
        .for_subject(
            &checked(PrincipalId::new(principal)),
            &checked(Did::new(owner.as_bytes())),
            &checked(AccountId::parse(account)),
            id(text(fixture, "asset_id")),
        ),
    )
}
fn custody(fixture: &Value, registry: &layerx_types::payload::ModuleRegistry) -> CustodySigner {
    let clock = checked(RuntimeClock::from_environment());
    let mut roots = RootCertStore::empty();
    checked(roots.add(CertificateDer::from(protected(
        text(fixture, "kms_ca_der"),
        65_536,
    ))));
    let tls = checked(MutualTlsConfig::new(
        roots,
        vec![CertificateDer::from(protected(
            text(fixture, "kms_client_cert_der"),
            65_536,
        ))],
        PrivateKeyDer::Pkcs8(PrivatePkcs8KeyDer::from(protected(
            text(fixture, "kms_client_key_der"),
            65_536,
        ))),
    ));
    let provider = checked(RemoteKmsProvider::new(
        text(fixture, "kms_provider"),
        checked(text(fixture, "kms_endpoint").parse()),
        text(fixture, "kms_server_name"),
        tls,
        frame_limits(),
        clock,
    ));
    let keys = checked(Keystore::open_production(
        text(fixture, "custody_store"),
        checked(number(fixture, "network_id").try_into()),
        provider,
    ));
    CustodySigner::new(
        keys,
        store(fixture),
        registry.clone(),
        checked(SigningLimits::new(
            checked(number(fixture, "signing_maximum").try_into()),
            number(fixture, "signing_window_seconds"),
        )),
    )
}
struct Ready;
impl Wake for Ready {
    fn wake(self: Arc<Self>) {}
}
fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(Ready));
    let mut context = Context::from_waker(&waker);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("bounded synchronous native boundary did not complete"),
    }
}
fn plan(fixture: &Value, case: &Value) -> JourneyPlan {
    let data: Value = checked(serde_json::from_slice(&protected(
        text(case, "intent_file"),
        65_536,
    )));
    let authorization = SendAuthorization::new(
        SendAuthorizationKind::Owner,
        PublicKey::new(id(text(&data, "authorization_public_key"))),
        AuthorizationSignature::new(checked(
            decode_hex(text(&data, "authorization_signature")).try_into(),
        )),
    );
    let send = checked(LxpSend::new(
        checked(AccountId::parse(text(&data, "from"))),
        checked(AccountId::parse(text(&data, "to"))),
        AssetId::new(id(text(fixture, "asset_id"))),
        Amount::from_u128(checked(text(&data, "amount").parse())),
        Sequence::from_u64(number(&data, "account_sequence")),
        IdempotencyKey::new(id(text(&data, "action_key"))),
        TimestampSeconds::from_u64(number(&data, "expires_at")),
        ContextHash::new(id(text(&data, "context_hash"))),
        authorization,
        checked(NetworkId::new(checked(
            number(fixture, "network_id").try_into(),
        ))),
        checked(ProtocolVersion::new(3)),
    ));
    let leg = checked(JourneyLeg::new(
        Intent::v1(IntentKind::LxpSend(send)),
        id(text(&data, "action_key")),
        checked(AgentDid::new(text(fixture, "owner"))),
        checked(AuthorityRef::new(text(&data, "authority"))),
        number(&data, "account_sequence"),
        number(&data, "account_sequence"),
        number(&data, "not_before"),
        number(&data, "not_after"),
        checked(text(&data, "fee_limit").parse()),
    ));
    checked(JourneyPlan::new(
        checked(JourneyId::new(text(case, "journey_id"))),
        JourneyKind::Move,
        id(text(case, "request_key")),
        checked(KeyId::new(text(fixture, "custody_key"))),
        Operation::ProtocolMutation,
        vec![leg],
    ))
}
fn row(case: &Value) -> RowKey {
    checked(RowKey::new(format!(
        "journey-{}",
        text(case, "request_key")
    )))
}
fn journal(scope: &PrincipalScope<'_>, case: &Value) -> Value {
    checked(serde_json::from_slice(
        scope
            .get(Table::Journeys, &row(case))
            .or_fail("actual retained native journey")
            .bytes(),
    ))
}
fn bytes(value: &Value) -> Vec<u8> {
    checked(serde_json::from_value(value.clone()))
}
fn original_call(
    contract: &AgentClient,
    journal: &Value,
) -> Call<IdempotentMutation<PrepareRequest>> {
    let leg = &journal["legs"][0];
    let prep = &leg["preparation"];
    let action: [u8; 32] = checked(bytes(&leg["action_key"]).try_into());
    let request = PrepareRequest {
        protocol_activity_type: checked(number(leg, "activity_type").try_into()),
        actor: checked(AgentDid::new(text(prep, "actor"))),
        authority: checked(AuthorityRef::new(text(prep, "authority"))),
        account_sequence: layerx_agent_api::Sequence(number(prep, "account_sequence")),
        timestamp_bound: checked(
            TimestampBound {
                not_before: layerx_agent_api::TimestampSeconds(number(prep, "not_before")),
                not_after: layerx_agent_api::TimestampSeconds(number(prep, "not_after")),
            }
            .validate(),
        ),
        idempotency_key: checked(IdempotencyRef::new(layerx_human_service::hex::lower(
            (action).as_ref(),
        ))),
        fee_limit: layerx_agent_api::Amount(checked(serde_json::from_value(
            prep["fee_limit"].clone(),
        ))),
        payload: checked(PayloadBytes::new(bytes(&leg["payload"]))),
        payload_hash: checked(bytes(&leg["payload_hash"]).try_into()),
    };
    let mut digest = Sha256::new();
    digest.update(b"layerx-human-journey-prepare/v1");
    digest.update(request.protocol_activity_type.to_be_bytes());
    for value in [request.actor.as_str(), request.authority.as_str()] {
        digest.update(checked(u32::try_from(value.len())).to_be_bytes());
        digest.update(value.as_bytes());
    }
    digest.update(request.account_sequence.get().to_be_bytes());
    digest.update(request.timestamp_bound.not_before.get().to_be_bytes());
    digest.update(request.timestamp_bound.not_after.get().to_be_bytes());
    let key = request.idempotency_key.as_str();
    digest.update(checked(u32::try_from(key.len())).to_be_bytes());
    digest.update(key.as_bytes());
    digest.update(request.fee_limit.get().to_be_bytes());
    digest.update(request.payload_hash);
    digest.update(Sha256::digest(request.payload.as_bytes()));
    contract.prepare(IdempotentMutation {
        request_id: RequestId(u64::from_be_bytes(checked(action[..8].try_into()))),
        key: checked(Key::new(action)),
        body_digest: BodyDigest(digest.finalize().into()),
        operation: request,
    })
}
fn signed_purpose(journal: &Value) -> SignedNativeSendPurposeV2 {
    let native = &journal["legs"][0]["native"];
    SignedNativeSendPurposeV2 {
        purpose: checked(NativeSendPurposeV2::from_canonical_bytes(&bytes(
            &native["purpose"],
        ))),
        signature: checked(bytes(&native["purpose_signature"]).try_into()),
    }
}
fn no_money(fixture: &Value, journal: &Value) {
    let leg = &journal["legs"][0];
    assert!(
        leg["submission_ref"].is_null()
            && leg["activity_id"].is_null()
            && leg["receipt"].is_null()
            && leg["receipt_digest"].is_null()
    );
    let store = checked(layerx_agentd::store::Store::open(text(
        fixture,
        "agent_store",
    )));
    let tenant = checked(layerx_agentd::store::TenantId::new(text(
        fixture,
        "agent_tenant",
    )));
    let key = checked(layerx_agentd::store::TenantKey::new(
        tenant,
        layerx_agentd::store::ObjectKind::Outbox,
        bytes(&leg["action_key"]),
    ));
    assert!(store.get(&key).is_none());
}
fn activity_signatures(scope: &PrincipalScope<'_>, journal: &Value) -> usize {
    let digest: [u8; 32] =
        checked(bytes(&journal["legs"][0]["native"]["disclosure_digest"]).try_into());
    let chain = checked(AuditChain::open(scope));
    checked(chain.entries(scope)).iter().filter(|entry| matches!(entry.event(), AuditEvent::SigningDecision { disclosure_digest, outcome: Decision::Granted, .. } if *disclosure_digest == digest)).count()
}
fn public_request(fixture: &Value, path: &str, request: &Value) -> (u16, Value) {
    assert!(matches!(
        path,
        "/v1/native/send/access/begin" | "/v1/native/send/access/confirm"
    ));
    let host = text(fixture, "human_tls_host");
    assert!(!host.is_empty() && !host.contains(['\r', '\n', '/', ':']));
    let connector = checked(
        TlsConnector::builder()
            .add_root_certificate(checked(Certificate::from_der(&protected(
                text(fixture, "human_tls_ca_der"),
                65_536,
            ))))
            .build(),
    );
    let tcp = checked(TcpStream::connect_timeout(
        &checked(text(fixture, "human_tls_address").parse()),
        Duration::from_secs(8),
    ));
    checked(tcp.set_read_timeout(Some(Duration::from_secs(8))));
    checked(tcp.set_write_timeout(Some(Duration::from_secs(8))));
    let mut tls = checked(connector.connect(host, tcp));
    let body = checked(serde_json::to_vec(&request["body"]));
    assert!(body.len() <= 1_048_576);
    let mut header = format!("POST {path} HTTP/1.1\r\nHost: {host}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n", body.len());
    let headers = request["headers"]
        .as_object()
        .or_fail("protected genuine browser session headers");
    assert!(headers.keys().any(|key| key.eq_ignore_ascii_case("cookie")));
    for (name, value) in headers {
        assert!([
            "authorization",
            "cookie",
            "origin",
            "idempotency-key",
            "x-layerx-csrf",
            "x-layerx-trace"
        ]
        .iter()
        .any(|allowed| name.eq_ignore_ascii_case(allowed)));
        let value = value.as_str().or_fail("protected session header value");
        assert!(!value.contains(['\r', '\n']));
        header.push_str(name);
        header.push_str(": ");
        header.push_str(value);
        header.push_str("\r\n");
    }
    header.push_str("\r\n");
    checked(tls.write_all(header.as_bytes()));
    checked(tls.write_all(&body));
    let mut response = Vec::new();
    checked(tls.take(1_212_417).read_to_end(&mut response));
    assert!(response.len() <= 1_212_416);
    let end = response
        .windows(4)
        .position(|bytes| bytes == b"\r\n\r\n")
        .or_fail("genuine HTTP framing")
        + 4;
    let head = checked(std::str::from_utf8(&response[..end]));
    let status = checked(
        head.lines()
            .next()
            .or_fail("HTTP status")
            .split(' ')
            .nth(1)
            .or_fail("HTTP numeric status")
            .parse(),
    );
    let length: usize = head
        .lines()
        .find_map(|line| {
            line.split_once(':')
                .filter(|(name, _)| name.eq_ignore_ascii_case("content-length"))
                .map(|(_, value)| checked(value.trim().parse()))
        })
        .or_fail("genuine bounded HTTP body length");
    assert_eq!(length, response.len() - end);
    (status, checked(serde_json::from_slice(&response[end..])))
}
fn refusal_key(request: &mut Value, key: &str) {
    id(key);
    let headers = request["headers"]
        .as_object_mut()
        .or_fail("genuine browser header object");
    let previous = headers
        .keys()
        .find(|name| name.eq_ignore_ascii_case("idempotency-key"))
        .cloned()
        .or_fail("genuine mutation idempotency header");
    assert!(headers[&previous]
        .as_str()
        .is_some_and(|original| original != key));
    headers.insert(previous, Value::String(key.to_owned()));
}
fn public_access(fixture: &Value, case: &Value) {
    let begin: Value = checked(serde_json::from_slice(&protected(
        text(case, "access_begin_request_file"),
        65_536,
    )));
    let confirm: Value = checked(serde_json::from_slice(&protected(
        text(case, "access_confirm_request_file"),
        65_536,
    )));
    assert_eq!(begin["body"]["access_id"], case["capability_id"]);
    let mut immutable = confirm["body"].clone();
    assert!(immutable
        .as_object_mut()
        .or_fail("genuine full consent request")
        .remove("step_up")
        .is_some());
    assert!(immutable == begin["body"]);
    let (status, disclosure) = public_request(fixture, "/v1/native/send/access/begin", &begin);
    assert_eq!(status, 200);
    assert_eq!(disclosure["ok"], true);
    assert_eq!(disclosure["result"]["access_id"], case["capability_id"]);
    assert!(disclosure["result"]["confirms"]
        .as_str()
        .is_some_and(|digest| digest.starts_with("opd_")));
    let (status, repeated) = public_request(fixture, "/v1/native/send/access/begin", &begin);
    assert_eq!(status, 200);
    assert_eq!(repeated["result"], disclosure["result"]);
    let mut changed = begin.clone();
    let maximum: u128 = checked(text(&begin["body"], "maximum_amount").parse());
    changed["body"]["maximum_amount"] = Value::String(
        maximum
            .checked_add(1)
            .unwrap_or_else(|| {
                panic!(
                    "{}",
                    "genuine fixture amount permits bounded refusal mutation"
                )
            })
            .to_string(),
    );
    refusal_key(&mut changed, text(case, "begin_refusal_key"));
    let (status, refused) = public_request(fixture, "/v1/native/send/access/begin", &changed);
    assert_eq!(status, 403);
    assert_eq!(refused["ok"], false);
    assert_eq!(refused["error"]["code"], "forbidden");
    let mut changed = confirm.clone();
    assert_ne!(
        changed["body"]["commitment"].as_str(),
        Some(text(fixture, "foreign_commitment"))
    );
    id(text(fixture, "foreign_commitment"));
    changed["body"]["commitment"] = Value::String(text(fixture, "foreign_commitment").to_owned());
    refusal_key(&mut changed, text(case, "confirm_refusal_key"));
    let (status, refused) = public_request(fixture, "/v1/native/send/access/confirm", &changed);
    assert_eq!(status, 403);
    assert_eq!(refused["ok"], false);
    assert_eq!(refused["error"]["code"], "forbidden");
    let mut retained = store(fixture);
    let scope = checked(retained.principal(&checked(PrincipalId::new(text(fixture, "principal")))));
    let disclosure_key = checked(RowKey::new(format!(
        "native-send-public-disclosure-{}",
        text(case, "capability_id")
    )));
    let stored: Value = checked(serde_json::from_slice(
        scope
            .get(Table::Journeys, &disclosure_key)
            .or_fail("actual immutable public consent disclosure")
            .bytes(),
    ));
    assert!(stored["body"] == begin["body"]);
    assert_eq!(stored["owner"], fixture["owner"]);
    assert!(stored["issued_at_ms"].as_u64().is_some_and(|time| time > 0));
    drop(scope);
    drop(retained);
    let (status, accepted) = public_request(fixture, "/v1/native/send/access/confirm", &confirm);
    assert_eq!(status, 200);
    assert_eq!(accepted["ok"], true);
    assert_eq!(
        accepted["result"]["native_access_id"],
        case["capability_id"]
    );
}

struct NativeCaseContext<'a> {
    fixture: &'a Value,
    case: &'a Value,
    index: usize,
    agent: &'a mut AgentRuntime,
    registry: &'a layerx_types::payload::ModuleRegistry,
    contract: &'a AgentClient,
    principal: &'a PrincipalId,
    trace: &'a TraceId,
}

fn assert_durable_approval(
    fixture: &Value,
    agent: &mut AgentRuntime,
    original: &Value,
    approval: [u8; 32],
    digest: [u8; 32],
) -> (
    layerx_agentd::store::TenantKey,
    layerx_human_service::server::agent_runtime::NativeEffectApprovalMaterial,
) {
    let daemon = checked(layerx_agentd::store::Store::open(text(
        fixture,
        "agent_store",
    )));
    let tenant = checked(layerx_agentd::store::TenantId::new(text(
        fixture,
        "agent_tenant",
    )));
    let durable_key = checked(layerx_agentd::prepare::DurablePreparation::store_key(
        &tenant, approval,
    ));
    let retained = daemon
        .get(&durable_key)
        .or_fail("actual durable held approval")
        .bytes()
        .to_vec();
    drop(daemon);
    let daemon = checked(layerx_agentd::store::Store::open(text(
        fixture,
        "agent_store",
    )));
    assert_eq!(
        daemon
            .get(&durable_key)
            .or_fail("held approval after reopening store")
            .bytes(),
        retained
    );
    let durable = checked(layerx_agentd::prepare::DurablePreparation::decode(
        tenant, &retained,
    ));
    assert_eq!(
        durable.state,
        layerx_agentd::prepare::LifecycleState::Prepared
    );
    assert!(durable
        .extensions
        .get(&6)
        .is_some_and(|bytes| !bytes.is_empty()));
    let material = checked(agent.native_effect_approval_material(approval, digest));
    assert_eq!(
        material.canonical_unsigned_bytes,
        bytes(&original["legs"][0]["native"]["unsigned_canonical_bytes"])
    );
    assert_eq!(
        <[u8; 32]>::from(Sha256::digest(&material.canonical_unsigned_bytes)),
        approval
    );
    (durable_key, material)
}

struct HeldPurposeChecks<'a, 'scope> {
    fixture: &'a Value,
    case: &'a Value,
    principal: &'a PrincipalId,
    agent: &'a mut AgentRuntime,
    signer: &'a CustodySigner,
    contract: &'a AgentClient,
    scope: &'a PrincipalScope<'scope>,
    original: &'a Value,
    approval: [u8; 32],
    digest: [u8; 32],
}

fn assert_held_purpose(checks: HeldPurposeChecks<'_, '_>) -> SignedNativeSendPurposeV2 {
    let HeldPurposeChecks {
        fixture,
        case,
        principal,
        agent,
        signer,
        contract,
        scope,
        original,
        approval,
        digest,
    } = checks;
    let call = original_call(contract, original);
    let purpose = signed_purpose(original);
    assert_eq!(purpose.purpose.transaction_type, 2);
    assert_eq!(purpose.purpose.chain_id, number(fixture, "network_id"));
    assert_eq!(purpose.purpose.activity.module, 1);
    assert_eq!(purpose.purpose.activity.ordinal, 5);
    assert_eq!(purpose.purpose.owner_did.as_str(), text(fixture, "owner"));
    let held_key = checked(signer.describe_key(
        principal,
        &checked(KeyId::new(text(fixture, "custody_key"))),
    ));
    assert_eq!(purpose.purpose.consent_public_key, held_key.public_key);
    assert_eq!(
        purpose.purpose.evm_sender,
        checked(signer.evm_wallet(
            principal,
            &checked(KeyId::new(text(fixture, "custody_key")))
        ))
    );
    checked(layerx_crypto::ed25519::verify_digest(
        &held_key.public_key,
        &purpose.signature,
        &Sha256::digest(checked(purpose.purpose.canonical_bytes())).into(),
    ));
    let original_action: [u8; 32] = checked(bytes(&original["legs"][0]["action_key"]).try_into());
    assert_eq!(
        purpose.purpose.economic_action,
        id(text(case, "capability_id"))
    );
    assert_eq!(purpose.purpose.idempotency_key, original_action);
    assert_ne!(
        purpose.purpose.economic_action,
        purpose.purpose.idempotency_key
    );
    let mut wrong_session = purpose.clone();
    wrong_session.purpose.session_id = checked(SessionId::new(text(fixture, "foreign_session_id")));
    assert_ne!(wrong_session.purpose.session_id, purpose.purpose.session_id);
    assert!(agent
        .native_journey_resume(&call, &wrong_session, approval, digest)
        .is_err());
    let mut wrong_consent_key = purpose.clone();
    wrong_consent_key.purpose.consent_public_key[0] ^= 1;
    assert!(agent
        .native_journey_resume(&call, &wrong_consent_key, approval, digest)
        .is_err());
    let mut wrong_sender = purpose.clone();
    wrong_sender.purpose.evm_sender[0] ^= 1;
    assert!(agent
        .native_journey_resume(&call, &wrong_sender, approval, digest)
        .is_err());
    let mut wrong_context = purpose.clone();
    wrong_context.purpose.commitment[0] ^= 1;
    assert!(agent
        .native_journey_resume(&call, &wrong_context, approval, digest)
        .is_err());
    let mut wrong_digest = digest;
    wrong_digest[0] ^= 1;
    assert!(agent
        .native_effect_approval_decide(
            approval,
            wrong_digest,
            text(case, "decision_key"),
            true,
            clock(fixture).height
        )
        .is_err());
    let mut foreign = runtime(
        fixture,
        text(fixture, "foreign_principal"),
        text(fixture, "foreign_owner"),
        text(fixture, "foreign_account"),
    );
    assert!(foreign.native_effect_approval_get_facts(approval).is_err());
    assert!(foreign
        .native_journey_resume(&call, &purpose, approval, digest)
        .is_err());
    assert_eq!(journal(scope, case), *original);
    purpose
}

struct StartNativeApproval<'a, 'scope> {
    fixture: &'a Value,
    case: &'a Value,
    agent: &'a mut AgentRuntime,
    registry: &'a layerx_types::payload::ModuleRegistry,
    contract: &'a AgentClient,
    scope: &'a mut PrincipalScope<'scope>,
    plan: &'a JourneyPlan,
    signer: &'a CustodySigner,
    trace: &'a TraceId,
}

fn start_native_approval(context: StartNativeApproval<'_, '_>) -> (Value, [u8; 32], [u8; 32]) {
    let StartNativeApproval {
        fixture,
        case,
        agent,
        registry,
        contract,
        scope,
        plan,
        signer,
        trace,
    } = context;
    let intent: Value = checked(serde_json::from_slice(&protected(
        text(case, "intent_file"),
        65_536,
    )));
    let context = checked(layerx_human_service::server::native_send::load_access(
        scope,
        &mut *agent,
        id(text(case, "capability_id")),
        clock(fixture).seconds,
    ));
    checked(
        agent.bind_native_journey_context(
            context,
            number(&intent, "not_after")
                .checked_mul(1000)
                .or_fail("protocol expiry bound"),
        ),
    );
    let mut engine = checked(JourneyEngine::start_native(
        &mut *scope,
        plan,
        registry,
        clock(fixture).seconds,
    ));
    for _ in 0..3 {
        if checked(engine.status()).phases() == [JourneyPhase::AwaitingApproval] {
            break;
        }
        checked(ready(engine.advance(
            &mut *scope,
            contract,
            &mut *agent,
            signer,
            registry,
            trace,
            clock(fixture).seconds,
        )));
    }
    assert_eq!(
        checked(engine.status()).phases(),
        [JourneyPhase::AwaitingApproval]
    );
    let original = journal(scope, case);
    assert!(original["legs"][0]["signed"].is_null());
    no_money(fixture, &original);
    assert_eq!(activity_signatures(scope, &original), 0);
    let (approval, digest) = engine
        .native_approval_binding()
        .or_fail("actual held native approval");
    (original, approval, digest)
}

struct NativeResumeContext<'a, 'scope> {
    fixture: &'a Value,
    case: &'a Value,
    index: usize,
    agent: &'a mut AgentRuntime,
    engine: &'a mut JourneyEngine,
    scope: &'a mut PrincipalScope<'scope>,
    signer: &'a CustodySigner,
    registry: &'a layerx_types::payload::ModuleRegistry,
    contract: &'a AgentClient,
    trace: &'a TraceId,
    original: &'a Value,
    purpose: &'a SignedNativeSendPurposeV2,
    approval: [u8; 32],
    digest: [u8; 32],
}

fn assert_decision_resume(context: &mut NativeResumeContext<'_, '_>) {
    let fixture = context.fixture;
    let case = context.case;
    let index = context.index;
    let agent = &mut *context.agent;
    let engine = &mut *context.engine;
    let scope = &mut *context.scope;
    let signer = context.signer;
    let registry = context.registry;
    let contract = context.contract;
    let trace = context.trace;
    let original = context.original;
    let purpose = context.purpose;
    let approval = context.approval;
    let digest = context.digest;
    if index == 2 {
        let expires = purpose.purpose.expires_at_ms;
        let runtime_clock = checked(RuntimeClock::from_environment());
        let mut deadline = checked(Deadline::start(
            runtime_clock.as_ref(),
            Duration::from_secs(45),
        ));
        while clock(fixture).milliseconds < expires {
            assert!(
                !checked(deadline.remaining(runtime_clock.as_ref())).is_zero(),
                "genuine fixture expiry must be within bounded window"
            );
            std::thread::sleep(Duration::from_millis(100));
        }
    }
    let decided = checked(agent.native_effect_approval_decide(
        approval,
        digest,
        text(case, "decision_key"),
        index != 1,
        clock(fixture).height,
    ));
    assert_eq!(
        decided.state,
        match index {
            0 => NativeEffectApprovalFactState::Granted,
            1 => NativeEffectApprovalFactState::Rejected,
            _ => NativeEffectApprovalFactState::Expired,
        }
    );
    assert_eq!(decided.approval_id, approval);
    assert_eq!(decided.held_digest, digest);
    checked(ready(engine.advance(
        &mut *scope,
        contract,
        &mut *agent,
        signer,
        registry,
        trace,
        clock(fixture).seconds,
    )));
    assert_eq!(
        checked(engine.status()).phases(),
        [if index == 0 {
            JourneyPhase::Prepared
        } else {
            JourneyPhase::Refused
        }]
    );
    let resumed = journal(scope, case);
    assert!(resumed["legs"][0]["signed"].is_null());
    assert_eq!(activity_signatures(scope, &resumed), 0);
    for field in ["idempotency_key", "plan_digest", "custody_key"] {
        assert_eq!(resumed[field], original[field]);
    }
    for field in ["action_key", "payload", "payload_hash", "preparation"] {
        assert_eq!(resumed["legs"][0][field], original["legs"][0][field]);
    }
    for field in [
        "purpose",
        "purpose_signature",
        "owner_public_key",
        "owner_key",
        "unsigned_canonical_bytes",
        "signing_preimage",
        "disclosure_digest",
        "created_head_sequence",
        "created_protocol_time_ms",
        "approval_id",
        "held_digest",
    ] {
        assert_eq!(
            resumed["legs"][0]["native"][field],
            original["legs"][0]["native"][field]
        );
    }
    if index == 0 {
        assert_eq!(
            bytes(&resumed["legs"][0]["native"]["release_ref"]),
            decided.release_ref.or_fail("genuine approval permission")
        );
    }
    no_money(fixture, &resumed);
}

fn assert_signed_release(
    context: &mut NativeResumeContext<'_, '_>,
    material: &layerx_human_service::server::agent_runtime::NativeEffectApprovalMaterial,
) {
    let fixture = context.fixture;
    let case = context.case;
    let index = context.index;
    let agent = &mut *context.agent;
    let engine = &mut *context.engine;
    let scope = &mut *context.scope;
    let signer = context.signer;
    let registry = context.registry;
    let contract = context.contract;
    let trace = context.trace;
    let purpose = context.purpose;
    if index == 0 {
        checked(ready(engine.advance(
            &mut *scope,
            contract,
            &mut *agent,
            signer,
            registry,
            trace,
            clock(fixture).seconds,
        )));
        assert_eq!(checked(engine.status()).phases(), [JourneyPhase::Signed]);
        let journal_entry = journal(scope, case);
        assert!(journal_entry["legs"][0]["signed"].is_object());
        assert_eq!(activity_signatures(scope, &journal_entry), 1);
        checked(layerx_human_kms::attestor::validate_native_send_release(
            &checked(purpose.canonical_bytes()),
            &purpose.purpose.owner_did,
            &purpose.purpose.consent_public_key,
            &purpose.purpose.evm_sender,
            &bytes(&journal_entry["legs"][0]["signed"]["raw_transaction"]),
        ));
        assert_eq!(
            bytes(&journal_entry["legs"][0]["signed"]["canonical_digest"]),
            Sha256::digest(&material.canonical_unsigned_bytes).to_vec()
        );
        no_money(fixture, &journal_entry);
    } else {
        assert_eq!(checked(engine.status()).state(), JourneyState::Refused);
    }
}

struct ReopenedNative<'a, 'scope> {
    fixture: &'a Value,
    case: &'a Value,
    agent: &'a mut AgentRuntime,
    engine: &'a JourneyEngine,
    scope: &'a mut PrincipalScope<'scope>,
    plan: &'a JourneyPlan,
    registry: &'a layerx_types::payload::ModuleRegistry,
    purpose: &'a SignedNativeSendPurposeV2,
    approval: [u8; 32],
    digest: [u8; 32],
}

fn assert_reopened_context(context: ReopenedNative<'_, '_>) {
    let ReopenedNative {
        fixture,
        case,
        agent,
        engine,
        scope,
        plan,
        registry,
        purpose,
        approval,
        digest,
    } = context;
    assert_eq!(
        checked(engine.native_context_binding()),
        Some(purpose.purpose.clone())
    );
    let context = checked(layerx_human_service::server::native_send::load_access(
        scope,
        &mut *agent,
        id(text(case, "capability_id")),
        clock(fixture).seconds,
    ));
    checked(
        context
            .session
            .validate(scope, context.session.expires_at_ms / 1_000 - 1),
    );
    assert!(context
        .session
        .validate(scope, context.session.expires_at_ms.div_ceil(1_000))
        .is_err());
    checked(agent.bind_native_journey_context(context, purpose.purpose.expires_at_ms));
    assert_eq!(
        checked(JourneyEngine::start_native(
            &mut *scope,
            plan,
            registry,
            clock(fixture).seconds
        ))
        .native_approval_binding(),
        Some((approval, digest))
    );
}

fn assert_durable_decision(
    fixture: &Value,
    case: &Value,
    index: usize,
    durable_key: &layerx_agentd::store::TenantKey,
    engine: &JourneyEngine,
    scope: &PrincipalScope<'_>,
) -> Value {
    let daemon = checked(layerx_agentd::store::Store::open(text(
        fixture,
        "agent_store",
    )));
    let tenant = checked(layerx_agentd::store::TenantId::new(text(
        fixture,
        "agent_tenant",
    )));
    let durable = checked(layerx_agentd::prepare::DurablePreparation::decode(
        tenant,
        daemon
            .get(durable_key)
            .or_fail("actual retained decision")
            .bytes(),
    ));
    assert_eq!(
        durable.state,
        match index {
            0 => layerx_agentd::prepare::LifecycleState::Prepared,
            1 => layerx_agentd::prepare::LifecycleState::Failed,
            _ => layerx_agentd::prepare::LifecycleState::Expired,
        }
    );
    assert_ne!(checked(engine.status()).state(), JourneyState::Done);
    journal(scope, case)
}

fn assert_native_case(context: NativeCaseContext<'_>) {
    let NativeCaseContext {
        fixture,
        case,
        index,
        agent,
        registry,
        contract,
        principal,
        trace,
    } = context;
    public_access(fixture, case);
    let plan = plan(fixture, case);
    let mut retained = store(fixture);
    let mut scope = checked(retained.principal(principal));
    let signer = custody(fixture, registry);
    let (original, approval, digest) = start_native_approval(StartNativeApproval {
        fixture,
        case,
        agent: &mut *agent,
        registry,
        contract,
        scope: &mut scope,
        plan: &plan,
        signer: &signer,
        trace,
    });
    let (durable_key, material) =
        assert_durable_approval(fixture, &mut *agent, &original, approval, digest);
    let purpose = assert_held_purpose(HeldPurposeChecks {
        fixture,
        case,
        principal,
        agent: &mut *agent,
        signer: &signer,
        contract,
        scope: &scope,
        original: &original,
        approval,
        digest,
    });
    drop(scope);
    drop(retained);
    drop(signer);
    let mut retained = store(fixture);
    let mut scope = checked(retained.principal(principal));
    let mut engine = checked(JourneyEngine::load(
        &scope,
        &checked(JourneyId::new(text(case, "journey_id"))),
    ))
    .or_fail("genuine native reopen");
    assert_eq!(journal(&scope, case), original);
    assert_reopened_context(ReopenedNative {
        fixture,
        case,
        agent: &mut *agent,
        engine: &engine,
        scope: &mut scope,
        plan: &plan,
        registry,
        purpose: &purpose,
        approval,
        digest,
    });
    let signer = custody(fixture, registry);
    let mut resume = NativeResumeContext {
        fixture,
        case,
        index,
        agent: &mut *agent,
        engine: &mut engine,
        scope: &mut scope,
        signer: &signer,
        registry,
        contract,
        trace,
        original: &original,
        purpose: &purpose,
        approval,
        digest,
    };
    assert_decision_resume(&mut resume);
    assert_signed_release(&mut resume, &material);
    let before_reopen =
        assert_durable_decision(fixture, case, index, &durable_key, &engine, &scope);
    drop(scope);
    drop(retained);
    drop(signer);
    let mut retained = store(fixture);
    let scope = checked(retained.principal(principal));
    let reopened = checked(JourneyEngine::load(
        &scope,
        &checked(JourneyId::new(text(case, "journey_id"))),
    ))
    .or_fail("genuine decided native reopen");
    assert_eq!(journal(&scope, case), before_reopen);
    assert_eq!(reopened.native_approval_binding(), Some((approval, digest)));
    no_money(fixture, &before_reopen);
}

#[test]
fn genuine_native_journey_holds_reopens_and_resumes_original_authority() {
    let path = std::env::var("PAXEER_X_NATIVE_JOURNEY_APPROVAL_FIXTURE").unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "protected genuine native Journey authority fixture required"
        )
    });
    let fixture: Value = checked(serde_json::from_slice(&protected(&path, 65_536)));
    assert_eq!(fixture["schema"], "paxeer-x.native-journey-approval.v2");
    assert_eq!(fixture["isolated_real_owner"], true);
    let principal = checked(PrincipalId::new(text(&fixture, "principal")));
    let mut agent = runtime(
        &fixture,
        principal.as_str(),
        text(&fixture, "owner"),
        text(&fixture, "account"),
    );
    let registry = agent.registry().clone();
    let contract = checked(AgentClient::daemon(
        text(&fixture, "agent_socket"),
        layerx_agent_api::agent_api_schema_v1().version,
    ));
    let trace = checked(TraceId::parse(text(&fixture, "trace_id")));
    let cases = fixture["cases"]
        .as_array()
        .or_fail("grant rejection expiry cases");
    assert_eq!(cases.len(), 3);
    for (index, case) in cases.iter().enumerate() {
        assert_native_case(NativeCaseContext {
            fixture: &fixture,
            case,
            index,
            agent: &mut agent,
            registry: &registry,
            contract: &contract,
            principal: &principal,
            trace: &trace,
        });
    }
}

struct RestartPlan {
    key: ed25519_dalek::SigningKey,
    public: [u8; 32],
    owner: AgentDid,
    debit: layerx_crypto::send::SendDebit,
    authorization_bytes: [u8; 64],
    plan: JourneyPlan,
}

fn restart_plan(journey_id: &JourneyId, request_key: [u8; 32]) -> RestartPlan {
    use layerx_crypto::send::SendDebit;
    let key = ed25519_dalek::SigningKey::from_bytes(&[0x31; 32]);
    let public = key.verifying_key().to_bytes();
    let owner = checked(AgentDid::new(format!(
        "did:layerx:{}",
        layerx_human_service::hex::lower((public).as_ref())
    )));
    let from = checked(AccountId::parse(&format!("agent:{}:main", owner.as_str())));
    let to = checked(AccountId::parse(&format!(
        "agent:did:layerx:{}:main",
        "42".repeat(32)
    )));
    let debit = SendDebit {
        from: checked(layerx_intents::canonical::account_id_for_protocol(&from, 3)),
        to: checked(layerx_intents::canonical::account_id_for_protocol(&to, 3)),
        asset: [0x43; 32],
        amount: 5,
        source_sequence: 7,
        idempotency_key: [0x44; 32],
        expires_at: 1_010,
        context_hash: [0x45; 32],
        conditions: Vec::new(),
        authorization_kind: SendAuthorizationKind::Owner as u8,
        network_id: 125,
        protocol_version: 3,
    };
    let local = layerx_crypto::local::LocalSigner::new([0x31; 32]);
    let authorization_bytes = checked(layerx_intents::canonical::send_authorization_bytes(&debit));
    let authorization = checked(ready(layerx_crypto::signer::Signer::sign(
        &local,
        checked(debit.signing_request(&authorization_bytes)),
    )));
    let authorization_bytes = *authorization.as_bytes();
    let send = checked(LxpSend::new(
        from,
        to,
        AssetId::new(debit.asset),
        Amount::from_u128(debit.amount),
        Sequence::from_u64(debit.source_sequence),
        IdempotencyKey::new(debit.idempotency_key),
        TimestampSeconds::from_u64(debit.expires_at),
        ContextHash::new(debit.context_hash),
        SendAuthorization::new(
            SendAuthorizationKind::Owner,
            PublicKey::new(public),
            AuthorizationSignature::new(authorization_bytes),
        ),
        checked(NetworkId::new(125)),
        checked(ProtocolVersion::new(3)),
    ));
    let plan = checked(JourneyPlan::new(
        journey_id.clone(),
        JourneyKind::Move,
        request_key,
        checked(KeyId::new("primary")),
        Operation::ProtocolMutation,
        vec![checked(JourneyLeg::new(
            Intent::v1(IntentKind::LxpSend(send)),
            debit.idempotency_key,
            owner.clone(),
            checked(AuthorityRef::new(layerx_human_service::hex::lower(
                (public).as_ref(),
            ))),
            7,
            7,
            1_000,
            1_010,
            700_000,
        ))],
    ));
    RestartPlan {
        key,
        public,
        owner,
        debit,
        authorization_bytes,
        plan,
    }
}

fn signed_restart_purpose(
    payload: &[u8],
    owner: AgentDid,
    public: [u8; 32],
    debit: &layerx_crypto::send::SendDebit,
    key: &ed25519_dalek::SigningKey,
) -> (SignedNativeSendPurposeV2, Value, [u8; 32]) {
    use ed25519_dalek::Signer as _;
    use layerx_agent_api::identity::{CapabilityId, NativeActivity, TenantId};
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
    use layerx_crypto::send::{ledger_payment_call, CallOptions};
    let wallet = [0x35; 32];
    let probe = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 7,
        gas_limit: 100_000,
        to: [0x33; 20],
        data: vec![0x44],
    };
    let (call, signature) = checked(decode_signed(&checked(probe.sign(&wallet))));
    let sender = checked(recover_sender(&call, &signature));
    let prepared = checked(ledger_payment_call(
        payload,
        &CallOptions {
            from: sender,
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 7,
            gas_limit: 100_000,
        },
    ));
    let canonical = prepared.call.encode_unsigned();
    let digest: [u8; 32] = Sha256::digest(&canonical).into();
    let purpose = NativeSendPurposeV2 {
        tenant: checked(TenantId::new("tenant-a")),
        agent_did: owner.clone(),
        owner_did: owner,
        consent_public_key: public,
        evm_sender: sender,
        session_id: checked(SessionId::new("51".repeat(32))),
        generation: 2,
        expires_at_ms: 1_010_000,
        capability_id: checked(CapabilityId::new("52".repeat(32))),
        transaction_type: 2,
        chain_id: 125,
        activity: checked(NativeActivity::new(1, 5)),
        preparation_id: digest,
        canonical_digest: digest,
        economic_action: [0x53; 32],
        idempotency_key: debit.idempotency_key,
        commitment: [0x54; 32],
    };
    checked(
        layerx_human_service::custody::NativeConsent::SendPurpose(&purpose)
            .validate_at(1_000_000, public),
    );
    let signed = SignedNativeSendPurposeV2 {
        purpose: purpose.clone(),
        signature: key
            .sign(&checked(
                layerx_human_service::custody::NativeConsent::SendPurpose(&purpose).digest(),
            ))
            .to_bytes(),
    };
    let native = serde_json::json!({
        "purpose": checked(purpose.canonical_bytes()),
        "owner_public_key": public, "purpose_signature": signed.signature.to_vec(),
        "unsigned_canonical_bytes": canonical, "signing_preimage": prepared.call.signing_hash().to_vec(),
        "disclosure_digest": checked(prepared.disclosure.audit_digest()), "owner_key": public,
        "created_head_sequence": 7, "created_protocol_time_ms": 1_000_000,
        "approval_id": digest, "held_digest": ([0x55_u8; 32]), "release_ref": null,
        "approval_state": "awaiting"
    });
    (signed, native, digest)
}

fn assert_reopened_purpose(
    reopened: &mut PrincipalStore,
    principal: &PrincipalId,
    journey_id: &JourneyId,
    row: &RowKey,
    expected: &SignedNativeSendPurposeV2,
    original: &Value,
) {
    {
        let mut scope = checked(reopened.principal(principal));
        let engine = checked(JourneyEngine::load(&scope, journey_id))
            .or_fail("revalidated V2 journey after restart");
        assert_eq!(
            checked(engine.status()).phases(),
            [JourneyPhase::AwaitingApproval]
        );
        assert_eq!(
            checked(engine.native_context_binding()),
            Some(expected.purpose.clone())
        );
        let persisted: Value = checked(serde_json::from_slice(
            scope
                .get(Table::Journeys, row)
                .or_fail("reopened record")
                .bytes(),
        ));
        assert_eq!(persisted, *original);
        assert_eq!(signed_purpose(&persisted), *expected);
        checked(layerx_crypto::ed25519::verify_digest(
            &expected.purpose.consent_public_key,
            &expected.signature,
            &Sha256::digest(checked(expected.purpose.canonical_bytes())).into(),
        ));
        for field in ["sender", "consent_key", "canonical_digest", "signature"] {
            let mut altered = original.clone();
            let mut purpose = expected.purpose.clone();
            match field {
                "sender" => purpose.evm_sender[0] ^= 1,
                "consent_key" => purpose.consent_public_key[0] ^= 1,
                "canonical_digest" => purpose.canonical_digest[0] ^= 1,
                _ => {
                    let mut signature = expected.signature;
                    signature[0] ^= 1;
                    altered["legs"][0]["native"]["purpose_signature"] =
                        checked(serde_json::to_value(signature.to_vec()));
                }
            }
            altered["legs"][0]["native"]["purpose"] =
                checked(serde_json::to_value(checked(purpose.canonical_bytes())));
            checked(scope.put(
                Table::Journeys,
                row.clone(),
                1_000,
                checked(serde_json::to_vec(&altered)),
            ));
            assert!(
                JourneyEngine::load(&scope, journey_id).is_err(),
                "reopen accepted changed {field}"
            );
        }
        checked(scope.put(
            Table::Journeys,
            row.clone(),
            1_000,
            checked(serde_json::to_vec(&original)),
        ));
        assert_eq!(
            checked(
                checked(JourneyEngine::load(&scope, journey_id))
                    .or_fail("retained original")
                    .status()
            )
            .phases(),
            [JourneyPhase::AwaitingApproval]
        );
    }
}

#[test]
fn native_send_v2_journey_persisted_before_restart_revalidates_its_purpose() {
    use layerx_human_test_support::{directory, install_and_open, retention_uniform, tenancy};
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    use std::os::unix::fs::DirBuilderExt as _;

    let root = directory("native-send-v2-journey-restart");
    checked(fs::DirBuilder::new().mode(0o700).create(&root));
    assert_eq!(
        checked(fs::metadata(&root)).permissions().mode() & 0o777,
        0o700
    );
    let principal = checked(PrincipalId::new("native-restart-owner"));
    let journey_id = checked(JourneyId::new("jrn_nativev2restart"));
    let request_key = [0x61; 32];
    let row = checked(RowKey::new(format!(
        "journey-{}",
        layerx_human_service::hex::lower((request_key).as_ref())
    )));
    let retention = retention_uniform(10_000);
    let (tenancy_digest, expected, original) = {
        let (mut retained, tenancy_digest) = install_and_open(
            &root.join("store"),
            &tenancy(&[(principal.as_str(), "tenant-a")]),
            retention,
        );
        let RestartPlan {
            key,
            public,
            owner,
            debit,
            authorization_bytes,
            plan,
        } = restart_plan(&journey_id, request_key);
        let activity = checked(ActivityType::new(ModuleId::Asset, 5));
        let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
            ModuleId::Asset,
            &[activity],
        ))]));
        let mut scope = checked(retained.principal(&principal));
        let engine = checked(JourneyEngine::start_native(
            &mut scope, &plan, &registry, 1_000,
        ));
        assert_eq!(checked(engine.status()).phases(), [JourneyPhase::Compiled]);
        let mut record: Value = checked(serde_json::from_slice(
            scope
                .get(Table::Journeys, &row)
                .or_fail("production journey record")
                .bytes(),
        ));
        let payload = bytes(&record["legs"][0]["payload"]);
        assert_eq!(
            payload,
            checked(layerx_intents::canonical::signed_send_payload(
                &debit,
                public,
                authorization_bytes
            ))
        );
        let (signed, native, digest) =
            signed_restart_purpose(&payload, owner, public, &debit, &key);
        record["legs"][0]["native"] = native;
        record["legs"][0]["phase"] = serde_json::json!("awaiting-approval");
        record["legs"][0]["prepared_ref"] = serde_json::json!("native-v2-restart");
        record["legs"][0]["prepared_digest"] = checked(serde_json::to_value(digest));
        record["transitions"]
            .as_array_mut()
            .or_fail("journey transitions")
            .push(serde_json::json!({
                "sequence": 2, "leg": 0, "phase": "awaiting-approval", "observed_at": 1_000
            }));
        checked(scope.put(
            Table::Journeys,
            row.clone(),
            1_000,
            checked(serde_json::to_vec(&record)),
        ));
        assert_eq!(
            checked(JourneyEngine::load(&scope, &journey_id))
                .or_fail("persisted V2 journey")
                .native_approval_binding(),
            Some((digest, [0x55; 32]))
        );
        // All store, scope, engine and signing handles are dropped at the restart boundary.
        (tenancy_digest, signed, record)
    };
    let mut reopened = checked(PrincipalStore::open(
        root.join("store"),
        retention,
        tenancy_digest,
    ));
    assert_reopened_purpose(
        &mut reopened,
        &principal,
        &journey_id,
        &row,
        &expected,
        &original,
    );
    drop(reopened);
    checked(fs::remove_dir_all(root));
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
