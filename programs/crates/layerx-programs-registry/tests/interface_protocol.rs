use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

#[path = "support/chain.rs"]
mod chain;

use chain::ChainFixture;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_programs::chain_evidence::{
    deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
    program_state::ChainProgramStateProof, programs_state_key, wind_down::ChainWindDownProof,
};
use layerx_programs::chain_proof::{
    decode::{decode_commitment_proof, ProofOp},
    merkle::{hash_from_byte_slices, SimpleProof},
    validators::TrustedValidator,
    vote::vote_sign_bytes,
};
use layerx_programs::{
    interface_state_key, interface_state_value, DeploymentProof, InterfaceCapability,
    InterfaceEntryPoint, InterfaceRefusal, InterfaceStateWitness, ProgramInterface,
    ProgramLifecycleProof, ProgramStateProof, ProtocolDeploymentVerifier, StateProof, ValueSchema,
    ValueType, VerifiedInterfaceRead,
};
use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, import_section, module, raw_section, type_section,
    unsigned_leb, OP_CALL, OP_DROP, OP_END, OP_I32_CONST, TYPE_I32,
};
use layerx_programs_runtime::{ProgramId, WasmEngine};
use sha2::{Digest, Sha256};

const CASES: [(&str, u16, bool); 7] = [
    ("abi1", 1, false),
    ("abi2", 2, false),
    ("abi2-dynamic", 2, true),
    ("abi3", 3, false),
    ("abi3-dynamic", 3, true),
    ("abi4", 4, false),
    ("abi4-dynamic", 4, true),
];
const DOMAIN_LEN: usize = b"LayerX/program-interface/v1\0".len();

fn capabilities(abi: u16, dynamic: bool) -> Vec<InterfaceCapability> {
    let mut result = Vec::new();
    if dynamic {
        result.push(InterfaceCapability::CallerAuthorizedSpend {
            asset: [2; 32],
            maximum_amount: 100,
            recipient_offset: 4,
            amount_offset: 36,
        });
    }
    if abi >= 3 {
        result.push(InterfaceCapability::OracleRead);
    }
    if abi >= 4 {
        result.push(InterfaceCapability::WebRead);
    }
    result
}

fn entry(abi: u16, dynamic: bool, input: u32, output: u32) -> InterfaceEntryPoint {
    InterfaceEntryPoint {
        name: "call".to_owned(),
        discriminator: [0x10, 0x20, 0x30, 0x40],
        calldata: ValueSchema::layerx(ValueType::Bytes { max_len: input }),
        response: ValueSchema::layerx(ValueType::Bytes { max_len: output }),
        capabilities: capabilities(abi, dynamic),
        event_topics: Vec::new(),
        failures: Vec::new(),
    }
}

