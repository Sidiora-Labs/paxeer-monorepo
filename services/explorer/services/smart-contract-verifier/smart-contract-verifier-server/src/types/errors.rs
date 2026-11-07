use thiserror::Error;
#[derive(Error, Debug)]
pub enum StandardJsonParseError {
    #[error("content is not a valid standard json: {0}")]
    InvalidContent(#[from] serde_json::Error),
    #[error("{0}")]
    BadRequest(#[from] anyhow::Error),
}

/// HTTP 500 / gRPC INTERNAL expose only this message and a log correlation ID.
pub(super) fn internal_error_status(error: impl std::fmt::Debug) -> tonic::Status {
    use std::sync::atomic::{AtomicU64, Ordering};
    static NEXT_ERROR_ID: AtomicU64 = AtomicU64::new(1);
    let error_id = format!(
        "verification-{:016x}",
        NEXT_ERROR_ID.fetch_add(1, Ordering::Relaxed)
    );
    tracing::error!(%error_id, error = ?error, "internal verification error");
    tonic::Status::internal(format!(
        "Internal verification error; reference: {error_id}"
    ))
}
