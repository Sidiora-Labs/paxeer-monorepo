use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::evidence::{AccountEvidencePolicy, RootSelector, SignedHeader};
use layerx_client::handover::SequencerHistory;
use layerx_client::rpc::module_state::{
    module_state, module_state_with_history, verify_module_evidence_with_history,
    ModuleReadContext, ModuleReadError,
};
use layerx_client::sequencer::{AccountProof, SequencerEndpoint, StorageProof};
use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use layerx_types::verify::VerificationLevel;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::batch_header_digest;
use serde_json::{json, Value};

const PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x07,
];
const HEIGHT: u64 = 0x1234;
const MODULE: u16 = 7;
const KEY: &[u8] = b"module-key";

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    bytes.iter().fold(String::from("0x"), |mut text, byte| {
        must(write!(text, "{byte:02x}"));
        text
    })
}

fn rlp(payload: &[u8], list: bool) -> Vec<u8> {
    if !list && payload.len() == 1 && payload[0] < 0x80 {
        return payload.to_vec();
    }
    let base = if list { 0xc0 } else { 0x80 };
    let mut result = Vec::new();
    if payload.len() < 56 {
        result.push(base + must(u8::try_from(payload.len())));
    } else {
        let length = payload.len().to_be_bytes();
        let start = length
            .iter()
            .position(|byte| *byte != 0)
            .unwrap_or(length.len());
        result.push(base + 55 + must(u8::try_from(length.len() - start)));
        result.extend_from_slice(&length[start..]);
    }
    result.extend_from_slice(payload);
    result
}

fn list(items: &[Vec<u8>]) -> Vec<u8> {
    rlp(&items.concat(), true)
}

fn leaf(path: &[u8; 32], value: &[u8]) -> Vec<u8> {
    let mut compact = vec![0x20];
    compact.extend_from_slice(path);
    list(&[rlp(&compact, false), rlp(value, false)])
}

fn slot() -> [u8; 32] {
    // The module identifier is a uint16 in network byte order, without ABI padding.
    keccak256(&[&[0, 7][..], KEY].concat())
}

fn signed_header(root: [u8; 32]) -> SignedHeader {
    let key = SigningKey::from_bytes(&[0x41; 32]);
    let public = key.verifying_key().to_bytes();
    let id = must(layerx_wire::handover::sequencer_id(&public));
    let mut encoder = Encoder::new(354);
    must(encoder.structure_header_version(0x1701, 3));
    must(encoder.u8(15));
    must(encoder.tag(1, 15));
    must(encoder.u16(3));
    must(encoder.tag(2, 15));
    must(encoder.u32(77));
    for (tag, value) in [(3, 1), (4, HEIGHT), (5, 10), (6, 12)] {
        must(encoder.tag(tag, 15));
        must(encoder.u64(value));
    }
    for (tag, value) in [
        (7, [1; 32]),
        (8, root),
        (9, [3; 32]),
        (10, [4; 32]),
        (11, [5; 32]),
        (12, [6; 32]),
        (13, [7; 32]),
    ] {
        must(encoder.tag(tag, 15));
        must(encoder.bytes(&value, 32));
    }
    must(encoder.tag(14, 15));
    must(encoder.u64(1_000));
    must(encoder.tag(15, 15));
    must(encoder.bytes(&id, 32));
    let canonical_bytes = encoder.finish();
    let signature = key
        .sign(&must(batch_header_digest(&canonical_bytes)))
        .to_bytes();
    SignedHeader {
        sequencer_id: id,
        public_key: public,
        first_batch_number: 1,
        last_batch_number: HEIGHT,
        canonical_bytes,
        signature,
    }
}

fn context(header: &SignedHeader) -> ModuleReadContext<'_> {
    ModuleReadContext {
        precompile: PRECOMPILE,
        committed_height: HEIGHT,
        signed_header: header,
        policy: AccountEvidencePolicy {
            expected_protocol_version: 3,
            expected_network_id: 77,
            handshake_sequencer_key: header.public_key,
            root_selector: RootSelector::Batch(HEIGHT),
        },
    }
}

