use serde_json::{json, Value};
use std::fs;
use std::io::{Cursor, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use layerx_platform_gateway::http::read_request;
use layerx_platform_gateway::{unified_service_catalogue, unified_service_route};

const WALLET: &str = include_str!("../../../../../services/wallet/gateway/src/routes/wallet.ts");
const SIGN: &str = include_str!("../../../../../services/wallet/gateway/src/routes/sign.ts");
const OWNER: &str = include_str!("../../../../../services/wallet/gateway/src/index.ts");
const SDK_PROVIDER: &str = include_str!("../../../../../sdk/wallet/src/provider.ts");
const SDK_WALLET: &str = include_str!("../../../../../sdk/wallet/src/index.ts");
const SDK_GAS: &str = include_str!("../../../../../sdk/wallet/src/modules/gas-station.ts");

#[test]
fn served_catalogue_lists_px_register_and_no_retired_lx_routes() {
    let routes = unified_service_catalogue()["routes"].as_array().unwrap_or_else(|| panic!("{}", "production routes"));
    for route in routes {
        assert_ne!(route["path"], "lx_register", "retired route: {route}");
        assert_ne!(route["path"], "lx_getHistory", "retired route: {route}");
    }
    assert!(routes.iter().any(|route| { route["id"] == "kernel:POST:px_register" && route["path"] == "px_register" }));
    assert!(routes.iter().any(|route| { route["id"] == "indexer:POST:px_getHistory" && route["path"] == "px_getHistory" }));
}

fn operation(method: &str, target: &str, source: &str) {
    let entry = unified_service_route(method, target).unwrap_or_else(|| panic!("{}", "actual unified service operation"));
    assert_eq!(entry["service"], "wallet-gateway");
    assert_eq!(entry["upstream"], "wallet-gateway");
    assert_eq!(entry["authentication"], "requireAuth");
    assert_eq!(entry["health_predicate"], "service-ready-and-network-version-bound");
    assert_eq!(entry["retries"], 0);
    assert_eq!(entry["proxy"], true);
    assert_eq!(entry["source"], source);
    let routes = unified_service_catalogue()["routes"].as_array().unwrap_or_else(|| panic!("{}", "production routes"));
    assert_eq!(routes.iter().filter(|route| route["method"] == method && route["path"] == entry["path"]).count(), 1);
}

#[test]
fn real_registered_wallet_handlers_have_the_same_unified_owner() {
    assert!(OWNER.contains("app.register(walletSponsorRoutes)"));
    assert!(OWNER.contains("app.register(signRoutes)"));
    assert!(SIGN.contains("await lxApprovalRoutes(app, pool, attestors, limiter)"));
    let wallet: String = WALLET.chars().filter(|value| !value.is_whitespace()).collect();
    let sign: String = SIGN.chars().filter(|value| !value.is_whitespace()).collect();
    for name in ["submit", "status"] {
        let path = format!("/v1/wallet/sponsored/{name}");
        assert!(wallet.contains(&format!("app.post('{path}',{{preHandler:requireAuth}}")));
        operation("POST", &path, "services/wallet/gateway/src/routes/wallet.ts");
    }
    for path in ["/v1/wallet/sign-digest", "/v1/wallet/lx/review", "/v1/wallet/lx/approve", "/v1/wallet/lx/sign"] {
        assert!(sign.contains(&format!("app.post('{path}',{{preHandler:requireAuth")));
        operation("POST", path, "services/wallet/gateway/src/routes/sign.ts");
    }
    for (method, route, target) in [
        ("POST", "/v1/wallet/sign-custody", "/v1/wallet/sign-custody".to_owned()),
        ("GET", "/v1/wallet/custody/:id", format!("/v1/wallet/custody/0x{}", "ab".repeat(32))),
        ("GET", "/v1/wallet/custody-authorization/:id", format!("/v1/wallet/custody-authorization/0x{}", "ab".repeat(32))),
    ] {
        assert!(!sign.contains(&format!("app.{}('{route}'", method.to_lowercase())));
        assert!(unified_service_route(method, &target).is_none(), "retired {method} {target}");
        assert!(unified_service_catalogue()["routes"].as_array().unwrap_or_else(|| panic!("value required")).iter().all(|entry| entry["path"] != route));
    }
    operation("GET", "/v1/wallet/lx/approvals/12345678-1234-4234-8234-123456789abc", "services/wallet/gateway/src/routes/sign.ts");

    assert!(sign.contains("app.get('/v1/wallet/lx/approvals/:id',{preHandler:requireAuth"));
}

#[test]
fn actual_sdk_callers_and_existing_ownership_checks_stay_connected() {
    assert!(SDK_PROVIDER.contains("/v1/wallet/sign-digest"));
    for path in ["/v1/wallet/sign-custody", "/v1/wallet/custody/", "/v1/wallet/custody-authorization/"] {
        assert!(!SDK_PROVIDER.contains(path), "retired SDK caller {path}");
    }
    for path in ["/v1/wallet/lx/review", "/v1/wallet/lx/approve", "/v1/wallet/lx/sign", "/v1/wallet/lx/approvals/"] {
        assert!(!SDK_WALLET.contains(path), "retired LX SDK caller {path}");
    }
    for path in ["/v1/wallet/evm/review", "/v1/wallet/evm/approve", "/v1/wallet/evm/approvals/"] {
        assert!(SDK_WALLET.contains(path), "actual EVM SDK caller {path}");
    }
    assert!(SDK_GAS.contains("/v1/wallet/sponsored/status"));
    assert!(SDK_GAS.contains("/v1/wallet/sponsored/submit"));
    let wallet: String = WALLET.chars().filter(|value| !value.is_whitespace()).collect();
    assert!(wallet.contains("wallet.address.toLowerCase()!==account.toLowerCase()"));
    assert!(!SIGN.contains("where id=$1 and user_id=$2"));
    assert!(SIGN.contains("where id=$1 and principal=$2 for update"));
    assert!(SIGN.contains("explicit_lx_approval_required"));
    assert!(SIGN.contains("attestor_custody_required"));
}

#[test]
fn newly_connected_financial_routes_are_closed_to_methods_queries_and_path_aliases() {
    let custody = format!("/v1/wallet/custody/0x{}", "ab".repeat(32));
    for (method, path) in [
        ("GET", "/v1/wallet/sign-digest"),
        ("PUT", "/v1/wallet/lx/approve"),
        ("DELETE", "/v1/wallet/sponsored/submit"),
        ("POST", custody.as_str()),
        ("GET", "/v1/wallet/lx/approvals/not-a-uuid"),
        ("GET", "/v1/wallet/custody/0x11"),
        ("POST", "/v1/wallet/lx/sign?tenant=other"),
        ("POST", "/v1/wallet/lx/sign/"),
        ("POST", "/v1/wallet/lx/%73ign"),
        ("POST", "/v1/wallet/lx/../sign"),
        ("POST", "/v1/wallet/sponsored/unknown"),
        ("GET", "/v1/wallet/lx/approvals/12345678-1234-4234-8234-123456789abc?principal=other"),
        ("GET", "/v1/sync/readiness?ready=true"),
        ("POST", "/v1/sync/readiness"),
        ("POST", "/internal/v1/wallet/sign-digest"),
        ("POST", "/v1/wallet/sign-digest?x=%0d%0aHost:other"),
    ] {
        assert!(unified_service_route(method, path).is_none(), "closed {method} {path}");
    }
}

#[test]
fn real_ingress_retains_the_principal_credential_and_exact_financial_body() {
    let body = br#"{"approval_id":"12345678-1234-4234-8234-123456789abc"}"#;
    let head = format!(
        "POST /v1/wallet/lx/sign HTTP/1.1\r\nHost: api-mainnet-beta.paxeer.network\r\nAuthorization: Bearer principal-session\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n",
        body.len()
    );
    let wire = [head.as_bytes(), body.as_slice()].concat();
    let request = read_request(&mut Cursor::new(wire), 4096).unwrap_or_else(|error| panic!("{}: {error:?}", "real production ingress"));
    assert_eq!(request.headers.get("authorization").map(String::as_str), Some("Bearer principal-session"));
    assert_eq!(request.body, body);
    operation(&request.method, &request.path, "services/wallet/gateway/src/routes/sign.ts");
}

#[test]
fn archive_sync_readiness_reaches_the_actual_sync_predicate() {
    let entry = unified_service_route("GET", "/v1/sync/readiness").unwrap_or_else(|| panic!("{}", "actual archive sync readiness"));
    assert_eq!(entry["service"], "archive");
    assert_eq!(entry["upstream"], "archive");
    assert_eq!(entry["authentication"], "public");
    assert_eq!(entry["source"], "services/platform/relay_archive/runtime.py");
    assert!(unified_service_route("GET", "/v1/sync/network").is_some());
    assert!(unified_service_route("GET", "/v1/history/activities?limit=10&cursor=0").is_some());
}

fn run(program: &str, arguments: &[&str]) {
    let output = Command::new(program).args(arguments).output().unwrap_or_else(|error| panic!("{program} must run: {error}"));
    assert!(output.status.success(), "{program} failed: {}", String::from_utf8_lossy(&output.stderr));
}

fn text(value: &Path) -> &str {
    value.to_str().unwrap_or_else(|| panic!("test path must be UTF-8"))
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0").and_then(|listener| listener.local_addr()).unwrap_or_else(|error| panic!("test port must be allocated: {error}")).port()
}

/// The real gateway binary over a real TLS Redis, answering JSON-RPC on a
/// plain loopback listener with no kernel, indexer or Paxeer upstream.
struct Gateway {
    process: Child,
    redis: Child,
    directory: PathBuf,
    port: u16,
    redis_port: u16,
}

impl Drop for Gateway {
    fn drop(&mut self) {
        let _ = self.process.kill();
        let _ = self.process.wait();
        let _ = self.redis.kill();
        let _ = self.redis.wait();
        let _ = fs::remove_dir_all(&self.directory);
    }
}

impl Gateway {
    fn start() -> Self {
        Self::start_with_explorer(None)
    }

    fn start_with_explorer(explorer: Option<&TcpListener>) -> Self {
        let directory = std::env::temp_dir().join(format!("layerx-unified-paths-{}-{}", std::process::id(), SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_nanos()));
        fs::create_dir(&directory).unwrap_or_else(|error| panic!("test directory must be created: {error}"));
        let file = |name: &str| directory.join(name);
        if let Some(explorer) = explorer {
            configure_explorer(&directory, explorer);
        }
        run(
            "openssl",
            &[
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                text(&file("server.key")),
                "-out",
                text(&file("server.pem")),
                "-days",
                "1",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost",
            ],
        );
        run("openssl", &["x509", "-in", text(&file("server.pem")), "-outform", "DER", "-out", text(&file("server.der"))]);
        let redis_port = free_port();
        fs::write(file("users.acl"), "user default off\nuser gateway on >gateway-secret ~* &* +@all\n").unwrap_or_else(|error| panic!("test Redis ACL must be written: {error}"));
        fs::write(
            file("redis.conf"),
            format!(
                "bind 127.0.0.1\nport 0\ntls-port {redis_port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\ndir {}\nprotected-mode yes\n",
                text(&file("server.pem")),
                text(&file("server.key")),
                text(&file("server.pem")),
                text(&file("users.acl")),
                text(&directory),
            ),
        )
        .unwrap_or_else(|error| panic!("test Redis config must be written: {error}"));
        let redis = Command::new("redis-server").arg(file("redis.conf")).stdout(Stdio::null()).stderr(Stdio::null()).spawn().unwrap_or_else(|error| panic!("real Redis server must start: {error}"));
        for (name, value) in [("redis-username", "gateway"), ("redis-password", "gateway-secret")] {
            fs::write(file(name), value).unwrap_or_else(|error| panic!("{name} must be written: {error}"));
        }
        let port = free_port();
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-gateway"));
        command
            .env_clear()
            .env("PATH", std::env::var_os("PATH").unwrap_or_default())
            .env("LAYERX_GATEWAY_LISTEN", format!("127.0.0.1:{port}"))
            .env("LAYERX_GATEWAY_LISTENER", "plain")
            .env("LAYERX_GATEWAY_OUTBOUND_CA_DER", file("server.der"))
            .env("LAYERX_GATEWAY_NETWORK_ID", "unified-service-paths")
            .env("LAYERX_GATEWAY_LXP_WIRE_VERSION", layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string())
            .env("LAYERX_GATEWAY_PROTOCOL_NETWORK_ID", "7")
            .env("LAYERX_GATEWAY_REDIS_URL", format!("rediss://localhost:{redis_port}"))
            .env("LAYERX_GATEWAY_REDIS_USERNAME_FILE", file("redis-username"))
            .env("LAYERX_GATEWAY_REDIS_PASSWORD_FILE", file("redis-password"))
            .stdout(Stdio::null())
            .stderr(Stdio::null());
        if explorer.is_some() {
            command.env("LAYERX_GATEWAY_ROUTE_BINDINGS_FILE", file("route-bindings.json"));
        }
        let gateway = command.spawn().unwrap_or_else(|error| panic!("gateway must start: {error}"));
        let mut started = Self { process: gateway, redis, directory, port, redis_port };
        for _ in 0..200 {
            if let Ok(Some(status)) = started.process.try_wait() {
                panic!("gateway refused start-up: {status}");
            }
            if started.exchange("GET", "/livez", b"").is_ok_and(|(status, _)| status == 200) {
                return started;
            }
            thread::sleep(Duration::from_millis(50));
        }
        panic!("gateway did not become live")
    }

    fn exchange(&self, method: &str, path: &str, body: &[u8]) -> Result<(u16, Vec<u8>), String> {
        let mut stream = TcpStream::connect(("127.0.0.1", self.port)).map_err(|error| error.to_string())?;
        stream.set_read_timeout(Some(Duration::from_secs(30))).map_err(|error| error.to_string())?;
        let mut request = format!("{method} {path} HTTP/1.1\r\nHost: localhost\r\nAccept: application/json\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).into_bytes();
        request.extend_from_slice(body);
        stream.write_all(&request).map_err(|error| error.to_string())?;
        let mut answer = Vec::new();
        stream.read_to_end(&mut answer).map_err(|error| error.to_string())?;
        let end = answer.windows(4).position(|window| window == b"\r\n\r\n").ok_or_else(|| "answer has no header terminator".to_owned())?;
        let head = String::from_utf8_lossy(&answer[..end]).into_owned();
        let status = head.split_whitespace().nth(1).and_then(|value| value.parse::<u16>().ok()).ok_or_else(|| format!("answer status is invalid: {head}"))?;
        Ok((status, answer[end + 4..].to_vec()))
    }

    fn call(&self, method: &str, params: &Value) -> Value {
        let request = json!({"jsonrpc": "2.0", "id": 9, "method": method, "params": params});
        let (status, body) = self.exchange("POST", "/", request.to_string().as_bytes()).unwrap_or_else(|error| panic!("{method} must be answered: {error}"));
        assert_eq!(status, 200, "{method}");
        let answer: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{method} answer must be JSON: {error}"));
        assert_eq!(answer["id"], 9, "{answer}");
        answer
    }

    fn assert_retired_paths(&self) {
        for path in ["/rpc", "/rpc/schema", "/rpc/ws", "/rpc/evm/ws"] {
            for method in ["GET", "POST"] {
                let (status, body) = self.exchange(method, path, b"{}").unwrap_or_else(|error| panic!("{}: {error:?}", "retired route response"));
                assert_eq!(status, 404, "{method} {path}");
                let body: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "retired route refusal"));
                assert_eq!(body["error"]["code"], "not_found");
            }
        }
    }

    fn websocket(&self) -> TcpStream {
        use layerx_platform_gateway::gateway_digest;
        use layerx_platform_gateway::store::{KeyRecord, RedisEndpoint, RedisStore};
        use zeroize::Zeroizing;

        let ca = native_tls::Certificate::from_der(&fs::read(self.directory.join("server.der")).unwrap_or_else(|error| panic!("{}: {error:?}", "outbound CA"))).unwrap_or_else(|error| panic!("{}: {error:?}", "outbound certificate"));
        let store = RedisStore::new(
            RedisEndpoint::parse(&format!("rediss://localhost:{}", self.redis_port)).unwrap_or_else(|error| panic!("{}: {error:?}", "Redis endpoint")),
            ca,
            Zeroizing::new("gateway".into()),
            Zeroizing::new("gateway-secret".into()),
        );
        let secret = format!("lxp_live_{}", "34".repeat(32));
        let record = KeyRecord {
            key_id: "lx-refusal-reader".into(),
            principal_digest: gateway_digest(&[b"lx-refusal-reader"]),
            salt: "lx-refusal-salt".into(),
            secret_digest: gateway_digest(&[b"gateway-key-v1", b"lx-refusal-salt", secret.as_bytes()]),
            signer_public_key: "11".repeat(32),
            scopes: "receipt:read".into(),
            quota_requests: 100,
            quota_window_seconds: 60,
            epoch: 1,
            disabled: false,
        };
        store.issue_key(&record, "issue-lx-refusal-reader").unwrap_or_else(|error| panic!("{}: {error:?}", "issue WebSocket reader key"));
        let mut stream = TcpStream::connect(("127.0.0.1", self.port)).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket connect"));
        stream.set_read_timeout(Some(Duration::from_secs(5))).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket timeout"));
        write!(
            stream,
            "GET / HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nAuthorization: LayerX-Key {}:{secret}\r\n\r\n",
            record.key_id
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket upgrade"));
        let mut headers = Vec::new();
        while !headers.ends_with(b"\r\n\r\n") {
            let mut byte = [0];
            stream.read_exact(&mut byte).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket upgrade response"));
            headers.push(byte[0]);
            assert!(headers.len() <= 4096, "bounded WebSocket upgrade");
        }
        let headers = String::from_utf8(headers).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket response headers"));
        assert!(headers.starts_with("HTTP/1.1 101 "), "{headers}");
        assert!(headers.contains("Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo="), "{headers}");
        stream
    }
}

fn websocket_call(stream: &mut TcpStream, method: &str, params: &Value) -> Value {
    let body = serde_json::to_vec(&json!({"jsonrpc": "2.0", "id": 9, "method": method, "params": params})).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket request"));
    let mut frame = vec![0x81];
    if body.len() < 126 {
        frame.push(0x80 | u8::try_from(body.len()).unwrap_or_else(|error| panic!("{}: {error:?}", "short frame length")));
    } else {
        frame.push(0x80 | 126);
        frame.extend_from_slice(&u16::try_from(body.len()).unwrap_or_else(|error| panic!("{}: {error:?}", "frame length")).to_be_bytes());
    }
    let mut mask = [0; 4];
    getrandom::fill(&mut mask).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket mask"));
    frame.extend_from_slice(&mask);
    frame.extend(body.iter().enumerate().map(|(i, byte)| byte ^ mask[i % 4]));
    stream.write_all(&frame).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket request write"));
    let mut header = [0; 2];
    stream.read_exact(&mut header).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket response header"));
    assert_eq!(header[0], 0x81, "text response for {method}");
    assert_eq!(header[1] & 0x80, 0, "server response is unmasked");
    let length = match header[1] {
        126 => {
            let mut length = [0; 2];
            stream.read_exact(&mut length).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket response length"));
            usize::from(u16::from_be_bytes(length))
        }
        127 => panic!("unbounded WebSocket refusal"),
        length => usize::from(length),
    };
    let mut body = vec![0; length];
    stream.read_exact(&mut body).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket response body"));
    let answer: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "WebSocket JSON response"));
    assert_eq!(answer["id"], 9, "{answer}");
    answer
}

