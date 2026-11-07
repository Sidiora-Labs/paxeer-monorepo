use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::{
    decode_signed, decode_unsigned, recover_sender, transaction_hash,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::budget::{
    BudgetLimiter, ProgramBudgetReservation, ProgramChargeKind, ReleaseKind,
    StagedProgramReservation,
};
use crate::human::HumanPeer;
use crate::outbox::Outbox;
use crate::prepare::EvmPrepared;
use crate::sign::{SigningPreparation, VerifiedSubmission};
use crate::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};

const EVM_RECEIPT_MODULE: u16 = 12;
const MAX_RECORD: usize = 4 * 1024 * 1024;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum CallApprovalError {
    Missing,
    Binding,
    Version,
    Corrupt,
    Budget,
    Store,
    Receipt,
    DebitProofRequired,
}

#[derive(Clone, Copy, Debug, Serialize, Deserialize, Eq, PartialEq)]
pub enum CallApprovalState {
    Awaiting,
    Granted,
    Rejected,
    Expired,
    Signed,
    Queued,
    Settled,
}

#[derive(Clone, Serialize, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub(crate) struct ReceiptKey {
    id: [u8; 32],
    public_key: [u8; 32],
    first: u64,
    last: u64,
}
impl ReceiptKey {
    fn actual(&self) -> SequencerAuthorization {
        SequencerAuthorization::new(self.id, self.public_key, self.first, self.last)
    }
}

#[derive(Clone, Serialize, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub(crate) struct CallDecision {
    pub key: [u8; 32],
    pub grant: bool,
    pub sequence: u64,
    pub core_ms: u64,
}

#[derive(Clone, Serialize, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub(crate) struct RetainedNativeCall {
    pub version: u8,
    pub tenant: String,
    pub principal: String,
    pub owner_did: String,
    pub owner_account: String,
    pub session: [u8; 32],
    pub generation: u64,
    pub sender: [u8; 20],
    pub canonical_bytes: Vec<u8>,
    pub preparation: [u8; 32],
    pub disclosure_digest: [u8; 32],
    pub observed_height: u64,
    pub admitted_at_ms: u64,
    pub budget: Vec<u8>,
    receipt_keys: Vec<ReceiptKey>,
    pub decision: Option<CallDecision>,
    pub signed: Option<Vec<u8>>,
    pub queued: bool,
    pub settlement: Option<Vec<u8>>,
    pub debit_settlement: bool,
}

impl RetainedNativeCall {
    pub fn restore(&self) -> Result<EvmPrepared, CallApprovalError> {
        if self.version != 1 {
            return Err(CallApprovalError::Version);
        }
        let call =
            decode_unsigned(&self.canonical_bytes).map_err(|_| CallApprovalError::Corrupt)?;
        let disclosure = bind_call(&call, self.sender).map_err(|_| CallApprovalError::Binding)?;
        if self.sender == [0; 20]
            || self.preparation != <[u8; 32]>::from(Sha256::digest(&self.canonical_bytes))
            || disclosure
                .audit_digest()
                .map_err(|_| CallApprovalError::Binding)?
                != self.disclosure_digest
        {
            return Err(CallApprovalError::Binding);
        }
        let signing_hash = call.signing_hash();
        let disclosure_digest = disclosure
            .signing_digest()
            .map_err(|_| CallApprovalError::Binding)?;
        let prepared = EvmPrepared {
            call,
            from: self.sender,
            disclosure,
            disclosure_digest,
            signing_hash,
            observed_head_height: self.observed_height,
        };
        crate::prepare::verify_call_binding(&prepared).map_err(|_| CallApprovalError::Binding)?;
        Ok(prepared)
    }

    pub fn budget(&self) -> Result<ProgramBudgetReservation, CallApprovalError> {
        ProgramBudgetReservation::decode(&self.budget).map_err(|_| CallApprovalError::Budget)
    }

    pub fn deadline(&self) -> Result<u64, CallApprovalError> {
        let prepared = self.restore()?;
        let budget = self.budget()?;
        Ok(prepared
            .disclosure
            .expiry
            .payload_expires_at
            .min(budget.core_deadline.ok_or(CallApprovalError::Budget)?.0))
    }

