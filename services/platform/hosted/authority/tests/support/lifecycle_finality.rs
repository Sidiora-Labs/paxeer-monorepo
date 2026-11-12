#[test]
fn historical_anchor_abi_preserves_checkpoint_decoding_layout() {
    let abi: serde_json::Value = serde_json::from_str(include_str!(
        "../../../../../../precompiles/layerxanchor/abi.json"
    ))
    .unwrap_or_else(|error| panic!("historical anchor ABI: {error}"));
    let methods = abi.as_array().unwrap_or_else(|| panic!("anchor ABI"));
    let latest = methods
        .iter()
        .find(|row| row["name"] == "latestFinalized")
        .unwrap_or_else(|| panic!("latestFinalized"));
    assert_eq!(latest["outputs"][0]["type"], "uint64");
    assert_eq!(latest["outputs"][1]["type"], "bool");
    let checkpoint = methods
        .iter()
        .find(|row| row["name"] == "checkpoint")
        .unwrap_or_else(|| panic!("checkpoint"));
    assert_eq!(checkpoint["inputs"][0]["type"], "uint64");
    let fields = checkpoint["outputs"][0]["components"]
        .as_array()
        .unwrap_or_else(|| panic!("checkpoint tuple"));
    assert_eq!(fields.len(), 18);
    for (index, name, kind) in [
        (0, "batchNumber", "uint64"),
        (1, "checkpointId", "bytes32"),
        (7, "stateRoot", "bytes32"),
        (11, "timestampMs", "uint64"),
        (12, "status", "uint8"),
        (17, "finalizedHeight", "uint64"),
    ] {
        assert_eq!(fields[index]["name"], name);
        assert_eq!(fields[index]["type"], kind);
    }
}

pub(super) fn assert_committed_budget_membership(
    proof: &serde_json::Value,
    finality: &serde_json::Value,
    receipt_sequence: u64,
) {
    assert_eq!(proof["finality"], *finality);
    assert_eq!(proof["head"]["committed_height"], finality["height"]);
    assert_eq!(proof["head"]["block_hash"], finality["block_hash"]);
    assert_eq!(
        proof["budget_state"]["checkpoint_digest"],
        finality["block_hash"]
    );
    assert_eq!(
        proof["budget_state"]["observed_head_sequence"],
        receipt_sequence
    );
    let state_height = finality["state_height"]
        .as_u64()
        .unwrap_or_else(|| panic!("state height"));
    assert_eq!(finality["height"].as_u64(), state_height.checked_add(1));
    for call in ["budget_call", "remaining_call"] {
        assert_eq!(proof["proofs"][call]["height"], state_height);
    }
    let join = &proof["proofs"]["account_join"];
    assert_eq!(join["finality"], *finality);
    assert_eq!(join["binding_call"]["height"], state_height);
    for membership in [
        &join["native_account_proof"],
        &join["receipt_sequence_proof"],
    ]
    .into_iter()
    .chain(
        join["balances"]
            .as_array()
            .unwrap_or_else(|| panic!("authenticated balances"))
            .iter()
            .map(|balance| &balance["proof"]),
    ) {
        assert_eq!(membership["code"], 0);
        assert_eq!(membership["height"], state_height.to_string());
        let ops = membership["proofOps"]["ops"]
            .as_array()
            .unwrap_or_else(|| panic!("proof ops"));
        assert_eq!(ops.len(), 2);
        assert_eq!(ops[0]["type"], "ics23:iavl");
        assert_eq!(ops[1]["type"], "ics23:simple");
    }
}
