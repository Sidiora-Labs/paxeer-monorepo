use base64::{
    engine::general_purpose::{STANDARD, URL_SAFE_NO_PAD},
    Engine as _,
};
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_human_service::auth::{
    AccountIdentity, AuthConfig, Device, Passkeys, RateLimit, SessionGrant,
};
use layerx_human_service::server::agent_runtime::{
    AgentRuntime, ManagedAgentEvidence, ManagedAgentView, VerifiedBalance,
};
use layerx_human_service::server::attestor_send_journal::{
    AttestorSendJournal, ATTESTOR_SEND_RETENTION_CAPACITY,
};
use layerx_human_service::server::backend::PrincipalContext;
use layerx_human_service::server::production_auth::{
    authorize_bearer_execution, authorize_execution, consume_context, AuthDiscoveryIndex,
    AuthorizationDisclosure, ExecutionCredentials, IndexAuthenticationKey,
    AUTH_INDEX_TRANSIENT_LIMIT,
};
use layerx_human_service::server::production_components::AccountBalance;
use layerx_human_service::server::schema::{ApiSchema, Operation};
use layerx_human_service::server::{
    ApiFailure, BackendResponse, BearerCredentials, HttpConfig, HumanApiComponents,
    IdentityDispatchError, IdentityProviderConfig, PrincipalLimits, ProductionComponents,
    ProductionComponentsConfig, Readiness, RemoteIdentityProvider, Router, ScopedRequest,
    SessionCredentials,
};
use layerx_human_service::store::{PrincipalStore, Table};
use layerx_human_test_support as support;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::collections::BTreeMap;
use std::fs;
use std::io::{Read as _, Write as _};
use std::os::unix::fs::PermissionsExt as _;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

const NOW: u64 = 1_700_000_000;
const ACCOUNT: &str = "act_00112233445566778899aabbccddeeff";
const TRACE: &str = "trc_00112233445566778899aabbccddeeff";
const ASSERTION: &str = "recorded-provider-admitted-assertion";
const DID: &str = "did:layerx:00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";

struct Directory(PathBuf);
impl Directory {
    fn new(label: &str) -> Self {
        let path = support::directory(label);
        fs::create_dir_all(&path).or_fail("create private fixture directory");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700)).or_fail("private directory");
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).or_fail("remove fixture directory");
    }
}
fn hex(bytes: &[u8]) -> String {
    layerx_human_service::hex::lower(bytes)
}
fn index(root: &std::path::Path) -> AuthDiscoveryIndex {
    AuthDiscoveryIndex::open(
        root,
        IndexAuthenticationKey::new([17; 32]).or_fail("index key"),
    )
    .or_fail("authenticated index")
}
fn disclosure<'a>(
    operation: &'a Operation,
    path: &'a str,
    parameters: &'a BTreeMap<String, String>,
    body: &'a Value,
) -> AuthorizationDisclosure<'a> {
    AuthorizationDisclosure {
        operation,
        destination: path,
        path_parameters: parameters,
        body,
        idempotency_key: None,
        trace: TRACE,
    }
}

#[test]
fn identity_field_allocation_is_bounded_on_the_provider_transport() {
    // Recorded LXIP response envelopes; malformed variants exercise the actual Unix client.
    let mut count_max = b"LXIP\x01\x00".to_vec();
    count_max.extend_from_slice(&u32::MAX.to_be_bytes());
    let mut missing_lengths = b"LXIP\x01\x00\x00\x00\x00\x02".to_vec();
    missing_lengths.extend_from_slice(&0u32.to_be_bytes());
    let mut overlong = b"LXIP\x01\x00\x00\x00\x00\x01".to_vec();
    overlong.extend_from_slice(&u32::MAX.to_be_bytes());
    let mut valid = b"LXIP\x01\x00\x00\x00\x00\x01".to_vec();
    valid.extend_from_slice(
        &u32::try_from(ACCOUNT.len())
            .unwrap_or_else(|_| panic!("account length fits u32"))
            .to_be_bytes(),
    );
    valid.extend_from_slice(ACCOUNT.as_bytes());
    let mut trailing = valid.clone();
    trailing.push(0);
    let invalid_utf8 = b"LXIP\x01\x00\x00\x00\x00\x01\x00\x00\x00\x01\xff".to_vec();
    let cases = vec![
        count_max,
        missing_lengths,
        overlong,
        trailing,
        invalid_utf8,
        b"LXIP\x01\x00".to_vec(),
        valid,
    ];
    let directory = Directory::new("identity-allocation");
    let socket = directory.0.join("identity.sock");
    let listener = UnixListener::bind(&socket).or_fail("provider socket");
    fs::set_permissions(&socket, fs::Permissions::from_mode(0o600)).or_fail("socket permissions");
    let worker = thread::spawn(move || {
        for response in cases {
            let (mut stream, _) = listener.accept().or_fail("provider accept");
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .or_fail("read deadline");
            let mut length = [0; 4];
            stream.read_exact(&mut length).or_fail("request length");
            let mut request = vec![0; u32::from_be_bytes(length) as usize];
            stream.read_exact(&mut request).or_fail("request");
            assert_eq!(&request[..6], b"LXIP\x01\x04");
            stream
                .write_all(
                    &u32::try_from(response.len())
                        .unwrap_or_else(|_| panic!("response length fits u32"))
                        .to_be_bytes(),
                )
                .or_fail("response length");
            stream.write_all(&response).or_fail("response");
        }
    });
    let provider = RemoteIdentityProvider::new(IdentityProviderConfig {
        socket,
        deadline: Duration::from_secs(5),
        maximum_frame_bytes: 1024,
        peer_uid: rustix::process::getuid().as_raw(),
        peer_gid: rustix::process::getgid().as_raw(),
    })
    .or_fail("provider");
    for _ in 0..6 {
        assert!(matches!(
            provider.resolve_assertion(ASSERTION),
            Err(IdentityDispatchError::ProviderEvidence)
        ));
    }
    assert_eq!(
        provider
            .resolve_assertion(ASSERTION)
            .unwrap_or_else(|error| panic!(
                "{}: {error:?}",
                "valid response after malformed frames"
            ))
            .principal,
        support::principal(ACCOUNT)
    );
    worker.join().or_fail("provider thread");
}

