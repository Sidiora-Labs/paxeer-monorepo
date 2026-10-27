use super::*;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ConstructMethod {
    LedgerSend {
        payload: Vec<u8>,
    },
    LedgerIssueGrant {
        grant: Grant,
    },
    LedgerDraw {
        from: [u8; 32],
        asset: [u8; 32],
        amount: u128,
        grant: [u8; 32],
        receiver_sequence: u64,
        idempotency_key: [u8; 32],
        context_hash: [u8; 32],
    },
    LedgerRevokeGrant {
        grant: [u8; 32],
        revocation_sequence: u64,
    },
    BankSend {
        from: [u8; 20],
        to: [u8; 20],
        denom: String,
        amount: [u8; 32],
    },
}

fn word(args: &[u8], index: usize) -> Result<[u8; 32], DisclosureError> {
    args.get(index * 32..(index + 1) * 32)
        .ok_or(DisclosureError::MalformedPayload)?
        .try_into()
        .map_err(|_| DisclosureError::MalformedPayload)
}

fn uint<const N: usize>(word: [u8; 32]) -> Result<[u8; N], DisclosureError> {
    if word[..32 - N].iter().any(|byte| *byte != 0) {
        return Err(DisclosureError::MalformedPayload);
    }
    word[32 - N..]
        .try_into()
        .map_err(|_| DisclosureError::MalformedPayload)
}

fn dynamic(args: &[u8], head_words: usize, index: usize) -> Result<&[u8], DisclosureError> {
    let offset = usize::from_be_bytes(uint(word(args, index)?)?);
    if offset != head_words * 32 {
        return Err(DisclosureError::MalformedPayload);
    }
    let length = usize::from_be_bytes(uint(word(args, head_words)?)?);
    let start = offset
        .checked_add(32)
        .ok_or(DisclosureError::MalformedPayload)?;
    let end = start
        .checked_add(length)
        .ok_or(DisclosureError::MalformedPayload)?;
    let padded_end = end
        .checked_add(31)
        .ok_or(DisclosureError::MalformedPayload)?
        & !31;
    if padded_end != args.len()
        || args
            .get(end..padded_end)
            .ok_or(DisclosureError::MalformedPayload)?
            .iter()
            .any(|byte| *byte != 0)
    {
        return Err(DisclosureError::MalformedPayload);
    }
    args.get(start..end)
        .ok_or(DisclosureError::MalformedPayload)
}

fn target(address: &[u8; 20], suffix: [u8; 2]) -> bool {
    address[..18] == [0; 18] && address[18..] == suffix
}

pub(super) fn decode_method(call: &Eip1559Call) -> Result<ConstructMethod, DisclosureError> {
    let selector: [u8; 4] = call
        .data
        .get(..4)
        .ok_or(DisclosureError::MalformedPayload)?
        .try_into()
        .map_err(|_| DisclosureError::MalformedPayload)?;
    let args = &call.data[4..];
    if target(&call.to, [0x10, 0x1a]) {
        return match selector {
            [0x2b, 0xb1, 0xae, 0x7c] => {
                let payload = dynamic(args, 1, 0)?;
                decode_send(payload).map_err(|_| DisclosureError::MalformedPayload)?;
                Ok(ConstructMethod::LedgerSend {
                    payload: payload.to_vec(),
                })
            }
            [0xd0, 0xd5, 0x34, 0x80] => {
                let payload = dynamic(args, 1, 0)?;
                let Payment::IssueGrant(grant) = Payment::decode(ModuleId::Asset, 7, payload, &[])
                    .map_err(|_| DisclosureError::MalformedPayload)?
                else {
                    return Err(DisclosureError::MalformedPayload);
                };
                Ok(ConstructMethod::LedgerIssueGrant { grant })
            }
            [0x2e, 0x89, 0x49, 0x0f] if args.len() == 7 * 32 => Ok(ConstructMethod::LedgerDraw {
                from: word(args, 0)?,
                asset: word(args, 1)?,
                amount: u128::from_be_bytes(uint(word(args, 2)?)?),
                grant: word(args, 3)?,
                receiver_sequence: u64::from_be_bytes(uint(word(args, 4)?)?),
                idempotency_key: word(args, 5)?,
                context_hash: word(args, 6)?,
            }),
            [0x58, 0xd1, 0x83, 0xa8] if args.len() == 2 * 32 => {
                Ok(ConstructMethod::LedgerRevokeGrant {
                    grant: word(args, 0)?,
                    revocation_sequence: u64::from_be_bytes(uint(word(args, 1)?)?),
                })
            }
            _ => Err(DisclosureError::MalformedPayload),
        };
    }
    if target(&call.to, [0x10, 0x01]) && selector == [0x5c, 0x05, 0x96, 0x1b] {
        return Ok(ConstructMethod::BankSend {
            from: uint(word(args, 0)?)?,
            to: uint(word(args, 1)?)?,
            denom: std::str::from_utf8(dynamic(args, 4, 2)?)
                .map_err(|_| DisclosureError::MalformedPayload)?
                .to_owned(),
            amount: word(args, 3)?,
        });
    }
    Err(DisclosureError::MalformedPayload)
}

