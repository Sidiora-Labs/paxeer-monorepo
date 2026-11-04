//! Repair 22.31.357: probe-did-account-shape is proved by the bound, nullable,
//! unbound, empty, foreign, unavailable and malformed gateway cases below.
//! The production command and history producer run in-process over real HTTP.
//! Baseline verification was not run: the ordered gate instructions require
//! implementation first and prohibit running verify_cmd directly.
//! Attempt 1 could not execute these tests: compilation failed with E0425 on
//! the removed rpc_account::account call in the included production source.
//! Wiring request: spec/layerx-merge/spec.kvx (fm-platform) should reference
//! these proving tests and the gate's verified revision in the task record.

#[path = "../src/main.rs"]
mod probe;

use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread;
use std::time::{Duration, Instant};

fn did() -> String {
    format!("did:layerx:{}", "ab".repeat(32))
}

fn identities(address: &str, pax: &str, owner: &str, account: &str, bound: bool) -> String {
    format!(
        r#"{{"bound":{bound},"evm_address":{address},"layerx_account":{account},"layerx_did":{owner},"pax_address":{pax}}}"#
    )
}

fn bound_identities() -> String {
    identities(
        &format!(r#""0x{}""#, "12".repeat(20)),
        r#""pax1boundidentity""#,
        &format!(r#""{}""#, did()),
        &format!(r#""{}""#, "34".repeat(32)),
        true,
    )
}

fn account_result(identities: &str) -> String {
    format!(r#""result":{{"account":{identities},"paxeer":null}}"#)
}

fn probe_listing(status: u16, account_reply: String) -> Result<String, String> {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let url = format!("http://{}/", listener.local_addr().unwrap());
    let owner = did();
    let worker = thread::spawn(move || {
        let exchanges = [
            (
                "px_getNetwork",
                "[]".to_owned(),
                1,
                200,
                r#""result":{"network_id":"paxeer-125","paxeer":{"chain_id":125,"latest_block":42},"consensus":{"latest_height":42}}"#.to_owned(),
            ),
            ("eth_chainId", "[]".to_owned(), 1, 200, r#""result":"0x7d""#.to_owned()),
            ("px_getAccount", format!(r#"["{owner}"]"#), 2, status, account_reply),
        ];
        for (method, params, id, status, reply) in exchanges {
            let deadline = Instant::now() + Duration::from_secs(5);
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing {method} request");
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("gateway accept: {error}"),
                }
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
            let mut content_length = None;
            loop {
                line.clear();
                assert!(reader.read_line(&mut line).unwrap() > 0);
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        content_length = Some(value.trim().parse::<usize>().unwrap());
                    }
                }
            }
            let length = content_length.unwrap();
            assert!(length <= 4096);
            let mut body = vec![0; length];
            reader.read_exact(&mut body).unwrap();
            let request = String::from_utf8(body).unwrap();
            assert!(request.contains(r#""jsonrpc":"2.0""#), "{request}");
            assert!(request.contains(&format!(r#""id":{id},"#)), "{request}");
            assert!(
                request.contains(&format!(r#""method":"{method}""#)),
                "{request}"
            );
            assert!(
                request.contains(&format!(r#""params":{params}"#)),
                "{request}"
            );
            let body = format!(r#"{{"jsonrpc":"2.0","id":{id},{reply}}}"#);
            write!(stream, "HTTP/1.1 {status} fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap();
            stream.flush().unwrap();
        }
    });
    let parsed = BTreeMap::from([
        ("rpc".to_owned(), url),
        ("network-id".to_owned(), "125".to_owned()),
        ("did".to_owned(), did()),
    ]);
    let outcome = probe::did_accounts(&parsed);
    worker.join().unwrap();
    outcome
}

fn empty_listing() -> String {
    format!(r#"{{"did":"{}","count":0,"accounts":[]}}"#, did())
}

#[test]
fn probe_did_account_shape_bound_gateway_listing() {
    let account = bound_identities();
    assert_eq!(
        probe_listing(200, account_result(&account)),
        Ok(format!(
            r#"{{"did":"{}","count":1,"accounts":[{account}]}}"#,
            did()
        )),
    );
}

#[test]
fn probe_did_account_shape_nullable_and_escaped_fields() {
    let account = identities(
        &format!(r#""0x{}""#, "12".repeat(20)),
        "null",
        &format!(r#""{}""#, did()),
        "null",
        true,
    );
    let escaped = bound_identities().replace("pax1boundidentity", r#"pax1\"quoted\\address\n"#);
    for account in [account, escaped] {
        assert_eq!(
            probe_listing(200, account_result(&account)),
            Ok(format!(
                r#"{{"did":"{}","count":1,"accounts":[{account}]}}"#,
                did()
            )),
        );
    }
}

#[test]
fn probe_did_account_shape_unbound_and_empty_gateway_listings() {
    let unbound = bound_identities().replace(r#""bound":true"#, r#""bound":false"#);
    let empty = identities("null", "null", "null", "null", false);
    let missing_address =
        bound_identities().replace(&format!(r#""0x{}""#, "12".repeat(20)), "null");
    let foreign = bound_identities().replace(&did(), &format!("did:layerx:{}", "cd".repeat(32)));
    for account in [unbound, empty, missing_address, foreign] {
        assert_eq!(
            probe_listing(200, account_result(&account)),
            Ok(empty_listing())
        );
    }
}

#[test]
fn probe_did_account_shape_unavailable_is_an_error() {
    assert_eq!(
        probe_listing(503, r#""result":null"#.to_owned()),
        Err("did account listing failed: Unavailable".to_owned()),
    );
}

#[test]
fn probe_did_account_shape_gateway_refusal_is_preserved() {
    let outcome = probe_listing(200, r#""error":{"code":-32001,"message":"Read unavailable","data":{"code":"identity_binding_unavailable"}}"#.to_owned());
    let error = outcome.unwrap_err();
    assert!(
        error.starts_with("did account listing failed: Rpc("),
        "{error}"
    );
    assert!(error.contains("-32001"), "{error}");
    assert!(error.contains("Read unavailable"), "{error}");
    assert!(error.contains("identity_binding_unavailable"), "{error}");
}

#[test]
fn probe_did_account_shape_malformed_native_fields_are_rejected() {
    let invalid_address = bound_identities().replace(
        &format!(r#""0x{}""#, "12".repeat(20)),
        r#""not-an-address""#,
    );
    let invalid_bound = bound_identities().replace(r#""bound":true"#, r#""bound":"true""#);
    for account in [invalid_address, invalid_bound] {
        assert_eq!(
            probe_listing(200, account_result(&account)),
            Err("did account listing failed: Malformed".to_owned()),
        );
    }
}
