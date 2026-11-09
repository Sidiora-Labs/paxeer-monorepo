use crate::corpus::committed::produced;
use layerx_agent_api::proof::ProofBundle;
use layerx_agent_api::read::VerifiedRead;
use layerx_agent_api::verify::Rung;

/// Both ends call the same public request/response codecs used by daemon RPC
/// adapters and SDK consumers. The peer is a loopback socket, not a codec double.
pub fn round_trip(
    target: layerx_agent_api::proof::ProofBundleTarget,
    requested: Rung,
) -> VerifiedRead<ProofBundle> {
    use layerx_agent_api::read::ReadRequest;
    use std::io::{Read, Write};
    use std::net::{Shutdown, TcpListener, TcpStream};
    use std::time::Duration;
    let listener = TcpListener::bind(("127.0.0.1", 0))
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let address = listener
        .local_addr()
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let selector = target
        .encode()
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let request = ReadRequest::proof_bundle(selector.as_bytes().to_vec(), requested.name())
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let server = std::thread::spawn(move || {
        let (mut stream, _) = listener
            .accept()
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let mut incoming = Vec::new();
        stream
            .read_to_end(&mut incoming)
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let server_request = ReadRequest::proof_bundle(incoming, requested.name())
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let response = produced();
        server_request
            .check_proof_response(&response)
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        stream
            .write_all(
                response
                    .encode_json()
                    .unwrap_or_else(|error| panic!("required value: {error:?}"))
                    .as_bytes(),
            )
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
    });
    let mut stream =
        TcpStream::connect(address).unwrap_or_else(|error| panic!("required value: {error:?}"));
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    stream
        .write_all(selector.as_bytes())
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    stream
        .shutdown(Shutdown::Write)
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let mut response = String::new();
    stream
        .read_to_string(&mut response)
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    server
        .join()
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let observed = VerifiedRead::<ProofBundle>::decode_json(&response)
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    request
        .check_proof_response(&observed)
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    assert_eq!(observed, produced());
    observed
}
