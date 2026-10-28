use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::{AmountRole, CounterpartyRole, DisclosureError};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::payments::Payment;
use layerx_crypto::send::{encode_send_envelope, EnvelopeOptions};
use layerx_types::payload::ModuleId;

const SENDER: [u8; 20] = [0xa1; 20];

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
    let options = EnvelopeOptions {
        actor: "did:layerx:alice",
        public_key: [0; 32],
        protocol_version: 3,
        network_id: 17,
        identity_sequence: 7,
        idempotency_key: [4; 32],
        fee_limit: 20,
        not_before: 10,
        not_after: 100,
    };
    let activity = encode_send_envelope(&payload, &options)?.disclosure;
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
fn unknown_target_or_selector_is_refused() -> Result<(), Box<dyn std::error::Error>> {
    for original in fixtures()? {
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
