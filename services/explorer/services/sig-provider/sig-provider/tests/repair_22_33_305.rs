#[path = "../src/aggregator_precompiles_test.rs"]
mod producer_path;

use ethabi::{Contract, Event, ParamType, RawLog, Token};
use producer_path::{encoded_event, event_request, precompile_aggregator, start_service, Peer};
use serde_json::{json, Value};
use sig_provider::{
    eth_bytecode_db, fourbyte, precompiles::PrecompileSource, sigeth, CompleteSignatureSource,
    EventLayout, ImportRefused, LookupUnavailable, SignatureSource, SourceAggregator, SourceStatus,
};
use sig_provider_proto::{
    actix_web,
    blockscout::sig_provider::v1::{
        abi_service_server::AbiService, signature_service_server::SignatureService, Abi,
        CreateSignaturesRequest, GetFunctionAbiRequest,
    },
    tonic,
};
use std::sync::Arc;

fn unlisted_function() -> (String, Vec<u8>) {
    let selector = ethabi::short_signature("repairUnlistedFunction", &[ParamType::Uint(256)]);
    let mut calldata = selector.to_vec();
    calldata.extend(ethabi::encode(&[Token::Uint(17.into())]));
    (hex::encode(selector), calldata)
}

fn remote_event() -> (Event, Vec<Token>, RawLog) {
    let contract = Contract::load(br#"[{"type":"event","name":"RepairRemoteEvent","anonymous":false,"inputs":[{"name":"value","type":"uint256","indexed":false}]}]"#.as_slice()).unwrap();
    let event = contract.event("RepairRemoteEvent").unwrap().clone();
    let values = vec![Token::Uint(19.into())];
    let raw = encoded_event(&event, &values);
    (event, values, raw)
}

fn unavailable(error: &anyhow::Error, expected_failed: usize) {
    let unavailable = error
        .downcast_ref::<LookupUnavailable>()
        .expect("typed unavailability must survive aggregation");
    assert_eq!(
        unavailable
            .coverage
            .iter()
            .filter(|outcome| matches!(outcome.status, SourceStatus::Failed(_)))
            .count(),
        expected_failed
    );
    assert!(unavailable
        .coverage
        .iter()
        .all(|outcome| !matches!(outcome.status, SourceStatus::Succeeded)));
    assert!(unavailable
        .coverage
        .iter()
        .any(|outcome| outcome.status == SourceStatus::OutsideCoverage));
    assert!(error.to_string().contains("signature lookup unavailable"));
}

#[test]
fn all_capable_failures_remain_unavailable_through_aggregation_and_service() {
    actix_web::rt::System::new().block_on(async {
        let refused = Peer::reply(400, json!({"error":"recorded directory refusal"})).await;
        let local = Arc::new(PrecompileSource::new());
        let (selector, calldata) = unlisted_function();
        assert!(
            SignatureSource::get_function_signatures(local.as_ref(), &selector)
                .await
                .unwrap()
                .is_empty()
        );
        let aggregator = Arc::new(SourceAggregator::new(
            vec![
                local.clone(),
                Arc::new(sigeth::Source::new(refused.endpoint.parse().unwrap())),
                Arc::new(fourbyte::Source::new(refused.endpoint.parse().unwrap())),
            ],
            vec![
                local,
                Arc::new(eth_bytecode_db::Source::new(
                    refused.endpoint.parse().unwrap(),
                )),
            ],
        ));
        unavailable(
            &aggregator
                .get_function_signatures(&selector)
                .await
                .unwrap_err(),
            2,
        );
        unavailable(
            &aggregator.get_function_abi(&calldata).await.unwrap_err(),
            2,
        );
        let (_, _, raw) = remote_event();
        unavailable(&aggregator.get_event_abi(raw.clone()).await.unwrap_err(), 3);
        unavailable(
            &aggregator.batch_get_event_abi(vec![raw]).await.unwrap_err(),
            3,
        );
        let (http, service) = start_service(aggregator, vec![]).await;
        let request = GetFunctionAbiRequest {
            tx_input: format!("0x{}", hex::encode(&calldata)),
        };
        let response = reqwest::Client::new()
            .get(format!("{}/api/v1/abi/function", http.endpoint))
            .query(&request)
            .send()
            .await
            .unwrap();
        // The out-of-scope service currently maps every aggregator error to
        // Internal/500. It must never turn unavailability into successful [].
        assert_eq!(
            response.status(),
            reqwest::StatusCode::INTERNAL_SERVER_ERROR
        );
        assert!(response
            .text()
            .await
            .unwrap()
            .contains("signature lookup unavailable"));
        let status = service
            .get_function_abi(tonic::Request::new(request))
            .await
            .unwrap_err();
        assert_eq!(status.code(), tonic::Code::Internal);
        assert!(status.message().contains("signature lookup unavailable"));
        let requests = refused.requests.lock().unwrap().clone();
        assert!(requests
            .iter()
            .any(|(path, _)| path.starts_with("GET /api/v1/signatures?function=")));
        assert!(requests
            .iter()
            .any(|(path, _)| path.starts_with("GET /api/v1/event-signatures/")));
        assert!(requests
            .iter()
            .any(|(path, _)| path == "POST /api/v2/event-descriptions:batch-search"));
        http.server.stop(true).await;
        refused.server.stop(true).await;
    });
}

#[test]
fn successful_misses_and_partial_results_preserve_source_coverage() {
    actix_web::rt::System::new().block_on(async {
        let failed = Peer::reply(400, json!({"error":"recorded refusal"})).await;
        let healthy = Peer::reply(200, json!({"next":null,"results":[]})).await;
        let aggregator = precompile_aggregator(vec![
            Arc::new(sigeth::Source::new(failed.endpoint.parse().unwrap())),
            Arc::new(fourbyte::Source::new(healthy.endpoint.parse().unwrap())),
        ]);
        let (selector, _) = unlisted_function();
        let report = aggregator
            .get_function_signatures_with_coverage(&selector)
            .await
            .unwrap();
        assert!(
            report.value.is_empty(),
            "healthy directory confirms a genuine miss"
        );
        assert_eq!(report.coverage.len(), 3);
        assert_eq!(report.coverage[0].status, SourceStatus::OutsideCoverage);
        assert!(matches!(report.coverage[1].status, SourceStatus::Failed(_)));
        assert_eq!(report.coverage[2].status, SourceStatus::Succeeded);
        assert_eq!(report.coverage[1].source, format!("{}/", failed.endpoint));
        let (_, _, raw) = remote_event();
        let event = aggregator
            .get_event_abi_with_coverage(raw.clone())
            .await
            .unwrap();
        assert!(event.value.is_empty());
        let batch = aggregator
            .batch_get_event_abi_with_coverage(vec![raw.clone(), raw])
            .await
            .unwrap();
        assert_eq!(batch.len(), 2);
        for row in batch {
            assert!(row.value.is_empty());
            assert_eq!(row.coverage, event.coverage);
        }
        let local_function = producer_path::producer("precompiles/spot/abi/stoporders.json");
        let function = local_function.function("executeStopLossOrder").unwrap();
        let calldata = function.encode_input(&[Token::Uint(23.into())]).unwrap();
        let report = aggregator
            .get_function_abi_with_coverage(&calldata)
            .await
            .unwrap();
        assert_eq!(report.value.len(), 1);
        assert_eq!(report.value[0].name, function.name);
        assert_eq!(report.coverage[0].status, SourceStatus::Succeeded);
        assert!(matches!(report.coverage[1].status, SourceStatus::Failed(_)));
        let (http, _) = start_service(aggregator, vec![]).await;
        let decoded: Vec<Abi> = reqwest::Client::new()
            .get(format!("{}/api/v1/abi/function", http.endpoint))
            .query(&GetFunctionAbiRequest {
                tx_input: hex::encode(calldata),
            })
            .send()
            .await
            .unwrap()
            .error_for_status()
            .unwrap()
            .json()
            .await
            .unwrap();
        assert_eq!(decoded, report.value);
        http.server.stop(true).await;
        failed.server.stop(true).await;
        healthy.server.stop(true).await;
    });
}

#[test]
fn batch_fallback_reads_the_real_directory_and_rejects_malformed_complete_rows() {
    actix_web::rt::System::new().block_on(async {
        let (event, values, raw) = remote_event();
        let signature = "RepairRemoteEvent(uint256)";
        let healthy = Peer::reply(200, json!({"next":null,"results":[{"text_signature":signature}]})).await;
        let malformed = Peer::reply(200, json!({"responses":[]})).await;
        let local = Arc::new(PrecompileSource::new());
        let aggregator = SourceAggregator::new(
            vec![local.clone(), Arc::new(fourbyte::Source::new(healthy.endpoint.parse().unwrap()))],
            vec![local, Arc::new(eth_bytecode_db::Source::new(malformed.endpoint.parse().unwrap()))],
        );
        let rows = aggregator.batch_get_event_abi_with_coverage(vec![raw.clone(), raw.clone()]).await.unwrap();
        assert_eq!(rows.len(), 2);
        for row in rows {
            assert_eq!(row.value.len(), 1);
            assert_eq!(row.value[0].layout, EventLayout::Candidate);
            assert_eq!(row.value[0].abi.name, event.name);
            assert_eq!(row.value[0].abi.inputs[0].value, values[0].to_string());
            assert_eq!(row.value[0].abi.inputs[0].indexed, Some(false));
            assert!(row.coverage.iter().any(|outcome| matches!(&outcome.status, SourceStatus::Failed(error) if error.contains("invalid batch response length"))));
        }
        assert_eq!(healthy.requests.lock().unwrap().len(), 2, "default batching must execute actual directory lookups");
        let local = Arc::new(PrecompileSource::new());
        let complete_only = SourceAggregator::new(vec![], vec![local, Arc::new(eth_bytecode_db::Source::new(malformed.endpoint.parse().unwrap()))]);
        unavailable(&complete_only.batch_get_event_abi(vec![raw]).await.unwrap_err(), 1);
        healthy.server.stop(true).await;
        malformed.server.stop(true).await;
    });
}

#[test]
fn import_refusal_is_not_hidden_by_local_or_successful_sources() {
    actix_web::rt::System::new().block_on(async {
        let abi = r#"[{"type":"function","name":"repairImport","inputs":[],"outputs":[],"stateMutability":"nonpayable"}]"#;
        let accepted = Peer::reply(200, json!({"num_processed":1,"num_imported":1,"num_duplicates":0,"num_ignored":0})).await;
        let refused = Peer::reply(200, json!({"ok":false})).await;
        let accepting: Arc<dyn SignatureSource + Send + Sync> = Arc::new(fourbyte::Source::new(accepted.endpoint.parse().unwrap()));
        let rejecting: Arc<dyn SignatureSource + Send + Sync> = Arc::new(sigeth::Source::new(refused.endpoint.parse().unwrap()));
        let aggregator = precompile_aggregator(vec![accepting.clone(), rejecting.clone()]);
        let error = aggregator.create_signatures(abi).await.unwrap_err();
        let refusal = error.downcast_ref::<ImportRefused>().unwrap();
        assert_eq!(refusal.coverage.len(), 3);
        assert_eq!(refusal.coverage[0].status, SourceStatus::OutsideCoverage);
        assert_eq!(refusal.coverage[1].status, SourceStatus::Succeeded);
        assert!(matches!(&refusal.coverage[2].status, SourceStatus::Failed(error) if error.contains("rejected")));
        let successful = precompile_aggregator(vec![accepting]);
        let report = successful.create_signatures_with_coverage(abi).await.unwrap();
        assert_eq!(report.coverage[0].status, SourceStatus::OutsideCoverage);
        assert_eq!(report.coverage[1].status, SourceStatus::Succeeded);
        let local_only = precompile_aggregator(vec![]);
        assert!(local_only.create_signatures(abi).await.unwrap_err().is::<ImportRefused>());
        let (http, service) = start_service(aggregator, vec![rejecting]).await;
        let request = CreateSignaturesRequest { abi: abi.to_owned() };
        let response = reqwest::Client::new().post(format!("{}/api/v1/signatures", http.endpoint)).json(&request).send().await.unwrap();
        assert_eq!(response.status(), reqwest::StatusCode::SERVICE_UNAVAILABLE);
        assert!(response.text().await.unwrap().contains("signature import failed"));
        let status = service.create_signatures(tonic::Request::new(request)).await.unwrap_err();
        assert_eq!(status.code(), tonic::Code::Unavailable);
        assert!(status.message().contains("rejected"));
        for peer in [&accepted, &refused] {
            let records = peer.requests.lock().unwrap().clone();
            assert!(!records.is_empty());
            for (path, body) in records {
                assert!(path.starts_with("POST /api/v1/import"));
                let body: Value = serde_json::from_slice(&body).unwrap();
                if body.get("data").is_some() { assert_eq!(body["data"][0], serde_json::from_str::<Value>(abi).unwrap()); }
                else { assert_eq!(body["contract_abi"], abi); }
            }
        }
        http.server.stop(true).await;
        accepted.server.stop(true).await;
        refused.server.stop(true).await;
    });
}

#[test]
fn embedded_event_metadata_is_exact_in_single_and_mixed_batch_results() {
    actix_web::rt::System::new().block_on(async {
        let producer = producer_path::producer("precompiles/layerxexchange/abi.json");
        let event = producer.event("OrderCancelRequested").unwrap();
        let values = vec![
            Token::FixedBytes(vec![0x11; 32]),
            Token::FixedBytes(vec![0x22; 32]),
            Token::Address([0x33; 20].into()),
            Token::Uint(0x44.into()),
        ];
        let raw = encoded_event(event, &values);
        let local = PrecompileSource::new();
        let descriptions =
            CompleteSignatureSource::get_event_signatures(&local, &hex::encode(event.signature()))
                .await
                .unwrap();
        assert!(descriptions.iter().any(|description| {
            serde_json::from_value::<Event>(serde_json::to_value(description).unwrap()).unwrap()
                == *event
        }));
        let failed = Peer::reply(400, json!({"error":"recorded refusal"})).await;
        let aggregator = precompile_aggregator(vec![Arc::new(sigeth::Source::new(
            failed.endpoint.parse().unwrap(),
        ))]);
        let report = aggregator
            .get_event_abi_with_coverage(raw.clone())
            .await
            .unwrap();
        assert!(report
            .value
            .iter()
            .all(|candidate| candidate.layout == EventLayout::Exact));
        producer_path::assert_producer_event(
            &report
                .value
                .iter()
                .map(|candidate| candidate.abi.clone())
                .collect::<Vec<_>>(),
            event,
            &values,
        );
        assert!(report
            .coverage
            .iter()
            .any(|outcome| matches!(outcome.status, SourceStatus::Failed(_))));
        let rows = aggregator
            .batch_get_event_abi_with_coverage(vec![raw.clone(), raw.clone()])
            .await
            .unwrap();
        assert_eq!(rows.len(), 2);
        for row in rows {
            assert_eq!(row.value.len(), 1);
            assert_eq!(row.value[0].layout, EventLayout::Exact);
            producer_path::assert_producer_event(&[row.value[0].abi.clone()], event, &values);
        }
        let (_, _, unknown) = remote_event();
        unavailable(
            &aggregator
                .batch_get_event_abi(vec![raw.clone(), unknown])
                .await
                .unwrap_err(),
            1,
        );
        // A known ABI that rejects the wire encoding must not fall back to
        // signature permutations, even while a directory is unavailable.
        let mut malformed = raw;
        malformed.data.extend([0; 32]);
        assert!(aggregator
            .get_event_abi(malformed)
            .await
            .unwrap()
            .is_empty());
        failed.server.stop(true).await;
    });
}

#[test]
fn unknown_indexed_layouts_are_explicit_candidates_including_the_producer_order() {
    actix_web::rt::System::new().block_on(async {
        // Recorded Uniswap V2 producer ABI: the final address is indexed.
        let abi: Value = serde_json::from_str(r#"[{"type":"event","name":"Swap","anonymous":false,"inputs":[{"name":"sender","type":"address","indexed":true},{"name":"amount0In","type":"uint256","indexed":false},{"name":"amount1In","type":"uint256","indexed":false},{"name":"amount0Out","type":"uint256","indexed":false},{"name":"amount1Out","type":"uint256","indexed":false},{"name":"to","type":"address","indexed":true}]}]"#).unwrap();
        let contract = Contract::load(serde_json::to_vec(&abi).unwrap().as_slice()).unwrap();
        let event = contract.event("Swap").unwrap();
        let values = vec![Token::Address([0x11;20].into()), Token::Uint(2.into()), Token::Uint(3.into()), Token::Uint(4.into()), Token::Uint(5.into()), Token::Address([0x66;20].into())];
        let raw = encoded_event(event, &values);
        let signature = "Swap(address,uint256,uint256,uint256,uint256,address)";
        let directory = Peer::reply(200, json!({"next":null,"results":[{"text_signature":signature}]})).await;
        let aggregator = precompile_aggregator(vec![Arc::new(fourbyte::Source::new(directory.endpoint.parse().unwrap()))]);
        let report = aggregator.get_event_abi_with_coverage(raw.clone()).await.unwrap();
        assert!(report.value.len() > 1, "signature-only layout is ambiguous");
        assert!(report.value.iter().all(|candidate| candidate.layout == EventLayout::Candidate));
        let correct = report.value.iter().find(|candidate| candidate.abi.inputs.iter().map(|input| input.indexed.unwrap()).collect::<Vec<_>>() == vec![true,false,false,false,false,true]).unwrap();
        assert_eq!(correct.abi.inputs.iter().map(|input| input.value.clone()).collect::<Vec<_>>(), values.iter().map(ToString::to_string).collect::<Vec<_>>());
        let (http, _) = start_service(aggregator, vec![]).await;
        let decoded: Vec<Abi> = reqwest::Client::new().get(format!("{}/api/v1/abi/event", http.endpoint)).query(&event_request(&raw))
            .send().await.unwrap().error_for_status().unwrap().json().await.unwrap();
        assert_eq!(decoded, report.value.into_iter().map(|candidate| candidate.abi).collect::<Vec<_>>());
        assert!(decoded.len() > 1, "HTTP must expose ambiguity rather than one knowingly misordered layout");
        // Drive the same raw log through the real complete-ABI adapter.
        let complete = Peer::reply(200, json!({"eventDescriptions":[{"name":event.name,"inputs":abi[0]["inputs"].to_string()}]})).await;
        let exact = SourceAggregator::new(vec![Arc::new(fourbyte::Source::new(directory.endpoint.parse().unwrap()))], vec![Arc::new(eth_bytecode_db::Source::new(complete.endpoint.parse().unwrap()))]);
        let report = exact.get_event_abi_with_coverage(raw).await.unwrap();
        assert_eq!(report.value.len(), 1);
        assert_eq!(report.value[0].layout, EventLayout::Exact);
        producer_path::assert_producer_event(&[report.value[0].abi.clone()], event, &values);
        http.server.stop(true).await;
        directory.server.stop(true).await;
        complete.server.stop(true).await;
    });
}
