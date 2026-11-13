use crate::{
    account_state::ProgramValueAccountBinding,
    chain_evidence::{freshness::chain_freshness, programs_state_key, HOST_STORE_NAME},
    chain_proof::{
        decode::{decode_commitment_proof, CommitmentProof},
        header::ChainHeader,
        ics23::{ExistenceProof, ProofSpec},
        ics23_absent::{adjacent, NonExistenceProof},
        proto::{ProtoReader, ProtoValue},
        state::{verify_absent, verify_state},
        VerifiedChainHead,
    },
    AccountStateError, CanonicalAccountLeaf, ProgramId, ReadFreshness, ValueAccount,
    VerifiedAccountSnapshot, MAX_PROGRAM_VALUE_ACCOUNTS,
};

/// Canonical chain query responses for one complete Programs primary-key range.
/// Bounds are absence proofs for `binding_range`; account responses address
/// lxphost's standalone `06 || account_id` records, not the old module mirror.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainProgramAccountProof {
    pub program: ProgramId,
    pub lower_bound: CommitmentProof,
    pub upper_bound: CommitmentProof,
    pub bindings: Vec<CommitmentProof>,
    pub accounts: Vec<CommitmentProof>,
    pub sequence: CommitmentProof,
    pub profile: Option<CommitmentProof>,
    pub owner: Option<CommitmentProof>,
}

/// Complete bindings and balances authenticated under one chain app hash.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedChainProgramAccounts {
    head: VerifiedChainHead,
    program: ProgramId,
    bindings: Vec<ProgramValueAccountBinding>,
    accounts: Vec<ValueAccount>,
    profile2: bool,
    sequence: u64,
}

/// The current trusted chain head and the caller's current freshness window.
#[derive(Clone, Copy, Debug)]
pub struct ChainAccountStateAuthority<'a> {
    head: &'a VerifiedChainHead,
    now_ms: u64,
    max_age_ms: u64,
}

impl<'a> ChainAccountStateAuthority<'a> {
    ///
    /// # Errors
    ///
    /// Returns an error when the account or state record cannot be validated or encoded.
    pub fn new(
        head: &'a VerifiedChainHead,
        now_ms: u64,
        max_age_ms: u64,
    ) -> Result<Self, AccountStateError> {
        if now_ms == 0 || max_age_ms == 0 {
            return Err(AccountStateError::InvalidFreshness);
        }
        chain_freshness(head, now_ms, max_age_ms).map_err(|_| AccountStateError::StaleRead)?;
        Ok(Self {
            head,
            now_ms,
            max_age_ms,
        })
    }

    pub(crate) fn verify(
        &self,
        snapshot: &VerifiedChainProgramAccounts,
    ) -> Result<(), AccountStateError> {
        if self.head != &snapshot.head {
            return Err(AccountStateError::UnverifiedReceipt);
        }
        chain_freshness(self.head, self.now_ms, self.max_age_ms)
            .map_err(|_| AccountStateError::StaleRead)?;
        Ok(())
    }
}

impl ChainProgramAccountProof {
    /// Encodes untrusted evidence for transport and durable replay.
    ///
    /// # Errors
    ///
    /// Returns an error when the account or state record cannot be validated or encoded.
    pub fn canonical_encoding(&self) -> Result<Vec<u8>, AccountStateError> {
        if self.bindings.len() > MAX_PROGRAM_VALUE_ACCOUNTS
            || self.accounts.len() > MAX_PROGRAM_VALUE_ACCOUNTS
        {
            return Err(AccountStateError::InvalidProof);
        }
        let mut bytes = b"LXCA\x01".to_vec();
        bytes.extend_from_slice(&self.program.bytes());
        for proof in [&self.lower_bound, &self.upper_bound, &self.sequence] {
            put_proof(&mut bytes, proof)?;
        }
        for proof in [&self.profile, &self.owner] {
            bytes.push(u8::from(proof.is_some()));
            if let Some(proof) = proof {
                put_proof(&mut bytes, proof)?;
            }
        }
        for proofs in [&self.bindings, &self.accounts] {
            bytes.extend_from_slice(
                &u32::try_from(proofs.len())
                    .map_err(|_| AccountStateError::InvalidProof)?
                    .to_be_bytes(),
            );
            for proof in proofs {
                put_proof(&mut bytes, proof)?;
            }
        }
        if bytes.len() > MAX_ACCOUNT_EVIDENCE_BYTES {
            return Err(AccountStateError::InvalidProof);
        }
        Ok(bytes)
    }

