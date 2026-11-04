use crate::{
    proto::{
        solidity_verifier_server::SolidityVerifier, BatchVerifyResponse,
        BatchVerifySolidityMultiPartRequest, BatchVerifySolidityStandardJsonRequest,
        ListCompilerVersionsRequest, ListCompilerVersionsResponse, VerifyResponse,
        VerifySolidityMultiPartRequest, VerifySolidityStandardJsonRequest,
    },
    services::common,
    settings::SoliditySettings,
    types,
    types::{LookupMethodsRequestWrapper, LookupMethodsResponseWrapper},
};
use anyhow::Context;
use smart_contract_verifier::{
    find_methods, solidity, EvmCompilersPool, SolcCompiler, SolcValidator,
};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    LookupMethodsRequest, LookupMethodsResponse,
};
use std::sync::Arc;
use tokio::sync::Semaphore;
use tonic::{Request, Response, Status};

pub struct SolidityVerifierService {
    compilers: Arc<EvmCompilersPool<SolcCompiler>>,
}

impl SolidityVerifierService {
    /// The shared collector for verification boundaries, including embedded services.
    #[doc(hidden)]
    pub fn verification_counter() -> &'static prometheus::IntCounterVec {
        &crate::metrics::VERIFICATION
    }

    pub async fn new(
        settings: SoliditySettings,
        compilers_threads_semaphore: Arc<Semaphore>,
    ) -> anyhow::Result<Self> {
        let solc_validator = Arc::new(SolcValidator::default());
        let fetcher = common::initialize_fetcher(
            settings.fetcher,
            settings.compilers_dir.clone(),
            settings.refresh_versions_schedule,
            Some(solc_validator),
        )
        .await
        .context("solidity fetcher initialization")?;

        let compilers: EvmCompilersPool<SolcCompiler> =
            EvmCompilersPool::new(fetcher, compilers_threads_semaphore);
        compilers.load_from_dir(&settings.compilers_dir).await;

        Ok(Self {
            compilers: Arc::new(compilers),
        })
    }
}

#[async_trait::async_trait]
impl SolidityVerifier for SolidityVerifierService {
    async fn verify_multi_part(
        &self,
        request: Request<VerifySolidityMultiPartRequest>,
    ) -> Result<Response<VerifyResponse>, Status> {
        let chain_ids = vec![request
            .get_ref()
            .metadata
            .as_ref()
            .and_then(|metadata| metadata.chain_id.clone())];
        super::verification_metrics::track_verification(
            chain_ids, "solidity", "multi-part", async {
        let request = request.into_inner();

        let chain_id = request
            .metadata
            .as_ref()
            .and_then(|metadata| metadata.chain_id.clone());
        let contract_address = request
            .metadata
            .as_ref()
            .and_then(|metadata| metadata.contract_address.clone());
        tracing::info!(
            chain_id =? chain_id,
            contract_address =? contract_address,
            "solidity multi-part verification request received"
        );

        common::validate_source_paths(request.source_files.keys())?;

        let maybe_verification_request =
            solidity::multi_part::VerificationRequest::try_from_with_post_actions(request);
        let (verification_request, post_actions) =
            common::process_solo_verification_request_conversion!(maybe_verification_request);

        let result = solidity::multi_part::verify(&self.compilers, verification_request).await;

        let verify_response = match result {
            Ok(value) => {
                types::verification_result::process_verification_result(value, &post_actions)?
            }
            Err(error) => types::verification_result::process_error(error)?,
        };


        Ok(Response::new(verify_response))

            },
        ).await
    }

