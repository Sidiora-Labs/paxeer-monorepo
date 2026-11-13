use layerx_paxai_worker::{
    auth::{admit, decode_service, Admission},
    jobs::{AdmissionView, Input},
};
use std::path::Path;

const DEFAULT_URI: &str = "https://localhost/paxai/v1";
const RUNNER_ENV: &str = "PAXAI_RUNNER";

impl Submit {
    fn admit(
        &self,
        authority: &FinalizedAuthority,
        evidence: &IdentityEvidence,
        metadata: &VerifiedMetadata,
    ) -> Checked<Result<Admission, ServiceError>> {
        let envelope = decode_service(&self.signed(&customer())?)?;
        Ok(admit(
            &envelope,
            &self.request,
            authority,
            evidence,
            metadata,
            &self.task,
        ))
    }
}

fn admitting() -> Checked<Admitting> {
    let market = opened(vec![endpoint(DEFAULT_URI, [0xE1; 32])])?;
    let authority = authority_at(&market, HEIGHT)?;
    let metadata = verified(&authority, &market.signed)?;
    let submit = submit(&authority, &capability(0x31))?;
    Ok(Admitting {
        market,
        authority,
        metadata,
        submit,
    })
}

impl Scratch {
    /// The configured runner path when no runner executable is installed.
    fn absent(&self) -> PathBuf {
        self.0.join("runner-not-installed")
    }
}

fn service(root: &Path, runner: PathBuf) -> Checked<JobService> {
    Ok(JobService::new(
        JobStore::open(root)?,
        ProcessRunner::new(runner, Vec::new(), Duration::from_secs(5)),
        delegate(),
    ))
}

fn real_runner() -> Checked<PathBuf> {
    std::env::var_os(RUNNER_ENV)
        .map(PathBuf::from)
        .ok_or(Failure::Unexpected(
            "real model runner executable: PAXAI_RUNNER is not provisioned",
        ))
}

impl Order {
    fn input(&self) -> Input<'_> {
        Input {
            manifest: &self.manifest,
            payload: &self.payload,
        }
    }
    fn view<'a>(
        &'a self,
        authority: &'a FinalizedAuthority,
        customer: &'a IdentityEvidence,
        metadata: &'a VerifiedMetadata,
    ) -> AdmissionView<'a> {
        AdmissionView {
            authority,
            customer,
            metadata,
            task: &self.submit.task,
        }
    }
    fn accepted(&self, acknowledgment: &[u8]) -> Checked<TaskBinding> {
        Ok(TaskBinding {
            status: TaskStatus::Accepted,
            acknowledgement: Some(acknowledgment_digest(acknowledgment)?),
            ..self.submit.task
        })
    }
}
