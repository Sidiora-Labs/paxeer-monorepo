use std::{collections::BTreeMap, fs, path::Path};

use layerx_programs_arbiter::{
    ics23::{decode_existence, ExistenceProof},
    AuthenticatedCatalogue, BoundaryProof, ChainCommitProof, ReplayError, ReplayMetadata,
    VerifiedReplay, VerifiedReplayAuthority,
};
use layerx_programs_runtime::portable_replay::PortableReplayRecord;
use layerx_proof::{
    inclusion::{verify_header, verify_receipt, SequencerAuthorization, VerifiedBatchHeader},
    merkle::{decode_proof, Proof},
    receipt::{
        verify_outcome_maintained_chain, verify_program_preexecution_rejection_maintained_chain,
        AuthorizedBatch, MaintainedOutcomeEvidence, VerifiedReceipt,
    },
};
use serde_json::Value;
use sha2::{Digest, Sha256};

fn field<'a>(v: &'a Value, key: &str) -> &'a str {
    v[key]
        .as_str()
        .unwrap_or_else(|| panic!("required native {key}"))
}
enum ReplayFiles {
    Native(std::path::PathBuf),
}
impl ReplayFiles {
    fn read(&self, path: &Path) -> Vec<u8> {
        match self {
            Self::Native(directory) => fs::read(directory.join(path)).expect("genuine native file"),
        }
    }
}
fn read(files: &ReplayFiles, directory: &Path, v: &Value, key: &str) -> Vec<u8> {
    files.read(&directory.join(field(v, key)))
}
fn hash(v: &Value, key: &str) -> [u8; 32] {
    let s = field(v, key);
    assert_eq!(s.len(), 64);
    std::array::from_fn(|i| {
        u8::from_str_radix(&s[2 * i..2 * i + 2], 16).expect("native pinned hash")
    })
}
fn verified(
    files: &ReplayFiles,
    directory: &Path,
    v: &Value,
    authorization: &SequencerAuthorization,
) -> (VerifiedReceipt, VerifiedBatchHeader, Proof) {
    let bytes = read(files, directory, v, "receipt_path");
    let header = read(files, directory, v, "header_path");
    let signature: [u8; 64] = read(files, directory, v, "header_signature_path")
        .try_into()
        .expect("actual header signature");
    let signed =
        verify_header(&header, &signature, authorization).expect("genuine sequencer header");
    let proof = decode_proof(&read(files, directory, v, "proof_path"))
        .expect("actual receipt Merkle proof");
    verify_receipt(&bytes, &proof, &header, &signature, authorization)
        .expect("actual signed receipt inclusion");
    let maintenance = read(files, directory, v, "maintenance_path");
    let maintenance_proof = decode_proof(&read(files, directory, v, "maintenance_proof_path"))
        .expect("actual maintenance proof");
    verify_receipt(
        &maintenance,
        &maintenance_proof,
        &header,
        &signature,
        authorization,
    )
    .expect("maintenance inclusion");
    let record = layerx_wire::batch_maintenance::decode_maintenance(&maintenance)
        .expect("maintenance decode");
    record
        .verify_header(signed.header())
        .expect("maintenance exact signed header");
    let decoded = layerx_wire::receipt::decode(&bytes).expect("canonical receipt");
    let protocol = decoded.protocol().expect("protocol receipt");
    let count = u32::try_from(signed.header().last_sequence() - signed.header().first_sequence())
        .expect("receipt count");
    let batch_id = layerx_wire::hash::receipt_execution_batch_id_maintenance(
        protocol,
        signed.header(),
        record.occupancy(),
        count,
    )
    .expect("actual execution batch id");
    assert_eq!(batch_id, protocol.batch_id());
    let batch = AuthorizedBatch::new(
        batch_id,
        protocol.asset(),
        signed.header().previous_state_root(),
        signed.header().resulting_state_root(),
        authorization.public_key(),
    );
    let receipts: Vec<Vec<u8>> = v["receipts"]
        .as_array()
        .expect("complete receipt chain")
        .iter()
        .map(|p| files.read(&directory.join(p.as_str().expect("receipt path"))))
        .collect();
    assert_eq!(receipts.len(), count as usize);
    let evidence = MaintainedOutcomeEvidence {
        header: &header,
        header_signature: &signature,
        activity_proof: &proof,
        maintenance: &maintenance,
        maintenance_proof: &maintenance_proof,
        authorization,
    };
    let verified = if protocol.module_id() == 9
        && protocol.operation() == 3
        && protocol.result_code() != 0
        && protocol.program_outcome().is_none()
    {
        verify_program_preexecution_rejection_maintained_chain(&bytes, &batch, &evidence, &receipts)
    } else {
        verify_outcome_maintained_chain(&bytes, &batch, &evidence, &receipts)
    }
    .expect("real maintained receipt chain");
    let mut bad_signature = signature;
    bad_signature[0] ^= 1;
    assert!(verify_header(&header, &bad_signature, authorization).is_err());
    (verified, signed, proof)
}

