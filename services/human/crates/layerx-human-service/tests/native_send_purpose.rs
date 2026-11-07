use base64::engine::general_purpose::{STANDARD, URL_SAFE_NO_PAD};
use base64::Engine as _;
use layerx_agent_api::identity::{
    NativeLocalGrantConsentV1, NativePreparationPurposeV1, NativeSendPurposeV2,
    SignedNativeSendPurposeV2,
};
use layerx_client::runtime_clock::RuntimeClock;
use layerx_human_kms::attestor::{
    native_local_grant_signing_bytes, AttestorClient, AttestorError, AttestorSigner,
};
use layerx_human_service::auth::{
    AccountIdentity, AuthConfig, Device, OperationDigest, Passkeys, RateLimit,
};
use layerx_human_service::custody::{
    CustodyError, CustodySigner, KeyClass, KeyId, Keystore, NativeConsent, NativeConsentRequest,
    Operation, PrincipalKeyBinding, SignAuthorization, SigningLimits,
};
use layerx_human_service::server::production_components::{AttestorCustodyConfig, AttestorKms};
use layerx_human_service::store::{
    AgentTenantId, PrincipalId, PrincipalStore, RetentionPeriod, RetentionPolicy, TenancyMap,
};
use layerx_human_service::trace::TraceId;
use layerx_types::clock::{Clock, Deadline};
use layerx_types::payload::ModuleRegistry;
use serde::Deserialize;
use serde_json::json;
use sha2::{Digest, Sha256};
use std::error::Error;
use std::fs;
use std::future::Future;
use std::io::{BufRead, BufReader, Write};
use std::net::SocketAddr;
use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};
use std::path::Path;
use std::process::{Child, ChildStdin, ChildStdout, Command, Stdio};
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::time::Duration;

type Result<T> = std::result::Result<T, Box<dyn Error>>;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Node {
    id: String,
    address: SocketAddr,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Fixture {
    schema: String,
    directory: String,
    nodes: Vec<Node>,
    signers: Vec<String>,
    root_der: String,
    client_der: String,
    client_pkcs8: String,
    owner: String,
    account: String,
    owner_assertion: String,
    second_assertion: String,
    foreign_assertion: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Material {
    purpose: String,
    capability: String,
    native_session: String,
    expiry_ms: u64,
}

fn read_private(path: &Path) -> Result<Vec<u8>> {
    let metadata = fs::symlink_metadata(path)?;
    if !metadata.is_file()
        || metadata.mode() & 0o077 != 0
        || metadata.nlink() != 1
        || metadata.len() > 2_097_152
    {
        return Err("private live fixture ownership, permissions or bounds refused".into());
    }
    Ok(fs::read(path)?)
}

fn write_private(path: &Path, value: serde_json::Value) -> Result<()> {
    use std::io::Write as _;
    let temporary = path.with_extension("pending");
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&temporary)?;
    fs::set_permissions(&temporary, fs::Permissions::from_mode(0o600))?;
    file.write_all(&serde_json::to_vec(&value)?)?;
    file.sync_all()?;
    fs::rename(temporary, path)?;
    Ok(())
}

fn wait(path: &Path, clock: &dyn Clock) -> Result<()> {
    let mut deadline = Deadline::start(clock, Duration::from_secs(120))?;
    while !path.exists() {
        if deadline.remaining(clock)?.is_zero() {
            return Err("actual provider authority handshake deadline".into());
        }
        std::thread::sleep(Duration::from_millis(20));
    }
    read_private(path)?;
    Ok(())
}

struct CurrentThread;
impl Wake for CurrentThread {
    fn wake(self: Arc<Self>) {
        std::thread::current().unpark();
    }
}

fn ready<F: Future>(future: F) -> Result<F::Output> {
    let waker = Waker::from(Arc::new(CurrentThread));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(result) => Ok(result),
        Poll::Pending => {
            Err("production synchronous attestor provider unexpectedly yielded".into())
        }
    }
}

fn client(fixture: &Fixture) -> Result<AttestorClient> {
    let nodes: Vec<_> = fixture
        .nodes
        .iter()
        .map(|node| (node.id.clone(), node.address))
        .collect();
    Ok(AttestorClient::new(
        &nodes,
        &[STANDARD.decode(&fixture.root_der)?],
        &[STANDARD.decode(&fixture.client_der)?],
        &STANDARD.decode(&fixture.client_pkcs8)?,
        Duration::from_secs(120),
    )?)
}

struct Authenticator {
    process: Child,
    input: ChildStdin,
    output: BufReader<ChildStdout>,
}

