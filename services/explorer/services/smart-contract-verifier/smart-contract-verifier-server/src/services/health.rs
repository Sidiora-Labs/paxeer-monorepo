use crate::proto::{
    health_check_response,
    health_server::{Health, HealthServer},
    HealthCheckRequest, HealthCheckResponse,
};
use std::collections::HashSet;
use tonic::server::NamedService;

#[derive(Default)]
pub struct HealthService {
    registered_services: HashSet<String>,
}

impl HealthService {
    pub fn new(registered_services: impl IntoIterator<Item = impl Into<String>>) -> Self {
        Self {
            registered_services: registered_services.into_iter().map(Into::into).collect(),
        }
    }
}

#[async_trait::async_trait]
impl Health for HealthService {
    async fn check(
        &self,
        request: tonic::Request<HealthCheckRequest>,
    ) -> Result<tonic::Response<HealthCheckResponse>, tonic::Status> {
        let service = request.into_inner().service;
        if !service.is_empty()
            && service != <HealthServer<Self> as NamedService>::NAME
            && !self.registered_services.contains(&service)
        {
            return Err(tonic::Status::not_found("unknown service"));
        }

        Ok(tonic::Response::new(HealthCheckResponse {
            status: health_check_response::ServingStatus::Serving as i32,
        }))
    }
}
