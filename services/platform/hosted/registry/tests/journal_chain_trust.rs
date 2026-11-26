#[allow(dead_code)]
#[path = "support/journal_chain_trust/mod.rs"]
mod support;
use support::chain;

use std::fs;
use std::os::unix::fs::{symlink, PermissionsExt as _};
use std::path::PathBuf;

use layerx_platform_registry::FileDeploymentJournal;
use layerx_programs::{
    chain_proof::{decode::decode_commitment_proof, trust_store::TrustRoot},
    DeploymentProof, ProgramId, ProgramInterface, UpgradePolicy,
};
use serde_json::json;
use sha2::{Digest as _, Sha256};

struct Directory(PathBuf);

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn package(admission: &DeploymentProof) -> Vec<u8> {
    let admission = admission
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    serde_json::to_vec(&json!({"admission": admission})).unwrap_or_else(|error| panic!("{error:?}"))
}

struct ChainFixtureCase {
    program: ProgramId,
    code_hash: [u8; 32],
    fixture: chain::ChainFixture,
    admission: DeploymentProof,
    interface: ProgramInterface,
}

fn chain_fixture() -> ChainFixtureCase {
    let support::ChainAdmission {
        chain,
        proof,
        interface,
    } = support::admission_in_store(
        "lxphost",
        "registry-chain-trust",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
    );
    ChainFixtureCase {
        program: support::program(),
        code_hash: support::code_hash(support::WASM_V1),
        fixture: chain,
        admission: proof,
        interface,
    }
}

#[test]
fn journal_verifies_deployments_from_chain_trust() {
    let root = Directory(
        PathBuf::from(env!("CARGO_TARGET_TMPDIR"))
            .join(format!("journal-chain-trust-{}", std::process::id(),)),
    );
    fs::create_dir_all(&root.0).unwrap_or_else(|error| panic!("{error:?}"));
    let trust_path = root.0.join("trust-root");
    let journal_path = root.0.join("journal");
    let ChainFixtureCase {
        program,
        code_hash,
        fixture,
        admission,
        interface,
    } = chain_fixture();
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust = TrustRoot::from_head(&head, fixture.verifier.validators.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust_bytes = trust
        .canonical_bytes()
        .unwrap_or_else(|error| panic!("{error:?}"));
    fs::write(&trust_path, &trust_bytes).unwrap_or_else(|error| panic!("{error:?}"));
    fs::set_permissions(&trust_path, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let proof = package(&admission);
    let now = u64::try_from(head.time_ms()).unwrap_or_else(|error| panic!("{error:?}"));
    let open = || {
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), &trust_path, 1_000)
            .unwrap_or_else(|error| panic!("{error:?}"))
    };
    let journal = open();
    let evidence = journal
        .append_chain_deployment(&proof, now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(evidence.program(), program);
    assert_eq!(evidence.version(), 1);
    assert_eq!(evidence.code_hash(), code_hash);
    assert_eq!(evidence.abi_version(), 1);
    assert_eq!(
        evidence.upgrade_policy(),
        UpgradePolicy::Authority(support::AUTHORITY)
    );
    assert_eq!(evidence.state_height(), chain::STATE_HEIGHT);
    assert_eq!(evidence.state_root(), head.app_hash());
    assert_eq!(evidence.module(), support::WASM_V1);
    assert_eq!(evidence.interface(), Some(&interface));
    let saved = journal_path.join("chain-proofs").join(format!(
        "{}.chain-deployment",
        layerx_programs::hex::encode(&Sha256::digest(&proof)),
    ));
    assert_eq!(
        fs::read(&saved).unwrap_or_else(|error| panic!("{error:?}")),
        proof
    );
    assert_eq!(
        open()
            .append_chain_deployment(&proof, now)
            .unwrap_or_else(|error| panic!("{error:?}")),
        evidence
    );
    assert!(journal
        .append_chain_deployment(&proof, now + 1_001)
        .is_err());

    assert_invalid_proofs(&journal, &admission, now, &journal_path);
    assert_invalid_trust_roots(&journal, &trust, &trust_path, &trust_bytes, &proof, now);
}

fn assert_invalid_proofs(
    journal: &FileDeploymentJournal,
    admission: &DeploymentProof,
    now: u64,
    journal_path: &std::path::Path,
) {
    let mut bad_code = admission.clone();
    bad_code.deployment.code = support::WASM_V2.to_vec();
    assert!(journal
        .append_chain_deployment(&package(&bad_code), now)
        .is_err());

    let mut bad_proof = admission.clone();
    bad_proof.state.commitment.proof_ops[0].data[0] ^= 1;
    bad_proof.state.state.ops[0].data[0] ^= 1;
    bad_proof.deployment.program_state.ops[0].data[0] ^= 1;
    assert!(journal
        .append_chain_deployment(&package(&bad_proof), now)
        .is_err());

    let mut bad_commit = admission.clone();
    *bad_commit
        .state
        .commitment
        .commit
        .last_mut()
        .unwrap_or_else(|| panic!("expected a value")) ^= 1;
    assert!(journal
        .append_chain_deployment(&package(&bad_commit), now)
        .is_err());

    for (chain_id, seeds) in [
        ("other-chain", [[1; 32], [2; 32], [3; 32], [4; 32]]),
        (
            "registry-chain-trust",
            [[11; 32], [12; 32], [13; 32], [14; 32]],
        ),
    ] {
        let untrusted = support::admission_in_store("lxphost", chain_id, &seeds);
        let untrusted_proof = package(&untrusted.proof);
        assert!(journal
            .append_chain_deployment(&untrusted_proof, now)
            .is_err());
    }
    assert_eq!(
        fs::read_dir(journal_path.join("chain-proofs"))
            .unwrap_or_else(|error| panic!("{error:?}"))
            .count(),
        1
    );
}

fn assert_invalid_trust_roots(
    journal: &FileDeploymentJournal,
    trust: &TrustRoot,
    trust_path: &std::path::Path,
    trust_bytes: &[u8],
    proof: &[u8],
    now: u64,
) {
    let journal_path = trust_path.with_file_name("journal");
    let saved = journal_path.join("chain-proofs").join(format!(
        "{}.chain-deployment",
        layerx_programs::hex::encode(&Sha256::digest(proof))
    ));
    let mut conflicting_root = trust.clone();
    conflicting_root.header_hash[0] ^= 1;
    fs::write(
        trust_path,
        conflicting_root
            .canonical_bytes()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(journal.append_chain_deployment(proof, now).is_err());
    let mut newer_root = trust.clone();
    newer_root.height += 1;
    fs::write(
        trust_path,
        newer_root
            .canonical_bytes()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(journal.append_chain_deployment(proof, now).is_err());
    fs::write(trust_path, trust_bytes).unwrap_or_else(|error| panic!("{error:?}"));
    fs::set_permissions(trust_path, fs::Permissions::from_mode(0o644))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(journal.append_chain_deployment(proof, now).is_err());
    fs::set_permissions(trust_path, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let link = trust_path.with_file_name("trust-link");
    symlink(trust_path, &link).unwrap_or_else(|error| panic!("{error:?}"));
    assert!(
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), &link, 1_000).is_err()
    );
    assert!(
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), trust_path, 0).is_err()
    );
    fs::write(trust_path, b"not a TrustRoot").unwrap_or_else(|error| panic!("{error:?}"));
    assert!(journal.append_chain_deployment(proof, now).is_err());
    fs::remove_file(trust_path).unwrap_or_else(|error| panic!("{error:?}"));
    assert!(journal.append_chain_deployment(proof, now).is_err());
    assert_eq!(
        fs::read(&saved).unwrap_or_else(|error| panic!("{error:?}")),
        proof
    );
}

