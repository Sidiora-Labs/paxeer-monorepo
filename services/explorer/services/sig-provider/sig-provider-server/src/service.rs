use async_trait::async_trait;
use ethabi::{ethereum_types::H256, Contract, RawLog};
use sig_provider::{SignatureSource, SourceAggregator};
use sig_provider_proto::blockscout::sig_provider::v1::{
    abi_service_server::AbiService, signature_service_server::SignatureService,
    BatchGetEventAbisRequest, BatchGetEventAbisResponse, CreateSignaturesRequest,
    CreateSignaturesResponse, GetEventAbiRequest, GetEventAbiResponse, GetFunctionAbiRequest,
    GetFunctionAbiResponse,
};
use std::sync::Arc;

#[derive(Clone)]
pub struct Service {
    agg: Arc<SourceAggregator>,
    import_sources: Vec<Arc<dyn SignatureSource + Send + Sync>>,
}

impl Service {
    pub fn new(agg: Arc<SourceAggregator>) -> Self {
        Self::with_import_sources(agg, Vec::new())
    }

    pub fn with_import_sources(
        agg: Arc<SourceAggregator>,
        import_sources: Vec<Arc<dyn SignatureSource + Send + Sync>>,
    ) -> Self {
        Self {
            agg,
            import_sources,
        }
    }
}

#[async_trait]
impl SignatureService for Service {
    async fn create_signatures(
        &self,
        request: tonic::Request<CreateSignaturesRequest>,
    ) -> Result<tonic::Response<CreateSignaturesResponse>, tonic::Status> {
        let request = request.into_inner();
        validate_parameter_types(&request.abi)?;
        let contract = Contract::load(request.abi.as_bytes())
            .map_err(|error| tonic::Status::invalid_argument(format!("invalid ABI: {error}")))?;
        let functions: Vec<_> = contract
            .functions()
            .map(|function| {
                (
                    hex::encode(function.short_signature()),
                    canonical_signature(
                        &function.name,
                        function.inputs.iter().map(|input| &input.kind),
                    ),
                )
            })
            .collect();
        let events: Vec<_> = contract
            .events()
            .map(|event| {
                (
                    hex::encode(event.signature()),
                    canonical_signature(&event.name, event.inputs.iter().map(|input| &input.kind)),
                )
            })
            .collect();
        if functions.is_empty() && events.is_empty() {
            return Ok(tonic::Response::new(CreateSignaturesResponse {}));
        }
        if self.import_sources.is_empty() {
            return Err(tonic::Status::unavailable(
                "no signature import sources configured",
            ));
        }

        let deliveries = self.import_sources.iter().map(|source| async {
            source
                .create_signatures(&request.abi)
                .await
                .map_err(|error| {
                    tonic::Status::unavailable(format!("signature import failed: {error}"))
                })?;
            // Source clients currently discard import HTTP statuses. A readback
            // confirms delivery even when that response alone cannot be trusted.
            for (selector, signature) in &functions {
                let signatures =
                    source
                        .get_function_signatures(selector)
                        .await
                        .map_err(|error| {
                            tonic::Status::unavailable(format!(
                                "signature import confirmation failed: {error}"
                            ))
                        })?;
                if !signatures.contains(signature) {
                    return Err(tonic::Status::unavailable(
                        "function signature import is not confirmed",
                    ));
                }
            }
            for (topic, signature) in &events {
                let signatures = source.get_event_signatures(topic).await.map_err(|error| {
                    tonic::Status::unavailable(format!(
                        "signature import confirmation failed: {error}"
                    ))
                })?;
                if !signatures.contains(signature) {
                    return Err(tonic::Status::unavailable(
                        "event signature import is not confirmed",
                    ));
                }
            }
            Ok(())
        });
        for delivery in futures::future::join_all(deliveries).await {
            delivery?;
        }
        Ok(tonic::Response::new(CreateSignaturesResponse {}))
    }
}

fn canonical_signature<'a>(
    name: &str,
    inputs: impl Iterator<Item = &'a ethabi::ParamType>,
) -> String {
    let inputs = inputs
        .map(ToString::to_string)
        .collect::<Vec<_>>()
        .join(",");
    format!("{name}({inputs})")
}

#[derive(serde::Deserialize)]
struct AbiEntryTypes {
    #[serde(default)]
    inputs: Vec<AbiParameterType>,
    #[serde(default)]
    outputs: Vec<AbiParameterType>,
}

#[derive(serde::Deserialize)]
struct AbiParameterType {
    #[serde(rename = "type")]
    kind: String,
    components: Option<Vec<AbiParameterType>>,
}