fn assert_auth_index_compaction(
    auth: &mut AuthDiscoveryIndex,
    principal: &layerx_human_service::store::PrincipalId,
) {
    auth.bind_account("owner@example.test", principal)
        .or_fail("durable account binding");
    for n in 0..AUTH_INDEX_TRANSIENT_LIMIT {
        auth.bind_assertion(&format!("asr_{n}"), principal, NOW)
            .or_fail("bounded assertion admission");
    }
    assert!(auth
        .bind_registration("reg_overflow", principal, NOW)
        .is_err());
    auth.compact(NOW + 1).or_fail("compact expired discoveries");
    auth.bind_step_up("stp_live", principal, NOW + 1000)
        .or_fail("new ceremony after full capacity");
    assert!(auth.resolve_assertion("asr_0", NOW + 1).is_err());
}

fn assert_durable_auth_index(
    root: &std::path::Path,
    principal: &layerx_human_service::store::PrincipalId,
) {
    let auth = index(root);
    assert_eq!(
        auth.resolve_account("owner@example.test")
            .unwrap_or_else(|error| panic!(
                "{}: {error:?}",
                "durable account after expiry and reopen"
            )),
        *principal
    );
    assert!(
        fs::metadata(root.join("authentication.index"))
            .or_fail("bounded persisted index")
            .len()
            < 2048
    );
}

fn bearer_context(
    store: &mut PrincipalStore,
    auth: &AuthDiscoveryIndex,
    principal: &layerx_human_service::store::PrincipalId,
    operation: &Operation,
    parameters: &BTreeMap<String, String>,
    body: &Value,
    context: &str,
) -> PrincipalContext {
    authorize_bearer_execution(
        store,
        auth,
        principal,
        ASSERTION,
        disclosure(operation, "/v1/account/balance", parameters, body),
        NOW + 1,
        30,
    )
    .or_fail(context)
    .into_bearer_context(ASSERTION, DID)
    .or_fail(context)
}

fn admission_contexts(
    store: &mut PrincipalStore,
    auth: &AuthDiscoveryIndex,
    principal: &layerx_human_service::store::PrincipalId,
    operation: &Operation,
    parameters: &BTreeMap<String, String>,
    body: &Value,
) -> (PrincipalContext, PrincipalContext) {
    let context = bearer_context(
        store,
        auth,
        principal,
        operation,
        parameters,
        body,
        "real capability producer",
    );
    let abandoned = bearer_context(
        store,
        auth,
        principal,
        operation,
        parameters,
        body,
        "abandoned capability",
    );
    (context, abandoned)
}

#[test]
fn auth_index_compacts_with_admission_and_replay_refusal_after_reopen() {
    let directory = Directory::new("auth-compaction");
    let store_root = directory.0.join("store");
    let (mut store, digest) = support::install_and_open(
        &store_root,
        &support::tenancy(&[(ACCOUNT, "tenant-a")]),
        support::retention_uniform(86400),
    );
    let principal = support::principal(ACCOUNT);
    let root = directory.0.join("index");
    let mut auth = index(&root);
    assert_auth_index_compaction(&mut auth, &principal);
    let schema = ApiSchema::v1().or_fail("schema");
    let operation = schema.operation("account.balance").or_fail("operation");
    let parameters = BTreeMap::new();
    let body = json!({});
    let (context, abandoned) =
        admission_contexts(&mut store, &auth, &principal, operation, &parameters, &body);
    drop(auth);
    drop(store);
    auth = index(&root);
    store = PrincipalStore::open(&store_root, support::retention_uniform(86400), digest)
        .or_fail("reopen store");
    assert_eq!(
        auth.resolve_account("owner@example.test")
            .or_fail("account retained"),
        principal
    );
    assert_eq!(
        auth.resolve_step_up("stp_live", NOW + 2)
            .or_fail("live discovery retained"),
        principal
    );
    consume_context(
        &auth,
        &context,
        disclosure(operation, "/v1/account/balance", &parameters, &body),
        NOW + 2,
    )
    .or_fail("retained authentication");
    assert!(consume_context(
        &auth,
        &context,
        disclosure(operation, "/v1/account/balance", &parameters, &body),
        NOW + 2
    )
    .is_err());
    drop(auth);
    auth = index(&root);
    assert!(consume_context(
        &auth,
        &context,
        disclosure(operation, "/v1/account/balance", &parameters, &body),
        NOW + 2
    )
    .is_err());
    auth.compact(NOW + 62)
        .or_fail("abandoned capability expiry");
    assert!(consume_context(
        &auth,
        &abandoned,
        disclosure(operation, "/v1/account/balance", &parameters, &body),
        NOW + 62
    )
    .is_err());
    assert!(
        auth.compact(NOW).is_err(),
        "clock rollback must not resurrect expired state"
    );
    let next = authorize_bearer_execution(
        &mut store,
        &auth,
        &principal,
        ASSERTION,
        disclosure(operation, "/v1/account/balance", &parameters, &body),
        NOW + 62,
        30,
    )
    .or_fail("continued capability issue")
    .into_bearer_context(ASSERTION, DID)
    .or_fail("new context");
    consume_context(
        &auth,
        &next,
        disclosure(operation, "/v1/account/balance", &parameters, &body),
        NOW + 63,
    )
    .or_fail("continued authentication");
    drop(auth);
    assert_durable_auth_index(&root, &principal);
}

