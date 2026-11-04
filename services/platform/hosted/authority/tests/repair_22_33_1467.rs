//! Task 22.33.1467 closes audit 22.30.85 receipt-staging-recovery.
//! Proof: restart_after_partial_write, restart_before_file_sync,
//! restart_before_rename, restart_before_directory_sync, restart_after_directory_sync,
//! legacy_pending_is_quarantined_before_replay_and_retention.
//! No out-of-path wiring is required. The bounded build/verify order forbids a
//! baseline verify; no unchanged-tree failures are claimed.

include!("../src/main.rs");

use chain_readiness_tests::{base64, must};
use ed25519_dalek::Signer as _;
use human::publication_interruption::{arm, Boundary};
use serde_json::{json as document, Value};
use sha2::{Digest as _, Sha256};
use std::os::unix::fs::{DirBuilderExt as _, MetadataExt as _, OpenOptionsExt as _};
use std::path::Path;

fn wire_proof(proof: &layerx_proof::merkle::Proof) -> String {
    let mut writer = layerx_wire::encode::Encoder::new(2048);
    must(writer.structure_header(0x4d50));
    must(writer.u32(proof.leaf_index()));
    must(writer.u32(proof.leaf_count()));
    must(writer.u8(u8::try_from(proof.siblings().len()).unwrap()));
    must(writer.bytes(&proof.siblings().concat(), 1024));
    hex::encode(&writer.finish())
}

fn receipt_records() -> Vec<Value> {
    let corpus: Value = must(serde_json::from_str(include_str!(
        "fixtures/native-module-outcomes.json"
    )));
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x22; 32]);
    let public = signer.verifying_key().to_bytes();
    let key_id: [u8; 32] = Sha256::digest(public).into();
    corpus["receipts"].as_array().unwrap()[..4]
        .iter()
        .map(|row| {
            assert_eq!(row["sequencer_public_key"], hex::encode(&public));
            let receipt = must(hex::decode(row["receipt"].as_str().unwrap()));
            must(layerx_proof::receipt::verify_sequencer_signature(
                &receipt, public,
            ));
            let maintenance = must(hex::decode(row["maintenance"].as_str().unwrap()));
            let leaves = [&receipt[..], &maintenance[..]];
            let (proof, root) = must(layerx_proof::merkle::build_proof(&leaves, 0));
            let (maintenance_proof, _) = must(layerx_proof::merkle::build_proof(&leaves, 1));
            let mut header = must(hex::decode(row["header"].as_str().unwrap()));
            let original = must(layerx_wire::receipt::decode_batch_header(&header));
            for (old, new) in [
                (original.sequencer_id(), key_id),
                (original.receipt_merkle_root(), root),
            ] {
                let offset = header.windows(32).position(|value| value == old).unwrap();
                header[offset..offset + 32].copy_from_slice(&new);
            }
            let signature = signer.sign(&must(layerx_wire::hash::batch_header_digest(&header)));
            document!({"receipt_hex":row["receipt"],"replica_document":{
                "authority_replica_id":hex::encode(&[9;32]),
                "sequencer_public_key":hex::encode(&public),
                "batch_evidence":{"header_hex":hex::encode(&header),
                    "header_signature":hex::encode(&signature.to_bytes()),
                    "receipt_proof_hex":wire_proof(&proof),
                    "batch_identity":{"kind":"batch_maintenance_v1",
                        "receipt_hex":row["maintenance"],
                        "receipt_proof_hex":wire_proof(&maintenance_proof),
                        "activity_receipts_hex":[row["receipt"]]}}}})
        })
        .collect()
}

struct Store {
    root: PathBuf,
    records: Vec<Value>,
    policy: Value,
}

