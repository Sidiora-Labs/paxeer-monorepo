use std::path::Path;
use std::str::FromStr;
use std::time::Duration;

use base64::{engine::general_purpose::STANDARD, Engine};
use ethabi::Token;
use layerx_mirror::committed::{CommittedBlock, CommittedHeader};
use layerx_mirror::rpc::RpcQuorumConfig;
use layerx_mirror::runtime::RuntimeConfig;
use layerx_mirror::signer::{
    RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SigningAlgorithm,
};
use layerx_mirror::solana::{
    SolanaArchiveClient, SolanaMirrorReadConfig, SolanaMirrorReader, SolanaProductionConfig,
};
use layerx_mirror::{Archive, ArchiveCommitment, CommittedCoordinate, NodeHead};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use sha3::Keccak256;
use solana_loader_v3_interface::state::UpgradeableLoaderState;
use solana_pubkey::Pubkey;

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn bytes(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    (0..text.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&text[index..index + 2], 16).unwrap())
        .collect()
}

fn key(value: &Value, field: &str) -> [u8; 32] {
    Pubkey::from_str(value[field].as_str().unwrap())
        .unwrap()
        .to_bytes()
}

fn produce() -> Value {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../crates/layerx-mirror/tests/fixtures/committed_blocks.json"
    ))
    .unwrap();
    let mut config: Value =
        serde_json::from_str(include_str!("../../config.example.json")).unwrap();
    config["paxeer"]["genesis_hash_hex"] = fixture["genesis"]["hash"].clone();
    let runtime: RuntimeConfig = serde_json::from_value(config).unwrap();
    let block = CommittedBlock {
        header: CommittedHeader::from_block_json(&fixture["committedBlock"]).unwrap(),
        receipts: fixture["receipts"]
            .as_array()
            .unwrap()
            .iter()
            .map(|receipt| bytes(receipt["receipt"].as_str().unwrap()))
            .collect(),
    };
    let archive = Archive::from_committed(
        &block,
        runtime.committed_network_id().unwrap(),
        NodeHead {
            latest_sealed_batch: block.header.height,
            latest_committed: Some(CommittedCoordinate {
                height: block.header.height,
                block_hash: block.header.block_hash,
            }),
        },
    )
    .unwrap();
    let payload = archive.bytes();
    let code = bytes("6000");
    let manifest = ethabi::encode(&[
        Token::Uint(payload.len().into()),
        Token::Uint(1_u64.into()),
        Token::FixedBytes(Sha256::digest(payload).to_vec()),
        Token::Bool(true),
    ]);
    json!({
        "archive": hex(payload),
        "commitment": hex(archive.commitment().as_bytes()),
        "height": block.header.height,
        "block_hash": hex(&block.header.block_hash),
        "manifest": format!("0x{}", hex(&manifest)),
        "chunk": format!("0x{}", hex(&ethabi::encode(&[Token::Bytes(payload.to_vec())]))),
        "publisher": format!("0x{}", hex(&ethabi::encode(&[Token::Address([1;20].into())]))),
        "selectors": {
            "publisher": hex(&ethabi::short_signature("publisher", &[])),
            "manifest": hex(&ethabi::short_signature("manifest", &[ethabi::ParamType::FixedBytes(32)])),
            "chunk": hex(&ethabi::short_signature("chunk", &[ethabi::ParamType::FixedBytes(32), ethabi::ParamType::Uint(32)]))
        },
        "code": format!("0x{}", hex(&code)),
        "code_hash": hex(&Keccak256::digest(&code)),
        "receipt_keys": runtime.paxeer.receipt_keys.iter().map(|key| json!({
            "sequencer_id_hex": key.sequencer_id_hex,
            "public_key_hex": key.public_key_hex,
            "first_height": key.first_height,
            "last_height": key.last_height,
        })).collect::<Vec<_>>(),
    })
}

