use std::{
    future::Future,
    pin::pin,
    sync::Arc,
    task::{Context, Poll, Wake, Waker},
};

use layerx_crypto::account_derivation::derive_from_mnemonic;
use layerx_crypto::disclosure::call::{encode_payment_call, reencode, ConstructMethod};
use layerx_crypto::disclosure::{AmountRole, CounterpartyRole, DisclosureError};
use layerx_crypto::evm_transaction::{assemble, decode_signed, recover_sender, Eip1559Call};
use layerx_crypto::payments::{
    bind_call, ledger_payment_call, CallOptions, DisclosedCall, Payment,
};
use layerx_crypto::signer::{sign_disclosed_call, LocalSigner, SignError, Signer};
use layerx_types::payload::ModuleId;

const ACTOR: &[u8] = &[
    0x98, 0x58, 0xef, 0xfd, 0x23, 0x2b, 0x40, 0x33, 0xe4, 0x7d, 0x90, 0x00, 0x3d, 0x41, 0xec, 0x34,
    0xec, 0xae, 0xda, 0x94,
];
const VECTORS: &[(ModuleId, u16, &str)] = &[
    (
        ModuleId::Asset,
        1,
        include_str!("fixtures/payments/1-1-evm.hex"),
    ),
    (
        ModuleId::Asset,
        2,
        include_str!("fixtures/payments/1-2.hex"),
    ),
    (
        ModuleId::Asset,
        3,
        include_str!("fixtures/payments/1-3.hex"),
    ),
    (
        ModuleId::Asset,
        4,
        include_str!("fixtures/payments/1-4.hex"),
    ),
    (
        ModuleId::Asset,
        6,
        include_str!("fixtures/payments/1-6.hex"),
    ),
    (
        ModuleId::Asset,
        7,
        include_str!("fixtures/payments/1-7.hex"),
    ),
    (
        ModuleId::Asset,
        8,
        include_str!("fixtures/payments/1-8.hex"),
    ),
    (
        ModuleId::Asset,
        10,
        include_str!("fixtures/payments/1-10.hex"),
    ),
    (
        ModuleId::Asset,
        11,
        include_str!("fixtures/payments/1-11.hex"),
    ),
    (
        ModuleId::Programs,
        5,
        include_str!("fixtures/payments/9-5.hex"),
    ),
    (
        ModuleId::Programs,
        6,
        include_str!("fixtures/payments/9-6.hex"),
    ),
];

fn hex(s: &str) -> Vec<u8> {
    s.trim()
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let Ok(s) = std::str::from_utf8(pair) else {
                panic!("fixture UTF-8")
            };
            let Ok(b) = u8::from_str_radix(s, 16) else {
                panic!("fixture hex")
            };
            b
        })
        .collect()
}

struct ThreadWake(std::thread::Thread);

impl Wake for ThreadWake {
    fn wake(self: Arc<Self>) {
        self.0.unpark();
    }
}

pub(crate) fn complete<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => return value,
            Poll::Pending => std::thread::park(),
        }
    }
}

pub(crate) fn signer() -> Result<LocalSigner, Box<dyn std::error::Error>> {
    let account = derive_from_mnemonic(
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
        "",
        0,
    )?;
    assert_eq!(account.evm_address().as_slice(), ACTOR);
    Ok(LocalSigner::from_derived_account(&account))
}

pub(crate) fn options(signer: &LocalSigner) -> CallOptions {
    CallOptions {
        from: signer.evm_address().expect("derived EVM key"),
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    }
}

pub(crate) fn send_fixture(options: &CallOptions) -> Result<DisclosedCall, DisclosureError> {
    ledger_payment_call(
        &hex(include_str!("fixtures/payments/native-1-5.hex")),
        options,
    )
}

fn payment_call(
    payment: &Payment,
    options: &CallOptions,
) -> Result<DisclosedCall, DisclosureError> {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
    DisclosedCall::from_call(
        Eip1559Call {
            chain_id: options.chain_id,
            nonce: options.nonce,
            max_priority_fee_per_gas: options.max_priority_fee_per_gas,
            max_fee_per_gas: options.max_fee_per_gas,
            gas_limit: options.gas_limit,
            to,
            data: encode_payment_call(payment, ACTOR)?,
        },
        options.from,
    )
}

fn payload_only(payment: &Payment) -> bool {
    matches!(
        payment,
        Payment::Pause { .. }
            | Payment::Unpause { .. }
            | Payment::ProgramTransfer { .. }
            | Payment::ProgramAccount { .. }
    )
}