struct Capture {
    path: String,
    receipt: VerifiedReceipt,
    header: VerifiedBatchHeader,
    admission: layerx_client::evidence::VerifiedAdmissionPrestate,
    commit: ChainCommitProof,
    host_store_root: [u8; 32],
    host_multistore_proof: Vec<u8>,
    code_proofs: BTreeMap<[u8; 32], ExistenceProof>,
}
impl Capture {
    fn catalogue(
        &self,
        code: &BTreeMap<[u8; 32], Vec<u8>>,
    ) -> Result<AuthenticatedCatalogue, ReplayError> {
        AuthenticatedCatalogue::verify_with_module(
            &self.admission,
            &self.commit,
            self.host_store_root,
            &self.host_multistore_proof,
            &self.code_proofs,
            code,
        )
    }
}

fn hex(v: &Value, key: &str) -> Vec<u8> {
    let value = field(v, key);
    assert_eq!(value.len() % 2, 0, "complete host proof hex");
    (0..value.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&value[i..i + 2], 16).expect("host proof hex"))
        .collect()
}

fn chain_commit(v: &Value) -> ChainCommitProof {
    let proof = decode_existence(&hex(v, "proof")).expect("sequencer key-list proof");
    assert_eq!(proof.value, hex(v, "value"));
    let commit = ChainCommitProof {
        height: v["height"].as_u64().expect("committed block height"),
        app_hash: hash(v, "app_hash"),
        store_root: hash(v, "store_root"),
        multistore_proof: hex(v, "multistore_proof"),
        receipt_key: hex(v, "key"),
        receipt_proof: proof,
    };
    commit
        .receipt_keys()
        .expect("app-hash authenticated receipt keys");
    commit
}

struct Fixture {
    files: ReplayFiles,
    directory: std::path::PathBuf,
    replay: Value,
    captures: Vec<Capture>,
    code: BTreeMap<[u8; 32], Vec<u8>>,
}
impl Fixture {
    fn load() -> Self {
        let path = std::env::var("LAYERX_AUTHENTICATED_REPLAY_INPUTS")
            .expect("required genuine native replay corpus; no fallback");
        Self::from_files(
            ReplayFiles::Native(std::path::PathBuf::new()),
            Path::new(&path),
        )
    }

