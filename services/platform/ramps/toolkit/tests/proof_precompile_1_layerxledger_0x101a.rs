//! Proves: RampOrder + encode_pay_exact/encode_budget_spend -> LedgerCall -> Eip1559Call::sign/decode_signed; SequencerLog + decode_ledger_payment -> LedgerPayment -> public Rust consumer.
//!
//! This contract's consumer is this integration crate, importing the public
//! items exactly as a ramp caller does. Native ABI entries define the input
//! words and indexed event fields independently of the Rust encoders/decoder.
//! Event inputs exercise the decoder boundary, without claiming node execution
//! or signature authentication from an unauthenticated SequencerLog. There is
//! no persistent state here: fresh calls must produce fresh, owned results.
//!
//! The on-ramp ABI carries checked Unix milliseconds; off-ramp compilation
//! targets the customer's maintained budget authority at 0x101b.

use layerx_client::sequencer::SequencerLog;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_ramp_toolkit::evm::{
    decode_ledger_payment, encode_budget_spend, encode_pay_exact, LedgerPayment, LEDGER_ADDRESS,
    PAY_ADDRESS,
};
use layerx_ramp_toolkit::{
    compile_operator_send, compile_payer_grant_draw, AuthenticatedPrincipal, CreateOrder,
    LedgerCall, OperatorIdentity, QuoteTerms, RampDirection, RampError, RampOrder,
};
use layerx_types::account::AccountId;
use serde_json::Value;
use sha3::{Digest as _, Keccak256};

#[path = "support/repair_22_33_1490.rs"]
mod repair_1490;

#[test]
fn maintained_customer_authority_refuses_retired_draw_before_broadcast() {
    repair_1490::customer_authority();
}

fn abi_entry(kind: &str, name: &str) -> Value {
    let document = match (kind, name) {
        ("function", "budgetSpend") => {
            include_str!("../../../../../precompiles/layerxpay/abi.json")
        }
        ("function", "payExact") | ("event", "PaymentExact" | "PaymentDrawn") => {
            include_str!("../../../../../precompiles/layerxledger/abi.json")
        }
        _ => panic!("unexpected ABI entry: {kind} {name}"),
    };
    let abi: Value = serde_json::from_str(document).expect("native payment ABI");
    abi.as_array()
        .expect("ABI entries")
        .iter()
        .find(|entry| entry["type"] == kind && entry["name"] == name)
        .expect("native ABI entry")
        .clone()
}

fn signature(entry: &Value) -> [u8; 32] {
    let types = entry["inputs"]
        .as_array()
        .expect("ABI inputs")
        .iter()
        .map(|input| input["type"].as_str().expect("ABI type"))
        .collect::<std::vec::Vec<_>>()
        .join(",");
    Keccak256::digest(format!(
        "{}({types})",
        entry["name"].as_str().expect("name")
    ))
    .into()
}

fn word(value: u128) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[16..].copy_from_slice(&value.to_be_bytes());
    bytes
}