const ACCOUNT: &str = "0x00000000000000000000000000000000000000ab";

#[test]
fn register_replaced() {
    let gateway = Gateway::start();
    gateway.assert_retired_paths();
    let signer = "ab".repeat(32);
    let signature = "cd".repeat(64);
    for params in [json!([signer, signature]), json!([signer, signature, "someone@example.com"]), json!([])] {
        let answer = gateway.call("lx_register", &params);
        assert!(answer.get("result").is_none(), "{answer}");
        assert_eq!(answer["error"]["code"], -32601, "{answer}");
    }
    let served = gateway.call("px_register", &json!([signer, signature]));
    assert!(served.get("result").is_none(), "{served}");
    assert_eq!(served["error"]["code"], -32002, "{served}");
    let schema = include_bytes!("../openrpc.json");
    let schema: Value = serde_json::from_slice(schema).unwrap_or_else(|error| panic!("schema JSON: {error}"));
    let names: Vec<&str> = schema["methods"].as_array().unwrap_or_else(|| panic!("schema methods")).iter().filter_map(|method| method["name"].as_str()).collect();
    assert!(!names.contains(&"lx_register"), "{names:?}");
    assert!(!names.contains(&"lx_getHistory"), "{names:?}");
    assert!(names.contains(&"px_getHistory"), "{names:?}");
    assert!(names.contains(&"px_register"), "{names:?}");
}