static ENVIRONMENT: Mutex<()> = Mutex::new(());
struct Environment(Vec<(String, Option<std::ffi::OsString>)>);
impl Environment {
    fn set(&mut self, key: &str, value: Option<&str>) {
        if !self.0.iter().any(|(name, _)| name == key) {
            self.0.push((key.to_owned(), std::env::var_os(key)));
        }
        match value {
            Some(value) => std::env::set_var(key, value),
            None => std::env::remove_var(key),
        }
    }
}
impl Drop for Environment {
    fn drop(&mut self) {
        for (key, value) in self.0.iter().rev() {
            match value {
                Some(value) => std::env::set_var(key, value),
                None => std::env::remove_var(key),
            }
        }
    }
}

const CONFIG_VARIABLES: &[(&str, &str)] = &[
    ("LAYERX_HUMAN_ACTIVITY_EXPORT_MAXIMUM_BYTES", "30"),
    ("LAYERX_HUMAN_ACTIVITY_FRESHNESS_SECONDS", "30"),
    (
        "LAYERX_HUMAN_AGENT_ACTOR",
        "did:layerx:1111111111111111111111111111111111111111111111111111111111111111",
    ),
    ("LAYERX_HUMAN_AGENT_AUTHORITY", "custody-human-primary"),
    ("LAYERX_HUMAN_AGENT_DEADLINE_SECONDS", "5"),
    ("LAYERX_HUMAN_AGENT_FEE_LIMIT", "30"),
    ("LAYERX_HUMAN_AGENT_MAX_CONNECTIONS", "5"),
    ("LAYERX_HUMAN_AGENT_MAX_FRAME_BYTES", "1024"),
    ("LAYERX_HUMAN_AGENT_MAX_QUEUED_BYTES", "1024"),
    ("LAYERX_HUMAN_AGENT_MAX_STREAMS", "5"),
    (
        "LAYERX_HUMAN_AGENT_OWNER_ACCOUNT",
        "0x1111111111111111111111111111111111111111",
    ),
    (
        "LAYERX_HUMAN_AGENT_PURPOSE_CATALOG",
        "/tmp/layerx-config-fixture/layerx_human_agent_purpose_catalog",
    ),
    (
        "LAYERX_HUMAN_AGENT_RECOVERY_ROOT",
        "ERERERERERERERERERERERERERERERERERERERERERE",
    ),
    ("LAYERX_HUMAN_AGENT_RECOVERY_THRESHOLD", "30"),
    (
        "LAYERX_HUMAN_AGENT_SOCKET",
        "/tmp/layerx-config-fixture/layerx_human_agent_socket",
    ),
    ("LAYERX_HUMAN_AGENT_TIMESTAMP_SPAN_SECONDS", "30"),
    ("LAYERX_HUMAN_ASSERTION_TTL_SECONDS", "30"),
    (
        "LAYERX_HUMAN_AUTH_INDEX_KEY",
        "ERERERERERERERERERERERERERERERERERERERERERE",
    ),
    (
        "LAYERX_HUMAN_AUTH_INDEX_ROOT",
        "/tmp/layerx-config-fixture/layerx_human_auth_index_root",
    ),
    ("LAYERX_HUMAN_AUTH_RATE_ATTEMPTS", "30"),
    ("LAYERX_HUMAN_AUTH_RATE_WINDOW_SECONDS", "30"),
    ("LAYERX_HUMAN_CAPABILITY_TTL_SECONDS", "30"),
    ("LAYERX_HUMAN_CEREMONY_TTL_SECONDS", "30"),
    ("LAYERX_HUMAN_CONTINUATION_UNKNOWN_DEADLINE_SECONDS", "30"),
    (
        "LAYERX_HUMAN_CUSTODY_ROOT",
        "/tmp/layerx-config-fixture/layerx_human_custody_root",
    ),
    ("LAYERX_HUMAN_EVM_GAS_LIMIT", "30"),
    ("LAYERX_HUMAN_EVM_MAX_FEE_PER_GAS", "30"),
    ("LAYERX_HUMAN_EVM_MAX_PRIORITY_FEE_PER_GAS", "30"),
    ("LAYERX_HUMAN_IDENTITY_BINDING_DEADLINE_SECONDS", "30"),
    ("LAYERX_HUMAN_IDENTITY_BINDING_PEER_GID", "30"),
    ("LAYERX_HUMAN_IDENTITY_BINDING_PEER_UID", "30"),
    (
        "LAYERX_HUMAN_IDENTITY_BINDING_SOCKET",
        "/tmp/layerx-config-fixture/layerx_human_identity_binding_socket",
    ),
    ("LAYERX_HUMAN_IDENTITY_BINDING_TENANT", "tenant-a"),
    ("LAYERX_HUMAN_IDENTITY_DEADLINE_SECONDS", "30"),
    ("LAYERX_HUMAN_IDENTITY_MAX_FRAME_BYTES", "30"),
    ("LAYERX_HUMAN_IDENTITY_PEER_GID", "30"),
    ("LAYERX_HUMAN_IDENTITY_PEER_UID", "30"),
    (
        "LAYERX_HUMAN_IDENTITY_SOCKET",
        "/tmp/layerx-config-fixture/layerx_human_identity_socket",
    ),
    (
        "LAYERX_HUMAN_KMS_CLIENT_CERTIFICATE_DER",
        "/tmp/layerx-config-fixture/layerx_human_kms_client_certificate_der",
    ),
    (
        "LAYERX_HUMAN_KMS_CLIENT_PRIVATE_KEY_DER",
        "/tmp/layerx-config-fixture/layerx_human_kms_client_private_key_der",
    ),
    ("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "30"),
    ("LAYERX_HUMAN_KMS_ENDPOINT", "127.0.0.1:1"),
    ("LAYERX_HUMAN_KMS_MAX_CONNECTIONS", "30"),
    ("LAYERX_HUMAN_KMS_MAX_FRAME_BYTES", "30"),
    ("LAYERX_HUMAN_KMS_MAX_QUEUED_BYTES", "30"),
    ("LAYERX_HUMAN_KMS_MAX_STREAMS", "30"),
    (
        "LAYERX_HUMAN_KMS_PROVIDER_REFERENCE",
        "configured-provider/v1",
    ),
    (
        "LAYERX_HUMAN_KMS_ROOT_CERTIFICATE_DER",
        "/tmp/layerx-config-fixture/layerx_human_kms_root_certificate_der",
    ),
    ("LAYERX_HUMAN_KMS_SERVER_NAME", "localhost"),
    ("LAYERX_HUMAN_NETWORK_ID", "125"),
    ("LAYERX_HUMAN_ONBOARDING_INITIAL_FUNDING", "30"),
    (
        "LAYERX_HUMAN_ONBOARDING_SPONSOR_PRINCIPAL",
        "act_00112233445566778899aabbccddeeff",
    ),
    ("LAYERX_HUMAN_ORIGIN", "https://paxportwallet.com"),
    ("LAYERX_HUMAN_REFRESH_TTL_SECONDS", "30"),
    ("LAYERX_HUMAN_RETENTION_AUDIT_SECONDS", "30"),
    ("LAYERX_HUMAN_RETENTION_CACHE_SECONDS", "30"),
    ("LAYERX_HUMAN_RETENTION_JOURNEYS_SECONDS", "30"),
    ("LAYERX_HUMAN_RETENTION_NOTIFICATIONS_SECONDS", "30"),
    ("LAYERX_HUMAN_RETENTION_TELEMETRY_SECONDS", "30"),
    ("LAYERX_HUMAN_RP_NAME", "Paxport"),
    ("LAYERX_HUMAN_SECURITY_DEADLINE_SECONDS", "30"),
    ("LAYERX_HUMAN_SECURITY_MAX_FRAME_BYTES", "30"),
    (
        "LAYERX_HUMAN_SECURITY_SOCKET",
        "/tmp/layerx-config-fixture/layerx_human_security_socket",
    ),
    ("LAYERX_HUMAN_SESSION_TTL_SECONDS", "30"),
    ("LAYERX_HUMAN_SIGNING_RATE_MAXIMUM", "30"),
    ("LAYERX_HUMAN_SIGNING_RATE_WINDOW_SECONDS", "30"),
    ("LAYERX_HUMAN_STEP_UP_TTL_SECONDS", "30"),
    (
        "LAYERX_HUMAN_STORE_ROOT",
        "/tmp/layerx-config-fixture/layerx_human_store_root",
    ),
    (
        "LAYERX_HUMAN_STREAM_CURSOR_KEY",
        "ERERERERERERERERERERERERERERERERERERERERERE",
    ),
    (
        "LAYERX_HUMAN_TENANCY_DIGEST",
        "ERERERERERERERERERERERERERERERERERERERERERE",
    ),
];

#[test]
fn protocol_selection_uses_the_real_environment_loader() {
    let _guard = ENVIRONMENT.lock().or_fail("environment lock");
    let mut environment = Environment(Vec::new());
    environment.set("LAYERX_HUMAN_ATTESTOR_NODES", None);
    for &(key, value) in CONFIG_VARIABLES {
        environment.set(key, Some(value));
    }
    environment.set("LAYERX_HUMAN_PROTOCOL_VERSION", None);
    assert_eq!(
        ProductionComponentsConfig::from_environment()
            .or_fail("absent selects merged protocol")
            .protocol_version(),
        3
    );
    environment.set("LAYERX_HUMAN_PROTOCOL_VERSION", Some("3"));
    assert_eq!(
        ProductionComponentsConfig::from_environment()
            .or_fail("explicit merged protocol")
            .protocol_version(),
        3
    );
    for invalid in ["", "2", "0", "4", "65536", "three"] {
        environment.set("LAYERX_HUMAN_PROTOCOL_VERSION", Some(invalid));
        let error = ProductionComponentsConfig::from_environment()
            .err()
            .or_fail("invalid protocol refused");
        assert!(
            error.contains("LAYERX_HUMAN_PROTOCOL_VERSION"),
            "{invalid:?}: {error}"
        );
    }
    environment.set(
        "LAYERX_HUMAN_ATTESTOR_NODES",
        Some("configured-but-invalid"),
    );
    assert!(ProductionComponentsConfig::from_environment()
        .err()
        .or_fail("invalid protocol precedes attestor I/O")
        .contains("LAYERX_HUMAN_PROTOCOL_VERSION"));
}

fn signed_send(nonce: u64, now: u64) -> (Value, Vec<u8>) {
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    let call = Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 80_000,
        to: bank,
        data: vec![1, 2, 3],
    };
    let raw = call
        .sign(&[19; 32])
        .or_fail("real EIP-1559 signing producer");
    let (_, signature) = decode_signed(&raw).or_fail("signed transaction codec");
    let wallet = recover_sender(&call, &signature).or_fail("real signer wallet");
    let mut action = [0; 32];
    action[24..].copy_from_slice(&(nonce + 1).to_be_bytes());
    let mut amount = [0; 32];
    amount[31] = 4;
    let transaction = layerx_human_service::custody::EvmTransaction {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 80_000,
        to: bank,
        value: [0; 32],
        calldata: Vec::new(),
    }
    .with_bank_transfer(wallet, [3; 20], "alxp", amount)
    .or_fail("real bank-send producer");
    let call = Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 80_000,
        to: bank,
        data: transaction.calldata.clone(),
    };
    let raw = call.sign(&[19; 32]).or_fail("sign production bank call");
    let authorization = layerx_human_service::custody::EvmPlanAuthorization {
        plan_id: [1; 32],
        action_key: action,
        tenant: "tenant-a".into(),
        principal: ACCOUNT.into(),
        binding_digest: [2; 32],
        wallet,
        not_before: now,
        not_after: now + 1,
        transaction,
    };
    let payload = json!({"authorization": authorization,
        "disclosure": {"recipient": vec![3; 20], "denom": "alxp", "amount": amount}});
    (payload, raw)
}

