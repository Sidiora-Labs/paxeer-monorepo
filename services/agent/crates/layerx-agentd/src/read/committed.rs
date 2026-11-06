//! Precompile calls and storage witnesses bound to one authenticated committed block.

use super::{CheckpointReadError, ServedCheckpoint};
use layerx_client::rpc::account::Freshness;
use layerx_client::sequencer::{SequencerEndpoint, SequencerError, StorageWitness};
use layerx_programs::hex;
use layerx_proof::state::{AccountAsset, CanonicalAccount};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use std::collections::BTreeMap;

#[derive(Clone, Debug)]
pub struct CommittedAccount {
    pub account_id: [u8; 32],
    pub name: Vec<u8>,
    pub kind: u8,
    pub asset: Option<AccountAsset>,
    pub next_sequence: u64,
    pub frozen: bool,
    pub authority_key: Option<[u8; 32]>,
}
impl CommittedAccount {
    pub fn has_asset(&self) -> bool {
        self.asset.is_some()
    }
    pub fn asset_id(&self) -> [u8; 32] {
        self.asset.map_or([0; 32], |asset| asset.asset_id)
    }
    pub fn balance(&self) -> u128 {
        self.asset.map_or(0, |asset| asset.balance)
    }
}
pub(crate) trait SourceAccount {
    fn account_id(&self) -> [u8; 32];
    fn name(&self) -> &[u8];
    fn kind(&self) -> u8;
    fn asset_id(&self) -> [u8; 32];
    fn has_asset(&self) -> bool;
    fn frozen(&self) -> bool;
}
macro_rules! source_account {
    ($account:ty) => {
        impl SourceAccount for $account {
            fn account_id(&self) -> [u8; 32] {
                self.account_id
            }
            fn name(&self) -> &[u8] {
                &self.name
            }
            fn kind(&self) -> u8 {
                self.kind
            }
            fn asset_id(&self) -> [u8; 32] {
                self.asset_id()
            }
            fn has_asset(&self) -> bool {
                self.has_asset()
            }
            fn frozen(&self) -> bool {
                self.frozen
            }
        }
    };
}
source_account!(CommittedAccount);
source_account!(CanonicalAccount);
#[cfg(test)]
impl From<CanonicalAccount> for CommittedAccount {
    fn from(value: CanonicalAccount) -> Self {
        Self {
            account_id: value.account_id,
            name: value.name,
            kind: value.kind,
            asset: value.asset,
            next_sequence: value.next_sequence,
            frozen: value.frozen,
            authority_key: value.authority_key,
        }
    }
}

#[derive(Clone, Debug)]
pub struct CommittedCaps {
    did: [u8; 32],
    checkpoint: ServedCheckpoint,
    accounts: BTreeMap<[u8; 32], CommittedAccount>,
    assets: BTreeMap<[u8; 32], CommittedAsset>,
    budgets: Vec<crate::budget::ProtocolBudgetRecord>,
    budget_funding: BTreeMap<[u8; 32], CommittedBudgetFunding>,
    grants: Vec<CommittedGrant>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedBudgetFunding {
    pub budget_id: [u8; 32],
    pub account_id: [u8; 32],
    pub asset_id: [u8; 32],
    allowance: u128,
    balance: u128,
    remaining: u128,
}

impl CommittedBudgetFunding {
    /// Checks the native budget view's funded amount against the independent
    /// ledger balance. Authentication of both reads belongs to CommittedReads.
    pub fn verify_values(
        record: &crate::budget::ProtocolBudgetRecord,
        balance: u128,
        served_remaining: u128,
    ) -> Result<Self, CommittedReadError> {
        let allowance = record.remaining();
        let remaining = allowance.min(balance);
        if served_remaining != remaining {
            return Err(CommittedReadError::Selector);
        }
        Ok(Self {
            budget_id: record.budget_id,
            account_id: record.budget_account,
            asset_id: record.asset_id,
            allowance,
            balance,
            remaining,
        })
    }
    pub const fn allowance(&self) -> u128 {
        self.allowance
    }
    pub const fn funded_balance(&self) -> u128 {
        self.balance
    }
    pub const fn remaining(&self) -> u128 {
        self.remaining
    }
}

#[derive(Clone, Debug)]
pub struct CommittedGrant {
    pub id: [u8; 32],
    pub encoding: Vec<u8>,
    pub revoked: bool,
    pub revoked_at: u64,
    pub module_mask: u64,
    pub ordinal_min: u16,
    pub ordinal_max: u16,
    pub asset: [u8; 32],
    pub maximum_per_activity: u128,
    pub maximum_total: u128,
    pub spent_total: u128,
    pub period_length: u64,
    pub maximum_per_period: u128,
    pub spent_this_period: u128,
    pub period_start: u64,
    pub purpose: [u8; 32],
}

impl CommittedCaps {
    pub const fn did(&self) -> [u8; 32] {
        self.did
    }
    pub const fn state_root(&self) -> [u8; 32] {
        self.checkpoint.app_hash
    }
    pub const fn observed_height(&self) -> u64 {
        self.checkpoint.height
    }
    pub const fn level(&self) -> VerificationLevel {
        VerificationLevel::CHECKPOINT_FINALISED
    }
    pub const fn freshness(&self) -> Freshness {
        checkpoint_freshness(&self.checkpoint)
    }
    pub fn all_accounts(&self) -> &BTreeMap<[u8; 32], CommittedAccount> {
        &self.accounts
    }
    pub fn budgets(&self) -> &[crate::budget::ProtocolBudgetRecord] {
        &self.budgets
    }
    pub fn budget_funding(
        &self,
        id: &[u8; 32],
    ) -> Result<&CommittedBudgetFunding, CommittedReadError> {
        self.budget_funding
            .get(id)
            .ok_or(CommittedReadError::Selector)
    }

