use super::{checked, registry, Result};
use layerx_intents::canonical as hash;
use layerx_types::account::AccountId;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, Payload};

pub(super) const CREATION_KINDS: [&str; 5] = [
    "onboarding-consent",
    "recovery-policy",
    "recovery-policy-with-delay-bounds",
    "budget-create-v1",
    "open-account",
];

pub(super) fn creation_cases(public: [u8; 32]) -> Result<Vec<(&'static str, Payload)>> {
    let consent = historical_consent_payload(public)?;
    let opening = checked(
        layerx_crypto::payments::Payment::OpenAccount { asset: [3; 32] }
            .encode(b"did:layerx:alice"),
    )?;
    Ok(vec![
        (
            "onboarding-consent",
            payload(ModuleId::Governance, 1, &consent)?,
        ),
        (
            "recovery-policy",
            payload(ModuleId::Governance, 3, &recovery(false)?)?,
        ),
        (
            "recovery-policy-with-delay-bounds",
            payload(ModuleId::Governance, 3, &recovery(true)?)?,
        ),
        (
            "budget-create-v1",
            payload(ModuleId::Budget, 1, &native_budget(1)?)?,
        ),
        ("open-account", payload(ModuleId::Asset, 4, &opening)?),
    ])
}

fn canonical_ed25519_public_key(public: [u8; 32]) -> bool {
    const FIELD_PRIME: [u8; 32] = [
        0xed, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
        0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
        0xff, 0x7f,
    ];
    let mut y = public;
    y[31] &= 0x7f;
    if y.iter().all(|byte| *byte == 0) || (y[0] == 1 && y[1..].iter().all(|byte| *byte == 0)) {
        return false;
    }
    (0..32)
        .rev()
        .find_map(|index| {
            if y[index] == FIELD_PRIME[index] {
                None
            } else {
                Some(y[index] < FIELD_PRIME[index])
            }
        })
        .unwrap_or(false)
}

pub(super) fn historical_consent_payload(public: [u8; 32]) -> Result<Vec<u8>> {
    if !canonical_ed25519_public_key(public) {
        return Err("noncanonical consent public key".into());
    }
    let sponsor = checked(hash::did_id_for_protocol(
        &checked(Did::new(b"did:layerx:sponsor"))?,
        3,
    ))?;
    let target = checked(hash::account_id_for_protocol(
        &checked(AccountId::parse("agent:did:layerx:alice:main"))?,
        3,
    ))?;
    let mut bytes = Vec::with_capacity(140);
    bytes.extend_from_slice(&[0x71, 1, 3, 5]);
    bytes.extend_from_slice(&sponsor);
    bytes.extend_from_slice(&[3; 32]);
    bytes.extend_from_slice(&target);
    bytes.extend_from_slice(&[4; 32]);
    bytes.extend_from_slice(&1010_u64.to_be_bytes());
    Ok(bytes)
}

pub(super) fn creation_limits(payload: &Payload) -> (u64, u128) {
    if payload.activity_type().module() == ModuleId::Governance
        && payload.activity_type().ordinal() == 1
    {
        (0, 0)
    } else {
        let sequence = match payload.activity_type().module() {
            ModuleId::Governance if payload.as_bytes().len() == 86 => 8,
            ModuleId::Governance => 7,
            ModuleId::Budget => 9,
            ModuleId::Asset => 10,
            other => panic!("undeclared native creation module: {other:?}"),
        };
        (sequence, 1)
    }
}

fn account(name: &str) -> Result<[u8; 32]> {
    checked(hash::account_id_for_protocol(
        &checked(AccountId::parse(name))?,
        3,
    ))
}

pub(super) fn payload(module: ModuleId, ordinal: u16, bytes: &[u8]) -> Result<Payload> {
    checked(Payload::new(
        &registry()?,
        checked(ActivityType::new(module, ordinal))?,
        bytes,
    ))
}

pub(super) fn native_budget(version: u16) -> Result<Vec<u8>> {
    checked(layerx_intents::vectors::native_budget_create(
        version,
        account(&format!(
            "agent:did:layerx:alice:budget:{}",
            "08".repeat(32)
        ))?,
        account(&format!("agent:did:layerx:alice:asset:{}", "03".repeat(32)))?,
    ))
}

pub(super) fn recovery(extended: bool) -> Result<Vec<u8>> {
    checked(layerx_intents::vectors::native_recovery_policy(
        extended,
        checked(hash::did_id_for_protocol(
            &checked(Did::new(b"did:layerx:alice"))?,
            3,
        ))?,
    ))
}

#[allow(clippy::needless_pass_by_value)]
pub(crate) fn setup_call(
    payload: Payload,
    (sequence, fee): (u64, u128),
) -> Result<layerx_crypto::evm_transaction::Eip1559Call> {
    use layerx_crypto::disclosure::call::ledger_selector;
    let bytes = payload.as_bytes();
    let (suffix, data) = match payload.activity_type().module() {
        ModuleId::Budget => {
            if bytes.len() != 211 {
                let version = bytes
                    .get(..2)
                    .map(|bytes| u16::from_be_bytes([bytes[0], bytes[1]]))
                    .ok_or("truncated budget version")?;
                return Err(format!(
                    "budget version {version} has no budgetCreate call form for {} bytes",
                    bytes.len()
                )
                .into());
            }
            let mut data = ledger_selector("budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)").to_vec();
            for (start, length) in [
                (2, 32),
                (34, 32),
                (66, 32),
                (98, 32),
                (130, 16),
                (146, 16),
                (162, 16),
                (178, 8),
                (186, 8),
                (194, 8),
                (202, 8),
                (210, 1),
            ] {
                let mut word = [0; 32];
                word[32 - length..]
                    .copy_from_slice(bytes.get(start..start + length).ok_or("truncated budget")?);
                data.extend(word);
            }
            (0x1b, data)
        }
        ModuleId::Governance => {
            let mut data = ledger_selector("recordLifecycle(bytes,bytes32)").to_vec();
            let mut offset = [0; 32];
            offset[31] = 64;
            data.extend(offset);
            data.extend([4; 32]);
            let mut length = [0; 32];
            length[24..].copy_from_slice(&(bytes.len() as u64).to_be_bytes());
            data.extend(length);
            data.extend(bytes);
            data.resize(4 + 96 + bytes.len().div_ceil(32) * 32, 0);
            (0x1c, data)
        }
        ModuleId::Asset => {
            let payment = checked(layerx_crypto::payments::Payment::decode(
                ModuleId::Asset,
                payload.activity_type().ordinal(),
                bytes,
                b"did:layerx:alice",
            ))?;
            (
                0x1a,
                checked(layerx_crypto::disclosure::call::encode_payment_call(
                    &payment,
                    b"did:layerx:alice",
                ))?,
            )
        }
        _ => return Err("unsupported setup payload".into()),
    };
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, suffix]);
    Ok(layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: sequence,
        max_priority_fee_per_gas: 0,
        max_fee_per_gas: fee,
        gas_limit: 1,
        to,
        data,
    })
}
