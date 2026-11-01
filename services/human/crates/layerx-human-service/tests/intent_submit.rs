#[path = "retired_crypto_codec_2.rs"]
mod retired_crypto_codec_2;

use layerx_human_test_support as support;

use std::fs;
use std::task::{Context, Poll, Waker};

use k256::ecdsa::SigningKey as EvmSigningKey;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_human_service::custody::{
    CustodySigner, EnvelopeKms, EvmAction, EvmPlanAuthorization, EvmTransaction, KeyClass,
    KeyEntropy, KeyId, Keystore, SigningLimits,
};
use layerx_human_service::journeys::{
    authority_label, intent_journey_id, plan, start_kernel_journey, verify_bindings, BalanceEntry,
    BindingExpectation, Constraints, Endpoint, FeeSchedule, IntentLegBinding, LegMechanism,
    Mechanism, ObservedState, SubmitPlanRequest, SubmitRefusal, UnifiedIntent, UnifiedPlan,
};
use layerx_human_service::store::{
    PrincipalId, PrincipalScope, PrincipalStore, RowKey, Table, TenancyDigest,
};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::AssetId;
use layerx_types::intent::{EvmAddress, TimestampSeconds};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use serde_json::{json, Value};
use sha3::{Digest as _, Keccak256};

use support::{directory, principal, retention_uniform, tenancy};

const NETWORK_ID: u32 = 77;
const ASSET: [u8; 32] = [0x33; 32];
const HOME: &str = "agent:did:layerx:human:main";
const WORKER: &str = "agent:did:layerx:worker:main";
const SEQUENCE: u64 = 7;
const SEND_FEE: u128 = 2;
const MAX_FEE: u128 = 20;

fn asset_send() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 5)
        .unwrap_or_else(|error| panic!("asset activity: {error:?}"))
}

fn registry() -> ModuleRegistry {
    let asset = ModuleRegistration::new(ModuleId::Asset, &[asset_send()])
        .unwrap_or_else(|error| panic!("asset registration: {error:?}"));
    let governance = ActivityType::new(ModuleId::Governance, 4)
        .unwrap_or_else(|error| panic!("governance activity: {error:?}"));
    let bridge = ActivityType::new(ModuleId::Bridge, 1)
        .unwrap_or_else(|error| panic!("bridge activity: {error:?}"));
    ModuleRegistry::new(&[
        asset,
        ModuleRegistration::new(ModuleId::Governance, &[governance])
            .unwrap_or_else(|error| panic!("governance registration: {error:?}")),
        ModuleRegistration::new(ModuleId::Bridge, &[bridge])
            .unwrap_or_else(|error| panic!("bridge registration: {error:?}")),
    ])
    .unwrap_or_else(|error| panic!("module registry: {error:?}"))
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
) -> UnifiedPlan {
    let intent = UnifiedIntent::new(
        source,
        destination,
        AssetId::new(ASSET),
        Amount::from_u128(amount),
        Constraints::new(TimestampSeconds::from_u64(1_200), MAX_FEE, false),
    )
    .unwrap_or_else(|error| panic!("intent: {error:?}"));
    plan(&intent, observed).unwrap_or_else(|error| panic!("plan: {error:?}"))
}

