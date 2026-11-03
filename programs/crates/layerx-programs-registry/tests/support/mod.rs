pub mod chain;

use chain::ChainFixture;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_programs::{
    chain_evidence::{
        deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, programs_state_key, wind_down::ChainWindDownProof,
    },
    chain_proof::{
        decode::decode_commitment_proof,
        merkle::{hash_from_byte_slices, SimpleProof},
        validators::TrustedValidator,
        vote::{vote_sign_bytes, Timestamp},
    },
    interface_state_key, interface_state_value, DeploymentProof, InterfaceEntryPoint,
    ProgramInterface, ProgramLifecycleProof, ProgramStateProof, ProtocolDeploymentVerifier,
    StateLeafWitness, ValueSchema, ValueType,
};
use layerx_programs_runtime::{hash_bytes, HashAlgorithm, ProgramId, UpgradePolicy};

pub const NOW: u64 = 1_700_000_150_000;
pub const PROGRAM_BYTES: [u8; 32] = [0x31; 32];
pub const AUTHORITY: [u8; 32] = [0x51; 32];
pub const WASM_V1: &[u8] = &[
    0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3, 2, 0,
    1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a', b'y', b'e', b'r',
    b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm', b'e', b'm', b'o', b'r',
    b'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
];
pub const WASM_V2: &[u8] = &[
    0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3, 2, 0,
    1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a', b'y', b'e', b'r',
    b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm', b'e', b'm', b'o', b'r',
    b'y', 2, 0, 10, 11, 2, 4, 0, 65, 1, 11, 4, 0, 65, 0, 11,
];
const CHAIN_ID: &str = "paxeer-registry-support";
const VALIDATOR_SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];

pub struct ProtocolFixture {
    pub chain: ChainFixture,
    pub proof: DeploymentProof,
    pub interface_witness: StateLeafWitness,
    pub block_height: u64,
    policy: UpgradePolicy,
    version: u32,
}

#[must_use]
pub fn verifier_for_fixture(
    fixture: &ProtocolFixture,
    staleness_ms: u64,
) -> ProtocolDeploymentVerifier {
    ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.chain.verifier.chain_id,
        fixture.chain.verifier.validators.validators().to_vec(),
        staleness_ms,
    )
    .unwrap_or_else(|error| panic!("chain verifier: {error}"))
}

#[must_use]
pub fn program() -> ProgramId {
    ProgramId::new(PROGRAM_BYTES).unwrap_or_else(|error| panic!("program: {error}"))
}

#[must_use]
pub fn code_hash(wasm: &[u8]) -> [u8; 32] {
    hash_bytes(HashAlgorithm::Sha256, wasm)
        .unwrap_or_else(|error| panic!("program code hash: {error}"))
}

#[must_use]
pub fn deploy_fixture(
    wasm: &[u8],
    policy: UpgradePolicy,
    block_height: u64,
    timestamp: u64,
) -> ProtocolFixture {
    deploy_fixture_with_interface(
        wasm,
        policy,
        block_height,
        timestamp,
        &fixture_interface(wasm),
    )
}

#[must_use]
pub fn deploy_fixture_with_interface(
    wasm: &[u8],
    policy: UpgradePolicy,
    block_height: u64,
    timestamp: u64,
    interface: &ProgramInterface,
) -> ProtocolFixture {
    fixture_with_interface(
        &deploy_payload(wasm, policy, interface),
        &FixtureState {
            wasm,
            policy,
            version: 1,
            block_height,
            timestamp,
            deprecated: false,
        },
        interface,
    )
}

#[must_use]
pub fn programs_call_fixture(payload: &[u8], block_height: u64, timestamp: u64) -> ProtocolFixture {
    fixture(
        payload,
        &FixtureState {
            wasm: WASM_V1,
            policy: UpgradePolicy::Authority(AUTHORITY),
            version: 1,
            block_height,
            timestamp,
            deprecated: false,
        },
    )
}

