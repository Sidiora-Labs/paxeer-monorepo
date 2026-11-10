fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use super::*;
use ed25519_dalek::{Signer, SigningKey};
use layerx_agentd::read::{
    BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator,
};
use layerx_proof::merkle::Proof;
use layerx_wire::encode::Encoder;

pub(super) type NativeCapture = (
    BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
    Vec<u8>,
);

fn base64(bytes: &[u8]) -> String {
    const LETTERS: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for group in bytes.chunks(3) {
        let n = (u32::from(group[0]) << 16)
            | (u32::from(*group.get(1).unwrap_or(&0)) << 8)
            | u32::from(*group.get(2).unwrap_or(&0));
        out.push(LETTERS[(n >> 18) as usize] as char);
        out.push(LETTERS[((n >> 12) & 63) as usize] as char);
        out.push(if group.len() > 1 {
            LETTERS[((n >> 6) & 63) as usize] as char
        } else {
            '='
        });
        out.push(if group.len() > 2 {
            LETTERS[(n & 63) as usize] as char
        } else {
            '='
        });
    }
    out
}

fn checkpoint_answers(answers: &mut Vec<Value>, height: u64, root: [u8; 32]) -> [u8; 32] {
    let key = SigningKey::from_bytes(&[1; 32]);
    let public_key = key.verifying_key().to_bytes();
    let validator = CommitValidator {
        address: CommitValidator::address_for_key(&public_key),
        public_key,
        voting_power: 10,
    };
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".into(),
        height,
        time: CommitTimestamp {
            seconds: 1_700_000_000,
            nanos: 0,
        },
        last_block_id: Some(BlockId {
            hash: [1; 32],
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: layerx_agentd::read::validator_set_hash(&[validator]),
        next_validators_hash: layerx_agentd::read::validator_set_hash(&[validator]),
        consensus_hash: [5; 32],
        app_hash: root,
        last_results_hash: Some([7; 32]),
        evidence_hash: Some([8; 32]),
        proposer_address: validator.address,
    };
    let block_hash = header.block_hash();
    let commit = BlockCommit {
        height,
        round: 1,
        block_id: BlockId {
            hash: block_hash,
            part_set_total: 1,
            part_set_hash: [9; 32],
        },
        votes: vec![],
    };
    let template: Vec<Value> = require_ok(
        serde_json::from_slice(include_bytes!(
            "../fixtures/repair_22_31_217/checkpoint-answers.json"
        )),
        "required test value",
    );
    for method in ["px_getCommit", "px_getValidators"] {
        let mut row = require_some(
            template.iter().find(|v| v["method"] == method),
            "required test value",
        )
        .clone();
        row["params"] = json!([height.to_string()]);
        row["result"]["height"] = json!(height.to_string());
        if method == "px_getCommit" {
            let signed = &mut row["result"]["signedHeader"];
            signed["header"]["height"] = json!(height.to_string());
            signed["header"]["app_hash"] = json!(hex::encode(root));
            signed["commit"]["height"] = json!(height.to_string());
            signed["commit"]["block_id"]["hash"] = json!(hex::encode(block_hash));
            signed["commit"]["signatures"][0]["signature"] = json!(base64(
                &key.sign(&commit.vote_sign_bytes("125", header.time))
                    .to_bytes()
            ));
        }
        answers.retain(|v| !(v["method"] == row["method"] && v["params"] == row["params"]));
        answers.push(row);
    }
    for full in [true, false] {
        let mut row = require_some(
            template
                .iter()
                .find(|v| v["method"] == "eth_getBlockByNumber" && v["params"][1] == full),
            "required test value",
        )
        .clone();
        row["params"] = json!([format!("0x{height:x}"), full]);
        row["result"]["number"] = json!(format!("0x{height:x}"));
        row["result"]["hash"] = json!(format!("0x{}", hex::encode(block_hash)));
        row["result"]["stateRoot"] = json!(format!("0x{}", hex::encode(root)));
        answers.retain(|v| !(v["method"] == row["method"] && v["params"] == row["params"]));
        answers.push(row);
    }
    block_hash
}

fn state_leaf(key: &[u8], value: &[u8]) -> [u8; 32] {
    Sha256::digest(
        [
            b"LXP/v1/state-leaf\0".as_slice(),
            &(require_ok(u32::try_from(key.len()), "test value fits u32")).to_be_bytes(),
            &(require_ok(u32::try_from(value.len()), "test value fits u32")).to_be_bytes(),
            key,
            value,
        ]
        .concat(),
    )
    .into()
}

fn account_proof(
    account: [u8; 32],
    asset: [u8; 32],
    _signed: &[u8],
) -> (layerx_proof::state::VerifiedAccountState, [u8; 32]) {
    let name = format!("module:programs:value:{}", hex::encode(account));
    let mut value = Encoder::new(512);
    require_ok(
        value.u16(require_ok(name.len().try_into(), "required test value")),
        "required test value",
    );
    require_ok(value.fixed(name.as_bytes()), "required test value");
    require_ok(value.u8(13), "required test value");
    require_ok(value.u128(77), "required test value");
    require_ok(value.fixed(&asset), "required test value");
    require_ok(value.u8(1), "required test value");
    require_ok(value.u64(0), "required test value");
    require_ok(value.u64(2), "required test value");
    require_ok(value.u8(0), "required test value");
    require_ok(value.u8(0), "required test value");
    require_ok(value.fixed(&[0; 32]), "required test value");
    require_ok(value.u8(0), "required test value");
    let value = value.finish();
    let account_root = state_leaf(&[vec![4], account.to_vec()].concat(), &value);
    let universal_root = state_leaf(b"account-tree", &account_root);
    let root = state_leaf(&[0, 0], &universal_root);
    let receipt = account_snapshot_receipt(account, asset, root);
    let (receipt_proof, receipt_root) = require_ok(
        layerx_proof::merkle::build_proof(&[&receipt], 0),
        "required test value",
    );
    let mut header = Encoder::new(512);
    require_ok(
        header.structure_header_version(0x1701, 3),
        "required test value",
    );
    require_ok(header.u8(15), "required test value");
    require_ok(header.tag(1, 15), "required test value");
    require_ok(header.u16(3), "required test value");
    require_ok(header.tag(2, 15), "required test value");
    require_ok(header.u32(125), "required test value");
    for (tag, n) in [(3, 1), (4, 8), (5, 8), (6, 8)] {
        require_ok(header.tag(tag, 15), "required test value");
        require_ok(header.u64(n), "required test value");
    }
    for (tag, hash) in [
        (7, [5; 32]),
        (8, root),
        (9, [8; 32]),
        (10, receipt_root),
        (11, [1; 32]),
        (12, [2; 32]),
        (13, [3; 32]),
    ] {
        require_ok(header.tag(tag, 15), "required test value");
        require_ok(header.bytes(&hash, 32), "required test value");
    }
    require_ok(header.tag(14, 15), "required test value");
    require_ok(header.u64(1_700_000_001_000), "required test value");
    require_ok(header.tag(15, 15), "required test value");
    require_ok(header.bytes(&[1; 32], 32), "required test value");
    let header_bytes = header.finish();
    let signer = SigningKey::from_bytes(&[8; 32]);
    let header_signature = signer
        .sign(&require_ok(
            layerx_wire::hash::batch_header_digest(&header_bytes),
            "required test value",
        ))
        .to_bytes();
    let single = require_ok(Proof::new(0, 1, vec![]), "required test value");
    let proof = layerx_proof::state::NestedAccountProof {
        account_id: account,
        account_root,
        universal_root,
        resulting_state_root: root,
        account_proof: single.clone(),
        account_tree_proof: single.clone(),
        universal_root_proof: single,
        receipt_bytes: receipt,
        receipt_proof,
        header_bytes,
        header_signature,
    };
    let authorization =
        SequencerAuthorization::new([1; 32], signer.verifying_key().to_bytes(), 1, 100);
    let verified = require_ok(
        layerx_proof::state::verify_nested_account(
            &value,
            account,
            Some(asset),
            &proof,
            &authorization,
        ),
        "real nested account proof and signature",
    );
    assert!(!verified.account().frozen);
    (verified, root)
}

fn answer(
    answers: &mut Vec<Value>,
    target: PrecompileTarget,
    signature: &[u8],
    arguments: &[u8],
    result: Vec<u8>,
) {
    raw_answer(answers, target.address(), signature, arguments, result);
}
fn raw_answer(
    answers: &mut Vec<Value>,
    target: [u8; 20],
    signature: &[u8],
    arguments: &[u8],
    result: Vec<u8>,
) {
    let mut data = transaction_hash(signature)[..4].to_vec();
    data.extend(arguments);
    let row = json!({"method":"eth_call","params":[{"to":format!("0x{}",hex::encode(target)),"data":format!("0x{}",hex::encode(data))},"0x8"],"result":format!("0x{}",hex::encode(result))});
    answers.retain(|v| !(v["method"] == row["method"] && v["params"] == row["params"]));
    answers.push(row);
}
fn dynamic(words: &[[u8; 32]], bytes: &[u8]) -> Vec<u8> {
    let mut out = words.concat();
    out.extend(number(((words.len() + 1) * 32) as u128));
    out.extend(number(bytes.len() as u128));
    out.extend(bytes);
    out.resize((words.len() + 2) * 32 + bytes.len().div_ceil(32) * 32, 0);
    out
}

pub(super) fn configure(name: &str, value: &mut Value, answers: &mut Vec<Value>) -> NativeCapture {
    let program: [u8; 32] = fixed(&value["program"]);
    // Settlement captures use the node's native bank denomination. The
    // independent preference cases keep their two registered asset tokens.
    let asset = transaction_hash(b"uhpx");
    value["fee_asset"] = json!(hex::encode(asset));
    value["assets"] = json!([hex::encode(asset)]);
    if name == "native_effect_settlement" {
        value["construct"]["arguments"][0]["word"] = json!(hex::encode(asset));
    }
    let source = require_ok(
        layerx_programs_runtime::derive_program_account(
            require_ok(
                layerx_programs::ProgramId::new(program),
                "required test value",
            ),
            &[1, 2, 3],
        ),
        "required test value",
    )
    .bytes();
    let corpus: Value = require_ok(
        serde_json::from_slice(include_bytes!("../fixtures/repair_22_31_217/corpus.json")),
        "required test value",
    );
    let destination: [u8; 32] = fixed(&corpus["peer_views"]["identity"]["main"]);
    let actor: [u8; 32] = fixed(&corpus["peer_views"]["identity"]["owner"]);
    let signing = require_ok(
        k256::ecdsa::SigningKey::from_bytes((&fixed::<32>(&value["signing_seed"])).into()),
        "required test value",
    );
    let payer = require_ok(
        layerx_crypto::secp256k1::evm_address(
            signing.verifying_key().to_encoded_point(false).as_bytes(),
        ),
        "required test value",
    );
    let mut fee_token = [0; 20];
    fee_token[18..].copy_from_slice(&[0x10, 0x18]);
    raw_answer(
        answers,
        fee_token,
        b"getFeeDenom(address)",
        &[vec![0; 12], payer.to_vec()].concat(),
        dynamic(&[], b"uhpx"),
    );
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    let bank_args = dynamic(&[], b"uhpx");
    raw_answer(
        answers,
        bank,
        b"symbol(string)",
        &bank_args,
        dynamic(&[], b"PAX"),
    );
    raw_answer(
        answers,
        bank,
        b"decimals(string)",
        &bank_args,
        number(18).to_vec(),
    );
    let mut metadata =
        fixed_bytes(&corpus["peer_views"]["fee_assets"][hex::encode([4; 32])]["metadata"]);
    metadata[..32].copy_from_slice(&asset);
    answer(
        answers,
        PrecompileTarget::Ledger,
        b"getAsset(bytes32)",
        &asset,
        metadata,
    );
    answer(
        answers,
        PrecompileTarget::Ledger,
        b"accountId(bytes32,bytes32)",
        &[actor, asset].concat(),
        destination.to_vec(),
    );
    complete_configure_3_budget_native_capture(CompleteConfigure3BudgetNativeCaptureContext {
        name,
        value,
        answers,
        program,
        asset,
        source,
        destination,
        actor,
    })
}

struct CompleteConfigure3BudgetNativeCaptureContext<'a> {
    name: &'a str,
    value: &'a mut Value,
    answers: &'a mut Vec<Value>,
    program: [u8; 32],
    asset: [u8; 32],
    source: [u8; 32],
    destination: [u8; 32],
    actor: [u8; 32],
}

