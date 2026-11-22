use super::{
    json_response, now, parse_hex32, response, upstream_json, Config, KernelBackend,
    KernelUnavailable, OutgoingResponse,
};
use std::cell::RefCell;
use std::collections::HashMap;
use std::net::{IpAddr, SocketAddr};
use std::sync::Mutex;

#[derive(Default)]
pub(super) struct ReadWindows {
    second: u64,
    total: u32,
    per_client: HashMap<String, u32>,
}

const READS_PER_SECOND: u32 = 120;
const READS_PER_CLIENT_PER_SECOND: u32 = 30;

thread_local! {
    static CLIENT: RefCell<String> = RefCell::new("unattributed".to_owned());
}

pub(super) fn bind_client(
    proxies: &[(IpAddr, u8)],
    peer: Option<SocketAddr>,
    forwarded: Option<&str>,
) {
    CLIENT.with(|cell| {
        *cell.borrow_mut() = super::rpc_register::client_ip(proxies, peer, forwarded);
    });
}

fn spend(windows: &mut ReadWindows, second: u64, client: &str) -> bool {
    if windows.second != second {
        windows.second = second;
        windows.total = 0;
        windows.per_client.clear();
    }
    if windows.total >= READS_PER_SECOND {
        return false;
    }
    let count = windows.per_client.entry(client.to_owned()).or_default();
    if *count >= READS_PER_CLIENT_PER_SECOND {
        return false;
    }
    *count += 1;
    windows.total += 1;
    true
}

pub(super) fn consume_read(read_windows: &Mutex<ReadWindows>) -> bool {
    consume_read_with_clock(read_windows, now)
}

fn consume_read_with_clock(
    read_windows: &Mutex<ReadWindows>,
    clock: fn() -> Result<u64, String>,
) -> bool {
    let Ok(second) = clock() else { return false };
    let Ok(mut windows) = read_windows.lock() else {
        return false;
    };
    CLIENT.with(|cell| spend(&mut windows, second, &cell.borrow()))
}

pub(super) fn target(path: &str) -> Result<Option<&str>, ()> {
    let parts: Vec<_> = path.split('/').collect();
    match parts.as_slice() {
        ["", "v1", "accounts", id, "balance"] => {
            parse_hex32(id).map_err(|_| ())?;
            Ok(Some(path))
        }
        ["", "v1", "dids", did, "accounts"] => {
            layerx_types::ids::Did::new(did.as_bytes()).map_err(|_| ())?;
            if did
                .bytes()
                .any(|b| !b.is_ascii_alphanumeric() && !b"-._:".contains(&b))
            {
                return Err(());
            }
            Ok(Some(path))
        }
        _ => Ok(None),
    }
}

pub(super) fn read(config: &Config, path: &str) -> OutgoingResponse {
    request(config, "GET", path, &[])
}

pub(super) fn request(config: &Config, method: &str, path: &str, body: &[u8]) -> OutgoingResponse {
    if !consume_read(&config.read_windows) {
        return response(429, "public_read_rate_limit", Some(1));
    }
    let (endpoint, token) = match config.backend(KernelBackend::PublicCore) {
        Ok(target) => target,
        Err(unavailable) => return unavailable.into(),
    };
    let Ok(upstream) = upstream_json(config, endpoint, token, method, path, None, body) else {
        return KernelUnavailable::unreachable(KernelBackend::PublicCore).into();
    };
    if upstream.content_type != "application/json" {
        return response(502, "invalid_core_response", None);
    }
    let Ok(body) = serde_json::from_slice::<serde_json::Value>(&upstream.body) else {
        return response(502, "invalid_core_response", None);
    };
    json_response(upstream.status, &body)
}

pub(super) fn configured_endpoint() -> Result<Option<super::Endpoint>, String> {
    std::env::var("LAYERX_GATEWAY_PUBLIC_CORE_URL")
        .ok()
        .map(|url| super::Endpoint::parse(&url))
        .transpose()
}

