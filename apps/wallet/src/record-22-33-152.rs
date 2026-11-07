// Implementation-time recorder; the wallet gate only reads its JSON output.
#[allow(dead_code)]
mod production_fixture {
    include!(
        "../../../services/human/crates/layerx-human-service/tests/support/production_fixture.rs"
    );

    pub(crate) fn recording_tenancy(binding: layerx_identity_binding::Client) -> IdentityTenancy {
        IdentityTenancy(binding)
    }
}

use base64::{engine::general_purpose::STANDARD, Engine as _};
use production_fixture::*;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::Arc;
use std::time::Duration;

fn recorded(bytes: &[u8]) -> Value {
    json!({"base64": STANDARD.encode(bytes), "sha256": cluster::hex(&Sha256::digest(bytes))})
}

fn exchange(fixture: &Fixture, listener: &TcpListener, body: &Value) -> Result<Value> {
    let router = fixture.router.as_ref().ok_or("router closed")?.clone();
    let listener = listener.try_clone()?;
    let address = listener.local_addr()?;
    let worker = std::thread::spawn(move || {
        let (mut stream, _) = listener.accept()?;
        router.serve_one(&mut stream, "wallet-intent-recording")
    });
    let mut stream = TcpStream::connect(address)?;
    stream.set_read_timeout(Some(Duration::from_secs(30)))?;
    let encoded = serde_json::to_vec(body)?;
    write!(stream, "POST /v1/intents/plan HTTP/1.1\r\nHost: paxportwallet.com\r\nOrigin: https://paxportwallet.com\r\nAuthorization: Bearer {}\r\nCookie: __Host-layerx_access={}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", fixture.assertion, fixture.browser_access, encoded.len())?;
    stream.write_all(&encoded)?;
    let mut response = Vec::new();
    stream.read_to_end(&mut response)?;
    worker.join().map_err(|_| "router worker panicked")??;
    Ok(recorded(&response))
}

fn envelope(record: &Value) -> Result<Value> {
    let bytes = STANDARD.decode(record["base64"].as_str().ok_or("recorded bytes")?)?;
    let response = std::str::from_utf8(&bytes)?;
    let (_, body) = response.split_once("\r\n\r\n").ok_or("HTTP response")?;
    Ok(serde_json::from_str(body)?)
}

fn recovered_observation(fixture: &mut Fixture) -> Result<Value> {
    use base64::engine::general_purpose::URL_SAFE_NO_PAD;
    use layerx_human_service::store::{PrincipalId, PrincipalStore, Table, TenancyDigest};
    fixture.close_router()?;
    assert!(fixture.router.is_none());
    let binding = layerx_identity_binding::Client::new(
        layerx_identity_binding::Config {
            socket: fixture.root.join("identity-binding.sock"),
            tenant: "tenant-a".to_owned(),
            peer_uid: rustix::process::getuid().as_raw(),
            peer_gid: rustix::process::getgid().as_raw(),
            deadline: Duration::from_secs(5),
        },
        fixture.clock.clone(),
    )?;
    let digest: [u8; 32] = URL_SAFE_NO_PAD
        .decode(std::env::var("LAYERX_HUMAN_TENANCY_DIGEST")?)?
        .try_into()
        .map_err(|_| "tenancy digest")?;
    let mut store = PrincipalStore::open_with_authority(
        fixture.root.join("store"),
        layerx_human_test_support::retention_uniform(86400),
        TenancyDigest::new(digest),
        Arc::new(recording_tenancy(binding)),
    )?;
    let principal = PrincipalId::new(fixture.document["principal"].as_str().ok_or("principal")?)?;
    let scope = store.principal(&principal)?;
    let keys = scope
        .keys(Table::Cache)
        .into_iter()
        .filter(|key| key.as_str().starts_with("intent-observation-"))
        .collect::<Vec<_>>();
    assert_eq!(keys.len(), 1);
    let key = &keys[0];
    let row = scope
        .get(Table::Cache, key)
        .ok_or("persisted intent observation")?;
    Ok(json!({"key": key.as_str(), "row": recorded(row.bytes())}))
}

fn main() -> Result<()> {
    let output = std::env::args().nth(1).ok_or("output JSON path")?;
    let mut fixture = Fixture::open()?;
    let listener = TcpListener::bind((std::net::Ipv4Addr::LOCALHOST, 0))?;
    let request = fixture.document["intent"].clone();
    let before = exchange(&fixture, &listener, &request)?;
    let first = envelope(&before)?;
    assert_eq!(first["ok"], true);
    assert_eq!(first["result"]["journey_kind"], "move");
    assert!(!first["result"]["legs"].as_array().ok_or("legs")?.is_empty());
    let recovered = recovered_observation(&mut fixture)?;
    let memo: Value = serde_json::from_slice(
        &STANDARD.decode(recovered["row"]["base64"].as_str().ok_or("row bytes")?)?,
    )?;
    assert_eq!(
        cluster::hex(&serde_json::from_value::<Vec<u8>>(
            memo["plan_digest"].clone()
        )?),
        first["result"]["plan_digest"]
    );
    let canonical_intent: Vec<u8> = serde_json::from_value(memo["canonical_intent"].clone())?;
    assert_eq!(serde_json::from_slice::<Value>(&canonical_intent)?, request);
    fixture.reopen()?;
    let after = exchange(&fixture, &listener, &request)?;
    assert_eq!(first["result"], envelope(&after)?["result"]);
    let recovered_again = recovered_observation(&mut fixture)?;
    assert_eq!(recovered, recovered_again);
    fixture.reopen()?;
    let mut refused_request = request.clone();
    refused_request["money"]["currency"] = json!("INVALID");
    let refusal = exchange(&fixture, &listener, &refused_request)?;
    assert_eq!(envelope(&refusal)?["ok"], false);
    let actor = fixture.document["actor"].as_str().ok_or("actor")?;
    let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
    let wallet = peer.identities.get(actor).ok_or("wallet identity")?.1;
    let document = json!({
        "format": 1,
        "producer": "layerx-human-service::server::ProductionComponents",
        "store": "layerx-human-service::store::PrincipalStore",
        "transport": "Router::serve_one over one TCP listener before and after Fixture::reopen",
        "request": request,
        "wallet_address": format!("0x{}", cluster::hex(&wallet)),
        "actor": actor,
        "sequence": NONCE,
        "not_after": fixture.document["not_after"],
        "before": before,
        "recovered_before_replanning": recovered,
        "after": after,
        "recovered_after_replanning": recovered_again,
        "refused_request": refused_request,
        "refusal": refusal,
    });
    std::fs::write(output, serde_json::to_vec_pretty(&document)?)?;
    Ok(())
}
