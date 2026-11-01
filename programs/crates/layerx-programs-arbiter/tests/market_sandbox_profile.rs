use layerx_client::evidence::{verify_arbiter_admission_v3, VerifiedAdmissionPrestate};
use layerx_program_sdk::arbiter::{
    MarketBillingCommitment, MarketSandboxProfile, ProfileLimits, MARKET_SANDBOX_BILLING_CAPACITY,
    MARKET_SANDBOX_PROFILE_CAPACITY,
};
use layerx_programs_arbiter::{
    ics23::{decode_existence, ExistenceProof},
    market::{MarketAdmissionEvidence, MarketCommittedNamespaces},
    BoundaryProof, ChainCommitProof, MarketEvidenceError, MarketReplayEvidence, MarketStepVerdict,
    NativeNamespaceProof, VerifiedMarketSandbox,
};
use layerx_programs_runtime::execute::{
    market_sandbox_input_digest, observe_market_sandbox_step, MarketSandboxRequest,
};
use layerx_programs_runtime::portable_replay::PortableBoundary;
use layerx_programs_runtime::replay::{market_sandbox_baseline_root, MarketSandboxReplayAuthority};
use layerx_programs_runtime::{
    FeeSchedule, PrincipalId, ProgramId, ProgramReplayProfile, ResourceBudget, Storage, WasmEngine,
    WasmValue, RUNTIME_VERSION,
};
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
use std::{
    collections::BTreeMap,
    fs,
    path::{Path, PathBuf},
};

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
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("proof hex byte"))
        .collect()
}
fn chain_commit(v: &Value) -> ChainCommitProof {
    ChainCommitProof {
        height: v["height"].as_u64().expect("committed block height"),
        app_hash: hash(v, "app_hash"),
        store_root: hash(v, "store_root"),
        multistore_proof: hex(v, "multistore_proof"),
        receipt_key: hex(v, "key"),
        receipt_proof: decode_existence(&hex(v, "proof")).expect("ICS23 receipt key proof"),
    }
}
fn verified(
    v: &Value,
    authorization: &SequencerAuthorization,
) -> (VerifiedReceipt, VerifiedBatchHeader, ChainCommitProof) {
    let commit = chain_commit(&v["chain_commit"]);
    assert!(
        commit
            .receipt_keys()
            .expect("ICS23-authenticated sequencer authority")
            .contains(authorization),
        "capture sequencer authority must be authenticated by the commit app hash"
    );
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
    commit
        .verify_receipt(&verified)
        .expect("receipt signed by app-hash authenticated keys");
    (verified, signed, commit)
}

struct Capture {
    path: String,
    receipt: VerifiedReceipt,
    header: VerifiedBatchHeader,
    commit: ChainCommitProof,
    admission: VerifiedAdmissionPrestate,
}

impl Capture {
    fn admission(&self) -> MarketAdmissionEvidence<'_> {
        MarketAdmissionEvidence {
            prestate: &self.admission,
            header: &self.header,
        }
    }
}

struct Namespace {
    head: ExistenceProof,
    store_root: [u8; 32],
    multistore_proof: Vec<u8>,
    manifest: Vec<u8>,
    values: BTreeMap<[u8; 32], Vec<u8>>,
}

impl Namespace {
    fn proof(&self) -> NativeNamespaceProof<'_> {
        NativeNamespaceProof {
            head: &self.head,
            store_root: self.store_root,
            multistore_proof: &self.multistore_proof,
            manifest: &self.manifest,
            value_blobs: &self.values,
        }
    }
}

struct Fixture {
    directory: PathBuf,
    market: Value,
    captures: Vec<Capture>,
    code: BTreeMap<[u8; 32], Vec<u8>>,
}

