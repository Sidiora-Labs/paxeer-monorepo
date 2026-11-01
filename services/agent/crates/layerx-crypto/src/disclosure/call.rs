use super::*;
use crate::payments::Registration;
use sha3::{Digest as _, Keccak256};

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ConstructMethod {
    LedgerOpenAccount {
        asset: [u8; 32],
    },
    LedgerRegister {
        registration: Registration,
    },
    LedgerMint {
        asset: [u8; 32],
        to: [u8; 32],
        amount: u128,
    },
    LedgerBurn {
        asset: [u8; 32],
        from: [u8; 32],
        amount: u128,
    },
    LedgerSend {
        payload: Vec<u8>,
    },
    LedgerIssueGrant {
        grant: Grant,
    },
    LedgerDraw {
        recipient: [u8; 32],
        purpose_hash: [u8; 32],
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
    PayBudgetCreate {
        payload: Vec<u8>,
    },
    PayBudgetFund {
        budget: [u8; 32],
        amount: u128,
    },
    PayBudgetDefund {
        budget: [u8; 32],
        amount: u128,
    },
    AuthLifecycle {
        payload: Vec<u8>,
        idempotency_key: [u8; 32],
    },
    PointerTransfer {
        pointer: [u8; 20],
        to: [u8; 20],
        amount: [u8; 32],
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

fn tail<'a>(args: &'a [u8], index: usize, cursor: &mut usize) -> Result<&'a [u8], DisclosureError> {
    let offset = usize::from_be_bytes(uint(word(args, index)?)?);
    if offset != *cursor {
        return Err(DisclosureError::MalformedPayload);
    }
    let length = usize::from_be_bytes(uint(word(args, offset / 32)?)?);
    let start = offset
        .checked_add(32)
        .ok_or(DisclosureError::MalformedPayload)?;
    let end = start
        .checked_add(length)
        .ok_or(DisclosureError::MalformedPayload)?;
    let padded = end
        .checked_add(31)
        .ok_or(DisclosureError::MalformedPayload)?
        & !31;
    if args
        .get(end..padded)
        .ok_or(DisclosureError::MalformedPayload)?
        .iter()
        .any(|b| *b != 0)
    {
        return Err(DisclosureError::MalformedPayload);
    }
    *cursor = padded;
    args.get(start..end)
        .ok_or(DisclosureError::MalformedPayload)
}

pub fn ledger_selector(signature: &str) -> [u8; 4] {
    let hash = Keccak256::digest(signature.as_bytes());
    [hash[0], hash[1], hash[2], hash[3]]
}

fn abi_word(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

fn abi_tail(value: &[u8]) -> Vec<u8> {
    let mut out = abi_word(value.len() as u128).to_vec();
    out.extend_from_slice(value);
    out.resize(out.len().div_ceil(32) * 32, 0);
    out
}

/// Encodes the ledger ABI after validating the canonical payment and its authorization.
pub fn encode_payment_call(payment: &Payment, actor: &[u8]) -> Result<Vec<u8>, DisclosureError> {
    let payload = payment.encode(actor)?;
    let (signature, args) = match payment {
        Payment::OpenAccount { asset } => ("openAccount(bytes32)", asset.to_vec()),
        Payment::Register(r) => {
            let symbol = abi_tail(r.symbol.as_bytes());
            let name = abi_tail(r.name.as_bytes());
            let custody = abi_tail(&r.custody_ref);
            let mut args = Vec::new();
            for word in [
                r.asset,
                r.salt,
                abi_word(256),
                abi_word((256 + symbol.len()) as u128),
                abi_word(r.decimals.into()),
                abi_word(r.supply_cap),
                abi_word(r.issuer_kind.into()),
                abi_word((256 + symbol.len() + name.len()) as u128),
            ] {
                args.extend_from_slice(&word);
            }
            args.extend(symbol);
            args.extend(name);
            args.extend(custody);
            (
                "register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)",
                args,
            )
        }
        Payment::Mint { asset, to, amount } => (
            "mint(bytes32,bytes32,uint128)",
            [*asset, *to, abi_word(*amount)].concat(),
        ),
        Payment::Burn {
            asset,
            from,
            amount,
        } => (
            "burn(bytes32,bytes32,uint128)",
            [*asset, *from, abi_word(*amount)].concat(),
        ),
        Payment::IssueGrant(_) => (
            "issueGrant(bytes)",
            [abi_word(32).to_vec(), abi_tail(&payload)].concat(),
        ),
        Payment::RevokeGrant {
            grant,
            revocation_sequence,
        } => (
            "revokeGrant(bytes32,uint64)",
            [*grant, abi_word((*revocation_sequence).into())].concat(),
        ),
        Payment::Receive {
            to,
            amount,
            grant,
            sequence,
            idempotency_key,
            context_hash,
            payer_grant,
            ..
        } => (
            "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)",
            [
                *grant,
                *to,
                abi_word(*amount),
                *idempotency_key,
                abi_word((*sequence).into()),
                payer_grant.purpose_hash,
                *context_hash,
            ]
            .concat(),
        ),
        _ => return Err(DisclosureError::MalformedPayload),
    };
    Ok([ledger_selector(signature).to_vec(), args].concat())
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
            value if value == ledger_selector("openAccount(bytes32)") && args.len() == 32 => {
                Ok(ConstructMethod::LedgerOpenAccount {
                    asset: word(args, 0)?,
                })
            }
            value
                if value
                    == ledger_selector(
                        "register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)",
                    ) =>
            {
                let mut cursor = 8 * 32;
                let symbol = std::str::from_utf8(tail(args, 2, &mut cursor)?)
                    .map_err(|_| DisclosureError::MalformedPayload)?
                    .to_owned();
                let name = std::str::from_utf8(tail(args, 3, &mut cursor)?)
                    .map_err(|_| DisclosureError::MalformedPayload)?
                    .to_owned();
                let custody_ref = tail(args, 7, &mut cursor)?.to_vec();
                let registration = Registration {
                    asset: word(args, 0)?,
                    salt: word(args, 1)?,
                    symbol,
                    name,
                    decimals: uint::<1>(word(args, 4)?)?[0],
                    supply_cap: u128::from_be_bytes(uint(word(args, 5)?)?),
                    issuer_kind: uint::<1>(word(args, 6)?)?[0],
                    custody_ref,
                };
                if cursor != args.len()
                    || registration.symbol.is_empty()
                    || registration.symbol.len() > 16
                    || !registration.symbol.is_ascii()
                    || registration.name.is_empty()
                    || registration.name.len() > 32
                    || registration.decimals > 38
                    || registration.custody_ref.len() > 128
                    || !matches!(registration.issuer_kind, 1 | 2)
                    || (registration.issuer_kind == 1 && !registration.custody_ref.is_empty())
                {
                    return Err(DisclosureError::MalformedPayload);
                }
                Ok(ConstructMethod::LedgerRegister { registration })
            }
            value
                if value == ledger_selector("mint(bytes32,bytes32,uint128)")
                    && args.len() == 96 =>
            {
                let amount = u128::from_be_bytes(uint(word(args, 2)?)?);
                if amount == 0 {
                    return Err(DisclosureError::MalformedPayload);
                }
                Ok(ConstructMethod::LedgerMint {
                    asset: word(args, 0)?,
                    to: word(args, 1)?,
                    amount,
                })
            }
            value
                if value == ledger_selector("burn(bytes32,bytes32,uint128)")
                    && args.len() == 96 =>
            {
                let amount = u128::from_be_bytes(uint(word(args, 2)?)?);
                if amount == 0 {
                    return Err(DisclosureError::MalformedPayload);
                }
                Ok(ConstructMethod::LedgerBurn {
                    asset: word(args, 0)?,
                    from: word(args, 1)?,
                    amount,
                })
            }
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
                recipient: word(args, 1)?,
                purpose_hash: word(args, 5)?,
                amount: u128::from_be_bytes(uint(word(args, 2)?)?),
                grant: word(args, 0)?,
                receiver_sequence: u64::from_be_bytes(uint(word(args, 4)?)?),
                idempotency_key: word(args, 3)?,
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
    if target(&call.to, [0x10, 0x1b]) {
        return match selector {
            value
                if value
                    == ledger_selector(
                        "budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)",
                    ) && args.len() == 384 =>
            {
                let mut payload = Vec::with_capacity(211);
                payload.extend_from_slice(&1_u16.to_be_bytes());
                for index in 0..4 {
                    payload.extend_from_slice(&word(args, index)?);
                }
                for index in 4..7 {
                    payload.extend_from_slice(&uint::<16>(word(args, index)?)?);
                }
                for index in 7..11 {
                    payload.extend_from_slice(&uint::<8>(word(args, index)?)?);
                }
                payload.extend_from_slice(&uint::<1>(word(args, 11)?)?);
                Ok(ConstructMethod::PayBudgetCreate { payload })
            }
            value
                if value == ledger_selector("budgetFund(bytes32,uint128)")
                    && args.len() == 64 =>
            {
                let amount = u128::from_be_bytes(uint(word(args, 1)?)?);
                if amount == 0 {
                    return Err(DisclosureError::MalformedPayload);
                }
                Ok(ConstructMethod::PayBudgetFund {
                    budget: word(args, 0)?,
                    amount,
                })
            }
            value
                if value == ledger_selector("budgetDefund(bytes32,uint128)")
                    && args.len() == 64 =>
            {
                let amount = u128::from_be_bytes(uint(word(args, 1)?)?);
                if amount == 0 {
                    return Err(DisclosureError::MalformedPayload);
                }
                Ok(ConstructMethod::PayBudgetDefund {
                    budget: word(args, 0)?,
                    amount,
                })
            }
            _ => Err(DisclosureError::MalformedPayload),
        };
    }
    if target(&call.to, [0x10, 0x1c]) {
        if selector != ledger_selector("recordLifecycle(bytes,bytes32)") {
            return Err(DisclosureError::MalformedPayload);
        }
        let payload = dynamic(args, 2, 0)?;
        if !(4..=1024).contains(&payload.len())
            || payload[0] != 0x71
            || !matches!(
                (payload[1], payload[2], payload[3], payload.len()),
                (1, 0, 2, 68) | (1, 2, 1, 8..=1024) | (3, 0, 3, 70) | (3, 0, 5, 86)
            )
        {
            return Err(DisclosureError::MalformedPayload);
        }
        return Ok(ConstructMethod::AuthLifecycle {
            payload: payload.to_vec(),
            idempotency_key: word(args, 1)?,
        });
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
    if !(call.to[..18] == [0; 18] && call.to[18] == 0x10)
        && selector == ledger_selector("transfer(address,uint256)")
        && args.len() == 64
    {
        return Ok(ConstructMethod::PointerTransfer {
            pointer: call.to,
            to: uint(word(args, 0)?)?,
            amount: word(args, 1)?,
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
        ConstructMethod::LedgerOpenAccount { .. } => 4,
        ConstructMethod::LedgerRegister { .. } | ConstructMethod::PayBudgetCreate { .. } => 1,
        ConstructMethod::LedgerMint { .. } => 10,
        ConstructMethod::LedgerBurn { .. } => 11,
        ConstructMethod::LedgerSend { .. }
        | ConstructMethod::BankSend { .. }
        | ConstructMethod::PointerTransfer { .. } => 5,
        ConstructMethod::LedgerIssueGrant { .. } => 7,
        ConstructMethod::LedgerDraw { .. } => 6,
        ConstructMethod::LedgerRevokeGrant { .. } => 8,
        ConstructMethod::PayBudgetFund { .. } => 2,
        ConstructMethod::PayBudgetDefund { .. } => 8,
        ConstructMethod::AuthLifecycle { payload, .. } => u16::from(payload[1]),
    };
    let module = if matches!(
        &method,
        ConstructMethod::PayBudgetCreate { .. }
            | ConstructMethod::PayBudgetFund { .. }
            | ConstructMethod::PayBudgetDefund { .. }
    ) {
        ModuleId::Budget
    } else if matches!(&method, ConstructMethod::AuthLifecycle { .. }) {
        ModuleId::Governance
    } else {
        ModuleId::Asset
    };
    let mut fields = DisclosureFields {
        activity_type: ActivityType::new(module, ordinal)
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
        ConstructMethod::LedgerOpenAccount { asset } => {
            fields.asset = asset;
            fields.payment = Some(Payment::OpenAccount { asset });
        }
        ConstructMethod::LedgerRegister { registration } => {
            fields.asset = registration.asset;
            disclose_amount(
                &mut fields.amounts,
                AmountRole::SupplyCap,
                registration.supply_cap,
            );
            fields.payment = Some(Payment::Register(registration));
        }
        ConstructMethod::LedgerMint { asset, to, amount } => {
            fields.asset = asset;
            disclose_party(&mut fields.counterparties, CounterpartyRole::Recipient, to);
            disclose_amount(&mut fields.amounts, AmountRole::Transfer, amount);
            fields.payment = Some(Payment::Mint { asset, to, amount });
        }
        ConstructMethod::LedgerBurn {
            asset,
            from,
            amount,
        } => {
            fields.asset = asset;
            disclose_party(&mut fields.counterparties, CounterpartyRole::Payer, from);
            disclose_amount(&mut fields.amounts, AmountRole::Transfer, amount);
            fields.payment = Some(Payment::Burn {
                asset,
                from,
                amount,
            });
        }
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
            recipient,
            amount,
            idempotency_key,
            ..
        } => {
            disclose_party(
                &mut fields.counterparties,
                CounterpartyRole::Recipient,
                recipient,
            );
            disclose_amount(&mut fields.amounts, AmountRole::Transfer, amount);
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
        ConstructMethod::PayBudgetCreate { payload } => {
            fields.idempotency_key = payload[2..34]
                .try_into()
                .map_err(|_| DisclosureError::MalformedPayload)?;
            fields.asset = payload[66..98]
                .try_into()
                .map_err(|_| DisclosureError::MalformedPayload)?;
            let amount = u128::from_be_bytes(
                payload[162..178]
                    .try_into()
                    .map_err(|_| DisclosureError::MalformedPayload)?,
            );
            disclose_amount(&mut fields.amounts, AmountRole::Transfer, amount);
        }
        ConstructMethod::PayBudgetFund { budget, amount } => {
            disclose_party(&mut fields.counterparties, CounterpartyRole::Payer, budget);
            disclose_amount(&mut fields.amounts, AmountRole::Transfer, amount);
        }
        ConstructMethod::PayBudgetDefund { budget, amount } => {
            disclose_party(&mut fields.counterparties, CounterpartyRole::Payer, budget);
            disclose_amount(&mut fields.amounts, AmountRole::Transfer, amount);
        }
        ConstructMethod::AuthLifecycle {
            idempotency_key, ..
        } => {
            fields.idempotency_key = idempotency_key;
        }
        ConstructMethod::PointerTransfer { to, amount, .. } => {
            let mut recipient = [0; 32];
            recipient[12..].copy_from_slice(&to);
            disclose_party(
                &mut fields.counterparties,
                CounterpartyRole::Recipient,
                recipient,
            );
            disclose_amount(
                &mut fields.amounts,
                AmountRole::Transfer,
                u128::from_be_bytes(uint(amount)?),
            );
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
        ConstructMethod::AuthLifecycle { .. } => Ok(None),
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn pointer_transfer_disclosure_names_the_recipient_and_amount() {
        let pointer = [3; 20];
        let from = [4; 20];
        let to = [5; 20];
        let mut recipient = [0; 32];
        recipient[12..].copy_from_slice(&to);
        for amount in [1, 50, u128::MAX] {
            let call = Eip1559Call {
                chain_id: 125,
                nonce: 42,
                max_priority_fee_per_gas: 2,
                max_fee_per_gas: 30,
                gas_limit: 100_000,
                to: pointer,
                data: [
                    ledger_selector("transfer(address,uint256)").to_vec(),
                    recipient.to_vec(),
                    abi_word(amount).to_vec(),
                ]
                .concat(),
            };
            let disclosure = bind_call(&call, from).expect("pointer transfer disclosure");
            assert_eq!(
                disclosure.construct_method().expect("pointer ABI"),
                Some(ConstructMethod::PointerTransfer {
                    pointer,
                    to,
                    amount: abi_word(amount),
                })
            );
            assert_eq!(disclosure.actor, from);
            assert_eq!(disclosure.authority, from);
            assert_eq!(
                disclosure.activity_type,
                ActivityType::new(ModuleId::Asset, 5).expect("asset send")
            );
            assert_eq!(
                disclosure.counterparties,
                vec![Counterparty {
                    role: CounterpartyRole::Recipient,
                    account: recipient,
                }]
            );
            assert_eq!(
                disclosure.amounts,
                vec![DisclosedAmount {
                    role: AmountRole::Transfer,
                    value: amount,
                }]
            );
            assert_eq!(
                disclosure.signing_digest().expect("digest"),
                call.signing_hash()
            );
            for suffix in 0x1000_u16..=0x10ff {
                let mut precompile = call.clone();
                precompile.to = [0; 20];
                precompile.to[18..].copy_from_slice(&suffix.to_be_bytes());
                assert!(bind_call(&precompile, from).is_err());
            }
            for suffix in [0x0fff_u16, 0x1100] {
                let mut boundary = call.clone();
                boundary.to = [0; 20];
                boundary.to[18..].copy_from_slice(&suffix.to_be_bytes());
                assert!(bind_call(&boundary, from).is_ok());
            }
            let mut malformed = call.clone();
            malformed.data[4] = 1;
            assert!(bind_call(&malformed, from).is_err());
            let mut trailing = call.clone();
            trailing.data.push(0);
            assert!(bind_call(&trailing, from).is_err());
            let mut truncated = call.clone();
            truncated.data.pop();
            assert!(bind_call(&truncated, from).is_err());
            let mut wrong_selector = call.clone();
            wrong_selector.data[0] ^= 1;
            assert!(bind_call(&wrong_selector, from).is_err());
            let mut oversized = call;
            oversized.data[36] = 1;
            assert!(bind_call(&oversized, from).is_err());
        }
    }
}