#[test]
fn fixtures_roundtrip_and_bind_all_fields() -> Result<(), Box<dyn std::error::Error>> {
    let signer = signer()?;
    let options = options(&signer);
    for &(module, ordinal, fixture) in VECTORS {
        let payload = hex(fixture);
        let payment = Payment::decode(module, ordinal, &payload, ACTOR)?;
        assert_eq!(payment.encode(ACTOR)?, payload);
        if payload_only(&payment) {
            assert!(payment_call(&payment, &options).is_err());
        } else {
            let prepared = payment_call(&payment, &options)?;
            let mut disclosure = bind_call(&prepared.call, options.from)?;
            assert_eq!(disclosure, prepared.disclosure);
            if matches!(payment, Payment::Receive { .. }) {
                assert!(matches!(
                    disclosure.construct_method()?,
                    Some(ConstructMethod::LedgerDraw { .. })
                ));
            } else {
                assert_eq!(disclosure.payment, Some(payment));
                disclosure.payment = None;
                assert!(reencode(&disclosure).is_err());
                disclosure = prepared.disclosure.clone();
            }
            assert_eq!(reencode(&disclosure)?, prepared.call.encode_unsigned());
            let _ = disclosure.audit_digest()?;
            let signature = complete(sign_disclosed_call(
                &signer,
                &prepared.call,
                &disclosure,
                options.from,
            ))?;
            let (call, signature) = decode_signed(&assemble(&prepared.call, &signature))?;
            assert_eq!(call, prepared.call);
            assert_eq!(recover_sender(&call, &signature)?, options.from);
        }
        for length in 0..payload.len() {
            assert!(Payment::decode(module, ordinal, &payload[..length], ACTOR).is_err());
        }
        let mut extra = payload;
        extra.push(0);
        assert!(Payment::decode(module, ordinal, &extra, ACTOR).is_err());
    }
    Ok(())
}

#[test]
fn monetary_disclosures_name_every_limit_and_transfer_party(
) -> Result<(), Box<dyn std::error::Error>> {
    let signer = signer()?;
    let options = options(&signer);
    for &(module, ordinal, fixture) in VECTORS {
        let payment = Payment::decode(module, ordinal, &hex(fixture), ACTOR)?;
        match &payment {
            Payment::Register(registration) => {
                let disclosure = payment_call(&payment, &options)?.disclosure;
                assert_eq!(
                    disclosure.amounts,
                    [layerx_crypto::disclosure::DisclosedAmount {
                        role: AmountRole::SupplyCap,
                        value: registration.supply_cap,
                    }]
                );
            }
            Payment::Receive {
                from,
                to,
                amount,
                payer_grant,
                ..
            } => {
                let draw = payment_call(&payment, &options)?.disclosure;
                assert_eq!(draw.counterparties.len(), 1);
                assert_eq!(draw.counterparties[0].role, CounterpartyRole::Recipient);
                assert_eq!(draw.counterparties[0].account, *to);
                assert_eq!(draw.amounts[0].value, *amount);
                // The maintained draw ABI carries the grant ID; its limits and payer
                // are disclosed by the separately encoded and verified grant.
                let disclosure =
                    payment_call(&Payment::IssueGrant((**payer_grant).clone()), &options)?
                        .disclosure;
                assert_eq!(disclosure.expiry.payload_expires_at, payer_grant.expiration);
                assert_eq!(disclosure.counterparties.len(), 2);
                assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
                assert_eq!(disclosure.counterparties[0].account, *from);
                assert_eq!(
                    disclosure.counterparties[1].role,
                    CounterpartyRole::Recipient
                );
                assert_eq!(disclosure.counterparties[1].account, *to);
                assert_eq!(
                    std::iter::once(draw.amounts[0].value)
                        .chain(disclosure.amounts.iter().map(|entry| entry.value))
                        .collect::<Vec<_>>(),
                    [*amount, payer_grant.per_draw_maximum, payer_grant.allowance]
                );
            }
            Payment::IssueGrant(grant) => {
                let disclosure = payment_call(&payment, &options)?.disclosure;
                assert_eq!(disclosure.expiry.payload_expires_at, grant.expiration);
                assert_eq!(disclosure.counterparties.len(), 2);
                assert_eq!(disclosure.counterparties[0].account, grant.from);
                assert_eq!(disclosure.counterparties[1].account, grant.recipient);
                assert_eq!(
                    disclosure
                        .amounts
                        .iter()
                        .map(|entry| entry.value)
                        .collect::<Vec<_>>(),
                    [grant.per_draw_maximum, grant.allowance]
                );
            }
            Payment::ProgramTransfer { legs, .. } => {
                assert!(payment_call(&payment, &options).is_err());
                // Preserve every leg's payer, recipient and amount checks using the
                // maintained ledger encodings; this does not submit the legacy program.
                let mut counterparties = Vec::new();
                let mut amounts = Vec::new();
                for leg in legs {
                    let burn = payment_call(
                        &Payment::Burn {
                            asset: leg.asset,
                            from: leg.from,
                            amount: leg.amount,
                        },
                        &options,
                    )?;
                    let mint = payment_call(
                        &Payment::Mint {
                            asset: leg.asset,
                            to: leg.to,
                            amount: leg.amount,
                        },
                        &options,
                    )?;
                    assert_eq!(burn.disclosure.amounts, mint.disclosure.amounts);
                    counterparties.extend(burn.disclosure.counterparties);
                    counterparties.extend(mint.disclosure.counterparties);
                    amounts.extend(mint.disclosure.amounts);
                }
                assert_eq!(counterparties.len(), legs.len() * 2);
                assert_eq!(amounts.len(), legs.len());
                for (index, leg) in legs.iter().enumerate() {
                    assert_eq!(counterparties[index * 2].account, leg.from);
                    assert_eq!(counterparties[index * 2 + 1].account, leg.to);
                    assert_eq!(amounts[index].value, leg.amount);
                }
            }
            _ => {}
        }
    }
    Ok(())
}

