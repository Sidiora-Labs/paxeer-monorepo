// Task record 22.33.1471 / audit 22.30.85 / acceptance 22998.1.
// budget-export-corpus-optional -> budget_export_corpus_optional
// human-tls-retired-anchor -> human_tls_retired_anchor
// human-budget-corpus-optional -> human_budget_corpus_optional
// retirement-guard-requires-anchor -> retirement_guard_requires_anchor
// Baseline: not run; implementation-first and hf-done-only instructions take
// precedence over the contradictory baseline step. No baseline failure claimed.
// Evidence: the private hf-build/hf-done ledger records the verified candidate.
// Wiring: services/platform/hosted/testnet/tests (platform/domain8) owns live-node
// capture qualification; this gate uses canonical in-memory peer stores and TLS.
include!("../src/main.rs");

include!("support/committed_human_fixture.rs");

fn assert_no_retired_calls(node: &Node) {
    let calls = node.state.lock().unwrap().calls.clone();
    assert!(!calls.is_empty());
    assert!(calls.iter().any(|r| r["method"] == "px_getCommit"));
    assert!(calls.iter().any(|r| r["method"] == "abci_query"));
    assert!(!calls
        .iter()
        .any(|r| r["method"] == "eth_call"
            && r["params"][0]["to"].as_str().unwrap().ends_with("1014")));
    for signature in [b"latestFinalized()".as_slice(), b"checkpoint(uint64)"] {
        let selector =
            hex::encode(&layerx_crypto::evm_transaction::transaction_hash(signature)[..4]);
        assert!(!calls.iter().any(|r| r["method"] == "eth_call"
            && r["params"][0]["data"]
                .as_str()
                .unwrap()
                .starts_with(&format!("0x{selector}"))));
    }
}

pub(crate) struct ProducedCorpus {
    directory: Directory,
    pub(crate) path: PathBuf,
}

pub(crate) fn produced_export_corpus() -> ProducedCorpus {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let (status, export) = tls_request(
        config,
        &budget_path("budget-proof", &fixture),
        "repair-human-token-208",
    );
    assert_eq!(status, 200, "{export}");
    assert_no_retired_calls(&node);
    let response = directory.0.join("export.json");
    fixture_must(fs::write(
        &response,
        fixture_must(serde_json::to_vec(&export)),
    ));
    fixture_must(fs::set_permissions(
        &response,
        fs::Permissions::from_mode(0o600),
    ));
    let corpus = document!({"cases":[{"response":response,"rpc_exchanges":node.state.lock().unwrap().exchanges,
        "budget_id":hex::encode(&fixture.id),"owner_account":hex::encode(&fixture.owner),"owner_did":fixture.did,
        "asset_id":hex::encode(&fixture.asset),"committed_height":export["finality"]["height"],
        "state_height":export["finality"]["state_height"],"block_hash":export["finality"]["block_hash"],"app_hash":export["finality"]["app_hash"]}]});
    let path = directory.0.join("corpus.json");
    fixture_must(fs::write(&path, fixture_must(serde_json::to_vec(&corpus))));
    fixture_must(fs::set_permissions(
        &path,
        fs::Permissions::from_mode(0o600),
    ));
    ProducedCorpus { directory, path }
}

fn protected_export(path: &Path, maximum: u64) -> Vec<u8> {
    let metadata = fixture_must(fs::symlink_metadata(path));
    assert!(metadata.is_file() && metadata.len() > 0 && metadata.len() <= maximum);
    assert_eq!(metadata.permissions().mode() & 0o077, 0);
    fixture_must(fs::read(path))
}

fn export_bytes(value: &Value) -> Result<Vec<u8>, ()> {
    let text = value.as_str().ok_or(())?;
    let decoded = hex::decode(text).map_err(|_| ())?;
    if hex::encode(&decoded) != text {
        return Err(());
    }
    Ok(decoded)
}