fn wasm(abi: u16, dynamic: bool) -> Vec<u8> {
    let mut imports = Vec::new();
    if dynamic {
        imports.push(("layerx_v2", "transfer_program_402", 3));
    }
    if abi >= 3 {
        imports.push(("layerx_v3", "oracle_read", 0));
    }
    if abi >= 4 {
        imports.push(("layerx_v4", "web_read", 0));
    }
    let n = imports.len() as u32;
    let mut exports = unsigned_leb(3);
    for (name, kind, index) in [
        ("layerx_reserve", 0_u8, n),
        ("call", 0, n + 1),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.push(kind);
        exports.extend(unsigned_leb(u64::from(index)));
    }
    let mut body = Vec::new();
    for (index, (_, _, ty)) in imports.iter().enumerate() {
        if *ty == 3 {
            body.extend([0x42, 0, 0x42, 0]);
            for _ in 0..8 {
                body.extend([OP_I32_CONST, 0]);
            }
        } else {
            for _ in 0..4 {
                body.extend([OP_I32_CONST, 0]);
            }
        }
        body.push(OP_CALL);
        body.extend(unsigned_leb(index as u64));
        body.push(OP_DROP);
    }
    body.extend([OP_I32_CONST, 0, OP_END]);
    module(&[
        type_section(&[
            (&[TYPE_I32; 4], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
            (
                &[
                    0x7e, 0x7e, TYPE_I32, TYPE_I32, TYPE_I32, TYPE_I32, TYPE_I32, TYPE_I32,
                    TYPE_I32, TYPE_I32,
                ],
                &[TYPE_I32],
            ),
        ]),
        import_section(&imports),
        function_section(&[1, 2]),
        raw_section(5, &[1, 1, 1, 1]),
        raw_section(7, &exports),
        code_section(&[
            func_body(&[], &[OP_I32_CONST, 0, OP_END]),
            func_body(&[], &body),
        ]),
    ])
}

fn bound(abi: u16, dynamic: bool, input: u32, output: u32) -> ProgramInterface {
    ProgramInterface::bind(
        &wasm(abi, dynamic),
        abi,
        vec![entry(abi, dynamic, input, output)],
    )
    .unwrap_or_else(|error| panic!("bind ABI {abi} dynamic {dynamic}: {error}"))
}

fn directory(variable: &str) -> PathBuf {
    PathBuf::from(std::env::var_os(variable).unwrap_or_else(|| panic!("missing {variable}")))
}

#[test]
fn emit_native_inputs() {
    let root = directory("PAXEER_X_INTERFACE_INPUTS");
    for (name, abi, dynamic) in CASES {
        let path = root.join(name);
        fs::create_dir_all(&path).expect("create case inputs");
        fs::write(path.join("module.wasm"), wasm(abi, dynamic)).expect("write actual module");
        fs::write(
            path.join("interface.bin"),
            bound(abi, dynamic, 64, 64).canonical_encoding(),
        )
        .expect("write canonical interface");
    }
    for (name, input, output) in [("abi2-widening", 128, 32), ("abi2-narrowing", 32, 128)] {
        let path = root.join(name);
        fs::create_dir_all(&path).expect("create upgrade input");
        fs::write(path.join("module.wasm"), wasm(2, false)).expect("write upgrade module");
        fs::write(
            path.join("interface.bin"),
            bound(2, false, input, output).canonical_encoding(),
        )
        .expect("write upgrade interface");
    }
}

fn unhex(text: &str) -> Vec<u8> {
    assert_eq!(text.len() % 2, 0, "odd hex length");
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let digit = |c: u8| match c {
                b'0'..=b'9' => c - b'0',
                b'a'..=b'f' => c - b'a' + 10,
                _ => panic!("noncanonical hex"),
            };
            (digit(pair[0]) << 4) | digit(pair[1])
        })
        .collect()
}