impl Authenticator {
    fn open() -> Result<Self> {
        let script = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../../../apps/web/e2e/software-authenticator.ts");
        let mut process = Command::new("node")
            .arg(script)
            .arg("https://paxportwallet.com")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit())
            .spawn()?;
        let input = process
            .stdin
            .take()
            .ok_or("authenticator stdin unavailable")?;
        let output = BufReader::new(
            process
                .stdout
                .take()
                .ok_or("authenticator stdout unavailable")?,
        );
        Ok(Self {
            process,
            input,
            output,
        })
    }

    fn credential(&mut self, operation: &str, ceremony: &str) -> Result<String> {
        writeln!(
            self.input,
            "{}",
            json!({"operation":operation,"ceremony":ceremony})
        )?;
        self.input.flush()?;
        let mut response = Vec::new();
        let count =
            std::io::Read::take(&mut self.output, 32_769).read_until(b'\n', &mut response)?;
        if count == 0 || count > 32_768 || response.last() != Some(&b'\n') {
            return Err("authenticator response bound or EOF refused".into());
        }
        let value: serde_json::Value = serde_json::from_slice(&response)?;
        let credential = value
            .get("credential")
            .and_then(serde_json::Value::as_str)
            .ok_or("authenticator credential missing")?;
        if credential.is_empty() || credential.len() > 16_384 {
            return Err("credential bound refused".into());
        }
        Ok(credential.to_owned())
    }
}

impl Drop for Authenticator {
    fn drop(&mut self) {
        let _ = self.process.kill();
        let _ = self.process.wait();
    }
}

fn current_time(clock: &dyn Clock) -> Result<u64> {
    Ok(clock.sample(Duration::from_secs(1))?.unix_seconds())
}

