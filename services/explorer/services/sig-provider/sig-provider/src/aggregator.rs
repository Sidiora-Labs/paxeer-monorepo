use crate::{sources::CompleteSignatureSource, SignatureSource};
use anyhow::Context;
use ethabi::{Event, EventParam, ParamType, RawLog, Token};
use itertools::Itertools;
use sig_provider_proto::blockscout::sig_provider::v1::{Abi, Argument};
use std::{collections::HashSet, sync::Arc};

pub struct SourceAggregator {
    sources: Vec<Arc<dyn SignatureSource + Send + Sync + 'static>>,
    complete_sources: Vec<Arc<dyn CompleteSignatureSource + Send + Sync + 'static>>,
}

/// Outcome of a source for a particular key and operation, in priority order.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum SourceStatus {
    Succeeded,
    Failed(String),
    OutsideCoverage,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SourceOutcome {
    pub source: String,
    pub operation: &'static str,
    pub status: SourceStatus,
}

#[derive(Clone, Debug)]
pub struct LookupReport<T> {
    pub value: T,
    pub coverage: Vec<SourceOutcome>,
}

#[derive(Clone, Debug)]
pub struct LookupUnavailable {
    pub coverage: Vec<SourceOutcome>,
}

impl std::fmt::Display for LookupUnavailable {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "signature lookup unavailable: {:?}", self.coverage)
    }
}
impl std::error::Error for LookupUnavailable {}

#[derive(Clone, Debug)]
pub struct ImportRefused {
    pub coverage: Vec<SourceOutcome>,
}
impl std::fmt::Display for ImportRefused {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "signature import refused: {:?}", self.coverage)
    }
}
impl std::error::Error for ImportRefused {}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum EventLayout {
    /// The source supplied the producer's indexed flags, names and order.
    Exact,
    /// One possible layout inferred from a canonical signature alone.
    Candidate,
}

#[derive(Clone, Debug)]
pub struct EventCandidate {
    pub abi: Abi,
    pub layout: EventLayout,
}

fn record_response<T>(
    source: String,
    operation: &'static str,
    response: Option<Result<T, anyhow::Error>>,
    coverage: &mut Vec<SourceOutcome>,
) -> Option<T> {
    let (status, value) = match response {
        None => (SourceStatus::OutsideCoverage, None),
        Some(Ok(value)) => (SourceStatus::Succeeded, Some(value)),
        Some(Err(error)) => (SourceStatus::Failed(format!("{error:#}")), None),
    };
    coverage.push(SourceOutcome {
        source,
        operation,
        status,
    });
    value
}

fn ensure_available(coverage: &[SourceOutcome]) -> Result<(), anyhow::Error> {
    // An empty finite inventory is not a successful answer from a capable
    // directory. Preserve successful partial answers, including genuine misses.
    let succeeded = coverage
        .iter()
        .any(|outcome| outcome.status == SourceStatus::Succeeded);
    let failed = coverage
        .iter()
        .any(|outcome| matches!(outcome.status, SourceStatus::Failed(_)));
    if failed && !succeeded {
        return Err(LookupUnavailable {
            coverage: coverage.to_vec(),
        }
        .into());
    }
    Ok(())
}

impl SourceAggregator {
    // You should provide sources in priority descending order (first - max priority)
    pub fn new(
        sources: Vec<Arc<dyn SignatureSource + Send + Sync + 'static>>,
        complete_sources: Vec<Arc<dyn CompleteSignatureSource + Send + Sync + 'static>>,
    ) -> SourceAggregator {
        SourceAggregator {
            sources,
            complete_sources,
        }
    }

    fn merge_signatures<T, I: IntoIterator<Item = T>, II: IntoIterator<Item = I>>(
        sigs: II,
    ) -> Vec<T>
    where
        T: Clone + Eq + std::hash::Hash,
    {
        let mut content: HashSet<T> = HashSet::default();
        sigs.into_iter()
            .flatten()
            .filter(|sig| content.insert(sig.clone()))
            .collect()
    }