    /// Selects only funded spending power at the inventory's held checkpoint.
    pub fn budget_source(
        &self,
        id: &[u8; 32],
        asset: [u8; 32],
        amount: u128,
    ) -> Result<&CommittedBudgetFunding, CommittedReadError> {
        let record = self
            .budgets
            .iter()
            .find(|record| &record.budget_id == id)
            .ok_or(CommittedReadError::Selector)?;
        let funding = self.budget_funding(id)?;
        if record.owner != self.did
            || record.asset_id != asset
            || record.closed
            || record.revoked
            || record.expiry != 0
                && self
                    .checkpoint
                    .header
                    .time
                    .seconds
                    .checked_mul(1000)
                    .and_then(|ms| {
                        ms.checked_add(i64::from(self.checkpoint.header.time.nanos / 1_000_000))
                    })
                    .and_then(|ms| u64::try_from(ms).ok())
                    .is_none_or(|ms| ms >= record.expiry)
            || amount == 0
            || amount > funding.remaining()
        {
            return Err(CommittedReadError::Selector);
        }
        Ok(funding)
    }
    pub fn grants(&self) -> &[CommittedGrant] {
        &self.grants
    }
    pub fn effective_asset(&self, asset: [u8; 32]) -> Result<CommittedAsset, CommittedReadError> {
        self.assets
            .get(&asset)
            .cloned()
            .ok_or(CommittedReadError::Selector)
    }
    pub fn issuance(&self, asset: [u8; 32]) -> Result<CommittedIssuance, CommittedReadError> {
        let value = self.effective_asset(asset)?;
        let metadata = value.metadata();
        if metadata.issuer_did != self.did || metadata.paused {
            return Err(CommittedReadError::Selector);
        }
        let ceiling = if metadata.supply_cap == 0 {
            u128::MAX
        } else {
            metadata.supply_cap
        };
        Ok(CommittedIssuance {
            account: layerx_wire::hash::asset_issuance_account_id(&asset)
                .map_err(|_| CommittedReadError::Selector)?,
            headroom: ceiling
                .checked_sub(metadata.total_units)
                .ok_or(CommittedReadError::Selector)?,
        })
    }
}

pub struct CommittedIssuance {
    account: [u8; 32],
    headroom: u128,
}
impl CommittedIssuance {
    pub const fn account(&self) -> [u8; 32] {
        self.account
    }
    pub const fn headroom(&self) -> u128 {
        self.headroom
    }
}

#[derive(Clone, Debug)]
pub struct CommittedAsset {
    metadata: layerx_client::rpc::payments::AssetMetadata,
    call: CommittedCall,
}
impl CommittedAsset {
    pub fn metadata(&self) -> &layerx_client::rpc::payments::AssetMetadata {
        &self.metadata
    }
    pub const fn registered(&self) -> bool {
        true
    }
    pub const fn paused(&self) -> bool {
        self.metadata.paused
    }
    pub const fn state_root(&self) -> [u8; 32] {
        self.call.state_root()
    }
    pub const fn level(&self) -> VerificationLevel {
        self.call.level()
    }
    pub const fn freshness(&self) -> Freshness {
        self.call.freshness()
    }
}

const fn checkpoint_freshness(checkpoint: &ServedCheckpoint) -> Freshness {
    Freshness {
        global_sequence: checkpoint.height,
        batch_number: checkpoint.height,
        observed_head_sequence: checkpoint.height,
        observed_checkpoint: checkpoint.block_hash,
    }
}

fn precompile(last: u8) -> [u8; 20] {
    let mut address = [0; 20];
    address[18] = 0x10;
    address[19] = last;
    address
}
fn calldata(signature: &[u8], arguments: &[u8]) -> Vec<u8> {
    let mut data = layerx_crypto::evm_transaction::transaction_hash(signature)[..4].to_vec();
    data.extend(arguments);
    data
}
fn word(bytes: &[u8], index: usize) -> Result<[u8; 32], CommittedReadError> {
    let start = index.checked_mul(32).ok_or(CommittedReadError::Selector)?;
    bytes
        .get(start..start + 32)
        .and_then(|value| value.try_into().ok())
        .ok_or(CommittedReadError::Selector)
}
fn uint(bytes: &[u8], index: usize, width: usize) -> Result<u128, CommittedReadError> {
    let value = word(bytes, index)?;
    if width == 0 || width > 16 || value[..32 - width].iter().any(|byte| *byte != 0) {
        return Err(CommittedReadError::Selector);
    }
    let mut output = [0; 16];
    output[16 - width..].copy_from_slice(&value[32 - width..]);
    Ok(u128::from_be_bytes(output))
}
fn dynamic(
    bytes: &[u8],
    index: usize,
    head: usize,
    maximum: usize,
) -> Result<Vec<u8>, CommittedReadError> {
    let offset =
        usize::try_from(uint(bytes, index, 8)?).map_err(|_| CommittedReadError::Selector)?;
    if offset < head * 32 || offset % 32 != 0 {
        return Err(CommittedReadError::Selector);
    }
    let length =
        usize::try_from(uint(bytes, offset / 32, 8)?).map_err(|_| CommittedReadError::Selector)?;
    if length > maximum {
        return Err(CommittedReadError::Selector);
    }
    let start = offset.checked_add(32).ok_or(CommittedReadError::Selector)?;
    let end = start
        .checked_add(length)
        .ok_or(CommittedReadError::Selector)?;
    let padded = start
        .checked_add(length.div_ceil(32) * 32)
        .ok_or(CommittedReadError::Selector)?;
    if bytes
        .get(end..padded)
        .is_none_or(|tail| tail.iter().any(|byte| *byte != 0))
    {
        return Err(CommittedReadError::Selector);
    }
    bytes
        .get(start..end)
        .map(|value| value.to_vec())
        .ok_or(CommittedReadError::Selector)
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum CommittedReadError {
    Rpc(SequencerError),
    Checkpoint(CheckpointReadError),
    Selector,
    CommittedChanged,
    Storage,
}

impl CommittedReadError {
    pub const fn code(&self) -> &'static str {
        match self {
            Self::CommittedChanged => "read.committed_changed",
            Self::Rpc(_) => "read.unavailable",
            Self::Checkpoint(_) => "read.checkpoint",
            Self::Selector => "read.selector",
            Self::Storage => "read.storage",
        }
    }
}

/// An operation owns one checkpoint; every subsequent read uses its height.
pub struct CommittedReads<'a> {
    endpoint: &'a SequencerEndpoint,
    checkpoint: ServedCheckpoint,
}

/// Exact call bytes, including the independent re-read and their committed coordinates.
/// This is deliberately distinct from the retired signed-batch capability inventory.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct CommittedCall {
    checkpoint: ServedCheckpoint,
    canonical_bytes: Vec<u8>,
    repeated: Vec<u8>,
    precompile: [u8; 20],
    calldata: Vec<u8>,
}

impl CommittedCall {
    pub fn precompile(&self) -> [u8; 20] {
        self.precompile
    }
    pub fn calldata(&self) -> &[u8] {
        &self.calldata
    }
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.canonical_bytes
    }
    pub fn repeated(&self) -> &[u8] {
        &self.repeated
    }
    pub fn checkpoint(&self) -> &ServedCheckpoint {
        &self.checkpoint
    }
    pub const fn state_root(&self) -> [u8; 32] {
        self.checkpoint.app_hash
    }
    pub const fn level(&self) -> VerificationLevel {
        VerificationLevel::CHECKPOINT_FINALISED
    }
    pub const fn achieved(&self) -> VerificationLevel {
        self.level()
    }
    pub const fn freshness(&self) -> Freshness {
        Freshness {
            global_sequence: self.checkpoint.height,
            batch_number: self.checkpoint.height,
            observed_head_sequence: self.checkpoint.height,
            observed_checkpoint: self.checkpoint.block_hash,
        }
    }
}

impl<'a> CommittedReads<'a> {
    pub fn effective_asset(&self, asset: [u8; 32]) -> Result<CommittedAsset, CommittedReadError> {
        let call = self.call(precompile(0x1a), &calldata(b"getAsset(bytes32)", &asset))?;
        let bytes = call.canonical_bytes();
        let paused = uint(bytes, 10, 1)?;
        let metadata = layerx_client::rpc::payments::AssetMetadata {
            asset_id: word(bytes, 0)?,
            symbol: dynamic(bytes, 1, 12, 16)?,
            name: String::from_utf8(dynamic(bytes, 2, 12, 32)?)
                .map_err(|_| CommittedReadError::Selector)?,
            decimals: uint(bytes, 3, 1)? as u8,
            supply_cap: uint(bytes, 4, 16)?,
            issuer_did: word(bytes, 5)?,
            salt: word(bytes, 6)?,
            issuer_kind: uint(bytes, 7, 1)? as u8,
            custody_kind: uint(bytes, 8, 1)? as u8,
            custody_reference: dynamic(bytes, 9, 12, 128)?,
            paused: paused == 1,
            total_units: uint(bytes, 11, 16)?,
        };
        if metadata.asset_id != asset
            || asset == [0; 32]
            || paused > 1
            || metadata.decimals > 38
            || metadata.symbol.is_empty()
            || !metadata.symbol.is_ascii()
            || metadata.issuer_kind > 2
        {
            return Err(CommittedReadError::Selector);
        }
        Ok(CommittedAsset { metadata, call })
    }