fn account(value: &str) -> [u8; 32] {
    layerx_wire::hash::account_id_for_protocol(
        &AccountId::parse(value).expect("account"),
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .expect("ledger account hash")
}

fn order(direction: RampDirection, amount: u128) -> RampOrder {
    RampOrder::bind(
        CreateOrder {
            order_id: "precompile-proof".into(),
            quote_id: "quote-proof".into(),
            payer_grant: (direction == RampDirection::OffRamp).then_some([0x51; 32]),
            payer_grant_purpose: (direction == RampDirection::OffRamp).then_some([0x62; 32]),
        },
        QuoteTerms {
            quote_id: "quote-proof".into(),
            direction,
            layerx_asset: [0x33; 32],
            layerx_amount: amount,
            external_currency: "EUR".into(),
            external_amount_minor: 100,
            rate_numerator: 10,
            rate_denominator: 1,
            fee_minor: 2,
            maximum_slippage_bps: 25,
            context: [0x73; 32],
            provider_token: "product-eur".into(),
            payout_token: "beneficiary-proof".into(),
            expires_at: u64::MAX / 1_000,
        },
        AuthenticatedPrincipal {
            principal_id: "customer-proof".into(),
            account: "agent:did:layerx:customer-proof:main".into(),
        },
        OperatorIdentity {
            principal_id: "operator-proof".into(),
            account: "agent:did:layerx:operator-proof:main".into(),
            signer_key_handle: "kms.operator-proof.primary".into(),
        },
        1_000,
    )
    .expect("real bound order")
}

fn observe_call(call: &LedgerCall, name: &str, fields: &[(&str, [u8; 32])]) {
    let abi = abi_entry("function", name);
    let inputs = abi["inputs"].as_array().expect("function inputs");
    assert_eq!(inputs.len(), fields.len());
    let (address, suffix) = match name {
        "payExact" => (LEDGER_ADDRESS, 0x1a),
        "budgetSpend" => (PAY_ADDRESS, 0x1b),
        _ => panic!("unexpected payment call: {name}"),
    };
    assert_eq!(call.to, address);
    assert_eq!(
        call.to,
        [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, suffix]
    );
    let transaction = Eip1559Call {
        chain_id: 125,
        nonce: 9,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 67_000_000,
        gas_limit: 200_000,
        to: call.to,
        data: call.data.clone(),
    };
    let raw = transaction.sign(&[1; 32]).expect("real transaction signer");
    let (observed, signed) = decode_signed(&raw).expect("real transaction decoder");
    assert_eq!(observed, transaction);
    assert_eq!(
        recover_sender(&observed, &signed).expect("recover signer"),
        [
            0x1a, 0x64, 0x2f, 0x0e, 0x3c, 0x3a, 0xf5, 0x45, 0xe7, 0xac, 0xbd, 0x38, 0xb0, 0x72,
            0x51, 0xb3, 0x99, 0x09, 0x14, 0xf1,
        ]
    );
    assert_eq!(observed.data.len(), 4 + 32 * fields.len());
    assert_eq!(&observed.data[..4], &signature(&abi)[..4]);
    for (index, (input, (name, expected))) in inputs.iter().zip(fields).enumerate() {
        assert_eq!(input["name"], *name);
        assert_eq!(
            &observed.data[4 + index * 32..4 + (index + 1) * 32],
            expected,
            "{name}"
        );
    }
    let mut malformed = raw;
    malformed.pop();
    assert!(
        decode_signed(&malformed).is_err(),
        "truncated transport refused"
    );
}

// The graph contract explicitly requires a declared symbol named Vec. This
// test checks the Vec<u8> carried by LedgerCall across the public boundary.
#[test]
#[allow(non_snake_case)]
fn Vec() {
    for direction in [RampDirection::OnRamp, RampDirection::OffRamp] {
        for amount in [1, (1_u128 << 100) + 19, u128::MAX] {
            let bound = order(direction, amount);
            let sequence = u64::MAX;
            for expires_at in [u64::MAX / 1_000 + 1, u64::MAX] {
                let mut invalid = bound.clone();
                invalid.quote.expires_at = expires_at;
                invalid.order_digest = invalid.digest();
                assert_eq!(invalid.quote.validate(1_000), Err(RampError::InvalidOrder));
                assert_eq!(
                    RampOrder::bind(
                        CreateOrder {
                            order_id: invalid.order_id.clone(),
                            quote_id: invalid.quote.quote_id.clone(),
                            payer_grant: invalid.payer_grant,
                            payer_grant_purpose: invalid.payer_grant_purpose,
                        },
                        invalid.quote.clone(),
                        invalid.customer.clone(),
                        invalid.operator.clone(),
                        1_000,
                    ),
                    Err(RampError::InvalidOrder)
                );
                if direction == RampDirection::OnRamp {
                    assert_eq!(
                        compile_operator_send(&invalid),
                        Err(RampError::InvalidOrder)
                    );
                }
            }
            for _ in 0..2 {
                let mut call = match direction {
                    RampDirection::OnRamp => compile_operator_send(&bound),
                    RampDirection::OffRamp => compile_payer_grant_draw(&bound, sequence),
                }
                .expect("compile fresh public LedgerCall");
                match direction {
                    RampDirection::OnRamp => {
                        let from = account(&bound.operator.account);
                        let to = account(&bound.customer.account);
                        let expires_at_ms = bound.quote.expires_at * 1_000;
                        assert_eq!(
                            call.data,
                            encode_pay_exact(
                                from,
                                to,
                                bound.quote.layerx_asset,
                                amount,
                                expires_at_ms,
                                bound.order_digest
                            )
                        );
                        observe_call(
                            &call,
                            "payExact",
                            &[
                                ("from", from),
                                ("to", to),
                                ("assetId", bound.quote.layerx_asset),
                                ("amount", word(amount)),
                                ("expiresAtMs", word(u128::from(expires_at_ms))),
                                ("paymentReference", bound.order_digest),
                            ],
                        );
                        assert_eq!(
                            compile_payer_grant_draw(&bound, sequence),
                            Err(RampError::InvalidOrder)
                        );
                    }
                    RampDirection::OffRamp => {
                        let grant = bound.payer_grant.expect("grant");
                        let recipient = account(&bound.operator.account);
                        let purpose = bound.payer_grant_purpose.expect("purpose");
                        let authorization = bound
                            .customer_payment_authorization()
                            .expect("customer budget authorization");
                        assert_eq!(authorization.budget_id, grant);
                        assert_eq!(authorization.purpose_hash, purpose);
                        assert_eq!(call.data, encode_budget_spend(grant, recipient, amount));
                        observe_call(
                            &call,
                            "budgetSpend",
                            &[
                                ("budgetId", grant),
                                ("recipient", recipient),
                                ("amount", word(amount)),
                            ],
                        );
                        assert_eq!(
                            compile_payer_grant_draw(&bound, 0),
                            Ok(call.clone()),
                            "the compatibility sequence is an EVM nonce, outside budgetSpend calldata"
                        );
                        assert_eq!(compile_operator_send(&bound), Err(RampError::InvalidOrder));
                    }
                }
                // Altering this owned result must not affect the next compile.
                call.data.fill(0);
                let mut malformed = bound.clone();
                malformed.quote.layerx_amount ^= 1;
                assert_eq!(
                    compile_operator_send(&malformed),
                    Err(RampError::OrderBinding)
                );
                assert_eq!(
                    compile_payer_grant_draw(&malformed, sequence),
                    Err(RampError::OrderBinding)
                );
            }
        }
    }
}

fn event_input(name: &str, fields: &[(&str, [u8; 32])]) -> SequencerLog {
    let abi = abi_entry("event", name);
    let inputs = abi["inputs"].as_array().expect("event inputs");
    assert_eq!(inputs.len(), fields.len());
    let mut log = SequencerLog {
        address: LEDGER_ADDRESS,
        topics: vec![signature(&abi)],
        data: std::vec::Vec::new(),
        log_index: 7,
    };
    for (input, (name, value)) in inputs.iter().zip(fields) {
        assert_eq!(input["name"], *name);
        if input["indexed"].as_bool().expect("indexed") {
            log.topics.push(*value);
        } else {
            log.data.extend_from_slice(value);
        }
    }
    log
}

#[test]
fn proof_precompile_1_layerxledger_0x101a() {
    let from = [0x11; 32];
    let to = [0x22; 32];
    let asset = [0x33; 32];
    let reference = [0x44; 32];
    let grant = [0x55; 32];
    let context = [0x66; 32];
    for amount in [0, 1, (1_u128 << 100) + 19, u128::MAX] {
        for _ in 0..2 {
            let exact = event_input(
                "PaymentExact",
                &[
                    ("paymentReference", reference),
                    ("from", from),
                    ("to", to),
                    ("assetId", asset),
                    ("amount", word(amount)),
                ],
            );
            let drawn = event_input(
                "PaymentDrawn",
                &[
                    ("grantId", grant),
                    ("from", from),
                    ("to", to),
                    ("assetId", asset),
                    ("amount", word(amount)),
                    ("idempotencyKey", reference),
                    ("contextHash", context),
                ],
            );
            for (mut input, expected) in [
                (
                    exact,
                    LedgerPayment::Exact {
                        from,
                        to,
                        reference,
                        asset,
                        amount,
                    },
                ),
                (
                    drawn,
                    LedgerPayment::Drawn {
                        from,
                        to,
                        grant,
                        asset,
                        amount,
                        idempotency_key: reference,
                        context,
                    },
                ),
            ] {
                let observed: LedgerPayment =
                    decode_ledger_payment(&input).expect("public consumer observes payment");
                assert_eq!(observed, expected);
                for byte in 32..48 {
                    let mut malformed = input.clone();
                    malformed.data[byte] = 1;
                    assert_eq!(
                        decode_ledger_payment(&malformed),
                        None,
                        "uint128 overflow at {byte}"
                    );
                }
                let mut malformed = input.clone();
                malformed.address[19] ^= 1;
                assert_eq!(decode_ledger_payment(&malformed), None);
                malformed = input.clone();
                malformed.topics[0] = [0; 32];
                assert_eq!(decode_ledger_payment(&malformed), None);
                for length in 0..input.topics.len() {
                    malformed = input.clone();
                    malformed.topics.truncate(length);
                    assert_eq!(decode_ledger_payment(&malformed), None);
                }
                malformed = input.clone();
                malformed.topics.push([0; 32]);
                assert_eq!(decode_ledger_payment(&malformed), None);
                for length in 0..input.data.len() {
                    malformed = input.clone();
                    malformed.data.truncate(length);
                    assert_eq!(decode_ledger_payment(&malformed), None);
                }
                malformed = input.clone();
                malformed.data.push(0);
                assert_eq!(decode_ledger_payment(&malformed), None);
                // No cached success after an input changes, and no borrowed
                // storage in the value already observed by the consumer.
                input.data.fill(0);
                input.topics.clear();
                assert_eq!(decode_ledger_payment(&input), None);
                assert_eq!(observed, expected);
            }
        }
    }
}
