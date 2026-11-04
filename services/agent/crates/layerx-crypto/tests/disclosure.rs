use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::{
    AmountRole, CounterpartyRole, DisclosedWithdrawal, DisclosureError, Expiry,
};
use layerx_crypto::evm_transaction::{decode_unsigned, recover_sender, Eip1559Call};
use layerx_crypto::signer::{sign_disclosed_call, SignError};
use layerx_wire::decode::Decoder;
use sha2::{Digest as _, Sha256};

mod support;
use support::complete as ready;

#[test]
fn canonical_send_discloses_every_authorised_semantic_and_reencodes_exactly() {
    let produced = support::send_call(25, 7);
    let canonical = produced.call.encode_unsigned();
    let sender = support::call_options(7).from;
    let disclosure = bind_call(&decode_unsigned(&canonical).expect("type-2 call"), sender)
        .expect("canonical send disclosure");
    assert_eq!(disclosure.activity_type.value(), 0x0001_0005);
    assert_eq!(disclosure.actor, sender);
    assert_eq!(disclosure.authority, sender);
    assert_eq!(disclosure.counterparties.len(), 2);
    assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
    assert_eq!(disclosure.counterparties[0].account, [0x11; 32]);
    assert_eq!(
        disclosure.counterparties[1].role,
        CounterpartyRole::Recipient
    );
    assert_eq!(disclosure.counterparties[1].account, [0x22; 32]);
    assert_eq!(disclosure.amounts.len(), 1);
    assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosure.amounts[0].value, 25);
    assert_eq!(disclosure.asset, [0x33; 32]);
    assert_eq!(disclosure.fee_limit, 3_000_000);
    assert_eq!(
        disclosure.expiry,
        Expiry {
            not_before: 0,
            not_after: u64::MAX,
            payload_expires_at: 100
        }
    );
    assert_eq!(disclosure.idempotency_key, [0x71; 32]);
    assert_eq!(disclosure.reencode(), Ok(canonical));
}

/// Retired custody payloads remain recorded evidence, never a transaction a
/// signer may authorize. Exercise the production type-2 decoder and call binder
/// against both the original envelope and a transaction carrying the old bytes.
fn refuse_retired(canonical: &[u8]) {
    assert!(decode_unsigned(canonical).is_err());
    let mut target = [0; 20];
    target[18..].copy_from_slice(&0x1013_u16.to_be_bytes());
    let call = Eip1559Call {
        to: target,
        data: canonical.to_vec(),
        ..support::send_call(25, 7).call
    };
    assert_eq!(
        bind_call(&call, support::call_options(7).from),
        Err(DisclosureError::MalformedPayload)
    );
}

#[test]
fn bridge_deposit_credit_discloses_and_signs_the_exact_compiled_semantics() {
    let canonical = support::archived_bridge_credit(25);
    let registry = support::bridge_registry();
    let archive =
        layerx_wire::activity::decode_unsigned(&canonical, &registry).expect("archived credit");
    assert_eq!(archive.activity_type().value(), 0x0008_0001);
    let payload = archive.payload();
    let mut payer = [0; 32];
    payer[12..].fill(0x11);
    let mut recorded_payer = [0; 32];
    recorded_payer[12..].copy_from_slice(&payload[171..191]);
    assert_eq!(recorded_payer, payer);
    assert_eq!(&payload[107..139], &[0x22; 32]);
    assert_eq!(&payload[75..107], &[0x33; 32]);
    assert_eq!(
        u128::from_be_bytes(payload[191..207].try_into().expect("amount")),
        25
    );
    let mut nullifier = Sha256::new();
    nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
    nullifier.update([0x41; 32]);
    assert_eq!(archive.idempotency_key(), nullifier.finalize()[..]);
    assert_eq!(
        layerx_wire::activity::encode_unsigned(&archive).expect("archive reencode"),
        canonical
    );
    refuse_retired(&canonical);
    // Successful signing now uses the supported native payment transaction.
    let produced = support::send_call(25, 7);
    assert!(ready(sign_disclosed_call(
        &support::evm_signer(),
        &produced.call,
        &produced.disclosure,
        support::call_options(7).from
    ))
    .is_ok());
}