    fn from_files(files: ReplayFiles, path: &Path) -> Self {
        let directory = path.parent().expect("replay manifest parent").to_path_buf();
        let replay: Value = serde_json::from_slice(&files.read(path)).expect("native replay JSON");
        let inputs = std::path::PathBuf::from(
            std::env::var("LAYERX_ARBITER_ADMISSION_INPUTS")
                .expect("required real signed admission corpus"),
        );
        let admission_directory = inputs.parent().expect("native admission corpus parent");
        let manifest: Value =
            serde_json::from_slice(&files.read(&inputs)).expect("native admission JSON");
        let network = u32::try_from(manifest["network_id"].as_u64().expect("network"))
            .expect("network width");
        let authorization = SequencerAuthorization::new(
            hash(&manifest, "sequencer_id"),
            hash(&manifest, "sequencer_public_key"),
            manifest["first_batch_number"]
                .as_u64()
                .expect("first batch"),
            manifest["last_batch_number"].as_u64().expect("last batch"),
        );
        let mut code = BTreeMap::new();
        let captures = manifest["captures"]
            .as_array()
            .expect("actual captures")
            .iter()
            .map(|v| {
                let (receipt, header, _) = verified(&files, admission_directory, v, &authorization);
                let admission = layerx_client::evidence::verify_arbiter_admission_v3(
                    &read(&files, admission_directory, v, "v3_path"),
                    &receipt,
                    network,
                )
                .expect("genuine sealed V3 prestate");
                if admission.activity().activity_type().module()
                    == layerx_types::payload::ModuleId::Programs
                    && admission.activity().activity_type().ordinal() == 1
                {
                    let payload = admission.activity().payload();
                    assert!(payload.len() >= 108);
                    let length = u32::from_be_bytes(
                        payload[100..104].try_into().expect("deploy code length"),
                    ) as usize;
                    let interface_length = u32::from_be_bytes(
                        payload[104..108]
                            .try_into()
                            .expect("deploy interface length"),
                    ) as usize;
                    let start = 108usize
                        .checked_add(interface_length)
                        .expect("bounded interface");
                    assert_eq!(start.checked_add(length), Some(payload.len()));
                    let wasm = payload[start..].to_vec();
                    let digest: [u8; 32] = Sha256::digest(&wasm).into();
                    assert_eq!(&digest, &payload[68..100]);
                    code.insert(digest, wasm);
                }
                let commit = chain_commit(&v["chain_commit"]);
                commit
                    .verify_receipt(&receipt)
                    .expect("committed signed host receipt");
                let mut code_proofs = BTreeMap::new();
                for entry in v["code_proofs"]
                    .as_array()
                    .expect("host module 9 code-blob proofs")
                {
                    let proof =
                        decode_existence(&read(&files, admission_directory, entry, "proof_path"))
                            .expect("host code-blob existence proof");
                    assert!(
                        code_proofs
                            .insert(hash(entry, "code_hash"), proof)
                            .is_none(),
                        "exactly one host proof per deployed code blob"
                    );
                }
                Capture {
                    path: field(v, "receipt_path").to_owned(),
                    receipt,
                    header,
                    admission,
                    commit,
                    host_store_root: hash(v, "host_store_root"),
                    host_multistore_proof: read(
                        &files,
                        admission_directory,
                        v,
                        "host_multistore_proof_path",
                    ),
                    code_proofs,
                }
            })
            .collect();
        Self {
            files,
            directory,
            replay,
            captures,
            code,
        }
    }
    fn receipt(&self, replay: &Value) -> &Capture {
        let name = field(replay, "receipt");
        self.captures
            .iter()
            .find(|c| Path::new(&c.path).file_name().and_then(|p| p.to_str()) == Some(name))
            .expect("replay exact genuine receipt binding")
    }
    fn file(&self, v: &Value, key: &str) -> Vec<u8> {
        self.files.read(&self.directory.join(field(v, key)))
    }
}

