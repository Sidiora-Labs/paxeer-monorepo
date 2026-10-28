use super::{Config, Endpoint, IncomingRequest};
use crate::rpc::error;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::sync::Mutex;
use zeroize::Zeroizing;

const URL_VARIABLE: &str = "LAYERX_GATEWAY_FAUCET_URL";
const TOKEN_VARIABLE: &str = "LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE";
const CLAIM_PATH: &str = "/v1/faucet/service-claims";
const IDEMPOTENCY_DOMAIN: &[u8] = b"layerx-faucet-claim-v1";
const CLAIMS_PER_MINUTE: u32 = 30;
const STARTER_IDEMPOTENCY_SCOPE: &str = "starter-credit";
const MAX_FAUCET_AMOUNT: u64 = (1 << 53) - 1;
pub(super) const px_requestFunds: &str = "px_requestFunds";

static FAUCET_WINDOW: Mutex<(u64, u32)> = Mutex::new((0, 0));

pub(super) struct Faucet {
    endpoint: Endpoint,
    token: Zeroizing<String>,
}

fn idempotency(principal_digest: &str, did: &str, public_key: &str) -> String {
    let mut hash = Sha256::new();
    for part in [
        IDEMPOTENCY_DOMAIN,
        principal_digest.as_bytes(),
        did.as_bytes(),
        public_key.as_bytes(),
    ] {
        hash.update((part.len() as u64).to_be_bytes());
        hash.update(part);
    }
    super::hex(&hash.finalize())
}

fn path_safe(value: &str, maximum: usize) -> bool {
    !value.is_empty()
        && value.len() <= maximum
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || b"-._:".contains(&byte))
}

fn lowercase_hex32(value: &str) -> bool {
    value.len() == 64
        && value
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
}

fn method_and_identifier(did: &str) -> Option<(&str, &str)> {
    let (method, identifier) = did.strip_prefix("did:")?.split_once(':')?;
    (!method.is_empty() && !identifier.is_empty()).then_some((method, identifier))
}

fn claim_params(params: Option<&Value>) -> Result<(String, String), i32> {
    let Some(Value::Array(args)) = params else {
        return Err(-32602);
    };
    let [Value::String(did), Value::String(public_key)] = args.as_slice() else {
        return Err(-32602);
    };
    if layerx_types::ids::Did::new(did.as_bytes()).is_err()
        || method_and_identifier(did).is_none()
        || !path_safe(did, 512)
        || !lowercase_hex32(public_key)
        || *public_key == "00".repeat(32)
    {
        return Err(-32602);
    }
    Ok((did.clone(), public_key.clone()))
}

fn consume(window: &mut (u64, u32), minute: u64) -> bool {
    if window.0 != minute {
        *window = (minute, 0);
    }
    if window.1 >= CLAIMS_PER_MINUTE {
        return false;
    }
    window.1 += 1;
    true
}

fn consume_claim() -> bool {
    let Ok(second) = super::now() else {
        return false;
    };
    let Ok(mut window) = FAUCET_WINDOW.lock() else {
        return false;
    };
    consume(&mut window, second / 60)
}

fn refusal(id: &Value, code: i32, message: &str, reason: &str) -> Value {
    let mut refused = error(id, code, message);
    refused["error"]["data"] = json!({ "code": reason });
    refused
}

fn claim_request(principal: &str, did: &str, public_key: &str) -> String {
    json!({
        "principal": principal,
        "did": did,
        "public_key": public_key,
    })
    .to_string()
}

