use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

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
    let expected_did =
        result_assertion::required(std::str::from_utf8(did.as_bytes()), "session DID").to_owned();
    let expected_key: String = layerx_programs::hex::encode(&signer_key);
    let hash = format!("0x{}", "a1".repeat(32));
    let funding_hash = hash.clone();
    let listener = result_assertion::required(TcpListener::bind("127.0.0.1:0"), "bind faucet RPC");
    let node = format!(
        "http://{}",
        result_assertion::required(listener.local_addr(), "RPC address")
    );
    let responder = std::thread::spawn(move || {
        serve_funded_claim(
            &listener,
            &expected_did,
            &expected_key,
            &funding_hash,
            session_token,
        );
    });
    let result = result_assertion::required(
        execute(&mut server, 50, &node, &session),
        "faucet.request must send px_requestFunds and return its funding hash",
    );
    result_assertion::required(responder.join(), "faucet RPC assertions");
    assert_eq!(result, json!({"tx_hash": hash}));
    assert_eq!(server.audit_entries(), 2);
    let audit = result_assertion::required(
        layerx_agentd::audit::Log::open(&root, server.binding().tenant()),
        "persisted faucet audit",
    );
    assert_eq!(
        result_assertion::required(
            layerx_agentd::audit::verify_chain(audit.path()),
            "valid audit chain"
        )
        .entries,
        2
    );
    result_assertion::required(std::fs::remove_dir_all(root), "remove test session state");
}

#[test]
fn faucet_request_calls_px_request_funds_needs_a_session_token() {
    let listener = result_assertion::required(TcpListener::bind("127.0.0.1:0"), "bind faucet RPC");
    result_assertion::required(
        listener.set_nonblocking(true),
        "nonblocking faucet listener",
    );
    let node = format!(
        "http://{}",
        result_assertion::required(listener.local_addr(), "RPC address")
    );
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
    let listener = result_assertion::required(TcpListener::bind("127.0.0.1:0"), "bind faucet RPC");
    let node = format!(
        "http://{}",
        result_assertion::required(listener.local_addr(), "RPC address")
    );
    let responder = std::thread::spawn(move || serve_unfunded_claims(&listener, &hash));
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
    result_assertion::required(responder.join(), "faucet RPC assertions");
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
            code: -32_602,
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

fn serve_funded_claim(
    listener: &TcpListener,
    expected_did: &str,
    expected_key: &str,
    funding_hash: &str,
    session_token: &str,
) {
    let (mut stream, _) = result_assertion::required(listener.accept(), "accept faucet request");
    result_assertion::required(
        stream.set_read_timeout(Some(Duration::from_secs(5))),
        "request timeout",
    );
    result_assertion::required(
        stream.set_write_timeout(Some(Duration::from_secs(5))),
        "response timeout",
    );
    let mut reader = BufReader::new(&stream);
    let mut line = String::new();
    result_assertion::required(reader.read_line(&mut line), "request line");
    assert_eq!(line, "POST / HTTP/1.1\r\n");
    let mut length = None;
    let mut content_type = None;
    let mut authorization = None;
    loop {
        line.clear();
        assert!(result_assertion::required(reader.read_line(&mut line), "request header") > 0);
        if line == "\r\n" {
            break;
        }
        let (name, value) = option_assertion::required(
            option_assertion::required(line.strip_suffix("\r\n"), "header line ending")
                .split_once(':'),
            "HTTP header",
        );
        if name.eq_ignore_ascii_case("content-length") {
            assert!(length.is_none(), "one content length");
            length = Some(result_assertion::required(
                value.trim().parse::<usize>(),
                "body length",
            ));
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
    let length = option_assertion::required(length, "content length");
    assert!(length <= 4096);
    let mut body = vec![0; length];
    result_assertion::required(reader.read_exact(&mut body), "request body");
    let request: serde_json::Value =
        result_assertion::required(serde_json::from_slice(&body), "JSON-RPC request");
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
    result_assertion::required(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body), "funding response");
}

fn serve_unfunded_claims(listener: &TcpListener, hash: &str) {
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
        let (mut stream, _) =
            result_assertion::required(listener.accept(), "accept faucet request");
        result_assertion::required(
            stream.set_read_timeout(Some(Duration::from_secs(5))),
            "request timeout",
        );
        result_assertion::required(
            stream.set_write_timeout(Some(Duration::from_secs(5))),
            "response timeout",
        );
        let mut reader = BufReader::new(&stream);
        let mut line = String::new();
        result_assertion::required(reader.read_line(&mut line), "request line");
        assert_eq!(line, "POST / HTTP/1.1\r\n");
        let mut length = None;
        loop {
            line.clear();
            assert!(result_assertion::required(reader.read_line(&mut line), "request header") > 0);
            if line == "\r\n" {
                break;
            }
            let (name, value) =
                option_assertion::required(line.trim_end().split_once(':'), "HTTP header");
            if name.eq_ignore_ascii_case("content-length") {
                assert!(length.is_none(), "one content length");
                length = Some(result_assertion::required(
                    value.trim().parse::<usize>(),
                    "body length",
                ));
            }
        }
        let length = option_assertion::required(length, "content length");
        assert!(length <= 4096);
        let mut body = vec![0; length];
        result_assertion::required(reader.read_exact(&mut body), "request body");
        let request: serde_json::Value =
            result_assertion::required(serde_json::from_slice(&body), "JSON-RPC request");
        assert_eq!(request["method"], "px_requestFunds");
        let body = json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
        result_assertion::required(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body), "funding response");
    }
}