#[test]
fn genuine_signed_serial_scheduled_trap_steps_and_bound_refusals() {
    assert_signed_serial_scheduled_trap_steps_and_bound_refusals(&Fixture::load());
}
fn assert_signed_serial_scheduled_trap_steps_and_bound_refusals(fixture: &Fixture) {
    let captures = fixture.replay["captures"]
        .as_array()
        .expect("replay captures");
    assert_eq!(captures.len(), 4);
    let mut verified_instructions = 0;
    let mut verified_traps = 0;
    for capture in captures {
        let trusted = fixture.receipt(capture);
        let proof = decode_existence(&fixture.file(capture, "metadata_proof"))
            .expect("native module9 proof");
        let portable =
            PortableReplayRecord::decode_untrusted(&fixture.file(capture, "witness"), 1_048_576)
                .expect("real native replay witness");
        let catalogue = trusted
            .catalogue(&fixture.code)
            .expect("genuine authenticated complete catalogue");
        let replay = VerifiedReplay::verify(
            &trusted.receipt,
            &trusted.admission,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &proof,
            &portable.authority_bytes,
            &portable.hosts_bytes,
            catalogue,
        )
        .expect("sealed replay authority");
        assert_eq!(
            replay.metadata().boundary_root,
            portable.record.boundary_root()
        );
        assert_eq!(
            replay.metadata().boundary_count as usize,
            portable.leaves.len()
        );
        assert_eq!(
            replay.metadata().witness_digest,
            portable.record.witness_digest()
        );
        let paths: Vec<BoundaryProof> = portable
            .leaves
            .iter()
            .enumerate()
            .map(|(i, leaf)| BoundaryProof {
                index: i as u32,
                leaf: leaf.clone(),
                siblings: portable
                    .merkle_path_untrusted(i as u32)
                    .expect("actual bounded Merkle path"),
            })
            .collect();
        let mut accepted = 0;
        for pair in paths.windows(2) {
            if let Ok(step) = replay.verify_step(&pair[0], &pair[1]) {
                assert_eq!(step.activity_id(), trusted.admission.activity_id());
                accepted += 1;
                let mut mutated = pair[0].clone();
                mutated.leaf[0] ^= 1;
                assert!(replay.verify_step(&mutated, &pair[1]).is_err());
                let mut index = pair[0].clone();
                index.index = replay.metadata().boundary_count;
                assert_eq!(
                    replay.verify_step(&index, &pair[1]),
                    Err(ReplayError::Bounds)
                );
                let mut path = pair[0].clone();
                path.siblings.push([0; 32]);
                assert!(replay.verify_step(&path, &pair[1]).is_err());
                assert!(replay.verify_step(&pair[1], &pair[0]).is_err());
            }
        }
        assert!(
            accepted > 0,
            "each genuine capture must replay actual engine instructions"
        );
        verified_instructions += accepted;
        if replay.metadata().terminal_status == 1 {
            let final_leaf = paths.last().expect("real terminal leaf");
            replay
                .verify_step(final_leaf, final_leaf)
                .expect("genuine final trap execute_step");
            verified_traps += 1;
        }
        let catalogue = || trusted.catalogue(&fixture.code).expect("real catalogue");
        let mut authority = portable.authority_bytes.clone();
        authority[0] ^= 1;
        assert!(VerifiedReplay::verify(
            &trusted.receipt,
            &trusted.admission,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &proof,
            &authority,
            &portable.hosts_bytes,
            catalogue()
        )
        .is_err());
        let mut hosts = portable.hosts_bytes.clone();
        hosts[0] ^= 1;
        assert!(VerifiedReplay::verify(
            &trusted.receipt,
            &trusted.admission,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &proof,
            &portable.authority_bytes,
            &hosts,
            catalogue()
        )
        .is_err());
        let mut metadata = proof.clone();
        metadata.value[32] ^= 1;
        assert!(VerifiedReplay::verify(
            &trusted.receipt,
            &trusted.admission,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &metadata,
            &portable.authority_bytes,
            &portable.hosts_bytes,
            catalogue()
        )
        .is_err());
        let mut wrong_key = proof.clone();
        wrong_key.key[14] ^= 1;
        assert!(VerifiedReplay::verify(
            &trusted.receipt,
            &trusted.admission,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &wrong_key,
            &portable.authority_bytes,
            &portable.hosts_bytes,
            catalogue()
        )
        .is_err());
        let mut changed_code = fixture.code.clone();
        let code = changed_code
            .get_mut(&replay.metadata().code_hash)
            .expect("real deployed code");
        code[0] ^= 1;
        assert!(trusted.catalogue(&changed_code).is_err());
        assert!(trusted.catalogue(&BTreeMap::new()).is_err());
    }
    assert!(verified_instructions >= 4);
    assert_eq!(verified_traps, 1);
}

