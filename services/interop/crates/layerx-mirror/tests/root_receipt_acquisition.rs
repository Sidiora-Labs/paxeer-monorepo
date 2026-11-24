mod support;

use layerx_mirror::committed::{
    CommittedBlockSource, CommittedCoordinate, CommittedHeader, CommittedRefusal,
};
use layerx_mirror::rpc::{RpcCluster, RpcError};
use layerx_mirror::source::{
    MirrorCanonicalPosition, MirrorObservation, MirrorSourceId, MirrorTargetIdentity,
};
use layerx_mirror::{Archive, MirrorEvidenceLevel, MirrorVerifier, NodeHead};
use layerx_proof::inclusion::SequencerAuthorization;
use serde_json::{json, Value};

fn header(document: &Value) -> CommittedHeader {
    CommittedHeader::from_block_json(&document["committedBlock"]).or_panic("recorded header")
}

fn assert_rpc_requests(
    rpc: &support::TestRpc,
    receipts: &[Value],
    height: u64,
    transactions: &[[u8; 32]],
) {
    let requests = rpc
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("requests: {error}"));
    for peer in 0..2 {
        let calls: Vec<_> = requests
            .iter()
            .filter(|(index, _)| *index == peer)
            .map(|(_, request)| request)
            .collect();
        assert_eq!(calls.len(), 3 + receipts.len());
        assert_eq!(calls[0]["method"], "eth_chainId");
        assert_eq!(calls[0]["params"], json!([]));
        assert_eq!(calls[1]["method"], "eth_getBlockByNumber");
        assert_eq!(calls[1]["params"], json!(["0x0", false]));
        assert_eq!(calls[2]["method"], "eth_getBlockByNumber");
        assert_eq!(calls[2]["params"], json!([format!("0x{height:x}"), false]));
        for (index, transaction) in transactions.iter().enumerate() {
            assert_eq!(calls[index + 3]["method"], "pax_sequencerReceipt");
            assert_eq!(
                calls[index + 3]["params"],
                json!([receipts[index]["txHash"]])
            );
            assert_eq!(support::bytes(&receipts[index]["txHash"]), *transaction);
        }
    }
}

#[test]
fn finding_20_root_acquisition_reaches_the_mirror_receipt_consumer() {
    let document = support::fixture();
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    assert_eq!(rpc.config.rpc.quorum, 2);
    for endpoint in &rpc.config.rpc.endpoints {
        let authority = endpoint
            .url
            .strip_prefix("https://")
            .or_panic("authenticated HTTPS endpoint");
        assert_eq!(authority.split_once('/').map(|(_, path)| path), Some(""));
    }
    let source = CommittedBlockSource::open(&rpc.config).or_panic("root source");
    let expected = header(&document);
    let block = source.acquire(expected.height).or_panic("root acquisition");
    assert_eq!(block.header, expected);
    let coordinate = CommittedCoordinate {
        height: block.header.height,
        block_hash: block.header.block_hash,
    };
    let chain_id = u32::try_from(rpc.config.chain_id)
        .unwrap_or_else(|error| panic!("archive chain ID: {error}"));
    let archive = Archive::from_committed(
        &block,
        chain_id,
        NodeHead {
            latest_sealed_batch: coordinate.height,
            latest_committed: Some(coordinate),
        },
    )
    .or_panic("archive producer");
    let observation = MirrorObservation::from_archive(
        MirrorSourceId::new("shared-native-root").or_panic("source identity"),
        MirrorTargetIdentity::Ethereum {
            chain_id: 1,
            genesis_hash: [1; 32],
            contract: [2; 20],
            code_hash: [3; 32],
            publisher: [4; 20],
        },
        archive.bytes(),
        chain_id,
        coordinate.height,
        archive.commitment(),
        MirrorCanonicalPosition::Ethereum {
            block_number: 10,
            block_hash: [5; 32],
            reference_head_number: 12,
            reference_head_hash: [6; 32],
        },
    )
    .or_panic("archive observation");
    let verifier =
        MirrorVerifier::admit_committed(archive.bytes(), observation).or_panic("archive consumer");
    let key = &document["receiptKey"];
    let keys = [SequencerAuthorization::new(
        support::bytes(&key["keyId"])
            .try_into()
            .unwrap_or_else(|_| panic!("key ID")),
        support::bytes(&key["publicKey"])
            .try_into()
            .unwrap_or_else(|_| panic!("public key")),
        key["firstHeight"].as_u64().or_panic("first height"),
        key["lastHeight"].as_u64().or_panic("last height"),
    )];
    assert_eq!(verifier.committed(), Ok(coordinate));
    let receipts = document["receipts"]
        .as_array()
        .or_panic("recorded receipts");
    assert_eq!(block.receipts.len(), receipts.len());
    for (index, receipt) in receipts.iter().enumerate() {
        assert_eq!(block.receipts[index], support::bytes(&receipt["receipt"]));
        let receipt_proof = verifier
            .committed_receipt(index, &keys)
            .or_panic("receipt consumer");
        assert_eq!(receipt_proof.level(), MirrorEvidenceLevel::Committed);
        assert_eq!(receipt_proof.batch_number(), coordinate.height);
        assert_eq!(
            layerx_wire::receipt::encode(receipt_proof.value()).or_panic("receipt codec"),
            block.receipts[index]
        );
    }
    assert_rpc_requests(
        &rpc,
        receipts,
        coordinate.height,
        &block.header.transactions,
    );
}

