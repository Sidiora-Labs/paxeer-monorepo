mod client;
mod keccak;
mod types;
mod v2;

#[cfg(test)]
mod test_service;

pub use client::{Client, ClientBuilder};
pub use types::{
    EmptyCustomError, GetSourceFilesResponse, MatchType, VerifyFromEtherscanError,
    VerifyFromEtherscanResponse,
};

#[derive(Clone, Debug, PartialEq, Eq, thiserror::Error)]
pub enum SourcifyError<E: std::error::Error> {
    #[error("'Internal Server Error': {0}")]
    InternalServerError(String),
    #[error("Chain is not supported: {0}")]
    ChainNotSupported(String),
    #[error("'Not Found': {0}")]
    NotFound(String),
    #[error("'Bad Request': {0}")]
    BadRequest(String),
    #[error("'Bad Gateway': {0}")]
    BadGateway(String),
    #[error("unexpected status code: {status_code} - {msg}")]
    UnexpectedStatusCode {
        status_code: reqwest::StatusCode,
        msg: String,
    },
    #[error("endpoint specific error: {0}")]
    Custom(E),
}

#[derive(Debug, thiserror::Error)]
pub enum Error<E: std::error::Error> {
    #[error("invalid Sourcify response: {0}")]
    InvalidResponse(String),
    #[error("invalid Sourcify client configuration: {0}")]
    InvalidConfiguration(String),
    #[error("Sourcify operation exceeded its configured total deadline")]
    VerificationTimeout,
    #[error("error occurred while sending request: {0}")]
    Reqwest(#[from] reqwest::Error),
    #[error("error with the middleware occurred while sending request: {0:#}")]
    ReqwestMiddleware(anyhow::Error),
    #[error("error got from the Sourcify: {0}")]
    Sourcify(#[from] SourcifyError<E>),
}