#[test]
fn authenticated_admission_authority_reaches_real_bounded_step_verifier() {
    let fixture = Fixture::load();
    let captures = fixture.replay["captures"]
        .as_array()
        .expect("real canonical replay captures");
    assert_eq!(captures.len(), 4);
    let mut verified_instructions = 0;
    let mut verified_traps = 0;
    let mut verified_scheduled = 0;
    for capture in captures {
        let trusted = fixture.receipt(capture);
        let proof = decode_existence(&fixture.file(capture, "metadata_proof"))
            .expect("real metadata state proof");
        let portable =
            PortableReplayRecord::decode_untrusted(&fixture.file(capture, "witness"), 1_048_576)
                .expect("real captured replay record");
        let authority = VerifiedReplayAuthority::derive(
            &trusted.receipt,
            &trusted.admission,
            &trusted.header,
            &trusted.commit,
        )
        .expect("authority independently derived from sealed historical admission");
        authority
            .verify_native(&portable.authority_bytes)
            .expect("canonical native authority equality");
        assert_eq!(authority.canonical_bytes(), portable.authority_bytes);
        assert_native_field_refusals(&authority, &portable.authority_bytes);
        let other_header = fixture
            .captures
            .iter()
            .find(|other| other.header.digest() != trusted.header.digest())
            .expect("genuine distinct signed batch header");
        assert!(VerifiedReplayAuthority::derive(
            &trusted.receipt,
            &trusted.admission,
            &other_header.header,
            &trusted.commit,
        )
        .is_err());
        let other_admission = fixture
            .captures
            .iter()
            .find(|other| other.admission.activity_id() != trusted.admission.activity_id())
            .expect("genuine distinct signed admission");
        assert!(VerifiedReplayAuthority::derive(
            &trusted.receipt,
            &other_admission.admission,
            &trusted.header,
            &trusted.commit,
        )
        .is_err());
        let mut bad_proof = trusted.commit.clone();
        bad_proof.receipt_proof.value[0] ^= 1;
        assert!(VerifiedReplayAuthority::derive(
            &trusted.receipt,
            &trusted.admission,
            &trusted.header,
            &bad_proof,
        )
        .is_err());
        let catalogue = || {
            trusted
                .catalogue(&fixture.code)
                .expect("genuine previous-root catalogue and signed deployed code")
        };
        let replay = VerifiedReplay::verify_with_batch_context(
            &trusted.receipt,
            &trusted.admission,
            &trusted.header,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &proof,
            &portable.authority_bytes,
            &portable.hosts_bytes,
            catalogue(),
        )
        .expect("sealed replay after independent authority comparison");
        assert_eq!(
            replay.metadata().boundary_root,
            portable.record.boundary_root()
        );
        assert_eq!(
            replay.metadata().boundary_count as usize,
            portable.leaves.len()
        );
        assert_eq!(
            replay.metadata().witness_digest,
            portable.record.witness_digest()
        );
        let paths: Vec<BoundaryProof> = portable
            .leaves
            .iter()
            .enumerate()
            .map(|(index, leaf)| BoundaryProof {
                index: index as u32,
                leaf: leaf.clone(),
                siblings: portable
                    .merkle_path_untrusted(index as u32)
                    .expect("actual boundary Merkle path"),
            })
            .collect();
        let mut accepted = 0;
        for pair in paths.windows(2) {
            if let Ok(step) = replay.verify_step(&pair[0], &pair[1]) {
                assert_eq!(step.activity_id(), trusted.admission.activity_id());
                accepted += 1;
                let mut bad_leaf = pair[0].clone();
                bad_leaf.leaf[0] ^= 1;
                assert!(replay.verify_step(&bad_leaf, &pair[1]).is_err());
                let mut bad_index = pair[0].clone();
                bad_index.index = replay.metadata().boundary_count;
                assert_eq!(
                    replay.verify_step(&bad_index, &pair[1]),
                    Err(ReplayError::Bounds)
                );
                let mut bad_path = pair[0].clone();
                bad_path.siblings.push([0; 32]);
                assert!(replay.verify_step(&bad_path, &pair[1]).is_err());
                assert!(replay.verify_step(&pair[1], &pair[0]).is_err());
            }
        }
        assert!(
            accepted > 0,
            "each canonical native capture must replay actual instructions"
        );
        verified_instructions += accepted;
        if capture["receipt_index"].as_u64() == Some(1) {
            verified_scheduled += 1;
        }
        if replay.metadata().terminal_status == 1 {
            let terminal = paths.last().expect("real terminal boundary");
            replay
                .verify_step(terminal, terminal)
                .expect("actual canonical trap instruction");
            verified_traps += 1;
        }
        let mut bad_authority = portable.authority_bytes.clone();
        bad_authority[0] ^= 1;
        assert!(VerifiedReplay::verify_with_batch_context(
            &trusted.receipt,
            &trusted.admission,
            &trusted.header,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &proof,
            &bad_authority,
            &portable.hosts_bytes,
            catalogue(),
        )
        .is_err());
        let mut bad_hosts = portable.hosts_bytes.clone();
        bad_hosts[0] ^= 1;
        assert!(VerifiedReplay::verify_with_batch_context(
            &trusted.receipt,
            &trusted.admission,
            &trusted.header,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &proof,
            &portable.authority_bytes,
            &bad_hosts,
            catalogue(),
        )
        .is_err());
        let mut bad_metadata = proof.clone();
        bad_metadata.value[32] ^= 1;
        assert!(VerifiedReplay::verify_with_batch_context(
            &trusted.receipt,
            &trusted.admission,
            &trusted.header,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &bad_metadata,
            &portable.authority_bytes,
            &portable.hosts_bytes,
            catalogue(),
        )
        .is_err());
        let mut bad_key = proof.clone();
        bad_key.key[14] ^= 1;
        assert!(VerifiedReplay::verify_with_batch_context(
            &trusted.receipt,
            &trusted.admission,
            &trusted.header,
            &trusted.commit,
            trusted.host_store_root,
            &trusted.host_multistore_proof,
            &bad_key,
            &portable.authority_bytes,
            &portable.hosts_bytes,
            catalogue(),
        )
        .is_err());
        let mut bad_code = fixture.code.clone();
        bad_code
            .get_mut(&replay.metadata().code_hash)
            .expect("actual deployed code")[0] ^= 1;
        assert!(trusted.catalogue(&bad_code).is_err());
        assert!(trusted.catalogue(&BTreeMap::new()).is_err());
    }
    assert!(verified_instructions >= 4);
    assert_eq!(verified_traps, 1);
    assert_eq!(verified_scheduled, 1);
}

