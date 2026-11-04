use crate::{
    proto::{
        health_actix::route_health,
        health_server::HealthServer,
        solidity_verifier_actix::route_solidity_verifier,
        solidity_verifier_server::SolidityVerifierServer,
        sourcify_verifier_actix::route_sourcify_verifier,
        sourcify_verifier_server::SourcifyVerifierServer,
        vyper_verifier_actix::route_vyper_verifier,
        vyper_verifier_server::VyperVerifierServer,
        zksync::solidity::{
            zk_sync_solidity_verifier_actix::route_zk_sync_solidity_verifier,
            zk_sync_solidity_verifier_server::ZkSyncSolidityVerifierServer,
        },
    },
    services::{
        zksync_solidity_verifier, HealthService, SolidityVerifierService, SourcifyVerifierService,
        VyperVerifierService,
    },
    settings::Settings,
};
use blockscout_service_launcher::launcher::{self, LaunchSettings};
use std::{future::Future, sync::Arc, time::Duration};
use tokio::sync::Semaphore;

const QUEUE_DEADLINE: Duration = Duration::from_secs(2);
const REQUEST_DEADLINE: Duration = Duration::from_secs(30);
const PROVIDER_STARTUP_DEADLINE: Duration = Duration::from_secs(10);

#[derive(Clone)]
struct Admission {
    admitted: Arc<Semaphore>,
    running: Arc<Semaphore>,
}

impl Admission {
    fn new(capacity: usize) -> Self {
        Self {
            admitted: Arc::new(Semaphore::new(capacity.saturating_mul(2))),
            running: Arc::new(Semaphore::new(capacity)),
        }
    }

    // RESOURCE_EXHAUSTED (HTTP 429): all active and queued slots are occupied.
    // DEADLINE_EXCEEDED (HTTP 504): queue >2s or response >30s.
    // A detached compiler may outlive its response. Keep its permits until it actually
    // finishes, including after client cancellation, so it cannot overcommit the pool.
    async fn execute<T: Send + 'static>(
        &self,
        operation: impl Future<Output = Result<tonic::Response<T>, tonic::Status>> + Send + 'static,
    ) -> Result<tonic::Response<T>, tonic::Status> {
        let admitted = self.admitted.clone().try_acquire_owned().map_err(|_| {
            tonic::Status::resource_exhausted("verification busy: bounded queue is full")
        })?;
        let running = tokio::time::timeout(QUEUE_DEADLINE, self.running.clone().acquire_owned())
            .await
            .map_err(|_| tonic::Status::deadline_exceeded("verification queue deadline exceeded"))?
            .map_err(|_| tonic::Status::unavailable("verification admission closed"))?;
        let task = tokio::spawn(async move {
            let (_admitted, _running) = (admitted, running);
            operation.await
        });
        tokio::time::timeout(REQUEST_DEADLINE, task)
            .await
            .map_err(|_| {
                tonic::Status::deadline_exceeded("verification response deadline exceeded")
            })?
            .map_err(|_| tonic::Status::internal("verification worker failed"))?
    }
}

struct BoundedService<T> {
    service: Option<Arc<T>>,
    admission: Admission,
}

impl<T> BoundedService<T> {
    fn new(service: Option<T>, admission: &Admission) -> Arc<Self> {
        Arc::new(Self {
            service: service.map(Arc::new),
            admission: admission.clone(),
        })
    }
}

macro_rules! bounded_service {
    ($service:ty, $trait:path, { $($method:ident($input:ty) -> $output:ty),* $(,)? },
     { $($read:ident($read_input:ty) -> $read_output:ty),* $(,)? }) => {
        #[async_trait::async_trait]
        impl $trait for BoundedService<$service> {
            $(async fn $method(&self, request: tonic::Request<$input>)
                -> Result<tonic::Response<$output>, tonic::Status> {
                let service = self.service.clone().ok_or_else(||
                    tonic::Status::unavailable("compiler provider unavailable; catalogue initialization failed"))?;
                self.admission.execute(async move { service.$method(request).await }).await
            })*
            $(async fn $read(&self, request: tonic::Request<$read_input>)
                -> Result<tonic::Response<$read_output>, tonic::Status> {
                let service = self.service.as_ref().ok_or_else(||
                    tonic::Status::unavailable("compiler provider unavailable; catalogue initialization failed"))?;
                service.$read(request).await
            })*
        }
    };
}

