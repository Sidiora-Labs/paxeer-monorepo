use gateway_http::{Client, Endpoint, OutboundRequest};
pub use layerx_platform_gateway::{explorer_target, pay_timing};
use rustls::pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::ServerConfig;
use serde::Deserialize;
use serde_json::{json, Value};
use std::io::Write;
use std::net::TcpListener;
use std::sync::Arc;
use std::thread;

const GATEWAY: &str = include_str!("../deployment.yaml");
const PAXEER: &str = include_str!("../../paxeer/deployment.yaml");

#[allow(dead_code)]
mod gateway_http {
    pub(super) fn request_with_orderly_close(client: &Client, endpoint: &Endpoint, request: &OutboundRequest<'_>) -> Result<UpstreamResponse, String> {
        let mut stream = client.connect_tls(endpoint)?;
        let response = exchange(&mut stream, endpoint, "", request, OutboundHeaders::default())?;
        // Consume the server's close_notify before replying and closing TCP.
        // Keep the real server's full shutdown assertion as well as every RPC assertion.
        let mut trailing = [0_u8; 1];
        assert_eq!(stream.read(&mut trailing).map_err(|error| error.to_string())?, 0);
        stream.shutdown().map_err(|error| error.to_string())?;
        stream.get_ref().shutdown(std::net::Shutdown::Write).map_err(|error| error.to_string())?;
        Ok(response)
    }

    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/http.rs"));
}

// Compile the deployed boundary's real HTTP parser, TLS server and node relay.
// The adapter only supplies listeners and configuration without process-wide env.
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
mod boundary {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/../paxeer/src/main.rs"));

    pub(super) fn serve(listener: TcpListener, tls: Arc<ServerConfig>, node: SocketAddr, requests: usize) {
        let config = Arc::new(Config {
            listen: listener.local_addr().expect("boundary address"),
            tls,
            node: NodeEndpoint::parse(&format!("http://{node}")).expect("node origin"),
            chain_id: 125,
            comet: None,
        });
        for _ in 0..requests {
            let (tcp, _) = listener.accept().expect("boundary connection");
            let permit = ConnectionPermit::acquire().expect("boundary connection capacity");
            let deadline = Instant::now() + CLIENT_IO_TIMEOUT;
            handle_connection(&config, tcp, permit, deadline).expect("real boundary TLS relay");
        }
    }
}

fn documents(source: &str) -> Vec<Value> {
    serde_yaml::Deserializer::from_str(source).map(|document| Value::deserialize(document).unwrap_or_else(|error| panic!("{}: {error:?}", "valid Kubernetes YAML"))).collect()
}

fn resource<'a>(documents: &'a [Value], kind: &str, name: &str) -> &'a Value {
    documents.iter().find(|document| document["kind"] == kind && document["metadata"]["name"] == name).unwrap_or_else(|| panic!("missing {kind}/{name}"))
}

fn matches_selector(selector: &Value, labels: &Value) -> bool {
    if let Some(required) = selector.get("matchLabels") {
        if !required.as_object().unwrap_or_else(|| panic!("{}", "label map")).iter().all(|(key, value)| labels.get(key) == Some(value)) {
            return false;
        }
    }
    selector.get("matchExpressions").is_none_or(|expressions| {
        expressions.as_array().unwrap_or_else(|| panic!("{}", "expressions")).iter().all(|expression| {
            let key = expression["key"].as_str().unwrap_or_else(|| panic!("{}", "selector key"));
            let values = expression["values"].as_array();
            match expression["operator"].as_str().unwrap_or_else(|| panic!("{}", "selector operator")) {
                "In" => labels.get(key).is_some_and(|value| values.unwrap_or_else(|| panic!("{}", "In values")).contains(value)),
                "NotIn" => labels.get(key).is_none_or(|value| !values.unwrap_or_else(|| panic!("{}", "NotIn values")).contains(value)),
                "Exists" => labels.get(key).is_some(),
                "DoesNotExist" => labels.get(key).is_none(),
                operator => panic!("unsupported selector operator {operator}"),
            }
        })
    })
}