impl AbiParameterType {
    fn is_valid(&self) -> bool {
        let (base, mut arrays) = self
            .kind
            .split_once('[')
            .map(|(base, _)| (base, &self.kind[base.len()..]))
            .unwrap_or((&self.kind, ""));
        while !arrays.is_empty() {
            let Some((length, rest)) = arrays
                .strip_prefix('[')
                .and_then(|array| array.split_once(']'))
            else {
                return false;
            };
            if !length.is_empty()
                && !length
                    .parse::<usize>()
                    .map(|length_value| length_value > 0 && length_value.to_string() == length)
                    .unwrap_or(false)
            {
                return false;
            }
            arrays = rest;
        }
        if base == "tuple" {
            return self
                .components
                .as_ref()
                .map(|components| components.iter().all(Self::is_valid))
                .unwrap_or(false);
        }
        if self.components.is_some() {
            return false;
        }
        match base {
            "address" | "bool" | "string" | "bytes" | "int" | "uint" => true,
            _ => {
                let (width, max_width, step) = if let Some(width) = base
                    .strip_prefix("uint")
                    .or_else(|| base.strip_prefix("int"))
                {
                    (width, 256, 8)
                } else if let Some(width) = base.strip_prefix("bytes") {
                    (width, 32, 1)
                } else {
                    return false;
                };
                width
                    .parse::<usize>()
                    .map(|value| {
                        value > 0
                            && value <= max_width
                            && value % step == 0
                            && value.to_string() == width
                    })
                    .unwrap_or(false)
            }
        }
    }
}

// ethabi treats unknown JSON ABI types as legacy enum names (uint8). Check
// their original spelling too, before accepting or forwarding an import.
#[allow(clippy::result_large_err)]
fn validate_parameter_types(abi: &str) -> Result<(), tonic::Status> {
    let document = format!("{{\"abi\":{abi}}}");
    let entries: Vec<AbiEntryTypes> = config::Config::builder()
        .add_source(config::File::from_str(&document, config::FileFormat::Json))
        .build()
        .and_then(|config| config.get("abi"))
        .map_err(|error| tonic::Status::invalid_argument(format!("invalid ABI: {error}")))?;
    if entries
        .iter()
        .flat_map(|entry| entry.inputs.iter().chain(&entry.outputs))
        .all(AbiParameterType::is_valid)
    {
        Ok(())
    } else {
        Err(tonic::Status::invalid_argument(
            "invalid ABI parameter type",
        ))
    }
}

// todo: remove once https://github.com/hyperium/tonic/pull/2282 is released
#[allow(clippy::result_large_err)]
fn decode(str: &str) -> Result<Vec<u8>, tonic::Status> {
    hex::decode(str.strip_prefix("0x").unwrap_or(str))
        .map_err(|e| tonic::Status::invalid_argument(e.to_string()))
}

#[async_trait]
impl AbiService for Service {
    async fn get_function_abi(
        &self,
        request: tonic::Request<GetFunctionAbiRequest>,
    ) -> Result<tonic::Response<GetFunctionAbiResponse>, tonic::Status> {
        let request = request.into_inner();
        let bytes = decode(&request.tx_input)?;
        self.agg
            .get_function_abi(&bytes)
            .await
            .map(|abi| tonic::Response::new(GetFunctionAbiResponse { abi }))
            .map_err(|e| tonic::Status::internal(e.to_string()))
    }

    async fn get_event_abi(
        &self,
        request: tonic::Request<GetEventAbiRequest>,
    ) -> Result<tonic::Response<GetEventAbiResponse>, tonic::Status> {
        let request = request.into_inner();

        let topics = parse_topics(request.topics)?;
        self.agg
            .get_event_abi(RawLog {
                data: decode(&request.data)?,
                topics,
            })
            .await
            .map(|abi| GetEventAbiResponse { abi })
            .map_err(|e| tonic::Status::internal(e.to_string()))
            .map(tonic::Response::new)
    }

    async fn batch_get_event_abis(
        &self,
        request: tonic::Request<BatchGetEventAbisRequest>,
    ) -> Result<tonic::Response<BatchGetEventAbisResponse>, tonic::Status> {
        let batch_request = request.into_inner();

        let mut raw_logs = Vec::new();
        for request in batch_request.requests {
            let topics = parse_topics(request.topics)?;
            raw_logs.push(RawLog {
                data: decode(&request.data)?,
                topics,
            });
        }

        let batch_abis = self
            .agg
            .batch_get_event_abi(raw_logs)
            .await
            .map_err(|e| tonic::Status::internal(e.to_string()))?;

        let mut responses = Vec::new();
        for abi in batch_abis {
            responses.push(GetEventAbiResponse { abi })
        }

        Ok(tonic::Response::new(BatchGetEventAbisResponse {
            responses,
        }))
    }
}

// todo: remove once https://github.com/hyperium/tonic/pull/2282 is released
#[allow(clippy::result_large_err)]
fn parse_topics(topics: String) -> Result<Vec<H256>, tonic::Status> {
    topics
        .split(',')
        .map(|topic| {
            let hex = decode(topic)?;
            if hex.len() != 32 {
                return Err(tonic::Status::invalid_argument(
                    "topic len must be 32 bytes",
                ));
            }
            Ok(H256::from_slice(&hex))
        })
        .collect()
}