    ///
    /// # Errors
    ///
    /// Returns an error when the account or state record cannot be validated or encoded.
    pub fn decode(bytes: &[u8]) -> Result<Self, AccountStateError> {
        if bytes.len() > MAX_ACCOUNT_EVIDENCE_BYTES {
            return Err(AccountStateError::InvalidProof);
        }
        let mut reader = AccountEvidenceReader { bytes };
        if reader.take(5)? != b"LXCA\x01" {
            return Err(AccountStateError::InvalidProof);
        }
        let program = ProgramId::new(
            reader
                .take(32)?
                .try_into()
                .map_err(|_| AccountStateError::InvalidProof)?,
        )
        .map_err(|_| AccountStateError::InvalidBinding)?;
        let lower_bound = reader.proof()?;
        let upper_bound = reader.proof()?;
        let sequence = reader.proof()?;
        let profile = reader.optional_proof()?;
        let owner = reader.optional_proof()?;
        let bindings = reader.proofs()?;
        let accounts = reader.proofs()?;
        if !reader.bytes.is_empty() {
            return Err(AccountStateError::InvalidProof);
        }
        let proof = Self {
            program,
            lower_bound,
            upper_bound,
            bindings,
            accounts,
            sequence,
            profile,
            owner,
        };
        if proof.canonical_encoding()? != bytes {
            return Err(AccountStateError::InvalidProof);
        }
        Ok(proof)
    }

    /// The two exclusive bound keys for the canonical, fixed-width primary index.
    #[must_use]
    pub fn binding_range(program: ProgramId) -> (Vec<u8>, Vec<u8>) {
        let mut primary = b"program-account\0p".to_vec();
        primary.extend_from_slice(&program.bytes());
        let lower = programs_state_key(&primary);
        let mut upper = lower.clone();
        while upper.last() == Some(&u8::MAX) {
            upper.pop();
        }
        if let Some(byte) = upper.last_mut() {
            *byte += 1;
        }
        (lower, upper)
    }

