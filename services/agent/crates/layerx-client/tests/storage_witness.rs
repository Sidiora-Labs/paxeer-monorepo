use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::time::Duration;

use base64::Engine as _;
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use layerx_client::storage_witness::{verify_storage_witness, StorageWitnessError};
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn decode(text: &str) -> Vec<u8> {
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
        .collect()
}

fn serve(responses: Vec<Value>, params: Value) -> (SequencerEndpoint, std::thread::JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let endpoint = must(SequencerEndpoint::new(
        &format!("http://{}/", must(listener.local_addr())),
        &[],
    ));
    let handle = std::thread::spawn(move || {
        for response in responses {
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
            assert_eq!(request["params"], params);
            let answer =
                json!({"jsonrpc":"2.0", "id":request["id"], "result":response}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
        }
    });
    (endpoint, handle)
}

#[test]
fn storage_witness_verifies_the_generated_vector_and_refuses_tampering() {
    let vector: Value = must(serde_json::from_str(include_str!(
        "../../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
    )));
    assert_eq!(
        vector["source"]["generator"],
        "modules/sequencer/keeper/ics23_evm_storage_vector_test.go::TestEvmStorageAppHashVector"
    );
    let entry = &vector["entries"][0];
    let address: [u8; 20] = must(decode(entry["address"].as_str().unwrap_or("")).try_into());
    let slot: [u8; 32] = must(decode(entry["slot"].as_str().unwrap_or("")).try_into());
    let app_hash: [u8; 32] = must(decode(entry["app_hash"].as_str().unwrap_or("")).try_into());
    let height = entry["height"].as_u64().unwrap_or_else(|| panic!("height"));
    let response = entry["response"].clone();
    // The canned answer is the JSON marshalled by the Go producer, including the CommitmentProof envelopes.
    let mut wrong_address = response.clone();
    wrong_address["address"] = json!("0x0000000000000000000000000000000000000000");
    let mut missing_value = response.clone();
    missing_value["hexValues"] = json!([]);
    let mut missing_proof = response.clone();
    missing_proof["storageProof"] = json!([]);
    let mut malformed_base64 = response.clone();
    malformed_base64["storageProof"][0]["ops"][0]["data"] = json!("!");
    let params = json!([
        format!("0x{}", entry["address"].as_str().unwrap_or("")),
        [format!("0x{}", entry["slot"].as_str().unwrap_or(""))],
        format!("{height:#x}")
    ]);
    let (endpoint, handle) = serve(
        vec![
            response.clone(),
            wrong_address,
            missing_value,
            missing_proof,
            malformed_base64,
        ],
        params,
    );
    let witnesses = must(endpoint.storage_witness(&address, &[slot], height));
    assert_eq!(witnesses.len(), 1);
    let witness = &witnesses[0];
    assert_eq!(witness.address, address);
    assert_eq!(witness.slot, slot);
    assert_eq!(
        witness.value.as_slice(),
        decode(entry["value"].as_str().unwrap_or(""))
    );
    assert_eq!(verify_storage_witness(witness, app_hash), Ok(()));
    let mut changed = witness.clone();
    changed.value[31] ^= 1;
    assert_eq!(
        verify_storage_witness(&changed, app_hash),
        Err(StorageWitnessError::Value)
    );
    let mut changed = witness.clone();
    changed.slot[31] ^= 1;
    assert_eq!(
        verify_storage_witness(&changed, app_hash),
        Err(StorageWitnessError::Key)
    );
    let mut changed = witness.clone();
    changed.address[0] ^= 1;
    assert_eq!(
        verify_storage_witness(&changed, app_hash),
        Err(StorageWitnessError::Key)
    );
    let mut changed_hash = app_hash;
    changed_hash[0] ^= 1;
    assert_eq!(
        verify_storage_witness(witness, changed_hash),
        Err(StorageWitnessError::AppHash)
    );
    let mut changed = witness.clone();
    changed.ops.swap(0, 1);
    assert_eq!(
        verify_storage_witness(&changed, app_hash),
        Err(StorageWitnessError::Operations)
    );
    let mut changed = witness.clone();
    let root = decode(entry["store_root"].as_str().unwrap_or(""));
    let offset = changed.ops[1]
        .data
        .windows(root.len())
        .position(|part| part == root)
        .unwrap_or_else(|| panic!("store root"));
    changed.ops[1].data[offset] ^= 1;
    assert_eq!(
        verify_storage_witness(&changed, app_hash),
        Err(StorageWitnessError::StoreRoot)
    );
    let mut changed = witness.clone();
    changed.ops[0].key[21] ^= 1;
    assert_eq!(
        verify_storage_witness(&changed, app_hash),
        Err(StorageWitnessError::Key)
    );
    for _ in 0..4 {
        assert_eq!(
            endpoint.storage_witness(&address, &[slot], height),
            Err(SequencerError::Malformed)
        );
    }
    must(handle.join());
    assert_eq!(
        must(
            base64::engine::general_purpose::STANDARD.decode(
                response["storageProof"][0]["ops"][0]["key"]
                    .as_str()
                    .unwrap_or("")
            )
        ),
        decode(entry["key"].as_str().unwrap_or(""))
    );
}