fn verify_native_export_proof(
    proof: &Value,
    key: &[u8],
    state_height: u64,
    root: [u8; 32],
) -> Result<Vec<u8>, ()> {
    if proof["code"] != 0
        || proof["height"] != state_height.to_string()
        || committed::base64(proof["key"].as_str().ok_or(())?)? != key
    {
        return Err(());
    }
    let value = committed::base64(proof["value"].as_str().ok_or(())?)?;
    let ops = proof["proofOps"]["ops"].as_array().ok_or(())?;
    if ops.len() != 2
        || ops[0]["type"] != "ics23:iavl"
        || ops[1]["type"] != "ics23:simple"
        || committed::base64(ops[0]["key"].as_str().ok_or(())?)? != key
        || committed::base64(ops[1]["key"].as_str().ok_or(())?)? != b"lxphost"
    {
        return Err(());
    }
    committed::verify_membership(
        key,
        &value,
        &committed::base64(ops[0]["data"].as_str().ok_or(())?)?,
        &committed::base64(ops[1]["data"].as_str().ok_or(())?)?,
        root,
    )?;
    Ok(value)
}

fn verify_committed_export(document: &Value, expected_finality: &Value) -> Result<(), ()> {
    use committed_budget_record::{number, Record};
    if document["schema"] != "layerx.human.budget-proof.v2"
        || document["protocol_version"] != 3
        || document.get("lni_interface_version").is_some()
        || document.get("authorization").is_some()
        || document.get("maximum_bytes").is_some()
        || &document["finality"] != expected_finality
    {
        return Err(());
    }
    let state = &document["budget_state"];
    let proofs = &document["proofs"];
    if proofs.as_object().ok_or(())?.len() != 3 {
        return Err(());
    }
    let budget = export_bytes(&proofs["budget_call"]["result"])?;
    let remaining = export_bytes(&proofs["remaining_call"]["result"])?;
    let record = Record::from_call(
        export_bytes(&state["budget_id"])?
            .try_into()
            .map_err(|_| ())?,
        &budget,
    )?;
    if export_bytes(&state["canonical_core_bytes"])? != budget
        || export_bytes(&state["asset"])? != record.asset
        || state["revocation_sequence"] != record.revocation
        || state["closed"] != record.closed
        || state["revoked"] != record.revoked
        || remaining.len() != 32
        || state["remaining"] != number(&remaining, 0, 16)?.to_string()
    {
        return Err(());
    }
    let head = &document["head"];
    let finality = &document["finality"];
    if head.as_object().ok_or(())?.len() != 2 || finality.as_object().ok_or(())?.len() != 4 {
        return Err(());
    }
    let committed = head["committed_height"].as_u64().ok_or(())?;
    let block_hash: [u8; 32] = export_bytes(&head["block_hash"])?
        .try_into()
        .map_err(|_| ())?;
    let state_height = finality["state_height"].as_u64().ok_or(())?;
    let app_hash: [u8; 32] = export_bytes(&finality["app_hash"])?
        .try_into()
        .map_err(|_| ())?;
    if committed == 0
        || finality["height"] != committed
        || block_hash == [0; 32]
        || app_hash == [0; 32]
        || export_bytes(&finality["block_hash"])? != block_hash
        || export_bytes(&state["checkpoint_digest"])? != block_hash
        || state_height.checked_add(1) != Some(committed)
        || record.revocation == 0
        || record.revocation > state["observed_head_sequence"].as_u64().ok_or(())?
    {
        return Err(());
    }
    for (name, signature) in [
        ("budget_call", b"getBudget(bytes32)".as_slice()),
        ("remaining_call", b"budgetRemaining(bytes32)".as_slice()),
    ] {
        let call = &proofs[name];
        let mut data = layerx_crypto::evm_transaction::transaction_hash(signature)[..4].to_vec();
        data.extend_from_slice(&record.id);
        if call.as_object().ok_or(())?.len() != 4
            || call["height"] != state_height
            || export_bytes(&call["to"])?
                != hex::decode("000000000000000000000000000000000000101b").map_err(|_| ())?
            || export_bytes(&call["data"])? != data
        {
            return Err(());
        }
    }
    for (name, id) in [
        ("owner", record.owner),
        ("budget", record.account),
        ("source", record.source.unwrap_or(record.owner)),
    ] {
        if export_bytes(&document["accounts"][name])? != id {
            return Err(());
        }
    }
    let join = &proofs["account_join"];
    if join.as_object().ok_or(())?.len() != 6 || &join["finality"] != finality {
        return Err(());
    }
    let row = &join["account"];
    if row["bound"] != true
        || row["frozen"] != false
        || export_bytes(&row["layerx_account"])? != record.owner
        || export_bytes(&row["asset_id"])? != record.asset
    {
        return Err(());
    }
    let owner_key = [vec![6], record.owner.to_vec()].concat();
    let owner_value = verify_native_export_proof(
        &join["native_account_proof"],
        &owner_key,
        state_height,
        app_hash,
    )?;
    let sequence = verify_native_export_proof(
        &join["receipt_sequence_proof"],
        &[2],
        state_height,
        app_hash,
    )?;
    let sequence = u64::from_be_bytes(sequence.try_into().map_err(|_| ())?);
    if sequence == 0 || state["observed_head_sequence"] != sequence {
        return Err(());
    }
    let binding = &join["binding_call"];
    let data = export_bytes(&binding["data"])?;
    let result = export_bytes(&binding["result"])?;
    if binding.as_object().ok_or(())?.len() != 4
        || binding["height"] != state_height
        || export_bytes(&binding["to"])?
            != hex::decode("0000000000000000000000000000000000001004").map_err(|_| ())?
        || data.len() != 36
        || data[..4]
            != layerx_crypto::evm_transaction::transaction_hash(b"getEvmAddrByLayerX(bytes32)")[..4]
        || result.len() != 32
        || result[..12] != [0; 12]
        || result[12..] == [0; 20]
        || row["evm_address"] != format!("0x{}", hex::encode(&result[12..]))
    {
        return Err(());
    }
    let did = format!("did:layerx:{}", hex::encode(&data[4..]));
    let identity = layerx_wire::hash::did_id_for_protocol(
        &layerx_types::ids::Did::new(did.as_bytes()).map_err(|_| ())?,
        3,
    )
    .map_err(|_| ())?;
    if export_bytes(&document["identity_key"])? != identity {
        return Err(());
    }
    let balances = join["balances"].as_array().ok_or(())?;
    let mut ids = vec![record.owner, record.account];
    if let Some(source) = record.source {
        ids.push(source);
    }
    if balances.len() != ids.len() {
        return Err(());
    }
    for (balance, id) in balances.iter().zip(ids) {
        if export_bytes(&balance["account_id"])? != id
            || export_bytes(&balance["asset_id"])? != record.asset
        {
            return Err(());
        }
        let value = verify_native_export_proof(
            &balance["proof"],
            &[vec![6], id.to_vec()].concat(),
            state_height,
            app_hash,
        )?;
        if id == record.owner && value != owner_value {
            return Err(());
        }
        let mut reader = layerx_wire::decode::Decoder::new(&value, 2048);
        if reader.bytes(32).map_err(|_| ())? != id {
            return Err(());
        }
        let name = std::str::from_utf8(reader.bytes(512).map_err(|_| ())?).map_err(|_| ())?;
        if id == record.owner && name != format!("agent:{did}:main") {
            return Err(());
        }
        let kind = reader.u8().map_err(|_| ())?;
        if kind == 0 {
            return Err(());
        }
        if balance["amount"] != reader.u128().map_err(|_| ())?.to_string()
            || reader.bytes(32).map_err(|_| ())? != record.asset
        {
            return Err(());
        }
        reader.u8().map_err(|_| ())?;
        reader.u64().map_err(|_| ())?;
        reader.u64().map_err(|_| ())?;
        reader.u8().map_err(|_| ())?;
        if reader.u8().map_err(|_| ())? != 0 {
            return Err(());
        }
        if reader.bytes(32).map_err(|_| ())?.len() != 32 {
            return Err(());
        }
        reader.u8().map_err(|_| ())?;
        reader.finish().map_err(|_| ())?;
    }
    let mut evidence = Sha256::new();
    evidence.update(b"layerx-human/rpc-budget-evidence/v2\0");
    evidence.update(block_hash);
    let join = serde_json::to_vec(&proofs["account_join"]).map_err(|_| ())?;
    for material in [&budget, &remaining, &join] {
        evidence.update(u64::try_from(material.len()).map_err(|_| ())?.to_be_bytes());
        evidence.update(material);
    }
    let digest: [u8; 32] = evidence.finalize().into();
    if export_bytes(&state["evidence_digest"])? != digest
        || export_bytes(&state["receipt_digest"])? != Sha256::digest(&budget).as_slice()
    {
        return Err(());
    }
    Ok(())
}