    pub async fn create_signatures(&self, abi: &str) -> Result<(), anyhow::Error> {
        self.create_signatures_with_coverage(abi).await.map(|_| ())
    }

    pub async fn create_signatures_with_coverage(
        &self,
        abi: &str,
    ) -> Result<LookupReport<()>, anyhow::Error> {
        let responses = futures::future::join_all(self.sources.iter().map(|source| async move {
            let response = if source.accepts_imports() {
                Some(source.create_signatures(abi).await)
            } else {
                None
            };
            (source.source(), response)
        }))
        .await;
        let mut coverage = Vec::new();
        for (source, response) in responses {
            record_response(source, "import", response, &mut coverage);
        }
        if !coverage
            .iter()
            .any(|outcome| outcome.status == SourceStatus::Succeeded)
            || coverage
                .iter()
                .any(|outcome| matches!(outcome.status, SourceStatus::Failed(_)))
        {
            return Err(ImportRefused { coverage }.into());
        }
        Ok(LookupReport {
            value: (),
            coverage,
        })
    }

    pub async fn get_function_signatures(&self, hex: &str) -> Result<Vec<String>, anyhow::Error> {
        Ok(self.get_function_signatures_with_coverage(hex).await?.value)
    }

    pub async fn get_function_signatures_with_coverage(
        &self,
        hex: &str,
    ) -> Result<LookupReport<Vec<String>>, anyhow::Error> {
        let responses = futures::future::join_all(self.sources.iter().map(|source| async move {
            let response = if source.covers_function(hex) {
                Some(source.get_function_signatures(hex).await)
            } else {
                None
            };
            (source.source(), response)
        }))
        .await;
        let mut coverage = Vec::new();
        let values = responses
            .into_iter()
            .filter_map(|(source, response)| {
                record_response(source, "function", response, &mut coverage)
            })
            .collect::<Vec<_>>();
        ensure_available(&coverage)?;
        Ok(LookupReport {
            value: Self::merge_signatures(values),
            coverage,
        })
    }

    pub async fn get_function_abi(&self, tx_input: &[u8]) -> Result<Vec<Abi>, anyhow::Error> {
        Ok(self.get_function_abi_with_coverage(tx_input).await?.value)
    }

    pub async fn get_function_abi_with_coverage(
        &self,
        tx_input: &[u8],
    ) -> Result<LookupReport<Vec<Abi>>, anyhow::Error> {
        if tx_input.len() < 4 {
            anyhow::bail!("tx input len must be at least 4 bytes");
        }
        let hex_sig = hex::encode(&tx_input[..4]);
        let report = self.get_function_signatures_with_coverage(&hex_sig).await?;
        let value = report
            .value
            .into_iter()
            .filter_map(|sig| {
                let (name, args) = parse_signature(&sig)?;
                let values = decode_txinput(&args, &tx_input[4..])?;
                Some(Abi {
                    name: name.into(),
                    inputs: parse_args("arg".into(), &args, &values),
                })
            })
            .collect();
        Ok(LookupReport {
            value,
            coverage: report.coverage,
        })
    }

    pub async fn get_event_abi(&self, raw: RawLog) -> Result<Vec<Abi>, anyhow::Error> {
        Ok(self
            .get_event_abi_with_coverage(raw)
            .await?
            .value
            .into_iter()
            .map(|candidate| candidate.abi)
            .collect())
    }

