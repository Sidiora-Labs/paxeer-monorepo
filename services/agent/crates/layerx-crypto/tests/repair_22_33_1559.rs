use std::collections::BTreeSet;

use layerx_crypto::disclosure::call::{
    bind_call, encode_bank_send, encode_payment_call, encode_program_call, ledger_selector,
    validate_call, validate_canonical, CallFeeContext, CallMonetaryAmount, ConstructMethod,
    ExecutionFeeBound, GasFeeBound,
};
use layerx_crypto::disclosure::{AmountRole, Disclosure, DisclosureError};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_crypto::payments::Payment;
use layerx_crypto::send::{ledger_payment_call, CallOptions};
use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
use layerx_types::intent::ProgramId;
use layerx_types::payload::{ActivityType, ModuleId};
use layerx_types::program_call::{NativeProgramCall, Resources};

type Outcome = Result<(), Box<dyn std::error::Error>>;

const SECRET: [u8; 32] = [1; 32];
const SENDER: [u8; 20] = [
    0x1a, 0x64, 0x2f, 0x0e, 0x3c, 0x3a, 0xf5, 0x45, 0xe7, 0xac, 0xbd, 0x38, 0xb0, 0x72, 0x51, 0xb3,
    0x99, 0x09, 0x14, 0xf1,
];

fn word(value: u128) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[16..].copy_from_slice(&value.to_be_bytes());
    bytes
}

fn hex(text: &str) -> Vec<u8> {
    let text = text.trim();
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("fixture UTF-8"), 16)
                .expect("fixture hex")
        })
        .collect()
}

fn call(target: u16, data: Vec<u8>) -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&target.to_be_bytes());
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data,
    }
}

fn native_program() -> NativeProgramCall<'static> {
    NativeProgramCall {
        program_id: ProgramId::new([0x31; 32]),
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: b"fixed input",
        capabilities: &[0, 0],
        access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
        response_capacity: 16,
        resources: Resources([1000, 2000, 3000, 4, 5, 6, 7]),
    }
}

// The producer signs real type-2 bytes; the consumer decodes the transport,
// recovers its sender, binds the ABI, and checks the retained canonical bytes.
fn transported(call: &Eip1559Call) -> Result<Disclosure, Box<dyn std::error::Error>> {
    let raw = call.sign(&SECRET)?;
    let (received, signature) = decode_signed(&raw)?;
    assert_eq!(&received, call);
    let sender = recover_sender(&received, &signature)?;
    assert_eq!(sender, SENDER);
    let disclosure = bind_call(&received, sender)?;
    validate_call(call, &disclosure, sender)?;
    validate_canonical(&disclosure, &call.encode_unsigned())?;
    assert_eq!(disclosure.signing_digest()?, call.signing_hash());
    Ok(disclosure)
}

#[test]
fn program_execution_fee_projection_preserves_both_bounds() -> Outcome {
    let asset = [0x72; 32];
    let context = CallFeeContext::ProgramExecution { fee_asset: asset };
    for execution in [0, 73, u128::MAX] {
        let transaction = call(0x101d, encode_program_call(&native_program(), execution));
        let disclosure = transported(&transaction)?;
        let view = disclosure.call_monetary(context)?;
        assert_eq!(
            view.gas_fee_bound,
            GasFeeBound {
                chain_id: 125,
                amount_wei: 3_000_000
            }
        );
        assert_eq!(
            view.execution_fee_bound,
            Some(ExecutionFeeBound {
                asset,
                amount_base_units: execution
            })
        );
        assert!(view.amounts.is_empty());
        let approval = view.to_string();
        assert!(approval.contains("Gas fee bound: 3000000 wei of chain 125 native currency"));
        assert!(approval.contains(&format!(
            "Execution fee bound: {execution} base units of protocol asset 0x{}",
            "72".repeat(32)
        )));
        assert_eq!(
            disclosure.call_monetary(CallFeeContext::GasOnly),
            Err(DisclosureError::FieldMismatch("execution_fee_asset"))
        );
        assert!(disclosure
            .call_monetary(CallFeeContext::ProgramExecution { fee_asset: [0; 32] })
            .is_err());

        let mut tampered = disclosure.clone();
        tampered.fee_limit += 1;
        assert!(tampered.call_monetary(context).is_err());
        let mut different = transaction.clone();
        different.data = encode_program_call(&native_program(), execution ^ 1);
        assert!(validate_call(&different, &disclosure, SENDER).is_err());
        let changed = transported(&different)?.call_monetary(context)?;
        assert_eq!(changed.gas_fee_bound, view.gas_fee_bound);
        assert_ne!(changed.execution_fee_bound, view.execution_fee_bound);

        let mut changed_gas = transaction;
        changed_gas.max_fee_per_gas = 31;
        let changed = transported(&changed_gas)?.call_monetary(context)?;
        assert_eq!(changed.gas_fee_bound.amount_wei, 3_100_000);
        assert_eq!(changed.execution_fee_bound, view.execution_fee_bound);
    }
    Ok(())
}

