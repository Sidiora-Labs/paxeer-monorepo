use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::program_lifecycle::{
    NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown,
};
use sha2::{Digest as _, Sha256};

use crate::programs::{ProgramOperationError, MAX_SIGNED_ACTIVITY_BYTES, PROGRAMS_PRECOMPILE};

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
        Self::bind(
            registry,
            1,
            &value.encode().map_err(|_| ProgramOperationError::Decode)?,
            signed,
        )
    }

    /// # Errors
    /// Refuses invalid code, flags, payload framing, scope, or activity binding.
    pub fn upgrade(
        registry: &ModuleRegistry,
        value: NativeProgramUpgrade<'_>,
        signed: &[u8],
    ) -> Result<Self, ProgramOperationError> {
        let digest: [u8; 32] = Sha256::digest(value.wasm).into();
        if digest != value.new_hash {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        Self::bind(
            registry,
            2,
            &value.encode().map_err(|_| ProgramOperationError::Decode)?,
            signed,
        )
    }

    /// # Errors
    /// Refuses invalid operations, payload bounds, scope, or activity binding.
    pub fn wind_down(
        registry: &ModuleRegistry,
        value: NativeProgramWindDown<'_>,
        signed: &[u8],
    ) -> Result<Self, ProgramOperationError> {
        Self::bind(
            registry,
            7,
            &value.encode().map_err(|_| ProgramOperationError::Decode)?,
            signed,
        )
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
        if transaction.to != PROGRAMS_PRECOMPILE || transaction.data != payload {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        match ordinal {
            1 => {
                NativeProgramDeploy::decode(payload).map_err(|_| ProgramOperationError::Decode)?;
            }
            2 => {
                NativeProgramUpgrade::decode(payload).map_err(|_| ProgramOperationError::Decode)?;
            }
            7 => {
                NativeProgramWindDown::decode(payload)
                    .map_err(|_| ProgramOperationError::Decode)?;
            }
            _ => return Err(ProgramOperationError::IdentityMismatch),
        }
        let bound_id = transaction_hash(signed);
        Ok(Self {
            ordinal,
            signed_activity: signed.to_vec(),
            activity_id: bound_id,
            idempotency_key: bound_id,
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

/// # Errors
/// Refuses invalid signatures, noncanonical receipts, or mismatched lifecycle scope.
pub fn verify_lifecycle_receipt(
    receipt: &[u8],
    expected_activity: [u8; 32],
    sequencer_public_key: [u8; 32],
) -> Result<layerx_wire::receipt::Receipt, ProgramOperationError> {
    let verified = layerx_proof::receipt::verify_sequencer_signature(receipt, sequencer_public_key)
        .map_err(|_| ProgramOperationError::Verification)?;
    let protocol = verified
        .protocol()
        .ok_or(ProgramOperationError::Verification)?;
    if protocol.protocol_version() != 3
        || protocol.module_id() != 9
        || protocol.module_version() != 4
        || protocol.operation() != 0
        || protocol.program_outcome().is_some()
        || protocol.activity_id() != expected_activity
    {
        return Err(ProgramOperationError::Verification);
    }
    Ok(verified)
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

    #[test]
    fn call_receipt_is_not_lifecycle_evidence() -> Result<(), String> {
        let fixture: serde_json::Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/receipt-programs-positive-v3.json"
        ))
        .map_err(|error| error.to_string())?;
        let receipt = bytes(&fixture, "canonical_receipt_hex")?;
        let key = bytes(&fixture["authorized_batch"], "sequencer_public_key_hex")?
            .try_into()
            .map_err(|_| "key length")?;
        assert!(verify_lifecycle_receipt(&receipt, [0x41; 32], key).is_err());
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
                .join(format!("native-program-{name}-v3.json"));
            let fixture: serde_json::Value =
                serde_json::from_slice(&std::fs::read(path).map_err(|error| error.to_string())?)
                    .map_err(|error| error.to_string())?;
            let payload = bytes(&fixture, "payload_hex")?;
            let transaction = layerx_crypto::evm_transaction::Eip1559Call {
                chain_id: 1,
                nonce: 7,
                max_priority_fee_per_gas: 2,
                max_fee_per_gas: 3,
                gas_limit: 30_000,
                to: PROGRAMS_PRECOMPILE,
                data: payload.clone(),
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
            assert_eq!(request.bound_idempotency_key(), transaction_hash(&signed));
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
            for length in 0..signed.len() {
                assert!(NativeProgramLifecycleRequest::bind(
                    &registry,
                    ordinal,
                    &payload,
                    &signed[..length]
                )
                .is_err());
            }
            let mut wrong_target = transaction.clone();
            wrong_target.to[19] ^= 1;
            let wrong_target = wrong_target
                .sign(&[1; 32])
                .map_err(|error| error.to_string())?;
            assert!(NativeProgramLifecycleRequest::bind(
                &registry,
                ordinal,
                &payload,
                &wrong_target
            )
            .is_err());
            let mut wrong_data = transaction.clone();
            wrong_data.data[0] ^= 1;
            let wrong_data = wrong_data
                .sign(&[1; 32])
                .map_err(|error| error.to_string())?;
            assert!(decode_signed(&wrong_data).is_ok());
            assert!(
                NativeProgramLifecycleRequest::bind(&registry, ordinal, &payload, &wrong_data)
                    .is_err()
            );
            let mut wrong_protocol = signed.clone();
            wrong_protocol[0] = 1;
            assert!(NativeProgramLifecycleRequest::bind(
                &registry,
                ordinal,
                &payload,
                &wrong_protocol
            )
            .is_err());
            let mut altered = payload.clone();
            altered[0] ^= 1;
            assert!(
                NativeProgramLifecycleRequest::bind(&registry, ordinal, &altered, &signed).is_err()
            );
            assert!(NativeProgramLifecycleRequest::bind(
                &registry,
                if ordinal == 1 { 2 } else { 1 },
                &payload,
                &signed
            )
            .is_err());
        }
        Ok(())
    }
}