    pub async fn get_event_abi_with_coverage(
        &self,
        raw: RawLog,
    ) -> Result<LookupReport<Vec<EventCandidate>>, anyhow::Error> {
        let topic = raw
            .topics
            .first()
            .context("log should contain at least one topic")?;
        let key = hex::encode(topic.as_bytes());
        let key = &key;
        let (complete, signatures) = futures::join!(
            futures::future::join_all(self.complete_sources.iter().map(|source| async move {
                let response = if source.covers_event(key) {
                    Some(source.get_event_signatures(key).await)
                } else {
                    None
                };
                (source.source(), response)
            })),
            futures::future::join_all(self.sources.iter().map(|source| async move {
                let response = if source.covers_event(key) {
                    Some(source.get_event_signatures(key).await)
                } else {
                    None
                };
                (source.source(), response)
            }))
        );
        let mut coverage = Vec::new();
        let complete = complete
            .into_iter()
            .filter_map(|(source, response)| {
                record_response(source, "complete_event", response, &mut coverage)
            })
            .collect::<Vec<_>>();
        let signatures = signatures
            .into_iter()
            .filter_map(|(source, response)| {
                record_response(source, "event", response, &mut coverage)
            })
            .collect::<Vec<_>>();
        ensure_available(&coverage)?;
        let value = process_event_signatures(
            &raw,
            Self::merge_signatures(complete),
            Self::merge_signatures(signatures),
        )?;
        Ok(LookupReport { value, coverage })
    }

    pub async fn batch_get_event_abi(
        &self,
        raw_logs: Vec<RawLog>,
    ) -> Result<Vec<Vec<Abi>>, anyhow::Error> {
        Ok(self
            .batch_get_event_abi_with_coverage(raw_logs)
            .await?
            .into_iter()
            .map(|report| {
                report
                    .value
                    .into_iter()
                    .map(|candidate| candidate.abi)
                    .collect()
            })
            .collect())
    }

    pub async fn batch_get_event_abi_with_coverage(
        &self,
        raw_logs: Vec<RawLog>,
    ) -> Result<Vec<LookupReport<Vec<EventCandidate>>>, anyhow::Error> {
        let keys: Vec<_> = raw_logs
            .iter()
            .map(|raw| {
                raw.topics
                    .first()
                    .map(|topic| hex::encode(topic.as_bytes()))
                    .context("log should contain at least one topic")
            })
            .collect::<Result<_, _>>()?;
        if keys.is_empty() {
            return Ok(Vec::new());
        }
        let keys = &keys;
        let (complete, signatures) = futures::join!(
            futures::future::join_all(self.complete_sources.iter().map(|source| async move {
                let covered: Vec<_> = keys.iter().map(|key| source.covers_event(key)).collect();
                let response = if covered.iter().any(|covered| *covered) {
                    Some(source.batch_get_event_signatures(keys).await)
                } else {
                    None
                };
                (source.source(), covered, response)
            })),
            futures::future::join_all(self.sources.iter().map(|source| async move {
                let covered: Vec<_> = keys.iter().map(|key| source.covers_event(key)).collect();
                let response = if covered.iter().any(|covered| *covered) {
                    Some(source.batch_get_event_signatures(keys).await)
                } else {
                    None
                };
                (source.source(), covered, response)
            }))
        );
        let mut results = Vec::new();
        for (index, raw) in raw_logs.iter().enumerate() {
            let mut coverage = Vec::new();
            let complete = batch_row(
                &complete,
                index,
                keys.len(),
                "complete_event",
                &mut coverage,
            );
            let signatures = batch_row(&signatures, index, keys.len(), "event", &mut coverage);
            ensure_available(&coverage)?;
            let value = process_event_signatures(
                raw,
                Self::merge_signatures(complete),
                Self::merge_signatures(signatures),
            )?;
            results.push(LookupReport { value, coverage });
        }
        Ok(results)
    }
}

type BatchResponse<T> = (
    String,
    Vec<bool>,
    Option<Result<Vec<Vec<T>>, anyhow::Error>>,
);

fn batch_row<T: Clone>(
    responses: &[BatchResponse<T>],
    index: usize,
    count: usize,
    operation: &'static str,
    coverage: &mut Vec<SourceOutcome>,
) -> Vec<Vec<T>> {
    responses
        .iter()
        .filter_map(|(source, covered, response)| {
            let response = if !covered[index] {
                None
            } else {
                Some(
                    match response.as_ref().expect("covered batch must be requested") {
                        Ok(rows) if rows.len() == count => Ok(rows[index].clone()),
                        Ok(rows) => Err(anyhow::anyhow!(
                            "invalid batch response length: expected {count}, got {}",
                            rows.len()
                        )),
                        Err(error) => Err(anyhow::anyhow!("{error:#}")),
                    },
                )
            };
            record_response(source.clone(), operation, response, coverage)
        })
        .collect()
}

