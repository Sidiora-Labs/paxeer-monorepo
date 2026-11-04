//! Task 22.31.273 / audit 22.30.146 repair record.
//!
//! Finding -> proving test (hyphens in finding IDs become underscores):
//! transaction-nullability -> transaction_nullability
//! smart-contract-abi -> smart_contract_abi
//! smart-contract-field-drift -> smart_contract_field_drift
//! transfer-total-discrimination -> transfer_total_discrimination
//! transfer-null-total -> transfer_null_total
//! trace-address -> trace_address
//! trace-call-type -> trace_call_type
//! trace-call-gas -> trace_call_gas
//! trace-create-gas -> trace_create_gas
//! trace-refund-address -> trace_refund_address
//! internal-creation-destination -> internal_creation_destination
//! log-wire-shape -> log_wire_shape
//! log-structured-parameter -> log_structured_parameter
//! state-change-address -> state_change_address
//! holder-address -> holder_address
//! contract-list-address -> contract_list_address
//! search-miss-nullability -> search_miss_nullability
//! search-native-result -> search_native_result
//! search-address-nullability -> search_address_nullability
//! search-token-supply -> search_token_supply
//! transaction-feed-native-items -> transaction_feed_native_items
//! address-list-market-nullability -> address_list_market_nullability
//! market-supply-integer -> market_supply_integer
//! health-blocks-envelope -> health_blocks_envelope
//! summary-variable-map -> summary_variable_map
//! health-node-height -> health_node_height
//!
//! These tests exercise the real crate's JSON codec and typed response parsers.
//! Existing HTTP recordings are used unchanged; branch cases also exercise the
//! nulls and structures documented by the producer functions named below.
//! Full Elixir/HTTP producer execution is not claimed by these codec tests.
//! Baseline verification was not run: the task's ordered gate instructions require
//! implementation first and forbid invoking verify_cmd before hf-done.
//! Gate revision/command/exit/private log are supplied by the runner's hf-done ledger.
//! The carried attempt passed 25 tests and failed internal_creation_destination:
//! AddressParam dropped the recorded implementation and scam fields on serialization.
//!
//! Wiring request (explorer client domain): update services/explorer/services/libs/blockscout-client/swaggers/blockscout-api-v2.yaml with the 26 corrected wire contracts before regeneration.
//! Wiring request (explorer client domain): align AddressParam and proxy implementation properties in services/explorer/services/libs/blockscout-client/swaggers/blockscout-api-v2.yaml with Helper.address_with_info and the committed recordings before regeneration.
//! Wiring request (explorer client domain): update services/explorer/services/libs/blockscout-client/swaggers/overrides/002_health.yaml and blockscout-api-final.yaml with blocks/latest_block.node before regeneration.
//! Search inventory: SearchView also emits user_operation and blob (covered here),
//! and tac_operation (an independent chain-specific interface outside this repair).

use blockscout_client::{apis::ResponseContent, models::*};
use rust_decimal::Decimal;
use serde::{de::DeserializeOwned, Serialize};
use serde_json::{json, Value};

const RECORDINGS: &str = concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/tests/recorded/eth_blockscout_com/"
);
const TX: &str = "_api_v2_transactions_0x4dd7e3f4.json";
const TRANSFERS: &str = "_api_v2_token-transfers.json";
const INTERNAL: &str = "_api_v2_transactions_0x4dd7e3f4_internal-transactions.json";
const LOGS: &str = "_api_v2_addresses_0x8FD4596d4E7788a71F82dAf4119D069a84E7d3f3_logs.json";
const CONTRACT: &str = "_api_v2_smart-contracts_0x8FD4596d4E7788a71F82dAf4119D069a84E7d3f3.json";

fn recording(name: &str) -> Value {
    let envelope: Value =
        serde_json::from_str(&std::fs::read_to_string(format!("{RECORDINGS}{name}")).unwrap())
            .unwrap();
    serde_json::from_str(envelope["response"]["body"].as_str().unwrap()).unwrap()
}

fn decode<T: DeserializeOwned>(wire: &Value) -> T {
    serde_json_path_to_error::from_str(&serde_json::to_string(wire).unwrap()).unwrap()
}

fn encoded<T: Serialize>(model: &T) -> Value {
    serde_json::to_value(model).unwrap()
}

fn response<T: DeserializeOwned>(wire: &Value) -> ResponseContent<T> {
    let content = serde_json::to_string(wire).unwrap();
    let entity = serde_json::from_str(&content).unwrap();
    ResponseContent {
        status: reqwest::StatusCode::OK,
        content,
        entity: Some(entity),
    }
}

