// Import the real submission/transport crate. Compile the private production
// lifecycle modules here as well, so fixture setup does not expose host-only
// constructors as public library APIs. Their original unit assertions remain intact.
pub use layerx_programs_sandbox::{
    sandbox_state_key, sandbox_terminal_key, SandboxActivityRefusal, SandboxProtocolContext,
    SandboxSubmission,
};
#[path = "../src/lease.rs"]
pub mod lease;
pub use lease::*;
#[path = "../src/escrow.rs"]
pub mod escrow;
pub use escrow::*;
#[path = "../src/usage.rs"]
pub mod usage;
pub use usage::*;
#[path = "../src/expiry.rs"]
pub mod expiry;
pub use expiry::*;
#[path = "../src/execute.rs"]
pub mod execute;
pub use execute::*;
#[path = "../src/snapshot.rs"]
pub mod snapshot;
pub use snapshot::*;
#[path = "../../layerx-programs-registry/tests/support/chain.rs"]
mod chain;
#[path = "../src/host_ffi/terminal.rs"]
mod native_terminal;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    decode::{decode_commitment_proof, ProofOp},
    proto::{ProtoReader, ProtoValue},
    vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
};
use layerx_programs_runtime::{
    hash_bytes, Abi, AuthorizationContext, Capability, CapabilitySet, FeeSchedule, HashAlgorithm,
    Meter, PrincipalId, ProgramId, ResourceBudget, Storage, UnavailableReceiptOracle,
    ValidationLimits, WasmEngine, WasmValue, ABI_V2_VERSION,
};
use sha2::{Digest, Sha256};
use std::{cell::RefCell, collections::BTreeMap, fs};

fn recorded(name: &str) -> Vec<u8> {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("src/repair_22_31_262_recordings")
        .join(format!("{name}.hex"));
    let text = fs::read_to_string(path).unwrap();
    let digits: Vec<_> = text.bytes().filter(|b| !b.is_ascii_whitespace()).collect();
    assert_eq!(digits.len() % 2, 0);
    digits
        .chunks_exact(2)
        .map(|pair| {
            let digit = |b: u8| match b {
                b'0'..=b'9' => b - b'0',
                b'a'..=b'f' => b - b'a' + 10,
                _ => panic!("invalid committed native hex"),
            };
            digit(pair[0]) * 16 + digit(pair[1])
        })
        .collect()
}

#[test]
fn truncated_submission_refuses_every_native_operation_prefix() {
    let context = SandboxProtocolContext::new(3, 4).unwrap();
    for name in ["sandboxFund", "sandboxActivate", "sandboxExecute"] {
        let bytes = recorded(&format!("{name}.payload"));
        let submission = SandboxSubmission::decode(&bytes, context).unwrap();
        assert_eq!(submission.payload(), bytes);
        for length in 0..bytes.len() {
            let decoded =
                std::panic::catch_unwind(|| SandboxSubmission::decode(&bytes[..length], context));
            assert_eq!(
                decoded.unwrap(),
                Err(SandboxActivityRefusal::NonCanonical),
                "{name}: prefix {length}"
            );
        }
    }
}

fn requested(id: u64, code_hash: [u8; 32]) -> Lease {
    let mut id_bytes = [1u8; 32];
    id_bytes[24..].copy_from_slice(&id.to_be_bytes());
    Lease::request(
        LeaseId::new(id_bytes).unwrap(),
        PrincipalId::new([2; 32]).unwrap(),
        ProgramId::new([3; 32]).unwrap(),
        code_hash,
        [4; 32],
        100_000,
        LeaseLimits {
            cpu_fuel: 100_000,
            memory_bytes: 1 << 20,
            storage_read_bytes: 1 << 20,
            storage_write_bytes: 1 << 20,
            output_values: 100,
            output_bytes: 1 << 20,
            table_elements: 100,
            namespace_bytes: 1 << 20,
        },
        1,
        20,
    )
    .unwrap()
}

