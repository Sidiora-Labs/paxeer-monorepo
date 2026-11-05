// Reuse the production collector and allowlist implementation. This module is
// owned by the Zksync service and shared with the other services in mod.rs, so it
// also compiles when the Zksync source is included by a standalone test target.
include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/metrics.rs"));

use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2 as verification_proto;
use std::future::Future;
use tonic::{Code, Response, Status};

pub(crate) trait VerificationOutcome {
    fn verification_status(&self, index: usize) -> &'static str;
}

impl VerificationOutcome for verification_proto::VerifyResponse {
    fn verification_status(&self, _index: usize) -> &'static str {
        match self.status() {
            verification_proto::verify_response::Status::Success => "SUCCESS",
            verification_proto::verify_response::Status::Failure
            | verification_proto::verify_response::Status::Unspecified => "FAILURE",
        }
    }
}

impl VerificationOutcome for verification_proto::BatchVerifyResponse {
    fn verification_status(&self, index: usize) -> &'static str {
        use verification_proto::{batch_verify_response, contract_verification_result};
        match &self.verification_result {
            Some(batch_verify_response::VerificationResult::ContractVerificationResults(
                results,
            )) => {
                match results
                    .items
                    .get(index)
                    .and_then(|item| item.verification_result.as_ref())
                {
                    Some(contract_verification_result::VerificationResult::Success(_)) => "SUCCESS",
                    Some(contract_verification_result::VerificationResult::Failure(_)) | None => {
                        "FAILURE"
                    }
                }
            }
            Some(batch_verify_response::VerificationResult::CompilationFailure(_)) | None => {
                "FAILURE"
            }
        }
    }
}

impl VerificationOutcome for verification_proto::zksync::solidity::VerifyResponse {
    fn verification_status(&self, _index: usize) -> &'static str {
        use verification_proto::zksync::solidity::verify_response::VerifyResponse;
        match &self.verify_response {
            Some(VerifyResponse::VerificationSuccess(_)) => "SUCCESS",
            Some(VerifyResponse::CompilationFailure(_))
            | Some(VerifyResponse::VerificationFailure(_))
            | None => "FAILURE",
        }
    }
}

fn transport_status(status: &Status) -> &'static str {
    match status.code() {
        Code::Ok => "OK",
        Code::Cancelled => "CANCELLED",
        Code::Unknown => "UNKNOWN",
        Code::InvalidArgument => "INVALID_ARGUMENT",
        Code::DeadlineExceeded => "DEADLINE_EXCEEDED",
        Code::NotFound => "NOT_FOUND",
        Code::AlreadyExists => "ALREADY_EXISTS",
        Code::PermissionDenied => "PERMISSION_DENIED",
        Code::ResourceExhausted => "RESOURCE_EXHAUSTED",
        Code::FailedPrecondition => "FAILED_PRECONDITION",
        Code::Aborted => "ABORTED",
        Code::OutOfRange => "OUT_OF_RANGE",
        Code::Unimplemented => "UNIMPLEMENTED",
        Code::Internal => "INTERNAL",
        Code::Unavailable => "UNAVAILABLE",
        Code::DataLoss => "DATA_LOSS",
        Code::Unauthenticated => "UNAUTHENTICATED",
    }
}

pub(crate) async fn track_verification<T: VerificationOutcome>(
    chain_ids: Vec<Option<String>>,
    language: &'static str,
    endpoint: &'static str,
    operation: impl Future<Output = Result<Response<T>, Status>>,
) -> Result<Response<T>, Status> {
    configure_chain_labels().map_err(|error| Status::internal(error.to_string()))?;
    let result = operation.await;
    for (index, chain) in chain_ids.iter().enumerate() {
        let chain = match chain.as_deref().filter(|chain| !chain.trim().is_empty()) {
            None => UNKNOWN_CHAIN.to_owned(),
            Some(chain) => match chain_label(chain).as_str() {
                UNKNOWN_CHAIN => "other".to_owned(),
                label => label.to_owned(),
            },
        };
        let status = match &result {
            Ok(response) => response.get_ref().verification_status(index),
            Err(status) => transport_status(status),
        };
        smart_contract_verifier_server::SolidityVerifierService::verification_counter()
            .with_label_values(&[&chain, language, endpoint, status])
            .inc();
    }
    result
}
