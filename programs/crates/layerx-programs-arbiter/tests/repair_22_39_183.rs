//! Runtime record -> producer envelope -> arbiter leaf consumers.

#[path = "proof_arbiter_crate.rs"]
mod producer;

// Compile the real market leaf consumer here so its crate-private entry point
// can be exercised without manufacturing a VerifiedMarketSandbox admission.
#[path = "../src/market.rs"]
mod market;
use layerx_programs::chain_proof::{proto, ChainProofError};
#[path = "../../layerx-programs-registry/src/chain_proof/multistore.rs"]
mod multistore;

use layerx_program_sdk::arbiter::{
    MarketBillingCommitment, MarketSandboxProfile, ProfileLimits, MARKET_SANDBOX_BILLING_CAPACITY,
    MARKET_SANDBOX_PROFILE_CAPACITY,
};
use layerx_programs_arbiter::bisect::{
    CommittedTrace, SandboxStepJudge, StepJudge, TraceCommitment,
};
pub use layerx_programs_arbiter::{
    ics23, AdmissionEvidence, AuthenticatedCatalogue, BoundaryProof, ChainCommitProof, ReplayError,
    VerifiedReplayAuthority,
};
use layerx_programs_runtime::{
    execute::{
        instantiate_market_sandbox_untrusted, market_sandbox_input_digest, MarketSandboxRequest,
    },
    portable_replay::{decode_record, replay_leaf_hash, PortableBoundary},
    replay::{
        market_sandbox_baseline_root, market_sandbox_namespace, MarketSandboxReplayAuthority,
    },
    test_support::{
        code_section, export_section, func_body, function_section, module, type_section, OP_END,
        OP_I32_CONST, TYPE_I32,
    },
    FeeSchedule, PrincipalId, ProgramId, ProgramReplayProfile, ResourceBudget, Storage, WasmEngine,
    WasmValue, RUNTIME_VERSION,
};
use sha2::{Digest, Sha256};

const MAX: usize = 1_048_576;

#[test]
fn authenticated_record_headers_bind_step_transport_after_reconstruction() {
    producer::repair_record_header_transport();
}