fn funded_active(id: u64, code_hash: [u8; 32]) -> DurableUsageState {
    let mut lease = requested(id, code_hash);
    lease
        .apply_host_activity(LeaseActivity::Fund, [6; 32], 1)
        .unwrap();
    let escrow = Escrow::funded_genesis(&lease, [7; 32]).unwrap();
    lease
        .apply_host_activity(LeaseActivity::Activate, [8; 32], 2)
        .unwrap();
    DurableUsageState {
        lease,
        escrow,
        ledger: UsageLedger::new(),
    }
}

fn meter() -> Meter {
    Meter::new(ResourceBudget::declared(), FeeSchedule::declared())
}

fn authority(state: &DurableUsageState) -> DestroyAuthority {
    DestroyAuthority {
        activity_id: [9; 32],
        expected_lease_root: state.lease.state_digest().unwrap(),
        expected_sequence: 1,
        boundary: state.lease.expiry(),
    }
}

fn destroy_real(state: &mut DurableUsageState, storage: &mut Storage) -> TerminalLeaseRecord {
    let plan = expiry::plan_teardown(state, authority(state)).unwrap();
    let refund_root = if plan.refund() == 0 {
        [0; 32]
    } else {
        [10; 32]
    };
    let charge_root = if plan.occupancy_charge() == 0 {
        [0; 32]
    } else {
        layerx_programs_runtime::sandbox_escrow_charge_root(
            &layerx_programs_runtime::transfer::SandboxEscrowCharge {
                host_program: state.lease.host_program(),
                execution_principal: state.lease.namespace().execution_principal().unwrap(),
                invocation_authority: plan.authority().activity_id,
                lease_id: state.lease.id().bytes(),
                expected_lease_digest: plan.authority().expected_lease_root,
                escrow_account: state.lease.escrow_account(),
                asset: state.lease.escrow_asset(),
                fee_destination: state.lease.fee_destination(),
                amount: plan.occupancy_charge(),
            },
        )
        .unwrap()
    };
    expiry::destroy(storage, &mut meter(), state, plan, charge_root, refund_root).unwrap()
}

#[test]
fn expiry_history_survives_lifetime_batches_replay_and_reopen() {
    let mut queue = ExpiryQueue::new();
    let mut states = BTreeMap::new();
    let mut ids = Vec::new();
    let mut destroyed = 0;
    for batch in 0..34u64 {
        for n in 0..128u64 {
            let mut state = funded_active(1 + batch * 128 + n, [4; 32]);
            let mut storage = Storage::new();
            storage
                .replace_protocol_namespace(
                    state.lease.namespace().storage_namespace().unwrap(),
                    &[(b"k".to_vec(), b"v".to_vec())],
                )
                .unwrap();
            state
                .lease
                .record_usage(
                    LeaseUsage {
                        namespace_bytes: 2,
                        ..LeaseUsage::default()
                    },
                    0,
                    2,
                    None,
                )
                .unwrap();
            queue.schedule(&state.lease).unwrap();
            let id = state.lease.id();
            ids.push(id);
            states.insert(id, (state, storage));
        }
        let after = sweep(&mut queue, 20, 128, |id, boundary| {
            assert_eq!(boundary, 20);
            let (state, storage) = states.get_mut(&id).unwrap();
            let record = destroy_real(state, storage);
            assert_eq!(record.reclaimed_cells(), 1);
            assert_eq!(record.reclaimed_bytes(), 2);
            assert_eq!(state.lease.state(), LeaseState::Destroyed);
            assert_eq!(
                state.escrow.spent() + state.escrow.refunded(),
                state.escrow.funded()
            );
            destroyed += 1;
            Ok(record)
        })
        .unwrap();
        assert!(after.leases().is_empty());
        assert_eq!(after.remaining_due(), 0);
    }
    assert_eq!(destroyed, 4352);
    assert!(destroyed > expiry::MAX_EXPIRY_QUEUE_ENTRIES);
    let pending = funded_active(5000, [4; 32]);
    queue.schedule(&pending.lease).unwrap();
    let replay = queue.terminal(ids[0]).unwrap().clone();
    let path = std::env::temp_dir().join(format!("repair-22-33-231-queue-{}", std::process::id()));
    let bytes = queue.canonical_state().unwrap();
    fs::write(&path, &bytes).unwrap();
    drop(queue);
    let saved = fs::read(&path).unwrap();
    fs::remove_file(path).unwrap();
    let mut queue = ExpiryQueue::decode_state(&saved).unwrap();
    assert_eq!(queue.canonical_state().unwrap(), bytes);
    for id in &ids {
        queue.terminal(*id).unwrap().verify().unwrap();
    }
    assert_eq!(queue.record_destroyed(replay), Err(ExpiryRefusal::Replay));
    assert_eq!(
        queue.schedule(&funded_active(1, [4; 32]).lease),
        Err(ExpiryRefusal::Replay)
    );
    assert_eq!(queue.due(20, 128).unwrap().leases(), &[pending.lease.id()]);
    let mut pending = pending;
    let after = sweep(&mut queue, 20, 128, |id, _| {
        assert_eq!(id, pending.lease.id());
        Ok(destroy_real(&mut pending, &mut Storage::new()))
    })
    .unwrap();
    assert!(after.leases().is_empty());
    let mut calls = 0;
    assert_eq!(
        sweep(&mut queue, 20, 129, |_, _| {
            calls += 1;
            unreachable!()
        }),
        Err(ExpiryRefusal::InvalidLimit)
    );
    assert_eq!(calls, 0);
    for n in [0, 1, saved.len() / 2, saved.len() - 1] {
        assert!(ExpiryQueue::decode_state(&saved[..n]).is_err());
    }
    let mut tampered = saved;
    *tampered.last_mut().unwrap() ^= 1;
    assert!(ExpiryQueue::decode_state(&tampered).is_err());
}

