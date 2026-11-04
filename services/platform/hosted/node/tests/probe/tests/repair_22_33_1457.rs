//! probe-retired-anchor: run the production custody commands and core native
//! readers through a loopback RPC peer with committed ABI codec vectors.

#[path = "../src/main.rs"]
mod probe;
#[path = "../src/repair_22_33_1457_vectors.rs"]
mod vectors;

use layerx_platform_core::{hex_encode, treasury_did, TreasurySigner as _};
use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{DirBuilderExt, PermissionsExt};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex, OnceLock};
use std::thread;
use std::time::Duration;

const ASSET: [u8; 32] = [0x42; 32];
const SEED: [u8; 32] = [0x73; 32];

fn evm_address(seed: &[u8; 32]) -> String {
    let signer = layerx_platform_core::SeedSigner::new(seed);
    format!(
        "0x{}",
        hex_encode(&signer.transaction_signer().evm_address().unwrap())
    )
}

#[derive(Clone, Copy, PartialEq)]
enum Fault {
    None,
    Nonce,
    Height,
    HeightWithoutActors,
    Root,
    Binding,
    Unbound,
    Name,
    Pointer,
    Decimals,
    Supply,
    Chain,
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "probe-22-33-1457-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::DirBuilder::new()
            .mode(0o700)
            .create(&path)
            .unwrap();
        Self(path)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).unwrap();
    }
}