#[test]
fn recorded_custody_credit_discloses_the_signed_activity() {
    let recorded = include_bytes!(
        "../../../../../tests/fixtures/custody/daemon-light-credit-receipt/activity"
    );
    assert_eq!(recorded.len(), 1821);
    let registry = support::bridge_registry();
    let activity =
        layerx_wire::activity::decode_signed(recorded, &registry).expect("recorded activity");
    assert_eq!(activity.network_id(), 77);
    assert_eq!(activity.protocol_version(), 3);
    assert_eq!(activity.signature().map(<[u8]>::len), Some(64));
    let payload = activity.payload();
    assert_eq!(payload.len(), 1495);
    assert_eq!(
        payload,
        include_bytes!("../../../../../tests/fixtures/custody/daemon-light-credit-receipt/credit")
    );
    assert_eq!(payload, &recorded[257..257 + payload.len()]);
    assert_eq!(&payload[..5], b"LXDC3");
    let canonical =
        layerx_wire::activity::encode_unsigned(&activity).expect("recorded unsigned activity");
    let mut nullifier = Sha256::new();
    nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
    nullifier.update(&payload[43..75]);
    assert_eq!(nullifier.finalize()[..], activity.idempotency_key());
    // Keep all source, recipient, asset and amount assertions on the recorded
    // codec; the custody executor and its signing route are retired.
    let decoded =
        layerx_wire::activity::decode_unsigned(&canonical, &registry).expect("archive codec");
    assert_eq!(&decoded.payload()[171..191], &payload[171..191]);
    assert_eq!(&decoded.payload()[107..139], &payload[107..139]);
    assert_eq!(&decoded.payload()[75..107], &payload[75..107]);
    assert_eq!(&decoded.payload()[191..207], &payload[191..207]);
    assert_eq!(
        layerx_wire::activity::encode_unsigned(&decoded).expect("archive reencode"),
        canonical
    );
    refuse_retired(&canonical);
}

#[test]
fn legacy_bridge_withdrawal_is_refused() {
    refuse_retired(&support::archived_bridge_withdraw(25));
}

#[test]
fn disclosed_signing_uses_the_exact_canonical_bytes() {
    let produced = support::send_call(25, 7);
    let canonical = produced.call.encode_unsigned();
    let call = decode_unsigned(&canonical).expect("canonical transaction");
    let sender = support::call_options(7).from;
    let disclosure = bind_call(&call, sender).expect("send disclosure");
    let signature = ready(sign_disclosed_call(
        &support::evm_signer(),
        &call,
        &disclosure,
        sender,
    ))
    .expect("matching disclosed call");
    assert_eq!(recover_sender(&call, &signature), Ok(sender));
    assert_eq!(
        disclosure.signing_digest().expect("digest"),
        call.signing_hash()
    );
}

#[test]
fn small_payment_disclosure_cannot_authorise_large_payment_bytes() {
    let produced = support::send_call(9_000_000, 7);
    let mut disclosure = produced.disclosure;
    disclosure.amounts[0].value = 1;
    let refusal = ready(sign_disclosed_call(
        &support::evm_signer(),
        &produced.call,
        &disclosure,
        support::call_options(7).from,
    ));
    assert_eq!(refusal, Err(SignError::DisclosureMismatch("call")));
    assert_eq!(
        disclosure.validate(),
        Err(DisclosureError::FieldMismatch("amounts"))
    );
    let error = disclosure.validate().expect_err("mismatching disclosure");
    assert!(error.to_string().contains("amounts"));
}

#[test]
fn byte_substitution_names_the_semantic_field_that_changed() {
    let small = support::send_call(1, 7);
    let large = support::send_call(9_000_000, 7);
    assert_ne!(small.call.encode_unsigned(), large.call.encode_unsigned());
    assert_eq!(
        ready(sign_disclosed_call(
            &support::evm_signer(),
            &large.call,
            &small.disclosure,
            support::call_options(7).from
        )),
        Err(SignError::DisclosureMismatch("call"))
    );
    let mut altered = large.disclosure;
    altered.amounts = small.disclosure.amounts;
    assert_eq!(
        altered.validate(),
        Err(DisclosureError::FieldMismatch("amounts"))
    );
}

#[test]
fn payload_commitment_mismatch_is_fail_closed() {
    let mut corrupted = support::archived_send(25);
    *corrupted.last_mut().expect("archived bytes") ^= 1;
    let activity = layerx_wire::activity::decode_unsigned(&corrupted, &support::registry())
        .expect("archive framing");
    assert_ne!(
        layerx_wire::hash::payload_hash(&activity).expect("actual hash"),
        activity.payload_hash()
    );
    refuse_retired(&corrupted);
    let produced = support::send_call(25, 7);
    let mut call = produced.call;
    // Corrupt the native authorization signature under canonical ABI framing.
    call.data[68 + 330] ^= 1;
    assert_eq!(
        bind_call(&call, support::call_options(7).from),
        Err(DisclosureError::MalformedPayload)
    );
}

fn withdrawal_payload() -> Vec<u8> {
    let mut payload = vec![0x33; 32];
    payload.extend_from_slice(&25u128.to_be_bytes());
    payload.extend_from_slice(&[0x44; 20]);
    payload.extend_from_slice(&[0x66; 32]);
    payload.extend_from_slice(&20u64.to_be_bytes());
    payload
}