#[test]
fn attestor_send_journal_continues_beyond_capacity_and_preserves_duplicates_after_restart() {
    let directory = Directory::new("attestor-send-replay");
    let mut journal = AttestorSendJournal::open(&directory.0, [23; 32]).or_fail("durable journal");
    assert!(
        AttestorSendJournal::open(&directory.0, [23; 32]).is_err(),
        "only one signing writer"
    );
    let mut last = None;
    for nonce in 0..=ATTESTOR_SEND_RETENTION_CAPACITY as u64 {
        let now = NOW + nonce * 2;
        let (payload, raw) = signed_send(nonce, now);
        let bytes = serde_json::to_vec(&payload).or_fail("authorization producer");
        assert_eq!(
            journal.reserve(&bytes, now).unwrap_or_else(|error| panic!(
                "{}: {error:?}",
                "continued reservation beyond capacity"
            )),
            None
        );
        assert_eq!(
            journal
                .complete(&bytes, raw.clone())
                .or_fail("durable completion"),
            raw
        );
        assert_eq!(
            journal.reserve(&bytes, now).or_fail("exact duplicate"),
            Some(raw.clone())
        );
        let retained = journal.retained_requests();
        assert!(retained <= 1, "safe expiry bounds retained work");
        last = Some((payload, raw, now));
    }
    let (payload, raw, now) = last.or_fail("last send");
    let bytes = serde_json::to_vec(&payload).or_fail("payload");
    drop(journal);
    journal = AttestorSendJournal::open(&directory.0, [23; 32]).or_fail("reopen signing journal");
    assert_eq!(
        journal
            .reserve(&bytes, now)
            .or_fail("exact replay after restart"),
        Some(raw)
    );
    let mut changed = payload.clone();
    changed["disclosure"]["denom"] = json!("another-denom");
    assert!(journal
        .reserve(
            &serde_json::to_vec(&changed).or_fail("changed payload"),
            now
        )
        .is_err());
    let (mut collision, _) = signed_send(0, now);
    collision["authorization"]["action_key"] = json!(vec![99; 32]);
    assert!(
        journal
            .reserve(&serde_json::to_vec(&collision).or_fail("collision"), now)
            .is_err(),
        "evicted nonce is still protected"
    );
    assert!(
        journal.reserve(&bytes, now + 2).is_err(),
        "expired exact request cannot sign again"
    );
    let (pending, _) = signed_send(ATTESTOR_SEND_RETENTION_CAPACITY as u64 + 1, now + 2);
    let pending_bytes = serde_json::to_vec(&pending).or_fail("pending payload");
    assert!(journal
        .reserve(&pending_bytes, now + 2)
        .or_fail("pending reservation")
        .is_none());
    drop(journal);
    journal =
        AttestorSendJournal::open(&directory.0, [23; 32]).or_fail("crash recovery of reservation");
    assert!(journal
        .reserve(&pending_bytes, now + 2)
        .or_fail("exact unfinished recovery")
        .is_none());
    let mut changed = pending;
    changed["authorization"]["action_key"] = json!(vec![98; 32]);
    assert!(
        journal
            .reserve(
                &serde_json::to_vec(&changed).or_fail("changed pending"),
                now + 2
            )
            .is_err(),
        "unfinished signing cannot reuse its nonce"
    );
    drop(journal);
    assert!(
        AttestorSendJournal::open(&directory.0, [24; 32]).is_err(),
        "authenticate persisted replay state"
    );
    let path = directory.0.join("attestor-sends.v1");
    let mut tampered = fs::read(&path).or_fail("journal bytes");
    tampered[10] ^= 1;
    fs::write(&path, tampered).or_fail("tamper journal");
    assert!(
        AttestorSendJournal::open(&directory.0, [23; 32]).is_err(),
        "tampered nonce floor must refuse reopen"
    );
}

