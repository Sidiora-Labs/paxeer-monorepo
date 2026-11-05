use crate::{
    proto::{
        sourcify_verifier_server::SourcifyVerifier, VerifyFromEtherscanSourcifyRequest,
        VerifyResponse, VerifySourcifyRequest,
    },
    settings::SourcifySettings,
    types::{
        VerifyFromEtherscanSourcifyRequestWrapper, VerifyResponseWrapper,
        VerifySourcifyRequestWrapper,
    },
};
use smart_contract_verifier::{sourcify as sc_sourcify, sourcify::Error, SourcifyApiClient};
use std::{sync::Arc, time::Duration};
use tonic::{Request, Response, Status};

pub struct SourcifyVerifierService {
    client: Arc<SourcifyApiClient>,
    operation_timeout: Duration,
}

impl SourcifyVerifierService {
    pub async fn new(settings: SourcifySettings) -> anyhow::Result<Self> {
        anyhow::ensure!(
            settings.request_timeout > 0,
            "Sourcify request_timeout must be positive"
        );
        let operation_timeout = Duration::from_secs(settings.request_timeout);
        let client = {
            SourcifyApiClient::new(
                settings.api_url,
                settings.request_timeout,
                settings.verification_attempts,
            )
            .expect("failed to build sourcify client")
        };

        Ok(Self {
            client: Arc::new(client),
            operation_timeout,
        })
    }
}

#[async_trait::async_trait]
impl SourcifyVerifier for SourcifyVerifierService {
    async fn verify(
        &self,
        request: Request<VerifySourcifyRequest>,
    ) -> Result<Response<VerifyResponse>, Status> {
        let chain_ids = vec![Some(request.get_ref().chain.clone())];
        super::verification_metrics::track_verification(chain_ids, "solidity", "sourcify", async {
            let request: VerifySourcifyRequestWrapper = request.into_inner().into();

            tracing::info!(
                chain_id = request.chain,
                contract_address = request.address,
                "Sourcify verification request received"
            );

            tracing::debug!(
                files = ?request.files,
                chosen_contract = request.chosen_contract,
                "Request details"
            );

            let result = process_verification_result(
                tokio::time::timeout(
                    self.operation_timeout,
                    sc_sourcify::api::verify(self.client.clone(), request.try_into()?),
                )
                .await
                .map_err(|_| Status::deadline_exceeded("Sourcify operation deadline exceeded"))?,
            )?;

            return Ok(Response::new(result.into_inner()));
        })
        .await
    }

    async fn verify_from_etherscan(
        &self,
        request: Request<VerifyFromEtherscanSourcifyRequest>,
    ) -> Result<Response<VerifyResponse>, Status> {
        let chain_ids = vec![Some(request.get_ref().chain.clone())];
        super::verification_metrics::track_verification(
            chain_ids,
            "solidity",
            "sourcify-from-etherscan",
            async {
                let request: VerifyFromEtherscanSourcifyRequestWrapper =
                    request.into_inner().into();

                tracing::info!(
                    chain_id = request.chain,
                    contract_address = request.address,
                    "Sourcify verification via etherscan request received"
                );

                let result = process_verification_result(
                    tokio::time::timeout(
                        self.operation_timeout,
                        sc_sourcify::api::verify_from_etherscan(
                            self.client.clone(),
                            request.try_into()?,
                        ),
                    )
                    .await
                    .map_err(|_| {
                        Status::deadline_exceeded("Sourcify operation deadline exceeded")
                    })?,
                )?;

                return Ok(Response::new(result.into_inner()));
            },
        )
        .await
    }
}

#[allow(clippy::result_large_err)]
fn process_verification_result(
    response: Result<sc_sourcify::Success, Error>,
) -> Result<VerifyResponseWrapper, Status> {
    match response {
        Ok(verification_success) => Ok(VerifyResponseWrapper::ok(verification_success)),
        Err(err) => match err {
            Error::Internal(err) => {
                tracing::error!("internal error: {err:#?}");
                Err(Status::internal(err.to_string()))
            }
            Error::BadRequest(err) => {
                tracing::error!("bad request error: {err:#?}");
                Err(Status::invalid_argument(err.to_string()))
            }
            Error::Verification(err) => Ok(VerifyResponseWrapper::err(err)),
            Error::Validation(err) => {
                tracing::debug!("invalid argument: {err:#?}");
                Err(Status::invalid_argument(err))
            }
        },
    }
}
