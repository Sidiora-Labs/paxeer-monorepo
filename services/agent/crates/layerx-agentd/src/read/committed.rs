//! Precompile calls and storage witnesses bound to one authenticated committed block.

use super::{CheckpointReadError, ServedCheckpoint};
use layerx_client::rpc::account::Freshness;
use layerx_client::sequencer::{SequencerEndpoint, SequencerError, StorageWitness};
use layerx_programs::hex;
use layerx_proof::state::{AccountAsset, CanonicalAccount};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use std::collections::BTreeMap;

type InventoryBinding = ([u8; 32], [u8; 32], [u8; 32]);

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
    #[must_use]
    pub fn has_asset(&self) -> bool {
        self.asset.is_some()
    }
    #[must_use]
    pub fn asset_id(&self) -> [u8; 32] {
        self.asset.map_or([0; 32], |asset| asset.asset_id)
    }
    #[must_use]
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
    /// ledger balance. Authentication of both reads belongs to `CommittedReads`.
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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
    #[must_use]
    pub const fn allowance(&self) -> u128 {
        self.allowance
    }
    #[must_use]
    pub const fn funded_balance(&self) -> u128 {
        self.balance
    }
    #[must_use]
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

impl CommittedGrant {
    fn verify_encoding(&self, owner: [u8; 32]) -> Result<(), CommittedReadError> {
        use layerx_wire::decode::Decoder;
        let decode = || -> Result<bool, layerx_wire::WireError> {
            let mut d = Decoder::new(&self.encoding, 0);
            d.structure_header(0x2001)?;
            let version = d.u8()?;
            if !matches!(version, 1..=4) || d.bytes(32)? != owner || d.bytes(32)? != owner {
                return Ok(false);
            }
            let _kind = d.u8()?;
            if d.bytes(32)?.len() != 32 {
                return Ok(false);
            }
            let same_scope = d.u64()? == self.module_mask
                && d.u16()? == self.ordinal_min
                && d.u16()? == self.ordinal_max
                && d.bytes(32)? == self.asset
                && d.u128()? == self.maximum_per_activity
                && d.u128()? == self.maximum_total
                && d.u128()? == self.spent_total
                && d.u64()? == self.period_length
                && d.u128()? == self.maximum_per_period
                && d.u128()? == self.spent_this_period
                && d.u64()? == self.period_start
                && d.bytes(32)? == self.purpose;
            if !same_scope {
                return Ok(false);
            }
            let _not_before = d.u64()?;
            let _not_after = d.u64()?;
            let _revocation_sequence = d.u64()?;
            let revoked = d.u8()?;
            let revoked_at = d.u64()?;
            Ok(revoked == u8::from(self.revoked)
                && revoked_at == self.revoked_at
                && !self.revoked
                && self.revoked_at == 0
                && d.bytes(64)?.len() == 64)
        };
        if self.ordinal_min > self.ordinal_max
            || !decode().map_err(|_| CommittedReadError::Selector)?
        {
            return Err(CommittedReadError::Selector);
        }
        Ok(())
    }
}

