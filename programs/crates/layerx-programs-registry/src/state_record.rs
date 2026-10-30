use crate::{
    AccountStateError, AccountStateHead, AccountStateJournal, CanonicalAccountLeaf, ExitRoute,
    JournalAccountStateAuthority, LifecycleReceipt, ProgramId, ProgramLifecycle,
    ProgramValueAccountBinding, ProvenAccountLeaf, ProvenProgramBinding, ReadFreshness, Registry,
    RegistryError, StateProof, VerifiedAccountSnapshot, VerifiedProgramAccountProfile2,
    VerifiedProgramBalanceRead, WindDownPolicy, WindDownStateAccess,
};

const MAX_SEED_BYTES: usize = 128;
const MAX_ACCOUNT_NAME_BYTES: usize = 512;
const MAX_PROOF_DEPTH: usize = 32;
const RECORD_MAGIC: &[u8; 5] = b"LXPS1";
const PROFILE2_RECORD_MAGIC: &[u8; 5] = b"LXPS2";
const MAX_PROFILE2_RECORD_BYTES: usize = 64 * 1024 * 1024;
const MAX_RECORD_ITEMS: usize = 4_096;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProtocolAdapterError {
    CoreRefused(i32),
    NonCanonicalView,
    AccountState(AccountStateError),
    Registry(RegistryError),
    CorruptRecord,
}

impl From<AccountStateError> for ProtocolAdapterError {
    fn from(value: AccountStateError) -> Self {
        Self::AccountState(value)
    }
}