#[test]
fn finding_20_root_source_preserves_chain_and_genesis_trust() {
    let document = support::fixture();
    let rpc = support::TestRpc::new(vec![document.clone(), document]);
    let mut config = rpc.config.clone();
    config.chain_id += 1;
    assert!(matches!(
        CommittedBlockSource::new(&config),
        Err(CommittedRefusal::ChainMismatch)
    ));
    config = rpc.config.clone();
    config.genesis_hash_hex = format!("0x{}", "ff".repeat(32));
    assert!(matches!(
        CommittedBlockSource::open(&config),
        Err(CommittedRefusal::GenesisMismatch)
    ));
    assert!(CommittedBlockSource::open(&rpc.config).is_ok());
}

#[test]
fn finding_20_root_acquisition_preserves_receipt_authority() {
    let document = support::fixture();
    let height = header(&document).height;
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    for (first, last) in [(0, height - 1), (height + 1, height + 2)] {
        let mut config = rpc.config.clone();
        config.receipt_keys[0].first_height = first;
        config.receipt_keys[0].last_height = last;
        let source = CommittedBlockSource::open(&config).or_panic("root source");
        assert_eq!(
            source.acquire(height),
            Err(CommittedRefusal::ReceiptSignature)
        );
    }
    let mut config = rpc.config.clone();
    config.receipt_keys[0].public_key_hex = format!("0x{}", "ff".repeat(32));
    let source = CommittedBlockSource::open(&config).or_panic("root source");
    assert_eq!(
        source.acquire(height),
        Err(CommittedRefusal::ReceiptSignature)
    );

    let mut altered = document;
    let mut signed = support::bytes(&altered["receipts"][1]["receipt"]);
    *signed.last_mut().or_panic("signed receipt") ^= 1;
    altered["receipts"][1]["receipt"] = json!(format!("0x{}", hex_bytes_lower(&signed)));
    let rpc = support::TestRpc::new(vec![altered.clone(), altered]);
    let source = CommittedBlockSource::open(&rpc.config).or_panic("root source");
    assert_eq!(
        source.acquire(height),
        Err(CommittedRefusal::ReceiptSignature)
    );
}

#[test]
fn finding_20_root_acquisition_requires_independent_backend_agreement() {
    let document = support::fixture();
    let height = header(&document).height;
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    let mut config = rpc.config.rpc.clone();
    config.endpoints[1].independent_backend = config.endpoints[0].independent_backend.clone();
    assert!(matches!(
        RpcCluster::new(&config),
        Err(RpcError::Configuration)
    ));
    config = rpc.config.rpc.clone();
    config.endpoints[1].url = config.endpoints[0].url.clone();
    assert!(matches!(
        RpcCluster::new(&config),
        Err(RpcError::Configuration)
    ));
    assert!(rpc
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("requests: {error}"))
        .is_empty());

    let mut different = document.clone();
    different["committedBlock"]["timestamp"] = json!("0x1");
    let majority =
        support::TestRpc::new(vec![document.clone(), different.clone(), document.clone()]);
    let source = CommittedBlockSource::new(&majority.config).or_panic("root source");
    assert_eq!(
        source.acquire(height).map(|block| block.header),
        Ok(header(&document))
    );
    let divergent = support::TestRpc::new(vec![document.clone(), different]);
    let source = CommittedBlockSource::new(&divergent.config).or_panic("root source");
    assert_eq!(source.acquire(height), Err(CommittedRefusal::Divergence));

    let mut different = document.clone();
    different["receipts"][1]["rung"] = json!("receipted");
    let divergent = support::TestRpc::new(vec![document, different]);
    let source = CommittedBlockSource::new(&divergent.config).or_panic("root source");
    assert_eq!(source.acquire(height), Err(CommittedRefusal::Divergence));
}

fn hex_bytes_lower(data: &[u8]) -> String {
    let mut encoded = String::with_capacity(data.len() * 2);
    for byte in data {
        assert!(std::fmt::Write::write_fmt(&mut encoded, format_args!("{byte:02x}")).is_ok());
    }
    encoded
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