#[test]
fn journal_refuses_deployment_outside_the_lxphost_store() {
    let root = Directory(
        PathBuf::from(env!("CARGO_TARGET_TMPDIR"))
            .join(format!("journal-wrong-store-{}", std::process::id())),
    );
    fs::create_dir_all(&root.0).unwrap_or_else(|error| panic!("{error:?}"));
    let trust_path = root.0.join("trust-root");
    let journal_path = root.0.join("journal");
    let support::ChainAdmission {
        chain: fixture,
        proof: admission,
        ..
    } = support::admission_in_store(
        "layerx",
        "registry-chain-trust",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
    );
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust = TrustRoot::from_head(&head, fixture.verifier.validators.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    fs::write(
        &trust_path,
        trust
            .canonical_bytes()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    fs::set_permissions(&trust_path, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let commitment = admission
        .state
        .commitment
        .encode()
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        decode_commitment_proof(&commitment)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .store_name,
        "layerx"
    );
    let proof = package(&admission);
    let journal =
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), &trust_path, 1_000)
            .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        fs::read_dir(&journal_path)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .count(),
        0
    );
    assert_eq!(
        journal
            .append_chain_deployment(
                &proof,
                u64::try_from(head.time_ms()).unwrap_or_else(|error| panic!("{error:?}"))
            )
            .err()
            .unwrap_or_else(|| panic!("expected an error")),
        "deployment proof must use the lxphost store"
    );
    assert_eq!(
        fs::read_dir(&journal_path)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .count(),
        0
    );
}
