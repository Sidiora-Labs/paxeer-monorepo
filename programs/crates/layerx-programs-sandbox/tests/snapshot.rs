#[path = "../../layerx-programs-registry/tests/support/chain.rs"]
mod chain;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    decode::decode_commitment_proof, header::ChainHeader, validators::TrustedValidator,
    vote::vote_sign_bytes,
};
use sha2::{Digest, Sha256};

use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, module, raw_section, type_section, unsigned_leb,
    OP_END, OP_I32_ADD, OP_I32_CONST, TYPE_I32,
};
use layerx_programs_runtime::{
    Abi, AuthorizationContext, CapabilitySet, FeeSchedule, Meter, PrincipalId, ProgramId,
    ProgramInstance, ResourceBudget, Storage, UnavailableReceiptOracle, ValidatedModule,
    ValidationLimits, WasmEngine, WasmValue, ABI_V1_VERSION, ABI_V2_VERSION,
};
use layerx_programs_sandbox::{
    restore, Lease, LeaseActivity, LeaseCapabilities, LeaseId, LeaseLimits, LeaseState,
    LeaseTransition, SandboxState, Snapshot, SnapshotRefusal, TransitionEvidence,
};

use layerx_programs_sandbox::snapshot::{RestoreRequest, RestoredSandbox};

const ENTRYPOINT: &[u8] = b"sandbox_transition";
const CALL_PREFIX: usize = 106;
const VALIDATOR_SEEDS: [[u8; 32]; 3] = [[1; 32], [2; 32], [3; 32]];

fn candidate(id: u8, tenant: u8, opened: u64, expiry: u64) -> Lease {
    let image = layerx_programs_runtime::hash_bytes(
        layerx_programs_runtime::HashAlgorithm::Sha256,
        &continuation_module(),
    )
    .unwrap();
    candidate_with_image(id, tenant, opened, expiry, image)
}

fn candidate_with_image(id: u8, tenant: u8, opened: u64, expiry: u64, image: [u8; 32]) -> Lease {
    Lease::request(
        LeaseId::new([id; 32]).unwrap_or_else(|error| panic!("lease id: {error}")),
        PrincipalId::new([tenant; 32]).unwrap_or_else(|error| panic!("tenant: {error}")),
        ProgramId::new([3; 32]).unwrap_or_else(|error| panic!("host: {error}")),
        image,
        [4; 32],
        10_000,
        LeaseLimits {
            cpu_fuel: 10_000,
            memory_bytes: 1 << 20,
            storage_read_bytes: 1 << 20,
            storage_write_bytes: 1 << 20,
            output_values: 1_000,
            output_bytes: 1 << 20,
            table_elements: 1_000,
            namespace_bytes: 1 << 20,
        },
        opened,
        expiry,
    )
    .unwrap_or_else(|error| panic!("lease: {error}"))
}

fn candidate_namespace(id: u8, tenant: u8, namespace_bytes: u64) -> Lease {
    Lease::request(
        LeaseId::new([id; 32]).unwrap_or_else(|error| panic!("lease id: {error}")),
        PrincipalId::new([tenant; 32]).unwrap_or_else(|error| panic!("tenant: {error}")),
        ProgramId::new([3; 32]).unwrap_or_else(|error| panic!("host: {error}")),
        layerx_programs_runtime::hash_bytes(
            layerx_programs_runtime::HashAlgorithm::Sha256,
            &continuation_module(),
        )
        .unwrap_or_else(|error| panic!("hash: {error}")),
        [4; 32],
        10_000,
        LeaseLimits {
            cpu_fuel: 10_000,
            memory_bytes: 1 << 20,
            storage_read_bytes: 1 << 20,
            storage_write_bytes: 1 << 20,
            output_values: 1_000,
            output_bytes: 1 << 20,
            table_elements: 1_000,
            namespace_bytes,
        },
        10,
        30,
    )
    .unwrap_or_else(|error| panic!("lease: {error}"))
}

fn call_payload(host: ProgramId, transition: LeaseTransition) -> Vec<u8> {
    let mut calldata = vec![0; 101];
    calldata[..2].copy_from_slice(&1u16.to_be_bytes());
    calldata[2..34].copy_from_slice(&transition.lease.bytes());
    calldata[34..66].copy_from_slice(&transition.tenant.bytes());
    calldata[66] = transition.activity as u8;
    calldata[67] = transition.from as u8;
    calldata[68] = transition.to as u8;
    calldata[69..].copy_from_slice(&transition.usage_observation_digest);
    let mut payload = vec![0; CALL_PREFIX];
    payload[..32].copy_from_slice(&host.bytes());
    payload[32..34].copy_from_slice(&2u16.to_be_bytes());
    payload[34..36].copy_from_slice(
        &u16::try_from(ENTRYPOINT.len())
            .unwrap_or_else(|error| panic!("entrypoint length: {error}"))
            .to_be_bytes(),
    );
    payload[36..40].copy_from_slice(
        &u32::try_from(calldata.len())
            .unwrap_or_else(|error| panic!("calldata length: {error}"))
            .to_be_bytes(),
    );
    payload[46..50].copy_from_slice(&1u32.to_be_bytes());
    payload.extend_from_slice(ENTRYPOINT);
    payload.extend_from_slice(&calldata);
    payload
}