// This transport composition delegates evidence and capability work to production
// components. The recorded provider admission supplies the principal fixture.
struct EvidenceApi {
    store: Mutex<PrincipalStore>,
    auth: AuthDiscoveryIndex,
    passkeys: Passkeys,
}
impl HumanApiComponents for EvidenceApi {
    fn authorize(
        &self,
        operation: &Operation,
        credentials: SessionCredentials<'_>,
        trace: &str,
    ) -> Result<PrincipalContext, ApiFailure> {
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        authorize_execution(
            &mut store,
            &self.passkeys,
            &self.auth,
            ExecutionCredentials {
                access: credentials.access_token,
                csrf: credentials.csrf_token,
                step_up: None,
            },
            AuthorizationDisclosure {
                operation,
                destination: credentials.intended_destination,
                path_parameters: credentials.path_parameters,
                body: credentials.body,
                idempotency_key: credentials.idempotency_key,
                trace,
            },
            NOW,
            30,
        )
        .map_err(|_| ApiFailure::unauthenticated())?
        .into_context()
    }
    fn admit_bearer_assertion(
        &self,
        operation: &Operation,
        credentials: BearerCredentials<'_>,
        trace: &str,
    ) -> Result<PrincipalContext, ApiFailure> {
        if credentials.assertion != ASSERTION {
            return Err(ApiFailure::unauthenticated());
        }
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        authorize_bearer_execution(
            &mut store,
            &self.auth,
            &support::principal(ACCOUNT),
            credentials.assertion,
            AuthorizationDisclosure {
                operation,
                destination: credentials.intended_destination,
                path_parameters: credentials.path_parameters,
                body: credentials.body,
                idempotency_key: credentials.idempotency_key,
                trace,
            },
            NOW,
            30,
        )
        .map_err(|_| ApiFailure::unauthenticated())?
        .into_bearer_context(credentials.assertion, DID)
    }
    fn execute(&self, request: ScopedRequest<'_>) -> Result<BackendResponse, ApiFailure> {
        if request.operation.name != "evidence.get" {
            return Err(ApiFailure::invalid_request(None));
        }
        let context = request
            .principal
            .as_ref()
            .ok_or_else(ApiFailure::unauthenticated)?;
        let path = format!("/v1/evidence/{}", request.path_parameters["evidence_id"]);
        consume_context(
            &self.auth,
            context,
            AuthorizationDisclosure {
                operation: request.operation,
                destination: &path,
                path_parameters: &request.path_parameters,
                body: &request.body,
                idempotency_key: request.idempotency_key.as_deref(),
                trace: &request.trace,
            },
            NOW,
        )
        .map_err(|_| ApiFailure::forbidden())?;
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        let scope = store
            .principal(&context.principal)
            .map_err(|_| ApiFailure::unavailable())?;
        ProductionComponents::evidence_get(&scope, &request)
    }
    fn stream(
        &self,
        _: ScopedRequest<'_>,
        _: usize,
        _: &mut dyn FnMut(Value) -> Result<(), ApiFailure>,
        _: &dyn Fn() -> bool,
    ) -> Result<(), ApiFailure> {
        Err(ApiFailure::invalid_request(None))
    }
    fn readiness(&self, _: &str) -> Result<Readiness, ApiFailure> {
        Err(ApiFailure::unavailable())
    }
}
fn http_evidence(backend: EvidenceApi, id: &str, session: &SessionGrant) -> (String, Value) {
    let router = Arc::new(
        Router::new(
            Arc::new(backend),
            PrincipalLimits::new(100, 60, 100).or_fail("limits"),
            HttpConfig {
                maximum_header_bytes: 32768,
                maximum_body_bytes: 32768,
                allowed_origin: "https://paxportwallet.com".into(),
                service_version: "repair-22.33.42".into(),
            },
            Arc::new(support::StepClock::new(NOW * 1000)),
        )
        .or_fail("real HTTP router"),
    );
    let (mut client, mut server) = UnixStream::pair().or_fail("HTTP transport");
    client
        .set_read_timeout(Some(Duration::from_secs(5)))
        .or_fail("HTTP read deadline");
    let worker = thread::spawn(move || {
        router
            .serve_one(&mut server, "evidence-fixture")
            .or_fail("serve HTTP");
    });
    write!(client, "GET /v1/evidence/{id} HTTP/1.1\r\nHost: paxportwallet.com\r\nCookie: __Host-layerx_access={}; __Host-layerx_csrf={}\r\nContent-Length: 0\r\n\r\n", session.access_token().expose(), session.csrf_token().expose()).or_fail("HTTP evidence request");
    let mut response = String::new();
    client
        .read_to_string(&mut response)
        .or_fail("HTTP response");
    worker.join().or_fail("HTTP worker");
    let (headers, body) = response.split_once("\r\n\r\n").or_fail("HTTP envelope");
    (
        headers.to_owned(),
        serde_json::from_str(body).or_fail("HTTP JSON"),
    )
}

