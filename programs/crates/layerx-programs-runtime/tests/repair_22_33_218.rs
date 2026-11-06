use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_runtime::{
    abi::UnavailableReceiptOracle,
    crypto::{hash::MAX_HASH_INPUT_BYTES, HashRefusal},
    hash_bytes, programs_trace_differential_gate, programs_trace_runner_artifact,
    qualification::{MAX_TRACE_RUNNER_EVIDENCE_BYTES, MAX_TRACE_RUNNER_PROVENANCE_BYTES},
    replay_recorded_execution, replay_recorded_execution_with_fee_history,
    terminal::{
        decode_terminal_payload, encode_failure_payload, ExecutionTerminal, ExecutionValue,
        FailureFields, FailureTerminal, TerminalDecodeError, TerminalDetail,
    },
    test_support::{
        add_module, code_section, execute_versioned_for_qualification, func_body, function_section,
        import_section, invoke_composed_for_qualification, module, raw_section, type_section,
        unsigned_leb, TYPE_I32,
    },
    AuthorizationContext, AuthorizedExecutionRequest, BudgetedV1FailureCause, Capability,
    CapabilitySet, CompositionContext, CompositionRules, ExecutionError, FeeSchedule,
    FeeScheduleHistory, FeeScheduleParameters, HashAlgorithm, PrincipalId, ProgramCatalog,
    ProgramId, RecordedExecution, ReplayRefusal, Storage, TracePolicy, TraceRunnerArtifact,
    TraceRunnerSubmission, TraceRunnerTrust, WasmEngine, WasmValue, ABI_MODULE, CALL_ENTRY_EXPORT,
    RUNTIME_VERSION,
};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

#[derive(Serialize, Deserialize)]
struct PersistedExecution {
    runtime: u16,
    abi: u16,
    fee: u32,
    metering: u32,
    wasm: Vec<u8>,
    export: String,
    args: [i32; 2],
    evidence: Vec<u8>,
}

#[derive(Serialize, Deserialize)]
struct PersistedJournal {
    schedules: Vec<[u64; 8]>,
    executions: Vec<PersistedExecution>,
}

fn parameters(version: u32, cpu: u64) -> FeeScheduleParameters {
    FeeScheduleParameters {
        version,
        fee_units_per_cpu_fuel: cpu,
        fee_units_per_memory_byte: 3,
        fee_units_per_storage_read_byte: 5,
        fee_units_per_storage_write_byte: 7,
        fee_units_per_output_value: 11,
        fee_units_per_output_byte: 13,
        fee_units_per_occupancy_byte_batch: 17,
    }
}

fn saved_schedule(schedule: FeeSchedule) -> [u64; 8] {
    [
        u64::from(schedule.version()),
        schedule.cpu_price(),
        schedule.memory_byte_price(),
        schedule.storage_read_byte_price(),
        schedule.storage_write_byte_price(),
        schedule.output_value_price(),
        schedule.output_byte_price(),
        schedule.occupancy_byte_batch_price(),
    ]
}

fn reopened_schedule(saved: [u64; 8]) -> FeeSchedule {
    FeeSchedule::new_complete(FeeScheduleParameters {
        version: u32::try_from(saved[0]).unwrap(),
        fee_units_per_cpu_fuel: saved[1],
        fee_units_per_memory_byte: saved[2],
        fee_units_per_storage_read_byte: saved[3],
        fee_units_per_storage_write_byte: saved[4],
        fee_units_per_output_value: saved[5],
        fee_units_per_output_byte: saved[6],
        fee_units_per_occupancy_byte_batch: saved[7],
    })
}