#[test]
fn active_queue_retains_its_bound_independently_of_terminal_history() {
    let mut queue = ExpiryQueue::new();
    for id in 1..=expiry::MAX_EXPIRY_QUEUE_ENTRIES {
        queue.schedule(&requested(id as u64, [4; 32])).unwrap();
    }
    assert_eq!(
        queue.schedule(&requested(5000, [4; 32])),
        Err(ExpiryRefusal::QueueFull)
    );
    let bytes = queue.canonical_state().unwrap();
    assert_eq!(ExpiryQueue::decode_state(&bytes).unwrap(), queue);
}

// This is the host's in-memory archive transport. It stores exactly the bytes
// emitted by the production publisher, keyed by call token and state kind.
thread_local! {
    static ARCHIVE: RefCell<BTreeMap<(u64, u16), Vec<u8>>> = const { RefCell::new(BTreeMap::new()) };
}
#[unsafe(no_mangle)]
extern "C" fn layerx_programs_sandbox_destroy_archive(
    token: u64,
    kind: u16,
    bytes: *const u8,
    length: u32,
) -> i32 {
    let bytes = unsafe { std::slice::from_raw_parts(bytes, length as usize) };
    ARCHIVE.with(|archive| {
        archive.borrow_mut().insert((token, kind), bytes.to_vec());
    });
    0
}