fn module_catalogue_vector() -> Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../tests/vectors/ics23-module-catalogue-app-hash.json");
    let vector: Value = serde_json::from_slice(
        &fs::read(path).expect("keeper-generated module catalogue app-hash vector"),
    )
    .expect("committed host vector JSON");
    assert_eq!(field(&vector["source"], "generator"), "modules/sequencer/keeper/ics23_module_catalogue_vector_test.go::TestModuleCatalogueAppHashVector");
    assert_eq!(
        vector["entries"]
            .as_array()
            .expect("two committed app hashes")
            .len(),
        2
    );
    assert_ne!(
        vector["entries"][0]["app_hash"],
        vector["entries"][1]["app_hash"]
    );
    assert_ne!(
        vector["entries"][0]["store_root"],
        vector["entries"][1]["store_root"]
    );
    vector
}

#[test]
fn replay_verifies_metadata_from_ics23_host_store_proof() {
    let vector = module_catalogue_vector();
    let entries = vector["entries"].as_array().expect("committed entries");
    for entry in entries {
        let record = &entry["metadata"];
        let fields = &record["fields"];
        let proof = decode_existence(&hex(record, "proof")).expect("host metadata proof");
        assert_eq!(proof.key, hex(record, "key"));
        assert_eq!(proof.value, hex(record, "value"));
        let activity_id = hash(fields, "activity_id");
        let verify = |app_hash,
                      root,
                      multistore: &[u8],
                      activity_id: &[u8; 32],
                      proof: &ExistenceProof| {
            ReplayMetadata::verify_under_app_hash(app_hash, root, multistore, activity_id, proof)
        };
        let app_hash = hash(entry, "app_hash");
        let root = hash(entry, "store_root");
        let multistore = hex(entry, "multistore_proof");
        let metadata = verify(app_hash, root, &multistore, &activity_id, &proof)
            .expect("ICS23 authenticated replay metadata");
        let number = |name: &str| fields[name].as_u64().expect("metadata number");
        assert_eq!(
            metadata,
            ReplayMetadata {
                network_id: u32::try_from(number("network_id")).unwrap(),
                activity_id,
                sequence: number("sequence"),
                previous_root: hash(fields, "previous_root"),
                program_id: hash(fields, "program_id"),
                code_hash: hash(fields, "code_hash"),
                input_digest: hash(fields, "input_digest"),
                runtime_version: u16::try_from(number("runtime_version")).unwrap(),
                abi_version: u16::try_from(number("abi_version")).unwrap(),
                fee_version: u32::try_from(number("fee_version")).unwrap(),
                metering_version: u32::try_from(number("metering_version")).unwrap(),
                maximum_boundaries: u32::try_from(number("maximum_boundaries")).unwrap(),
                maximum_bytes: u32::try_from(number("maximum_bytes")).unwrap(),
                terminal_status: u8::try_from(number("terminal_status")).unwrap(),
                boundary_count: u32::try_from(number("boundary_count")).unwrap(),
                authority_root: hash(fields, "authority_root"),
                hosts_root: hash(fields, "hosts_root"),
                boundary_root: hash(fields, "boundary_root"),
                witness_digest: hash(fields, "witness_digest"),
                result_code: i32::try_from(fields["result_code"].as_i64().unwrap()).unwrap(),
                blob_digest: hash(fields, "blob_digest"),
            }
        );
        assert_eq!(metadata.code_hash, hash(&entry["blobs"][0], "code_hash"));
        let other = entries
            .iter()
            .find(|other| other["app_hash"] != entry["app_hash"])
            .unwrap();
        assert_eq!(
            verify(
                hash(other, "app_hash"),
                root,
                &multistore,
                &activity_id,
                &proof
            ),
            Err(ReplayError::Metadata)
        );
        let mut changed = proof.clone();
        changed.value[0] ^= 1;
        assert_eq!(
            verify(app_hash, root, &multistore, &activity_id, &changed),
            Err(ReplayError::Metadata)
        );
        let mut wrong_key = proof.clone();
        wrong_key.key[0] = 5;
        assert_eq!(
            verify(app_hash, root, &multistore, &activity_id, &wrong_key),
            Err(ReplayError::Metadata)
        );
        let mut wrong_activity = activity_id;
        wrong_activity[0] ^= 1;
        assert_eq!(
            verify(app_hash, root, &multistore, &wrong_activity, &proof),
            Err(ReplayError::Metadata)
        );
        let mut wrong_root = root;
        wrong_root[0] ^= 1;
        assert_eq!(
            verify(app_hash, wrong_root, &multistore, &activity_id, &proof),
            Err(ReplayError::Metadata)
        );
        let mut wrong_multistore = multistore.clone();
        *wrong_multistore.last_mut().expect("host multistore proof") ^= 1;
        assert_eq!(
            verify(app_hash, root, &wrong_multistore, &activity_id, &proof),
            Err(ReplayError::Metadata)
        );
    }
}

