//! Verified read, proof, availability, export, and projection contract types.

use crate::identity::{Asset, ContractError};
use crate::verify::Rung;
use crate::write_contract::CanonicalBytes;
use crate::{Amount, TimestampSeconds};
use layerx_proof::export_codec::{parse_fact_set, FactRefError, FactSelector};

macro_rules! required_reference {
    ($name:ident, $field:literal) => {
        #[derive(Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
        pub struct $name(String);

        impl $name {
            /// Constructs a non-empty read reference.
            ///
            /// # Errors
            /// Returns [`ContractError::Empty`] when the reference is empty.
            pub fn new(value: impl Into<String>) -> Result<Self, ContractError> {
                let value = value.into();
                if value.is_empty() {
                    return Err(ContractError::Empty($field));
                }
                Ok(Self(value))
            }

            #[must_use]
            pub fn as_str(&self) -> &str {
                &self.0
            }
        }
    };
}

required_reference!(AccountRef, "account");
required_reference!(ModuleRef, "module");
required_reference!(BatchRef, "batch");
required_reference!(CheckpointRef, "checkpoint");
/// Ordered position within a block, without a lossy text or sequence conversion.
#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct HistoryCursor {
    pub block_height: u64,
    pub tx_index: u64,
    pub log_index: u64,
}
required_reference!(ProviderRef, "provider");
required_reference!(FactRef, "fact");

impl FactRef {
    /// Parses this reference with the single shared strict fact grammar.
    ///
    /// # Errors
    /// Returns the grammar refusal of [`FactSelector::parse`].
    pub fn selector(&self) -> Result<FactSelector, FactRefError> {
        FactSelector::parse(self.as_str())
    }
}

/// Validates an export request: 1..=16 unique facts, each in the strict shared grammar.
///
/// # Errors
/// Returns the first grammar, count or duplicate refusal of [`parse_fact_set`].
pub fn validate_export_request(
    request: ReadRequest<Vec<FactRef>>,
) -> Result<ReadRequest<Vec<FactRef>>, FactRefError> {
    let texts: Vec<&str> = request.selector.iter().map(FactRef::as_str).collect();
    parse_fact_set(texts.as_slice())?;
    Ok(request)
}

/// Checks that a verified export answers exactly the requested facts at the requested rung.
///
/// # Errors
/// Returns [`ContractError::Mismatch`] when the stated facts differ from the request,
/// [`ContractError::OutOfRange`] when the achieved rung is below the requested rung, or the
/// evidence refusal of [`OfflineExport::validate`].
pub fn check_export_response(
    request: &ReadRequest<Vec<FactRef>>,
    response: &VerifiedRead<OfflineExport>,
) -> Result<(), ContractError> {
    if response.value.facts != request.selector {
        return Err(ContractError::Mismatch("export_facts"));
    }
    if response.achieved_rung < request.requested_rung {
        return Err(ContractError::OutOfRange("export_rung"));
    }
    if response.achieved_rung != Rung::Committed {
        return Err(ContractError::OutOfRange("offline_rung"));
    }
    if response.value.height != response.freshness.value_height {
        return Err(ContractError::Mismatch("export_height"));
    }
    response.value.clone().validate()?;
    Ok(())
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RelativeTo {
    Block { height: u64, hash: [u8; 32] },
}

/// Full freshness coordinate attached to every authoritative read.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Freshness {
    pub chain_head: u64,
    pub value_height: u64,
    pub relative_to: RelativeTo,
}

mod sealed {
    pub trait Sealed {}
}

/// Values whose bytes and meaning originate from the `LayerX` core or its evidence.
pub trait CoreProduced: sealed::Sealed {}

/// An authoritative value that always carries its achieved rung and freshness.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedRead<T: CoreProduced> {
    pub value: T,
    pub achieved_rung: Rung,
    pub freshness: Freshness,
}