#[test]
fn native_terminal_refusals_preserve_token_result_until_single_consumption() {
    let token = 71;
    let funded = Lease::decode_state(&recorded("sandboxFund.after.lease.value")).unwrap();
    let escrow = Escrow::funded_genesis(&funded, [7; 32]).unwrap();
    let lease = Lease::decode_state(&recorded("sandboxActivate.after.lease.value")).unwrap();
    let mut state = DurableUsageState {
        lease,
        escrow,
        ledger: UsageLedger::new(),
    };
    let plan = expiry::plan_teardown(&state, authority(&state)).unwrap();
    let settled = expiry::settle_teardown(&mut state, plan, [0; 32], [10; 32]).unwrap();
    native_terminal::publish_destroy(
        token,
        &state,
        settled.ledger_state(),
        settled.usage_lease_state(),
        settled.receipt_digest(),
        settled.refund_root(),
        plan.refund(),
    )
    .unwrap();
    ARCHIVE.with(|archive| {
        let archive = archive.borrow();
        let lease = Lease::decode_state(&archive[&(token, 0)]).unwrap();
        assert_eq!(lease.state(), LeaseState::Destroyed);
        assert_eq!(lease.id(), state.lease.id());
        let escrow = Escrow::decode_state(&lease, &archive[&(token, 1)]).unwrap();
        assert_eq!(escrow.remaining().unwrap(), 0);
        assert_eq!(archive[&(token, 2)], settled.ledger_state());
        assert_eq!(archive[&(token, 3)], settled.usage_lease_state());
    });
    let read = native_terminal::layerx_programs_sandbox_destroy_terminal;
    let mut output = [0xa5; 235];
    for wrong in [0, token - 1, u64::MAX] {
        assert_eq!(read(wrong, output.as_mut_ptr(), 235), -213);
        assert_eq!(output, [0xa5; 235]);
    }
    assert_eq!(read(token, std::ptr::null_mut(), 235), -3);
    for capacity in [0, 1, 143] {
        assert_eq!(read(token, output.as_mut_ptr(), capacity), -3);
        assert_eq!(output, [0xa5; 235]);
    }
    assert_eq!(read(token, output.as_mut_ptr(), 235), 0);
    assert_eq!(&output[..32], &settled.receipt_digest());
    assert_eq!(&output[32..64], &settled.refund_root());
    assert_eq!(&output[64..80], &plan.refund().to_be_bytes());
    assert_eq!(&output[80..112], &state.lease.state_digest().unwrap());
    assert_eq!(
        &output[112..144],
        &Sha256::digest(state.escrow.canonical_state())[..]
    );
    assert_eq!(&output[144..], &[0xa5; 91]);
    output.fill(0x5a);
    assert_eq!(read(token, output.as_mut_ptr(), 235), -3);
    assert_eq!(output, [0x5a; 235]);
}

fn continuation_module() -> Vec<u8> {
    use layerx_programs_runtime::test_support::*;
    module(&[
        type_section(&[(&[TYPE_I32, TYPE_I64], &[TYPE_I32])]),
        function_section(&[0]),
        raw_section(5, &[1, 0, 1]),
        raw_section(6, &[1, TYPE_I32, 1, OP_I32_CONST, 7, OP_END]),
        raw_section(
            7,
            &[
                3, 1, b'g', 3, 0, 6, b'm', b'e', b'm', b'o', b'r', b'y', 2, 0, 8, b'c', b'o', b'n',
                b't', b'i', b'n', b'u', b'e', 0, 0,
            ],
        ),
        code_section(&[func_body(&[], &[0x20, 0, OP_END])]),
    ])
}

fn captured() -> (Lease, SandboxState) {
    let code = continuation_module();
    let state = funded_active(8000, hash_bytes(HashAlgorithm::Sha256, &code).unwrap());
    let lease = state.lease;
    let validated = WasmEngine::new(ValidationLimits::declared())
        .unwrap()
        .validate_v2(&code)
        .unwrap();
    let mut storage = Storage::new();
    storage
        .replace_protocol_namespace(
            lease.namespace().storage_namespace().unwrap(),
            &[(b"k".to_vec(), b"v".to_vec())],
        )
        .unwrap();
    let authorization = AuthorizationContext::new(
        lease.namespace().execution_principal().unwrap(),
        CapabilitySet::new([Capability::StorageRead, Capability::StorageWrite]).unwrap(),
    );
    let abi = Abi::new(
        ABI_V2_VERSION,
        lease.host_program(),
        authorization,
        storage,
        &UnavailableReceiptOracle,
    )
    .unwrap();
    let mut instance = validated.instantiate_sandbox(meter(), abi).unwrap();
    let captured = Snapshot::capture(
        &lease,
        &mut instance,
        "continue",
        &[WasmValue::I32(1), WasmValue::I64(2)],
    )
    .unwrap();
    (lease, captured.state().clone())
}

fn length_at(bytes: &[u8], offset: &mut usize) -> usize {
    let n = u64::from_be_bytes(bytes[*offset..*offset + 8].try_into().unwrap()) as usize;
    *offset += 8;
    n
}
fn skip_bytes(bytes: &[u8], offset: &mut usize) {
    let n = length_at(bytes, offset);
    *offset += n;
}
fn skip_value(bytes: &[u8], offset: &mut usize) {
    *offset += match bytes[*offset] {
        0 => 5,
        1 => 9,
        _ => panic!("producer value"),
    };
}