    pub fn inventory(
        &self,
        did: &Did,
        assets: &[[u8; 32]],
        fee_asset: [u8; 32],
    ) -> Result<CommittedCaps, CommittedReadError> {
        let text = std::str::from_utf8(did.as_bytes()).map_err(|_| CommittedReadError::Selector)?;
        let public = text
            .strip_prefix("did:layerx:")
            .ok_or(CommittedReadError::Selector)?;
        if public.len() != 64
            || !public
                .bytes()
                .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
        {
            return Err(CommittedReadError::Selector);
        }
        let public: [u8; 32] = hex::decode(public)
            .map_err(|_| CommittedReadError::Selector)?
            .try_into()
            .map_err(|_| CommittedReadError::Selector)?;
        let owner = layerx_wire::hash::did_id_for_protocol(did, 3)
            .map_err(|_| CommittedReadError::Selector)?;
        let addr = layerx_client::sequencer::ADDR_PRECOMPILE;
        let address = self.call(addr, &calldata(b"getEvmAddrByLayerX(bytes32)", &public))?;
        let address = word(address.canonical_bytes(), 0)?;
        if address[..12] != [0; 12] || address[12..] == [0; 20] {
            return Err(CommittedReadError::Selector);
        }
        let unified = self.call(addr, &calldata(b"getUnifiedAccount(address)", &address))?;
        if word(unified.canonical_bytes(), 0)? != address
            || word(unified.canonical_bytes(), 2)? != public
        {
            return Err(CommittedReadError::Selector);
        }
        let main = word(unified.canonical_bytes(), 3)?;
        let binding = self.call(addr, &calldata(b"getLayerXDid(address)", &address))?;
        if word(binding.canonical_bytes(), 0)? != public
            || dynamic(binding.canonical_bytes(), 1, 2, 512)? != did.as_bytes()
        {
            return Err(CommittedReadError::Selector);
        }
        let mut inventory = CommittedCaps {
            did: owner,
            checkpoint: self.checkpoint.clone(),
            accounts: BTreeMap::new(),
            assets: BTreeMap::new(),
            budgets: Vec::new(),
            budget_funding: BTreeMap::new(),
            grants: Vec::new(),
        };
        let selected: std::collections::BTreeSet<_> =
            assets.iter().copied().chain([fee_asset]).collect();
        if selected.len() > 4096 || selected.contains(&[0; 32]) {
            return Err(CommittedReadError::Selector);
        }
        for asset in selected {
            let metadata = self.effective_asset(asset)?;
            let account = layerx_types::account::AccountId::for_asset(text, asset, fee_asset)
                .map_err(|_| CommittedReadError::Selector)?;
            let account_id = layerx_wire::hash::account_id_for_protocol(&account, 3)
                .map_err(|_| CommittedReadError::Selector)?;
            if asset == fee_asset && account_id != main {
                return Err(CommittedReadError::Selector);
            }
            let returned = self.call(
                precompile(0x1a),
                &calldata(b"accountId(bytes32,bytes32)", &[owner, asset].concat()),
            )?;
            if returned.canonical_bytes() != account_id {
                return Err(CommittedReadError::Selector);
            }
            let returned = self.call(
                precompile(0x1a),
                &calldata(b"accountOwner(bytes32)", &account_id),
            )?;
            if returned.canonical_bytes() != owner {
                return Err(CommittedReadError::Selector);
            }
            let balance = self.call(
                precompile(0x1a),
                &calldata(b"balanceOf(bytes32,bytes32)", &[account_id, asset].concat()),
            )?;
            let sequence = self.call(
                precompile(0x1a),
                &calldata(
                    b"nextSequence(bytes32,bytes32)",
                    &[account_id, asset].concat(),
                ),
            )?;
            inventory.accounts.insert(
                account_id,
                CommittedAccount {
                    account_id,
                    name: account.canonical().as_bytes().to_vec(),
                    kind: if asset == fee_asset { 1 } else { 14 },
                    asset: Some(AccountAsset {
                        asset_id: asset,
                        balance: uint(balance.canonical_bytes(), 0, 16)?,
                    }),
                    next_sequence: uint(sequence.canonical_bytes(), 0, 8)? as u64,
                    frozen: metadata.paused(),
                    authority_key: Some(public),
                },
            );
            inventory.assets.insert(asset, metadata);
        }
        Ok(inventory)
    }

    pub fn inventory_records(
        &self,
        inventory: &mut CommittedCaps,
        budgets: &[[u8; 32]],
        grants: &[[u8; 32]],
    ) -> Result<(), CommittedReadError> {
        if inventory.checkpoint != self.checkpoint || budgets.len() > 4096 || grants.len() > 4096 {
            return Err(CommittedReadError::Selector);
        }
        let mut selected_budgets = Vec::new();
        let mut selected_funding = BTreeMap::new();
        for id in budgets {
            let value =
                self.budget_state(precompile(0x1b), &calldata(b"getBudget(bytes32)", id))?;
            let state = crate::budget::ProtocolBudgetState::decode_get_budget(
                *id,
                self.checkpoint.height,
                value.canonical_bytes(),
            )
            .map_err(|_| CommittedReadError::Selector)?;
            let record = state.record();
            let remaining =
                self.budget_state(precompile(0x1b), &calldata(b"budgetRemaining(bytes32)", id))?;
            let balance = self.call(
                precompile(0x1a),
                &calldata(
                    b"balanceOf(bytes32,bytes32)",
                    &[record.budget_account, record.asset_id].concat(),
                ),
            )?;
            if balance.canonical_bytes().len() != 32 {
                return Err(CommittedReadError::Selector);
            }
            let balance = uint(balance.canonical_bytes(), 0, 16)?;
            if remaining.canonical_bytes().len() != 32
                || record.owner != inventory.did
                || selected_budgets
                    .iter()
                    .any(|previous: &crate::budget::ProtocolBudgetRecord| previous.budget_id == *id)
            {
                return Err(CommittedReadError::Selector);
            }
            let funding = CommittedBudgetFunding::verify_values(
                record,
                balance,
                uint(remaining.canonical_bytes(), 0, 16)?,
            )?;
            selected_funding.insert(*id, funding);
            selected_budgets.push(record.clone());
        }
        let mut selected_grants = Vec::new();
        for id in grants {
            let value = self.call(precompile(0x1c), &calldata(b"getGrant(bytes32)", id))?;
            let bytes = value.canonical_bytes();
            let revoked = uint(bytes, 1, 1)?;
            let encoding = dynamic(bytes, 0, 3, 1_048_576)?;
            if encoding.is_empty()
                || revoked > 1
                || selected_grants
                    .iter()
                    .any(|previous: &CommittedGrant| previous.id == *id)
            {
                return Err(CommittedReadError::Selector);
            }
            let scope = self.call(precompile(0x1c), &calldata(b"capabilityScope(bytes32)", id))?;
            let scope = scope.canonical_bytes();
            if scope.len() != 384 {
                return Err(CommittedReadError::Selector);
            }
            let grant = CommittedGrant {
                id: *id,
                encoding,
                revoked: revoked == 1,
                revoked_at: uint(bytes, 2, 8)? as u64,
                module_mask: uint(scope, 0, 8)? as u64,
                ordinal_min: uint(scope, 1, 2)? as u16,
                ordinal_max: uint(scope, 2, 2)? as u16,
                asset: word(scope, 3)?,
                maximum_per_activity: uint(scope, 4, 16)?,
                maximum_total: uint(scope, 5, 16)?,
                spent_total: uint(scope, 6, 16)?,
                period_length: uint(scope, 7, 8)? as u64,
                maximum_per_period: uint(scope, 8, 16)?,
                spent_this_period: uint(scope, 9, 16)?,
                period_start: uint(scope, 10, 8)? as u64,
                purpose: word(scope, 11)?,
            };
            if grant.ordinal_min > grant.ordinal_max {
                return Err(CommittedReadError::Selector);
            }
            selected_grants.push(grant);
        }
        inventory.budgets = selected_budgets;
        inventory.budget_funding = selected_funding;
        inventory.grants = selected_grants;
        Ok(())
    }