    pub fn state(&self) -> Result<CallApprovalState, CallApprovalError> {
        if self.settlement.is_some() {
            return Ok(CallApprovalState::Settled);
        }
        if self.queued {
            return Ok(CallApprovalState::Queued);
        }
        if self.signed.is_some() {
            return Ok(CallApprovalState::Signed);
        }
        let Some(decision) = &self.decision else {
            return Ok(CallApprovalState::Awaiting);
        };
        if decision.sequence >= self.budget()?.expiry_sequence
            || decision.core_ms >= self.deadline()?
        {
            Ok(CallApprovalState::Expired)
        } else if decision.grant {
            Ok(CallApprovalState::Granted)
        } else {
            Ok(CallApprovalState::Rejected)
        }
    }

    pub fn immutable_bytes(&self) -> Result<Vec<u8>, CallApprovalError> {
        let mut immutable = self.clone();
        immutable.decision = None;
        immutable.signed = None;
        immutable.queued = false;
        immutable.settlement = None;
        immutable.debit_settlement = false;
        let mut bytes = b"layerx/native-call-approval/v1\0".to_vec();
        bytes.extend(serde_json::to_vec(&immutable).map_err(|_| CallApprovalError::Corrupt)?);
        Ok(bytes)
    }

    pub fn held_digest(&self) -> Result<[u8; 32], CallApprovalError> {
        Ok(Sha256::digest(self.immutable_bytes()?).into())
    }

    fn receipt(&self, bytes: &[u8]) -> Result<layerx_wire::receipt::Receipt, CallApprovalError> {
        let keys: Vec<_> = self.receipt_keys.iter().map(ReceiptKey::actual).collect();
        let receipt = verify_sequencer_signature(bytes, keys.as_slice())
            .map_err(|_| CallApprovalError::Receipt)?;
        let protocol = receipt.protocol().ok_or(CallApprovalError::Receipt)?;
        let signed = self.signed.as_ref().ok_or(CallApprovalError::Binding)?;
        let prepared = self.restore()?;
        let mut from = [0; 32];
        from[12..].copy_from_slice(&self.sender);
        let mut to = [0; 32];
        to[12..].copy_from_slice(&prepared.call.to);
        let summary = layerx_proof::sequencer_logs::decode_summary(&receipt)
            .map_err(|_| CallApprovalError::Receipt)?;
        if !self.queued
            || protocol.activity_id() != transaction_hash(signed)
            || protocol.module_id() != EVM_RECEIPT_MODULE
            || protocol.operation() != 2
            || protocol.from() != from
            || protocol.to() != to
            || protocol.debit_sequence() != prepared.call.nonce
            || protocol.amount() != 0
            || summary.gas_used() > prepared.call.gas_limit
            || u128::from(summary.effective_gas_price()) > prepared.call.max_fee_per_gas
            || protocol.global_sequence() == 0
            || protocol.batch_id()[..24] != [0; 24]
            || u64::from_be_bytes(
                protocol.batch_id()[24..]
                    .try_into()
                    .map_err(|_| CallApprovalError::Receipt)?,
            ) <= self.observed_height
            || protocol.timestamp() < self.admitted_at_ms
        {
            return Err(CallApprovalError::Receipt);
        }
        Ok(receipt)
    }

    fn exact_fee_debit(
        &self,
        receipt: &layerx_wire::receipt::Receipt,
    ) -> Result<(), CallApprovalError> {
        let reservation = self.budget()?;
        let protocol = receipt.protocol().ok_or(CallApprovalError::Receipt)?;
        let summary = layerx_proof::sequencer_logs::decode_summary(receipt)
            .map_err(|_| CallApprovalError::Receipt)?;
        let mut source = [0; 32];
        source[12..].copy_from_slice(&self.sender);
        if reservation.allocations().is_some()
            || reservation.charges.len() != 1
            || reservation.charges[0].kind != ProgramChargeKind::Fee
            || summary.sponsor() != [0; 20]
            || reservation.charges[0].source != source
            || reservation.charges[0].asset
                != <[u8; 32]>::from(Sha256::digest(summary.fee_denom().as_bytes()))
            || reservation.charges[0].amount != protocol.fee_charged()
        {
            return Err(CallApprovalError::DebitProofRequired);
        }
        Ok(())
    }

