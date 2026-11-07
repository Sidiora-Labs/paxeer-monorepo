use layerx_human_test_support::StepClock;
use layerx_types::clock::{Clock, Deadline};
use std::fmt::Debug;
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::identity::{
    AgentDid, AuthorityRef, CapabilityId, NativePreparationPurposeV1, SessionId, TenantId,
};
use layerx_agent_api::prepare::{IdempotencyRef, PayloadBytes, PrepareRequest, TimestampBound};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::send::SendDebit;
use layerx_crypto::SignatureMessage;
use layerx_human_kms::attestor::{
    decode_unsigned_evm_transaction, native_local_grant_signing_bytes,
};
use layerx_human_service::agents::AgentFailure;
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_human_service::server::native_send::{
    access_operation, load_access, preview_send, unsigned_access, HumanOwnerNativeContextV1,
    HumanOwnerNativeSessionV1, NativeSendAccessRequest, NATIVE_SEND,
};
use layerx_human_service::store::{PrincipalScope, PrincipalStore, RowKey, Table};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_types::account::AccountId;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

const OWNER: &str = "did:layerx:nonce-owner";
const PRINCIPAL: &str = "nonce-principal";
const TENANT: &str = "nonce-tenant";
const NONCE: u64 = 19;
const NOW: u64 = 100;
const HEAD: u64 = 88;
const EXPIRY_MS: u64 = 200_000;
const GENERATION: u64 = 3;
const ACTION: [u8; 32] = [0x31; 32];
const GRANT: [u8; 32] = [0x32; 32];
const TOKEN: [u8; 32] = [0x33; 32];
const COMMITMENT: [u8; 32] = [0x34; 32];
const ASSET: [u8; 32] = [0x35; 32];
const MAX_FRAME: usize = 8192;

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("native Send nonce peer: {error:?}"))
}

fn hex(value: [u8; 32]) -> String {
    value.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn owner() -> SigningKey {
    SigningKey::from_bytes(&[0x36; 32])
}

fn bytes(out: &mut Vec<u8>, value: &[u8]) {
    out.extend_from_slice(&checked(u32::try_from(value.len())).to_be_bytes());
    out.extend_from_slice(value);
}

fn frame(opcode: u8, body: &[u8]) -> Vec<u8> {
    let mut out = b"LXHAGT01".to_vec();
    out.push(opcode);
    out.extend_from_slice(body);
    out
}

struct Peer {
    root: PathBuf,
    server: Option<JoinHandle<()>>,
}

impl Peer {
    fn serve(exchanges: Vec<(Vec<u8>, Vec<u8>)>) -> (Self, AgentRuntime) {
        let root = directory("native-send-nonce-peer");
        checked(std::fs::create_dir_all(&root));
        let socket = root.join("peer.sock");
        let listener = checked(UnixListener::bind(&socket));
        checked(listener.set_nonblocking(true));
        let clock: Arc<dyn Clock> = Arc::new(StepClock::new(100000));
        let peer_clock = Arc::clone(&clock);
        let server = thread::spawn(move || {
            for (expected, response) in exchanges {
                let mut deadline =
                    checked(Deadline::start(peer_clock.as_ref(), Duration::from_secs(5)));
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(
                                !checked(deadline.remaining(peer_clock.as_ref())).is_zero(),
                                "peer request deadline"
                            );
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("peer accept: {error}"),
                    }
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
                assert_eq!(checked(read_frame(&mut stream, MAX_FRAME)), expected);
                checked(write_frame(&mut stream, &response, MAX_FRAME));
            }
        });
        let runtime = checked(AgentRuntime::new_with_clock(
            &socket,
            Limits {
                maximum_frame_bytes: MAX_FRAME,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: MAX_FRAME,
                deadline: Duration::from_secs(5),
            },
            checked(ModuleRegistry::new(&[])),
            clock,
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

fn account_id(name: &str) -> [u8; 32] {
    checked(layerx_intents::canonical::account_id_for_protocol(
        &checked(AccountId::parse(name)),
        3,
    ))
}

fn request() -> PrepareRequest {
    let debit = SendDebit {
        from: account_id("agent:did:layerx:nonce-owner:main"),
        to: account_id("agent:did:layerx:nonce-recipient:main"),
        asset: ASSET,
        amount: 37,
        source_sequence: NONCE,
        idempotency_key: ACTION,
        expires_at: EXPIRY_MS / 1000,
        context_hash: COMMITMENT,
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: 125,
        protocol_version: 3,
    };
    let canonical = checked(layerx_intents::canonical::send_authorization_bytes(&debit));
    let digest = checked(SignatureMessage::new(
        layerx_intents::canonical::Domain::SignaturePreimage,
        debit.protocol_version,
        debit.network_id,
        &canonical,
    ))
    .digest();
    let signature = owner().sign(&digest).to_bytes();
    let payload = checked(layerx_intents::canonical::signed_send_payload(
        &debit,
        owner().verifying_key().to_bytes(),
        signature,
    ));
    let activity = checked(ActivityType::new(ModuleId::Asset, 5));
    let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Asset,
        &[activity],
    ))]));
    let payload_hash = checked(layerx_intents::canonical::payload_hash_for(&checked(
        Payload::new(&registry, activity, &payload),
    )));
    PrepareRequest {
        protocol_activity_type: NATIVE_SEND,
        actor: checked(AgentDid::new(OWNER)),
        authority: checked(AuthorityRef::new(hex(owner().verifying_key().to_bytes()))),
        account_sequence: layerx_agent_api::Sequence(NONCE),
        timestamp_bound: checked(
            TimestampBound {
                not_before: layerx_agent_api::TimestampSeconds(NOW * 1000),
                not_after: layerx_agent_api::TimestampSeconds(EXPIRY_MS),
            }
            .validate(),
        ),
        idempotency_key: checked(IdempotencyRef::new(hex(ACTION))),
        fee_limit: layerx_agent_api::Amount(100_000),
        payload: checked(PayloadBytes::new(payload)),
        payload_hash,
    }
}

