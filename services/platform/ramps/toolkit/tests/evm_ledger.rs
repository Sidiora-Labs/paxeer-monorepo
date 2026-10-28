use layerx_client::sequencer::SequencerLog;
use layerx_ramp_toolkit::evm::{
    decode_ledger_payment, encode_draw, encode_pay_exact, LedgerPayment, LEDGER_ADDRESS,
};
use sha3::{Digest as _, Keccak256};

#[test]
fn pay_exact_calldata_uses_ledger_vector_selector() {
    let from = [0x11; 32];
    let to = [0x22; 32];
    let asset = [0x33; 32];
    let reference = [0x44; 32];
    for (amount, expires_at) in [(1_000_000, 7), (0, 0), (u128::MAX, u64::MAX)] {
        let data = encode_pay_exact(from, to, asset, amount, expires_at, reference);
        assert_eq!(data.len(), 4 + 6 * 32);
        assert_eq!(data[..4], [0x78, 0xea, 0x11, 0xdb]);
        assert_eq!(data[4..36], from);
        assert_eq!(data[36..68], to);
        assert_eq!(data[68..100], asset);
        assert_eq!(data[100..116], [0; 16]);
        assert_eq!(data[116..132], amount.to_be_bytes());
        assert_eq!(data[132..156], [0; 24]);
        assert_eq!(data[156..164], expires_at.to_be_bytes());
        assert_eq!(data[164..196], reference);
    }
}

#[test]
fn draw_calldata_places_seven_contract_words() {
    let from = [0x11; 32];
    let asset = [0x22; 32];
    let grant = [0x44; 32];
    let idempotency_key = [0x66; 32];
    let context = [0x77; 32];
    for (amount, receiver_sequence) in [
        (
            0x0102_0304_0506_0708_090a_0b0c_0d0e_0f10,
            0x1122_3344_5566_7788,
        ),
        (0, 0),
        (u128::MAX, u64::MAX),
    ] {
        let data = encode_draw(
            from,
            asset,
            amount,
            grant,
            receiver_sequence,
            idempotency_key,
            context,
        );
        assert_eq!(data.len(), 4 + 7 * 32);
        assert_eq!(data[..4], [0x2e, 0x89, 0x49, 0x0f]);
        assert_eq!(data[4..36], from);
        assert_eq!(data[36..68], asset);
        assert_eq!(data[68..84], [0; 16]);
        assert_eq!(data[84..100], amount.to_be_bytes());
        assert_eq!(data[100..132], grant);
        assert_eq!(data[132..156], [0; 24]);
        assert_eq!(data[156..164], receiver_sequence.to_be_bytes());
        assert_eq!(data[164..196], idempotency_key);
        assert_eq!(data[196..228], context);
    }
}

#[test]
fn payment_exact_log_matches_native_ledger_abi() {
    let abi: serde_json::Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxledger/abi.json"
    ))
    .expect("native ledger ABI");
    let event = abi
        .as_array()
        .expect("ABI definitions")
        .iter()
        .find(|entry| entry["type"] == "event" && entry["name"] == "PaymentExact")
        .expect("native PaymentExact event");
    let from = [0x11; 32];
    let to = [0x22; 32];
    let reference = [0x33; 32];
    let asset = [0x44; 32];
    let amount = u128::MAX;
    let inputs = event["inputs"].as_array().expect("event inputs");
    let types: Vec<_> = inputs
        .iter()
        .map(|input| input["type"].as_str().expect("ABI type"))
        .collect();
    let signature = format!("PaymentExact({})", types.join(","));
    let mut topics = vec![Keccak256::digest(signature.as_bytes()).into()];
    let mut data = Vec::new();
    for input in inputs {
        let word = match input["name"].as_str().expect("ABI field name") {
            "paymentReference" => reference,
            "from" => from,
            "to" => to,
            "assetId" => asset,
            "amount" => {
                let mut word = [0; 32];
                word[16..].copy_from_slice(&amount.to_be_bytes());
                word
            }
            name => panic!("unexpected PaymentExact field: {name}"),
        };
        if input["indexed"].as_bool().expect("indexed flag") {
            topics.push(word);
        } else {
            data.extend_from_slice(&word);
        }
    }
    let log = SequencerLog {
        address: LEDGER_ADDRESS,
        topics,
        data,
        log_index: 0,
    };
    assert_eq!(
        decode_ledger_payment(&log),
        Some(LedgerPayment::Exact {
            from,
            to,
            reference,
            asset,
            amount,
        })
    );
}