fn balance_for_rung(rung: Option<Rung>) -> AccountBalance {
    AccountBalance::from_agent(VerifiedBalance {
        account: [1; 32],
        asset: [2; 32],
        currency: "LXP".into(),
        observed_at: "2023-11-14T22:13:20Z".into(),
        age_seconds: 0,
        amount: 123,
        verification: rung,
        global_sequence: 10,
        batch_number: 1,
        observed_head_sequence: 10,
        observed_checkpoint: [0; 32],
        canonical_bytes: 123u128.to_be_bytes().to_vec(),
        proof_material: vec![1; 32],
    })
    .or_fail("production balance")
}

#[test]
fn balance_fixture_uses_production_material_through_persistence_reopen_and_http() {
    let directory = Directory::new("balance-fixture");
    let root = directory.0.join("store");
    let auth_root = directory.0.join("index");
    let (mut store, digest) = support::install_and_open(
        &root,
        &support::tenancy(&[(ACCOUNT, "tenant-a")]),
        support::retention_uniform(86400),
    );
    let (passkeys, grant) = browser_session(&mut store);
    index(&auth_root)
        .bind_session(&grant, &support::principal(ACCOUNT))
        .or_fail("durable browser discovery");
    let schema = ApiSchema::v1().or_fail("schema");
    for (rung, label, class) in [
        (None, "unverified", "submission-record"),
        (Some(Rung::Submitted), "unverified", "submission-record"),
        (Some(Rung::Receipted), "receipt-verified", "layerx-receipt"),
        (Some(Rung::Committed), "committed", "checkpoint-proof"),
    ] {
        let balance = balance_for_rung(rung);
        let bytes = balance
            .evidence_material()
            .or_fail("production evidence encoder");
        assert_eq!(
            AccountBalance::material_rung(&bytes).or_fail("production evidence decoder"),
            rung
        );
        let hash: [u8; 32] = Sha256::digest(&bytes).into();
        let id = format!("evd_{}", hex(&hash));
        let key = support::row_key(&format!("state-proof-{}", hex(&hash)));
        store
            .principal(&support::principal(ACCOUNT))
            .or_fail("scope")
            .put(Table::Cache, key.clone(), NOW, bytes.clone())
            .or_fail("persist evidence");
        let projected = balance.project(hash, 60);
        assert_eq!(projected["verification"], label);
        schema
            .encode_response(
                schema
                    .operation("account.balance")
                    .or_fail("balance operation"),
                &projected,
            )
            .or_fail("balance schema");
        for _ in 0..2 {
            let (headers, response) = http_evidence(
                EvidenceApi {
                    store: Mutex::new(store),
                    auth: index(&auth_root),
                    passkeys: passkeys.clone(),
                },
                &id,
                &grant,
            );
            assert!(
                headers.starts_with("HTTP/1.1 200 "),
                "{headers}: {response}"
            );
            assert_eq!(response["result"]["verification"], label);
            assert_eq!(response["result"]["class"], class);
            assert_eq!(
                STANDARD
                    .decode(
                        response["result"]["bytes_base64"]
                            .as_str()
                            .or_fail("evidence bytes")
                    )
                    .or_fail("decode HTTP material"),
                bytes
            );
            store = PrincipalStore::open(&root, support::retention_uniform(86400), digest)
                .unwrap_or_else(|error| {
                    panic!("{}: {error:?}", "reopen after stopping HTTP consumer")
                });
        }
        let mut altered = bytes.clone();
        altered[5] ^= 1;
        store
            .principal(&support::principal(ACCOUNT))
            .or_fail("scope")
            .put(Table::Cache, key, NOW, altered)
            .or_fail("persist tampered label");
        let (headers, response) = http_evidence(
            EvidenceApi {
                store: Mutex::new(store),
                auth: index(&auth_root),
                passkeys: passkeys.clone(),
            },
            &id,
            &grant,
        );
        assert!(
            headers.starts_with("HTTP/1.1 503 "),
            "tampered evidence accepted: {headers}: {response}"
        );
        store = PrincipalStore::open(&root, support::retention_uniform(86400), digest)
            .or_fail("reopen after tamper refusal");
    }
}