impl<T: CoreProduced> VerifiedRead<T> {
    #[must_use]
    pub const fn new(value: T, achieved_rung: Rung, freshness: Freshness) -> Self {
        Self {
            value,
            achieved_rung,
            freshness,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReadRequest<S> {
    pub selector: S,
    pub requested_rung: Rung,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BalanceSelector {
    pub account: AccountRef,
    pub asset: Asset,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ModuleStateSelector {
    pub module: ModuleRef,
    pub key: CanonicalBytes,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HistorySelector {
    pub from_height: u64,
    pub to_height: u64,
    pub cursor: Option<HistoryCursor>,
    pub page_limit: u32,
}

impl HistorySelector {
    /// Validates an ordered, bounded history request.
    ///
    /// # Errors
    /// Returns [`ContractError::Zero`] for an inverted range or zero page limit.
    pub fn validate(self) -> Result<Self, ContractError> {
        if self.to_height < self.from_height {
            return Err(ContractError::Zero("history_range"));
        }
        if self.page_limit == 0 {
            return Err(ContractError::Zero("page_limit"));
        }
        if self.page_limit > 65_535 {
            return Err(ContractError::OutOfRange("page_limit"));
        }
        Ok(self)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BalanceValue {
    pub account: AccountRef,
    pub asset: Asset,
    pub amount: Amount,
    pub canonical_state: CanonicalBytes,
    pub height: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AccountValue(pub CanonicalBytes);

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ModuleStateValue(pub CanonicalBytes);

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HistoryValue {
    pub records: Vec<CanonicalBytes>,
    pub next_cursor: Option<HistoryCursor>,
    pub height: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BatchValue(pub CanonicalBytes);

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CheckpointValue {
    pub commit: CanonicalBytes,
    pub validator_set: CanonicalBytes,
    pub height: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProofBundle {
    pub format: String,
    pub verification_level: Rung,
    pub raw_transaction: CanonicalBytes,
    pub signed_receipt: CanonicalBytes,
    pub receipt_digest: [u8; 32],
    pub receipt_key: layerx_client::sequencer::ReceiptKeyEntry,
    pub block_header: Option<CanonicalBytes>,
    pub receipt_inclusion: Option<CanonicalBytes>,
    pub commit: Option<CanonicalBytes>,
    pub validator_set: Option<CanonicalBytes>,
    pub account_proof: Option<CanonicalBytes>,
}

pub const MAX_PROOF_BUNDLE_BYTES: usize = 262_144;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProofBundleTarget {
    Activity([u8; 32]),
    AccountState {
        activity_id: [u8; 32],
        account_id: [u8; 32],
    },
    Receipt([u8; 32]),
}

impl ProofBundleTarget {
    /// # Errors
    /// Returns an error for a wrong selector length, version or tag, or a zero activity or account identifier.
    pub fn decode(bytes: &[u8]) -> Result<Self, ContractError> {
        if !matches!(bytes.len(), 35 | 67) || bytes[..2] != [0, 1] {
            return Err(ContractError::Mismatch("proof_bundle_target"));
        }
        let activity_id: [u8; 32] = bytes[3..35]
            .try_into()
            .map_err(|_| ContractError::Mismatch("proof_bundle_target"))?;
        if activity_id == [0; 32] {
            return Err(ContractError::Zero("activity_id"));
        }
        match (bytes[2], bytes.len()) {
            (1, 35) => Ok(Self::Activity(activity_id)),
            (3, 35) => Ok(Self::Receipt(activity_id)),
            (2, 67) => {
                let account_id: [u8; 32] = bytes[35..67]
                    .try_into()
                    .map_err(|_| ContractError::Mismatch("account_id"))?;
                if account_id == [0; 32] {
                    return Err(ContractError::Zero("account_id"));
                }
                Ok(Self::AccountState {
                    activity_id,
                    account_id,
                })
            }
            _ => Err(ContractError::Mismatch("proof_bundle_target")),
        }
    }

    #[must_use]
    pub const fn activity_id(self) -> [u8; 32] {
        match self {
            Self::Activity(id) | Self::Receipt(id) => id,
            Self::AccountState { activity_id, .. } => activity_id,
        }
    }

    /// # Errors
    /// Returns an error for a zero activity or account identifier.
    pub fn encode(self) -> Result<CanonicalBytes, ContractError> {
        let mut bytes = vec![
            0,
            1,
            match self {
                Self::Activity(_) => 1,
                Self::AccountState { .. } => 2,
                Self::Receipt(_) => 3,
            },
        ];
        bytes.extend_from_slice(&self.activity_id());
        if let Self::AccountState { account_id, .. } = self {
            bytes.extend_from_slice(&account_id);
        }
        Self::decode(&bytes)?;
        CanonicalBytes::new(bytes)
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum ProofBundleVariant {
    Activity = 1,
    Account = 2,
    Receipt = 3,
    MaintainedAccount = 4,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProofBundleRecord {
    pub variant: ProofBundleVariant,
    pub canonical_value: CanonicalBytes,
    pub native_proof: CanonicalBytes,
    pub activity_receipt: Option<(CanonicalBytes, CanonicalBytes)>,
}

impl ProofBundleRecord {
    /// # Errors
    /// Returns an error for inconsistent maintained-account receipt fields, empty fields, or an oversized encoding.
    pub fn encode(&self) -> Result<CanonicalBytes, ContractError> {
        if (self.variant == ProofBundleVariant::MaintainedAccount)
            != self.activity_receipt.is_some()
        {
            return Err(ContractError::Mismatch("proof_bundle_variant"));
        }
        let mut bytes = b"LXPB1".to_vec();
        bytes.push(self.variant as u8);
        let mut append = |field: &CanonicalBytes| -> Result<(), ContractError> {
            let value = field.as_bytes();
            if value.is_empty()
                || bytes
                    .len()
                    .checked_add(4)
                    .and_then(|n| n.checked_add(value.len()))
                    .filter(|n| *n <= MAX_PROOF_BUNDLE_BYTES)
                    .is_none()
            {
                return Err(ContractError::OutOfRange("proof_bundle_bytes"));
            }
            let length = u32::try_from(value.len())
                .map_err(|_| ContractError::OutOfRange("proof_bundle_bytes"))?;
            bytes.extend_from_slice(&length.to_be_bytes());
            bytes.extend_from_slice(value);
            Ok(())
        };
        append(&self.canonical_value)?;
        append(&self.native_proof)?;
        if let Some((receipt, proof)) = &self.activity_receipt {
            append(receipt)?;
            append(proof)?;
        }
        CanonicalBytes::new(bytes)
    }

    /// # Errors
    /// Returns an error for an invalid magic value or variant, truncated, empty or oversized fields, or trailing bytes.
    pub fn decode(bytes: &[u8]) -> Result<Self, ContractError> {
        if bytes.len() < 6 || bytes.len() > MAX_PROOF_BUNDLE_BYTES || &bytes[..5] != b"LXPB1" {
            return Err(ContractError::Mismatch("proof_bundle_record"));
        }
        let variant = match bytes[5] {
            1 => ProofBundleVariant::Activity,
            2 => ProofBundleVariant::Account,
            3 => ProofBundleVariant::Receipt,
            4 => ProofBundleVariant::MaintainedAccount,
            _ => return Err(ContractError::Mismatch("proof_bundle_variant")),
        };
        let mut offset = 6_usize;
        let mut field = || -> Result<CanonicalBytes, ContractError> {
            let length_end = offset
                .checked_add(4)
                .ok_or(ContractError::OutOfRange("proof_bundle_bytes"))?;
            let length = u32::from_be_bytes(
                bytes
                    .get(offset..length_end)
                    .ok_or(ContractError::Mismatch("proof_bundle_record"))?
                    .try_into()
                    .map_err(|_| ContractError::Mismatch("proof_bundle_record"))?,
            ) as usize;
            let end = length_end
                .checked_add(length)
                .ok_or(ContractError::OutOfRange("proof_bundle_bytes"))?;
            let value = CanonicalBytes::new(
                bytes
                    .get(length_end..end)
                    .ok_or(ContractError::Mismatch("proof_bundle_record"))?
                    .to_vec(),
            )?;
            offset = end;
            Ok(value)
        };
        let canonical_value = field()?;
        let native_proof = field()?;
        let activity_receipt = if variant == ProofBundleVariant::MaintainedAccount {
            Some((field()?, field()?))
        } else {
            None
        };
        if offset != bytes.len() {
            return Err(ContractError::Mismatch("proof_bundle_trailing"));
        }
        Ok(Self {
            variant,
            canonical_value,
            native_proof,
            activity_receipt,
        })
    }
}

impl ProofBundle {
    /// Checks the mirror's field set and rung; consumers must independently verify evidence.
    /// # Errors
    /// Returns an error for an invalid bundle, insufficient evidence for the selector or requested rung, or a response rung that disagrees with the bundle.
    pub fn check_response(
        request: &ReadRequest<ProofBundleTarget>,
        response: &VerifiedRead<Self>,
    ) -> Result<(), ContractError> {
        request.selector.encode()?;
        response.value.validate()?;
        let expected = request.selector.required_rung();
        if response.achieved_rung < expected
            || response.achieved_rung < request.requested_rung
            || response.value.verification_level != response.achieved_rung
        {
            return Err(ContractError::OutOfRange("proof_bundle_rung"));
        }
        if matches!(request.selector, ProofBundleTarget::AccountState { .. })
            && response.value.account_proof.is_none()
        {
            return Err(ContractError::Empty("account_proof"));
        }
        Ok(())
    }

    /// # Errors
    /// Returns an error for a wrong format, zero receipt key, reversed height range, invalid rung or evidence field set, or an oversized bundle.
    pub fn validate(&self) -> Result<(), ContractError> {
        if self.format != "layerx-receipt-proof-v2" {
            return Err(ContractError::Mismatch("proof_bundle_format"));
        }
        if self.receipt_key.key_id == [0; 32] || self.receipt_key.public_key == [0; 32] {
            return Err(ContractError::Zero("receipt_key"));
        }
        if self.receipt_key.first_height > self.receipt_key.last_height {
            return Err(ContractError::OutOfRange("receipt_key_heights"));
        }
        let committed = [
            &self.block_header,
            &self.receipt_inclusion,
            &self.commit,
            &self.validator_set,
        ];
        match self.verification_level {
            Rung::Submitted => return Err(ContractError::OutOfRange("proof_bundle_rung")),
            Rung::Receipted
                if committed.iter().any(|field| field.is_some())
                    || self.account_proof.is_some() =>
            {
                return Err(ContractError::Mismatch("proof_bundle_committed_fields"));
            }
            Rung::Committed if committed.iter().any(|field| field.is_none()) => {
                return Err(ContractError::Empty("proof_bundle_committed_fields"));
            }
            _ => {}
        }
        let mut size = 32_usize + 32 + 32 + 8 + 8 + self.format.len();
        for field in [
            Some(&self.raw_transaction),
            Some(&self.signed_receipt),
            self.block_header.as_ref(),
            self.receipt_inclusion.as_ref(),
            self.commit.as_ref(),
            self.validator_set.as_ref(),
            self.account_proof.as_ref(),
        ]
        .into_iter()
        .flatten()
        {
            size = size
                .checked_add(field.as_bytes().len())
                .ok_or(ContractError::OutOfRange("proof_bundle_bytes"))?;
        }
        if size > MAX_PROOF_BUNDLE_BYTES {
            return Err(ContractError::OutOfRange("proof_bundle_bytes"));
        }
        Ok(())
    }

    /// # Errors
    /// Returns an error for an invalid target, a proof count other than one, an invalid record, or a variant that disagrees with the target.
    pub fn record(
        target: &CanonicalBytes,
        proofs: &[CanonicalBytes],
    ) -> Result<ProofBundleRecord, ContractError> {
        let target = ProofBundleTarget::decode(target.as_bytes())?;
        if proofs.len() != 1 {
            return Err(ContractError::OutOfRange("proof_bundle_count"));
        }
        let record = ProofBundleRecord::decode(proofs[0].as_bytes())?;
        if !matches!(
            (target, record.variant),
            (ProofBundleTarget::Activity(_), ProofBundleVariant::Activity)
                | (ProofBundleTarget::Receipt(_), ProofBundleVariant::Receipt)
                | (
                    ProofBundleTarget::AccountState { .. },
                    ProofBundleVariant::Account | ProofBundleVariant::MaintainedAccount
                )
        ) {
            return Err(ContractError::Mismatch("proof_bundle_target_variant"));
        }
        Ok(record)
    }
}

macro_rules! core_produced {
    ($($name:ty),+ $(,)?) => {
        $(
            impl sealed::Sealed for $name {}
            impl CoreProduced for $name {}
        )+
    };
}

core_produced!(
    BalanceValue,
    AccountValue,
    ModuleStateValue,
    HistoryValue,
    BatchValue,
    CheckpointValue,
    ProofBundle,
);

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum AvailabilityClass {
    Activities,
    Receipts,
    Oracle,
    StateDiff,
    Recovery,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ClassReport {
    pub class: AvailabilityClass,
    pub complete: bool,
    pub verified_chunks: u32,
    pub verified_bytes: u64,
    pub failure: Option<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProviderReport {
    pub provider: ProviderRef,
    pub classes: Vec<ClassReport>,
    pub failure: Option<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AvailabilityCompletion {
    Complete { provider: ProviderRef },
    Partial,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AvailabilityReport {
    pub completion: AvailabilityCompletion,
    pub classes: Vec<ClassReport>,
    pub providers: Vec<ProviderReport>,
}

impl sealed::Sealed for AvailabilityReport {}
impl CoreProduced for AvailabilityReport {}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AvailabilityRequest {
    pub selector: String,
    pub requested_rung: Rung,
    pub maximum_bytes: u64,
    pub maximum_chunks: u32,
    pub deadline: TimestampSeconds,
}

impl AvailabilityRequest {
    /// Enforces finite retrieval bounds.
    ///
    /// # Errors
    /// Returns [`ContractError::Zero`] for any zero bound or empty selector.
    pub fn validate(self) -> Result<Self, ContractError> {
        if self.selector.is_empty() {
            return Err(ContractError::Empty("availability_selector"));
        }
        if self.maximum_bytes == 0 || self.maximum_chunks == 0 || self.deadline.0 == 0 {
            return Err(ContractError::Zero("availability_bound"));
        }
        Ok(self)
    }
}

/// Self-contained artifacts needed to verify a stated fact set offline.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OfflineExport {
    pub facts: Vec<FactRef>,
    pub bundles: Vec<ProofBundle>,
    pub height: u64,
}

impl sealed::Sealed for OfflineExport {}
impl CoreProduced for OfflineExport {}

impl OfflineExport {
    /// Refuses an export that states no facts or carries no verification evidence.
    ///
    /// # Errors
    /// Returns [`ContractError::Empty`] when facts or all evidence are absent.
    pub fn validate(self) -> Result<Self, ContractError> {
        if self.facts.is_empty() {
            return Err(ContractError::Empty("fact_set"));
        }
        if self.bundles.is_empty() {
            return Err(ContractError::Empty("offline_evidence"));
        }
        for bundle in &self.bundles {
            bundle.validate()?;
            if bundle.verification_level != Rung::Committed {
                return Err(ContractError::OutOfRange("offline_rung"));
            }
        }
        Ok(self)
    }
}

/// Explicitly non-authoritative estimate; it cannot satisfy [`CoreProduced`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProjectionResult<T> {
    pub projected: T,
    pub rationale: String,
    pub observed_freshness: Freshness,
}

impl<T> ProjectionResult<T> {
    /// Creates a projection with a mandatory rationale.
    ///
    /// # Errors
    /// Returns [`ContractError::Empty`] when the rationale is empty.
    pub fn new(
        projected: T,
        rationale: impl Into<String>,
        observed_freshness: Freshness,
    ) -> Result<Self, ContractError> {
        let rationale = rationale.into();
        if rationale.is_empty() {
            return Err(ContractError::Empty("projection_rationale"));
        }
        Ok(Self {
            projected,
            rationale,
            observed_freshness,
        })
    }
}

/// Hypothetical fee meter for one canonical activity, read against committed native state.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct FeeProjectionRequest {
    /// Full protocol module and ordinal packed into the canonical u32 form.
    pub protocol_activity_type: u32,
    /// Canonical activity size in bytes; zero is accepted by the native meter.
    pub canonical_bytes: u64,
    pub execution_units: u64,
    pub storage_units: u64,
}

impl FeeProjectionRequest {
    /// Enforces the native meter bound without clamping.
    ///
    /// # Errors
    /// Returns [`ContractError::OutOfRange`] when `canonical_bytes` exceeds the native maximum.
    pub const fn validate(self) -> Result<Self, ContractError> {
        if self.canonical_bytes > layerx_client::fee::MAX_FEE_METER_CANONICAL_BYTES {
            return Err(ContractError::OutOfRange("canonical_bytes"));
        }
        Ok(self)
    }

    #[must_use]
    pub const fn meter(self) -> layerx_client::fee::FeeMeter {
        layerx_client::fee::FeeMeter {
            activity_type: self.protocol_activity_type,
            canonical_bytes: self.canonical_bytes,
            execution_units: self.execution_units,
            storage_units: self.storage_units,
        }
    }
}

/// Fee computed by the committed native schedule at one captured head; never an executed fee.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FeeProjection {
    pub request: FeeProjectionRequest,
    pub parameter_version: u32,
    pub fee: Amount,
    pub gas_units: u64,
    pub denom: String,
    pub canonical_schedule: CanonicalBytes,
    /// Committed snapshot the native read answered from; equal to the captured head.
    pub snapshot_height: u64,
    pub snapshot_state_root: [u8; 32],
}

impl FeeProjection {
    /// Builds the projected value from one authenticated fee observation.
    ///
    /// # Errors
    /// Returns the meter refusal, or [`ContractError::Mismatch`] when the observation's
    /// snapshot is not the captured head.
    pub fn from_observation(
        request: FeeProjectionRequest,
        observation: layerx_client::fee::FeeObservation,
        gas_units: u64,
        denom: String,
    ) -> Result<Self, ContractError> {
        let (head, snapshot) = observation.into_parts();
        Self::from_committed_snapshot(request, head, snapshot, gas_units, denom)
    }

    /// Converts a committed fee snapshot using the RPC head captured for the read.
    ///
    /// # Errors
    /// Returns the meter refusal or rejects a stale, unversioned, or empty snapshot.
    pub fn from_committed_snapshot(
        request: FeeProjectionRequest,
        head: layerx_client::rpc::Head,
        snapshot: layerx_client::rpc::payments::CommittedSnapshot<
            layerx_client::rpc::payments::FeeEstimate,
        >,
        gas_units: u64,
        denom: String,
    ) -> Result<Self, ContractError> {
        let request = request.validate()?;
        if denom.is_empty() {
            return Err(ContractError::Empty("fee_denom"));
        }
        if snapshot.observed_sequence != head.chain_sequence {
            return Err(ContractError::Mismatch("fee_snapshot_height"));
        }
        if snapshot.value.parameter_version == 0 {
            return Err(ContractError::Zero("parameter_version"));
        }
        if snapshot.state_root == [0; 32] {
            return Err(ContractError::Zero("snapshot_state_root"));
        }
        Ok(Self {
            request,
            parameter_version: snapshot.value.parameter_version,
            fee: Amount(snapshot.value.fee),
            gas_units,
            denom,
            canonical_schedule: CanonicalBytes::new(snapshot.value.canonical_schedule)?,
            snapshot_height: snapshot.observed_sequence,
            snapshot_state_root: snapshot.state_root,
        })
    }

    /// Wraps the value as an explicitly non-authoritative projection.
    ///
    /// # Errors
    /// Returns [`ContractError::Mismatch`] when the freshness does not name this snapshot as
    /// both chain head and value height, or the rationale refusal of [`ProjectionResult::new`].
    pub fn into_projection(
        self,
        rationale: impl Into<String>,
        observed_freshness: Freshness,
    ) -> Result<ProjectionResult<Self>, ContractError> {
        if observed_freshness.chain_head != self.snapshot_height
            || observed_freshness.value_height != self.snapshot_height
        {
            return Err(ContractError::Mismatch("fee_snapshot_height"));
        }
        ProjectionResult::new(self, rationale, observed_freshness)
    }
}

#[cfg(test)]
mod proof_bundle_framing_tests {
    use super::{
        CanonicalBytes, ContractError, ProofBundle, ProofBundleRecord, ProofBundleTarget,
        ProofBundleVariant, MAX_PROOF_BUNDLE_BYTES,
    };

    #[test]
    fn target_preserves_native_pb1_and_refuses_noncanonical_selectors() -> Result<(), ContractError>
    {
        for target in [
            ProofBundleTarget::Activity([1; 32]),
            ProofBundleTarget::Receipt([2; 32]),
            ProofBundleTarget::AccountState {
                activity_id: [3; 32],
                account_id: [4; 32],
            },
        ] {
            let encoded = target.encode()?;
            assert_eq!(ProofBundleTarget::decode(encoded.as_bytes())?, target);
            let mut trailing = encoded.as_bytes().to_vec();
            trailing.push(0);
            assert!(ProofBundleTarget::decode(&trailing).is_err());
            let mut version = encoded.as_bytes().to_vec();
            version[1] = 2;
            assert!(ProofBundleTarget::decode(&version).is_err());
            let mut unknown = encoded.as_bytes().to_vec();
            unknown[2] = 4;
            assert!(ProofBundleTarget::decode(&unknown).is_err());
        }
        assert!(ProofBundleTarget::Activity([0; 32]).encode().is_err());
        assert!(ProofBundleTarget::AccountState {
            activity_id: [1; 32],
            account_id: [0; 32]
        }
        .encode()
        .is_err());
        for length in 0..35 {
            assert!(ProofBundleTarget::decode(&vec![0; length]).is_err());
        }
        Ok(())
    }

    #[test]
    fn framing_keeps_four_variants_and_maintenance_link_distinct() -> Result<(), ContractError> {
        for variant in [
            ProofBundleVariant::Activity,
            ProofBundleVariant::Receipt,
            ProofBundleVariant::Account,
            ProofBundleVariant::MaintainedAccount,
        ] {
            let record = ProofBundleRecord {
                variant,
                canonical_value: CanonicalBytes::new(vec![1])?,
                native_proof: CanonicalBytes::new(vec![2])?,
                activity_receipt: if variant == ProofBundleVariant::MaintainedAccount {
                    Some((CanonicalBytes::new(vec![3])?, CanonicalBytes::new(vec![4])?))
                } else {
                    None
                },
            };
            let encoded = record.encode()?;
            assert_eq!(ProofBundleRecord::decode(encoded.as_bytes())?, record);
            let mut trailing = encoded.as_bytes().to_vec();
            trailing.push(0);
            assert!(ProofBundleRecord::decode(&trailing).is_err());
            let mut unknown = encoded.as_bytes().to_vec();
            unknown[5] = 5;
            assert!(ProofBundleRecord::decode(&unknown).is_err());
            let mut empty = encoded.as_bytes().to_vec();
            empty[6..10].fill(0);
            assert!(ProofBundleRecord::decode(&empty).is_err());
            let mut wrong_link = record.clone();
            wrong_link.activity_receipt = if record.activity_receipt.is_some() {
                None
            } else {
                Some((CanonicalBytes::new(vec![3])?, CanonicalBytes::new(vec![4])?))
            };
            assert!(wrong_link.encode().is_err());
        }
        Ok(())
    }

    #[test]
    fn framing_refuses_oversize_duplicate_and_wrong_target_variant() -> Result<(), ContractError> {
        let mut record = ProofBundleRecord {
            variant: ProofBundleVariant::Receipt,
            canonical_value: CanonicalBytes::new(vec![1; MAX_PROOF_BUNDLE_BYTES - 15])?,
            native_proof: CanonicalBytes::new(vec![2])?,
            activity_receipt: None,
        };
        assert_eq!(record.encode()?.as_bytes().len(), MAX_PROOF_BUNDLE_BYTES);
        record.canonical_value = CanonicalBytes::new(vec![1; MAX_PROOF_BUNDLE_BYTES - 14])?;
        assert!(record.encode().is_err());
        assert!(ProofBundleRecord::decode(&vec![1; MAX_PROOF_BUNDLE_BYTES + 1]).is_err());
        record.canonical_value = CanonicalBytes::new(vec![1])?;
        let encoded = record.encode()?;
        let mut target = ProofBundleTarget::Receipt([1; 32]).encode()?;
        let mut proofs = vec![encoded.clone()];
        assert!(ProofBundle::record(&target, &proofs).is_ok());
        proofs.push(encoded);
        assert!(ProofBundle::record(&target, &proofs).is_err());
        proofs.pop();
        target = ProofBundleTarget::Activity([1; 32]).encode()?;
        assert!(ProofBundle::record(&target, &proofs).is_err());
        Ok(())
    }
}

// The daemon's RPC and retained-export codecs share this boundary. Keeping the
// codec on the public contract makes structural checks identical for both consumers;
// these checks do not replace independent signature, inclusion or commit verification.
mod wire {
    use super::{CanonicalBytes, ContractError, Freshness, ProofBundle, RelativeTo, Rung};
    use layerx_types::json::{self, JsonValue};

    pub const MAX_FRAME: usize = 1_048_576;

    pub fn malformed() -> ContractError {
        ContractError::Malformed("agent_api_wire")
    }

    /// # Errors
    /// Returns an error for an oversized frame or malformed JSON.
    pub fn parse(text: &str) -> Result<JsonValue, ContractError> {
        if text.len() > MAX_FRAME {
            return Err(ContractError::OutOfRange("agent_api_frame"));
        }
        json::parse(text).map_err(|_| malformed())
    }

    /// # Errors
    /// Returns a malformed-wire error for a non-object, a missing required field, or an unknown field.
    pub fn fields<'a>(
        value: &'a JsonValue,
        required: &[&str],
        optional: &[&str],
    ) -> Result<&'a JsonValue, ContractError> {
        let JsonValue::Object(fields) = value else {
            return Err(malformed());
        };
        if required.iter().any(|name| value.field(name).is_none())
            || fields.iter().any(|(name, _)| {
                !required.contains(&name.as_str()) && !optional.contains(&name.as_str())
            })
        {
            return Err(malformed());
        }
        Ok(value)
    }

    /// # Errors
    /// Returns a malformed-wire error for a missing field or a value that is not a string.
    pub fn text<'a>(value: &'a JsonValue, name: &str) -> Result<&'a str, ContractError> {
        value.str_at(name).map_err(|_| malformed())
    }

    /// # Errors
    /// Returns a malformed-wire error for a missing field or a noncanonical or out-of-range decimal u64.
    pub fn decimal(value: &JsonValue, name: &str) -> Result<u64, ContractError> {
        let text = text(value, name)?;
        let number: u64 = text.parse().map_err(|_| malformed())?;
        if number.to_string() != text {
            return Err(malformed());
        }
        Ok(number)
    }

    pub fn hex(bytes: &[u8]) -> String {
        const DIGITS: &[u8; 16] = b"0123456789abcdef";
        let mut text = String::from("0x");
        for byte in bytes {
            text.push(char::from(DIGITS[usize::from(byte >> 4)]));
            text.push(char::from(DIGITS[usize::from(byte & 15)]));
        }
        text
    }

    /// # Errors
    /// Returns an error for missing, noncanonical or empty lowercase hexadecimal bytes.
    pub fn bytes(value: &JsonValue, name: &str) -> Result<CanonicalBytes, ContractError> {
        let text = text(value, name)?
            .strip_prefix("0x")
            .ok_or_else(malformed)?;
        if text.len() % 2 != 0
            || !text
                .bytes()
                .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
        {
            return Err(malformed());
        }
        CanonicalBytes::new(json::decode_hex(&format!("0x{text}")).map_err(|_| malformed())?)
    }

    /// # Errors
    /// Returns an error for invalid hexadecimal bytes or a decoded length different from N.
    pub fn array<const N: usize>(value: &JsonValue, name: &str) -> Result<[u8; N], ContractError> {
        bytes(value, name)?
            .as_bytes()
            .try_into()
            .map_err(|_| malformed())
    }

    /// # Errors
    /// Returns an error for malformed wire fields, an unknown rung, or a bundle that fails structural validation.
    pub fn bundle(value: &JsonValue) -> Result<ProofBundle, ContractError> {
        fields(
            value,
            &[
                "format",
                "verificationLevel",
                "rawTransaction",
                "signedReceipt",
                "receiptDigest",
                "receiptKey",
            ],
            &[
                "blockHeader",
                "receiptInclusion",
                "commit",
                "validatorSet",
                "accountProof",
            ],
        )?;
        let key = value.field("receiptKey").ok_or_else(malformed)?;
        fields(
            key,
            &["keyId", "publicKey", "firstHeight", "lastHeight"],
            &[],
        )?;
        let optional = |name| value.field(name).map(|_| bytes(value, name)).transpose();
        let bundle = ProofBundle {
            format: text(value, "format")?.to_owned(),
            verification_level: Rung::from_name(text(value, "verificationLevel")?)
                .ok_or_else(malformed)?,
            raw_transaction: bytes(value, "rawTransaction")?,
            signed_receipt: bytes(value, "signedReceipt")?,
            receipt_digest: array(value, "receiptDigest")?,
            receipt_key: layerx_client::sequencer::ReceiptKeyEntry {
                key_id: array(key, "keyId")?,
                public_key: array(key, "publicKey")?,
                first_height: decimal(key, "firstHeight")?,
                last_height: decimal(key, "lastHeight")?,
            },
            block_header: optional("blockHeader")?,
            receipt_inclusion: optional("receiptInclusion")?,
            commit: optional("commit")?,
            validator_set: optional("validatorSet")?,
            account_proof: optional("accountProof")?,
        };
        bundle.validate()?;
        Ok(bundle)
    }

    /// # Errors
    /// Returns an error for malformed freshness fields, an invalid block hash, or noncanonical heights.
    pub fn freshness(value: &JsonValue) -> Result<Freshness, ContractError> {
        fields(value, &["chain_head", "value_height", "relative_to"], &[])?;
        let block = value.field("relative_to").ok_or_else(malformed)?;
        fields(block, &["height", "hash"], &[])?;
        Ok(Freshness {
            chain_head: decimal(value, "chain_head")?,
            value_height: decimal(value, "value_height")?,
            relative_to: RelativeTo::Block {
                height: decimal(block, "height")?,
                hash: array(block, "hash")?,
            },
        })
    }
}

impl Freshness {
    /// Losslessly renders the same coordinates used by the RPC adapter.
    #[must_use]
    pub fn encode_json(&self) -> String {
        let RelativeTo::Block { height, hash } = self.relative_to;
        format!(
            r#"{{"chain_head":"{}","value_height":"{}","relative_to":{{"height":"{height}","hash":"{}"}}}}"#,
            self.chain_head,
            self.value_height,
            wire::hex(&hash)
        )
    }

    /// Decodes exact decimal heights and the complete block reference.
    ///
    /// # Errors
    /// Refuses unknown fields, noncanonical numbers and malformed hashes.
    pub fn decode_json(text: &str) -> Result<Self, ContractError> {
        wire::freshness(&wire::parse(text)?)
    }
}

impl ReadRequest<CanonicalBytes> {
    /// Checks the response at the RPC owner's typed crossing.
    ///
    /// # Errors
    /// Refuses malformed selectors, incomplete evidence and insufficient rungs.
    pub fn check_proof_response(
        &self,
        response: &VerifiedRead<ProofBundle>,
    ) -> Result<(), ContractError> {
        ProofBundle::check_response(
            &ReadRequest {
                selector: ProofBundleTarget::decode(self.selector.as_bytes())?,
                requested_rung: self.requested_rung,
            },
            response,
        )
    }

    /// Converts the RPC's canonical selector bytes into the public typed request.
    ///
    /// # Errors
    /// Refuses malformed selectors and retired or unknown rung names.
    pub fn proof_bundle(target: Vec<u8>, rung: &str) -> Result<Self, ContractError> {
        ProofBundleTarget::decode(&target)?;
        Ok(Self {
            selector: CanonicalBytes::new(target)?,
            requested_rung: Rung::from_name(rung)
                .ok_or(ContractError::Malformed("requested_rung"))?,
        })
    }
}

impl ProofBundleTarget {
    /// Minimum evidence the consumer needs for this selector.
    #[must_use]
    pub const fn required_rung(self) -> Rung {
        match self {
            Self::AccountState { .. } => Rung::Committed,
            _ => Rung::Receipted,
        }
    }
}

impl ProofBundle {
    /// Serializes the receipt-proof-v2 field set used by RPC and retained exports.
    ///
    /// # Errors
    /// Refuses invalid field combinations and oversized evidence.
    pub fn encode_json(&self) -> Result<String, ContractError> {
        self.validate()?;
        let key = self.receipt_key;
        let mut text = format!(
            r#"{{"format":"layerx-receipt-proof-v2","verificationLevel":"{}","rawTransaction":"{}","signedReceipt":"{}","receiptDigest":"{}","receiptKey":{{"keyId":"{}","publicKey":"{}","firstHeight":"{}","lastHeight":"{}"}}"#,
            self.verification_level.name(),
            wire::hex(self.raw_transaction.as_bytes()),
            wire::hex(self.signed_receipt.as_bytes()),
            wire::hex(&self.receipt_digest),
            wire::hex(&key.key_id),
            wire::hex(&key.public_key),
            key.first_height,
            key.last_height
        );
        for (name, bytes) in [
            ("blockHeader", &self.block_header),
            ("receiptInclusion", &self.receipt_inclusion),
            ("commit", &self.commit),
            ("validatorSet", &self.validator_set),
            ("accountProof", &self.account_proof),
        ] {
            if let Some(bytes) = bytes {
                text.push_str(",\"");
                text.push_str(name);
                text.push_str("\":\"");
                text.push_str(&wire::hex(bytes.as_bytes()));
                text.push('"');
            }
        }
        text.push('}');
        Ok(text)
    }

    /// Structural consumer of the exact receipt-proof-v2 wire item.
    /// Signatures and commitments still require the consumer's independently pinned trust.
    ///
    /// # Errors
    /// Refuses malformed, trailing, duplicate, unknown, missing or inconsistent fields.
    pub fn decode_json(text: &str) -> Result<Self, ContractError> {
        wire::bundle(&wire::parse(text)?)
    }
}

impl VerifiedRead<ProofBundle> {
    /// Checks the producer's envelope before either RPC adapter consumes it.
    ///
    /// # Errors
    /// Refuses a malformed bundle or disagreement between the envelope and bundle rung.
    pub fn validate(&self) -> Result<(), ContractError> {
        self.value.validate()?;
        if self.achieved_rung != self.value.verification_level {
            return Err(ContractError::Mismatch("proof_bundle_rung"));
        }
        Ok(())
    }

    /// Encodes all authoritative coordinates, including the block hash.
    ///
    /// # Errors
    /// Refuses an inconsistent envelope or a frame exceeding the HTTP bound.
    pub fn encode_json(&self) -> Result<String, ContractError> {
        self.validate()?;
        let text = format!(
            r#"{{"value":{},"achieved_verification_level":"{}","freshness":{}}}"#,
            self.value.encode_json()?,
            self.achieved_rung.name(),
            self.freshness.encode_json()
        );
        if text.len() > wire::MAX_FRAME {
            return Err(ContractError::OutOfRange("agent_api_frame"));
        }
        Ok(text)
    }

    /// Consumes an authoritative response using the same checks as the daemon adapter.
    ///
    /// # Errors
    /// Refuses malformed data and disagreements between the nested and achieved rungs.
    pub fn decode_json(text: &str) -> Result<Self, ContractError> {
        let value = wire::parse(text)?;
        wire::fields(
            &value,
            &["value", "achieved_verification_level", "freshness"],
            &[],
        )?;
        let response = Self::new(
            wire::bundle(value.field("value").ok_or_else(wire::malformed)?)?,
            Rung::from_name(wire::text(&value, "achieved_verification_level")?)
                .ok_or_else(wire::malformed)?,
            wire::freshness(value.field("freshness").ok_or_else(wire::malformed)?)?,
        );
        response.validate()?;
        Ok(response)
    }
}

impl OfflineExport {
    /// Encodes the portable export consumed by the daemon's durable export reader.
    ///
    /// # Errors
    /// Refuses an invalid export, fact grammar or frame exceeding the HTTP bound.
    pub fn encode_json(&self) -> Result<String, ContractError> {
        self.clone().validate()?;
        parse_fact_set(&self.facts.iter().map(FactRef::as_str).collect::<Vec<_>>())
            .map_err(|_| ContractError::Malformed("fact_set"))?;
        let facts = self
            .facts
            .iter()
            .map(|fact| format!("{:?}", fact.as_str()))
            .collect::<Vec<_>>()
            .join(",");
        let bundles = self
            .bundles
            .iter()
            .map(ProofBundle::encode_json)
            .collect::<Result<Vec<_>, _>>()?
            .join(",");
        let text = format!(
            r#"{{"facts":[{facts}],"bundles":[{bundles}],"height":"{}"}}"#,
            self.height
        );
        if text.len() > wire::MAX_FRAME {
            return Err(ContractError::OutOfRange("agent_api_frame"));
        }
        Ok(text)
    }

    /// Decodes the producer's portable bytes at the retained-export boundary.
    ///
    /// # Errors
    /// Refuses malformed fields, fact grammar and incomplete committed evidence.
    pub fn decode_json(text: &str) -> Result<Self, ContractError> {
        use layerx_types::json::JsonValue;
        let value = wire::parse(text)?;
        wire::fields(&value, &["facts", "bundles", "height"], &[])?;
        let facts = value
            .array_at("facts")
            .map_err(|_| wire::malformed())?
            .iter()
            .map(|fact| {
                let JsonValue::String(text) = fact else {
                    return Err(wire::malformed());
                };
                FactRef::new(text.clone())
            })
            .collect::<Result<Vec<_>, _>>()?;
        parse_fact_set(&facts.iter().map(FactRef::as_str).collect::<Vec<_>>())
            .map_err(|_| ContractError::Malformed("fact_set"))?;
        let bundles = value
            .array_at("bundles")
            .map_err(|_| wire::malformed())?
            .iter()
            .map(wire::bundle)
            .collect::<Result<_, _>>()?;
        Self {
            facts,
            bundles,
            height: wire::decimal(&value, "height")?,
        }
        .validate()
    }
}