fn complete_configure_3_budget_native_capture(
    context: CompleteConfigure3BudgetNativeCaptureContext<'_>,
) -> NativeCapture {
    let CompleteConfigure3BudgetNativeCaptureContext {
        name,
        value,
        answers,
        program,
        asset,
        source,
        destination,
        actor,
    } = context;

    answer(
        answers,
        PrecompileTarget::Ledger,
        b"balanceOf(bytes32,bytes32)",
        &[destination, asset].concat(),
        number(1_000_000).to_vec(),
    );
    answer(
        answers,
        PrecompileTarget::Ledger,
        b"nextSequence(bytes32,bytes32)",
        &[destination, asset].concat(),
        number(0).to_vec(),
    );
    if name == "wind_down_exit" {
        value["source_account"] = json!(hex::encode(source));
        value["accounts"] = json!([hex::encode(source)]);
        value["construct"]["arguments"][1]["word"] = json!(hex::encode(source));
        let payload = require_ok(
            layerx_types::program_lifecycle::NativeProgramWindDown {
                program_id: layerx_types::intent::ProgramId::new(program),
                operation: layerx_types::program_lifecycle::ProgramWindDownOperation::Exit {
                    account: source,
                },
            }
            .encode(),
            "required test value",
        );
        value["generated_payload"] = json!(hex::encode(payload));
    } else if name == "wind_down" {
        value["construct"]["arguments"][1]["word"] = json!(hex::encode(program));
        let payload = require_ok(
            layerx_types::program_lifecycle::NativeProgramWindDown {
                program_id: layerx_types::intent::ProgramId::new(program),
                operation: layerx_types::program_lifecycle::ProgramWindDownOperation::Deprecate {
                    exit_program: program,
                    deadline_batch: 42,
                },
            }
            .encode(),
            "required test value",
        );
        value["generated_payload"] = json!(hex::encode(payload));
    }
    let construct = construct(value);
    let signed = require_ok(
        Eip1559Call {
            chain_id: 125,
            nonce: 0,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 3,
            gas_limit: 100_000,
            to: construct.target.address(),
            data: require_ok(construct.encode(), "required test value"),
        }
        .sign(&fixed(&value["signing_seed"])),
        "required test value",
    );
    complete_configure_2_budget_native_capture(CompleteConfigure2BudgetNativeCaptureContext {
        name,
        value,
        answers,
        program,
        asset,
        source,
        destination,
        actor,
        signed,
    })
}