fn transaction() -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1a]);
    Eip1559Call {
        chain_id: 125,
        nonce: NONCE,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 5,
        gas_limit: 20_000,
        to,
        data: checked(
            layerx_agentd::prepare::PrecompileCall::ledger_transfer(
                request().payload.as_bytes().to_vec(),
            )
            .encode(),
        ),
    }
}

fn owner_exchange() -> (Vec<u8>, Vec<u8>) {
    let public = owner().verifying_key().to_bytes();
    let mut request = Vec::new();
    bytes(&mut request, TENANT.as_bytes());
    request.extend_from_slice(&ACTION);
    request.extend_from_slice(&TOKEN);
    request.extend_from_slice(&GENERATION.to_be_bytes());
    request.extend_from_slice(&public);
    request.extend_from_slice(&u64::from_be_bytes(checked(ACTION[..8].try_into())).to_be_bytes());
    let mut response = Vec::new();
    bytes(&mut response, OWNER.as_bytes());
    response.push(2);
    response.extend_from_slice(&GRANT);
    for coordinate in [GENERATION, HEAD, NOW * 1000, NONCE, 120, EXPIRY_MS] {
        response.extend_from_slice(&coordinate.to_be_bytes());
    }
    response.extend_from_slice(&public);
    response.extend_from_slice(&80_u64.to_be_bytes());
    response.extend_from_slice(&ASSET);
    (frame(64, &request), frame(0, &response))
}

fn nonce_exchange(pending: u64) -> (Vec<u8>, Vec<u8>) {
    let mut request = Vec::new();
    bytes(&mut request, OWNER.as_bytes());
    bytes(
        &mut request,
        hex(owner().verifying_key().to_bytes()).as_bytes(),
    );
    let mut response = pending.to_be_bytes().to_vec();
    response.push(Rung::Committed.wire_rank());
    (frame(13, &request), frame(0, &response))
}

fn retain_access(
    scope: &mut PrincipalScope<'_>,
    runtime: &mut AgentRuntime,
) -> HumanOwnerNativeContextV1 {
    let session: HumanOwnerNativeSessionV1 = checked(serde_json::from_value(serde_json::json!({
        "version": 1,
        "tenant": TENANT,
        "principal": PRINCIPAL,
        "owner": OWNER,
        "owner_public_key": owner().verifying_key().to_bytes(),
        "session_id": ACTION,
        "generation": GENERATION,
        "grant_id": GRANT,
        "grant_receipt_digest": vec![0x41; 32],
        "grant_activity_id": vec![0x42; 32],
        "grant_sequence": 40,
        "expires_at_ms": EXPIRY_MS,
        "token": TOKEN
    })));
    checked(session.validate(scope, NOW));
    let request = NativeSendAccessRequest {
        action_key: ACTION,
        owner: checked(Did::new(OWNER.as_bytes())),
        owner_public_key: session.owner_public_key,
        not_before: NOW,
        expires_at: EXPIRY_MS / 1000,
        native_fee_budget: None,
        counterparty: account_id("agent:did:layerx:nonce-recipient:main"),
        asset: ASSET,
        maximum_amount: 37,
        rate_window_ms: 60_000,
        maximum_uses: 1,
        commitment: COMMITMENT,
    };
    let coordinates = checked(runtime.native_send_owner_context(
        &session,
        u64::from_be_bytes(checked(ACTION[..8].try_into())),
    ));
    assert_eq!(coordinates.account_sequence, NONCE);
    let grant = checked(unsigned_access(&request, &session, &coordinates));
    let digest: [u8; 32] = Sha256::digest(checked(native_local_grant_signing_bytes(&grant))).into();
    let signature = owner().sign(&digest);
    checked(owner().verifying_key().verify_strict(&digest, &signature));
    let context = HumanOwnerNativeContextV1 {
        version: 1,
        session,
        capability_id: request.action_key,
        capability: grant.capability,
        session_scope: grant.session_scope,
        grant_signature: signature.to_bytes().to_vec(),
        expires_at_ms: grant.expires_at_ms,
        consent_request_digest: checked(access_operation(scope, &request)).bytes(),
        commitment: request.commitment,
        nonce: coordinates.account_sequence,
    };
    checked(scope.put(
        Table::Journeys,
        checked(RowKey::new(format!(
            "human-native-send-access-{}",
            hex(ACTION)
        ))),
        NOW,
        checked(serde_json::to_vec(&context)),
    ));
    context
}