#[test]
fn native_send_provider_and_sdk_real_cluster() -> Result<()> {
    let clock = RuntimeClock::from_environment()?;
    let path = std::env::var("PAXEER_X_NATIVE_SEND_LIVE_FIXTURE")?;
    let fixture: Fixture = serde_json::from_slice(&read_private(Path::new(&path))?)?;
    if fixture.schema != "layerx-human-native-send-live.v1" {
        return Err("closed genuine live fixture profile".into());
    }
    let directory = Path::new(&fixture.directory);
    let nodes: Vec<_> = fixture
        .nodes
        .iter()
        .map(|node| (node.id.clone(), node.address))
        .collect();
    let config = AttestorCustodyConfig::new(
        nodes,
        fixture.signers.clone(),
        STANDARD.decode(&fixture.root_der)?,
        STANDARD.decode(&fixture.client_der)?,
        STANDARD.decode(&fixture.client_pkcs8)?,
        Duration::from_secs(120),
    )?;
    let provider = AttestorKms::connect_with_clock(config, clock.clone())?;
    provider.admit_assertion(&fixture.owner, &fixture.owner_assertion)?;
    let principal = PrincipalId::new("native-owner")?;
    let key = KeyId::new("primary")?;
    let binding = PrincipalKeyBinding::new(
        b"layerx-human-custody/v1\0native-owner\0primary\0human-primary".to_vec(),
        125,
        KeyClass::HumanPrimary,
        "attestor-quorum/v1",
    )?;
    let provider_key = format!(
        "lx-{}",
        binding.digest()[..16]
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    );
    write_private(
        &directory.join("provision.json"),
        json!({"key_id":provider_key}),
    )?;
    wait(&directory.join("provision-ready"), clock.as_ref())?;
    let keystore = Keystore::open_production(directory.join("custody"), 125, provider.clone())?;
    let public = provider.create_owned_key(
        &keystore,
        &principal,
        &key,
        &fixture.owner,
        &fixture.account,
    )?;
    assert_eq!(keystore.describe(&principal, &key)?.public_key, public);
    write_private(
        &directory.join("generated.json"),
        json!({"public":public.to_vec()}),
    )?;
    wait(&directory.join("signing.json"), clock.as_ref())?;
    let material: Material =
        serde_json::from_slice(&read_private(&directory.join("signing.json"))?)?;
    let purpose_bytes = STANDARD.decode(&material.purpose)?;
    let bound_domain = purpose_bytes
        .split_inclusive(|byte| *byte == 0)
        .next()
        .ok_or("native send purpose domain missing")?;
    eprintln!(
        "native send domains: consumer={:?} producer={:?}",
        String::from_utf8_lossy(NativeSendPurposeV2::DOMAIN),
        String::from_utf8_lossy(bound_domain)
    );
    let purpose = NativeSendPurposeV2::from_canonical_bytes(&purpose_bytes)
        .map_err(|error| format!("{error:?}"))?;
    let foreign_principal = PrincipalId::new("native-foreign")?;
    let tenancy = TenancyMap::new([
        (principal.clone(), AgentTenantId::new("tenant-a")?),
        (foreign_principal.clone(), AgentTenantId::new("tenant-b")?),
    ])?;
    let store_root = directory.join("human-store");
    let tenancy_digest = tenancy.install(&store_root)?;
    let period = RetentionPeriod::new(86400);
    let shared_store = Arc::new(std::sync::Mutex::new(PrincipalStore::open(
        &store_root,
        RetentionPolicy {
            journeys: period,
            notifications: period,
            audit: period,
            telemetry: period,
            cache: period,
        },
        tenancy_digest,
    )?));
    let custody = CustodySigner::new_shared(
        keystore,
        Arc::clone(&shared_store),
        ModuleRegistry::new(&[]).map_err(|error| format!("{error:?}"))?,
        SigningLimits::new(32, 60)?,
    );
    let now = current_time(clock.as_ref())?;
    let mut trace_entropy = [0; 16];
    getrandom::fill(&mut trace_entropy)
        .map_err(|error| std::io::Error::other(error.to_string()))?;
    let trace = TraceId::mint(trace_entropy);
    let mut store = shared_store
        .lock()
        .map_err(|_| "native send store lock poisoned")?;
    let mut scope = store.principal(&principal)?;
    let result = ready(custody.sign_native_consent_in_scope(
        &mut scope,
        NativeConsentRequest::new(
            &principal,
            &key,
            NativeConsent::SendPurpose(&purpose),
            SignAuthorization::new(Operation::ProtocolMutation, None),
            now,
            trace.clone(),
        ),
    ))??;
    let digest: [u8; 32] = Sha256::digest(
        purpose
            .canonical_bytes()
            .map_err(|error| format!("{error:?}"))?,
    )
    .into();
    assert_eq!(result.signer_public_key(), public);
    ed25519_dalek::Verifier::verify(
        &ed25519_dalek::VerifyingKey::from_bytes(&public)?,
        &digest,
        &ed25519_dalek::Signature::from_bytes(result.signature()),
    )
    .map_err(|_| "native purpose signature verification refused")?;
    let canonical = purpose
        .canonical_bytes()
        .map_err(|error| format!("{error:?}"))?;
    assert_eq!(
        NativeSendPurposeV2::from_canonical_bytes(&canonical)
            .map_err(|error| format!("{error:?}"))?,
        purpose
    );
    assert!(NativePreparationPurposeV1::from_canonical_bytes(&canonical).is_err());
    assert_eq!(purpose.consent_public_key, public);
    let signed_purpose = SignedNativeSendPurposeV2 {
        purpose: purpose.clone(),
        signature: *result.signature(),
    }
    .validate()
    .map_err(|error| format!("{error:?}"))?;
    let verifier = ed25519_dalek::VerifyingKey::from_bytes(&public)?;
    let signature = ed25519_dalek::Signature::from_bytes(&signed_purpose.signature);
    for field in 0..17 {
        let mut changed = purpose.clone();
        match field {
            0 => changed.generation += 1,
            1 => changed.expires_at_ms -= 1,
            2 => changed.preparation_id[0] ^= 1,
            3 => changed.canonical_digest[0] ^= 1,
            4 => changed.economic_action[0] ^= 1,
            5 => changed.idempotency_key[0] ^= 1,
            6 => changed.commitment[0] ^= 1,
            7 => changed.chain_id += 1,
            8 => changed.consent_public_key[0] ^= 1,
            9 => {
                changed.activity = layerx_agent_api::identity::NativeActivity::new(1, 6)
                    .map_err(|error| format!("{error:?}"))?
            }
            10 => changed.transaction_type += 1,
            11 => {
                changed.session_id = layerx_agent_api::identity::SessionId::new("33".repeat(32))
                    .map_err(|error| format!("{error:?}"))?
            }
            12 => {
                changed.capability_id =
                    layerx_agent_api::identity::CapabilityId::new("44".repeat(32))
                        .map_err(|error| format!("{error:?}"))?
            }
            13 => {
                changed.tenant = layerx_agent_api::identity::TenantId::new("tenant-b")
                    .map_err(|error| format!("{error:?}"))?
            }
            14 => {
                changed.agent_did = layerx_agent_api::identity::AgentDid::new(format!(
                    "did:layerx:{}",
                    "bb".repeat(32)
                ))
                .map_err(|error| format!("{error:?}"))?
            }
            15 => changed.evm_sender[0] ^= 1,
            _ => {
                changed.owner_did = layerx_agent_api::identity::AgentDid::new(format!(
                    "did:layerx:{}",
                    "bb".repeat(32)
                ))
                .map_err(|error| format!("{error:?}"))?
            }
        }
        if let Ok(bytes) = changed.canonical_bytes() {
            let changed_digest: [u8; 32] = Sha256::digest(bytes).into();
            assert!(
                ed25519_dalek::Verifier::verify(&verifier, &changed_digest, &signature).is_err()
            );
        } else {
            assert!(changed.validate().is_err());
        }
    }
    for length in 0..canonical.len() {
        assert!(NativeSendPurposeV2::from_canonical_bytes(&canonical[..length]).is_err());
    }
    let mut trailing = canonical.clone();
    trailing.push(0);
    assert!(NativeSendPurposeV2::from_canonical_bytes(&trailing).is_err());
    let mut wrong_owner = signed_purpose.clone();
    wrong_owner.purpose.consent_public_key[0] ^= 1;
    assert!(NativeConsent::SendPurpose(&wrong_owner.purpose)
        .validate_at(now.saturating_mul(1000), public)
        .is_err());
    assert!(layerx_crypto::ed25519::verify_digest(
        &public,
        &wrong_owner.signature,
        &Sha256::digest(
            wrong_owner
                .purpose
                .canonical_bytes()
                .map_err(|error| format!("{error:?}"))?
        )
        .into(),
    )
    .is_err());
    let mut expired = purpose.clone();
    expired.expires_at_ms = now.saturating_mul(1000);
    assert!(NativeConsent::SendPurpose(&expired)
        .validate_at(now.saturating_mul(1000), public)
        .is_err());
    let mut foreign_key = public;
    foreign_key[0] ^= 1;
    assert!(NativeConsent::SendPurpose(&purpose)
        .validate_at(now.saturating_mul(1000), foreign_key)
        .is_err());
    let grant = NativeLocalGrantConsentV1 {
        capability: STANDARD.decode(&material.capability)?,
        session_scope: STANDARD.decode(&material.native_session)?,
        expires_at_ms: material.expiry_ms,
        owner_public_key: public,
        signature: [0; 64],
    };
    let missing = ready(custody.sign_native_consent_in_scope(
        &mut scope,
        NativeConsentRequest::new(
            &principal,
            &key,
            NativeConsent::LocalGrant(&grant),
            SignAuthorization::new(Operation::SecuritySettings, None),
            now,
            trace.clone(),
        ),
    ))?;
    assert!(matches!(missing, Err(CustodyError::StepUpRequired)));
    let signers: Vec<_> = fixture.signers.iter().map(String::as_str).collect();
    let sdk = AttestorSigner::new(client(&fixture)?, &provider_key, public, &signers, 125)?;
    let signature =
        sdk.sign_native_local_grant(&grant, "rust-genuine-grant", &fixture.second_assertion)?;
    let grant_digest: [u8; 32] = Sha256::digest(native_local_grant_signing_bytes(&grant)?).into();
    assert_eq!(signature.digest(), &grant_digest);
    ed25519_dalek::Verifier::verify(
        &ed25519_dalek::VerifyingKey::from_bytes(&public)?,
        &grant_digest,
        &ed25519_dalek::Signature::from_bytes(signature.signature()),
    )
    .map_err(|_| "native SDK grant signature verification refused")?;

    let passkeys = Passkeys::new(AuthConfig {
        rp_id: "paxportwallet.com".to_owned(),
        rp_name: "LayerX".to_owned(),
        origin: "https://paxportwallet.com".to_owned(),
        ceremony_ttl_secs: 300,
        assertion_ttl_secs: 60,
        session_ttl_secs: 300,
        refresh_ttl_secs: 3600,
        step_up_ttl_secs: 60,
        rate_limit: RateLimit {
            attempts: 100,
            window_secs: 60,
        },
    })?;
    let account = AccountIdentity::new(principal.as_str(), "Native consent qualification")?;
    let mut authenticator = Authenticator::open()?;
    let registration = passkeys.begin_registration(
        &mut scope,
        &account,
        "Disposable passkey",
        current_time(clock.as_ref())?,
    )?;
    let credential = authenticator.credential("register", &registration.ceremony)?;
    passkeys.finish_registration(
        &mut scope,
        &registration.registration_id,
        &credential,
        current_time(clock.as_ref())?,
    )?;
    let assertion = passkeys.begin_assertion(&mut scope, current_time(clock.as_ref())?)?;
    let credential = authenticator.credential("assert", &assertion.ceremony)?;
    passkeys.finish_assertion(
        &mut scope,
        &assertion.assertion_id,
        &credential,
        current_time(clock.as_ref())?,
    )?;
    let session = passkeys.open_session(
        &mut scope,
        &assertion.assertion_id,
        Device::mint("Disposable authenticator", "qualification")?,
        current_time(clock.as_ref())?,
    )?;
    let confirms = OperationDigest::new(grant_digest);
    let challenge = passkeys.begin_step_up(
        &mut scope,
        session.access_token().expose(),
        session.csrf_token().expose(),
        confirms,
        current_time(clock.as_ref())?,
    )?;
    let credential = authenticator.credential("assert", &challenge.ceremony)?;
    let mut forged: serde_json::Value =
        serde_json::from_slice(&URL_SAFE_NO_PAD.decode(&credential)?)?;
    let mut altered_signature = URL_SAFE_NO_PAD.decode(
        forged["signature"]
            .as_str()
            .ok_or("assertion signature missing")?,
    )?;
    altered_signature[0] ^= 1;
    forged["signature"] = json!(URL_SAFE_NO_PAD.encode(altered_signature));
    let forged = URL_SAFE_NO_PAD.encode(serde_json::to_vec(&forged)?);
    assert!(passkeys
        .finish_step_up(
            &mut scope,
            &challenge.challenge_id,
            &forged,
            current_time(clock.as_ref())?
        )
        .is_err());
    let authenticated = passkeys.finish_step_up(
        &mut scope,
        &challenge.challenge_id,
        &credential,
        current_time(clock.as_ref())?,
    )?;
    assert!(passkeys
        .finish_step_up(
            &mut scope,
            &challenge.challenge_id,
            &credential,
            current_time(clock.as_ref())?
        )
        .is_err());
    let capability_digest: [u8; 32] = Sha256::digest(&grant.capability).into();
    assert!(matches!(
        CustodySigner::bind_authenticated_step_up(
            &passkeys,
            &mut scope,
            &authenticated,
            OperationDigest::new(digest),
            Operation::SecuritySettings,
            grant_digest,
            capability_digest,
            current_time(clock.as_ref())?
        ),
        Err(CustodyError::InvalidEvidence)
    ));
    drop(scope);
    let mut foreign_scope = store.principal(&foreign_principal)?;
    assert!(matches!(
        CustodySigner::bind_authenticated_step_up(
            &passkeys,
            &mut foreign_scope,
            &authenticated,
            confirms,
            Operation::SecuritySettings,
            grant_digest,
            capability_digest,
            current_time(clock.as_ref())?
        ),
        Err(CustodyError::InvalidEvidence)
    ));
    drop(foreign_scope);
    let mut scope = store.principal(&principal)?;
    let evidence = CustodySigner::bind_authenticated_step_up(
        &passkeys,
        &mut scope,
        &authenticated,
        confirms,
        Operation::SecuritySettings,
        grant_digest,
        capability_digest,
        current_time(clock.as_ref())?,
    )?;
    let mut changed_grant = grant.clone();
    changed_grant.owner_public_key[0] ^= 1;
    let mismatched = ready(custody.sign_native_consent_in_scope(
        &mut scope,
        NativeConsentRequest::new(
            &principal,
            &key,
            NativeConsent::LocalGrant(&changed_grant),
            SignAuthorization::new(Operation::SecuritySettings, Some(&evidence)),
            current_time(clock.as_ref())?,
            trace.clone(),
        ),
    ))?;
    assert!(matches!(mismatched, Err(CustodyError::StepUpMismatch)));
    let expiry_skewed_grant = NativeLocalGrantConsentV1 {
        expires_at_ms: grant.expires_at_ms - 1,
        ..grant.clone()
    };
    let invalid_evidence = ready(custody.sign_native_consent_in_scope(
        &mut scope,
        NativeConsentRequest::new(
            &principal,
            &key,
            NativeConsent::LocalGrant(&expiry_skewed_grant),
            SignAuthorization::new(Operation::SecuritySettings, Some(&evidence)),
            current_time(clock.as_ref())?,
            trace.clone(),
        ),
    ))?;
    assert!(matches!(
        invalid_evidence,
        Err(CustodyError::InvalidEvidence)
    ));
    provider.admit_assertion(&fixture.owner, &fixture.second_assertion)?;
    let signed = ready(custody.sign_native_consent_in_scope(
        &mut scope,
        NativeConsentRequest::new(
            &principal,
            &key,
            NativeConsent::LocalGrant(&grant),
            SignAuthorization::new(Operation::SecuritySettings, Some(&evidence)),
            current_time(clock.as_ref())?,
            trace.clone(),
        ),
    ))??;
    assert_eq!(signed.signer_public_key(), public);
    assert_eq!(signed.disclosure_digest(), grant_digest);
    ed25519_dalek::Verifier::verify(
        &ed25519_dalek::VerifyingKey::from_bytes(&public)?,
        &grant_digest,
        &ed25519_dalek::Signature::from_bytes(signed.signature()),
    )
    .map_err(|_| "native custody grant signature verification refused")?;
    let replayed = ready(custody.sign_native_consent_in_scope(
        &mut scope,
        NativeConsentRequest::new(
            &principal,
            &key,
            NativeConsent::LocalGrant(&grant),
            SignAuthorization::new(Operation::SecuritySettings, Some(&evidence)),
            current_time(clock.as_ref())?,
            trace.clone(),
        ),
    ))?;
    assert!(matches!(replayed, Err(CustodyError::StepUpReplayed)));
    println!("native custody genuine passkey issuance and grant consent verified");
    println!("native custody principal digest forgery ceremony-replay and consumed-evidence refusals verified");
    let stranger =
        sdk.sign_native_send_purpose(&purpose, "rust-foreign-owner", &fixture.foreign_assertion);
    assert!(matches!(stranger,Err(AttestorError::Refused{code,..}) if code=="token_not_owner"));
    Ok(())
}

