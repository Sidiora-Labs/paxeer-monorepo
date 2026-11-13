//! Deployment transaction, execution-result and predecessor authentication.

use super::{
    bundle_helpers, chain_error, decode_program_record_value, put_chain_field, put_chain_ops,
    put_chain_option, take_array, take_bytes, take_chain_ops, take_chain_option, ChainHeader,
    ChainProofVerifier, CommitmentProof, ProgramStateProof, ProofOp, ProtocolEvidenceError,
    SimpleProof, UpgradePolicy, VerifiedHeader, MAX_EVIDENCE_BYTES, PROGRAM_KEY_PREFIX,
};
use crate::chain_proof::{
    proto::{ProtoReader, ProtoValue},
    results::TxResult,
};
use std::collections::{BTreeMap, BTreeSet};

const DOMAIN: &[u8] = b"LXTX\x03";
const EVM_MESSAGE: &[u8] = b"/paxprotocol.paxchain.evm.MsgEVMTransaction";

/// Versioned transport carried in `ProgramStateProof::transaction` for deployment
/// admissions. The transaction proof still addresses the enclosed raw transaction.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DeploymentExecutionProof {
    pub transaction: Vec<u8>,
    /// Header H and the authenticated prior program record (or its absence).
    pub previous: CommitmentProof,
    pub result: TxResult,
    pub result_proof: SimpleProof,
    pub interface_ops: Vec<ProofOp>,
    /// A hooked upgrade's execution evidence, committed in the host store.
    pub migration: Option<crate::MigrationExecutionEvidence>,
    pub migration_ops: Vec<ProofOp>,
    pub migration_previous_ops: Vec<ProofOp>,
}

impl DeploymentExecutionProof {
    ///
    /// # Errors
    ///
    /// Returns an error when the deployment input cannot be validated or encoded.
    pub fn canonical_encoding(&self) -> Result<Vec<u8>, ProtocolEvidenceError> {
        let mut bytes = DOMAIN.to_vec();
        put_chain_field(&mut bytes, &self.transaction)?;
        put_chain_field(&mut bytes, &self.previous.encode().map_err(chain_error)?)?;
        bytes.extend_from_slice(&self.result.code.to_be_bytes());
        put_chain_field(&mut bytes, &self.result.data)?;
        bytes.extend_from_slice(&self.result.gas_wanted.to_be_bytes());
        bytes.extend_from_slice(&self.result.gas_used.to_be_bytes());
        let proof = &self.result_proof;
        if proof.aunts.len() > 64 {
            return Err(ProtocolEvidenceError::Encoding);
        }
        bytes.extend_from_slice(&proof.total.to_be_bytes());
        bytes.extend_from_slice(&proof.index.to_be_bytes());
        bytes.extend_from_slice(&proof.leaf_hash);
        bytes.push(u8::try_from(proof.aunts.len()).map_err(|_| ProtocolEvidenceError::Encoding)?);
        for aunt in &proof.aunts {
            bytes.extend_from_slice(aunt);
        }
        put_chain_ops(&mut bytes, &self.interface_ops)?;
        put_chain_option(
            &mut bytes,
            self.migration
                .as_ref()
                .map(crate::MigrationExecutionEvidence::canonical_bytes),
        )?;
        put_chain_ops(&mut bytes, &self.migration_ops)?;
        put_chain_ops(&mut bytes, &self.migration_previous_ops)?;
        if bytes.len() > MAX_EVIDENCE_BYTES {
            return Err(ProtocolEvidenceError::Encoding);
        }
        Ok(bytes)
    }