impl Fixture {
    fn load() -> Self {
        let path = std::env::var("LAYERX_MARKET_SANDBOX_INPUTS")
            .expect("required genuine signed tenant/provider Market profile/claim corpus is unavailable; no fabricated authority or fallback");
        let directory = Path::new(&path)
            .parent()
            .expect("actual Market corpus directory")
            .to_path_buf();
        let market: Value = serde_json::from_slice(&fs::read(&path).expect("real Market manifest"))
            .expect("real Market JSON");
        let admission_path = directory.join(field(&market, "admission_inputs"));
        let admission: Value = serde_json::from_slice(
            &fs::read(admission_path).expect("genuine signed admission manifest"),
        )
        .expect("actual admission JSON");
        let network = u32::try_from(admission["network_id"].as_u64().expect("actual network"))
            .expect("network width");
        let authorization = SequencerAuthorization::new(
            hash(&admission, "sequencer_id"),
            hash(&admission, "sequencer_public_key"),
            admission["first_batch_number"]
                .as_u64()
                .expect("first real batch"),
            admission["last_batch_number"]
                .as_u64()
                .expect("last real batch"),
        );
        let mut code = BTreeMap::new();
        let captures = admission["captures"]
            .as_array()
            .expect("real signed captures")
            .iter()
            .map(|capture| {
                let (receipt, header, commit) = verified(capture, &authorization);
                let prestate =
                    verify_arbiter_admission_v3(&read(capture, "v3_path"), &receipt, network)
                        .expect("genuine sealed V3 admission");
                if prestate.activity().activity_type().module()
                    == layerx_types::payload::ModuleId::Programs
                    && prestate.activity().activity_type().ordinal() == 1
                {
                    let payload = prestate.activity().payload();
                    assert!(payload.len() >= 108);
                    let length = u32::from_be_bytes(
                        payload[100..104].try_into().expect("signed code length"),
                    ) as usize;
                    let interface = u32::from_be_bytes(
                        payload[104..108]
                            .try_into()
                            .expect("signed interface length"),
                    ) as usize;
                    let start = 108usize.checked_add(interface).expect("bounded interface");
                    assert_eq!(start.checked_add(length), Some(payload.len()));
                    let wasm = payload[start..].to_vec();
                    let digest: [u8; 32] = Sha256::digest(&wasm).into();
                    assert_eq!(&digest, &payload[68..100]);
                    code.insert(digest, wasm);
                }
                Capture {
                    path: field(capture, "receipt_path").to_owned(),
                    receipt,
                    header,
                    commit,
                    admission: prestate,
                }
            })
            .collect();
        Self {
            directory,
            market,
            captures,
            code,
        }
    }

    fn file(&self, object: &Value, key: &str) -> Vec<u8> {
        fs::read(self.directory.join(field(object, key)))
            .expect("required actual Market evidence file")
    }

    fn capture(&self, object: &Value, key: &str) -> &Capture {
        let name = field(object, key);
        self.captures
            .iter()
            .find(|capture| {
                Path::new(&capture.path)
                    .file_name()
                    .and_then(|name| name.to_str())
                    == Some(name)
            })
            .expect("exact genuine signed receipt reference")
    }

    fn namespace(&self, object: &Value) -> Namespace {
        let head = decode_existence(&self.file(object, "head"))
            .expect("actual module9 namespace ICS23 existence proof");
        assert_eq!(head.key.get(..3), Some([1, 0, 9].as_slice()));
        let manifest = self.file(object, "manifest");
        let values = object["value_blobs"]
            .as_array()
            .expect("real content-addressed namespace values")
            .iter()
            .map(|value| {
                let digest = hash(value, "sha256");
                let bytes = self.file(value, "path");
                assert_eq!(<[u8; 32]>::from(Sha256::digest(&bytes)), digest);
                (digest, bytes)
            })
            .collect();
        Namespace {
            head,
            store_root: hash(object, "store_root"),
            multistore_proof: hex(object, "multistore_proof"),
            manifest,
            values,
        }
    }

    fn boundary(&self, object: &Value) -> BoundaryProof {
        BoundaryProof {
            index: u32::try_from(object["index"].as_u64().expect("real boundary index"))
                .expect("index width"),
            leaf: self.file(object, "leaf"),
            siblings: object["siblings"]
                .as_array()
                .expect("actual bounded Merkle path")
                .iter()
                .map(|value| hash(value, "sha256"))
                .collect(),
        }
    }
}

