use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::time::Duration;

use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_mcp::server::{tool_operation, InvocationOutcome, ToolKind, FAUCET_REQUEST};
use layerx_mcp::tools::faucet::{
    execute, invocation_outcome, request_funds, FaucetError, FaucetSession,
};
use serde_json::json;

#[path = "faucet.rs"]
mod faucet;

#[test]
fn faucet_request_calls_px_request_funds() {
    let root = std::env::temp_dir().join(format!("mcp-request-funds-{}", std::process::id()));
    let mut server = faucet::bind(&root, &[FAUCET_REQUEST.required_scope]);
    assert_eq!(server.tools(), [FAUCET_REQUEST]);
    assert_eq!(server.audit_entries(), 0);
    let did = server.binding().agent().clone();
    let signer = LocalSigner::new([0x42; 32]);
    let signer_key = signer.public_key();
    let session_token = "identity-session-token";
    let session = FaucetSession {
        did: &did,
        signer_key: &signer_key,
        session_token,
    };
    let expected_did = std::str::from_utf8(did.as_bytes())
        .expect("session DID")
        .to_owned();
    let expected_key: String = signer_key
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    let hash = format!("0x{}", "a1".repeat(32));
    let funding_hash = hash.clone();
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind faucet RPC");
    let node = format!("http://{}", listener.local_addr().expect("RPC address"));
    let responder = std::thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accept faucet request");
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("request timeout");
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .expect("response timeout");
        let mut reader = BufReader::new(&stream);
        let mut line = String::new();
        reader.read_line(&mut line).expect("request line");
        assert_eq!(line, "POST / HTTP/1.1\r\n");
        let mut length = None;
        let mut content_type = None;
        let mut authorization = None;
        loop {
            line.clear();
            assert!(reader.read_line(&mut line).expect("request header") > 0);
            if line == "\r\n" {
                break;
            }
            let (name, value) = line
                .strip_suffix("\r\n")
                .expect("header line ending")
                .split_once(':')
                .expect("HTTP header");
            if name.eq_ignore_ascii_case("content-length") {
                assert!(length.is_none(), "one content length");
                length = Some(value.trim().parse::<usize>().expect("body length"));
            }
            if name.eq_ignore_ascii_case("content-type") {
                content_type = Some(value.trim().to_owned());
            }
            if name.eq_ignore_ascii_case("authorization") {
                assert!(authorization.is_none(), "one authorization header");
                authorization = Some(value.to_owned());
            }
        }
        assert_eq!(content_type.as_deref(), Some("application/json"));
        assert_eq!(authorization, Some(format!(" Bearer {session_token}")));
        let length = length.expect("content length");
        assert!(length <= 4096);
        let mut body = vec![0; length];
        reader.read_exact(&mut body).expect("request body");
        let request: serde_json::Value = serde_json::from_slice(&body).expect("JSON-RPC request");
        assert_eq!(
            request,
            json!({
                "jsonrpc": "2.0",
                "id": 1,
                "method": "px_requestFunds",
                "params": [expected_did, expected_key],
            })
        );
        let body = json!({
            "jsonrpc": "2.0",
            "id": request["id"],
            "result": {
                "funded": true,
                "funding_id": "b2".repeat(32),
                "transaction_id": funding_hash,
                "amount": "1000",
                "network": "layerx-testnet",
                "starter_credit": {"grant_id": "starter-grant"},
            },
        })
        .to_string();
        write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).expect("funding response");
    });
    let result = execute(&mut server, 50, &node, &session)
        .expect("faucet.request must send px_requestFunds and return its funding hash");
    responder.join().expect("faucet RPC assertions");
    assert_eq!(result, json!({"tx_hash": hash}));
    assert_eq!(server.audit_entries(), 2);
    let audit = layerx_agentd::audit::Log::open(&root, server.binding().tenant())
        .expect("persisted faucet audit");
    assert_eq!(
        layerx_agentd::audit::verify_chain(audit.path())
            .expect("valid audit chain")
            .entries,
        2
    );
    std::fs::remove_dir_all(root).expect("remove test session state");
}

