use crate::protocol_evidence::{ProgramBundleError, ProtocolEvidenceError};
use crate::{
    chain_evidence::programs_state_key,
    chain_proof::{
        decode::{decode_commitment_proof, CommitmentProof, ProofOp, MAX_FIELD_BYTES},
        header::ChainHeader,
        ics23::ExistenceProof,
        proto::{ProtoReader, ProtoValue},
        state::{verify_absent, verify_state},
        validators::{TrustedValidator, ValidatorSet},
        vote::{BlockId, CommitSig, PartSetHeader},
        ChainProofError, ChainProofVerifier, Commit, VerifiedChainHead,
    },
    interface_state_key, InterfaceStateWitness, ProgramId, StateProof, VerifiedInterfaceRead,
};

use super::program_state::{ChainProgramStateProof, VerifiedProgramRecord};
pub(crate) use super::wind_down::ChainWindDownProof;

const MAGIC: &[u8; 4] = b"LXPB";
const VERSION: u8 = 2;
const MAX_BUNDLE_BYTES: usize = 40 * 1024 * 1024;

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct ProgramStateBundle {
    pub commitment: CommitmentProof,
    pub program: ProgramId,
    pub program_record_ops: Vec<ProofOp>,
    pub interface_ops: Vec<ProofOp>,
    pub wind_down: Option<ChainWindDownProof>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct VerifiedProgramBundle {
    program: VerifiedProgramRecord,
    chain: VerifiedChainHead,
    interface: VerifiedInterfaceRead,
    wind_down: Option<Option<[u8; 54]>>,
}

impl VerifiedProgramBundle {
    pub(crate) const fn program_head(&self) -> &VerifiedProgramRecord {
        &self.program
    }

    pub(crate) const fn chain_head(&self) -> &VerifiedChainHead {
        &self.chain
    }

    pub(crate) const fn interface(&self) -> &VerifiedInterfaceRead {
        &self.interface
    }

    pub(crate) const fn wind_down(&self) -> Option<Option<[u8; 54]>> {
        self.wind_down
    }
}

impl ProgramStateBundle {
    pub(crate) fn canonical_encoding(&self) -> Result<Vec<u8>, ProgramBundleError> {
        let mut bytes = MAGIC.to_vec();
        bytes.push(VERSION);
        put_field(
            &mut bytes,
            &self.commitment.encode().map_err(encoding)?,
            MAX_BUNDLE_BYTES,
        )?;
        bytes.extend_from_slice(&self.program.bytes());
        put_ops(&mut bytes, &self.program_record_ops)?;
        put_ops(&mut bytes, &self.interface_ops)?;
        match &self.wind_down {
            None => bytes.push(0),
            Some(proof) => {
                if proof.program != self.program {
                    return Err(ProgramBundleError::ProgramMismatch);
                }
                bytes.push(1);
                match &proof.value {
                    None => bytes.push(0),
                    Some(value) => {
                        if value.len() != 54 {
                            return Err(invalid_encoding());
                        }
                        bytes.push(1);
                        put_field(&mut bytes, value, 54)?;
                    }
                }
                put_ops(&mut bytes, &proof.ops)?;
            }
        }
        if bytes.len() > MAX_BUNDLE_BYTES {
            return Err(invalid_encoding());
        }
        Ok(bytes)
    }

    pub(crate) fn decode(bytes: &[u8]) -> Result<Self, ProgramBundleError> {
        if bytes.len() > MAX_BUNDLE_BYTES {
            return Err(invalid_encoding());
        }
        let mut reader = Reader { bytes, cursor: 0 };
        if reader.take(4)? != MAGIC || reader.byte()? != VERSION {
            return Err(invalid_encoding());
        }
        let commitment =
            decode_commitment_proof(reader.field(MAX_BUNDLE_BYTES)?).map_err(encoding)?;
        let program = ProgramId::new(
            reader
                .take(32)?
                .try_into()
                .map_err(|_| invalid_encoding())?,
        )
        .map_err(|_| invalid_encoding())?;
        let program_record_ops = reader.ops()?;
        let interface_ops = reader.ops()?;
        let wind_down = match reader.byte()? {
            0 => None,
            1 => {
                let value = match reader.byte()? {
                    0 => None,
                    1 => {
                        let value = reader.field(54)?;
                        if value.len() != 54 {
                            return Err(invalid_encoding());
                        }
                        Some(value.to_vec())
                    }
                    _ => return Err(invalid_encoding()),
                };
                Some(ChainWindDownProof {
                    program,
                    value,
                    ops: reader.ops()?,
                })
            }
            _ => return Err(invalid_encoding()),
        };
        if reader.cursor != bytes.len() {
            return Err(invalid_encoding());
        }
        let bundle = Self {
            commitment,
            program,
            program_record_ops,
            interface_ops,
            wind_down,
        };
        if bundle.canonical_encoding()? != bytes {
            return Err(invalid_encoding());
        }
        Ok(bundle)
    }
}

pub(crate) fn program_bundle_at(
    verifier: &ChainProofVerifier,
    bundle: &ProgramStateBundle,
    expected_program: ProgramId,
) -> Result<VerifiedProgramBundle, ProgramBundleError> {
    if bundle.program != expected_program {
        return Err(ProgramBundleError::ProgramMismatch);
    }
    let commitment = &bundle.commitment;
    let header = ChainHeader::decode(&commitment.signed_header).map_err(encoding)?;
    let commit = decode_commit(&commitment.commit).map_err(encoding)?;
    let validators = decode_validators(&commitment.validator_set).map_err(encoding)?;
    if validators != verifier.validators {
        return Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::InvalidTrustAnchor,
        ));
    }
    let chain = verifier
        .verify_head(&header, &commit)
        .map_err(|_| ProgramBundleError::Evidence(ProtocolEvidenceError::StateRoot))?;
    if commitment.store_name != "layerx" {
        return Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::StateProof,
        ));
    }
    match &commitment.value {
        Some(value) => verify_state(
            &chain,
            &commitment.store_name,
            &commitment.key,
            value,
            &commitment.proof_ops,
        ),
        None => verify_absent(
            &chain,
            &commitment.store_name,
            &commitment.key,
            &commitment.proof_ops,
        ),
    }
    .map_err(|_| ProgramBundleError::Evidence(ProtocolEvidenceError::StateProof))?;
    let program = ChainProgramStateProof {
        program: bundle.program,
        ops: bundle.program_record_ops.clone(),
    }
    .verify(&chain)
    .map_err(ProgramBundleError::Evidence)?;
    let interface_leaf = existence(&bundle.interface_ops).map_err(encoding)?;
    let key = interface_state_key(expected_program);
    if interface_leaf.key != programs_state_key(&key) {
        return Err(ProgramBundleError::ProgramMismatch);
    }
    let mut ops = bundle.interface_ops.clone();
    ops.extend_from_slice(&bundle.program_record_ops);
    let interface = VerifiedInterfaceRead::verify_at_head(
        &chain,
        expected_program,
        &InterfaceStateWitness {
            key,
            value: interface_leaf.value,
            proof: StateProof {
                leaf_index: 0,
                leaf_count: 1,
                siblings: Vec::new(),
            },
        },
        &ops,
    )
    .map_err(ProgramBundleError::Interface)?;
    let wind_down = bundle
        .wind_down
        .as_ref()
        .map(|proof| {
            if proof.program != expected_program {
                return Err(ProgramBundleError::ProgramMismatch);
            }
            proof.verify(&chain).map_err(ProgramBundleError::Evidence)
        })
        .transpose()?;
    Ok(VerifiedProgramBundle {
        program,
        chain,
        interface,
        wind_down,
    })
}

