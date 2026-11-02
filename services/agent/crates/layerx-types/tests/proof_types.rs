//! Proves: VerificationLevel / ResultCode / Retriability -> public Rust items and wire ranks -> ApiError::core_rejection, VerificationBacklog, ApprovalEventKind, audit::verification_level, budget_verification_level.
//!
//! Repairs: layerx-agent-api/src/error.rs duplicated Retriability and lacked a
//! constructor preserving the producer's retry classification. The API now uses
//! the canonical enum and derives retry metadata from the exact result code.
//! layerx-types/src/verify.rs now owns the checked inverse of wire_rank, replacing
//! the duplicate audit table in layerx-agentd/src/audit/record.rs; budget proof
//! decoding uses that same boundary and still requires exactly checkpoint finality.
//!
//! These labels carry no persistent state and do not construct Verified<T>.
//! Each consumer observes fresh producer output twice, with intervening different
//! output, and refuses malformed ranks at its own decoding boundary.
//!
//! The types crate intentionally has no dependency on its downstream consumers.
//! Following tests/verification.rs, this proof compiles a consumer probe against
//! the actual built library. The API module and its tests are loaded by path;
//! the daemon's dependency-free boundary declarations are read verbatim from the
//! production files, including their real helper functions and types. No test
//! implementation substitutes for a producer, transport, or consumer. The probe
//! covers the verification fields, not daemon I/O or the cryptographic verifiers.

use std::fs;
use std::path::{Path, PathBuf};

#[path = "../../../tests/support/rustc.rs"]
mod rustc;

/// Selects an entire production declaration, retaining its derive/repr attributes.
/// The selected declarations contain no braces in string literals or comments.
fn production_item(source: &str, declaration: &str) -> String {
    let start = source
        .find(declaration)
        .unwrap_or_else(|| panic!("production declaration missing: {declaration}"));
    assert!(
        !source[start + declaration.len()..].contains(declaration),
        "ambiguous production declaration: {declaration}"
    );
    let mut attributed_start = start;
    while attributed_start > 0 {
        let preceding = &source[..attributed_start];
        let line_end = preceding.trim_end_matches('\n').len();
        let line_start = preceding[..line_end].rfind('\n').map_or(0, |at| at + 1);
        if !preceding[line_start..line_end].starts_with("#[") {
            break;
        }
        attributed_start = line_start;
    }
    let opening = start
        + source[start..]
            .find('{')
            .unwrap_or_else(|| panic!("production declaration has no body: {declaration}"));
    let mut depth = 0_u32;
    for (offset, byte) in source.as_bytes()[opening..].iter().enumerate() {
        match byte {
            b'{' => depth += 1,
            b'}' => {
                depth -= 1;
                if depth == 0 {
                    return source[attributed_start..=opening + offset].to_owned();
                }
            }
            _ => {}
        }
    }
    panic!("unterminated production declaration: {declaration}");
}

fn read(path: &Path) -> String {
    fs::read_to_string(path)
        .unwrap_or_else(|error| panic!("read production source {}: {error}", path.display()))
}

fn dependency_rlib() -> (PathBuf, PathBuf) {
    let executable =
        std::env::current_exe().unwrap_or_else(|error| panic!("test executable: {error}"));
    let dependencies = executable
        .parent()
        .unwrap_or_else(|| panic!("test dependency directory missing"))
        .to_path_buf();
    let library = fs::read_dir(&dependencies)
        .unwrap_or_else(|error| panic!("test dependency directory: {error}"))
        .map(|entry| {
            entry
                .unwrap_or_else(|error| panic!("dependency: {error}"))
                .path()
        })
        .filter(|path| {
            path.file_name().is_some_and(|name| {
                name.to_string_lossy().starts_with("liblayerx_types-")
                    && path
                        .extension()
                        .is_some_and(|extension| extension == "rlib")
            })
        })
        .max_by_key(|path| {
            fs::metadata(path)
                .and_then(|metadata| metadata.modified())
                .ok()
        })
        .unwrap_or_else(|| panic!("built layerx-types library missing"));
    (dependencies, library)
}