fn preview_exchange(context: &HumanOwnerNativeContextV1) -> (Vec<u8>, Vec<u8>) {
    let prepare = request();
    let grant = checked(context.signed_grant());
    let mut request = Vec::new();
    bytes(&mut request, TENANT.as_bytes());
    request.extend_from_slice(&ACTION);
    request.extend_from_slice(&TOKEN);
    request.extend_from_slice(&GENERATION.to_be_bytes());
    request.extend_from_slice(&u64::from_be_bytes(checked(ACTION[..8].try_into())).to_be_bytes());
    request.extend_from_slice(&prepare.protocol_activity_type.to_be_bytes());
    bytes(&mut request, prepare.actor.as_str().as_bytes());
    bytes(&mut request, prepare.authority.as_str().as_bytes());
    for coordinate in [NONCE, NOW * 1000, EXPIRY_MS] {
        request.extend_from_slice(&coordinate.to_be_bytes());
    }
    bytes(&mut request, prepare.idempotency_key.as_str().as_bytes());
    request.extend_from_slice(&prepare.fee_limit.0.to_be_bytes());
    bytes(&mut request, prepare.payload.as_bytes());
    request.extend_from_slice(&prepare.payload_hash);
    request.extend_from_slice(&ACTION);
    request.extend_from_slice(&context.session.owner_public_key);
    request.extend_from_slice(&EXPIRY_MS.to_be_bytes());
    request.extend_from_slice(&COMMITMENT);
    bytes(&mut request, &grant.capability);
    bytes(&mut request, &grant.session_scope);
    request.extend_from_slice(&grant.expires_at_ms.to_be_bytes());
    request.extend_from_slice(&grant.owner_public_key);
    request.extend_from_slice(&grant.signature);
    let call = transaction();
    let canonical = call.encode_unsigned();
    let purpose = NativePreparationPurposeV1 {
        tenant: checked(TenantId::new(TENANT)),
        agent_did: checked(AgentDid::new(OWNER)),
        session_id: checked(SessionId::new(hex(ACTION))),
        generation: GENERATION,
        expires_at_ms: EXPIRY_MS,
        capability_id: checked(CapabilityId::new(hex(ACTION))),
        preparation_id: [0x43; 32],
        canonical_digest: Sha256::digest(&canonical).into(),
        commitment: COMMITMENT,
    };
    let mut response = Vec::new();
    bytes(&mut response, &canonical);
    bytes(&mut response, &call.signing_hash());
    bytes(&mut response, &checked(purpose.canonical_bytes()));
    response.extend_from_slice(&HEAD.to_be_bytes());
    response.extend_from_slice(&(NOW * 1000).to_be_bytes());
    response.extend_from_slice(&context.session.owner_public_key);
    response.extend_from_slice(&80_u64.to_be_bytes());
    (frame(63, &request), frame(0, &response))
}

