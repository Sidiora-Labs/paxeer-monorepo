//! Owned node-boundary lifecycle. Mount one instance behind each authenticated dispatcher.
use std::collections::BTreeMap;
use std::io::Read;

use layerx_client::sequencer::{SequencerEndpoint, SequencerError, SequencerReceipt};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use sha2::{Digest, Sha256};

use crate::outbox::{Outbox, OutboxError, SubmissionState};
use crate::store::{ObjectKind, Store, StoreError, TenantId};
use crate::tenant::ResolvedPrincipal;

use super::admission::{
    AdmissionError, BoundaryAdmission, BoundaryWork, CoreAvailability, CoreOutcome,
    LaneUtilization, Priority,
};
use super::deadline::{
    CancellationToken, DeadlineError, DisconnectOutcome, RequestDeadline, RequestTracker,
    TrackedWork, WorkMetrics, WorkView, WriteStage,
};
use super::quota::{ClientActivity, Quota, QuotaError, QuotaHealth};
use super::{RateLimiter, RateRequest, Refusal, Utilization};

#[derive(Clone, Debug)]
pub enum NodeWork {
    Head { priority: Priority },
    Submit { signed_bytes: Vec<u8> },
    Receipt { submission_id: [u8; 32] },
}

impl NodeWork {
    fn priority(&self) -> Priority {
        match self {
            Self::Head { priority } => *priority,
            Self::Submit { .. } => Priority::Submission,
            Self::Receipt { .. } => Priority::ReceiptResolution,
        }
    }

    fn bytes(&self) -> Result<Vec<u8>, LifecycleError> {
        match self {
            Self::Head {
                priority: Priority::Submission | Priority::ReceiptResolution,
            } => Err(LifecycleError::InvalidWork),
            Self::Head { .. } => Ok(b"eth_blockNumber".to_vec()),
            Self::Submit { signed_bytes } => {
                let (call, signature) =
                    decode_signed(signed_bytes).map_err(|_| LifecycleError::InvalidWork)?;
                recover_sender(&call, &signature).map_err(|_| LifecycleError::InvalidWork)?;
                Ok(signed_bytes.clone())
            }
            Self::Receipt { submission_id } => Ok(submission_id.to_vec()),
        }
    }

    fn operation_class(&self) -> &'static str {
        match self {
            Self::Head { .. } => "read",
            Self::Submit { .. } => "submit",
            Self::Receipt { .. } => "receipt",
        }
    }
}

#[derive(Debug)]
pub enum LifecycleError {
    Admission(AdmissionError),
    Deadline(DeadlineError),
    Quota(QuotaError),
    Rate(Refusal),
    Outbox(OutboxError),
    Store(StoreError),
    Transport(std::io::Error),
    InvalidWork,
    OwnerMismatch,
    NotDisconnected,
    UnknownRequest,
}

macro_rules! convert {
    ($error:ty, $variant:ident) => {
        impl From<$error> for LifecycleError {
            fn from(error: $error) -> Self {
                Self::$variant(error)
            }
        }
    };
}
convert!(AdmissionError, Admission);
convert!(DeadlineError, Deadline);
convert!(QuotaError, Quota);
convert!(Refusal, Rate);
convert!(OutboxError, Outbox);
convert!(StoreError, Store);

#[derive(Clone, Debug)]
struct OwnedRequest {
    tenant: TenantId,
    client: String,
    work: NodeWork,
    token: CancellationToken,
}

/// Opaque transport ownership, returned only after durable submission and dispatch admission.
#[derive(Debug)]
pub struct BoundaryCall {
    request_id: u64,
    work: NodeWork,
    token: CancellationToken,
    deadline: RequestDeadline,
}

#[derive(Debug)]
#[allow(
    clippy::large_enum_variant,
    reason = "Preserve the public node response variants returned by boundary execution."
)]
pub enum NodeResponse {
    Head(u64),
    Submitted([u8; 32]),
    Receipt(Option<SequencerReceipt>),
}

