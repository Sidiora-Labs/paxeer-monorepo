#[path = "retired_crypto_codec_2.rs"]
mod retired_crypto_codec_2;

#[allow(dead_code)]
#[path = "../../layerx-human-kms/tests/support/attestor_cluster.rs"]
mod cluster;

#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

use layerx_human_test_support as support;

use std::fs;
use std::future::Future;
use std::pin::pin;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex, MutexGuard};
use std::task::{Context, Poll, Wake, Waker};
use std::time::Duration;

use k256::ecdsa::SigningKey as EvmSigningKey;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_human_service::custody::{
    EvmPlanAuthorization, EvmTransaction, KeyClass, KeyId, Keystore,
};
use layerx_human_service::journeys::{
    authority_label, intent_journey_id, plan, verify_bindings, AllowanceId, AllowanceKind,
    AllowanceScope, BalanceEntry, BindingExpectation, Constraints, Endpoint, ExecutableShape,
    FeeSchedule, IntentLegBinding, LegMechanism, Mechanism, ObservedState, PlannedLeg, Refusal,
    SignedAllowance, SubmitPlanRequest, SubmitRefusal, UnifiedIntent, UnifiedPlan,
};
use layerx_human_service::notify::JourneyId;
use layerx_human_service::server::production_components::{AttestorCustodyConfig, AttestorKms};
use layerx_human_service::store::{
    PrincipalId, PrincipalScope, PrincipalStore, RowKey, Table, TenancyDigest,
};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::AssetId;
use layerx_types::intent::{EvmAddress, TimestampSeconds};
use layerx_types::payload::{ActivityType, ModuleId};
use serde_json::{json, Value};
use sha3::{Digest as _, Keccak256};

use support::{directory, principal, retention_uniform, tenancy};

type EvmAction = layerx_human_service::custody::EvmAction;
type IntentSubmission = layerx_human_service::journeys::IntentSubmission;

// Keep the test boundary on the production entry point and check its persisted EVM result.
fn start_kernel_journey(
    scope: &mut layerx_human_service::store::PrincipalScope<'_>,
    plan: &UnifiedPlan,
    request: &SubmitPlanRequest,
    expectation: &BindingExpectation,
    journey_id: JourneyId,
    signed: &[EvmAction],
) -> Result<IntentSubmission, SubmitRefusal> {
    let result = layerx_human_service::journeys::start_kernel_journey(
        scope,
        plan,
        request,
        expectation,
        journey_id.clone(),
        signed,
    )?;
    assert_eq!(result.journey_id(), &journey_id);
    assert_eq!(result.state(), "processing");
    let key = RowKey::new(format!("intent-evm-{}", journey_id.as_str()))
        .unwrap_or_else(|error| panic!("journey key: {error}"));
    let record: Value = serde_json::from_slice(
        scope
            .get(Table::Journeys, &key)
            .unwrap_or_else(|| panic!("EVM journey record missing"))
            .bytes(),
    )
    .unwrap_or_else(|error| panic!("EVM journey record: {error}"));
    let legs = record["legs"]
        .as_array()
        .unwrap_or_else(|| panic!("EVM legs"));
    assert_eq!(legs.len(), signed.len());
    for (leg, action) in legs.iter().zip(signed) {
        let hash = layerx_crypto::evm_transaction::transaction_hash(&action.raw_transaction);
        let (call, signature) =
            layerx_crypto::evm_transaction::decode_signed(&action.raw_transaction)
                .unwrap_or_else(|error| panic!("persisted journey transaction: {error:?}"));
        assert_eq!(call.nonce, action.authorization.transaction.nonce);
        assert_eq!(call.chain_id, action.authorization.transaction.chain_id);
        assert_eq!(call.data, action.authorization.transaction.calldata);
        assert_eq!(
            layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                .unwrap_or_else(|error| panic!("journey transaction signer: {error:?}")),
            action.authorization.wallet
        );
        assert_eq!(leg["tx_hash"], format!("0x{}", hex(&hash)));
        assert_eq!(leg["nonce"], action.authorization.transaction.nonce);
    }
    Ok(result)
}

const NETWORK_ID: u32 = 125;
const ASSET: [u8; 32] = [0x33; 32];
const HOME: &str = "agent:did:layerx:human:main";
const WORKER: &str = "agent:did:layerx:worker:main";
const SEQUENCE: u64 = 7;
const SEND_FEE: u128 = 2;
const MAX_FEE: u128 = 20;

struct NoopWake;

impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    let mut context = Context::from_waker(&waker);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("intent journey future unexpectedly blocked"),
    }
}

fn asset_send() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 5)
        .unwrap_or_else(|error| panic!("asset activity: {error:?}"))
}