#[test]
fn explorer_program_paths_are_not_proxied() {
    let explorer = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{}: {error:?}", "explorer listener must bind"));
    explorer.set_nonblocking(true).unwrap_or_else(|error| panic!("{}: {error:?}", "explorer listener must be nonblocking"));
    let gateway = Gateway::start_with_explorer(Some(&explorer));
    let program = "ab".repeat(32);
    for path in [format!("/v1/programs/{program}"), format!("/v1/programs/{program}/reads/resolve")] {
        let (status, body) = gateway.exchange("GET", &path, b"").unwrap_or_else(|error| panic!("{path} must be answered: {error}"));
        let answer: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{path} answer must be JSON: {error}"));
        assert_eq!(status, 404, "{path} {answer}");
        assert_eq!(answer["error"]["code"], "not_found", "{path} {answer}");
        match explorer.accept() {
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
            Err(error) => panic!("explorer connection check failed: {error}"),
            Ok(_) => panic!("{path} reached the explorer upstream"),
        }
    }
}

#[test]
fn lx_reads_switched_off() {
    let gateway = Gateway::start();
    gateway.assert_retired_paths();
    let mut websocket = gateway.websocket();
    let key = "ab".repeat(32);
    let did = format!("did:layerx:{key}");
    for (method, params) in [
        ("lx_getHistory", json!([key])),
        ("lx_getHistory", json!([ACCOUNT])),
        ("lx_getSequence", json!([did, "identity"])),
        ("lx_getAccount", json!([key])),
        ("lx_getBalance", json!([key])),
        ("lx_getBalances", json!([did])),
        ("lx_getReceipt", json!([key])),
        ("lx_getActivityStatus", json!([key])),
        ("lx_getBatchHeader", json!(["1"])),
        ("lx_getCheckpoint", json!([key])),
        ("lx_getProof", json!(["activity", key])),
        ("lx_listAssets", json!([])),
        ("lx_getAsset", json!([key])),
        ("lx_getNodeInfo", json!([])),
        ("lx_requestFunds", json!([key])),
        ("lx_sendActivity", json!(["00", "executed"])),
        ("lx_estimateFee", json!(["00"])),
        ("lx_getProgramEvents", json!([{"topic": "ab", "from_sequence": 0, "limit": 1}])),
        ("lx_getWalletCaps", json!([])),
        ("lx_subscribe", json!(["receipts"])),
        ("lx_unsubscribe", json!(["retired-subscription"])),
        ("lx_subscription", json!([])),
        ("lx_unknown", json!({"invalid": true})),
    ] {
        let answer = gateway.call(method, &params);
        assert!(answer.get("result").is_none(), "{method} {answer}");
        assert_eq!(answer["error"]["code"], -32601, "{method} {answer}");
        assert_eq!(answer["error"]["message"], "Method not found", "{method} {answer}");
        assert_eq!(websocket_call(&mut websocket, method, &params), answer, "WebSocket {method}");
    }
    for method in ["px_getHistory", "px_getUnifiedHistory"] {
        let answer = gateway.call(method, &json!([ACCOUNT]));
        assert_ne!(answer["error"]["code"], -32601, "{method} {answer}");
        assert_ne!(answer["error"]["code"], -32602, "{method} {answer}");
    }
    let schema = include_bytes!("../openrpc.json");
    let schema: Value = serde_json::from_slice(schema).unwrap_or_else(|error| panic!("{}: {error:?}", "served OpenRPC JSON"));
    assert!(schema["methods"]
        .as_array()
        .unwrap_or_else(|| panic!("{}", "schema methods"))
        .iter()
        .all(|method| !method["name"].as_str().unwrap_or_else(|| panic!("{}", "method name")).starts_with("lx_")));
}

