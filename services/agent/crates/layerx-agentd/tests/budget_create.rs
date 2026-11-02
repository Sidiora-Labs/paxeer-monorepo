#[path = "budget_mutations_layerxpay.rs"]
mod layerxpay;

use layerx_agentd::budget::{
    budget_create_identity, core_expiry_ms, BudgetCreationError, LocalLimit,
};
use layerx_agentd::prepare::evm::PrecompileTarget;
use layerx_client::sequencer::{ExecutedReceipt, SequencerLog, SequencerReceipt};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerxpay::{identity, registry, request, signed_call};

fn receipt(signed: &[u8]) -> SequencerReceipt {
    let (call, signature) = decode_signed(signed).expect("signed EVM call");
    let mut data = identity().asset.to_vec();
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&identity().initial_amount.to_be_bytes());
    SequencerReceipt::Committed {
        receipt: ExecutedReceipt {
            tx_hash: transaction_hash(signed),
            nonce: call.nonce,
            seq: 99,
            height: 42,
            index: 0,
            timestamp: 1_000,
            success: true,
            return_data: Vec::new(),
            logs: vec![SequencerLog {
                address: PrecompileTarget::Pay.address(),
                topics: vec![
                    transaction_hash(b"BudgetCreated(bytes32,bytes32,uint128)"),
                    identity().budget_id,
                ],
                data,
                log_index: 0,
            }],
            gas_used: 100_000,
            cumulative_gas_used: 100_000,
            effective_gas_price: 1,
            fee_denom: "upax".into(),
            fee_amount: [0; 32],
            sponsor: [0; 20],
            from: recover_sender(&call, &signature).expect("sender"),
            to: call.to,
            contract_address: [0; 20],
            vm_error: String::new(),
            key_id: [1; 32],
            receipt: Vec::new(),
            signing_key: ed25519_dalek::SigningKey::from_bytes(&[8; 32])
                .verifying_key()
                .to_bytes(),
        },
        block_number: 42,
        block_hash: [0x42; 32],
    }
}

fn executed(receipt: &mut SequencerReceipt) -> &mut ExecutedReceipt {
    match receipt {
        SequencerReceipt::Committed { receipt, .. } | SequencerReceipt::Receipted(receipt) => {
            receipt
        }
        SequencerReceipt::Submitted { .. } => panic!("execution fields required"),
    }
}

#[test]
fn budget_identity_is_read_from_the_create_receipt_log() {
    let signed = signed_call(&request(), &identity());
    let receipt = receipt(&signed);
    let confirmed = budget_create_identity(&(signed.as_slice(), &receipt), &registry())
        .expect("receipt identity");
    assert_eq!(confirmed.budget_id, identity().budget_id);
    assert_eq!(confirmed.asset, request().asset);
    assert_eq!(confirmed.initial_amount, identity().initial_amount);
    assert_eq!(confirmed.per_period_limit, request().ceiling);
    assert_eq!(confirmed.expiry_ms, request().expiry_ms);
}

#[test]
fn confirmed_creation_requires_exact_log_identity_and_height() {
    let signed = signed_call(&request(), &identity());
    let mut receipt = receipt(&signed);
    executed(&mut receipt).logs[0].topics[1] = [8; 32];
    assert_eq!(
        budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
        Err(BudgetCreationError::IdentityMismatch)
    );
    executed(&mut receipt).logs[0].topics[1] = identity().budget_id;
    executed(&mut receipt).height = 43;
    assert_eq!(
        budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
        Err(BudgetCreationError::UnverifiedReceipt)
    );
}

#[test]
fn committed_get_budget_retains_the_creation_identity() {
    let result = layerxpay::budget_result();
    let state = layerx_agentd::budget::ProtocolBudgetState::decode_get_budget(
        identity().budget_id,
        42,
        &result,
    )
    .expect("getBudget");
    assert_eq!(state.record().budget_id, identity().budget_id);
    assert_eq!(state.record().asset_id, request().asset);
    assert_eq!(state.record().budget_account, identity().budget_account);
    assert_eq!(state.record().per_period_limit, request().ceiling);
    assert_eq!(state.record().expiry, request().expiry_ms);
    assert_eq!(state.committed_height(), 42);
}

#[test]
fn executed_receipt_without_commit_fails_closed() {
    let signed = signed_call(&request(), &identity());
    let receipt = match receipt(&signed) {
        SequencerReceipt::Committed { receipt, .. } => SequencerReceipt::Receipted(receipt),
        _ => panic!("committed fixture"),
    };
    assert_eq!(
        budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
        Err(BudgetCreationError::UnverifiedReceipt)
    );
}

#[test]
fn receipt_asset_or_funding_substitution_fails_closed() {
    let signed = signed_call(&request(), &identity());
    for offset in [0, 31, 32, 47, 48, 63] {
        let mut receipt = receipt(&signed);
        executed(&mut receipt).logs[0].data[offset] ^= 1;
        assert_eq!(
            budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
            Err(BudgetCreationError::IdentityMismatch)
        );
    }
}

#[test]
fn activity_that_is_not_a_budget_call_is_refused() {
    let signed = signed_call(&request(), &identity());
    let (mut call, _) = decode_signed(&signed).expect("call");
    call.to = PrecompileTarget::Ledger.address();
    let signed = call.sign(&[0xa5; 32]).expect("sign");
    assert_eq!(
        budget_create_identity(signed.as_slice(), &registry()),
        Err(BudgetCreationError::NotBudgetCreation)
    );
    call.to = PrecompileTarget::Pay.address();
    call.data[..4].copy_from_slice(&[0; 4]);
    assert_eq!(
        budget_create_identity(
            call.sign(&[0xa5; 32]).expect("sign").as_slice(),
            &registry()
        ),
        Err(BudgetCreationError::NotBudgetCreation)
    );
}