// Check Kubernetes' additive policy semantics on the parsed deployment artifacts.
// This is a manifest check; it does not claim that a cluster CNI was exercised.
fn admitted(manifests: &[Value], namespace: &str, labels: &Value, direction: &str, peer: (&str, &Value), protocol: &str, port: u16) -> bool {
    let (peer_namespace, peer_labels) = peer;
    let policy_type = if direction == "egress" { "Egress" } else { "Ingress" };
    let policies: Vec<_> = manifests
        .iter()
        .filter(|document| {
            document["kind"] == "NetworkPolicy" && document["metadata"]["namespace"] == namespace && matches_selector(&document["spec"]["podSelector"], labels) && document["spec"]["policyTypes"].as_array().unwrap_or_else(|| panic!("{}", "explicit policy types")).contains(&json!(policy_type))
        })
        .collect();
    assert!(!policies.is_empty(), "the tested workload must be isolated");
    policies.iter().any(|policy| {
        policy["spec"][direction].as_array().is_some_and(|rules| {
            rules.iter().any(|rule| {
                let port_allowed = rule.get("ports").is_none_or(|ports| {
                    ports
                        .as_array()
                        .unwrap_or_else(|| panic!("{}", "policy ports"))
                        .iter()
                        .any(|entry| entry.get("protocol").and_then(Value::as_str).unwrap_or("TCP") == protocol && entry["port"].as_u64() == Some(u64::from(port)))
                });
                let peers_key = if direction == "egress" { "to" } else { "from" };
                port_allowed
                    && rule.get(peers_key).is_none_or(|peers| {
                        peers.as_array().unwrap_or_else(|| panic!("{}", "policy peers")).iter().any(|peer| {
                            // ipBlock peers do not admit a pod selected by namespace/labels.
                            if peer.get("ipBlock").is_some() {
                                return false;
                            }
                            let namespace_allowed = peer.get("namespaceSelector").map_or(namespace == peer_namespace, |selector| matches_selector(selector, &json!({"kubernetes.io/metadata.name": peer_namespace})));
                            namespace_allowed && peer.get("podSelector").is_none_or(|selector| matches_selector(selector, peer_labels))
                        })
                    })
            })
        })
    })
}

fn rpc_endpoint() -> Endpoint {
    let documents = documents(GATEWAY);
    let deployment = resource(&documents, "Deployment", "layerx-gateway");
    let environment = deployment["spec"]["template"]["spec"]["containers"][0]["env"].as_array().unwrap_or_else(|| panic!("{}", "gateway env"));
    let urls = environment.iter().find(|variable| variable["name"] == "LAYERX_GATEWAY_PAXEER_RPC_URLS").unwrap_or_else(|| panic!("{}", "RPC binding"));
    let urls: Vec<String> = serde_json::from_str(urls["value"].as_str().unwrap_or_else(|| panic!("{}", "RPC URLs string"))).unwrap_or_else(|error| panic!("{}: {error:?}", "RPC URL array"));
    assert_eq!(urls.len(), 1, "one controlled in-cluster RPC path");
    Endpoint::parse(&urls[0]).unwrap_or_else(|error| panic!("{}: {error:?}", "HTTPS RPC origin"))
}