fn process_event_signatures(
    raw: &RawLog,
    complete_signatures: Vec<alloy_json_abi::Event>,
    signatures: Vec<String>,
) -> Result<Vec<EventCandidate>, anyhow::Error> {
    let has_complete_signatures = !complete_signatures.is_empty();
    let mut candidates = Vec::new();
    for alloy_event in complete_signatures {
        let event = try_from_alloy_event_to_ethabi_event(alloy_event)?;
        if let Some(decoded) = crate::sources::decode_complete_event(&event, raw) {
            let names: Vec<_> = decoded
                .params
                .iter()
                .map(|param| param.name.clone())
                .collect();
            let values: Vec<_> = decoded
                .params
                .into_iter()
                .map(|param| param.value)
                .collect();
            let args: Vec<_> = event
                .inputs
                .iter()
                .map(|param| param.kind.clone())
                .collect();
            let mut inputs = parse_args_with_names(&names, &args, &values);
            for (input, param) in inputs.iter_mut().zip(&event.inputs) {
                input.indexed = Some(param.indexed);
            }
            candidates.push(EventCandidate {
                abi: Abi {
                    name: event.name,
                    inputs,
                },
                layout: EventLayout::Exact,
            });
        }
    }
    if !has_complete_signatures {
        for signature in signatures {
            let Some((name, args)) = parse_signature(&signature) else {
                continue;
            };
            for (values, indexed) in decode_log(name.to_owned(), &args, raw.clone())? {
                let mut inputs = parse_args("arg".into(), &args, &values);
                for (input, indexed) in inputs.iter_mut().zip(indexed) {
                    input.indexed = Some(indexed);
                }
                candidates.push(EventCandidate {
                    abi: Abi {
                        name: name.into(),
                        inputs,
                    },
                    layout: EventLayout::Candidate,
                });
            }
        }
    }
    let mut seen = HashSet::new();
    candidates.retain(|candidate| seen.insert(candidate.abi.clone()));
    Ok(candidates)
}

fn parse_signature(sig: &str) -> Option<(&str, Vec<ParamType>)> {
    let start = sig.find('(')?;
    let name = &sig[..start];
    let sig = &sig[start..];
    ethabi::param_type::Reader::read(sig)
        .ok()
        .and_then(|param| match param {
            ParamType::Tuple(params) => Some(params),
            _ => None,
        })
        .map(|params| (name, params))
}

fn decode_txinput(args: &[ParamType], tx_args: &[u8]) -> Option<Vec<Token>> {
    let decoded = ethabi::decode(args, tx_args).ok()?;

    // decode will not fail if it decodes only part of the input data
    // so we will encode the result and check, that we decoded the whole data
    let encoded = ethabi::encode(&decoded);
    if tx_args != encoded {
        return None;
    }
    Some(decoded)
}

fn decode_log(
    name: String,
    args: &[ParamType],
    raw: RawLog,
) -> Result<Vec<(Vec<Token>, Vec<bool>)>, anyhow::Error> {
    const MAX_COMBINATIONS: usize = 10000;
    let mut candidates = Vec::new();
    for (index, indexes) in (0..args.len())
        .combinations(raw.topics.len() - 1)
        .enumerate()
    {
        anyhow::ensure!(
            index < MAX_COMBINATIONS,
            "ambiguous event layout exceeds {MAX_COMBINATIONS} combinations; complete ABI required"
        );
        let mut indexed = vec![false; args.len()];
        for index in indexes {
            indexed[index] = true;
        }
        let event = Event {
            name: name.clone(),
            anonymous: false,
            inputs: args
                .iter()
                .zip(&indexed)
                .enumerate()
                .map(|(index, (kind, indexed))| EventParam {
                    name: index.to_string(),
                    kind: kind.clone(),
                    indexed: *indexed,
                })
                .collect(),
        };
        if let Some(log) = crate::sources::decode_complete_event(&event, &raw) {
            candidates.push((
                log.params.into_iter().map(|param| param.value).collect(),
                indexed,
            ));
        }
    }
    Ok(candidates)
}