fn account(value: &str) -> AccountId {
    AccountId::parse(value).unwrap_or_else(|error| panic!("account: {error:?}"))
}

fn actor() -> AgentDid {
    AgentDid::new("did:layerx:owner").unwrap_or_else(|error| panic!("actor: {error:?}"))
}

fn authority() -> AuthorityRef {
    AuthorityRef::new("custody-human-primary")
        .unwrap_or_else(|error| panic!("authority: {error:?}"))
}

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    bytes.iter().fold(String::new(), |mut output, byte| {
        let _ = write!(output, "{byte:02x}");
        output
    })
}

fn evm_address(key: &EvmSigningKey) -> EvmAddress {
    let point = key.verifying_key().to_encoded_point(false);
    let hash = Keccak256::digest(&point.as_bytes()[1..]);
    let mut address = [0_u8; 20];
    address.copy_from_slice(&hash[12..]);
    EvmAddress::new(address)
}

fn fees() -> FeeSchedule {
    FeeSchedule::new(vec![(LegMechanism::Protocol(Mechanism::Send), SEND_FEE)])
        .unwrap_or_else(|error| panic!("fees: {error:?}"))
}

fn observed(now: u64) -> ObservedState {
    let home = Endpoint::human(account(HOME)).unwrap_or_else(|error| panic!("home: {error:?}"));
    ObservedState::new(
        TimestampSeconds::from_u64(now),
        account(HOME),
        None,
        vec![BalanceEntry::new(
            home,
            AssetId::new(ASSET),
            Amount::from_u128(500),
        )],
        Vec::new(),
        Vec::new(),
        fees(),
    )
    .unwrap_or_else(|error| panic!("observed state: {error:?}"))
}

fn planned(
    source: Endpoint,
    destination: Endpoint,
    amount: u128,
    observed: &ObservedState,
    now: u64,
) -> UnifiedPlan {
    let intent = UnifiedIntent::new(
        source,
        destination,
        AssetId::new(ASSET),
        Amount::from_u128(amount),
        Constraints::new(TimestampSeconds::from_u64(now + 300), MAX_FEE, false),
    )
    .unwrap_or_else(|error| panic!("intent: {error:?}"));
    plan(&intent, observed).unwrap_or_else(|error| panic!("plan: {error:?}"))
}

fn kernel_plan(now: u64) -> UnifiedPlan {
    planned(
        Endpoint::human(account(HOME)).unwrap_or_else(|error| panic!("home: {error:?}")),
        Endpoint::agent(account(WORKER)).unwrap_or_else(|error| panic!("worker: {error:?}")),
        90,
        &observed(now),
        now,
    )
}

fn binding_document(plan: &UnifiedPlan, index: usize, sequence: u64, window: (u64, u64)) -> Value {
    let leg = &plan.legs()[index];
    json!({
        "leg_index": index,
        "action_key": hex(&plan.action_key(index).unwrap_or_else(|error| panic!("action key: {error:?}"))),
        "actor": actor().as_str(),
        "authority": authority_label(leg.authority()),
        "relationship": "self",
        "account_sequence": sequence,
        "not_before": window.0,
        "not_after": window.1,
        "fee_limit": {"amount": leg.fee().saturating_add(1).to_string(), "currency": "LXP"},
    })
}

fn submission_body(plan: &UnifiedPlan, digest: [u8; 32], bindings: Vec<Value>) -> Value {
    json!({
        "plan_digest": hex(&plan.digest()),
        "signed_digest": hex(&digest),
        "bindings": bindings,
    })
}

fn valid_request(plan: &UnifiedPlan, window: (u64, u64)) -> SubmitPlanRequest {
    let bindings = (0..plan.legs().len())
        .map(|index| binding_document(plan, index, SEQUENCE, window))
        .collect();
    SubmitPlanRequest::from_json(&submission_body(plan, plan.digest(), bindings))
        .unwrap_or_else(|error| panic!("submission: {error}"))
}

fn expectation(now: u64) -> BindingExpectation {
    BindingExpectation {
        actor: actor(),
        authority: authority(),
        account_sequence: SEQUENCE,
        currency: "LXP".to_owned(),
        now,
    }
}

fn raw_transaction(call: &layerx_crypto::evm_transaction::Eip1559Call, from: [u8; 20]) -> Vec<u8> {
    use layerx_crypto::account_derivation::derive_from_seed;
    use layerx_crypto::disclosure::call::bind_call;
    use layerx_crypto::local::LocalSigner;
    use layerx_crypto::signer::sign_disclosed_call;

    let account = derive_from_seed(&[0xa5; 32], 0)
        .unwrap_or_else(|error| panic!("transaction account: {error:?}"));
    let signer = LocalSigner::from_derived_account(&account);
    assert_ne!(account.evm_address(), from);
    let disclosure =
        bind_call(call, from).unwrap_or_else(|error| panic!("call disclosure: {error:?}"));
    let signature = ready(sign_disclosed_call(&signer, call, &disclosure, from))
        .unwrap_or_else(|error| panic!("sign transaction: {error:?}"));
    layerx_crypto::evm_transaction::assemble(call, &signature)
}