impl CommittedCaps {
    #[must_use]
    pub const fn did(&self) -> [u8; 32] {
        self.did
    }
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.checkpoint.app_hash
    }
    #[must_use]
    pub const fn observed_height(&self) -> u64 {
        self.checkpoint.height
    }
    #[must_use]
    #[allow(
        clippy::unused_self,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub const fn level(&self) -> VerificationLevel {
        VerificationLevel::COMMITTED
    }
    #[must_use]
    pub const fn freshness(&self) -> Freshness {
        checkpoint_freshness(&self.checkpoint)
    }
    #[must_use]
    pub fn all_accounts(&self) -> &BTreeMap<[u8; 32], CommittedAccount> {
        &self.accounts
    }
    #[must_use]
    pub fn budgets(&self) -> &[crate::budget::ProtocolBudgetRecord] {
        &self.budgets
    }
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
    pub fn budget_funding(
        &self,
        id: &[u8; 32],
    ) -> Result<&CommittedBudgetFunding, CommittedReadError> {
        self.budget_funding
            .get(id)
            .ok_or(CommittedReadError::Selector)
    }

    /// Selects only funded spending power at the inventory's held checkpoint.
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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
    #[must_use]
    pub fn grants(&self) -> &[CommittedGrant] {
        &self.grants
    }
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
    pub fn effective_asset(&self, asset: [u8; 32]) -> Result<CommittedAsset, CommittedReadError> {
        self.assets
            .get(&asset)
            .cloned()
            .ok_or(CommittedReadError::Selector)
    }
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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
    #[must_use]
    pub const fn account(&self) -> [u8; 32] {
        self.account
    }
    #[must_use]
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
    #[must_use]
    pub fn metadata(&self) -> &layerx_client::rpc::payments::AssetMetadata {
        &self.metadata
    }
    #[must_use]
    #[allow(
        clippy::unused_self,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub const fn registered(&self) -> bool {
        true
    }
    #[must_use]
    pub const fn paused(&self) -> bool {
        self.metadata.paused
    }
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.call.state_root()
    }
    #[must_use]
    pub const fn level(&self) -> VerificationLevel {
        self.call.level()
    }
    #[must_use]
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
        .map(<[u8]>::to_vec)
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
    #[must_use]
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
    #[must_use]
    pub fn precompile(&self) -> [u8; 20] {
        self.precompile
    }
    #[must_use]
    pub fn calldata(&self) -> &[u8] {
        &self.calldata
    }
    #[must_use]
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.canonical_bytes
    }
    #[must_use]
    pub fn repeated(&self) -> &[u8] {
        &self.repeated
    }
    #[must_use]
    pub fn checkpoint(&self) -> &ServedCheckpoint {
        &self.checkpoint
    }
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.checkpoint.app_hash
    }
    #[must_use]
    #[allow(
        clippy::unused_self,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub const fn level(&self) -> VerificationLevel {
        VerificationLevel::COMMITTED
    }
    #[must_use]
    pub const fn achieved(&self) -> VerificationLevel {
        self.level()
    }
    #[must_use]
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
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
    pub fn effective_asset(&self, asset: [u8; 32]) -> Result<CommittedAsset, CommittedReadError> {
        let call = self.call(precompile(0x1a), &calldata(b"getAsset(bytes32)", &asset))?;
        let bytes = call.canonical_bytes();
        let paused = uint(bytes, 10, 1)?;
        let metadata = layerx_client::rpc::payments::AssetMetadata {
            asset_id: word(bytes, 0)?,
            symbol: dynamic(bytes, 1, 12, 16)?,
            name: String::from_utf8(dynamic(bytes, 2, 12, 32)?)
                .map_err(|_| CommittedReadError::Selector)?,
            decimals: u8::try_from(uint(bytes, 3, 1)?).map_err(|_| CommittedReadError::Selector)?,
            supply_cap: uint(bytes, 4, 16)?,
            issuer_did: word(bytes, 5)?,
            salt: word(bytes, 6)?,
            issuer_kind: u8::try_from(uint(bytes, 7, 1)?)
                .map_err(|_| CommittedReadError::Selector)?,
            custody_kind: u8::try_from(uint(bytes, 8, 1)?)
                .map_err(|_| CommittedReadError::Selector)?,
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

    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
    pub fn inventory(
        &self,
        did: &Did,
        assets: &[[u8; 32]],
        fee_asset: [u8; 32],
    ) -> Result<CommittedCaps, CommittedReadError> {
        let text = std::str::from_utf8(did.as_bytes()).map_err(|_| CommittedReadError::Selector)?;
        let (public, owner, main) = self.inventory_binding(did, text)?;
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
        crate::reject_if(selected.len() > 4096 || selected.contains(&[0; 32]), || {
            CommittedReadError::Selector
        })?;
        for asset in selected {
            let metadata = self.effective_asset(asset)?;
            let account = layerx_types::account::AccountId::for_asset(text, asset, fee_asset)
                .map_err(|_| CommittedReadError::Selector)?;
            let account_id = layerx_wire::hash::account_id_for_protocol(&account, 3)
                .map_err(|_| CommittedReadError::Selector)?;
            crate::reject_if(asset == fee_asset && account_id != main, || {
                CommittedReadError::Selector
            })?;
            let returned = self.call(
                precompile(0x1a),
                &calldata(b"accountId(bytes32,bytes32)", &[owner, asset].concat()),
            )?;
            crate::reject_if(returned.canonical_bytes() != account_id, || {
                CommittedReadError::Selector
            })?;
            let returned = self.call(
                precompile(0x1a),
                &calldata(b"accountOwner(bytes32)", &account_id),
            )?;
            crate::reject_if(returned.canonical_bytes() != owner, || {
                CommittedReadError::Selector
            })?;
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
                    next_sequence: u64::try_from(uint(sequence.canonical_bytes(), 0, 8)?)
                        .map_err(|_| CommittedReadError::Selector)?,
                    frozen: metadata.paused(),
                    authority_key: Some(public),
                },
            );
            inventory.assets.insert(asset, metadata);
        }
        Ok(inventory)
    }
    fn inventory_binding(
        &self,
        did: &Did,
        text: &str,
    ) -> Result<InventoryBinding, CommittedReadError> {
        let public = text
            .strip_prefix("did:layerx:")
            .ok_or(CommittedReadError::Selector)?;
        crate::reject_if(
            public.len() != 64
                || !public
                    .bytes()
                    .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte)),
            || CommittedReadError::Selector,
        )?;
        let public: [u8; 32] = hex::decode(public)
            .map_err(|_| CommittedReadError::Selector)?
            .try_into()
            .map_err(|_| CommittedReadError::Selector)?;
        let owner = layerx_wire::hash::did_id_for_protocol(did, 3)
            .map_err(|_| CommittedReadError::Selector)?;
        let addr = layerx_client::sequencer::ADDR_PRECOMPILE;
        let address = self.call(addr, &calldata(b"getEvmAddrByLayerX(bytes32)", &public))?;
        let address = word(address.canonical_bytes(), 0)?;
        crate::reject_if(address[..12] != [0; 12] || address[12..] == [0; 20], || {
            CommittedReadError::Selector
        })?;
        let unified = self.call(addr, &calldata(b"getUnifiedAccount(address)", &address))?;
        crate::reject_if(
            word(unified.canonical_bytes(), 0)? != address
                || word(unified.canonical_bytes(), 2)? != public,
            || CommittedReadError::Selector,
        )?;
        let main = word(unified.canonical_bytes(), 3)?;
        let binding = self.call(addr, &calldata(b"getLayerXDid(address)", &address))?;
        crate::reject_if(
            word(binding.canonical_bytes(), 0)? != public
                || dynamic(binding.canonical_bytes(), 1, 2, 512)? != did.as_bytes(),
            || CommittedReadError::Selector,
        )?;

        Ok((public, owner, main))
    }

    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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
            if selected_grants
                .iter()
                .any(|previous: &CommittedGrant| previous.id == *id)
            {
                return Err(CommittedReadError::Selector);
            }
            let grant = self.grant(*id, inventory.did)?;
            selected_grants.push(grant);
        }
        inventory.budgets = selected_budgets;
        inventory.budget_funding = selected_funding;
        inventory.grants = selected_grants;
        Ok(())
    }

    /// Reads both authority views at the held checkpoint and binds the canonical
    /// grant's identity, scope and revocation record before admitting it.
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
    pub fn grant(
        &self,
        id: [u8; 32],
        owner: [u8; 32],
    ) -> Result<CommittedGrant, CommittedReadError> {
        let value = self.call(precompile(0x1c), &calldata(b"getGrant(bytes32)", &id))?;
        let bytes = value.canonical_bytes();
        let revoked = uint(bytes, 1, 1)?;
        if revoked > 1 {
            return Err(CommittedReadError::Selector);
        }
        let encoding = dynamic(bytes, 0, 3, 1_048_576)?;
        let scope = self.call(
            precompile(0x1c),
            &calldata(b"capabilityScope(bytes32)", &id),
        )?;
        let scope = scope.canonical_bytes();
        if scope.len() != 384 {
            return Err(CommittedReadError::Selector);
        }
        let grant = CommittedGrant {
            id,
            encoding,
            revoked: revoked == 1,
            revoked_at: u64::try_from(uint(bytes, 2, 8)?)
                .map_err(|_| CommittedReadError::Selector)?,
            module_mask: u64::try_from(uint(scope, 0, 8)?)
                .map_err(|_| CommittedReadError::Selector)?,
            ordinal_min: u16::try_from(uint(scope, 1, 2)?)
                .map_err(|_| CommittedReadError::Selector)?,
            ordinal_max: u16::try_from(uint(scope, 2, 2)?)
                .map_err(|_| CommittedReadError::Selector)?,
            asset: word(scope, 3)?,
            maximum_per_activity: uint(scope, 4, 16)?,
            maximum_total: uint(scope, 5, 16)?,
            spent_total: uint(scope, 6, 16)?,
            period_length: u64::try_from(uint(scope, 7, 8)?)
                .map_err(|_| CommittedReadError::Selector)?,
            maximum_per_period: uint(scope, 8, 16)?,
            spent_this_period: uint(scope, 9, 16)?,
            period_start: u64::try_from(uint(scope, 10, 8)?)
                .map_err(|_| CommittedReadError::Selector)?,
            purpose: word(scope, 11)?,
        };
        grant.verify_encoding(owner)?;
        Ok(grant)
    }

    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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
                next_sequence: u64::try_from(uint(sequence.canonical_bytes(), 0, 8)?)
                    .map_err(|_| CommittedReadError::Selector)?,
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
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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

    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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
    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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

    #[must_use]
    pub fn checkpoint(&self) -> &ServedCheckpoint {
        &self.checkpoint
    }

    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
    pub fn capabilities(
        &self,
        precompile: [u8; 20],
        calldata: &[u8],
    ) -> Result<CommittedCall, CommittedReadError> {
        self.call(precompile, calldata)
    }

    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
    pub fn budget_state(
        &self,
        precompile: [u8; 20],
        calldata: &[u8],
    ) -> Result<CommittedCall, CommittedReadError> {
        self.call(precompile, calldata)
    }

    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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

    /// # Errors
    /// Returns an error if the requested committed selector, checkpoint or returned state is
    /// invalid, changes during acquisition, or cannot be read.
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
#[path = "program_account_fixture.rs"]
mod program_account_fixture;