    pub fn validate(&self, program: bool) -> Result<(), CallApprovalError> {
        let prepared = self.restore()?;
        let budget = self.budget()?;
        let fees: Vec<_> = budget
            .charges
            .iter()
            .filter(|charge| charge.kind == ProgramChargeKind::Fee)
            .collect();
        if fees.len() != 1 || fees[0].amount != prepared.disclosure.fee_limit {
            return Err(CallApprovalError::Budget);
        }
        if self.tenant.is_empty()
            || self.principal.is_empty()
            || self.principal.len() > 255
            || self.owner_did.is_empty()
            || self.owner_account.is_empty()
            || self.session == [0; 32]
            || self.generation == 0
            || self.admitted_at_ms == 0
            || budget.id != self.preparation
            || budget.expiry_sequence <= self.observed_height
            || self.admitted_at_ms >= self.deadline()?
            || (prepared.disclosure.activity_type.module()
                == layerx_types::payload::ModuleId::Programs)
                != program
            || self.receipt_keys.is_empty()
            || self.receipt_keys.len() > 64
            || self
                .receipt_keys
                .iter()
                .any(|key| key.id == [0; 32] || key.public_key == [0; 32] || key.first > key.last)
            || self
                .receipt_keys
                .iter()
                .map(|key| key.id)
                .collect::<std::collections::BTreeSet<_>>()
                .len()
                != self.receipt_keys.len()
        {
            return Err(CallApprovalError::Binding);
        }
        TenantId::new(self.tenant.clone()).map_err(|_| CallApprovalError::Binding)?;
        if let Some(decision) = &self.decision {
            if decision.key == [0; 32]
                || decision.sequence < self.observed_height
                || decision.core_ms < self.admitted_at_ms
            {
                return Err(CallApprovalError::Binding);
            }
        }
        if let Some(bytes) = &self.signed {
            let decision = self.decision.as_ref().ok_or(CallApprovalError::Binding)?;
            if !decision.grant
                || decision.sequence >= budget.expiry_sequence
                || decision.core_ms >= self.deadline()?
            {
                return Err(CallApprovalError::Binding);
            }
            let (call, signature) = decode_signed(bytes).map_err(|_| CallApprovalError::Binding)?;
            if call != prepared.call
                || recover_sender(&call, &signature).map_err(|_| CallApprovalError::Binding)?
                    != self.sender
            {
                return Err(CallApprovalError::Binding);
            }
        } else if self.queued || self.settlement.is_some() {
            return Err(CallApprovalError::Binding);
        }
        if self.debit_settlement && self.settlement.is_none() {
            return Err(CallApprovalError::Binding);
        }
        if let Some(receipt) = &self.settlement {
            let decoded = self.receipt(receipt)?;
            if !self.debit_settlement {
                self.exact_fee_debit(&decoded)?;
            }
        }
        Ok(())
    }
}

pub(crate) trait CallCarrier: Clone + Eq + Serialize + for<'de> Deserialize<'de> {
    const PROGRAM: bool;
    const PREFIX: &'static [u8];
    fn from_call(call: RetainedNativeCall) -> Result<Self, CallApprovalError>;
    fn call(&self) -> Option<&RetainedNativeCall>;
    fn stage_unsigned_release<'a>(
        &self,
        budgets: &'a BudgetLimiter,
    ) -> Result<crate::budget::StagedRelease<'a>, CallApprovalError>;
}

pub(crate) fn validate<C: CallCarrier>(carrier: &C) -> Result<bool, CallApprovalError> {
    let Some(call) = carrier.call() else {
        return Ok(false);
    };
    call.validate(C::PROGRAM)?;
    if carrier != &C::from_call(call.clone())? {
        return Err(CallApprovalError::Binding);
    }
    Ok(true)
}

