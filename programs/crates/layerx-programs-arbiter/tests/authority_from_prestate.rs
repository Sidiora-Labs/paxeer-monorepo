use std::{collections::BTreeMap, fs, path::Path};

use layerx_client::evidence::{verify_arbiter_admission_v3, VerifiedAdmissionPrestate};
use layerx_programs_arbiter::{
    ics23::{decode_existence, ExistenceProof},
    AuthenticatedCatalogue, BoundaryProof, ChainCommitProof, ReplayError, VerifiedReplay,
    VerifiedReplayAuthority,
};
use layerx_programs_runtime::portable_replay::PortableReplayRecord;
use layerx_proof::{
    inclusion::{verify_header, verify_receipt, SequencerAuthorization, VerifiedBatchHeader},
    merkle::decode_proof,
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
fn read(v: &Value, key: &str) -> Vec<u8> {
    fs::read(field(v, key)).expect("genuine native file")
}
fn hash(v: &Value, key: &str) -> [u8; 32] {
    let s = field(v, key);
    assert_eq!(s.len(), 64);
    std::array::from_fn(|i| {
        u8::from_str_radix(&s[2 * i..2 * i + 2], 16).expect("native pinned hash")
    })
}
fn hex(v: &Value, key: &str) -> Vec<u8> {
    let text = field(v, key);
    assert_eq!(text.len() % 2, 0, "complete fixture hex bytes");
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("fixture hex byte"))
        .collect()
}

fn committed_vector() -> Value {
    serde_json::from_str(include_str!(
        "../../../tests/vectors/ics23-receipt-keys-app-hash.json"
    ))
    .expect("committed receipt-key list and multistore proof vector")
}

fn chain_commit(entry: &Value) -> ChainCommitProof {
    let commit = ChainCommitProof {
        height: entry["height"].as_u64().expect("committed block height"),
        app_hash: hash(entry, "app_hash"),
        store_root: hash(entry, "store_root"),
        multistore_proof: hex(entry, "multistore_proof"),
        receipt_key: hex(entry, "key"),
        receipt_proof: decode_existence(&hex(entry, "proof"))
            .expect("host receipt-key list existence proof"),
    };
    assert_eq!(commit.receipt_proof.value, hex(entry, "value"));
    commit
}

#[test]
fn chain_commit_proof_verifies_pinned_receipt_keys_under_app_hash() {
    let vector = committed_vector();
    let entries = vector["entries"].as_array().expect("host entries");
    assert!(!entries.is_empty(), "committed receipt-key list proofs");
    for entry in entries {
        let commit = chain_commit(entry);
        let keys = commit
            .receipt_keys()
            .expect("app-hash authenticated receipt keys");
        assert_eq!(keys.len(), 2, "complete pinned host key list");
        let value = hex(entry, "value");
        for (key, encoded) in keys.iter().zip(value.chunks_exact(80)) {
            assert_eq!(key.sequencer_id().as_slice(), &encoded[..32]);
            assert_eq!(key.public_key().as_slice(), &encoded[32..64]);
            assert_eq!(key.first_batch_number().to_be_bytes(), encoded[64..72]);
            assert_eq!(key.last_batch_number().to_be_bytes(), encoded[72..80]);
        }
        let bytes = hex(entry, "receipt");
        let receipt = layerx_proof::receipt::verify_sequencer_signature(&bytes, keys.as_slice())
            .expect("receipt signature under the ICS23-proved pinned key list");
        let protocol = receipt.protocol().expect("host protocol receipt");
        let mut batch_id = [0; 32];
        batch_id[24..].copy_from_slice(&commit.height.to_be_bytes());
        assert_eq!(
            protocol.batch_id(),
            batch_id,
            "receipt's committed block height"
        );
        let covering: Vec<_> = keys
            .iter()
            .filter(|key| {
                key.sequencer_id() == protocol.context_hash()
                    && key.first_batch_number() <= commit.height
                    && commit.height <= key.last_batch_number()
            })
            .collect();
        assert_eq!(
            covering.len(),
            1,
            "unique pinned signer covering the receipt height"
        );
        assert_eq!(covering[0].sequencer_id(), [0xa1; 32]);
        let public = "d04ab232742bb4ab3a1368bd4615e4e6d0224ab71a016baf8520a332c9778737";
        let expected: [u8; 32] = std::array::from_fn(|i| {
            u8::from_str_radix(&public[2 * i..2 * i + 2], 16).expect("pinned public key")
        });
        assert_eq!(covering[0].public_key(), expected);
        commit
            .verify_receipt_bytes(&bytes)
            .expect("committed host receipt");
    }
}