#[test]
fn faucet_request_calls_px_request_funds_needs_a_session_token() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind faucet RPC");
    listener
        .set_nonblocking(true)
        .expect("nonblocking faucet listener");
    let node = format!("http://{}", listener.local_addr().expect("RPC address"));
    let mut invalid_tokens = vec![
        String::new(),
        "session\r\nInjected: header".to_owned(),
        "a".repeat(4097),
    ];
    invalid_tokens.extend(
        (0_u8..=127)
            .filter(|byte| byte.is_ascii_control() || byte.is_ascii_whitespace())
            .map(|byte| format!("session{}token", char::from(byte))),
    );
    for token in invalid_tokens {
        let result = request_funds(&node, "did:layerx:model", &[0x42; 32], &token);
        assert!(
            matches!(result, Err(FaucetError::InvalidRequest)),
            "invalid token must be refused: {result:?}"
        );
        match listener.accept() {
            Err(error) => assert_eq!(error.kind(), std::io::ErrorKind::WouldBlock),
            Ok(_) => panic!("invalid token opened a connection"),
        }
    }
}

#[test]
fn faucet_request_calls_px_request_funds_refuses_unfunded_claims() {
    let hash = format!("0x{}", "a1".repeat(32));
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind faucet RPC");
    let node = format!("http://{}", listener.local_addr().expect("RPC address"));
    let responder = std::thread::spawn(move || {
        let results = [
            json!({
                "funded": false,
                "funding_id": "b2".repeat(32),
                "transaction_id": hash,
                "amount": "1000",
                "network": "layerx-testnet",
            }),
            json!({
                "funded": true,
                "funding_id": "b2".repeat(32),
                "amount": "1000",
                "network": "layerx-testnet",
            }),
            json!(hash),
        ];
        for result in results {
            let (mut stream, _) = listener.accept().expect("accept faucet request");
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("request timeout");
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .expect("response timeout");
            let mut reader = BufReader::new(&stream);
            let mut line = String::new();
            reader.read_line(&mut line).expect("request line");
            assert_eq!(line, "POST / HTTP/1.1\r\n");
            let mut length = None;
            loop {
                line.clear();
                assert!(reader.read_line(&mut line).expect("request header") > 0);
                if line == "\r\n" {
                    break;
                }
                let (name, value) = line.trim_end().split_once(':').expect("HTTP header");
                if name.eq_ignore_ascii_case("content-length") {
                    assert!(length.is_none(), "one content length");
                    length = Some(value.trim().parse::<usize>().expect("body length"));
                }
            }
            let length = length.expect("content length");
            assert!(length <= 4096);
            let mut body = vec![0; length];
            reader.read_exact(&mut body).expect("request body");
            let request: serde_json::Value =
                serde_json::from_slice(&body).expect("JSON-RPC request");
            assert_eq!(request["method"], "px_requestFunds");
            let body = json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).expect("funding response");
        }
    });
    for _ in 0..3 {
        let result = request_funds(
            &node,
            "did:layerx:model",
            &[0x42; 32],
            "identity-session-token",
        );
        assert!(
            matches!(result, Err(FaucetError::InvalidResponse)),
            "{result:?}"
        );
        assert_eq!(invocation_outcome(&result), InvocationOutcome::Unknown);
    }
    responder.join().expect("faucet RPC assertions");
    assert_eq!(
        invocation_outcome(&Err(FaucetError::InvalidResponse)),
        InvocationOutcome::Unknown
    );
    assert_eq!(
        invocation_outcome(&Err(FaucetError::Transport(std::io::Error::new(
            std::io::ErrorKind::TimedOut,
            "response timeout",
        )))),
        InvocationOutcome::Unknown
    );
    for error in [
        FaucetError::Rpc {
            code: -32602,
            message: "Faucet claim refused".to_owned(),
            data: None,
        },
        FaucetError::InvalidRequest,
        FaucetError::Endpoint,
        FaucetError::Http(403),
    ] {
        assert_eq!(invocation_outcome(&Err(error)), InvocationOutcome::Refused);
    }
    assert_eq!(
        invocation_outcome(&Ok("funding-transaction".to_owned())),
        InvocationOutcome::Completed
    );
}

#[test]
fn faucet_request_maps_to_no_agent_operation() {
    assert_eq!(tool_operation("faucet.request"), None);
    assert_eq!(FAUCET_REQUEST.required_scope, "write:faucet:claim");
    assert_eq!(FAUCET_REQUEST.kind, ToolKind::Write);
    assert!(FAUCET_REQUEST.mutation.contains("px_requestFunds"));
}