    ///
    /// # Errors
    ///
    /// Returns an error when the deployment input cannot be validated or encoded.
    pub fn decode(bytes: &[u8]) -> Result<Self, ProtocolEvidenceError> {
        if bytes.len() > MAX_EVIDENCE_BYTES || !bytes.starts_with(DOMAIN) {
            return Err(ProtocolEvidenceError::Encoding);
        }
        let mut cursor = DOMAIN.len();
        let transaction = take_bytes(bytes, &mut cursor, MAX_EVIDENCE_BYTES)?;
        let previous = crate::chain_proof::decode::decode_commitment_proof(&take_bytes(
            bytes,
            &mut cursor,
            MAX_EVIDENCE_BYTES,
        )?)
        .map_err(chain_error)?;
        let code = u32::from_be_bytes(take_array(bytes, &mut cursor)?);
        let data = take_bytes(bytes, &mut cursor, MAX_EVIDENCE_BYTES)?;
        let gas_wanted = i64::from_be_bytes(take_array(bytes, &mut cursor)?);
        let gas_used = i64::from_be_bytes(take_array(bytes, &mut cursor)?);
        let total = i64::from_be_bytes(take_array(bytes, &mut cursor)?);
        let index = i64::from_be_bytes(take_array(bytes, &mut cursor)?);
        let leaf_hash = take_array(bytes, &mut cursor)?;
        let count = take_array::<1>(bytes, &mut cursor)?[0];
        if count > 64 {
            return Err(ProtocolEvidenceError::Encoding);
        }
        let mut aunts = Vec::with_capacity(count as usize);
        for _ in 0..count {
            aunts.push(take_array(bytes, &mut cursor)?);
        }
        let interface_ops = take_chain_ops(bytes, &mut cursor)?;
        let migration = take_chain_option(bytes, &mut cursor)?
            .map(|bytes| {
                crate::MigrationExecutionEvidence::decode_canonical(&bytes)
                    .map_err(|_| ProtocolEvidenceError::Encoding)
            })
            .transpose()?;
        let migration_ops = take_chain_ops(bytes, &mut cursor)?;
        let migration_previous_ops = take_chain_ops(bytes, &mut cursor)?;
        let proof = Self {
            transaction,
            previous,
            result: TxResult {
                code,
                data,
                gas_wanted,
                gas_used,
            },
            result_proof: SimpleProof {
                total,
                index,
                leaf_hash,
                aunts,
            },
            interface_ops,
            migration,
            migration_ops,
            migration_previous_ops,
        };
        if cursor != bytes.len() || proof.canonical_encoding()? != bytes {
            return Err(ProtocolEvidenceError::Encoding);
        }
        Ok(proof)
    }

    pub(super) fn verify(
        &self,
        verifier: &ChainProofVerifier,
        state: &ProgramStateProof,
        next: &VerifiedHeader,
        deployment: &crate::chain_evidence::deployment::VerifiedDeploymentEvidence,
    ) -> Result<([u8; 32], Option<[u8; 32]>), ProtocolEvidenceError> {
        let (header, commit, previous) = bundle_helpers::commitment_head(verifier, &self.previous)?;
        let next_header =
            ChainHeader::decode(&state.commitment.signed_header).map_err(chain_error)?;
        if header.height.checked_add(1) != Some(next.height())
            || next_header.chain_id != header.chain_id
            || next_header.last_block_id.hash != previous.header_hash()
            || next_header.last_block_id.part_set_total != commit.block_id.part_set_header.total
            || next_header.last_block_id.part_set_hash != commit.block_id.part_set_header.hash
            || header.next_validators_hash != next_header.validators_hash
            || next.time_ms() <= previous.time_ms()
            || self.result_proof.index != state.transaction_proof.index
            || self.result_proof.total != state.transaction_proof.total
        {
            return Err(ProtocolEvidenceError::DeploymentMismatch);
        }
        let identity = crate::verify_tx(&previous, &self.transaction, &state.transaction_proof)
            .map_err(chain_error)?;
        crate::verify_tx_result(next, &self.result, &self.result_proof).map_err(chain_error)?;
        if self.result.code != 0 {
            return Err(ProtocolEvidenceError::Receipt);
        }
        successful_evm_result(&self.result.data)?;
        let call = deployment_call(&self.transaction)?;
        let mut key = PROGRAM_KEY_PREFIX.to_vec();
        key.extend_from_slice(&deployment.program().bytes());
        if self.previous.key != crate::chain_evidence::programs_state_key(&key) {
            return Err(ProtocolEvidenceError::DeploymentMismatch);
        }
        let interface_key = crate::chain_evidence::programs_state_key(&crate::interface_state_key(
            deployment.program(),
        ));
        match deployment.interface() {
            Some(interface) => {
                let value = crate::interface_state_value(
                    deployment.program(),
                    deployment.version(),
                    interface,
                )
                .map_err(|_| ProtocolEvidenceError::DeploymentMismatch)?;
                crate::verify_state(
                    next,
                    crate::chain_evidence::HOST_STORE_NAME,
                    &interface_key,
                    &value,
                    &self.interface_ops,
                )
                .map_err(chain_error)?;
            }
            None => {
                crate::verify_absent(
                    next,
                    crate::chain_evidence::HOST_STORE_NAME,
                    &interface_key,
                    &self.interface_ops,
                )
                .map_err(chain_error)?;
            }
        }
        let prior = match &self.previous.value {
            None => None,
            Some(value) => Some(decode_program_record_value(&key, value)?),
        };
        let old_hash = prior.map(|record| record.code_hash);
        if call.program != deployment.program().bytes()
            || call.abi != deployment.abi_version()
            || call.code != deployment.module()
            || crate::hash::sha256(call.code) != deployment.code_hash()
            || call.interface
                != deployment
                    .interface()
                    .map(crate::ProgramInterface::canonical_encoding)
            || call.old_hash != old_hash
        {
            return Err(ProtocolEvidenceError::DeploymentMismatch);
        }
        match prior {
            None if deployment.version() == 1
                && !call.upgrade
                && call.policy == Some(deployment.upgrade_policy()) => {}
            Some(prior)
                if call.upgrade
                    && prior.version.checked_add(1) == Some(deployment.version())
                    && prior.policy == deployment.upgrade_policy()
                    && matches!(prior.policy, UpgradePolicy::Authority(_)) =>
            {
                layerx_programs_runtime::admit_abi_upgrade(prior.abi_version, call.abi)
                    .map_err(|_| ProtocolEvidenceError::DeploymentMismatch)?;
            }
            _ => return Err(ProtocolEvidenceError::DeploymentMismatch),
        }
        if call.has_migration != self.migration.is_some() {
            return Err(ProtocolEvidenceError::Receipt);
        }
        self.verify_migration(deployment, old_hash, next, &previous)?;
        Ok((identity, old_hash))
    }