#[test]
fn gateway_rpc_egress_policy_admits_controlled_boundary() {
    let gateway = documents(GATEWAY);
    let paxeer = documents(PAXEER);
    let deployment = resource(&gateway, "Deployment", "layerx-gateway");
    let node = resource(&paxeer, "StatefulSet", "paxeer");
    assert_eq!(node["metadata"]["namespace"], "layerx-testnet");
    let observer = resource(&paxeer, "Service", "paxeer-observer-boundary");
    assert_eq!(observer["spec"]["ports"][0]["port"], 9443);
    assert_eq!(observer["spec"]["ports"][0]["targetPort"], "observer-https");
    let boundary = node["spec"]["template"]["spec"]["containers"]
        .as_array()
        .unwrap_or_else(|| panic!("{}", "boundary containers"))
        .iter()
        .find(|container| container["name"] == "boundary")
        .unwrap_or_else(|| panic!("{}", "tracked boundary container"));
    assert!(boundary["ports"].as_array().unwrap_or_else(|| panic!("{}", "boundary listeners")).iter().any(|port| port["name"] == "observer-https" && port["containerPort"] == 9444));
    let labels = &deployment["spec"]["template"]["metadata"]["labels"];
    let node_labels = &node["spec"]["template"]["metadata"]["labels"];
    let endpoint = rpc_endpoint();
    let service = resource(&paxeer, "Service", "paxeer-boundary");
    let namespace = service["metadata"]["namespace"].as_str().unwrap_or_else(|| panic!("{}", "service namespace"));
    assert_eq!(deployment["metadata"]["namespace"], namespace);
    assert_eq!(endpoint.host, format!("paxeer-boundary.{namespace}.svc.cluster.local"));
    assert!(endpoint.base_path.is_empty(), "root node RPC, without /rpc");
    let selector = json!({"matchLabels": service["spec"]["selector"]});
    assert!(matches_selector(&selector, node_labels));
    let port = service["spec"]["ports"]
        .as_array()
        .unwrap_or_else(|| panic!("{}", "service ports"))
        .iter()
        .find(|port| port["port"] == endpoint.port)
        .unwrap_or_else(|| panic!("{}", "exposed RPC port"));
    let containers = node["spec"]["template"]["spec"]["containers"].as_array().unwrap_or_else(|| panic!("{}", "node containers"));
    assert!(containers
        .iter()
        .any(|container| container["ports"].as_array().is_some_and(|ports| ports.iter().any(|candidate| candidate["name"] == port["targetPort"] && candidate["containerPort"] == endpoint.port))));
    assert!(admitted(&gateway, namespace, labels, "egress", (namespace, node_labels), "TCP", endpoint.port));
    assert!(admitted(&paxeer, namespace, node_labels, "ingress", (namespace, labels), "TCP", endpoint.port));
    for protocol in ["UDP", "TCP"] {
        assert!(admitted(&gateway, namespace, labels, "egress", ("kube-system", &json!({"k8s-app": "kube-dns"})), protocol, 53));
    }
    assert!(!admitted(&gateway, namespace, labels, "egress", (namespace, node_labels), "TCP", 443));
    assert!(!admitted(&gateway, namespace, labels, "egress", ("other-namespace", node_labels), "TCP", endpoint.port));
    assert!(!admitted(&gateway, namespace, labels, "egress", (namespace, &json!({"app": "unrelated"})), "TCP", endpoint.port));
    assert!(!admitted(&gateway, namespace, labels, "egress", (namespace, node_labels), "UDP", endpoint.port));
}