fn owner(peer: &HumanPeer) -> Result<TenantId, CallApprovalError> {
    let subject = peer.subject.as_ref().ok_or(CallApprovalError::Binding)?;
    let namespace =
        layerx_identity_binding::subject_namespace(&subject.transport_tenant, &peer.principal)
            .map_err(|_| CallApprovalError::Binding)?;
    let account = layerx_types::account::AccountId::parse(&subject.account)
        .map_err(|_| CallApprovalError::Binding)?;
    layerx_types::ids::Did::new(subject.owner.as_bytes())
        .map_err(|_| CallApprovalError::Binding)?;
    if peer.uid == 0
        || subject.transport_principal.is_empty()
        || peer.tenant != namespace
        || account.canonical() != subject.account
        || !subject
            .account
            .starts_with(&format!("agent:{}:", subject.owner))
    {
        return Err(CallApprovalError::Binding);
    }
    TenantId::new(peer.tenant.clone()).map_err(|_| CallApprovalError::Binding)
}

pub(crate) fn key<C: CallCarrier>(
    tenant: TenantId,
    id: [u8; 32],
) -> Result<TenantKey, CallApprovalError> {
    TenantKey::new(
        tenant,
        ObjectKind::PreparedActivity,
        [C::PREFIX, &id].concat(),
    )
    .map_err(|_| CallApprovalError::Binding)
}
fn binding_key<C: CallCarrier>(
    tenant: TenantId,
    id: [u8; 32],
) -> Result<TenantKey, CallApprovalError> {
    TenantKey::new(
        tenant,
        ObjectKind::PreparedActivity,
        [
            b"native-call-approval-binding-v1:".as_slice(),
            C::PREFIX,
            &id,
        ]
        .concat(),
    )
    .map_err(|_| CallApprovalError::Binding)
}
fn encoded<C: CallCarrier>(carrier: &C) -> Result<Vec<u8>, CallApprovalError> {
    if !validate(carrier)? {
        return Err(CallApprovalError::Version);
    }
    let bytes = serde_json::to_vec(carrier).map_err(|_| CallApprovalError::Corrupt)?;
    if bytes.len() > MAX_RECORD {
        return Err(CallApprovalError::Corrupt);
    }
    Ok(bytes)
}
fn write<C: CallCarrier>(
    store: &mut Store,
    carrier: &C,
    mut updates: Vec<(TenantKey, Vec<u8>)>,
) -> Result<(), CallApprovalError> {
    let call = carrier.call().ok_or(CallApprovalError::Version)?;
    if C::PROGRAM {
        updates.push(super::program_requirement::call_companion(call)?);
    }
    updates.push((
        key::<C>(
            TenantId::new(call.tenant.clone()).map_err(|_| CallApprovalError::Binding)?,
            call.preparation,
        )?,
        encoded(carrier)?,
    ));
    store
        .update_local_batch(updates)
        .map_err(|_| CallApprovalError::Store)
}

pub(crate) fn read<C: CallCarrier>(
    store: &Store,
    peer: &HumanPeer,
    id: [u8; 32],
) -> Result<C, CallApprovalError> {
    let tenant = owner(peer)?;
    let raw = store
        .get(&key::<C>(tenant.clone(), id)?)
        .ok_or(CallApprovalError::Missing)?;
    if raw.class() != StorageClass::LocalOnly || raw.bytes().len() > MAX_RECORD {
        return Err(CallApprovalError::Corrupt);
    }
    let carrier: C = serde_json::from_slice(raw.bytes()).map_err(|_| CallApprovalError::Corrupt)?;
    if encoded(&carrier)?.as_slice() != raw.bytes() {
        return Err(CallApprovalError::Corrupt);
    }
    let call = carrier.call().ok_or(CallApprovalError::Version)?;
    let subject = peer.subject.as_ref().ok_or(CallApprovalError::Binding)?;
    if call.preparation != id
        || call.tenant != tenant.as_str()
        || call.principal != peer.principal
        || call.owner_did != subject.owner
        || call.owner_account != subject.account
    {
        return Err(CallApprovalError::Binding);
    }
    let binding = store
        .get(&binding_key::<C>(tenant.clone(), id)?)
        .ok_or(CallApprovalError::Missing)?;
    if binding.class() != StorageClass::LocalOnly || binding.bytes() != call.held_digest()? {
        return Err(CallApprovalError::Binding);
    }
    if C::PROGRAM {
        super::program_requirement::check_call(store, call)?;
    }
    if call.debit_settlement
        && !crate::budget::program_settlement_recorded(store, &tenant, id)
            .map_err(|_| CallApprovalError::Budget)?
    {
        return Err(CallApprovalError::Binding);
    }
    Ok(carrier)
}