#[test]
fn canonical_abi_domains_and_legacy_vectors() {
    for (_, abi, dynamic) in CASES {
        let interface = bound(abi, dynamic, 64, 64);
        let version = if abi >= 3 {
            abi
        } else if dynamic {
            2
        } else {
            1
        };
        let domain: &[u8] = match version {
            1 => b"LayerX/program-interface/v1\0",
            2 => b"LayerX/program-interface/v2\0",
            3 => b"LayerX/program-interface/v3\0",
            4 => b"LayerX/program-interface/v4\0",
            _ => unreachable!(),
        };
        assert!(interface.canonical_encoding().starts_with(domain));
        let mut expected = domain.to_vec();
        expected.extend_from_slice(&Sha256::digest(wasm(abi, dynamic)));
        expected.extend_from_slice(&abi.to_be_bytes());
        expected.extend(unhex("0001000463616c6c10203040012000000040012000000040"));
        expected.extend_from_slice(&(capabilities(abi, dynamic).len() as u16).to_be_bytes());
        if dynamic {
            expected.push(10);
            expected.extend([2; 32]);
            expected.extend_from_slice(&100_u128.to_be_bytes());
            expected.extend_from_slice(&4_u32.to_be_bytes());
            expected.extend_from_slice(&36_u32.to_be_bytes());
        }
        if abi >= 3 {
            expected.push(11);
        }
        if abi >= 4 {
            expected.push(12);
        }
        expected.extend([0, 0, 0, 0]);
        assert_eq!(
            interface.canonical_encoding(),
            expected,
            "ABI {abi} frozen wire vector"
        );
        assert_eq!(
            interface.digest().as_bytes().as_slice(),
            Sha256::digest(&expected).as_slice()
        );
        assert_eq!(ProgramInterface::decode(&expected), Ok(interface.clone()));
        let program = ProgramId::new([0x31; 32]).expect("program ID");
        let mut record = vec![0; 33];
        record.extend_from_slice(&interface.code_hash());
        record.extend_from_slice(&abi.to_be_bytes());
        record.extend_from_slice(&1_u32.to_be_bytes());
        let fixture = ChainEvidence::new(
            program,
            record,
            interface_state_value(program, 1, &interface).expect("interface state"),
            wasm(abi, dynamic),
            &interface,
            expected.clone(),
        );
        let head = fixture
            .chain
            .verifier
            .verify_head(&fixture.chain.header, &fixture.chain.commit)
            .expect("signed chain head");
        let ops = fixture.interface_ops();
        let read = VerifiedInterfaceRead::verify_at_head(&head, program, &fixture.witness, &ops)
            .expect("chain-proven canonical interface");
        assert_eq!(read.interface, interface);
        assert_eq!(read.receipt_digest, head.app_hash());
        assert_eq!(read.state_root, head.app_hash());
        assert_eq!(read.freshness.observed_sequence, (head.height() - 1) as u64);
        let mut bad = ops.clone();
        bad[0].data[0] ^= 1;
        assert!(
            VerifiedInterfaceRead::verify_at_head(&head, program, &fixture.witness, &bad).is_err()
        );
        let mut bad = ops.clone();
        bad[2].data[0] ^= 1;
        assert!(
            VerifiedInterfaceRead::verify_at_head(&head, program, &fixture.witness, &bad).is_err()
        );
        for wrong in [0_u16, 5, u16::MAX] {
            let mut bad = expected.clone();
            bad[DOMAIN_LEN + 32..DOMAIN_LEN + 34].copy_from_slice(&wrong.to_be_bytes());
            assert!(ProgramInterface::decode(&bad).is_err());
        }
        for wrong_domain in [b'0', b'1', b'2', b'3', b'4', b'5'] {
            if wrong_domain == b'0' + version as u8 {
                continue;
            }
            let mut bad = expected.clone();
            bad[DOMAIN_LEN - 2] = wrong_domain;
            assert!(ProgramInterface::decode(&bad).is_err());
        }
    }
}

#[test]
fn exact_capabilities_exports_and_upgrade_policy() {
    for (_, abi, dynamic) in CASES {
        let module = wasm(abi, dynamic);
        let original = entry(abi, dynamic, 64, 64);
        let engine = WasmEngine::declared().expect("declared real engine");
        let validated = match abi {
            1 => engine.validate(&module),
            2 => engine.validate_v2(&module),
            3 => engine.validate_v3(&module),
            4 => engine.validate_v4(&module),
            _ => unreachable!(),
        }
        .expect("real imported module validation");
        let mask = if dynamic { 1 << 7 } else { 0 }
            | if abi >= 3 { 1 << 10 } else { 0 }
            | if abi >= 4 { 1 << 11 } else { 0 };
        assert!(validated.interface_capability_mask_matches("call", mask));
        for bit in 0..12 {
            assert!(!validated.interface_capability_mask_matches("call", mask ^ (1 << bit)));
        }
        let mut missing = original.clone();
        missing.name = "missing".to_owned();
        assert_eq!(
            ProgramInterface::bind(&module, abi, vec![missing]),
            Err(InterfaceRefusal::MissingExport)
        );
        let mut excess = original.clone();
        excess
            .capabilities
            .insert(0, InterfaceCapability::StorageRead);
        assert!(ProgramInterface::bind(&module, abi, vec![excess]).is_err());
        if !original.capabilities.is_empty() {
            let mut omitted = original.clone();
            omitted.capabilities.pop();
            assert!(ProgramInterface::bind(&module, abi, vec![omitted]).is_err());
        }
        let prior = bound(abi, dynamic, 64, 64);
        assert_eq!(
            bound(abi, dynamic, 128, 32).authorize_upgrade(&prior, false),
            Ok(())
        );
        let narrower = bound(abi, dynamic, 32, 128);
        assert_eq!(
            narrower.authorize_upgrade(&prior, false),
            Err(InterfaceRefusal::NarrowingUpgrade)
        );
        assert_eq!(narrower.authorize_upgrade(&prior, true), Ok(()));
        for unsupported in [0, 5, u16::MAX] {
            assert!(ProgramInterface::bind(&module, unsupported, vec![original.clone()]).is_err());
        }
    }
    for abi in [1, 2] {
        assert!(
            ProgramInterface::bind(&wasm(3, false), abi, vec![entry(3, false, 64, 64)]).is_err()
        );
        assert!(
            ProgramInterface::bind(&wasm(4, false), abi, vec![entry(4, false, 64, 64)]).is_err()
        );
    }
    assert!(ProgramInterface::bind(&wasm(4, false), 3, vec![entry(4, false, 64, 64)]).is_err());
    assert!(ProgramInterface::bind(&wasm(2, true), 1, vec![entry(2, true, 64, 64)]).is_err());
    for newer in [2, 3, 4] {
        for older in 1..newer {
            assert!(bound(older, false, 64, 64)
                .authorize_upgrade(&bound(newer, false, 64, 64), true)
                .is_err());
        }
    }
    let mut corrupt = wasm(4, false);
    corrupt[0] ^= 1;
    assert!(ProgramInterface::bind(&corrupt, 4, vec![entry(4, false, 64, 64)]).is_err());
    assert!(ProgramInterface::bind(&vec![0; 1_048_577], 4, vec![entry(4, false, 64, 64)]).is_err());
}

