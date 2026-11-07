use layerx_crypto::disclosure::call::{bind_call, ledger_selector};
use layerx_crypto::disclosure::{Disclosure, DisclosureError};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_human_service::approvals::{
    ApprovalActivityClass, ApprovalPresentation, DisclosureRenderError, DisclosureRenderer,
    RenderedApproval, RenderedApprovalFacts, RenderedCounterparty, VerifiedBudgetAfter,
};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

fn approval_disclosure() -> Result<(Disclosure, [u8; 32]), String> {
    let word = |value: u128| {
        let mut word = [0; 32];
        word[16..].copy_from_slice(&value.to_be_bytes());
        word
    };
    let address = |value: u8| {
        let mut address = [0; 32];
        address[12..].fill(value);
        address
    };
    let mut data = ledger_selector("transfer(address,uint256)").to_vec();
    for argument in [address(0x14), word(25)] {
        data.extend_from_slice(&argument);
    }
    let call = Eip1559Call {
        chain_id: 77,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 1,
        gas_limit: 3,
        to: [0x15; 20],
        data,
    };
    let signed = call
        .sign(&[0x68; 32])
        .map_err(|error| format!("{error:?}"))?;
    let (decoded, signature) = decode_signed(&signed).map_err(|error| format!("{error:?}"))?;
    let sender = recover_sender(&decoded, &signature).map_err(|error| format!("{error:?}"))?;
    let disclosure = bind_call(&decoded, sender).map_err(|error| format!("{error:?}"))?;
    let digest = disclosure
        .audit_digest()
        .map_err(|error| format!("{error:?}"))?;
    Ok((disclosure, digest))
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
            class: ApprovalActivityClass::MoveMoney,
            copy_key: "approval.activity.move_money",
            plain_copy: format!(
                "Move 25 {} to {}. Fees can be up to 3.",
                "00".repeat(32),
                "00".repeat(12) + &"14".repeat(20)
            ),
            facts: RenderedApprovalFacts {
                amount: Some(25),
                counterparty: RenderedCounterparty::Account({
                    let mut account = [0; 32];
                    account[12..].fill(0x14);
                    account
                }),
                asset: Some([0; 32]),
                fee_limit: 3,
                expires_at: u64::MAX,
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
    let mut changed_disclosure = disclosure.clone();
    changed_disclosure.amounts[0].value += 1;
    assert_eq!(
        DisclosureRenderer::render(&changed_disclosure, digest, budget),
        Err(DisclosureRenderError::DefectiveDisclosure(
            DisclosureError::FieldMismatch("amounts")
        ))
    );
    Ok(())
}