#[test]
fn invalid_shapes_are_refused() -> Result<(), Box<dyn std::error::Error>> {
    assert!(Payment::decode(ModuleId::Asset, 9, &[0; 80], ACTOR).is_err());
    for &(module, ordinal, fixture) in VECTORS {
        let payload = hex(fixture);
        let mut p = Payment::decode(module, ordinal, &payload, ACTOR)?;
        match &mut p {
            Payment::Register(r) => r.decimals = 39,
            Payment::Receive { amount, .. }
            | Payment::Mint { amount, .. }
            | Payment::Burn { amount, .. } => *amount = 0,
            Payment::IssueGrant(g) => g.per_draw_maximum = 0,
            Payment::ProgramTransfer { legs, .. } => legs.clear(),
            Payment::ProgramAccount { seed, .. } => *seed = vec![0; 129],
            Payment::OpenAccount { .. }
            | Payment::Pause { .. }
            | Payment::Unpause { .. }
            | Payment::RevokeGrant { .. } => continue,
        }
        assert!(p.encode(ACTOR).is_err());
    }
    let p = hex(VECTORS[0].2);
    assert!(Payment::decode(ModuleId::Asset, 1, &p, b"did:layerx:bob").is_err());
    Ok(())
}

#[test]
fn every_accepted_field_mutation_is_refused_at_the_signing_boundary(
) -> Result<(), Box<dyn std::error::Error>> {
    let signer = signer()?;
    let options = options(&signer);
    for &(module, ordinal, fixture) in VECTORS {
        let payload = hex(fixture);
        let payment = Payment::decode(module, ordinal, &payload, ACTOR)?;
        let prepared = if payload_only(&payment) {
            assert!(payment_call(&payment, &options).is_err());
            send_fixture(&options)?
        } else {
            payment_call(&payment, &options)?
        };
        assert!(complete(sign_disclosed_call(
            &signer,
            &prepared.call,
            &prepared.disclosure,
            options.from
        ))
        .is_ok());
        for offset in 0..payload.len() {
            let mut changed = payload.clone();
            changed[offset] ^= 1;
            if let Ok(payment) = Payment::decode(module, ordinal, &changed, ACTOR) {
                let mut altered = prepared.disclosure.clone();
                altered.payment = Some(payment);
                assert!(
                    complete(sign_disclosed_call(
                        &signer,
                        &prepared.call,
                        &altered,
                        options.from
                    ))
                    .is_err(),
                    "undisclosed field {module:?}/{ordinal} at {offset}"
                );
            }
        }
    }
    Ok(())
}