struct Fixture {
    root: std::path::PathBuf,
    store_root: std::path::PathBuf,
    tenancy_digest: TenancyDigest,
    principal: PrincipalId,
    keystore: Keystore,
    kms: AttestorKms,
    config: AttestorCustodyConfig,
    _cluster: cluster::Cluster,
    _clock: SigningClock,
    _environment: MutexGuard<'static, ()>,
    now: u64,
}

impl Fixture {
    fn new(label: &str) -> Self {
        let environment = SIGNING_ENVIRONMENT
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        let root = directory(label);
        fs::create_dir_all(&root).unwrap_or_else(|error| panic!("fixture root: {error}"));
        let clock = SigningClock::start(&root);
        let store_root = root.join("human-store");
        let tenancy_digest = tenancy(&[("owner", "tenant-a")])
            .install(&store_root)
            .unwrap_or_else(|error| panic!("tenancy: {error}"));
        let principal = principal("owner");
        let mut cluster = cluster::Cluster::start(&[asset_send().value().to_string()])
            .unwrap_or_else(|error| panic!("attestor cluster: {error}"));
        let config = AttestorCustodyConfig::new(
            cluster.nodes.clone(),
            cluster::SIGNERS.iter().map(|id| (*id).to_owned()).collect(),
            cluster
                .pki
                .certificate_der("node-ca")
                .unwrap_or_else(|error| panic!("root certificate: {error}")),
            cluster
                .pki
                .certificate_der("gateway")
                .unwrap_or_else(|error| panic!("client certificate: {error}")),
            cluster
                .pki
                .key_der("gateway")
                .unwrap_or_else(|error| panic!("client key: {error}")),
            Duration::from_secs(120),
        )
        .unwrap_or_else(|error| panic!("attestor config: {error}"));
        let kms = AttestorKms::connect(config.clone())
            .unwrap_or_else(|error| panic!("attestor custody: {error}"));
        let keystore = Keystore::open_production(root.join("evm-custody"), NETWORK_ID, kms.clone())
            .unwrap_or_else(|error| panic!("EVM keystore: {error}"));
        let owner = "intent-owner";
        let wallet_public = cluster
            .generate_key("intent-wallet", "secp256k1", owner, "")
            .unwrap_or_else(|error| panic!("wallet generation: {error}"));
        let wallet = layerx_crypto::secp256k1::evm_address(&wallet_public)
            .unwrap_or_else(|error| panic!("wallet address: {error:?}"));
        let binding = keystore
            .binding(&principal, &custody_key(), KeyClass::HumanPrimary)
            .unwrap_or_else(|error| panic!("custody binding: {error}"));
        let key_id = format!("lx-{}", hex(&binding.digest()[..16]));
        cluster
            .approve_generate(&key_id, "ed25519", owner)
            .unwrap_or_else(|error| panic!("primary key approval: {error}"));
        let public = kms
            .create_owned_key(
                &keystore,
                &principal,
                &custody_key(),
                owner,
                &format!("0x{}", hex(&wallet)),
            )
            .unwrap_or_else(|error| panic!("primary key: {error}"));
        cluster
            .approve_sign(&key_id, &public)
            .unwrap_or_else(|error| panic!("signing approval: {error}"));
        let assertion = cluster
            .tokens
            .mint(owner)
            .unwrap_or_else(|error| panic!("identity assertion: {error}"));
        kms.admit_assertion(owner, &assertion)
            .unwrap_or_else(|error| panic!("admit assertion: {error}"));
        use layerx_types::clock::Clock as _;
        let now = layerx_client::runtime_clock::RuntimeClock::from_environment()
            .unwrap_or_else(|error| panic!("runtime clock: {error:?}"))
            .sample(Duration::from_secs(1))
            .unwrap_or_else(|error| panic!("clock sample: {error:?}"))
            .unix_seconds();
        Self {
            root,
            store_root,
            tenancy_digest,
            principal,
            keystore,
            kms,
            config,
            _cluster: cluster,
            _clock: clock,
            _environment: environment,
            now,
        }
    }

    fn store(&self) -> PrincipalStore {
        PrincipalStore::open(
            &self.store_root,
            retention_uniform(10_000),
            self.tenancy_digest,
        )
        .unwrap_or_else(|error| panic!("principal store: {error}"))
    }