fn parse_arg(name: String, param: &ParamType, value: &Token) -> Argument {
    let components = match (param, value) {
        (ParamType::Tuple(param), Token::Tuple(value)) => {
            parse_args(format!("{name}_"), param, value)
        }
        _ => Default::default(),
    };
    Argument {
        name,
        r#type: param.to_string(),
        components,
        indexed: None,
        value: value.to_string(),
    }
}

fn parse_args_with_names(names: &[String], args: &[ParamType], values: &[Token]) -> Vec<Argument> {
    let inputs = names
        .iter()
        .zip(args.iter())
        .zip(values.iter())
        .map(|((name, arg), value)| parse_arg(name.clone(), arg, value))
        .collect();
    inputs
}

fn parse_args(pref: String, args: &[ParamType], values: &[Token]) -> Vec<Argument> {
    let names = (0..args.len())
        .map(|index| format!("{pref}{index}"))
        .collect::<Vec<_>>();
    parse_args_with_names(&names, args, values)
}

fn try_from_alloy_event_to_ethabi_event(
    alloy_event: alloy_json_abi::Event,
) -> Result<ethabi::Event, anyhow::Error> {
    serde_json::from_value(
        serde_json::to_value(alloy_event).context("serializing alloy_json_abi::Event")?,
    )
    .context("deserializing ethabi::Event")
}

#[cfg(test)]
mod tests {
    use crate::sources::MockSignatureSource;

    use super::*;
    use ethabi::ethereum_types::{H160, H256, U256};
    use pretty_assertions::assert_eq;

    #[tokio::test]
    async fn function() {
        let tests = vec![
            (
                "70a0823100000000000000000000000000000000219ab540356cbb839cbe05303d7705fa",
                Abi {
                    name: "balanceOf".into(),
                    inputs: vec![Argument {
                        name: "arg0".into(),
                        r#type: "address".into(),
                        components: vec![],
                        indexed: None,
                        value: "00000000219ab540356cbb839cbe05303d7705fa".into(),
                    }],
                },
            ),
            (
                "70a082310000000000000000000000000000000000000000000000000000000000bc61591234567812345678000000000000000000000000000000000000000000000000",
                Abi {
                    name: "branch_passphrase_public".into(),
                    inputs: vec![
                        Argument {
                            name: "arg0".into(),
                            r#type: "uint256".into(),
                            components: vec![],
                            indexed: None,
                            value: "bc6159".into(), // hex number 123456789
                        },
                        Argument {
                            name: "arg1".into(),
                            r#type: "bytes8".into(),
                            components: vec![],
                            indexed: None,
                            value: "1234567812345678".into(),
                        },
                    ],
                },
            ),
            (
                "70a082310000000000000000000000000000000000000000000000000000000000bc615900000000000000000000000000000000219ab540356cbb839cbe05303d7705fa",
                Abi {
                    name: "passphrase_calculate_transfer".into(),
                    inputs: vec![
                        Argument {
                            name: "arg0".into(),
                            r#type: "uint64".into(),
                            components: vec![],
                            indexed: None,
                            value: "bc6159".into(), // hex number 123456789
                        },
                        Argument {
                            name: "arg1".into(),
                            r#type: "address".into(),
                            components: vec![],
                            indexed: None,
                            value: "00000000219ab540356cbb839cbe05303d7705fa".into(),
                        },
                    ],
                },
            ),
        ];

        for (input, abi) in tests {
            let mut source = MockSignatureSource::new();
            source
                .expect_source()
                .return_const("test source".to_owned());
            source.expect_covers_function().return_const(true);
            source.expect_covers_event().return_const(true);
            source
                .expect_get_function_signatures()
                .with(mockall::predicate::eq("70a08231"))
                .times(1)
                .returning(|_| {
                    Ok(vec![
                        "balanceOf(address)".into(),
                        "branch_passphrase_public(uint256,bytes8)".into(),
                        "passphrase_calculate_transfer(uint64,address)".into(),
                    ])
                });
            let source = Arc::new(source);

            let agg = Arc::new(SourceAggregator::new(vec![source.clone()], vec![]));

            let function = agg
                .get_function_abi(&hex::decode(input).unwrap())
                .await
                .unwrap();
            assert_eq!(abi, function[0]);
        }
    }

