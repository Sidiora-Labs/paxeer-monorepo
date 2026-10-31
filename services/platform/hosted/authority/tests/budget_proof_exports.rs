use layerx_platform_authority::hex;
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;

#[path = "../src/human/budget_state/record.rs"]
mod record;

fn protected(path: &Path, maximum: u64) -> Vec<u8> {
    let metadata = fs::symlink_metadata(path).expect("real corpus metadata");
    assert!(metadata.is_file() && metadata.len() > 0 && metadata.len() <= maximum);
    assert_eq!(metadata.permissions().mode() & 0o077, 0);
    fs::read(path).expect("real protected corpus")
}

fn bytes(value: &Value) -> Result<Vec<u8>, ()> {
    let text = value.as_str().ok_or(())?;
    let decoded = hex::decode(text).map_err(|_| ())?;
    if hex::encode(&decoded) != text {
        return Err(());
    }
    Ok(decoded)
}

fn verify_export(document: &Value) -> Result<(), ()> {
    use record::{number, word, Record};
    if document["schema"] != "layerx.human.budget-proof.v2"
        || document["protocol_version"] != 3
        || document.get("lni_interface_version").is_some()
        || document.get("authorization").is_some()
    {
        return Err(());
    }
    let state = &document["budget_state"];
    let proofs = &document["proofs"];
    if proofs.as_object().ok_or(())?.len() != 4 {
        return Err(());
    }
    let budget = bytes(&proofs["budget_call"])?;
    let remaining = bytes(&proofs["remaining_call"])?;
    let checkpoint = bytes(&proofs["checkpoint_call"])?;
    let record = Record::from_call(
        bytes(&state["budget_id"])?.try_into().map_err(|_| ())?,
        &budget,
    )?;
    if bytes(&state["canonical_core_bytes"])? != budget
        || bytes(&state["asset"])? != record.asset
        || state["revocation_sequence"] != record.revocation
        || state["closed"] != record.closed
        || state["revoked"] != record.revoked
        || remaining.len() != 32
        || state["remaining"] != number(&remaining, 0, 16)?.to_string()
        || checkpoint.len() != 18 * 32
        || number(&checkpoint, 12, 1)? != 2
    {
        return Err(());
    }
    let head = &document["head"];
    let finality = &document["finality"];
    if head.as_object().ok_or(())?.len() != 3 || finality.as_object().ok_or(())?.len() != 5 {
        return Err(());
    }
    let batch = u64::try_from(number(&checkpoint, 0, 8)?).map_err(|_| ())?;
    let finalized = u64::try_from(number(&checkpoint, 17, 8)?).map_err(|_| ())?;
    let committed = head["committed_height"].as_u64().ok_or(())?;
    let checkpoint_id = word(&checkpoint, 1)?;
    if finality["status"] != 2
        || finality["batch_number"] != batch
        || head["batch_number"] != batch
        || finality["finalized_height"] != finalized
        || finalized > committed
        || word(&checkpoint, 7)? == [0; 32]
        || checkpoint_id == [0; 32]
        || bytes(&finality["state_root"])? != word(&checkpoint, 7)?
        || bytes(&finality["checkpoint_id"])? != checkpoint_id
        || bytes(&head["checkpoint_id"])? != checkpoint_id
        || bytes(&state["checkpoint_digest"])? != checkpoint_id
        || state["observed_head_sequence"] != committed
    {
        return Err(());
    }
    for (name, id) in [
        ("owner", record.owner),
        ("budget", record.account),
        ("source", record.source.unwrap_or(record.owner)),
    ] {
        if bytes(&document["accounts"][name])? != id {
            return Err(());
        }
    }
    if !proofs["account_join"]["layerx"]["accounts"]
        .as_array()
        .ok_or(())?
        .iter()
        .any(|row| {
            row["bound"] == true
                && bytes(&row["layerx_account"]).is_ok_and(|id| id == record.owner)
                && row["evm_address"]
                    .as_str()
                    .and_then(|s| s.strip_prefix("0x"))
                    .and_then(|s| hex::decode(s).ok())
                    .is_some_and(|id| id.len() == 20 && id != [0; 20])
        })
    {
        return Err(());
    }
    let mut evidence = Sha256::new();
    evidence.update(b"layerx-human/rpc-budget-evidence/v2\0");
    evidence.update(checkpoint_id);
    let join = serde_json::to_vec(&proofs["account_join"]).map_err(|_| ())?;
    for material in [&budget, &remaining, &checkpoint, &join] {
        evidence.update(u64::try_from(material.len()).map_err(|_| ())?.to_be_bytes());
        evidence.update(material);
    }
    let digest: [u8; 32] = evidence.finalize().into();
    if bytes(&state["evidence_digest"])? != digest {
        return Err(());
    }
    Ok(())
}

#[test]
fn real_budget_exports_bind_each_selector_finality_and_tamper_refusals() {
    let Some(fixture) = std::env::var_os("PAXEER_X_BUDGET_PROOF_CORPUS") else {
        return;
    };
    let fixture: Value =
        serde_json::from_slice(&protected(Path::new(&fixture), 65_536)).expect("corpus JSON");
    let cases = fixture["cases"].as_array().expect("real export cases");
    assert!(!cases.is_empty());
    for case in cases {
        let document: Value = serde_json::from_slice(&protected(
            Path::new(case["response"].as_str().expect("response path")),
            1_048_576,
        ))
        .expect("real export JSON");
        assert!(verify_export(&document).is_ok());
        assert_eq!(document["budget_state"]["budget_id"], case["budget_id"]);
        assert_eq!(document["budget_state"]["asset"], case["asset_id"]);
        assert_eq!(document["accounts"]["owner"], case["owner_account"]);
        assert_eq!(document["finality"]["checkpoint_id"], case["checkpoint_id"]);
        for name in ["budget_call", "remaining_call", "checkpoint_call"] {
            let mut corrupt = document.clone();
            let mut raw = bytes(&corrupt["proofs"][name]).expect("call bytes");
            *raw.last_mut().expect("nonempty call") ^= 1;
            corrupt["proofs"][name] = serde_json::json!(hex::encode(&raw));
            assert!(verify_export(&corrupt).is_err(), "tampered {name}");
        }
        let mut corrupt = document.clone();
        corrupt["proofs"]["account_join"]["layerx"]["accounts"] = serde_json::json!([]);
        assert!(verify_export(&corrupt).is_err());
        let mut corrupt = document.clone();
        corrupt["head"]["committed_height"] = serde_json::json!(0);
        assert!(verify_export(&corrupt).is_err());
    }
}

#[test]
fn budget_abi_matches_node_precompile_types() {
    let abi: Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxpay/abi.json"
    ))
    .expect("node ABI");
    let get = abi
        .as_array()
        .expect("ABI")
        .iter()
        .find(|row| row["name"] == "getBudget")
        .expect("getBudget");
    assert_eq!(get["inputs"][0]["type"], "bytes32");
    let fields = get["outputs"][0]["components"].as_array().expect("tuple");
    assert_eq!(fields.len(), 18);
    assert_eq!(fields[16]["type"], "bytes32[]");
    assert_eq!(fields[17]["name"], "sourceAccount");
}
