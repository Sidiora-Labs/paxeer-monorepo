use std::path::PathBuf;
use std::sync::{mpsc, Arc};
use std::thread;
use std::time::Duration;

use layerx_human_service::custody::{KmsError, RemoteKmsProvider};
use layerx_human_service::server::movement_provider::{
    MovementProviderCodec, MovementProviderRequest as Request,
    MovementProviderResponse as Response, MovementProviderService, NativeMovementCodec,
};
use layerx_paxeer_client::{
    raw_call, DepositProofVerifier, FinalityTracker, TrackerConfig, TransactionHash,
};
use sha2::{Digest, Sha256};

use crate::config::{hex_string, Config};
use crate::journal::{private_directory, Journal};
use crate::Error;

/// The typed result of the movement execution authority readiness probe.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum ExecutorDependency {
    /// The authority admitted the restricted executor identity and answered
    /// the probe for this provider and network.
    Usable,
    /// No execution authority is configured (evidence-only mode).
    Unconfigured,
    /// The authority was absent, refused the identity, or answered for a
    /// different provider, network or challenge.
    Failed(KmsError),
    /// The probe did not complete within the total readiness deadline.
    DeadlineExceeded,
}

pub(crate) struct EvidenceService {
    journal: Journal,
    codec: NativeMovementCodec,
    tracker_config: TrackerConfig,
    evidence_root: PathBuf,
    chain_id: u64,
    executor: Option<Arc<RemoteKmsProvider>>,
    executor_deadline: Duration,
    policy: layerx_paxeer_client::DepositProofConfig,
}

impl EvidenceService {
    pub fn new(config: &Config, journal: Journal) -> Result<Self, Error> {
        private_directory(&config.evidence_root)?;
        FinalityTracker::new(config.tracker.clone(), TransactionHash::new([1; 32]))
            .map_err(|_| Error::Configuration)?;
        DepositProofVerifier::new(config.proof.clone()).map_err(|_| Error::Configuration)?;
        crate::config::validated_protocol(config.listener.protocol)?;
        if config.proof.endpoints != config.tracker.endpoints
            || config.proof.minimum_endpoint_agreement != config.tracker.minimum_endpoint_agreement
            || config.proof.required_confirmations != config.tracker.required_confirmations
            || config.proof.layerx_protocol_version != config.listener.protocol
        {
            return Err(Error::Configuration);
        }
        Ok(Self {
            journal,
            codec: NativeMovementCodec::new(),
            tracker_config: config.tracker.clone(),
            evidence_root: config.evidence_root.clone(),
            chain_id: config.proof.paxeer_chain_id,
            executor: config.executor.clone(),
            executor_deadline: config.listener.deadline,
            policy: config.proof.clone(),
        })
    }

    fn execute(&mut self, request: &Request) -> Response {
        match request {
            Request::PlanMove(plan) => {
                if self.executor.is_none() {
                    return Response::Unavailable;
                }
                if crate::planning::validate(plan, &self.policy).is_err() {
                    return Response::ContractViolation;
                }
                crate::planning::move_plan(plan)
                    .map(Response::MovePlan)
                    .unwrap_or(Response::Unavailable)
            }
            Request::Readiness => self.readiness(),
        }
    }

    /// Answers `Ready` only while every condition the executing paths depend on
    /// currently holds, and logs the first condition that does not.
    fn readiness(&self) -> Response {
        match self.readiness_fault() {
            None => Response::Ready,
            Some(reason) => {
                eprintln!("movement provider is not ready: {reason}");
                Response::Unavailable
            }
        }
    }