#[cfg(test)]
pub(crate) mod tests {
    use super::*;
    use crate::read::BlockId;
    use serde_json::{json, Value};
    use std::io::{Read as _, Write as _};

    trait FixtureResult<T> {
        fn require(self, context: &str) -> T;
    }

    impl<T, E: std::fmt::Debug> FixtureResult<T> for Result<T, E> {
        fn require(self, context: &str) -> T {
            match self {
                Ok(value) => value,
                Err(error) => panic!("{context}: {error:?}"),
            }
        }
    }

    trait FixtureOption<T> {
        fn require_some(self, context: &str) -> T;
    }

    impl<T> FixtureOption<T> for Option<T> {
        fn require_some(self, context: &str) -> T {
            match self {
                Some(value) => value,
                None => panic!("{context}"),
            }
        }
    }
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
        let listener = std::net::TcpListener::bind("127.0.0.1:0").require("bind RPC fixture");
        let address = listener.local_addr().require("fixture address");
        let worker = std::thread::spawn(move || {
            for (method, answer) in answers {
                let (mut stream, _) = listener.accept().require("RPC request");
                stream
                    .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                    .require("read deadline");
                let mut bytes = Vec::new();
                let (offset, length) = loop {
                    let mut byte = [0];
                    stream.read_exact(&mut byte).require("request header");
                    bytes.push(byte[0]);
                    if bytes.ends_with(b"\r\n\r\n") {
                        let headers = String::from_utf8(bytes.clone()).require("HTTP headers");
                        let length = headers
                            .lines()
                            .find_map(|line| {
                                let (name, value) = line.split_once(':')?;
                                name.eq_ignore_ascii_case("content-length")
                                    .then(|| value.trim().parse::<usize>().require("length"))
                            })
                            .require_some("request length");
                        break (bytes.len(), length);
                    }
                };
                bytes.resize(offset + length, 0);
                stream
                    .read_exact(&mut bytes[offset..])
                    .require("request body");
                let request: Value = serde_json::from_slice(&bytes[offset..]).require("JSON RPC");
                assert_eq!(request["method"], method);
                match method {
                    "eth_blockNumber" | "eth_chainId" => assert_eq!(request["params"], json!([])),
                    "px_getCommit" | "px_getValidators" => {
                        assert_eq!(request["params"], json!(["8"]));
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(request["params"][0], "0x8");
                        assert!(request["params"][1].is_boolean());
                    }
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
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).require("RPC reply");
            }
        });
        (
            SequencerEndpoint::new(&format!("http://{address}"), &[]).require("RPC endpoint"),
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
            "time": timestamp, "last_block_id": id(h.last_block_id.require_some("parent")),
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
    fn capabilities_and_budget_are_bound_to_one_acquired_checkpoint() -> Result<(), String> {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        stable_call(&mut answers, &checkpoint, &[7; 32]);
        stable_call(&mut answers, &checkpoint, &[9; 32]);
        let (endpoint, worker) = endpoint(answers);
        let reads = CommittedReads::open(&endpoint, 8)
            .map_err(|error| format!("committed checkpoint: {error:?}"))?;
        let caps = reads
            .capabilities([1; 20], &[1; 4])
            .map_err(|error| format!("capabilities: {error:?}"))?;
        assert_eq!(caps.level(), VerificationLevel::COMMITTED);
        assert_eq!(caps.level().wire_rank(), 6);
        assert_eq!(caps.canonical_bytes(), &[7; 32]);
        assert_eq!(caps.state_root(), checkpoint.app_hash);
        assert_eq!(caps.freshness().observed_head_sequence, 8);
        assert_eq!(caps.checkpoint().block_hash, checkpoint.block_hash);
        let budget = reads
            .budget_state([2; 20], &[2; 4])
            .map_err(|error| format!("budget: {error:?}"))?;
        assert_eq!(budget.canonical_bytes(), &[9; 32]);
        assert_eq!(budget.checkpoint(), caps.checkpoint());
        assert!(worker.join().is_ok(), "RPC server");
        Ok(())
    }

    #[test]
    fn changed_second_call_is_refused() -> Result<(), String> {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        answers.extend([("eth_call", json!("0x01")), ("eth_call", json!("0x02"))]);
        let (endpoint, worker) = endpoint(answers);
        let reads =
            CommittedReads::open(&endpoint, 8).map_err(|error| format!("checkpoint: {error:?}"))?;
        let error = reads
            .capabilities([1; 20], &[1; 4])
            .err()
            .ok_or("changed call")?;
        assert_eq!(error, CommittedReadError::CommittedChanged);
        assert_eq!(error.code(), "read.committed_changed");
        assert!(worker.join().is_ok(), "RPC server");
        Ok(())
    }

    #[test]
    fn changed_block_or_app_hash_is_refused() -> Result<(), String> {
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
            let reads = CommittedReads::open(&endpoint, 8)
                .map_err(|error| format!("checkpoint: {error:?}"))?;
            assert_eq!(
                reads.budget_state([1; 20], &[1; 4]),
                Err(CommittedReadError::CommittedChanged)
            );
            assert!(worker.join().is_ok(), "RPC server");
        }
        Ok(())
    }

