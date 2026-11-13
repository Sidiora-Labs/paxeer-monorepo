use layerx_client::px::PxEndpoint;
use layerx_client::rpc::history::{did_accounts, did_evm_address};
use layerx_client::sequencer::{RpcRefusal, SequencerError};
use layerx_platform_core::{custody_source_address, hex_encode};
use serde_json::{json, Value};
use std::io::Write;
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::Duration;

fn gateway(did: &str, responses: Vec<Value>) -> (PxEndpoint, JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let endpoint = PxEndpoint::new(
        &format!(
            "http://{}/",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("required value: {error:?}"))
        ),
        &[],
    )
    .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let did = did.to_owned();
    let worker = thread::spawn(move || {
        for mut response in responses {
            let (mut stream, _) = listener
                .accept()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let request = layerx_platform_internal::http::parse_client_request(&mut stream)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            assert_eq!(request.method, "POST");
            let request: Value = serde_json::from_slice(&request.body)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], "px_getAccount");
            assert_eq!(request["params"], json!([did]));
            response["jsonrpc"] = json!("2.0");
            response["id"] = request["id"].clone();
            let body = response.to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap_or_else(|error| panic!("required value: {error:?}"));
            stream
                .flush()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
        }
    });
    (endpoint, worker)
}

fn account_document(did: &str) -> Value {
    json!({
        "account": {
            "evm_address": format!("0x{}", hex_encode(&[0x12; 20])),
            "pax_address": "pax1boundidentity",
            "layerx_did": did,
            "layerx_account": hex_encode(&[0x34; 32]),
            "bound": true
        },
        "paxeer": {
            "address": format!("0x{}", hex_encode(&[0x12; 20])),
            "balance": "0x7b",
            "nonce": "0x7",
            "height": 42
        }
    })
}

#[test]
fn custody_source_address_reads_the_gateway_account_document() {
    let did = format!("did:layerx:{}", hex_encode(&[0xab; 32]));
    let document = account_document(&did);
    assert!(document.get("layerx").is_none());
    let (endpoint, worker) = gateway(&did, vec![json!({"result": document}); 4]);
    let joined = endpoint
        .account(&did)
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    assert_eq!(joined.layerx, None);
    assert_eq!(joined.account.evm_address, Some([0x12; 20]));
    assert_eq!(did_evm_address(&endpoint, &did), Ok(Some([0x12; 20])));
    assert_eq!(
        did_accounts(&endpoint, &did).unwrap_or_else(|error| panic!("required value: {error:?}")),
        vec![document["account"].clone()]
    );
    assert_eq!(custody_source_address(&endpoint, &did), Ok([0x12; 20]));
    worker
        .join()
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
}

#[test]
fn custody_source_address_refuses_an_unbound_did_and_reports_transport_failures() {
    let did = format!("did:layerx:{}", hex_encode(&[0xab; 32]));
    let bound = account_document(&did);
    let mut unbound = bound.clone();
    unbound["account"]["bound"] = json!(false);
    let mut missing_address = bound.clone();
    missing_address["account"]["evm_address"] = Value::Null;
    missing_address["paxeer"] = Value::Null;
    let mut foreign = bound.clone();
    foreign["account"]["layerx_did"] = json!(format!("did:layerx:{}", hex_encode(&[0xcd; 32])));
    let mut gateway_unbound = missing_address.clone();
    gateway_unbound["account"]["bound"] = json!(false);
    gateway_unbound["account"]["pax_address"] = Value::Null;
    for document in [unbound, missing_address, foreign, gateway_unbound] {
        let (endpoint, worker) = gateway(&did, vec![json!({"result": document}); 3]);
        assert_eq!(did_evm_address(&endpoint, &did), Ok(None));
        assert!(did_accounts(&endpoint, &did)
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .is_empty());
        assert_eq!(
            custody_source_address(&endpoint, &did),
            Err("custody DID has no bound evm_address".to_owned())
        );
        worker
            .join()
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
    }

    let refusal = RpcRefusal {
        code: -32001,
        message: "Read unavailable".to_owned(),
        data: Some(json!({"code": "identity_binding_unavailable"})),
    };
    let (endpoint, worker) = gateway(
        &did,
        vec![
            json!({"error": {
                "code": refusal.code, "message": refusal.message, "data": refusal.data
            }});
            2
        ],
    );
    assert_eq!(
        did_evm_address(&endpoint, &did),
        Err(SequencerError::Rpc(refusal.clone()))
    );
    assert_eq!(
        custody_source_address(&endpoint, &did),
        Err(format!(
            "custody DID accounts: {:?}",
            SequencerError::Rpc(refusal)
        ))
    );
    worker
        .join()
        .unwrap_or_else(|error| panic!("required value: {error:?}"));

    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let endpoint = PxEndpoint::new(
        &format!(
            "http://{}/",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("required value: {error:?}"))
        ),
        &[],
    )
    .unwrap_or_else(|error| panic!("required value: {error:?}"));
    drop(listener);
    assert_eq!(
        did_evm_address(&endpoint, &did),
        Err(SequencerError::Unavailable)
    );
    assert_eq!(
        custody_source_address(&endpoint, &did),
        Err("custody DID accounts: Unavailable".to_owned())
    );
}