#[test]
fn native_send_preview_carries_nonce_and_fee_fields() {
    let root = directory("native-send-nonce-store");
    let (mut store, digest) = install_and_open(
        &root,
        &tenancy(&[(PRINCIPAL, TENANT)]),
        retention_uniform(10_000),
    );
    let id = principal(PRINCIPAL);
    let mut scope = checked(store.principal(&id));
    let (owner_peer, mut runtime) = Peer::serve(vec![owner_exchange()]);
    let context = retain_access(&mut scope, &mut runtime);
    owner_peer.finish();
    let (peer, mut runtime) = Peer::serve(vec![
        nonce_exchange(NONCE),
        nonce_exchange(NONCE),
        preview_exchange(&context),
        nonce_exchange(NONCE),
    ]);
    let retained = checked(load_access(&scope, &mut runtime, ACTION, NOW));
    assert_eq!(retained.nonce, NONCE);
    let preview = checked(preview_send(
        &scope,
        &mut runtime,
        &request(),
        ACTION,
        EXPIRY_MS,
        COMMITMENT,
        NOW,
    ));
    let call = checked(decode_unsigned_evm_transaction(&preview.canonical_bytes));
    assert_eq!(call, transaction());
    assert_eq!(preview.nonce, NONCE);
    assert_eq!(preview.nonce, call.nonce);
    assert_eq!(preview.max_fee_per_gas, call.max_fee_per_gas);
    assert_eq!(
        preview.max_priority_fee_per_gas,
        call.max_priority_fee_per_gas
    );
    assert!(preview.max_fee_per_gas > 0);
    assert!(preview.max_priority_fee_per_gas > 0);
    assert!(preview.max_priority_fee_per_gas <= preview.max_fee_per_gas);
    assert_eq!(preview.signing_preimage, call.signing_hash());
    assert_eq!(call.chain_id, 125);
    assert_eq!(preview.purpose.commitment, context.commitment);
    drop(scope);
    drop(store);
    let mut reopened = checked(PrincipalStore::open(
        &root,
        retention_uniform(10_000),
        digest,
    ));
    let scope = checked(reopened.principal(&id));
    let restored = checked(load_access(&scope, &mut runtime, ACTION, NOW));
    assert_eq!(restored.nonce, NONCE);
    assert_eq!(restored.commitment, context.commitment);
    assert_eq!(restored.grant_signature, context.grant_signature);
    assert_eq!(
        restored.consent_request_digest,
        context.consent_request_digest
    );
    drop(scope);
    drop(reopened);
    peer.finish();
    checked(std::fs::remove_dir_all(root));
}

#[test]
fn native_send_access_below_pending_nonce_is_expired() {
    let root = directory("native-send-expired-store");
    let (mut store, digest) = install_and_open(
        &root,
        &tenancy(&[(PRINCIPAL, TENANT)]),
        retention_uniform(10_000),
    );
    let id = principal(PRINCIPAL);
    let mut scope = checked(store.principal(&id));
    let (owner_peer, mut runtime) = Peer::serve(vec![owner_exchange()]);
    let context = retain_access(&mut scope, &mut runtime);
    owner_peer.finish();
    assert_eq!(context.nonce, NONCE);
    let (peer, mut runtime) =
        Peer::serve(vec![nonce_exchange(NONCE + 1), nonce_exchange(NONCE + 1)]);
    assert!(matches!(
        load_access(&scope, &mut runtime, ACTION, NOW),
        Err(AgentFailure::Refused("native Send access expired"))
    ));
    drop(scope);
    drop(store);
    let mut reopened = checked(PrincipalStore::open(
        &root,
        retention_uniform(10_000),
        digest,
    ));
    let scope = checked(reopened.principal(&id));
    assert!(matches!(
        load_access(&scope, &mut runtime, ACTION, NOW),
        Err(AgentFailure::Refused("native Send access expired"))
    ));
    drop(scope);
    drop(reopened);
    peer.finish();
    checked(std::fs::remove_dir_all(root));
}