fn claim_result(id: &Value, status: u16, content_type: &str, body: &[u8]) -> Value {
    if content_type != "application/json" {
        return error(id, -32603, "Invalid upstream response");
    }
    let Ok(document) = serde_json::from_slice::<Value>(body) else {
        return error(id, -32603, "Invalid upstream response");
    };
    if status == 202 {
        let mut pending = error(id, -32001, "Faucet claim unavailable");
        pending["error"]["data"] = json!({
            "code": "faucet_claim_pending", "state": "pending", "upstream": document
        });
        return pending;
    }
    if status != 200 {
        let code = match status {
            400 | 409 => -32602,
            401 | 403 | 404 => -32002,
            429 => -32005,
            _ => -32001,
        };
        let mut refused = error(id, code, "Faucet claim refused");
        refused["error"]["data"] = document;
        return refused;
    }
    let funded = document["funded"] == json!(true)
        && document["funding_id"].as_str().is_some_and(|id| {
            !id.is_empty() && id.len() <= 128 && id.bytes().all(|byte| byte.is_ascii_hexdigit())
        })
        && document["transaction_id"]
            .as_str()
            .is_some_and(|id| !id.is_empty() && id.len() <= 128)
        && document["amount"].as_str().is_some_and(|amount| {
            !amount.is_empty()
                && amount.len() <= 39
                && amount.bytes().all(|byte| byte.is_ascii_digit())
                && amount != "0"
        })
        && document["network"]
            .as_str()
            .is_some_and(|network| !network.is_empty() && network.len() <= 64);
    if !funded {
        return error(id, -32603, "Faucet claim evidence incomplete");
    }
    json!({"jsonrpc":"2.0", "id":id, "result":document})
}

struct Grant {
    grant_id: String,
    amount: String,
    document: Value,
}

fn grant_result(
    id: &Value,
    sub: &str,
    status: u16,
    content_type: &str,
    body: &[u8],
) -> Result<Grant, Value> {
    if content_type != "application/json" {
        return Err(error(id, -32603, "Invalid upstream response"));
    }
    let Ok(document) = serde_json::from_slice::<Value>(body) else {
        return Err(error(id, -32603, "Invalid upstream response"));
    };
    if status != 200 {
        let code = match status {
            403 | 409 => -32002,
            429 => -32005,
            400 | 404 => -32602,
            _ => -32001,
        };
        let mut refused = error(id, code, "Starter credit refused");
        refused["error"]["data"] = document;
        return Err(refused);
    }
    let grant_id = document["grant_id"]
        .as_str()
        .filter(|grant| lowercase_hex32(&format!("{grant}{grant}")))
        .map(str::to_owned);
    let amount = document["amount"]
        .as_str()
        .filter(|amount| {
            !amount.is_empty()
                && amount.len() <= 39
                && amount.bytes().all(|b| b.is_ascii_digit())
                && *amount != "0"
        })
        .map(str::to_owned);
    match (grant_id, amount) {
        (Some(grant_id), Some(amount)) if document["sub"] == json!(sub) => Ok(Grant {
            grant_id,
            amount,
            document,
        }),
        _ => Err(error(id, -32603, "Starter credit evidence incomplete")),
    }
}

fn faucet_representable(amount: &str) -> bool {
    amount
        .parse::<u64>()
        .is_ok_and(|value| value <= MAX_FAUCET_AMOUNT && value.to_string() == amount)
}

fn identity_call(config: &Config, path: &str, body: &Value) -> Result<super::UpstreamResponse, ()> {
    let token = config.registration_token.as_ref().ok_or(())?;
    let (identity, _) = config
        .backend(super::KernelBackend::Identity)
        .map_err(|_| ())?;
    super::upstream_json(
        config,
        identity,
        token.as_str(),
        "POST",
        path,
        None,
        body.to_string().as_bytes(),
    )
    .map_err(|_| ())
}

fn bound_signer(allowed: &[String], public_key: &str) -> bool {
    allowed
        .iter()
        .any(|key| key.eq_ignore_ascii_case(public_key))
}