fn existence(ops: &[ProofOp]) -> Result<ExistenceProof, ChainProofError> {
    let op = ops.first().ok_or(ChainProofError::Mismatch)?;
    let mut reader = ProtoReader::new(&op.data);
    let field = reader.next_field()?.ok_or(ChainProofError::Mismatch)?;
    let proof = match (field.number, field.value) {
        (1, ProtoValue::Bytes(bytes)) => ExistenceProof::decode(bytes)?,
        _ => return Err(ChainProofError::Mismatch),
    };
    reader.finish()?;
    Ok(proof)
}

fn decode_commit(bytes: &[u8]) -> Result<Commit, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut height = None;
    let mut round = None;
    let mut block_id = None;
    let mut signatures = Vec::new();
    while let Some(field) = reader.next_field()? {
        match (field.number, field.value) {
            (1, ProtoValue::Varint(value)) if height.is_none() => {
                height = Some(i64::try_from(value).map_err(|_| ChainProofError::NonCanonical)?);
            }
            (2, ProtoValue::Varint(value)) if round.is_none() => {
                round = Some(i32::try_from(value).map_err(|_| ChainProofError::NonCanonical)?);
            }
            (3, ProtoValue::Bytes(value)) if block_id.is_none() => {
                block_id = Some(decode_block_id(value)?)
            }
            (4, ProtoValue::Bytes(value)) => signatures.push(CommitSig::decode(value)?),
            _ => return Err(ChainProofError::NonCanonical),
        }
    }
    reader.finish()?;
    Ok(Commit {
        height: height.ok_or(ChainProofError::NonCanonical)?,
        round: round.unwrap_or(0),
        block_id: block_id.ok_or(ChainProofError::NonCanonical)?,
        signatures,
    })
}