pub(crate) fn assert_export_corpus(fixture: &Value) {
    let cases = fixture["cases"].as_array().expect("real export cases");
    assert!(!cases.is_empty());
    for case in cases {
        let document: Value = serde_json::from_slice(&protected_export(
            Path::new(case["response"].as_str().expect("response path")),
            1_048_576,
        ))
        .expect("real export JSON");
        let finality = serde_json::json!({
            "height":case["committed_height"], "block_hash":case["block_hash"], "app_hash":case["app_hash"], "state_height":case["state_height"]
        });
        assert!(verify_committed_export(&document, &finality).is_ok());
        assert_eq!(document["budget_state"]["budget_id"], case["budget_id"]);
        assert_eq!(document["budget_state"]["asset"], case["asset_id"]);
        assert_eq!(document["accounts"]["owner"], case["owner_account"]);
        assert_eq!(document["finality"]["block_hash"], case["block_hash"]);
        assert_eq!(document["finality"]["app_hash"], case["app_hash"]);
        for name in ["budget_call", "remaining_call"] {
            let mut corrupt = document.clone();
            let mut raw = export_bytes(&corrupt["proofs"][name]["result"]).expect("call bytes");
            *raw.last_mut().expect("nonempty call") ^= 1;
            corrupt["proofs"][name]["result"] = serde_json::json!(hex::encode(&raw));
            assert!(
                verify_committed_export(&corrupt, &finality).is_err(),
                "tampered {name}"
            );
        }
        for name in ["budget_call", "remaining_call"] {
            for field in ["to", "data", "height"] {
                let mut corrupt = document.clone();
                corrupt["proofs"][name][field] = serde_json::json!(0);
                assert!(
                    verify_committed_export(&corrupt, &finality).is_err(),
                    "tampered {name}.{field}"
                );
            }
        }
        let mut corrupt = document.clone();
        corrupt["finality"]["block_hash"] = serde_json::json!(hex::encode(&[0xaa; 32]));
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        let mut corrupt = document.clone();
        corrupt["finality"]["height"] = serde_json::json!(0);
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        let mut corrupt = document.clone();
        corrupt["finality"]["app_hash"] = serde_json::json!(hex::encode(&[0xaa; 32]));
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        let mut corrupt = document.clone();
        corrupt["proofs"]["account_join"]["layerx"]["accounts"] = serde_json::json!([]);
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        let mut corrupt = document.clone();
        corrupt["head"]["committed_height"] = serde_json::json!(0);
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        for field in ["key", "value", "height", "proofOps"] {
            let mut corrupt = document.clone();
            corrupt["proofs"]["account_join"]["native_account_proof"][field] = document!(null);
            // Recompute the envelope digest: the authenticated membership check
            // must reject this independently of the export checksum.
            let mut digest = Sha256::new();
            digest.update(b"layerx-human/rpc-budget-evidence/v2\0");
            digest.update(export_bytes(&corrupt["finality"]["block_hash"]).unwrap());
            for material in [
                export_bytes(&corrupt["proofs"]["budget_call"]["result"]).unwrap(),
                export_bytes(&corrupt["proofs"]["remaining_call"]["result"]).unwrap(),
                serde_json::to_vec(&corrupt["proofs"]["account_join"]).unwrap(),
            ] {
                digest.update((material.len() as u64).to_be_bytes());
                digest.update(material);
            }
            corrupt["budget_state"]["evidence_digest"] = document!(hex::encode(&digest.finalize()));
            assert!(
                verify_committed_export(&corrupt, &finality).is_err(),
                "accepted native {field}"
            );
        }
    }
}