pub(crate) fn hold<C: CallCarrier>(
    store: &mut Store,
    peer: &HumanPeer,
    session: [u8; 32],
    generation: u64,
    prepared: &EvmPrepared,
    staged: StagedProgramReservation<'_>,
    admitted_at_ms: u64,
    receipt_keys: &[SequencerAuthorization],
) -> Result<C, CallApprovalError> {
    crate::prepare::verify_call_binding(prepared).map_err(|_| CallApprovalError::Binding)?;
    let tenant = owner(peer)?;
    let canonical_bytes = prepared.call.encode_unsigned();
    let preparation = Sha256::digest(&canonical_bytes).into();
    let subject = peer.subject.as_ref().ok_or(CallApprovalError::Binding)?;
    let call = RetainedNativeCall {
        version: 1,
        owner_did: subject.owner.clone(),
        owner_account: subject.account.clone(),
        tenant: tenant.as_str().to_owned(),
        principal: peer.principal.clone(),
        session,
        generation,
        sender: prepared.from,
        canonical_bytes,
        preparation,
        disclosure_digest: prepared
            .disclosure
            .audit_digest()
            .map_err(|_| CallApprovalError::Binding)?,
        observed_height: prepared.observed_head_height,
        admitted_at_ms,
        budget: staged
            .record()
            .encode()
            .map_err(|_| CallApprovalError::Budget)?,
        receipt_keys: receipt_keys
            .iter()
            .map(|key| ReceiptKey {
                id: key.sequencer_id(),
                public_key: key.public_key(),
                first: key.first_batch_number(),
                last: key.last_batch_number(),
            })
            .collect(),
        decision: None,
        signed: None,
        queued: false,
        settlement: None,
        debit_settlement: false,
    };
    let carrier = C::from_call(call)?;
    let bytes = encoded(&carrier)?;
    let binding = carrier
        .call()
        .ok_or(CallApprovalError::Version)?
        .held_digest()?
        .to_vec();
    let mut inserts = vec![
        (key::<C>(tenant.clone(), preparation)?, bytes),
        (binding_key::<C>(tenant, preparation)?, binding),
    ];
    if C::PROGRAM {
        inserts.push(super::program_requirement::call_companion(
            carrier.call().ok_or(CallApprovalError::Version)?,
        )?);
    }
    store
        .apply_program_approval_batch(Vec::new(), inserts, Vec::new())
        .map_err(|_| CallApprovalError::Store)?;
    let _ = staged.publish();
    Ok(carrier)
}

pub(crate) fn decide<C: CallCarrier>(
    store: &mut Store,
    budgets: &BudgetLimiter,
    peer: &HumanPeer,
    id: [u8; 32],
    digest: [u8; 32],
    decision_key: [u8; 32],
    grant: bool,
    sequence: u64,
    core_ms: u64,
) -> Result<C, CallApprovalError> {
    let held: C = read(store, peer, id)?;
    let mut call = held.call().ok_or(CallApprovalError::Version)?.clone();
    if call.held_digest()? != digest || decision_key == [0; 32] {
        return Err(CallApprovalError::Binding);
    }
    if let Some(decision) = &call.decision {
        if decision.key == decision_key && decision.grant == grant {
            return Ok(held);
        }
        return Err(CallApprovalError::Binding);
    }
    call.decision = Some(CallDecision {
        key: decision_key,
        grant,
        sequence,
        core_ms,
    });
    let state = call.state()?;
    let carrier = C::from_call(call)?;
    if matches!(
        state,
        CallApprovalState::Rejected | CallApprovalState::Expired
    ) {
        let staged = carrier.stage_unsigned_release(budgets)?;
        if !staged.found() {
            return Err(CallApprovalError::Budget);
        }
        write(store, &carrier, Vec::new())?;
        let _ = staged.publish();
    } else {
        write(store, &carrier, Vec::new())?;
    }
    Ok(carrier)
}