impl Store {
    fn new() -> Self {
        let mut nonce = [0; 16];
        must(getrandom::fill(&mut nonce));
        let root = must(fs::canonicalize(std::env::temp_dir()))
            .join(format!("authority-repair-1467-{}", hex::encode(&nonce)));
        must(fs::DirBuilder::new().mode(0o700).create(&root));
        must(fs::DirBuilder::new().mode(0o700).create(root.join("state")));
        let records = receipt_records();
        let policy = document!({"tenant":"repair-tenant","principal":"repair-principal",
            "account_id":hex::encode(&[1;32]),"asset_id":hex::encode(&[4;32]),
            "activities":records.iter().map(activity).collect::<Vec<_>>(),"budgets":[],
            "maximum_age_seconds":60,"maximum_age_sequences":100,"identities":[]});
        protected_write(
            &root.join("policy.json"),
            &must(serde_json::to_vec(
                &document!({"principals":[policy.clone()]}),
            )),
        );
        let public = must(hex::decode32(
            records[0]["replica_document"]["sequencer_public_key"]
                .as_str()
                .unwrap(),
        ));
        let key_id: [u8; 32] = Sha256::digest(public).into();
        protected_write(
            &root.join("keys.json"),
            &must(serde_json::to_vec(
                &document!([{"keyId":base64(&key_id),"publicKey":base64(&public),
                "firstHeight":1,"lastHeight":1000}]),
            )),
        );
        Self {
            root,
            records,
            policy,
        }
    }

    fn reopen(&self) -> Config {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let tls = must(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(
                        include_bytes!("../src/human/fixtures/server.der").to_vec(),
                    )],
                    must(PrivateKeyDer::try_from(
                        include_bytes!("../src/human/fixtures/server-key.der").to_vec(),
                    )),
                ),
        );
        let public = must(hex::decode32(
            self.records[0]["replica_document"]["sequencer_public_key"]
                .as_str()
                .unwrap(),
        ));
        let address = "127.0.0.1:1".parse().unwrap();
        let mut config = Config {
            listen: address,
            tls: Arc::new(tls),
            tokens: vec![],
            human: None,
            replica_address: address,
            replica_host: address.to_string(),
            replica_token: Zeroizing::new("unused-replica-token".to_owned()),
            evidence_read_token: None,
            replica_id: [9; 32],
            chain_receipts: must(ChainReceipts::load(
                "http://127.0.0.1:1",
                &self.root.join("keys.json"),
            )),
            chain_rpc_url: "http://127.0.0.1:1".to_owned(),
            protocol_network_id: 77,
            network_id: "repair-1467".to_owned(),
            wire_version: "3".to_owned(),
            authorization: SequencerAuthorization::new(
                Sha256::digest(public).into(),
                public,
                1,
                1000,
            ),
            sequencer_public_key: public,
        };
        config.fixture_human_policy(&self.policy, &self.root);
        config
    }

    fn state(&self) -> PathBuf {
        self.root.join("state")
    }

    fn committed(&self, index: usize) -> PathBuf {
        self.state()
            .join(format!("{}.json", activity(&self.records[index])))
    }
}

impl Drop for Store {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.root));
    }
}

fn protected_write(path: &Path, bytes: &[u8]) {
    let mut file = must(
        fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(path),
    );
    must(file.write_all(bytes));
    must(file.sync_all());
}

fn activity(record: &Value) -> String {
    hex::encode(
        &must(receipt_locator(&must(hex::decode(
            record["receipt_hex"].as_str().unwrap(),
        ))))
        .activity_id,
    )
}

fn retain(config: &Config, record: &Value) -> Result<(), ()> {
    config.human.as_ref().unwrap().retain(
        &must(hex::decode(record["receipt_hex"].as_str().unwrap())),
        &must(serde_json::to_vec(&record["replica_document"])),
        config,
    )
}

fn evidence(config: &Config) -> Vec<layerx_platform_authority::AuthorityFacts> {
    config
        .fixture_receipt_evidence()
        .unwrap_or_else(|response| {
            panic!(
                "replay failed {}: {}",
                response.status,
                String::from_utf8_lossy(&response.body)
            )
        })
}