    pub fn program_accounts(
        &self,
        inventory: &mut CommittedCaps,
        program: [u8; 32],
        accounts: &[[u8; 32]],
    ) -> Result<(), CommittedReadError> {
        if inventory.checkpoint != self.checkpoint {
            return Err(CommittedReadError::Selector);
        }
        let records = crate::budget::execution_state::read_program_records(
            self.endpoint,
            &self.checkpoint,
            program,
            accounts,
        )
        .map_err(|_| CommittedReadError::Selector)?;
        for (id, value) in records.accounts {
            let sequence = self.call(
                precompile(0x1a),
                &calldata(
                    b"nextSequence(bytes32,bytes32)",
                    &[id, value.asset].concat(),
                ),
            )?;
            let account = CommittedAccount {
                account_id: id,
                name: format!("module:programs:value:{}", hex::encode(&id)).into_bytes(),
                kind: 13,
                asset: Some(AccountAsset {
                    asset_id: value.asset,
                    balance: value.balance,
                }),
                next_sequence: uint(sequence.canonical_bytes(), 0, 8)? as u64,
                frozen: value.frozen,
                authority_key: None,
            };
            if inventory.accounts.insert(id, account).is_some() {
                return Err(CommittedReadError::Selector);
            }
        }
        Ok(())
    }
    /// Resumes from the saved checkpoint, reacquiring its consensus material
    /// and state root from the node before any committed read is served.
    pub fn resume(
        endpoint: &'a SequencerEndpoint,
        checkpoint: &ServedCheckpoint,
    ) -> Result<Self, CommittedReadError> {
        Ok(Self {
            endpoint,
            checkpoint: checkpoint
                .reacquire(endpoint)
                .map_err(CommittedReadError::Checkpoint)?,
        })
    }

    pub fn open(endpoint: &'a SequencerEndpoint, height: u64) -> Result<Self, CommittedReadError> {
        let chain_id = endpoint
            .chain_id()
            .map_err(CommittedReadError::Rpc)?
            .to_string();
        let block_hash = endpoint
            .block_by_number(height)
            .map_err(CommittedReadError::Rpc)?
            .ok_or(CommittedReadError::CommittedChanged)?
            .hash;
        let checkpoint = ServedCheckpoint::acquire_at_committed_block(
            endpoint,
            height,
            Some(block_hash),
            &chain_id,
        )
        .map_err(CommittedReadError::Checkpoint)?;
        Ok(Self {
            endpoint,
            checkpoint,
        })
    }

    /// Reuses the checkpoint already held by the operation instead of selecting another head.
    pub fn at_checkpoint(
        endpoint: &'a SequencerEndpoint,
        checkpoint: &ServedCheckpoint,
    ) -> Result<Self, CommittedReadError> {
        let verified = super::checkpoint(
            checkpoint.height,
            &checkpoint.header.chain_id,
            &checkpoint.header,
            &checkpoint.commit,
            &checkpoint.validators,
        )
        .map_err(CommittedReadError::Checkpoint)?;
        if verified != *checkpoint {
            return Err(CommittedReadError::CommittedChanged);
        }
        Ok(Self {
            endpoint,
            checkpoint: verified,
        })
    }

    pub fn checkpoint(&self) -> &ServedCheckpoint {
        &self.checkpoint
    }

    pub fn capabilities(
        &self,
        precompile: [u8; 20],
        calldata: &[u8],
    ) -> Result<CommittedCall, CommittedReadError> {
        self.call(precompile, calldata)
    }

    pub fn budget_state(
        &self,
        precompile: [u8; 20],
        calldata: &[u8],
    ) -> Result<CommittedCall, CommittedReadError> {
        self.call(precompile, calldata)
    }

    pub fn program_exists(&self, program: [u8; 32]) -> Result<bool, CommittedReadError> {
        let data = calldata(b"getProgram(bytes32)", &program);
        let first = self
            .endpoint
            .call_at(self.checkpoint.height, &precompile(0x1d), &data);
        let second = self
            .endpoint
            .call_at(self.checkpoint.height, &precompile(0x1d), &data);
        if first != second {
            return Err(CommittedReadError::CommittedChanged);
        }
        self.unchanged()?;
        match first {
            Ok(bytes) if bytes.len() == 192 => Ok(true),
            Err(SequencerError::Rpc(refusal))
                if refusal.message.contains("LXP_NOT_FOUND: getProgram:") =>
            {
                Ok(false)
            }
            Err(error) => Err(CommittedReadError::Rpc(error)),
            _ => Err(CommittedReadError::Selector),
        }
    }

    fn unchanged(&self) -> Result<(), CommittedReadError> {
        let block = self
            .endpoint
            .block_by_number(self.checkpoint.height)
            .map_err(CommittedReadError::Rpc)?
            .ok_or(CommittedReadError::CommittedChanged)?;
        let app_hash = self
            .endpoint
            .state_root(self.checkpoint.height)
            .map_err(CommittedReadError::Rpc)?;
        if block.number != self.checkpoint.height
            || block.hash != self.checkpoint.block_hash
            || app_hash != self.checkpoint.app_hash
        {
            return Err(CommittedReadError::CommittedChanged);
        }
        Ok(())
    }

    fn call(
        &self,
        precompile: [u8; 20],
        calldata: &[u8],
    ) -> Result<CommittedCall, CommittedReadError> {
        if calldata.len() < 4 {
            return Err(CommittedReadError::Selector);
        }
        let canonical_bytes = self
            .endpoint
            .call_at(self.checkpoint.height, &precompile, calldata)
            .map_err(CommittedReadError::Rpc)?;
        let repeated = self
            .endpoint
            .call_at(self.checkpoint.height, &precompile, calldata)
            .map_err(CommittedReadError::Rpc)?;
        if canonical_bytes != repeated {
            return Err(CommittedReadError::CommittedChanged);
        }
        self.unchanged()?;
        Ok(CommittedCall {
            checkpoint: self.checkpoint.clone(),
            canonical_bytes,
            repeated,
            precompile,
            calldata: calldata.to_vec(),
        })
    }

    pub fn storage(
        &self,
        address: [u8; 20],
        slot: [u8; 32],
    ) -> Result<StorageWitness, CommittedReadError> {
        let witnesses = self
            .endpoint
            .storage_witness(&address, &[slot], self.checkpoint.height)
            .map_err(CommittedReadError::Rpc)?;
        let [witness] = witnesses.as_slice() else {
            return Err(CommittedReadError::Storage);
        };
        if witness.address != address || witness.slot != slot {
            return Err(CommittedReadError::Storage);
        }
        layerx_client::storage_witness::verify_storage_witness(witness, self.checkpoint.app_hash)
            .map_err(|_| CommittedReadError::Storage)?;
        self.unchanged()?;
        Ok(witness.clone())
    }
}