    fn verify_migration(
        &self,
        deployment: &crate::chain_evidence::deployment::VerifiedDeploymentEvidence,
        old_hash: Option<[u8; 32]>,
        next: &VerifiedHeader,
        previous: &VerifiedHeader,
    ) -> Result<(), ProtocolEvidenceError> {
        if let Some(migration) = &self.migration {
            if migration.abi_version() != deployment.abi_version() {
                return Err(ProtocolEvidenceError::Receipt);
            }
            let accounting = migration
                .chain_accounting()
                .ok_or(ProtocolEvidenceError::Receipt)?;
            if accounting[93..125] != deployment.program().bytes()
                || Some(
                    <[u8; 32]>::try_from(&accounting[125..157])
                        .map_err(|_| ProtocolEvidenceError::Receipt)?,
                ) != old_hash
                || accounting[157..189] != deployment.code_hash()
            {
                return Err(ProtocolEvidenceError::Receipt);
            }
            let mut key = b"progmig/account/v1/".to_vec();
            key.extend_from_slice(&accounting[61..93]);
            let key = crate::chain_evidence::programs_state_key(&key);
            crate::verify_state(
                next,
                crate::chain_evidence::HOST_STORE_NAME,
                &key,
                accounting,
                &self.migration_ops,
            )
            .map_err(chain_error)?;
            crate::verify_absent(
                previous,
                crate::chain_evidence::HOST_STORE_NAME,
                &key,
                &self.migration_previous_ops,
            )
            .map_err(chain_error)?;
        } else if !self.migration_ops.is_empty() || !self.migration_previous_ops.is_empty() {
            return Err(ProtocolEvidenceError::Encoding);
        }
        Ok(())
    }
}

struct DeploymentCall<'a> {
    program: [u8; 32],
    abi: u16,
    policy: Option<UpgradePolicy>,
    old_hash: Option<[u8; 32]>,
    code: &'a [u8],
    interface: Option<&'a [u8]>,
    upgrade: bool,
    has_migration: bool,
}

