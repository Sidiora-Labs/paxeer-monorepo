use super::{
    directory, must, rpc_hex, sequencer_receipt_with_details, serve, serve_node, signed_transaction,
};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::boot::{Gate, Status};
use layerx_agentd::budget::ProtocolBudgetState;
use layerx_agentd::config::{load, StartupConfig};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::protocol_evidence::{EvidenceAuthority, RawStateEvidence};
use layerx_agentd::read::{
    checkpoint, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp,
    CommitValidator, CommitVote, CommittedReads, ServedCheckpoint,
};
use layerx_client::rpc::receipt::{
    lookup_authenticated, AuthenticatedReceipt, Lookup, LookupContext,
};
use serde_json::{json, Value};
use std::collections::BTreeMap;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;

pub fn keys(seed: u8, first: u64, last: u64) -> Value {
    json!({"keys": [{"keyId": rpc_hex(&[1; 32]),
        "publicKey": rpc_hex(SigningKey::from_bytes(&[seed; 32]).verifying_key().as_bytes()),
        "firstHeight": first, "lastHeight": last}]})
}

fn loaded_config(
    root: &Path,
    node: &NodeRpcConfig,
    key: layerx_client::sequencer::ReceiptKeyEntry,
) -> StartupConfig {
    let configured = super::node_transcripts::startup(root, node, key);
    let layerx_agentd::config::ReceiptKeySource::PinnedFile(authority) =
        configured.receipt_key_source
    else {
        panic!("protected authority file");
    };
    let path = root.join("startup.kv");
    let source = format!("network_id=125\nnode_endpoint={}\nexpected_chain_id=125\ntenants=tenant-a\npolicy_sources=tenant-a:{}\nsigner_configurations=tenant-a:{}\nverification_defaults=tenant-a:checkpoint-finalised\nreceipt_key_source={}\n",
        node.url, root.join("policy.kvx").display(), root.join("signer.kvx").display(), authority.display());
    must(std::fs::write(&path, source));
    must(std::fs::set_permissions(
        &path,
        std::fs::Permissions::from_mode(0o600),
    ));
    must(load(&path, &BTreeMap::new()))
}

pub fn authority(seed: u8, first: u64, last: u64) -> EvidenceAuthority {
    let reply = keys(seed, first, last);
    let key = must(layerx_client::sequencer::decode_receipt_keys(&reply))[0];
    let (config, server) = serve_node(vec![
        ("eth_chainId", json!([]), Some(json!("0x7d"))),
        ("px_getReceiptKeys", json!([]), Some(reply)),
    ]);
    let root = directory("current-authority");
    must(std::fs::create_dir(&root));
    let startup = loaded_config(&root, &config, key);
    let node = must(NodeRpc::connect(&config));
    let authority = super::real_authority::node_authority(&startup, &node);
    must(server.join());
    must(std::fs::remove_dir_all(root));
    authority
}

pub fn startup_restart() {
    let reply = keys(8, 100, 199);
    let key = must(layerx_client::sequencer::decode_receipt_keys(&reply))[0];
    let mut exchanges = Vec::new();
    for answer in [
        reply.clone(),
        reply,
        json!({"keys": []}),
        json!({"keys": [{}]}),
    ] {
        exchanges.extend([
            ("eth_chainId", json!([]), Some(json!("0x7d"))),
            ("px_getReceiptKeys", json!([]), Some(answer)),
        ]);
    }
    let (config, server) = serve_node(exchanges);
    let root = directory("current-startup");
    must(std::fs::create_dir(&root));
    let configured = loaded_config(&root, &config, key);
    for generation in 0..4 {
        let loaded = must(load(root.join("startup.kv"), &BTreeMap::new()));
        assert_eq!(loaded, configured);
        let mut gate = must(Gate::new(&loaded));
        assert_eq!(gate.status(), &Status::NotReady);
        assert!(gate.guard_write(|| ()).is_err());
        let node = must(NodeRpc::connect(&NodeRpcConfig {
            url: loaded.node_endpoint.clone(),
            chain_id: loaded.expected_chain_id,
            deadline: std::time::Duration::from_secs(2),
        }));
        let ready = gate.readiness_gate(&node);
        if generation < 2 {
            assert_eq!(must(ready).receipt_keys, vec![key]);
            assert!(gate.evidence_authority().is_ok());
            assert_eq!(gate.guard_write(|| ()), Ok(()));
            gate.disconnected();
        } else if generation == 2 {
            assert_eq!(ready, Err(layerx_agentd::boot::GateError::NoReceiptKeys));
        } else {
            assert_eq!(
                ready,
                Err(layerx_agentd::boot::GateError::NodeRpc(
                    layerx_agentd::node_rpc::NodeRpcError::Sequencer(
                        layerx_client::sequencer::SequencerError::Malformed
                    )
                ))
            );
        }
        assert_eq!(gate.status(), &Status::NotReady);
        assert!(gate.guard_write(|| ()).is_err());
        assert!(gate.evidence_authority().is_err());
    }
    must(server.join());
    must(std::fs::remove_dir_all(root));
}

