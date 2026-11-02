#[path = "prepare.rs"]
mod preparation;

use layerx_agentd::prepare::evm::{prepare_bound, PrepareError};
use layerx_crypto::evm_transaction::transaction_hash;

#[test]
fn prepared_transfer_signing_hash_matches_the_eip1559_golden() {
    let prepared = prepare_bound(
        preparation::state(),
        preparation::defaults(),
        preparation::request(),
        100_000,
    )
    .unwrap_or_else(|error| panic!("prepare: {error:?}"));
    // Type 2, chain 17, nonce 5, priority 2, cap 22, gas 100000,
    // target 0x101a, value 0, send(bytes), empty access list.
    let mut golden = hex::decode(concat!(
        "02f901e611050216830186a094000000000000000000000000000000000000101a80b901c4",
        "2bb1ae7c0000000000000000000000000000000000000000000000000000000000000020",
        "000000000000000000000000000000000000000000000000000000000000016c"
    ))
    .unwrap();
    let payload = preparation::send_payload();
    assert_eq!(payload.len(), 364);
    golden.extend_from_slice(&payload);
    golden.extend_from_slice(&[0; 20]);
    golden.push(0xc0);
    assert_eq!(golden.len(), 490);
    assert_eq!(prepared.call.chain_id, 17);
    assert_eq!(prepared.call.nonce, 5);
    assert_eq!(prepared.call.max_priority_fee_per_gas, 2);
    assert_eq!(prepared.call.max_fee_per_gas, 22);
    assert_eq!(prepared.call.gas_limit, 100_000);
    assert_eq!(
        prepared.call.to,
        [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a]
    );
    assert_eq!(prepared.call.data, golden[37..489]);
    assert_eq!(prepared.signing_hash, transaction_hash(&golden));
    assert_eq!(prepared.call.signing_hash(), prepared.signing_hash);
    assert_eq!(prepared.observed_head_height, 88);
}

#[test]
fn prepare_refuses_an_expected_nonce_behind_the_pending_nonce() {
    for expected in [4, 6] {
        let mut request = preparation::request();
        request.expected_nonce = Some(expected);
        assert_eq!(
            prepare_bound(
                preparation::state(),
                preparation::defaults(),
                request,
                100_000
            ),
            Err(PrepareError::StaleNonce {
                expected,
                pending: 5
            })
        );
    }
    let mut request = preparation::request();
    request.expected_nonce = None;
    let prepared = prepare_bound(
        preparation::state(),
        preparation::defaults(),
        request,
        100_000,
    )
    .unwrap();
    assert_eq!(prepared.call.nonce, 5);
}

#[test]
fn prepare_derives_fee_caps_from_the_head_base_fee() {
    let mut request = preparation::request();
    request.max_priority_fee_per_gas = Some(3);
    request.max_fee_per_gas = Some(100);
    let prepared = prepare_bound(
        preparation::state(),
        preparation::defaults(),
        request.clone(),
        100_000,
    )
    .unwrap();
    assert_eq!(prepared.call.max_priority_fee_per_gas, 3);
    assert_eq!(prepared.call.max_fee_per_gas, 23);

    request.max_fee_per_gas = Some(22);
    assert_eq!(
        prepare_bound(
            preparation::state(),
            preparation::defaults(),
            request.clone(),
            100_000
        ),
        Err(PrepareError::InvalidDefaults)
    );
    request.max_fee_per_gas = Some(23);
    assert!(prepare_bound(
        preparation::state(),
        preparation::defaults(),
        request.clone(),
        100_000
    )
    .is_ok());

    request.max_fee_per_gas = None;
    let mut state = preparation::state();
    state.base_fee_per_gas = 20;
    let prepared = prepare_bound(state, preparation::defaults(), request.clone(), 100_000).unwrap();
    assert_eq!(prepared.call.max_fee_per_gas, 43);
    let mut defaults = preparation::defaults();
    defaults.max_fee_per_gas = 42;
    assert_eq!(
        prepare_bound(state, defaults, request.clone(), 100_000),
        Err(PrepareError::InvalidDefaults)
    );
    state.base_fee_per_gas = 0;
    let prepared = prepare_bound(state, preparation::defaults(), request.clone(), 100_000).unwrap();
    assert_eq!(prepared.call.max_fee_per_gas, 3);
    state.base_fee_per_gas = u128::MAX;
    assert_eq!(
        prepare_bound(state, preparation::defaults(), request.clone(), 100_000),
        Err(PrepareError::InvalidDefaults)
    );
    state.base_fee_per_gas = u128::MAX / 2;
    assert_eq!(
        prepare_bound(state, preparation::defaults(), request, 100_000),
        Err(PrepareError::InvalidDefaults)
    );
}
