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
    assert_eq!(scope.u32().expect("signed network"), 125);
    assert_eq!(
        scope.u16().expect("signed protocol"),
        layerx_wire::limits::PROTOCOL_VERSION
    );
    scope.finish().expect("exact authorization scope");
    assert_eq!(route.network_id.value(), 125);
    let intent = Intent::from(IntentKind::LxpSend(send));
    let compiled = compile(&intent, &proof_human_intents::registry()).expect("compile real send");
    assert_eq!(compiled.payload().as_bytes(), signed);
    let produced = ledger_payment_call(compiled.payload().as_bytes(), &options)
        .expect("produce merged-network ledger call");
    assert_eq!(produced.call.chain_id, 125);
    assert_eq!(&produced.call.to[18..], &0x101a_u16.to_be_bytes());
    for _ in 0..2 {
        let observed = bind_call(&produced.call, options.from).expect("independent consumer");
        DisclosureCheck::verify_agent(&intent, &observed).expect("intent matches received call");
        assert_eq!(
            observed.construct_method().expect("ledger ABI decode"),
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
    let produced = ledger_payment_call(&valid, &options).expect("valid network-125 producer");
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
            bind_call(&produced.call, options.from).expect("valid delivery after refusal"),
            produced.disclosure
        );
    }
    assert_eq!(
        &signed[network_offset..network_offset + 4],
        &77_u32.to_be_bytes()
    );
}