    ///
    /// # Errors
    ///
    /// Returns an error when the account or state record cannot be validated or encoded.
    pub fn verify(
        &self,
        head: &VerifiedChainHead,
    ) -> Result<VerifiedChainProgramAccounts, AccountStateError> {
        let bindings = self.verify_bindings(head)?;
        let sequence = u64::from_be_bytes(
            membership(head, &self.sequence, &[2])?
                .try_into()
                .map_err(|_| AccountStateError::InvalidFreshness)?,
        );
        if sequence == 0 {
            return Err(AccountStateError::InvalidFreshness);
        }
        let mut accounts = std::collections::BTreeMap::new();
        for response in &self.accounts {
            let value = response
                .value
                .as_deref()
                .ok_or(AccountStateError::InvalidAccountLeaf)?;
            let leaf = CanonicalAccountLeaf::decode_host_record(value)?;
            let mut key = vec![6];
            key.extend_from_slice(&leaf.account_id);
            membership(head, response, &key)?;
            let verified = VerifiedAccountSnapshot::verify_chain(head, &leaf, &response.proof_ops)?;
            if accounts.insert(leaf.account_id, verified).is_some() {
                return Err(AccountStateError::DuplicateAccount {
                    account_id: leaf.account_id,
                });
            }
        }
        let mut resolved = Vec::with_capacity(bindings.len());
        for binding in &bindings {
            let account =
                accounts
                    .remove(&binding.account_id)
                    .ok_or(AccountStateError::MissingAccount {
                        account_id: binding.account_id,
                    })?;
            let leaf = account.account();
            if leaf.asset_id != binding.asset_id
                || leaf.created_at_sequence != binding.registered_sequence
                || binding.registered_sequence > sequence
            {
                return Err(AccountStateError::AccountMismatch {
                    account_id: binding.account_id,
                });
            }
            resolved.push(ValueAccount {
                seed: binding.seed.clone(),
                account_id: binding.account_id,
                asset_id: binding.asset_id,
                balance: leaf.balance,
                frozen: bool::from(leaf.frozen),
                observed_sequence: sequence,
                receipt_digest: head.header_hash(),
                state_root: head.app_hash(),
            });
        }
        let profile2 = match (&self.profile, &self.owner) {
            (None, None) => false,
            (Some(profile), Some(owner)) => {
                let mut key = b"program-owner\0".to_vec();
                key.extend_from_slice(&self.program.bytes());
                let value = membership(head, owner, &programs_state_key(&key))?;
                if value.len() != 33 || value[0] != 1 || value[1..] == [0; 32] {
                    return Err(AccountStateError::InvalidProof);
                }
                let mut expected = value.to_vec();
                expected[0] = 2;
                let mut key = b"program-account-profile\0".to_vec();
                key.extend_from_slice(&self.program.bytes());
                if membership(head, profile, &programs_state_key(&key))? != expected {
                    return Err(AccountStateError::InvalidProof);
                }
                true
            }
            _ => return Err(AccountStateError::InvalidProof),
        };
        Ok(VerifiedChainProgramAccounts {
            head: head.clone(),
            program: self.program,
            bindings,
            accounts: resolved,
            profile2,
            sequence,
        })
    }
    fn verify_bindings(
        &self,
        head: &VerifiedChainHead,
    ) -> Result<Vec<ProgramValueAccountBinding>, AccountStateError> {
        if self.bindings.len() > MAX_PROGRAM_VALUE_ACCOUNTS
            || self.bindings.len() != self.accounts.len()
        {
            return Err(AccountStateError::InvalidProof);
        }
        let (lower, upper) = Self::binding_range(self.program);
        absence(head, &self.lower_bound, &lower)?;
        absence(head, &self.upper_bound, &upper)?;
        let start = NonExistenceProof::decode(inner(&self.lower_bound, 2)?)
            .map_err(|_| AccountStateError::InvalidProof)?;
        let end = NonExistenceProof::decode(inner(&self.upper_bound, 2)?)
            .map_err(|_| AccountStateError::InvalidProof)?;
        if start.left.as_ref().is_some_and(|p| p.key >= lower)
            || end.right.as_ref().is_some_and(|p| p.key <= upper)
        {
            return Err(AccountStateError::InvalidProof);
        }
        let mut bindings = Vec::with_capacity(self.bindings.len());
        let mut previous: Option<ExistenceProof> = None;
        for response in &self.bindings {
            let value = membership(head, response, &response.key)?;
            let binding = ProgramValueAccountBinding::decode_primary_value(value)?;
            if binding.program != self.program
                || response.key != programs_state_key(&binding.primary_key())
            {
                return Err(AccountStateError::InvalidBinding);
            }
            let proof = ExistenceProof::decode(inner(response, 1)?)
                .map_err(|_| AccountStateError::InvalidProof)?;
            if let Some(prior) = &previous {
                if prior.key >= proof.key
                    || !adjacent(&prior.path, &proof.path, &ProofSpec::iavl().inner_spec)
                {
                    return Err(AccountStateError::InvalidProof);
                }
            } else if start.right.as_ref().map(|p| &p.key) != Some(&proof.key) {
                return Err(AccountStateError::InvalidProof);
            }
            previous = Some(proof);
            bindings.push(binding);
        }
        match previous {
            Some(last) if end.left.as_ref().map(|p| &p.key) == Some(&last.key) => {}
            None if start.right.as_ref().is_none_or(|p| p.key >= upper)
                && end.left.as_ref().is_none_or(|p| p.key <= lower) => {}
            _ => return Err(AccountStateError::InvalidProof),
        }
        bindings.sort_by_key(|b| b.account_id);
        if bindings
            .windows(2)
            .any(|p| p[0].account_id == p[1].account_id)
        {
            return Err(AccountStateError::InvalidBinding);
        }
        Ok(bindings)
    }
}

impl VerifiedChainProgramAccounts {
    #[must_use]
    pub const fn chain_head(&self) -> &VerifiedChainHead {
        &self.head
    }
    #[must_use]
    pub const fn program(&self) -> ProgramId {
        self.program
    }
    #[must_use]
    pub fn bindings(&self) -> &[ProgramValueAccountBinding] {
        &self.bindings
    }
    #[must_use]
    pub fn accounts(&self) -> &[ValueAccount] {
        &self.accounts
    }
    #[must_use]
    pub fn freshness(&self) -> ReadFreshness {
        ReadFreshness {
            observed_sequence: self.sequence,
            observed_at: u64::from_be_bytes(self.head.time_ms().to_be_bytes()),
        }
    }
    pub(crate) fn admits_abi(&self, abi: u16) -> bool {
        abi == 2
            || (self.profile2
                && layerx_programs_runtime::abi_policy::account_profile2_guest_supported(abi))
    }
    pub(crate) fn resolve_program(
        &self,
        program: ProgramId,
        prior: &[ProgramValueAccountBinding],
        authority: &ChainAccountStateAuthority<'_>,
    ) -> Result<Vec<ValueAccount>, AccountStateError> {
        authority.verify(self)?;
        if program != self.program
            || prior.iter().any(|b| {
                self.bindings
                    .binary_search_by_key(&b.account_id, |candidate| candidate.account_id)
                    .ok()
                    .is_none_or(|index| self.bindings[index] != *b)
            })
        {
            return Err(AccountStateError::InvalidBinding);
        }
        Ok(self.accounts.clone())
    }
}