#[test]
fn pause_and_unpause_share_the_open_account_body_and_stay_distinct(
) -> Result<(), Box<dyn std::error::Error>> {
    let pause = hex(VECTORS[1].2);
    let unpause = hex(VECTORS[2].2);
    let open = hex(VECTORS[3].2);
    assert_eq!(pause.len(), 34);
    assert_eq!(pause, unpause);
    assert_eq!(pause, open);
    let mut decoded = Vec::new();
    for ordinal in [2, 3, 4] {
        let payment = Payment::decode(ModuleId::Asset, ordinal, &pause, ACTOR)?;
        assert_eq!(payment.activity_type(), (ModuleId::Asset, ordinal));
        assert_eq!(payment.encode(ACTOR)?, pause);
        let signer = signer()?;
        let options = options(&signer);
        if ordinal == 4 {
            let disclosure = payment_call(&payment, &options)?.disclosure;
            assert_eq!(disclosure.payment.as_ref(), Some(&payment));
        } else {
            assert!(payment_call(&payment, &options).is_err());
        }
        decoded.push(payment);
    }
    assert_ne!(decoded[0], decoded[1]);
    assert_ne!(decoded[1], decoded[2]);
    assert_ne!(decoded[0], decoded[2]);
    Ok(())
}

#[test]
fn receive_discloses_source_sequence_independently() -> Result<(), Box<dyn std::error::Error>> {
    let payload = hex(include_str!("fixtures/payments/1-6-source-sequence-23.hex"));
    let payment = Payment::decode(ModuleId::Asset, 6, &payload, ACTOR)?;
    assert_eq!(payment.encode(ACTOR)?, payload);
    let signer = signer()?;
    let prepared = payment_call(&payment, &options(&signer))?;
    let disclosure = bind_call(&prepared.call, options(&signer).from)?;
    assert_eq!(disclosure.envelope_sequence(), 7);
    assert_eq!(disclosure.payload_sequence()?, Some(23));
    assert_eq!(reencode(&disclosure)?, prepared.call.encode_unsigned());
    Ok(())
}

#[test]
fn payer_grant_signature_and_identifier_are_bound() {
    let payload = hex(VECTORS[5].2);
    assert_eq!(payload.len(), 346);
    for offset in 0..payload.len() {
        let mut changed = payload.clone();
        changed[offset] ^= 1;
        assert!(Payment::decode(ModuleId::Asset, 7, &changed, ACTOR).is_err());
    }
}

#[test]
fn receive_rejects_forged_authorization_and_grant() {
    let payload = hex(VECTORS[4].2);
    assert_eq!(payload.len(), 733);
    for offset in 0..payload.len() {
        let mut changed = payload.clone();
        changed[offset] ^= 1;
        assert!(
            Payment::decode(ModuleId::Asset, 6, &changed, ACTOR).is_err(),
            "offset {offset}"
        );
    }
}

#[test]
fn native_generated_receive_and_grant_are_byte_identical() -> Result<(), Box<dyn std::error::Error>>
{
    let receive = hex(include_str!("fixtures/payments/native-1-6.hex"));
    let grant = hex(include_str!("fixtures/payments/native-1-7.hex"));
    assert_eq!(receive.len(), 733);
    assert_eq!(grant.len(), 346);
    assert_eq!(&receive[387..], grant);
    for (ordinal, payload) in [(6, receive), (7, grant)] {
        let decoded = Payment::decode(ModuleId::Asset, ordinal, &payload, ACTOR)?;
        assert_eq!(decoded.encode(ACTOR)?, payload);
        for offset in 0..payload.len() {
            let mut changed = payload.clone();
            changed[offset] ^= 1;
            assert!(
                Payment::decode(ModuleId::Asset, ordinal, &changed, ACTOR).is_err(),
                "ordinal {ordinal}, offset {offset}"
            );
        }
    }
    Ok(())
}

const VERSION_PREFIXED_ASSET_ORDINALS: &[u16] = &[1, 2, 3, 4, 8, 10, 11];

#[test]
fn asset_bodies_refuse_a_version_prefix_other_than_one() {
    for &(module, ordinal, fixture) in VECTORS {
        if module != ModuleId::Asset || !VERSION_PREFIXED_ASSET_ORDINALS.contains(&ordinal) {
            continue;
        }
        let payload = hex(fixture);
        assert_eq!(&payload[0..2], &[0x00, 0x01]);
        assert!(Payment::decode(module, ordinal, &payload, ACTOR).is_ok());
        for version in [0x0000_u16, 0x0002, 0x0100, 0xffff] {
            let mut changed = payload.clone();
            changed[0..2].copy_from_slice(&version.to_be_bytes());
            assert_eq!(
                Payment::decode(module, ordinal, &changed, ACTOR),
                Err(DisclosureError::MalformedPayload),
                "{module:?}/{ordinal} version {version:#06x}"
            );
        }
    }
}