#[test]
fn catalogue_refuses_module_blob_absent_from_app_hash() {
    let vector = module_catalogue_vector();
    let entries = vector["entries"].as_array().expect("committed entries");
    let load = |entry: &Value| {
        let mut proofs = BTreeMap::new();
        let mut code = BTreeMap::new();
        let blobs = entry["blobs"].as_array().expect("proven code blobs");
        assert_eq!(blobs.len(), 1);
        for blob in blobs {
            let code_hash = hash(blob, "code_hash");
            let proof = decode_existence(&hex(blob, "proof")).expect("host blob proof");
            assert_eq!(proof.key, hex(blob, "key"));
            assert_eq!(proof.value, hex(blob, "value"));
            assert_eq!(<[u8; 32]>::from(Sha256::digest(&proof.value)), code_hash);
            assert!(code.insert(code_hash, hex(blob, "value")).is_none());
            assert!(proofs.insert(code_hash, proof).is_none());
        }
        (proofs, code)
    };
    let (proofs, code) = load(&entries[0]);
    let (foreign, other_code) = load(&entries[1]);
    assert_eq!(code, other_code);
    let app_hash = hash(&entries[0], "app_hash");
    let root = hash(&entries[0], "store_root");
    let multistore = hex(&entries[0], "multistore_proof");
    let verify = |app_hash,
                  root,
                  multistore: &[u8],
                  proofs: &BTreeMap<[u8; 32], ExistenceProof>,
                  code: &BTreeMap<[u8; 32], Vec<u8>>| {
        AuthenticatedCatalogue::verify_module_blobs(app_hash, root, multistore, proofs, code)
    };
    assert_eq!(verify(app_hash, root, &multistore, &proofs, &code), Ok(()));
    assert_eq!(
        verify(
            hash(&entries[1], "app_hash"),
            hash(&entries[1], "store_root"),
            &hex(&entries[1], "multistore_proof"),
            &foreign,
            &other_code
        ),
        Ok(())
    );
    assert_eq!(
        verify(app_hash, root, &multistore, &foreign, &code),
        Err(ReplayError::Catalogue)
    );
    let code_hash = *code.keys().next().expect("deployed token blob");
    let mut absent = proofs.clone();
    absent.remove(&code_hash);
    assert_eq!(
        verify(app_hash, root, &multistore, &absent, &code),
        Err(ReplayError::Catalogue)
    );
    let mut changed = proofs.clone();
    changed.get_mut(&code_hash).unwrap().value[0] ^= 1;
    assert_eq!(
        verify(app_hash, root, &multistore, &changed, &code),
        Err(ReplayError::Catalogue)
    );
    let mut wrong_module = proofs.clone();
    wrong_module.get_mut(&code_hash).unwrap().key[2] = 8;
    assert_eq!(
        verify(app_hash, root, &multistore, &wrong_module, &code),
        Err(ReplayError::Catalogue)
    );
    let mut malformed = proofs.clone();
    malformed.get_mut(&code_hash).unwrap().leaf.hash = 0;
    assert_eq!(
        verify(app_hash, root, &multistore, &malformed, &code),
        Err(ReplayError::Catalogue)
    );
    let mut changed_code = code.clone();
    changed_code.get_mut(&code_hash).unwrap()[0] ^= 1;
    assert_eq!(
        verify(app_hash, root, &multistore, &proofs, &changed_code),
        Err(ReplayError::Catalogue)
    );
    let mut wrong_root = root;
    wrong_root[0] ^= 1;
    assert_eq!(
        verify(app_hash, wrong_root, &multistore, &proofs, &code),
        Err(ReplayError::Catalogue)
    );
    assert_eq!(
        verify(
            hash(&entries[1], "app_hash"),
            root,
            &multistore,
            &proofs,
            &code
        ),
        Err(ReplayError::Catalogue)
    );
}

