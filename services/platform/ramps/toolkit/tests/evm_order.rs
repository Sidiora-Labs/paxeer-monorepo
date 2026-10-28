use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerLog};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_proof::receipt::ReceiptCheck;
use layerx_ramp_toolkit::evm::LEDGER_ADDRESS;
use layerx_ramp_toolkit::{
    compile_operator_send, compile_payer_grant_draw, verify_order_receipt, AuthenticatedPrincipal,
    CreateOrder, OperatorIdentity, QuoteTerms, RampDirection, RampError, RampOrder,
    ReceiptEvidence,
};
use layerx_types::account::AccountId;
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};
use sha3::Keccak256;

fn order(direction: RampDirection) -> RampOrder {
    RampOrder::bind(
        CreateOrder {
            order_id: "order-1".into(),
            quote_id: "quote-1".into(),
            payer_grant: (direction == RampDirection::OffRamp).then_some([7; 32]),
            payer_grant_purpose: (direction == RampDirection::OffRamp).then_some([6; 32]),
        },
        QuoteTerms {
            quote_id: "quote-1".into(),
            direction,
            layerx_asset: [1; 32],
            layerx_amount: 1_000,
            external_currency: "EUR".into(),
            external_amount_minor: 100,
            rate_numerator: 10,
            rate_denominator: 1,
            fee_minor: 2,
            maximum_slippage_bps: 25,
            context: [8; 32],
            provider_token: "product-eur".into(),
            payout_token: "beneficiary-123".into(),
            expires_at: 2_000,
        },
        AuthenticatedPrincipal {
            principal_id: "customer-1".into(),
            account: "agent:did:layerx:customer-1:main".into(),
        },
        OperatorIdentity {
            principal_id: "operator-1".into(),
            account: "agent:did:layerx:operator-1:main".into(),
            signer_key_handle: "kms.operator-1.primary".into(),
        },
        1_000,
    )
    .expect("bound ramp order")
}

fn account_id(account: &str) -> [u8; 32] {
    layerx_wire::hash::account_id(&AccountId::parse(account).expect("account"))
        .expect("ledger account id")
}

#[test]
fn on_ramp_compiles_pay_exact_with_order_reference() {
    let order = order(RampDirection::OnRamp);
    let call = compile_operator_send(&order).expect("operator ledger call");
    assert_eq!(call.to, LEDGER_ADDRESS);
    assert_eq!(&call.to[18..], &[0x10, 0x1a]);
    assert_eq!(call.data.len(), 196);
    assert_eq!(&call.data[..4], &[0x78, 0xea, 0x11, 0xdb]);
    assert_eq!(&call.data[4..36], &account_id(&order.operator.account));
    assert_eq!(&call.data[36..68], &account_id(&order.customer.account));
    assert_eq!(&call.data[68..100], &order.quote.layerx_asset);
    assert_eq!(&call.data[100..116], &[0; 16]);
    assert_eq!(
        &call.data[116..132],
        &order.quote.layerx_amount.to_be_bytes()
    );
    assert_eq!(&call.data[132..156], &[0; 24]);
    assert_eq!(&call.data[156..164], &order.quote.expires_at.to_be_bytes());
    assert_eq!(&call.data[164..196], &order.order_digest);
    assert_eq!(
        compile_operator_send(&self::order(RampDirection::OffRamp)),
        Err(RampError::InvalidOrder)
    );
}

#[test]
fn off_ramp_compiles_draw_against_customer_grant() {
    let order = order(RampDirection::OffRamp);
    let call = compile_payer_grant_draw(&order, 9).expect("customer grant draw");
    assert_eq!(call.to, LEDGER_ADDRESS);
    assert_eq!(call.data.len(), 228);
    assert_eq!(
        &call.data[..4],
        &Keccak256::digest(b"draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)")[..4]
    );
    assert_eq!(&call.data[4..36], &order.payer_grant.expect("grant"));
    assert_eq!(
        &call.data[36..68],
        &account_id(&format!(
            "agent:did:layerx:operator-1:asset:{}",
            "01".repeat(32)
        ))
    );
    assert_eq!(&call.data[68..84], &[0; 16]);
    assert_eq!(
        &call.data[84..100],
        &order.quote.layerx_amount.to_be_bytes()
    );
    assert_eq!(&call.data[100..132], &order.order_digest);
    assert_eq!(&call.data[132..156], &[0; 24]);
    assert_eq!(&call.data[156..164], &9_u64.to_be_bytes());
    assert_eq!(
        &call.data[164..196],
        &order.payer_grant_purpose.expect("grant purpose")
    );
    assert_eq!(&call.data[196..228], &order.context);
    assert_eq!(
        compile_payer_grant_draw(&self::order(RampDirection::OnRamp), 9),
        Err(RampError::PayerGrantRequired)
    );
    let mut missing = order.clone();
    missing.payer_grant = None;
    missing.order_digest = missing.digest();
    assert_eq!(
        compile_payer_grant_draw(&missing, 9),
        Err(RampError::PayerGrantRequired)
    );
    for purpose in [None, Some([0; 32])] {
        let mut missing = order.clone();
        missing.payer_grant_purpose = purpose;
        missing.order_digest = missing.digest();
        assert_eq!(
            compile_payer_grant_draw(&missing, 9),
            Err(RampError::PayerGrantRequired)
        );
    }
    let mut tampered = order.clone();
    tampered.payer_grant_purpose = Some([5; 32]);
    assert_ne!(tampered.digest(), order.order_digest);
    assert_eq!(
        compile_payer_grant_draw(&tampered, 9),
        Err(RampError::OrderBinding)
    );
}