#[test]
fn receive_frame_marker_and_field_count_are_pinned() {
    let payload = hex(VECTORS[4].2);
    assert_eq!(&payload[0..4], &[0x52, 0x01, 0x00, 0x0a]);
    assert!(Payment::decode(ModuleId::Asset, 6, &payload, ACTOR).is_ok());
    for marker in [0x0000_u16, 0x0152, 0x5200, 0x5202] {
        let mut changed = payload.clone();
        changed[0..2].copy_from_slice(&marker.to_be_bytes());
        assert_eq!(
            Payment::decode(ModuleId::Asset, 6, &changed, ACTOR),
            Err(DisclosureError::MalformedPayload),
            "marker {marker:#06x}"
        );
    }
    for count in [0x0000_u16, 0x0009, 0x000b, 0xffff] {
        let mut changed = payload.clone();
        changed[2..4].copy_from_slice(&count.to_be_bytes());
        assert_eq!(
            Payment::decode(ModuleId::Asset, 6, &changed, ACTOR),
            Err(DisclosureError::MalformedPayload),
            "count {count}"
        );
    }
}

#[test]
fn program_transfer_leg_count_bounds_are_enforced() {
    let payload = hex(VECTORS[9].2);
    assert_eq!(payload.len(), 258);
    assert_eq!(&payload[32..34], &[0x00, 0x02]);
    assert!(Payment::decode(ModuleId::Programs, 5, &payload, ACTOR).is_ok());
    for count in [0x0000_u16, 0x0101, 0xffff] {
        let mut changed = payload.clone();
        changed[32..34].copy_from_slice(&count.to_be_bytes());
        assert_eq!(
            Payment::decode(ModuleId::Programs, 5, &changed, ACTOR),
            Err(DisclosureError::MalformedPayload),
            "count {count}"
        );
    }
}

#[test]
fn program_account_seed_marker_is_pinned() {
    let payload = hex(VECTORS[10].2);
    assert_eq!(payload.len(), 77);
    assert_eq!(&payload[32..37], b"LXPA1");
    assert!(Payment::decode(ModuleId::Programs, 6, &payload, ACTOR).is_ok());
    for offset in 32..37 {
        let mut changed = payload.clone();
        changed[offset] ^= 1;
        assert_eq!(
            Payment::decode(ModuleId::Programs, 6, &changed, ACTOR),
            Err(DisclosureError::MalformedPayload),
            "offset {offset}"
        );
    }
}

#[test]
fn an_unregistered_activity_names_its_type() {
    assert_eq!(
        Payment::decode(ModuleId::Asset, 9, &[0; 80], ACTOR),
        Err(DisclosureError::UnsupportedActivity(0x0001_0009))
    );
    assert_eq!(
        Payment::decode(ModuleId::Asset, 5, &[0; 80], ACTOR),
        Err(DisclosureError::UnsupportedActivity(0x0001_0005))
    );
    assert_eq!(
        Payment::decode(ModuleId::Programs, 7, &[0; 80], ACTOR),
        Err(DisclosureError::UnsupportedActivity(0x0009_0007))
    );
    assert_eq!(
        Payment::decode(ModuleId::Escrow, 1, &[0; 80], ACTOR),
        Err(DisclosureError::UnsupportedActivity(0x0002_0001))
    );
}