#[test]
fn bank_monetary_projection_reaches_typed_policy_and_human_view() -> Outcome {
    let denom = format!("layerx/{}", "03".repeat(32));
    let mut above_u128 = [0; 32];
    above_u128[15] = 1;
    let cases = [
        ([0; 32], "0".to_owned()),
        (word(1), "1".to_owned()),
        (word(u128::MAX), u128::MAX.to_string()),
        (
            above_u128,
            "340282366920938463463374607431768211456".to_owned(),
        ),
        (
            [0xff; 32],
            "115792089237316195423570985008687907853269984665640564039457584007913129639935"
                .to_owned(),
        ),
    ];
    for (amount, decimal) in cases {
        let transaction = call(0x1001, encode_bank_send(SENDER, [0xb2; 20], &denom, amount));
        let disclosure = transported(&transaction)?;
        let view = disclosure.call_monetary(CallFeeContext::GasOnly)?;
        assert_eq!(view.amounts.len(), 1);
        // A generic policy consumer must handle each monetary unit explicitly.
        match &view.amounts[0] {
            CallMonetaryAmount::Bank {
                role,
                denom: received,
                value,
            } => {
                assert_eq!(*role, AmountRole::Transfer);
                assert_eq!(received, &denom);
                assert_eq!(*value, amount);
            }
            CallMonetaryAmount::Protocol { .. } | CallMonetaryAmount::Pointer { .. } => {
                panic!("bank units narrowed or relabelled")
            }
        }
        assert_eq!(
            view.amounts[0].to_string(),
            format!("Transfer: {decimal} base units of {denom:?}")
        );
        assert!(view
            .to_string()
            .contains(&format!("Transfer: {decimal} base units of {denom:?}")));
        assert_eq!(view.execution_fee_bound, None);
        assert!(disclosure
            .call_monetary(CallFeeContext::ProgramExecution { fee_asset: [3; 32] })
            .is_err());
        let mut changed = transaction.clone();
        changed.data[4 + 3 * 32 + 31] ^= 1;
        assert!(validate_call(&changed, &disclosure, SENDER).is_err());
        assert_ne!(
            transported(&changed)?
                .call_monetary(CallFeeContext::GasOnly)?
                .amounts,
            view.amounts
        );
        let renamed = call(
            0x1001,
            encode_bank_send(SENDER, [0xb2; 20], "other-denom", amount),
        );
        assert!(validate_call(&renamed, &disclosure, SENDER).is_err());
        let renamed = transported(&renamed)?.call_monetary(CallFeeContext::GasOnly)?;
        assert_eq!(
            renamed.amounts,
            vec![CallMonetaryAmount::Bank {
                role: AmountRole::Transfer,
                denom: "other-denom".to_owned(),
                value: amount,
            }]
        );
        let mut tampered = disclosure.clone();
        tampered.counterparties[0].account[31] ^= 1;
        assert!(tampered.call_monetary(CallFeeContext::GasOnly).is_err());
    }
    let escaped = transported(&call(
        0x1001,
        encode_bank_send(SENDER, [2; 20], "coin\n\"approval", word(5)),
    ))?
    .call_monetary(CallFeeContext::GasOnly)?;
    assert_eq!(
        escaped.amounts[0].to_string(),
        "Transfer: 5 base units of \"coin\\n\\\"approval\""
    );
    Ok(())
}