    fn reopen_custody(&mut self) {
        self.kms = AttestorKms::connect(self.config.clone())
            .unwrap_or_else(|error| panic!("reopen custody: {error}"));
        self.keystore =
            Keystore::open_production(self.root.join("evm-custody"), NETWORK_ID, self.kms.clone())
                .unwrap_or_else(|error| panic!("reopen keystore: {error}"));
    }

    fn authorize_send_transaction(
        &self,
        scope: &PrincipalScope<'_>,
        plan: &UnifiedPlan,
        request: &SubmitPlanRequest,
    ) -> Vec<EvmAction> {
        let assertion = self
            ._cluster
            .tokens
            .mint("intent-owner")
            .unwrap_or_else(|error| panic!("identity assertion: {error}"));
        self.kms
            .admit_assertion("intent-owner", &assertion)
            .unwrap_or_else(|error| panic!("admit assertion: {error}"));
        let wallet = self
            .keystore
            .evm_wallet(&self.principal, &custody_key())
            .unwrap_or_else(|error| panic!("wallet: {error}"));
        plan.legs()
            .iter()
            .zip(&request.bindings)
            .map(|(leg, binding)| {
                let recipient = evm_address(
                    &EvmSigningKey::from_slice(&[0x6b; 32])
                        .unwrap_or_else(|error| panic!("recipient key: {error}")),
                )
                .bytes();
                let mut source = [0; 32];
                source[12..].copy_from_slice(&wallet);
                let mut destination = [0; 32];
                destination[12..].copy_from_slice(&recipient);
                let word = |amount: u128| {
                    let mut bytes = [0; 32];
                    bytes[16..].copy_from_slice(&amount.to_be_bytes());
                    bytes
                };
                let denom = "LXP";
                let mut data = [
                    vec![0x5c, 0x05, 0x96, 0x1b],
                    source.to_vec(),
                    destination.to_vec(),
                    word(128).to_vec(),
                    word(leg.amount().value()).to_vec(),
                    word(denom.len() as u128).to_vec(),
                    denom.as_bytes().to_vec(),
                ]
                .concat();
                data.resize(4 + (data.len() - 4).div_ceil(32) * 32, 0);
                let mut to = [0; 20];
                to[18..].copy_from_slice(&0x1001_u16.to_be_bytes());
                let call = layerx_crypto::evm_transaction::Eip1559Call {
                    chain_id: u64::from(NETWORK_ID),
                    nonce: binding.account_sequence,
                    max_priority_fee_per_gas: 1,
                    max_fee_per_gas: 10,
                    gas_limit: 100_000,
                    to,
                    data,
                };
                let authorization = EvmPlanAuthorization {
                    plan_id: plan.digest(),
                    action_key: binding.action_key,
                    tenant: scope.tenant().as_str().to_owned(),
                    principal: scope.principal().as_str().to_owned(),
                    binding_digest: self
                        .keystore
                        .evm_binding(&self.principal, &custody_key())
                        .unwrap_or_else(|error| panic!("custody binding: {error}"))
                        .digest(),
                    wallet,
                    not_before: binding.not_before,
                    not_after: binding.not_after,
                    transaction: EvmTransaction {
                        chain_id: call.chain_id,
                        nonce: call.nonce,
                        max_priority_fee_per_gas: 1,
                        max_fee_per_gas: 10,
                        gas_limit: call.gas_limit,
                        to: call.to,
                        value: [0; 32],
                        calldata: call.data,
                    },
                };
                let raw_transaction = self
                    .keystore
                    .authorize_send_transaction(
                        &self.principal,
                        &custody_key(),
                        &authorization,
                        recipient,
                        denom.to_owned(),
                        word(leg.amount().value()),
                    )
                    .unwrap_or_else(|error| {
                        panic!(
                            "authorize send: {error}; refusal: {:?}",
                            self.kms.last_refusal_code()
                        )
                    });
                EvmAction {
                    authorization,
                    transaction_hash: Some(layerx_crypto::evm_transaction::transaction_hash(
                        &raw_transaction,
                    )),
                    raw_transaction,
                    acknowledged: false,
                }
            })
            .collect()
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

static SIGNING_ENVIRONMENT: Mutex<()> = Mutex::new(());

struct SigningClock {
    original: Vec<(&'static str, Option<std::ffi::OsString>)>,
    shutdown: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<std::io::Result<()>>>,
}
impl SigningClock {
    fn start(root: &std::path::Path) -> Self {
        let mut server = runtime_clock_server::Server::bind(root)
            .unwrap_or_else(|error| panic!("signing clock: {error}"));
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
        let original = [
            ("LAYERX_RUNTIME_CLOCK_SOCKET", socket.into_os_string()),
            (
                "LAYERX_RUNTIME_CLOCK_PID",
                std::process::id().to_string().into(),
            ),
            (
                "LAYERX_RUNTIME_CLOCK_UID",
                rustix::process::getuid().as_raw().to_string().into(),
            ),
        ]
        .into_iter()
        .map(|(name, value)| {
            let previous = std::env::var_os(name);
            std::env::set_var(name, value);
            (name, previous)
        })
        .collect();
        Self {
            original,
            shutdown,
            worker: Some(worker),
        }
    }
}

impl Drop for SigningClock {
    fn drop(&mut self) {
        for (name, value) in self.original.iter().rev() {
            match value {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker
                .join()
                .unwrap_or_else(|_| panic!("signing clock panicked"))
                .unwrap_or_else(|error| panic!("signing clock: {error}"));
        }
    }
}

fn custody_key() -> KeyId {
    KeyId::new("human-primary").unwrap_or_else(|error| panic!("key: {error}"))
}

#[test]
fn intent_submit_kernel_send_persists_a_processing_record_with_each_leg_hash() {
    let mut fixture = Fixture::new("intent-submit-kernel");
    let now = fixture.now;
    let plan = kernel_plan(now);
    assert_eq!(
        plan.legs()[0].mechanism(),
        LegMechanism::Protocol(Mechanism::Send)
    );
    let request = valid_request(&plan, (now - 5, now + 200));
    let expectation = expectation(now);
    verify_bindings(&plan, &request, &expectation)
        .unwrap_or_else(|error| panic!("bindings: {error}"));
    let journey_id = intent_journey_id("kernel-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let actions = fixture.authorize_send_transaction(&scope, &plan, &request);
    let mut wrong_signer = actions.clone();
    let (call, _) = layerx_crypto::evm_transaction::decode_signed(&actions[0].raw_transaction)
        .unwrap_or_else(|error| panic!("signed call: {error:?}"));
    wrong_signer[0].raw_transaction = raw_transaction(&call, actions[0].authorization.wallet);
    wrong_signer[0].transaction_hash = Some(layerx_crypto::evm_transaction::transaction_hash(
        &wrong_signer[0].raw_transaction,
    ));
    assert_eq!(
        start_kernel_journey(
            &mut scope,
            &plan,
            &request,
            &expectation,
            journey_id.clone(),
            &wrong_signer
        )
        .err(),
        Some(SubmitRefusal::LegMismatch { index: 0 })
    );
    let absent = RowKey::new(format!("intent-evm-{}", journey_id.as_str()))
        .unwrap_or_else(|error| panic!("journey key: {error}"));
    assert!(scope.get(Table::Journeys, &absent).is_none());
    let initial = start_kernel_journey(
        &mut scope,
        &plan,
        &request,
        &expectation,
        journey_id.clone(),
        &actions,
    )
    .unwrap_or_else(|error| panic!("start: {error}"));
    assert_eq!(initial.to_json()["state"], "processing");
    assert_eq!(initial.journey_id(), &journey_id);
    let key = RowKey::new(format!("intent-evm-{}", journey_id.as_str()))
        .unwrap_or_else(|error| panic!("journey key: {error}"));
    let record: Value = serde_json::from_slice(
        scope
            .get(Table::Journeys, &key)
            .unwrap_or_else(|| panic!("persisted kernel journey missing"))
            .bytes(),
    )
    .unwrap_or_else(|error| panic!("journey record: {error}"));
    assert_eq!(record["journey_id"], journey_id.as_str());
    assert_eq!(record["plan_digest"], hex(&plan.digest()));
    assert_eq!(record["state"], "processing");
    assert_eq!(record["state_copy_key"], "journey.state.processing");
    let legs = record["legs"]
        .as_array()
        .unwrap_or_else(|| panic!("persisted journey legs missing"));
    assert_eq!(legs.len(), actions.len());
    for (index, (leg, action)) in legs.iter().zip(&actions).enumerate() {
        assert_eq!(leg["index"], json!(index));
        assert_eq!(leg["nonce"], action.authorization.transaction.nonce);
        assert_eq!(
            leg["tx_hash"],
            format!(
                "0x{}",
                hex(&action
                    .transaction_hash
                    .unwrap_or_else(|| panic!("signed transaction hash missing")))
            )
        );
        assert_eq!(leg["action_key"], hex(&action.authorization.action_key));
        assert_eq!(leg["raw_transaction"], json!(action.raw_transaction));
    }

    // Task 14.0.6: removed the kernel-native journey drive, which production does not use.
    drop(scope);
    drop(store);
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("reopened scope: {error}"));
    let repeated = start_kernel_journey(
        &mut scope,
        &plan,
        &request,
        &expectation,
        journey_id.clone(),
        &actions,
    )
    .unwrap_or_else(|error| panic!("repeat: {error}"));
    assert_eq!(repeated.journey_id(), &journey_id);
    assert_eq!(repeated.to_json()["state"], "processing");

    let mut conflicting_request = request.clone();
    conflicting_request.bindings[0].account_sequence += 1;
    let mut conflicting_expectation = expectation.clone();
    conflicting_expectation.account_sequence += 1;
    fixture.reopen_custody();
    let conflicting_actions =
        fixture.authorize_send_transaction(&scope, &plan, &conflicting_request);
    assert_ne!(
        conflicting_actions[0].authorization.transaction.nonce,
        actions[0].authorization.transaction.nonce
    );
    assert_eq!(
        start_kernel_journey(
            &mut scope,
            &plan,
            &conflicting_request,
            &conflicting_expectation,
            journey_id.clone(),
            &conflicting_actions,
        )
        .err(),
        Some(SubmitRefusal::AlreadySubmitted)
    );
    let retained: Value = serde_json::from_slice(
        scope
            .get(Table::Journeys, &key)
            .unwrap_or_else(|| panic!("persisted kernel journey missing after retry"))
            .bytes(),
    )
    .unwrap_or_else(|error| panic!("retained journey record: {error}"));
    assert_eq!(retained, record);
}

#[test]
fn intent_submit_refuses_a_stale_digest_with_a_typed_forbidden_code() {
    let fixture = Fixture::new("intent-submit-stale");
    let now = fixture.now;
    let plan = kernel_plan(now);
    let mut stale = plan.digest();
    stale[0] ^= 0x01;
    let bindings = vec![binding_document(&plan, 0, SEQUENCE, (now - 5, now + 200))];
    let request = SubmitPlanRequest::from_json(&submission_body(&plan, stale, bindings))
        .unwrap_or_else(|error| panic!("submission: {error}"));
    let refusal = verify_bindings(&plan, &request, &expectation(now))
        .err()
        .unwrap_or_else(|| panic!("stale digest accepted"));
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let journey_id = intent_journey_id("stale-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let actions = fixture.authorize_send_transaction(&scope, &plan, &request);
    assert_eq!(
        start_kernel_journey(
            &mut scope,
            &plan,
            &request,
            &expectation(now),
            journey_id.clone(),
            &actions
        )
        .err(),
        Some(SubmitRefusal::SignedDigestMismatch)
    );
    let key = RowKey::new(format!("intent-evm-{}", journey_id.as_str()))
        .unwrap_or_else(|error| panic!("journey key: {error}"));
    assert!(scope.get(Table::Journeys, &key).is_none());
    assert_eq!(refusal, SubmitRefusal::SignedDigestMismatch);
    assert_eq!(refusal.status(), 403);
    assert_eq!(refusal.code(), "forbidden");
    assert_eq!(refusal.copy_key(), "error.request.forbidden");
    assert_eq!(refusal.retry(), "final");

    let mut body = submission_body(
        &plan,
        plan.digest(),
        vec![binding_document(&plan, 0, SEQUENCE, (now - 5, now + 200))],
    );
    body["plan_digest"] = json!(hex(&stale));
    let request =
        SubmitPlanRequest::from_json(&body).unwrap_or_else(|error| panic!("submission: {error}"));
    assert_eq!(
        verify_bindings(&plan, &request, &expectation(now)).err(),
        Some(SubmitRefusal::PlanDigestMismatch)
    );
}

#[test]
fn intent_submit_refuses_a_wrong_account_sequence_with_a_typed_conflict() {
    let fixture = Fixture::new("intent-submit-sequence");
    let now = fixture.now;
    let plan = kernel_plan(now);
    let bindings = vec![binding_document(
        &plan,
        0,
        SEQUENCE + 1,
        (now - 5, now + 200),
    )];
    let request = SubmitPlanRequest::from_json(&submission_body(&plan, plan.digest(), bindings))
        .unwrap_or_else(|error| panic!("submission: {error}"));
    let refusal = verify_bindings(&plan, &request, &expectation(now))
        .err()
        .unwrap_or_else(|| panic!("wrong sequence accepted"));
    assert_eq!(refusal, SubmitRefusal::StaleSequence { index: 0 });
    assert_eq!(refusal.status(), 409);
    assert_eq!(refusal.code(), "conflict");
    assert_eq!(refusal.copy_key(), "error.move.quote-expired");

    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let journey_id = intent_journey_id("sequence-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let actions = fixture.authorize_send_transaction(&scope, &plan, &request);
    assert_eq!(
        start_kernel_journey(
            &mut scope,
            &plan,
            &request,
            &expectation(now),
            journey_id.clone(),
            &actions,
        )
        .err(),
        Some(SubmitRefusal::StaleSequence { index: 0 })
    );
    let key = RowKey::new(format!("intent-evm-{}", journey_id.as_str()))
        .unwrap_or_else(|error| panic!("journey key: {error}"));
    assert!(scope.get(Table::Journeys, &key).is_none());
}

#[test]
fn intent_submit_refuses_an_expired_validity_window_with_a_typed_quote_expiry() {
    let fixture = Fixture::new("intent-submit-expired");
    let now = fixture.now;
    let plan = kernel_plan(now);
    let request = valid_request(&plan, (now - 5, now + 200));
    let refusal = verify_bindings(&plan, &request, &expectation(now + 201))
        .err()
        .unwrap_or_else(|| panic!("expired window accepted"));
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let journey_id = intent_journey_id("expired-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let actions = fixture.authorize_send_transaction(&scope, &plan, &request);
    assert_eq!(
        start_kernel_journey(
            &mut scope,
            &plan,
            &request,
            &expectation(now + 201),
            journey_id.clone(),
            &actions
        )
        .err(),
        Some(SubmitRefusal::WindowExpired { index: 0 })
    );
    let key = RowKey::new(format!("intent-evm-{}", journey_id.as_str()))
        .unwrap_or_else(|error| panic!("journey key: {error}"));
    assert!(scope.get(Table::Journeys, &key).is_none());
    assert_eq!(refusal, SubmitRefusal::WindowExpired { index: 0 });
    assert_eq!(refusal.status(), 409);
    assert_eq!(refusal.code(), "quote-expired");
    assert_eq!(refusal.copy_key(), "error.move.quote-expired");
    assert_eq!(refusal.retry(), "structural");

    let mut forged = request.clone();
    forged.bindings[0].actor = "did:layerx:intruder".to_owned();
    assert_eq!(
        verify_bindings(&plan, &forged, &expectation(now)).err(),
        Some(SubmitRefusal::LegMismatch { index: 0 })
    );
    let mut greedy = request;
    greedy.bindings[0].fee_limit = MAX_FEE + 1;
    assert_eq!(
        verify_bindings(&plan, &greedy, &expectation(now)).err(),
        Some(SubmitRefusal::FeeLimitMismatch { index: 0 })
    );
}

#[test]
fn intent_submit_response_matches_the_golden_field_set() {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../schema/human-api/golden/intent.submit.response.json");
    let golden: Value = serde_json::from_str(
        &fs::read_to_string(&path).unwrap_or_else(|error| panic!("golden: {error}")),
    )
    .unwrap_or_else(|error| panic!("golden json: {error}"));
    let expected = golden["body"]["result"]
        .as_object()
        .unwrap_or_else(|| panic!("golden result"))
        .clone();

    let fixture = Fixture::new("intent-submit-golden");
    let now = fixture.now;
    let plan = kernel_plan(now);
    let request = valid_request(&plan, (now - 5, now + 200));
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let actions = fixture.authorize_send_transaction(&scope, &plan, &request);
    let journey_id = intent_journey_id("golden-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let submission = start_kernel_journey(
        &mut scope,
        &plan,
        &request,
        &expectation(now),
        journey_id.clone(),
        &actions,
    )
    .unwrap_or_else(|error| panic!("start: {error}"));
    let response = submission.to_json();
    assert_eq!(response["state"], "processing");
    let actual = response
        .as_object()
        .unwrap_or_else(|| panic!("response object"));
    assert_eq!(
        actual.keys().collect::<Vec<_>>(),
        expected.keys().collect::<Vec<_>>()
    );
    for (name, value) in &expected {
        let decoded = &actual[name];
        assert_eq!(
            std::mem::discriminant(decoded),
            std::mem::discriminant(value),
            "field {name} has a different JSON type"
        );
    }
    let journey_id = actual["journey_id"].as_str().unwrap_or_default();
    assert!(journey_id.starts_with("jrn_") && journey_id.len() > 4);
    assert!(journey_id[4..]
        .bytes()
        .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit()));
    assert_eq!(actual["plan_digest"], json!(hex(&plan.digest())));
    assert_eq!(
        actual["state_copy_key"],
        json!(format!(
            "status.{}",
            actual["state"].as_str().unwrap_or_default()
        ))
    );
    let binding =
        IntentLegBinding::from_json(&binding_document(&plan, 0, SEQUENCE, (now - 5, now + 200)))
            .unwrap_or_else(|error| panic!("binding: {error}"));
    assert_eq!(binding.fee_currency, "LXP");
}

fn home() -> Endpoint {
    Endpoint::human(account(HOME)).unwrap_or_else(|error| panic!("home: {error:?}"))
}

fn worker() -> Endpoint {
    Endpoint::agent(account(WORKER)).unwrap_or_else(|error| panic!("worker: {error:?}"))
}

fn mechanisms(plan: &UnifiedPlan) -> Vec<LegMechanism> {
    plan.legs().iter().map(PlannedLeg::mechanism).collect()
}

#[test]
fn unified_shape_planner_and_submit_classifier_share_one_contract() {
    let send = LegMechanism::Protocol(Mechanism::Send);
    let cases = [(
        kernel_plan(1_000),
        ExecutableShape::Kernel,
        vec![send],
        (0, 1),
    )];
    let mut labels = std::collections::BTreeSet::new();
    for (plan, shape, expected, kernel) in &cases {
        assert_eq!(&mechanisms(plan), expected, "{}", shape.label());
        assert_eq!(plan.executable_shape(), *shape);
        assert_eq!(ExecutableShape::classify(plan.legs()), Ok(*shape));
        assert_eq!(ExecutableShape::of_mechanisms(expected), Some(*shape));
        assert_eq!(shape.kernel_legs(plan.legs().len()), *kernel);
        for (index, leg) in plan.legs().iter().enumerate() {
            assert_eq!(leg.index(), index);
            assert_eq!(
                ExecutableShape::is_kernel(leg.mechanism()),
                (kernel.0..kernel.1).contains(&index),
                "{} leg {index}",
                shape.label()
            );
        }
        labels.insert(shape.label());
    }
    assert_eq!(labels.len(), cases.len());
}

#[test]
fn unified_shape_refuses_unsupported_reordered_and_discontinuous_legs() {
    let send = LegMechanism::Protocol(Mechanism::Send);
    for combination in [
        vec![],
        vec![LegMechanism::Protocol(Mechanism::BudgetCreate)],
        vec![
            send,
            LegMechanism::Protocol(Mechanism::ReceiveUnderPayerGrant),
        ],
    ] {
        assert_eq!(
            ExecutableShape::of_mechanisms(&combination),
            None,
            "{combination:?}"
        );
    }

    let topped_up = UnifiedIntent::new(
        worker(),
        home(),
        AssetId::new(ASSET),
        Amount::from_u128(90),
        Constraints::new(TimestampSeconds::from_u64(1_200), MAX_FEE, true),
    )
    .unwrap_or_else(|error| panic!("intent: {error:?}"));
    assert_eq!(
        plan(&topped_up, &observed(1_000)).err(),
        Some(Refusal::TopUpNotAuthorized { shortfall: 92 })
    );
    let allowance = SignedAllowance::new(
        AllowanceId::new([0x61; 32]).unwrap_or_else(|error| panic!("allowance id: {error}")),
        AllowanceKind::BudgetAllowance,
        AllowanceScope::new(home(), worker(), AssetId::new(ASSET), send),
        Amount::from_u128(1_000),
        Amount::from_u128(1_000),
        TimestampSeconds::from_u64(5_000),
    )
    .unwrap_or_else(|error| panic!("allowance: {error}"));
    let authorised = ObservedState::new(
        TimestampSeconds::from_u64(1_000),
        account(HOME),
        None,
        vec![BalanceEntry::new(
            home(),
            AssetId::new(ASSET),
            Amount::from_u128(500),
        )],
        vec![allowance],
        Vec::new(),
        fees(),
    )
    .unwrap_or_else(|error| panic!("authorised state: {error:?}"));
    let forward =
        plan(&topped_up, &authorised).unwrap_or_else(|error| panic!("send top-up: {error:?}"));
    assert_eq!(mechanisms(&forward), vec![send, send]);
    assert_eq!(
        ExecutableShape::classify(forward.legs()),
        Ok(ExecutableShape::Kernel)
    );
    let mut reordered = forward.legs().to_vec();
    reordered.swap(0, 1);
    assert_eq!(
        ExecutableShape::classify(&reordered),
        Err(Refusal::UnsupportedShape)
    );
    assert_eq!(
        ExecutableShape::classify(&forward.legs()[1..]),
        Err(Refusal::UnsupportedShape)
    );
    let mut broken = forward.legs().to_vec();
    broken[0] = planned(
        home(),
        Endpoint::agent(account("agent:did:layerx:other:main"))
            .unwrap_or_else(|error| panic!("other agent: {error:?}")),
        25,
        &observed(1_000),
        1_000,
    )
    .legs()[0]
        .clone();
    assert_eq!(
        ExecutableShape::of_mechanisms(&mechanisms(&forward)),
        Some(ExecutableShape::Kernel)
    );
    assert_eq!(
        ExecutableShape::classify(&broken),
        Err(Refusal::UnsupportedShape)
    );
}