#[derive(Debug)]
pub enum CallError {
    Cancelled,
    Deadline(DeadlineError),
    Node(SequencerError),
}

impl BoundaryCall {
    #[must_use]
    pub const fn request_id(&self) -> u64 {
        self.request_id
    }

    /// Uses the actual sequencer JSON-RPC codec and transport. Cancellation is checked
    /// before crossing the boundary; transmitting writes retain their resolver token.
    /// # Errors
    /// Returns an error if the call is cancelled, exceeds its deadline, or fails at the node
    /// boundary.
    pub fn execute(
        &self,
        node: &SequencerEndpoint,
        observed_at_ms: u64,
    ) -> Result<NodeResponse, CallError> {
        if self.token.is_cancelled() {
            return Err(CallError::Cancelled);
        }
        self.deadline
            .boundary_call(observed_at_ms, 30_000)
            .map_err(CallError::Deadline)?;
        match &self.work {
            NodeWork::Head { .. } => node.block_number().map(NodeResponse::Head),
            NodeWork::Submit { signed_bytes } => {
                node.send_raw_transaction(signed_bytes).and_then(|hash| {
                    if hash == transaction_hash(signed_bytes) {
                        Ok(NodeResponse::Submitted(hash))
                    } else {
                        Err(SequencerError::Malformed)
                    }
                })
            }
            NodeWork::Receipt { submission_id } => node
                .sequencer_receipt(submission_id)
                .map(NodeResponse::Receipt),
        }
        .map_err(CallError::Node)
    }
}

/// A single explicit owner ties authenticated requests, scheduling, client shedding,
/// cancellation and the durable outbox together. Callers execute transport calls outside
/// its lock so a disconnected client cannot block another tenant's admission.
pub struct TenantBoundary {
    admission: BoundaryAdmission,
    tracker: RequestTracker,
    quota: Quota,
    rate: RateLimiter,
    outbox: Outbox,
    requests: BTreeMap<u64, OwnedRequest>,
    recovered: BTreeMap<(TenantId, [u8; 32]), u64>,
}