pub(super) fn fields(
    call: &Eip1559Call,
    from: [u8; 20],
) -> Result<DisclosureFields, DisclosureError> {
    let method = decode_method(call)?;
    let ordinal = match &method {
        ConstructMethod::LedgerSend { .. } | ConstructMethod::BankSend { .. } => 5,
        ConstructMethod::LedgerIssueGrant { .. } => 7,
        ConstructMethod::LedgerDraw { .. } => 6,
        ConstructMethod::LedgerRevokeGrant { .. } => 8,
    };
    let mut fields = DisclosureFields {
        activity_type: ActivityType::new(ModuleId::Asset, ordinal)
            .map_err(|_| DisclosureError::MalformedPayload)?,
        actor: from.to_vec(),
        authority: from.to_vec(),
        counterparties: Vec::new(),
        amounts: Vec::new(),
        asset: [0; 32],
        fee_limit: u128::from(call.gas_limit)
            .checked_mul(call.max_fee_per_gas)
            .ok_or(DisclosureError::MalformedPayload)?,
        expiry: Expiry {
            not_before: 0,
            not_after: u64::MAX,
            payload_expires_at: u64::MAX,
        },
        idempotency_key: [0; 32],
        evm_payout_binding: None,
        withdrawal: None,
        payment: None,
        authority_grant: None,
        session_grant: None,
        onboarding: None,
        native_operation: None,
    };
    match method {
        ConstructMethod::LedgerSend { payload } => {
            let send = decode_send(&payload).map_err(|_| DisclosureError::MalformedPayload)?;
            disclose_party(
                &mut fields.counterparties,
                CounterpartyRole::Payer,
                send.from,
            );
            disclose_party(
                &mut fields.counterparties,
                CounterpartyRole::Recipient,
                send.to,
            );
            disclose_amount(&mut fields.amounts, AmountRole::Transfer, send.amount);
            fields.asset = send.asset;
            fields.idempotency_key = send.idempotency_key;
            fields.expiry.payload_expires_at = send.expires_at;
        }
        ConstructMethod::LedgerIssueGrant { grant } => {
            disclose_grant(&grant, &mut fields.counterparties, &mut fields.amounts);
            fields.asset = grant.asset;
            fields.expiry.payload_expires_at = grant.expiration;
            fields.payment = Some(Payment::IssueGrant(grant));
        }
        ConstructMethod::LedgerDraw {
            from,
            asset,
            amount,
            idempotency_key,
            ..
        } => {
            disclose_party(&mut fields.counterparties, CounterpartyRole::Payer, from);
            disclose_amount(&mut fields.amounts, AmountRole::Transfer, amount);
            fields.asset = asset;
            fields.idempotency_key = idempotency_key;
        }
        ConstructMethod::LedgerRevokeGrant {
            grant,
            revocation_sequence,
        } => {
            fields.payment = Some(Payment::RevokeGrant {
                grant,
                revocation_sequence,
            });
        }
        ConstructMethod::BankSend { from, to, .. } => {
            // Bank denominations and uint256 amounts are exposed by construct_method;
            // they are not protocol asset ids or uint128 ledger amounts.
            let mut payer = [0; 32];
            payer[12..].copy_from_slice(&from);
            let mut recipient = [0; 32];
            recipient[12..].copy_from_slice(&to);
            disclose_party(&mut fields.counterparties, CounterpartyRole::Payer, payer);
            disclose_party(
                &mut fields.counterparties,
                CounterpartyRole::Recipient,
                recipient,
            );
        }
    }
    Ok(fields)
}

pub(super) fn payload_sequence(call: &Eip1559Call) -> Result<Option<u64>, DisclosureError> {
    match decode_method(call)? {
        ConstructMethod::LedgerSend { payload } => Ok(Some(decode_send(&payload)?.sequence)),
        ConstructMethod::LedgerDraw {
            receiver_sequence, ..
        } => Ok(Some(receiver_sequence)),
        _ => Ok(None),
    }
}

/// Discloses a supported construct call from its canonical ABI arguments.
///
/// # Errors
/// Refuses unknown targets or selectors, noncanonical ABI, invalid payloads,
/// trailing data, and a maximum transaction fee that overflows uint128.
pub fn bind_call(call: &Eip1559Call, from: [u8; 20]) -> Result<Disclosure, DisclosureError> {
    let fields = fields(call, from)?;
    Ok(Disclosure {
        activity_type: fields.activity_type,
        actor: fields.actor,
        authority: fields.authority,
        counterparties: fields.counterparties,
        amounts: fields.amounts,
        asset: fields.asset,
        fee_limit: fields.fee_limit,
        expiry: fields.expiry,
        idempotency_key: fields.idempotency_key,
        evm_payout_binding: fields.evm_payout_binding,
        withdrawal: fields.withdrawal,
        payment: fields.payment,
        authority_grant: fields.authority_grant,
        session_grant: fields.session_grant,
        onboarding: fields.onboarding,
        native_operation: fields.native_operation,
        source: Source::Call(call.clone()),
        signing_digest: call.signing_hash(),
        budget_context: None,
        call_sender: Some(from),
    })
}
