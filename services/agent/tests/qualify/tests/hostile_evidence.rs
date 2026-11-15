use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};

use ed25519_dalek::SigningKey;
use layerx_agent_api::read::RelativeTo;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck};
use layerx_sdk::{Client, Rung, SdkError};
use serde_json::{json, Value};

#[allow(dead_code)]
#[path = "../fabrication.rs"]
mod fabrication;
#[allow(dead_code)]
#[path = "../fuzz.rs"]
mod fuzz;
#[path = "../hostile_node.rs"]
mod hostile_node;

use hostile_node::{
    attack_record, owner_record, reject_attack, verify_owner_record, HostileAttack,
};

fn repository() -> &'static Path {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .expect("repository")
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = std::env::temp_dir().join(format!(
            "repair-22-33-1575-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed),
        ));
        #[cfg(unix)]
        {
            use std::os::unix::fs::DirBuilderExt as _;
            fs::DirBuilder::new()
                .mode(0o700)
                .create(&root)
                .expect("private test evidence");
        }
        #[cfg(not(unix))]
        fs::create_dir(&root).expect("test evidence");
        Self(root)
    }

    fn agent(&self) -> PathBuf {
        self.0.join("services/agent")
    }

    fn workspace(&self) {
        // A clean decoy at the old path proves the caller selects services/agent.
        for root in [self.agent(), self.0.join("agent")] {
            fs::create_dir_all(root.join("crates/sample/src")).expect("fixture tree");
            fs::write(root.join("unsafe-allowlist.toml"), "version = 1\n").expect("policy");
            fs::write(root.join("nondeterminism-allowlist.toml"), "version = 1\n")
                .expect("clock policy");
            fs::write(
                root.join("crates/sample/Cargo.toml"),
                "[package]\nname = \"sample\"\nversion = \"0.1.0\"\n",
            )
            .expect("manifest");
            fs::write(
                root.join("crates/sample/src/lib.rs"),
                "pub struct Value(u64);\n",
            )
            .expect("source");
        }
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        if !std::thread::panicking() {
            fs::remove_dir_all(&self.0).expect("remove own evidence");
        }
    }
}

#[test]
fn fabrication_generated_shape_owner_codec_to_major_2_read() {
    let producer = owner_record().expect("recorded node/Comet producer");
    let frame = serde_json::to_vec(&producer).expect("recorded transport encode");
    let received: Value = serde_json::from_slice(&frame).expect("real transport codec");
    let verified = verify_owner_record(&received).expect("independent real verifiers");
    let read = fabrication::verified_owner_read(&received).expect("current major-2 read");
    assert_eq!(read.achieved_rung, Rung::Committed);
    assert_eq!(
        (
            read.value.height,
            read.freshness.chain_head,
            read.freshness.value_height
        ),
        (10, 10, 10)
    );
    assert_eq!(
        read.freshness.relative_to,
        RelativeTo::Block {
            height: 10,
            hash: verified.head.header_hash()
        }
    );
    assert_eq!(
        serde_json::from_slice::<Value>(read.value.commit.as_bytes()).expect("retained commit"),
        producer["commit"]
    );
    assert_eq!(
        serde_json::from_slice::<Value>(read.value.validator_set.as_bytes())
            .expect("retained validator set"),
        producer["validators"]
    );
    assert_eq!(
        verified.canonical,
        hostile_node::unhex(
            producer["receiptJson"]["receipt"]
                .as_str()
                .expect("receipt")
        )
        .expect("canonical bytes")
    );
    assert_eq!(
        Client::accept_verified_read(Rung::Committed, read.clone()),
        Ok(read.clone())
    );
    let mut submitted = read.clone();
    submitted.achieved_rung = Rung::Submitted;
    assert_eq!(
        Client::accept_verified_read(Rung::Receipted, submitted),
        Err(SdkError::UnverifiedRead)
    );
    let mut receipted = read;
    receipted.achieved_rung = Rung::Receipted;
    assert_eq!(
        Client::accept_verified_read(Rung::Committed, receipted),
        Err(SdkError::VerificationBelowRequested {
            requested: Rung::Committed,
            achieved: Rung::Receipted
        })
    );
    fabrication::agent_fabrication_evidence_gate().expect("real fabrication evidence gate");
}

#[test]
fn fabrication_generated_shape_rejects_signatures_values_and_block_bindings() {
    for attack in [
        HostileAttack::AlteredBalance,
        HostileAttack::AlteredReceipt,
        HostileAttack::ResignedReceipt,
    ] {
        let damaged = attack_record(attack).expect("independent mutation");
        let error = fabrication::verified_owner_read(&damaged).expect_err("no hostile value");
        assert!(error.contains("receipt-verifier"), "{attack:?}: {error}");
    }
    for (name, value) in [
        ("blockHash", json!(format!("0x{}", "01".repeat(32)))),
        ("blockNumber", json!("0xb")),
    ] {
        let mut damaged = owner_record().expect("owner transcript");
        damaged["receiptJson"][name] = value;
        let error = fabrication::verified_owner_read(&damaged).expect_err("block substitution");
        assert!(error.contains("block-binding"), "{name}: {error}");
    }
    let mut damaged = owner_record().expect("owner transcript");
    damaged["validators"]["validators"][0]["voting_power"] = json!("1");
    let error = fabrication::verified_owner_read(&damaged).expect_err("substituted trust");
    assert!(error.contains("validator-pins"), "{error}");
}