#[test]
fn snapshot_decoder_bounds_counts_lengths_and_truncated_runtime_capture() {
    let (lease, state) = captured();
    let bytes = state.canonical_bytes().unwrap();
    assert_eq!(SandboxState::from_canonical(&lease, &bytes).unwrap(), state);
    assert_eq!(state.namespace_cells().len(), 1);
    assert_eq!(state.continuation().arguments.len(), 2);
    let memory_offset = b"LayerX/programs/sandbox/snapshot/v1\0".len() + 96;
    let mut at = memory_offset;
    skip_bytes(&bytes, &mut at);
    let globals_offset = at;
    let globals = length_at(&bytes, &mut at);
    assert!(globals > 0);
    for _ in 0..globals {
        skip_bytes(&bytes, &mut at);
        skip_value(&bytes, &mut at);
    }
    skip_bytes(&bytes, &mut at);
    let arguments_offset = at;
    let args = length_at(&bytes, &mut at);
    for _ in 0..args {
        skip_value(&bytes, &mut at);
    }
    let cells_offset = at;
    assert_eq!(length_at(&bytes, &mut at), 1);
    let key_offset = at;
    skip_bytes(&bytes, &mut at);
    let value_offset = at;
    for (offset, max) in [
        (globals_offset, MAX_SANDBOX_GLOBALS),
        (arguments_offset, MAX_SANDBOX_OPERAND_STACK),
        (cells_offset, MAX_SANDBOX_NAMESPACE_CELLS),
    ] {
        for count in [max as u64 + 1, u32::MAX as u64, u64::MAX] {
            let mut malformed = bytes.clone();
            malformed[offset..offset + 8].copy_from_slice(&count.to_be_bytes());
            assert_eq!(
                std::panic::catch_unwind(|| SandboxState::from_canonical(&lease, &malformed))
                    .unwrap(),
                Err(SnapshotRefusal::StateBoundExceeded)
            );
        }
        let mut short = bytes[..offset + 8].to_vec();
        for count in [1, max as u64] {
            short[offset..offset + 8].copy_from_slice(&count.to_be_bytes());
            assert_eq!(
                SandboxState::from_canonical(&lease, &short),
                Err(SnapshotRefusal::NonCanonicalContinuation)
            );
        }
    }
    for offset in [memory_offset, key_offset, value_offset] {
        let mut malformed = bytes.clone();
        malformed[offset..offset + 8].copy_from_slice(&u64::MAX.to_be_bytes());
        assert_eq!(
            std::panic::catch_unwind(|| SandboxState::from_canonical(&lease, &malformed)).unwrap(),
            Err(SnapshotRefusal::StateBoundExceeded)
        );
    }
    for n in 0..bytes.len() {
        assert!(
            std::panic::catch_unwind(|| SandboxState::from_canonical(&lease, &bytes[..n]))
                .unwrap()
                .is_err(),
            "snapshot prefix {n}"
        );
    }
    let mut trailing = bytes.clone();
    trailing.push(0);
    assert_eq!(
        SandboxState::from_canonical(&lease, &trailing),
        Err(SnapshotRefusal::NonCanonicalContinuation)
    );
}

fn proof_ops(bytes: &[u8]) -> Vec<ProofOp> {
    let mut reader = ProtoReader::new(bytes);
    let mut out = Vec::new();
    while let Some(field) = reader.next_field().unwrap() {
        assert_eq!(field.number, 1);
        let ProtoValue::Bytes(bytes) = field.value else {
            panic!("proof operation");
        };
        let mut r = ProtoReader::new(bytes);
        let mut op = ProofOp {
            op_type: String::new(),
            key: Vec::new(),
            data: Vec::new(),
        };
        while let Some(field) = r.next_field().unwrap() {
            let ProtoValue::Bytes(bytes) = field.value else {
                panic!("proof field");
            };
            match field.number {
                1 => op.op_type = String::from_utf8(bytes.to_vec()).unwrap(),
                2 => op.key = bytes.to_vec(),
                3 => op.data = bytes.to_vec(),
                _ => panic!("unexpected proof field"),
            }
        }
        out.push(op);
    }
    out
}

