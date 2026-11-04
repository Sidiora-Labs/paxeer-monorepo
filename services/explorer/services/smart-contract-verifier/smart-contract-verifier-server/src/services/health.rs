use crate::proto::{
    health_check_response,
    health_server::{Health, HealthServer},
    HealthCheckRequest, HealthCheckResponse,
};
use tonic::server::NamedService;

#[derive(Default)]
pub struct HealthService {}

#[async_trait::async_trait]
impl Health for HealthService {
    async fn check(
        &self,
        request: tonic::Request<HealthCheckRequest>,
    ) -> Result<tonic::Response<HealthCheckResponse>, tonic::Status> {
        let service = request.into_inner().service;
        if !service.is_empty() && service != <HealthServer<Self> as NamedService>::NAME {
            return Err(tonic::Status::not_found("unknown service"));
        }

        Ok(tonic::Response::new(HealthCheckResponse {
            status: health_check_response::ServingStatus::Serving as i32,
        }))
    }
}
