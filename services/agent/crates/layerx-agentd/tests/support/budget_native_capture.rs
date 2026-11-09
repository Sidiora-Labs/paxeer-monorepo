use super::*;
use ed25519_dalek::{Signer, SigningKey};
use layerx_agentd::read::{
    BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator,
};
use layerx_proof::merkle::Proof;
use layerx_wire::encode::Encoder;

pub(super) struct NativeCapture {
    pub proofs: BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
    pub receipt: Vec<u8>,
}

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
    let template: Vec<Value> = serde_json::from_slice(include_bytes!(
        "../fixtures/repair_22_31_217/checkpoint-answers.json"
    ))
    .unwrap();
    for method in ["px_getCommit", "px_getValidators"] {
        let mut row = template
            .iter()
            .find(|v| v["method"] == method)
            .unwrap()
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
        let mut row = template
            .iter()
            .find(|v| v["method"] == "eth_getBlockByNumber" && v["params"][1] == full)
            .unwrap()
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
            &(key.len() as u32).to_be_bytes(),
            &(value.len() as u32).to_be_bytes(),
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
    value.u16(name.len().try_into().unwrap()).unwrap();
    value.fixed(name.as_bytes()).unwrap();
    value.u8(13).unwrap();
    value.u128(77).unwrap();
    value.fixed(&asset).unwrap();
    value.u8(1).unwrap();
    value.u64(0).unwrap();
    value.u64(2).unwrap();
    value.u8(0).unwrap();
    value.u8(0).unwrap();
    value.fixed(&[0; 32]).unwrap();
    value.u8(0).unwrap();
    let value = value.finish();
    let account_root = state_leaf(&[vec![4], account.to_vec()].concat(), &value);
    let universal_root = state_leaf(b"account-tree", &account_root);
    let root = state_leaf(&[0, 0], &universal_root);
    let receipt = account_snapshot_receipt(account, asset, root);
    let (receipt_proof, receipt_root) = layerx_proof::merkle::build_proof(&[&receipt], 0).unwrap();
    let mut header = Encoder::new(512);
    header.structure_header_version(0x1701, 3).unwrap();
    header.u8(15).unwrap();
    header.tag(1, 15).unwrap();
    header.u16(3).unwrap();
    header.tag(2, 15).unwrap();
    header.u32(125).unwrap();
    for (tag, n) in [(3, 1), (4, 8), (5, 8), (6, 8)] {
        header.tag(tag, 15).unwrap();
        header.u64(n).unwrap();
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
        header.tag(tag, 15).unwrap();
        header.bytes(&hash, 32).unwrap();
    }
    header.tag(14, 15).unwrap();
    header.u64(1_700_000_001_000).unwrap();
    header.tag(15, 15).unwrap();
    header.bytes(&[1; 32], 32).unwrap();
    let header_bytes = header.finish();
    let signer = SigningKey::from_bytes(&[8; 32]);
    let header_signature = signer
        .sign(&layerx_wire::hash::batch_header_digest(&header_bytes).unwrap())
        .to_bytes();
    let single = Proof::new(0, 1, vec![]).unwrap();
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
    let verified = layerx_proof::state::verify_nested_account(
        &value,
        account,
        Some(asset),
        &proof,
        &authorization,
    )
    .expect("real nested account proof and signature");
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
    let source = layerx_programs_runtime::derive_program_account(
        layerx_programs::ProgramId::new(program).unwrap(),
        &[1, 2, 3],
    )
    .unwrap()
    .bytes();
    let corpus: Value =
        serde_json::from_slice(include_bytes!("../fixtures/repair_22_31_217/corpus.json")).unwrap();
    let destination: [u8; 32] = fixed(&corpus["peer_views"]["identity"]["main"]);
    let actor: [u8; 32] = fixed(&corpus["peer_views"]["identity"]["owner"]);
    let signing =
        k256::ecdsa::SigningKey::from_bytes((&fixed::<32>(&value["signing_seed"])).into()).unwrap();
    let payer = layerx_crypto::secp256k1::evm_address(
        signing.verifying_key().to_encoded_point(false).as_bytes(),
    )
    .unwrap();
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
        let payload = layerx_types::program_lifecycle::NativeProgramWindDown {
            program_id: layerx_types::intent::ProgramId::new(program),
            operation: layerx_types::program_lifecycle::ProgramWindDownOperation::Exit {
                account: source,
            },
        }
        .encode()
        .unwrap();
        value["generated_payload"] = json!(hex::encode(payload));
    } else if name == "wind_down" {
        value["construct"]["arguments"][1]["word"] = json!(hex::encode(program));
        let payload = layerx_types::program_lifecycle::NativeProgramWindDown {
            program_id: layerx_types::intent::ProgramId::new(program),
            operation: layerx_types::program_lifecycle::ProgramWindDownOperation::Deprecate {
                exit_program: program,
                deadline_batch: 42,
            },
        }
        .encode()
        .unwrap();
        value["generated_payload"] = json!(hex::encode(payload));
    }
    let construct = construct(value);
    let signed = Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
        to: construct.target.address(),
        data: construct.encode().unwrap(),
    }
    .sign(&fixed(&value["signing_seed"]))
    .unwrap();
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
            layerx_programs_runtime::transfer::verify_applied_kernel_legs(&leg,transfer_root).unwrap();
            ("ProgramValueExited(bytes32,bytes32,bytes32)",[source,transfer_root].concat())
        }
        "native_effect_settlement" => {
            let issuance = layerx_wire::hash::asset_issuance_account_id(&asset).unwrap();
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
        let deployed = [number(1), [0x22; 32], hash, number(2), number(1), number(1)].concat();
        answer(
            answers,
            PrecompileTarget::Programs,
            b"getProgram(bytes32)",
            &program,
            deployed,
        );
        if name == "deploy" {
            let row = answers.last_mut().unwrap();
            row.as_object_mut().unwrap().remove("result");
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
                number(if name == "wind_down_exit" { 1 } else { 0 }),
                number(if name == "wind_down_exit" { 1 } else { 0 }),
            ]
            .concat(),
        );
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
    // Inventory at the executed head uses the same immutable peer account records.
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
    let mut observation = node_receipt_at(&signed, &logs, 9, root, [7; 32], &denomination);
    observation["blockHash"] = json!(format!("0x{}", hex::encode(block9)));
    let receipt = fixed_bytes(&observation["receipt"]);
    answers.retain(|v| v["method"] != "pax_sequencerReceipt");
    answers.push(json!({"method":"pax_sequencerReceipt","params":[format!("0x{}",hex::encode(transaction_hash(&signed)))],"result":observation}));
    answers.push(json!({"method":"eth_sendRawTransaction","params":[format!("0x{}",hex::encode(&signed))],"result":format!("0x{}",hex::encode(transaction_hash(&signed)))}));
    let public = SigningKey::from_bytes(&[8; 32]).verifying_key().to_bytes();
    answers.push(json!({"method":"px_getReceiptKeys","params":[],"result":{"keys":[{"keyId":format!("0x{}",hex::encode([1;32])),"publicKey":format!("0x{}",hex::encode(public)),"firstHeight":1,"lastHeight":100}]}}));
    value["receipt_public_key"] = json!(hex::encode(public));
    NativeCapture { proofs, receipt }
}