    fn encode_tx_input_tuple() -> String {
        use ethabi::Token::*;
        let res = ethabi::encode(&[
            Tuple(vec![
                Uint(U256::from_dec_str("123456789").unwrap()),
                Address(H160(
                    hex::decode("00000000219ab540356cbb839cbe05303d7705fa")
                        .unwrap()
                        .try_into()
                        .unwrap(),
                )),
                Bytes(vec![123]),
            ]),
            Tuple(vec![
                Uint(U256::from_dec_str("123").unwrap()),
                FixedArray(vec![
                    FixedBytes(vec![11, 12, 13, 14]),
                    FixedBytes(vec![101, 102, 103, 104]),
                ]),
            ]),
        ]);
        hex::encode(res)
    }

    #[tokio::test]
    async fn function_tuple() {
        let encoded = encode_tx_input_tuple();
        let input = "68705463".to_string() + &encoded;

        let mut source = MockSignatureSource::new();
        source
            .expect_source()
            .return_const("test source".to_owned());
        source.expect_covers_function().return_const(true);
        source.expect_covers_event().return_const(true);
        source
            .expect_get_function_signatures()
            .with(mockall::predicate::eq("68705463"))
            .times(1)
            .returning(|_| {
                Ok(vec![
                    "test((uint256,address,bytes),(uint8,bytes32[2]))".into()
                ])
            });
        let source = Arc::new(source);

        let agg = Arc::new(SourceAggregator::new(vec![source.clone()], vec![]));

        let function = agg
            .get_function_abi(&hex::decode(input).unwrap())
            .await
            .unwrap();

        let expected = Abi {
            name: "test".into(),
            inputs: vec![
                Argument {
                    name: "arg0".into(),
                    r#type: "(uint256,address,bytes)".into(),
                    components: vec![
                        Argument {
                            name: "arg0_0".into(),
                            r#type: "uint256".into(),
                            components: vec![],
                            indexed: None,
                            value: "75bcd15".into(),
                        },
                        Argument {
                            name: "arg0_1".into(),
                            r#type: "address".into(),
                            components: vec![],
                            indexed: None,
                            value: "00000000219ab540356cbb839cbe05303d7705fa".into(),
                        },
                        Argument {
                            name: "arg0_2".into(),
                            r#type: "bytes".into(),
                            components: vec![],
                            indexed: None,
                            value: "7b".into(),
                        },
                    ],
                    indexed: None,
                    value: "(75bcd15,00000000219ab540356cbb839cbe05303d7705fa,7b)".into(),
                },
                Argument {
                    name: "arg1".into(),
                    r#type: "(uint8,bytes32[2])".into(),
                    components: vec![
                        Argument {
                            name: "arg1_0".into(),
                            r#type: "uint8".into(),
                            components: vec![],
                            indexed: None,
                            value: "7b".into(),
                        },
                        Argument {
                            name: "arg1_1".into(),
                            r#type: "bytes32[2]".into(),
                            components: vec![],
                            indexed: None,
                            value: "[0b0c0d0e00000000000000000000000000000000000000000000000000000000,6566676800000000000000000000000000000000000000000000000000000000]".into(),
                        },
                    ],
                    indexed: None,
                    value: "(7b,[0b0c0d0e00000000000000000000000000000000000000000000000000000000,6566676800000000000000000000000000000000000000000000000000000000])".into(),
                },
            ],
        };
        assert_eq!(expected, function[0]);
    }