#[test]
fn native_send_v2_rotated_primary_key_signs_and_releases_under_stable_owner_did() -> Result<()> {
    rotated_primary_key_release_proof(|signed, _, raw| {
        let purpose = &signed.purpose;
        layerx_human_kms::attestor::validate_native_send_release(
            &signed
                .canonical_bytes()
                .map_err(|error| format!("{error:?}"))?,
            &purpose.owner_did,
            &purpose.consent_public_key,
            &purpose.evm_sender,
            raw,
        )?;
        Ok(())
    })
}

#[test]
fn native_send_v2_purpose_with_foreign_consent_key_or_sender_is_refused() -> Result<()> {
    use ed25519_dalek::Signer as _;
    use layerx_human_kms::attestor::validate_native_send_release;

    rotated_primary_key_release_proof(|signed, current, raw| {
        let owner = &signed.purpose.owner_did;
        let public = signed.purpose.consent_public_key;
        let sender = signed.purpose.evm_sender;
        let foreign = ed25519_dalek::SigningKey::from_bytes(&[0x77; 32]);
        assert_ne!(foreign.verifying_key().to_bytes(), public);
        let mut wrong_key = signed.clone();
        wrong_key.purpose.consent_public_key = foreign.verifying_key().to_bytes();
        wrong_key.signature = foreign
            .sign(&NativeConsent::SendPurpose(&wrong_key.purpose).digest()?)
            .to_bytes();
        assert!(matches!(
            NativeConsent::SendPurpose(&wrong_key.purpose).validate_at(1_000_000, public),
            Err(CustodyError::InvalidEvidence)
        ));
        assert!(wrong_key
            .purpose
            .check_current_owner(owner, &public, &sender)
            .is_err());
        assert!(validate_native_send_release(
            &wrong_key
                .canonical_bytes()
                .map_err(|error| format!("{error:?}"))?,
            owner,
            &public,
            &sender,
            raw,
        )
        .is_err());
        let mut wrong_sender = signed.clone();
        wrong_sender.purpose.evm_sender[0] ^= 1;
        wrong_sender.signature = current
            .sign(&NativeConsent::SendPurpose(&wrong_sender.purpose).digest()?)
            .to_bytes();
        assert!(wrong_sender
            .purpose
            .check_current_owner(owner, &public, &sender)
            .is_err());
        let frame = wrong_sender
            .canonical_bytes()
            .map_err(|error| format!("{error:?}"))?;
        assert!(validate_native_send_release(&frame, owner, &public, &sender, raw).is_err());
        // Even if the supplied current wallet matches, the independently recovered sender must agree.
        assert!(validate_native_send_release(
            &frame,
            owner,
            &public,
            &wrong_sender.purpose.evm_sender,
            raw,
        )
        .is_err());
        Ok(())
    })
}