#[test]
fn unsafe_policy_contradiction_valid_empty_policy_and_real_scanner() {
    let directory = Directory::new();
    directory.workspace();
    for policy in [
        "version = 1\n",
        "# no exceptions\nversion=1 # current\nexceptions = []\n",
        "exceptions=[]\nversion = 1\n",
    ] {
        fs::write(directory.agent().join("unsafe-allowlist.toml"), policy)
            .expect("empty versioned policy");
        assert_eq!(
            fuzz::unsafe_policy_exception_count(&directory.agent()),
            Ok(0)
        );
        let result = fuzz::agent_boundary_scan(&directory.0, &directory.0.join("logs"))
            .expect("production scanner");
        assert_eq!(result.require_success(), Ok(0));
        assert_eq!(result.exit, 0);
        assert_eq!(result.stdout, "agent boundary purity passed\n");
        assert_eq!(
            fs::read_to_string(result.stdout_log).expect("stdout log"),
            result.stdout
        );
    }
    assert_eq!(
        fuzz::unsafe_policy_exception_count(&repository().join("services/agent")),
        Ok(0)
    );
}

#[test]
fn unsafe_policy_contradiction_rejects_invalid_or_unapproved_policy() {
    let directory = Directory::new();
    directory.workspace();
    for policy in [
        "",
        "version=2\n",
        "version=\"1\"\n",
        "version=1\nversion=1\n",
        "version=1\nexceptions=[]\nexceptions=[]\n",
        "version=1\npaths=[\"crates/sample/src/lib.rs\"]\n",
        "version=1\n[[exceptions]]\npath=\"crates/sample/src/lib.rs\"\n",
        "version=1\nexceptions=[\"crates/sample/src/lib.rs\"]\n",
    ] {
        fs::write(directory.agent().join("unsafe-allowlist.toml"), policy).expect("hostile policy");
        assert!(
            fuzz::agent_boundary_scan(&directory.0, &directory.0.join("logs")).is_err(),
            "accepted {policy:?}"
        );
    }
}

#[test]
fn boundary_checker_root_runs_actual_scanner_preserving_failure_exit_and_logs() {
    let directory = Directory::new();
    directory.workspace();
    fs::write(
        directory.agent().join("crates/sample/src/lib.rs"),
        "pub unsafe fn unchecked() {}\n",
    )
    .expect("unapproved source");
    let result = fuzz::agent_boundary_scan(&directory.0, &directory.0.join("logs"))
        .expect("real scanner evidence");
    assert_eq!(
        result.agent_root,
        directory.agent().canonicalize().expect("canonical agent")
    );
    assert_eq!(result.exit, 1);
    assert_eq!(result.stdout, "");
    let expected = format!(
        "unapproved-unsafe: {}\n",
        directory.agent().join("crates/sample/src/lib.rs").display()
    );
    assert_eq!(result.stderr, expected);
    assert_eq!(
        fs::read_to_string(&result.stderr_log).expect("failure log"),
        expected
    );
    let error = result
        .require_success()
        .expect_err("never swallow scanner failure");
    assert!(error.contains("exit=1"));
    assert!(error.contains(result.stderr_log.to_str().expect("log path")));
    assert!(error.contains("unapproved-unsafe"));
    let decoy = fuzz::scan_agent_root(&directory.0.join("agent"), &directory.0.join("decoy-logs"))
        .expect("decoy scan");
    assert_eq!(
        decoy.require_success(),
        Ok(0),
        "old root would miss the unsafe code"
    );
}

#[test]
fn boundary_checker_root_inspects_the_canonical_repository_tree() {
    let directory = Directory::new();
    let result =
        fuzz::agent_boundary_scan(repository(), &directory.0).expect("actual repository scanner");
    assert_eq!(
        result.agent_root,
        repository()
            .join("services/agent")
            .canonicalize()
            .expect("real agent tree")
    );
    assert!(!result.stderr.contains("missing-unsafe-allowlist"));
    assert!(!result.stderr.contains("unreadable-crate-tree"));
    assert_eq!(
        fs::read_to_string(&result.stderr_log).expect("exact scanner log"),
        result.stderr
    );
    assert_eq!(
        fs::read_to_string(&result.stdout_log).expect("exact scanner log"),
        result.stdout
    );
    // Existing violations are not waived: the same evidence must refuse the
    // sanitizer gate with its original status and complete retained diagnostics.
    if result.exit == 0 {
        assert_eq!(result.require_success(), Ok(0));
    } else {
        assert_eq!(result.exit, 1);
        assert!(!result.stderr.is_empty());
        let error = result
            .require_success()
            .expect_err("retain real-tree refusal");
        assert!(error.contains(result.stderr.trim()));
        assert!(error.contains("exit=1"));
        eprintln!(
            "Existing agent scanner refusal (outside repair scope): {}",
            result.stderr
        );
    }
}

