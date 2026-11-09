//! Proves: route_request / native_read_response -> HttpPool / response / TCP HTTP -> proof::decode_response.
//!
//! This file is also the host's shared HTTP module: the proof compiles the same
//! listener, request parser, error producers and writer without the host-only MCP
//! dependency graph. No transport is replaced. The error anchor has no durable
//! state; each assertion makes two independent connections. Native error mapping
//! takes the real NativeReadError variants, including ResultTooLarge, rather than
//! constructing wire error messages. Malformed-result checks corrupt bytes of an
//! actual producer response at the consumer's decoding boundary.
//!
//! No runtime defect is repaired: this extracts the existing path without changing
//! its error codes or status semantics.

#[path = "proof_native_reads.rs"]
mod daemon;

pub(crate) use daemon::{
    parse_request, response, serve_read_routes, DeadlineStream, HttpPool, Parsed, HEADER_LIMIT,
    REQUEST_DEADLINE, SUPERVISION_INTERVAL,
};

#[cfg(test)]
mod proof {
    use super::daemon::native_read_response;
    use layerx_agentd::node_rpc::NodeRpcConfig;
    use layerx_agentd::read::{NativeReadError, ProgramBalanceReadRoute};
    use layerx_client::sequencer::{RpcRefusal, SequencerError};
    use std::io::{Read, Write};
    use std::net::{TcpListener, TcpStream};
    use std::sync::Arc;
    use std::time::Duration;

    use super::*;
    use layerx_agentd::read::{LayerxdProgramBalanceReader, ProgramAuthority};
    use layerx_programs::{
        chain_proof::validators::TrustedValidator, ProtocolDeploymentVerifier, Registry,
    };
    use serde::Deserialize;
    use serde_json::{json, Value};
    use std::net::SocketAddr;
    use std::sync::atomic::{AtomicUsize, Ordering};

    const BEARER: &str = "proof-serve-errors-authenticated-client";

    #[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq)]
    #[serde(rename_all = "snake_case")]
    enum Code {
        ProgramStateUnavailable,
        InvalidProgram,
        InvalidRead,
        ReadTooLarge,
        VerifiedReadUnavailable,
        Unauthorized,
    }

    impl Code {
        fn status(&self) -> u16 {
            match self {
                Self::InvalidProgram | Self::InvalidRead => 400,
                Self::ReadTooLarge => 413,
                Self::ProgramStateUnavailable | Self::VerifiedReadUnavailable => 503,
                Self::Unauthorized => 401,
            }
        }
    }

    #[derive(Deserialize)]
    #[serde(deny_unknown_fields)]
    struct ErrorBody {
        error: Code,
    }

    #[derive(Debug, PartialEq)]
    enum Observed {
        Value(Value),
        Refused(Code),
    }

    fn decode_response(bytes: &[u8]) -> Result<Observed, String> {
        let raw = std::str::from_utf8(bytes).map_err(|error| error.to_string())?;
        let (head, body) = raw.split_once("\r\n\r\n").ok_or("missing HTTP head")?;
        let mut lines = head.split("\r\n");
        let status_line = lines.next().ok_or("missing status")?;
        let mut parts = status_line.split_ascii_whitespace();
        if parts.next() != Some("HTTP/1.1") {
            return Err("invalid HTTP version".into());
        }
        let status: u16 = parts
            .next()
            .ok_or("missing status code")?
            .parse::<u16>()
            .map_err(|e| e.to_string())?;
        let mut length = None;
        let mut content_type = false;
        let mut no_store = false;
        let mut close = false;
        for line in lines {
            let (name, value) = line.split_once(':').ok_or("invalid header")?;
            match name.to_ascii_lowercase().as_str() {
                "content-length" => {
                    if length.is_some() {
                        return Err("duplicate length".into());
                    }
                    length = Some(value.trim().parse::<usize>().map_err(|e| e.to_string())?);
                }
                "content-type" => content_type = value.trim() == "application/json",
                "cache-control" => no_store = value.trim() == "no-store",
                "connection" => close = value.trim() == "close",
                _ => {}
            }
        }
        if length != Some(body.len()) || !content_type || !no_store || !close {
            return Err("invalid response framing or cache policy".into());
        }
        if status == 200 {
            let value: Value = serde_json::from_str(body).map_err(|e| e.to_string())?;
            if !value.is_object() || value.get("error").is_some() {
                return Err("invalid success".into());
            }
            return Ok(Observed::Value(value));
        }
        let envelope: ErrorBody = serde_json::from_str(body).map_err(|e| e.to_string())?;
        if status != envelope.error.status() {
            return Err("status and code disagree".into());
        }
        Ok(Observed::Refused(envelope.error))
    }