#[test]
fn gateway_rpc_egress_real_tls_relay_reaches_node_root() {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let certificate = rcgen::generate_simple_self_signed(vec!["localhost".to_owned()]).unwrap_or_else(|error| panic!("{}: {error:?}", "local TLS certificate"));
    let tls = Arc::new(
        ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(vec![certificate.cert.der().clone()], PrivateKeyDer::from(PrivatePkcs8KeyDer::from(certificate.signing_key.serialize_der())))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "boundary TLS")),
    );
    let client = Client::without_identity(native_tls::Certificate::from_der(certificate.cert.der()).unwrap_or_else(|error| panic!("{}: {error:?}", "trusted boundary certificate")));
    let node = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{}: {error:?}", "node transcript listener"));
    let node_address = node.local_addr().unwrap_or_else(|error| panic!("{}: {error:?}", "node address"));
    let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{}: {error:?}", "TLS boundary listener"));
    let boundary_port = listener.local_addr().unwrap_or_else(|error| panic!("{}: {error:?}", "boundary address")).port();
    let requests = [json!({"jsonrpc":"2.0","id":"route-network","method":"eth_chainId","params":[]}), json!({"jsonrpc":"2.0","id":"px-read","method":"eth_blockNumber","params":[]})];
    let expected = requests.clone();
    let node_thread = thread::spawn(move || {
        for (request, result) in expected.iter().zip(["0x7d", "0x2a"]) {
            let (mut stream, _) = node.accept().unwrap_or_else(|error| panic!("{}: {error:?}", "relayed node request"));
            stream.set_read_timeout(Some(std::time::Duration::from_secs(5))).unwrap_or_else(|error| panic!("{}: {error:?}", "node read bound"));
            let identity = layerx_platform_internal::http::parse_client_request(&mut stream).unwrap_or_else(|error| panic!("{}: {error:?}", "boundary chain identity request"));
            assert_eq!(identity.method, "POST");
            assert_eq!(identity.path, "/");
            // The included Paxeer boundary probes with numeric ID 1; the
            // gateway routing probe is the separate client document below.
            assert_eq!(identity.body, br#"{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}"#);
            assert_eq!(
                serde_json::from_slice::<Value>(&identity.body).unwrap_or_else(|error| panic!("{}: {error:?}", "identity RPC document")),
                json!({"jsonrpc":"2.0", "id":1, "method":"eth_chainId", "params":[]})
            );
            assert_eq!(identity.headers.get("connection").map(String::as_str), Some("keep-alive"));
            let identity_body = json!({"jsonrpc":"2.0", "id":1, "result":"0x7d"}).to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: keep-alive\r\n\r\n{}", identity_body.len(), identity_body).unwrap_or_else(|error| panic!("{}: {error:?}", "node chain identity response"));
            stream.flush().unwrap_or_else(|error| panic!("{}: {error:?}", "flush chain identity response"));
            let received = layerx_platform_internal::http::parse_client_request(&mut stream).unwrap_or_else(|error| panic!("{}: {error:?}", "real node HTTP codec"));
            assert_eq!(received.method, "POST");
            assert_eq!(received.path, "/");
            assert_eq!(serde_json::from_slice::<Value>(&received.body).unwrap_or_else(|error| panic!("{}: {error:?}", "node RPC document")), *request);
            let response = layerx_platform_internal::http::json(200, &json!({"jsonrpc":"2.0", "id":request["id"], "result":result}));
            layerx_platform_internal::http::write_response(&mut stream, &response).unwrap_or_else(|error| panic!("{}: {error:?}", "node transcript response"));
        }
    });
    let boundary_thread = thread::spawn(move || boundary::serve(listener, tls, node_address, 2));
    // Substitute only the test address for the Service's listener; retain its
    // HTTPS codec and root path, then traverse the actual boundary node relay.
    let mut endpoint = rpc_endpoint();
    endpoint.host = "localhost".to_owned();
    endpoint.port = boundary_port;
    for (request, result) in requests.iter().zip(["0x7d", "0x2a"]) {
        let body = serde_json::to_vec(request).unwrap_or_else(|error| panic!("{}: {error:?}", "gateway request"));
        let response = gateway_http::request_with_orderly_close(
            &client,
            &endpoint,
            &OutboundRequest {
                method: "POST",
                path: "/",
                idempotency: None,
                content_type: "application/json",
                body: &body,
            },
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real gateway client reaches node through TLS boundary"));
        assert_eq!(response.status, 200);
        assert_eq!(response.content_type, "application/json");
        assert_eq!(serde_json::from_slice::<Value>(&response.body).unwrap_or_else(|error| panic!("{}: {error:?}", "gateway response")), json!({"jsonrpc":"2.0", "id":request["id"], "result":result}));
    }
    node_thread.join().unwrap_or_else(|error| panic!("{}: {error:?}", "node transcript completed"));
    boundary_thread.join().unwrap_or_else(|error| panic!("{}: {error:?}", "real boundary completed"));
}