#[test]
fn reopens_recorded_abi_history_and_recovers_exact_evidence() {
    // Only serialized transport bytes survive this scope. The real engine,
    // validated artifacts, executor records and fee history are all stopped.
    let archive = {
        let engine = WasmEngine::declared().unwrap();
        let wasm = add_module();
        let mut history = FeeScheduleHistory::new(FeeSchedule::declared()).unwrap();
        history
            .record(FeeSchedule::new_complete(parameters(2, 19)))
            .unwrap();
        let mut executions = Vec::new();
        for abi in 1..=5 {
            let validated = engine.validate_versioned(abi, &wasm).unwrap();
            for prices in history.schedules() {
                let args = [WasmValue::I32(20), WasmValue::I32(22)];
                let record =
                    execute_versioned_for_qualification(&validated, *prices, "add", &args).unwrap();
                assert_eq!(record.abi_version, abi);
                assert_eq!(record.outputs, vec![WasmValue::I32(42)]);
                assert_eq!(
                    record.usage.fee_units,
                    u128::from(record.usage.cpu_fuel) * u128::from(prices.cpu_price())
                        + u128::from(record.usage.output_values)
                            * u128::from(prices.output_value_price())
                );
                executions.push(PersistedExecution {
                    runtime: record.runtime_version,
                    abi: record.abi_version,
                    fee: prices.version(),
                    metering: record.metering_schedule_version,
                    wasm: wasm.clone(),
                    export: "add".to_string(),
                    args: [20, 22],
                    evidence: record.canonical_evidence(),
                });
            }
        }
        // Head advances after the receipts were produced. Recovery must select
        // each recorded version, rather than this latest governed schedule.
        history
            .record(FeeSchedule::new_complete(parameters(3, 31)))
            .unwrap();
        serde_json::to_vec(&PersistedJournal {
            schedules: history
                .schedules()
                .iter()
                .copied()
                .map(saved_schedule)
                .collect(),
            executions,
        })
        .unwrap()
    };
    let recovered: PersistedJournal = serde_json::from_slice(&archive).unwrap();
    let mut history = FeeScheduleHistory::new(reopened_schedule(recovered.schedules[0])).unwrap();
    for saved in &recovered.schedules[1..] {
        history.record(reopened_schedule(*saved)).unwrap();
    }
    assert_eq!(history.schedules().last().unwrap().version(), 3);
    assert_eq!(recovered.executions.len(), 10);
    for saved in recovered.executions {
        let args = saved.args.map(WasmValue::I32);
        let record = RecordedExecution {
            runtime_version: saved.runtime,
            abi_version: saved.abi,
            fee_schedule_version: saved.fee,
            metering_schedule_version: saved.metering,
            wasm: &saved.wasm,
            export: &saved.export,
            args: &args,
        };
        assert_eq!(
            replay_recorded_execution_with_fee_history(&record, &history),
            Ok(saved.evidence.clone())
        );
        if saved.fee == FeeSchedule::declared().version() {
            assert_eq!(
                replay_recorded_execution(&record),
                Ok(saved.evidence.clone())
            );
        } else {
            assert_eq!(
                replay_recorded_execution(&record),
                Err(ReplayRefusal::UnknownFeeScheduleVersion { version: saved.fee })
            );
        }
        for (changed, refusal) in [
            (
                RecordedExecution {
                    runtime_version: RUNTIME_VERSION + 1,
                    ..record.clone()
                },
                ReplayRefusal::UnknownRuntimeVersion {
                    version: RUNTIME_VERSION + 1,
                },
            ),
            (
                RecordedExecution {
                    abi_version: 6,
                    ..record.clone()
                },
                ReplayRefusal::UnknownAbiVersion { version: 6 },
            ),
            (
                RecordedExecution {
                    abi_version: 0,
                    ..record.clone()
                },
                ReplayRefusal::UnknownAbiVersion { version: 0 },
            ),
            (
                RecordedExecution {
                    fee_schedule_version: 4,
                    ..record.clone()
                },
                ReplayRefusal::UnknownFeeScheduleVersion { version: 4 },
            ),
            (
                RecordedExecution {
                    metering_schedule_version: saved.metering + 1,
                    ..record.clone()
                },
                ReplayRefusal::UnknownMeteringScheduleVersion {
                    version: saved.metering + 1,
                },
            ),
        ] {
            assert_eq!(
                replay_recorded_execution_with_fee_history(&changed, &history),
                Err(refusal)
            );
        }
    }
}

fn submission(
    platform: &[u8],
    key: &SigningKey,
    executable: &[u8],
    manifest: &[u8],
    export: &str,
) -> (TraceRunnerSubmission, TraceRunnerTrust) {
    let artifact = programs_trace_runner_artifact(
        platform,
        executable,
        manifest,
        &add_module(),
        export,
        &[WasmValue::I32(20), WasmValue::I32(22)],
        TracePolicy::new(1, 32).unwrap(),
    )
    .unwrap();
    let digest = artifact.attestation_digest().unwrap();
    let artifact = artifact
        .attach_attestation_signature(&key.sign(&digest).to_bytes())
        .unwrap();
    let encoded = artifact.canonical_bytes().unwrap();
    assert_eq!(
        digest,
        <[u8; 32]>::from(Sha256::digest(&encoded[..encoded.len() - 64]))
    );
    let trust = TraceRunnerTrust {
        platform_identity: platform.to_vec(),
        executable_identity: Sha256::digest(executable).into(),
        build_manifest_identity: Sha256::digest(manifest).into(),
        attestation_public_key: key.verifying_key().to_bytes(),
    };
    // Producer -> canonical transport -> newly opened artifact consumer.
    let artifact = TraceRunnerArtifact::from_canonical_bytes(&encoded).unwrap();
    assert_eq!(artifact.canonical_bytes().unwrap(), encoded);
    (
        TraceRunnerSubmission {
            artifact,
            executable_bytes: executable.to_vec(),
            build_manifest_bytes: manifest.to_vec(),
        },
        trust,
    )
}