fn committed_market_vector() -> Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../tests/vectors/ics23-market-namespaces-app-hash.json");
    serde_json::from_slice(&fs::read(path).expect("keeper-generated market vector"))
        .expect("market namespace vector JSON")
}

fn committed_namespace(entry: &Value) -> Namespace {
    let cells = entry["cells"].as_array().expect("canonical market cells");
    Namespace {
        head: decode_existence(&hex(entry, "proof")).expect("keeper namespace proof"),
        store_root: hash(entry, "store_root"),
        multistore_proof: hex(entry, "multistore_proof"),
        manifest: hex(entry, "manifest"),
        values: cells
            .iter()
            .map(|cell| (hash(cell, "hash"), hex(cell, "value")))
            .collect(),
    }
}

#[test]
fn market_namespace_verifies_against_ics23_app_hash() {
    let vector = committed_market_vector();
    let program = hash(&vector, "market_program");
    let entries = vector["entries"].as_array().expect("two block snapshots");
    assert_eq!(entries.len(), 2);
    for entry in entries {
        let namespace = committed_namespace(&entry["namespace"]);
        let app_hash = hash(&entry["chain_commit"], "app_hash");
        let expected: BTreeMap<Vec<u8>, Vec<u8>> = entry["namespace"]["cells"]
            .as_array()
            .expect("market records")
            .iter()
            .map(|cell| (hex(cell, "key"), hex(cell, "value")))
            .collect();
        let offer = expected
            .iter()
            .find(|(key, _)| key.starts_with(layerx_programs_market::OFFER_PREFIX))
            .expect("committed offer");
        let lease = expected
            .iter()
            .find(|(key, _)| key.starts_with(layerx_programs_market::LEASE_PREFIX))
            .expect("committed lease");
        let decoded_offer =
            layerx_programs_market::decode_offer(offer.1).expect("canonical market offer");
        let decoded_lease =
            layerx_programs_market::decode_lease(lease.1).expect("canonical funded market lease");
        assert_eq!(
            decoded_offer.id.as_slice(),
            &offer.0[layerx_programs_market::OFFER_PREFIX.len()..]
        );
        assert_eq!(decoded_lease.offer_id, decoded_offer.id);
        assert_eq!(decoded_lease.provider, decoded_offer.provider);
        assert_eq!(namespace.proof().verify(app_hash, program), Ok(expected));
        let mut wrong_hash = app_hash;
        wrong_hash[0] ^= 1;
        assert_eq!(
            namespace.proof().verify(wrong_hash, program),
            Err(MarketEvidenceError::Storage)
        );
        assert_eq!(
            namespace.proof().verify(namespace.store_root, program),
            Err(MarketEvidenceError::Storage)
        );
        let mut wrong_head = namespace.head.clone();
        wrong_head.value[6] ^= 1;
        assert_eq!(
            NativeNamespaceProof {
                head: &wrong_head,
                store_root: namespace.store_root,
                multistore_proof: &namespace.multistore_proof,
                manifest: &namespace.manifest,
                value_blobs: &namespace.values,
            }
            .verify(app_hash, program),
            Err(MarketEvidenceError::Storage)
        );
        let mut wrong_root = namespace.store_root;
        wrong_root[0] ^= 1;
        assert_eq!(
            NativeNamespaceProof {
                head: &namespace.head,
                store_root: wrong_root,
                multistore_proof: &namespace.multistore_proof,
                manifest: &namespace.manifest,
                value_blobs: &namespace.values,
            }
            .verify(app_hash, program),
            Err(MarketEvidenceError::Storage)
        );
        let mut multistore_proof = namespace.multistore_proof.clone();
        multistore_proof[4] ^= 1;
        assert_eq!(
            NativeNamespaceProof {
                head: &namespace.head,
                store_root: namespace.store_root,
                multistore_proof: &multistore_proof,
                manifest: &namespace.manifest,
                value_blobs: &namespace.values,
            }
            .verify(app_hash, program),
            Err(MarketEvidenceError::Storage)
        );
        let mut wrong_program = program;
        wrong_program[0] ^= 1;
        assert_eq!(
            namespace.proof().verify(app_hash, wrong_program),
            Err(MarketEvidenceError::Storage)
        );
        let mut wrong_module = namespace.head.clone();
        wrong_module.key[2] = 8;
        assert_eq!(
            NativeNamespaceProof {
                head: &wrong_module,
                store_root: namespace.store_root,
                multistore_proof: &namespace.multistore_proof,
                manifest: &namespace.manifest,
                value_blobs: &namespace.values,
            }
            .verify(app_hash, program),
            Err(MarketEvidenceError::Storage)
        );
        let mut manifest = namespace.manifest.clone();
        manifest[0] ^= 1;
        assert_eq!(
            NativeNamespaceProof {
                head: &namespace.head,
                store_root: namespace.store_root,
                multistore_proof: &namespace.multistore_proof,
                manifest: &manifest,
                value_blobs: &namespace.values,
            }
            .verify(app_hash, program),
            Err(MarketEvidenceError::Storage)
        );
        let mut values = namespace.values.clone();
        values.values_mut().next().expect("nonempty cell")[0] ^= 1;
        assert_eq!(
            NativeNamespaceProof {
                head: &namespace.head,
                store_root: namespace.store_root,
                multistore_proof: &namespace.multistore_proof,
                manifest: &namespace.manifest,
                value_blobs: &values,
            }
            .verify(app_hash, program),
            Err(MarketEvidenceError::Storage)
        );
    }
}