    #[tokio::test]
    async fn event() {
        let tests = vec![
            (
                RawLog {
                    data: hex::decode(
                        "00000000000000000000000000000000000000000000000000000000006acfc0",
                    )
                    .unwrap(),
                    topics: vec![
                        H256::from_slice(
                            &hex::decode(
                                "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
                            )
                            .unwrap(),
                        ),
                        H256::from_slice(
                            &hex::decode(
                                "000000000000000000000000b8ace4d9bc469ddc8e788e636e817c299a1a8150",
                            )
                            .unwrap(),
                        ),
                        H256::from_slice(
                            &hex::decode(
                                "000000000000000000000000f76c5b19e86c256482f4aad1dae620a0c3ac0cd6",
                            )
                            .unwrap(),
                        ),
                    ],
                },
                "Transfer(address,address,uint256)",
                Abi {
                    name: "Transfer".into(),
                    inputs: vec![
                        Argument {
                            name: "arg0".into(),
                            r#type: "address".into(),
                            components: vec![],
                            indexed: Some(true),
                            value: "b8ace4d9bc469ddc8e788e636e817c299a1a8150".into(),
                        },
                        Argument {
                            name: "arg1".into(),
                            r#type: "address".into(),
                            components: vec![],
                            indexed: Some(true),
                            value: "f76c5b19e86c256482f4aad1dae620a0c3ac0cd6".into(),
                        },
                        Argument {
                            name: "arg2".into(),
                            r#type: "uint256".into(),
                            components: vec![],
                            indexed: Some(false),
                            value: "6acfc0".into(),
                        },
                    ],
                },
            ),
            (
                RawLog {
                    data: hex::decode(
                        "0000000000000000000000000000000000000000000000083ed9ef578babdb5c00000000000000000000000000000000000000000000bf05c05e3ce0f57a7e39",
                    )
                    .unwrap(),
                    topics: vec![
                        H256::from_slice(
                            &hex::decode(
                                "1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1",
                            )
                            .unwrap(),
                        ),
                    ],
                },
                "Sync(uint112,uint112)",
                Abi {
                    name: "Sync".into(),
                    inputs: vec![
                        Argument {
                            name: "arg0".into(),
                            r#type: "uint112".into(),
                            components: vec![],
                            indexed: Some(false),
                            value: "83ed9ef578babdb5c".into(),
                        },
                        Argument {
                            name: "arg1".into(),
                            r#type: "uint112".into(),
                            components: vec![],
                            indexed: Some(false),
                            value: "bf05c05e3ce0f57a7e39".into(),
                        },
                    ],
                },
            ),

        ];
        for (input, sig, abi) in tests {
            let expected = hex::encode(input.topics[0].as_bytes());
            let mut source = MockSignatureSource::new();
            source
                .expect_source()
                .return_const("test source".to_owned());
            source.expect_covers_function().return_const(true);
            source.expect_covers_event().return_const(true);
            source
                .expect_get_event_signatures()
                .withf(move |hex| hex == expected)
                .times(1)
                .returning(|_| Ok(vec![sig.into()]));
            let source = Arc::new(source);

            let agg = Arc::new(SourceAggregator::new(vec![source.clone()], vec![]));

            let event = agg.get_event_abi(input).await.unwrap();
            assert_eq!(abi, event[0]);
        }
    }