/// The chain read path and its bounded, unchanged indexer query.
fn chain_target(target: &str) -> Result<Option<(&str, &str)>, ()> {
    let (path, query) = target.split_once('?').unwrap_or((target, ""));
    let parts: Vec<_> = path.split('/').collect();
    let paged = match parts.as_slice() {
        ["", "v1", "chains"] => false,
        ["", "v1", "chains", id] | ["", "v1", "chains", id, "summary" | "transfers" | "events"] => {
            if id.is_empty()
                || !id.bytes().all(|byte| byte.is_ascii_digit())
                || id.parse::<u32>().is_err()
            {
                return Err(());
            }
            matches!(parts.last().copied(), Some("transfers" | "events"))
        }
        _ => return Ok(None),
    };
    if (paged && (query.len() > 512 || !super::http::query_is_canonical(query)))
        || (!paged && !query.is_empty())
    {
        return Err(());
    }
    Ok(Some((path, query)))
}

fn read_chain(
    config: &Config,
    path: &str,
    query: &str,
    clock: fn() -> Result<u64, String>,
) -> OutgoingResponse {
    if !consume_read_with_clock(&config.read_windows, clock) {
        return response(429, "public_read_rate_limit", Some(1));
    }
    let Some(indexer) = config.indexer.as_ref() else {
        return response(503, "indexer_not_configured", None);
    };
    let source = super::history::Source {
        indexer,
        client: Some(&config.client),
    };
    match source.get(path, query) {
        Ok((status, body)) => json_response(status, &body),
        Err("invalid_indexer_response") => response(502, "invalid_indexer_response", None),
        Err(code) => response(503, code, None),
    }
}

pub(super) fn route(config: &Config, request: &super::IncomingRequest) -> Option<OutgoingResponse> {
    route_with_clock(config, request, now)
}

