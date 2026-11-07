#[path = "support/repair_22_33_44_approval.rs"]
mod durable_approval;

#[path = "retired_crypto_codec_2.rs"]
mod retired_crypto_codec_2;

use layerx_human_test_support as support;

use ed25519_dalek::SigningKey;
use layerx_agentd::approval::ApprovalService;
use layerx_agentd::prepare::evm::{
    prepare_bound, EvmPreparationState, PrecompileCall, PreparationDefaults, PrepareRequest,
    Prepared,
};
use layerx_agentd::store::TenantId;
use layerx_human_service::approvals::{
    ApprovalActivityClass, ApprovalPresentation, DisclosureRenderError, DisclosureRenderer,
    RenderedApproval, RenderedCounterparty, VerifiedBudgetAfter,
};
use layerx_intents::{compile, Intent, IntentKind, LxpSend};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey, SendAuthorization,
    SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use proptest::prelude::*;
use sha2::{Digest as _, Sha256};

const NETWORK_ID: u32 = 77;
const ACCOUNT_SEQUENCE: u64 = 7;
const CATALOG: &str = include_str!("../../../../../apps/web/copy/catalog.ts");

fn activity(module: ModuleId, ordinal: u16) -> ActivityType {
    ActivityType::new(module, ordinal)
        .unwrap_or_else(|error| panic!("activity {module:?}/{ordinal}: {error:?}"))
}

fn registry() -> ModuleRegistry {
    let asset = ModuleRegistration::new(ModuleId::Asset, &[activity(ModuleId::Asset, 5)])
        .unwrap_or_else(|error| panic!("asset registry: {error:?}"));
    ModuleRegistry::new(&[asset]).unwrap_or_else(|error| panic!("module registry: {error:?}"))
}

fn account(value: &str) -> AccountId {
    AccountId::parse(value).unwrap_or_else(|error| panic!("account {value}: {error:?}"))
}

fn owner() -> AccountId {
    account("agent:did:layerx:approval-render:main")
}

fn recipient(marker: u64) -> AccountId {
    account(&format!("agent:did:layerx:recipient-{marker}:main"))
}

fn nonzero_key(mut key: [u8; 32]) -> [u8; 32] {
    key[0] |= 1;
    key
}