fn proven_transition(
    lease: &Lease,
    mut transition: LeaseTransition,
    block_height: u64,
) -> (LeaseTransition, Vec<u8>, chain::ChainFixture) {
    transition.activity_id = Sha256::digest(call_payload(lease.host_program(), transition)).into();
    let mut canonical = lease
        .canonical_state_bytes()
        .unwrap_or_else(|error| panic!("lease state: {error}"));
    // The fixed fields precede the state tag and the length-prefixed receipt history.
    let state_offset =
        b"LayerX/programs/sandbox/lease-state/v3\0".len() + 8 * 32 + 16 + 18 * 8 + 4 + 7 * 8 + 16;
    canonical[state_offset] = transition.to as u8;
    canonical[state_offset + 1] = u8::try_from(lease.history().len() + 1)
        .unwrap_or_else(|error| panic!("history length: {error}"));
    let history_end = state_offset + 2 + lease.history().len() * (3 + 3 * 32 + 8);
    let mut receipt = vec![
        transition.activity as u8,
        transition.from as u8,
        transition.to as u8,
    ];
    receipt.extend_from_slice(&transition.activity_id);
    receipt.extend_from_slice(&transition.usage_observation_digest);
    let mut digest = Sha256::new();
    digest.update(lease.id().bytes());
    digest.update(&receipt);
    digest.update(block_height.to_be_bytes());
    receipt.extend_from_slice(&digest.finalize());
    receipt.extend_from_slice(&block_height.to_be_bytes());
    canonical.splice(history_end..history_end, receipt);
    let key = layerx_programs_sandbox::sandbox_state_key(0, lease.id().bytes())
        .unwrap_or_else(|error| panic!("lease key: {error}"));
    let mut fixture = chain::ChainFixture::new_in_store(
        "lxphost",
        "paxeer-lease-chain",
        &VALIDATOR_SEEDS,
        vec![(key, canonical.clone())],
    )
    .unwrap_or_else(|error| panic!("chain: {error:?}"));
    assert!(block_height > 0, "state height must be positive");
    fixture.header.height = i64::try_from(block_height)
        .ok()
        .and_then(|height| height.checked_add(1))
        .unwrap_or_else(|| panic!("block height overflow"));
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture
        .header
        .hash()
        .unwrap_or_else(|error| panic!("header hash: {error:?}"))
        .to_vec();
    let keys = VALIDATOR_SEEDS.map(|seed| SigningKey::from_bytes(&seed));
    for signature in &mut fixture.commit.signatures {
        let key = keys
            .iter()
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .unwrap_or_else(|| panic!("validator signing key absent"));
        let bytes = vote_sign_bytes(
            &fixture.header.chain_id,
            fixture.commit.height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            signature.timestamp,
        )
        .unwrap_or_else(|error| panic!("vote sign bytes: {error:?}"));
        signature.signature = key.sign(&bytes).to_bytes().to_vec();
    }
    (transition, canonical, fixture)
}