#[cfg(test)]
pub(crate) mod tests {
    use super::*;
    use crate::read::BlockId;
    use serde_json::{json, Value};
    use std::io::{Read as _, Write as _};
    fn rpc_block(checkpoint: &ServedCheckpoint) -> Value {
        let hex = |bytes: &[u8]| format!("0x{}", layerx_programs::hex::encode(bytes));
        json!({"number": format!("{:#x}", checkpoint.height), "hash": hex(&checkpoint.block_hash),
            "parentHash": hex(&[1; 32]), "timestamp": format!("{:#x}", checkpoint.header.time.seconds),
            "transactionsRoot": hex(&[4; 32]), "receiptsRoot": hex(&[7; 32]),
            "stateRoot": hex(&checkpoint.app_hash), "transactions": []})
    }

    pub(crate) fn endpoint(
        answers: Vec<(&'static str, Value)>,
    ) -> (SequencerEndpoint, std::thread::JoinHandle<()>) {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("bind RPC fixture");
        let address = listener.local_addr().expect("fixture address");
        let worker = std::thread::spawn(move || {
            for (method, answer) in answers {
                let (mut stream, _) = listener.accept().expect("RPC request");
                stream
                    .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                    .expect("read deadline");
                let mut bytes = Vec::new();
                let (offset, length) = loop {
                    let mut byte = [0];
                    stream.read_exact(&mut byte).expect("request header");
                    bytes.push(byte[0]);
                    if bytes.ends_with(b"\r\n\r\n") {
                        let headers = String::from_utf8(bytes.clone()).expect("HTTP headers");
                        let length = headers
                            .lines()
                            .find_map(|line| {
                                let (name, value) = line.split_once(':')?;
                                name.eq_ignore_ascii_case("content-length")
                                    .then(|| value.trim().parse::<usize>().expect("length"))
                            })
                            .expect("request length");
                        break (bytes.len(), length);
                    }
                };
                bytes.resize(offset + length, 0);
                stream
                    .read_exact(&mut bytes[offset..])
                    .expect("request body");
                let request: Value = serde_json::from_slice(&bytes[offset..]).expect("JSON RPC");
                assert_eq!(request["method"], method);
                match method {
                    "eth_blockNumber" => assert_eq!(request["params"], json!([])),
                    "px_getCommit" | "px_getValidators" => {
                        assert_eq!(request["params"], json!(["8"]))
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(request["params"][0], "0x8");
                        assert!(request["params"][1].is_boolean());
                    }
                    "eth_chainId" => assert_eq!(request["params"], json!([])),
                    "eth_call" => assert_eq!(request["params"][1], "0x8"),
                    "eth_getProof" => assert_eq!(request["params"][2], "0x8"),
                    _ => panic!("unexpected RPC method {method}"),
                }
                let answer = if let Some(expected) = answer.get("_expected_data") {
                    assert_eq!(request["params"][0]["data"], *expected);
                    assert_eq!(
                        request["params"][0]["to"],
                        "0x000000000000000000000000000000000000101d"
                    );
                    answer["_result"].clone()
                } else {
                    answer
                };
                let body = if let Some(error) = answer.get("_error") {
                    json!({"jsonrpc": "2.0", "id": request["id"], "error": error}).to_string()
                } else {
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": answer}).to_string()
                };
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).expect("RPC reply");
            }
        });
        (
            SequencerEndpoint::new(&format!("http://{address}"), &[]).expect("RPC endpoint"),
            worker,
        )
    }

    fn base64(bytes: &[u8]) -> String {
        const ALPHABET: &[u8; 64] =
            b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
        let mut encoded = String::new();
        for chunk in bytes.chunks(3) {
            let word = (u32::from(chunk[0]) << 16)
                | (u32::from(*chunk.get(1).unwrap_or(&0)) << 8)
                | u32::from(*chunk.get(2).unwrap_or(&0));
            encoded.push(ALPHABET[(word >> 18) as usize] as char);
            encoded.push(ALPHABET[((word >> 12) & 63) as usize] as char);
            encoded.push(if chunk.len() > 1 {
                ALPHABET[((word >> 6) & 63) as usize] as char
            } else {
                '='
            });
            encoded.push(if chunk.len() > 2 {
                ALPHABET[(word & 63) as usize] as char
            } else {
                '='
            });
        }
        encoded
    }

    fn rpc_commit(signed: &ServedCheckpoint) -> Value {
        assert_eq!(signed.header.time.seconds, 1_700_000_000);
        let timestamp = format!("2023-11-14T22:13:20.{:09}Z", signed.header.time.nanos);
        let hex = |bytes: &[u8]| layerx_programs::hex::encode(bytes);
        let id = |id: BlockId| {
            json!({"hash": hex(&id.hash), "parts": {
            "total": id.part_set_total, "hash": hex(&id.part_set_hash)}})
        };
        let h = &signed.header;
        let mut signatures: Vec<_> = signed
            .commit
            .votes
            .iter()
            .map(|vote| {
                json!({
                    "block_id_flag": 2, "validator_address": hex(&vote.validator_address),
                    "timestamp": timestamp, "signature": base64(&vote.signature),
                })
            })
            .collect();
        signatures.push(
            json!({"block_id_flag": 1, "validator_address": "", "signature": null,
            "timestamp": "0001-01-01T00:00:00Z"}),
        );
        json!({"header": {
            "version": {"block": "11", "app": "1"}, "chain_id": "125", "height": signed.height.to_string(),
            "time": timestamp, "last_block_id": id(h.last_block_id.expect("parent")),
            "last_commit_hash": hex(&[3;32]), "data_hash": hex(&[4;32]),
            "validators_hash": hex(&h.validators_hash), "next_validators_hash": hex(&h.next_validators_hash),
            "consensus_hash": hex(&[5;32]), "app_hash": hex(&h.app_hash),
            "last_results_hash": hex(&[7;32]), "evidence_hash": hex(&[8;32]),
            "proposer_address": hex(&h.proposer_address),
        }, "commit": {"height": signed.height.to_string(), "round": 1,
            "block_id": id(signed.commit.block_id), "signatures": signatures}})
    }

    fn rpc_validators(signed: &ServedCheckpoint) -> Value {
        json!({"height": signed.height.to_string(), "total": signed.validators.len().to_string(),
            "validators": signed.validators.iter().map(|validator| json!({
                "address": layerx_programs::hex::encode(&validator.address),
                "pubKey": {"type": "tendermint/PubKeyEd25519", "value": base64(&validator.public_key)},
                "votingPower": validator.voting_power.to_string(),
            })).collect::<Vec<_>>()})
    }

    fn acquisition_answers(signed: &ServedCheckpoint, block: Value) -> Vec<(&'static str, Value)> {
        vec![
            ("eth_getBlockByNumber", block),
            (
                "px_getCommit",
                json!({"height": signed.height.to_string(), "signedHeader": rpc_commit(signed)}),
            ),
            ("px_getValidators", rpc_validators(signed)),
            ("eth_getBlockByNumber", rpc_block(signed)),
        ]
    }