#[test]
fn market_evidence_refuses_mixed_app_hashes() {
    let vector = committed_market_vector();
    let program = hash(&vector, "market_program");
    let entries = vector["entries"]
        .as_array()
        .expect("two committed snapshots");
    assert_eq!(entries.len(), 2);
    let tenant_commit = chain_commit(&entries[0]["chain_commit"]);
    let provider_commit = chain_commit(&entries[1]["chain_commit"]);
    let tenant_state = committed_namespace(&entries[0]["namespace"]);
    let provider_state = committed_namespace(&entries[1]["namespace"]);
    let tenant_receipt = hex(&entries[0]["receipt"], "value");
    let provider_receipt = hex(&entries[1]["receipt"], "value");
    assert_ne!(tenant_receipt, provider_receipt);
    assert_ne!(tenant_commit.app_hash, provider_commit.app_hash);
    assert_eq!(provider_commit.height, tenant_commit.height + 1);
    for (entry, commit) in entries.iter().zip([&tenant_commit, &provider_commit]) {
        let receipt = &entry["receipt"];
        let proof = decode_existence(&hex(receipt, "proof")).expect("committed receipt existence");
        let root = hash(receipt, "store_root");
        let value = hex(receipt, "value");
        assert_eq!(root, hash(&entry["namespace"], "store_root"));
        assert_eq!(hash(receipt, "app_hash"), commit.app_hash);
        assert_eq!(
            hex(receipt, "multistore_proof"),
            hex(&entry["namespace"], "multistore_proof")
        );
        layerx_programs_arbiter::ics23::verify_existence(
            &proof,
            root,
            &hex(receipt, "key"),
            &value,
        )
        .expect("actual signed receipt committed in the keeper store");
        commit
            .verify_receipt_bytes(&value)
            .expect("signature under app-hash authenticated key table");
        let mut altered = value;
        let last = altered.len() - 1;
        altered[last] ^= 1;
        assert!(commit.verify_receipt_bytes(&altered).is_err());
    }
    let evaluate =
        |tenant: &Namespace, provider: &Namespace, tc: &ChainCommitProof, pc: &ChainCommitProof| {
            MarketCommittedNamespaces {
                program,
                tenant_receipt: &tenant_receipt,
                provider_receipt: &provider_receipt,
                tenant_commit: tc,
                provider_commit: pc,
                tenant_state: tenant.proof(),
                provider_state: provider.proof(),
            }
            .verify()
        };
    let authenticated = evaluate(
        &tenant_state,
        &provider_state,
        &tenant_commit,
        &provider_commit,
    )
    .expect("both receipts and namespaces authenticated together");
    assert_ne!(authenticated.tenant, authenticated.provider);
    for (tenant, provider) in [
        (&provider_state, &provider_state),
        (&tenant_state, &tenant_state),
        (&provider_state, &tenant_state),
    ] {
        assert!(matches!(
            evaluate(tenant, provider, &tenant_commit, &provider_commit),
            Err(MarketStepVerdict::InvalidEvidence(
                MarketEvidenceError::Storage
            ))
        ));
    }
    let mut wrong_tenant_commit = tenant_commit.clone();
    wrong_tenant_commit.app_hash[0] ^= 1;
    assert!(matches!(
        evaluate(
            &tenant_state,
            &provider_state,
            &wrong_tenant_commit,
            &provider_commit
        ),
        Err(MarketStepVerdict::InvalidEvidence(
            MarketEvidenceError::Admission
        ))
    ));
    let mut wrong_provider_commit = provider_commit.clone();
    wrong_provider_commit.app_hash[0] ^= 1;
    assert!(matches!(
        evaluate(
            &tenant_state,
            &provider_state,
            &tenant_commit,
            &wrong_provider_commit
        ),
        Err(MarketStepVerdict::InvalidEvidence(
            MarketEvidenceError::Admission
        ))
    ));
}

