use layerx_agentd::prepare::evm::{
    prepare_bound, verify_call_binding, CallBindingError, EvmPreparationState, PrecompileCall,
    PreparationDefaults, PrepareRequest, Prepared as EvmPrepared,
};
use layerx_crypto::disclosure::{call, AmountRole, CounterpartyRole, DisclosureError};
use layerx_types::payload::{ActivityType, ModuleId};
use layerx_wire::encode::Encoder;

fn activity_type() -> ActivityType {
    assert_ok(ActivityType::new(ModuleId::Asset, 5), "send activity")
}

fn send_payload() -> Vec<u8> {
    let mut encoder = Encoder::new(512);
    encoder
        .u16(0x5301)
        .unwrap_or_else(|error| panic!("tag: {error:?}"));
    encoder
        .u16(10)
        .unwrap_or_else(|error| panic!("fields: {error:?}"));
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("from: {error:?}"));
    encoder
        .fixed(&[0x22; 32])
        .unwrap_or_else(|error| panic!("to: {error:?}"));
    encoder
        .fixed(&[0x33; 32])
        .unwrap_or_else(|error| panic!("asset: {error:?}"));
    encoder
        .u128(25)
        .unwrap_or_else(|error| panic!("amount: {error:?}"));
    encoder
        .u64(5)
        .unwrap_or_else(|error| panic!("sequence: {error:?}"));
    encoder
        .fixed(&[4; 32])
        .unwrap_or_else(|error| panic!("idempotency: {error:?}"));
    encoder
        .u64(1_010)
        .unwrap_or_else(|error| panic!("expiry: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("context: {error:?}"));
    encoder
        .u8(0)
        .unwrap_or_else(|error| panic!("conditions: {error:?}"));
    encoder
        .u8(1)
        .unwrap_or_else(|error| panic!("authority kind: {error:?}"));
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("controller: {error:?}"));
    encoder
        .fixed(&[0x66; 32])
        .unwrap_or_else(|error| panic!("public key: {error:?}"));
    encoder
        .fixed(&[0x77; 64])
        .unwrap_or_else(|error| panic!("signature: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("signed context: {error:?}"));
    encoder
        .u32(17)
        .unwrap_or_else(|error| panic!("network: {error:?}"));
    encoder
        .u16(layerx_wire::limits::PROTOCOL_VERSION)
        .unwrap_or_else(|error| panic!("version: {error:?}"));
    crate::send_authorization::sign(encoder.finish())
}

fn prepared() -> EvmPrepared {
    assert_ok(
        prepare_bound(
            EvmPreparationState {
                chain_id: 17,
                pending_nonce: 5,
                base_fee_per_gas: 3,
                head_height: 88,
            },
            PreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 12,
                maximum_payload_bytes: 1_024,
            },
            PrepareRequest {
                from: [0x11; 20],
                call: PrecompileCall::ledger_transfer(send_payload()),
                expected_nonce: Some(5),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: Some(7),
                declared_payload_limit: 1_024,
            },
            100_000,
        ),
        "prepare ledger type-2 call",
    )
}

#[test]
fn preparation_discloses_semantics_decoded_from_the_bytes() {
    let prepared = prepared();
    let disclosure = &prepared.disclosure;
    let exact = prepared.call.encode_unsigned();
    assert_eq!(exact[0], 2);
    assert_eq!(
        call::bind_call(&prepared.call, prepared.from),
        Ok(disclosure.clone())
    );
    assert_eq!(disclosure.activity_type, activity_type());
    assert_eq!(disclosure.actor, [0x11; 20]);
    assert_eq!(disclosure.authority, [0x11; 20]);
    assert_eq!(disclosure.counterparties.len(), 2);
    assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
    assert_eq!(
        disclosure.counterparties[1].role,
        CounterpartyRole::Recipient
    );
    assert_eq!(disclosure.amounts.len(), 1);
    assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosure.amounts[0].value, 25);
    assert_eq!(disclosure.asset, [0x33; 32]);
    assert_eq!(disclosure.fee_limit, 700_000);
    assert_eq!(disclosure.expiry.payload_expires_at, 1_010);
    assert_eq!(disclosure.expiry.not_before, 0);
    assert_eq!(disclosure.expiry.not_after, u64::MAX);
    assert_eq!(disclosure.idempotency_key, [4; 32]);
    assert_eq!(call::reencode(disclosure), Ok(exact.clone()));
    assert_eq!(call::validate_canonical(disclosure, &exact), Ok(()));
    assert_eq!(disclosure.signing_digest(), Ok(prepared.signing_hash));
    assert_eq!(prepared.disclosure_digest, prepared.signing_hash);
    assert_eq!(verify_call_binding(&prepared), Ok(()));
    assert_eq!(prepared.call.chain_id, 17);
    assert_eq!(prepared.call.nonce, 5);
    assert_eq!(prepared.observed_head_height, 88);
}

#[test]
fn mutation_after_disclosure_generation_is_refused() {
    let original = prepared();
    let mut exact = original.call.encode_unsigned();
    let last = exact.len() - 1;
    exact[last] ^= 1;
    assert_eq!(
        call::validate_canonical(&original.disclosure, &exact),
        Err(DisclosureError::FieldMismatch("canonical_bytes"))
    );
    let mut changed = original.clone();
    changed.call.data[68 + 100] ^= 1; // Exact ledger amount byte.
    assert_eq!(
        verify_call_binding(&changed),
        Err(CallBindingError::SigningHashMismatch)
    );
    changed.signing_hash = changed.call.signing_hash();
    assert!(
        verify_call_binding(&changed).is_err(),
        "updating the hash cannot retain stale semantics"
    );
    let mut changed = original.clone();
    changed.disclosure.amounts[0].value += 1;
    assert!(verify_call_binding(&changed).is_err());
    let mut changed = original;
    changed.from[0] ^= 1;
    assert!(verify_call_binding(&changed).is_err());
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}