    pub(crate) fn open_answers(checkpoint: &ServedCheckpoint) -> Vec<(&'static str, Value)> {
        let mut answers = vec![
            ("eth_chainId", json!("0x7d")),
            ("eth_getBlockByNumber", rpc_block(checkpoint)),
        ];
        answers.extend(acquisition_answers(checkpoint, rpc_block(checkpoint)));
        answers
    }

    pub(crate) fn stable_call(
        answers: &mut Vec<(&'static str, Value)>,
        checkpoint: &ServedCheckpoint,
        value: &[u8],
    ) {
        let value = json!(format!("0x{}", hex::encode(value)));
        answers.extend([
            ("eth_call", value.clone()),
            ("eth_call", value),
            ("eth_getBlockByNumber", rpc_block(checkpoint)),
            ("eth_getBlockByNumber", rpc_block(checkpoint)),
        ]);
    }

    #[test]
    fn capabilities_and_budget_are_bound_to_one_acquired_checkpoint() {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        stable_call(&mut answers, &checkpoint, &[7; 32]);
        stable_call(&mut answers, &checkpoint, &[9; 32]);
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8).expect("committed checkpoint");
        let caps = reads.capabilities([1; 20], &[1; 4]).expect("capabilities");
        assert_eq!(caps.canonical_bytes(), &[7; 32]);
        assert_eq!(caps.state_root(), checkpoint.app_hash);
        assert_eq!(caps.freshness().observed_head_sequence, 8);
        assert_eq!(caps.checkpoint().block_hash, checkpoint.block_hash);
        let budget = reads.budget_state([2; 20], &[2; 4]).expect("budget");
        assert_eq!(budget.canonical_bytes(), &[9; 32]);
        assert_eq!(budget.checkpoint(), caps.checkpoint());
        worker.join().expect("RPC server");
    }

    #[test]
    fn changed_second_call_is_refused() {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        answers.extend([("eth_call", json!("0x01")), ("eth_call", json!("0x02"))]);
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
        let error = reads
            .capabilities([1; 20], &[1; 4])
            .expect_err("changed call");
        assert_eq!(error, CommittedReadError::CommittedChanged);
        assert_eq!(error.code(), "read.committed_changed");
        worker.join().expect("RPC server");
    }

    #[test]
    fn changed_block_or_app_hash_is_refused() {
        for changed_root in [false, true] {
            let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
            let mut answers = open_answers(&checkpoint);
            let mut block = rpc_block(&checkpoint);
            block[if changed_root { "stateRoot" } else { "hash" }] =
                json!(format!("0x{}", hex::encode(&[99; 32])));
            answers.extend([
                ("eth_call", json!("0x01")),
                ("eth_call", json!("0x01")),
                ("eth_getBlockByNumber", block.clone()),
                ("eth_getBlockByNumber", block),
            ]);
            let (endpoint, worker) = endpoint(answers);
            let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
            assert_eq!(
                reads.budget_state([1; 20], &[1; 4]),
                Err(CommittedReadError::CommittedChanged)
            );
            worker.join().expect("RPC server");
        }
    }

    #[test]
    fn program_owner_uses_the_held_checkpoint_and_rejects_changed_settlement_read() {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        stable_call(&mut answers, &checkpoint, &[4; 32]);
        answers.extend([("eth_call", json!("0x01")), ("eth_call", json!("0x02"))]);
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
        let owner = crate::protocol_evidence::verified_program_owner(
            &endpoint,
            reads.checkpoint(),
            &[3; 32],
        )
        .expect("owner");
        assert_eq!(owner, [4; 32]);
        let mut calldata =
            layerx_crypto::evm_transaction::transaction_hash(b"getValueAccount(bytes32,bytes32)")
                [..4]
                .to_vec();
        calldata.extend([3; 32]);
        calldata.extend([5; 32]);
        assert!(matches!(
            crate::protocol_evidence::verified_program_record(
                &endpoint,
                reads.checkpoint(),
                &calldata
            ),
            Err(crate::human::HumanOperationError::Refused)
        ));
        worker.join().expect("RPC server");
    }
    pub(crate) fn budget_result() -> Vec<u8> {
        budget_result_with_spend(25)
    }

    pub(crate) fn budget_result_with_spend(spent: u64) -> Vec<u8> {
        let mut result = vec![0; 20 * 32];
        for (index, amount) in [
            (0, 32_u64),
            (5, 100),
            (6, 100),
            (8, spent),
            (10, 1000),
            (11, 1),
            (12, 10000),
            (14, 1),
            (17, 576),
        ] {
            result[index * 32 + 24..(index + 1) * 32].copy_from_slice(&amount.to_be_bytes());
        }
        for index in 1..=4 {
            result[index * 32..(index + 1) * 32].fill(index as u8);
        }
        result
    }

    #[test]
    fn human_budget_read_and_recovery_evidence_use_committed_calls() {
        crate::human_runtime::committed_budget_reads_restore_the_human_balance_and_refuse_changes();
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        stable_call(&mut answers, &checkpoint, &budget_result());
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
        let mut pay = [0; 20];
        pay[18..].copy_from_slice(&[0x10, 0x1b]);
        let mut data =
            layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
        data.extend([9; 32]);
        let call = reads.budget_state(pay, &data).expect("budget call");
        let raw = crate::protocol_evidence::RawStateEvidence::budget_witness(call.clone(), [9; 32])
            .expect("budget evidence");
        let authority =
            crate::protocol_evidence::EvidenceAuthority::pinned_to_handshake(3, 125, [1; 32])
                .expect("authority");
        let verified = authority.verify_state(&raw).expect("committed evidence");
        let record = crate::budget::ProtocolBudgetRecord::decode(verified.canonical_state())
            .expect("canonical record");
        assert_eq!(record.budget_id, [9; 32]);
        assert_eq!(record.spent_this_period, 25);
        assert_eq!(verified.observed_head_sequence(), 8);
        assert!(crate::protocol_evidence::RawStateEvidence::budget_witness(call, [8; 32]).is_err());
        worker.join().expect("RPC server");
    }

    fn budget_recovery_fixture(
        record: &[u8],
    ) -> (
        std::path::PathBuf,
        crate::store::Store,
        crate::store::TenantId,
        CommittedCall,
    ) {
        let mut nonce = [0; 16];
        getrandom::fill(&mut nonce).expect("temporary directory nonce");
        let root = std::env::temp_dir().join(format!(
            "layerx-committed-budget-recovery-{}-{}",
            std::process::id(),
            hex::encode(&nonce)
        ));
        std::fs::create_dir(&root).expect("fresh temporary directory");
        let store = crate::store::Store::open(&root).expect("recovery store");
        let tenant = crate::store::TenantId::new("committed-budget-recovery").expect("tenant");
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        stable_call(&mut answers, &checkpoint, record);
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
        let mut pay = [0; 20];
        pay[18..].copy_from_slice(&[0x10, 0x1b]);
        let mut data =
            layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
        data.extend([9; 32]);
        let call = reads.budget_state(pay, &data).expect("budget call");
        assert_eq!(call.checkpoint(), &checkpoint);
        assert_eq!(call.canonical_bytes(), call.repeated());
        worker.join().expect("RPC server");
        (root, store, tenant, call)
    }