struct RpcPeer {
    url: String,
    requests: Arc<Mutex<Vec<String>>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl RpcPeer {
    fn new(fault: Fault) -> Self {
        // Provision the production reader's explicit binding inside this test
        // process once; the gate needs no caller-supplied environment setting.
        static BINDINGS: OnceLock<()> = OnceLock::new();
        BINDINGS.get_or_init(|| {
            std::env::set_var(
                "LAYERX_CORE_NATIVE_ASSET_DENOMS",
                format!(r#"{{"{}":"uhpx"}}"#, hex_encode(&ASSET)),
            );
        });
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let url = format!("http://{}/", listener.local_addr().unwrap());
        let requests = Arc::new(Mutex::new(Vec::new()));
        let recorded = Arc::clone(&requests);
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let source_address = evm_address(&SEED);
            let destination_address = evm_address(&[0x74; 32]);
            let mut network_reads = 0;
            let mut nonce_reads = 0;
            let mut binding_reads = 0;
            let mut block_reads = 0;
            while !shutdown.load(Ordering::Relaxed) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("RPC peer accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut reader = BufReader::new(&mut stream);
                let mut line = String::new();
                reader.read_line(&mut line).unwrap();
                assert_eq!(line, "POST / HTTP/1.1\r\n");
                let mut length = None;
                loop {
                    line.clear();
                    assert!(reader.read_line(&mut line).unwrap() > 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(value.trim().parse::<usize>().unwrap());
                        }
                    }
                }
                let length = length.unwrap();
                assert!(length <= 8192);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).unwrap();
                let body = String::from_utf8(body).unwrap();
                recorded.lock().unwrap().push(body.clone());
                let id = body
                    .split_once(r#""id":"#)
                    .unwrap()
                    .1
                    .split(',')
                    .next()
                    .unwrap();
                assert!(id.bytes().all(|byte| byte.is_ascii_digit()));
                let method = body
                    .split_once(r#""method":""#)
                    .unwrap()
                    .1
                    .split('"')
                    .next()
                    .unwrap();
                let params = body
                    .split_once(r#""params":"#)
                    .unwrap()
                    .1
                    .strip_suffix('}')
                    .unwrap();
                let result = match method {
                    "eth_chainId" => {
                        assert_eq!(params, "[]");
                        if fault == Fault::Chain {
                            r#""0x7e""#.into()
                        } else {
                            r#""0x7d""#.into()
                        }
                    }
                    "px_getNetwork" => {
                        assert_eq!(params, "[]");
                        network_reads += 1;
                        let height = if (fault == Fault::Height && network_reads > 2)
                            || (fault == Fault::HeightWithoutActors && network_reads > 1)
                        {
                            2
                        } else {
                            1
                        };
                        // The EVM head is ahead: only consensus.latest_height
                        // is the native committed height used by the command.
                        format!(
                            r#"{{"network_id":"paxeer-125","paxeer":{{"chain_id":125,"latest_block":99}},"consensus":{{"latest_height":{height}}}}}"#
                        )
                    }
                    "px_getAccount" => {
                        binding_reads += 1;
                        let source = treasury_did(&SEED);
                        let destination = treasury_did(&[0x74; 32]);
                        let (did, address) = if params == format!(r#"["{source}"]"#) {
                            (source, &source_address)
                        } else {
                            assert_eq!(params, format!(r#"["{destination}"]"#));
                            (destination, &destination_address)
                        };
                        let address = if fault == Fault::Binding && binding_reads > 1 {
                            "0x2222222222222222222222222222222222222222"
                        } else {
                            address
                        };
                        let bound = fault != Fault::Unbound;
                        format!(
                            r#"{{"account":{{"bound":{bound},"evm_address":"{address}","pax_address":null,"layerx_did":"{did}","layerx_account":null}},"paxeer":null}}"#
                        )
                    }
                    "eth_getTransactionCount" => {
                        nonce_reads += 1;
                        let address = if fault == Fault::Binding && binding_reads > 1 {
                            "0x2222222222222222222222222222222222222222"
                        } else if params == format!(r#"["{destination_address}","pending"]"#) {
                            &destination_address
                        } else {
                            &source_address
                        };
                        assert_eq!(params, format!(r#"["{address}","pending"]"#));
                        if fault == Fault::Nonce && nonce_reads > 1 {
                            r#""0x8""#.into()
                        } else {
                            r#""0x7""#.into()
                        }
                    }
                    "eth_getBlockByNumber" => {
                        assert!(
                            matches!(params, r#"["0x1",true]"# | r#"["0x1",false]"#),
                            "{body}"
                        );
                        block_reads += 1;
                        if fault == Fault::Root && block_reads >= 4 {
                            vectors::BLOCK.replace(vectors::APP_HASH, &"91".repeat(32))
                        } else {
                            vectors::BLOCK.to_owned()
                        }
                    }
                    "eth_call" => {
                        let (index, (_, _, reply)) = vectors::CALLS
                            .iter()
                            .enumerate()
                            .find(|(_, (to, data, _))| {
                                params == format!(r#"[{{"data":"{data}","to":"{to}"}},"0x1"]"#)
                            })
                            .unwrap_or_else(|| panic!("unexpected native call: {body}"));
                        let reply = match (fault, index) {
                            (Fault::Name, 0) => "0x",
                            (Fault::Decimals, 2) => vectors::INVALID_DECIMALS,
                            (Fault::Supply, 3) => &reply[..reply.len() - 2],
                            (Fault::Pointer, 4) => vectors::NO_POINTER,
                            _ => reply,
                        };
                        format!(r#""{reply}""#)
                    }
                    _ => panic!("unexpected RPC method: {body}"),
                };
                let response = format!(r#"{{"jsonrpc":"2.0","id":{id},"result":{result}}}"#);
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()).unwrap();
                stream.flush().unwrap();
            }
        });
        Self {
            url,
            requests,
            stop,
            worker: Some(worker),
        }
    }

    fn options(&self, directory: &Directory) -> BTreeMap<String, String> {
        let anchor = directory.0.join("ca.der");
        std::fs::write(&anchor, vectors::TRUST_ANCHOR).unwrap();
        let seed = directory.0.join("seed");
        std::fs::write(&seed, SEED).unwrap();
        BTreeMap::from([
            ("rpc".into(), self.url.clone()),
            ("trust-anchor".into(), anchor.to_str().unwrap().into()),
            ("network-id".into(), "125".into()),
            ("asset".into(), hex_encode(&ASSET)),
            ("source-did".into(), treasury_did(&SEED)),
            ("destination-did".into(), treasury_did(&[0x74; 32])),
            ("seed-file".into(), seed.to_str().unwrap().into()),
            ("amount".into(), "3".into()),
            (
                "output".into(),
                directory.0.join("send").to_str().unwrap().into(),
            ),
        ])
    }

    fn assert_native_calls(&self) {
        let requests = self.requests.lock().unwrap();
        for (target, calldata, _) in vectors::CALLS {
            assert!(
                requests
                    .iter()
                    .any(|request| request.contains(target) && request.contains(calldata)),
                "missing {target} {calldata}"
            );
        }
        for request in requests.iter() {
            assert!(!request.contains("0x0000000000000000000000000000000000001013"));
            assert!(!request.contains("0x0000000000000000000000000000000000001014"));
            assert!(!request.contains("lx_"));
        }
    }
}

impl Drop for RpcPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Relaxed);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !thread::panicking() {
                result.unwrap();
            }
        }
    }
}

fn account(did: &str) -> (String, String) {
    let name = format!("agent:{did}:asset:{}", hex_encode(&ASSET));
    let parsed = layerx_types::account::AccountId::parse(&name).unwrap();
    let id = layerx_wire::hash::account_id_for_protocol(
        &parsed,
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .unwrap();
    (name, hex_encode(&id))
}

#[test]
fn probe_retired_anchor_profile_reports_native_committed_evidence() {
    let directory = Directory::new();
    let peer = RpcPeer::new(Fault::None);
    let mut options = peer.options(&directory);
    let path = directory.0.join("profile");
    options.insert("profile-output".into(), path.to_str().unwrap().into());
    let output = probe::custody_asset_profile(&options).unwrap();
    let (source, source_id) = account(&treasury_did(&SEED));
    let (destination, destination_id) = account(&treasury_did(&[0x74; 32]));
    assert_eq!(
        output,
        format!(
            r#"{{"asset":"{}","profile_sha256":"{}","profile_hex":"{}","trusted_height":1,"trust_source":"authenticated_native_bank_pointer","head_sequence":1,"global_sequence":1,"committed_height":1,"block_hash":"{}","app_hash":"{}","achieved":"COMMITTED","source_account":"{source}","source_account_id":"{source_id}","destination_account":"{destination}","destination_account_id":"{destination_id}","profile_path":"{}"}}"#,
            hex_encode(&ASSET),
            vectors::PROFILE_HASH,
            hex_encode(vectors::PROFILE.as_bytes()),
            vectors::BLOCK_HASH,
            vectors::APP_HASH,
            path.display()
        )
    );
    assert_eq!(std::fs::read(&path).unwrap(), vectors::PROFILE.as_bytes());
    assert_eq!(
        std::fs::metadata(path).unwrap().permissions().mode() & 0o777,
        0o600
    );
    peer.assert_native_calls();
}

#[test]
fn probe_retired_anchor_profile_without_actors_still_checks_native_snapshot() {
    for fault in [Fault::None, Fault::Root, Fault::HeightWithoutActors] {
        let directory = Directory::new();
        let peer = RpcPeer::new(fault);
        let mut options = peer.options(&directory);
        options.remove("source-did");
        options.remove("destination-did");
        let result = probe::custody_asset_profile(&options);
        if fault == Fault::None {
            let output = result.unwrap();
            assert!(output.contains(r#""committed_height":1"#));
            assert!(!output.contains("batch_number"));
            peer.assert_native_calls();
        } else if fault == Fault::Root {
            assert_eq!(result, Err("native committed block changed".into()));
        } else {
            assert_eq!(
                result,
                Err("custody preparation changed during profile reads; refresh required".into())
            );
        }
        assert!(!peer
            .requests
            .lock()
            .unwrap()
            .iter()
            .any(|request| request.contains("eth_getTransactionCount")));
    }
}

#[test]
fn probe_retired_anchor_send_uses_native_profile_and_validated_nonce() {
    let directory = Directory::new();
    let peer = RpcPeer::new(Fault::None);
    let options = peer.options(&directory);
    let activity = probe::write_asset_send(&options).unwrap();
    let endpoint = layerx_client::sequencer::SequencerEndpoint::new(&peer.url, &[]).unwrap();
    let network = layerx_client::px::PxEndpoint::new(&peer.url, &[]).unwrap();
    let selected = layerx_platform_core::read_custody_asset(
        &endpoint,
        &network,
        125,
        1,
        ASSET,
        &treasury_did(&SEED),
    )
    .unwrap();
    assert_eq!(selected.profile(), vectors::PROFILE.as_bytes());
    assert_eq!(selected.trusted_height(), 1);
    assert_eq!(selected.account_sequence(), 7);
    let timestamp = vectors::TIMESTAMP * 1000;
    let signed = layerx_platform_core::build_asset_send_with_identity_sequence(
        &SEED,
        7,
        &layerx_platform_core::SendRequest {
            network_id: 125,
            source_did: treasury_did(&SEED),
            destination_did: treasury_did(&[0x74; 32]),
            asset: ASSET,
            amount: 3,
            account_sequence: 7,
            idempotency_key: [0x62; 32],
            not_before_ms: timestamp,
            expires_at_ms: timestamp + 300_000,
            fee_limit: 1000,
        },
        &selected,
    )
    .unwrap();
    assert_eq!(activity, hex_encode(&signed.activity_id));
    let path = PathBuf::from(&options["output"]);
    assert_eq!(std::fs::read(&path).unwrap(), signed.canonical);
    assert_eq!(
        std::fs::metadata(&path).unwrap().permissions().mode() & 0o777,
        0o600
    );
    assert_eq!(
        hex_encode(&signed.source_account),
        account(&treasury_did(&SEED)).1
    );
    assert_eq!(
        hex_encode(&signed.destination_account),
        account(&treasury_did(&[0x74; 32])).1
    );
    assert!(probe::write_asset_send(&options)
        .unwrap_err()
        .contains("File exists"));
    assert_eq!(std::fs::read(&path).unwrap(), signed.canonical);
    peer.assert_native_calls();
}

#[test]
fn probe_retired_anchor_changed_nonce_height_or_binding_refuses_output() {
    for fault in [Fault::Nonce, Fault::Height, Fault::Binding] {
        for send in [false, true] {
            let directory = Directory::new();
            let peer = RpcPeer::new(fault);
            let mut options = peer.options(&directory);
            // A single actor ensures a changed binding is compared across
            // preparation reads of that same actor.
            if !send {
                options.remove("destination-did");
            }
            let profile = directory.0.join("profile");
            options.insert("profile-output".into(), profile.to_str().unwrap().into());
            let result = if send {
                probe::write_asset_send(&options)
            } else {
                probe::custody_asset_profile(&options)
            };
            assert!(result.unwrap_err().contains("refresh required"));
            assert!(!profile.exists());
            assert!(!PathBuf::from(&options["output"]).exists());
        }
    }
}

#[test]
fn probe_retired_anchor_native_metadata_validation_is_preserved() {
    for (fault, expected) in [
        (Fault::Name, "truncated custody ABI word"),
        (Fault::Pointer, "native asset pointer unavailable"),
        (Fault::Decimals, "invalid native decimals"),
        (Fault::Supply, "invalid native supply"),
    ] {
        for send in [false, true] {
            let directory = Directory::new();
            let peer = RpcPeer::new(fault);
            let options = peer.options(&directory);
            let result = if send {
                probe::write_asset_send(&options)
            } else {
                probe::custody_asset_profile(&options)
            };
            assert_eq!(result, Err(expected.into()));
            assert!(!PathBuf::from(&options["output"]).exists());
        }
    }
}

#[test]
fn probe_retired_anchor_wrong_network_or_unbound_actor_is_refused() {
    for (fault, expected) in [
        (Fault::Chain, "network id mismatch"),
        (Fault::Unbound, "custody DID has no bound evm_address"),
    ] {
        for send in [false, true] {
            let directory = Directory::new();
            let peer = RpcPeer::new(fault);
            let options = peer.options(&directory);
            let result = if send {
                probe::write_asset_send(&options)
            } else {
                probe::custody_asset_profile(&options)
            };
            assert_eq!(result, Err(expected.into()));
            assert!(!PathBuf::from(&options["output"]).exists());
        }
    }
}

#[test]
fn probe_retired_anchor_send_rejects_owner_substitution_before_rpc() {
    let directory = Directory::new();
    let peer = RpcPeer::new(Fault::None);
    let mut options = peer.options(&directory);
    options.insert("source-did".into(), treasury_did(&[0x74; 32]));
    assert_eq!(
        probe::write_asset_send(&options),
        Err("source DID does not match the seed owner".into())
    );
    assert!(peer.requests.lock().unwrap().is_empty());
    assert!(!PathBuf::from(&options["output"]).exists());
}
