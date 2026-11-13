//! Task 22.33.1533 repair record; the private hf-done ledger supplies revision and gate evidence.
//! `authority_cache_unbounded` -> `authority_cache_bounds_rotation_and_refuses_revocation`
//! `boot_registry_transport` -> `readiness::readiness_refresh_closes_writes_and_preserves_generation`
//! `compat_registry_version` -> `readiness::compatibility_matrix_names_contract_major_two`
//! `health_bearer_registry` -> `health_listener_separates_credentials_and_rejects_reuse`
//! `missing_agent_contract_pages` -> `receipt_roundtrip::receipt_rpc_roundtrip_preserves_bindings_and_explicit_rung_after_store_reopen`
//! `retired_bridge_time_fixture` -> `committed_time::actual_native_batch_proves_time_count_and_exact_receipt_denomination`
//! Wiring request (Domain 9): spec/layerx-merge/interfaces.kvx row 9.rs-agentd-boot-handshake-gate must name `Gate::readiness_gate(&NodeRpc)`, `ReadinessStatus`, current callers, and chain/key-policy refusals.
//! Wiring request (Domain 9): spec/layerx-merge/interfaces.kvx row 9.rs-agentd-compat-contract-version-j43 must declare `CONTRACT_VERSION` = 2 and align dependent rows, retaining rejection of 1.
//! Wiring request (Domain 9): spec/layerx-merge/interfaces.kvx row 9.http-agentd-human-owner-get-healthz must name `LAYERX_AGENT_PROGRAM_BEARER_TOKEN` and link contracts/agent-config.md.
//! Wiring request (Domain 9): services/agent/crates/layerx-agentd/src/protocol_evidence.rs and `protocol_evidence_native_tests.rs` must mark retained custody decode coverage migration-only and route current time proofs to the committed ledger fixture.
//! Baseline: the test target did not exist at the starting revision; no pre-change failures are claimed.
//! Contract pages already exist on the task base; this repair preserves their catalogues and completes the missing round-trip details.
//! The explicit implementation-first order and hf-done-only verification rule supersede `do_10`'s baseline run.
//!
//! Requalification preserves the current readiness cases and uses the current keys object/integer-height RPC contract.
//! Attempt 4 baseline (gotcha 3173): `committed_time::actual_native_batch_proves_time_count_and_exact_receipt_denomination` and `receipt_roundtrip::receipt_rpc_roundtrip_retains_absent_and_submitted_answers` failed; the other 13 cases passed.
//! Requalification aligns the history envelope with the recorded gateway freshness schema and retains exact receipt denomination and typed absent/pending outcomes.
//! The health listener calls the production `owner_health` handler used by `human_owner_mode.rs`.

fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use layerx_agentd::human_receipt_frame::owner_health::{
    serve_health, validate_health_credentials, AUTHORITY_BEARER_ENV, HEALTH_BEARER_ENV,
};

#[cfg(test)]
#[path = "../src/protocol_evidence_time_tests.rs"]
mod committed_time;
#[cfg(test)]
#[path = "readiness_node_rpc.rs"]
mod readiness;
#[cfg(test)]
#[path = "repair_22_31_220.rs"]
mod receipt_roundtrip;

#[cfg(test)]
mod tests {
    use super::*;
    use layerx_agentd::authority::{
        require_valid, resolve, AuthorityCache, AuthorityError, AuthorityResolver, AuthorityState,
        DEFAULT_AUTHORITY_CACHE_CAPACITY,
    };
    use layerx_agentd::identity::ProtocolAuthority;
    use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
    use serde_json::{json, Value};
    use std::collections::{BTreeMap, BTreeSet};
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::{TcpListener, TcpStream};
    use std::sync::{
        atomic::{AtomicUsize, Ordering},
        Arc,
    };
    use std::thread;
    use std::time::Duration;