impl From<RegistryError> for ProtocolAdapterError {
    fn from(value: RegistryError) -> Self {
        Self::Registry(value)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProtocolProgramStateRead {
    balances: VerifiedProgramBalanceRead,
    snapshot: VerifiedAccountSnapshot,
    bindings: Vec<ProgramValueAccountBinding>,
    lifecycle: ProgramLifecycle,
    routes: Vec<ExitRoute>,
    history: Vec<LifecycleReceipt>,
    account_profile2: Option<VerifiedProgramAccountProfile2>,
}

impl ProtocolProgramStateRead {
    pub fn verify_snapshot(
        registry: &mut Registry,
        program: ProgramId,
        snapshot: VerifiedAccountSnapshot,
        bindings: Vec<ProgramValueAccountBinding>,
        program_lifecycle: ProgramLifecycle,
        routes: Vec<ExitRoute>,
        history: Vec<LifecycleReceipt>,
        head: AccountStateHead,
        now: u64,
        staleness_limit: u64,
    ) -> Result<Self, ProtocolAdapterError> {
        registry.replay_protocol_state(program, &bindings, &routes, program_lifecycle, &history)?;
        let journal = ProtocolJournal { head };
        let authority = JournalAccountStateAuthority::new(journal, now, staleness_limit)?;
        let balances = registry.read_value_accounts(program, &snapshot, &authority)?;
        Ok(Self {
            balances,
            snapshot,
            bindings,
            lifecycle: program_lifecycle,
            routes,
            history,
            account_profile2: None,
        })
    }

    #[must_use]
    pub const fn program(&self) -> ProgramId {
        self.balances.program()
    }

    #[must_use]
    pub const fn balances(&self) -> &VerifiedProgramBalanceRead {
        &self.balances
    }

    #[must_use]
    pub fn routes(&self) -> &[ExitRoute] {
        &self.routes
    }

    #[must_use]
    pub fn history(&self) -> &[LifecycleReceipt] {
        &self.history
    }

    #[must_use]
    pub fn into_balances(self) -> VerifiedProgramBalanceRead {
        self.balances
    }

    /// # Errors
    ///
    /// Returns `CorruptRecord` when a collection or proof exceeds its encoding bound.
    pub fn canonical_encode(&self) -> Result<Vec<u8>, ProtocolAdapterError> {
        let mut encoded = RECORD_MAGIC.to_vec();
        encoded.extend_from_slice(&self.balances.program().bytes());
        encoded.push(lifecycle_byte(self.lifecycle));
        put_bindings(&mut encoded, &self.bindings)?;
        put_routes(&mut encoded, &self.routes)?;
        put_history(&mut encoded, &self.history)?;
        put_snapshot(&mut encoded, &self.snapshot)?;
        Ok(encoded)
    }

    /// Restores one proof record only after an independent production node
    /// receipt resolver has supplied both its verified receipt facts and the
    /// current canonical account-state head. Stored bytes never authorize
    /// themselves and a historical-but-valid receipt is not published as a
    /// live balance read.
    ///
    /// # Errors
    ///
    /// Returns an error for malformed records, invalid time bounds, mismatched
    /// receipt or state heads, or failed account-state and registry verification.
    pub fn restore_verified(
        bytes: &[u8],
        registry: &mut Registry,
        verified_receipt: AccountStateHead,
        current_head: AccountStateHead,
        now: u64,
        staleness_limit: u64,
    ) -> Result<Self, ProtocolAdapterError> {
        if now == 0 || staleness_limit == 0 {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        Self::restore_inner(
            bytes,
            registry,
            verified_receipt,
            current_head,
            now,
            staleness_limit,
        )
    }

    #[must_use]
    pub const fn account_snapshot(&self) -> &VerifiedAccountSnapshot {
        &self.snapshot
    }

    pub fn account_profile2(&self) -> Option<&VerifiedProgramAccountProfile2> {
        self.account_profile2.as_ref()
    }

    pub fn restore_verified_profile2(
        bytes: &[u8],
        registry: &mut Registry,
        verified_receipt: AccountStateHead,
        current_head: AccountStateHead,
        now: u64,
        staleness_limit: u64,
    ) -> Result<Self, ProtocolAdapterError> {
        if now == 0 || staleness_limit == 0 || bytes.len() > MAX_PROFILE2_RECORD_BYTES {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        let mut wrapper = Cursor::new(bytes);
        if wrapper.take(PROFILE2_RECORD_MAGIC.len())? != PROFILE2_RECORD_MAGIC {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        let inner_length =
            usize::try_from(wrapper.u32()?).map_err(|_| ProtocolAdapterError::CorruptRecord)?;
        let inner = wrapper.take(inner_length)?;
        let mut cursor = Cursor::new(inner);
        if cursor.take(RECORD_MAGIC.len())? != RECORD_MAGIC {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        let program =
            ProgramId::new(cursor.array()?).map_err(|_| ProtocolAdapterError::CorruptRecord)?;
        let lifecycle = lifecycle(i32::from(cursor.byte()?))?;
        let bindings = take_bindings(&mut cursor, program)?;
        let routes = take_routes(&mut cursor)?;
        let history = take_history(&mut cursor, program)?;
        let snapshot = take_snapshot(&mut cursor)?;
        if !cursor.is_empty() {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        let profile_key_length = usize::from(wrapper.u16()?);
        let profile_key = wrapper.take(profile_key_length)?;
        let mut expected_profile_key = b"program-account-profile\0".to_vec();
        expected_profile_key.extend_from_slice(&program.bytes());
        let profile_value_length = usize::from(wrapper.u16()?);
        let profile_value = wrapper.take(profile_value_length)?;
        if profile_key != expected_profile_key || profile_value.len() != 33 || profile_value[0] != 2
        {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        let owner: [u8; 32] = profile_value[1..]
            .try_into()
            .map_err(|_| ProtocolAdapterError::CorruptRecord)?;
        let programs_root: [u8; 32] = wrapper.array()?;
        let programs_root_proof = take_proof(&mut wrapper)?;
        let profile_proof = take_proof(&mut wrapper)?;
        let owner_key_length = usize::from(wrapper.u16()?);
        let owner_key = wrapper.take(owner_key_length)?;
        let owner_value_length = usize::from(wrapper.u16()?);
        let owner_value = wrapper.take(owner_value_length)?;
        let mut expected_owner_key = b"program-owner\0".to_vec();
        expected_owner_key.extend_from_slice(&program.bytes());
        let mut expected_owner_value = vec![1];
        expected_owner_value.extend_from_slice(&owner);
        let owner_proof = take_proof(&mut wrapper)?;
        if !wrapper.is_empty()
            || owner_key != expected_owner_key
            || owner_value != expected_owner_value
            || programs_root != snapshot.programs_root
        {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        let snapshot_head = AccountStateHead {
            receipt_digest: snapshot.receipt_digest,
            state_root: snapshot.state_root,
            freshness: snapshot.freshness,
        };
        if verified_receipt != snapshot_head {
            return Err(AccountStateError::UnverifiedReceipt.into());
        }
        if current_head != verified_receipt {
            return Err(AccountStateError::StaleRead.into());
        }
        let authority = JournalAccountStateAuthority::new(
            ProtocolJournal { head: current_head },
            now,
            staleness_limit,
        )?;
        let profile = VerifiedProgramAccountProfile2::verify(
            program,
            owner,
            &snapshot,
            &programs_root_proof,
            &profile_proof,
            &owner_proof,
            &authority,
        )?;
        let mut candidate = registry.clone();
        candidate.replay_protocol_state_profile2(
            program, &bindings, &routes, lifecycle, &history, &profile,
        )?;
        let balances =
            candidate.read_value_accounts_profile2(program, &snapshot, &authority, &profile)?;
        let state = Self {
            balances,
            snapshot,
            bindings,
            lifecycle,
            routes,
            history,
            account_profile2: Some(profile),
        };
        if state.canonical_encode()? != inner {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        *registry = candidate;
        Ok(state)
    }

    fn restore_inner(
        bytes: &[u8],
        registry: &mut Registry,
        verified_receipt: AccountStateHead,
        current_head: AccountStateHead,
        now: u64,
        staleness_limit: u64,
    ) -> Result<Self, ProtocolAdapterError> {
        let mut cursor = Cursor::new(bytes);
        if cursor.take(RECORD_MAGIC.len())? != RECORD_MAGIC {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        let program =
            ProgramId::new(cursor.array()?).map_err(|_| ProtocolAdapterError::CorruptRecord)?;
        let lifecycle = lifecycle(i32::from(cursor.byte()?))?;
        let bindings = take_bindings(&mut cursor, program)?;
        let routes = take_routes(&mut cursor)?;
        let history = take_history(&mut cursor, program)?;
        let snapshot = take_snapshot(&mut cursor)?;
        if !cursor.is_empty() {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        let snapshot_head = AccountStateHead {
            receipt_digest: snapshot.receipt_digest,
            state_root: snapshot.state_root,
            freshness: snapshot.freshness,
        };
        if verified_receipt != snapshot_head {
            return Err(ProtocolAdapterError::AccountState(
                AccountStateError::UnverifiedReceipt,
            ));
        }
        if current_head != verified_receipt {
            return Err(ProtocolAdapterError::AccountState(
                AccountStateError::StaleRead,
            ));
        }
        registry.replay_protocol_state(program, &bindings, &routes, lifecycle, &history)?;
        let journal = ProtocolJournal { head: current_head };
        let authority = JournalAccountStateAuthority::new(journal, now, staleness_limit)?;
        let balances = registry.read_value_accounts(program, &snapshot, &authority)?;
        Ok(Self {
            balances,
            snapshot,
            bindings,
            lifecycle,
            routes,
            history,
            account_profile2: None,
        })
    }
}

fn lifecycle_byte(value: ProgramLifecycle) -> u8 {
    match value {
        ProgramLifecycle::Active => 1,
        ProgramLifecycle::Deprecated => 2,
        ProgramLifecycle::Tombstoned => 3,
    }
}

fn put_u16(encoded: &mut Vec<u8>, value: usize) -> Result<(), ProtocolAdapterError> {
    let value = u16::try_from(value).map_err(|_| ProtocolAdapterError::CorruptRecord)?;
    encoded.extend_from_slice(&value.to_be_bytes());
    Ok(())
}

fn put_binding(
    encoded: &mut Vec<u8>,
    value: &ProgramValueAccountBinding,
) -> Result<(), ProtocolAdapterError> {
    encoded.push(value.record_version);
    encoded.extend_from_slice(&value.program.bytes());
    encoded.extend_from_slice(&value.account_id);
    encoded.extend_from_slice(&value.asset_id);
    put_u16(encoded, value.seed.len())?;
    encoded.extend_from_slice(&value.seed);
    encoded.extend_from_slice(&value.registered_sequence.to_be_bytes());
    encoded.extend_from_slice(&value.registration_event_digest);
    Ok(())
}

fn put_bindings(
    encoded: &mut Vec<u8>,
    values: &[ProgramValueAccountBinding],
) -> Result<(), ProtocolAdapterError> {
    if values.len() > MAX_RECORD_ITEMS {
        return Err(ProtocolAdapterError::CorruptRecord);
    }
    put_u16(encoded, values.len())?;
    for value in values {
        put_binding(encoded, value)?;
    }
    Ok(())
}

fn put_routes(encoded: &mut Vec<u8>, values: &[ExitRoute]) -> Result<(), ProtocolAdapterError> {
    if values.len() > MAX_RECORD_ITEMS {
        return Err(ProtocolAdapterError::CorruptRecord);
    }
    put_u16(encoded, values.len())?;
    for value in values {
        encoded.extend_from_slice(&value.account_id);
        encoded.extend_from_slice(&value.asset_id);
        encoded.extend_from_slice(&value.destination);
        put_u16(encoded, value.seed.len())?;
        encoded.extend_from_slice(&value.seed);
    }
    Ok(())
}

fn put_history(
    encoded: &mut Vec<u8>,
    values: &[LifecycleReceipt],
) -> Result<(), ProtocolAdapterError> {
    if values.len() > MAX_RECORD_ITEMS {
        return Err(ProtocolAdapterError::CorruptRecord);
    }
    put_u16(encoded, values.len())?;
    for value in values {
        encoded.push(lifecycle_byte(value.prior));
        encoded.push(lifecycle_byte(value.current));
        encoded.extend_from_slice(&value.authority);
        encoded.extend_from_slice(&value.effective_sequence.to_be_bytes());
        encoded.extend_from_slice(&value.wind_down.exit_program);
        encoded.extend_from_slice(&value.wind_down.deadline.to_be_bytes());
        encoded.extend_from_slice(&value.live_value_accounts.to_be_bytes());
    }
    Ok(())
}

fn put_proof(encoded: &mut Vec<u8>, value: &StateProof) -> Result<(), ProtocolAdapterError> {
    encoded.extend_from_slice(&value.leaf_index.to_be_bytes());
    encoded.extend_from_slice(&value.leaf_count.to_be_bytes());
    if value.siblings.len() > MAX_PROOF_DEPTH {
        return Err(ProtocolAdapterError::CorruptRecord);
    }
    encoded
        .push(u8::try_from(value.siblings.len()).map_err(|_| ProtocolAdapterError::CorruptRecord)?);
    for sibling in &value.siblings {
        encoded.extend_from_slice(sibling);
    }
    Ok(())
}

fn put_snapshot(
    encoded: &mut Vec<u8>,
    value: &VerifiedAccountSnapshot,
) -> Result<(), ProtocolAdapterError> {
    if value.bindings.len() > MAX_RECORD_ITEMS || value.accounts.len() > MAX_RECORD_ITEMS {
        return Err(ProtocolAdapterError::CorruptRecord);
    }
    encoded.extend_from_slice(&value.protocol_version.to_be_bytes());
    encoded.extend_from_slice(&value.receipt_digest);
    encoded.extend_from_slice(&value.state_root);
    encoded.extend_from_slice(&value.universal_root);
    encoded.extend_from_slice(&value.programs_root);
    encoded.extend_from_slice(&value.account_root);
    encoded.extend_from_slice(&value.freshness.observed_sequence.to_be_bytes());
    encoded.extend_from_slice(&value.freshness.observed_at.to_be_bytes());
    put_proof(encoded, &value.account_tree_proof)?;
    put_proof(encoded, &value.universal_root_proof)?;
    put_proof(encoded, &value.programs_root_proof)?;
    put_u16(encoded, value.bindings.len())?;
    for binding in &value.bindings {
        put_binding(encoded, &binding.binding)?;
        put_proof(encoded, &binding.proof)?;
    }
    put_u16(encoded, value.accounts.len())?;
    for account in &value.accounts {
        encoded.extend_from_slice(&account.leaf.account_id);
        put_u16(encoded, account.leaf.name.len())?;
        encoded.extend_from_slice(&account.leaf.name);
        encoded.push(account.leaf.kind);
        encoded.extend_from_slice(&account.leaf.balance.to_be_bytes());
        encoded.extend_from_slice(&account.leaf.asset_id);
        encoded.push(u8::from(account.leaf.has_asset));
        encoded.extend_from_slice(&account.leaf.next_sequence.to_be_bytes());
        encoded.extend_from_slice(&account.leaf.created_at_sequence.to_be_bytes());
        encoded.push(u8::from(account.leaf.frozen));
        encoded.push(u8::from(account.leaf.has_open_reference));
        encoded.extend_from_slice(&account.leaf.authority_key);
        encoded.push(u8::from(account.leaf.has_authority_key));
        put_proof(encoded, &account.proof)?;
    }
    Ok(())
}

struct Cursor<'a> {
    remaining: &'a [u8],
}

impl<'a> Cursor<'a> {
    const fn new(remaining: &'a [u8]) -> Self {
        Self { remaining }
    }

    fn take(&mut self, length: usize) -> Result<&'a [u8], ProtocolAdapterError> {
        let (value, remaining) = self
            .remaining
            .split_at_checked(length)
            .ok_or(ProtocolAdapterError::CorruptRecord)?;
        self.remaining = remaining;
        Ok(value)
    }

    fn array<const N: usize>(&mut self) -> Result<[u8; N], ProtocolAdapterError> {
        self.take(N)?
            .try_into()
            .map_err(|_| ProtocolAdapterError::CorruptRecord)
    }

    fn byte(&mut self) -> Result<u8, ProtocolAdapterError> {
        Ok(self.take(1)?[0])
    }

    fn u16(&mut self) -> Result<u16, ProtocolAdapterError> {
        Ok(u16::from_be_bytes(self.array()?))
    }

    fn u32(&mut self) -> Result<u32, ProtocolAdapterError> {
        Ok(u32::from_be_bytes(self.array()?))
    }

    fn u64(&mut self) -> Result<u64, ProtocolAdapterError> {
        Ok(u64::from_be_bytes(self.array()?))
    }

    const fn is_empty(&self) -> bool {
        self.remaining.is_empty()
    }
}

fn take_count(cursor: &mut Cursor<'_>) -> Result<usize, ProtocolAdapterError> {
    let count = usize::from(cursor.u16()?);
    if count > MAX_RECORD_ITEMS {
        return Err(ProtocolAdapterError::CorruptRecord);
    }
    Ok(count)
}

fn take_binding(
    cursor: &mut Cursor<'_>,
) -> Result<ProgramValueAccountBinding, ProtocolAdapterError> {
    let record_version = cursor.byte()?;
    let program =
        ProgramId::new(cursor.array()?).map_err(|_| ProtocolAdapterError::CorruptRecord)?;
    let account_id = cursor.array()?;
    let asset_id = cursor.array()?;
    let seed_length = usize::from(cursor.u16()?);
    if seed_length > MAX_SEED_BYTES {
        return Err(ProtocolAdapterError::CorruptRecord);
    }
    let seed = cursor.take(seed_length)?.to_vec();
    let registered_sequence = cursor.u64()?;
    let registration_event_digest = cursor.array()?;
    Ok(ProgramValueAccountBinding {
        record_version,
        program,
        account_id,
        seed,
        asset_id,
        registered_sequence,
        registration_event_digest,
    })
}

fn take_bindings(
    cursor: &mut Cursor<'_>,
    program: ProgramId,
) -> Result<Vec<ProgramValueAccountBinding>, ProtocolAdapterError> {
    let count = take_count(cursor)?;
    let mut values = Vec::with_capacity(count);
    for _ in 0..count {
        let value = take_binding(cursor)?;
        if value.program != program {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        values.push(value);
    }
    Ok(values)
}

fn take_routes(cursor: &mut Cursor<'_>) -> Result<Vec<ExitRoute>, ProtocolAdapterError> {
    let count = take_count(cursor)?;
    let mut values = Vec::with_capacity(count);
    for _ in 0..count {
        let account_id = cursor.array()?;
        let asset_id = cursor.array()?;
        let destination = cursor.array()?;
        let seed_length = usize::from(cursor.u16()?);
        if seed_length > MAX_SEED_BYTES {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        values.push(ExitRoute {
            seed: cursor.take(seed_length)?.to_vec(),
            account_id,
            asset_id,
            destination,
        });
    }
    Ok(values)
}

fn take_history(
    cursor: &mut Cursor<'_>,
    program: ProgramId,
) -> Result<Vec<LifecycleReceipt>, ProtocolAdapterError> {
    let count = take_count(cursor)?;
    let mut values = Vec::with_capacity(count);
    for _ in 0..count {
        values.push(LifecycleReceipt {
            program,
            prior: lifecycle(i32::from(cursor.byte()?))?,
            current: lifecycle(i32::from(cursor.byte()?))?,
            authority: cursor.array()?,
            effective_sequence: cursor.u64()?,
            wind_down: WindDownPolicy {
                exit_program: cursor.array()?,
                deadline: cursor.u64()?,
                state_access: WindDownStateAccess::ReadOnly,
            },
            live_value_accounts: cursor.u32()?,
        });
    }
    Ok(values)
}

fn take_proof(cursor: &mut Cursor<'_>) -> Result<StateProof, ProtocolAdapterError> {
    let leaf_index = cursor.u32()?;
    let leaf_count = cursor.u32()?;
    let depth = usize::from(cursor.byte()?);
    if depth > MAX_PROOF_DEPTH {
        return Err(ProtocolAdapterError::CorruptRecord);
    }
    let mut siblings = Vec::with_capacity(depth);
    for _ in 0..depth {
        siblings.push(cursor.array()?);
    }
    Ok(StateProof {
        leaf_index,
        leaf_count,
        siblings,
    })
}

fn take_snapshot(cursor: &mut Cursor<'_>) -> Result<VerifiedAccountSnapshot, ProtocolAdapterError> {
    let protocol_version = cursor.u16()?;
    let receipt_digest = cursor.array()?;
    let state_root = cursor.array()?;
    let universal_root = cursor.array()?;
    let programs_root = cursor.array()?;
    let account_root = cursor.array()?;
    let freshness = ReadFreshness {
        observed_sequence: cursor.u64()?,
        observed_at: cursor.u64()?,
    };
    let account_tree_proof = take_proof(cursor)?;
    let universal_root_proof = take_proof(cursor)?;
    let programs_root_proof = take_proof(cursor)?;
    let binding_count = take_count(cursor)?;
    let mut bindings = Vec::with_capacity(binding_count);
    for _ in 0..binding_count {
        bindings.push(ProvenProgramBinding {
            binding: take_binding(cursor)?,
            proof: take_proof(cursor)?,
        });
    }
    let account_count = take_count(cursor)?;
    let mut accounts = Vec::with_capacity(account_count);
    for _ in 0..account_count {
        let account_id = cursor.array()?;
        let name_length = usize::from(cursor.u16()?);
        if name_length > MAX_ACCOUNT_NAME_BYTES {
            return Err(ProtocolAdapterError::CorruptRecord);
        }
        accounts.push(ProvenAccountLeaf {
            leaf: CanonicalAccountLeaf {
                account_id,
                name: cursor.take(name_length)?.to_vec(),
                kind: cursor.byte()?,
                balance: u128::from_be_bytes(cursor.array()?),
                asset_id: cursor.array()?,
                has_asset: (take_bool(cursor)?).into(),
                next_sequence: cursor.u64()?,
                created_at_sequence: cursor.u64()?,
                frozen: (take_bool(cursor)?).into(),
                has_open_reference: (take_bool(cursor)?).into(),
                authority_key: cursor.array()?,
                has_authority_key: (take_bool(cursor)?).into(),
            },
            proof: take_proof(cursor)?,
        });
    }
    Ok(VerifiedAccountSnapshot {
        protocol_version,
        receipt_digest,
        state_root,
        universal_root,
        programs_root,
        account_root,
        account_tree_proof,
        universal_root_proof,
        programs_root_proof,
        freshness,
        bindings,
        accounts,
    })
}

fn take_bool(cursor: &mut Cursor<'_>) -> Result<bool, ProtocolAdapterError> {
    match cursor.byte()? {
        0 => Ok(false),
        1 => Ok(true),
        _ => Err(ProtocolAdapterError::CorruptRecord),
    }
}

#[derive(Clone, Copy)]
struct ProtocolJournal {
    head: AccountStateHead,
}

impl AccountStateJournal for ProtocolJournal {
    fn account_state_head(
        &self,
        receipt_digest: [u8; 32],
    ) -> Result<AccountStateHead, AccountStateError> {
        if receipt_digest == self.head.receipt_digest {
            Ok(self.head)
        } else {
            Err(AccountStateError::UnverifiedReceipt)
        }
    }

    fn current_account_state_head(&self) -> Result<AccountStateHead, AccountStateError> {
        Ok(self.head)
    }
}

pub fn lifecycle(value: i32) -> Result<ProgramLifecycle, ProtocolAdapterError> {
    match value {
        1 => Ok(ProgramLifecycle::Active),
        2 => Ok(ProgramLifecycle::Deprecated),
        3 => Ok(ProgramLifecycle::Tombstoned),
        _ => Err(ProtocolAdapterError::NonCanonicalView),
    }
}

pub fn restore_verified(
    bytes: &[u8],
    registry: &mut Registry,
    verified_receipt: AccountStateHead,
    current_head: AccountStateHead,
    now: u64,
    staleness_limit: u64,
) -> Result<ProtocolProgramStateRead, ProtocolAdapterError> {
    ProtocolProgramStateRead::restore_verified(
        bytes,
        registry,
        verified_receipt,
        current_head,
        now,
        staleness_limit,
    )
}

pub fn restore_verified_profile2(
    bytes: &[u8],
    registry: &mut Registry,
    verified_receipt: AccountStateHead,
    current_head: AccountStateHead,
    now: u64,
    staleness_limit: u64,
) -> Result<ProtocolProgramStateRead, ProtocolAdapterError> {
    ProtocolProgramStateRead::restore_verified_profile2(
        bytes,
        registry,
        verified_receipt,
        current_head,
        now,
        staleness_limit,
    )
}