use crate::proto::*;
use crate::proto::{
    health_server::Health, solidity_verifier_server::SolidityVerifier,
    sourcify_verifier_server::SourcifyVerifier, vyper_verifier_server::VyperVerifier,
    zksync::solidity::zk_sync_solidity_verifier_server::ZkSyncSolidityVerifier,
};
bounded_service!(SolidityVerifierService, solidity_verifier_server::SolidityVerifier, {
    verify_multi_part(VerifySolidityMultiPartRequest) -> VerifyResponse,
    verify_standard_json(VerifySolidityStandardJsonRequest) -> VerifyResponse,
    batch_verify_multi_part(BatchVerifySolidityMultiPartRequest) -> BatchVerifyResponse,
    batch_verify_standard_json(BatchVerifySolidityStandardJsonRequest) -> BatchVerifyResponse,
}, {
    list_compiler_versions(ListCompilerVersionsRequest) -> ListCompilerVersionsResponse,
    lookup_methods(LookupMethodsRequest) -> LookupMethodsResponse,
});
bounded_service!(VyperVerifierService, vyper_verifier_server::VyperVerifier, {
    verify_multi_part(VerifyVyperMultiPartRequest) -> VerifyResponse,
    verify_standard_json(VerifyVyperStandardJsonRequest) -> VerifyResponse,
}, { list_compiler_versions(ListCompilerVersionsRequest) -> ListCompilerVersionsResponse });
bounded_service!(SourcifyVerifierService, sourcify_verifier_server::SourcifyVerifier, {
    verify(VerifySourcifyRequest) -> VerifyResponse,
    verify_from_etherscan(VerifyFromEtherscanSourcifyRequest) -> VerifyResponse,
}, {});
bounded_service!(zksync_solidity_verifier::Service,
    zksync::solidity::zk_sync_solidity_verifier_server::ZkSyncSolidityVerifier, {
    verify_standard_json(zksync::solidity::VerifyStandardJsonRequest) -> zksync::solidity::VerifyResponse,
}, { list_compilers(zksync::solidity::ListCompilersRequest) -> zksync::solidity::ListCompilersResponse });

async fn initialize_provider<T>(
    name: &'static str,
    enabled: bool,
    initialize: impl Future<Output = anyhow::Result<T>>,
) -> Option<T> {
    if !enabled {
        return None;
    }
    match tokio::time::timeout(PROVIDER_STARTUP_DEADLINE, initialize).await {
        Ok(Ok(service)) => Some(service),
        Ok(Err(error)) => {
            tracing::error!(provider = name, error = %error, "compiler provider not ready");
            None
        }
        Err(_) => {
            tracing::error!(
                provider = name,
                "compiler provider startup deadline exceeded"
            );
            None
        }
    }
}

struct ProviderHealth {
    ready: HealthService,
    unavailable: Vec<&'static str>,
}

#[async_trait::async_trait]
impl health_server::Health for ProviderHealth {
    async fn check(
        &self,
        request: tonic::Request<HealthCheckRequest>,
    ) -> Result<tonic::Response<HealthCheckResponse>, tonic::Status> {
        if self
            .unavailable
            .contains(&request.get_ref().service.as_str())
        {
            return Ok(tonic::Response::new(HealthCheckResponse {
                status: health_check_response::ServingStatus::NotServing as i32,
            }));
        }
        self.ready.check(request).await
    }
}

#[derive(Clone)]
struct HttpRouter {
    solidity_verifier: Option<Arc<BoundedService<SolidityVerifierService>>>,
    vyper_verifier: Option<Arc<BoundedService<VyperVerifierService>>>,
    sourcify_verifier: Option<Arc<BoundedService<SourcifyVerifierService>>>,
    zksync_solidity_verifier: Option<Arc<BoundedService<zksync_solidity_verifier::Service>>>,
    health: Arc<ProviderHealth>,
}

impl launcher::HttpRouter for HttpRouter {
    fn register_routes(&self, service_config: &mut actix_web::web::ServiceConfig) {
        let service_config =
            service_config.configure(|config| route_health(config, self.health.clone()));

        let service_config = if let Some(solidity) = &self.solidity_verifier {
            service_config.configure(|config| route_solidity_verifier(config, solidity.clone()))
        } else {
            service_config
        };
        let service_config = if let Some(vyper) = &self.vyper_verifier {
            service_config.configure(|config| route_vyper_verifier(config, vyper.clone()))
        } else {
            service_config
        };
        let service_config = if let Some(sourcify) = &self.sourcify_verifier {
            service_config.configure(|config| route_sourcify_verifier(config, sourcify.clone()))
        } else {
            service_config
        };
        let service_config = if let Some(zksync_solidity) = &self.zksync_solidity_verifier {
            service_config.configure(|config| {
                route_zk_sync_solidity_verifier(config, zksync_solidity.clone())
            })
        } else {
            service_config
        };

        let _ = service_config;
    }
}