    fn recover_tenant_budget(
        store: &mut crate::store::Store,
        tenant: &crate::store::TenantId,
        call: CommittedCall,
    ) -> Result<crate::human_runtime::TenantBudgetRecovery, crate::human_runtime::RecoveryRefusal>
    {
        let protocol_budget = crate::budget::ProtocolBudgetState {
            evidence: crate::protocol_evidence::RawStateEvidence::budget_witness(call, [9; 32])
                .expect("budget evidence"),
        };
        crate::human_runtime::recover_tenant_budget(
            store,
            tenant,
            &crate::human_runtime::BudgetRecoveryRequest {
                budget_id: [9; 32],
                protocol_budget,
                verifier: crate::protocol_evidence::EvidenceAuthority::pinned_to_handshake(
                    3, 125, [1; 32],
                )
                .expect("authority"),
                receipts_with_evidence: &[],
                receipts_without_evidence: &[],
                ceiling_maximum: 1_000,
                current_sequence: 1,
            },
        )
    }

    #[test]
    fn tenant_budget_recovery_reconciles_over_committed_budget_state() {
        let (root, mut store, tenant, call) = budget_recovery_fixture(&budget_result_with_spend(0));
        drop(store);
        store = crate::store::Store::open(&root).expect("reopen recovery store");
        let recovery = recover_tenant_budget(&mut store, &tenant, call)
            .unwrap_or_else(|error| panic!("recover committed budget: {error:?}"));
        let accounting = recovery.recovered.budget_accounting;
        assert_eq!(accounting.protocol_consumed, Some(0));
        assert_eq!(accounting.receipt_consumed, 0);
        assert_eq!(accounting.held_unresolved, 0);
        assert_eq!(accounting.unresolved_count, 0);
        assert!(accounting.reconciled);
        assert!(
            recovery
                .recovered
                .ceiling
                .snapshot()
                .expect("ceiling snapshot")
                .reconciled
        );
        assert!(recovery.recovered.require_write_ready().is_ok());
        assert!(recovery.admission.is_ok());
        drop(recovery);
        drop(store);
        std::fs::remove_dir_all(root).expect("remove recovery store");

        let (root, mut store, tenant, call) = budget_recovery_fixture(&budget_result());
        drop(store);
        store = crate::store::Store::open(&root).expect("reopen recovery store");
        let recovery = recover_tenant_budget(&mut store, &tenant, call)
            .unwrap_or_else(|error| panic!("recover committed budget: {error:?}"));
        let accounting = recovery.recovered.budget_accounting;
        assert_eq!(accounting.protocol_consumed, Some(25));
        assert_eq!(accounting.receipt_consumed, 0);
        assert_eq!(accounting.held_unresolved, 0);
        assert_eq!(accounting.unresolved_count, 0);
        assert!(!accounting.reconciled);
        assert!(recovery.recovered.require_write_ready().is_err());
        assert!(recovery.admission.is_err());
        drop(recovery);
        drop(store);
        std::fs::remove_dir_all(root).expect("remove recovery store");
    }

    #[test]
    fn tenant_budget_recovery_refuses_committed_evidence_whose_state_changed() {
        let (root, mut store, tenant, mut call) = budget_recovery_fixture(&budget_result());
        let original =
            crate::protocol_evidence::RawStateEvidence::budget_witness(call.clone(), [9; 32])
                .expect("original budget evidence");
        call.canonical_bytes[8 * 32 + 31] ^= 1;
        assert_ne!(call.canonical_bytes(), call.repeated());
        let changed =
            crate::protocol_evidence::RawStateEvidence::budget_witness(call.clone(), [9; 32])
                .expect("changed budget evidence remains decodable");
        assert_ne!(changed.canonical_state(), original.canonical_state());
        assert!(matches!(
            recover_tenant_budget(&mut store, &tenant, call),
            Err(crate::human_runtime::RecoveryRefusal::BudgetState { budget_id, reason })
                if budget_id == [9; 32] && reason == "Committed"
        ));
        assert!(store
            .list_object_ids(&tenant, crate::store::ObjectKind::Budget)
            .is_empty());
        assert!(store
            .list_object_ids(&tenant, crate::store::ObjectKind::Capability)
            .is_empty());
        drop(store);
        let reopened = crate::store::Store::open(&root).expect("reopen refused recovery store");
        assert!(reopened
            .list_object_ids(&tenant, crate::store::ObjectKind::Budget)
            .is_empty());
        assert!(reopened
            .list_object_ids(&tenant, crate::store::ObjectKind::Capability)
            .is_empty());
        drop(reopened);
        std::fs::remove_dir_all(root).expect("remove recovery store");
    }

    #[test]
    fn deployment_absence_and_fee_are_read_at_the_same_checkpoint() {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        let absence = json!({"_error": {"code": 3, "message": "LXP_NOT_FOUND: getProgram: -7"}});
        answers.extend([
            ("eth_call", absence.clone()),
            ("eth_call", absence),
            ("eth_getBlockByNumber", rpc_block(&checkpoint)),
            ("eth_getBlockByNumber", rpc_block(&checkpoint)),
        ]);
        let integer = |value: u64| {
            let mut word = [0; 32];
            word[24..].copy_from_slice(&value.to_be_bytes());
            word
        };
        let fees = [
            integer(1),
            integer(1),
            integer(2),
            integer(3),
            integer(4),
            integer(5),
            integer(6),
            integer(7),
            [7; 32],
        ]
        .concat();
        stable_call(&mut answers, &checkpoint, &fees);
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
        let records = crate::budget::execution_state::read_program_records(
            &endpoint,
            reads.checkpoint(),
            [3; 32],
            &[],
        )
        .expect("absence");
        assert!(records.program.is_none());
        assert!(records.interface.is_none());
        assert_eq!(records.fee_schedule.fee_asset, [7; 32]);
        worker.join().expect("RPC server");
    }

    #[test]
    fn program_sources_select_the_chain_bound_principal_inventory() {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let public = [4; 32];
        let did = Did::new(format!("did:layerx:{}", hex::encode(&public)).as_bytes()).expect("DID");
        let owner = layerx_wire::hash::did_id_for_protocol(&did, 3).expect("owner");
        let asset = [7; 32];
        let account = layerx_types::account::AccountId::for_asset(
            std::str::from_utf8(did.as_bytes()).expect("text"),
            asset,
            asset,
        )
        .expect("account");
        let account_id = layerx_wire::hash::account_id_for_protocol(&account, 3).expect("ID");
        let integer = |value: u64| {
            let mut word = [0; 32];
            word[24..].copy_from_slice(&value.to_be_bytes());
            word
        };
        let mut address = [0; 32];
        address[12..].fill(5);
        let unified = [address, integer(128), public, account_id, integer(0)].concat();
        let mut binding = [public, integer(64), integer(did.as_bytes().len() as u64)].concat();
        binding.extend(did.as_bytes());
        binding.resize(binding.len().div_ceil(32) * 32, 0);
        let metadata = crate::budget::fee_policy::tests::asset_result(asset, b"PAX", 18, false);
        let mut answers = open_answers(&checkpoint);
        for value in [
            address.to_vec(),
            unified,
            binding,
            metadata,
            account_id.to_vec(),
            owner.to_vec(),
            integer(50).to_vec(),
            integer(6).to_vec(),
        ] {
            stable_call(&mut answers, &checkpoint, &value);
        }
        let mut budget = budget_result();
        budget[32..64].copy_from_slice(&owner);
        stable_call(&mut answers, &checkpoint, &budget);
        stable_call(&mut answers, &checkpoint, &integer(75));
        let mut grant = [integer(96), integer(0), integer(0), integer(3)].concat();
        grant.extend([1, 2, 3]);
        grant.resize(160, 0);
        stable_call(&mut answers, &checkpoint, &grant);
        let scope = [
            integer(1),
            integer(5),
            integer(5),
            asset,
            integer(10),
            integer(100),
            integer(20),
            integer(1000),
            integer(50),
            integer(10),
            integer(1),
            [8; 32],
        ]
        .concat();
        stable_call(&mut answers, &checkpoint, &scope);
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
        let mut caps = reads.inventory(&did, &[asset], asset).expect("inventory");
        reads
            .inventory_records(&mut caps, &[[9; 32]], &[[6; 32]])
            .expect("selected budget and grant");
        assert_eq!(caps.budgets()[0].spent_this_period, 25);
        assert_eq!(caps.grants()[0].maximum_per_activity, 10);
        crate::budget::VerifiedResolvedProgramCharges::assert_committed_inventory(
            &caps, &did, asset, account_id,
        );
        assert_eq!(caps.all_accounts()[&account_id].balance(), 50);
        assert_eq!(caps.all_accounts()[&account_id].next_sequence, 6);
        assert_eq!(caps.state_root(), checkpoint.app_hash);
        worker.join().expect("RPC server");
    }