fn account(storage: StorageProof, storage_hash: [u8; 32]) -> (AccountProof, SignedHeader) {
    let code_hash = keccak256(&[]);
    let account_value = list(&[
        rlp(&[1], false),
        rlp(&[0x12, 0x34], false),
        rlp(&storage_hash, false),
        rlp(&code_hash, false),
    ]);
    let account_node = leaf(&keccak256(&PRECOMPILE), &account_value);
    let header = signed_header(keccak256(&account_node));
    let mut balance = [0; 32];
    balance[30..].copy_from_slice(&[0x12, 0x34]);
    (
        AccountProof {
            address: PRECOMPILE,
            balance,
            nonce: 1,
            code_hash,
            storage_hash,
            account_proof: vec![account_node],
            storage: vec![storage],
        },
        header,
    )
}

fn fixture(key: [u8; 32], quantity: &[u8]) -> (AccountProof, SignedHeader) {
    let storage_node = leaf(&keccak256(&key), &rlp(quantity, false));
    let mut value = [0; 32];
    value[32 - quantity.len()..].copy_from_slice(quantity);
    account(
        StorageProof {
            key,
            value,
            proof: vec![storage_node.clone()],
        },
        keccak256(&storage_node),
    )
}

fn quantity(bytes: &[u8; 32]) -> String {
    let text = hex(bytes);
    let digits = text[2..].trim_start_matches('0');
    if digits.is_empty() {
        String::from("0x0")
    } else {
        format!("0x{digits}")
    }
}

fn branched_fixture() -> (AccountProof, SignedHeader) {
    let path = keccak256(&slot());
    let mut compact = vec![0x30 | (path[1] & 15)];
    compact.extend_from_slice(&path[2..]);
    let target = list(&[rlp(&compact, false), rlp(&rlp(&[0x80, 0xab], false), false)]);
    let sibling = list(&[rlp(&compact, false), rlp(&rlp(&[0x2a], false), false)]);
    let mut children = vec![rlp(&[], false); 17];
    children[usize::from(path[1] >> 4)] = rlp(&keccak256(&target), false);
    children[usize::from((path[1] >> 4) ^ 1)] = rlp(&keccak256(&sibling), false);
    let branch = list(&children);
    let extension = list(&[rlp(&[0, path[0]], false), rlp(&keccak256(&branch), false)]);
    let mut value = [0; 32];
    value[30..].copy_from_slice(&[0x80, 0xab]);
    account(
        StorageProof {
            key: slot(),
            value,
            proof: vec![extension.clone(), branch, target],
        },
        keccak256(&extension),
    )
}

fn response(proof: &AccountProof) -> Value {
    json!({
        "address": hex(&proof.address),
        "balance": quantity(&proof.balance),
        "nonce": format!("{:#x}", proof.nonce),
        "codeHash": hex(&proof.code_hash),
        "storageHash": hex(&proof.storage_hash),
        "accountProof": proof.account_proof.iter().map(|node| hex(node)).collect::<Vec<_>>(),
        "storageProof": proof.storage.iter().map(|storage| json!({
            "key": hex(&storage.key),
            "value": quantity(&storage.value),
            "proof": storage.proof.iter().map(|node| hex(node)).collect::<Vec<_>>()
        })).collect::<Vec<_>>()
    })
}

fn serve(proofs: Vec<AccountProof>) -> (SequencerEndpoint, JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let endpoint = must(SequencerEndpoint::new(
        &format!("http://{}/", must(listener.local_addr())),
        &[],
    ));
    let handle = std::thread::spawn(move || {
        for proof in proofs {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
            let mut length = 0;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = must(value.trim().parse::<usize>());
                }
            }
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], "eth_getProof");
            assert_eq!(
                request["params"],
                json!([hex(&PRECOMPILE), [hex(&slot())], "0x1234"])
            );
            let answer = json!({"jsonrpc": "2.0", "id": request["id"], "result": response(&proof)})
                .to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
        }
    });
    (endpoint, handle)
}

