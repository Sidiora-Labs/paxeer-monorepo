#[path = "support/send_authorization.rs"]
mod send_authorization;

use layerx_agentd::prepare::evm::{
    prepare_bound, EvmPreparationState, PrecompileCall, PreparationDefaults, PrepareError,
    PrepareRequest,
};
use layerx_wire::encode::Encoder;

pub fn state() -> EvmPreparationState {
    EvmPreparationState {
        chain_id: 17,
        pending_nonce: 5,
        base_fee_per_gas: 10,
        head_height: 88,
    }
}

pub fn defaults() -> PreparationDefaults {
    PreparationDefaults {
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 100,
        maximum_payload_bytes: 1_024,
    }
}

pub fn request() -> PrepareRequest {
    PrepareRequest {
        from: [0x11; 20],
        call: PrecompileCall::ledger_transfer(send_payload()),
        expected_nonce: Some(5),
        max_priority_fee_per_gas: None,
        max_fee_per_gas: Some(22),
        declared_payload_limit: 1_024,
    }
}

pub fn send_payload() -> Vec<u8> {
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
    send_authorization::sign(encoder.finish())
}

#[test]
fn canonical_bytes_and_preimage_come_only_from_layerx_wire() {
    let prepared = prepare_bound(state(), defaults(), request(), 100_000)
        .unwrap_or_else(|error| panic!("prepare: {error:?}"));
    assert_eq!(prepared.call.chain_id, 17);
    assert_eq!(prepared.call.nonce, 5);
    assert_eq!(prepared.call.gas_limit, 100_000);
    assert_eq!(prepared.observed_head_height, 88);
    assert_eq!(
        prepared.call.to,
        layerx_agentd::prepare::PrecompileTarget::Ledger.address()
    );
    assert_eq!(prepared.signing_hash, prepared.call.signing_hash());
    let data = &prepared.call.data;
    assert_eq!(&data[..4], &[0x2b, 0xb1, 0xae, 0x7c]);
    assert_eq!(&data[4..35], &[0; 31]);
    assert_eq!(data[35], 32);
    assert_eq!(&data[36..66], &[0; 30]);
    assert_eq!(&data[66..68], &(send_payload().len() as u16).to_be_bytes());
    assert_eq!(&data[68..68 + send_payload().len()], send_payload());
    assert!(data[68 + send_payload().len()..]
        .iter()
        .all(|byte| *byte == 0));
}

#[test]
fn stale_sequence_and_unavailable_core_are_refused_without_guessing() {
    let mut stale = request();
    stale.expected_nonce = Some(4);
    assert_eq!(
        prepare_bound(state(), defaults(), stale, 100_000),
        Err(PrepareError::StaleNonce {
            expected: 4,
            pending: 5
        })
    );
    let unavailable =
        layerx_agentd::prepare::ProductionEvmPreparationBoundary::new("http://127.0.0.1:0", &[])
            .unwrap_or_else(|error| panic!("endpoint: {error:?}"));
    assert_eq!(
        layerx_agentd::prepare::evm::prepare_activity(&unavailable, defaults(), request()),
        Err(PrepareError::Sequencer(
            layerx_client::sequencer::SequencerError::Unavailable
        ))
    );
}

#[test]
fn explicit_bounds_are_never_widened_and_payload_limits_are_exact() {
    let mut too_low = request();
    too_low.max_fee_per_gas = Some(21);
    assert_eq!(
        prepare_bound(state(), defaults(), too_low, 100_000),
        Err(PrepareError::InvalidDefaults)
    );
    let mut oversized = request();
    let actual = oversized.call.encode().unwrap().len();
    oversized.declared_payload_limit = actual - 1;
    assert_eq!(
        prepare_bound(state(), defaults(), oversized, 100_000),
        Err(PrepareError::PayloadLimitExceeded {
            actual,
            maximum: actual - 1
        })
    );
    let mut exact = request();
    exact.declared_payload_limit = actual;
    assert!(prepare_bound(state(), defaults(), exact, 100_000).is_ok());
    assert_eq!(
        prepare_bound(state(), defaults(), request(), 0),
        Err(PrepareError::InvalidDefaults)
    );
}