#[test]
fn genuine_signed_market_profile_claim_steps_and_cryptographic_refusals() {
    let fixture = Fixture::load();
    let cases = fixture.market["cases"]
        .as_array()
        .expect("actual Market profile/claim corpus cases");
    let mut correct = 0;
    let mut incorrect = 0;
    let mut traps = 0;
    for case in cases {
        let tenant = fixture.capture(case, "tenant_receipt");
        let provider = fixture.capture(case, "provider_receipt");
        let tenant_state = fixture.namespace(&case["tenant_namespace"]);
        let provider_state = fixture.namespace(&case["provider_namespace"]);
        let baseline = fixture.file(case, "baseline_storage");
        let baseline_record = case
            .get("baseline_namespace")
            .expect("explicit authenticated baseline namespace or proved absence required");
        let baseline_namespace = if baseline_record.is_null() {
            None
        } else {
            Some(fixture.namespace(baseline_record))
        };
        let input = fixture.file(case, "input");
        let pre = fixture.boundary(&case["pre"]);
        let post = fixture.boundary(&case["post"]);
        let initial = fixture.boundary(&case["initial"]);
        let final_boundary = fixture.boundary(&case["final"]);
        let evidence = |tenant_namespace: &Namespace,
                        provider_namespace: &Namespace,
                        code: &BTreeMap<[u8; 32], Vec<u8>>,
                        input: &[u8]| {
            VerifiedMarketSandbox::verify(MarketReplayEvidence {
                expected_market_program: hash(&fixture.market, "market_program"),
                tenant_receipt: &tenant.receipt,
                tenant_admission: tenant.admission(),
                tenant_commit: &tenant.commit,
                provider_receipt: &provider.receipt,
                provider_admission: provider.admission(),
                provider_commit: &provider.commit,
                initial_boundary: &initial,
                final_boundary: &final_boundary,
                tenant_state: tenant_namespace.proof(),
                provider_state: provider_namespace.proof(),
                code_blobs: code,
                baseline_storage_bytes: &baseline,
                input_bytes: input,
                baseline_state: baseline_namespace.as_ref().map(Namespace::proof),
            })
        };
        let sealed = evidence(&tenant_state, &provider_state, &fixture.code, &input)
            .expect("actual signed tenant authorization and provider billing rooted in genuine module9 state");
        match (field(case, "expected"), sealed.verify_step(&pre, &post)) {
            ("correct", MarketStepVerdict::Correct) => correct += 1,
            ("incorrect", MarketStepVerdict::Incorrect) => incorrect += 1,
            (expected, observed) => {
                panic!("actual proof-backed transition expected {expected}, observed {observed:?}")
            }
        }
        if field(case, "kind") == "trap" {
            traps += 1;
        }
        let mut invalid = pre.clone();
        invalid
            .leaf
            .first_mut()
            .expect("actual nonempty boundary")
            .clone_from(&0xff);
        if invalid.leaf == pre.leaf {
            invalid.leaf[0] ^= 1;
        }
        assert!(matches!(
            sealed.verify_step(&invalid, &post),
            MarketStepVerdict::InvalidEvidence(_)
        ));
        let mut bad_tenant = Namespace {
            head: tenant_state.head.clone(),
            store_root: tenant_state.store_root,
            multistore_proof: tenant_state.multistore_proof.clone(),
            manifest: tenant_state.manifest.clone(),
            values: tenant_state.values.clone(),
        };
        bad_tenant.head.value[0] ^= 1;
        assert!(evidence(&bad_tenant, &provider_state, &fixture.code, &input).is_err());
        let mut bad_provider = Namespace {
            head: provider_state.head.clone(),
            store_root: provider_state.store_root,
            multistore_proof: provider_state.multistore_proof.clone(),
            manifest: provider_state.manifest.clone(),
            values: provider_state.values.clone(),
        };
        bad_provider.manifest[0] ^= 1;
        assert!(evidence(&tenant_state, &bad_provider, &fixture.code, &input).is_err());
        let mut bad_input = input.clone();
        if bad_input.is_empty() {
            bad_input.push(1);
        } else {
            bad_input[0] ^= 1;
        }
        assert!(evidence(&tenant_state, &provider_state, &fixture.code, &bad_input).is_err());
        assert!(evidence(&tenant_state, &provider_state, &BTreeMap::new(), &input).is_err());
        let swapped = VerifiedMarketSandbox::verify(MarketReplayEvidence {
            expected_market_program: hash(&fixture.market, "market_program"),
            tenant_receipt: &provider.receipt,
            tenant_admission: provider.admission(),
            tenant_commit: &provider.commit,
            provider_receipt: &tenant.receipt,
            provider_admission: tenant.admission(),
            provider_commit: &tenant.commit,
            initial_boundary: &initial,
            final_boundary: &final_boundary,
            tenant_state: tenant_state.proof(),
            provider_state: provider_state.proof(),
            code_blobs: &fixture.code,
            baseline_storage_bytes: &baseline,
            input_bytes: &input,
            baseline_state: baseline_namespace.as_ref().map(Namespace::proof),
        });
        assert!(swapped.is_err());
    }
    assert!(
        correct >= 2,
        "real integer and trap transition positives required"
    );
    assert!(
        incorrect >= 1,
        "real committed incorrect provider transition required"
    );
    assert!(traps >= 1, "actual signed trap profile required");
}