fn grpc_router(
    solidity_verifier: Option<Arc<BoundedService<SolidityVerifierService>>>,
    vyper_verifier: Option<Arc<BoundedService<VyperVerifierService>>>,
    sourcify_verifier: Option<Arc<BoundedService<SourcifyVerifierService>>>,
    zksync_solidity_verifier: Option<Arc<BoundedService<zksync_solidity_verifier::Service>>>,
    health: Arc<ProviderHealth>,
) -> tonic::transport::server::Router {
    tonic::transport::Server::builder()
        .add_service(HealthServer::from_arc(health))
        .add_optional_service(solidity_verifier.map(SolidityVerifierServer::from_arc))
        .add_optional_service(vyper_verifier.map(VyperVerifierServer::from_arc))
        .add_optional_service(sourcify_verifier.map(SourcifyVerifierServer::from_arc))
        .add_optional_service(zksync_solidity_verifier.map(ZkSyncSolidityVerifierServer::from_arc))
}

pub async fn run(settings: Settings) -> Result<(), anyhow::Error> {
    crate::metrics::configure_chain_labels()?;
    blockscout_service_launcher::tracing::init_logs(
        "smart_contract_verifier",
        &settings.tracing,
        &settings.jaeger,
    )?;
    anyhow::ensure!(
        settings.compilers.max_threads.get() <= 1024,
        "compiler concurrency must not exceed 1024"
    );
    let compilers_lock = Arc::new(Semaphore::new(settings.compilers.max_threads.get()));
    let admission = Admission::new(settings.compilers.max_threads.get());
    let (solidity_enabled, vyper_enabled, sourcify_enabled, zksync_enabled) = (
        settings.solidity.enabled,
        settings.vyper.enabled,
        settings.sourcify.enabled,
        settings.zksync_solidity.enabled,
    );
    // Independent providers initialize concurrently within one bounded startup window.
    let (solidity, vyper, sourcify, zksync) = tokio::join!(
        initialize_provider(
            "solidity",
            solidity_enabled,
            SolidityVerifierService::new(settings.solidity, compilers_lock.clone())
        ),
        initialize_provider(
            "vyper",
            vyper_enabled,
            VyperVerifierService::new(settings.vyper, compilers_lock.clone())
        ),
        initialize_provider(
            "sourcify",
            sourcify_enabled,
            SourcifyVerifierService::new(settings.sourcify)
        ),
        initialize_provider(
            "zksync",
            zksync_enabled,
            zksync_solidity_verifier::Service::new(
                settings.zksync_solidity,
                compilers_lock.clone()
            )
        ),
    );
    use tonic::server::NamedService;
    let mut ready = Vec::new();
    let mut unavailable = Vec::new();
    if solidity.is_some() {
        ready.push(SolidityVerifierServer::<BoundedService<SolidityVerifierService>>::NAME);
    }
    if vyper.is_some() {
        ready.push(VyperVerifierServer::<BoundedService<VyperVerifierService>>::NAME);
    }
    if sourcify.is_some() {
        ready.push(SourcifyVerifierServer::<BoundedService<SourcifyVerifierService>>::NAME);
    }
    if zksync.is_some() {
        ready.push(
            ZkSyncSolidityVerifierServer::<BoundedService<zksync_solidity_verifier::Service>>::NAME,
        );
    }
    for (enabled, available, name) in [
        (
            solidity_enabled,
            solidity.is_some(),
            SolidityVerifierServer::<BoundedService<SolidityVerifierService>>::NAME,
        ),
        (
            vyper_enabled,
            vyper.is_some(),
            VyperVerifierServer::<BoundedService<VyperVerifierService>>::NAME,
        ),
        (
            sourcify_enabled,
            sourcify.is_some(),
            SourcifyVerifierServer::<BoundedService<SourcifyVerifierService>>::NAME,
        ),
        (
            zksync_enabled,
            zksync.is_some(),
            ZkSyncSolidityVerifierServer::<BoundedService<zksync_solidity_verifier::Service>>::NAME,
        ),
    ] {
        if enabled && !available {
            unavailable.push(name);
        }
    }
    let health = Arc::new(ProviderHealth {
        ready: HealthService::new(ready),
        unavailable,
    });
    // Keep enabled routes registered even when unavailable, with an explicit 503/UNAVAILABLE.
    let solidity_verifier = solidity_enabled.then(|| BoundedService::new(solidity, &admission));
    let vyper_verifier = vyper_enabled.then(|| BoundedService::new(vyper, &admission));
    let sourcify_verifier = sourcify_enabled.then(|| BoundedService::new(sourcify, &admission));
    let zksync_solidity_verifier = zksync_enabled.then(|| BoundedService::new(zksync, &admission));
    let grpc_router = grpc_router(
        solidity_verifier.clone(),
        vyper_verifier.clone(),
        sourcify_verifier.clone(),
        zksync_solidity_verifier.clone(),
        health.clone(),
    );
    let http_router = HttpRouter {
        solidity_verifier,
        vyper_verifier,
        sourcify_verifier,
        zksync_solidity_verifier,
        health,
    };
    let launch_settings = LaunchSettings {
        service_name: "smart_contract_verifier".to_owned(),
        server: settings.server,
        metrics: settings.metrics,
    };

    launcher::launch(&launch_settings, http_router, grpc_router).await
}