    #[test]
    fn program_owner_uses_the_held_checkpoint_and_rejects_changed_settlement_read(
    ) -> Result<(), String> {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        stable_call(&mut answers, &checkpoint, &[4; 32]);
        answers.extend([("eth_call", json!("0x01")), ("eth_call", json!("0x02"))]);
        let (endpoint, worker) = endpoint(answers);
        let reads =
            CommittedReads::open(&endpoint, 8).map_err(|error| format!("checkpoint: {error:?}"))?;
        let owner = crate::protocol_evidence::verified_program_owner(
            &endpoint,
            reads.checkpoint(),
            &[3; 32],
        )
        .map_err(|error| format!("owner: {error:?}"))?;
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
        assert!(worker.join().is_ok(), "RPC server");
        Ok(())
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
        for index in 1_u8..=4 {
            let offset = usize::from(index);
            result[offset * 32..(offset + 1) * 32].fill(index);
        }
        result
    }

    #[test]
    fn human_budget_read_and_recovery_evidence_use_committed_calls() -> Result<(), String> {
        crate::human_runtime::committed_budget_reads_restore_the_human_balance_and_refuse_changes();
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        stable_call(&mut answers, &checkpoint, &budget_result());
        let (endpoint, worker) = endpoint(answers);
        let reads =
            CommittedReads::open(&endpoint, 8).map_err(|error| format!("checkpoint: {error:?}"))?;
        let mut pay = [0; 20];
        pay[18..].copy_from_slice(&[0x10, 0x1b]);
        let mut data =
            layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
        data.extend([9; 32]);
        let call = reads
            .budget_state(pay, &data)
            .map_err(|error| format!("budget call: {error:?}"))?;
        let raw = crate::protocol_evidence::RawStateEvidence::budget_witness(call.clone(), [9; 32])
            .map_err(|error| format!("budget evidence: {error:?}"))?;
        let authority =
            crate::protocol_evidence::EvidenceAuthority::pinned_to_handshake(3, 125, [1; 32])
                .map_err(|error| format!("authority: {error:?}"))?;
        let verified = authority
            .verify_state(&raw)
            .map_err(|error| format!("committed evidence: {error:?}"))?;
        let record = crate::budget::ProtocolBudgetRecord::decode(verified.canonical_state())
            .map_err(|error| format!("canonical record: {error:?}"))?;
        assert_eq!(record.budget_id, [9; 32]);
        assert_eq!(record.spent_this_period, 25);
        assert_eq!(verified.observed_head_sequence(), 8);
        assert!(crate::protocol_evidence::RawStateEvidence::budget_witness(call, [8; 32]).is_err());
        assert!(worker.join().is_ok(), "RPC server");
        Ok(())
    }

