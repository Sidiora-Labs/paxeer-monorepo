use std::fs;
use std::net::TcpListener;
use std::path::PathBuf;
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_platform_registry::{FileProgramStateJournal, ProgramStateCursor};
use layerx_programs::{hex, ProgramId};
use layerx_programs_runtime::crypto::hash::{hash_bytes, HashAlgorithm};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

struct Root(PathBuf);

impl Drop for Root {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn rpc_response(encoded: &[u8]) -> Vec<u8> {
    serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": 1,
        "result": format!("0x{}", hex::encode(encoded)),
    }))
    .unwrap()
}

#[test]
fn registry_reads_program_state_through_eth_call() {
    let manifest = include_str!("../Cargo.toml");
    assert!(!manifest.contains("layerx-programs-protocol-adapter"));
    let root = Root(std::env::temp_dir().join(format!(
        "registry-program-state-{}-{}",
        std::process::id(),
        SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_nanos(),
    )));
    let journal = FileProgramStateJournal::open(root.0.clone()).unwrap();
    let program = ProgramId::new([0x72; 32]).unwrap();
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let endpoint = format!("http://{}/rpc", listener.local_addr().unwrap());
    // Observe the actual client request; closing without an answer also proves
    // that a transport failure cannot publish cached program state.
    let observer = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(5);
        let mut stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error)
                    if error.kind() == std::io::ErrorKind::WouldBlock
                        && Instant::now() < deadline =>
                {
                    thread::sleep(Duration::from_millis(5));
                }
                Err(error) => panic!("program-state request observer: {error}"),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        let message = layerx_platform_internal::http::read_http_message(
            &mut stream,
            layerx_platform_internal::http::MAX_REQUEST_BYTES,
        )
        .unwrap();
        assert_eq!(message.headers.get("").unwrap(), "POST /rpc HTTP/1.1");
        serde_json::from_slice::<Value>(&message.body).unwrap()
    });
    assert!(journal
        .store(
            &endpoint,
            program,
            32,
            Instant::now() + Duration::from_secs(5)
        )
        .is_err());
    let request = observer.join().unwrap();
    let selector = hash_bytes(HashAlgorithm::Keccak256, b"getStateHead(bytes32)").unwrap();
    assert_eq!(request["jsonrpc"], "2.0");
    assert_eq!(request["id"], 1);
    assert_eq!(request["method"], "eth_call");
    assert_eq!(
        request["params"],
        json!([
            {
                "to": "0x000000000000000000000000000000000000101d",
                "data": format!("0x{}{}", hex::encode(&selector[..4]), hex::encode(&program.bytes())),
            },
            "0x20",
        ])
    );
    assert_eq!(fs::read_dir(&root.0).unwrap().count(), 0);
    assert_eq!(journal.cursor().unwrap(), ProgramStateCursor::default());

    // The native empty-storage manifest head, as emitted by getStateHead.
    let mut state = vec![0, 1, 0, 0, 0, 0];
    state.extend_from_slice(&Sha256::digest(&state));
    let mut encoded = vec![0; 128];
    encoded[31] = 32;
    encoded[63] = 38;
    encoded[64..102].copy_from_slice(&state);
    let response = rpc_response(&encoded);
    assert_eq!(journal.store_response(program, &response).unwrap(), state);
    let digest = Sha256::digest(&state);
    let record = root.0.join(format!(
        "{}.{}.program-state",
        hex::encode(&program.bytes()),
        hex::encode(&digest),
    ));
    assert_eq!(fs::read(&record).unwrap(), state);
    journal.audit().unwrap();
    assert_eq!(journal.cursor().unwrap(), ProgramStateCursor::default());

    for (index, value) in [
        (0, 1),
        (31, 64),
        (32, 1),
        (63, 37),
        (64, 1),
        (102, 1),
        (101, 0),
    ] {
        let mut invalid = encoded.clone();
        invalid[index] = value;
        assert!(journal
            .store_response(program, &rpc_response(&invalid))
            .is_err());
    }
    assert!(journal
        .store_response(program, &rpc_response(&encoded[..127]))
        .is_err());
    let mut trailing = encoded.clone();
    trailing.extend_from_slice(&[0; 32]);
    assert!(journal
        .store_response(program, &rpc_response(&trailing))
        .is_err());
    for invalid in [
        json!({"jsonrpc": "2.0", "id": 1, "error": {"code": 3, "message": "execution reverted"}}),
        json!({"jsonrpc": "2.0", "id": 2, "result": format!("0x{}", hex::encode(&encoded))}),
        json!({"jsonrpc": "1.0", "id": 1, "result": "0x"}),
        json!({"jsonrpc": "2.0", "id": 1, "result": "0xzz"}),
        json!({"jsonrpc": "2.0", "id": 1, "result": null}),
    ] {
        assert!(journal
            .store_response(program, &serde_json::to_vec(&invalid).unwrap())
            .is_err());
    }
    assert!(journal.store_response(program, b"{").is_err());
    assert!(journal
        .store(&endpoint, program, 0, Instant::now())
        .is_err());
    assert!(journal
        .store(&endpoint, program, 32, Instant::now())
        .is_err());
    assert!(journal
        .store("http://chain.example/rpc", program, 32, Instant::now())
        .is_err());
    assert_eq!(fs::read_dir(&root.0).unwrap().count(), 1);
    assert_eq!(fs::read(&record).unwrap(), state);
    assert_eq!(journal.cursor().unwrap(), ProgramStateCursor::default());
    fs::write(&record, b"corrupt").unwrap();
    assert!(journal.audit().is_err());
}
