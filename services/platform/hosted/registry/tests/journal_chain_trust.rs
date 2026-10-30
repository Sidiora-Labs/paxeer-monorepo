#[allow(dead_code)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/chain.rs"]
mod chain;
#[allow(dead_code)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/mod.rs"]
mod support;

use std::fs;
use std::os::unix::fs::{symlink, PermissionsExt as _};
use std::path::PathBuf;

use layerx_platform_registry::FileDeploymentJournal;
use layerx_programs::{
    chain_evidence::programs_state_key,
    chain_proof::{decode::decode_commitment_proof, trust_store::TrustRoot},
    InterfaceEntryPoint, ProgramId, ProgramInterface, UpgradePolicy, ValueSchema, ValueType,
};
use serde_json::json;
use sha2::{Digest as _, Sha256};

struct Directory(PathBuf);

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn journal_verifies_deployments_from_chain_trust() {
    let root = Directory(
        PathBuf::from(env!("CARGO_TARGET_TMPDIR"))
            .join(format!("journal-chain-trust-{}", std::process::id(),)),
    );
    fs::create_dir_all(&root.0).unwrap();
    let trust_path = root.0.join("trust-root");
    let journal_path = root.0.join("journal");
    let program = ProgramId::new([31; 32]).unwrap();
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program.bytes());
    let key = programs_state_key(&key);
    let code_hash: [u8; 32] = Sha256::digest(support::WASM_V1).into();
    let mut record = vec![1];
    record.extend_from_slice(&[32; 32]);
    record.extend_from_slice(&code_hash);
    record.extend_from_slice(&1_u16.to_be_bytes());
    record.extend_from_slice(&7_u32.to_be_bytes());
    let pairs = vec![(key.clone(), record)];
    let fixture = chain::ChainFixture::new(
        "registry-chain-trust",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        pairs.clone(),
    )
    .unwrap();
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let trust = TrustRoot::from_head(&head, fixture.verifier.validators.clone()).unwrap();
    let trust_bytes = trust.canonical_bytes().unwrap();
    fs::write(&trust_path, &trust_bytes).unwrap();
    fs::set_permissions(&trust_path, fs::Permissions::from_mode(0o600)).unwrap();
    let interface = ProgramInterface::bind(
        support::WASM_V1,
        1,
        vec![InterfaceEntryPoint {
            name: "call".to_owned(),
            discriminator: [1, 2, 3, 4],
            calldata: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            response: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            capabilities: Vec::new(),
            event_topics: Vec::new(),
            failures: Vec::new(),
        }],
    )
    .unwrap();
    let package = |commitment: Vec<u8>, code: &[u8], interface: Option<&[u8]>| {
        serde_json::to_vec(&json!({"commitment": commitment, "code": code, "interface": interface}))
            .unwrap()
    };
    let proof = package(
        fixture.encode_key(&key).unwrap(),
        support::WASM_V1,
        Some(interface.canonical_encoding()),
    );
    let now = u64::try_from(head.time_ms()).unwrap();
    let open = || {
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), &trust_path, 1_000)
            .unwrap()
    };
    let journal = open();
    let evidence = journal.append_chain_deployment(&proof, now).unwrap();
    assert_eq!(evidence.program(), program);
    assert_eq!(evidence.version(), 7);
    assert_eq!(evidence.code_hash(), code_hash);
    assert_eq!(evidence.abi_version(), 1);
    assert_eq!(
        evidence.upgrade_policy(),
        UpgradePolicy::Authority([32; 32])
    );
    assert_eq!(evidence.state_height(), chain::STATE_HEIGHT);
    assert_eq!(evidence.state_root(), head.app_hash());
    assert_eq!(evidence.module(), support::WASM_V1);
    assert_eq!(evidence.interface(), Some(&interface));
    let saved = journal_path.join("chain-proofs").join(format!(
        "{}.chain-deployment",
        layerx_programs::hex::encode(&Sha256::digest(&proof)),
    ));
    assert_eq!(fs::read(&saved).unwrap(), proof);
    assert_eq!(
        open().append_chain_deployment(&proof, now).unwrap(),
        evidence
    );
    assert!(journal
        .append_chain_deployment(&proof, now + 1_001)
        .is_err());

    let bad_code = package(
        fixture.encode_key(&key).unwrap(),
        support::WASM_V2,
        Some(interface.canonical_encoding()),
    );
    assert!(journal.append_chain_deployment(&bad_code, now).is_err());
    let mut commitment = decode_commitment_proof(&fixture.encode_key(&key).unwrap()).unwrap();
    commitment.proof_ops[0].data[0] ^= 1;
    assert!(journal
        .append_chain_deployment(
            &package(commitment.encode().unwrap(), support::WASM_V1, None),
            now
        )
        .is_err());
    let mut commitment = decode_commitment_proof(&fixture.encode_key(&key).unwrap()).unwrap();
    *commitment.commit.last_mut().unwrap() ^= 1;
    assert!(journal
        .append_chain_deployment(
            &package(commitment.encode().unwrap(), support::WASM_V1, None),
            now
        )
        .is_err());

    for (chain_id, seeds) in [
        ("other-chain", [[1; 32], [2; 32], [3; 32], [4; 32]]),
        (
            "registry-chain-trust",
            [[11; 32], [12; 32], [13; 32], [14; 32]],
        ),
    ] {
        let untrusted = chain::ChainFixture::new(chain_id, &seeds, pairs.clone()).unwrap();
        let untrusted_proof = package(
            untrusted.encode_key(&key).unwrap(),
            support::WASM_V1,
            Some(interface.canonical_encoding()),
        );
        assert!(journal
            .append_chain_deployment(&untrusted_proof, now)
            .is_err());
    }
    assert_eq!(
        fs::read_dir(journal_path.join("chain-proofs"))
            .unwrap()
            .count(),
        1
    );

    let mut conflicting_root = trust.clone();
    conflicting_root.header_hash[0] ^= 1;
    fs::write(&trust_path, conflicting_root.canonical_bytes().unwrap()).unwrap();
    assert!(journal.append_chain_deployment(&proof, now).is_err());
    let mut newer_root = trust.clone();
    newer_root.height += 1;
    fs::write(&trust_path, newer_root.canonical_bytes().unwrap()).unwrap();
    assert!(journal.append_chain_deployment(&proof, now).is_err());
    fs::write(&trust_path, &trust_bytes).unwrap();
    fs::set_permissions(&trust_path, fs::Permissions::from_mode(0o644)).unwrap();
    assert!(journal.append_chain_deployment(&proof, now).is_err());
    fs::set_permissions(&trust_path, fs::Permissions::from_mode(0o600)).unwrap();
    let link = root.0.join("trust-link");
    symlink(&trust_path, &link).unwrap();
    assert!(
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), &link, 1_000).is_err()
    );
    assert!(
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), &trust_path, 0).is_err()
    );
    fs::write(&trust_path, b"not a TrustRoot").unwrap();
    assert!(journal.append_chain_deployment(&proof, now).is_err());
    fs::remove_file(&trust_path).unwrap();
    assert!(journal.append_chain_deployment(&proof, now).is_err());
    assert_eq!(fs::read(&saved).unwrap(), proof);
}
