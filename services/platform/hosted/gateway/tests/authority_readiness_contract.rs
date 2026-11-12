include!("../../authority/src/main.rs");

#[path = "../src/authority_readiness.rs"]
mod gateway_readiness;

use gateway_readiness::{decode_authority_readiness, probe_authority_readiness, AuthorityUnready};
use layerx_platform_gateway::http::{Client, Endpoint};
#[path = "../src/backend_availability.rs"]
mod backend_availability;
use backend_availability::{BackendAvailability, KernelBackend};
use native_tls::Certificate;
use std::process::Command;

const NETWORK: &str = "gateway-authority-contract";
const NETWORK_NUMBER: u32 = 7331;
const WIRE: &str = "3";

struct AuthorityCase {
    directory: PathBuf,
    tls: Arc<ServerConfig>,
    client: Client,
    token: String,
}

impl AuthorityCase {
    fn start() -> Self {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let mut nonce = [0; 32];
        getrandom::fill(&mut nonce).unwrap_or_else(|error| panic!("{}: {error:?}", "random test credentials"));
        let token = hex::encode(&nonce);
        let directory = env::temp_dir().join(format!("gateway-authority-contract-{token}"));
        fs::create_dir(&directory).unwrap_or_else(|error| panic!("{}: {error:?}", "private test directory"));
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&directory, fs::Permissions::from_mode(0o700)).unwrap_or_else(|error| panic!("{}: {error:?}", "private directory permissions"));
        }
        let run = |arguments: &[&str]| {
            let output = Command::new("openssl").current_dir(&directory).args(arguments).output().unwrap_or_else(|error| panic!("{}: {error:?}", "openssl must run"));
            assert!(output.status.success(), "{}", String::from_utf8_lossy(&output.stderr));
        };
        run(&["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", "key.pem", "-out", "cert.pem", "-days", "1", "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost"]);
        run(&["x509", "-in", "cert.pem", "-outform", "DER", "-out", "cert.der"]);
        run(&["pkcs8", "-topk8", "-nocrypt", "-in", "key.pem", "-outform", "DER", "-out", "key.der"]);
        let certificate = fs::read(directory.join("cert.der")).unwrap_or_else(|error| panic!("{}: {error:?}", "certificate"));
        let key = fs::read(directory.join("key.der")).unwrap_or_else(|error| panic!("{}: {error:?}", "private key"));
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(vec![CertificateDer::from(certificate.clone())], PrivateKeyDer::try_from(key).unwrap_or_else(|error| panic!("{}: {error:?}", "PKCS8 key")))
                .unwrap_or_else(|error| panic!("{}: {error:?}", "authority TLS configuration")),
        );
        let client = Client::without_identity(Certificate::from_der(&certificate).unwrap_or_else(|error| panic!("{}: {error:?}", "CA DER")));
        Self { directory, tls, client, token }
    }

    fn probe(&self, ready: bool, status: u16, network: &str, number: u32, wire: &str) -> (Result<(), AuthorityUnready>, Vec<u8>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{}: {error:?}", "authority listener"));
        let endpoint = Endpoint::parse(&format!("https://localhost:{}", listener.local_addr().unwrap_or_else(|error| panic!("{}: {error:?}", "address")).port())).unwrap_or_else(|error| panic!("{}: {error:?}", "TLS endpoint"));
        let response = json(
            status,
            &serde_json::json!(AuthorityReadinessResponse {
                ready,
                network_id: NETWORK,
                protocol_network_id: NETWORK_NUMBER,
                wire_version: WIRE,
            }),
        );
        let body = response.body.clone();
        let tls = Arc::clone(&self.tls);
        let token = self.token.clone();
        let server = thread::spawn(move || {
            let (tcp, _) = listener.accept().unwrap_or_else(|error| panic!("{}: {error:?}", "authority connection"));
            tcp.set_read_timeout(Some(Duration::from_secs(8))).unwrap_or_else(|error| panic!("{}: {error:?}", "read deadline"));
            tcp.set_write_timeout(Some(Duration::from_secs(8))).unwrap_or_else(|error| panic!("{}: {error:?}", "write deadline"));
            let mut stream = StreamOwned::new(ServerConnection::new(tls).unwrap_or_else(|error| panic!("{}: {error:?}", "TLS connection")), tcp);
            let request = read_http_message(&mut stream).unwrap_or_else(|error| panic!("{}: {error:?}", "authority HTTP parser"));
            assert_eq!(request.method, "GET");
            assert_eq!(request.path, "/readyz");
            assert_eq!(request.headers.get("authorization"), Some(&format!("Bearer {token}")));
            write_response(&mut stream, &response, false).unwrap_or_else(|error| panic!("{}: {error:?}", "authority HTTP writer"));
            stream.conn.send_close_notify();
            stream.flush().unwrap_or_else(|error| panic!("{}: {error:?}", "TLS close"));
        });
        let actual = probe_authority_readiness(&self.client, &endpoint, &self.token, network, number, wire);
        server.join().unwrap_or_else(|error| panic!("{}: {error:?}", "authority service thread"));
        (actual, body)
    }
}

impl Drop for AuthorityCase {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.directory).unwrap_or_else(|error| panic!("{}: {error:?}", "test credentials cleanup"));
    }
}

#[cfg(test)]
mod authority_readiness_contract_tests {
    use super::*;