pub(super) fn execute(case: &Case) {
    let mut answers = case.replay.answers.lock().unwrap();
    answers
        .iter_mut()
        .find(|v| v["method"] == "eth_blockNumber")
        .unwrap()["result"] = json!("0x9");
}

pub(super) fn peer(case: &Case) -> layerx_agentd::human::HumanPeer {
    let owner = case.value["did"].as_str().unwrap().to_owned();
    layerx_agentd::human::HumanPeer {
        uid: 1000,
        principal: "operator".into(),
        tenant: case.tenant().as_str().into(),
        subject: Some(layerx_agentd::human::HumanSubject {
            transport_tenant: "native-budget-peer".into(),
            transport_principal: "unix-owner".into(),
            account: format!("agent:{owner}:main"),
            owner,
            asset: case.id("fee_asset"),
            registration: None,
        }),
    }
}

pub(super) fn retain(
    case: &Case,
    sources: &VerifiedResolvedProgramCharges,
    native: &EvmPrepared,
    submission: &VerifiedSubmission,
    registry: &ModuleRegistry,
) -> PathBuf {
    use layerx_agentd::approval::native_effect::NativeEffectApprovalCarrier as Effect;
    use layerx_agentd::approval::native_program::NativeProgramApprovalCarrier as Program;
    use std::os::unix::fs::PermissionsExt;
    let root = temporary(
        case.value["store"]
            .as_str()
            .expect("corpus store producer path"),
    );
    let mut store = Store::open(&root).unwrap();
    let budget = BudgetLimiter::new(vec![]).unwrap();
    let configs = sources
        .charges()
        .iter()
        .enumerate()
        .map(|(index, _)| LimitConfig {
            id: LimitId([index as u8 + 1; 16]),
            name: format!("native debit {index}"),
            scope: LimitScope::Tenant(sources.actor()),
            ceiling: 1_000_000,
            consumed: 0,
        })
        .collect::<Vec<_>>();
    let installed =
        install_enrolment_limits(&mut store, &budget, &case.tenant(), &configs).unwrap();
    let allocations = sources
        .budget_allocations(|charge| {
            let index = sources
                .charges()
                .iter()
                .position(|value| value == charge)
                .unwrap();
            Ok(vec![installed[index].limit_id])
        })
        .unwrap();
    let path = root.join("denominations.json");
    let limits = sources.charges().iter().zip(&installed).map(|(charge,limit)| json!({
        "stable_id":hex::encode(limit.stable_id),"asset":hex::encode(charge.asset),"source":hex::encode(charge.source.account())
    })).collect::<Vec<_>>();
    std::fs::write(
        &path,
        serde_json::to_vec(&json!({"version":1,"tenant":case.tenant().as_str(),"limits":limits}))
            .unwrap(),
    )
    .unwrap();
    std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600)).unwrap();
    let verified =
        layerx_agentd::config::load_program_budget_denominations(&path, &case.tenant()).unwrap();
    install_program_enrolment_denominations(&mut store, &budget, &verified).unwrap();
    let now = case.value["now_ms"].as_u64().unwrap();
    let request = ProgramReservationRequest {
        id: allocations.preparation_digest(),
        charges: allocations.charges().to_vec(),
        expiry_sequence: 18,
        current_sequence: 8,
        core_deadline: Some(CoreTimestampMs(now + 30_000)),
    };
    let staged = budget
        .stage_program_allocation(&request, &allocations, CoreTimestampMs(now))
        .unwrap();
    let public = SigningKey::from_bytes(&[8; 32]).verifying_key().to_bytes();
    let keys = [SequencerAuthorization::new([1; 32], public, 1, 100)];
    let peer = peer(case);
    let id = allocations.preparation_digest();
    let mut outbox = layerx_agentd::outbox::Outbox::default();
    if native.call.to == PrecompileTarget::Programs.address() {
        let held = Program::hold_call(&mut store, &peer, [3; 32], 1, native, staged, now, &keys)
            .expect("production lifecycle approval hold");
        Program::decide_call(
            &mut store,
            &budget,
            &peer,
            id,
            held.call_held_digest().unwrap(),
            [4; 32],
            true,
            8,
            now + 1,
        )
        .unwrap();
        Program::queue_call(
            &mut store,
            &mut outbox,
            &peer,
            id,
            submission,
            registry,
            8,
            now + 2,
        )
        .expect("production native program queue");
    } else {
        let held = Effect::hold_call(&mut store, &peer, [3; 32], 1, native, staged, now, &keys)
            .expect("production burn approval hold");
        Effect::decide_call(
            &mut store,
            &budget,
            &peer,
            id,
            held.call_held_digest().unwrap(),
            [4; 32],
            true,
            8,
            now + 1,
        )
        .unwrap();
        Effect::queue_call(
            &mut store,
            &mut outbox,
            &peer,
            id,
            submission,
            registry,
            8,
            now + 2,
        )
        .expect("production native effect queue");
    }
    assert_eq!(
        outbox
            .bytes_for_transmission(submission.activity_id())
            .unwrap(),
        submission.exact_bytes()
    );
    assert_eq!(
        case.endpoint
            .send_raw_transaction(submission.exact_bytes())
            .expect("submit exact producer bytes to loopback peer"),
        submission.activity_id()
    );
    drop(store);
    drop(budget);
    drop(outbox);
    root
}