pub(crate) fn queue<C: CallCarrier>(
    store: &mut Store,
    outbox: &mut Outbox,
    peer: &HumanPeer,
    id: [u8; 32],
    verified: &VerifiedSubmission,
    registry: &layerx_types::payload::ModuleRegistry,
    sequence: u64,
    core_ms: u64,
) -> Result<C, CallApprovalError> {
    let held: C = read(store, peer, id)?;
    let mut call = held.call().ok_or(CallApprovalError::Version)?.clone();
    let prepared = call.restore()?;
    let checked = crate::sign::verify_before_submit(
        verified.exact_bytes(),
        &SigningPreparation {
            transaction: prepared,
            actor_address: call.sender,
        },
        &call.sender,
        registry,
    )
    .map_err(|_| CallApprovalError::Binding)?;
    if &checked != verified
        || sequence < call.observed_height
        || sequence >= call.budget()?.expiry_sequence
        || core_ms < call.admitted_at_ms
        || core_ms >= call.deadline()?
        || !matches!(
            call.state()?,
            CallApprovalState::Granted | CallApprovalState::Signed | CallApprovalState::Queued
        )
        || call
            .signed
            .as_deref()
            .is_some_and(|bytes| bytes != verified.exact_bytes())
    {
        return Err(CallApprovalError::Binding);
    }
    if call.signed.is_none() {
        call.signed = Some(verified.exact_bytes().to_vec());
        write(store, &C::from_call(call)?, Vec::new())?;
    }
    recover::<C>(store, outbox, peer, id)
}

pub(crate) fn recover<C: CallCarrier>(
    store: &mut Store,
    outbox: &mut Outbox,
    peer: &HumanPeer,
    id: [u8; 32],
) -> Result<C, CallApprovalError> {
    let held: C = read(store, peer, id)?;
    let mut call = held.call().ok_or(CallApprovalError::Version)?.clone();
    let Some(signed) = &call.signed else {
        return Ok(held);
    };
    let hash = transaction_hash(signed);
    let tenant = owner(peer)?;
    let outbox_key = TenantKey::new(tenant.clone(), ObjectKind::Outbox, hash.to_vec())
        .map_err(|_| CallApprovalError::Binding)?;
    if store.get(&outbox_key).is_some() {
        outbox
            .restore(store, tenant, hash)
            .map_err(|_| CallApprovalError::Binding)?;
        if outbox
            .exact_signed_bytes(hash)
            .map_err(|_| CallApprovalError::Binding)?
            != signed
        {
            return Err(CallApprovalError::Binding);
        }
    } else {
        if call.queued {
            return Err(CallApprovalError::Binding);
        }
        outbox
            .enqueue(store, tenant, signed.clone())
            .map_err(|_| CallApprovalError::Store)?;
    }
    if !call.queued {
        call.queued = true;
        let recovered = C::from_call(call)?;
        write(store, &recovered, Vec::new())?;
        return Ok(recovered);
    }
    Ok(held)
}