fn configure_explorer(directory: &Path, explorer: &TcpListener) {
    let file = |name: &str| directory.join(name);
    let authorization = file("explorer-authorization");
    fs::write(&authorization, format!("Bearer {}", "ab".repeat(32))).unwrap_or_else(|error| panic!("explorer authority must be written: {error}"));
    fs::set_permissions(&authorization, fs::Permissions::from_mode(0o600)).unwrap_or_else(|error| panic!("explorer authority must be protected: {error}"));
    let bindings = file("route-bindings.json");
    fs::write(
        &bindings,
        json!({
            "schema": "paxeer-x.route-bindings.v1",
            "chain_id": 125,
            "network_id": "unified-service-paths",
            "wire_version": layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
            "allowed_origins": [],
            "services": {},
            "upstreams": {
                "layerx-explorer": {
                    "service": "explorer",
                    "binding": {
                        "url": format!("https://localhost:{}", explorer.local_addr().unwrap_or_else(|error| panic!("{}: {error:?}", "explorer address")).port()),
                        "health_path": "/v1/readiness",
                        "identity_path": "/v1/identity",
                        "health_authorization_file": authorization,
                        "ready_pointer": "/complete",
                        "ready_value": true,
                        "network_pointer": "/network_id",
                        "network_value": "unified-service-paths",
                        "version_pointer": "/wire_version",
                        "version_value": layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string()
                    }
                }
            },
            "passthrough": []
        })
        .to_string(),
    )
    .unwrap_or_else(|error| panic!("explorer binding must be written: {error}"));
    fs::set_permissions(&bindings, fs::Permissions::from_mode(0o600)).unwrap_or_else(|error| panic!("explorer binding must be protected: {error}"));
}