fn account_snapshot_receipt(account: [u8; 32], asset: [u8; 32], state: [u8; 32]) -> Vec<u8> {
    // Account-state proofs retain a kernel account-open receipt under the
    // signed snapshot header; transaction execution uses module 12 separately.
    let key = SigningKey::from_bytes(&[8; 32]);
    let encode = |signature: Option<[u8; 64]>| {
        let mut e = Encoder::new(4096);
        e.structure_header_version(0x5201, 3).unwrap();
        e.u16(3).unwrap();
        e.bytes(&Sha256::digest(account), 32).unwrap();
        e.u64(8).unwrap();
        for root in [[5; 32], state, [8; 32]] {
            e.bytes(&root, 32).unwrap();
        }
        e.i32(0).unwrap();
        e.sequence_length(1, 512).unwrap();
        e.u16(1).unwrap();
        e.u16(0).unwrap();
        e.u16(4).unwrap();
        e.u8(3).unwrap();
        e.u8(0).unwrap();
        e.bytes(&[0; 32], 32).unwrap();
        e.bytes(&account, 256).unwrap();
        e.u128(0).unwrap();
        e.bytes(&number(8), 32).unwrap();
        e.u16(1).unwrap();
        e.u32(1).unwrap();
        e.u32(0).unwrap();
        e.u8(4).unwrap();
        e.bytes(&asset, 32).unwrap();
        e.u128(0).unwrap();
        e.bytes(&[0; 32], 32).unwrap();
        e.u128(0).unwrap();
        e.u128(0).unwrap();
        e.u64(0).unwrap();
        e.bytes(&account, 32).unwrap();
        e.u128(0).unwrap();
        e.u128(0).unwrap();
        e.bytes(&[0; 32], 32).unwrap();
        e.bytes(&[0; 32], 32).unwrap();
        e.bytes(&[1; 32], 32).unwrap();
        e.u64(1_700_000_001_000).unwrap();
        e.u8(u8::from(signature.is_some())).unwrap();
        if let Some(signature) = signature {
            e.bytes(&signature, 64).unwrap();
        }
        e.finish()
    };
    let signature = key
        .sign(&layerx_wire::hash::receipt_digest(&encode(None)).unwrap())
        .to_bytes();
    let encoded = encode(Some(signature));
    let decoded =
        layerx_wire::receipt::decode(&encoded).expect("canonical account snapshot receipt");
    assert_eq!(layerx_wire::receipt::encode(&decoded).unwrap(), encoded);
    layerx_proof::receipt::verify_sequencer_signature(&encoded, key.verifying_key().to_bytes())
        .expect("snapshot proof receipt signature");
    encoded
}