fn evidence(
    lease: &Lease,
    transition: LeaseTransition,
    block_height: u64,
) -> (LeaseTransition, TransitionEvidence) {
    let (transition, canonical, fixture) = proven_transition(lease, transition, block_height);
    let key = layerx_programs_sandbox::sandbox_state_key(0, lease.id().bytes())
        .unwrap_or_else(|error| panic!("lease key: {error}"));
    let commitment = decode_commitment_proof(
        &fixture
            .encode_key(&key)
            .unwrap_or_else(|error| panic!("encode lease commitment: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("decode lease commitment: {error:?}"));
    assert_eq!(commitment.store_name, "lxphost");
    assert_eq!(commitment.key, key);
    assert_eq!(commitment.value.as_deref(), Some(canonical.as_slice()));
    let header = ChainHeader::decode(&commitment.signed_header)
        .unwrap_or_else(|error| panic!("decode header: {error:?}"));
    let head = fixture
        .verifier
        .verify_head(&header, &fixture.commit)
        .unwrap_or_else(|error| panic!("head: {error:?}"));
    let call_evidence = TransitionEvidence::verify_call(
        &head,
        lease,
        transition,
        &AuthorizationContext::new(lease.tenant(), CapabilitySet::empty()),
        commitment
            .value
            .as_deref()
            .unwrap_or_else(|| panic!("lease commitment value absent")),
        &commitment.proof_ops,
    )
    .unwrap_or_else(|error| panic!("evidence: {error}"));
    assert_eq!(call_evidence.state_height(), block_height);
    assert_eq!(call_evidence.tx_hash(), transition.activity_id);
    (transition, call_evidence)
}

fn advance(mut lease: Lease, edges: &[(LeaseActivity, LeaseState, LeaseState, u64)]) -> Lease {
    let request = LeaseTransition {
        lease: lease.id(),
        tenant: lease.tenant(),
        activity: LeaseActivity::Request,
        from: LeaseState::Requested,
        to: LeaseState::Requested,
        activity_id: [0; 32],
        usage_observation_digest: lease
            .request_binding_digest()
            .unwrap_or_else(|error| panic!("request digest: {error}")),
    };
    let (request, proof) = evidence(&lease, request, lease.opened_at());
    let mut book = layerx_programs_sandbox::LeaseBook::new();
    book.insert_requested(lease, request, proof)
        .unwrap_or_else(|error| panic!("request: {error}"));
    for &(activity, from, to, block_height) in edges {
        lease = book
            .get(request.lease)
            .unwrap_or_else(|| panic!("lease"))
            .clone();
        let transition = LeaseTransition {
            lease: lease.id(),
            tenant: lease.tenant(),
            activity,
            from,
            to,
            activity_id: [0; 32],
            usage_observation_digest: [0; 32],
        };
        let (transition, proof) = evidence(&lease, transition, block_height);
        book.transition(lease.id(), transition, proof)
            .unwrap_or_else(|error| panic!("advance: {error}"));
    }
    book.get(request.lease)
        .unwrap_or_else(|| panic!("advanced lease"))
        .clone()
}

fn advance_existing(
    mut lease: Lease,
    edges: &[(LeaseActivity, LeaseState, LeaseState, u64)],
) -> Lease {
    for &(activity, from, to, block_height) in edges {
        let transition = LeaseTransition {
            lease: lease.id(),
            tenant: lease.tenant(),
            activity,
            from,
            to,
            activity_id: [0; 32],
            usage_observation_digest: [0; 32],
        };
        let (transition, proof) = evidence(&lease, transition, block_height);
        lease
            .transition(transition, proof)
            .unwrap_or_else(|error| panic!("advance existing: {error}"));
    }
    lease
}

fn active(id: u8, tenant: u8) -> Lease {
    advance(
        candidate(id, tenant, 10, 30),
        &[
            (
                LeaseActivity::Fund,
                LeaseState::Requested,
                LeaseState::Funded,
                11,
            ),
            (
                LeaseActivity::Activate,
                LeaseState::Funded,
                LeaseState::Active,
                12,
            ),
        ],
    )
}

fn funded(id: u8, tenant: u8) -> Lease {
    advance(
        candidate(id, tenant, 20, 40),
        &[(
            LeaseActivity::Fund,
            LeaseState::Requested,
            LeaseState::Funded,
            21,
        )],
    )
}

fn activation(lease: &Lease, block_height: u64) -> (LeaseTransition, TransitionEvidence) {
    evidence(
        lease,
        LeaseTransition {
            lease: lease.id(),
            tenant: lease.tenant(),
            activity: LeaseActivity::Activate,
            from: LeaseState::Funded,
            to: LeaseState::Active,
            activity_id: [0; 32],
            usage_observation_digest: [0; 32],
        },
        block_height,
    )
}

fn meter() -> Meter {
    Meter::new(
        ResourceBudget::new_complete(1_000, 1 << 20, 1 << 20, 1 << 20, 100, 1 << 20, 100),
        FeeSchedule::declared(),
    )
}

fn continuation_module() -> Vec<u8> {
    continuation_module_with_global(true)
}

fn continuation_module_with_global(export_global: bool) -> Vec<u8> {
    let global_section = raw_section(6, &[1, TYPE_I32, 1, OP_I32_CONST, 0, OP_END]);
    let memory_section = raw_section(5, &[1, 0, 1]);
    let declarations = if export_global {
        vec![
            ("seed", 0, 0),
            ("continue", 0, 1),
            ("memory", 2, 0),
            ("counter", 3, 0),
        ]
    } else {
        vec![("seed", 0, 0), ("continue", 0, 1), ("memory", 2, 0)]
    };
    let mut exports = unsigned_leb(declarations.len() as u64);
    for (name, kind, index) in declarations {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.push(kind);
        exports.extend(unsigned_leb(index));
    }
    module(&[
        type_section(&[(&[], &[TYPE_I32])]),
        function_section(&[0, 0]),
        memory_section,
        global_section,
        raw_section(7, &exports),
        code_section(&[
            func_body(&[], &[OP_I32_CONST, 41, 0x24, 0, OP_I32_CONST, 41, OP_END]),
            func_body(
                &[],
                &[
                    0x23,
                    0,
                    OP_I32_CONST,
                    1,
                    OP_I32_ADD,
                    0x24,
                    0,
                    0x23,
                    0,
                    OP_END,
                ],
            ),
        ]),
    ])
}

fn validated_continuation() -> layerx_programs_runtime::ValidatedModule {
    WasmEngine::new(ValidationLimits::declared())
        .unwrap_or_else(|error| panic!("engine: {error}"))
        .validate(&continuation_module())
        .unwrap_or_else(|error| panic!("validation: {error}"))
}

fn running_instance(
    lease: &Lease,
    module: &layerx_programs_runtime::ValidatedModule,
    storage: Storage,
) -> layerx_programs_runtime::ProgramInstance {
    let capabilities = LeaseCapabilities::derive(lease).unwrap();
    assert_eq!(
        capabilities.namespace(),
        lease.namespace().storage_namespace().unwrap()
    );
    assert_ne!(capabilities.principal(), lease.tenant());
    let authorization =
        AuthorizationContext::new(capabilities.principal(), capabilities.grants().clone());
    let abi_version = match module.abi_revision() {
        layerx_programs_runtime::AbiRevision::V1 => ABI_V1_VERSION,
        layerx_programs_runtime::AbiRevision::V2 => ABI_V2_VERSION,
        other => panic!("unexpected continuation ABI: {other:?}"),
    };
    let abi = Abi::new(
        abi_version,
        lease.host_program(),
        authorization,
        storage,
        &UnavailableReceiptOracle,
    )
    .unwrap_or_else(|error| panic!("abi: {error}"));
    module
        .instantiate_sandbox(meter(), abi)
        .unwrap_or_else(|error| panic!("sandbox instance: {error}"))
}

fn snapshot_transition(
    lease: &Lease,
    digest: [u8; 32],
    block_height: u64,
) -> (LeaseTransition, TransitionEvidence) {
    evidence(
        lease,
        LeaseTransition {
            lease: lease.id(),
            tenant: lease.tenant(),
            activity: LeaseActivity::Snapshot,
            from: LeaseState::Active,
            to: LeaseState::Active,
            activity_id: [0; 32],
            usage_observation_digest: digest,
        },
        block_height,
    )
}

#[test]
fn snapshot_destroy_restore_and_continue_preserves_exact_execution_state() {
    let mut source = active(1, 9);
    let module = validated_continuation();
    let (mut uninterrupted_instance, snapshot, uninterrupted, mut source_storage) =
        capture_seeded_snapshot(&mut source, &module);
    let restarted_snapshot = verify_snapshot_restart(&source, &snapshot, &source_storage);

    let restarted_destroyed = destroy_snapshot_source(source, &mut source_storage, &snapshot);
    let mut target = funded(2, 9);
    let mut target_storage = Storage::new();
    let (activate, activation_evidence) = activation(&target, 22);
    let mut restore_meter = meter();
    let target_authorization = AuthorizationContext::new(target.tenant(), CapabilitySet::empty());
    let request = RestoreRequest {
        snapshot_digest: snapshot.digest(),
        supplied: restarted_snapshot,
        authorization: target_authorization,
        module: &module,
        activation: activate,
        evidence: activation_evidence,
    };
    verify_restore_bounds(
        &restarted_destroyed,
        &mut target,
        &mut target_storage,
        &request,
        &snapshot,
    );
    let restored = restore(
        &restarted_destroyed,
        &mut target,
        &mut target_storage,
        &mut restore_meter,
        request,
    )
    .unwrap_or_else(|error| panic!("restore: {error}"));
    assert_eq!(
        restored.state().linear_memory(),
        uninterrupted.linear_memory()
    );
    assert_eq!(restored.state().globals(), uninterrupted.globals());
    assert_eq!(
        restored.state().continuation(),
        uninterrupted.continuation()
    );
    assert_eq!(restored.state().source_lease(), target.id());
    assert_eq!(restored.state().source_namespace(), target.namespace());
    assert_eq!(target.state(), LeaseState::Active);
    assert_eq!(target.restored_from(), Some(snapshot.digest()));
    let restore_usage = restore_meter
        .finish()
        .unwrap_or_else(|error| panic!("usage: {error}"));
    assert!(restore_usage.storage_write_bytes > snapshot.storage_bytes());
    assert_eq!(
        restore_usage.storage_write_bytes,
        snapshot.byte_length() + 65_536 + 16 + 15
    );

    verify_continuation(&mut uninterrupted_instance, &restored);
}

#[test]
fn ownership_digest_and_target_bindings_are_refused_before_metering() {
    let mut source = active(3, 9);
    let module = validated_continuation();
    let mut source_instance = running_instance(&source, &module, Storage::new());
    source_instance
        .call("seed", &[])
        .unwrap_or_else(|error| panic!("seed: {error}"));
    let canonical_capture = Snapshot::capture(&source, &mut source_instance, "continue", &[])
        .unwrap_or_else(|error| panic!("capture: {error}"));
    let (edge, proof) = snapshot_transition(
        &source,
        canonical_capture
            .digest()
            .unwrap_or_else(|error| panic!("digest: {error}")),
        13,
    );
    let snapshot = Snapshot::commit(
        &mut source,
        &mut source_instance,
        canonical_capture,
        edge,
        proof,
    )
    .unwrap_or_else(|error| panic!("snapshot: {error}"));
    let source_storage = source_instance
        .storage_snapshot()
        .unwrap_or_else(|| panic!("storage"));
    let mut target = funded(4, 9);
    let mut target_storage = Storage::new();
    let intruder = PrincipalId::new([8; 32]).unwrap_or_else(|error| panic!("intruder: {error}"));
    let mut intruder_meter = meter();
    let (activate, activation_evidence) = activation(&target, 22);
    let intruder_authorization = AuthorizationContext::new(intruder, CapabilitySet::empty());
    assert!(matches!(
        restore(
            &source,
            &mut target,
            &mut target_storage,
            &mut intruder_meter,
            layerx_programs_sandbox::snapshot::RestoreRequest {
                snapshot_digest: snapshot.digest(),
                supplied: snapshot.state().clone(),
                authorization: intruder_authorization,
                module: &module,
                activation: activate,
                evidence: activation_evidence,
            },
        ),
        Err(SnapshotRefusal::NotSnapshotOwner)
    ));
    assert_eq!(
        intruder_meter
            .finish()
            .map(|usage| usage.storage_write_bytes),
        Ok(0)
    );

    let mut altered_instance = running_instance(&source, &module, source_storage.clone());
    let altered = Snapshot::capture(&source, &mut altered_instance, "continue", &[])
        .unwrap_or_else(|error| panic!("altered: {error}"));
    let mut mismatch_meter = meter();
    let (activate, activation_evidence) = activation(&target, 22);
    let target_authorization = AuthorizationContext::new(target.tenant(), CapabilitySet::empty());
    assert!(matches!(
        restore(
            &source,
            &mut target,
            &mut target_storage,
            &mut mismatch_meter,
            layerx_programs_sandbox::snapshot::RestoreRequest {
                snapshot_digest: snapshot.digest(),
                supplied: altered.state().clone(),
                authorization: target_authorization,
                module: &module,
                activation: activate,
                evidence: activation_evidence,
            },
        ),
        Err(SnapshotRefusal::DigestMismatch)
    ));
    assert_eq!(
        mismatch_meter
            .finish()
            .map(|usage| usage.storage_write_bytes),
        Ok(0)
    );

    verify_foreign_target_refusal(&source, &module, &snapshot);
}

#[test]
fn canonical_integer_discriminants_distinguish_runtime_value_widths() {
    let source = active(7, 9);
    let module = validated_continuation();
    let mut instance = running_instance(&source, &module, Storage::new());
    let i32_state = Snapshot::capture(&source, &mut instance, "continue", &[WasmValue::I32(1)])
        .unwrap_or_else(|error| panic!("i32 state: {error}"));
    let i64_state = Snapshot::capture(&source, &mut instance, "continue", &[WasmValue::I64(1)])
        .unwrap_or_else(|error| panic!("i64 state: {error}"));
    assert_ne!(
        i32_state.state().canonical_bytes(),
        i64_state.state().canonical_bytes()
    );
    assert_ne!(i32_state.digest(), i64_state.digest());
}

#[test]
fn aggregate_snapshot_chunks_never_cross_the_lease_namespace_ceiling() {
    let mut lease = advance(
        candidate_namespace(8, 9, 1_024),
        &[
            (
                LeaseActivity::Fund,
                LeaseState::Requested,
                LeaseState::Funded,
                11,
            ),
            (
                LeaseActivity::Activate,
                LeaseState::Funded,
                LeaseState::Active,
                12,
            ),
        ],
    );
    let mut storage = Storage::new();
    let module = validated_continuation();
    let mut instance = running_instance(&lease, &module, storage);
    let mut refused = false;
    for index in 0u8..64 {
        let state = Snapshot::capture(
            &lease,
            &mut instance,
            "continue",
            &[WasmValue::I32(i32::from(index))],
        )
        .unwrap_or_else(|error| panic!("state: {error}"));
        let (edge, proof) = snapshot_transition(
            &lease,
            state
                .digest()
                .unwrap_or_else(|error| panic!("digest: {error}")),
            13 + u64::from(index),
        );
        match Snapshot::commit(&mut lease, &mut instance, state, edge, proof) {
            Ok(_) => {}
            Err(SnapshotRefusal::TargetBoundExceeded) => {
                refused = true;
                break;
            }
            Err(error) => panic!("unexpected snapshot refusal: {error}"),
        }
    }
    storage = instance
        .storage_snapshot()
        .unwrap_or_else(|| panic!("storage"));
    assert!(refused);
    assert!(
        storage
            .namespace_persistent_bytes(
                lease
                    .namespace()
                    .storage_namespace()
                    .unwrap_or_else(|error| panic!("namespace: {error}"))
            )
            .unwrap_or_else(|error| panic!("occupancy: {error}"))
            + storage
                .protocol_prefix_bytes(lease.namespace().snapshot_storage_namespace(), b"snapshot")
                .unwrap_or_else(|error| panic!("snapshot occupancy: {error}"))
            <= lease.limits().namespace_bytes
    );
}

#[test]
fn continuation_capture_fails_closed_when_a_mutable_global_is_hidden() {
    let module = WasmEngine::new(ValidationLimits::declared())
        .unwrap_or_else(|error| panic!("engine: {error}"))
        .validate(&continuation_module_with_global(false))
        .unwrap_or_else(|error| panic!("validation: {error}"));
    let mut instance = module
        .instantiate()
        .unwrap_or_else(|error| panic!("instance: {error}"));
    assert!(matches!(
        instance.capture_continuation("continue", &[]),
        Err(layerx_programs_runtime::ExecutionFault::EngineFault { .. })
    ));
}

fn capture_seeded_snapshot(
    source: &mut Lease,
    module: &ValidatedModule,
) -> (ProgramInstance, Snapshot, SandboxState, Storage) {
    let mut source_storage = Storage::new();
    let namespace = source
        .namespace()
        .storage_namespace()
        .unwrap_or_else(|error| panic!("namespace: {error}"));
    let live_key = b"counter".to_vec();
    let mut transaction = source_storage.transaction(namespace);
    transaction
        .write(&live_key, &41u64.to_be_bytes())
        .unwrap_or_else(|error| panic!("live write: {error}"));
    assert_eq!(transaction.commit(), 1);
    let mut uninterrupted_instance = running_instance(source, module, source_storage);
    assert_eq!(
        uninterrupted_instance.call("seed", &[]),
        Ok(vec![WasmValue::I32(41)])
    );
    let uninterrupted_capture =
        Snapshot::capture(source, &mut uninterrupted_instance, "continue", &[])
            .unwrap_or_else(|error| panic!("sandbox state: {error}"));
    let uninterrupted = uninterrupted_capture.state().clone();
    let (snapshot_edge, snapshot_evidence) = snapshot_transition(
        source,
        uninterrupted_capture
            .digest()
            .unwrap_or_else(|error| panic!("digest: {error}")),
        13,
    );
    let snapshot = Snapshot::commit(
        source,
        &mut uninterrupted_instance,
        uninterrupted_capture,
        snapshot_edge,
        snapshot_evidence,
    )
    .unwrap_or_else(|error| panic!("snapshot: {error}"));
    let snapshot_usage = uninterrupted_instance
        .meter()
        .finish()
        .unwrap_or_else(|error| panic!("usage: {error}"));
    assert!(snapshot_usage.storage_write_bytes >= snapshot.storage_bytes());
    let source_storage = uninterrupted_instance
        .storage_snapshot()
        .unwrap_or_else(|| panic!("runtime storage"));
    (
        uninterrupted_instance,
        snapshot,
        uninterrupted,
        source_storage,
    )
}

fn verify_snapshot_restart(
    source: &Lease,
    snapshot: &Snapshot,
    source_storage: &Storage,
) -> SandboxState {
    assert_eq!(source.snapshot_records().len(), 1);
    assert_eq!(source.snapshot_records()[0].digest(), snapshot.digest());
    assert_eq!(source.snapshot_records()[0].namespace(), source.namespace());
    assert_eq!(
        source.snapshot_records()[0].byte_length(),
        snapshot.byte_length()
    );
    assert!(source.snapshot_records()[0].chunk_count() > 0);
    assert_eq!(source.snapshot_records()[0].owner(), source.tenant());
    assert_eq!(source.snapshot_records()[0].source_lease(), source.id());
    assert_eq!(
        source.snapshot_records()[0].host_program(),
        source.host_program()
    );
    assert_eq!(
        source.snapshot_records()[0].image_code_hash(),
        source.image_code_hash()
    );
    let restarted_source = Lease::decode_state(
        &source
            .canonical_state_bytes()
            .unwrap_or_else(|error| panic!("source state: {error}")),
    )
    .unwrap_or_else(|error| panic!("restart lease: {error}"));
    assert_eq!(
        restarted_source.snapshot_records(),
        source.snapshot_records()
    );
    let restarted_snapshot = SandboxState::from_canonical(
        &restarted_source,
        &snapshot
            .state()
            .canonical_bytes()
            .unwrap_or_else(|error| panic!("snapshot bytes: {error}")),
    )
    .unwrap_or_else(|error| panic!("restart snapshot: {error}"));
    assert_eq!(restarted_snapshot, *snapshot.state());
    assert!(
        source_storage
            .namespace_persistent_bytes(
                source
                    .namespace()
                    .storage_namespace()
                    .unwrap_or_else(|error| panic!("namespace: {error}"))
            )
            .unwrap_or_else(|error| panic!("source occupancy: {error}"))
            > 0
    );

    restarted_snapshot
}

fn destroy_snapshot_source(
    source: Lease,
    source_storage: &mut Storage,
    snapshot: &Snapshot,
) -> Lease {
    let mut destroyed = advance_existing(
        source,
        &[
            (
                LeaseActivity::BeginSettlement,
                LeaseState::Active,
                LeaseState::Settling,
                14,
            ),
            (
                LeaseActivity::Expire,
                LeaseState::Settling,
                LeaseState::Expired,
                15,
            ),
        ],
    );
    let (destroy, destroy_evidence) = evidence(
        &destroyed,
        LeaseTransition {
            lease: destroyed.id(),
            tenant: destroyed.tenant(),
            activity: LeaseActivity::Destroy,
            from: LeaseState::Expired,
            to: LeaseState::Destroyed,
            activity_id: [0; 32],
            usage_observation_digest: [0; 32],
        },
        16,
    );
    let mut reclaim_meter = meter();
    destroyed
        .destroy_with_evidence(
            source_storage,
            &mut reclaim_meter,
            destroy,
            destroy_evidence,
        )
        .unwrap_or_else(|error| panic!("destroy: {error}"));
    assert_eq!(destroyed.state(), LeaseState::Destroyed);
    assert_eq!(destroyed.snapshot_records().len(), 1);
    assert_eq!(destroyed.snapshot_records()[0].digest(), snapshot.digest());
    assert_eq!(
        destroyed.snapshot_records()[0].namespace(),
        destroyed.namespace()
    );
    assert_eq!(
        source_storage.namespace_persistent_bytes(
            destroyed
                .namespace()
                .storage_namespace()
                .unwrap_or_else(|error| panic!("namespace: {error}"))
        ),
        Ok(0)
    );
    let restarted_destroyed = Lease::decode_state(
        &destroyed
            .canonical_state_bytes()
            .unwrap_or_else(|error| panic!("destroyed state: {error}")),
    )
    .unwrap_or_else(|error| panic!("restart destroyed: {error}"));
    restarted_destroyed
}

fn verify_restore_bounds(
    source: &Lease,
    target: &mut Lease,
    target_storage: &mut Storage,
    request: &RestoreRequest<'_>,
    snapshot: &Snapshot,
) {
    let reconstructed_bytes = snapshot.byte_length() + 15;
    let total_restored_bytes = reconstructed_bytes + 65_536 + 16;
    for (attempted, runtime_copy) in [(reconstructed_bytes, false), (total_restored_bytes, true)] {
        let limit = attempted - 1;
        let mut bounded_meter = Meter::new(
            ResourceBudget::new_complete(1_000, 1 << 20, 1 << 20, limit, 100, 1 << 20, 100),
            FeeSchedule::declared(),
        );
        let target_before = target.clone();
        let storage_before = target_storage.clone();
        let meter_before = bounded_meter.clone();
        let refusal = restore(
            source,
            target,
            target_storage,
            &mut bounded_meter,
            layerx_programs_sandbox::snapshot::RestoreRequest {
                snapshot_digest: request.snapshot_digest,
                supplied: request.supplied.clone(),
                authorization: request.authorization.clone(),
                module: request.module,
                activation: request.activation,
                evidence: request.evidence,
            },
        )
        .err();
        let expected = layerx_programs_runtime::MeterRefusal::BudgetExceeded {
            resource: layerx_programs_runtime::ResourceKind::StorageWrite,
            limit,
            attempted,
        };
        if runtime_copy {
            assert_eq!(
                refusal,
                Some(SnapshotRefusal::Runtime(
                    layerx_programs_runtime::ExecutionFault::Resource { refusal: expected }
                ))
            );
        } else {
            assert_eq!(refusal, Some(SnapshotRefusal::Meter(expected)));
        }
        assert_eq!(*target, target_before);
        assert_eq!(*target_storage, storage_before);
        assert_eq!(bounded_meter.finish(), meter_before.finish());
    }
}

fn verify_continuation(uninterrupted_instance: &mut ProgramInstance, restored: &RestoredSandbox) {
    let uninterrupted_before = uninterrupted_instance
        .meter()
        .finish()
        .unwrap_or_else(|error| panic!("uninterrupted before: {error}"));
    let uninterrupted_output = uninterrupted_instance
        .call("continue", &[])
        .unwrap_or_else(|error| panic!("uninterrupted continuation: {error}"));
    let uninterrupted_after = uninterrupted_instance
        .meter()
        .finish()
        .unwrap_or_else(|error| panic!("uninterrupted after: {error}"));
    let restored_output = restored.outputs();
    let execution_usage = restored
        .instance()
        .meter()
        .finish()
        .unwrap_or_else(|error| panic!("restored runtime usage: {error}"));
    assert_eq!(restored_output, uninterrupted_output);
    assert_eq!(restored_output, [WasmValue::I32(42)]);
    let uninterrupted_delta = (
        uninterrupted_after.cpu_fuel - uninterrupted_before.cpu_fuel,
        uninterrupted_after.memory_bytes - uninterrupted_before.memory_bytes,
        uninterrupted_after.storage_read_bytes - uninterrupted_before.storage_read_bytes,
        uninterrupted_after.storage_write_bytes - uninterrupted_before.storage_write_bytes,
        uninterrupted_after.output_values - uninterrupted_before.output_values,
        uninterrupted_after.output_bytes - uninterrupted_before.output_bytes,
    );
    assert_eq!(execution_usage.cpu_fuel, uninterrupted_delta.0);
}

fn verify_foreign_target_refusal(source: &Lease, module: &ValidatedModule, snapshot: &Snapshot) {
    let mut foreign = funded(5, 8);
    let mut foreign_storage = Storage::new();
    let mut foreign_meter = meter();
    let (activate, activation_evidence) = activation(&foreign, 22);
    let foreign_authorization = AuthorizationContext::new(foreign.tenant(), CapabilitySet::empty());
    assert!(matches!(
        restore(
            source,
            &mut foreign,
            &mut foreign_storage,
            &mut foreign_meter,
            layerx_programs_sandbox::snapshot::RestoreRequest {
                snapshot_digest: snapshot.digest(),
                supplied: snapshot.state().clone(),
                authorization: foreign_authorization,
                module,
                activation: activate,
                evidence: activation_evidence,
            },
        ),
        Err(SnapshotRefusal::NotSnapshotOwner)
    ));
}

fn storage_continuation_module() -> Vec<u8> {
    use layerx_programs_runtime::test_support::{import_section, TYPE_I64};
    let imports = [
        ("layerx_v1", "storage_read", 0),
        ("layerx_v1", "storage_write", 0),
        ("layerx_v1", "transfer_402", 1),
        ("layerx_v1", "event_emit", 0),
        ("layerx_v2", "storage_read_scoped", 2),
        ("layerx_v2", "storage_write_scoped", 2),
        ("layerx_v1", "program_call", 3),
    ];
    fn constant(mut value: i64, body: &mut Vec<u8>) {
        body.push(OP_I32_CONST);
        loop {
            let mut byte = (value as u8) & 127;
            value >>= 7;
            let done = (value == 0 && byte & 64 == 0) || (value == -1 && byte & 64 != 0);
            if !done {
                byte |= 128;
            }
            body.push(byte);
            if done {
                break;
            }
        }
    }
    fn check(index: u8, args: &[i64], expected: i64, body: &mut Vec<u8>) {
        for arg in args {
            constant(*arg, body);
        }
        body.extend([0x10, index]);
        constant(expected, body);
        body.extend([0x46, OP_I32_ADD]);
    }
    let mut body = Vec::new();
    constant(0, &mut body);
    check(0, &[0, 1, 128, 1], 2, &mut body);
    // Increment the byte actually read through the ABI, keeping it for the result.
    constant(64, &mut body);
    constant(128, &mut body);
    body.extend([0x2d, 0, 0]);
    constant(1, &mut body);
    body.extend([OP_I32_ADD, 0x3a, 0, 0]);
    check(1, &[0, 1, 64, 1], 0, &mut body);
    body.extend([0x42, 0, 0x42, 1]);
    for arg in [32, 32, 32, 32] {
        constant(arg, &mut body);
    }
    body.extend([0x10, 2]);
    constant(-1, &mut body);
    body.extend([0x46, OP_I32_ADD]);
    check(3, &[0, 1, 0, 1], -1, &mut body);
    check(4, &[2, 0, 1, 128, 1], -1, &mut body);
    check(5, &[2, 0, 1, 64, 1], -1, &mut body);
    check(6, &[32, 32, 0, 0, 96, 2], -1, &mut body);
    constant(128, &mut body);
    body.extend([0x2d, 0, 0]);
    constant(16, &mut body);
    body.extend([0x6c, OP_I32_ADD, OP_END]);
    let mut data = vec![0u8; 98];
    data[0] = b'k';
    data[32..64].fill(8);
    let mut segment = vec![1, 0, OP_I32_CONST, 0, OP_END];
    segment.extend(unsigned_leb(data.len() as u64));
    segment.extend(data);
    let mut exports = vec![2];
    for (name, kind, index) in [("continue", 0, 7), ("memory", 2, 0)] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.push(kind);
        exports.push(index);
    }
    module(&[
        type_section(&[
            (&[TYPE_I32; 4], &[TYPE_I32]),
            (
                &[TYPE_I64, TYPE_I64, TYPE_I32, TYPE_I32, TYPE_I32, TYPE_I32],
                &[TYPE_I32],
            ),
            (&[TYPE_I32; 5], &[TYPE_I32]),
            (&[TYPE_I32; 6], &[TYPE_I32]),
            (&[], &[TYPE_I32]),
        ]),
        import_section(&imports),
        function_section(&[4]),
        raw_section(5, &[1, 0, 1]),
        raw_section(7, &exports),
        code_section(&[func_body(&[], &body)]),
        raw_section(11, &segment),
    ])
}

#[test]
fn restore_lease_storage_continues_after_reopen_without_injected_authority() {
    use layerx_programs_runtime::{Capability, StorageNamespace};
    let module = WasmEngine::new(ValidationLimits::declared())
        .unwrap()
        .validate_v2(&storage_continuation_module())
        .unwrap();
    let mut source = advance(
        candidate_with_image(81, 9, 10, 30, module.code_hash()),
        &[
            (
                LeaseActivity::Fund,
                LeaseState::Requested,
                LeaseState::Funded,
                11,
            ),
            (
                LeaseActivity::Activate,
                LeaseState::Funded,
                LeaseState::Active,
                12,
            ),
        ],
    );
    let target = advance(
        candidate_with_image(82, 9, 20, 40, module.code_hash()),
        &[(
            LeaseActivity::Fund,
            LeaseState::Requested,
            LeaseState::Funded,
            21,
        )],
    );
    let source_namespace = LeaseCapabilities::derive(&source).unwrap().namespace();
    let target_namespace = LeaseCapabilities::derive(&target).unwrap().namespace();
    let tenant_namespace = StorageNamespace::principal(source.host_program(), source.tenant());
    let shared_namespace = StorageNamespace::shared(source.host_program());
    let neighbour_namespace =
        StorageNamespace::principal(source.host_program(), PrincipalId::new([10; 32]).unwrap());
    let mut storage = Storage::new();
    for (namespace, value) in [
        (source_namespace, 41),
        (tenant_namespace, 125),
        (shared_namespace, 126),
        (neighbour_namespace, 127),
    ] {
        let mut tx = storage.transaction(namespace);
        tx.write(b"k", &[value]).unwrap();
        assert_eq!(tx.commit(), 1);
    }
    let mut instance = running_instance(&source, &module, storage);
    let capture = Snapshot::capture(&source, &mut instance, "continue", &[]).unwrap();
    let (edge, proof) = snapshot_transition(&source, capture.digest().unwrap(), 13);
    let snapshot = Snapshot::commit(&mut source, &mut instance, capture, edge, proof).unwrap();
    let persisted_storage = instance
        .storage_snapshot()
        .unwrap()
        .replay_state_bytes(1 << 24)
        .unwrap();
    let persisted_source = source.canonical_state_bytes().unwrap();
    let persisted_snapshot = snapshot.state().canonical_bytes().unwrap();
    let digest = snapshot.digest();
    drop(instance);
    drop(snapshot);
    drop(source);
    let source = Lease::decode_state(&persisted_source).unwrap();
    let supplied = SandboxState::from_canonical(&source, &persisted_snapshot).unwrap();
    let hostile_grants = CapabilitySet::new([
        Capability::StorageRead,
        Capability::StorageWrite,
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
        Capability::EmitEvent,
        Capability::Transfer402 {
            asset: [8; 32],
            to: [8; 32],
            maximum_amount: 100,
        },
        Capability::Call {
            program: ProgramId::new([8; 32]).unwrap(),
        },
    ])
    .unwrap();
    for grants in [CapabilitySet::empty(), hostile_grants] {
        let mut target = Lease::decode_state(&target.canonical_state_bytes().unwrap()).unwrap();
        let mut storage =
            Storage::decode_untrusted_replay_state(&persisted_storage, 1 << 24).unwrap();
        let (activation, evidence) = activation(&target, 22);
        let mut restore_meter = meter();
        let before_meter = restore_meter.clone();
        let before_storage = storage.clone();
        let before_target = target.clone();
        assert!(matches!(
            restore(
                &source,
                &mut target,
                &mut storage,
                &mut restore_meter,
                RestoreRequest {
                    snapshot_digest: digest,
                    supplied: supplied.clone(),
                    authorization: AuthorizationContext::new(
                        PrincipalId::new([10; 32]).unwrap(),
                        grants.clone()
                    ),
                    module: &module,
                    activation,
                    evidence
                }
            ),
            Err(SnapshotRefusal::NotSnapshotOwner)
        ));
        assert_eq!(target, before_target);
        assert_eq!(storage, before_storage);
        assert_eq!(restore_meter.finish(), before_meter.finish());
        let owner = target.tenant();
        let restored = restore(
            &source,
            &mut target,
            &mut storage,
            &mut restore_meter,
            RestoreRequest {
                snapshot_digest: digest,
                supplied: supplied.clone(),
                authorization: AuthorizationContext::new(owner, grants),
                module: &module,
                activation,
                evidence,
            },
        )
        .unwrap();
        // Seven ABI authorization checks and the old cell byte are independent.
        assert_eq!(restored.outputs(), &[WasmValue::I32(41 * 16 + 7)]);
        assert_eq!(target.state(), LeaseState::Active);
        assert_eq!(restored.state().source_namespace(), target.namespace());
        assert_eq!(
            storage.transaction(target_namespace).read(b"k").unwrap(),
            Some(vec![42])
        );
        for (namespace, value) in [
            (source_namespace, 41),
            (tenant_namespace, 125),
            (shared_namespace, 126),
            (neighbour_namespace, 127),
        ] {
            assert_eq!(
                storage.transaction(namespace).read(b"k").unwrap(),
                Some(vec![value])
            );
        }
        let persisted = storage.replay_state_bytes(1 << 24).unwrap();
        let lease_bytes = target.canonical_state_bytes().unwrap();
        drop(restored);
        drop(storage);
        drop(target);
        let target = Lease::decode_state(&lease_bytes).unwrap();
        let reopened = Storage::decode_untrusted_replay_state(&persisted, 1 << 24).unwrap();
        let mut resumed = running_instance(&target, &module, reopened);
        assert_eq!(
            resumed.call("continue", &[]),
            Ok(vec![WasmValue::I32(42 * 16 + 7)])
        );
        let mut committed = resumed.storage_snapshot().unwrap();
        assert_eq!(
            committed.transaction(target_namespace).read(b"k").unwrap(),
            Some(vec![43])
        );
        for (namespace, value) in [
            (source_namespace, 41),
            (tenant_namespace, 125),
            (shared_namespace, 126),
            (neighbour_namespace, 127),
        ] {
            assert_eq!(
                committed.transaction(namespace).read(b"k").unwrap(),
                Some(vec![value])
            );
        }
    }
}