pub(crate) fn settle<C: CallCarrier>(
    store: &mut Store,
    budgets: &BudgetLimiter,
    peer: &HumanPeer,
    id: [u8; 32],
    receipt_bytes: &[u8],
) -> Result<C, CallApprovalError> {
    let held: C = read(store, peer, id)?;
    let mut call = held.call().ok_or(CallApprovalError::Version)?.clone();
    let receipt = call.receipt(receipt_bytes)?;
    if let Some(existing) = &call.settlement {
        return if existing == receipt_bytes {
            Ok(held)
        } else {
            Err(CallApprovalError::Binding)
        };
    }
    let protocol = receipt.protocol().ok_or(CallApprovalError::Receipt)?;
    let reservation = call.budget()?;
    call.exact_fee_debit(&receipt)?;
    let updates = crate::budget::program_consumption_updates(store, &owner(peer)?, &reservation)
        .map_err(|_| CallApprovalError::Budget)?;
    let staged = crate::budget::stage_release(
        budgets,
        id,
        ReleaseKind::Executed,
        protocol.global_sequence(),
    )
    .map_err(|_| CallApprovalError::Budget)?;
    if !staged.found() {
        return Err(CallApprovalError::Budget);
    }
    call.settlement = Some(receipt_bytes.to_vec());
    let carrier = C::from_call(call)?;
    write(store, &carrier, updates)?;
    let _ = staged.publish();
    Ok(carrier)
}

pub(crate) fn settle_with_debits<C: CallCarrier>(
    store: &mut Store,
    budgets: &BudgetLimiter,
    peer: &HumanPeer,
    id: [u8; 32],
    receipt_bytes: &[u8],
    proof: &crate::budget::program_settlement::VerifiedProgramDebitSettlement,
) -> Result<C, CallApprovalError> {
    let held: C = read(store, peer, id)?;
    let mut call = held.call().ok_or(CallApprovalError::Version)?.clone();
    let receipt = call.receipt(receipt_bytes)?;
    let protocol = receipt.protocol().ok_or(CallApprovalError::Receipt)?;
    let reservation = call.budget()?;
    if proof.reservation_id() != id
        || proof.reservation_digest()
            != reservation
                .settlement_binding()
                .map_err(|_| CallApprovalError::Budget)?
        || proof.activity_id() != protocol.activity_id()
        || proof.global_sequence() != protocol.global_sequence()
        || proof.terminal_receipt() != <[u8; 32]>::from(Sha256::digest(receipt_bytes))
    {
        return Err(CallApprovalError::Binding);
    }
    if let Some(previous) = &call.settlement {
        return if previous == receipt_bytes && call.debit_settlement {
            Ok(held)
        } else {
            Err(CallApprovalError::Binding)
        };
    }
    let tenant = owner(peer)?;
    let stage = budgets
        .stage_program_settlement(store, &tenant, &reservation, proof)
        .map_err(|_| CallApprovalError::Budget)?;
    call.settlement = Some(receipt_bytes.to_vec());
    call.debit_settlement = true;
    let carrier = C::from_call(call)?;
    let mut updates = stage.updates().to_vec();
    updates.push((key::<C>(tenant, id)?, encoded(&carrier)?));
    store
        .apply_program_approval_batch(updates, stage.inserts().to_vec(), Vec::new())
        .map_err(|_| CallApprovalError::Store)?;
    let _ = stage.publish();
    Ok(carrier)
}