#[test]
fn off_ramp_draw_follows_the_native_ledger_abi() {
    let abi: serde_json::Value = serde_json::from_str(
        &std::fs::read_to_string(
            std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../../../precompiles/layerxledger/abi.json"),
        )
        .expect("native ledger ABI"),
    )
    .expect("ledger ABI JSON");
    let draw = abi
        .as_array()
        .expect("ABI entries")
        .iter()
        .find(|entry| entry["type"] == "function" && entry["name"] == "draw")
        .expect("native draw function");
    let inputs = draw["inputs"].as_array().expect("draw inputs");
    let names: Vec<_> = inputs
        .iter()
        .map(|input| input["name"].as_str().expect("input name"))
        .collect();
    assert_eq!(
        names,
        [
            "grantId",
            "recipient",
            "amount",
            "idempotencyKey",
            "sequence",
            "purposeHash",
            "contextHash"
        ]
    );

    let mut order = order(RampDirection::OffRamp);
    order.quote.layerx_asset = [0xab; 32];
    order.quote.layerx_amount = (1_u128 << 100) + 1_000;
    order.order_digest = order.digest();
    let sequence = 0x0102_0304_0506_0708_u64;
    let call = compile_payer_grant_draw(&order, sequence).expect("native draw call");
    let mut amount = [0; 32];
    amount[16..].copy_from_slice(&order.quote.layerx_amount.to_be_bytes());
    let mut sequence_word = [0; 32];
    sequence_word[24..].copy_from_slice(&sequence.to_be_bytes());
    let expected = [
        order.payer_grant.expect("payer grant"),
        account_id(&format!(
            "agent:did:layerx:operator-1:asset:{}",
            "ab".repeat(32)
        )),
        amount,
        order.order_digest,
        sequence_word,
        order.payer_grant_purpose.expect("grant purpose"),
        order.context,
    ];
    let signature = format!(
        "draw({})",
        inputs
            .iter()
            .map(|input| input["type"].as_str().expect("input type"))
            .collect::<Vec<_>>()
            .join(",")
    );
    assert_eq!(call.to, LEDGER_ADDRESS);
    assert_eq!(call.data.len(), 4 + expected.len() * 32);
    assert_eq!(
        &call.data[..4],
        &Keccak256::digest(signature.as_bytes())[..4]
    );
    for (index, word) in expected.iter().enumerate() {
        assert_eq!(
            &call.data[4 + index * 32..4 + (index + 1) * 32],
            word,
            "{}",
            names[index]
        );
    }

    for direction in [RampDirection::OnRamp, RampDirection::OffRamp] {
        let bound = self::order(direction);
        for purpose in [None, Some([0; 32]), Some([6; 32])] {
            let result = RampOrder::bind(
                CreateOrder {
                    order_id: bound.order_id.clone(),
                    quote_id: bound.quote.quote_id.clone(),
                    payer_grant: bound.payer_grant,
                    payer_grant_purpose: purpose,
                },
                bound.quote.clone(),
                bound.customer.clone(),
                bound.operator.clone(),
                1_000,
            );
            if (direction == RampDirection::OffRamp && purpose == Some([6; 32]))
                || (direction == RampDirection::OnRamp && purpose.is_none())
            {
                assert_eq!(result, Ok(bound.clone()));
            } else {
                assert_eq!(result, Err(RampError::InvalidOrder));
            }
        }
    }
}