pub fn authenticated_receipt(id: u8) -> AuthenticatedReceipt {
    let raw = signed_transaction(id);
    let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
    let (view, key) = sequencer_receipt_with_details(&raw, true, 8, 25, 100);
    let (endpoint, server) = serve(vec![
        ("px_getReceiptKeys", json!([]), Some(keys(8, 100, 199))),
        ("pax_sequencerReceipt", json!([rpc_hex(&hash)]), Some(view)),
        (
            "eth_getTransactionReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);
    let keys = must(endpoint.receipt_keys());
    assert_eq!(keys, vec![key]);
    let Lookup::Verified(receipt) = must(lookup_authenticated(
        &endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )) else {
        panic!("authenticated receipt required");
    };
    assert_eq!(
        receipt
            .receipt()
            .protocol()
            .expect("protocol receipt")
            .amount(),
        25
    );
    must(server.join());
    receipt
}

pub fn committed_budget(
    budget_id: [u8; 32],
    spent: u128,
) -> (ProtocolBudgetState, ServedCheckpoint) {
    committed_budget_with_period(
        budget_id,
        spent,
        80,
        1_000,
        signed_checkpoint(150, 1_700_000_000, 123_000_000),
    )
}

pub fn committed_budget_with_period(
    budget_id: [u8; 32],
    spent: u128,
    period_start_ms: u64,
    period_length_ms: u64,
    checkpoint: ServedCheckpoint,
) -> (ProtocolBudgetState, ServedCheckpoint) {
    let mut result = vec![0; 20 * 32];
    for (index, amount) in [
        (0, 32),
        (5, 1000),
        (6, 1000),
        (8, spent),
        (10, u128::from(period_length_ms)),
        (11, u128::from(period_start_ms)),
        (
            12,
            u128::from(
                period_start_ms
                    .checked_add(period_length_ms)
                    .expect("budget expiry fits milliseconds")
                    .max(5_000),
            ),
        ),
        (14, 1),
        (17, 576),
    ] {
        result[index * 32 + 16..(index + 1) * 32].copy_from_slice(&amount.to_be_bytes());
    }
    for index in 1..=4 {
        result[index * 32..(index + 1) * 32].fill(index as u8);
    }
    let mut pay = [0; 20];
    pay[18..].copy_from_slice(&[0x10, 0x1b]);
    let mut calldata =
        layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
    calldata.extend(budget_id);
    let params = json!([{"to": rpc_hex(&pay), "data": rpc_hex(&calldata)}, format!("{:#x}", checkpoint.height)]);
    let block = json!({"number":format!("{:#x}", checkpoint.height), "hash":rpc_hex(&checkpoint.block_hash),
        "parentHash":rpc_hex(&checkpoint.header.last_block_id.expect("parent").hash),
        "timestamp":format!("{:#x}", checkpoint.header.time.seconds), "transactionsRoot":rpc_hex(&[4;32]),
        "receiptsRoot":rpc_hex(&[7;32]), "stateRoot":rpc_hex(&checkpoint.app_hash), "transactions":[]});
    let (endpoint, server) = serve(vec![
        ("eth_call", params.clone(), Some(json!(rpc_hex(&result)))),
        ("eth_call", params, Some(json!(rpc_hex(&result)))),
        (
            "eth_getBlockByNumber",
            json!([format!("{:#x}", checkpoint.height), true]),
            Some(block.clone()),
        ),
        (
            "eth_getBlockByNumber",
            json!([format!("{:#x}", checkpoint.height), false]),
            Some(block),
        ),
    ]);
    let reads = must(CommittedReads::at_checkpoint(&endpoint, &checkpoint));
    let call = must(reads.budget_state(pay, &calldata));
    let evidence = must(RawStateEvidence::budget_witness(call, budget_id));
    must(server.join());
    (ProtocolBudgetState { evidence }, checkpoint)
}

pub fn signed_checkpoint(height: u64, seconds: i64, nanos: u32) -> ServedCheckpoint {
    let keys: Vec<_> = [1, 2, 3]
        .map(|seed| SigningKey::from_bytes(&[seed; 32]))
        .into();
    let validators: Vec<_> = keys
        .iter()
        .zip([4, 3, 2])
        .map(|(key, voting_power)| {
            let public_key = key.verifying_key().to_bytes();
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power,
            }
        })
        .collect();
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".into(),
        height,
        time: CommitTimestamp { seconds, nanos },
        last_block_id: Some(BlockId {
            hash: [1; 32],
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: validator_set_hash(&validators),
        next_validators_hash: validator_set_hash(&validators),
        consensus_hash: [5; 32],
        app_hash: [6; 32],
        last_results_hash: Some([7; 32]),
        evidence_hash: Some([8; 32]),
        proposer_address: validators[0].address,
    };
    let mut commit = BlockCommit {
        height,
        round: 1,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [9; 32],
        },
        votes: Vec::new(),
    };
    for (key, validator) in keys.iter().zip(&validators).take(2) {
        let timestamp = CommitTimestamp { seconds, nanos };
        commit.votes.push(CommitVote {
            validator_address: validator.address,
            voting_power: validator.voting_power,
            timestamp,
            signature: key
                .sign(&commit.vote_sign_bytes(&header.chain_id, timestamp))
                .to_bytes(),
        });
    }
    checkpoint(height, "125", &header, &commit, &validators).expect("signed block commit")
}