    fn budget_recovery_fixture(
        record: &[u8],
    ) -> Result<
        (
            std::path::PathBuf,
            crate::store::Store,
            crate::store::TenantId,
            CommittedCall,
        ),
        String,
    > {
        use std::os::unix::fs::DirBuilderExt;

        let mut nonce = [0; 16];
        getrandom::fill(&mut nonce)
            .map_err(|error| format!("temporary directory nonce: {error:?}"))?;
        let root = std::fs::canonicalize(std::env::temp_dir())
            .map_err(|error| format!("canonical temporary directory: {error:?}"))?
            .join(format!(
                "layerx-committed-budget-recovery-{}-{}",
                std::process::id(),
                hex::encode(&nonce)
            ));
        std::fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .map_err(|error| format!("fresh private temporary directory: {error:?}"))?;
        let store = crate::store::Store::open(&root)
            .map_err(|error| format!("recovery store: {error:?}"))?;
        let tenant = crate::store::TenantId::new("committed-budget-recovery")
            .map_err(|error| format!("tenant: {error:?}"))?;
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let mut answers = open_answers(&checkpoint);
        stable_call(&mut answers, &checkpoint, record);
        let (endpoint, worker) = endpoint(answers);
        let reads =
            CommittedReads::open(&endpoint, 8).map_err(|error| format!("checkpoint: {error:?}"))?;
        let mut pay = [0; 20];
        pay[18..].copy_from_slice(&[0x10, 0x1b]);
        let mut data =
            layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
        data.extend([9; 32]);
        let call = reads
            .budget_state(pay, &data)
            .map_err(|error| format!("budget call: {error:?}"))?;
        assert_eq!(call.checkpoint(), &checkpoint);
        assert_eq!(call.canonical_bytes(), call.repeated());
        assert!(worker.join().is_ok(), "RPC server");
        Ok((root, store, tenant, call))
    }