struct CompleteConfigure2BudgetNativeCaptureContext<'a> {
    name: &'a str,
    value: &'a mut Value,
    answers: &'a mut Vec<Value>,
    program: [u8; 32],
    asset: [u8; 32],
    source: [u8; 32],
    destination: [u8; 32],
    actor: [u8; 32],
    signed: Vec<u8>,
}

fn complete_configure_2_budget_native_capture(
    context: CompleteConfigure2BudgetNativeCaptureContext<'_>,
) -> NativeCapture {
    let CompleteConfigure2BudgetNativeCaptureContext {
        name,
        value,
        answers,
        program,
        asset,
        source,
        destination,
        actor,
        signed,
    } = context;

    let mut proofs = BTreeMap::new();
    let root = if name == "wind_down_exit" {
        let (proof, root) = account_proof(source, asset, &signed);
        proofs.insert(source, proof);
        root
    } else {
        [6; 32]
    };
    checkpoint_answers(answers, 8, root);
    let block9 = checkpoint_answers(answers, 9, [7; 32]);
    let hash = fixed::<32>(&json!(
        "be0a6bdf6584320e066ad5d26fbd9a54c523907f861176d8f2eff2ea5c37330f"
    ));
    let mut logs = Vec::new();
    let (event, data) = match name {
        "deploy" => ("ProgramDeployed(bytes32,bytes32)", hash.to_vec()),
        "upgrade" => ("ProgramUpgraded(bytes32,bytes32,bytes32)", [hash,hash].concat()),
        "wind_down" => ("ProgramDeprecated(bytes32,uint8,uint8,bytes32,bytes32,uint64,uint64,uint16,uint16,bytes32)",
            [number(1),number(2),actor,program,number(42),number(9),number(0),number(0),[0x42;32]].concat()),
        "wind_down_exit" => {
            let mut leg = vec![0];leg.extend(source);leg.extend(destination);leg.extend(asset);leg.extend(77u128.to_be_bytes());leg.extend(1u16.to_be_bytes());
            let transfer_root: [u8;32] = Sha256::digest([b"LXP/v1/merkle-leaf\0".as_slice(),&leg].concat()).into();
            require_ok(layerx_programs_runtime::transfer::verify_applied_kernel_legs(&leg,transfer_root), "required test value");
            ("ProgramValueExited(bytes32,bytes32,bytes32)",[source,transfer_root].concat())
        }
        "native_effect_settlement" => {
            let issuance = require_ok(layerx_wire::hash::asset_issuance_account_id(&asset), "required test value");
            logs.push(layerx_client::sequencer::SequencerLog {address:PrecompileTarget::Ledger.address(),
                topics:vec![transaction_hash(b"Transfer(bytes32,bytes32,bytes32,uint128,uint8)"),destination,issuance,asset],data:[number(7),number(11)].concat(),log_index:0});
            ("",Vec::new())
        }
        _ => panic!("native execution case"),
    };
    if name != "native_effect_settlement" {
        logs.push(layerx_client::sequencer::SequencerLog {
            address: PrecompileTarget::Programs.address(),
            topics: vec![transaction_hash(event.as_bytes()), program],
            data,
            log_index: 0,
        });
        complete_configure_1_budget_native_capture(CompleteConfigure1BudgetNativeCaptureContext {
            name,
            answers,
            program,
            asset,
            source,
            destination,
            actor,
            hash,
        });
    }
    // Inventory at the executed head uses the same immutable peer account records.
    finish_configure_1_budget_native_capture(FinishConfigure1BudgetNativeCaptureContext {
        value,
        answers,
        signed,
        proofs,
        root,
        block9,
        logs,
    })
}