fn peer_agent(rung: Rung) -> ManagedAgentView {
    ManagedAgentView {
        agent_id: "agt_00112233445566778899aabbccddeeff".into(),
        name: "Research agent".into(),
        purpose: "research".into(),
        state: 1,
        monthly_limit: 100,
        currency: "LXP".into(),
        limit_enforcement: 0,
        period_start: NOW.to_string(),
        period_end: (NOW + 86400).to_string(),
        spent: 30,
        remaining: 70,
        spend_verification: rung,
        created_at: NOW.to_string(),
        updated_at: NOW.to_string(),
        evidence: vec![ManagedAgentEvidence {
            evidence_id: hex(&[31; 32]),
            class: "agent-creation".into(),
            verification: rung,
        }],
    }
}
fn read_peer_response(
    directory: &Directory,
    bytes: Vec<u8>,
) -> Result<ManagedAgentView, layerx_human_service::journeys::AgentBoundaryError> {
    let socket = directory.0.join("managed-peer.sock");
    let listener = UnixListener::bind(&socket).or_fail("peer listener");
    let worker = thread::spawn(move || {
        let (mut stream, _) = listener.accept().or_fail("peer accept");
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .or_fail("peer read deadline");
        let mut length = [0; 4];
        stream.read_exact(&mut length).or_fail("frame length");
        let mut request = vec![0; u32::from_be_bytes(length) as usize];
        stream.read_exact(&mut request).or_fail("agent request");
        assert_eq!(&request[..8], b"LXHAGT01");
        stream
            .write_all(
                &u32::try_from(bytes.len())
                    .unwrap_or_else(|_| panic!("response length fits u32"))
                    .to_be_bytes(),
            )
            .or_fail("peer response length");
        stream.write_all(&bytes).or_fail("peer response");
    });
    let mut runtime = AgentRuntime::new_with_clock(
        &socket,
        layerx_human_service::server::default_component_limits(),
        support::evidence_registry(),
        Arc::new(support::StepClock::new(NOW * 1000)),
    )
    .or_fail("real agent consumer");
    let result = runtime.agent_get("agt_00112233445566778899aabbccddeeff");
    worker.join().or_fail("peer worker");
    fs::remove_file(socket).or_fail("remove peer socket");
    result
}