    #[test]
    fn authority_readiness_contract() {
        let case = AuthorityCase::start();
        let (actual, body) = case.probe(true, 200, NETWORK, NETWORK_NUMBER, WIRE);
        assert_eq!(actual, Ok(()));
        for (actual, expected) in [
            (case.probe(false, 503, NETWORK, NETWORK_NUMBER, WIRE).0, Err(AuthorityUnready::Unavailable)),
            (case.probe(true, 200, "different", NETWORK_NUMBER, WIRE).0, Err(AuthorityUnready::IdentityMismatch)),
            (case.probe(true, 200, NETWORK, NETWORK_NUMBER + 1, WIRE).0, Err(AuthorityUnready::IdentityMismatch)),
            (case.probe(true, 200, NETWORK, NETWORK_NUMBER, "0").0, Err(AuthorityUnready::IdentityMismatch)),
        ] {
            assert_eq!(actual, expected);
            let backend = BackendAvailability::probed(KernelBackend::Authority.name(), actual.is_ok());
            assert_eq!(backend.backend, "independent_receipt_authority");
            assert_eq!(backend.document()["state"], "unavailable");
        }
        let backend = BackendAvailability::probed(KernelBackend::Authority.name(), actual.is_ok());
        assert_eq!(backend.backend, "independent_receipt_authority");
        assert_eq!(backend.document()["state"], "ready");
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{}: {error:?}", "unused endpoint"));
        let endpoint = Endpoint::parse(&format!("https://localhost:{}", listener.local_addr().unwrap_or_else(|error| panic!("{}: {error:?}", "address")).port())).unwrap_or_else(|error| panic!("{}: {error:?}", "endpoint"));
        drop(listener);
        assert_eq!(probe_authority_readiness(&case.client, &endpoint, &case.token, NETWORK, NETWORK_NUMBER, WIRE), Err(AuthorityUnready::Transport));
        let mut count = 1;
        {
            let original: serde_json::Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "response JSON"));
            let decode = |status, content_type: &str, value: &serde_json::Value| decode_authority_readiness(status, content_type, &serde_json::to_vec(value).unwrap_or_else(|error| panic!("{}: {error:?}", "JSON")), NETWORK, NETWORK_NUMBER, WIRE);
            assert_eq!(decode(200, "application/json", &original), Ok(()));
            count += 1;
            for field in ["ready", "network_id", "protocol_network_id", "wire_version"] {
                let mut missing = original.clone();
                missing.as_object_mut().unwrap_or_else(|| panic!("{}", "object")).remove(field);
                assert_eq!(decode(200, "application/json", &missing), Err(AuthorityUnready::InvalidSchema));
                count += 1;
                let mut wrong_type = original.clone();
                wrong_type[field] = serde_json::Value::Null;
                assert_eq!(decode(200, "application/json", &wrong_type), Err(AuthorityUnready::InvalidSchema));
                count += 1;
            }
            for value in [serde_json::json!("7331"), serde_json::json!(-1), serde_json::json!(4_294_967_296_u64), serde_json::json!(7331.5), serde_json::json!(true)] {
                let mut invalid = original.clone();
                invalid["protocol_network_id"] = value;
                assert_eq!(decode(200, "application/json", &invalid), Err(AuthorityUnready::InvalidSchema));
                count += 1;
            }
            let mut unknown = original.clone();
            unknown["unexpected"] = serde_json::json!(true);
            assert_eq!(decode(200, "application/json", &unknown), Err(AuthorityUnready::InvalidSchema));
            count += 1;
            let mut false_ready = original.clone();
            false_ready["ready"] = serde_json::json!(false);
            assert_eq!(decode(200, "application/json", &false_ready), Err(AuthorityUnready::Unavailable));
            count += 1;
            assert_eq!(decode(503, "application/json", &original), Err(AuthorityUnready::Unavailable));
            count += 1;
            assert_eq!(decode(200, "text/plain", &original), Err(AuthorityUnready::Unavailable));
            count += 1;
            assert_eq!(decode_authority_readiness(200, "application/json", b"{", NETWORK, NETWORK_NUMBER, WIRE), Err(AuthorityUnready::InvalidSchema));
            count += 1;
            for (field, value) in [("network_id", serde_json::json!("different")), ("protocol_network_id", serde_json::json!(NETWORK_NUMBER + 1)), ("wire_version", serde_json::json!("0"))] {
                let mut mismatch = original.clone();
                mismatch[field] = value;
                assert_eq!(decode(200, "application/json", &mismatch), Err(AuthorityUnready::IdentityMismatch));
                count += 1;
            }
        }
        println!("PAXEER_X_AUTHORITY_CASES={count}");
    }
}

#[cfg(test)]
mod authority_lni_compatibility_tests {
    use super::*;
    #[test]
    fn authenticated_lni_preserves_the_strict_four_field_contract() {
        let case = AuthorityCase::start();
        let (actual, body) = case.probe(true, 200, NETWORK, NETWORK_NUMBER, WIRE);
        assert_eq!(actual, Ok(()));
        let original: serde_json::Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "actual serializer JSON"));
        assert_eq!(original.as_object().unwrap_or_else(|| panic!("{}", "object")).len(), 4);
        let decode = |value: &serde_json::Value| decode_authority_readiness(200, "application/json", &serde_json::to_vec(value).unwrap_or_else(|error| panic!("{}: {error:?}", "JSON")), NETWORK, NETWORK_NUMBER, WIRE);
        assert_eq!(decode(&original), Ok(()));
        let mut false_ready = original.clone();
        false_ready["ready"] = serde_json::json!(false);
        assert_eq!(decode(&false_ready), Err(AuthorityUnready::Unavailable));
        let mut unexpected = original;
        unexpected["dependencies"] = serde_json::json!({});
        assert_eq!(decode(&unexpected), Err(AuthorityUnready::InvalidSchema));
        println!("PAXEER_X_LNI_CASES=3");
    }
}