#[test]
fn host_provenance_and_attestations_cross_guest_hash_boundary() {
    let guest_limit = usize::try_from(MAX_HASH_INPUT_BYTES).unwrap();
    let first_key = SigningKey::from_bytes(&[17; 32]);
    let second_key = SigningKey::from_bytes(&[29; 32]);
    for length in [guest_limit - 1, guest_limit, guest_limit + 1] {
        let executable = vec![0x65; length];
        let manifest = vec![0x6d; length];
        let (first, first_trust) =
            submission(b"runner-a", &first_key, &executable, &manifest, "add");
        let (second, second_trust) =
            submission(b"runner-b", &second_key, &executable, &manifest, "add");
        let evidence = programs_trace_differential_gate(
            first.clone(),
            &first_trust,
            second.clone(),
            &second_trust,
        )
        .unwrap();
        assert_eq!(evidence[0], 0, "real traced execution must succeed");
        let mut tampered = first.clone();
        tampered.executable_bytes[0] ^= 1;
        assert!(programs_trace_differential_gate(
            tampered,
            &first_trust,
            second.clone(),
            &second_trust
        )
        .is_err());
        let mut tampered = first.clone();
        tampered.build_manifest_bytes[0] ^= 1;
        assert!(programs_trace_differential_gate(
            tampered,
            &first_trust,
            second.clone(),
            &second_trust
        )
        .is_err());
        let mut wire = first.artifact.canonical_bytes().unwrap();
        let last = wire.len() - 1;
        wire[last] ^= 1;
        let tampered = TraceRunnerSubmission {
            artifact: TraceRunnerArtifact::from_canonical_bytes(&wire).unwrap(),
            ..first.clone()
        };
        assert!(programs_trace_differential_gate(
            tampered,
            &first_trust,
            second.clone(),
            &second_trust
        )
        .is_err());
        assert!(programs_trace_differential_gate(
            first.clone(),
            &second_trust,
            second,
            &second_trust
        )
        .is_err());
        if length > guest_limit {
            assert_eq!(
                hash_bytes(HashAlgorithm::Sha256, &executable),
                Err(HashRefusal::InputTooLong {
                    length: length as u64,
                    limit: MAX_HASH_INPUT_BYTES
                })
            );
        } else {
            assert_eq!(
                hash_bytes(HashAlgorithm::Sha256, &executable),
                Ok(Sha256::digest(&executable).into())
            );
        }
    }
    // Keep the real trace input within its guest ceiling. The separately
    // hashed provenance workload includes the module and policy as well.
    // The encoded argument payload includes its own u64 argument count,
    // in addition to the trace preimage's u64 payload length.
    let trace_input_framing = b"LXP/program-trace-input/v1\0".len() + 4 + 8 + 8 + 2 * 5;
    let export = "x".repeat(guest_limit - trace_input_framing);
    let (first, first_trust) =
        submission(b"runner-a", &first_key, b"runner a", b"manifest a", &export);
    let (second, second_trust) = submission(
        b"runner-b",
        &second_key,
        b"runner b",
        b"manifest b",
        &export,
    );
    let wasm = add_module();
    let mut workload = b"LXP/program-trace-runner-workload/v1\0".to_vec();
    workload.extend_from_slice(&(wasm.len() as u64).to_be_bytes());
    workload.extend_from_slice(&wasm);
    workload.extend_from_slice(&(export.len() as u64).to_be_bytes());
    workload.extend_from_slice(export.as_bytes());
    workload.extend_from_slice(&2_u64.to_be_bytes());
    for value in [20_i32, 22_i32] {
        workload.push(0);
        workload.extend_from_slice(&value.to_be_bytes());
    }
    workload.extend_from_slice(&TracePolicy::new(1, 32).unwrap().canonical_bytes());
    workload.extend_from_slice(&RUNTIME_VERSION.to_be_bytes());
    assert!(workload.len() > guest_limit);
    assert!(workload.len() <= MAX_TRACE_RUNNER_PROVENANCE_BYTES);
    assert_eq!(
        hash_bytes(HashAlgorithm::Sha256, &workload),
        Err(HashRefusal::InputTooLong {
            length: workload.len() as u64,
            limit: MAX_HASH_INPUT_BYTES,
        })
    );
    // The real producer accepted these bytes through host_provenance_hash.
    // Check its workload identity after the signed canonical transport.
    let encoded = first.artifact.canonical_bytes().unwrap();
    let workload_offset = b"LXP/trace-runner-artifact/v1\0".len() + 2 * 32;
    assert_eq!(
        &encoded[workload_offset..workload_offset + 32],
        &<[u8; 32]>::from(Sha256::digest(&workload))
    );
    let evidence =
        programs_trace_differential_gate(first, &first_trust, second, &second_trust).unwrap();
    assert_eq!(evidence[0], 1);
    assert!(std::str::from_utf8(&evidence[9..])
        .unwrap()
        .contains(&export));
}