#[test]
fn canonical_encoding_refuses_malformed_bounds() {
    let canonical = bound(4, false, 64, 64).canonical_encoding().to_vec();
    for length in 0..canonical.len() {
        assert!(ProgramInterface::decode(&canonical[..length]).is_err());
    }
    let mut appended = canonical.clone();
    appended.push(0);
    assert!(ProgramInterface::decode(&appended).is_err());
    assert!(ProgramInterface::decode(&vec![0; 953]).is_err());
    let mut largest = canonical[..canonical.len() - 4].to_vec();
    largest.extend_from_slice(&26_u16.to_be_bytes());
    for index in 0..26_u8 {
        let mut topic = [0; 32];
        topic[31] = index;
        largest.extend(topic);
    }
    largest.extend([0, 0]);
    let padding = 952 - largest.len();
    let name_offset = DOMAIN_LEN + 36;
    largest[name_offset..name_offset + 2].copy_from_slice(
        &u16::try_from(4 + padding)
            .expect("bounded name")
            .to_be_bytes(),
    );
    largest.splice(name_offset + 6..name_offset + 6, vec![b'x'; padding]);
    assert_eq!(largest.len(), 952);
    assert!(ProgramInterface::decode(&largest).is_ok());
    largest.push(0);
    assert!(ProgramInterface::decode(&largest).is_err());
    for count in [0_u16, 257, u16::MAX] {
        let mut bad = canonical.clone();
        bad[DOMAIN_LEN + 34..DOMAIN_LEN + 36].copy_from_slice(&count.to_be_bytes());
        assert!(ProgramInterface::decode(&bad).is_err());
    }
    let cap_offset = DOMAIN_LEN + 36 + 2 + 4 + 4 + 6 + 6 + 2;
    for tags in [[12, 11], [11, 11], [11, 13], [0xff, 12]] {
        let mut bad = canonical.clone();
        bad[cap_offset..cap_offset + 2].copy_from_slice(&tags);
        assert!(ProgramInterface::decode(&bad).is_err());
    }
    for abi in [1_u16, 2, 3] {
        let mut bad = canonical.clone();
        bad[DOMAIN_LEN - 2] = b'0' + abi as u8;
        bad[DOMAIN_LEN + 32..DOMAIN_LEN + 34].copy_from_slice(&abi.to_be_bytes());
        assert!(ProgramInterface::decode(&bad).is_err());
    }
    let module = wasm(4, false);
    let mut malformed = entry(4, false, 64, 64);
    malformed.name = "x".repeat(129);
    assert!(ProgramInterface::bind(&module, 4, vec![malformed]).is_err());
    let mut malformed = entry(4, false, 64, 64);
    malformed.event_topics = vec![[1; 32]; 257];
    assert!(ProgramInterface::bind(&module, 4, vec![malformed]).is_err());
    let mut value = ValueType::U8;
    for _ in 0..18 {
        value = ValueType::Option(Box::new(value));
    }
    let mut malformed = entry(4, false, 64, 64);
    malformed.calldata = ValueSchema::layerx(value);
    assert!(ProgramInterface::bind(&module, 4, vec![malformed]).is_err());
    let mut malformed = entry(4, false, 64, 64);
    malformed.capabilities.reverse();
    assert!(ProgramInterface::bind(&module, 4, vec![malformed]).is_err());
    assert!(ProgramInterface::bind(&module, 4, vec![entry(4, false, 64, 64); 257]).is_err());
    for offset in [u32::MAX, u32::MAX - 15] {
        let mut malformed = entry(2, true, 64, 64);
        malformed.capabilities = vec![InterfaceCapability::CallerAuthorizedSpend {
            asset: [2; 32],
            maximum_amount: 100,
            recipient_offset: offset,
            amount_offset: offset,
        }];
        assert!(ProgramInterface::bind(&wasm(2, true), 2, vec![malformed]).is_err());
    }
}

