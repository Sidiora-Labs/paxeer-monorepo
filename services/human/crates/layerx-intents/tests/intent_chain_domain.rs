#[path = "proof_human_intents.rs"]
mod proof_human_intents;

use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::send::{ledger_payment_call, CallOptions};
use layerx_intents::{compile, DisclosureCheck, Intent, IntentKind};
use layerx_wire::decode::Decoder;

fn call_options() -> CallOptions {
    CallOptions {
        from: [0x24; 20],
        chain_id: 125,
        nonce: 11,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 7,
        gas_limit: 100_000,
    }
}

#[test]
fn intent_proof_chain_domain_merged_network_producer_and_consumer_agree() {
    let (send, route, signed) = proof_human_intents::signed_send();
    let options = call_options();
    let mut scope = Decoder::new(&signed[signed.len() - 6..], 0);
    assert_eq!(
        scope
            .u32()
            .unwrap_or_else(|error| panic!("signed network: {error:?}")),
        125
    );
    assert_eq!(
        scope
            .u16()
            .unwrap_or_else(|error| panic!("signed protocol: {error:?}")),
        layerx_wire::limits::PROTOCOL_VERSION
    );
    scope
        .finish()
        .unwrap_or_else(|error| panic!("exact authorization scope: {error:?}"));
    assert_eq!(route.network_id.value(), 125);
    let intent = Intent::from(IntentKind::LxpSend(send));
    let compiled = compile(&intent, &proof_human_intents::registry())
        .unwrap_or_else(|error| panic!("compile real send: {error:?}"));
    assert_eq!(compiled.payload().as_bytes(), signed);
    let produced = ledger_payment_call(compiled.payload().as_bytes(), &options)
        .unwrap_or_else(|error| panic!("produce merged-network ledger call: {error:?}"));
    assert_eq!(produced.call.chain_id, 125);
    assert_eq!(&produced.call.to[18..], &0x101a_u16.to_be_bytes());
    for _ in 0..2 {
        let observed = bind_call(&produced.call, options.from)
            .unwrap_or_else(|error| panic!("independent consumer: {error:?}"));
        DisclosureCheck::verify_agent(&intent, &observed)
            .unwrap_or_else(|error| panic!("intent matches received call: {error:?}"));
        assert_eq!(
            observed
                .construct_method()
                .unwrap_or_else(|error| panic!("ledger ABI decode: {error:?}")),
            Some(ConstructMethod::LedgerSend {
                payload: signed.clone()
            })
        );
        assert_eq!(observed, produced.disclosure);
    }
}

#[test]
fn intent_proof_chain_domain_network_77_signature_cannot_be_relabelled_as_125() {
    let (_, route, signed) = proof_human_intents::signed_send_for_network(77);
    assert_eq!(route.network_id.value(), 77);
    let options = call_options();
    let mut relabelled = signed.clone();
    let network_offset = relabelled.len() - 6;
    assert_eq!(
        &relabelled[network_offset..network_offset + 4],
        &77_u32.to_be_bytes()
    );
    relabelled[network_offset..network_offset + 4].copy_from_slice(&125_u32.to_be_bytes());
    assert!(matches!(
        ledger_payment_call(&relabelled, &options),
        Err(DisclosureError::MalformedPayload)
    ));

    let (_, _, valid) = proof_human_intents::signed_send();
    let produced = ledger_payment_call(&valid, &options)
        .unwrap_or_else(|error| panic!("valid network-125 producer: {error:?}"));
    for _ in 0..2 {
        let mut received = produced.call.clone();
        let payload_offset = 4 + 32 + 32;
        assert_eq!(
            &received.data[payload_offset..payload_offset + valid.len()],
            valid
        );
        received.data[payload_offset..payload_offset + valid.len()].copy_from_slice(&relabelled);
        assert_eq!(
            bind_call(&received, options.from),
            Err(DisclosureError::MalformedPayload)
        );
        assert_eq!(
            bind_call(&produced.call, options.from)
                .unwrap_or_else(|error| panic!("valid delivery after refusal: {error:?}")),
            produced.disclosure
        );
    }
    assert_eq!(
        &signed[network_offset..network_offset + 4],
        &77_u32.to_be_bytes()
    );
}