fn check_head(
    head: &VerifiedChainHead,
    proof: &CommitmentProof,
    key: &[u8],
) -> Result<(), AccountStateError> {
    let header =
        ChainHeader::decode(&proof.signed_header).map_err(|_| AccountStateError::InvalidProof)?;
    if proof.store_name != HOST_STORE_NAME
        || proof.key != key
        || header.hash().map_err(|_| AccountStateError::InvalidProof)? != head.header_hash()
    {
        return Err(AccountStateError::UnverifiedReceipt);
    }
    Ok(())
}
fn membership<'a>(
    head: &VerifiedChainHead,
    proof: &'a CommitmentProof,
    key: &[u8],
) -> Result<&'a [u8], AccountStateError> {
    check_head(head, proof, key)?;
    let value = proof
        .value
        .as_deref()
        .ok_or(AccountStateError::InvalidProof)?;
    verify_state(head, HOST_STORE_NAME, key, value, &proof.proof_ops)
        .map_err(|_| AccountStateError::InvalidProof)?;
    Ok(value)
}
fn absence(
    head: &VerifiedChainHead,
    proof: &CommitmentProof,
    key: &[u8],
) -> Result<(), AccountStateError> {
    check_head(head, proof, key)?;
    if proof.value.is_some() {
        return Err(AccountStateError::InvalidProof);
    }
    verify_absent(head, HOST_STORE_NAME, key, &proof.proof_ops)
        .map_err(|_| AccountStateError::InvalidProof)?;
    Ok(())
}
fn inner(proof: &CommitmentProof, variant: u32) -> Result<&[u8], AccountStateError> {
    let mut reader = ProtoReader::new(
        &proof
            .proof_ops
            .first()
            .ok_or(AccountStateError::InvalidProof)?
            .data,
    );
    let field = reader
        .next_field()
        .map_err(|_| AccountStateError::InvalidProof)?
        .ok_or(AccountStateError::InvalidProof)?;
    let ProtoValue::Bytes(bytes) = field.value else {
        return Err(AccountStateError::InvalidProof);
    };
    if field.number != variant {
        return Err(AccountStateError::InvalidProof);
    }
    reader
        .finish()
        .map_err(|_| AccountStateError::InvalidProof)?;
    Ok(bytes)
}

const MAX_ACCOUNT_EVIDENCE_BYTES: usize = 64 * 1024 * 1024;
fn put_proof(bytes: &mut Vec<u8>, proof: &CommitmentProof) -> Result<(), AccountStateError> {
    let encoded = proof
        .encode()
        .map_err(|_| AccountStateError::InvalidProof)?;
    if bytes
        .len()
        .checked_add(4)
        .and_then(|n| n.checked_add(encoded.len()))
        .is_none_or(|n| n > MAX_ACCOUNT_EVIDENCE_BYTES)
    {
        return Err(AccountStateError::InvalidProof);
    }
    bytes.extend_from_slice(
        &u32::try_from(encoded.len())
            .map_err(|_| AccountStateError::InvalidProof)?
            .to_be_bytes(),
    );
    bytes.extend_from_slice(&encoded);
    Ok(())
}
struct AccountEvidenceReader<'a> {
    bytes: &'a [u8],
}
impl<'a> AccountEvidenceReader<'a> {
    fn take(&mut self, n: usize) -> Result<&'a [u8], AccountStateError> {
        let part = self.bytes.get(..n).ok_or(AccountStateError::InvalidProof)?;
        self.bytes = &self.bytes[n..];
        Ok(part)
    }
    fn count(&mut self) -> Result<usize, AccountStateError> {
        usize::try_from(u32::from_be_bytes(
            self.take(4)?
                .try_into()
                .map_err(|_| AccountStateError::InvalidProof)?,
        ))
        .map_err(|_| AccountStateError::InvalidProof)
    }
    fn proof(&mut self) -> Result<CommitmentProof, AccountStateError> {
        let n = self.count()?;
        decode_commitment_proof(self.take(n)?).map_err(|_| AccountStateError::InvalidProof)
    }
    fn optional_proof(&mut self) -> Result<Option<CommitmentProof>, AccountStateError> {
        match self.take(1)? {
            [0] => Ok(None),
            [1] => Ok(Some(self.proof()?)),
            _ => Err(AccountStateError::InvalidProof),
        }
    }
    fn proofs(&mut self) -> Result<Vec<CommitmentProof>, AccountStateError> {
        let n = self.count()?;
        if n > MAX_PROGRAM_VALUE_ACCOUNTS || n > self.bytes.len() / 4 {
            return Err(AccountStateError::InvalidProof);
        }
        (0..n).map(|_| self.proof()).collect()
    }
}