    #[test]
    fn storage_verifies_the_generated_chain_witness_under_the_checkpoint_app_hash() {
        use ed25519_dalek::{Signer as _, SigningKey};
        let vector: Value = serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
        )))
        .expect("generated vector");
        let entry = &vector["entries"][0];
        let decode =
            |key: &str| hex::decode(entry[key].as_str().expect("hex vector")).expect("hex");
        let mut checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        checkpoint.header.app_hash = decode("app_hash").try_into().expect("root");
        checkpoint.app_hash = checkpoint.header.app_hash;
        checkpoint.block_hash = checkpoint.header.block_hash();
        checkpoint.commit.block_id.hash = checkpoint.block_hash;
        for index in 0..checkpoint.commit.votes.len() {
            let message = checkpoint.commit.vote_sign_bytes(
                &checkpoint.header.chain_id,
                checkpoint.commit.votes[index].timestamp,
            );
            checkpoint.commit.votes[index].signature =
                SigningKey::from_bytes(&[index as u8 + 1; 32])
                    .sign(&message)
                    .to_bytes();
        }
        let mut answers = open_answers(&checkpoint);
        answers.extend([
            ("eth_getProof", entry["response"].clone()),
            ("eth_getBlockByNumber", rpc_block(&checkpoint)),
            ("eth_getBlockByNumber", rpc_block(&checkpoint)),
        ]);
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
        let witness = reads
            .storage(
                decode("address").try_into().expect("address"),
                decode("slot").try_into().expect("slot"),
            )
            .expect("verified storage");
        assert_eq!(witness.value.as_slice(), decode("value"));
        let mut tampered = witness;
        tampered.value[31] ^= 1;
        assert!(layerx_client::storage_witness::verify_storage_witness(
            &tampered,
            checkpoint.app_hash
        )
        .is_err());
        worker.join().expect("RPC server");
    }

    #[test]
    fn program_views_decode_one_program_and_one_account_and_refuse_changed_settlement() {
        use crate::budget::execution_state::read_program_records;
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let program = [3; 32];
        let asset = [7; 32];
        let seed = [1, 2, 3];
        let account = layerx_programs_runtime::derive_program_account(
            layerx_programs::ProgramId::new(program).expect("program"),
            &seed,
        )
        .expect("derived account")
        .bytes();
        let integer = |value: u64| {
            let mut word = [0; 32];
            word[24..].copy_from_slice(&value.to_be_bytes());
            word
        };
        let program_result = [
            integer(1),
            [4; 32],
            [5; 32],
            integer(2),
            integer(1),
            integer(1),
        ]
        .concat();
        let wind_down = [
            integer(1),
            [0; 32],
            integer(0),
            integer(1),
            integer(1),
            integer(1),
        ]
        .concat();
        let interface = [integer(0), [0; 32], integer(96), integer(0)].concat();
        let fees = [
            integer(1),
            integer(1),
            integer(2),
            integer(3),
            integer(4),
            integer(5),
            integer(6),
            integer(7),
            asset,
        ]
        .concat();
        let mut binding = [program, asset, integer(96), integer(3)].concat();
        binding.extend(seed);
        binding.resize(160, 0);
        let registered = [account, asset, integer(2)].concat();
        let value = [integer(50), integer(0)].concat();
        let mut account_arguments = [program, asset, integer(96), integer(3)].concat();
        account_arguments.extend(seed);
        account_arguments.resize(160, 0);
        let requests: Vec<(&[u8], Vec<u8>, Vec<u8>)> = vec![
            (b"getProgram(bytes32)", program.to_vec(), program_result),
            (b"getProgramOwner(bytes32)", program.to_vec(), vec![4; 32]),
            (b"getWindDown(bytes32)", program.to_vec(), wind_down),
            (b"getInterface(bytes32)", program.to_vec(), interface),
            (b"getCurrentFeeSchedule()", Vec::new(), fees),
            (b"getProgramAccountById(bytes32)", account.to_vec(), binding),
            (
                b"getProgramAccount(bytes32,bytes32,bytes)",
                account_arguments,
                registered,
            ),
            (
                b"getValueAccount(bytes32,bytes32)",
                [program, account].concat(),
                value,
            ),
        ];
        for changed in [false, true] {
            let mut answers = open_answers(&checkpoint);
            stable_call(&mut answers, &checkpoint, &requests[0].2);
            for (index, (signature, arguments, result)) in requests.iter().enumerate() {
                let mut calldata =
                    layerx_crypto::evm_transaction::transaction_hash(signature)[..4].to_vec();
                calldata.extend(arguments);
                let expected = format!("0x{}", hex::encode(&calldata));
                let first = json!({"_expected_data": expected, "_result": format!("0x{}", hex::encode(result))});
                let different = changed && index == requests.len() - 1;
                let second = if different {
                    json!({"_expected_data": expected, "_result": "0x00"})
                } else {
                    first.clone()
                };
                answers.extend([("eth_call", first), ("eth_call", second)]);
                if !different {
                    answers.extend([
                        ("eth_getBlockByNumber", rpc_block(&checkpoint)),
                        ("eth_getBlockByNumber", rpc_block(&checkpoint)),
                    ]);
                }
            }
            let (endpoint, worker) = endpoint(answers);
            let reads = CommittedReads::open(&endpoint, 8).expect("checkpoint");
            let result = read_program_records(&endpoint, reads.checkpoint(), program, &[account]);
            if changed {
                assert!(matches!(
                    result,
                    Err(crate::budget::program_settlement::ProgramSettlementError::SourceSnapshot)
                ));
            } else {
                let result = result.expect("program views");
                result.assert_committed_lifecycle(program, account, asset);
                assert_eq!(result.owner, [4; 32]);
                assert_eq!(
                    result.program.as_ref().expect("registered").code_hash,
                    [5; 32]
                );
                assert_eq!(
                    result.wind_down.as_ref().expect("wind down").account_count,
                    1
                );
                assert_eq!(result.fee_schedule.fee_asset, asset);
                let binding = result.accounts.get(&account).expect("one account");
                assert_eq!(binding.seed, seed);
                assert_eq!(binding.asset, asset);
                assert_eq!(binding.registered_sequence, 2);
                assert_eq!(binding.balance, 50);
                assert!(!binding.frozen);
            }
            worker.join().expect("RPC server");
        }
    }
}