    fn authority_config() -> (Vec<u8>, ProtocolDeploymentVerifier) {
        use std::os::unix::fs::DirBuilderExt;
        use std::process::Command;
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = std::env::temp_dir().join(format!(
            "serve-errors-authority-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .expect("private authority directory");
        let key = root.join("key.pem");
        let certificate = Command::new("openssl")
            .args([
                "req",
                "-x509",
                "-newkey",
                "ed25519",
                "-nodes",
                "-subj",
                "/CN=serve-errors-proof",
                "-days",
                "1",
                "-outform",
                "DER",
                "-keyout",
            ])
            .arg(&key)
            .output()
            .expect("OpenSSL certificate generation");
        assert!(
            certificate.status.success(),
            "OpenSSL certificate generation failed"
        );
        let public = Command::new("openssl")
            .args(["pkey", "-pubout", "-outform", "DER", "-in"])
            .arg(&key)
            .output()
            .expect("OpenSSL public key export");
        std::fs::remove_dir_all(&root).expect("remove ephemeral authority key");
        assert!(public.status.success());
        assert_eq!(
            public.stdout.len(),
            44,
            "Ed25519 SubjectPublicKeyInfo length"
        );
        let validator = TrustedValidator::new(public.stdout[12..].try_into().unwrap(), 1);
        let verifier =
            ProtocolDeploymentVerifier::from_chain_trust("paxeer-125", vec![validator], 1000)
                .unwrap();
        (certificate.stdout, verifier)
    }

    struct Service {
        address: SocketAddr,
        _pool: HttpPool,
        node: Option<(daemon::disposable_node::LocalNode, String)>,
    }

    impl Service {
        fn start(native_chain: Option<u64>, node_chain: u64) -> Self {
            let (node, url, _key) = daemon::disposable_node::LocalNode::start();
            let endpoint = || {
                layerx_client::sequencer::SequencerEndpoint::new(&url, &[])
                    .expect("disposable loopback node endpoint")
            };
            assert_eq!(endpoint().chain_id().expect("producer chain id"), 125);
            let (ca, verifier) = authority_config();
            let reader = LayerxdProgramBalanceReader::connect(
                &url,
                "node-proof-authorization".into(),
                ProgramAuthority {
                    endpoint: "https://localhost:1",
                    authorization: "authority-proof-authorization".into(),
                    replica_id: [1; 32],
                    ca_der: &ca,
                },
                verifier,
                Registry::new(),
            )
            .expect("real program reader configuration");
            let native = native_chain.map(|chain| node.native_read_route(&url, chain));
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let address = listener.local_addr().unwrap();
            let pool = serve_read_routes(
                listener,
                BEARER.into(),
                ProgramBalanceReadRoute::new(reader),
                native,
                NodeRpcConfig {
                    url: url.clone(),
                    chain_id: node_chain,
                    deadline: Duration::from_secs(2),
                },
                ca,
                endpoint(),
            )
            .unwrap();
            Self {
                address,
                _pool: pool,
                node: Some((node, url)),
            }
        }

        fn endpoint(&self) -> layerx_client::sequencer::SequencerEndpoint {
            layerx_client::sequencer::SequencerEndpoint::new(
                &self
                    .node
                    .as_ref()
                    .expect("service owns its disposable node")
                    .1,
                &[],
            )
            .expect("disposable loopback node endpoint")
        }

        fn get(&self, path: &str, bearer: &str) -> Vec<u8> {
            let mut stream = TcpStream::connect(self.address).unwrap();
            stream
                .set_read_timeout(Some(Duration::from_secs(6)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(6)))
                .unwrap();
            write!(stream, "GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {bearer}\r\nConnection: close\r\n\r\n").unwrap();
            stream.flush().unwrap();
            let mut raw = Vec::new();
            stream.read_to_end(&mut raw).unwrap();
            raw
        }

        fn twice(&self, path: &str, code: Code) {
            for _ in 0..2 {
                let raw = self.get(path, BEARER);
                assert_eq!(decode_response(&raw).unwrap(), Observed::Refused(code));
                assert!(
                    decode_response(&raw[..raw.len() - 1]).is_err(),
                    "truncated producer response must be refused"
                );
            }
            assert_eq!(
                decode_response(&self.get(path, "wrong-credential")).unwrap(),
                Observed::Refused(Code::Unauthorized)
            );
        }
    }

    #[test]
    fn program_state_unavailable() {
        let path = format!("/v1/programs/{}/balances", "11".repeat(32));
        // Both actual producers: refreshing authority fails on a wrong chain,
        // and, without native reads, the node connection itself rejects that chain.
        Service::start(Some(126), 125).twice(&path, Code::ProgramStateUnavailable);
        Service::start(None, 126).twice(&path, Code::ProgramStateUnavailable);
    }

    #[test]
    fn invalid_program() {
        let service = Service::start(Some(125), 125);
        for text in [
            "unknown".to_owned(),
            "00".repeat(32),
            "gg".repeat(32),
            "11".repeat(33),
        ] {
            service.twice(
                &format!("/v1/programs/{text}/balances"),
                Code::InvalidProgram,
            );
        }
    }

    #[test]
    fn invalid_read() {
        let service = Service::start(Some(125), 125);
        for path in [
            "/v1/reads/unknown/1",
            "/v1/reads/receipt/zz",
            "/v1/reads/checkpoint/01",
            "/v1/reads/history/account?limit=1&cursor=bad",
        ] {
            service.twice(path, Code::InvalidRead);
        }
    }

    #[test]
    fn verified_read_unavailable() {
        let service = Service::start(Some(125), 125);
        let path = format!("/v1/reads/availability/{}", u64::MAX);
        for _ in 0..2 {
            let raw = service.get(&path, BEARER);
            assert_eq!(
                decode_response(&raw).unwrap(),
                Observed::Refused(Code::VerifiedReadUnavailable)
            );
            let first_body = raw.windows(4).position(|w| w == b"\r\n\r\n").unwrap() + 4;
            assert_eq!(
                &raw[first_body..],
                b"{\"error\":\"verified_read_unavailable\"}"
            );
            assert!(decode_response(&raw[..raw.len() - 1]).is_err());

            // Corrupt the actual response only at the consumer boundary, keeping
            // its framing valid so unknown fields reach the strict JSON decoder.
            for (field, value) in [
                ("detail", json!("private-native-read-rpc-message")),
                ("detail", Value::Null),
                (
                    "diagnostic",
                    json!({"data": "private-native-read-rpc-data"}),
                ),
            ] {
                let mut body: Value = serde_json::from_slice(&raw[first_body..]).unwrap();
                body.as_object_mut().unwrap().insert(field.into(), value);
                let body = body.to_string();
                let head = std::str::from_utf8(&raw[..first_body]).unwrap().replace(
                    &format!("Content-Length: {}\r\n", raw.len() - first_body),
                    &format!("Content-Length: {}\r\n", body.len()),
                );
                let mut malformed = head.into_bytes();
                malformed.extend_from_slice(body.as_bytes());
                let error = decode_response(&malformed).unwrap_err();
                assert!(error.contains("unknown field"), "{error}");
            }
        }
        assert_eq!(
            decode_response(&service.get(&path, "wrong-credential")).unwrap(),
            Observed::Refused(Code::Unauthorized)
        );
    }

    #[test]
    fn native_read_refusals_redact_internal_errors() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let requests = Arc::new(AtomicUsize::new(0));
        let observed = Arc::clone(&requests);
        let pool = HttpPool::start(listener, move |mut stream| {
            let answer = match parse_request(&mut stream, BEARER) {
                Parsed::Admitted(path) => {
                    assert_eq!(path, "/v1/reads/checkpoint/1");
                    let index = observed.fetch_add(1, Ordering::SeqCst);
                    native_read_response(Err(NativeReadError::Connection(SequencerError::Rpc(
                        RpcRefusal {
                            code: -32043,
                            message: format!("private-native-read-rpc-message-{index}"),
                            data: Some(json!({
                                "diagnostic": format!("private-native-read-rpc-data-{index}"),
                                "request": index,
                            })),
                        },
                    ))))
                }
                Parsed::Refused(status, body) => (status, body.to_owned()),
                Parsed::Closed => return,
            };
            response(&mut stream, answer.0, &answer.1).unwrap();
        })
        .unwrap();
        let service = Service {
            address,
            _pool: pool,
            node: None,
        };
        for _ in 0..2 {
            let raw = service.get("/v1/reads/checkpoint/1", BEARER);
            assert!(raw.starts_with(b"HTTP/1.1 503 "));
            assert_eq!(
                decode_response(&raw).unwrap(),
                Observed::Refused(Code::VerifiedReadUnavailable)
            );
            let first_body = raw.windows(4).position(|w| w == b"\r\n\r\n").unwrap() + 4;
            assert_eq!(
                &raw[first_body..],
                b"{\"error\":\"verified_read_unavailable\"}"
            );
            let text = std::str::from_utf8(&raw[first_body..]).unwrap();
            for diagnostic in [
                "Connection",
                "Rpc",
                "RpcRefusal",
                "detail",
                "message",
                "data",
                "diagnostic",
                "-32043",
                "private-native-read-rpc-message",
                "private-native-read-rpc-data",
            ] {
                assert!(
                    !text.contains(diagnostic),
                    "leaked diagnostic: {diagnostic}"
                );
            }
            assert!(decode_response(&raw[..raw.len() - 1]).is_err());
        }
        assert_eq!(
            decode_response(&service.get("/v1/reads/checkpoint/1", "wrong-credential")).unwrap(),
            Observed::Refused(Code::Unauthorized)
        );
        assert_eq!(
            requests.load(Ordering::SeqCst),
            2,
            "independent producer invocations"
        );
    }

    #[test]
    fn read_too_large() {
        // This anchor's producer is the native-error-to-HTTP classifier. Feed its
        // actual bounded-read error type, then use the production pool, admission
        // parser and response writer. There is no JSON-RPC peer or transport double.
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let requests = Arc::new(AtomicUsize::new(0));
        let observed = Arc::clone(&requests);
        let pool = HttpPool::start(listener, move |mut stream| {
            let answer = match parse_request(&mut stream, BEARER) {
                Parsed::Admitted(_) => {
                    observed.fetch_add(1, Ordering::SeqCst);
                    native_read_response(Err(NativeReadError::ResultTooLarge))
                }
                Parsed::Refused(status, body) => (status, body.to_owned()),
                Parsed::Closed => return,
            };
            response(&mut stream, answer.0, &answer.1).unwrap();
        })
        .unwrap();
        let service = Service {
            address,
            _pool: pool,
            node: None,
        };
        service.twice("/v1/reads/history/account?limit=1", Code::ReadTooLarge);
        assert_eq!(
            requests.load(Ordering::SeqCst),
            2,
            "each result must come from a fresh producer invocation"
        );
    }

    #[test]
    fn happy_path_and_malformed_producer_result() {
        let service = Service::start(Some(125), 125);
        let height = service.endpoint().block_number().expect("real node head");
        let path = format!("/v1/reads/checkpoint/{height}");
        let (node, url) = service
            .node
            .as_ref()
            .expect("service owns its disposable node");
        let expected = node
            .native_read_route(url, 125)
            .read(&path)
            .expect("real checkpoint producer");
        for _ in 0..2 {
            let raw = service.get(&path, BEARER);
            assert_eq!(
                decode_response(&raw).unwrap(),
                Observed::Value(expected.clone())
            );
            let mut malformed = raw.clone();
            let first_body = malformed.windows(4).position(|w| w == b"\r\n\r\n").unwrap() + 4;
            // Same byte length and valid HTTP framing: the consumer's JSON decoder
            // must reject the damaged producer payload independently of framing.
            malformed[first_body] = b'!';
            assert!(decode_response(&malformed).is_err());
        }
        let raw = service.get("/v1/reads/receipt/zz", BEARER);
        let mut malformed = raw.clone();
        let offset = malformed
            .windows(b"invalid_read".len())
            .position(|w| w == b"invalid_read")
            .unwrap();
        malformed[offset] = b'x';
        assert!(
            decode_response(&malformed).is_err(),
            "unknown producer error code must be refused"
        );
        let mut mismatched = String::from_utf8(raw).unwrap();
        mismatched = mismatched.replacen("400", "503", 1);
        assert!(
            decode_response(mismatched.as_bytes()).is_err(),
            "status/code mismatch must be refused"
        );
        assert_eq!(
            native_read_response(Ok(json!({"value": 7}))),
            (200, "{\"value\":7}".into())
        );
    }
}
