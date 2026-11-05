use std::collections::BTreeMap;

use layerx_agent_api::error::RequestId;
use layerx_agentd::agent_rpc_adapters::program_call_answer;
use layerx_agentd::human_runtime::{accept_raw_write, write_submission};
use layerx_agentd::ops::program::ProgramOperations;
use layerx_agentd::outbox::{Outbox, SubmissionState};
use layerx_agentd::prepare::evm::PrecompileTarget;
use layerx_agentd::store::{Store, TenantId};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_types::intent::ProgramId;
use layerx_types::program_call::{NativeProgramCall, Resources};
use layerx_types::rung::Rung;

#[test]
fn program_call_returns_tx_hash_and_rung() {
    let call = NativeProgramCall {
        program_id: ProgramId::new([0x31; 32]),
        guest_abi: 1,
        entrypoint: b"layerx_call",
        calldata: &[1, 2, 3],
        capabilities: &[0, 0],
        access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
        response_capacity: 1024,
        resources: Resources([1_000_000, 16_777_216, 100, 200, 4, 1024, 4096]),
    };
    let precompile = ProgramOperations::<SequencerEndpoint>::call_precompile(call, 987).unwrap();
    assert_eq!(
        precompile.target.address(),
        PrecompileTarget::Programs.address()
    );
    let signature =
        transaction_hash(b"call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)");
    let data = precompile.encode().unwrap();
    assert_eq!(data[..4], signature[..4]);
    let transaction = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 1_000_000,
        to: precompile.target.address(),
        data,
    };
    let raw = transaction.sign(&[0xa5; 32]).unwrap();
    let hash = transaction_hash(&raw);
    let root = std::env::temp_dir().join(format!("program-adapters-rungs-{}", std::process::id()));
    std::fs::create_dir(&root).unwrap();
    let mut store = Store::open(&root).unwrap();
    let tenant = TenantId::new("program-adapters").unwrap();
    let mut outbox = Outbox::default();
    assert_eq!(
        outbox
            .enqueue(&mut store, tenant.clone(), raw.clone())
            .unwrap(),
        hash
    );
    assert!(accept_raw_write(&mut outbox, &mut store, hash, [9; 32]).is_err());
    let status = accept_raw_write(&mut outbox, &mut store, hash, hash).unwrap();
    assert_eq!(status.state, SubmissionState::Submitted);
    let answer = || write_submission(&status, None).unwrap();
    let (value, rung) = program_call_answer(RequestId(1), &raw, answer()).unwrap();
    assert_eq!(value["tx_hash"], hex::encode(hash));
    assert_eq!(value["submission_ref"], hex::encode(hash));
    assert_eq!(value["state"], "Submitted");
    assert_eq!(value["rung"], "submitted");
    assert_eq!(rung, Rung::Submitted);
    let mut wrong = transaction.clone();
    wrong.to = PrecompileTarget::Ledger.address();
    assert!(
        program_call_answer(RequestId(2), &wrong.sign(&[0xa5; 32]).unwrap(), answer()).is_err()
    );
    wrong.to = transaction.to;
    wrong.nonce += 1;
    assert!(
        program_call_answer(RequestId(3), &wrong.sign(&[0xa5; 32]).unwrap(), answer()).is_err()
    );
    assert!(program_call_answer(RequestId(4), &raw[..raw.len() - 1], answer()).is_err());
    let mut restored = Outbox::default();
    restored.restore(&store, tenant, hash).unwrap();
    assert_eq!(restored.exact_signed_bytes(hash).unwrap(), raw);
    let (tracked, tracked_rung) = program_call_answer(
        RequestId(5),
        &raw,
        write_submission(restored.status(hash).unwrap(), None).unwrap(),
    )
    .unwrap();
    assert_eq!(tracked, value);
    assert_eq!(tracked_rung, rung);
    drop(store);
    std::fs::remove_dir_all(root).unwrap();
}

fn fields<'a>(schema: &'a str, name: &str) -> BTreeMap<&'a str, &'a str> {
    let heading = format!("[type.{name}]");
    let section = schema
        .split_once(&heading)
        .unwrap()
        .1
        .split("\n[")
        .next()
        .unwrap();
    section
        .lines()
        .filter_map(|line| line.split_once(" = "))
        .collect()
}

#[test]
fn programs_schema_names_no_kernel_protocol() {
    let schema = include_str!("../../../schema/agent-api/programs.kvx");
    let golden = include_str!("../../../schema/agent-api/golden/programs.kvx");
    assert_eq!(schema, golden);
    for text in [schema, golden] {
        let lower = text.to_ascii_lowercase();
        for obsolete in [
            "protocol3",
            "protocol_version",
            "module_id",
            "ordinal",
            "sequencer",
            "kernel",
            "signed_discovery",
        ] {
            assert!(
                !lower.contains(obsolete),
                "obsolete schema authority: {obsolete}"
            );
        }
        assert!(text.contains("layerxprograms 0x101D"));
        assert!(text.contains("EIP-1559"));
        assert!(text.contains("submitted, receipted, committed rungs"));
        for (name, selectors) in [
            (
                "NativeProgramDeployRequest",
                vec![("selector", "\"0xbde3ff66\"")],
            ),
            (
                "NativeProgramUpgradeRequest",
                vec![("selector", "\"0xa860929f\"")],
            ),
            (
                "NativeProgramWindDownRequest",
                vec![
                    ("selector_route", "\"0x23330dd5\""),
                    ("selector_deprecate", "\"0x86ba1a99\""),
                    ("selector_tombstone", "\"0xa512f1a9\""),
                    ("selector_exit", "\"0x4dc50730\""),
                ],
            ),
        ] {
            let request = fields(text, name);
            assert_eq!(
                request["precompile"],
                "\"0x000000000000000000000000000000000000101d\""
            );
            assert_eq!(request["required"], "[\"signed_activity\"]");
            for key in ["signed_activity", "payload", "identity_binding"] {
                assert!(!request[key].is_empty());
            }
            for (key, value) in selectors {
                assert_eq!(request[key], value);
            }
            if name == "NativeProgramWindDownRequest" {
                assert!(!request.contains_key("selector"));
            }
        }
        let submission = fields(text, "ProgramSubmission");
        assert_eq!(
            submission["rung"],
            "[\"submitted\",\"receipted\",\"committed\"]"
        );
        assert!(submission["required"].contains("tx_hash"));
        assert_eq!(
            fields(text, "NativeProgramWindDown")["variants"],
            "[\"route\",\"deprecate\",\"tombstone\",\"exit\"]"
        );
    }
}