#[must_use]
pub fn wrong_abi_fixture(block_height: u64, timestamp: u64) -> ProtocolFixture {
    let mut fixture = deploy_fixture(
        WASM_V1,
        UpgradePolicy::Authority(AUTHORITY),
        block_height,
        timestamp,
    );
    let interface =
        ProgramInterface::bind(WASM_V1, 1, fixture_interface(WASM_V1).entries().to_vec())
            .unwrap_or_else(|error| panic!("fixture interface: {error}"));
    let mut encoding = interface.canonical_encoding().to_vec();
    let abi_offset = b"LayerX/program-interface/v1\0".len() + 32;
    encoding[abi_offset..abi_offset + 2].copy_from_slice(&3_u16.to_be_bytes());
    fixture.proof.deployment.interface = Some(encoding);
    fixture
}

#[must_use]
pub fn upgrade_fixture(
    old_wasm: &[u8],
    new_wasm: &[u8],
    block_height: u64,
    timestamp: u64,
) -> ProtocolFixture {
    fixture(
        &upgrade_payload(old_wasm, new_wasm),
        &FixtureState {
            wasm: new_wasm,
            policy: UpgradePolicy::Authority(AUTHORITY),
            version: 2,
            block_height,
            timestamp,
            deprecated: false,
        },
    )
}

#[must_use]
pub fn deprecated_state(fixture: &ProtocolFixture, timestamp: u64) -> ProgramStateProof {
    self::fixture(
        &fixture.proof.state.transaction,
        &FixtureState {
            wasm: &fixture.proof.deployment.code,
            policy: fixture.policy,
            version: fixture.version,
            block_height: fixture
                .block_height
                .checked_add(1)
                .unwrap_or_else(|| panic!("state height overflow")),
            timestamp,
            deprecated: true,
        },
    )
    .proof
    .state
}

struct FixtureState<'a> {
    wasm: &'a [u8],
    policy: UpgradePolicy,
    version: u32,
    block_height: u64,
    timestamp: u64,
    deprecated: bool,
}

fn fixture(transaction: &[u8], state: &FixtureState<'_>) -> ProtocolFixture {
    fixture_with_interface(transaction, state, &fixture_interface(state.wasm))
}

