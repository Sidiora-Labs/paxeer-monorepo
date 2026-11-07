use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::thread;
use std::time::Duration;

use k256::ecdsa::SigningKey;
use layerx_mirror::committed::{CommittedBlock, CommittedHeader};
use layerx_mirror::ethereum::{EthereumArchiveClient, EthereumProductionConfig};
use layerx_mirror::rpc::{RpcEndpointConfig, RpcQuorumConfig};
use layerx_mirror::signer::{
    RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SigningAlgorithm,
};
use layerx_mirror::store::{
    MirrorChain, PublicationJournal, PublicationPhase, PublicationStage, TransactionIdentity,
};
use layerx_mirror::{Archive, NodeHead};
use layerx_mirror_signer::{Options, SignerListener};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use sha3::Keccak256;

fn bytes(value: &str) -> Vec<u8> {
    let digits = value.strip_prefix("0x").expect("hex prefix");
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&digits[i..i + 2], 16).expect("hex byte"))
        .collect()
}

fn hex(value: &[u8]) -> String {
    let mut text = String::from("0x");
    for byte in value {
        use std::fmt::Write;
        write!(text, "{byte:02x}").expect("hex output");
    }
    text
}

fn main() {
    let arguments: Vec<_> = std::env::args().collect();
    let mode = &arguments[1];
    let settings: Value =
        serde_json::from_slice(&fs::read(&arguments[2]).expect("settings")).expect("settings JSON");
    let directory = PathBuf::from(settings["directory"].as_str().expect("directory"));
    let fixture: Value = serde_json::from_slice(
        &fs::read(settings["fixture"].as_str().expect("fixture path")).expect("recorded source"),
    )
    .expect("recorded source JSON");
    let block = CommittedBlock {
        header: CommittedHeader::from_block_json(&fixture["committedBlock"])
            .expect("recorded committed header"),
        receipts: fixture["receipts"]
            .as_array()
            .expect("recorded receipts")
            .iter()
            .map(|receipt| bytes(receipt["receipt"].as_str().expect("canonical receipt")))
            .collect(),
    };
    let archive = Archive::from_committed(
        &block,
        125,
        NodeHead {
            latest_sealed_batch: block.header.height,
            latest_committed: None,
        },
    )
    .expect("archive from recorded producer");
    let chunks = archive.bytes().len().div_ceil(1024);
    assert!(chunks > 1, "recovery must resume a multi-chunk archive");
    let journal_directory = directory.join("journal");
    let recovered_transaction = if mode == "submit" {
        None
    } else {
        let journal = PublicationJournal::open(&journal_directory, MirrorChain::Ethereum, 1000)
            .expect("reopen actual publisher journal");
        let manifest = journal
            .record(archive.commitment(), PublicationStage::Manifest)
            .expect("persisted manifest transaction");
        assert!(
            !manifest.signed_payload.is_empty(),
            "signed bytes survived exit"
        );
        assert_eq!(manifest.committed, archive.data().committed_coordinate());
        if mode == "recover" {
            assert_eq!(manifest.phase, PublicationPhase::Pending);
            assert!(journal
                .record(archive.commitment(), PublicationStage::Chunk(0))
                .is_none());
        } else {
            assert_eq!(manifest.phase, PublicationPhase::Finalized);
            assert_eq!(
                journal
                    .record(archive.commitment(), PublicationStage::Chunk(0))
                    .expect("resumed chunk")
                    .phase,
                PublicationPhase::Pending
            );
        }
        let TransactionIdentity::Ethereum(transaction) = manifest.transaction else {
            panic!("persisted Ethereum transaction identity");
        };
        assert_eq!(journal.cursor().latest_batch, None);
        Some(transaction)
    };

    // Use the production signer service and its production UDS protocol.
    let key = directory.join("ethereum.key");
    let mut scalar = [0_u8; 32];
    scalar[31] = 1;
    fs::write(&key, &hex(&scalar)[2..]).expect("local-only signer key");
    fs::set_permissions(&key, fs::Permissions::from_mode(0o600)).expect("key permissions");
    let options = Options {
        socket: directory.join(format!("{mode}.sock")),
        ethereum_key_file: key,
        ethereum_key_handle: "repair/ethereum".into(),
        solana_keypair_file: directory.join("unused-solana-key"),
        solana_key_handle: "repair/solana".into(),
    };
    let listener = SignerListener::bind(&options).expect("production signer listener");
    thread::spawn(move || listener.serve());
    let public_key = SigningKey::from_slice(&scalar)
        .expect("local test scalar")
        .verifying_key()
        .to_encoded_point(true)
        .as_bytes()
        .to_vec();
    let signer = RemoteChainSigner::new(RemoteSignerConfig {
        endpoint: SignerEndpoint::Uds {
            socket: options.socket,
        },
        algorithm: SigningAlgorithm::Secp256k1Recoverable,
        key_handle: options.ethereum_key_handle,
        public_key,
        timeout: Duration::from_secs(5),
    })
    .expect("production remote signer");
    let rpc = RpcQuorumConfig {
        endpoints: settings["endpoints"]
            .as_array()
            .expect("two independent EVM backends")
            .iter()
            .enumerate()
            .map(|(index, endpoint)| RpcEndpointConfig {
                url: endpoint.as_str().expect("HTTPS endpoint").into(),
                ca_certificate_der: directory.join("ca.der"),
                bearer_token_file: directory.join("bearer"),
                independent_backend: format!("anvil-{index}"),
            })
            .collect(),
        quorum: 2,
        connect_timeout_ms: 5000,
        request_timeout_ms: 10000,
        maximum_response_bytes: 1024 * 1024,
    };
    let mut client = EthereumArchiveClient::open(
        EthereumProductionConfig {
            rpc,
            chain_id: 31337,
            genesis_hash: bytes(settings["genesis"].as_str().expect("genesis"))
                .try_into()
                .expect("hash"),
            archive_contract: bytes(settings["contract"].as_str().expect("contract"))
                .try_into()
                .expect("address"),
            archive_code_hash: Keccak256::digest(bytes(
                settings["runtime"].as_str().expect("runtime"),
            ))
            .into(),
            first_batch_number: 1000,
            required_confirmations: 1,
            maximum_reorg_depth: 64,
            chunk_bytes: 1024,
            transaction_gas_limit: 29_000_000,
            maximum_fee_per_gas: 100_000_000_000,
            maximum_priority_fee_per_gas: 2_000_000_000,
            journal_directory,
        },
        signer,
    )
    .expect("open real publisher");
    let mut progress = client
        .advance(&archive)
        .expect("publisher verifies actual receipt");
    if mode == "submit" {
        assert_eq!(progress.stage, PublicationStage::Manifest);
        assert_eq!(progress.phase, PublicationPhase::Pending);
    } else if mode == "recover" {
        assert_eq!(progress.stage, PublicationStage::Manifest);
        assert_eq!(progress.phase, PublicationPhase::Finalized);
        assert_eq!(progress.transaction_hash, recovered_transaction);
        progress = client
            .advance(&archive)
            .expect("resume first chunk after recovered begin");
        assert_eq!(progress.stage, PublicationStage::Chunk(0));
        assert_eq!(progress.phase, PublicationPhase::Pending);
    } else {
        assert_eq!(mode, "finish");
        for _ in 0..(2 * chunks + 4) {
            if progress.phase == PublicationPhase::RetrievedVerified {
                break;
            }
            progress = client
                .advance(&archive)
                .expect("resume chunks and finalize");
        }
        assert_eq!(progress.stage, PublicationStage::Finalize);
        assert_eq!(progress.phase, PublicationPhase::RetrievedVerified);
        assert_eq!(
            client
                .retrieve(archive.commitment())
                .expect("retrieve real archive"),
            Some(archive.bytes().to_vec())
        );
        assert_eq!(client.cursor().latest_batch, Some(block.header.height));
        assert_eq!(
            client.cursor().latest_committed,
            archive.data().committed_coordinate()
        );
        drop(client);
        let journal =
            PublicationJournal::open(directory.join("journal"), MirrorChain::Ethereum, 1000)
                .expect("reopen completed journal");
        assert_eq!(journal.cursor(), progress.cursor);
    }
    println!(
        "{}",
        json!({
            "commitment": hex(archive.commitment().as_bytes()),
            "digest": hex(&Sha256::digest(archive.bytes())),
            "archive": hex(archive.bytes()),
            "blockHash": hex(&block.header.block_hash),
            "height": block.header.height,
            "chunks": chunks,
            "transaction": hex(&progress.transaction_hash.expect("stage transaction")),
        })
    );
}