#[test]
fn payments_reexports_the_ledger_call_builders() -> Result<(), Box<dyn std::error::Error>> {
    use layerx_crypto::payments::{send_context_hash, Disclosure, SendCondition, SendDebit};

    let signer = signer()?;
    let options = options(&signer);
    let debit = SendDebit {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount: 123,
        source_sequence: 7,
        idempotency_key: [4; 32],
        expires_at: 100,
        context_hash: send_context_hash(&[1; 32], &[2; 32], &[3; 32], 123, &[4; 32]),
        conditions: vec![SendCondition {
            kind: 1,
            timestamp: 10,
        }],
        authorization_kind: 1,
        network_id: 17,
        protocol_version: 3,
    };
    assert_eq!(
        debit.context_hash,
        layerx_crypto::send::send_context_hash(
            &debit.from,
            &debit.to,
            &debit.asset,
            debit.amount,
            &debit.idempotency_key
        )
    );
    let payload = complete(debit.sign(&signer))?;
    let exported: DisclosedCall = ledger_payment_call(&payload, &options)?;
    let original = layerx_crypto::send::ledger_payment_call(&payload, &options)?;
    assert_eq!(exported.call.data, original.call.data);
    assert_eq!(exported.call, original.call);
    let exported_disclosure: Disclosure = bind_call(&exported.call, options.from)?;
    let original_disclosure =
        layerx_crypto::disclosure::call::bind_call(&original.call, options.from)?;
    assert_eq!(exported_disclosure, original_disclosure);
    assert_eq!(exported.disclosure, exported_disclosure);
    assert_eq!(original.disclosure, original_disclosure);
    let exported_signature = complete(sign_disclosed_call(
        &signer,
        &exported.call,
        &exported_disclosure,
        options.from,
    ))?;
    let original_signature = complete(sign_disclosed_call(
        &signer,
        &original.call,
        &original_disclosure,
        options.from,
    ))?;
    assert_eq!(exported_signature, original_signature);
    let (call, signature) = decode_signed(&assemble(&exported.call, &exported_signature))?;
    assert_eq!(call, exported.call);
    assert_eq!(recover_sender(&call, &signature)?, options.from);
    let mut changed = exported.call.clone();
    changed.nonce += 1;
    assert_eq!(
        complete(sign_disclosed_call(
            &signer,
            &changed,
            &exported_disclosure,
            options.from
        )),
        Err(SignError::DisclosureMismatch("call"))
    );
    Ok(())
}

#[test]
fn registration_fixtures_bind_their_issuer_and_preserve_legacy_payload_checks(
) -> Result<(), Box<dyn std::error::Error>> {
    let signer = signer()?;
    let options = options(&signer);
    let prepared = payment_call(
        &Payment::decode(ModuleId::Asset, 1, &hex(VECTORS[0].2), ACTOR)?,
        &options,
    )?;
    let native_actor = b"did:layerx:alice";
    let native_payload = hex(include_str!("fixtures/payments/1-1.hex"));
    let native = Payment::decode(ModuleId::Asset, 1, &native_payload, native_actor)?;
    assert_eq!(native.encode(native_actor)?, native_payload);
    assert!(Payment::decode(ModuleId::Asset, 1, &native_payload, ACTOR).is_err());
    let maintained = Payment::decode(ModuleId::Asset, 1, &hex(VECTORS[0].2), ACTOR)?;
    let Payment::Register(registration) = &maintained else {
        return Err("registration fixture decoded to another payment".into());
    };
    let issuer = layerx_wire::hash::did_id_for_protocol(
        &layerx_types::ids::Did::new(ACTOR).map_err(|error| format!("{error:?}"))?,
        3,
    )
    .map_err(|error| format!("{error:?}"))?;
    assert_eq!(
        registration.asset,
        layerx_crypto::payments::asset_id(&issuer, &registration.salt)
    );
    let mut produced = native.clone();
    let Payment::Register(produced_registration) = &mut produced else {
        return Err("native fixture decoded to another payment".into());
    };
    produced_registration.asset =
        layerx_crypto::payments::asset_id(&issuer, &produced_registration.salt);
    assert_eq!(produced, maintained);
    assert_eq!(produced.encode(ACTOR)?, hex(VECTORS[0].2));
    assert_eq!(prepared.disclosure.actor, options.from);
    assert_eq!(prepared.disclosure.payment.as_ref(), Some(&produced));
    let reviewed = bind_call(&prepared.call, options.from)?;
    assert_eq!(
        reviewed.audit_digest()?,
        prepared.disclosure.audit_digest()?
    );
    assert_eq!(reencode(&reviewed)?, prepared.call.encode_unsigned());
    for length in 0..native_payload.len() {
        assert!(
            Payment::decode(ModuleId::Asset, 1, &native_payload[..length], native_actor).is_err()
        );
    }
    let mut trailing = native_payload.clone();
    trailing.push(0);
    assert!(Payment::decode(ModuleId::Asset, 1, &trailing, native_actor).is_err());
    for offset in 0..native_payload.len() {
        let mut changed = native_payload.clone();
        changed[offset] ^= 1;
        if let Ok(payment) = Payment::decode(ModuleId::Asset, 1, &changed, native_actor) {
            let mut altered = prepared.disclosure.clone();
            altered.payment = Some(payment);
            assert!(
                complete(sign_disclosed_call(
                    &signer,
                    &prepared.call,
                    &altered,
                    options.from
                ))
                .is_err(),
                "legacy registration field at {offset}"
            );
        }
    }
    Ok(())
}