const VALIDATOR_SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];

struct ChainEvidence {
    chain: ChainFixture,
    proof: DeploymentProof,
    witness: InterfaceStateWitness,
}

impl ChainEvidence {
    fn new(
        program: ProgramId,
        record: Vec<u8>,
        interface_value: Vec<u8>,
        code: Vec<u8>,
        interface: &ProgramInterface,
        transaction: Vec<u8>,
    ) -> Self {
        let mut record_key = b"program\0".to_vec();
        record_key.extend_from_slice(&program.bytes());
        let record_key = programs_state_key(&record_key);
        let interface_key = interface_state_key(program);
        let mut chain = ChainFixture::new(
            "paxeer-interface-protocol",
            &VALIDATOR_SEEDS,
            vec![
                (record_key.clone(), record.clone()),
                (programs_state_key(&interface_key), interface_value.clone()),
            ],
        )
        .expect("chain fixture");
        let transaction_hash: [u8; 32] = Sha256::digest(&transaction).into();
        let transaction_root = hash_from_byte_slices(&[&transaction_hash]);
        chain.header.data_hash = transaction_root.to_vec();
        chain.commit.block_id.hash = chain.header.hash().expect("chain header hash").to_vec();
        for signature in &mut chain.commit.signatures {
            let key = VALIDATOR_SEEDS
                .iter()
                .map(SigningKey::from_bytes)
                .find(|key| {
                    TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                        .address
                        .as_slice()
                        == signature.validator_address
                })
                .expect("validator signer");
            let bytes = vote_sign_bytes(
                &chain.header.chain_id,
                chain.commit.height,
                chain.commit.round,
                Some(&chain.commit.block_id),
                signature.timestamp,
            )
            .expect("vote sign bytes");
            signature.signature = key.sign(&bytes).to_bytes().to_vec();
        }
        let commitment = decode_commitment_proof(
            &chain
                .encode_key(&record_key)
                .expect("encode chain commitment"),
        )
        .expect("decode chain commitment");
        let program_state = ChainProgramStateProof {
            program,
            ops: chain.prove(&record_key).expect("program ICS23 ops"),
        };
        let mut status_key = b"wind-down\0s".to_vec();
        status_key.extend_from_slice(&program.bytes());
        let status_ops = chain
            .prove_absent(&programs_state_key(&status_key))
            .expect("active lifecycle ICS23 ops");
        let proof = DeploymentProof {
            state: ProgramStateProof {
                commitment,
                state: program_state.clone(),
                lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                    program,
                    record: Some(record),
                    record_ops: program_state.ops.clone(),
                    status: None,
                    status_ops: status_ops.clone(),
                }),
                wind_down: ChainWindDownProof {
                    program,
                    value: None,
                    ops: status_ops,
                },
                transaction,
                transaction_proof: SimpleProof {
                    total: 1,
                    index: 0,
                    leaf_hash: transaction_root,
                    aunts: Vec::new(),
                },
            },
            deployment: ChainDeploymentProof {
                program_state,
                code,
                interface: Some(interface.canonical_encoding().to_vec()),
            },
        };
        Self {
            chain,
            proof,
            witness: InterfaceStateWitness {
                key: interface_key,
                value: interface_value,
                proof: StateProof {
                    leaf_index: 0,
                    leaf_count: 0,
                    siblings: Vec::new(),
                },
            },
        }
    }

    fn interface_ops(&self) -> Vec<ProofOp> {
        let mut ops = self
            .chain
            .prove(&programs_state_key(&self.witness.key))
            .expect("interface ICS23 ops");
        ops.extend(self.proof.state.state.ops.clone());
        ops
    }

    fn verifier(&self) -> ProtocolDeploymentVerifier {
        ProtocolDeploymentVerifier::from_chain_trust(
            &self.chain.verifier.chain_id,
            self.chain.verifier.validators.validators().to_vec(),
            1000,
        )
        .expect("chain validator trust")
    }

    fn now_ms(&self) -> u64 {
        u64::try_from(
            self.chain.header.time.seconds * 1000
                + i64::from(self.chain.header.time.nanos / 1_000_000),
        )
        .expect("chain time")
    }
}