fn assert_native_field_refusals(authority: &VerifiedReplayAuthority, bytes: &[u8]) {
    let domain = b"LXP/program-replay-authority/v1\0";
    assert!(bytes.starts_with(domain));
    let length = u32::from_be_bytes(
        bytes[domain.len()..domain.len() + 4]
            .try_into()
            .expect("native signed activity length"),
    ) as usize;
    let base = domain
        .len()
        .checked_add(4)
        .and_then(|value| value.checked_add(length))
        .expect("bounded signed activity");
    let footer = bytes
        .len()
        .checked_sub(272)
        .expect("exact canonical admission footer");
    assert_eq!(
        u64::from_be_bytes(
            bytes[base + 168..base + 176]
                .try_into()
                .expect("real scope marker")
        ),
        1
    );
    let scope = base + 176;
    let signer_count = usize::try_from(u64::from_be_bytes(
        bytes[scope + 208..scope + 216]
            .try_into()
            .expect("real signer count"),
    ))
    .expect("signer count width");
    let approval_count_offset = scope
        .checked_add(216)
        .and_then(|value| value.checked_add(signer_count.checked_mul(32)?))
        .expect("bounded signer list");
    let approval_count = usize::try_from(u64::from_be_bytes(
        bytes[approval_count_offset..approval_count_offset + 8]
            .try_into()
            .expect("real approval count"),
    ))
    .expect("approval count width");
    assert_eq!(
        approval_count_offset
            .checked_add(8)
            .and_then(|value| value.checked_add(approval_count.checked_mul(32)?)),
        Some(footer)
    );
    let mut fields = vec![
        ("signed activity", domain.len() + 4),
        ("actor", base),
        ("principal", base + 32),
        ("authority kind", base + 64),
        ("verified key", base + 72),
        ("authority hash", base + 104),
        ("grant", base + 136),
        ("scope selection", base + 168),
        ("activity binding", footer),
        ("payer", footer + 32),
        ("available fee units", footer + 64),
        ("signed fee limit", footer + 80),
        ("fee version", footer + 96),
        ("meter version", footer + 104),
        ("parameter version", footer + 112),
        ("protocol", footer + 248),
        ("batch", footer + 256),
        ("epoch", footer + 264),
    ];
    fields.extend([
        ("scope module mask", scope),
        ("scope ordinal minimum", scope + 8),
        ("scope ordinal maximum", scope + 16),
        ("scope asset", scope + 24),
        ("scope per-activity limit", scope + 56),
        ("scope total limit", scope + 72),
        ("scope spent total", scope + 88),
        ("scope period length", scope + 104),
        ("scope period limit", scope + 112),
        ("scope period spent", scope + 128),
        ("scope period start", scope + 144),
        ("scope purpose", scope + 152),
        ("scope earliest sequence", scope + 184),
        ("scope earliest timestamp", scope + 192),
        ("scope signer threshold", scope + 200),
        ("scope signer count", scope + 208),
        ("scope approval count", approval_count_offset),
    ]);
    for index in 0..signer_count {
        fields.push(("scope signer", scope + 216 + index * 32));
    }
    for index in 0..approval_count {
        fields.push(("scope approval", approval_count_offset + 8 + index * 32));
    }
    for index in 0..9 {
        fields.push(("meter coefficient", footer + 120 + index * 8));
    }
    for index in 0..7 {
        fields.push(("fee price", footer + 192 + index * 8));
    }
    for (name, index) in fields {
        let mut changed = bytes.to_vec();
        changed[index] ^= 1;
        assert!(
            authority.verify_native(&changed).is_err(),
            "uncommitted {name} refused independently of metadata hash"
        );
    }
    for length in [0, domain.len(), bytes.len() - 1] {
        assert!(authority.verify_native(&bytes[..length]).is_err());
    }
    let mut trailing = bytes.to_vec();
    trailing.push(0);
    assert!(authority.verify_native(&trailing).is_err());
}