fn decode_block_id(bytes: &[u8]) -> Result<BlockId, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut hash = None;
    let mut parts = None;
    while let Some(field) = reader.next_field()? {
        match (field.number, field.value) {
            (1, ProtoValue::Bytes(value)) if hash.is_none() => hash = Some(value.to_vec()),
            (2, ProtoValue::Bytes(value)) if parts.is_none() => {
                let mut part_reader = ProtoReader::new(value);
                let mut total = None;
                let mut part_hash = None;
                while let Some(part) = part_reader.next_field()? {
                    match (part.number, part.value) {
                        (1, ProtoValue::Varint(value)) if total.is_none() => {
                            total = Some(
                                u32::try_from(value).map_err(|_| ChainProofError::NonCanonical)?,
                            );
                        }
                        (2, ProtoValue::Bytes(value)) if part_hash.is_none() => {
                            part_hash = Some(value.to_vec())
                        }
                        _ => return Err(ChainProofError::NonCanonical),
                    }
                }
                part_reader.finish()?;
                parts = Some(PartSetHeader {
                    total: total.ok_or(ChainProofError::NonCanonical)?,
                    hash: part_hash.ok_or(ChainProofError::NonCanonical)?,
                });
            }
            _ => return Err(ChainProofError::NonCanonical),
        }
    }
    reader.finish()?;
    let block = BlockId {
        hash: hash.ok_or(ChainProofError::NonCanonical)?,
        part_set_header: parts.ok_or(ChainProofError::NonCanonical)?,
    };
    if block.hash.len() != 32
        || block.part_set_header.hash.len() != 32
        || !(1..=101).contains(&block.part_set_header.total)
    {
        return Err(ChainProofError::NonCanonical);
    }
    Ok(block)
}

fn decode_validators(bytes: &[u8]) -> Result<ValidatorSet, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut validators = Vec::new();
    let mut proposer = None;
    let mut total = None;
    while let Some(field) = reader.next_field()? {
        match (field.number, field.value) {
            (1, ProtoValue::Bytes(value)) => validators.push(decode_validator(value)?),
            (2, ProtoValue::Bytes(value)) if proposer.is_none() => {
                proposer = Some(decode_validator(value)?)
            }
            (3, ProtoValue::Varint(value)) if total.is_none() => total = Some(value),
            _ => return Err(ChainProofError::NonCanonical),
        }
    }
    reader.finish()?;
    let set = ValidatorSet::new(validators)?;
    if set.validators().is_empty()
        || total != Some(set.total_power())
        || !proposer
            .as_ref()
            .is_some_and(|proposer| set.validators().contains(proposer))
    {
        return Err(ChainProofError::NonCanonical);
    }
    Ok(set)
}

