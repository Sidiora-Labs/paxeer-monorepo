use sig_provider_proto::blockscout::sig_provider::v1::{
    abi_service_server::AbiServiceServer, health_check_response, health_server::Health,
    health_server::HealthServer, signature_service_server::SignatureServiceServer,
    HealthCheckRequest, HealthCheckResponse,
};
use tonic::server::NamedService;

const SUPPORTED_SERVICES: &[&str] = &[
    "",
    <HealthServer<HealthService> as NamedService>::NAME,
    <SignatureServiceServer<crate::Service> as NamedService>::NAME,
    <AbiServiceServer<crate::Service> as NamedService>::NAME,
];

#[derive(Default)]
pub struct HealthService {}

#[async_trait::async_trait]
impl Health for HealthService {
    async fn check(
        &self,
        request: tonic::Request<HealthCheckRequest>,
    ) -> Result<tonic::Response<HealthCheckResponse>, tonic::Status> {
        if !SUPPORTED_SERVICES.contains(&request.into_inner().service.as_str()) {
            return Err(tonic::Status::not_found("unknown service"));
        }
        Ok(tonic::Response::new(HealthCheckResponse {
            status: health_check_response::ServingStatus::Serving as i32,
        }))
    }
}