struct CompleteConfigure1BudgetNativeCaptureContext<'a> {
    name: &'a str,
    answers: &'a mut Vec<Value>,
    program: [u8; 32],
    asset: [u8; 32],
    source: [u8; 32],
    destination: [u8; 32],
    actor: [u8; 32],
    hash: [u8; 32],
}

fn complete_configure_1_budget_native_capture(
    context: CompleteConfigure1BudgetNativeCaptureContext<'_>,
) {
    let CompleteConfigure1BudgetNativeCaptureContext {
        name,
        answers,
        program,
        asset,
        source,
        destination,
        actor,
        hash,
    } = context;

    let deployed = [number(1), [0x22; 32], hash, number(2), number(1), number(1)].concat();
    answer(
        answers,
        PrecompileTarget::Programs,
        b"getProgram(bytes32)",
        &program,
        deployed,
    );
    if name == "deploy" {
        let row = require_some(answers.last_mut(), "required test value");
        require_some(row.as_object_mut(), "required test value").remove("result");
        row["error"] =
            json!({"code":-32000,"message":"LXP_NOT_FOUND: getProgram: missing program"});
    }
    answer(
        answers,
        PrecompileTarget::Programs,
        b"getProgramOwner(bytes32)",
        &program,
        actor.to_vec(),
    );
    answer(
        answers,
        PrecompileTarget::Programs,
        b"getWindDown(bytes32)",
        &program,
        [
            number(if name == "wind_down_exit" { 2 } else { 1 }),
            program,
            number(42),
            number(3),
            number(u128::from(name == "wind_down_exit")),
            number(u128::from(name == "wind_down_exit")),
        ]
        .concat(),
    );
    finish_configure_2_budget_native_capture(FinishConfigure2BudgetNativeCaptureContext {
        name,
        answers,
        program,
        asset,
        source,
        destination,
    });
}