fn send_intent(recipient_marker: u64, amount: u128, asset: [u8; 32], key: [u8; 32]) -> Intent {
    let signer = layerx_crypto::local::LocalSigner::new([0x31; 32]);
    let key = nonzero_key(key);
    let debit = layerx_crypto::send::SendDebit {
        from: layerx_intents::canonical::account_id_for_protocol(
            &account("agent:did:layerx:approval-render:main"),
            layerx_intents::canonical::PROTOCOL_VERSION,
        )
        .unwrap_or_else(|error| panic!("source account: {error:?}")),
        to: layerx_intents::canonical::account_id_for_protocol(
            &recipient(recipient_marker),
            layerx_intents::canonical::PROTOCOL_VERSION,
        )
        .unwrap_or_else(|error| panic!("destination account: {error:?}")),
        asset,
        amount,
        source_sequence: ACCOUNT_SEQUENCE,
        idempotency_key: key,
        expires_at: durable_approval::activity_expiry(),
        context_hash: [0x55; 32],
        conditions: Vec::new(),
        authorization_kind: SendAuthorizationKind::Owner as u8,
        network_id: NETWORK_ID,
        protocol_version: layerx_intents::canonical::PROTOCOL_VERSION,
    };

    let send = LxpSend::new(
        owner(),
        recipient(recipient_marker),
        AssetId::new(asset),
        Amount::from_u128(amount),
        Sequence::from_u64(ACCOUNT_SEQUENCE),
        IdempotencyKey::new(key),
        TimestampSeconds::from_u64(durable_approval::activity_expiry()),
        ContextHash::new([0x55; 32]),
        support::sign_send(
            &signer,
            &debit,
            SendAuthorization::new(
                SendAuthorizationKind::Owner,
                PublicKey::new([0x31; 32]),
                AuthorizationSignature::new([0x41; 64]),
            ),
        ),
        NetworkId::new(NETWORK_ID).unwrap_or_else(|error| panic!("network: {error:?}")),
        ProtocolVersion::new(layerx_intents::canonical::PROTOCOL_VERSION)
            .unwrap_or_else(|error| panic!("protocol: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("send intent: {error:?}"));
    Intent::v1(IntentKind::LxpSend(send))
}

fn prepare(intent: &Intent, key: [u8; 32], fee_limit: u128) -> Prepared {
    let compiled = compile(intent, &registry()).expect("compile real transfer intent");
    let signing_key = k256::ecdsa::SigningKey::from_slice(&[0x71; 32]).expect("EVM sender key");
    let from = layerx_crypto::secp256k1::evm_address(
        signing_key
            .verifying_key()
            .to_encoded_point(false)
            .as_bytes(),
    )
    .expect("sender address");
    let prepared = prepare_bound(
        EvmPreparationState {
            chain_id: 125,
            pending_nonce: ACCOUNT_SEQUENCE,
            base_fee_per_gas: 0,
            head_height: 91,
        },
        PreparationDefaults {
            max_priority_fee_per_gas: 0,
            max_fee_per_gas: fee_limit,
            maximum_payload_bytes: 1_024,
        },
        PrepareRequest {
            from,
            call: PrecompileCall::ledger_transfer(compiled.payload().as_bytes().to_vec()),
            expected_nonce: Some(ACCOUNT_SEQUENCE),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1_024,
        },
        100_000,
    )
    .expect("real EVM prepare and disclosure");
    assert_eq!(prepared.disclosure.idempotency_key, nonzero_key(key));
    prepared
}

fn held_digest(prepared: &Prepared, marker: u8) -> [u8; 32] {
    let root = support::directory("render-durable-approval");
    let tenant = TenantId::new("tenant-approval-render").expect("tenant");
    let clock = durable_approval::clock();
    let (registry, limiter, expiry) = durable_approval::register_and_reopen(
        &root.join("registry"),
        &tenant,
        prepared,
        marker,
        100,
        899,
        clock.as_ref(),
    );
    let record = ApprovalService::new(&registry, &limiter, &expiry)
        .get(
            &tenant,
            [marker; 32],
            101,
            durable_approval::current_time(clock.as_ref()),
        )
        .expect("production approval presentation");
    let digest = record.canonical_bytes_digest;
    assert_eq!(
        digest,
        durable_approval::preparation(prepared)
            .disclosure
            .canonical_digest
    );
    drop(expiry);
    drop(registry);
    std::fs::remove_dir_all(root).expect("cleanup approval store");
    digest
}

fn verified_budget(remaining: u128, observed_at_sequence: u64) -> VerifiedBudgetAfter {
    let account = [0x41; 32];
    let asset = [0x42; 32];
    let mut canonical = Vec::with_capacity(80);
    canonical.extend_from_slice(&account);
    canonical.extend_from_slice(&asset);
    canonical.extend_from_slice(&remaining.to_be_bytes());
    let authority = support::evidence_verifier(&SigningKey::from_bytes(&[0x84; 32]));
    let state = authority
        .verify_state(&support::raw_state_leaf(canonical, observed_at_sequence))
        .unwrap_or_else(|error| panic!("verified balance state: {error:?}"));
    let canonical: [u8; 80] = state
        .canonical_state()
        .try_into()
        .unwrap_or_else(|_| panic!("canonical balance state length"));
    assert_eq!(
        state.level(),
        layerx_types::verify::VerificationLevel::STATE_PROVEN
    );
    assert_eq!(canonical[..32], account);
    assert_eq!(canonical[32..64], asset);
    let verified_remaining = u128::from_be_bytes(
        canonical[64..]
            .try_into()
            .unwrap_or_else(|_| panic!("canonical balance amount")),
    );
    let mut evidence = Sha256::new();
    evidence.update(canonical);
    evidence.update(state.observed_head_sequence().to_be_bytes());
    VerifiedBudgetAfter {
        remaining: verified_remaining,
        rung: Rung::Committed,
        evidence_digest: evidence.finalize().into(),
        observed_at_sequence: state.observed_head_sequence(),
    }
}

fn rendered(
    prepared: &Prepared,
    marker: u8,
    budget_after: VerifiedBudgetAfter,
) -> RenderedApproval {
    let digest = held_digest(prepared, marker);
    match DisclosureRenderer::render(&prepared.disclosure, digest, budget_after)
        .unwrap_or_else(|error| panic!("render approval: {error}"))
    {
        ApprovalPresentation::Rendered(rendered) => *rendered,
        ApprovalPresentation::Unrenderable(value) => {
            panic!("known class was unrenderable: {value:?}")
        }
    }
}

#[test]
fn every_real_v1_hold_class_has_plain_catalog_copy() {
    for class in ApprovalActivityClass::ALL {
        assert!(CATALOG.contains(&format!(
            "key: \"{}\", message: \"{}\"",
            class.copy_key(),
            class.copy_template()
        )));
    }
}

#[test]
fn real_send_hold_has_plain_catalog_copy() {
    let send = prepare(&send_intent(1, 25, [0x21; 32], [1; 32]), [1; 32], 3);
    let value = rendered(&send, 1, verified_budget(975, 101));
    let class = ApprovalActivityClass::MoveMoney;
    assert_eq!(value.class, class);
    assert_eq!(value.copy_key, class.copy_key());
    assert!(CATALOG.contains(&format!(
        "key: \"{}\", message: \"{}\"",
        class.copy_key(),
        class.copy_template()
    )));
    assert!(!value.plain_copy.is_empty());
}

#[test]
fn unknown_activity_is_explicitly_unrenderable_and_never_approvable() {
    let prepared = prepare(&send_intent(4, 10, [0x24; 32], [4; 32]), [4; 32], 2);
    let mut unknown = prepared.disclosure.clone();
    unknown.activity_type = activity(ModuleId::Asset, 99);
    let presentation = DisclosureRenderer::render(
        &unknown,
        held_digest(&prepared, 5),
        verified_budget(990, 105),
    )
    .unwrap_or_else(|error| panic!("unknown result: {error}"));
    assert!(!presentation.can_approve());
    let ApprovalPresentation::Unrenderable(value) = presentation else {
        panic!("unknown activity became approvable");
    };
    assert_eq!(value.activity_type, activity(ModuleId::Asset, 99).value());
    assert_eq!(value.copy_key, "approval.activity.unrenderable");
    assert_eq!(value.plain_copy, "This request cannot be reviewed here.");
    assert!(CATALOG.contains("key: \"approval.activity.unrenderable\""));
}

#[test]
fn mismatched_digest_and_unverified_budget_are_refused_before_content() {
    let prepared = prepare(&send_intent(5, 10, [0x25; 32], [5; 32]), [5; 32], 2);
    let mut digest = held_digest(&prepared, 6);
    digest[0] ^= 1;
    assert_eq!(
        DisclosureRenderer::render(&prepared.disclosure, digest, verified_budget(990, 106)),
        Err(DisclosureRenderError::DigestMismatch)
    );
    let digest = held_digest(&prepared, 7);
    let mut budget = verified_budget(990, 107);
    budget.rung = Rung::Submitted;
    let presentation = DisclosureRenderer::render(&prepared.disclosure, digest, budget)
        .expect("Submitted budget with evidence renders its unverified context");
    assert!(!presentation.can_approve());
    let ApprovalPresentation::Rendered(value) = presentation else {
        panic!("known activity with budget evidence must render");
    };
    assert_eq!(value.budget_after.rung, Rung::Submitted);
    assert_eq!(value.budget_after, budget);
    budget.evidence_digest = [0; 32];
    let mut mismatched_digest = digest;
    mismatched_digest[0] ^= 1;
    assert_eq!(
        DisclosureRenderer::render(&prepared.disclosure, mismatched_digest, budget),
        Err(DisclosureRenderError::DigestMismatch)
    );
    assert_eq!(
        DisclosureRenderer::render(&prepared.disclosure, digest, budget),
        Err(DisclosureRenderError::UnverifiedBudget)
    );
}

proptest! {
    #![proptest_config(ProptestConfig::with_cases(96))]

    #[test]
    fn property_send_facts_and_budget_equal_their_verified_sources(
        amount in 1_u128..1_000_000_000_u128,
        recipient_marker in any::<u64>(),
        asset in any::<[u8; 32]>(),
        fee_limit in 1_u128..1_000_000_u128,
        remaining in 0_u128..10_000_000_000_u128,
        key in any::<[u8; 32]>(),
    ) {
        let prepared = prepare(
            &send_intent(recipient_marker, amount, asset, key),
            key,
            fee_limit,
        );
        let budget = verified_budget(remaining, 1_000);
        let value = rendered(&prepared, 8, budget);
        prop_assert_eq!(value.class, ApprovalActivityClass::MoveMoney);
        prop_assert_eq!(value.facts.amount, Some(prepared.disclosure.amounts[0].value));
        prop_assert_eq!(
            value.facts.counterparty,
            RenderedCounterparty::Account(prepared.disclosure.counterparties[1].account)
        );
        prop_assert_eq!(value.facts.asset, Some(prepared.disclosure.asset));
        prop_assert_eq!(value.facts.fee_limit, prepared.disclosure.fee_limit);
        prop_assert_eq!(value.facts.expires_at, prepared.disclosure.expiry.payload_expires_at);
        prop_assert_eq!(value.budget_after, budget);
    }

    #[test]
    fn property_any_changed_disclosure_fact_fails_reencoding(
        field in 0_u8..5,
        amount in 1_u128..1_000_000_u128,
        key in any::<[u8; 32]>(),
    ) {
        let prepared = prepare(&send_intent(10, amount, [0x31; 32], key), key, 7);
        let digest = held_digest(&prepared, 10);
        let mut changed = prepared.disclosure;
        match field {
            0 => changed.amounts[0].value = changed.amounts[0].value.saturating_add(1),
            1 => changed.counterparties[1].account[0] ^= 1,
            2 => changed.asset[0] ^= 1,
            3 => changed.fee_limit = changed.fee_limit.saturating_add(1),
            _ => changed.expiry.payload_expires_at = changed.expiry.payload_expires_at.saturating_sub(1),
        }
        prop_assert!(matches!(
            DisclosureRenderer::render(&changed, digest, verified_budget(100, 1_002)),
            Err(DisclosureRenderError::DefectiveDisclosure(_))
        ));
    }
}