    async fn verify_standard_json(
        &self,
        request: Request<VerifySolidityStandardJsonRequest>,
    ) -> Result<Response<VerifyResponse>, Status> {
        let chain_ids = vec![request
            .get_ref()
            .metadata
            .as_ref()
            .and_then(|metadata| metadata.chain_id.clone())];
        super::verification_metrics::track_verification(
            chain_ids, "solidity", "standard-json", async {
        let request = request.into_inner();

        let chain_id = request
            .metadata
            .as_ref()
            .and_then(|metadata| metadata.chain_id.clone());
        let contract_address = request
            .metadata
            .as_ref()
            .and_then(|metadata| metadata.contract_address.clone());
        tracing::info!(
            chain_id =? chain_id,
            contract_address =? contract_address,
            "solidity standard-json verification request received"
        );

        let input = serde_json::from_str(&request.input);
        let maybe_verification_request = common::validate_standard_json_source_paths(input, || {
            solidity::standard_json::VerificationRequest::try_from_with_post_actions(request)
        });
        let (verification_request, post_actions) =
            common::process_solo_verification_request_conversion!(maybe_verification_request);

        let result = solidity::standard_json::verify(&self.compilers, verification_request).await;

        let verify_response = match result {
            Ok(value) => {
                types::verification_result::process_verification_result(value, &post_actions)?
            }
            Err(error) => types::verification_result::process_error(error)?,
        };


        Ok(Response::new(verify_response))

            },
        ).await
    }

    async fn batch_verify_multi_part(
        &self,
        request: Request<BatchVerifySolidityMultiPartRequest>,
    ) -> Result<Response<BatchVerifyResponse>, Status> {
        let chain_ids = request
            .get_ref()
            .contracts
            .iter()
            .map(|contract| {
                contract
                    .metadata
                    .as_ref()
                    .and_then(|metadata| metadata.chain_id.clone())
            })
            .collect();
        super::verification_metrics::track_verification(
            chain_ids,
            "solidity",
            "batch-multi-part",
            async {
                let request = request.into_inner();

                common::validate_source_paths(request.sources.keys())?;

                let maybe_verification_request =
                    solidity::multi_part::BatchVerificationRequest::try_from(request);
                let verification_request = common::process_batch_verification_request_conversion!(
                    maybe_verification_request
                );

                let result =
                    solidity::multi_part::batch_verify(&self.compilers, verification_request).await;

                let verify_response = match result {
                    Ok(value) => types::batch_verification::process_verification_results(value)?,
                    Err(error) => types::batch_verification::process_error(error)?,
                };

                Ok(Response::new(verify_response))
            },
        )
        .await
    }

    async fn batch_verify_standard_json(
        &self,
        request: Request<BatchVerifySolidityStandardJsonRequest>,
    ) -> Result<Response<BatchVerifyResponse>, Status> {
        let chain_ids = request
            .get_ref()
            .contracts
            .iter()
            .map(|contract| {
                contract
                    .metadata
                    .as_ref()
                    .and_then(|metadata| metadata.chain_id.clone())
            })
            .collect();
        super::verification_metrics::track_verification(
            chain_ids,
            "solidity",
            "batch-standard-json",
            async {
                let request = request.into_inner();

                let input = serde_json::from_str(&request.input);
                let maybe_verification_request =
                    common::validate_standard_json_source_paths(input, || {
                        solidity::standard_json::BatchVerificationRequest::try_from(request)
                    });
                let verification_request = common::process_batch_verification_request_conversion!(
                    maybe_verification_request
                );

                let result =
                    solidity::standard_json::batch_verify(&self.compilers, verification_request)
                        .await;

                let verify_response = match result {
                    Ok(value) => types::batch_verification::process_verification_results(value)?,
                    Err(error) => types::batch_verification::process_error(error)?,
                };

                Ok(Response::new(verify_response))
            },
        )
        .await
    }

    async fn list_compiler_versions(
        &self,
        _request: Request<ListCompilerVersionsRequest>,
    ) -> Result<Response<ListCompilerVersionsResponse>, Status> {
        let compiler_versions = self.compilers.all_versions();
        Ok(Response::new(ListCompilerVersionsResponse {
            compiler_versions: common::versions_to_sorted_string(compiler_versions),
        }))
    }

    async fn lookup_methods(
        &self,
        request: Request<LookupMethodsRequest>,
    ) -> Result<Response<LookupMethodsResponse>, Status> {
        let request: LookupMethodsRequestWrapper = request.into_inner().into();
        let methods = find_methods(request.try_into()?);
        let response = LookupMethodsResponseWrapper::from(methods);
        Ok(Response::new(response.into()))
    }
}