    // Committed in-memory node-boundary observations; the cache and write checker are production.
    struct AuthorityObservations {
        states: BTreeMap<[u8; 32], AuthorityState>,
        calls: usize,
        unavailable: bool,
    }
    impl AuthorityResolver for AuthorityObservations {
        fn authority_state(
            &mut self,
            authority: &ProtocolAuthority,
        ) -> Result<Option<AuthorityState>, AuthorityError> {
            self.calls += 1;
            if self.unavailable {
                return Err(AuthorityError::BoundaryUnavailable);
            }
            let key = match authority {
                ProtocolAuthority::PrimaryKey(key)
                | ProtocolAuthority::SessionKey(key)
                | ProtocolAuthority::CapabilityGrant(key) => key,
            };
            Ok(self.states.get(key).cloned())
        }
    }
    fn state(key: [u8; 32], sequence: u64) -> AuthorityState {
        AuthorityState {
            authority: ProtocolAuthority::SessionKey(key),
            generation: 1,
            valid_through_sequence: sequence + 100,
            revoked: false,
            permitted_activity_types: BTreeSet::from([7]),
            observed_sequence: sequence,
        }
    }
    fn observations() -> AuthorityObservations {
        AuthorityObservations {
            states: BTreeMap::new(),
            calls: 0,
            unavailable: false,
        }
    }

    #[test]
    fn authority_cache_bounds_rotation_and_refuses_revocation() {
        let mut boundary = observations();
        let mut cache = AuthorityCache::new(7);
        assert_eq!(cache.capacity(), 7);
        assert_eq!(
            AuthorityCache::default().capacity(),
            DEFAULT_AUTHORITY_CACHE_CAPACITY
        );
        for sequence in 1_u64..=2000 {
            let mut key = [0; 32];
            key[24..].copy_from_slice(&sequence.to_be_bytes());
            let current = state(key, sequence);
            boundary.states.insert(key, current.clone());
            let resolved = require_ok(
                resolve(&mut cache, &mut boundary, &current.authority, sequence, 100),
                "required test value",
            );
            assert_eq!(
                require_ok(
                    require_valid(&current.authority, 1, 7, sequence, &resolved),
                    "required test value"
                )
                .generation,
                1
            );
            assert_eq!(
                cache.len(),
                require_ok(usize::try_from(sequence), "required test value").min(7)
            );
            // The peer keeps only its current recording; the cache must independently bound retention.
            boundary.states.clear();
        }
        let key = [8; 32];
        let authority = ProtocolAuthority::SessionKey(key);
        boundary.states.insert(key, state(key, 3000));
        let opened = require_ok(
            resolve(&mut cache, &mut boundary, &authority, 3000, 0),
            "required test value",
        );
        assert_eq!(
            cache.len(),
            1,
            "stale rotations must be pruned, not merely ignored"
        );
        assert!(require_valid(&authority, 1, 7, 3000, &opened).is_ok());
        let mut rotated = state(key, 3001);
        rotated.generation = 2;
        boundary.states.insert(key, rotated);
        let resolved = require_ok(
            resolve(&mut cache, &mut boundary, &authority, 3001, 0),
            "required test value",
        );
        complete_authority_cache_bounds_rotation_and_refuses_revocation_1_repair_22_33_1533(
            boundary, cache, key, &authority, &resolved,
        );
    }

    fn complete_authority_cache_bounds_rotation_and_refuses_revocation_1_repair_22_33_1533(
        mut boundary: AuthorityObservations,
        mut cache: AuthorityCache,
        key: [u8; 32],
        authority: &ProtocolAuthority,
        resolved: &AuthorityState,
    ) {
        assert_eq!(
            require_valid(authority, 1, 7, 3001, resolved),
            Err(AuthorityError::Rotated)
        );
        let mut revoked = state(key, 3002);
        revoked.revoked = true;
        boundary.states.insert(key, revoked);
        let resolved = require_ok(
            resolve(&mut cache, &mut boundary, authority, 3002, 0),
            "required test value",
        );
        assert_eq!(
            require_valid(authority, 1, 7, 3002, &resolved),
            Err(AuthorityError::Revoked)
        );
        assert!(
            cache.is_empty(),
            "revoked observations must not consume retention"
        );
        let mut expired = state(key, 3003);
        expired.valid_through_sequence = 3002;
        boundary.states.insert(key, expired);
        let resolved = require_ok(
            resolve(&mut cache, &mut boundary, authority, 3003, 0),
            "required test value",
        );
        assert_eq!(
            require_valid(authority, 1, 7, 3003, &resolved),
            Err(AuthorityError::Expired)
        );
        assert!(cache.is_empty());
        boundary.states.insert(key, state(key, 3004));
        let resolved = require_ok(
            resolve(&mut cache, &mut boundary, authority, 3004, 0),
            "required test value",
        );
        assert_eq!(
            require_valid(authority, 1, 99, 3004, &resolved),
            Err(AuthorityError::Scope)
        );
        assert_eq!(
            require_valid(authority, 1, 7, 3005, &resolved),
            Err(AuthorityError::Stale {
                current: 3005,
                observed: 3004
            })
        );
        boundary.unavailable = true;
        assert_eq!(
            resolve(&mut cache, &mut boundary, authority, 3005, 0),
            Err(AuthorityError::BoundaryUnavailable)
        );
        assert!(cache.is_empty());
        boundary.unavailable = false;
        boundary.states.clear();
        assert_eq!(
            resolve(&mut cache, &mut boundary, authority, 3005, 0),
            Err(AuthorityError::Missing)
        );
    }