fn assert_evidence(config: &Config, records: &[&Value]) {
    let facts = evidence(config);
    assert_eq!(facts.len(), records.len());
    for (item, record) in facts.iter().zip(records) {
        let receipt = must(hex::decode(record["receipt_hex"].as_str().unwrap()));
        let decoded = must(layerx_wire::receipt::decode(&receipt));
        let protocol = decoded.protocol().unwrap();
        assert_eq!(item.activity_id, protocol.activity_id());
        assert_eq!(item.global_sequence, protocol.global_sequence());
        assert_eq!(item.batch_id, protocol.batch_id());
    }
}

fn pending_paths(store: &Store) -> Vec<PathBuf> {
    must(fs::read_dir(store.state()))
        .map(|entry| must(entry).path())
        .filter(|path| {
            path.extension()
                .is_some_and(|extension| extension == "pending")
        })
        .collect()
}

fn exercise_boundary(boundary: Boundary) {
    for replay_first in [true, false] {
        let store = Store::new();
        let config = store.reopen();
        must(retain(&config, &store.records[0]));
        let committed = must(fs::read(store.committed(0)));
        let interrupted = {
            let _reset = arm(boundary);
            retain(&config, &store.records[1])
        };
        assert!(interrupted.is_err(), "did not interrupt {boundary:?}");
        let published = matches!(boundary, Boundary::DirectorySync | Boundary::Durable);
        let pending = pending_paths(&store);
        assert_eq!(pending.len(), usize::from(!published));
        let staging_bytes = if published {
            assert_eq!(
                must(fs::read(store.committed(1))),
                must(serde_json::to_vec(&store.records[1]))
            );
            None
        } else {
            assert!(!store.committed(1).exists());
            assert_ne!(pending[0].file_name().unwrap(), ".receipt.pending");
            assert_eq!(must(fs::metadata(&pending[0])).mode() & 0o777, 0o600);
            let bytes = must(fs::read(&pending[0]));
            let expected = must(serde_json::to_vec(&store.records[1]));
            assert_eq!(
                bytes,
                if boundary == Boundary::Write {
                    expected[..expected.len() / 2].to_vec()
                } else {
                    expected
                }
            );
            Some(bytes)
        };
        drop(config);
        let config = store.reopen();
        if replay_first {
            let expected = if published {
                vec![&store.records[0], &store.records[1]]
            } else {
                vec![&store.records[0]]
            };
            assert_evidence(&config, &expected);
        }
        must(retain(&config, &store.records[2]));
        assert!(pending_paths(&store).is_empty());
        if let Some(bytes) = staging_bytes {
            let quarantine = store.state().join(".receipt-quarantine");
            assert_eq!(must(fs::metadata(&quarantine)).mode() & 0o777, 0o700);
            let entries: Vec<_> = must(fs::read_dir(quarantine))
                .map(|entry| must(entry).path())
                .collect();
            assert_eq!(entries.len(), 1);
            assert_eq!(must(fs::read(&entries[0])), bytes);
        }
        let expected = if published {
            vec![&store.records[0], &store.records[1], &store.records[2]]
        } else {
            vec![&store.records[0], &store.records[2]]
        };
        assert_evidence(&config, &expected);
        must(retain(&config, &store.records[1]));
        assert_evidence(
            &config,
            &[&store.records[0], &store.records[1], &store.records[2]],
        );
        assert_eq!(must(fs::read(store.committed(0))), committed);
        drop(config);
        let reopened = store.reopen();
        assert_evidence(
            &reopened,
            &[&store.records[0], &store.records[1], &store.records[2]],
        );
    }
}

#[test]
fn restart_after_partial_write() {
    exercise_boundary(Boundary::Write);
}

#[test]
fn restart_before_file_sync() {
    exercise_boundary(Boundary::Sync);
}

#[test]
fn restart_before_rename() {
    exercise_boundary(Boundary::Rename);
}