    #[tokio::test]
    async fn event_dynamic() {
        let input = RawLog {
            data: hex::decode(
                "000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000097465737431323334350000000000000000000000000000000000000000000000",
            )
            .unwrap(),
            topics: vec![
                H256::from_slice(
                    &hex::decode(
                        "74cb234c0dd0ccac09c19041a69978ccb865f1f44a2877a009549898f6395b10",
                    )
                    .unwrap(),
                ),
                H256::from_slice(
                    &hex::decode(
                        "2c3138faa13a5122f618ced1b0b4be95398183af357f6769e61ee2bccacc8b54",
                    )
                    .unwrap(),
                ),
            ],
        };
        let sig = "Test(string,string)";
        let abi = Abi {
            name: "Test".into(),
            inputs: vec![
                Argument {
                    name: "arg0".into(),
                    r#type: "string".into(),
                    components: vec![],
                    indexed: Some(true),
                    value: "2c3138faa13a5122f618ced1b0b4be95398183af357f6769e61ee2bccacc8b54"
                        .into(),
                },
                Argument {
                    name: "arg1".into(),
                    r#type: "string".into(),
                    components: vec![],
                    indexed: Some(false),
                    value: "test12345".into(),
                },
            ],
        };
        let expected = hex::encode(input.topics[0].as_bytes());
        let mut source = MockSignatureSource::new();
        source
            .expect_source()
            .return_const("test source".to_owned());
        source.expect_covers_function().return_const(true);
        source.expect_covers_event().return_const(true);
        source
            .expect_get_event_signatures()
            .withf(move |hex| hex == expected)
            .times(1)
            .returning(|_| Ok(vec![sig.into()]));
        let source = Arc::new(source);

        let agg = Arc::new(SourceAggregator::new(vec![source.clone()], vec![]));

        let event = agg.get_event_abi(input).await.unwrap();
        assert_eq!(abi, event[0]);
    }

    #[tokio::test]
    async fn event_tuple() {
        let input = RawLog {
            data: hex::decode("000000000000000000000000b8ace4d9bc469ddc8e788e636e817c299a1a8150000000000000000000000000f76c5b19e86c256482f4aad1dae620a0c3ac0cd6")
                .unwrap(),
            topics: vec![
                H256::from_slice(
                    &hex::decode(
                        "5db533d27f83c494aa583a6f8222343e612dd3efd69499ca6ae5dda6c6097df0",
                    )
                    .unwrap(),
                ),
                H256::from_slice(
                    &hex::decode(
                        "000000000000000000000000b8ace4d9bc469ddc8e788e636e817c299a1a8150",
                    )
                    .unwrap(),
                ),
            ],
        };
        let sig = "Test(address,(address,address))";
        let abi = Abi {
            name: "Test".into(),
            inputs: vec![
                Argument {
                    name: "arg0".into(),
                    r#type: "address".into(),
                    components: vec![],
                    indexed: Some(
                        true,
                    ),
                    value: "b8ace4d9bc469ddc8e788e636e817c299a1a8150".into(),
                },
                Argument {
                    name: "arg1".into(),
                    r#type: "(address,address)".into(),
                    components: vec![
                        Argument {
                            name: "arg1_0".into(),
                            r#type: "address".into(),
                            components: vec![],
                            indexed: None,
                            value: "b8ace4d9bc469ddc8e788e636e817c299a1a8150".into(),
                        },
                        Argument {
                            name: "arg1_1".into(),
                            r#type: "address".into(),
                            components: vec![],
                            indexed: None,
                            value: "f76c5b19e86c256482f4aad1dae620a0c3ac0cd6".into(),
                        },
                    ],
                    indexed: Some(
                        false,
                    ),
                    value: "(b8ace4d9bc469ddc8e788e636e817c299a1a8150,f76c5b19e86c256482f4aad1dae620a0c3ac0cd6)".into(),
                },
            ],
        };
        let expected = hex::encode(input.topics[0].as_bytes());
        let mut source = MockSignatureSource::new();
        source
            .expect_source()
            .return_const("test source".to_owned());
        source.expect_covers_function().return_const(true);
        source.expect_covers_event().return_const(true);
        source
            .expect_get_event_signatures()
            .withf(move |hex| hex == expected)
            .times(1)
            .returning(|_| Ok(vec![sig.into()]));
        let source = Arc::new(source);

        let agg = Arc::new(SourceAggregator::new(vec![source.clone()], vec![]));

        let event = agg.get_event_abi(input).await.unwrap();
        assert_eq!(abi, event[0]);
    }
}

#[cfg(test)]
#[path = "aggregator_precompiles_test.rs"]
mod aggregator_precompiles_test;