    #[test]
    fn authority_cache_eviction_is_deterministic_and_retains_regression_refusals() {
        let mut boundary = observations();
        for key in [1_u8, 2, 3, 4] {
            boundary.states.insert([key; 32], state([key; 32], 10));
        }
        let mut cache = AuthorityCache::new(2);
        // Equal observation times evict the lexicographically lowest key regardless of insertion order.
        for key in [2_u8, 1, 3] {
            require_ok(
                resolve(
                    &mut cache,
                    &mut boundary,
                    &ProtocolAuthority::SessionKey([key; 32]),
                    10,
                    3,
                ),
                "required test value",
            );
        }
        assert_eq!(cache.len(), 2);
        let calls = boundary.calls;
        require_ok(
            resolve(
                &mut cache,
                &mut boundary,
                &ProtocolAuthority::SessionKey([2; 32]),
                10,
                3,
            ),
            "required test value",
        );
        assert_eq!(boundary.calls, calls);
        require_ok(
            resolve(
                &mut cache,
                &mut boundary,
                &ProtocolAuthority::SessionKey([1; 32]),
                10,
                3,
            ),
            "required test value",
        );
        assert_eq!(boundary.calls, calls + 1);
        assert_eq!(cache.len(), 2);
        let authority = ProtocolAuthority::SessionKey([3; 32]);
        assert_eq!(
            resolve(&mut cache, &mut boundary, &authority, 9, 3),
            Err(AuthorityError::SequenceRegression {
                current: 9,
                observed: 10
            })
        );
        assert!(cache.is_empty(), "all future observations are invalidated");
        assert_eq!(
            resolve(&mut cache, &mut boundary, &authority, 9, 3),
            Err(AuthorityError::SequenceRegression {
                current: 9,
                observed: 10
            })
        );
        assert!(cache.is_empty());
        boundary.states.insert([3; 32], state([3; 32], 9));
        require_ok(
            resolve(&mut cache, &mut boundary, &authority, 9, 3),
            "required test value",
        );
        let mut disabled = AuthorityCache::new(0);
        for _ in 0..2 {
            require_ok(
                resolve(&mut disabled, &mut boundary, &authority, 9, 3),
                "required test value",
            );
        }
        assert!(disabled.is_empty());
        // Expiry and revocation do not pin otherwise fresh entries.
        let mut expired = state([1; 32], 9);
        expired.valid_through_sequence = 9;
        boundary.states.insert([1; 32], expired);
        let mut expiring = AuthorityCache::new(2);
        require_ok(
            resolve(
                &mut expiring,
                &mut boundary,
                &ProtocolAuthority::SessionKey([1; 32]),
                9,
                10,
            ),
            "required test value",
        );
        require_ok(
            resolve(&mut expiring, &mut boundary, &authority, 10, 10),
            "required test value",
        );
        assert_eq!(expiring.len(), 1);
    }

    fn health_call(address: std::net::SocketAddr, request: &str) -> String {
        let mut client = require_ok(TcpStream::connect(address), "required test value");
        require_ok(
            client.set_read_timeout(Some(Duration::from_secs(3))),
            "required test value",
        );
        require_ok(client.write_all(request.as_bytes()), "required test value");
        let mut response = String::new();
        require_ok(client.read_to_string(&mut response), "required test value");
        response
    }