#[test]
fn registry_call_constructors_cover_every_bound_selector() -> Outcome {
    let mut calls: Vec<(Eip1559Call, &str)> = Vec::new();
    // Real payment encoders consume the committed canonical native fixtures.
    for (ordinal, fixture, signature) in [
        (
            4,
            include_str!("fixtures/payments/1-4.hex"),
            "openAccount(bytes32)",
        ),
        (
            1,
            include_str!("fixtures/payments/1-1.hex"),
            "register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)",
        ),
        (
            10,
            include_str!("fixtures/payments/1-10.hex"),
            "mint(bytes32,bytes32,uint128)",
        ),
        (
            11,
            include_str!("fixtures/payments/1-11.hex"),
            "burn(bytes32,bytes32,uint128)",
        ),
        (
            7,
            include_str!("fixtures/payments/1-7.hex"),
            "issueGrant(bytes)",
        ),
        (
            8,
            include_str!("fixtures/payments/1-8.hex"),
            "revokeGrant(bytes32,uint64)",
        ),
        (
            6,
            include_str!("fixtures/payments/1-6.hex"),
            "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)",
        ),
    ] {
        let payment =
            Payment::decode(ModuleId::Asset, ordinal, &hex(fixture), b"did:layerx:alice")?;
        calls.push((
            call(0x101a, encode_payment_call(&payment, b"did:layerx:alice")?),
            signature,
        ));
    }
    let send = ledger_payment_call(
        &hex(include_str!("fixtures/payments/native-1-5.hex")),
        &CallOptions {
            from: SENDER,
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 2,
            max_fee_per_gas: 30,
            gas_limit: 100_000,
        },
    )?;
    calls.push((send.call, "send(bytes)"));
    calls.push((
        call(0x101d, encode_program_call(&native_program(), 73)),
        "call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)",
    ));
    calls.push((
        call(
            0x1001,
            encode_bank_send(SENDER, [2; 20], "upax", [0xff; 32]),
        ),
        "send(address,address,string,uint256)",
    ));

    // Committed ABI transcripts from disclosure_call.rs: budget create v1/v2,
    // fund/defund, lifecycle, and pointer calls have no named daemon constructor.
    let budget_words = [
        [0x21; 32],
        [0x22; 32],
        [0x23; 32],
        [0x24; 32],
        word(1000),
        word(0),
        word(123),
        word(10),
        word(20),
        word(100),
        word(7),
        word(1),
    ];
    for (native, signature) in [
        (false, "budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)"),
        (true, "budgetCreateNative(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8,bytes32,uint64)"),
    ] {
        let mut words = budget_words.to_vec();
        if native { words.extend([[0x25; 32], word(9)]); }
        calls.push((call(0x101b, [ledger_selector(signature).to_vec(), words.concat()].concat()), signature));
    }
    for signature in [
        "budgetFund(bytes32,uint128)",
        "budgetDefund(bytes32,uint128)",
    ] {
        calls.push((
            call(
                0x101b,
                [
                    ledger_selector(signature).to_vec(),
                    [0x31; 32].to_vec(),
                    word(123).to_vec(),
                ]
                .concat(),
            ),
            signature,
        ));
    }
    let lifecycle = "recordLifecycle(bytes,bytes32)";
    let mut payload = vec![0x71, 1, 0, 2];
    payload.resize(68, 0x23);
    let mut args = [word(64), [0x42; 32], word(payload.len() as u128)].concat();
    args.extend(payload);
    args.resize(args.len().div_ceil(32) * 32, 0);
    calls.push((
        call(0x101c, [ledger_selector(lifecycle).to_vec(), args].concat()),
        lifecycle,
    ));
    let pointer = "transfer(address,uint256)";
    let mut recipient = [0; 32];
    recipient[12..].fill(5);
    calls.push((
        call(
            0x2000,
            [
                ledger_selector(pointer).to_vec(),
                recipient.to_vec(),
                word(123).to_vec(),
            ]
            .concat(),
        ),
        pointer,
    ));

    // The actual issuer supplies the session bytes consumed by the auth ABI.
    let grant = issue_session_key(&SessionKeyRequest {
        grantor: [0x41; 32],
        session_public_key: [0x42; 32],
        not_before: 10,
        expires_at: Some(100),
        revocation_sequence: Some(7),
        permitted_activity_types: vec![
            ActivityType::new(ModuleId::Asset, 5).map_err(|e| format!("{e:?}"))?
        ],
        fee_budget: None,
        purpose: SessionPurpose::Activity,
    })?;
    let session = "grantSession(bytes,uint64,bytes32)";
    let mut args = [
        word(96),
        word(9),
        [0x43; 32],
        word(grant.registration_payload.len() as u128),
    ]
    .concat();
    args.extend(grant.registration_payload);
    args.resize(args.len().div_ceil(32) * 32, 0);
    calls.push((
        call(0x101c, [ledger_selector(session).to_vec(), args].concat()),
        session,
    ));

    assert_eq!(calls.len(), 17);
    let mut selectors = BTreeSet::new();
    for (transaction, signature) in calls {
        let disclosure = transported(&transaction)?;
        let method = disclosure.construct_method()?.expect("call constructor");
        assert_eq!(method.abi_signature(), signature);
        assert_eq!(
            &transaction.data[..4],
            &ledger_selector(method.abi_signature())
        );
        assert!(selectors.insert(method.abi_signature()));
        let context = if matches!(method, ConstructMethod::ProgramCall { .. }) {
            CallFeeContext::ProgramExecution {
                fee_asset: [0x72; 32],
            }
        } else {
            CallFeeContext::GasOnly
        };
        let view = disclosure.call_monetary(context)?;
        match method {
            ConstructMethod::BankSend { denom, amount, .. } => assert_eq!(
                view.amounts,
                vec![CallMonetaryAmount::Bank {
                    role: AmountRole::Transfer,
                    denom,
                    value: amount
                }]
            ),
            ConstructMethod::PointerTransfer { pointer, .. } => assert_eq!(
                view.amounts,
                vec![CallMonetaryAmount::Pointer {
                    role: AmountRole::Transfer,
                    pointer,
                    value: 123
                }]
            ),
            _ => assert_eq!(
                view.amounts,
                disclosure
                    .amounts
                    .iter()
                    .map(|amount| CallMonetaryAmount::Protocol {
                        role: amount.role,
                        asset: disclosure.asset,
                        value: amount.value
                    })
                    .collect::<Vec<_>>()
            ),
        }
    }
    assert_eq!(selectors.len(), 17);
    Ok(())
}