struct ProbeDirectory(PathBuf);

impl Drop for ProbeDirectory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn proof_types_real_consumers_observe_producers_and_refuse_malformed_results() {
    let crates = Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .unwrap_or_else(|| panic!("agent crates directory missing"));
    let api = crates.join("layerx-agent-api");
    let daemon = crates.join("layerx-agentd/src");
    let identity = production_item(
        &read(&api.join("src/identity.rs")),
        "pub enum ContractError",
    );
    let admin = read(&daemon.join("admin.rs"));
    let approval = read(&daemon.join("approval/events.rs"));
    let audit = read(&daemon.join("audit/record.rs"));
    let budget = read(&daemon.join("budget/budget_proof.rs"));
    let mut probe = format!(
        r#"
extern crate self as layerx_agent_api;
mod identity {{ {identity} }}
#[path = {error_path:?}]
pub mod error;
pub mod idempotency {{
    pub use crate::error::{{classify_repeat, BodyDigest, IdempotencyOutcome, IdempotentMutation, Key}};
}}
pub mod verify {{
    pub use crate::error::{{ApiSuccess, Rung, VerificationStatus}};
}}
#[path = {api_tests_path:?}]
mod api_tests;
"#,
        error_path = api.join("src/error.rs"),
        api_tests_path = api.join("tests/errors.rs"),
    );
    for (name, source, declarations) in [
        (
            "admin",
            admin.as_str(),
            vec!["pub struct VerificationBacklog", "impl VerificationBacklog"],
        ),
        (
            "approval",
            approval.as_str(),
            vec!["pub enum ApprovalEventKind", "impl ApprovalEventKind"],
        ),
        (
            "audit",
            audit.as_str(),
            vec!["fn verification_level(rank: u8)"],
        ),
        (
            "budget",
            budget.as_str(),
            vec![
                "pub enum BudgetProofError",
                "fn require(value: bool)",
                "fn budget_verification_level(rank: u8)",
            ],
        ),
    ] {
        probe.push_str(&format!(
            "mod {name} {{\nuse layerx_types::verify::{{InvalidVerificationRank, VerificationLevel}};\n"
        ));
        for declaration in declarations {
            // Private production functions stay private; their tests live in the
            // same generated module, as ordinary module unit tests do.
            probe.push_str(&production_item(source, declaration));
            probe.push('\n');
        }
        probe.push_str(match name {
            "audit" => AUDIT_PROOF,
            "admin" => ADMIN_PROOF,
            "approval" => APPROVAL_PROOF,
            "budget" => BUDGET_PROOF,
            _ => unreachable!(),
        });
        probe.push_str("}\n");
    }
    probe.push_str(LEVEL_PRODUCERS);
    let directory = ProbeDirectory(
        std::env::temp_dir().join(format!("layerx-proof-types-{}", std::process::id())),
    );
    fs::create_dir(&directory.0).unwrap_or_else(|error| panic!("probe directory: {error}"));
    let source = directory.0.join("consumers.rs");
    let binary = directory.0.join("consumers");
    fs::write(&source, probe).unwrap_or_else(|error| panic!("probe source: {error}"));
    let (dependencies, library) = dependency_rlib();
    let compiled = rustc::command(&dependencies)
        .args([
            "--edition=2021",
            "--test",
            "--crate-name",
            "proof_types_consumers",
        ])
        .arg(&source)
        .arg("--extern")
        .arg(format!("layerx_types={}", library.display()))
        .arg("-L")
        .arg(format!("dependency={}", dependencies.display()))
        .arg("-o")
        .arg(&binary)
        .output()
        .unwrap_or_else(|error| panic!("compile real consumers: {error}"));
    assert!(
        compiled.status.success(),
        "real consumers must compile against the public types:\n{}",
        String::from_utf8_lossy(&compiled.stderr)
    );
    let observed = std::process::Command::new(&binary)
        .arg("--nocapture")
        .output()
        .unwrap_or_else(|error| panic!("run real consumers: {error}"));
    let output = String::from_utf8_lossy(&observed.stdout);
    println!("{output}");
    assert!(
        observed.status.success(),
        "consumer assertions failed:\n{output}\n{}",
        String::from_utf8_lossy(&observed.stderr)
    );
    for required in [
        "api_protocol_rejections_keep_canonical_retry_classification",
        "audit_decodes_each_producer_twice_and_refuses_unknown_ranks",
        "admin_observes_exact_level_and_settlement_refusal",
        "approval_never_promotes_local_decisions",
        "budget_decodes_exact_checkpoint_and_refuses_other_labels",
    ] {
        assert!(
            output.contains(required),
            "consumer test did not execute: {required}"
        );
    }
}