#[test]
fn failed_creation_never_returns_a_budget_identity() {
    let signed = signed_call(&request(), &identity());
    let mut receipt = receipt(&signed);
    executed(&mut receipt).success = false;
    assert_eq!(
        budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
        Err(BudgetCreationError::CoreRejected)
    );
}

#[test]
fn unverified_or_substituted_canonical_activity_is_refused() {
    for bytes in [vec![], vec![1; 211], vec![2; 251]] {
        assert_eq!(
            budget_create_identity(bytes.as_slice(), &registry()),
            Err(BudgetCreationError::NotBudgetCreation)
        );
    }
    let signed = signed_call(&request(), &identity());
    let (mut call, _) = decode_signed(&signed).expect("call");
    call.data[4 + 4 * 32] = 1;
    assert_eq!(
        budget_create_identity(
            call.sign(&[0xa5; 32]).expect("sign").as_slice(),
            &registry()
        ),
        Err(BudgetCreationError::NotBudgetCreation)
    );
}

#[test]
fn unverifiable_creation_log_is_refused() {
    let signed = signed_call(&request(), &identity());
    for target in [
        PrecompileTarget::Ledger,
        PrecompileTarget::Auth,
        PrecompileTarget::Programs,
    ] {
        let mut receipt = receipt(&signed);
        executed(&mut receipt).logs[0].address = target.address();
        assert_eq!(
            budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
            Err(BudgetCreationError::UnverifiedReceipt)
        );
    }
    let mut receipt = receipt(&signed);
    let duplicate = executed(&mut receipt).logs[0].clone();
    executed(&mut receipt).logs.push(duplicate);
    assert_eq!(
        budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
        Err(BudgetCreationError::UnverifiedReceipt)
    );
}

#[test]
fn receipt_for_another_transaction_cannot_confirm_a_budget() {
    let signed = signed_call(&request(), &identity());
    let mut receipt = receipt(&signed);
    executed(&mut receipt).tx_hash = [8; 32];
    assert_eq!(
        budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
        Err(BudgetCreationError::ReceiptActivityMismatch)
    );
}

#[test]
fn rejected_or_wrong_sender_receipt_is_refused() {
    let signed = signed_call(&request(), &identity());
    let mut receipt = receipt(&signed);
    executed(&mut receipt).from = [8; 20];
    assert_eq!(
        budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
        Err(BudgetCreationError::IdentityMismatch)
    );
    executed(&mut receipt).from = budget_create_identity(signed.as_slice(), &registry())
        .expect("sender")
        .sender;
    executed(&mut receipt).vm_error = "reverted".into();
    assert_eq!(
        budget_create_identity(&(signed.as_slice(), &receipt), &registry()),
        Err(BudgetCreationError::UnverifiedReceipt)
    );
}

#[test]
fn local_limit_is_never_described_as_protocol_equivalent() {
    let request = request();
    let local = LocalLimit::new(request.tenant, [1; 32], request.asset, request.ceiling);
    assert_eq!(local.enforcement, "daemon-enforced");
    assert_eq!(
        local.bypass_statement,
        "daemon-enforced only; bypassing layerx-agentd bypasses this limit"
    );
}

#[test]
fn request_expiry_is_milliseconds_and_equality_is_expired() {
    assert_eq!(core_expiry_ms(90), Ok(90_000));
    assert_eq!(core_expiry_ms(0), Err(BudgetCreationError::InvalidLimit));
    assert_eq!(
        core_expiry_ms(u64::MAX),
        Err(BudgetCreationError::InvalidLimit)
    );
    let mut request = request();
    request.expiry_ms = request.core_time_ms;
    assert_eq!(
        request.create_protocol_budget(&identity()),
        Err(BudgetCreationError::Expired)
    );
    request.expiry_ms += 1;
    assert!(request.create_protocol_budget(&identity()).is_ok());
}

#[test]
fn signed_create_calldata_is_the_exact_deployed_abi() {
    let mut identity = identity();
    for (version, count) in [(1, 12), (2, 14)] {
        identity.encoding_version = version;
        identity.source_account = [5; 32];
        identity.source_sequence = 40;
        let signed = signed_call(&request(), &identity);
        let (call, _) = decode_signed(&signed).expect("signed call");
        assert_eq!(call.data.len(), 4 + count * 32);
        assert_eq!(call.to, PrecompileTarget::Pay.address());
        let decoded = budget_create_identity(signed.as_slice(), &registry()).expect("ABI identity");
        assert_eq!(decoded.encoding_version, version);
        assert_eq!(decoded.budget_id, identity.budget_id);
        assert_eq!(decoded.initial_amount, identity.initial_amount);
        if version == 2 {
            assert_eq!(decoded.source_account, identity.source_account);
            assert_eq!(decoded.source_sequence, identity.source_sequence);
        }
        let mut truncated = call.clone();
        truncated.data.pop();
        assert_eq!(
            budget_create_identity(
                truncated.sign(&[0xa5; 32]).expect("sign").as_slice(),
                &registry()
            ),
            Err(BudgetCreationError::NotBudgetCreation)
        );
    }
}