fn payment_log(order: &RampOrder) -> SequencerLog {
    let operator = account_id(&order.operator.account);
    let customer = account_id(&order.customer.account);
    let mut data = order.quote.layerx_asset.to_vec();
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&order.quote.layerx_amount.to_be_bytes());
    let topics = match order.direction() {
        RampDirection::OnRamp => vec![
            Keccak256::digest(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)").into(),
            order.order_digest,
            operator,
            customer,
        ],
        RampDirection::OffRamp => {
            data.extend_from_slice(&order.order_digest);
            data.extend_from_slice(&order.context);
            vec![
                Keccak256::digest(
                    b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)",
                )
                .into(),
                order.payer_grant.expect("grant"),
                customer,
                operator,
            ]
        }
    };
    SequencerLog {
        address: LEDGER_ADDRESS,
        topics,
        data,
        log_index: 0,
    }
}

fn bytes(out: &mut Vec<u8>, value: &[u8]) {
    out.extend_from_slice(
        &u32::try_from(value.len())
            .expect("bounded bytes")
            .to_be_bytes(),
    );
    out.extend_from_slice(value);
}

fn effect(out: &mut Vec<u8>, ordinal: u16, kind: u8, body: &[u8]) {
    out.extend_from_slice(&12_u16.to_be_bytes());
    out.extend_from_slice(&ordinal.to_be_bytes());
    out.extend_from_slice(&0_u16.to_be_bytes());
    out.extend_from_slice(&[kind, 0]);
    bytes(out, &[0; 32]);
    bytes(out, body);
}

fn signed_evidence(order: &RampOrder, logs: Vec<SequencerLog>) -> ReceiptEvidence {
    let call = match order.direction() {
        RampDirection::OnRamp => compile_operator_send(order),
        RampDirection::OffRamp => compile_payer_grant_draw(order, 9),
    }
    .expect("ledger call");
    let raw_transaction = Eip1559Call {
        chain_id: 1,
        nonce: 9,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 67_000_000,
        gas_limit: 100_000,
        to: call.to,
        data: call.data,
    }
    .sign(&[3; 32])
    .expect("signed EIP-1559 transaction");
    let key = SigningKey::from_bytes(&[42; 32]);
    let key_id = [6; 32];
    let mut receipt = Vec::new();
    receipt.extend_from_slice(&2_u16.to_be_bytes());
    receipt.extend_from_slice(&0x5201_u16.to_be_bytes());
    receipt.extend_from_slice(&2_u16.to_be_bytes());
    bytes(&mut receipt, &Keccak256::digest(&raw_transaction));
    receipt.extend_from_slice(&1_u64.to_be_bytes());
    for root in [[1; 32], [2; 32], Sha256::digest([]).into()] {
        bytes(&mut receipt, &root);
    }
    receipt.extend_from_slice(&0_i32.to_be_bytes());
    receipt.extend_from_slice(
        &u32::try_from(logs.len() + 1)
            .expect("effects")
            .to_be_bytes(),
    );
    let mut summary = vec![0; 68];
    summary.extend_from_slice(&Sha256::digest([]));
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&u32::try_from(logs.len()).expect("logs").to_be_bytes());
    summary.push(5);
    summary.extend_from_slice(b"upaxe");
    effect(&mut receipt, 0, 1, &summary);
    for (index, log) in logs.iter().enumerate() {
        let mut body = log.address.to_vec();
        body.push(u8::try_from(log.topics.len()).expect("topics"));
        for topic in &log.topics {
            body.extend_from_slice(topic);
        }
        body.extend_from_slice(&Sha256::digest(&log.data));
        effect(
            &mut receipt,
            u16::try_from(index + 1).expect("ordinal"),
            3,
            &body,
        );
    }
    receipt.extend_from_slice(&0_u128.to_be_bytes());
    let mut height = [0; 32];
    height[24..].copy_from_slice(&100_u64.to_be_bytes());
    bytes(&mut receipt, &height);
    receipt.extend_from_slice(&12_u16.to_be_bytes());
    receipt.extend_from_slice(&1_u32.to_be_bytes());
    receipt.extend_from_slice(&0_u32.to_be_bytes());
    receipt.push(0);
    bytes(&mut receipt, &Sha256::digest(b"upaxe"));
    receipt.extend_from_slice(&0_u128.to_be_bytes());
    bytes(&mut receipt, &[0; 32]);
    receipt.extend_from_slice(&0_u128.to_be_bytes());
    receipt.extend_from_slice(&0_u128.to_be_bytes());
    receipt.extend_from_slice(&9_u64.to_be_bytes());
    let mut to = [0; 32];
    to[12..].copy_from_slice(&LEDGER_ADDRESS);
    bytes(&mut receipt, &to);
    receipt.extend_from_slice(&0_u128.to_be_bytes());
    receipt.extend_from_slice(&0_u128.to_be_bytes());
    bytes(&mut receipt, &[0; 32]);
    bytes(&mut receipt, &[0; 32]);
    bytes(&mut receipt, &key_id);
    receipt.extend_from_slice(&1_001_u64.to_be_bytes());
    receipt.push(0);
    let decoded = layerx_wire::receipt::decode(&receipt).expect("canonical unsigned receipt");
    let unsigned = layerx_wire::receipt::encode_unsigned(&decoded).expect("unsigned encoding");
    assert_eq!(unsigned, receipt);
    let digest = layerx_wire::hash::receipt_digest(&unsigned).expect("signing digest");
    let signature = key.sign(&digest).to_bytes();
    receipt.pop();
    receipt.push(1);
    bytes(&mut receipt, &signature);
    ReceiptEvidence {
        raw_transaction,
        signed_receipt: receipt,
        logs,
        receipt_keys: vec![ReceiptKeyEntry {
            key_id,
            public_key: key.verifying_key().to_bytes(),
            first_height: 100,
            last_height: 100,
        }],
    }
}