const LEVEL_PRODUCERS: &str = r#"
fn levels() -> [layerx_types::verify::VerificationLevel; 6] {
    use layerx_types::verify::VerificationLevel as L;
    [L::UNVERIFIED, L::SEQUENCER_SIGNED, L::BATCH_INCLUDED,
     L::STATE_PROVEN, L::CHECKPOINT_FINALISED, L::SETTLEMENT_ANCHORED]
}
"#;

const AUDIT_PROOF: &str = r#"
#[test]
fn audit_decodes_each_producer_twice_and_refuses_unknown_ranks() {
    for (rank, producer) in crate::levels().into_iter().enumerate() {
        assert_eq!(usize::from(producer.wire_rank()), rank);
        for _ in 0..2 {
            let transported = producer.wire_rank();
            let observed = verification_level(transported).expect("producer label");
            assert_eq!(observed, producer);
            for requested in crate::levels() {
                assert_eq!(observed.compare(requested), rank.cmp(&usize::from(requested.wire_rank())));
            }
            assert_eq!(verification_level(255), Err(InvalidVerificationRank(255)));
        }
    }
    for rank in 6..=255 {
        assert_eq!(verification_level(rank), Err(InvalidVerificationRank(rank)));
    }
}
"#;

const ADMIN_PROOF: &str = r#"
#[test]
fn admin_observes_exact_level_and_settlement_refusal() {
    use layerx_types::rung::Rung;
    let rungs = [Some(Rung::Submitted), Some(Rung::Submitted),
        Some(Rung::Receipted), Some(Rung::Receipted), Some(Rung::Committed), None];
    for (requested_rank, requested) in crate::levels().into_iter().enumerate() {
        for observed in crate::levels() {
            for _ in 0..2 {
                let item = VerificationBacklog {
                    idempotency_key: [51; 32], observed, requested,
                    queued_at_ms: 100, attempts: 0,
                };
                assert_eq!(item.is_backlogged(), observed.wire_rank() < requested.wire_rank());
                assert_eq!(item.retry_rung(), rungs[requested_rank]);
                let achieved = VerificationBacklog { observed: requested, ..item };
                assert!(!achieved.is_backlogged(), "already achieved is not a backlog");
            }
        }
    }
}
"#;

const APPROVAL_PROOF: &str = r#"
#[test]
fn approval_never_promotes_local_decisions() {
    for kind in [ApprovalEventKind::Created, ApprovalEventKind::Granted,
                 ApprovalEventKind::Rejected, ApprovalEventKind::Expired] {
        for _ in 0..2 {
            let producer = kind.verification_level();
            let observed = VerificationLevel::from_wire_rank(producer.wire_rank()).expect("local label");
            assert_eq!(observed, VerificationLevel::UNVERIFIED);
            assert!(observed.compare(VerificationLevel::SEQUENCER_SIGNED).is_lt());
        }
    }
}
"#;

const BUDGET_PROOF: &str = r#"
#[test]
fn budget_decodes_exact_checkpoint_and_refuses_other_labels() {
    for _ in 0..2 {
        let producer = VerificationLevel::CHECKPOINT_FINALISED;
        let observed = budget_verification_level(producer.wire_rank()).expect("checkpoint");
        assert_eq!(observed, producer);
        assert_eq!(observed.rung(), Some(layerx_types::rung::Rung::Committed));
        for rank in 0..=255 {
            if rank != producer.wire_rank() {
                assert_eq!(budget_verification_level(rank), Err(BudgetProofError::Refused));
            }
        }
    }
}
"#;