fn rotated_primary_key_release_proof(
    check: impl FnOnce(&SignedNativeSendPurposeV2, &ed25519_dalek::SigningKey, &[u8]) -> Result<()>,
) -> Result<()> {
    use ed25519_dalek::Signer as _;
    use layerx_agent_api::identity::{AgentDid, CapabilityId, NativeActivity, SessionId, TenantId};
    use layerx_crypto::send::{ledger_payment_call, CallOptions, SendDebit};
    use layerx_human_service::custody::{EnvelopeKms, KeyEntropy, KmsProvider, SignRequest};
    use layerx_human_test_support::{directory, install_and_open, retention_uniform, tenancy};
    use layerx_types::intent::SendAuthorizationKind;
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration};

    let root = directory("native-send-v2-rotation");
    fs::create_dir(&root)?;
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
    let secret_path = root.join("envelope-root");
    fs::write(&secret_path, [0x42; 64])?;
    fs::set_permissions(&secret_path, fs::Permissions::from_mode(0o600))?;
    let provider = || EnvelopeKms::new("file-kms://native-send-v2", &secret_path);
    let keys = Keystore::open_development(root.join("keys"), 125, provider()?)?;
    let principal = PrincipalId::new("rotating-owner")?;
    let key = KeyId::new("primary")?;
    let original = keys.generate(
        &principal,
        &key,
        KeyClass::HumanPrimary,
        KeyEntropy::new([0x31; 32], [0x32; 16], [0x33; 24])?,
    )?;
    let owner = AgentDid::new(format!(
        "did:layerx:{}",
        original
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    ))
    .map_err(|error| format!("{error:?}"))?;
    let current = keys.rotate(&principal, &key)?;
    assert_ne!(original, current.public_key);
    assert_eq!(keys.describe(&principal, &key)?, current);
    let sender = keys.evm_wallet(&principal, &key)?;
    let seed = provider()?.export_primary_key(
        &keys.evm_binding(&principal, &key)?,
        &keys.evm_provider_reference(&principal, &key)?,
    )?;
    let consent_key = ed25519_dalek::SigningKey::from_bytes(&seed);
    assert_eq!(consent_key.verifying_key().to_bytes(), current.public_key);
    let debit = SendDebit {
        from: [0x41; 32],
        to: [0x42; 32],
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
    let authorization_digest = layerx_intents::canonical::send_authorization_bytes(&debit)
        .map_err(|error| format!("{error:?}"))?;
    let local = layerx_crypto::local::LocalSigner::new(*seed);
    let authorization = ready(layerx_crypto::signer::Signer::sign(
        &local,
        debit
            .signing_request(&authorization_digest)
            .map_err(|error| format!("{error:?}"))?,
    ))??;
    let payload = layerx_intents::canonical::signed_send_payload(
        &debit,
        current.public_key,
        *authorization.as_bytes(),
    )
    .map_err(|error| format!("{error:?}"))?;
    let prepared = ledger_payment_call(
        &payload,
        &CallOptions {
            from: sender,
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 7,
            gas_limit: 100_000,
        },
    )
    .map_err(|error| format!("{error:?}"))?;
    let canonical = prepared.call.encode_unsigned();
    let digest = Sha256::digest(&canonical).into();
    let purpose = NativeSendPurposeV2 {
        tenant: TenantId::new("tenant-a").map_err(|error| format!("{error:?}"))?,
        agent_did: owner.clone(),
        owner_did: owner.clone(),
        consent_public_key: current.public_key,
        evm_sender: sender,
        session_id: SessionId::new("51".repeat(32)).map_err(|error| format!("{error:?}"))?,
        generation: 2,
        expires_at_ms: 1_010_000,
        capability_id: CapabilityId::new("52".repeat(32)).map_err(|error| format!("{error:?}"))?,
        transaction_type: 2,
        chain_id: 125,
        activity: NativeActivity::new(1, 5).map_err(|error| format!("{error:?}"))?,
        preparation_id: digest,
        canonical_digest: digest,
        economic_action: [0x53; 32],
        idempotency_key: debit.idempotency_key,
        commitment: [0x54; 32],
    };
    NativeConsent::SendPurpose(&purpose).validate_at(1_000_000, current.public_key)?;
    let signed = SignedNativeSendPurposeV2 {
        purpose: purpose.clone(),
        signature: consent_key
            .sign(&NativeConsent::SendPurpose(&purpose).digest()?)
            .to_bytes(),
    };
    let request = layerx_agent_api::prepare::PrepareRequest {
        protocol_activity_type: layerx_human_service::server::native_send::NATIVE_SEND,
        actor: owner.clone(),
        authority: layerx_agent_api::identity::AuthorityRef::new(
            current
                .public_key
                .iter()
                .map(|byte| format!("{byte:02x}"))
                .collect::<String>(),
        )
        .map_err(|error| format!("{error:?}"))?,
        account_sequence: layerx_agent_api::Sequence(7),
        timestamp_bound: layerx_agent_api::prepare::TimestampBound {
            not_before: layerx_agent_api::TimestampSeconds(1_000),
            not_after: layerx_agent_api::TimestampSeconds(1_010),
        },
        idempotency_key: layerx_agent_api::prepare::IdempotencyRef::new(
            debit
                .idempotency_key
                .iter()
                .map(|byte| format!("{byte:02x}"))
                .collect::<String>(),
        )
        .map_err(|error| format!("{error:?}"))?,
        fee_limit: layerx_agent_api::Amount(700_000),
        payload_hash: Sha256::digest(&payload).into(),
        payload: layerx_agent_api::prepare::PayloadBytes::new(payload)
            .map_err(|error| format!("{error:?}"))?,
    };
    let wire = layerx_human_service::server::native_send::encode_native_send_prepare_v2(
        &request, &signed, None,
    )
    .map_err(|error| format!("{error:?}"))?;
    assert_eq!(wire["purpose"]["purpose"]["version"], "2");
    assert_eq!(wire["purpose"]["purpose"]["owner_did"], owner.as_str());
    assert_eq!(wire["purpose"]["purpose"]["transaction_type"], "2");
    assert_eq!(wire["purpose"]["purpose"]["chain_id"], "125");
    assert_eq!(
        wire["purpose"]["purpose"]["consent_public_key"],
        current
            .public_key
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    );
    assert_eq!(
        wire["purpose"]["purpose"]["evm_sender"],
        sender
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    );
    assert!(wire["purpose"].get("owner_public_key").is_none());
    let activity = ActivityType::new(ModuleId::Asset, 5).map_err(|error| format!("{error:?}"))?;
    let registry = ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &[activity])
        .map_err(|error| format!("{error:?}"))?])
    .map_err(|error| format!("{error:?}"))?;
    let (store, _) = install_and_open(
        &root.join("store"),
        &tenancy(&[(principal.as_str(), "tenant-a")]),
        retention_uniform(10_000),
    );
    let store = Arc::new(std::sync::Mutex::new(store));
    let custody = CustodySigner::new_shared(
        keys,
        Arc::clone(&store),
        registry.clone(),
        SigningLimits::new(32, 60)?,
    );
    let trace = TraceId::mint([0x61; 16]);
    let mut retained = store
        .lock()
        .map_err(|_| "native rotation store lock poisoned")?;
    let mut scope = retained.principal(&principal)?;
    let transaction_signature = ready(custody.sign_in_scope(
        &mut scope,
        SignRequest::new(
            &principal,
            &key,
            &trace,
            SignAuthorization::new(Operation::ProtocolMutation, None),
            &canonical,
            &prepared.disclosure,
            1_000,
        ),
    ))??;
    let transaction_signature =
        layerx_crypto::signer::AgentSignature::new(*transaction_signature.signature())
            .map_err(|error| format!("{error:?}"))?;
    let raw = layerx_crypto::evm_transaction::assemble(&prepared.call, &transaction_signature);
    let frame = signed
        .canonical_bytes()
        .map_err(|error| format!("{error:?}"))?;
    layerx_human_kms::attestor::validate_native_send_release(
        &frame,
        &owner,
        &current.public_key,
        &sender,
        &raw,
    )?;
    assert_eq!(signed.purpose.owner_did, owner);
    assert_ne!(signed.purpose.consent_public_key, original);
    assert!(NativeConsent::SendPurpose(&purpose)
        .validate_at(1_000_000, original)
        .is_err());
    assert!(layerx_human_kms::attestor::validate_native_send_release(
        &frame, &owner, &original, &sender, &raw
    )
    .is_err());
    let mut different_sender = sender;
    different_sender[0] ^= 1;
    assert!(layerx_human_kms::attestor::validate_native_send_release(
        &frame,
        &owner,
        &current.public_key,
        &different_sender,
        &raw
    )
    .is_err());
    let mut changed = signed.clone();
    changed.purpose.evm_sender = different_sender;
    changed.signature = consent_key
        .sign(&NativeConsent::SendPurpose(&changed.purpose).digest()?)
        .to_bytes();
    assert!(layerx_human_kms::attestor::validate_native_send_release(
        &changed
            .canonical_bytes()
            .map_err(|error| format!("{error:?}"))?,
        &owner,
        &current.public_key,
        &different_sender,
        &raw,
    )
    .is_err());
    check(&signed, &consent_key, &raw)?;
    drop(scope);
    drop(retained);
    drop(store);
    drop(custody);
    drop(seed);
    fs::remove_dir_all(root)?;
    Ok(())
}