#[test]
fn module_state_reads_the_storage_slot_proof_at_the_committed_height() {
    let (proof, header) = fixture(slot(), &[0x80, 0xab]);
    let mut wrong_value = proof.clone();
    wrong_value.storage[0].value[31] ^= 1;
    let mut wrong_node = proof.clone();
    wrong_node.storage[0].proof[0][10] ^= 1;
    let mut wrong_root = proof.clone();
    wrong_root.storage_hash[0] ^= 1;
    let mut wrong_account = proof.clone();
    wrong_account.account_proof[0][10] ^= 1;
    let mut missing = proof.clone();
    missing.storage[0].proof.clear();
    let mut wrong_balance = proof.clone();
    wrong_balance.balance[31] ^= 1;
    let mut wrong_nonce = proof.clone();
    wrong_nonce.nonce = 2;
    let mut duplicate = proof.clone();
    duplicate.storage.push(proof.storage[0].clone());
    let (endpoint, handle) = serve(vec![
        proof.clone(),
        wrong_value,
        wrong_node,
        wrong_root,
        wrong_account,
        missing,
        wrong_balance,
        wrong_nonce,
        duplicate,
    ]);
    let read = must(module_state(&endpoint, MODULE, KEY, context(&header)));
    assert_eq!(read.canonical_bytes(), &proof.storage[0].value);
    assert_eq!(&read.canonical_bytes()[30..], &[0x80, 0xab]);
    assert_eq!(read.proof(), &proof);
    assert_eq!(read.evidence().level(), VerificationLevel::STATE_PROVEN);
    assert_eq!(
        read.evidence().state_root(),
        keccak256(&proof.account_proof[0])
    );
    assert_eq!(read.evidence().signed_header(), &header);
    assert_eq!(read.evidence().checkpoint_id(), None);
    for _ in 0..7 {
        assert_eq!(
            module_state(&endpoint, MODULE, KEY, context(&header)).err(),
            Some(ModuleReadError::InvalidProof)
        );
    }
    assert_eq!(
        module_state(&endpoint, MODULE, KEY, context(&header)).err(),
        Some(ModuleReadError::SelectorMismatch)
    );
    must(handle.join());

    let (branched, branched_header) = branched_fixture();
    let (endpoint, handle) = serve(vec![branched.clone()]);
    assert_eq!(
        must(module_state(
            &endpoint,
            MODULE,
            KEY,
            context(&branched_header)
        ))
        .canonical_bytes(),
        &branched.storage[0].value
    );
    must(handle.join());

    // A proven empty trie returns the zero storage word, including leading zeroes.
    let (zero, zero_header) = account(
        StorageProof {
            key: slot(),
            value: [0; 32],
            proof: vec![],
        },
        keccak256(&[0x80]),
    );
    let (endpoint, handle) = serve(vec![zero]);
    assert_eq!(
        must(module_state(&endpoint, MODULE, KEY, context(&zero_header))).canonical_bytes(),
        &[0; 32]
    );
    must(handle.join());

    let (endpoint, handle) = serve(vec![proof.clone(), proof.clone()]);
    let mut bad_signature = header.clone();
    bad_signature.signature[0] ^= 1;
    assert!(matches!(
        module_state(&endpoint, MODULE, KEY, context(&bad_signature)),
        Err(ModuleReadError::Header(_))
    ));
    let mut wrong_height = context(&header);
    wrong_height.policy.root_selector = RootSelector::Batch(HEIGHT + 1);
    assert_eq!(
        module_state(&endpoint, MODULE, KEY, wrong_height).err(),
        Some(ModuleReadError::SelectorMismatch)
    );
    must(handle.join());
}

#[test]
fn module_state_with_history_refuses_a_storage_proof_for_another_slot() {
    let genesis = include_bytes!("../../layerx-proof/tests/fixtures/signed-authority/genesis.bin");
    let material = must(layerx_wire::handover::decode_genesis_trust(genesis));
    let history = must(SequencerHistory::from_genesis_artifact(
        genesis,
        material.network_id,
        material.canonical_state_root,
        material.initial_sequencer_key,
    ));
    let other_slot = keccak256(&[&[0, 7][..], b"another-key"].concat());
    assert_ne!(other_slot, slot());
    let (wrong, header) = fixture(other_slot, &[0x2a]);
    let (endpoint, handle) = serve(vec![wrong.clone()]);
    assert_eq!(
        module_state_with_history(&endpoint, MODULE, KEY, context(&header), &history).err(),
        Some(ModuleReadError::SelectorMismatch),
    );
    must(handle.join());
    assert_eq!(
        verify_module_evidence_with_history(&wrong, MODULE, KEY, context(&header), &history).err(),
        Some(ModuleReadError::SelectorMismatch)
    );
    // This history has not authenticated the target height; a matching slot must
    // fail on authority, demonstrating that the refusal above was slot-specific.
    let (correct, header) = fixture(slot(), &[0x2a]);
    assert_eq!(
        verify_module_evidence_with_history(&correct, MODULE, KEY, context(&header), &history)
            .err(),
        Some(ModuleReadError::History)
    );
}