fn kernel_plan() -> UnifiedPlan {
    planned(
        Endpoint::human(account(HOME)).unwrap_or_else(|error| panic!("home: {error:?}")),
        Endpoint::agent(account(WORKER)).unwrap_or_else(|error| panic!("worker: {error:?}")),
        90,
        &observed(1_000),
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

struct Fixture {
    root: std::path::PathBuf,
    store_root: std::path::PathBuf,
    tenancy_digest: TenancyDigest,
    principal: PrincipalId,
    signer: CustodySigner,
}

impl Fixture {
    fn new(label: &str) -> Self {
        let root = directory(label);
        fs::create_dir_all(&root).unwrap_or_else(|error| panic!("fixture root: {error}"));
        let store_root = root.join("human-store");
        let secret = root.join("kms-root");
        fs::write(&secret, [0x42; 64]).unwrap_or_else(|error| panic!("KMS root: {error}"));
        let tenancy_digest = tenancy(&[("owner", "tenant-a")])
            .install(&store_root)
            .unwrap_or_else(|error| panic!("tenancy: {error}"));
        let principal = principal("owner");
        let provider = EnvelopeKms::new("file-kms://human-primary", &secret)
            .unwrap_or_else(|error| panic!("provider: {error}"));
        let keystore = Keystore::open_development(root.join("custody"), NETWORK_ID, provider)
            .unwrap_or_else(|error| panic!("keystore: {error}"));
        keystore
            .generate(
                &principal,
                &custody_key(),
                KeyClass::HumanPrimary,
                KeyEntropy::new([0x51; 32], [0x52; 16], [0x53; 24])
                    .unwrap_or_else(|error| panic!("entropy: {error}")),
            )
            .unwrap_or_else(|error| panic!("generate key: {error}"));
        let signer_store =
            PrincipalStore::open(&store_root, retention_uniform(10_000), tenancy_digest)
                .unwrap_or_else(|error| panic!("signer store: {error}"));
        let signer = CustodySigner::new(
            keystore,
            signer_store,
            registry(),
            SigningLimits::new(1_000, 10_000).unwrap_or_else(|error| panic!("limits: {error}")),
        );
        Self {
            root,
            store_root,
            tenancy_digest,
            principal,
            signer,
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

    fn signed_actions(
        &self,
        scope: &PrincipalScope<'_>,
        plan: &UnifiedPlan,
        request: &SubmitPlanRequest,
    ) -> Vec<EvmAction> {
        let account = layerx_crypto::account_derivation::derive_from_seed(&[0x5a; 32], 0)
            .unwrap_or_else(|error| panic!("EVM account: {error:?}"));
        let signer = layerx_crypto::local::LocalSigner::from_derived_account(&account);
        let wallet = account.evm_address();
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
                let disclosure = layerx_crypto::disclosure::call::bind_call(&call, wallet)
                    .unwrap_or_else(|error| panic!("send disclosure: {error:?}"));
                let mut future =
                    layerx_crypto::signer::sign_disclosed_call(&signer, &call, &disclosure, wallet);
                let signature = match future
                    .as_mut()
                    .poll(&mut Context::from_waker(Waker::noop()))
                {
                    Poll::Ready(result) => {
                        result.unwrap_or_else(|error| panic!("sign send: {error:?}"))
                    }
                    Poll::Pending => panic!("local EVM signer did not finish"),
                };
                drop(future);
                let raw_transaction = layerx_crypto::evm_transaction::assemble(&call, &signature);
                EvmAction {
                    authorization: EvmPlanAuthorization {
                        plan_id: plan.digest(),
                        action_key: binding.action_key,
                        tenant: scope.tenant().as_str().to_owned(),
                        principal: scope.principal().as_str().to_owned(),
                        binding_digest: self
                            .signer
                            .evm_binding(&self.principal, &custody_key())
                            .unwrap_or_else(|error| panic!("custody binding: {error:?}"))
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
                    },
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

fn custody_key() -> KeyId {
    KeyId::new("human-primary").unwrap_or_else(|error| panic!("key: {error}"))
}

#[test]
fn intent_submit_kernel_send_persists_a_processing_record_with_each_leg_hash() {
    let fixture = Fixture::new("intent-submit-kernel");
    let plan = kernel_plan();
    assert_eq!(
        plan.legs()[0].mechanism(),
        LegMechanism::Protocol(Mechanism::Send)
    );
    let request = valid_request(&plan, (995, 1_100));
    let expectation = expectation(1_000);
    verify_bindings(&plan, &request, &expectation)
        .unwrap_or_else(|error| panic!("bindings: {error}"));
    let journey_id = intent_journey_id("kernel-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let actions = fixture.signed_actions(&scope, &plan, &request);
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
    let conflicting_actions = fixture.signed_actions(&scope, &plan, &conflicting_request);
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
    let plan = kernel_plan();
    let mut stale = plan.digest();
    stale[0] ^= 0x01;
    let bindings = vec![binding_document(&plan, 0, SEQUENCE, (995, 1_100))];
    let request = SubmitPlanRequest::from_json(&submission_body(&plan, stale, bindings))
        .unwrap_or_else(|error| panic!("submission: {error}"));
    let refusal = verify_bindings(&plan, &request, &expectation(1_000))
        .err()
        .unwrap_or_else(|| panic!("stale digest accepted"));
    let fixture = Fixture::new("intent-submit-stale");
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let journey_id = intent_journey_id("stale-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let actions = fixture.signed_actions(&scope, &plan, &request);
    assert_eq!(
        start_kernel_journey(
            &mut scope,
            &plan,
            &request,
            &expectation(1_000),
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
        vec![binding_document(&plan, 0, SEQUENCE, (995, 1_100))],
    );
    body["plan_digest"] = json!(hex(&stale));
    let request =
        SubmitPlanRequest::from_json(&body).unwrap_or_else(|error| panic!("submission: {error}"));
    assert_eq!(
        verify_bindings(&plan, &request, &expectation(1_000)).err(),
        Some(SubmitRefusal::PlanDigestMismatch)
    );
}

#[test]
fn intent_submit_refuses_a_wrong_account_sequence_with_a_typed_conflict() {
    let fixture = Fixture::new("intent-submit-sequence");
    let plan = kernel_plan();
    let bindings = vec![binding_document(&plan, 0, SEQUENCE + 1, (995, 1_100))];
    let request = SubmitPlanRequest::from_json(&submission_body(&plan, plan.digest(), bindings))
        .unwrap_or_else(|error| panic!("submission: {error}"));
    let refusal = verify_bindings(&plan, &request, &expectation(1_000))
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
    let actions = fixture.signed_actions(&scope, &plan, &request);
    assert_eq!(
        start_kernel_journey(
            &mut scope,
            &plan,
            &request,
            &expectation(1_000),
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
    let plan = kernel_plan();
    let request = valid_request(&plan, (995, 1_100));
    let refusal = verify_bindings(&plan, &request, &expectation(1_101))
        .err()
        .unwrap_or_else(|| panic!("expired window accepted"));
    let fixture = Fixture::new("intent-submit-expired");
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let journey_id = intent_journey_id("expired-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let actions = fixture.signed_actions(&scope, &plan, &request);
    assert_eq!(
        start_kernel_journey(
            &mut scope,
            &plan,
            &request,
            &expectation(1_101),
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
        verify_bindings(&plan, &forged, &expectation(1_000)).err(),
        Some(SubmitRefusal::LegMismatch { index: 0 })
    );
    let mut greedy = request;
    greedy.bindings[0].fee_limit = MAX_FEE + 1;
    assert_eq!(
        verify_bindings(&plan, &greedy, &expectation(1_000)).err(),
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
    let plan = kernel_plan();
    let request = valid_request(&plan, (995, 1_100));
    let mut store = fixture.store();
    let mut scope = store
        .principal(&fixture.principal)
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let actions = fixture.signed_actions(&scope, &plan, &request);
    let journey_id = intent_journey_id("golden-submit", plan.digest())
        .unwrap_or_else(|error| panic!("journey id: {error}"));
    let submission = start_kernel_journey(
        &mut scope,
        &plan,
        &request,
        &expectation(1_000),
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
    let binding = IntentLegBinding::from_json(&binding_document(&plan, 0, SEQUENCE, (995, 1_100)))
        .unwrap_or_else(|error| panic!("binding: {error}"));
    assert_eq!(binding.fee_currency, "LXP");
}