#[test]
fn verify_order_receipt_accepts_matching_payment_log() {
    for direction in [RampDirection::OnRamp, RampDirection::OffRamp] {
        let order = order(direction);
        let evidence = signed_evidence(&order, vec![payment_log(&order)]);
        let leg = verify_order_receipt(&order, &evidence).expect("verified payment");
        assert_eq!(
            leg.tx_hash,
            <[u8; 32]>::from(Keccak256::digest(&evidence.raw_transaction))
        );
        assert_eq!(
            leg.receipt_digest,
            <[u8; 32]>::from(Keccak256::digest(&evidence.signed_receipt))
        );
        assert_eq!(leg.height, 100);
        assert_eq!(leg.rung, Rung::Receipted);
        if direction == RampDirection::OnRamp {
            let mut log = payment_log(&order);
            log.topics[1..].rotate_left(1);
            assert_eq!(
                verify_order_receipt(&order, &signed_evidence(&order, vec![log])),
                Err(RampError::ReceiptMismatch)
            );
        }
        let mut changed = evidence.clone();
        changed.raw_transaction.push(0);
        assert_eq!(
            verify_order_receipt(&order, &changed),
            Err(RampError::ReceiptMismatch)
        );
        let mut changed = evidence.clone();
        changed.logs[0].data[63] ^= 1;
        assert_eq!(
            verify_order_receipt(&order, &changed),
            Err(RampError::ReceiptMismatch)
        );
        let mut changed = evidence.clone();
        *changed.signed_receipt.last_mut().expect("signature") ^= 1;
        assert_eq!(
            verify_order_receipt(&order, &changed),
            Err(RampError::Receipt(ReceiptCheck::SequencerSignature))
        );
        for index in 0..4 {
            let mut log = payment_log(&order);
            log.topics[index][0] ^= 1;
            let changed = signed_evidence(&order, vec![log]);
            assert_eq!(
                verify_order_receipt(&order, &changed),
                Err(RampError::ReceiptMismatch)
            );
        }
        for index in [0, 63] {
            let mut log = payment_log(&order);
            log.data[index] ^= 1;
            assert_eq!(
                verify_order_receipt(&order, &signed_evidence(&order, vec![log])),
                Err(RampError::ReceiptMismatch)
            );
        }
        let first = payment_log(&order);
        let mut second = first.clone();
        second.log_index = 1;
        assert_eq!(
            verify_order_receipt(&order, &signed_evidence(&order, vec![first, second])),
            Err(RampError::ReceiptMismatch)
        );
        assert_eq!(
            verify_order_receipt(&order, &signed_evidence(&order, vec![])),
            Err(RampError::ReceiptMismatch)
        );
    }
}

#[test]
fn verify_order_receipt_refuses_key_outside_height_range() {
    let order = order(RampDirection::OnRamp);
    let mut evidence = signed_evidence(&order, vec![payment_log(&order)]);
    assert!(verify_order_receipt(&order, &evidence).is_ok());
    evidence.receipt_keys[0].first_height = 1;
    evidence.receipt_keys[0].last_height = 99;
    assert_eq!(
        verify_order_receipt(&order, &evidence),
        Err(RampError::Receipt(ReceiptCheck::SequencerSignature))
    );
    evidence.receipt_keys[0].first_height = 101;
    evidence.receipt_keys[0].last_height = 200;
    assert_eq!(
        verify_order_receipt(&order, &evidence),
        Err(RampError::Receipt(ReceiptCheck::SequencerSignature))
    );
    evidence.receipt_keys.clear();
    assert_eq!(
        verify_order_receipt(&order, &evidence),
        Err(RampError::Receipt(ReceiptCheck::SequencerSignature))
    );
}