fn route_with_clock(
    config: &Config,
    request: &super::IncomingRequest,
    clock: fn() -> Result<u64, String>,
) -> Option<OutgoingResponse> {
    match chain_target(&request.path) {
        Ok(None) => {}
        target => {
            return Some(if request.method != "GET" {
                response(405, "method_not_allowed", None)
            } else {
                match target {
                    Ok(Some((path, query))) => read_chain(config, path, query, clock),
                    _ => response(400, "invalid_read_selector", None),
                }
            })
        }
    }
    if matches!(request.path.as_str(), "/" | "/openrpc.json") {
        return Some(super::rpc::route(config, request));
    }
    match target(&request.path) {
        Ok(Some(path)) => Some(if request.method == "GET" {
            read(config, path)
        } else {
            response(405, "method_not_allowed", None)
        }),
        Err(()) => Some(response(400, "invalid_read_selector", None)),
        Ok(None) => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn one_client_exhausting_its_budget_leaves_reads_for_another() {
        let mut windows = ReadWindows {
            second: 0,
            total: 0,
            per_client: HashMap::new(),
        };
        let second = 42;
        let client_a = "192.0.2.1";
        let client_b = "192.0.2.2";

        for _ in 0..READS_PER_CLIENT_PER_SECOND {
            assert!(spend(&mut windows, second, client_a));
        }
        assert!(!spend(&mut windows, second, client_a));
        assert_eq!(windows.per_client[client_a], READS_PER_CLIENT_PER_SECOND);
        assert_eq!(windows.total, READS_PER_CLIENT_PER_SECOND);
        assert!(spend(&mut windows, second, client_b));

        let mut granted = READS_PER_CLIENT_PER_SECOND + 1;
        for index in 0..READS_PER_SECOND {
            let client = format!("198.51.100.{index}");
            for _ in 0..=READS_PER_CLIENT_PER_SECOND {
                if spend(&mut windows, second, &client) {
                    granted += 1;
                }
                assert_eq!(windows.total, granted);
                assert!(windows.total <= READS_PER_SECOND);
            }
        }
        assert_eq!(granted, READS_PER_SECOND);
        assert!(windows
            .per_client
            .values()
            .all(|&count| count <= READS_PER_CLIENT_PER_SECOND));
        assert!(!spend(&mut windows, second, client_b));

        for _ in 0..READS_PER_CLIENT_PER_SECOND {
            assert!(spend(&mut windows, second + 1, client_a));
        }
        assert_eq!(windows.second, second + 1);
        assert_eq!(windows.total, READS_PER_CLIENT_PER_SECOND);
        assert_eq!(windows.per_client.len(), 1);
        assert!(!spend(&mut windows, second + 1, client_a));
        assert!(spend(&mut windows, second + 1, client_b));
    }

    #[test]
    fn public_read_routes_validate_identifiers_and_exact_paths() {
        let id = "ab".repeat(32);
        let path = format!("/v1/accounts/{id}/balance");
        assert_eq!(target(&path), Ok(Some(path.as_str())));
        assert_eq!(
            target("/v1/dids/did:layerx:alice/accounts"),
            Ok(Some("/v1/dids/did:layerx:alice/accounts"))
        );
        for path in [
            "/v1/accounts/123/balance",
            "/v1/dids/did:layerx:alice?admin/accounts",
            "/v1/dids//accounts",
        ] {
            assert!(target(path).is_err(), "{path}");
        }
        for path in [
            "/v1/state",
            "/v1/state?x=1",
            "/v1/accounts",
            "/v1/activities",
            "/v1/dids/did:layerx:alice/accounts/extra",
        ] {
            assert_eq!(target(path), Ok(None));
        }
    }
}

#[cfg(test)]
mod chain_tests {
    use super::*;
    use crate::{history::Indexer, http, IncomingRequest};
    use layerx_indexer::chains::ChainRecord;
    use layerx_indexer::store::{EventRow, Store, TransferRow, Unit};
    use serde_json::{json, Value};
    use std::collections::BTreeMap;
    use std::net::TcpListener;
    use std::sync::Arc;

    const PATHS: [&str; 5] = [
        "/v1/chains",
        "/v1/chains/1",
        "/v1/chains/1/summary",
        "/v1/chains/1/transfers",
        "/v1/chains/1/events",
    ];

    fn config(indexer: Option<Indexer>) -> Config {
        let ca = native_tls::Certificate::from_pem(include_bytes!(
            "../../../../../consensus/rpc/jsonrpc/server/test.crt"
        ))
        .unwrap_or_else(|error| panic!("test certificate: {error}"));
        Config {
            routes: crate::routes::Registry::configured("layerx-testnet", "1")
                .unwrap_or_else(|error| panic!("test routes: {error}")),
            listen: "127.0.0.1:0"
                .parse()
                .unwrap_or_else(|error| panic!("test address: {error}")),
            listener: crate::Listener::Plain,
            client: http::Client::without_identity(ca.clone()),
            client_identity: false,
            kernel: None,
            identity: None,
            receipt_keys: None,
            committed_state: None,
            state_clock: crate::now,
            paxeer: None,
            capabilities: crate::capabilities::Cache::new(std::time::Duration::from_secs(1)),
            read_windows: Mutex::default(),
            indexer,
            registration_token: None,
            trusted_proxies: Vec::new(),
            faucet: None,
            store: Arc::new(crate::RedisStore::new(
                crate::RedisEndpoint::parse("rediss://localhost:6379")
                    .unwrap_or_else(|error| panic!("test Redis endpoint: {error}")),
                ca,
                zeroize::Zeroizing::new("gateway".to_owned()),
                zeroize::Zeroizing::new(String::new()),
            )),
            event_producer: false,
            network_id: "layerx-testnet".to_owned(),
            wire_version: "1".to_owned(),
            protocol_version: 1,
            protocol_network_id: 1,
        }
    }

    fn request(method: &str, path: &str) -> IncomingRequest {
        IncomingRequest {
            method: method.to_owned(),
            path: path.to_owned(),
            headers: BTreeMap::new(),
            body: Vec::new(),
        }
    }

    fn document(answer: &OutgoingResponse) -> Value {
        assert_eq!(answer.content_type, "application/json");
        serde_json::from_slice(&answer.body).unwrap_or_else(|error| panic!("gateway JSON: {error}"))
    }

    fn refusal(answer: &OutgoingResponse, status: u16, code: &str) {
        assert_eq!(answer.status, status);
        let body = document(answer);
        assert_eq!(body["ok"], false);
        assert_eq!(body["error"]["code"], code);
    }

    fn endpoint(listener: &TcpListener) -> Indexer {
        Indexer::parse(&format!(
            "http://{}",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("indexer address: {error}"))
        ))
        .unwrap_or_else(|error| panic!("indexer endpoint: {error}"))
    }

    fn store() -> Store {
        let store =
            Store::open_in_memory().unwrap_or_else(|error| panic!("indexer store: {error}"));
        store
            .insert_chain(&ChainRecord {
                id: 1,
                name: "tenant".to_owned(),
                operator: format!("0x{}", "11".repeat(20)),
                status: "active".to_owned(),
                chain_id: 125_000_001,
                created_height: 0,
                bond: "1000".to_owned(),
                gas_allowance: 500_000,
                byte_allowance: 65536,
                fee_share_bps: 100,
            })
            .unwrap_or_else(|error| panic!("chain record: {error}"));
        store
            .commit(
                &Unit {
                    chain: "pax:1".to_owned(),
                    position: 1,
                    hash: "block1".to_owned(),
                    parent: String::new(),
                    link: "block1".to_owned(),
                    boundary: 1,
                    transfers: vec![TransferRow {
                        height_or_seq: 1,
                        tx_index: 0,
                        kind: "bank_transfer".to_owned(),
                        direction: "in",
                        account: "alice".to_owned(),
                        counterparty: Some("bob".to_owned()),
                        asset: "ulxp".to_owned(),
                        amount: "7".to_owned(),
                        tx_id: "tx1".to_owned(),
                        ordinal: 1,
                        decoded: json!({"amount":"7"}),
                    }],
                    events: vec![EventRow {
                        height_or_seq: 1,
                        source: "evm".to_owned(),
                        name: "transfer".to_owned(),
                        contract: None,
                        account: Some("alice".to_owned()),
                        tx_id: "tx1".to_owned(),
                        ordinal: 1,
                        decoded: json!({"amount":"7"}),
                    }],
                    ..Unit::default()
                },
                64,
            )
            .unwrap_or_else(|error| panic!("indexed block: {error}"));
        store
    }

    type IndexerReply = (IncomingRequest, u16, Value);

    /// Serves the indexer's real read API and returns the request and document it answered.
    fn indexer() -> (Config, std::thread::JoinHandle<IndexerReply>) {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("indexer listener: {error}"));
        let config = config(Some(endpoint(&listener)));
        let store = store();
        let worker = std::thread::spawn(move || {
            let (mut stream, _) = listener
                .accept()
                .unwrap_or_else(|error| panic!("indexer accept: {error}"));
            let request = http::read_request(&mut stream, 16 * 1024)
                .unwrap_or_else(|error| panic!("indexer request: {error}"));
            let answer = layerx_indexer::api::route(&store, &request.method, &request.path);
            layerx_platform_internal::http::write_response(&mut stream, &answer)
                .unwrap_or_else(|error| panic!("indexer reply: {error}"));
            (
                request,
                answer.status,
                serde_json::from_str(&answer.body)
                    .unwrap_or_else(|error| panic!("indexer JSON: {error}")),
            )
        });
        (config, worker)
    }

    fn read(path: &str) -> (OutgoingResponse, IncomingRequest) {
        let (config, worker) = indexer();
        let mut request = request("GET", path);
        request
            .headers
            .insert("authorization".to_owned(), "Bearer caller".to_owned());
        request
            .headers
            .insert("cookie".to_owned(), "session=caller".to_owned());
        request
            .headers
            .insert("x-forwarded-for".to_owned(), "192.0.2.10".to_owned());
        request
            .headers
            .insert("x-request-id".to_owned(), "caller".to_owned());
        let answer = crate::route_with_peer(&config, &request, None);
        let (upstream, status, body) = worker
            .join()
            .unwrap_or_else(|error| panic!("indexer worker: {error:?}"));
        assert_eq!(answer.status, status, "{path}");
        assert_eq!(document(&answer), body, "{path}");
        assert_eq!(upstream.method, "GET");
        assert_eq!(upstream.path, path.trim_end_matches('?'));
        for name in request.headers.keys() {
            assert!(!upstream.headers.contains_key(name), "caller header {name}");
        }
        assert!(upstream.body.is_empty());
        (answer, upstream)
    }

    #[test]
    fn chain_reads_list_from_indexer() {
        let (answer, _) = read(PATHS[0]);
        assert_eq!(answer.status, 200);
        assert_eq!(document(&answer)["chains"][0]["name"], "tenant");
    }

    #[test]
    fn chain_reads_one_from_indexer() {
        let (answer, _) = read(PATHS[1]);
        assert_eq!(answer.status, 200);
        assert_eq!(document(&answer)["id"], 1);
        assert_eq!(document(&answer)["chain_id"], 125_000_001);
    }

    #[test]
    fn chain_reads_summary_from_indexer() {
        let (answer, _) = read(PATHS[2]);
        assert_eq!(answer.status, 200);
        assert_eq!(
            document(&answer),
            json!({"id":1,"tx_count":0,"gas_used":0,"last_height":0})
        );
    }

    #[test]
    fn chain_reads_transfers_from_indexer() {
        let (answer, _) = read(PATHS[3]);
        assert_eq!(answer.status, 200);
        assert_eq!(document(&answer)["items"][0]["amount"], "7");
    }

    #[test]
    fn chain_reads_events_from_indexer() {
        let (answer, _) = read(PATHS[4]);
        assert_eq!(answer.status, 200);
        assert_eq!(document(&answer)["items"][0]["name"], "transfer");
    }

    fn unused_indexer() -> (Config, TcpListener) {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("indexer listener: {error}"));
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("nonblocking indexer: {error}"));
        (config(Some(endpoint(&listener))), listener)
    }

    fn assert_unused(listener: &TcpListener) {
        match listener.accept() {
            Err(error) => assert_eq!(error.kind(), std::io::ErrorKind::WouldBlock),
            Ok(_) => panic!("refused request reached the indexer"),
        }
    }

    #[test]
    fn chain_reads_invalid_chain_number_never_reaches_indexer() {
        let (config, listener) = unused_indexer();
        for id in [
            "",
            "-1",
            "+1",
            "4294967296",
            "18446744073709551616",
            "1.0",
            "0x1",
            "%31",
            "tenant",
            "1%2f2",
            "１",
        ] {
            for suffix in ["", "/summary", "/transfers", "/events"] {
                let target = format!("/v1/chains/{id}{suffix}");
                let answer = crate::route_with_peer(&config, &request("GET", &target), None);
                refusal(&answer, 400, "invalid_read_selector");
            }
        }
        assert_unused(&listener);
        for id in ["0", "1", "0001", "4294967295"] {
            let path = format!("/v1/chains/{id}");
            assert_eq!(chain_target(&path), Ok(Some((path.as_str(), ""))));
        }
    }

    #[test]
    fn chain_reads_queries_are_bounded_and_passed_unchanged() {
        for feed in ["transfers", "events"] {
            for query in [
                "cursor=1&limit=2&kind=bank_transfer",
                "limit=%32&cursor=%31",
                "limit=0",
                "cursor=bad",
                "kind=BAD",
                "unknown=1",
                "limit=1&limit=2",
                "kind=a%2Fb",
            ] {
                read(&format!("/v1/chains/1/{feed}?{query}"));
            }
            let query = format!("kind={}", "a".repeat(507));
            assert_eq!(query.len(), 512);
            let (answer, _) = read(&format!("/v1/chains/1/{feed}?{query}"));
            assert_eq!(answer.status, 400);
        }
        for path in PATHS {
            read(&format!("{path}?"));
        }
        let (config, listener) = unused_indexer();
        for feed in ["transfers", "events"] {
            for query in [
                format!("kind={}", "a".repeat(508)),
                "kind=a b".to_owned(),
                "kind=a+b".to_owned(),
                "kind=a#b".to_owned(),
                "kind=a?b".to_owned(),
                "kind=a/b".to_owned(),
                "kind=a\\b".to_owned(),
                "kind=é".to_owned(),
                "kind=%".to_owned(),
                "kind=%1".to_owned(),
                "kind=%GG".to_owned(),
                "kind=a\r\nb".to_owned(),
            ] {
                let target = format!("/v1/chains/1/{feed}?{query}");
                refusal(
                    &crate::route_with_peer(&config, &request("GET", &target), None),
                    400,
                    "invalid_read_selector",
                );
            }
        }
        for path in &PATHS[..3] {
            refusal(
                &crate::route_with_peer(&config, &request("GET", &format!("{path}?limit=1")), None),
                400,
                "invalid_read_selector",
            );
        }
        assert_unused(&listener);
    }

    #[test]
    fn chain_reads_methods_and_exact_paths() {
        let (config, listener) = unused_indexer();
        for path in PATHS {
            for method in ["POST", "PUT", "PATCH", "DELETE", "HEAD"] {
                refusal(
                    &crate::route_with_peer(&config, &request(method, path), None),
                    405,
                    "method_not_allowed",
                );
            }
        }
        for path in [
            "/v1/chains/1/",
            "/v1/chains/1/summary/",
            "/v1/chains/1/transfers/extra",
            "/v1/chains/by-name/tenant",
            "/v1/chains/1/unknown",
        ] {
            assert_eq!(chain_target(path), Ok(None));
            assert_eq!(
                crate::route_with_peer(&config, &request("GET", path), None).status,
                404,
                "{path}"
            );
        }
        assert_unused(&listener);
    }

    #[test]
    fn chain_reads_unknown_chain_keeps_indexer_status_and_document() {
        for id in ["0", "999", "4294967295"] {
            for suffix in ["", "/summary", "/transfers", "/events"] {
                let (answer, _) = read(&format!("/v1/chains/{id}{suffix}"));
                assert_eq!(answer.status, 404);
                assert_eq!(document(&answer), json!({"error":"unknown_chain"}));
            }
        }
    }

    fn fixed_second() -> Result<u64, String> {
        Ok(42)
    }

    #[test]
    fn chain_reads_share_public_read_budget() {
        CLIENT.with(|cell| *cell.borrow_mut() = "client".to_owned());
        for path in PATHS {
            let config = config(None);
            {
                let mut windows = config
                    .read_windows
                    .lock()
                    .unwrap_or_else(|error| panic!("read budget: {error}"));
                for _ in 0..READS_PER_CLIENT_PER_SECOND - 1 {
                    assert!(spend(&mut windows, 42, "client"));
                }
            }
            let request = request("GET", path);
            let answer = route_with_clock(&config, &request, fixed_second)
                .unwrap_or_else(|| panic!("chain route"));
            refusal(&answer, 503, "indexer_not_configured");
            let denied = route_with_clock(&config, &request, fixed_second)
                .unwrap_or_else(|| panic!("chain route"));
            refusal(&denied, 429, "public_read_rate_limit");
            assert_eq!(denied.retry_after, Some(1));
            assert_eq!(document(&denied)["error"]["retry_after_seconds"], 1);
        }
        let config = config(None);
        {
            let mut windows = config
                .read_windows
                .lock()
                .unwrap_or_else(|error| panic!("read budget: {error}"));
            for index in 0..READS_PER_SECOND - 1 {
                assert!(spend(&mut windows, 42, &format!("other-{index}")));
            }
        }
        let answer = route_with_clock(&config, &request("GET", PATHS[0]), fixed_second)
            .unwrap_or_else(|| panic!("chain route"));
        refusal(&answer, 503, "indexer_not_configured");
        for path in PATHS {
            let denied = route_with_clock(&config, &request("GET", path), fixed_second)
                .unwrap_or_else(|| panic!("chain route"));
            refusal(&denied, 429, "public_read_rate_limit");
        }
    }

    #[test]
    fn chain_reads_non_json_indexer_response_is_bad_gateway() {
        for (content_type, body) in [
            ("text/plain", "unavailable"),
            ("application/json", "not json"),
        ] {
            let listener = TcpListener::bind("127.0.0.1:0")
                .unwrap_or_else(|error| panic!("indexer listener: {error}"));
            let config = config(Some(endpoint(&listener)));
            let worker = std::thread::spawn(move || {
                use std::io::Write;
                let (mut stream, _) = listener
                    .accept()
                    .unwrap_or_else(|error| panic!("indexer accept: {error}"));
                http::read_request(&mut stream, 16 * 1024)
                    .unwrap_or_else(|error| panic!("indexer request: {error}"));
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap_or_else(|error| panic!("indexer reply: {error}"));
            });
            refusal(
                &crate::route_with_peer(&config, &request("GET", PATHS[0]), None),
                502,
                "invalid_indexer_response",
            );
            worker
                .join()
                .unwrap_or_else(|error| panic!("indexer worker: {error:?}"));
        }
    }

    #[test]
    fn chain_reads_unconfigured_and_unreachable_indexer_are_unavailable() {
        let config = config(None);
        for path in PATHS {
            refusal(
                &crate::route_with_peer(&config, &request("GET", path), None),
                503,
                "indexer_not_configured",
            );
        }
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("indexer listener: {error}"));
        let config = self::config(Some(endpoint(&listener)));
        drop(listener);
        refusal(
            &crate::route_with_peer(&config, &request("GET", PATHS[0]), None),
            503,
            "indexer_unreachable",
        );
    }

    #[test]
    fn chain_reads_disallowed_origin_is_refused_before_indexer() {
        let (config, listener) = unused_indexer();
        for path in PATHS {
            let mut request = request("GET", path);
            request
                .headers
                .insert("origin".to_owned(), "https://unlisted.example".to_owned());
            refusal(
                &crate::route_with_peer(&config, &request, None),
                403,
                "origin_not_allowed",
            );
        }
        assert_unused(&listener);
    }
}
