use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::program_lifecycle::{
    NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown, ProgramUpgradePolicy,
    ProgramWindDownOperation,
};
use sha2::{Digest as _, Sha256};

use crate::programs::{ProgramOperationError, MAX_SIGNED_ACTIVITY_BYTES, PROGRAMS_PRECOMPILE};

fn word(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

fn abi(selector: [u8; 4], mut head: Vec<[u8; 32]>, dynamic: &[(usize, &[u8])]) -> Vec<u8> {
    let mut tail = Vec::new();
    for (index, bytes) in dynamic {
        head[*index] = word((head.len() * 32 + tail.len()) as u128);
        tail.extend_from_slice(&word(bytes.len() as u128));
        tail.extend_from_slice(bytes);
        tail.resize(tail.len().next_multiple_of(32), 0);
    }
    let mut data = selector.to_vec();
    for value in head {
        data.extend_from_slice(&value);
    }
    data.extend_from_slice(&tail);
    data
}

/// Encodes the merged Programs deploy ABI.
/// # Errors
/// Refuses invalid native fields or a code hash that does not match the Wasm.
pub fn deploy_calldata(value: NativeProgramDeploy<'_>) -> Result<Vec<u8>, ProgramOperationError> {
    value.encode().map_err(|_| ProgramOperationError::Decode)?;
    if <[u8; 32]>::from(Sha256::digest(value.wasm)) != value.new_hash {
        return Err(ProgramOperationError::IdentityMismatch);
    }
    let (policy, authority) = match value.policy {
        ProgramUpgradePolicy::Immutable => (0, [0; 32]),
        ProgramUpgradePolicy::Authority(authority) => (1, authority),
    };
    Ok(abi(
        [0xbd, 0xe3, 0xff, 0x66],
        vec![
            value.program_id.bytes(),
            word(value.guest_abi.into()),
            word(policy),
            authority,
            value.new_hash,
            [0; 32],
            [0; 32],
        ],
        &[(5, value.interface.unwrap_or_default()), (6, value.wasm)],
    ))
}

/// Encodes the merged Programs upgrade ABI, including its fee ceiling.
/// # Errors
/// Refuses invalid native fields or a code hash that does not match the Wasm.
pub fn upgrade_calldata(
    value: NativeProgramUpgrade<'_>,
    fee_limit: u128,
) -> Result<Vec<u8>, ProgramOperationError> {
    value.encode().map_err(|_| ProgramOperationError::Decode)?;
    if <[u8; 32]>::from(Sha256::digest(value.wasm)) != value.new_hash {
        return Err(ProgramOperationError::IdentityMismatch);
    }
    let flags =
        u128::from(!value.migration_hook.is_empty()) | (u128::from(value.clear_interface) << 1);
    Ok(abi(
        [0xa8, 0x60, 0x92, 0x9f],
        vec![
            value.program_id.bytes(),
            word(value.guest_abi.into()),
            word(flags),
            value.old_hash,
            value.new_hash,
            [0; 32],
            [0; 32],
            [0; 32],
            word(fee_limit),
        ],
        &[
            (5, value.migration_hook),
            (6, value.interface.unwrap_or_default()),
            (7, value.wasm),
        ],
    ))
}

/// Encodes the mode-specific merged Programs wind-down ABI.
/// # Errors
/// Refuses invalid native fields and bounds.
pub fn wind_down_calldata(
    value: NativeProgramWindDown<'_>,
) -> Result<Vec<u8>, ProgramOperationError> {
    value.encode().map_err(|_| ProgramOperationError::Decode)?;
    let program = value.program_id.bytes();
    Ok(match value.operation {
        ProgramWindDownOperation::Route {
            account,
            asset,
            destination,
            seed,
        } => abi(
            [0x23, 0x33, 0x0d, 0xd5],
            vec![program, account, asset, destination, [0; 32]],
            &[(4, seed)],
        ),
        ProgramWindDownOperation::Deprecate {
            exit_program,
            deadline_batch,
        } => abi(
            [0x86, 0xba, 0x1a, 0x99],
            vec![program, exit_program, word(deadline_batch.into())],
            &[],
        ),
        ProgramWindDownOperation::Tombstone => abi([0xa5, 0x12, 0xf1, 0xa9], vec![program], &[]),
        ProgramWindDownOperation::Exit { account } => {
            abi([0x4d, 0xc5, 0x07, 0x30], vec![program, account], &[])
        }
        ProgramWindDownOperation::BoundedExit {
            account,
            maximum_exit_amount,
        } => abi(
            [0xad, 0xe5, 0xd9, 0xe6],
            vec![program, account, word(maximum_exit_amount)],
            &[],
        ),
    })
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NativeProgramLifecycleRequest {
    ordinal: u16,
    signed_activity: Vec<u8>,
    activity_id: [u8; 32],
    idempotency_key: [u8; 32],
}

pub type NativeProgramDeployRequest = NativeProgramLifecycleRequest;
pub type NativeProgramUpgradeRequest = NativeProgramLifecycleRequest;
pub type NativeProgramWindDownRequest = NativeProgramLifecycleRequest;

impl NativeProgramLifecycleRequest {
    /// # Errors
    /// Refuses invalid code, payload framing, scope, or signed activity binding.
    pub fn deploy(
        registry: &ModuleRegistry,
        value: NativeProgramDeploy<'_>,
        signed: &[u8],
    ) -> Result<Self, ProgramOperationError> {
        let digest: [u8; 32] = Sha256::digest(value.wasm).into();
        if digest != value.new_hash {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        Self::bind(registry, 1, &deploy_calldata(value)?, signed)
    }

    /// # Errors
    /// Refuses invalid code, flags, payload framing, scope, or activity binding.
    pub fn upgrade(
        registry: &ModuleRegistry,
        value: NativeProgramUpgrade<'_>,
        signed: &[u8],
    ) -> Result<Self, ProgramOperationError> {
        let (transaction, _) = decode_signed(signed).map_err(|_| ProgramOperationError::Decode)?;
        let fee = transaction
            .data
            .get(4 + 8 * 32..4 + 9 * 32)
            .ok_or(ProgramOperationError::Decode)?;
        if fee[..16] != [0; 16] {
            return Err(ProgramOperationError::Decode);
        }
        let fee = u128::from_be_bytes(
            fee[16..]
                .try_into()
                .map_err(|_| ProgramOperationError::Decode)?,
        );
        Self::upgrade_with_fee_limit(registry, value, fee, signed)
    }

    /// # Errors
    /// Refuses any mismatch with the caller's explicit upgrade fee ceiling.
    pub fn upgrade_with_fee_limit(
        registry: &ModuleRegistry,
        value: NativeProgramUpgrade<'_>,
        fee_limit: u128,
        signed: &[u8],
    ) -> Result<Self, ProgramOperationError> {
        Self::bind(registry, 2, &upgrade_calldata(value, fee_limit)?, signed)
    }

    /// # Errors
    /// Refuses invalid operations, payload bounds, scope, or activity binding.
    pub fn wind_down(
        registry: &ModuleRegistry,
        value: NativeProgramWindDown<'_>,
        signed: &[u8],
    ) -> Result<Self, ProgramOperationError> {
        Self::bind(registry, 7, &wind_down_calldata(value)?, signed)
    }

    fn bind(
        _registry: &ModuleRegistry,
        ordinal: u16,
        payload: &[u8],
        signed: &[u8],
    ) -> Result<Self, ProgramOperationError> {
        if signed.is_empty() || signed.len() > MAX_SIGNED_ACTIVITY_BYTES {
            return Err(ProgramOperationError::Bounds);
        }
        let (transaction, signature) =
            decode_signed(signed).map_err(|_| ProgramOperationError::Decode)?;
        recover_sender(&transaction, &signature).map_err(|_| ProgramOperationError::Decode)?;
        if transaction.chain_id != 125
            || transaction.to != PROGRAMS_PRECOMPILE
            || transaction.data != payload
        {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        let selector = payload.get(..4).ok_or(ProgramOperationError::Decode)?;
        let valid = match ordinal {
            1 => selector == [0xbd, 0xe3, 0xff, 0x66],
            2 => selector == [0xa8, 0x60, 0x92, 0x9f],
            7 => [
                [0x23, 0x33, 0x0d, 0xd5],
                [0x86, 0xba, 0x1a, 0x99],
                [0xa5, 0x12, 0xf1, 0xa9],
                [0x4d, 0xc5, 0x07, 0x30],
                [0xad, 0xe5, 0xd9, 0xe6],
            ]
            .iter()
            .any(|expected| selector == expected),
            _ => false,
        };
        if !valid {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        let bound_id = transaction_hash(signed);
        Ok(Self {
            ordinal,
            signed_activity: signed.to_vec(),
            activity_id: bound_id,
            idempotency_key: lifecycle_idempotency(bound_id, &transaction.data)?,
        })
    }

    #[must_use]
    pub const fn ordinal(&self) -> u16 {
        self.ordinal
    }
    #[must_use]
    pub fn signed_activity(&self) -> &[u8] {
        &self.signed_activity
    }
    #[must_use]
    pub const fn bound_activity_id(&self) -> [u8; 32] {
        self.activity_id
    }
    #[must_use]
    pub const fn bound_idempotency_key(&self) -> [u8; 32] {
        self.idempotency_key
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProgramLifecycleSubmission {
    Submitted {
        activity_id: [u8; 32],
        idempotency_key: [u8; 32],
        retained_signed_activity: Vec<u8>,
    },
    Acknowledged {
        activity_id: [u8; 32],
        receipt: Vec<u8>,
        result_code: i32,
    },
    Unknown {
        activity_id: [u8; 32],
        idempotency_key: [u8; 32],
        retained_signed_activity: Vec<u8>,
    },
}

pub(crate) fn lifecycle_idempotency(
    hash: [u8; 32],
    data: &[u8],
) -> Result<[u8; 32], ProgramOperationError> {
    let selector = data.get(..4).ok_or(ProgramOperationError::Decode)?;
    Ok(Sha256::new()
        .chain_update(b"layerxprograms/idempotency/v1")
        .chain_update(hash)
        .chain_update(0_u32.to_be_bytes())
        .chain_update(selector)
        .finalize()
        .into())
}

/// Encodes the live Programs ABI from the exact typed lifecycle payload.
/// # Errors
///
/// Returns `Decode` for an unsupported ordinal or malformed lifecycle payload, and preserves the typed lifecycle encoder refusal.
pub fn lifecycle_calldata(ordinal: u16, payload: &[u8]) -> Result<Vec<u8>, ProgramOperationError> {
    lifecycle_calldata_with_fee(ordinal, payload, 0)
}

/// Encodes a lifecycle call with the upgrade method's explicit execution fee budget.
/// Deploy and wind-down methods carry no execution fee argument and refuse a nonzero one.
/// # Errors
///
/// Returns `Decode` for unsupported ordinals, malformed payloads or a nonzero fee on deploy or wind-down; preserves hash and field validation errors.
pub fn lifecycle_calldata_with_fee(
    ordinal: u16,
    payload: &[u8],
    execution_fee: u128,
) -> Result<Vec<u8>, ProgramOperationError> {
    if ordinal != 2 && execution_fee != 0 {
        return Err(ProgramOperationError::IdentityMismatch);
    }
    match ordinal {
        1 => deploy_calldata(
            NativeProgramDeploy::decode(payload).map_err(|_| ProgramOperationError::Decode)?,
        ),
        2 => upgrade_calldata(
            NativeProgramUpgrade::decode(payload).map_err(|_| ProgramOperationError::Decode)?,
            execution_fee,
        ),
        7 => wind_down_calldata(
            NativeProgramWindDown::decode(payload).map_err(|_| ProgramOperationError::Decode)?,
        ),
        _ => Err(ProgramOperationError::IdentityMismatch),
    }
}

/// # Errors
/// Refuses invalid signatures, noncanonical receipts, or mismatched lifecycle scope.
pub fn verify_lifecycle_receipt<'a>(
    receipt: &[u8],
    expected_activity: [u8; 32],
    receipt_keys: impl Into<layerx_proof::receipt::ReceiptKeys<'a>>,
) -> Result<layerx_wire::receipt::Receipt, ProgramOperationError> {
    let keys = receipt_keys.into();
    if !matches!(keys, layerx_proof::receipt::ReceiptKeys::HeightRanges(_)) {
        return Err(ProgramOperationError::Verification);
    }
    let verified = layerx_proof::receipt::verify_sequencer_signature(receipt, keys)
        .map_err(|_| ProgramOperationError::Verification)?;
    let protocol = verified
        .protocol()
        .ok_or(ProgramOperationError::Verification)?;
    let mut to = [0; 32];
    to[12..].copy_from_slice(&PROGRAMS_PRECOMPILE);
    if protocol.activity_id() != expected_activity
        || protocol.to() != to
        || protocol.module_id() != 12
        || protocol.operation() != 2
        || !matches!(protocol.result_code(), 0 | 1)
        || protocol.parameter_version() != 0
    {
        return Err(ProgramOperationError::Verification);
    }
    layerx_proof::sequencer_logs::decode_summary(&verified)
        .map_err(|_| ProgramOperationError::Verification)?;
    Ok(verified)
}

pub(crate) fn lifecycle_event_topic(data: &[u8]) -> Result<&'static str, ProgramOperationError> {
    Ok(match data.get(..4) {
        Some([0xbd, 0xe3, 0xff, 0x66]) => {
            "2765c1020c93cab47e389edaa5b5c68834bdd76f7a5d384adb460fc86e688709"
        }
        Some([0xa8, 0x60, 0x92, 0x9f]) => {
            "36b27ad7ada15e27c12e726d56c3dbd2aaa084a9250e5282c7cd38eb78a1a8ab"
        }
        Some([0x23, 0x33, 0x0d, 0xd5]) => {
            "6b9aceca11d63f8d54de57f0d38646082c9375dc119465a34e7099ec6f92ac57"
        }
        Some([0x86, 0xba, 0x1a, 0x99]) => {
            "769e024ff1115e1c26a069148f332c81f2578f4ea67dde7d5076a9e9fd5e1eb8"
        }
        Some([0xa5, 0x12, 0xf1, 0xa9]) => {
            "47fc53b89ce902506c8f3a0424a87f764a54dcb5ccb9c985639991a3a55b410c"
        }
        Some([0x4d, 0xc5, 0x07, 0x30] | [0xad, 0xe5, 0xd9, 0xe6]) => {
            "f07e690dd4cedc62197bec125b1418cfee68758e9c183deac3b50902e52f59ca"
        }
        _ => return Err(ProgramOperationError::Verification),
    })
}

pub(crate) fn verify_lifecycle_outcome(
    verified: &layerx_client::rpc::receipt::AuthenticatedReceipt,
    request: &NativeProgramLifecycleRequest,
) -> Result<(), ProgramOperationError> {
    let (transaction, signature) =
        decode_signed(request.signed_activity()).map_err(|_| ProgramOperationError::Decode)?;
    let sender =
        recover_sender(&transaction, &signature).map_err(|_| ProgramOperationError::Decode)?;
    let execution = verified.execution();
    if execution.from != sender
        || execution.to != PROGRAMS_PRECOMPILE
        || execution.nonce != transaction.nonce
    {
        return Err(ProgramOperationError::Verification);
    }
    if execution.success {
        let event = lifecycle_event_topic(&transaction.data)?;
        if !execution.logs.iter().any(|log| {
            log.address == PROGRAMS_PRECOMPILE
                && log.topics.len() == 2
                && crate::rpc::encode_hex(&log.topics[0]) == event
                && transaction.data.get(4..36) == Some(log.topics[1].as_slice())
        }) {
            return Err(ProgramOperationError::Verification);
        }
    }
    Ok(())
}

/// # Errors
/// Refuses an invalid Programs activity registration.
pub fn programs_module_registry() -> Result<ModuleRegistry, ProgramOperationError> {
    let activities = [1, 2, 3, 7].map(|ordinal| ActivityType::new(ModuleId::Programs, ordinal));
    let activities = activities
        .into_iter()
        .collect::<Result<Vec<_>, _>>()
        .map_err(|_| ProgramOperationError::Decode)?;
    let registration = ModuleRegistration::new(ModuleId::Programs, &activities)
        .map_err(|_| ProgramOperationError::Decode)?;
    ModuleRegistry::new(&[registration]).map_err(|_| ProgramOperationError::Decode)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn bytes(value: &serde_json::Value, name: &str) -> Result<Vec<u8>, String> {
        let text = value[name].as_str().ok_or_else(|| name.to_owned())?;
        if text.len() % 2 != 0 {
            return Err(name.to_owned());
        }
        text.as_bytes()
            .chunks_exact(2)
            .map(|pair| {
                let pair = std::str::from_utf8(pair).map_err(|error| error.to_string())?;
                u8::from_str_radix(pair, 16).map_err(|error| error.to_string())
            })
            .collect()
    }

    fn assert_lifecycle_rejections(
        registry: &ModuleRegistry,
        ordinal: u16,
        calldata: &[u8],
        signed: &[u8],
        transaction: &layerx_crypto::evm_transaction::Eip1559Call,
    ) -> Result<(), String> {
        for length in 0..signed.len() {
            assert!(NativeProgramLifecycleRequest::bind(
                registry,
                ordinal,
                calldata,
                &signed[..length]
            )
            .is_err());
        }
        let mut wrong_target = transaction.clone();
        wrong_target.to[19] ^= 1;
        let wrong_target = wrong_target
            .sign(&[1; 32])
            .map_err(|error| error.to_string())?;
        assert!(
            NativeProgramLifecycleRequest::bind(registry, ordinal, calldata, &wrong_target)
                .is_err()
        );
        let mut wrong_data = transaction.clone();
        wrong_data.data[0] ^= 1;
        let wrong_data = wrong_data
            .sign(&[1; 32])
            .map_err(|error| error.to_string())?;
        assert!(decode_signed(&wrong_data).is_ok());
        assert!(
            NativeProgramLifecycleRequest::bind(registry, ordinal, calldata, &wrong_data).is_err()
        );
        let mut wrong_protocol = signed.to_vec();
        wrong_protocol[0] = 1;
        assert!(
            NativeProgramLifecycleRequest::bind(registry, ordinal, calldata, &wrong_protocol)
                .is_err()
        );
        let mut altered = calldata.to_vec();
        altered[0] ^= 1;
        assert!(NativeProgramLifecycleRequest::bind(registry, ordinal, &altered, signed).is_err());
        assert!(NativeProgramLifecycleRequest::bind(
            registry,
            if ordinal == 1 { 2 } else { 1 },
            calldata,
            signed
        )
        .is_err());
        Ok(())
    }

    #[test]
    fn c_signed_lifecycle_fixtures_bind_exactly() -> Result<(), String> {
        let registry = programs_module_registry().map_err(|error| format!("{error:?}"))?;
        for (name, ordinal) in [
            ("deploy", 1),
            ("upgrade", 2),
            ("wind-down-route", 7),
            ("wind-down-deprecate", 7),
            ("wind-down-tombstone", 7),
            ("wind-down-exit", 7),
        ] {
            let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../../../sdk/platform/conformance/fixtures")
                .join(format!("native-program-{name}-v4.json"));
            let fixture: serde_json::Value =
                serde_json::from_slice(&std::fs::read(path).map_err(|error| error.to_string())?)
                    .map_err(|error| error.to_string())?;
            let payload = bytes(&fixture, "payload_hex")?;
            let calldata_text = fixture["evm"]["calldata"].as_str().ok_or("calldata")?;
            let calldata_value =
                serde_json::json!({"calldata": calldata_text.strip_prefix("0x").ok_or("prefix")?});
            let calldata = bytes(&calldata_value, "calldata")?;
            let transaction = layerx_crypto::evm_transaction::Eip1559Call {
                chain_id: 125,
                nonce: 7,
                max_priority_fee_per_gas: 2,
                max_fee_per_gas: 3,
                gas_limit: 30_000,
                to: PROGRAMS_PRECOMPILE,
                data: calldata.clone(),
            };
            let signed = transaction
                .sign(&[1; 32])
                .map_err(|error| error.to_string())?;
            let request = match ordinal {
                1 => NativeProgramLifecycleRequest::deploy(
                    &registry,
                    NativeProgramDeploy::decode(&payload).map_err(|error| format!("{error:?}"))?,
                    &signed,
                ),
                2 => NativeProgramLifecycleRequest::upgrade(
                    &registry,
                    NativeProgramUpgrade::decode(&payload).map_err(|error| format!("{error:?}"))?,
                    &signed,
                ),
                _ => NativeProgramLifecycleRequest::wind_down(
                    &registry,
                    NativeProgramWindDown::decode(&payload)
                        .map_err(|error| format!("{error:?}"))?,
                    &signed,
                ),
            }
            .map_err(|error| format!("{error:?}"))?;
            assert_eq!(request.ordinal(), ordinal);
            assert_eq!(request.bound_activity_id(), transaction_hash(&signed));
            assert_eq!(
                request.bound_idempotency_key(),
                lifecycle_idempotency(transaction_hash(&signed), &transaction.data)
                    .map_err(|error| format!("{error:?}"))?
            );
            assert_eq!(request.signed_activity(), signed);
            let (decoded, signature) = decode_signed(&signed).map_err(|error| error.to_string())?;
            assert_eq!(decoded, transaction);
            assert_eq!(
                recover_sender(&decoded, &signature).map_err(|error| error.to_string())?,
                [
                    0x1a, 0x64, 0x2f, 0x0e, 0x3c, 0x3a, 0xf5, 0x45, 0xe7, 0xac, 0xbd, 0x38, 0xb0,
                    0x72, 0x51, 0xb3, 0x99, 0x09, 0x14, 0xf1,
                ]
            );
            assert_lifecycle_rejections(&registry, ordinal, &calldata, &signed, &transaction)?;
        }
        Ok(())
    }
}