    fn readiness_fault(&self) -> Option<String> {
        if self.journal.readable().is_err() {
            return Some("the durable journal cannot be read".to_owned());
        }
        if private_directory(&self.evidence_root).is_err() {
            return Some("the evidence root is not a protected directory".to_owned());
        }
        match self.executor_dependency() {
            ExecutorDependency::Usable => {}
            ExecutorDependency::Unconfigured => {
                return Some("no movement execution authority is configured".to_owned());
            }
            ExecutorDependency::Failed(error) => {
                return Some(format!(
                    "the movement execution authority failed the executor probe: {error}"
                ));
            }
            ExecutorDependency::DeadlineExceeded => {
                return Some(
                    "the movement execution authority did not complete the executor probe within the deadline"
                        .to_owned(),
                );
            }
        }
        if self.agreeing_origins() < self.tracker_config.minimum_endpoint_agreement {
            return Some("too few paxeer origins answer on the configured chain".to_owned());
        }
        None
    }

    /// Runs the authenticated, read-only executor probe against the
    /// configured execution authority and answers within the total readiness
    /// deadline whatever the transport does. A probe still in flight when the
    /// deadline passes is abandoned; its answer can no longer grant readiness.
    fn executor_dependency(&self) -> ExecutorDependency {
        let Some(executor) = self.executor.clone() else {
            return ExecutorDependency::Unconfigured;
        };
        let network = self.policy.layerx_network_id;
        let (finished, answer) = mpsc::sync_channel(1);
        if thread::Builder::new()
            .name("movement-executor-probe".into())
            .spawn(move || {
                let _ = finished.send(executor.probe_executor(network));
            })
            .is_err()
        {
            return ExecutorDependency::Failed(KmsError::Unavailable);
        }
        match answer.recv_timeout(self.executor_deadline) {
            Ok(Ok(())) => ExecutorDependency::Usable,
            Ok(Err(error)) => ExecutorDependency::Failed(error),
            Err(mpsc::RecvTimeoutError::Timeout) => ExecutorDependency::DeadlineExceeded,
            Err(mpsc::RecvTimeoutError::Disconnected) => {
                ExecutorDependency::Failed(KmsError::Unavailable)
            }
        }
    }

    /// Counts configured origins that answer on `chain_id`, stopping as soon as
    /// the configured agreement is met so a readiness gate cannot be stalled by
    /// the slowest origin.
    fn agreeing_origins(&self) -> usize {
        let required = self.tracker_config.minimum_endpoint_agreement;
        let mut agreeing = 0;
        for endpoint in &self.tracker_config.endpoints {
            if agreeing >= required {
                break;
            }
            if endpoint.expected_chain_id == self.chain_id
                && raw_call(endpoint, "eth_chainId", &[]).is_ok()
            {
                agreeing += 1;
            }
        }
        agreeing
    }
}

impl MovementProviderService for EvidenceService {
    fn dispatch(&mut self, request: Request) -> Response {
        let Ok(bytes) = self.codec.encode_request(&request) else {
            return Response::ContractViolation;
        };
        let key = match &request {
            Request::PlanMove(plan) => {
                let Ok(key) = crate::planning::identity(plan, plan.operation.as_bytes()) else {
                    return Response::ContractViolation;
                };
                hex_string(&key)
            }
            _ => hex_string(&Sha256::digest(&bytes)),
        };
        if let Err(error) = self.journal.begin(&key, &bytes) {
            return match error {
                Error::Conflict => Response::ContractViolation,
                _ => Response::Unavailable,
            };
        }
        if matches!(request, Request::PlanMove(_)) {
            if let Some(encoded) = self
                .journal
                .record(&key)
                .and_then(|record| record.response.as_ref())
            {
                if let Ok(response) = self.codec.decode_response(encoded) {
                    if !matches!(
                        response,
                        Response::Unavailable | Response::ContractViolation
                    ) {
                        return response;
                    }
                }
            }
        }
        let response = self.execute(&request);
        let Ok(encoded) = self.codec.encode_response(&response) else {
            return Response::ContractViolation;
        };
        if self.journal.complete(&key, &encoded).is_err() {
            return Response::Unavailable;
        }
        response
    }
}