struct NativeEvidence {
    fields: BTreeMap<String, String>,
}
impl NativeEvidence {
    fn read(path: &Path) -> Self {
        let text = fs::read_to_string(path).expect("native evidence.kvx must exist");
        let mut fields = BTreeMap::new();
        for line in text.lines() {
            let (key, value) = line.split_once('=').expect("canonical evidence key=value");
            assert!(
                fields.insert(key.to_owned(), value.to_owned()).is_none(),
                "duplicate evidence key"
            );
        }
        Self { fields }
    }
    fn text(&self, key: &str) -> &str {
        self.fields
            .get(key)
            .unwrap_or_else(|| panic!("missing native field {key}"))
    }
    fn bytes(&self, key: &str) -> Vec<u8> {
        unhex(self.text(key))
    }
    fn fixed<const N: usize>(&self, key: &str) -> [u8; N] {
        self.bytes(key)
            .try_into()
            .unwrap_or_else(|_| panic!("wrong length {key}"))
    }
    fn fixture(&self, interface: &ProgramInterface) -> ChainEvidence {
        let dynamic = interface
            .entries()
            .iter()
            .flat_map(|entry| &entry.capabilities)
            .any(|capability| {
                matches!(
                    capability,
                    InterfaceCapability::CallerAuthorizedSpend { .. }
                )
            });
        let program = ProgramId::new(self.fixed("program_id")).expect("native program ID");
        assert_eq!(
            self.bytes("interface_record.key"),
            interface_state_key(program)
        );
        let mut record_key = b"program\0".to_vec();
        record_key.extend_from_slice(&program.bytes());
        assert_eq!(self.bytes("program_record.key"), record_key);
        ChainEvidence::new(
            program,
            self.bytes("program_record.value"),
            self.bytes("interface_record.value"),
            wasm(interface.abi_version(), dynamic),
            interface,
            self.bytes("activity"),
        )
    }
}

#[test]
fn native_receipt_authorized_interface_reads() {
    let root = directory("PAXEER_X_INTERFACE_FIXTURES");
    for (name, abi, dynamic) in CASES {
        verify_native_case(&root, name, bound(abi, dynamic, 64, 64));
    }
    verify_native_case(&root, "upgrade-widening", bound(2, false, 128, 32));
    verify_native_case(&root, "upgrade-breaking", bound(2, false, 32, 128));
    for name in ["upgrade-narrowing", "upgrade-downgrade"] {
        let path = root.join(name);
        let evidence = NativeEvidence::read(&path.join("evidence.kvx"));
        let prior = if name == "upgrade-narrowing" {
            bound(2, false, 128, 32)
        } else {
            bound(2, false, 32, 128)
        };
        let proposed = if name == "upgrade-narrowing" {
            bound(2, false, 32, 128)
        } else {
            bound(1, false, 64, 64)
        };
        assert_eq!(
            evidence.text("result_code"),
            if name == "upgrade-narrowing" {
                "-3"
            } else {
                "-101"
            }
        );
        let fixture = evidence.fixture(&prior);
        let head = fixture
            .chain
            .verifier
            .verify_head(&fixture.chain.header, &fixture.chain.commit)
            .expect("chain head");
        let read = VerifiedInterfaceRead::verify_at_head(
            &head,
            fixture.proof.state.state.program,
            &fixture.witness,
            &fixture.interface_ops(),
        )
        .expect("unchanged interface after refused native upgrade");
        assert_eq!(read.interface, prior);
        assert!(
            proposed
                .authorize_upgrade(&read.interface, name == "upgrade-downgrade")
                .is_err(),
            "{name} must refuse"
        );
    }
}

