// Reuse the committed in-memory node, TLS transport and production authority harness.
include!("repair_22_31_208.rs");

#[path = "support/lifecycle_finality.rs"]
mod lifecycle_finality;

fn lifecycle_budget_path(fixture: &Fixture) -> String {
    format!(
        "/v1/agent/budget-proof?tenant=repair-tenant&principal=repair-principal&budget_id={}",
        hex::encode(&fixture.id)
    )
}

fn assert_exported_membership(fixture: &Fixture, proof: &Value, height: u64) {
    let finality = document!({"height":height,"state_height":height-1,
        "block_hash":hex::encode(&[height as u8;32]),
        "app_hash":hex::encode(&fixture.store.app_hash)});
    lifecycle_finality::assert_committed_budget_membership(proof, &finality, fixture.sequence);
    assert_ne!(fixture.sequence, height);
    let join = &proof["proofs"]["account_join"];
    let sequence_key = vec![2];
    let owner_key = [vec![6], fixture.owner.to_vec()].concat();
    let budget_key = [vec![6], fixture.budget.to_vec()].concat();
    let balances = join["balances"].as_array().expect("balances");
    assert_eq!(balances.len(), 2);
    assert_eq!(balances[0]["account_id"], hex::encode(&fixture.owner));
    assert_eq!(balances[0]["amount"], "500");
    assert_eq!(balances[1]["account_id"], hex::encode(&fixture.budget));
    assert_eq!(balances[1]["amount"], "77");
    assert_eq!(proof["budget_state"]["remaining"], "77");
    for (key, membership) in [
        (&sequence_key, &join["receipt_sequence_proof"]),
        (&owner_key, &join["native_account_proof"]),
        (&owner_key, &balances[0]["proof"]),
        (&budget_key, &balances[1]["proof"]),
    ] {
        let expected = &fixture.store.queries[&hex::encode(key)];
        assert_eq!(membership["key"], base64(key));
        assert_eq!(membership["value"], expected["value"]);
        assert_eq!(membership["proofOps"], expected["proofOps"]);
        let ops = &membership["proofOps"]["ops"];
        must(committed::verify_membership(
            key,
            &must(committed::base64(membership["value"].as_str().unwrap())),
            &must(committed::base64(ops[0]["data"].as_str().unwrap())),
            &must(committed::base64(ops[1]["data"].as_str().unwrap())),
            fixture.store.app_hash,
        ));
    }
}

#[test]
fn lifecycle_budget_finality_proves_node_membership_at_the_plan_height() {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let path = lifecycle_budget_path(&fixture);
    let mut previous: Option<Value> = None;
    for height in [9, 10] {
        {
            let mut state = node.state.lock().unwrap();
            state.height = height;
            state.calls.clear();
        }
        let (status, proof) = tls_request(config.clone(), &path, "repair-human-token-208");
        assert_eq!(status, 200, "{proof}");
        assert_exported_membership(&fixture, &proof, height);
        if let Some(prior) = previous {
            assert_eq!(
                proof["budget_state"]["remaining"],
                prior["budget_state"]["remaining"]
            );
            assert_eq!(
                proof["budget_state"]["observed_head_sequence"],
                prior["budget_state"]["observed_head_sequence"]
            );
            assert_ne!(
                proof["finality"]["block_hash"],
                prior["finality"]["block_hash"]
            );
            assert_ne!(
                proof["budget_state"]["evidence_digest"],
                prior["budget_state"]["evidence_digest"]
            );
        }
        let calls = node.state.lock().unwrap().calls.clone();
        assert!(calls.iter().any(|call| call["method"] == "px_getCommit"
            && call["params"] == document!([height.to_string()])));
        let queries: Vec<_> = calls
            .iter()
            .filter(|call| call["method"] == "abci_query")
            .collect();
        assert_eq!(queries.len(), 4);
        for query in queries {
            assert_eq!(query["params"]["height"], (height - 1).to_string());
            assert_eq!(query["params"]["prove"], true);
        }
        let evm_calls: Vec<_> = calls
            .iter()
            .filter(|call| call["method"] == "eth_call")
            .collect();
        assert_eq!(evm_calls.len(), 5);
        for call in evm_calls {
            assert_eq!(call["params"][1], format!("0x{:x}", height - 1));
            let to = call["params"][0]["to"].as_str().expect("precompile");
            assert!(["1004", "101a", "101b"]
                .iter()
                .any(|suffix| to.ends_with(suffix)));
        }
        previous = Some(proof);
    }
}

#[test]
fn lifecycle_budget_finality_refuses_wrong_membership_and_replaced_commits() {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let path = lifecycle_budget_path(&fixture);
    for fault in [
        "proof",
        "value",
        "key",
        "height",
        "store",
        "apphash",
        "balance-proof",
        "reorg",
    ] {
        node.fault(fault);
        let (status, answer) = tls_request(config.clone(), &path, "repair-human-token-208");
        assert_eq!(status, 503, "accepted {fault}: {answer}");
        assert_eq!(answer["error"]["code"], "budget_proof_export_unavailable");
        assert!(
            answer.get("budget_state").is_none(),
            "exported {fault}: {answer}"
        );
    }
    node.fault("");
    let (status, proof) = tls_request(config, &path, "repair-human-token-208");
    assert_eq!(status, 200, "{proof}");
    assert_exported_membership(&fixture, &proof, 9);
}