    #[test]
    fn health_listener_separates_credentials_and_rejects_reuse() {
        let health = "health-readiness-credential-1533-0001";
        let authority = "outbound-authority-credential-1533-0002";
        assert_eq!(HEALTH_BEARER_ENV, "LAYERX_AGENT_PROGRAM_BEARER_TOKEN");
        assert_eq!(AUTHORITY_BEARER_ENV, "LAYERX_AGENT_HUMAN_AUTHORITY_BEARER");
        require_ok(
            validate_health_credentials("127.0.0.1:0", health, authority),
            "required test value",
        );
        assert!(validate_health_credentials("127.0.0.1:0", authority, authority).is_err());
        assert!(validate_health_credentials("0.0.0.0:0", health, authority).is_err());
        assert!(validate_health_credentials("127.0.0.1:0", "short", authority).is_err());
        let node_listener = require_ok(TcpListener::bind("127.0.0.1:0"), "required test value");
        let node_config = NodeRpcConfig {
            url: format!(
                "http://{}/",
                require_ok(node_listener.local_addr(), "required test value")
            ),
            chain_id: 125,
            deadline: Duration::from_secs(2),
        };
        let node = thread::spawn(move || {
            let (stream, _) = require_ok(node_listener.accept(), "required test value");
            require_ok(
                stream.set_read_timeout(Some(Duration::from_secs(2))),
                "required test value",
            );
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(
                    require_ok(reader.read_line(&mut line), "required test value"),
                    0
                );
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(require_ok(
                        value.trim().parse::<usize>(),
                        "required test value",
                    ));
                }
            }
            let mut bytes = vec![0; require_some(length, "required test value")];
            require_ok(reader.read_exact(&mut bytes), "required test value");
            let request: Value = require_ok(serde_json::from_slice(&bytes), "required test value");
            assert_eq!(request["method"], "eth_chainId");
            assert_eq!(request["params"], json!([]));
            let body = json!({"jsonrpc":"2.0","id":request["id"],"result":"0x7d"}).to_string();
            require_ok(
                write!(
                    reader.into_inner(),
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                ),
                "required test value",
            );
        });
        continue_health_listener_separates_credentials_and_rejects_reuse_2_repair_22_33_1533(
            health,
            authority,
            node_config,
            node,
        );
    }

    fn continue_health_listener_separates_credentials_and_rejects_reuse_2_repair_22_33_1533(
        health: &'static str,
        authority: &'static str,
        node_config: NodeRpcConfig,
        node: thread::JoinHandle<()>,
    ) {
        let listener = require_ok(TcpListener::bind("127.0.0.1:0"), "required test value");
        let address = require_ok(listener.local_addr(), "required test value");
        let calls = Arc::new(AtomicUsize::new(0));
        let observed = Arc::clone(&calls);
        let cases = [
            (format!("GET /healthz HTTP/1.1\r\nAuthorization: Bearer {authority}\r\n\r\n"), 401),
            (format!("GET /healthz HTTP/1.1\r\nAuthorization: Bearer {health}\r\n\r\n"), 200),
            (format!("GET /healthz HTTP/1.1\r\nAuthorization: Bearer {health}\r\n\r\n"), 503),
            ("GET /healthz HTTP/1.1\r\n\r\n".to_owned(), 401),
            (format!("GET /other HTTP/1.1\r\nAuthorization: Bearer {health}\r\n\r\n"), 404),
            (format!("GET /healthz HTTP/1.1\r\nAuthorization: Bearer {health}\r\nAuthorization: Bearer {authority}\r\n\r\n"), 401),
            ("POST /healthz HTTP/1.1\r\n\r\n".to_owned(), 400),
            (format!("GET /healthz HTTP/1.1\r\nAuthorization: Bearer {health}\r\nContent-Length: 1\r\n\r\nx"), 413),
            (format!("GET /healthz HTTP/1.1\r\nAuthorization: Bearer {health}\r\nTransfer-Encoding: chunked\r\n\r\n"), 400),
            ("x".repeat(16 * 1024), 431),
        ];
        let count = cases.len();
        let server = thread::spawn(move || {
            for index in 0..count {
                let (mut stream, _) = require_ok(listener.accept(), "required test value");
                require_ok(
                    stream.set_read_timeout(Some(Duration::from_secs(2))),
                    "required test value",
                );
                require_ok(
                    stream.set_write_timeout(Some(Duration::from_secs(2))),
                    "required test value",
                );
                require_ok(
                    serve_health(
                        &mut stream,
                        health,
                        || index != 2,
                        || {
                            observed.fetch_add(1, Ordering::SeqCst);
                            NodeRpc::connect(&node_config)
                                .map(|_| ())
                                .map_err(|e| format!("{e:?}"))
                        },
                    ),
                    "required test value",
                );
            }
        });
        continue_health_listener_separates_credentials_and_rejects_reuse_1_repair_22_33_1533(
            ContinueHealthListenerSeparatesCredentialsAndRejectsReuse1Repair22331533Context {
                health,
                authority,
                node,
                address,
                calls,
                cases,
                server,
            },
        );
    }

    struct ContinueHealthListenerSeparatesCredentialsAndRejectsReuse1Repair22331533Context {
        health: &'static str,
        authority: &'static str,
        node: thread::JoinHandle<()>,
        address: std::net::SocketAddr,
        calls: Arc<AtomicUsize>,
        cases: [(String, i32); 10],
        server: thread::JoinHandle<()>,
    }

    fn continue_health_listener_separates_credentials_and_rejects_reuse_1_repair_22_33_1533(
        context: ContinueHealthListenerSeparatesCredentialsAndRejectsReuse1Repair22331533Context,
    ) {
        let ContinueHealthListenerSeparatesCredentialsAndRejectsReuse1Repair22331533Context {
            health,
            authority,
            node,
            address,
            calls,
            cases,
            server,
        } = context;

        for (request, status) in cases {
            let response = health_call(address, &request);
            assert!(
                response.starts_with(&format!("HTTP/1.1 {status} ")),
                "{response}"
            );
            assert!(response.contains("Cache-Control: no-store"));
            if status == 200 {
                assert!(response.ends_with("{\"ready\":true}"));
            }
            if status == 503 {
                assert!(response.ends_with("{\"ready\":false}"));
            }
            assert!(!response.contains(health));
            assert!(!response.contains(authority));
        }
        require_ok(server.join(), "required test value");
        require_ok(node.join(), "required test value");
        assert_eq!(
            calls.load(Ordering::SeqCst),
            1,
            "unauthorized callers must never probe dependencies"
        );
        // The listener can be reopened with the same credentials; no readiness is recovered from memory.
        let listener = require_ok(TcpListener::bind("127.0.0.1:0"), "required test value");
        let address = require_ok(listener.local_addr(), "required test value");
        let server = thread::spawn(move || {
            let (mut stream, _) = require_ok(listener.accept(), "required test value");
            require_ok(
                stream.set_read_timeout(Some(Duration::from_secs(2))),
                "required test value",
            );
            require_ok(
                serve_health(
                    &mut stream,
                    health,
                    || true,
                    || Err("dependency unavailable".to_owned()),
                ),
                "required test value",
            );
        });
        let response = health_call(
            address,
            &format!("GET /healthz HTTP/1.1\r\nAuthorization: Bearer {health}\r\n\r\n"),
        );
        assert!(response.starts_with("HTTP/1.1 503 "));
        require_ok(server.join(), "required test value");
    }

    #[test]
    fn maintained_contracts_cover_the_executed_roundtrip_and_configuration() {
        let api = include_str!("../../../../../contracts/agent-api.md");
        let config = include_str!("../../../../../contracts/agent-config.md");
        let peer = include_str!("../../../../../contracts/agent-human-peer.md");
        for field in [
            "program.receipt",
            "canonical_bytes",
            "authorised_batch",
            "verification_level",
            "tx_hash",
            "receipt_hash",
            "block_height",
            "rung",
            "signed_receipt",
            "receipt_key_id",
            "public_key",
            "first_height",
            "last_height",
            "inclusion_height",
        ] {
            assert!(api.contains(field), "missing API field {field}");
            assert!(peer.contains(field), "missing peer field {field}");
        }
        for key in [
            HEALTH_BEARER_ENV,
            AUTHORITY_BEARER_ENV,
            "LAYERX_AGENT_NODE_RPC_URL",
            "LAYERX_AGENT_NODE_CHAIN_ID",
        ] {
            assert!(config.contains(key), "missing setting {key}");
        }
        for requirement in [
            "restart",
            "readiness_gate",
            "expected_chain_id",
            "receipt_key_source",
        ] {
            assert!(
                config.contains(requirement),
                "missing startup contract {requirement}"
            );
        }
        assert!(api.contains("contract major 2"));
        assert!(api.contains("envelope version 1"));
        assert!(peer.contains("migration"));
    }
}