#[test]
pub(crate) fn budget_export_corpus_optional() {
    let corpus = produced_export_corpus();
    let fixture: Value = fixture_must(serde_json::from_slice(&protected_export(
        &corpus.path,
        65_536,
    )));
    assert!(corpus.directory.0.join("export.json").is_file());
    assert_export_corpus(&fixture);
}

#[test]
pub(crate) fn human_tls_retired_anchor() {
    for capability in [false, true] {
        let fixture = Arc::new(identity_fixture(capability));
        let node = Node::start(fixture.clone());
        let directory = Directory::new();
        let config = config_fixture(&node, &fixture, &directory.0);
        retain_records(&config, &fixture.records);
        assert_eq!(
            fixture_must(fs::read_dir(directory.0.join("state"))).count(),
            1
        );
        drop(config);
        // Reopen the real authority evidence store before driving any route.
        let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
        let policy = fixture.policy();
        let identity = &policy["identities"][0];
        let base = format!("{}&did={}", human_path("identity"), fixture.did);
        let mut paths = vec![base];
        if capability {
            let scope = &identity["capabilities"][0];
            paths.push(format!(
                "{}&did={}&authority={}&action_key={}&capability_id={}",
                human_path("capability-scope"),
                fixture.did,
                scope["authority"].as_str().unwrap(),
                scope["action_key"].as_str().unwrap(),
                scope["capability_id"].as_str().unwrap()
            ));
        } else {
            paths.push(format!(
                "{}&did={}&recovery=false",
                human_path("key-policy"),
                fixture.did
            ));
        }
        for path in &paths {
            assert_eq!(tls_request(config.clone(), path, "wrong-token").0, 401);
            let (status, answer) = tls_request(config.clone(), path, "repair-human-token-208");
            assert_eq!(status, 200, "{path}: {answer}");
            assert_eq!(answer["finality"]["height"], 9);
            assert_eq!(answer["finality"]["state_height"], 8);
            assert_eq!(answer["finality"]["block_hash"], hex::encode(&[9; 32]));
            assert_eq!(
                answer["finality"]["app_hash"],
                hex::encode(&fixture.store.app_hash)
            );
            if path.contains("/identity?") {
                assert_eq!(answer["head_sequence"], fixture.sequence);
                assert_eq!(
                    answer["authorities"].as_array().unwrap().len(),
                    if capability { 2 } else { 1 }
                );
            } else {
                assert_eq!(answer["checkpoint_digest"], hex::encode(&[9; 32]));
                assert_eq!(answer["verification"], 4);
                if capability {
                    assert_eq!(answer["activity_types"], document!([3]));
                    assert_eq!(
                        answer["capability_id"],
                        identity["capabilities"][0]["capability_id"]
                    );
                    assert_eq!(answer["observed_sequence"], fixture.sequence);
                } else {
                    assert_eq!(
                        answer["policy_revision"],
                        identity["rotation"]["policy_revision"]
                    );
                    assert_eq!(answer["observed_head_sequence"], fixture.sequence);
                }
            }
        }
        for fault in [
            "apphash",
            "proof",
            "value",
            "key",
            "height",
            "store",
            "landing",
            "uncommitted",
            "receipt-height",
            "key-id",
            "signing-key",
        ] {
            node.fault(fault);
            for path in &paths {
                let (status, answer) = tls_request(config.clone(), path, "repair-human-token-208");
                assert_eq!(status, 503, "accepted {fault}: {answer}");
                assert_eq!(
                    answer["error"]["code"],
                    match fault {
                        "apphash" => "committed_header_unavailable",
                        "proof" | "value" | "key" | "height" | "store" =>
                            "identity_state_proof_unavailable",
                        _ => "committed_receipt_evidence_unavailable",
                    }
                );
            }
        }
        assert_no_retired_calls(&node);
    }
}