    fn recover_tenant_budget(
        store: &mut crate::store::Store,
        tenant: &crate::store::TenantId,
        call: CommittedCall,
    ) -> Result<crate::human_runtime::TenantBudgetRecovery, crate::human_runtime::RecoveryRefusal>
    {
        let protocol_budget = crate::budget::ProtocolBudgetState {
            evidence: crate::protocol_evidence::RawStateEvidence::budget_witness(call, [9; 32])
                .require("budget evidence"),
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
                .require("authority"),
                receipts_with_evidence: &[],
                receipts_without_evidence: &[],
                ceiling_maximum: 1_000,
                current_sequence: 1,
            },
        )
    }

    #[test]
    fn tenant_budget_recovery_reconciles_over_committed_budget_state() -> Result<(), String> {
        let (root, mut store, tenant, call) =
            budget_recovery_fixture(&budget_result_with_spend(0))?;
        drop(store);
        store = crate::store::Store::open(&root)
            .map_err(|error| format!("reopen recovery store: {error:?}"))?;
        let recovery = recover_tenant_budget(&mut store, &tenant, call)
            .map_err(|error| format!("recover committed budget: {error:?}"))?;
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
                .map_err(|error| format!("ceiling snapshot: {error:?}"))?
                .reconciled
        );
        assert!(recovery.recovered.require_write_ready().is_ok());
        assert!(recovery.admission.is_ok());
        drop(recovery);
        drop(store);
        std::fs::remove_dir_all(root)
            .map_err(|error| format!("remove recovery store: {error:?}"))?;

        let (root, mut store, tenant, call) = budget_recovery_fixture(&budget_result())?;
        drop(store);
        store = crate::store::Store::open(&root)
            .map_err(|error| format!("reopen recovery store: {error:?}"))?;
        let recovery = recover_tenant_budget(&mut store, &tenant, call)
            .map_err(|error| format!("recover committed budget: {error:?}"))?;
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
        std::fs::remove_dir_all(root)
            .map_err(|error| format!("remove recovery store: {error:?}"))?;
        Ok(())
    }

    #[test]
    fn tenant_budget_recovery_refuses_committed_evidence_whose_state_changed() -> Result<(), String>
    {
        let (root, mut store, tenant, mut call) = budget_recovery_fixture(&budget_result())?;
        let original =
            crate::protocol_evidence::RawStateEvidence::budget_witness(call.clone(), [9; 32])
                .map_err(|error| format!("original budget evidence: {error:?}"))?;
        call.canonical_bytes[8 * 32 + 31] ^= 1;
        assert_ne!(call.canonical_bytes(), call.repeated());
        let changed =
            crate::protocol_evidence::RawStateEvidence::budget_witness(call.clone(), [9; 32])
                .map_err(|error| format!("changed budget evidence remains decodable: {error:?}"))?;
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
        let reopened = crate::store::Store::open(&root)
            .map_err(|error| format!("reopen refused recovery store: {error:?}"))?;
        assert!(reopened
            .list_object_ids(&tenant, crate::store::ObjectKind::Budget)
            .is_empty());
        assert!(reopened
            .list_object_ids(&tenant, crate::store::ObjectKind::Capability)
            .is_empty());
        drop(reopened);
        std::fs::remove_dir_all(root)
            .map_err(|error| format!("remove recovery store: {error:?}"))?;
        Ok(())
    }

    #[test]
    fn deployment_absence_and_fee_are_read_at_the_same_checkpoint() -> Result<(), String> {
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
        let reads =
            CommittedReads::open(&endpoint, 8).map_err(|error| format!("checkpoint: {error:?}"))?;
        let records = crate::budget::execution_state::read_program_records(
            &endpoint,
            reads.checkpoint(),
            [3; 32],
            &[],
        )
        .map_err(|error| format!("absence: {error:?}"))?;
        assert!(records.program.is_none());
        assert!(records.interface.is_none());
        assert_eq!(records.fee_schedule.fee_asset, [7; 32]);
        assert!(worker.join().is_ok(), "RPC server");
        Ok(())
    }

    fn append_inventory_grant_calls(
        answers: &mut Vec<(&'static str, Value)>,
        checkpoint: &ServedCheckpoint,
        owner: [u8; 32],
        asset: [u8; 32],
    ) -> Result<(), String> {
        use layerx_crypto::authority_grant::{AuthorityGrant, GrantKind, GrantScope};
        let integer = |value: u64| {
            let mut word = [0; 32];
            word[24..].copy_from_slice(&value.to_be_bytes());
            word
        };
        let canonical = AuthorityGrant {
            grantor: owner,
            grantee: owner,
            kind: GrantKind::DelegatedCapability,
            delegate_key: ed25519_dalek::SigningKey::from_bytes(&[6; 32])
                .verifying_key()
                .to_bytes(),
            scope: GrantScope {
                module_mask: 2,
                ordinal_min: 5,
                ordinal_max: 5,
                asset,
                maximum_per_activity: 10,
                maximum_total: 100,
                period_length: 1000,
                maximum_per_period: 50,
                period_start: 1,
                purpose: [8; 32],
            },
            not_before: 1,
            not_after: 2_000_000_000_000,
            revocation_sequence: 1,
            fee_budget: None,
        }
        .encode()
        .map_err(|error| format!("authority producer grant: {error:?}"))?;
        let mut grant = [
            integer(96),
            integer(0),
            integer(0),
            integer(canonical.len() as u64),
        ]
        .concat();
        grant.extend(&canonical);
        grant.resize(grant.len().div_ceil(32) * 32, 0);
        stable_call(answers, checkpoint, &grant);
        let scope = [
            integer(2),
            integer(5),
            integer(5),
            asset,
            integer(10),
            integer(100),
            integer(0),
            integer(1000),
            integer(50),
            integer(0),
            integer(1),
            [8; 32],
        ]
        .concat();
        stable_call(answers, checkpoint, &scope);
        Ok(())
    }

    #[test]
    fn program_sources_select_the_chain_bound_principal_inventory() -> Result<(), String> {
        let checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let public = [4; 32];
        let did = Did::new(format!("did:layerx:{}", hex::encode(&public)).as_bytes())
            .map_err(|error| format!("DID: {error:?}"))?;
        let owner = layerx_wire::hash::did_id_for_protocol(&did, 3)
            .map_err(|error| format!("owner: {error:?}"))?;
        let asset = [7; 32];
        let account = layerx_types::account::AccountId::for_asset(
            std::str::from_utf8(did.as_bytes()).map_err(|error| format!("text: {error:?}"))?,
            asset,
            asset,
        )
        .map_err(|error| format!("account: {error:?}"))?;
        let account_id = layerx_wire::hash::account_id_for_protocol(&account, 3)
            .map_err(|error| format!("ID: {error:?}"))?;
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
        stable_call(&mut answers, &checkpoint, &integer(75));
        append_inventory_grant_calls(&mut answers, &checkpoint, owner, asset)?;
        let (endpoint, worker) = endpoint(answers);
        let reads =
            CommittedReads::open(&endpoint, 8).map_err(|error| format!("checkpoint: {error:?}"))?;
        let mut caps = reads
            .inventory(&did, &[asset], asset)
            .map_err(|error| format!("inventory: {error:?}"))?;
        assert_eq!(caps.level(), VerificationLevel::COMMITTED);
        assert_eq!(caps.level().wire_rank(), 6);
        reads
            .inventory_records(&mut caps, &[[9; 32]], &[[6; 32]])
            .map_err(|error| format!("selected budget and grant: {error:?}"))?;
        assert_eq!(caps.budgets()[0].spent_this_period, 25);
        let funding = caps
            .budget_funding(&[9; 32])
            .map_err(|error| format!("funded budget: {error:?}"))?;
        assert_eq!(funding.allowance(), 75);
        assert_eq!(funding.funded_balance(), 75);
        assert_eq!(funding.remaining(), 75);
        assert_eq!(caps.grants()[0].maximum_per_activity, 10);
        crate::budget::VerifiedResolvedProgramCharges::assert_committed_inventory(
            &caps, &did, asset, account_id,
        );
        assert_eq!(caps.all_accounts()[&account_id].balance(), 50);
        assert_eq!(caps.all_accounts()[&account_id].next_sequence, 6);
        assert_eq!(caps.state_root(), checkpoint.app_hash);
        assert!(worker.join().is_ok(), "RPC server");
        Ok(())
    }

    #[test]
    fn storage_verifies_the_generated_chain_witness_under_the_checkpoint_app_hash(
    ) -> Result<(), String> {
        use ed25519_dalek::{Signer as _, SigningKey};
        let vector: Value = serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
        )))
        .map_err(|error| format!("generated vector: {error:?}"))?;
        let entry = &vector["entries"][0];
        let decode = |key: &str| -> Result<Vec<u8>, String> {
            let value = entry[key].as_str().ok_or("hex vector")?;
            hex::decode(value).map_err(|error| format!("hex: {error:?}"))
        };
        let mut checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        checkpoint.header.app_hash = decode("app_hash")?
            .try_into()
            .map_err(|error| format!("root: {error:?}"))?;
        checkpoint.app_hash = checkpoint.header.app_hash;
        checkpoint.block_hash = checkpoint.header.block_hash();
        checkpoint.commit.block_id.hash = checkpoint.block_hash;
        for index in 0..checkpoint.commit.votes.len() {
            let message = checkpoint.commit.vote_sign_bytes(
                &checkpoint.header.chain_id,
                checkpoint.commit.votes[index].timestamp,
            );
            let seed =
                u8::try_from(index + 1).map_err(|error| format!("validator index: {error:?}"))?;
            checkpoint.commit.votes[index].signature = SigningKey::from_bytes(&[seed; 32])
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
        let reads =
            CommittedReads::open(&endpoint, 8).map_err(|error| format!("checkpoint: {error:?}"))?;
        let witness = reads
            .storage(
                decode("address")?
                    .try_into()
                    .map_err(|error| format!("address: {error:?}"))?,
                decode("slot")?
                    .try_into()
                    .map_err(|error| format!("slot: {error:?}"))?,
            )
            .map_err(|error| format!("verified storage: {error:?}"))?;
        assert_eq!(witness.value.as_slice(), decode("value")?);
        let mut tampered = witness;
        tampered.value[31] ^= 1;
        assert!(layerx_client::storage_witness::verify_storage_witness(
            &tampered,
            checkpoint.app_hash
        )
        .is_err());
        assert!(worker.join().is_ok(), "RPC server");
        Ok(())
    }

    fn program_view_requests(
        program: [u8; 32],
        asset: [u8; 32],
        seed: [u8; 3],
        account: [u8; 32],
    ) -> Vec<(&'static [u8], Vec<u8>, Vec<u8>)> {
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
        let value = [integer(50), integer(1)].concat();
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
        requests
    }

    #[test]
    fn program_views_decode_one_program_and_one_account_and_refuse_changed_settlement(
    ) -> Result<(), String> {
        use crate::budget::execution_state::read_program_records_with_account_proofs;
        let mut checkpoint = ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let program = [3; 32];
        let asset = [7; 32];
        let seed = [1, 2, 3];
        let account = layerx_programs_runtime::derive_program_account(
            layerx_programs::ProgramId::new(program)
                .map_err(|error| format!("program: {error:?}"))?,
            &seed,
        )
        .map_err(|error| format!("derived account: {error:?}"))?
        .bytes();
        let proof =
            super::program_account_fixture::committed_account(&mut checkpoint, account, asset, 50);
        assert_eq!(
            proof.header().header().resulting_state_root(),
            checkpoint.app_hash
        );
        assert!(!proof.account().frozen);
        let proofs = BTreeMap::from([(account, proof)]);
        let requests = program_view_requests(program, asset, seed, account);
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
            let reads = CommittedReads::open(&endpoint, 8)
                .map_err(|error| format!("checkpoint: {error:?}"))?;
            let result = read_program_records_with_account_proofs(
                &endpoint,
                reads.checkpoint(),
                program,
                &[account],
                &proofs,
            );
            if changed {
                assert!(matches!(
                    result,
                    Err(crate::budget::program_settlement::ProgramSettlementError::SourceSnapshot)
                ));
            } else {
                let result = result.map_err(|error| format!("program views: {error:?}"))?;
                result.assert_committed_lifecycle(program, account, asset);
                assert_eq!(result.owner, [4; 32]);
                assert_eq!(
                    result.program.as_ref().ok_or("registered")?.code_hash,
                    [5; 32]
                );
                assert_eq!(
                    result.wind_down.as_ref().ok_or("wind down")?.account_count,
                    1
                );
                assert_eq!(result.fee_schedule.fee_asset, asset);
                let binding = result.accounts.get(&account).ok_or("one account")?;
                assert_eq!(binding.seed, seed);
                assert_eq!(binding.asset, asset);
                assert_eq!(binding.registered_sequence, 2);
                assert_eq!(binding.balance, 50);
                assert!(!binding.frozen);
            }
            assert!(worker.join().is_ok(), "RPC server");
        }
        Ok(())
    }
}