#[test]
fn ledger_payment_log_from_other_address_is_ignored() {
    let from = [0x11; 32];
    let to = [0x22; 32];
    let reference = [0x33; 32];
    let asset = [0x44; 32];
    let grant = [0x55; 32];
    let context = [0x66; 32];
    let exact_topic = [
        0xc4, 0xda, 0x5d, 0x6f, 0x75, 0x7c, 0x25, 0x1c, 0xd7, 0x09, 0x47, 0xb7, 0x52, 0x91, 0x7e,
        0x56, 0x71, 0xb6, 0x77, 0x51, 0x7e, 0x3d, 0x95, 0x3f, 0x36, 0x2a, 0x1d, 0xf6, 0x64, 0x8e,
        0xf5, 0xb1,
    ];
    let drawn_topic = [
        0x13, 0x47, 0x3d, 0x10, 0x3a, 0x2d, 0x52, 0x17, 0x20, 0x16, 0xc8, 0x72, 0xee, 0xae, 0x01,
        0x53, 0xe2, 0x54, 0xaf, 0x3c, 0x21, 0xa1, 0xd9, 0x87, 0x49, 0x2a, 0xb7, 0x37, 0xf1, 0xa3,
        0x4f, 0xe1,
    ];
    assert_eq!(LEDGER_ADDRESS[..18], [0; 18]);
    assert_eq!(LEDGER_ADDRESS[18..], [0x10, 0x1a]);
    assert_eq!(
        exact_topic.as_slice(),
        Keccak256::digest(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)").as_slice(),
    );
    assert_eq!(
        drawn_topic.as_slice(),
        Keccak256::digest(b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)")
            .as_slice(),
    );
    for amount in [0, 1_000_000, u128::MAX] {
        let mut data = asset.to_vec();
        data.extend_from_slice(&[0; 16]);
        data.extend_from_slice(&amount.to_be_bytes());
        let exact = SequencerLog {
            address: LEDGER_ADDRESS,
            topics: vec![exact_topic, reference, from, to],
            data,
            log_index: 0,
        };
        assert_eq!(
            decode_ledger_payment(&exact),
            Some(LedgerPayment::Exact {
                from,
                to,
                reference,
                asset,
                amount,
            })
        );
        let mut drawn = exact.clone();
        drawn.topics = vec![drawn_topic, grant, from, to];
        drawn.data.extend_from_slice(&reference);
        drawn.data.extend_from_slice(&context);
        assert_eq!(
            decode_ledger_payment(&drawn),
            Some(LedgerPayment::Drawn {
                from,
                to,
                grant,
                asset,
                amount,
                idempotency_key: reference,
                context,
            })
        );
        for log in [exact, drawn] {
            for byte in 0..20 {
                let mut other = log.clone();
                other.address[byte] ^= 1;
                assert_eq!(decode_ledger_payment(&other), None);
            }
            let mut other = log.clone();
            other.topics[0] = [0; 32];
            assert_eq!(decode_ledger_payment(&other), None);
            for topics in 0..4 {
                other = log.clone();
                other.topics.truncate(topics);
                assert_eq!(decode_ledger_payment(&other), None);
            }
            other = log.clone();
            other.topics.push([0; 32]);
            assert_eq!(decode_ledger_payment(&other), None);
            for length in 0..log.data.len() {
                other = log.clone();
                other.data.truncate(length);
                assert_eq!(decode_ledger_payment(&other), None);
            }
            other = log.clone();
            other.data.push(0);
            assert_eq!(decode_ledger_payment(&other), None);
            for byte in 32..48 {
                other = log.clone();
                other.data[byte] = 1;
                assert_eq!(decode_ledger_payment(&other), None);
            }
        }
    }
}