struct FinishConfigure2BudgetNativeCaptureContext<'a> {
    name: &'a str,
    answers: &'a mut Vec<Value>,
    program: [u8; 32],
    asset: [u8; 32],
    source: [u8; 32],
    destination: [u8; 32],
}

fn finish_configure_2_budget_native_capture(
    context: FinishConfigure2BudgetNativeCaptureContext<'_>,
) {
    let FinishConfigure2BudgetNativeCaptureContext {
        name,
        answers,
        program,
        asset,
        source,
        destination,
    } = context;

    answer(
        answers,
        PrecompileTarget::Programs,
        b"getInterface(bytes32)",
        &program,
        dynamic(&[number(0), [0; 32]], &[]),
    );
    answer(
        answers,
        PrecompileTarget::Programs,
        b"getCurrentFeeSchedule()",
        &[],
        [
            number(1),
            number(1),
            number(1),
            number(1),
            number(1),
            number(1),
            number(1),
            number(1),
            asset,
        ]
        .concat(),
    );
    if name == "wind_down_exit" {
        answer(
            answers,
            PrecompileTarget::Programs,
            b"getProgramAccountById(bytes32)",
            &source,
            dynamic(&[program, asset], &[1, 2, 3]),
        );
        let args = dynamic(&[program, asset], &[1, 2, 3]);
        answer(
            answers,
            PrecompileTarget::Programs,
            b"getProgramAccount(bytes32,bytes32,bytes)",
            &args,
            [source, asset, number(2)].concat(),
        );
        answer(
            answers,
            PrecompileTarget::Programs,
            b"getValueAccount(bytes32,bytes32)",
            &[program, source].concat(),
            [number(77), number(1)].concat(),
        );
        answer(
            answers,
            PrecompileTarget::Programs,
            b"getExitRoute(bytes32,bytes32)",
            &[program, source].concat(),
            dynamic(&[asset, destination], &[1, 2, 3]),
        );
    }
}

