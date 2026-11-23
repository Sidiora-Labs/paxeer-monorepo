include!("committed_budget_exports.rs");

fn protected(path: &Path, maximum: u64) -> Vec<u8> {
    let metadata = fs::symlink_metadata(path).must_with("real corpus metadata");
    assert!(metadata.is_file() && metadata.len() > 0 && metadata.len() <= maximum);
    assert_eq!(metadata.permissions().mode() & 0o077, 0);
    fs::read(path).must_with("real protected corpus")
}

#[test]
fn real_budget_exports_bind_each_selector_finality_and_tamper_refusals() {
    let corpus = produced_export_corpus();
    let fixture: Value =
        serde_json::from_slice(&protected(&corpus.path, 65_536)).must_with("produced corpus JSON");
    assert_export_corpus(&fixture);
}

#[test]
fn budget_abi_matches_node_precompile_types() {
    let abi: Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxpay/abi.json"
    ))
    .must_with("node ABI");
    let get = abi
        .as_array()
        .must_with("ABI")
        .iter()
        .find(|row| row["name"] == "getBudget")
        .must_with("getBudget");
    assert_eq!(get["inputs"][0]["type"], "bytes32");
    let fields = get["outputs"][0]["components"]
        .as_array()
        .must_with("tuple");
    assert_eq!(fields.len(), 18);
    assert_eq!(fields[16]["type"], "bytes32[]");
    assert_eq!(fields[17]["name"], "sourceAccount");
}

#[test]
fn budget_export_from_the_fixture_answers_equals_the_committed_export() {
    let fixture = Arc::new(Fixture::new(false));
    let produced = produced_export_corpus();
    let corpus: Value = serde_json::from_slice(&protected_export(&produced.path, 65_536))
        .must_with("produced corpus JSON");
    let cases = corpus["cases"]
        .as_array()
        .must_with("required replay cases");
    assert!(!cases.is_empty());
    let policy = fixture.policy();
    let id = hex::encode(&fixture.id);
    let maximum_age_ms = u128::from(
        policy["maximum_age_seconds"]
            .as_u64()
            .must_with("maximum age"),
    ) * 1000;
    for case in cases {
        let exchanges = case["rpc_exchanges"]
            .as_array()
            .must_with("required captured RPC exchanges")
            .clone();
        assert!(!exchanges.is_empty());
        let committed_height = case["committed_height"]
            .as_u64()
            .must_with("committed height");
        let state_height = case["state_height"].as_u64().must_with("state height");
        let block = &exchanges
            .iter()
            .find(|row| {
                row["method"] == "eth_getBlockByNumber"
                    && row["params"][0] == format!("{committed_height:#x}")
            })
            .must_with("committed block exchange")["result"];
        let timestamp_ms = u128::from(
            u64::from_str_radix(
                block["timestamp"]
                    .as_str()
                    .must_with("block timestamp")
                    .strip_prefix("0x")
                    .must_with("timestamp prefix"),
                16,
            )
            .must_with("timestamp quantity"),
        ) * 1000;
        let expected: Value = serde_json::from_slice(&protected_export(
            Path::new(case["response"].as_str().must_with("response path")),
            1_048_576,
        ))
        .must_with("produced export JSON");
        let replay = Node::with_transcript(fixture.clone(), Some(exchanges));
        let directory = Directory::new();
        let config = config_fixture(&replay, &fixture, &directory.0);
        let eth_call_count = || {
            replay
                .state
                .lock()
                .must_with("replay state")
                .calls
                .iter()
                .filter(|call| call["method"] == "eth_call")
                .count()
        };
        let response = config
            .fixture_budget_export(&policy, &id, timestamp_ms)
            .unwrap_or_else(|response| {
                panic!(
                    "export refused: {}",
                    String::from_utf8_lossy(&response.body)
                )
            });
        assert_eq!(response.status, 200);
        assert_eq!(eth_call_count(), 5);
        let document: Value = serde_json::from_slice(&response.body).must_with("export JSON");
        let finality = serde_json::json!({
            "height":case["committed_height"], "block_hash":case["block_hash"],
            "app_hash":case["app_hash"], "state_height":case["state_height"]
        });
        assert!(verify_committed_export(&document, &finality).is_ok());
        assert!(document["proofs"].get("checkpoint_call").is_none());
        assert_eq!(document["budget_state"]["budget_id"], id);
        assert_eq!(document["accounts"]["owner"], hex::encode(&fixture.owner));
        let boundary = config
            .fixture_budget_export(&policy, &id, timestamp_ms + maximum_age_ms)
            .unwrap_or_else(|response| {
                panic!(
                    "freshness boundary refused: {}",
                    String::from_utf8_lossy(&response.body)
                )
            });
        assert_eq!(boundary.body, response.body);
        assert_eq!(eth_call_count(), 10);
        assert!(config
            .fixture_budget_export(&policy, &id, timestamp_ms + maximum_age_ms + 1)
            .is_err());
        assert_eq!(eth_call_count(), 15);
        assert!(config
            .fixture_budget_export(&policy, &id, timestamp_ms - 1)
            .is_err());
        assert_eq!(eth_call_count(), 20);
        let state = replay.state.lock().must_with("replay state");
        assert!(state
            .calls
            .iter()
            .filter(|call| call["method"] == "eth_call")
            .all(|call| call["params"][1] == format!("{state_height:#x}")));
        assert_eq!(document, expected);
    }
}