#[test]
fn transaction_nullability() {
    use blockscout_client::apis::transactions_api::{GetTxSuccess, GetTxsSuccess};
    let mined = recording(TX);
    let mut pending = mined.clone();
    for key in [
        "timestamp",
        "block_number",
        "status",
        "gas_used",
        "position",
        "base_fee_per_gas",
        "exchange_rate",
    ] {
        pending[key] = Value::Null;
    }
    pending["result"] = json!("pending");
    let mut creation = mined.clone();
    creation["to"] = Value::Null;
    creation["created_contract"] = mined["to"].clone();
    let mut historical = mined.clone();
    historical["base_fee_per_gas"] = Value::Null;
    historical["exchange_rate"] = Value::Null;
    for wire in [&mined, &pending, &creation, &historical] {
        let transport = response::<GetTxSuccess>(wire);
        assert!(transport.try_as_success().is_some());
        let tx = transport.try_parse_as_success_model().unwrap();
        let round_trip = encoded(&tx);
        for field in [
            "timestamp",
            "block_number",
            "status",
            "to",
            "gas_used",
            "base_fee_per_gas",
            "exchange_rate",
            "position",
            "created_contract",
        ] {
            if matches!(field, "block_number" | "position") {
                assert_eq!(
                    decode::<Option<Decimal>>(&round_trip[field]),
                    decode::<Option<Decimal>>(&wire[field])
                );
            } else if matches!(field, "to" | "created_contract") {
                assert_eq!(round_trip[field].is_null(), wire[field].is_null());
                assert_eq!(round_trip[field]["hash"], wire[field]["hash"]);
            } else {
                assert_eq!(round_trip[field], wire[field], "lost transaction {field}");
            }
        }
        assert_eq!(round_trip["hash"], mined["hash"]);
        let page = json!({"items": [wire], "next_page_params": null});
        assert!(response::<GetTxsSuccess>(&page).try_as_success().is_some());
        assert_eq!(
            encoded(&decode::<GetBlockTxs200Response>(&page))["items"][0]["hash"],
            mined["hash"]
        );
        let address_page =
            response::<blockscout_client::apis::addresses_api::GetAddressTxsSuccess>(&page);
        assert!(address_page.try_as_success().is_some());
        assert_eq!(
            address_page.try_parse_as_success_model().unwrap().items[0].hash,
            mined["hash"]
        );
    }
    assert!(decode::<Transaction>(&pending).block_number.is_none());
    assert!(decode::<Transaction>(&creation).to.is_none());
    assert!(decode::<Transaction>(&creation).created_contract.is_some());
}

#[test]
fn smart_contract_abi() {
    use blockscout_client::apis::smart_contracts_api::GetSmartContractSuccess;
    let wire = recording(CONTRACT);
    assert!(wire["abi"].as_array().unwrap().len() > 1);
    let response = response::<GetSmartContractSuccess>(&wire);
    let contract = response.try_parse_as_success_model().unwrap();
    assert!(response.try_as_success().is_some());
    assert_eq!(encoded(&contract)["abi"], wire["abi"]);
    assert_eq!(
        encoded(&contract)["compiler_version"],
        wire["compiler_version"]
    );
    let unverified: SmartContract = decode(
        &json!({"abi": null, "proxy_type": null, "implementations": [], "conflicting_implementations": null}),
    );
    assert!(unverified.abi.is_none());
    assert!(serde_json::from_value::<SmartContract>(json!({"abi": "[]"})).is_err());
}

#[test]
fn smart_contract_field_drift() {
    // SmartContractView.prepare_smart_contract + Proxy.conflicting_implementations_info.
    let mut wire = recording(CONTRACT);
    wire["optimization_runs"] = json!(200);
    wire["proxy_type"] = json!("eip1967");
    wire["implementations"] = json!([
        {"address_hash": "0x1111111111111111111111111111111111111111", "name": "Implementation"},
        {"address_hash": "0x2222222222222222222222222222222222222222", "name": null}
    ]);
    wire["conflicting_implementations"] = json!([
        {"proxy_type": "eip1967", "implementations": wire["implementations"]},
        {"proxy_type": "eip1167", "implementations": [{"address_hash": "0x3333333333333333333333333333333333333333"}]}
    ]);
    let contract: SmartContract = decode(&wire);
    assert_eq!(contract.optimization_runs, Some(Decimal::from(200)));
    assert_eq!(contract.implementations.as_ref().unwrap().len(), 2);
    let out = encoded(&contract);
    for key in ["proxy_type", "implementations"] {
        assert_eq!(out[key], wire[key]);
    }
    assert_eq!(
        decode::<Decimal>(&out["optimization_runs"]),
        Decimal::from(200)
    );
    let conflicts = contract.conflicting_implementations.unwrap();
    assert_eq!(conflicts.len(), 2);
    assert_eq!(
        conflicts[1].implementations[0].address_hash,
        wire["conflicting_implementations"][1]["implementations"][0]["address_hash"]
    );
    assert_eq!(conflicts[1].proxy_type, "eip1167");
}