struct FinishConfigure1BudgetNativeCaptureContext<'a> {
    value: &'a mut Value,
    answers: &'a mut Vec<Value>,
    signed: Vec<u8>,
    proofs: BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
    root: [u8; 32],
    block9: [u8; 32],
    logs: Vec<layerx_client::sequencer::SequencerLog>,
}

fn finish_configure_1_budget_native_capture(
    context: FinishConfigure1BudgetNativeCaptureContext<'_>,
) -> NativeCapture {
    let FinishConfigure1BudgetNativeCaptureContext {
        value,
        answers,
        signed,
        proofs,
        root,
        block9,
        logs,
    } = context;

    let at9 = answers
        .iter()
        .filter(|v| v["method"] == "eth_call" && v["params"][1] == "0x8")
        .cloned()
        .map(|mut v| {
            v["params"][1] = json!("0x9");
            v
        })
        .collect::<Vec<_>>();
    answers.extend(at9);
    let denomination = "uhpx";
    let mut observation = node_receipt_at(&signed, &logs, 9, root, [7; 32], denomination);
    observation["blockHash"] = json!(format!("0x{}", hex::encode(block9)));
    let receipt = fixed_bytes(&observation["receipt"]);
    answers.retain(|v| v["method"] != "pax_sequencerReceipt");
    answers.push(json!({"method":"pax_sequencerReceipt","params":[format!("0x{}",hex::encode(transaction_hash(&signed)))],"result":observation}));
    answers.push(json!({"method":"eth_sendRawTransaction","params":[format!("0x{}",hex::encode(&signed))],"result":format!("0x{}",hex::encode(transaction_hash(&signed)))}));
    let public = SigningKey::from_bytes(&[8; 32]).verifying_key().to_bytes();
    answers.push(json!({"method":"px_getReceiptKeys","params":[],"result":{"keys":[{"keyId":format!("0x{}",hex::encode([1;32])),"publicKey":format!("0x{}",hex::encode(public)),"firstHeight":1,"lastHeight":100}]}}));
    value["receipt_public_key"] = json!(hex::encode(public));
    (proofs, receipt)
}