#[test]
fn raw_market_engine_integer_trap_and_sdk_profile_codecs() {
    use layerx_programs_runtime::execute::instantiate_market_sandbox_untrusted;
    use layerx_programs_runtime::replay::market_sandbox_namespace;
    use layerx_programs_runtime::test_support::{
        code_section, export_section, func_body, function_section, module, type_section, OP_END,
        OP_I32_CONST, TYPE_I32,
    };
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
        let engine = WasmEngine::declared().expect("actual declared engine");
        let validated = engine
            .validate_versioned(2, &wasm)
            .expect("actual integer-only guest");
        let program = ProgramId::new(Sha256::digest(&wasm).into())
            .expect("actual code-derived sandbox identity");
        let tenant = PrincipalId::new(Sha256::digest(b"raw-engine-tenant").into())
            .expect("raw execution principal");
        let lease: [u8; 32] = Sha256::digest(b"raw-engine-lease").into();
        let baseline = Storage::new();
        let budget = ResourceBudget::new_complete(100_000, 65_536, 1024, 1024, 1, 64, 0);
        let fees = FeeSchedule::declared();
        let authority = MarketSandboxReplayAuthority {
            profile_binding: Sha256::digest(b"raw-engine-profile").into(),
            namespace: market_sandbox_namespace(program, lease)
                .expect("actual namespace derivation"),
            lease_id: lease,
            namespace_limit: 1024,
            program,
            tenant,
            payment_account: tenant.bytes(),
            code_hash: validated.code_hash(),
            input_digest: market_sandbox_input_digest("compute", &[])
                .expect("real primitive input digest"),
            runtime_version: RUNTIME_VERSION,
            abi_version: 2,
            fee_schedule_version: fees.version(),
            metering_schedule_version: validated.metering_schedule_version(),
            budget,
            fees,
            fee_budget: 1_000_000_000,
            baseline_state_root: market_sandbox_baseline_root(&baseline)
                .expect("actual storage commitment"),
            baseline_storage: baseline,
        };
        let mut instance = instantiate_market_sandbox_untrusted(&validated, &authority)
            .expect("actual isolated Market sandbox instance");
        let execution = instance
            .call_market_sandbox_untrusted(MarketSandboxRequest {
                module: &validated,
                entrypoint: "compute",
                args: &[],
                authority: &authority,
                replay_profile: ProgramReplayProfile::new(128, 1_048_576)
                    .expect("bounded real profile"),
            })
            .expect("actual captured integer/trap execution");
        assert_eq!(execution.profile_binding, authority.profile_binding);
        assert!(execution.usage.cpu_fuel > 0);
        assert!(execution.final_namespace_bytes <= authority.namespace_limit);
        assert_eq!(
            execution.record.boundary_count() as usize,
            execution.boundary_leaves.len()
        );
        assert_ne!(execution.record.boundary_root(), [0; 32]);
        if trapped {
            assert!(execution.terminal_fault.is_some());
            assert_eq!(execution.record.terminal_status(), 1);
            assert!(execution.values.is_empty());
        } else {
            assert!(execution.terminal_fault.is_none());
            assert_eq!(execution.record.terminal_status(), 0);
            assert_eq!(execution.values, vec![WasmValue::I32(12)]);
        }
        let boundaries: Vec<PortableBoundary> = execution
            .boundary_leaves
            .iter()
            .map(|leaf| {
                PortableBoundary::decode_untrusted(leaf, 1_048_576)
                    .expect("actual captured portable boundary")
            })
            .collect();
        let mut observed_instructions = 0;
        for pair in boundaries.windows(2) {
            if let Ok(observed) =
                observe_market_sandbox_step(&validated, &pair[0], &authority, 1_048_576)
            {
                if observed.replay == pair[1].replay
                    && observed.semantic_bytes == pair[1].semantic_bytes
                {
                    observed_instructions += 1;
                }
            }
        }
        if trapped {
            let terminal = boundaries.last().expect("actual terminal trap leaf");
            let observed = observe_market_sandbox_step(&validated, terminal, &authority, 1_048_576)
                .expect("actual single trap instruction replay");
            assert!(observed.trap.is_some());
        } else {
            assert!(
                observed_instructions > 0,
                "actual Engine instruction replay required"
            );
        }
        let mut wrong_authority = authority.clone();
        wrong_authority.code_hash[0] ^= 1;
        assert!(observe_market_sandbox_step(
            &validated,
            &boundaries[0],
            &wrong_authority,
            1_048_576
        )
        .is_err());
        wrong_authority = authority.clone();
        wrong_authority.namespace[0] ^= 1;
        assert!(instantiate_market_sandbox_untrusted(&validated, &wrong_authority).is_err());
        wrong_authority = authority.clone();
        wrong_authority.input_digest[0] ^= 1;
        assert!(instance
            .call_market_sandbox_untrusted(MarketSandboxRequest {
                module: &validated,
                entrypoint: "compute",
                args: &[],
                authority: &wrong_authority,
                replay_profile: ProgramReplayProfile::new(128, 1_048_576)
                    .expect("unchanged real bounds"),
            })
            .is_err());
        let mut entrypoint = [0; 64];
        entrypoint[..7].copy_from_slice(b"compute");
        let profile = MarketSandboxProfile {
            network_id: 7,
            market_program: Sha256::digest(b"raw-codec-market").into(),
            sandbox_program: program.bytes(),
            offer_id: Sha256::digest(b"raw-codec-offer").into(),
            lease_id: lease,
            claim_id: Sha256::digest(b"raw-codec-claim").into(),
            provider: Sha256::digest(b"raw-codec-provider").into(),
            tenant: tenant.bytes(),
            code_hash: authority.code_hash,
            input_digest: authority.input_digest,
            attested_input_commitment: Sha256::digest(b"raw-codec-input-commitment").into(),
            namespace: authority.namespace,
            baseline_state_root: authority.baseline_state_root,
            initial_execution_state_root: execution.initial_commitment.digest,
            entrypoint_bytes: entrypoint,
            entrypoint_length: 7,
            runtime_version: RUNTIME_VERSION,
            abi_version: 2,
            fee_schedule_version: authority.fee_schedule_version,
            metering_schedule_version: authority.metering_schedule_version,
            limits: ProfileLimits {
                cpu_fuel: budget.cpu_fuel(),
                memory_bytes: budget.memory_bytes(),
                storage_read_bytes: budget.storage_read_bytes(),
                storage_write_bytes: budget.storage_write_bytes(),
                output_values: u64::from(budget.output_values()),
                output_bytes: budget.output_bytes(),
                table_elements: u64::from(budget.table_elements()),
                namespace_bytes: authority.namespace_limit,
            },
            fee_budget: authority.fee_budget,
            interval_start: 1,
            interval_end: 2,
            response_deadline: 3,
            maximum_boundaries: 128,
            maximum_bytes: 1_048_576,
        };
        let mut profile_bytes = [0; MARKET_SANDBOX_PROFILE_CAPACITY];
        let profile_length = profile
            .encode(&mut profile_bytes)
            .expect("actual SDK codec accepts actual runtime commitments");
        assert_eq!(
            MarketSandboxProfile::decode(&profile_bytes[..profile_length])
                .expect("canonical SDK decode"),
            profile
        );
        assert!(MarketSandboxProfile::decode(&profile_bytes[..profile_length - 1]).is_err());
        let mut trailing = profile_bytes[..profile_length].to_vec();
        trailing.push(0);
        assert!(MarketSandboxProfile::decode(&trailing).is_err());
        let mut invalid_profile = profile;
        invalid_profile.maximum_boundaries = 1;
        assert!(invalid_profile.encode(&mut profile_bytes).is_err());
        let usage = execution.usage;
        let output_bytes = 12i32.to_be_bytes();
        let billing = MarketBillingCommitment {
            profile_digest: Sha256::digest(&profile_bytes[..profile_length]).into(),
            provider_trace_root: execution.record.boundary_root(),
            final_execution_state_root: execution.final_commitment.digest,
            output_digest: Sha256::digest(if trapped {
                b"trap".as_slice()
            } else {
                &output_bytes
            })
            .into(),
            usage: [
                usage.cpu_fuel,
                usage.memory_bytes,
                usage.storage_read_bytes,
                usage.storage_write_bytes,
                u64::from(usage.output_values),
                usage.output_bytes,
            ],
            payable: usage.fee_units,
            challenger_stake: 1,
            challenge_window_batches: 1,
            boundary_count: execution.record.boundary_count(),
        };
        let mut billing_bytes = [0; MARKET_SANDBOX_BILLING_CAPACITY];
        let length = billing
            .encode(&mut billing_bytes)
            .expect("actual SDK billing over runtime values");
        assert_eq!(
            MarketBillingCommitment::decode(&billing_bytes[..length]).expect("canonical billing"),
            billing
        );
        assert!(MarketBillingCommitment::decode(&billing_bytes[..length - 1]).is_err());
        billing_bytes[0] ^= 1;
        assert!(MarketBillingCommitment::decode(&billing_bytes[..length]).is_err());
    }
}
