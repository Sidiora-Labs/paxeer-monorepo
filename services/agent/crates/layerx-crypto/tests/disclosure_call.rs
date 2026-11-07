use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::{AmountRole, CounterpartyRole, DisclosureError};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::payments::Payment;
use layerx_crypto::send::{ledger_payment_call, CallOptions};
use layerx_types::payload::ModuleId;

const SENDER: [u8; 20] = [0xa1; 20];

#[test]
fn program_call_round_trips_through_bind_call() -> Result<(), Box<dyn std::error::Error>> {
    use layerx_crypto::disclosure::call::{decode_program_call, encode_program_call};
    use layerx_crypto::disclosure::DisclosedNativeOperation;
    use layerx_types::intent::ProgramId;
    use layerx_types::program_call::{NativeProgramCall, Resources};

    let native = NativeProgramCall {
        program_id: ProgramId::new([0x31; 32]),
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: &[1, 2, 3, 4],
        capabilities: &[0, 0],
        access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
        response_capacity: 16,
        resources: Resources([1_000_000, 16_777_216, 1_048_576, 64, 7, 4096, u64::MAX]),
    };
    let fee_limit = u128::MAX;
    let data = encode_program_call(&native, fee_limit);
    assert_eq!(&data[..4], &[0x54, 0xbd, 0x2b, 0x48]);
    assert_eq!(data.len(), 772);
    for (index, offset) in [(2, 480), (3, 544), (4, 608), (5, 672)] {
        assert_eq!(&data[4 + index * 32..4 + (index + 1) * 32], &word(offset));
    }
    let transaction = call(0x101d, data[..4].try_into()?, &data[4..]);
    let (decoded, decoded_fee) = decode_program_call(&transaction.data)?;
    assert_eq!(decoded.native(), native);
    assert_eq!(decoded_fee, fee_limit);
    assert_eq!(encode_program_call(&decoded.native(), decoded_fee), data);
    let disclosure = bind_call(&transaction, SENDER)?;
    assert_eq!(disclosure.activity_type.module(), ModuleId::Programs);
    assert_eq!(disclosure.activity_type.ordinal(), 3);
    assert_eq!(disclosure.actor, SENDER);
    assert_eq!(disclosure.authority, SENDER);
    assert_eq!(disclosure.fee_limit, 3_000_000);
    assert_eq!(disclosure.signing_digest()?, transaction.signing_hash());
    let Some(DisclosedNativeOperation::ProgramCall(disclosed)) = &disclosure.native_operation
    else {
        panic!("missing native program call");
    };
    assert_eq!(disclosed.program_id, native.program_id);
    assert_eq!(disclosed.entrypoint, native.entrypoint);
    assert_eq!(disclosed.calldata, native.calldata);
    assert_eq!(disclosed.resources, native.resources);
    assert_eq!(disclosed.native(), native);
    assert_eq!(
        disclosure.construct_method()?,
        Some(ConstructMethod::ProgramCall {
            call: Box::new(decoded),
            fee_limit
        })
    );
    for index in 1..15 {
        let mut invalid = transaction.clone();
        invalid.data[4 + index * 32] ^= 1;
        assert_eq!(
            bind_call(&invalid, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
    }
    for length in 0..data.len() {
        assert_eq!(
            decode_program_call(&data[..length]),
            Err(DisclosureError::MalformedPayload)
        );
    }
    let mut invalid = transaction.clone();
    *invalid.data.last_mut().expect("access tail padding") = 1;
    assert_eq!(
        bind_call(&invalid, SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let mut invalid = transaction.clone();
    invalid.data.push(0);
    assert_eq!(
        bind_call(&invalid, SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let mut invalid = transaction.clone();
    invalid.data[0] ^= 1;
    assert_eq!(
        bind_call(&invalid, SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let mut invalid = transaction.clone();
    invalid.to[18] ^= 1;
    assert_eq!(
        bind_call(&invalid, SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let mut invalid = transaction;
    invalid.max_fee_per_gas = u128::MAX;
    assert_eq!(
        bind_call(&invalid, SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let mut changed = disclosure;
    let Some(DisclosedNativeOperation::ProgramCall(disclosed)) = &mut changed.native_operation
    else {
        panic!("missing native program call");
    };
    disclosed.resources.0[0] += 1;
    assert!(changed.signing_digest().is_err());
    Ok(())
}

fn hex(text: &str) -> Vec<u8> {
    text.trim()
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("fixture UTF-8"), 16)
                .expect("fixture hex")
        })
        .collect()
}

fn word(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

fn dynamic(payload: &[u8]) -> Vec<u8> {
    let mut args = word(32).to_vec();
    args.extend_from_slice(&word(payload.len() as u128));
    args.extend_from_slice(payload);
    args.resize(args.len().div_ceil(32) * 32, 0);
    args
}

fn lifecycle_args(payload: &[u8], idempotency_key: [u8; 32]) -> Vec<u8> {
    let mut args = word(64).to_vec();
    args.extend_from_slice(&idempotency_key);
    args.extend_from_slice(&word(payload.len() as u128));
    args.extend_from_slice(payload);
    args.resize(args.len().div_ceil(32) * 32, 0);
    args
}

fn call(target: u16, selector: [u8; 4], args: &[u8]) -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&target.to_be_bytes());
    let mut data = selector.to_vec();
    data.extend_from_slice(args);
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

fn fixtures() -> Result<Vec<Eip1559Call>, Box<dyn std::error::Error>> {
    let send = hex(include_str!("fixtures/payments/native-1-5.hex"));
    let issue = hex(include_str!("fixtures/payments/1-7.hex"));
    let revoke = hex(include_str!("fixtures/payments/1-8.hex"));
    let receive = hex(include_str!("fixtures/payments/1-6.hex"));
    let Payment::Receive {
        to,
        amount,
        grant,
        sequence,
        idempotency_key,
        context_hash,
        payer_grant,
        ..
    } = Payment::decode(ModuleId::Asset, 6, &receive, b"did:layerx:alice")?
    else {
        panic!("receive fixture")
    };
    let mut draw = Vec::new();
    for value in [
        grant,
        to,
        word(amount),
        idempotency_key,
        word(u128::from(sequence)),
        payer_grant.purpose_hash,
        context_hash,
    ] {
        draw.extend_from_slice(&value);
    }
    let Payment::RevokeGrant {
        grant,
        revocation_sequence,
    } = Payment::decode(ModuleId::Asset, 8, &revoke, b"did:layerx:alice")?
    else {
        panic!("revoke fixture")
    };
    let mut revoke_args = grant.to_vec();
    revoke_args.extend_from_slice(&word(u128::from(revocation_sequence)));

    let mut payer = [0; 32];
    payer[12..].copy_from_slice(&SENDER);
    let mut recipient = [0; 32];
    recipient[12..].fill(0xb2);
    let denom = format!(
        "layerx/{}",
        &include_str!("fixtures/payments/native-1-5.hex")[136..200]
    );
    let mut bank = Vec::new();
    for value in [
        payer,
        recipient,
        word(128),
        [0xff; 32],
        word(denom.len() as u128),
    ] {
        bank.extend_from_slice(&value);
    }
    bank.extend_from_slice(denom.as_bytes());
    bank.resize(bank.len().div_ceil(32) * 32, 0);
    Ok(vec![
        call(0x101a, [0x2b, 0xb1, 0xae, 0x7c], &dynamic(&send)),
        call(0x101a, [0xd0, 0xd5, 0x34, 0x80], &dynamic(&issue)),
        call(0x101a, [0x2e, 0x89, 0x49, 0x0f], &draw),
        call(0x101a, [0x58, 0xd1, 0x83, 0xa8], &revoke_args),
        call(0x1001, [0x5c, 0x05, 0x96, 0x1b], &bank),
    ])
}

#[test]
fn ledger_send_call_discloses_the_send_payload_fields() -> Result<(), Box<dyn std::error::Error>> {
    let calls = fixtures()?;
    let send = &calls[0];
    let payload = hex(include_str!("fixtures/payments/native-1-5.hex"));
    let options = CallOptions {
        from: SENDER,
        chain_id: send.chain_id,
        nonce: send.nonce,
        max_priority_fee_per_gas: send.max_priority_fee_per_gas,
        max_fee_per_gas: send.max_fee_per_gas,
        gas_limit: send.gas_limit,
    };
    let activity = ledger_payment_call(&payload, &options)?.disclosure;
    let disclosure = bind_call(send, SENDER)?;
    assert_eq!(disclosure.counterparties, activity.counterparties);
    assert_eq!(disclosure.amounts, activity.amounts);
    assert_eq!(disclosure.asset, activity.asset);
    assert_eq!(disclosure.idempotency_key, activity.idempotency_key);
    assert_eq!(
        disclosure.expiry.payload_expires_at,
        activity.expiry.payload_expires_at
    );
    assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
    assert_eq!(disclosure.counterparties[0].account, [1; 32]);
    assert_eq!(disclosure.counterparties[1].account, [2; 32]);
    assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosure.amounts[0].value, 123);
    assert_eq!(disclosure.actor, SENDER);
    assert_eq!(disclosure.fee_limit, 3_000_000);
    assert_eq!(disclosure.payload_sequence()?, Some(7));
    assert_eq!(
        disclosure.construct_method()?,
        Some(ConstructMethod::LedgerSend { payload })
    );
    for (index, call) in calls.iter().enumerate() {
        let disclosure = bind_call(call, SENDER)?;
        assert_eq!(disclosure.signing_digest()?, call.signing_hash());
        assert_eq!(disclosure.envelope_sequence(), call.nonce);
        let method = disclosure.construct_method()?.expect("call method");
        match (index, method) {
            (0, ConstructMethod::LedgerSend { .. }) => {}
            (1, ConstructMethod::LedgerIssueGrant { grant }) => {
                assert_eq!(disclosure.asset, grant.asset);
                assert_eq!(disclosure.counterparties[0].account, grant.from);
                assert_eq!(disclosure.amounts[0].value, grant.per_draw_maximum);
                assert_eq!(disclosure.amounts[1].value, grant.allowance);
                assert_eq!(disclosure.expiry.payload_expires_at, grant.expiration);
                assert_eq!(disclosure.payment, Some(Payment::IssueGrant(grant)));
            }
            (
                2,
                ConstructMethod::LedgerDraw {
                    recipient,
                    amount,
                    receiver_sequence,
                    idempotency_key,
                    ..
                },
            ) => {
                assert_eq!(disclosure.counterparties[0].account, recipient);
                assert_eq!(
                    disclosure.counterparties[0].role,
                    CounterpartyRole::Recipient
                );
                assert_eq!(disclosure.amounts[0].value, amount);
                assert_eq!(disclosure.payload_sequence()?, Some(receiver_sequence));
                assert_eq!(disclosure.idempotency_key, idempotency_key);
            }
            (
                3,
                ConstructMethod::LedgerRevokeGrant {
                    grant,
                    revocation_sequence,
                },
            ) => {
                assert_eq!(
                    disclosure.payment,
                    Some(Payment::RevokeGrant {
                        grant,
                        revocation_sequence
                    })
                );
            }
            (
                4,
                ConstructMethod::BankSend {
                    from,
                    to,
                    denom,
                    amount,
                },
            ) => {
                assert_eq!(from, SENDER);
                assert_eq!(to, [0xb2; 20]);
                assert_eq!(denom, format!("layerx/{}", "03".repeat(32)));
                assert_eq!(amount, [0xff; 32]);
                assert_eq!(&disclosure.counterparties[0].account[12..], &from);
            }
            _ => panic!("wrong construct method"),
        }
        let mut altered = disclosure.clone();
        altered.actor[0] ^= 1;
        altered.authority[0] ^= 1;
        assert!(altered.signing_digest().is_err());
        let mut changed = call.clone();
        changed.nonce += 1;
        assert_ne!(
            disclosure.audit_digest()?,
            bind_call(&changed, SENDER)?.audit_digest()?
        );
    }
    Ok(())
}

#[test]
fn auth_lifecycle_call_discloses_the_governance_payload() -> Result<(), Box<dyn std::error::Error>>
{
    use layerx_crypto::disclosure::call::ledger_selector;

    let selector = ledger_selector("recordLifecycle(bytes,bytes32)");
    let idempotency_key = [0x42; 32];
    for (ordinal, variant, fields, length) in [
        (1, 0, 2, 68),
        (1, 2, 1, 8),
        (1, 2, 1, 1024),
        (3, 0, 3, 70),
        (3, 0, 5, 86),
    ] {
        let mut payload = vec![0x71, ordinal, variant, fields];
        payload.resize(length, 0x23);
        let transaction = call(0x101c, selector, &lifecycle_args(&payload, idempotency_key));
        let disclosure = bind_call(&transaction, SENDER)?;
        assert_eq!(disclosure.activity_type.module(), ModuleId::Governance);
        assert_eq!(disclosure.activity_type.ordinal(), u16::from(ordinal));
        assert_eq!(disclosure.idempotency_key, idempotency_key);
        assert!(disclosure.counterparties.is_empty());
        assert!(disclosure.amounts.is_empty());
        assert_eq!(disclosure.asset, [0; 32]);
        assert_eq!(disclosure.payment, None);
        assert_eq!(disclosure.payload_sequence()?, None);
        assert_eq!(disclosure.canonical_payload(), transaction.data);
        assert_eq!(disclosure.signing_digest()?, transaction.signing_hash());
        assert_eq!(
            disclosure.construct_method()?,
            Some(ConstructMethod::AuthLifecycle {
                payload,
                idempotency_key,
            })
        );
        let mut altered = disclosure.clone();
        altered.idempotency_key[0] ^= 1;
        assert!(altered.signing_digest().is_err());
    }
    for (tag, ordinal, variant, fields, length) in [
        (0x71, 5, 1, 3, 52),
        (0x1a, 1, 0, 2, 68),
        (0x71, 2, 0, 4, 92),
        (0x71, 1, 1, 2, 68),
        (0x71, 1, 0, 1, 68),
        (0x71, 1, 0, 2, 67),
        (0x71, 1, 0, 2, 69),
        (0x71, 1, 2, 2, 8),
        (0x71, 1, 2, 1, 7),
        (0x71, 1, 2, 1, 1025),
        (0x71, 3, 1, 3, 70),
        (0x71, 3, 0, 4, 70),
        (0x71, 3, 0, 3, 69),
        (0x71, 3, 0, 3, 71),
        (0x71, 3, 0, 3, 86),
        (0x71, 3, 0, 5, 70),
        (0x71, 3, 0, 5, 85),
        (0x71, 3, 0, 5, 87),
    ] {
        let mut payload = vec![tag, ordinal, variant, fields];
        payload.resize(length, 0x23);
        assert_eq!(
            bind_call(
                &call(0x101c, selector, &lifecycle_args(&payload, idempotency_key)),
                SENDER
            ),
            Err(DisclosureError::MalformedPayload),
            "header {tag:#x}/{ordinal}/{variant}/{fields}, length {length}"
        );
    }
    for length in 0..4 {
        let payload = &([0x71, 1, 0, 2])[..length];
        assert_eq!(
            bind_call(
                &call(0x101c, selector, &lifecycle_args(payload, idempotency_key)),
                SENDER
            ),
            Err(DisclosureError::MalformedPayload)
        );
    }
    Ok(())
}

#[test]
fn unknown_target_or_selector_is_refused() -> Result<(), Box<dyn std::error::Error>> {
    let mut payload = vec![0x71, 1, 0, 2];
    payload.resize(68, 0x23);
    let args = lifecycle_args(&payload, [0x42; 32]);
    assert_eq!(
        bind_call(&call(0x101c, [0; 4], &args), SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let selector =
        layerx_crypto::disclosure::call::ledger_selector("recordLifecycle(bytes,bytes32)");
    for offset in [word(0), word(32), word(96), [0xff; 32]] {
        let mut invalid = args.clone();
        invalid[..32].copy_from_slice(&offset);
        assert_eq!(
            bind_call(&call(0x101c, selector, &invalid), SENDER),
            Err(DisclosureError::MalformedPayload)
        );
    }
    let mut invalid = args.clone();
    invalid[64..96].fill(0xff);
    assert_eq!(
        bind_call(&call(0x101c, selector, &invalid), SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let mut invalid = args.clone();
    *invalid.last_mut().expect("ABI padding") = 1;
    assert_eq!(
        bind_call(&call(0x101c, selector, &invalid), SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let mut invalid = args.clone();
    invalid.extend_from_slice(&[0; 32]);
    assert_eq!(
        bind_call(&call(0x101c, selector, &invalid), SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    let mut calls = fixtures()?;
    calls.push(call(0x101c, selector, &args));
    for original in calls {
        let mut unknown = original.clone();
        unknown.to[18..].copy_from_slice(&0x1002_u16.to_be_bytes());
        assert_eq!(
            bind_call(&unknown, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
        unknown = original.clone();
        unknown.to[0] = 1;
        assert_eq!(
            bind_call(&unknown, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
        unknown = original.clone();
        unknown.data[..4].fill(0);
        assert_eq!(
            bind_call(&unknown, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
        for length in 0..original.data.len() {
            let mut truncated = original.clone();
            truncated.data.truncate(length);
            assert_eq!(
                bind_call(&truncated, SENDER),
                Err(DisclosureError::MalformedPayload),
                "length {length}"
            );
        }
        let mut overflow = original;
        overflow.max_fee_per_gas = u128::MAX;
        assert_eq!(
            bind_call(&overflow, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
    }
    let calls = fixtures()?;
    for index in [0, 1, 4] {
        let offset_word = if index == 4 { 2 } else { 0 };
        let length_word = if index == 4 { 4 } else { 1 };
        for value in [word(0), word(64), [0xff; 32]] {
            let mut invalid = calls[index].clone();
            invalid.data[4 + offset_word * 32..4 + (offset_word + 1) * 32].copy_from_slice(&value);
            assert_eq!(
                bind_call(&invalid, SENDER),
                Err(DisclosureError::MalformedPayload)
            );
        }
        let mut invalid = calls[index].clone();
        invalid.data[4 + length_word * 32..4 + (length_word + 1) * 32].fill(0xff);
        assert_eq!(
            bind_call(&invalid, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
    }
    for (index, word_index) in [(2, 2), (2, 4), (3, 1), (4, 0), (4, 1)] {
        let mut invalid = calls[index].clone();
        invalid.data[4 + word_index * 32] = 1;
        assert_eq!(
            bind_call(&invalid, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
    }
    let mut bad_signature = calls[0].clone();
    bad_signature.data[68 + 20] ^= 1;
    assert_eq!(
        bind_call(&bad_signature, SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    Ok(())
}

#[test]
fn trailing_calldata_is_refused() -> Result<(), Box<dyn std::error::Error>> {
    for original in fixtures()? {
        assert!(bind_call(&original, SENDER).is_ok());
        for length in 1..=33 {
            let mut trailing = original.clone();
            trailing.data.resize(trailing.data.len() + length, 0);
            assert_eq!(
                bind_call(&trailing, SENDER),
                Err(DisclosureError::MalformedPayload)
            );
        }
    }
    for index in [0, 1, 4] {
        let mut nonzero_padding = fixtures()?[index].clone();
        *nonzero_padding.data.last_mut().expect("ABI padding") = 1;
        assert_eq!(
            bind_call(&nonzero_padding, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
    }
    Ok(())
}

#[test]
fn ledger_account_registration_mint_and_burn_are_bound() -> Result<(), Box<dyn std::error::Error>> {
    use layerx_crypto::disclosure::call::{encode_payment_call, ledger_selector};
    use layerx_crypto::payments::Registration;
    let registration = Registration {
        asset: [3; 32],
        salt: [4; 32],
        symbol: "USD".into(),
        name: "Dollar".into(),
        decimals: 6,
        supply_cap: 1_000_000,
        issuer_kind: 2,
        custody_ref: vec![5; 17],
    };
    let cases = [
        (
            Payment::OpenAccount { asset: [3; 32] },
            "openAccount(bytes32)",
            4,
        ),
        (
            Payment::Register(registration),
            "register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)",
            1,
        ),
        (
            Payment::Mint {
                asset: [3; 32],
                to: [2; 32],
                amount: 71,
            },
            "mint(bytes32,bytes32,uint128)",
            10,
        ),
        (
            Payment::Burn {
                asset: [3; 32],
                from: [1; 32],
                amount: 29,
            },
            "burn(bytes32,bytes32,uint128)",
            11,
        ),
    ];
    for (payment, signature, ordinal) in cases {
        let data = encode_payment_call(&payment, b"did:layerx:alice")?;
        assert_eq!(&data[..4], &ledger_selector(signature));
        let transaction = call(0x101a, data[..4].try_into()?, &data[4..]);
        let disclosure = bind_call(&transaction, SENDER)?;
        assert_eq!(
            disclosure.activity_type,
            layerx_types::payload::ActivityType::new(ModuleId::Asset, ordinal)
                .map_err(|e| format!("{e:?}"))?
        );
        assert_eq!(disclosure.asset, [3; 32]);
        assert_eq!(disclosure.payment, Some(payment.clone()));
        assert_eq!(disclosure.signing_digest()?, transaction.signing_hash());
        match payment {
            Payment::OpenAccount { .. } => {
                assert!(disclosure.amounts.is_empty());
                assert!(disclosure.counterparties.is_empty());
            }
            Payment::Register(r) => {
                assert_eq!(disclosure.amounts[0].role, AmountRole::SupplyCap);
                assert_eq!(disclosure.amounts[0].value, r.supply_cap);
            }
            Payment::Mint { to, amount, .. } => {
                assert_eq!(
                    disclosure.counterparties[0].role,
                    CounterpartyRole::Recipient
                );
                assert_eq!(disclosure.counterparties[0].account, to);
                assert_eq!(disclosure.amounts[0].value, amount);
            }
            Payment::Burn { from, amount, .. } => {
                assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
                assert_eq!(disclosure.counterparties[0].account, from);
                assert_eq!(disclosure.amounts[0].value, amount);
            }
            _ => unreachable!(),
        }
        for length in 0..transaction.data.len() {
            let mut invalid = transaction.clone();
            invalid.data.truncate(length);
            assert_eq!(
                bind_call(&invalid, SENDER),
                Err(DisclosureError::MalformedPayload)
            );
        }
        let mut invalid = transaction.clone();
        invalid.data.push(0);
        assert_eq!(
            bind_call(&invalid, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
        let mut invalid = disclosure.clone();
        invalid.asset[0] ^= 1;
        assert!(invalid.signing_digest().is_err());
        if ordinal == 1 {
            for index in [2, 3, 7] {
                let mut invalid = transaction.clone();
                invalid.data[4 + index * 32..4 + (index + 1) * 32].fill(0);
                assert_eq!(
                    bind_call(&invalid, SENDER),
                    Err(DisclosureError::MalformedPayload)
                );
            }
            let mut invalid = transaction.clone();
            *invalid.data.last_mut().expect("padding") = 1;
            assert_eq!(
                bind_call(&invalid, SENDER),
                Err(DisclosureError::MalformedPayload)
            );
        } else if ordinal == 10 || ordinal == 11 {
            let mut invalid = transaction.clone();
            invalid.data[68] = 1;
            assert_eq!(
                bind_call(&invalid, SENDER),
                Err(DisclosureError::MalformedPayload)
            );
        }
    }
    Ok(())
}

#[test]
fn daemon_precompile_constructors_bind() -> Result<(), Box<dyn std::error::Error>> {
    use layerx_crypto::disclosure::call::{
        encode_program_call, ledger_selector, reencode, validate_call, validate_canonical,
    };
    use layerx_crypto::disclosure::DisclosedNativeOperation;
    use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
    use layerx_types::intent::ProgramId;
    use layerx_types::payload::ActivityType;
    use layerx_types::program_call::{NativeProgramCall, Resources};

    // Fixed payload supplied to prepare::evm::PrecompileCall::ledger_transfer.
    let payload = hex(include_str!("fixtures/payments/native-1-5.hex"));
    let transfer = call(0x101a, [0x2b, 0xb1, 0xae, 0x7c], &dynamic(&payload));
    let disclosure = bind_call(&transfer, SENDER)?;
    assert_eq!(disclosure.counterparties.len(), 2);
    assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
    assert_eq!(disclosure.counterparties[0].account, [1; 32]);
    assert_eq!(
        disclosure.counterparties[1].role,
        CounterpartyRole::Recipient
    );
    assert_eq!(disclosure.counterparties[1].account, [2; 32]);
    assert_eq!(disclosure.amounts.len(), 1);
    assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosure.amounts[0].value, 123);
    assert_eq!(disclosure.asset, [3; 32]);
    assert_eq!(disclosure.expiry.payload_expires_at, 100);
    assert_eq!(disclosure.payload_sequence()?, Some(7));
    assert_eq!(
        disclosure.construct_method()?,
        Some(ConstructMethod::LedgerSend { payload })
    );
    let mut cases = vec![(transfer, ModuleId::Asset, 5, [4; 32])];

    // The twelve ABI words of BudgetRequest::create_protocol_budget, v1 and v2.
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
    for native in [false, true] {
        let mut words = budget_words.to_vec();
        if native {
            words.extend([[0x25; 32], word(9)]);
        }
        let signature = if native {
            "budgetCreateNative(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8,bytes32,uint64)"
        } else {
            "budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)"
        };
        let transaction = call(0x101b, ledger_selector(signature), &words.concat());
        let disclosure = bind_call(&transaction, SENDER)?;
        assert_eq!(disclosure.asset, [0x23; 32]);
        assert_eq!(disclosure.expiry.payload_expires_at, 100);
        assert_eq!(disclosure.amounts.len(), 2);
        assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
        assert_eq!(disclosure.amounts[0].value, 123);
        assert_eq!(disclosure.amounts[1].role, AmountRole::SpendingLimit);
        assert_eq!(disclosure.amounts[1].value, 1000);
        let recipient = disclosure.counterparties.last().expect("budget recipient");
        assert_eq!(recipient.role, CounterpartyRole::Recipient);
        assert_eq!(recipient.account, [0x22; 32]);
        if native {
            assert_eq!(disclosure.counterparties.len(), 2);
            assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
            assert_eq!(disclosure.counterparties[0].account, [0x25; 32]);
            assert_eq!(disclosure.payload_sequence()?, Some(9));
            let Some(DisclosedNativeOperation::BudgetCreate(budget)) = &disclosure.native_operation
            else {
                panic!("missing native budget creation");
            };
            assert_eq!(budget.encoding_version, 2);
            assert_eq!(budget.budget_id, [0x21; 32]);
            assert_eq!(budget.budget_account, [0x22; 32]);
            assert_eq!(budget.asset, [0x23; 32]);
            assert_eq!(budget.purpose, [0x24; 32]);
            assert_eq!(budget.per_period_limit, 1000);
            assert_eq!(budget.carry_cap, 0);
            assert_eq!(budget.initial_amount, 123);
            assert_eq!(budget.period_length_ms, 10);
            assert_eq!(budget.period_start_ms, 20);
            assert_eq!(budget.expiry_ms, 100);
            assert_eq!(budget.revocation_sequence, 7);
            assert_eq!(budget.rollover, 1);
            assert_eq!(budget.source_account, [0x25; 32]);
            assert_eq!(budget.source_sequence, 9);
        } else {
            assert_eq!(disclosure.counterparties.len(), 1);
            assert_eq!(disclosure.payload_sequence()?, None);
            assert!(matches!(
                disclosure.construct_method()?,
                Some(ConstructMethod::PayBudgetCreate { .. })
            ));
        }
        // All integer words keep their declared ABI widths.
        let mut narrow_words = vec![4, 5, 6, 7, 8, 9, 10, 11];
        if native {
            narrow_words.push(13);
        }
        for index in narrow_words {
            let mut invalid = transaction.clone();
            invalid.data[4 + index * 32] = 1;
            assert!(bind_call(&invalid, SENDER).is_err());
        }
        cases.push((transaction, ModuleId::Budget, 1, [0x21; 32]));
    }

    // agent_rpc_dispatch::session_open puts the grant, expiry sequence and action
    // key in that order. Issue a real, bounded protocol grant from fixed inputs.
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
    let mut args = [
        word(96),
        word(9),
        [0x43; 32],
        word(grant.registration_payload.len() as u128),
    ]
    .concat();
    args.extend_from_slice(&grant.registration_payload);
    args.resize(args.len().div_ceil(32) * 32, 0);
    assert_eq!(
        ledger_selector("grantSession(bytes,uint64,bytes32)"),
        [0x11, 0x07, 0xb4, 0x9e]
    );
    let session = call(0x101c, [0x11, 0x07, 0xb4, 0x9e], &args);
    let disclosure = bind_call(&session, SENDER)?;
    assert_eq!(disclosure.expiry.payload_expires_at, 100);
    assert!(disclosure.counterparties.is_empty());
    assert!(disclosure.amounts.is_empty());
    assert_eq!(disclosure.asset, [0; 32]);
    let session_grant = disclosure
        .session_grant
        .as_ref()
        .expect("session registration");
    assert_eq!(session_grant.grant, grant);
    assert_eq!(session_grant.expiry_sequence, 9);
    assert_eq!(session_grant.action_key, [0x43; 32]);
    assert_eq!(session_grant.replacement, None);
    assert_eq!(
        disclosure.construct_method()?,
        Some(ConstructMethod::AuthSession {
            grant,
            expiry_sequence: 9,
            purpose: [0x43; 32],
        })
    );
    cases.push((session, ModuleId::Governance, 5, [0x43; 32]));

    // ProgramOperations::call_precompile's fifteen-word head and four tails.
    // Exercise both ordinary program bytes and the PAXAI retry/expiry envelope.
    let mut envelope = vec![0; 239];
    envelope[..8].copy_from_slice(b"PAXAI1\0\x01");
    envelope[42..74].fill(0x31);
    envelope[194..202].copy_from_slice(&100_u64.to_be_bytes());
    envelope[202..234].fill(0x32);
    for (calldata, key, expiry) in [
        (b"fixed input".as_slice(), [0; 32], u64::MAX),
        (envelope.as_slice(), [0x32; 32], 100),
    ] {
        let native = NativeProgramCall {
            program_id: ProgramId::new([0x31; 32]),
            guest_abi: 5,
            entrypoint: b"layerx_call",
            calldata,
            capabilities: &[0, 0],
            access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
            response_capacity: 16,
            resources: Resources([1000, 2000, 3000, 4, 5, 6, 7]),
        };
        let execution_fee = 73;
        let mut head = vec![native.program_id.bytes(), word(native.guest_abi.into())];
        let mut tails = Vec::new();
        for bytes in [
            native.entrypoint,
            native.calldata,
            native.capabilities,
            native.access_declaration,
        ] {
            head.push(word((15 * 32 + tails.len()) as u128));
            tails.extend_from_slice(&word(bytes.len() as u128));
            tails.extend_from_slice(bytes);
            tails.resize(tails.len().div_ceil(32) * 32, 0);
        }
        head.push(word(native.response_capacity.into()));
        head.extend(native.resources.0.into_iter().map(|n| word(n.into())));
        head.push(word(execution_fee));
        let mut args = head.concat();
        args.extend(tails);
        let transaction = call(0x101d, [0x54, 0xbd, 0x2b, 0x48], &args);
        assert_eq!(
            transaction.data,
            encode_program_call(&native, execution_fee)
        );
        let disclosure = bind_call(&transaction, SENDER)?;
        assert_eq!(disclosure.expiry.payload_expires_at, expiry);
        assert_eq!(disclosure.expiry.not_after, expiry);
        assert_eq!(disclosure.asset, [0; 32]);
        assert!(disclosure.counterparties.is_empty());
        assert!(disclosure.amounts.is_empty());
        let Some(DisclosedNativeOperation::ProgramCall(disclosed)) = &disclosure.native_operation
        else {
            panic!("missing native program call");
        };
        assert_eq!(disclosed.native(), native);
        assert_eq!(
            disclosure.construct_method()?,
            Some(ConstructMethod::ProgramCall {
                call: disclosed.clone(),
                fee_limit: execution_fee,
            })
        );
        cases.push((transaction, ModuleId::Programs, 3, key));
    }

    for (transaction, module, ordinal, key) in cases {
        let disclosure = bind_call(&transaction, SENDER)?;
        assert_eq!(disclosure.activity_type.module(), module);
        assert_eq!(disclosure.activity_type.ordinal(), ordinal);
        assert_eq!(disclosure.idempotency_key, key);
        assert_eq!(disclosure.actor, SENDER);
        assert_eq!(disclosure.authority, SENDER);
        assert_eq!(disclosure.fee_limit, 3_000_000);
        assert_eq!(disclosure.signing_digest()?, transaction.signing_hash());
        let canonical = transaction.encode_unsigned();
        assert_eq!(reencode(&disclosure)?, canonical);
        validate_canonical(&disclosure, &canonical)?;
        validate_call(&transaction, &disclosure, SENDER)?;
        let mut different_sender = SENDER;
        different_sender[0] ^= 1;
        // The EIP-1559 hash omits from; the retained approval must refuse a new
        // sender, and its audit identity must change on an independent binding.
        assert!(validate_call(&transaction, &disclosure, different_sender).is_err());
        assert_ne!(
            disclosure.audit_digest()?,
            bind_call(&transaction, different_sender)?.audit_digest()?
        );
        for index in 0..transaction.data.len() {
            let mut changed = transaction.clone();
            changed.data[index] ^= 1;
            assert!(validate_call(&changed, &disclosure, SENDER).is_err());
            assert!(validate_canonical(&disclosure, &changed.encode_unsigned()).is_err());
            if let Ok(changed_disclosure) = bind_call(&changed, SENDER) {
                assert_ne!(
                    changed_disclosure.signing_digest()?,
                    disclosure.signing_digest()?,
                    "data byte {index}"
                );
            }
        }
        for length in 0..transaction.data.len() {
            let mut truncated = transaction.clone();
            truncated.data.truncate(length);
            assert!(
                bind_call(&truncated, SENDER).is_err(),
                "truncated at {length}"
            );
        }
        let mut trailing = transaction.clone();
        trailing.data.push(0);
        assert!(bind_call(&trailing, SENDER).is_err());
        let mut changed = disclosure.clone();
        changed.idempotency_key[0] ^= 1;
        assert!(reencode(&changed).is_err());
        assert!(validate_canonical(&changed, &canonical).is_err());
    }
    Ok(())
}

fn monetary_transport(
    call: &Eip1559Call,
) -> Result<layerx_crypto::disclosure::Disclosure, Box<dyn std::error::Error>> {
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
    let account = layerx_crypto::account_derivation::derive_from_seed(&[7; 64], 0)?;
    let raw = call.sign(account.evm_secret().expect("transaction key"))?;
    let (received, signature) = decode_signed(&raw)?;
    let sender = recover_sender(&received, &signature)?;
    assert_eq!(sender, account.evm_address());
    assert_eq!(&received, call);
    Ok(bind_call(&received, sender)?)
}

#[test]
pub(crate) fn monetary_call_tests_miss_common_view_program_execution_fee(
) -> Result<(), Box<dyn std::error::Error>> {
    use layerx_crypto::disclosure::call::{
        encode_program_call, CallFeeContext, ExecutionFeeBound, GasFeeBound,
    };
    use layerx_types::intent::ProgramId;
    use layerx_types::program_call::{NativeProgramCall, Resources};
    let native = NativeProgramCall {
        program_id: ProgramId::new([0x31; 32]),
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: &[1, 2, 3, 4],
        capabilities: &[0, 0],
        access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
        response_capacity: 16,
        resources: Resources([1000, 2000, 3000, 4, 5, 6, 7]),
    };
    for execution_fee in [0, 73, u128::MAX] {
        let data = encode_program_call(&native, execution_fee);
        let call = call(0x101d, data[..4].try_into()?, &data[4..]);
        let disclosure = monetary_transport(&call)?;
        let view = disclosure.call_monetary(CallFeeContext::ProgramExecution {
            fee_asset: [0x72; 32],
        })?;
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
                asset: [0x72; 32],
                amount_base_units: execution_fee
            })
        );
        assert!(view
            .to_string()
            .contains(&format!("Execution fee bound: {execution_fee} base units")));
        assert!(view.to_string().contains("Gas fee bound: 3000000 wei"));
        assert!(disclosure.call_monetary(CallFeeContext::GasOnly).is_err());
        let mut changed = disclosure;
        changed.fee_limit = 1;
        assert!(changed
            .call_monetary(CallFeeContext::ProgramExecution {
                fee_asset: [0x72; 32]
            })
            .is_err());
    }
    Ok(())
}

#[test]
pub(crate) fn monetary_call_tests_miss_common_view_bank_denomination_and_uint256(
) -> Result<(), Box<dyn std::error::Error>> {
    use layerx_crypto::disclosure::call::{encode_bank_send, CallFeeContext, CallMonetaryAmount};
    let account = layerx_crypto::account_derivation::derive_from_seed(&[7; 64], 0)?;
    let denomination = format!("layerx/{}", "03".repeat(32));
    for amount in [[0xff; 32], {
        let mut value = [0; 32];
        value[15] = 1;
        value
    }] {
        let data = encode_bank_send(account.evm_address(), [0xb2; 20], &denomination, amount);
        let call = call(0x1001, data[..4].try_into()?, &data[4..]);
        let disclosure = monetary_transport(&call)?;
        let view = disclosure.call_monetary(CallFeeContext::GasOnly)?;
        assert_eq!(
            view.amounts,
            vec![CallMonetaryAmount::Bank {
                role: AmountRole::Transfer,
                denom: denomination.clone(),
                value: amount
            }]
        );
        assert_eq!(view.gas_fee_bound.amount_wei, 3_000_000);
        assert_eq!(view.execution_fee_bound, None);
        assert!(view.to_string().contains(&denomination));
        assert!(view.to_string().contains(if amount == [0xff; 32] {
            "115792089237316195423570985008687907853269984665640564039457584007913129639935"
        } else {
            "340282366920938463463374607431768211456"
        }));
        let mut altered = disclosure;
        altered.counterparties[1].account[0] ^= 1;
        assert!(altered.call_monetary(CallFeeContext::GasOnly).is_err());
    }
    Ok(())
}