fn fixture_with_interface(
    transaction: &[u8],
    state: &FixtureState<'_>,
    interface: &ProgramInterface,
) -> ProtocolFixture {
    let record = program_record(state.wasm, state.policy, state.version);
    let program_key = programs_state_key(&program_key());
    let interface_key = interface_state_key(program());
    let interface_value = interface_state_value(program(), state.version, interface)
        .unwrap_or_else(|error| panic!("interface state value: {error}"));
    let mut pairs = vec![
        (program_key.clone(), record.clone()),
        (programs_state_key(&interface_key), interface_value.clone()),
    ];
    let status = state.deprecated.then(status_record);
    let status_key = programs_state_key(&status_key());
    if let Some(value) = &status {
        pairs.push((status_key.clone(), value.clone()));
    }
    let mut chain = ChainFixture::new(CHAIN_ID, &VALIDATOR_SEEDS, pairs)
        .unwrap_or_else(|error| panic!("chain fixture: {error:?}"));
    chain.header.height = i64::try_from(state.block_height)
        .ok()
        .and_then(|height| height.checked_add(1))
        .unwrap_or_else(|| panic!("block height overflow"));
    assert!(state.block_height > 0, "state height must be positive");
    chain.header.time.seconds = i64::try_from(state.timestamp / 1_000)
        .unwrap_or_else(|_| panic!("block timestamp overflow"));
    chain.header.time.nanos = i32::try_from(state.timestamp % 1_000 * 1_000_000)
        .unwrap_or_else(|_| panic!("block timestamp nanoseconds"));
    let transaction_hash = code_hash(transaction);
    let transaction_root = hash_from_byte_slices(&[&transaction_hash]);
    chain.header.data_hash = transaction_root.to_vec();
    chain.commit.height = chain.header.height;
    chain.commit.block_id.hash = chain
        .header
        .hash()
        .unwrap_or_else(|error| panic!("block header hash: {error:?}"))
        .to_vec();
    let keys = VALIDATOR_SEEDS
        .iter()
        .map(SigningKey::from_bytes)
        .collect::<Vec<_>>();
    for signature in &mut chain.commit.signatures {
        let key = keys
            .iter()
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .unwrap_or_else(|| panic!("validator signing key absent"));
        signature.timestamp = Timestamp {
            seconds: chain.header.time.seconds,
            nanos: chain.header.time.nanos,
        };
        let bytes = vote_sign_bytes(
            &chain.header.chain_id,
            chain.commit.height,
            chain.commit.round,
            Some(&chain.commit.block_id),
            signature.timestamp,
        )
        .unwrap_or_else(|error| panic!("vote sign bytes: {error:?}"));
        signature.signature = key.sign(&bytes).to_bytes().to_vec();
    }
    let commitment = decode_commitment_proof(
        &chain
            .encode_key(&program_key)
            .unwrap_or_else(|error| panic!("encode commitment: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("decode commitment: {error:?}"));
    let program_state = ChainProgramStateProof {
        program: program(),
        ops: chain
            .prove(&program_key)
            .unwrap_or_else(|error| panic!("program proof: {error:?}")),
    };
    let status_ops = if status.is_some() {
        chain.prove(&status_key)
    } else {
        chain.prove_absent(&status_key)
    }
    .unwrap_or_else(|error| panic!("status proof: {error:?}"));
    let interface_witness = StateLeafWitness {
        key: interface_key.clone(),
        value: interface_value,
        proof: chain
            .prove(&programs_state_key(&interface_key))
            .unwrap_or_else(|error| panic!("interface proof: {error:?}")),
    };
    ProtocolFixture {
        chain,
        proof: DeploymentProof {
            state: ProgramStateProof {
                commitment,
                state: program_state.clone(),
                lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                    program: program(),
                    record: Some(record),
                    record_ops: program_state.ops.clone(),
                    status: status.clone(),
                    status_ops: status_ops.clone(),
                }),
                wind_down: ChainWindDownProof {
                    program: program(),
                    value: status,
                    ops: status_ops,
                },
                transaction: transaction.to_vec(),
                transaction_proof: SimpleProof {
                    total: 1,
                    index: 0,
                    leaf_hash: transaction_root,
                    aunts: Vec::new(),
                },
            },
            deployment: ChainDeploymentProof {
                program_state,
                code: state.wasm.to_vec(),
                interface: Some(interface.canonical_encoding().to_vec()),
            },
        },
        interface_witness,
        block_height: state.block_height,
        policy: state.policy,
        version: state.version,
    }
}

fn deploy_payload(wasm: &[u8], policy: UpgradePolicy, interface: &ProgramInterface) -> Vec<u8> {
    use layerx_types::program_lifecycle::{NativeProgramDeploy, ProgramUpgradePolicy};
    NativeProgramDeploy {
        program_id: layerx_types::intent::ProgramId::new(PROGRAM_BYTES),
        guest_abi: 1,
        policy: match policy {
            UpgradePolicy::Immutable => ProgramUpgradePolicy::Immutable,
            UpgradePolicy::Authority(authority) => ProgramUpgradePolicy::Authority(authority),
        },
        new_hash: code_hash(wasm),
        interface: Some(interface.canonical_encoding()),
        wasm,
    }
    .encode()
    .unwrap_or_else(|error| panic!("native deployment encoding: {error:?}"))
}

/// Produces an actual runtime deployment before constructing its signed chain
/// membership proofs. The expected record comes from the runtime receipt.
pub fn produced_deployment(
    wasm: &[u8],
    policy: UpgradePolicy,
    block_height: u64,
    timestamp: u64,
) -> (
    layerx_programs_runtime::Lifecycle,
    layerx_programs::DeploymentRecord,
    ProtocolFixture,
) {
    use layerx_programs_runtime::{Deploy, Lifecycle, ProgramVersion};
    let mut lifecycle = Lifecycle::declared().unwrap_or_else(|error| panic!("runtime: {error}"));
    let version = ProgramVersion {
        code_hash: code_hash(wasm),
        wasm: wasm.to_vec(),
        abi_version: 1,
    };
    let receipt = lifecycle
        .deploy(Deploy {
            program: program(),
            code_hash: version.code_hash,
            wasm: version.wasm.clone(),
            abi_version: version.abi_version,
            upgrade_policy: policy,
        })
        .unwrap_or_else(|error| panic!("runtime producer: {error}"));
    let record = layerx_programs::DeploymentRecord::from_deployment(
        &receipt,
        &version,
        policy,
        block_height.checked_add(1).expect("header height"),
        timestamp,
    )
    .unwrap_or_else(|error| panic!("producer record: {error}"));
    let fixture = deploy_fixture(wasm, policy, block_height, timestamp);
    assert_eq!(fixture.proof.deployment.code, record.module);
    assert_eq!(fixture.proof.state.state.program, record.program);
    (lifecycle, record, fixture)
}

impl ProtocolFixture {
    pub fn program_bundle(&self) -> layerx_programs::ProgramStateBundle {
        layerx_programs::ProgramStateBundle {
            commitment: self.proof.state.commitment.clone(),
            program: self.proof.state.state.program,
            program_record_ops: self.proof.state.state.ops.clone(),
            interface_ops: self.interface_witness.proof.clone(),
            wind_down: Some(self.proof.state.wind_down.clone()),
        }
    }
}

fn upgrade_payload(old_wasm: &[u8], new_wasm: &[u8]) -> Vec<u8> {
    use layerx_types::program_lifecycle::NativeProgramUpgrade;
    let interface_encoding = fixture_interface_encoding(new_wasm);
    NativeProgramUpgrade {
        program_id: layerx_types::intent::ProgramId::new(PROGRAM_BYTES),
        guest_abi: 1,
        old_hash: code_hash(old_wasm),
        new_hash: code_hash(new_wasm),
        migration_hook: &[],
        clear_interface: false,
        interface: Some(&interface_encoding),
        wasm: new_wasm,
    }
    .encode()
    .unwrap_or_else(|error| panic!("native upgrade encoding: {error:?}"))
}

#[must_use]
pub fn fixture_interface(wasm: &[u8]) -> ProgramInterface {
    ProgramInterface::bind(
        wasm,
        1,
        vec![InterfaceEntryPoint {
            name: "call".to_owned(),
            discriminator: [0x10, 0x20, 0x30, 0x40],
            calldata: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            response: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            capabilities: Vec::new(),
            event_topics: vec![[0x44; 32]],
            failures: Vec::new(),
        }],
    )
    .unwrap_or_else(|error| panic!("program interface: {error}"))
}

fn fixture_interface_encoding(wasm: &[u8]) -> Vec<u8> {
    fixture_interface(wasm).canonical_encoding().to_vec()
}

fn program_key() -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&PROGRAM_BYTES);
    key
}

fn status_key() -> Vec<u8> {
    let mut key = b"wind-down\0s".to_vec();
    key.extend_from_slice(&PROGRAM_BYTES);
    key
}

fn program_record(wasm: &[u8], policy: UpgradePolicy, version: u32) -> Vec<u8> {
    let mut record = Vec::with_capacity(71);
    match policy {
        UpgradePolicy::Immutable => {
            record.push(0);
            record.extend_from_slice(&[0; 32]);
        }
        UpgradePolicy::Authority(authority) => {
            record.push(1);
            record.extend_from_slice(&authority);
        }
    }
    record.extend_from_slice(&code_hash(wasm));
    record.extend_from_slice(&1_u16.to_be_bytes());
    record.extend_from_slice(&version.to_be_bytes());
    record
}

fn status_record() -> Vec<u8> {
    let mut record = vec![0; 54];
    record[0] = 1;
    record[1] = 2;
    record[2..34].copy_from_slice(&PROGRAM_BYTES);
    record[34..42].copy_from_slice(&1_800_000_000_u64.to_be_bytes());
    record[42..50].copy_from_slice(&72_u64.to_be_bytes());
    record
}