#[test]
fn native_withdrawal_discloses_every_field_and_signs_exact_bytes() {
    let payload = withdrawal_payload();
    let canonical = support::archived_native_withdraw(&payload, 0x0001_0009);
    let mut input = Decoder::new(&payload, 0);
    assert_eq!(input.fixed(32).expect("asset"), [0x33; 32]);
    assert_eq!(input.u128().expect("amount"), 25);
    assert_eq!(input.fixed(20).expect("recipient"), [0x44; 20]);
    assert_eq!(input.fixed(32).expect("anchor"), [0x66; 32]);
    assert_eq!(input.u64().expect("fee"), 20);
    assert!(input.finish().is_ok());
    let kind = layerx_types::payload::ActivityType::new(layerx_types::payload::ModuleId::Asset, 9)
        .expect("kind");
    let module = layerx_types::payload::ModuleRegistration::new(
        layerx_types::payload::ModuleId::Asset,
        &[kind],
    )
    .expect("module");
    let registry = layerx_types::payload::ModuleRegistry::new(&[module]).expect("registry");
    let archived = layerx_wire::activity::decode_unsigned(&canonical, &registry).expect("archive");
    assert_eq!(archived.activity_type().value(), 0x0001_0009);
    assert_eq!(archived.actor_did(), b"did:layerx:alice");
    assert_eq!(archived.authority().len(), 32);
    assert_eq!(archived.fee_limit(), 20);
    assert_eq!(archived.idempotency_key(), [0x71; 32]);
    assert_eq!(archived.account_sequence(), 7);
    assert_eq!(
        layerx_wire::activity::encode_unsigned(&archived).expect("reencode"),
        canonical
    );
    refuse_retired(&canonical);
    // A withdrawal cannot be injected into the semantics of a supported call.
    let produced = support::send_call(25, 7);
    assert_eq!(
        produced.disclosure.counterparties[0].role,
        CounterpartyRole::Payer
    );
    assert_eq!(produced.disclosure.asset, [0x33; 32]);
    assert_eq!(produced.disclosure.amounts[0].value, 25);
    assert!(ready(sign_disclosed_call(
        &support::evm_signer(),
        &produced.call,
        &produced.disclosure,
        support::call_options(7).from
    ))
    .is_ok());
    for field in 0..3 {
        let mut changed = produced.disclosure.clone();
        let mut withdrawal = DisclosedWithdrawal {
            account_sequence: 7,
            evm_recipient: [0x44; 20],
            request_anchor: [0x66; 32],
        };
        if field == 0 {
            withdrawal.evm_recipient[0] ^= 1;
        }
        if field == 1 {
            withdrawal.request_anchor[0] ^= 1;
        }
        if field == 2 {
            withdrawal.account_sequence += 1;
        }
        changed.withdrawal = Some(withdrawal);
        assert!(ready(sign_disclosed_call(
            &support::evm_signer(),
            &produced.call,
            &changed,
            support::call_options(7).from
        ))
        .is_err());
        assert_eq!(
            changed.validate(),
            Err(DisclosureError::FieldMismatch("withdrawal"))
        );
        assert!(changed.audit_digest().is_err());
    }
}

#[test]
fn native_withdrawal_refuses_old_unknown_truncated_extended_and_invalid_fields() {
    let payload = withdrawal_payload();
    for length in 0..payload.len() {
        refuse_retired(&support::archived_native_withdraw(
            &payload[..length],
            0x0001_0009,
        ));
    }
    let mut invalid = vec![vec![0; 216], payload.clone()];
    invalid[1].push(0);
    for range in [32..48, 48..68, 68..100] {
        let mut altered = payload.clone();
        altered[range].fill(0);
        invalid.push(altered);
    }
    let mut fee = payload.clone();
    fee[107] ^= 1;
    invalid.push(fee);
    for malformed in invalid {
        refuse_retired(&support::archived_native_withdraw(&malformed, 0x0001_0009));
    }
    refuse_retired(&support::archived_native_withdraw(&payload, 0x0001_000c));
}

#[test]
fn send_discloses_independent_identity_and_source_sequences() {
    let produced = support::send_call(25, 19);
    let disclosure = produced.disclosure;
    assert_eq!(disclosure.envelope_sequence(), 19);
    assert_eq!(disclosure.payload_sequence(), Ok(Some(7)));
    assert!(ready(sign_disclosed_call(
        &support::evm_signer(),
        &produced.call,
        &disclosure,
        support::call_options(19).from
    ))
    .is_ok());
    let other = support::send_call(25, 20);
    assert!(ready(sign_disclosed_call(
        &support::evm_signer(),
        &other.call,
        &disclosure,
        support::call_options(20).from
    ))
    .is_err());
}

#[test]
fn divergent_sequences_do_not_bypass_inner_signature_verification() {
    let produced = support::send_call(25, 19);
    let Some(ConstructMethod::LedgerSend { payload: original }) =
        produced.disclosure.construct_method().expect("send method")
    else {
        panic!("send payload")
    };
    for offset in [116, original.len() - 102, original.len() - 1] {
        let mut payload = original.clone();
        payload[offset] ^= 1;
        assert!(
            layerx_crypto::send::ledger_payment_call(&payload, &support::call_options(19)).is_err()
        );
    }
}