#[test]
fn hostile_certificate_unchecked_reaches_real_quorum_consumer() {
    let baseline = owner_record().expect("recorded producer");
    verify_owner_record(&baseline).expect("full signed quorum positive control");
    let damaged = attack_record(HostileAttack::SubthresholdCertificate).expect("quorum mutation");
    assert_eq!(damaged["receiptJson"], baseline["receiptJson"]);
    assert_eq!(damaged["batch"], baseline["batch"]);
    assert_eq!(
        damaged["commit"]["signed_header"]["header"],
        baseline["commit"]["signed_header"]["header"]
    );
    let failure =
        reject_attack(HostileAttack::SubthresholdCertificate).expect("real quorum rejection");
    assert_eq!(failure.consumer, "commit-verifier");
    assert_eq!(failure.reason, "Proof(InsufficientPower)");
}

#[test]
fn hostile_certificate_unchecked_resigned_receipt_has_valid_attacker_signature() {
    let baseline = owner_record().expect("producer");
    let damaged =
        attack_record(HostileAttack::ResignedReceipt).expect("actually re-signed receipt");
    assert_eq!(damaged["batch"], baseline["batch"]);
    assert_eq!(damaged["commit"], baseline["commit"]);
    let canonical =
        hostile_node::unhex(damaged["receiptJson"]["receipt"].as_str().expect("receipt"))
            .expect("canonical bytes");
    let decoded = layerx_wire::receipt::decode(&canonical).expect("real codec");
    let unsigned = layerx_wire::receipt::encode_unsigned(&decoded).expect("unsigned codec");
    let digest = layerx_wire::hash::receipt_digest(&unsigned).expect("receipt domain");
    let signature = decoded
        .protocol()
        .expect("protocol receipt")
        .sequencer_signature()
        .expect("actual signature");
    let attacker = SigningKey::from_bytes(&[0x57; 32])
        .verifying_key()
        .to_bytes();
    layerx_crypto::ed25519::verify_digest(&attacker, &signature, &digest)
        .expect("mathematically valid attacker signature");
    let failure = reject_attack(HostileAttack::ResignedReceipt).expect("pinned verifier refusal");
    assert_eq!(failure.consumer, "receipt-verifier");
    assert_eq!(failure.reason, "Verification");
}

#[test]
fn hostile_certificate_unchecked_rejects_expired_and_unknown_receipt_authority() {
    let observed = hostile_node::verify_receipt_authority_cases()
        .expect("real producer corpus through receipt consumer");
    for id in [
        "receipted-primary-key",
        "height-outside-key-range",
        "signature-key-outside-table",
        "unknown-key-id",
    ] {
        assert!(observed.contains(id), "missing authority case {id}");
    }
    let baseline = owner_record().expect("producer");
    let canonical = hostile_node::unhex(
        baseline["receiptJson"]["receipt"]
            .as_str()
            .expect("receipt"),
    )
    .expect("receipt bytes");
    let key = &baseline["pinnedKey"];
    let fixed = |name: &str| {
        hostile_node::unhex(key[name].as_str().expect("key field"))
            .expect("key bytes")
            .try_into()
            .expect("key width")
    };
    let expired = SequencerAuthorization::new(fixed("keyId"), fixed("publicKey"), 1, 9);
    let failure =
        verify_sequencer_signature(&canonical, &[expired]).expect_err("expired signing authority");
    assert_eq!(failure.check, ReceiptCheck::SequencerSignature);
}

#[test]
fn hostile_certificate_unchecked_records_only_independently_rejected_attacks() {
    let expected = BTreeSet::from([
        HostileAttack::AlteredBalance,
        HostileAttack::AlteredReceipt,
        HostileAttack::ResignedReceipt,
        HostileAttack::SubthresholdCertificate,
        HostileAttack::TruncatedProof,
        HostileAttack::ReorderedEvents,
        HostileAttack::WithheldAvailability,
    ]);
    for attack in &expected {
        let failure = reject_attack(*attack).expect("real rejection before coverage");
        assert!(!failure.reason.is_empty());
        match attack {
            HostileAttack::TruncatedProof => assert_eq!(failure.consumer, "receipt-coverage"),
            HostileAttack::WithheldAvailability => assert_eq!(failure.reason, "MissingClass"),
            _ => {}
        }
    }
    assert_eq!(
        hostile_node::agent_hostile_node_harness().expect("complete real hostile harness"),
        expected
    );
}