#[test]
fn host_and_artifact_bounds_refuse_oversize() {
    let wasm = add_module();
    let policy = TracePolicy::new(1, 32).unwrap();
    let oversize = vec![7; MAX_TRACE_RUNNER_PROVENANCE_BYTES + 1];
    for (executable, manifest) in [
        (&oversize[..], &b"manifest"[..]),
        (&b"runner"[..], &oversize[..]),
    ] {
        assert!(programs_trace_runner_artifact(
            b"platform",
            executable,
            manifest,
            &wasm,
            "add",
            &[],
            policy
        )
        .unwrap_err()
        .contains("exceeds bound"));
    }
    let maximum = programs_trace_runner_artifact(
        b"platform",
        &oversize[..MAX_TRACE_RUNNER_PROVENANCE_BYTES],
        b"manifest",
        &wasm,
        "add",
        &[WasmValue::I32(20), WasmValue::I32(22)],
        policy,
    )
    .unwrap();
    let encoded_maximum = maximum.canonical_bytes().unwrap();
    assert_eq!(
        TraceRunnerArtifact::from_canonical_bytes(&encoded_maximum).unwrap(),
        maximum
    );
    assert!(maximum.attestation_digest().is_ok());
    drop(oversize);
    let platform = vec![b'p'; usize::from(u16::MAX) + 1];
    assert!(programs_trace_runner_artifact(
        &platform,
        b"runner",
        b"manifest",
        &wasm,
        "add",
        &[],
        policy
    )
    .is_err());
    let (submission, _) = submission(
        b"platform",
        &SigningKey::from_bytes(&[7; 32]),
        b"runner",
        b"manifest",
        "add",
    );
    let wire = submission.artifact.canonical_bytes().unwrap();
    let evidence_length_offset =
        b"LXP/trace-runner-artifact/v1\0".len() + 96 + 2 + b"platform".len();
    let mut oversized = wire.clone();
    oversized[evidence_length_offset..evidence_length_offset + 8]
        .copy_from_slice(&((MAX_TRACE_RUNNER_EVIDENCE_BYTES as u64) + 1).to_be_bytes());
    assert!(TraceRunnerArtifact::from_canonical_bytes(&oversized)
        .unwrap_err()
        .contains("evidence exceeds bound"));
    assert!(TraceRunnerArtifact::from_canonical_bytes(&wire[..wire.len() - 1]).is_err());
    let mut trailing = wire.clone();
    trailing.push(0);
    assert!(TraceRunnerArtifact::from_canonical_bytes(&trailing).is_err());
    assert!(submission
        .artifact
        .clone()
        .attach_attestation_signature(&[0; 63])
        .is_err());
}