#[test]
fn authority_refuses_receipt_proof_for_another_app_hash() {
    let vector = committed_vector();
    let entries = vector["entries"].as_array().expect("host entries");
    assert!(!entries.is_empty(), "committed receipt-key list proofs");
    for entry in entries {
        let commit = chain_commit(entry);
        commit.receipt_keys().expect("valid host receipt-key proof");
        let mut wrong_app_hash = commit.clone();
        wrong_app_hash.app_hash[0] ^= 1;
        assert!(matches!(
            wrong_app_hash.receipt_keys(),
            Err(ReplayError::Receipt)
        ));
        let mut wrong_value = commit.clone();
        wrong_value.receipt_proof.value[0] ^= 1;
        assert!(matches!(
            wrong_value.receipt_keys(),
            Err(ReplayError::Receipt)
        ));
        let mut wrong_key = commit.clone();
        wrong_key.receipt_key[0] ^= 1;
        assert!(matches!(
            wrong_key.receipt_keys(),
            Err(ReplayError::Receipt)
        ));
        let mut wrong_proof = commit.clone();
        wrong_proof.receipt_proof.leaf.prefix[0] ^= 1;
        assert!(matches!(
            wrong_proof.receipt_keys(),
            Err(ReplayError::Receipt)
        ));
        let bytes = hex(entry, "receipt");
        let mut wrong_signature = bytes.clone();
        *wrong_signature.last_mut().expect("receipt signature") ^= 1;
        assert!(matches!(
            commit.verify_receipt_bytes(&wrong_signature),
            Err(ReplayError::Receipt)
        ));
        let decoded = layerx_wire::receipt::decode(&bytes).expect("canonical host receipt");
        let batch_id = decoded.protocol().expect("protocol receipt").batch_id();
        let offsets: Vec<_> = bytes
            .windows(32)
            .enumerate()
            .filter_map(|(i, window)| (window == batch_id).then_some(i))
            .collect();
        assert_eq!(offsets.len(), 1, "unique encoded receipt height");
        let mut uncovered_height = bytes.clone();
        uncovered_height[offsets[0] + 24..offsets[0] + 32].copy_from_slice(&1_u64.to_be_bytes());
        assert!(matches!(
            commit.verify_receipt_bytes(&uncovered_height),
            Err(ReplayError::Receipt)
        ));
    }
}

fn verified(
    v: &Value,
    authorization: &SequencerAuthorization,
) -> (VerifiedReceipt, VerifiedBatchHeader) {
    let bytes = read(v, "receipt_path");
    let header = read(v, "header_path");
    let signature: [u8; 64] = read(v, "header_signature_path")
        .try_into()
        .expect("actual header signature");
    let signed =
        verify_header(&header, &signature, authorization).expect("genuine sequencer header");
    let proof = decode_proof(&read(v, "proof_path")).expect("actual receipt Merkle proof");
    verify_receipt(&bytes, &proof, &header, &signature, authorization)
        .expect("actual signed receipt inclusion");
    let maintenance = read(v, "maintenance_path");
    let maintenance_proof =
        decode_proof(&read(v, "maintenance_proof_path")).expect("actual maintenance proof");
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
        .map(|p| fs::read(p.as_str().expect("receipt path")).expect("real receipt chain entry"))
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
    (verified, signed)
}

struct Capture {
    path: String,
    receipt: VerifiedReceipt,
    header: VerifiedBatchHeader,
    receipt_proof: ChainCommitProof,
    admission: VerifiedAdmissionPrestate,
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
            &self.receipt_proof,
            self.host_store_root,
            &self.host_multistore_proof,
            &self.code_proofs,
            code,
        )
    }
}
struct Fixture {
    directory: std::path::PathBuf,
    replay: Value,
    captures: Vec<Capture>,
    code: BTreeMap<[u8; 32], Vec<u8>>,
}
impl Fixture {
    fn load() -> Self {
        let path = std::env::var("LAYERX_AUTHENTICATED_REPLAY_INPUTS")
            .expect("required genuine native replay corpus; no fallback");
        let directory = Path::new(&path)
            .parent()
            .expect("native corpus parent")
            .to_path_buf();
        let replay: Value =
            serde_json::from_slice(&fs::read(path).expect("native replay manifest"))
                .expect("native replay JSON");
        let inputs = std::env::var("LAYERX_ARBITER_ADMISSION_INPUTS")
            .expect("required real signed admission corpus");
        let manifest: Value =
            serde_json::from_slice(&fs::read(inputs).expect("native admission manifest"))
                .expect("native admission JSON");
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
                let (receipt, header) = verified(v, &authorization);
                let receipt_proof = chain_commit(&v["chain_commit"]);
                receipt_proof
                    .verify_receipt(&receipt)
                    .expect("genuine app-hash authenticated receipt keys");
                let admission = verify_arbiter_admission_v3(&read(v, "v3_path"), &receipt, network)
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
                let mut code_proofs = BTreeMap::new();
                for entry in v["code_proofs"]
                    .as_array()
                    .expect("host module 9 code-blob proofs")
                {
                    let proof = decode_existence(&read(entry, "proof_path"))
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
                    receipt_proof,
                    admission,
                    host_store_root: hash(v, "host_store_root"),
                    host_multistore_proof: read(v, "host_multistore_proof_path"),
                    code_proofs,
                }
            })
            .collect();
        Self {
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
        fs::read(self.directory.join(field(v, key))).expect("real replay evidence file")
    }
}

#[test]
fn canonical_native_authority_matches_authenticated_historical_facts() {
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
            &trusted.receipt_proof,
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
            &trusted.receipt_proof,
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
            &trusted.receipt_proof,
        )
        .is_err());
        let mut bad_proof = trusted.receipt_proof.clone();
        bad_proof.receipt_proof.leaf.prefix[0] ^= 1;
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
            &trusted.receipt_proof,
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
            &trusted.receipt_proof,
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
            &trusted.receipt_proof,
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
            &trusted.receipt_proof,
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
            &trusted.receipt_proof,
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
        assert!(AuthenticatedCatalogue::verify(&trusted.admission, &bad_code).is_err());
        assert!(AuthenticatedCatalogue::verify(&trusted.admission, &BTreeMap::new()).is_err());
    }
    assert!(verified_instructions >= 4);
    assert_eq!(verified_traps, 1);
    assert_eq!(verified_scheduled, 1);
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