#[test]
pub(crate) fn human_budget_corpus_optional() {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let state_path = budget_path("budget-state", &fixture);
    let export_path = budget_path("budget-proof", &fixture);
    let (status, state) = tls_request(config.clone(), &state_path, "repair-human-token-208");
    assert_eq!(status, 200, "{state}");
    let (status, export) = tls_request(config.clone(), &export_path, "repair-human-token-208");
    assert_eq!(status, 200, "{export}");
    assert_eq!(export["budget_state"], state);
    assert_eq!(state["remaining"], "77");
    assert_eq!(state["observed_head_sequence"], 31);
    assert_eq!(export["schema"], "layerx.human.budget-proof.v2");
    assert_eq!(export["accounts"]["owner"], hex::encode(&fixture.owner));
    assert_eq!(export["accounts"]["budget"], hex::encode(&fixture.budget));
    assert!(verify_committed_export(&export, &export["finality"]).is_ok());
    assert!(export.get("lni_interface_version").is_none());
    assert!(!directory.0.join("absent.sock").exists());
    // A separate replay requires the corpus just produced through TLS. No
    // environment-variable branch can turn absence into a successful test.
    let replay_corpus = produced_export_corpus();
    let corpus: Value = fixture_must(serde_json::from_slice(&protected_export(
        &replay_corpus.path,
        65_536,
    )));
    let cases = corpus["cases"].as_array().expect("required replay cases");
    assert!(!cases.is_empty());
    for case in cases {
        let exchanges = case["rpc_exchanges"]
            .as_array()
            .expect("required captured RPC exchanges")
            .clone();
        assert!(!exchanges.is_empty());
        let replay = Node::with_transcript(fixture.clone(), Some(exchanges));
        let expected: Value = fixture_must(serde_json::from_slice(&protected_export(
            Path::new(case["response"].as_str().unwrap()),
            1_048_576,
        )));
        let replay_directory = Directory::new();
        let replay_config = Arc::new(config_fixture(&replay, &fixture, &replay_directory.0));
        let (status, state) =
            tls_request(replay_config.clone(), &state_path, "repair-human-token-208");
        assert_eq!(status, 200, "{state}");
        let (status, export) = tls_request(replay_config, &export_path, "repair-human-token-208");
        assert_eq!(status, 200, "{export}");
        assert_eq!(state, expected["budget_state"]);
        assert_eq!(export["budget_state"], state);
        assert_eq!(export["schema"], "layerx.human.budget-proof.v2");
        for field in ["head", "finality", "proofs", "accounts", "identity_key"] {
            assert_eq!(export[field], expected[field], "{field}");
        }
        assert!(export.get("lni_interface_version").is_none());
        assert!(!replay_directory.0.join("absent.sock").exists());
        assert_no_retired_calls(&replay);
    }
    for fault in [
        "proof",
        "value",
        "key",
        "height",
        "store",
        "apphash",
        "unbound",
        "balance-proof",
        "ledger-mismatch",
        "ledger-overflow",
        "ledger-truncated",
        "reorg",
    ] {
        node.fault(fault);
        for path in [&state_path, &export_path] {
            let (status, answer) = tls_request(config.clone(), path, "repair-human-token-208");
            assert_eq!(status, 503, "accepted {fault}: {answer}");
        }
    }
    assert_no_retired_calls(&node);
}

#[test]
fn retirement_guard_requires_anchor() {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let (status, export) = tls_request(
        config,
        &budget_path("budget-proof", &fixture),
        "repair-human-token-208",
    );
    assert_eq!(status, 200, "{export}");
    assert!(verify_committed_export(&export, &export["finality"]).is_ok());
    assert_no_retired_calls(&node);
}