#[test]
fn restart_before_directory_sync() {
    exercise_boundary(Boundary::DirectorySync);
}

#[test]
fn restart_after_directory_sync() {
    exercise_boundary(Boundary::Durable);
}

#[test]
fn legacy_pending_is_quarantined_before_replay_and_retention() {
    for replay_first in [true, false] {
        for leftover in [
            Vec::new(),
            b"{\"receipt_hex\":".to_vec(),
            must(serde_json::to_vec(&receipt_records()[1])),
        ] {
            let store = Store::new();
            let config = store.reopen();
            must(retain(&config, &store.records[0]));
            protected_write(&store.state().join(".receipt.pending"), &leftover);
            drop(config);
            let config = store.reopen();
            if replay_first {
                assert_evidence(&config, &[&store.records[0]]);
            }
            must(retain(&config, &store.records[2]));
            assert_evidence(&config, &[&store.records[0], &store.records[2]]);
            assert!(!store.state().join(".receipt.pending").exists());
            let quarantined: Vec<_> = must(fs::read_dir(store.state().join(".receipt-quarantine")))
                .map(|entry| must(fs::read(must(entry).path())))
                .collect();
            assert_eq!(quarantined, vec![leftover]);
        }
    }
}

#[test]
fn repeated_interruptions_keep_unique_staging_and_quarantined_bytes() {
    let store = Store::new();
    let config = store.reopen();
    must(retain(&config, &store.records[0]));
    let first = {
        let _reset = arm(Boundary::Write);
        assert!(retain(&config, &store.records[1]).is_err());
        pending_paths(&store).pop().unwrap()
    };
    let first_bytes = must(fs::read(&first));
    let second = {
        let _reset = arm(Boundary::Rename);
        assert!(retain(&config, &store.records[2]).is_err());
        pending_paths(&store).pop().unwrap()
    };
    assert_ne!(first, second);
    let second_bytes = must(fs::read(&second));
    drop(config);
    let config = store.reopen();
    assert_evidence(&config, &[&store.records[0]]);
    let mut quarantined: Vec<_> = must(fs::read_dir(store.state().join(".receipt-quarantine")))
        .map(|entry| must(fs::read(must(entry).path())))
        .collect();
    let mut expected = vec![first_bytes, second_bytes];
    quarantined.sort();
    expected.sort();
    assert_eq!(quarantined, expected);
    must(retain(&config, &store.records[3]));
    assert_evidence(&config, &[&store.records[0], &store.records[3]]);
}

#[test]
fn recovery_keeps_rejecting_corrupt_committed_records_and_unknown_files() {
    for unknown in [false, true] {
        let store = Store::new();
        let config = store.reopen();
        must(retain(&config, &store.records[0]));
        protected_write(&store.state().join(".receipt.pending"), b"incomplete");
        let path = if unknown {
            store.state().join("unexpected.json")
        } else {
            store.committed(0)
        };
        if unknown {
            protected_write(&path, b"{}");
        } else {
            must(fs::write(&path, b"{}"));
        }
        drop(config);
        let config = store.reopen();
        assert!(config.fixture_receipt_evidence().is_err());
        assert_eq!(must(fs::read(&path)), b"{}");
        assert!(pending_paths(&store).is_empty());
    }
}

#[test]
fn recovery_refuses_symlinked_pending_files_or_quarantine() {
    for quarantine in [false, true] {
        let store = Store::new();
        let config = store.reopen();
        must(retain(&config, &store.records[0]));
        let source = if quarantine {
            store.state().join(".receipt-quarantine")
        } else {
            store.state().join(".receipt.pending")
        };
        must(std::os::unix::fs::symlink(store.committed(0), &source));
        drop(config);
        let config = store.reopen();
        assert!(config.fixture_receipt_evidence().is_err());
        assert!(retain(&config, &store.records[1]).is_err());
        assert_eq!(
            must(fs::read(store.committed(0))),
            must(serde_json::to_vec(&store.records[0]))
        );
    }
}
