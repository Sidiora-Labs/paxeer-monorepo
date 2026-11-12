include!("../src/main.rs");
use layerx_platform_authority::assertion::Must;
include!("support/committed_human_fixture.rs");

#[path = "../../../../agent/crates/layerx-agentd/tests/support/committed_authority_documents.rs"]
mod committed_documents;

fn committed_document(config: Arc<Config>, path: &str, export: bool) -> Value {
    let (status, document) = tls_request(config, path, "repair-human-token-208");
    assert_eq!(status, 200, "{path}: {document}");
    let state = if export {
        &document["budget_state"]
    } else {
        &document
    };
    let rank = u8::try_from(
        state["verification"]
            .as_u64()
            .must_with("verification rank"),
    )
    .must_with("byte rank");
    assert_eq!(
        rank,
        layerx_types::verify::VerificationLevel::COMMITTED.wire_rank()
    );
    assert_eq!(
        layerx_types::verify::VerificationLevel::from_wire_rank(rank),
        Ok(layerx_types::verify::VerificationLevel::COMMITTED)
    );
    document
}

#[test]
fn repair_22_31_535_producers_emit_committed_documents() {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let budget = committed_document(
        config.clone(),
        &budget_path("budget-state", &fixture),
        false,
    );
    let export = committed_document(config, &budget_path("budget-proof", &fixture), true);
    assert_eq!(
        budget, export["budget_state"],
        "read and export share the producer"
    );
    let mut corpus =
        document!({"budget_id":hex::encode(&fixture.id), "budget":budget, "export":export});

    for capability in [false, true] {
        let fixture = Arc::new(identity_fixture(capability));
        let node = Node::start(fixture.clone());
        let directory = Directory::new();
        let config = config_fixture(&node, &fixture, &directory.0);
        retain_records(&config, &fixture.records);
        drop(config);
        let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
        let policy = fixture.policy();
        if capability {
            let scope = &policy["identities"][0]["capabilities"][0];
            let path = format!(
                "{}&did={}&authority={}&action_key={}&capability_id={}",
                human_path("capability-scope"),
                fixture.did,
                scope["authority"].as_str().must(),
                scope["action_key"].as_str().must(),
                scope["capability_id"].as_str().must()
            );
            corpus["scope"] = committed_document(config, &path, false);
            corpus["scope_did"] = document!(fixture.did);
        } else {
            let path = format!(
                "{}&did={}&recovery=false",
                human_path("key-policy"),
                fixture.did
            );
            corpus["policy"] = committed_document(config, &path, false);
            corpus["policy_did"] = document!(fixture.did);
        }
    }
    let repository = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let output = committed_documents::corpus_path(&repository);
    let parent = output.parent().must();
    fs::create_dir_all(parent).must_with(committed_documents::CORPUS_READ_EXPECTATION);
    fixture_must(fs::set_permissions(
        parent,
        fs::Permissions::from_mode(0o700),
    ));
    let pending = parent.join("documents.pending");
    fixture_must(fs::write(
        &pending,
        fixture_must(serde_json::to_vec(&corpus)),
    ));
    fixture_must(fs::set_permissions(
        &pending,
        fs::Permissions::from_mode(0o600),
    ));
    fixture_must(fs::rename(pending, output));
}
