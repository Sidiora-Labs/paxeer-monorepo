use super::{
    json_response, now, parse_hex32, response, upstream_json, Config, KernelBackend,
    KernelUnavailable, OutgoingResponse,
};
use std::cell::RefCell;
use std::collections::HashMap;
use std::net::{IpAddr, SocketAddr};
use std::sync::{LazyLock, Mutex};

struct ReadWindows {
    second: u64,
    total: u32,
    per_client: HashMap<String, u32>,
}

static READ_WINDOWS: LazyLock<Mutex<ReadWindows>> = LazyLock::new(|| {
    Mutex::new(ReadWindows {
        second: 0,
        total: 0,
        per_client: HashMap::new(),
    })
});
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

pub(super) fn consume_read() -> bool {
    let Ok(second) = now() else { return false };
    let Ok(mut windows) = READ_WINDOWS.lock() else {
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
    if !consume_read() {
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

pub(super) fn route(config: &Config, request: &super::IncomingRequest) -> Option<OutgoingResponse> {
    if matches!(request.path.as_str(), "/rpc" | "/rpc/schema") {
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