fn loader_fixture(elf: &Path) -> Value {
    let identity: Value =
        serde_json::from_str(include_str!("../../solana-devnet-2026-08-31.json")).unwrap();
    let program = key(&identity, "program_id");
    let program_data = key(&identity, "program_data_account");
    let publisher = key(&identity, "publisher_ed25519_public_key");
    let code = std::fs::read(elf).unwrap();
    assert!(code.starts_with(b"\x7fELF"));
    let program_bytes = bincode::serialize(&UpgradeableLoaderState::Program {
        programdata_address: Pubkey::new_from_array(program_data),
    })
    .unwrap();
    let data = |authority| {
        let mut data = bincode::serialize(&UpgradeableLoaderState::ProgramData {
            slot: 42,
            upgrade_authority_address: authority,
        })
        .unwrap();
        data.resize(UpgradeableLoaderState::size_of_programdata_metadata(), 0);
        data.extend_from_slice(&code);
        STANDARD.encode(data)
    };
    json!({
        "identity": identity,
        "program_hex": hex(&program),
        "program_data_hex": hex(&program_data),
        "publisher_hex": hex(&publisher),
        "program": STANDARD.encode(program_bytes),
        "mutable": data(Some(Pubkey::new_from_array(publisher))),
        "immutable": data(None),
        "code_hash": hex(&Sha256::digest(&code)),
        "elf_bytes": code.len(),
    })
}

fn identity(input: &Path) -> Value {
    let document: Value = serde_json::from_slice(&std::fs::read(input).unwrap()).unwrap();
    let rpc: RpcQuorumConfig = serde_json::from_value(document["rpc"].clone()).unwrap();
    let deployment = &document["deployment"];
    let genesis_hash = key(deployment, "genesis_hash");
    let archive_program = key(deployment, "program_id");
    let program_data_account = key(deployment, "program_data_account");
    let publisher = key(deployment, "publisher_ed25519_public_key");
    let upgradeable_loader = key(deployment, "upgradeable_loader_id");
    let program_code_hash: [u8; 32] = bytes(document["code_hash"].as_str().unwrap())
        .try_into()
        .unwrap();
    let signer = RemoteChainSigner::new(RemoteSignerConfig {
        endpoint: SignerEndpoint::Uds {
            socket: input.with_extension("sock"),
        },
        algorithm: SigningAlgorithm::Ed25519,
        key_handle: "mirror/solana/test".into(),
        public_key: publisher.to_vec(),
        timeout: Duration::from_secs(1),
    })
    .unwrap();
    let publisher_result = SolanaArchiveClient::open(
        SolanaProductionConfig {
            rpc: rpc.clone(),
            genesis_hash,
            archive_program,
            upgradeable_loader,
            program_data_account,
            program_code_hash,
            first_batch_number: 1000,
            required_rooted_slots: 32,
            maximum_ancestry: 4096,
            chunk_bytes: 640,
            journal_directory: input.with_extension("journal"),
        },
        signer,
    )
    .map(|_| "accepted".to_owned())
    .unwrap_or_else(|error| format!("{error:?}"));
    let reader = SolanaMirrorReader::open(SolanaMirrorReadConfig {
        rpc,
        genesis_hash,
        archive_program,
        upgradeable_loader,
        program_data_account,
        program_code_hash,
        publisher,
    })
    .unwrap();
    let reader_result = reader
        .retrieve(ArchiveCommitment::from_bytes([9; 32]))
        .map(|value| {
            assert!(value.is_none());
            "accepted".to_owned()
        })
        .unwrap_or_else(|error| format!("{error:?}"));
    json!({"publisher": publisher_result, "reader": reader_result})
}

fn main() {
    let arguments: Vec<_> = std::env::args().collect();
    let output = match arguments[1].as_str() {
        "produce" => produce(),
        "loader-fixture" => loader_fixture(Path::new(&arguments[2])),
        "identity" => identity(Path::new(&arguments[2])),
        _ => panic!("unknown driver operation"),
    };
    println!("{output}");
}