#[test]
fn projection_uses_versioned_typed_rungs_through_peer_codec_and_reopen() {
    let directory = Directory::new("managed-rung-peer");
    let schema = ApiSchema::v1().or_fail("schema");
    for (rung, label) in [
        (Rung::Submitted, "unverified"),
        (Rung::Receipted, "receipt-verified"),
        (Rung::Committed, "committed"),
    ] {
        let producer = peer_agent(rung);
        let bytes = producer
            .encode_peer_response()
            .or_fail("versioned production peer codec");
        let retained = directory.0.join("managed-state.response");
        fs::write(&retained, &bytes).or_fail("retain producer record");
        for response in [
            bytes.clone(),
            fs::read(&retained).or_fail("reopen retained response"),
        ] {
            let consumed = read_peer_response(&directory, response).or_fail("typed peer consumer");
            assert_eq!(consumed.spend_verification, rung);
            assert_eq!(consumed.evidence[0].verification, rung);
            let projected = consumed.project().or_fail("real managed projection");
            assert_eq!(projected["spend"]["verification"], label);
            assert_eq!(projected["evidence"][0]["verification"], label);
            schema
                .encode_response(
                    schema.operation("agent.get").or_fail("agent operation"),
                    &projected,
                )
                .or_fail("HTTP managed schema");
        }
        let marker = bytes
            .iter()
            .position(|byte| *byte == 0xa3)
            .or_fail("explicit rung version marker");
        let mut retired = bytes.clone();
        retired.remove(marker);
        retired[marker] = 4;
        assert!(
            read_peer_response(&directory, retired).is_err(),
            "unversioned retained legacy record needs producer migration"
        );
        let mut invalid = bytes;
        invalid[marker + 1] = 3;
        assert!(
            read_peer_response(&directory, invalid).is_err(),
            "retired rung is refused in a versioned record"
        );
    }
}

const RP_ID: &str = "paxportwallet.com";
const ORIGIN: &str = "https://paxportwallet.com";
const FLAG_UP: u8 = 1;
const FLAG_UV: u8 = 4;
const FLAG_AT: u8 = 64;
fn required<T, E: std::fmt::Debug>(value: Result<T, E>, label: &str) -> T {
    value.unwrap_or_else(|error| panic!("{label}: {error:?}"))
}
struct SoftwareAuthenticator {
    signing_key: SigningKey,
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
            signing_key: SigningKey::from_bytes(&seed),
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
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.counter = self.counter.saturating_add(1);
        let authenticator_data = self.authenticator_data(self.counter, false);
        let client_data = client_data("webauthn.get", challenge);
        let client_hash = Sha256::digest(&client_data);
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
        let map = CborValue::Map(vec![
            (
                CborValue::Text("fmt".to_owned()),
                CborValue::Text("none".to_owned()),
            ),
            (
                CborValue::Text("attStmt".to_owned()),
                CborValue::Map(Vec::new()),
            ),
            (
                CborValue::Text("authData".to_owned()),
                CborValue::Bytes(self.authenticator_data(0, true)),
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
        let mut bytes = Vec::new();
        bytes.extend_from_slice(&Sha256::digest(RP_ID.as_bytes()));
        bytes.push(FLAG_UP | FLAG_UV | if attested { FLAG_AT } else { 0 });
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
        let map = CborValue::Map(vec![
            (CborValue::Integer(1.into()), CborValue::Integer(1.into())),
            (
                CborValue::Integer(3.into()),
                CborValue::Integer((-8).into()),
            ),
            (
                CborValue::Integer((-1).into()),
                CborValue::Integer(6.into()),
            ),
            (
                CborValue::Integer((-2).into()),
                CborValue::Bytes(self.signing_key.verifying_key().to_bytes().to_vec()),
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

fn decode_ceremony(value: &str) -> Value {
    let bytes = required(URL_SAFE_NO_PAD.decode(value), "decode ceremony");
    required(serde_json::from_slice(&bytes), "parse ceremony")
}

fn required_text<'value>(value: &'value Value, pointer: &str) -> &'value str {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .unwrap_or_else(|| panic!("missing ceremony field {pointer}"))
}

fn encode_response(value: &Value) -> String {
    URL_SAFE_NO_PAD.encode(required(serde_json::to_vec(value), "serialize response"))
}

fn client_data(kind: &str, challenge: &str) -> Vec<u8> {
    required(
        serde_json::to_vec(&json!({
            "type": kind,
            "challenge": challenge,
            "origin": ORIGIN,
            "crossOrigin": false,
        })),
        "encode client data",
    )
}

fn browser_session(store: &mut PrincipalStore) -> (Passkeys, SessionGrant) {
    let passkeys = Passkeys::new(AuthConfig {
        rp_id: RP_ID.into(),
        rp_name: "LayerX".into(),
        origin: ORIGIN.into(),
        ceremony_ttl_secs: 300,
        assertion_ttl_secs: 60,
        session_ttl_secs: 300,
        refresh_ttl_secs: 3600,
        step_up_ttl_secs: 60,
        rate_limit: RateLimit {
            attempts: 100,
            window_secs: 60,
        },
    })
    .or_fail("real passkeys");
    let mut scope = store
        .principal(&support::principal(ACCOUNT))
        .or_fail("browser principal");
    let mut authenticator = SoftwareAuthenticator::new();
    let registration = passkeys
        .begin_registration(
            &mut scope,
            &AccountIdentity::new(ACCOUNT, "Owner").or_fail("account"),
            "Primary",
            NOW,
        )
        .or_fail("registration challenge");
    let credential = authenticator.register(&registration.ceremony);
    passkeys
        .finish_registration(&mut scope, &registration.registration_id, &credential, NOW)
        .or_fail("signed WebAuthn registration");
    let assertion = passkeys
        .begin_assertion(&mut scope, NOW)
        .or_fail("assertion challenge");
    let credential = authenticator.assert(&assertion.ceremony);
    let proof = passkeys
        .finish_assertion(&mut scope, &assertion.assertion_id, &credential, NOW)
        .or_fail("signed WebAuthn assertion");
    let grant = passkeys
        .open_session(
            &mut scope,
            &proof.assertion_id,
            Device::mint("Browser", "web").or_fail("device"),
            NOW,
        )
        .or_fail("real browser session");
    (passkeys, grant)
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