fn decode_validator(bytes: &[u8]) -> Result<TrustedValidator, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut address = None;
    let mut public_key = None;
    let mut power = None;
    let mut priority = None;
    while let Some(field) = reader.next_field()? {
        match (field.number, field.value) {
            (1, ProtoValue::Bytes(value)) if address.is_none() => address = Some(value),
            (2, ProtoValue::Bytes(value)) if public_key.is_none() => {
                let mut key_reader = ProtoReader::new(value);
                let key = key_reader
                    .next_field()?
                    .ok_or(ChainProofError::NonCanonical)?;
                public_key = match (key.number, key.value) {
                    (1, ProtoValue::Bytes(value)) => Some(
                        value
                            .try_into()
                            .map_err(|_| ChainProofError::NonCanonical)?,
                    ),
                    _ => return Err(ChainProofError::NonCanonical),
                };
                key_reader.finish()?;
            }
            (3, ProtoValue::Varint(value)) if power.is_none() => power = Some(value),
            (4, ProtoValue::Varint(value)) if priority.is_none() => priority = Some(value),
            _ => return Err(ChainProofError::NonCanonical),
        }
    }
    reader.finish()?;
    let validator = TrustedValidator::new(
        public_key.ok_or(ChainProofError::NonCanonical)?,
        power.ok_or(ChainProofError::NonCanonical)?,
    );
    if address != Some(validator.address.as_slice()) {
        return Err(ChainProofError::NonCanonical);
    }
    Ok(validator)
}

fn encoding(_: ChainProofError) -> ProgramBundleError {
    invalid_encoding()
}

fn invalid_encoding() -> ProgramBundleError {
    ProgramBundleError::Evidence(ProtocolEvidenceError::Encoding)
}

fn put_field(bytes: &mut Vec<u8>, value: &[u8], limit: usize) -> Result<(), ProgramBundleError> {
    if value.len() > limit {
        return Err(invalid_encoding());
    }
    bytes.extend_from_slice(
        &u32::try_from(value.len())
            .map_err(|_| invalid_encoding())?
            .to_be_bytes(),
    );
    bytes.extend_from_slice(value);
    Ok(())
}

fn put_ops(bytes: &mut Vec<u8>, ops: &[ProofOp]) -> Result<(), ProgramBundleError> {
    if ops.len() != 2 {
        return Err(invalid_encoding());
    }
    bytes.extend_from_slice(&2_u32.to_be_bytes());
    for op in ops {
        put_field(bytes, op.op_type.as_bytes(), MAX_FIELD_BYTES)?;
        put_field(bytes, &op.key, MAX_FIELD_BYTES)?;
        put_field(bytes, &op.data, MAX_FIELD_BYTES)?;
    }
    Ok(())
}

struct Reader<'a> {
    bytes: &'a [u8],
    cursor: usize,
}

impl<'a> Reader<'a> {
    fn take(&mut self, length: usize) -> Result<&'a [u8], ProgramBundleError> {
        let end = self
            .cursor
            .checked_add(length)
            .ok_or_else(invalid_encoding)?;
        let value = self
            .bytes
            .get(self.cursor..end)
            .ok_or_else(invalid_encoding)?;
        self.cursor = end;
        Ok(value)
    }

    fn byte(&mut self) -> Result<u8, ProgramBundleError> {
        Ok(self.take(1)?[0])
    }

    fn u32(&mut self) -> Result<u32, ProgramBundleError> {
        Ok(u32::from_be_bytes(
            self.take(4)?.try_into().map_err(|_| invalid_encoding())?,
        ))
    }

    fn field(&mut self, limit: usize) -> Result<&'a [u8], ProgramBundleError> {
        let length = usize::try_from(self.u32()?).map_err(|_| invalid_encoding())?;
        if length > limit {
            return Err(invalid_encoding());
        }
        self.take(length)
    }

    fn ops(&mut self) -> Result<Vec<ProofOp>, ProgramBundleError> {
        if self.u32()? != 2 {
            return Err(invalid_encoding());
        }
        (0..2)
            .map(|_| {
                Ok(ProofOp {
                    op_type: std::str::from_utf8(self.field(MAX_FIELD_BYTES)?)
                        .map_err(|_| invalid_encoding())?
                        .to_owned(),
                    key: self.field(MAX_FIELD_BYTES)?.to_vec(),
                    data: self.field(MAX_FIELD_BYTES)?.to_vec(),
                })
            })
            .collect()
    }
}