fn request_funds(
    config: &Config,
    request: &IncomingRequest,
    id: &Value,
    params: Option<&Value>,
) -> Value {
    let (did, public_key) = match claim_params(params) {
        Ok(parsed) => parsed,
        Err(code) => return error(id, code, "Invalid params"),
    };
    let Some(faucet) = &config.faucet else {
        return refusal(
            id,
            -32001,
            "Faucet claim unavailable",
            "faucet_not_configured",
        );
    };
    let (principal, session) = match super::session(config, request) {
        Ok(authenticated) => authenticated,
        Err(answer) if answer.status == 401 => {
            return refusal(id, -32002, "Faucet claim refused", "session_required");
        }
        Err(_) => {
            return refusal(
                id,
                -32001,
                "Faucet claim unavailable",
                "identity_unavailable",
            );
        }
    };
    if !bound_signer(&session.allowed_signer_public_keys, &public_key) {
        return refusal(id, -32002, "Faucet claim refused", "signer_not_bound");
    }
    if config.registration_token.is_none() {
        return refusal(
            id,
            -32001,
            "Faucet claim unavailable",
            "registration_not_configured",
        );
    }
    let sub = principal.as_str();
    let grant = match identity_call(config, "/v1/starter-credits", &json!({ "sub": sub })) {
        Ok(upstream) => match grant_result(
            id,
            sub,
            upstream.status,
            &upstream.content_type,
            &upstream.body,
        ) {
            Ok(grant) => grant,
            Err(refused) => return refused,
        },
        Err(()) => {
            return refusal(
                id,
                -32001,
                "Faucet claim unavailable",
                "identity_unavailable",
            )
        }
    };
    if !faucet_representable(&grant.amount) {
        return refusal(
            id,
            -32001,
            "Starter credit unavailable",
            "starter_credit_amount_unrepresentable",
        );
    }
    let key = idempotency(sub, &grant.grant_id, STARTER_IDEMPOTENCY_SCOPE);
    let body = claim_request(sub, &did, &public_key);
    if !consume_claim() {
        return refusal(id, -32005, "Faucet claim unavailable", "faucet_rate_limit");
    }
    let mut funded = match super::upstream_json(
        config,
        &faucet.endpoint,
        faucet.token.as_str(),
        "POST",
        CLAIM_PATH,
        Some(&key),
        body.as_bytes(),
    ) {
        Ok(upstream) => claim_result(id, upstream.status, &upstream.content_type, &upstream.body),
        Err(_) => return refusal(id, -32001, "Faucet claim unavailable", "faucet_unavailable"),
    };
    if funded.get("result").is_none() {
        return funded;
    }
    if funded["result"]["amount"] != json!(grant.amount) {
        return refusal(
            id,
            -32603,
            "Starter credit amount mismatch",
            "starter_credit_amount_mismatch",
        );
    }
    let settle = json!({ "sub": sub, "grant_id": grant.grant_id });
    match identity_call(config, "/v1/starter-credits/settle", &settle) {
        Ok(upstream) => {
            if let Err(refused) = grant_result(
                id,
                sub,
                upstream.status,
                &upstream.content_type,
                &upstream.body,
            ) {
                return refused;
            }
        }
        Err(()) => {
            return refusal(
                id,
                -32001,
                "Faucet claim unavailable",
                "identity_unavailable",
            )
        }
    }
    funded["result"]["starter_credit"] = grant.document;
    funded
}

pub(super) fn dispatch(
    config: &Config,
    request: &IncomingRequest,
    method: &str,
    id: &Value,
    params: Option<&Value>,
) -> Option<Value> {
    (method == px_requestFunds || method == "lx_requestFunds")
        .then(|| request_funds(config, request, id, params))
}