fn fields(bytes: &[u8]) -> Result<BTreeMap<u32, ProtoValue<'_>>, ProtocolEvidenceError> {
    let mut reader = ProtoReader::new(bytes);
    let mut fields = BTreeMap::new();
    while let Some(field) = reader.next_field().map_err(chain_error)? {
        if fields.insert(field.number, field.value).is_some() {
            return Err(ProtocolEvidenceError::Encoding);
        }
    }
    Ok(fields)
}
fn byte_field<'a>(
    fields: &BTreeMap<u32, ProtoValue<'a>>,
    number: u32,
) -> Result<&'a [u8], ProtocolEvidenceError> {
    match fields.get(&number) {
        Some(ProtoValue::Bytes(value)) => Ok(value),
        _ => Err(ProtocolEvidenceError::Encoding),
    }
}
fn any(bytes: &[u8]) -> Result<(&[u8], &[u8]), ProtocolEvidenceError> {
    let fields = fields(bytes)?;
    if fields.len() != 2 {
        return Err(ProtocolEvidenceError::Encoding);
    }
    Ok((byte_field(&fields, 1)?, byte_field(&fields, 2)?))
}
fn successful_evm_result(bytes: &[u8]) -> Result<(), ProtocolEvidenceError> {
    let data = fields(bytes)?;
    if data.len() != 1 {
        return Err(ProtocolEvidenceError::Receipt);
    }
    let message = fields(byte_field(&data, 1)?)?;
    if byte_field(&message, 1)? != EVM_MESSAGE {
        return Err(ProtocolEvidenceError::Receipt);
    }
    let mut response = ProtoReader::new(byte_field(&message, 2)?);
    let mut seen = BTreeSet::new();
    while let Some(field) = response.next_field().map_err(chain_error)? {
        if !(1..=5).contains(&field.number)
            || (field.number != 5 && !seen.insert(field.number))
            || (field.number == 2
                && !matches!(field.value, ProtoValue::Bytes(bytes) if bytes.is_empty()))
        {
            return Err(ProtocolEvidenceError::Receipt);
        }
    }
    Ok(())
}
fn deployment_call(transaction: &[u8]) -> Result<DeploymentCall<'_>, ProtocolEvidenceError> {
    let tx = fields(transaction)?;
    if tx.keys().any(|key| !(1..=3).contains(key)) {
        return Err(ProtocolEvidenceError::UnsupportedActivity);
    }
    let body = fields(byte_field(&tx, 1)?)?;
    if body.keys().any(|key| ![1, 2, 3].contains(key)) {
        return Err(ProtocolEvidenceError::UnsupportedActivity);
    }
    let (kind, message) = any(byte_field(&body, 1)?)?;
    if kind != EVM_MESSAGE {
        return Err(ProtocolEvidenceError::UnsupportedActivity);
    }
    let message = fields(message)?;
    if message.keys().any(|key| ![1, 2].contains(key)) {
        return Err(ProtocolEvidenceError::UnsupportedActivity);
    }
    let (kind, tx_data) = any(byte_field(&message, 1)?)?;
    let (to, data) = match kind {
        b"/paxprotocol.paxchain.eth.LegacyTx" => (4, 6),
        b"/paxprotocol.paxchain.eth.AccessListTx" => (5, 7),
        b"/paxprotocol.paxchain.eth.DynamicFeeTx" => (6, 8),
        _ => return Err(ProtocolEvidenceError::UnsupportedActivity),
    };
    let tx_data = fields(tx_data)?;
    let destination = core::str::from_utf8(byte_field(&tx_data, to)?)
        .map_err(|_| ProtocolEvidenceError::Encoding)?;
    let address = destination
        .strip_prefix("0x")
        .ok_or(ProtocolEvidenceError::UnsupportedActivity)?;
    let mut expected = [0u8; 20];
    expected[18..].copy_from_slice(&[0x10, 0x1d]);
    if crate::hex::decode(address).map_err(|_| ProtocolEvidenceError::Encoding)? != expected {
        return Err(ProtocolEvidenceError::UnsupportedActivity);
    }
    abi_call(byte_field(&tx_data, data)?)
}
fn word(bytes: &[u8], index: usize) -> Result<[u8; 32], ProtocolEvidenceError> {
    let start = index
        .checked_mul(32)
        .ok_or(ProtocolEvidenceError::Encoding)?;
    bytes
        .get(start..start + 32)
        .ok_or(ProtocolEvidenceError::Encoding)?
        .try_into()
        .map_err(|_| ProtocolEvidenceError::Encoding)
}
fn integer(bytes: [u8; 32]) -> Result<usize, ProtocolEvidenceError> {
    if bytes[..28] != [0; 28] {
        return Err(ProtocolEvidenceError::Encoding);
    }
    Ok(u32::from_be_bytes(
        bytes[28..]
            .try_into()
            .map_err(|_| ProtocolEvidenceError::Encoding)?,
    ) as usize)
}
fn dynamic<'a>(
    bytes: &'a [u8],
    index: usize,
    cursor: &mut usize,
) -> Result<&'a [u8], ProtocolEvidenceError> {
    if integer(word(bytes, index)?)? != *cursor {
        return Err(ProtocolEvidenceError::Encoding);
    }
    let length = integer(word(
        bytes
            .get(*cursor..)
            .ok_or(ProtocolEvidenceError::Encoding)?,
        0,
    )?)?;
    let start = cursor
        .checked_add(32)
        .ok_or(ProtocolEvidenceError::Encoding)?;
    let end = start
        .checked_add(length)
        .ok_or(ProtocolEvidenceError::Encoding)?;
    let padded = end.checked_add(31).ok_or(ProtocolEvidenceError::Encoding)? / 32 * 32;
    let value = bytes
        .get(start..end)
        .ok_or(ProtocolEvidenceError::Encoding)?;
    if bytes
        .get(end..padded)
        .ok_or(ProtocolEvidenceError::Encoding)?
        .iter()
        .any(|byte| *byte != 0)
    {
        return Err(ProtocolEvidenceError::Encoding);
    }
    *cursor = padded;
    Ok(value)
}
fn abi_call(input: &[u8]) -> Result<DeploymentCall<'_>, ProtocolEvidenceError> {
    let selector = input
        .get(..4)
        .ok_or(ProtocolEvidenceError::UnsupportedActivity)?;
    let bytes = &input[4..];
    let program = word(bytes, 0)?;
    let abi =
        u16::try_from(integer(word(bytes, 1)?)?).map_err(|_| ProtocolEvidenceError::Encoding)?;
    let flag = integer(word(bytes, 2)?)?;
    let mut cursor;
    let (policy, old_hash, interface, code, upgrade, has_migration) = match selector {
        [0xbd, 0xe3, 0xff, 0x66] => {
            let authority = word(bytes, 3)?;
            let policy = match (flag, authority == [0; 32]) {
                (0, true) => UpgradePolicy::Immutable,
                (1, false) => UpgradePolicy::Authority(authority),
                _ => return Err(ProtocolEvidenceError::DeploymentMismatch),
            };
            cursor = 7 * 32;
            let interface = dynamic(bytes, 5, &mut cursor)?;
            let code = dynamic(bytes, 6, &mut cursor)?;
            (Some(policy), None, interface, code, false, false)
        }
        [0xa8, 0x60, 0x92, 0x9f] => {
            if flag & !3 != 0 {
                return Err(ProtocolEvidenceError::Encoding);
            }
            cursor = 9 * 32;
            let hook = dynamic(bytes, 5, &mut cursor)?;
            let interface = dynamic(bytes, 6, &mut cursor)?;
            let code = dynamic(bytes, 7, &mut cursor)?;
            if (flag & 1 != 0) == hook.is_empty() || word(bytes, 8)?[..16] != [0; 16] {
                return Err(ProtocolEvidenceError::Encoding);
            }
            (
                None,
                Some(word(bytes, 3)?),
                interface,
                code,
                true,
                !hook.is_empty(),
            )
        }
        _ => return Err(ProtocolEvidenceError::UnsupportedActivity),
    };
    if cursor != bytes.len() || word(bytes, 4)? != crate::hash::sha256(code) {
        return Err(ProtocolEvidenceError::DeploymentMismatch);
    }
    Ok(DeploymentCall {
        program,
        abi,
        policy,
        old_hash,
        code,
        interface: (!interface.is_empty()).then_some(interface),
        upgrade,
        has_migration,
    })
}
