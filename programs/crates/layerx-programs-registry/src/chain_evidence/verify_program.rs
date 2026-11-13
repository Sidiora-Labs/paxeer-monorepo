use crate::{
    chain_evidence::{
        freshness::chain_freshness,
        lifecycle::ChainLifecycleProof,
        program_state::{ChainProgramStateProof, VerifiedProgramRecord},
        wind_down::ChainWindDownProof,
    },
    chain_proof::VerifiedChainHead,
    ProgramId, ProgramLifecycle, ProtocolEvidenceError, ReadFreshness, UpgradePolicy,
};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedProgramHead {
    record: VerifiedProgramRecord,
    lifecycle: ProgramLifecycle,
    wind_down: Option<[u8; 54]>,
    freshness: ReadFreshness,
    valid_until_ms: u64,
}

impl VerifiedProgramHead {
    #[must_use]
    pub const fn program(&self) -> ProgramId {
        self.record.program()
    }

    #[must_use]
    pub const fn version(&self) -> u32 {
        self.record.version()
    }

    #[must_use]
    pub const fn code_hash(&self) -> [u8; 32] {
        self.record.code_hash()
    }

    #[must_use]
    pub const fn abi_version(&self) -> u16 {
        self.record.abi_version()
    }

    #[must_use]
    pub const fn policy(&self) -> UpgradePolicy {
        self.record.upgrade_policy()
    }

    #[must_use]
    pub const fn lifecycle(&self) -> ProgramLifecycle {
        self.lifecycle
    }

    #[must_use]
    pub const fn wind_down(&self) -> Option<[u8; 54]> {
        self.wind_down
    }

    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.record.state_root()
    }

    #[must_use]
    pub const fn freshness(&self) -> ReadFreshness {
        self.freshness
    }

    #[must_use]
    pub const fn valid_until_ms(&self) -> u64 {
        self.valid_until_ms
    }
}

pub(crate) fn current_program_at(
    head: &VerifiedChainHead,
    proof: &ChainProgramStateProof,
    lifecycle: &ChainLifecycleProof,
    wind_down: &ChainWindDownProof,
    expected_program: ProgramId,
    now_ms: u64,
    max_age_ms: u64,
) -> Result<VerifiedProgramHead, ProtocolEvidenceError> {
    if proof.program != expected_program {
        return Err(ProtocolEvidenceError::ProgramRecord);
    }
    if lifecycle.program != expected_program || wind_down.program != expected_program {
        return Err(ProtocolEvidenceError::LifecycleProof);
    }
    let freshness = chain_freshness(head, now_ms, max_age_ms)?;
    let valid_until_ms = freshness
        .observed_at
        .checked_add(max_age_ms)
        .ok_or(ProtocolEvidenceError::Stale)?;
    let record = proof.verify(head)?;
    let state = lifecycle.verify(head)?;
    let status = wind_down.verify(head)?;
    if state == ProgramLifecycle::Tombstoned
        || lifecycle.status.as_deref() != status.as_ref().map(<[u8; 54]>::as_slice)
    {
        return Err(ProtocolEvidenceError::LifecycleProof);
    }
    Ok(VerifiedProgramHead {
        record,
        lifecycle: state,
        wind_down: status,
        freshness,
        valid_until_ms,
    })
}
