use layerx_crypto::disclosure::{bind, Disclosure};
use layerx_human_service::approvals::{
    ApprovalActivityClass, ApprovalPresentation, DisclosureRenderError, DisclosureRenderer,
    RenderedApproval, RenderedApprovalFacts, RenderedCounterparty, VerifiedBudgetAfter,
};
use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload};
use layerx_types::rung::Rung;
use layerx_wire::activity::encode_unsigned_envelope;
use layerx_wire::hash::payload_hash_for;
use sha2::{Digest as _, Sha256};

fn approval_disclosure() -> Result<(Disclosure, [u8; 32]), String> {
    let kind = ActivityType::new(ModuleId::Bridge, 1).map_err(|error| format!("{error:?}"))?;
    let registration =
        ModuleRegistration::new(ModuleId::Bridge, &[kind]).map_err(|error| format!("{error:?}"))?;
    let registry = ModuleRegistry::new(&[registration]).map_err(|error| format!("{error:?}"))?;
    let mut payload = vec![0; 368];
    payload[..5].copy_from_slice(b"LXDC3");
    payload[5..37].fill(0x12);
    payload[37..41].copy_from_slice(&77_u32.to_be_bytes());
    payload[41..43].copy_from_slice(&1_u16.to_be_bytes());
    payload[43..75].fill(0x11);
    payload[75..107].fill(0x15);
    payload[107..139].fill(0x14);
    payload[139..171].fill(0x12);
    payload[171..191].fill(0x13);
    payload[191..207].copy_from_slice(&25_u128.to_be_bytes());
    payload[359..363].copy_from_slice(&2_u32.to_be_bytes());
    payload[363..368].copy_from_slice(b"LXLB1");
    let proof_hash = Sha256::digest(&payload[363..]);
    payload[327..359].copy_from_slice(&proof_hash);
    let payload = Payload::new(&registry, kind, &payload).map_err(|error| format!("{error:?}"))?;
    let actor = Did::new(b"did:layerx:approval-rungs").map_err(|error| format!("{error:?}"))?;
    let mut nullifier = Sha256::new();
    nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
    nullifier.update([0x11; 32]);
    let mut builder = EnvelopeBuilder::new();
    builder
        .protocol_version(1)
        .and_then(|builder| builder.network_id(77))
        .and_then(|builder| builder.activity_type(kind))
        .and_then(|builder| builder.actor_did(actor))
        .and_then(|builder| builder.authority(Authority::protocol_module(&[0x12; 32])?))
        .and_then(|builder| builder.account_sequence(7))
        .and_then(|builder| builder.timestamp_bound(TimestampBound::new(90, 1_000)?))
        .and_then(|builder| {
            builder.idempotency_key(IdempotencyKey::new(nullifier.finalize().into()))
        })
        .and_then(|builder| builder.fee_limit(Amount::from_u128(3)))
        .map_err(|error| format!("{error:?}"))?;
    builder
        .payload_hash(payload_hash_for(&payload).map_err(|error| format!("{error:?}"))?)
        .and_then(|builder| builder.payload(payload))
        .map_err(|error| format!("{error:?}"))?;
    let envelope = builder.build().map_err(|error| format!("{error:?}"))?;
    let canonical = encode_unsigned_envelope(&envelope).map_err(|error| format!("{error:?}"))?;
    let disclosure = bind(&canonical, &registry).map_err(|error| format!("{error:?}"))?;
    Ok((disclosure, Sha256::digest(&canonical).into()))
}

fn budget(rung: Rung) -> VerifiedBudgetAfter {
    let remaining = 975_u128;
    let observed_at_sequence = 101_u64;
    let mut evidence = Sha256::new();
    evidence.update([0x13; 32]);
    evidence.update([0x15; 32]);
    evidence.update(remaining.to_be_bytes());
    evidence.update(observed_at_sequence.to_be_bytes());
    VerifiedBudgetAfter {
        remaining,
        rung,
        evidence_digest: evidence.finalize().into(),
        observed_at_sequence,
    }
}

#[test]
fn approval_budget_read_below_committed_renders_unverified() -> Result<(), String> {
    let (disclosure, digest) = approval_disclosure()?;
    for rung in [Rung::Submitted, Rung::Receipted] {
        assert_eq!(
            DisclosureRenderer::render(&disclosure, digest, budget(rung)),
            Err(DisclosureRenderError::UnverifiedBudget),
            "budget at {rung:?} must not become verified approval content"
        );
    }
    Ok(())
}

#[test]
fn approval_budget_read_at_committed_renders_unchanged() -> Result<(), String> {
    let (disclosure, digest) = approval_disclosure()?;
    let budget = budget(Rung::Committed);
    let presentation = DisclosureRenderer::render(&disclosure, digest, budget)
        .map_err(|error| format!("{error:?}"))?;
    assert!(presentation.can_approve());
    assert_eq!(
        presentation,
        ApprovalPresentation::Rendered(Box::new(RenderedApproval {
            class: ApprovalActivityClass::AddMoney,
            copy_key: "approval.activity.add_money",
            plain_copy: format!(
                "Add 25 {} to {}. Fees can be up to 3.",
                "15".repeat(32),
                "14".repeat(32)
            ),
            facts: RenderedApprovalFacts {
                amount: Some(25),
                counterparty: RenderedCounterparty::Account([0x14; 32]),
                asset: Some([0x15; 32]),
                fee_limit: 3,
                expires_at: 1_000,
            },
            budget_after: budget,
            disclosure_digest: digest,
        }))
    );
    let mut missing_evidence = budget;
    missing_evidence.evidence_digest = [0; 32];
    assert_eq!(
        DisclosureRenderer::render(&disclosure, digest, missing_evidence),
        Err(DisclosureRenderError::UnverifiedBudget)
    );
    let mut changed_digest = digest;
    changed_digest[0] ^= 1;
    assert_eq!(
        DisclosureRenderer::render(&disclosure, changed_digest, budget),
        Err(DisclosureRenderError::DigestMismatch)
    );
    Ok(())
}