fn verify_native_case(root: &Path, name: &str, expected: ProgramInterface) {
    let path = root.join(name);
    let evidence = NativeEvidence::read(&path.join("evidence.kvx"));
    let fixture = evidence.fixture(&expected);
    let verifier = fixture.verifier();
    let now = fixture.now_ms();
    let proof = fixture.proof.clone();
    assert_eq!(
        evidence.text("result_code"),
        "0",
        "{name} successful native receipt required"
    );
    assert_eq!(
        DeploymentProof::decode(
            &proof
                .canonical_encoding()
                .expect("encode chain deployment proof")
        )
        .expect("native proof roundtrip"),
        proof
    );
    let deployment = verifier
        .verify_deployment(&proof, now)
        .unwrap_or_else(|error| panic!("{name}: real native deployment: {error}"));
    assert_eq!(
        ProgramInterface::bind_deployment(&deployment, expected.entries().to_vec()),
        Ok(expected.clone())
    );
    let program = ProgramId::new(evidence.fixed("program_id")).expect("native program ID");
    let head = verifier
        .verify_current_program(&proof.state, program, now)
        .expect("real current program");
    let chain_head = fixture
        .chain
        .verifier
        .verify_head(&fixture.chain.header, &fixture.chain.commit)
        .expect("chain head");
    assert_eq!(head.state_root(), chain_head.app_hash());
    let witness = fixture.witness.clone();
    let ops = fixture.interface_ops();
    let read = VerifiedInterfaceRead::verify_at_head(&chain_head, program, &witness, &ops)
        .expect("chain-authorized native interface");
    assert_eq!(read.interface, expected);
    assert_eq!(read.receipt_digest, chain_head.app_hash());
    assert_eq!(read.state_root, head.state_root());
    let mut bad = proof.clone();
    bad.state.commitment.commit[0] ^= 1;
    assert!(verifier.verify_deployment(&bad, now).is_err());
    let mut bad = proof.clone();
    bad.state.transaction_proof.leaf_hash[0] ^= 1;
    assert!(verifier.verify_deployment(&bad, now).is_err());
    let mut bad = proof.clone();
    bad.state.transaction[0] ^= 1;
    assert!(verifier.verify_deployment(&bad, now).is_err());
    let mut bad = proof.clone();
    bad.state.commitment.signed_header[0] ^= 1;
    assert!(verifier.verify_deployment(&bad, now).is_err());
    let mut bad = proof.clone();
    bad.deployment.program_state.ops[0].data[0] ^= 1;
    assert!(verifier.verify_deployment(&bad, now).is_err());
    assert!(verifier.verify_deployment(&proof, now + 1001).is_err());
    for index in [0, 32, 36, 68, witness.value.len() - 1] {
        let mut bad = witness.clone();
        bad.value[index] ^= 1;
        assert!(VerifiedInterfaceRead::verify_at_head(&chain_head, program, &bad, &ops).is_err());
    }
    let mut bad = witness.clone();
    bad.key[0] ^= 1;
    assert!(VerifiedInterfaceRead::verify_at_head(&chain_head, program, &bad, &ops).is_err());
    let mut bad = ops.clone();
    bad[0].data[0] ^= 1;
    assert!(VerifiedInterfaceRead::verify_at_head(&chain_head, program, &witness, &bad).is_err());
    let mut bad = ops.clone();
    bad[2].data[0] ^= 1;
    assert!(VerifiedInterfaceRead::verify_at_head(&chain_head, program, &witness, &bad).is_err());
}