pub(super) fn configured() -> Result<Option<Faucet>, String> {
    if std::env::var_os(URL_VARIABLE).is_none() && std::env::var_os(TOKEN_VARIABLE).is_none() {
        return Ok(None);
    }
    let url = std::env::var(URL_VARIABLE)
        .map_err(|_| format!("{URL_VARIABLE} is required when {TOKEN_VARIABLE} is set"))?;
    Ok(Some(Faucet {
        endpoint: Endpoint::parse(&url)?,
        token: super::read_secret(TOKEN_VARIABLE)?,
    }))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::TcpListener;
    use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
    use std::sync::Arc;
    use std::thread;
    use std::time::Duration;

    struct StarterCreditUpstream {
        url: String,
        requests: Arc<AtomicUsize>,
        claims: Arc<AtomicUsize>,
        stop: Arc<AtomicBool>,
        worker: Option<thread::JoinHandle<()>>,
    }

    impl StarterCreditUpstream {
        fn start(
            tls: native_tls::TlsAcceptor,
            answer: impl Fn(&str, &Value) -> Value + Send + 'static,
        ) -> Self {
            let listener = TcpListener::bind("127.0.0.1:0").expect("bind TLS upstream");
            let url = format!(
                "https://localhost:{}",
                listener.local_addr().unwrap().port()
            );
            listener.set_nonblocking(true).unwrap();
            let requests = Arc::new(AtomicUsize::new(0));
            let claims = Arc::new(AtomicUsize::new(0));
            let stop = Arc::new(AtomicBool::new(false));
            let (count, claim_count, stopped) = (
                Arc::clone(&requests),
                Arc::clone(&claims),
                Arc::clone(&stop),
            );
            let worker = thread::spawn(move || {
                while !stopped.load(Ordering::Acquire) {
                    let tcp = match listener.accept() {
                        Ok((tcp, _)) => tcp,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(1));
                            continue;
                        }
                        Err(error) => panic!("accept TLS upstream: {error}"),
                    };
                    tcp.set_read_timeout(Some(Duration::from_secs(3))).unwrap();
                    tcp.set_write_timeout(Some(Duration::from_secs(3))).unwrap();
                    let mut stream = tls.accept(tcp).expect("TLS handshake");
                    let request = layerx_platform_internal::http::parse_client_request(&mut stream)
                        .expect("read upstream request");
                    count.fetch_add(1, Ordering::AcqRel);
                    if request.path == CLAIM_PATH {
                        claim_count.fetch_add(1, Ordering::AcqRel);
                    }
                    assert_eq!(request.method, "POST");
                    assert_eq!(request.headers["authorization"], "Bearer service-token");
                    let body = serde_json::from_slice(&request.body).expect("upstream JSON");
                    let response =
                        layerx_platform_internal::http::json(200, &answer(&request.path, &body));
                    layerx_platform_internal::http::write_response(&mut stream, &response)
                        .expect("write upstream response");
                }
            });
            Self {
                url,
                requests,
                claims,
                stop,
                worker: Some(worker),
            }
        }
    }

    impl Drop for StarterCreditUpstream {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::Release);
            if let Some(worker) = self.worker.take() {
                let result = worker.join();
                if !thread::panicking() {
                    result.expect("TLS upstream must finish without errors");
                }
            }
        }
    }

    #[test]
    fn starter_credit_amount_is_the_faucet_default_claim() {
        let signup = include_str!("../../identity/src/signup.rs");
        let faucet = include_str!("../../faucet/src/main.rs");
        let starter_amount = signup
            .split_once("pub const STARTER_CREDIT_AMOUNT: &str = \"")
            .expect("starter credit amount declaration")
            .1
            .split_once('"')
            .expect("starter credit amount literal")
            .0;
        let default_amount = faucet
            .split_once("parse_u64(\"LAYERX_FAUCET_CLAIM_AMOUNT\",")
            .expect("faucet claim amount default")
            .1
            .split_once(')')
            .expect("faucet default literal")
            .0
            .trim()
            .replace('_', "")
            .parse::<u64>()
            .expect("faucet default is an integer literal");
        assert_eq!(starter_amount, default_amount.to_string());
        assert!(faucet_representable(starter_amount));
    }

    #[test]
    fn starter_credit_unrepresentable_grant_never_calls_the_faucet() {
        let directory = std::env::temp_dir().join(format!(
            "layerx-starter-credit-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        std::fs::create_dir(&directory).expect("create TLS fixture directory");
        let cert_path = directory.join("cert.pem");
        let key_path = directory.join("key.pem");
        let generated = std::process::Command::new("openssl")
            .args([
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-days",
                "1",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost",
                "-keyout",
            ])
            .arg(&key_path)
            .arg("-out")
            .arg(&cert_path)
            .output()
            .expect("generate local TLS certificate");
        assert!(
            generated.status.success(),
            "{}",
            String::from_utf8_lossy(&generated.stderr)
        );
        let cert = std::fs::read(&cert_path).unwrap();
        let key = std::fs::read(&key_path).unwrap();
        std::fs::remove_dir_all(&directory).expect("remove TLS fixture files");
        let ca = native_tls::Certificate::from_pem(&cert).expect("local CA");
        let tls = native_tls::TlsAcceptor::new(
            native_tls::Identity::from_pkcs8(&cert, &key).expect("local TLS identity"),
        )
        .expect("local TLS acceptor");
        let identity = StarterCreditUpstream::start(tls.clone(), |path, body| match path {
            "/v1/sessions/introspect" => {
                assert_eq!(body, &json!({"token": "session-token"}));
                json!({"active": true, "sub": "beta.starter-credit", "allowed_signer_public_keys": ["ab".repeat(32)]})
            }
            "/v1/starter-credits" => {
                assert_eq!(body, &json!({"sub": "beta.starter-credit"}));
                json!({"sub": "beta.starter-credit", "grant_id": "ab".repeat(16), "asset": "PAX", "amount": "10000000000000000000", "granted_at": 1, "settled_at": null})
            }
            _ => panic!("unexpected identity request: {path}"),
        });
        let faucet = StarterCreditUpstream::start(tls, |path, _| {
            assert_eq!(path, CLAIM_PATH);
            funded(&"ef".repeat(32))
        });
        let config = Config {
            wallet_caps: None,
            routes: crate::routes::Registry::configured("layerx-testnet", "1").unwrap(),
            listen: "127.0.0.1:0".parse().unwrap(),
            listener: crate::Listener::Plain,
            client: crate::Client::without_identity(ca.clone()),
            client_identity: false,
            kernel: Some(crate::Kernel {
                component: Endpoint::parse(&identity.url).unwrap(),
                component_token: Zeroizing::new("service-token".to_owned()),
                public_core: None,
                authority: Endpoint::parse(&identity.url).unwrap(),
                authority_token: Zeroizing::new("service-token".to_owned()),
                identity: Endpoint::parse(&identity.url).unwrap(),
                identity_token: Zeroizing::new("service-token".to_owned()),
                registry: Endpoint::parse(&identity.url).unwrap(),
                registry_token: Zeroizing::new("service-token".to_owned()),
                key_provisioning_key: Zeroizing::new([0; 32]),
                modules: layerx_types::payload::ModuleRegistry::new(&[]).unwrap(),
            }),
            receipt_keys: None,
            paxeer: None,
            capabilities: crate::capabilities::Cache::new(Duration::from_secs(1)),
            indexer: None,
            registration_token: Some(Zeroizing::new("service-token".to_owned())),
            trusted_proxies: Vec::new(),
            faucet: Some(Faucet {
                endpoint: Endpoint::parse(&faucet.url).unwrap(),
                token: Zeroizing::new("service-token".to_owned()),
            }),
            store: Arc::new(crate::RedisStore::new(
                crate::RedisEndpoint::parse("rediss://localhost:6379").unwrap(),
                ca,
                Zeroizing::new("gateway".to_owned()),
                Zeroizing::new(String::new()),
            )),
            event_producer: false,
            network_id: "layerx-testnet".to_owned(),
            wire_version: "1".to_owned(),
            protocol_version: 1,
            protocol_network_id: 1,
            idempotency_seconds: 3600,
        };
        let request = IncomingRequest {
            method: "POST".to_owned(),
            path: "/rpc".to_owned(),
            headers: [(
                "authorization".to_owned(),
                "Bearer session-token".to_owned(),
            )]
            .into(),
            body: Vec::new(),
        };
        let budget_before = *FAUCET_WINDOW.lock().unwrap();
        let answer = dispatch(
            &config,
            &request,
            "px_requestFunds",
            &json!("starter-credit"),
            Some(&json!(["did:layerx:alice", "ab".repeat(32)])),
        )
        .expect("px_requestFunds is dispatched");
        assert_eq!(
            answer,
            refusal(
                &json!("starter-credit"),
                -32001,
                "Starter credit unavailable",
                "starter_credit_amount_unrepresentable",
            )
        );
        assert_eq!(identity.requests.load(Ordering::Acquire), 2);
        assert_eq!(faucet.requests.load(Ordering::Acquire), 0);
        assert_eq!(faucet.claims.load(Ordering::Acquire), 0);
        assert_eq!(*FAUCET_WINDOW.lock().unwrap(), budget_before);
    }

    #[test]
    fn px_request_funds_uses_the_claim_path() {
        let ca = native_tls::Certificate::from_pem(include_bytes!(
            "../../../../../consensus/rpc/jsonrpc/server/test.crt"
        ))
        .unwrap_or_else(|error| panic!("test certificate must parse: {error}"));
        let config = Config {
            wallet_caps: None,
            routes: crate::routes::Registry::configured("layerx-testnet", "1")
                .unwrap_or_else(|error| panic!("test routes must configure: {error}")),
            listen: "127.0.0.1:0"
                .parse()
                .unwrap_or_else(|error| panic!("test address must parse: {error}")),
            listener: crate::Listener::Plain,
            client: crate::Client::without_identity(ca.clone()),
            client_identity: false,
            kernel: None,
            receipt_keys: None,
            paxeer: None,
            capabilities: crate::capabilities::Cache::new(std::time::Duration::from_secs(1)),
            indexer: None,
            registration_token: None,
            trusted_proxies: Vec::new(),
            faucet: None,
            store: std::sync::Arc::new(crate::RedisStore::new(
                crate::RedisEndpoint::parse("rediss://localhost:6379")
                    .unwrap_or_else(|error| panic!("test Redis endpoint must parse: {error}")),
                ca,
                Zeroizing::new("gateway".to_owned()),
                Zeroizing::new(String::new()),
            )),
            event_producer: false,
            network_id: "layerx-testnet".to_owned(),
            wire_version: "1".to_owned(),
            protocol_version: 1,
            protocol_network_id: 1,
            idempotency_seconds: 3600,
        };
        let request = IncomingRequest {
            method: "POST".to_owned(),
            path: "/rpc".to_owned(),
            headers: Default::default(),
            body: Vec::new(),
        };
        let id = json!("claim-1");
        for method in ["px_requestFunds", "lx_requestFunds"] {
            for params in [
                None,
                Some(json!([])),
                Some(json!(["did:layerx:alice"])),
                Some(json!(["did:layerx:alice", "AB".repeat(32)])),
                Some(json!(["did:layerx:alice", "00".repeat(32)])),
                Some(json!(["invalid", "ab".repeat(32)])),
                Some(json!(["did:layerx:alice", "ab".repeat(32), "extra"])),
                Some(json!({"did": "did:layerx:alice"})),
            ] {
                assert_eq!(claim_params(params.as_ref()), Err(-32602));
                assert_eq!(
                    dispatch(&config, &request, method, &id, params.as_ref()),
                    Some(error(&id, -32602, "Invalid params")),
                    "{method}: {params:?}"
                );
            }
            let params = json!(["did:layerx:alice", "ab".repeat(32)]);
            assert_eq!(
                dispatch(&config, &request, method, &id, Some(&params)),
                Some(refusal(
                    &id,
                    -32001,
                    "Faucet claim unavailable",
                    "faucet_not_configured"
                ))
            );
            assert_eq!(
                crate::rpc::dispatch(
                    &config,
                    &request,
                    &json!({"jsonrpc": "2.0", "id": id, "method": method, "params": params})
                ),
                Some(
                    crate::KernelUnavailable::not_configured(crate::KernelBackend::Identity)
                        .rpc(&id)
                )
            );
        }
        assert_eq!(
            dispatch(&config, &request, "px_unknown", &id, Some(&json!([]))),
            None
        );
    }

    fn funded(funding: &str) -> Value {
        json!({
            "funded": true,
            "funding_id": funding,
            "transaction_id": "cd".repeat(32),
            "amount": "1000000",
            "network": "layerx-testnet",
        })
    }

    #[test]
    fn the_claim_idempotency_key_is_a_pinned_domain_separated_digest() {
        let principal = "beta.7399f031b011aa1198718d62c3f79984";
        let did = "did:layerx:alice";
        let key = "ab".repeat(32);
        assert_eq!(
            idempotency(principal, did, &key),
            "6afc28e8efcfabecdefc70c7706e7e44365b5f5446bc1b960821164847c1252a"
        );
        assert_eq!(idempotency(principal, did, &key).len(), 64);
        assert_ne!(
            idempotency(principal, did, &key),
            idempotency("beta.other", did, &key)
        );
        assert_ne!(
            idempotency(principal, did, &key),
            idempotency(principal, "did:layerx:bob", &key)
        );
        assert_ne!(
            idempotency(principal, did, &key),
            idempotency(principal, did, &"cd".repeat(32))
        );
        assert_ne!(
            idempotency("a", "bc", &key),
            idempotency("ab", "c", &key),
            "the digest is length prefixed so field boundaries cannot be shifted"
        );
    }

    #[test]
    fn claim_params_require_one_did_and_one_lowercase_signer_key() {
        let key = "ab".repeat(32);
        assert_eq!(
            claim_params(Some(&json!(["did:layerx:alice", key]))),
            Ok(("did:layerx:alice".to_owned(), key.clone()))
        );
        for args in [
            json!([]),
            json!(["did:layerx:alice"]),
            json!(["did:layerx:alice", key, key]),
            json!(["did:layerx:alice", "AB".repeat(32)]),
            json!(["did:layerx:alice", "zz".repeat(32)]),
            json!(["did:layerx:alice", "ab".repeat(31)]),
            json!(["did:layerx:alice", "00".repeat(32)]),
            json!(["layerx:alice", key]),
            json!(["did:layerx:../bob", key]),
            json!(["did:layerx:a b", key]),
            json!(["did::alice", key]),
            json!(["did:layerx", key]),
            json!(["", key]),
            json!([1, key]),
            json!({"did": "did:layerx:alice"}),
        ] {
            assert_eq!(claim_params(Some(&args)), Err(-32602), "{args}");
        }
        assert_eq!(claim_params(None), Err(-32602));
    }

    #[test]
    fn the_claim_budget_is_bounded_and_resets_with_the_minute() {
        let mut window = (0_u64, 0_u32);
        for _ in 0..CLAIMS_PER_MINUTE {
            assert!(consume(&mut window, 11));
        }
        assert!(!consume(&mut window, 11));
        assert_eq!(window, (11, CLAIMS_PER_MINUTE));
        assert!(consume(&mut window, 12));
        assert_eq!(window, (12, 1));
    }

    #[test]
    fn only_a_key_the_session_authorises_may_be_funded() {
        let key = "ab".repeat(32);
        let other = "cd".repeat(32);
        assert!(bound_signer(std::slice::from_ref(&key), &key));
        assert!(bound_signer(&[other.clone(), key.clone()], &key));
        assert!(bound_signer(&[key.to_ascii_uppercase()], &key));
        assert!(!bound_signer(&[], &key));
        assert!(!bound_signer(&[other], &key));
    }

    #[test]
    fn only_complete_funding_evidence_becomes_a_result() {
        let funding = "ef".repeat(32);
        let accepted = claim_result(
            &json!(4),
            200,
            "application/json",
            funded(&funding).to_string().as_bytes(),
        );
        assert_eq!(accepted["id"], 4);
        assert_eq!(accepted["result"], funded(&funding));
        assert!(accepted.get("error").is_none());

        for incomplete in [
            json!({"funded": false, "funding_id": funding, "transaction_id": "cd", "amount": "1", "network": "layerx-testnet"}),
            json!({"funded": true, "transaction_id": "cd", "amount": "1", "network": "layerx-testnet"}),
            json!({"funded": true, "funding_id": funding, "amount": "1", "network": "layerx-testnet"}),
            json!({"funded": true, "funding_id": funding, "transaction_id": "cd", "network": "layerx-testnet"}),
            json!({"funded": true, "funding_id": funding, "transaction_id": "cd", "amount": "1"}),
            json!({"funded": true, "funding_id": funding, "transaction_id": "cd", "amount": "0", "network": "layerx-testnet"}),
            json!({"funded": true, "funding_id": funding, "transaction_id": "cd", "amount": 1, "network": "layerx-testnet"}),
            json!({"funded": true, "funding_id": "zz", "transaction_id": "cd", "amount": "1", "network": "layerx-testnet"}),
            json!({"funded": "true", "funding_id": funding, "transaction_id": "cd", "amount": "1", "network": "layerx-testnet"}),
        ] {
            let refused = claim_result(
                &json!(4),
                200,
                "application/json",
                incomplete.to_string().as_bytes(),
            );
            assert_eq!(refused["error"]["code"], -32603, "{incomplete}");
            assert!(refused.get("result").is_none());
        }
        for (content_type, body) in [("application/json", "not json"), ("text/html", "{}")] {
            assert_eq!(
                claim_result(&json!(4), 200, content_type, body.as_bytes())["error"]["code"],
                -32603
            );
        }
    }

    #[test]
    fn a_pending_or_refused_claim_is_never_reported_as_funded() {
        let upstream = json!({"state": "still_checking", "retry": "after"});
        let pending = claim_result(
            &json!("f1"),
            202,
            "application/json",
            upstream.to_string().as_bytes(),
        );
        assert_eq!(pending["error"]["code"], -32001);
        assert_eq!(pending["error"]["data"]["state"], "pending");
        assert_eq!(pending["error"]["data"]["code"], "faucet_claim_pending");
        assert_eq!(pending["error"]["data"]["upstream"], upstream);
        assert!(pending.get("result").is_none());

        for (status, code) in [
            (400_u16, -32602_i32),
            (409, -32602),
            (401, -32002),
            (403, -32002),
            (404, -32002),
            (429, -32005),
            (503, -32001),
            (500, -32001),
        ] {
            let upstream = json!({"error": {"code": "identity_quota", "retry": "after"}});
            let refused = claim_result(
                &json!("f1"),
                status,
                "application/json",
                upstream.to_string().as_bytes(),
            );
            assert_eq!(refused["error"]["code"], code, "{status}");
            assert_eq!(refused["error"]["data"], upstream);
            assert!(refused.get("result").is_none());
        }
    }

    #[test]
    fn the_service_claim_request_matches_the_faucet_contract() {
        let principal = "beta.7399f031b011aa1198718d62c3f79984";
        let key = "ab".repeat(32);
        assert_eq!(
            claim_request(principal, "did:layerx:alice", &key),
            format!(
                "{{\"did\":\"did:layerx:alice\",\"principal\":\"{principal}\",\"public_key\":\"{key}\"}}"
            )
        );
        let request: Value =
            serde_json::from_str(&claim_request(principal, "did:layerx:alice", &key))
                .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(request.as_object().map(serde_json::Map::len), Some(3));
        assert!(path_safe(principal, 512));
        assert!(!path_safe("", 512));
        assert!(!path_safe(&"a".repeat(513), 512));
        assert!(lowercase_hex32(&key));
    }

    #[test]
    fn the_starter_credit_grant_is_only_accepted_for_the_session_subject() {
        let sub = "beta.7399f031b011aa1198718d62c3f79984";
        let grant = json!({"sub": sub, "grant_id": "ab".repeat(16), "asset": "PAX", "amount": "10000000000000000000", "granted_at": 1, "settled_at": null});
        let accepted = grant_result(
            &json!(1),
            sub,
            200,
            "application/json",
            grant.to_string().as_bytes(),
        )
        .unwrap_or_else(|refused| panic!("{refused}"));
        assert_eq!(accepted.grant_id, "ab".repeat(16));
        assert_eq!(accepted.amount, "10000000000000000000");
        for divergent in [
            json!({"sub": "beta.other", "grant_id": "ab".repeat(16), "amount": "10000000000000000000"}),
            json!({"sub": sub, "grant_id": "zz".repeat(16), "amount": "10000000000000000000"}),
            json!({"sub": sub, "grant_id": "ab".repeat(16), "amount": "0"}),
            json!({"sub": sub, "amount": "10000000000000000000"}),
        ] {
            let refused = grant_result(
                &json!(1),
                sub,
                200,
                "application/json",
                divergent.to_string().as_bytes(),
            )
            .err()
            .unwrap_or_else(|| panic!("{divergent}"));
            assert_eq!(refused["error"]["code"], -32603, "{divergent}");
        }
        for (status, code) in [
            (403_u16, -32002_i32),
            (409, -32002),
            (429, -32005),
            (503, -32001),
        ] {
            let upstream =
                json!({"error": {"code": "starter_credit_already_claimed", "retry": "never"}});
            let refused = grant_result(
                &json!(1),
                sub,
                status,
                "application/json",
                upstream.to_string().as_bytes(),
            )
            .err()
            .unwrap_or_else(|| panic!("{status}"));
            assert_eq!(refused["error"]["code"], code, "{status}");
            assert_eq!(refused["error"]["data"], upstream);
        }
    }

    #[test]
    fn only_a_grant_the_faucet_can_fund_exactly_is_sent_to_the_faucet() {
        for amount in ["1", "1000000", "9007199254740991"] {
            assert!(faucet_representable(amount), "{amount}");
        }
        for amount in [
            "10000000000000000000",
            "9007199254740992",
            "18446744073709551616",
            "01000000",
            "",
            "-1",
            "+1",
        ] {
            assert!(!faucet_representable(amount), "{amount}");
        }
    }

    #[test]
    fn faucet_refusals_carry_a_machine_readable_reason() {
        for (code, reason) in [
            (-32001, "faucet_not_configured"),
            (-32002, "session_required"),
            (-32002, "signer_not_bound"),
            (-32005, "faucet_rate_limit"),
            (-32001, "faucet_unavailable"),
            (-32001, "identity_unavailable"),
            (-32001, "starter_credit_amount_unrepresentable"),
            (-32603, "starter_credit_amount_mismatch"),
        ] {
            let refused = refusal(&json!(1), code, "Faucet claim unavailable", reason);
            assert_eq!(refused["error"]["code"], code);
            assert_eq!(refused["error"]["data"]["code"], reason);
            assert!(refused.get("result").is_none());
        }
    }
}