macro_rules! impl_call_approval {
    ($carrier:ty) => {
        impl $carrier {
            pub fn hold_call(store: &mut crate::store::Store, peer: &crate::human::HumanPeer,
                session: [u8;32], generation: u64, prepared: &crate::prepare::EvmPrepared,
                staged: crate::budget::StagedProgramReservation<'_>, admitted_at_ms: u64,
                receipt_keys: &[layerx_proof::inclusion::SequencerAuthorization]) -> Result<Self, super::call_bound::CallApprovalError> {
                super::call_bound::hold(store, peer, session, generation, prepared, staged, admitted_at_ms, receipt_keys)
            }
            pub fn read_call(store: &crate::store::Store, peer: &crate::human::HumanPeer, id: [u8;32]) -> Result<Self, super::call_bound::CallApprovalError> {
                super::call_bound::read(store, peer, id)
            }
            pub fn restore_call(&self) -> Result<crate::prepare::EvmPrepared, super::call_bound::CallApprovalError> {
                if !super::call_bound::validate(self)? { return Err(super::call_bound::CallApprovalError::Version); }
                self.call_bound.as_ref().ok_or(super::call_bound::CallApprovalError::Version)?.restore()
            }
            pub fn call_state(&self) -> Result<super::call_bound::CallApprovalState, super::call_bound::CallApprovalError> {
                if !super::call_bound::validate(self)? { return Err(super::call_bound::CallApprovalError::Version); }
                self.call_bound.as_ref().ok_or(super::call_bound::CallApprovalError::Version)?.state()
            }
            pub fn call_held_digest(&self) -> Result<[u8;32], super::call_bound::CallApprovalError> {
                if !super::call_bound::validate(self)? { return Err(super::call_bound::CallApprovalError::Version); }
                self.call_bound.as_ref().ok_or(super::call_bound::CallApprovalError::Version)?.held_digest()
            }
            pub fn call_preparation_id(&self) -> Result<[u8;32], super::call_bound::CallApprovalError> {
                if !super::call_bound::validate(self)? { return Err(super::call_bound::CallApprovalError::Version); }
                Ok(self.call_bound.as_ref().ok_or(super::call_bound::CallApprovalError::Version)?.preparation)
            }
            pub fn call_budget(&self) -> Result<crate::budget::ProgramBudgetReservation, super::call_bound::CallApprovalError> {
                if !super::call_bound::validate(self)? { return Err(super::call_bound::CallApprovalError::Version); }
                self.call_bound.as_ref().ok_or(super::call_bound::CallApprovalError::Version)?.budget()
            }
            pub fn restore_call_budget(&self, budgets: &crate::budget::BudgetLimiter) -> Result<(), super::call_bound::CallApprovalError> {
                if matches!(self.call_state()?, super::call_bound::CallApprovalState::Awaiting | super::call_bound::CallApprovalState::Granted
                    | super::call_bound::CallApprovalState::Signed | super::call_bound::CallApprovalState::Queued) {
                    budgets.restore_program_reservation(&self.call_budget()?).map_err(|_| super::call_bound::CallApprovalError::Budget)?;
                }
                Ok(())
            }
            pub fn decide_call(store: &mut crate::store::Store, budgets: &crate::budget::BudgetLimiter, peer: &crate::human::HumanPeer,
                id: [u8;32], digest: [u8;32], key: [u8;32], grant: bool, sequence: u64, core_ms: u64) -> Result<Self, super::call_bound::CallApprovalError> {
                super::call_bound::decide(store, budgets, peer, id, digest, key, grant, sequence, core_ms)
            }
            pub fn queue_call(store: &mut crate::store::Store, outbox: &mut crate::outbox::Outbox, peer: &crate::human::HumanPeer,
                id: [u8;32], verified: &crate::sign::VerifiedSubmission, registry: &layerx_types::payload::ModuleRegistry,
                sequence: u64, core_ms: u64) -> Result<Self, super::call_bound::CallApprovalError> {
                super::call_bound::queue(store, outbox, peer, id, verified, registry, sequence, core_ms)
            }
            pub fn recover_call_submission(store: &mut crate::store::Store, outbox: &mut crate::outbox::Outbox,
                peer: &crate::human::HumanPeer, id: [u8;32]) -> Result<Self, super::call_bound::CallApprovalError> {
                super::call_bound::recover(store, outbox, peer, id)
            }
            pub fn settle_call_with_debits(store: &mut crate::store::Store, budgets: &crate::budget::BudgetLimiter,
                peer: &crate::human::HumanPeer, id: [u8;32], receipt: &[u8],
                proof: &crate::budget::program_settlement::VerifiedProgramDebitSettlement) -> Result<Self, super::call_bound::CallApprovalError> {
                super::call_bound::settle_with_debits(store, budgets, peer, id, receipt, proof)
            }
            pub fn settle_call(store: &mut crate::store::Store, budgets: &crate::budget::BudgetLimiter, peer: &crate::human::HumanPeer,
                id: [u8;32], receipt: &[u8]) -> Result<Self, super::call_bound::CallApprovalError> {
                super::call_bound::settle(store, budgets, peer, id, receipt)
            }
        }
    };
}
pub(crate) use impl_call_approval;