#[test]
fn transfer_total_discrimination() {
    // TokenTransferView.prepare_token_transfer_total's ERC20/721/1155/404 branches.
    let source = recording(TRANSFERS)["items"][0].clone();
    let instance = json!({"id": "42", "metadata": {"name": "Instance 42", "attributes": [{"trait_type": "level", "value": 7}]}});
    let totals = [
        ("ERC-20", json!({"value": "0", "decimals": "18"})),
        (
            "ERC-721",
            json!({"token_id": "42", "token_instance": instance}),
        ),
        (
            "ERC-1155",
            json!({"token_id": "42", "token_instance": instance, "value": "3", "decimals": null}),
        ),
        (
            "ERC-404",
            json!({"token_id": "42", "token_instance": instance, "value": "1000000000000000000", "decimals": "18"}),
        ),
        (
            "ERC-404",
            json!({"value": "2000000000000000000", "decimals": "18"}),
        ),
    ];
    for (standard, total) in totals {
        let mut wire = source.clone();
        wire["token"]["type"] = json!(standard);
        wire["token_type"] = json!(standard);
        wire["total"] = total.clone();
        let transfer: TokenTransfer = decode(&wire);
        assert_eq!(transfer.token.r#type.as_deref(), Some(standard));
        assert_eq!(transfer.token_type.as_deref(), Some(standard));
        match (&transfer.total, standard, total.get("token_id")) {
            (Some(TokenTransferTotal::TotalErc20(_)), "ERC-20" | "ERC-404", None) => {}
            (Some(TokenTransferTotal::TotalErc721(nft)), "ERC-721", Some(_)) => {
                assert_eq!(nft.token_id.as_deref(), Some("42"))
            }
            (Some(TokenTransferTotal::TotalErc1155(nft)), "ERC-1155" | "ERC-404", Some(_)) => {
                assert_eq!(nft.token_id.as_deref(), Some("42"))
            }
            other => panic!("incorrect total discrimination: {other:?}"),
        }
        let out = encoded(&transfer);
        for key in total
            .as_object()
            .unwrap()
            .keys()
            .filter(|key| key.as_str() != "token_instance")
        {
            assert_eq!(out["total"][key], total[key]);
        }
        if total.get("token_instance").is_some() {
            assert_eq!(
                out["total"]["token_instance"]["metadata"],
                total["token_instance"]["metadata"]
            );
            assert_eq!(out["total"]["token_instance"]["id"], "42");
        }
    }
    for invalid in [
        json!({}),
        json!({"token_instance": instance}),
        json!({"token_id": "42", "unexpected": 1}),
        json!({"value": "3"}),
    ] {
        assert!(serde_json::from_value::<TokenTransferTotal>(invalid).is_err());
    }
}

#[test]
fn transfer_null_total() {
    let mut wire = recording(TRANSFERS)["items"][0].clone();
    wire["total"] = Value::Null;
    let missing: TokenTransfer = decode(&wire);
    assert!(missing.total.is_none());
    assert_eq!(
        encoded(&missing)["transaction_hash"],
        wire["transaction_hash"]
    );
    assert_eq!(
        decode::<Decimal>(&encoded(&missing)["log_index"]),
        decode::<Decimal>(&wire["log_index"])
    );
    assert!(encoded(&missing)["total"].is_null());
    wire["total"] = json!({"value": "0", "decimals": null});
    let zero: TokenTransfer = decode(&wire);
    assert!(matches!(
        zero.total,
        Some(TokenTransferTotal::TotalErc20(_))
    ));
    assert_ne!(encoded(&zero)["total"], Value::Null);
    let page = json!({"items": [encoded(&missing), wire], "next_page_params": null});
    assert_eq!(decode::<GetTokenTransfers200Response>(&page).items.len(), 2);
    assert_eq!(
        decode::<GetTransactionTokenTransfers200Response>(&page)
            .items
            .len(),
        2
    );
}

fn call_trace() -> Value {
    // TransactionControllerTest /raw-trace: node payload passed through unchanged.
    json!({
        "action": {"callType": "call", "from": "0xa931c862e662134b85e4dc4baf5c70cc9ba74db4", "to": "0x1469b17ebf82fedf56f04109e5207bdc4554288c", "gas": "0x8600", "input": "0xb118e2db0000000000000000000000000000000000000000000000000000000000000008", "value": "0x174876e800"},
        "result": {"gasUsed": "0x7d37", "output": "0x"},
        "subtraces": 0, "traceAddress": [], "type": "call"
    })
}

fn create_trace() -> Value {
    json!({"action": {"from": "0xa931c862e662134b85e4dc4baf5c70cc9ba74db4", "gas": "0x8600", "init": "0x6000", "value": "0x0"}, "result": {"address": "0x1469b17ebf82fedf56f04109e5207bdc4554288c", "code": "0x00", "gasUsed": "0x1234"}, "subtraces": 0, "traceAddress": [1, 0], "type": "create"})
}

fn self_destruct_trace() -> Value {
    json!({"action": {"address": "0x1469b17ebf82fedf56f04109e5207bdc4554288c", "refundAddress": "0xa931c862e662134b85e4dc4baf5c70cc9ba74db4", "balance": "0x123"}, "subtraces": 0, "traceAddress": [2], "type": "suicide"})
}

#[test]
fn trace_address() {
    use blockscout_client::apis::transactions_api::GetTransactionRawTraceSuccess;
    let wire = json!([call_trace(), create_trace(), self_destruct_trace()]);
    let transport = response::<GetTransactionRawTraceSuccess>(&wire);
    let traces = transport.try_parse_as_success_model().unwrap();
    assert!(transport.try_as_success().is_some());
    assert_eq!(traces.len(), 3);
    for (trace, source) in traces.iter().zip(wire.as_array().unwrap()) {
        let out = encoded(trace);
        for key in ["action", "result", "traceAddress", "type", "error"] {
            assert_eq!(out[key], source[key]);
        }
        assert_eq!(trace.subtraces, decode::<Decimal>(&source["subtraces"]));
    }
    assert_eq!(traces[1].trace_address, vec![1, 0]);
    assert!(!encoded(&traces[0])
        .as_object()
        .unwrap()
        .contains_key("trace_address"));
}

#[test]
fn trace_call_type() {
    let trace: RawTrace = decode(&call_trace());
    match trace.action {
        RawTraceAction::RawTraceCallAction(action) => {
            assert_eq!(action.call_type, "call");
            assert_eq!(encoded(&action)["callType"], "call");
            assert!(encoded(&action).get("call_type").is_none());
        }
        _ => panic!("call action lost"),
    }
}

#[test]
fn trace_call_gas() {
    let trace: RawTrace = decode(&call_trace());
    match trace.result.unwrap() {
        RawTraceResult::RawTraceCallResult(result) => {
            assert_eq!(result.gas_used, "0x7d37");
            assert_eq!(encoded(&result), call_trace()["result"]);
        }
        _ => panic!("call result lost"),
    }
}

#[test]
fn trace_create_gas() {
    let trace: RawTrace = decode(&create_trace());
    assert!(matches!(
        trace.action,
        RawTraceAction::RawTraceCreateAction(_)
    ));
    match trace.result.unwrap() {
        RawTraceResult::RawTraceCreateResult(result) => {
            assert_eq!(result.gas_used, "0x1234");
            assert_eq!(encoded(&result), create_trace()["result"]);
        }
        _ => panic!("creation result lost"),
    }
}

#[test]
fn trace_refund_address() {
    let trace: RawTrace = decode(&self_destruct_trace());
    match trace.action {
        RawTraceAction::RawTraceSelfDestructAction(action) => {
            assert_eq!(encoded(&action), self_destruct_trace()["action"]);
            assert_eq!(
                action.refund_address,
                "0xa931c862e662134b85e4dc4baf5c70cc9ba74db4"
            );
        }
        _ => panic!("self-destruct action lost"),
    }
}

#[test]
fn internal_creation_destination() {
    let call = recording(INTERNAL)["items"][0].clone();
    let mut items = vec![call.clone()];
    for kind in ["create", "create2", "failed_create"] {
        let mut wire = call.clone();
        wire["type"] = json!(if kind == "create2" {
            "create2"
        } else {
            "create"
        });
        wire["to"] = Value::Null;
        wire["created_contract"] = if kind == "failed_create" {
            Value::Null
        } else {
            call["to"].clone()
        };
        wire["success"] = json!(kind != "failed_create");
        let internal: InternalTransaction = decode(&wire);
        assert!(internal.to.is_none());
        assert_eq!(internal.created_contract.is_some(), kind != "failed_create");
        assert_eq!(internal.success, kind != "failed_create");
        assert_eq!(internal.r#type, wire["type"]);
        let out = encoded(&internal);
        for field in ["from", "to", "created_contract"] {
            assert_address_fields(&out[field], &wire[field]);
        }
        items.push(wire);
    }
    let page = json!({"items": items, "next_page_params": null});
    assert_eq!(
        decode::<GetInternalTransactions200Response>(&page)
            .items
            .len(),
        4
    );
    assert_eq!(
        decode::<GetAddressInternalTxs200Response>(&page)
            .items
            .len(),
        4
    );
    assert_eq!(
        decode::<GetTransactionInternalTxs200Response>(&page)
            .items
            .len(),
        4
    );
    assert!(decode::<InternalTransaction>(&call).to.is_some());
    let global = decode::<GetInternalTransactions200Response>(&page);
    let address = decode::<GetAddressInternalTxs200Response>(&page);
    let transaction = decode::<GetTransactionInternalTxs200Response>(&page);
    for parsed in [&global.items, &address.items, &transaction.items] {
        for (internal, wire) in parsed.iter().zip(page["items"].as_array().unwrap()) {
            let out = encoded(internal);
            for field in ["from", "to", "created_contract"] {
                assert_address_fields(&out[field], &wire[field]);
            }
            assert_eq!(out["success"], wire["success"]);
            assert_eq!(out["type"], wire["type"]);
            assert_eq!(out["transaction_hash"], wire["transaction_hash"]);
        }
    }
}

fn assert_address_fields(actual: &Value, expected: &Value) {
    assert_eq!(actual.is_null(), expected.is_null());
    if let Some(fields) = expected.as_object() {
        let actual_fields = actual.as_object().unwrap();
        assert_eq!(actual_fields.len(), fields.len());
        for (field, value) in fields {
            assert_eq!(
                actual_fields.get(field),
                Some(value),
                "lost address {field}"
            );
        }
    }
}

#[test]
fn log_wire_shape() {
    let verified = recording(LOGS)["items"][0].clone();
    let mut unverified = verified.clone();
    unverified["decoded"] = Value::Null;
    unverified["smart_contract"] = Value::Null;
    for wire in [&verified, &unverified] {
        let log: Log = decode(wire);
        assert_eq!(
            log.address_hash.as_ref().unwrap().hash,
            wire["address"]["hash"]
        );
        let out = encoded(&log);
        for key in ["address", "data", "topics", "decoded", "smart_contract"] {
            if key == "address" || key == "smart_contract" {
                assert_eq!(out[key]["hash"], wire[key]["hash"]);
            } else {
                assert_eq!(out[key], wire[key]);
            }
        }
        assert!(out.get("address_hash").is_none());
        let page = json!({"items": [wire], "next_page_params": null});
        assert_eq!(
            decode::<GetTransactionLogs200Response>(&page).items.len(),
            1
        );
        assert_eq!(decode::<GetAddressLogs200Response>(&page).items.len(), 1);
    }
    assert!(decode::<Log>(&unverified).decoded.is_none());
    assert!(decode::<Log>(&unverified).smart_contract.is_none());
}

#[test]
fn log_structured_parameter() {
    // DecodingHelper.value_json: scalar, array, tuple, byte string, rescue/null.
    let mut wire = recording(LOGS)["items"][0].clone();
    let values = [
        json!("123"),
        json!(["1", "2"]),
        json!(["1", ["0x1234", ["2", "3"]]]),
        json!("0x1234"),
        Value::Null,
    ];
    wire["decoded"]["parameters"] = Value::Array(values.iter().enumerate().map(|(i, value)| json!({"name": format!("p{i}"), "type": (["uint256", "uint256[]", "tuple", "bytes", "uint256"][i]), "value": value, "indexed": false})).collect());
    let log: Log = decode(&wire);
    let decoded = log.decoded.as_ref().unwrap();
    assert_eq!(decoded.parameters.len(), values.len());
    for (parameter, value) in decoded.parameters.iter().zip(values) {
        assert_eq!(parameter.value, value);
    }
    assert_eq!(encoded(&log)["decoded"], wire["decoded"]);
}

#[test]
fn state_change_address() {
    let address = recording(TX)["from"].clone();
    for (kind, token, change) in [
        ("coin", Value::Null, json!("-3")),
        (
            "token",
            json!({"type": "ERC-20", "address_hash": address["hash"]}),
            json!("-3"),
        ),
        (
            "token",
            json!({"type": "ERC-721", "address_hash": address["hash"]}),
            json!([{"direction": "out", "total": {"token_id": "42", "token_instance": null}}]),
        ),
    ] {
        let wire = json!({"address": address, "is_miner": false, "type": kind, "token": token, "token_id": "42", "balance_before": "10", "balance_after": "7", "change": change});
        let model: StateChange = decode(&wire);
        assert_eq!(model.address_hash.as_ref().unwrap().hash, address["hash"]);
        let out = encoded(&model);
        for key in ["balance_before", "balance_after", "change", "token_id"] {
            assert_eq!(out[key], wire[key]);
        }
        assert_eq!(out["address"]["hash"], address["hash"]);
        assert!(out.get("address_hash").is_none());
        let page = json!({"items": [wire], "next_page_params": null});
        assert_eq!(
            decode::<GetTransactionStateChanges200Response>(&page)
                .items
                .len(),
            1
        );
    }
}

#[test]
fn holder_address() {
    for name in [
        "_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_holders.json",
        "_api_v2_tokens_0xD4416b13_instances_25625468_holders.json",
    ] {
        let page = recording(name);
        let holders: Vec<Holder> = decode(&page["items"]);
        assert!(!holders.is_empty());
        for (holder, wire) in holders.iter().zip(page["items"].as_array().unwrap()) {
            assert_eq!(
                holder.address_hash.as_ref().unwrap().hash,
                wire["address"]["hash"]
            );
            let out = encoded(holder);
            for key in ["token_id", "value"] {
                assert_eq!(out[key], wire[key]);
            }
            assert_eq!(out["address"]["hash"], wire["address"]["hash"]);
            assert!(out.get("address_hash").is_none());
        }
        assert_eq!(
            decode::<GetTokenHolders200Response>(&page).items.len(),
            holders.len()
        );
        assert_eq!(
            decode::<GetTokenInstanceHolders200Response>(&page)
                .items
                .len(),
            holders.len()
        );
    }
}

#[test]
fn contract_list_address() {
    let page = recording("_api_v2_smart-contracts.json");
    let model: GetSmartContracts200Response = decode(&page);
    assert!(!model.items.is_empty());
    for (contract, wire) in model.items.iter().zip(page["items"].as_array().unwrap()) {
        assert_eq!(
            contract.address_hash.as_ref().unwrap().hash,
            wire["address"]["hash"]
        );
        assert_eq!(contract.verified_at, wire["verified_at"]);
        assert_eq!(
            encoded(contract)["address"]["hash"],
            wire["address"]["hash"]
        );
        assert_eq!(encoded(contract)["verified_at"], wire["verified_at"]);
    }
}

#[test]
fn search_miss_nullability() {
    use blockscout_client::apis::search_api::SearchRedirectSuccess;
    for wire in [
        json!({"redirect": false, "type": null, "parameter": null}),
        json!({"redirect": true, "type": "address", "parameter": "0x1111111111111111111111111111111111111111"}),
    ] {
        let transport = response::<SearchRedirectSuccess>(&wire);
        assert!(transport.try_as_success().is_some());
        let result = transport.try_parse_as_success_model().unwrap();
        assert_eq!(encoded(&result), wire);
        assert_eq!(result.parameter.is_some(), result.redirect);
        assert_eq!(result.r#type.is_some(), result.redirect);
    }
}

#[test]
fn search_native_result() {
    use search_200_response_items_inner::Search200ResponseItemsInner as Item;
    let mut page = recording("_api_v2_search_q=USDT.json");
    let items = page["items"].as_array_mut().unwrap();
    items.extend([
        json!({"type": "price_update", "hash": "0x1234", "block_number": 125, "timestamp": "2026-01-01T00:00:00Z"}),
        json!({"type": "transaction", "transaction_hash": "0x2345", "url": "/tx/0x2345", "timestamp": "2026-01-01T00:00:00Z"}),
        json!({"type": "block", "block_hash": "0x3456", "block_number": 124, "url": "/block/124", "timestamp": "2026-01-01T00:00:00Z"}),
        json!({"type": "user_operation", "user_operation_hash": "0x4567", "timestamp": null, "priority": 3}),
        json!({"type": "blob", "blob_hash": "0x5678", "timestamp": "2026-01-01T00:00:00Z", "priority": 4}),
    ]);
    let model: Search200Response = decode(&page);
    let n = model.items.len();
    assert!(matches!(&model.items[0], Item::SearchResultToken(_)));
    assert!(matches!(
        &model.items[n - 5],
        Item::SearchResultPriceUpdate(_)
    ));
    assert!(matches!(
        &model.items[n - 4],
        Item::SearchResultTransaction(_)
    ));
    assert!(matches!(&model.items[n - 3], Item::SearchResultBlock(_)));
    assert!(matches!(
        &model.items[n - 2],
        Item::SearchResultUserOperation(_)
    ));
    assert!(matches!(&model.items[n - 1], Item::SearchResultBlob(_)));
    assert_eq!(encoded(&model.items[n - 5]), page["items"][n - 5]);
    assert_eq!(encoded(&model.items[n - 2]), page["items"][n - 2]);
    assert_eq!(encoded(&model.items[n - 1]), page["items"][n - 1]);
    for invalid in [
        json!({"type": "price_update", "name": null, "url": null}),
        json!({"type": "future_result", "hash": "0x1234"}),
    ] {
        assert!(serde_json::from_value::<Item>(invalid).is_err());
    }
}

#[test]
fn search_address_nullability() {
    for wire in [
        json!({"type": "address", "name": null, "url": "/address/0x1234", "address_hash": "0x1234", "is_smart_contract_verified": false}),
        json!({"type": "ens_domain", "name": "unresolved.eth", "url": null, "address_hash": null, "is_smart_contract_verified": null}),
    ] {
        let item: Search200ResponseItemsInner = decode(&wire);
        match &item {
            Search200ResponseItemsInner::SearchResultAddressOrContract(address) => {
                assert_eq!(address.r#type, wire["type"])
            }
            _ => panic!("address variant lost"),
        }
        assert_eq!(encoded(&item), wire);
    }
}

#[test]
fn search_token_supply() {
    let page = recording("_api_v2_search_q=USDT.json");
    let mut wire = page["items"][0].clone();
    assert_eq!(wire["type"], "token");
    wire["total_supply"] = Value::Null;
    wire["exchange_rate"] = Value::Null;
    let item: Search200ResponseItemsInner = decode(&wire);
    match &item {
        Search200ResponseItemsInner::SearchResultToken(token) => {
            assert!(token.total_supply.is_none());
            assert!(token.exchange_rate.is_none());
            assert_eq!(encoded(token)["address_hash"], wire["address_hash"]);
        }
        _ => panic!("token variant lost"),
    }
    assert!(encoded(&item)["total_supply"].is_null());
    wire["total_supply"] = json!("0");
    assert_eq!(
        decode::<SearchResultToken>(&wire).total_supply.as_deref(),
        Some("0")
    );
}

fn native_feed_item() -> Value {
    // FeedView.prepare_update, including its sparse sender and integer discriminator.
    json!({"hash": "0x1234", "block_number": 20077922, "block_hash": "0x2345", "timestamp": "2026-01-01T00:00:00Z", "index": 2, "from": {"hash": "0x1111111111111111111111111111111111111111", "is_contract": false}, "to": null, "value": "0", "fee": {"type": "actual", "value": "0"}, "gas_used": "234", "gas_limit": "456", "status": "ok", "result": "success", "method": "submitPrices", "transaction_types": ["price_update"], "type": 127, "confirmations": 5, "price_update": {"feeder_bech32": "pax1feeder", "observation_count": 2, "observations": [
        {"market_id": 1, "accepted": true, "symbol": "BTC", "index_price": "100000000000000000000000", "mark_price": "100100000000000000000000", "source_timestamp_ms": 1767225600000u64, "sequence": 42},
        {"market_id": 2, "accepted": false, "symbol": null, "index_price": "2000000000000000000000", "mark_price": "2100000000000000000000", "source_timestamp_ms": 1767225600001u64, "sequence": 43}
    ]}})
}

#[test]
fn transaction_feed_native_items() {
    use blockscout_client::apis::feed::{FeedItem, TransactionFeedPage};
    use blockscout_client::apis::transactions_api::GetTxsSuccess;

    let evm = recording(TX);
    let native = native_feed_item();
    // PriceUpdateList.paging_params adds its 1_000_000 native index offset.
    let cursor = json!({"block_number": 20077922, "index": 1000002, "items_count": 2});
    let page = json!({"items": [native, evm], "next_page_params": cursor});
    let transport = response::<GetTxsSuccess>(&page);
    assert!(transport.try_as_success().is_some());
    let model = transport.try_parse_as_success_model().unwrap();
    match &model.items[0] {
        FeedItem::Native(update) => {
            assert_eq!(update.price_update.observation_count, 2);
            assert_eq!(update.price_update.observations[1].symbol, None);
            assert!(!update.price_update.observations[1].accepted);
            assert_eq!(
                update.price_update.observations[0].index_price,
                "100000000000000000000000"
            );
            assert_eq!(encoded(update), native);
        }
        _ => panic!("native feed item lost"),
    }
    assert!(matches!(&model.items[1], FeedItem::Evm(_)));
    assert_eq!(model.next_page_params, cursor);
    let next_page = json!({"items": [evm, native], "next_page_params": null});
    let next: TransactionFeedPage = decode(&next_page);
    assert!(matches!(&next.items[0], FeedItem::Evm(_)));
    assert!(matches!(&next.items[1], FeedItem::Native(_)));
    assert!(next.next_page_params.is_null());
    let mut malformed = native.clone();
    malformed.as_object_mut().unwrap().remove("price_update");
    assert!(serde_json::from_value::<FeedItem>(malformed).is_err());
}

#[test]
fn address_list_market_nullability() {
    let mut address =
        recording("_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de.json");
    address["transactions_count"] =
        recording("_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de_counters.json")
            ["transactions_count"]
            .clone();
    for (price, supply) in [
        (Value::Null, json!("1000000")),
        (json!("1.2"), Value::Null),
        (Value::Null, Value::Null),
    ] {
        let page = json!({"exchange_rate": price, "total_supply": supply, "items": [address], "next_page_params": null});
        let model: GetAddresses200Response = decode(&page);
        assert_eq!(model.items.len(), 1);
        let out = encoded(&model);
        assert_eq!(out["exchange_rate"], price);
        assert_eq!(out["total_supply"], supply);
        assert_eq!(out["items"][0]["hash"], address["hash"]);
        assert_eq!(out["items"][0]["coin_balance"], address["coin_balance"]);
    }
}

#[test]
fn market_supply_integer() {
    use get_market_chart_200_response::AvailableSupply;
    let chart = json!([{"date": "2026-01-01", "closing_price": "1.234", "market_cap": "123400"}]);
    for supply in [
        json!(0),
        json!(1234),
        json!("123456789012345678901234567890.123456789"),
    ] {
        let wire = json!({"available_supply": supply, "chart_data": chart});
        let model: GetMarketChart200Response = decode(&wire);
        match &model.available_supply {
            AvailableSupply::Integer(value) => assert_eq!(Some(*value), supply.as_i64()),
            AvailableSupply::String(value) => assert_eq!(Some(value.as_str()), supply.as_str()),
        }
        assert_eq!(encoded(&model), wire);
    }
    for invalid in [json!(0.5), Value::Null, json!(true)] {
        assert!(serde_json::from_value::<GetMarketChart200Response>(
            json!({"available_supply": invalid, "chart_data": chart})
        )
        .is_err());
    }
}

#[test]
fn health_blocks_envelope() {
    use blockscout_client::apis::health_api::HealthSuccess;
    let healthy = recording("_api_health.json");
    let mut unhealthy = healthy.clone();
    unhealthy["healthy"] = json!(false);
    unhealthy["metadata"]["blocks"]["healthy"] = json!(false);
    let error = json!({"code": 5002, "message": "There are no blocks in the DB."});
    unhealthy["metadata"]["blocks"]["error"] = error.clone();
    unhealthy["error"] = error.clone();
    for wire in [&healthy, &unhealthy] {
        let response = response::<HealthSuccess>(wire);
        let model = response.try_parse_as_success_model().unwrap();
        assert!(response.try_as_success().is_some());
        let blocks = &model.metadata.as_ref().unwrap().blocks;
        assert_eq!(blocks.healthy, model.healthy);
        assert_eq!(
            encoded(&blocks.latest_block)["db"],
            wire["metadata"]["blocks"]["latest_block"]["db"]
        );
        assert_eq!(encoded(&model)["error"], wire["error"]);
        if !blocks.healthy {
            assert_eq!(blocks.error.as_ref().unwrap().code, 5002);
            assert_eq!(encoded(&blocks.error), error);
        }
    }
}

#[test]
fn summary_variable_map() {
    use summary_template_variables::SummaryTemplateValue;
    // TransactionControllerTest's summary response and arbitrary preload variable names.
    let variables = json!({
        "action_type": {"type": "string", "value": "Swap"},
        "outgoing_amount": {"type": "currency", "value": "0.3"},
        "incoming_amount": {"type": "currency", "value": "9693.997876398680187001"},
        "incoming_token": {"type": "token", "value": {"address_hash": "0x1111", "symbol": "TKN", "decimals": "18", "reputation": "ok"}},
        "rnd_address": {"type": "address", "value": {"hash": "0x2222", "ens_domain_name": null}},
        "market": {"type": "dexTag", "value": {"name": "Uniswap V3", "icon": "https://blockscout-content.s3.amazonaws.com/uniswap.png", "url": "https://uniswap.org/?utm_source=Blockscout"}}
    });
    let template = "{action_type} {outgoing_amount} for {incoming_amount} {incoming_token} via {rnd_address} on {market}";
    let wire = json!({"success": true, "data": {"summaries": [{"summary_template": template, "summary_template_variables": variables}]}});
    let model: TransactionSummary = decode(&wire);
    let summary = &model.data.summaries.as_ref().unwrap()[0];
    let vars = &summary.summary_template_variables;
    assert_eq!(vars.len(), 6);
    let mut rendered = summary.summary_template.clone();
    for (name, value) in vars.iter() {
        let text = match value {
            SummaryTemplateValue::String(s) | SummaryTemplateValue::Currency(s) => s.clone(),
            SummaryTemplateValue::Token(token) => {
                assert_eq!(token.properties["reputation"], "ok");
                token.token.symbol.clone().unwrap()
            }
            SummaryTemplateValue::Address(address) => address.hash.clone(),
            SummaryTemplateValue::DexTag(tag) => tag.name.clone(),
        };
        rendered = rendered.replace(&format!("{{{name}}}"), &text);
    }
    assert_eq!(
        rendered,
        "Swap 0.3 for 9693.997876398680187001 TKN via 0x2222 on Uniswap V3"
    );
    let out = encoded(vars);
    for (name, value) in variables.as_object().unwrap() {
        assert_eq!(out[name]["type"], value["type"]);
        if name == "incoming_token" {
            for (key, expected) in value["value"].as_object().unwrap() {
                assert_eq!(out[name]["value"][key], *expected);
            }
        } else {
            assert_eq!(out[name], *value);
        }
    }
    assert!(serde_json::from_value::<SummaryTemplateVariables>(
        json!({"invalid": {"type": "currency", "value": {}}})
    )
    .is_err());
}

#[test]
fn health_node_height() {
    let mut wire = recording("_api_health.json");
    let latest = &mut wire["metadata"]["blocks"]["latest_block"];
    latest["db"]["number"] = json!("100");
    latest["cache"]["number"] = json!("102");
    latest["node"]["number"] = json!("110");
    let health: HealthCheckResponse = decode(&wire);
    let blocks = health.metadata.unwrap().blocks;
    assert_eq!(blocks.latest_block.db.number, "100");
    assert_eq!(blocks.latest_block.cache.number, "102");
    assert_eq!(blocks.latest_block.node.number, "110");
    assert_eq!(
        encoded(&blocks.latest_block),
        wire["metadata"]["blocks"]["latest_block"]
    );
}