#[test]
fn registry_transition_contract_matches_native_chain_state_producer() {
    let previous = Lease::decode_state(&recorded("sandboxActivate.before.lease.value")).unwrap();
    let canonical = recorded("sandboxActivate.after.lease.value");
    let next = Lease::decode_state(&canonical).unwrap();
    let transition = next.history().last().unwrap().transition;
    let ops = proof_ops(&recorded("sandboxActivate.after.lease.ops"));
    let mut fixture = chain::ChainFixture::new_in_store(
        "lxphost",
        "repair-native",
        &[[17; 32]],
        vec![(b"encoder".to_vec(), vec![1])],
    )
    .unwrap();
    fixture.header.height = 101;
    fixture.header.time.seconds = 1_700_000_001;
    fixture.header.time.nanos = 123_000_000;
    fixture.header.app_hash = recorded("sandboxActivate.after.root");
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture.header.hash().unwrap().to_vec();
    let key = SigningKey::from_bytes(&[17; 32]);
    let timestamp = Timestamp {
        seconds: fixture.header.time.seconds,
        nanos: fixture.header.time.nanos,
    };
    let sign_bytes = vote_sign_bytes(
        &fixture.header.chain_id,
        fixture.commit.height,
        fixture.commit.round,
        Some(&fixture.commit.block_id),
        timestamp,
    )
    .unwrap();
    fixture.commit.signatures = vec![CommitSig {
        flag: BlockIdFlag::Commit,
        validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
        timestamp,
        signature: key.sign(&sign_bytes).to_bytes().to_vec(),
    }];
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let authorization =
        AuthorizationContext::new(previous.tenant(), CapabilitySet::new([]).unwrap());
    let evidence = TransitionEvidence::verify_call(
        &head,
        &previous,
        transition,
        &authorization,
        &canonical,
        &ops,
    )
    .unwrap();
    assert_eq!(evidence.lease_digest(), next.state_digest().unwrap());
    assert_eq!(evidence.state_height(), 100);
    assert_eq!(
        sandbox_state_key(0, previous.id().bytes()).unwrap(),
        recorded("sandboxActivate.after.lease.key")
    );
    let mut bad = canonical.clone();
    *bad.last_mut().unwrap() ^= 1;
    assert!(TransitionEvidence::verify_call(
        &head,
        &previous,
        transition,
        &authorization,
        &bad,
        &ops
    )
    .is_err());
    let mut wrong_ops = ops.clone();
    wrong_ops[1].key = b"bank".to_vec();
    assert!(TransitionEvidence::verify_call(
        &head,
        &previous,
        transition,
        &authorization,
        &canonical,
        &wrong_ops
    )
    .is_err());
    assert!(TransitionEvidence::verify_call(
        &head,
        &previous,
        transition,
        &authorization,
        &canonical,
        &[]
    )
    .is_err());
    let retired =
        include_bytes!("../../layerx-programs-registry/tests/fixtures/maintained-head/receipt");
    assert!(decode_commitment_proof(retired).is_err());
    assert!(TransitionEvidence::verify_call(
        &head,
        &previous,
        transition,
        &authorization,
        retired,
        &ops
    )
    .is_err());
}

#[test]
fn programs_contract_is_published_with_the_crate() {
    let contract = include_str!("../src/programs_contract.md");
    let publication = include_str!("../src/lib.rs");
    assert!(publication.contains("include_str!(\"programs_contract.md\")"));
    for term in [
        "VerifiedChainHead",
        "canonical_lease",
        "lease_ops",
        "lxphost",
        "144-byte",
        "-213",
        "-3",
        "restart",
        "decode_state",
    ] {
        assert!(contract.contains(term), "contract lacks {term}");
    }
    let record = include_str!("../src/repair_22_33_231.kvx");
    for id in [
        "truncated-submission",
        "expiry-lifetime-capacity",
        "terminal-token-lifecycle",
        "registry-transition-proof",
        "missing-programs-contract",
        "snapshot-allocation-bounds",
    ] {
        assert!(record.contains(&format!("[finding.{id}]")));
    }
    assert!(record.contains("spec/layerx-merge/interfaces.kvx"));
    assert!(record.contains("contracts/programs.md"));
}