pub(super) fn execute(case: &Case) {
    let mut answers = require_ok(case.replay.answers.lock(), "required test value");
    require_some(
        answers
            .iter_mut()
            .find(|v| v["method"] == "eth_blockNumber"),
        "required test value",
    )["result"] = json!("0x9");
}

fn account_snapshot_receipt(account: [u8; 32], asset: [u8; 32], state: [u8; 32]) -> Vec<u8> {
    // Account-state proofs retain a kernel account-open receipt under the
    // signed snapshot header; transaction execution uses module 12 separately.
    let key = SigningKey::from_bytes(&[8; 32]);
    let encode = |signature: Option<[u8; 64]>| {
        let mut e = Encoder::new(4096);
        require_ok(e.structure_header_version(0x5201, 3), "required test value");
        require_ok(e.u16(3), "required test value");
        require_ok(e.bytes(&Sha256::digest(account), 32), "required test value");
        require_ok(e.u64(8), "required test value");
        for root in [[5; 32], state, [8; 32]] {
            require_ok(e.bytes(&root, 32), "required test value");
        }
        require_ok(e.i32(0), "required test value");
        require_ok(e.sequence_length(1, 512), "required test value");
        require_ok(e.u16(1), "required test value");
        require_ok(e.u16(0), "required test value");
        require_ok(e.u16(4), "required test value");
        require_ok(e.u8(3), "required test value");
        require_ok(e.u8(0), "required test value");
        require_ok(e.bytes(&[0; 32], 32), "required test value");
        require_ok(e.bytes(&account, 256), "required test value");
        require_ok(e.u128(0), "required test value");
        require_ok(e.bytes(&number(8), 32), "required test value");
        require_ok(e.u16(1), "required test value");
        require_ok(e.u32(1), "required test value");
        require_ok(e.u32(0), "required test value");
        require_ok(e.u8(4), "required test value");
        require_ok(e.bytes(&asset, 32), "required test value");
        require_ok(e.u128(0), "required test value");
        require_ok(e.bytes(&[0; 32], 32), "required test value");
        require_ok(e.u128(0), "required test value");
        require_ok(e.u128(0), "required test value");
        require_ok(e.u64(0), "required test value");
        require_ok(e.bytes(&account, 32), "required test value");
        require_ok(e.u128(0), "required test value");
        require_ok(e.u128(0), "required test value");
        require_ok(e.bytes(&[0; 32], 32), "required test value");
        require_ok(e.bytes(&[0; 32], 32), "required test value");
        require_ok(e.bytes(&[1; 32], 32), "required test value");
        require_ok(e.u64(1_700_000_001_000), "required test value");
        require_ok(e.u8(u8::from(signature.is_some())), "required test value");
        if let Some(signature) = signature {
            require_ok(e.bytes(&signature, 64), "required test value");
        }
        e.finish()
    };
    let signature = key
        .sign(&require_ok(
            layerx_wire::hash::receipt_digest(&encode(None)),
            "required test value",
        ))
        .to_bytes();
    let encoded = encode(Some(signature));
    let decoded = require_ok(
        layerx_wire::receipt::decode(&encoded),
        "canonical account snapshot receipt",
    );
    assert_eq!(
        require_ok(
            layerx_wire::receipt::encode(&decoded),
            "required test value"
        ),
        encoded
    );
    require_ok(
        layerx_proof::receipt::verify_sequencer_signature(&encoded, key.verifying_key().to_bytes()),
        "snapshot proof receipt signature",
    );
    encoded
}