#[test]
fn runtime_record_envelopes_cross_market_and_bisection_leaf_consumers() {
    for trapped in [false, true] {
        let body = if trapped {
            vec![0, OP_END]
        } else {
            vec![OP_I32_CONST, 7, OP_I32_CONST, 5, 0x6a, OP_END]
        };
        let wasm = module(&[
            type_section(&[(&[], &[TYPE_I32])]),
            function_section(&[0]),
            export_section(&[("compute", 0)]),
            code_section(&[func_body(&[], &body)]),
        ]);
        let validated = WasmEngine::declared()
            .unwrap()
            .validate_versioned(2, &wasm)
            .unwrap();
        let program = ProgramId::new(validated.code_hash()).unwrap();
        let tenant = PrincipalId::new(Sha256::digest(b"record-header-tenant").into()).unwrap();
        let lease = Sha256::digest(b"record-header-lease").into();
        let baseline = Storage::new();
        let fees = FeeSchedule::declared();
        let authority = MarketSandboxReplayAuthority {
            profile_binding: Sha256::digest(b"record-header-profile").into(),
            namespace: market_sandbox_namespace(program, lease).unwrap(),
            lease_id: lease,
            namespace_limit: 1024,
            program,
            tenant,
            payment_account: tenant.bytes(),
            code_hash: validated.code_hash(),
            input_digest: market_sandbox_input_digest("compute", &[]).unwrap(),
            runtime_version: RUNTIME_VERSION,
            abi_version: 2,
            fee_schedule_version: fees.version(),
            metering_schedule_version: validated.metering_schedule_version(),
            budget: ResourceBudget::new_complete(100_000, 65_536, 1024, 1024, 1, 64, 0),
            fees,
            fee_budget: 1_000_000_000,
            baseline_state_root: market_sandbox_baseline_root(&baseline).unwrap(),
            baseline_storage: baseline,
        };
        let mut instance = instantiate_market_sandbox_untrusted(&validated, &authority).unwrap();
        let execution = instance
            .call_market_sandbox_untrusted(MarketSandboxRequest {
                module: &validated,
                entrypoint: "compute",
                args: &[],
                authority: &authority,
                replay_profile: ProgramReplayProfile::new(128, MAX as u32).unwrap(),
            })
            .unwrap();
        if trapped {
            assert!(execution.terminal_fault.is_some());
            assert_eq!(execution.record.terminal_status(), 1);
        } else {
            assert!(execution.terminal_fault.is_none());
            assert_eq!(execution.values, vec![WasmValue::I32(12)]);
        }
        let (record, leaves) = decode_record(execution.record.canonical_bytes(), MAX).unwrap();
        assert_eq!(leaves, execution.boundary_leaves);
        let committed = CommittedTrace::new(leaves.clone()).unwrap();
        assert_eq!(committed.root(), record.boundary_root());
        let mut entrypoint_bytes = [0; 64];
        entrypoint_bytes[..7].copy_from_slice(b"compute");
        let profile = MarketSandboxProfile {
            network_id: 7,
            market_program: Sha256::digest(b"record-header-market").into(),
            sandbox_program: program.bytes(),
            offer_id: Sha256::digest(b"record-header-offer").into(),
            lease_id: lease,
            claim_id: Sha256::digest(b"record-header-claim").into(),
            provider: Sha256::digest(b"record-header-provider").into(),
            tenant: tenant.bytes(),
            code_hash: record.code_hash(),
            input_digest: record.input_digest(),
            attested_input_commitment: Sha256::digest(b"record-header-input").into(),
            namespace: authority.namespace,
            baseline_state_root: authority.baseline_state_root,
            initial_execution_state_root: execution.initial_commitment.digest,
            entrypoint_bytes,
            entrypoint_length: 7,
            runtime_version: record.runtime_version(),
            abi_version: record.abi_version(),
            fee_schedule_version: record.fee_schedule_version(),
            metering_schedule_version: record.metering_schedule_version(),
            limits: ProfileLimits {
                cpu_fuel: 100_000,
                memory_bytes: 65_536,
                storage_read_bytes: 1024,
                storage_write_bytes: 1024,
                output_values: 1,
                output_bytes: 64,
                table_elements: 0,
                namespace_bytes: 1024,
            },
            fee_budget: authority.fee_budget,
            interval_start: 10,
            interval_end: 11,
            response_deadline: 17,
            maximum_boundaries: record.profile().maximum_boundaries(),
            maximum_bytes: record.profile().maximum_bytes(),
        };
        let mut profile_bytes = vec![0; MARKET_SANDBOX_PROFILE_CAPACITY];
        let n = profile.encode(&mut profile_bytes).unwrap();
        profile_bytes.truncate(n);
        let profile = MarketSandboxProfile::decode(&profile_bytes).unwrap();
        let billing = MarketBillingCommitment {
            profile_digest: Sha256::digest(&profile_bytes).into(),
            provider_trace_root: record.boundary_root(),
            final_execution_state_root: execution.final_commitment.digest,
            output_digest: execution.output_digest,
            usage: execution.billing_usage(1).unwrap(),
            payable: 1,
            challenger_stake: 1,
            challenge_window_batches: 6,
            boundary_count: record.boundary_count(),
        };
        let mut billing_bytes = vec![0; MARKET_SANDBOX_BILLING_CAPACITY];
        let n = billing.encode(&mut billing_bytes).unwrap();
        billing_bytes.truncate(n);
        let billing = MarketBillingCommitment::decode(&billing_bytes).unwrap();
        for _ in 0..2 {
            let trace = TraceCommitment {
                root: billing.provider_trace_root,
                boundary_count: billing.boundary_count,
                initial_state_root: profile.initial_execution_state_root,
                maximum_bytes: profile.maximum_bytes,
            };
            let judge = SandboxStepJudge::new(&validated, &authority, trace).unwrap();
            let paths: Vec<_> = leaves
                .iter()
                .enumerate()
                .map(|(index, leaf)| {
                    let mut path = committed.proof(index as u32).unwrap();
                    path.leaf = record.header().boundary_envelope(leaf, MAX).unwrap();
                    path
                })
                .collect();
            for (path, raw) in paths.iter().zip(&leaves) {
                let (header, extracted) =
                    PortableBoundary::split_envelope(&path.leaf, MAX).unwrap();
                assert_eq!(&header, record.header());
                assert_eq!(extracted, raw);
                assert_ne!(
                    replay_leaf_hash(path.index, raw).unwrap(),
                    replay_leaf_hash(path.index, &path.leaf).unwrap()
                );
                let opened = trace.open_leaf(path).unwrap();
                let market_boundary =
                    market::verify_market_boundary(path, &profile, &billing).unwrap();
                assert_eq!(market_boundary.header, header);
                assert_eq!(market_boundary.reencode_untrusted(MAX).unwrap(), *raw);
                assert_eq!(opened.reencode_untrusted(MAX).unwrap(), *raw);
            }
            for pair in paths.windows(2) {
                assert!(
                    judge.step_holds(&pair[0], &pair[1]),
                    "producer transition {} -> {}",
                    pair[0].index,
                    pair[1].index
                );
            }
            if trapped {
                assert!(judge.step_holds(paths.last().unwrap(), paths.last().unwrap()));
            }
            let mut bad = paths[0].clone();
            bad.leaf = leaves[0].clone();
            assert!(trace.open_leaf(&bad).is_none());
            assert!(matches!(
                market::verify_market_boundary(&bad, &profile, &billing),
                Err(market::MarketEvidenceError::Encoding)
            ));
            assert!(!judge.step_holds(&bad, &paths[1]));
            let mut header = record.header().clone();
            header.identity.module_code_hash[0] ^= 1;
            let mut bad = paths[0].clone();
            bad.leaf = header.boundary_envelope(&leaves[0], MAX).unwrap();
            assert!(matches!(
                market::verify_market_boundary(&bad, &profile, &billing),
                Err(market::MarketEvidenceError::Identity)
            ));
            assert!(!judge.step_holds(&bad, &paths[1]));
            let mut bad_post = paths[1].clone();
            bad_post.leaf = header.boundary_envelope(&leaves[1], MAX).unwrap();
            assert!(!judge.step_holds(&paths[0], &bad_post));
            let mut bad = paths.last().unwrap().clone();
            bad.siblings.push([0; 32]);
            assert!(trace.open_leaf(&bad).is_none());
            assert!(matches!(
                market::verify_market_boundary(&bad, &profile, &billing),
                Err(market::MarketEvidenceError::Path)
            ));
            let mut bad_profile = profile;
            bad_profile.maximum_boundaries -= 1;
            assert!(matches!(
                market::verify_market_boundary(&paths[0], &bad_profile, &billing),
                Err(market::MarketEvidenceError::Identity)
            ));
            let mut bad_profile = profile;
            bad_profile.maximum_bytes = paths[0].leaf.len() as u32 - 1;
            assert!(matches!(
                market::verify_market_boundary(&paths[0], &bad_profile, &billing),
                Err(market::MarketEvidenceError::Bounds)
            ));
        }
    }
}