fn composed_guest(callee: Option<ProgramId>) -> Vec<u8> {
    let imported = u64::from(callee.is_some());
    let grants = CapabilitySet::empty().canonical_encoding();
    let mut exports = unsigned_leb(3);
    for (name, kind, index) in [
        ("layerx_reserve", 0, imported),
        (CALL_ENTRY_EXPORT, 0, imported + 1),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.push(kind);
        exports.extend(unsigned_leb(index));
    }
    let mut entry = Vec::new();
    let mut data = Vec::new();
    if let Some(callee) = callee {
        for value in [0, 32, 32, 0, 32, u8::try_from(grants.len()).unwrap()] {
            assert!(value < 64);
            entry.extend([0x41, value]);
        }
        entry.extend([0x10, 0]);
        data.extend(callee.bytes());
        data.extend(grants);
    } else {
        entry.extend([0x41, 0]);
    }
    entry.push(0x0b);
    let mut data_section = vec![1, 0, 0x41, 0, 0x0b];
    data_section.extend(unsigned_leb(data.len() as u64));
    data_section.extend(data);
    module(&[
        type_section(&[
            (&[TYPE_I32; 6], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
        ]),
        import_section(&if callee.is_some() {
            vec![(ABI_MODULE, "program_call", 0)]
        } else {
            vec![]
        }),
        function_section(&[1, 2]),
        raw_section(5, &[1, 1, 1, 1]),
        raw_section(7, &exports),
        code_section(&[func_body(&[], &[0x41, 0, 0x0b]), func_body(&[], &entry)]),
        raw_section(11, &data_section),
    ])
}

#[test]
fn runtime_contract_matches_real_terminal_success_and_refusal() {
    let contract = include_str!("../src/runtime_contract.md");
    for promised in [
        "# Runtime crate",
        "1, 2, 3, 4, 5",
        "WrongVersion",
        "ReplayRefusal",
        "128 MiB",
        "1048576",
        "restart",
    ] {
        assert!(
            contract.contains(promised),
            "missing contract clause {promised}"
        );
    }
    assert!(include_str!("../src/terminal.rs").contains("include_str!(\"runtime_contract.md\")"));
    let engine = WasmEngine::declared().unwrap();
    let validated = engine.validate(&add_module()).unwrap();
    let produced = execute_versioned_for_qualification(
        &validated,
        FeeSchedule::declared(),
        "add",
        &[WasmValue::I32(20), WasmValue::I32(22)],
    )
    .unwrap();
    let encoded = produced.canonical_evidence();
    let decoded = decode_terminal_payload(1, 1, &encoded).unwrap();
    assert!(
        matches!(decoded.detail, TerminalDetail::Execution(ExecutionTerminal::Legacy { encoding_version: 2, values, usage, .. }) if values == vec![ExecutionValue::I32(42)] && usage == produced.usage)
    );
    assert_eq!(
        decode_terminal_payload(1, 2, &encoded),
        Err(TerminalDecodeError::MismatchedAbi)
    );
    assert_eq!(
        decode_terminal_payload(2, 1, &encoded),
        Err(TerminalDecodeError::MismatchedKind)
    );
    for (expected, actual) in [(3, 4), (4, 5)] {
        let root_id = ProgramId::new([1; 32]).unwrap();
        let child_id = ProgramId::new([2; 32]).unwrap();
        let root = engine
            .validate_versioned(expected, &composed_guest(Some(child_id)))
            .unwrap();
        let child = engine
            .validate_versioned(actual, &composed_guest(None))
            .unwrap();
        let mut catalog = ProgramCatalog::new();
        catalog.insert(child_id, child);
        let storage = Storage::new();
        let before = storage.clone();
        let refusal = invoke_composed_for_qualification(
            &storage,
            AuthorizedExecutionRequest {
                module: &root,
                program: root_id,
                authorization: AuthorizationContext::new(
                    PrincipalId::new([3; 32]).unwrap(),
                    CapabilitySet::new([Capability::Call { program: child_id }]).unwrap(),
                ),
                receipts: &UnavailableReceiptOracle,
                entrypoint: CALL_ENTRY_EXPORT,
                calldata: &[],
                composition: CompositionContext::catalog(catalog, CompositionRules::declared()),
                response_capacity: 0,
            },
        )
        .unwrap_err();
        assert_eq!(storage, before);
        let ExecutionError::Composition(refusal) = refusal else {
            panic!("expected real composition refusal");
        };
        let encoded =
            encode_failure_payload(&BudgetedV1FailureCause::Composition(refusal)).unwrap();
        let decoded = decode_terminal_payload(2, expected, &encoded).unwrap();
        assert_eq!(
            decoded.detail,
            TerminalDetail::Failure(FailureTerminal::Composition {
                tag: 2,
                fields: FailureFields::Revisions {
                    expected: expected as u8,
                    actual: actual as u8
                }
            })
        );
        for bad in [0, 6, u8::MAX] {
            for offset in [encoded.len() - 2, encoded.len() - 1] {
                let mut malformed = encoded.clone();
                malformed[offset] = bad;
                assert_eq!(
                    decode_terminal_payload(2, expected, &malformed),
                    Err(TerminalDecodeError::Malformed)
                );
            }
        }
    }
}