impl TenantBoundary {
    /// Restores unknown ownership from the real durable outbox before accepting work.
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn open(
        store: &mut Store,
        admission: BoundaryAdmission,
        mut quota: Quota,
        rate: RateLimiter,
        now: u64,
    ) -> Result<Self, LifecycleError> {
        if !rate.is_durable() {
            return Err(LifecycleError::Rate(Refusal::CounterUnavailable));
        }
        quota.restore(store, now)?;
        let mut boundary = Self {
            admission,
            tracker: RequestTracker::default(),
            quota,
            rate,
            outbox: Outbox::default(),
            requests: BTreeMap::new(),
            recovered: BTreeMap::new(),
        };
        let mut next_id = u64::MAX;
        for tenant in store.tenant_ids_for_kind(ObjectKind::Outbox) {
            for id in store.list_object_ids(&tenant, ObjectKind::Outbox) {
                let submission_id: [u8; 32] =
                    id.try_into().map_err(|_| LifecycleError::InvalidWork)?;
                boundary
                    .outbox
                    .restore(store, tenant.clone(), submission_id)?;
                let state = boundary
                    .outbox
                    .status(submission_id)
                    .ok_or(LifecycleError::UnknownRequest)?
                    .state;
                if state == SubmissionState::Submitted {
                    boundary.outbox.transition(
                        store,
                        submission_id,
                        SubmissionState::Unknown,
                        "limits restart retains uncertain transport",
                        None,
                    )?;
                }
                if matches!(
                    state,
                    SubmissionState::Submitted
                        | SubmissionState::Unknown
                        | SubmissionState::Executed
                ) {
                    boundary.tracker.recover_submission(
                        next_id,
                        &boundary.outbox,
                        submission_id,
                        now,
                    )?;
                    boundary
                        .recovered
                        .insert((tenant.clone(), submission_id), next_id);
                    next_id = next_id.checked_sub(1).ok_or(LifecycleError::InvalidWork)?;
                }
            }
        }
        Ok(boundary)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    #[allow(
        clippy::too_many_arguments,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn enqueue(
        &mut self,
        store: &mut Store,
        principal: &ResolvedPrincipal,
        request_id: u64,
        work: NodeWork,
        deadline: RequestDeadline,
        retry: bool,
        now: u64,
    ) -> Result<CancellationToken, LifecycleError> {
        if self.tracker.view(request_id).is_some() || self.requests.contains_key(&request_id) {
            return Err(LifecycleError::Deadline(DeadlineError::DuplicateRequest(
                request_id,
            )));
        }
        deadline.boundary_call(now, 1)?;
        let bytes = work.bytes()?;
        let priority = work.priority();
        let client = client_id(principal);
        // Recovery traffic has separate reserved lanes; observation cardinality cannot deny it.
        self.quota
            .admit_work(store, &principal.tenant, &client, priority, now)?;
        if !matches!(priority, Priority::Submission | Priority::ReceiptResolution) {
            let decision = super::shed(
                &mut self.quota,
                store,
                ClientActivity {
                    tenant: principal.tenant.clone(),
                    client_id: client.clone(),
                    operation_digest: Sha256::digest(&bytes).into(),
                    retry,
                    observed_at_ms: now,
                },
            )?;
            if decision.is_some() {
                self.shed_client(store, &principal.tenant, &client, now)?;
            }
            self.quota
                .admit_work(store, &principal.tenant, &client, priority, now)?;
        }
        self.admission.admit(
            BoundaryWork {
                request_id,
                tenant: principal.tenant.as_str().to_owned(),
                priority,
                bytes,
            },
            CoreAvailability::Ready,
        )?;
        let rate = RateRequest {
            tenant: principal.tenant.as_str().to_owned(),
            agent: layerx_programs::hex::encode(&Sha256::digest(principal.agent.as_bytes())),
            session: client.clone(),
            capability: "authenticated".to_owned(),
            operation_class: work.operation_class().to_owned(),
            logical_time_ms: now,
            cost: 1,
        };
        if let Err(error) = self.rate.admit(&rate) {
            self.admission.cancel_queued(request_id);
            return Err(error.into());
        }
        let token = match &work {
            NodeWork::Submit { signed_bytes } => {
                self.tracker
                    .begin_write(request_id, transaction_hash(signed_bytes), deadline)?
            }
            _ => self.tracker.begin_read(request_id, deadline)?,
        };
        self.requests.insert(
            request_id,
            OwnedRequest {
                tenant: principal.tenant.clone(),
                client,
                work,
                token: token.clone(),
            },
        );
        Ok(token)
    }

    fn shed_client(
        &mut self,
        store: &mut Store,
        tenant: &TenantId,
        client: &str,
        now: u64,
    ) -> Result<(), LifecycleError> {
        let ids = self
            .requests
            .iter()
            .filter_map(|(id, request)| {
                (request.tenant == *tenant
                    && request.client == client
                    && !matches!(
                        request.work.priority(),
                        Priority::Submission | Priority::ReceiptResolution
                    ))
                .then_some(*id)
            })
            .collect::<Vec<_>>();
        for id in ids {
            if self.detach_write(store, id, now)? == DisconnectOutcome::Cancelled {
                self.admission.cancel_queued(id);
                self.requests.remove(&id);
            }
        }
        Ok(())
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn start_next(
        &mut self,
        store: &mut Store,
        now: u64,
    ) -> Result<Option<BoundaryCall>, LifecycleError> {
        self.expire(store, now)?;
        let Some(dispatch) = self.admission.dispatch(CoreAvailability::Ready)? else {
            return Ok(None);
        };
        let id = dispatch.work.request_id;
        let request = self
            .requests
            .get(&id)
            .cloned()
            .ok_or(LifecycleError::UnknownRequest)?;
        let deadline = self
            .tracker
            .view(id)
            .ok_or(LifecycleError::UnknownRequest)?
            .deadline;
        let setup = (|| -> Result<(), LifecycleError> {
            if let NodeWork::Submit { signed_bytes } = &request.work {
                self.tracker.advance_write(id, WriteStage::Signing, now)?;
                let hash =
                    self.outbox
                        .enqueue(store, request.tenant.clone(), signed_bytes.clone())?;
                self.tracker
                    .advance_write(id, WriteStage::DurableQueued, now)?;
                // Persist uncertainty before any socket can observe the exact signed bytes.
                self.outbox.transition(
                    store,
                    hash,
                    SubmissionState::Submitted,
                    "limits transport owns exact bytes",
                    None,
                )?;
                self.tracker
                    .advance_write(id, WriteStage::Transmitting, now)?;
            }
            Ok(())
        })();
        if let Err(error) = setup {
            self.admission.finish(id, CoreOutcome::Completed)?;
            if self.detach_write(store, id, now)? == DisconnectOutcome::Cancelled {
                self.requests.remove(&id);
            }
            return Err(error);
        }
        Ok(Some(BoundaryCall {
            request_id: id,
            work: request.work,
            token: request.token,
            deadline,
        }))
    }

    /// Completes scheduling ownership. A successful broadcast is still not a receipt.
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    #[allow(
        clippy::needless_pass_by_value,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn finish_call(
        &mut self,
        store: &mut Store,
        call: BoundaryCall,
        result: &Result<NodeResponse, CallError>,
        now: u64,
    ) -> Result<(), LifecycleError> {
        self.admission
            .finish(call.request_id, CoreOutcome::Completed)?;
        if matches!(call.work, NodeWork::Submit { .. }) {
            let _ = result;
            self.detach_write(store, call.request_id, now)?;
        } else {
            // The caller can already have disconnected while transport was in progress.
            if self.tracker.view(call.request_id).is_some() {
                self.tracker.complete(call.request_id)?;
            }
            self.requests.remove(&call.request_id);
        }
        Ok(())
    }

    fn detach_write(
        &mut self,
        store: &mut Store,
        id: u64,
        now: u64,
    ) -> Result<DisconnectOutcome, LifecycleError> {
        let view = self
            .tracker
            .view(id)
            .ok_or(LifecycleError::UnknownRequest)?;
        if now < view.deadline.started_at_ms {
            return Err(DeadlineError::TimeRegressed.into());
        }
        if let TrackedWork::Write {
            submission_id,
            stage,
            ..
        } = view.work
        {
            if matches!(
                stage,
                WriteStage::Transmitting | WriteStage::Acknowledged | WriteStage::UnknownResolving
            ) {
                if self
                    .outbox
                    .status(submission_id)
                    .is_some_and(|status| status.state == SubmissionState::Submitted)
                {
                    self.outbox.transition(
                        store,
                        submission_id,
                        SubmissionState::Unknown,
                        "caller lifetime ended; receipt resolver retains ownership",
                        None,
                    )?;
                }
            } else if self
                .outbox
                .status(submission_id)
                .is_some_and(|status| status.state == SubmissionState::Queued)
            {
                self.outbox.transition(
                    store,
                    submission_id,
                    SubmissionState::Failed,
                    "caller cancelled before transmission",
                    None,
                )?;
            }
        }
        let outcome = super::cancel(&mut self.tracker, id, now)?;
        Ok(outcome)
    }

    /// The authenticated transport invokes this on EOF, reset or explicit cancellation.
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn disconnect(
        &mut self,
        store: &mut Store,
        principal: &ResolvedPrincipal,
        id: u64,
        now: u64,
    ) -> Result<DisconnectOutcome, LifecycleError> {
        let request = self
            .requests
            .get(&id)
            .ok_or(LifecycleError::UnknownRequest)?;
        if request.tenant != principal.tenant || request.client != client_id(principal) {
            return Err(LifecycleError::OwnerMismatch);
        }
        let outcome = self.detach_write(store, id, now)?;
        if outcome == DisconnectOutcome::Cancelled {
            self.admission.cancel_queued(id);
            self.requests.remove(&id);
        }
        Ok(outcome)
    }

    /// Reads the transport's actual EOF instead of treating a caller claim as a disconnect.
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn transport_eof(
        &mut self,
        store: &mut Store,
        principal: &ResolvedPrincipal,
        id: u64,
        transport: &mut impl Read,
        now: u64,
    ) -> Result<DisconnectOutcome, LifecycleError> {
        let mut byte = [0];
        match transport.read(&mut byte) {
            Ok(0) => self.disconnect(store, principal, id, now),
            Ok(_) => Err(LifecycleError::NotDisconnected),
            Err(error)
                if matches!(
                    error.kind(),
                    std::io::ErrorKind::ConnectionReset
                        | std::io::ErrorKind::BrokenPipe
                        | std::io::ErrorKind::UnexpectedEof
                ) =>
            {
                self.disconnect(store, principal, id, now)
            }
            Err(error) => Err(LifecycleError::Transport(error)),
        }
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn expire(&mut self, store: &mut Store, now: u64) -> Result<(), LifecycleError> {
        let expired = self
            .requests
            .keys()
            .copied()
            .filter(|id| {
                self.tracker
                    .view(*id)
                    .is_some_and(|view| view.caller_connected && now >= view.deadline.expires_at_ms)
            })
            .collect::<Vec<_>>();
        for id in expired {
            if self.detach_write(store, id, now)? == DisconnectOutcome::Cancelled {
                self.admission.cancel_queued(id);
                self.requests.remove(&id);
            }
        }
        Ok(())
    }

    /// Release only on a terminal state produced by the real outbox resolver.
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn synchronize_resolution(&mut self, id: u64) -> Result<bool, LifecycleError> {
        let view = self
            .tracker
            .view(id)
            .ok_or(LifecycleError::UnknownRequest)?;
        let TrackedWork::Write { submission_id, .. } = view.work else {
            return Err(LifecycleError::InvalidWork);
        };
        if !self
            .outbox
            .status(submission_id)
            .ok_or(LifecycleError::UnknownRequest)?
            .state
            .terminal()
        {
            return Ok(false);
        }
        self.tracker.resolved_by_receipt(id)?;
        self.requests.remove(&id);
        self.recovered.retain(|_, request| *request != id);
        Ok(true)
    }

    pub fn outbox_mut(&mut self) -> &mut Outbox {
        &mut self.outbox
    }
    #[must_use]
    pub fn view(&self, id: u64) -> Option<WorkView> {
        self.tracker.view(id)
    }
    #[must_use]
    pub fn recovered_request(&self, tenant: &TenantId, submission: [u8; 32]) -> Option<u64> {
        self.recovered.get(&(tenant.clone(), submission)).copied()
    }
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn metrics(&self, now: u64) -> Result<WorkMetrics, LifecycleError> {
        Ok(self.tracker.metrics(now)?)
    }
    #[must_use]
    pub fn lane_utilization(&self, priority: Priority) -> LaneUtilization {
        self.admission.utilization(priority)
    }
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn rate_utilization(
        &self,
        tenant: &TenantId,
        now: u64,
    ) -> Result<Vec<Utilization>, LifecycleError> {
        Ok(self.rate.utilization(tenant.as_str(), now)?)
    }
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn quota_health(
        &self,
        store: &Store,
        tenant: &TenantId,
        now: u64,
    ) -> Result<QuotaHealth, LifecycleError> {
        Ok(self.quota.health(store, tenant, now)?)
    }
}

#[must_use]
pub fn client_id(principal: &ResolvedPrincipal) -> String {
    layerx_programs::hex::encode(&principal.session_id.0)
}