pub(crate) fn consent_expiry_round_trip(label: &str) {
    let root = directory(label);
    let (mut store, digest) = install_and_open(
        &root,
        &tenancy(&[(PRINCIPAL, TENANT)]),
        retention_uniform(10_000),
    );
    let id = principal(PRINCIPAL);
    let mut scope = checked(store.principal(&id));
    let (owner_peer, mut runtime) = Peer::serve(vec![owner_exchange()]);
    let context = retain_access(&mut scope, &mut runtime);
    owner_peer.finish();
    let original = checked(serde_json::to_vec(&context));
    let mut unsigned = checked(context.signed_grant());
    let signature = ed25519_dalek::Signature::from_bytes(&unsigned.signature);
    unsigned.signature = [0; 64];
    let signed_digest: [u8; 32] =
        Sha256::digest(checked(native_local_grant_signing_bytes(&unsigned))).into();
    checked(
        owner()
            .verifying_key()
            .verify_strict(&signed_digest, &signature),
    );
    // Session access consumes seconds; purpose and grant expiry remain milliseconds.
    assert_eq!(context.expires_at_ms, EXPIRY_MS);
    let before_seconds = EXPIRY_MS / 1_000 - 1;
    let before_milliseconds = before_seconds.checked_mul(1_000).expect("milliseconds");
    assert!(before_milliseconds < EXPIRY_MS);
    let (peer, mut runtime) = Peer::serve(vec![nonce_exchange(NONCE), nonce_exchange(NONCE)]);
    checked(context.session.validate(&scope, before_seconds));
    let loaded = checked(load_access(&scope, &mut runtime, ACTION, before_seconds));
    assert_eq!(checked(serde_json::to_vec(&loaded)), original);
    drop(loaded);
    drop(scope);
    drop(store);
    let mut store = checked(PrincipalStore::open(
        &root,
        retention_uniform(10_000),
        digest,
    ));
    let scope = checked(store.principal(&id));
    let recovered = checked(load_access(&scope, &mut runtime, ACTION, before_seconds));
    assert_eq!(checked(serde_json::to_vec(&recovered)), original);
    for seconds in [EXPIRY_MS.div_ceil(1_000), EXPIRY_MS.div_ceil(1_000) + 1] {
        assert!(matches!(
            load_access(&scope, &mut runtime, ACTION, seconds),
            Err(AgentFailure::Refused("Human owner session unavailable"))
        ));
        assert!(recovered.session.validate(&scope, seconds).is_err());
    }
    assert_eq!(
        scope
            .get(
                Table::Journeys,
                &checked(RowKey::new(format!(
                    "human-native-send-access-{}",
                    hex(ACTION)
                )))
            )
            .expect("immutable consent")
            .bytes(),
        original
    );
    drop(scope);
    drop(store);
    peer.finish();
    checked(std::fs::remove_dir_all(root));
}

pub(crate) fn progressing_head_preserves_exact_action() {
    let root = directory("repair-22-33-63-progressing-head");
    let (mut store, _) = install_and_open(
        &root,
        &tenancy(&[(PRINCIPAL, TENANT)]),
        retention_uniform(10_000),
    );
    let id = principal(PRINCIPAL);
    let mut scope = checked(store.principal(&id));
    let (owner_peer, mut runtime) = Peer::serve(vec![owner_exchange()]);
    let context = retain_access(&mut scope, &mut runtime);
    owner_peer.finish();
    let retained = scope
        .get(
            Table::Journeys,
            &checked(RowKey::new(format!(
                "human-native-send-access-{}",
                hex(ACTION)
            ))),
        )
        .expect("retained consent")
        .bytes()
        .to_vec();
    let first = preview_exchange(&context);
    let mut second = preview_exchange(&context);
    // The preview carries its own returned snapshot. Advance only the peer's head.
    let offset = second.1.len() - 56;
    assert_eq!(&second.1[offset..offset + 8], &HEAD.to_be_bytes());
    second.1[offset..offset + 8].copy_from_slice(&(HEAD + 1).to_be_bytes());
    let (peer, mut runtime) = Peer::serve(vec![
        nonce_exchange(NONCE),
        first,
        nonce_exchange(NONCE),
        second,
    ]);
    let first = checked(preview_send(
        &scope,
        &mut runtime,
        &request(),
        ACTION,
        EXPIRY_MS,
        COMMITMENT,
        NOW,
    ));
    let second = checked(preview_send(
        &scope,
        &mut runtime,
        &request(),
        ACTION,
        EXPIRY_MS,
        COMMITMENT,
        NOW,
    ));
    assert_eq!(first.head_sequence, HEAD);
    assert_eq!(second.head_sequence, HEAD + 1);
    assert!(second.protocol_time_ms >= first.protocol_time_ms);
    assert_eq!(second.canonical_bytes, first.canonical_bytes);
    assert_eq!(second.signing_preimage, first.signing_preimage);
    assert_eq!(second.purpose, first.purpose);
    assert_eq!(
        second.purpose.canonical_digest,
        <[u8; 32]>::from(Sha256::digest(&second.canonical_bytes))
    );
    assert_eq!(
        second.purpose.capability_id.to_bytes().expect("capability"),
        ACTION
    );
    assert_eq!(second.nonce, NONCE);
    assert_eq!(
        scope
            .get(
                Table::Journeys,
                &checked(RowKey::new(format!(
                    "human-native-send-access-{}",
                    hex(ACTION)
                )))
            )
            .expect("unchanged authorization")
            .bytes(),
        retained
    );
    assert_eq!(scope.keys(Table::Journeys).len(), 1);
    drop(scope);
    drop(store);
    peer.finish();
    checked(std::fs::remove_dir_all(root));
}
