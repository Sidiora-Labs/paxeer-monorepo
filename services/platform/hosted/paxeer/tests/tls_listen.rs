use rustls::pki_types::{CertificateDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use std::io::{Read, Write};
use std::net::TcpStream;
use std::sync::Arc;
use std::time::Duration;

#[path = "../../../tests/support/tls_boundary.rs"]
mod tls_boundary;

#[test]
fn tls_listen_reported_address() {
    tls_boundary::qualify("tls_listen_reported_address", |endpoint| {
        let authority = endpoint
            .strip_prefix("https://")
            .ok_or("HTTPS endpoint required")?;
        let (host, port) = authority.rsplit_once(':').ok_or("endpoint port required")?;
        let port = port.parse::<u16>().map_err(|error| error.to_string())?;
        let ca_path = std::env::var("LAYERX_TLS_QUAL_CA_DER").map_err(|error| error.to_string())?;
        let ca = std::fs::read(ca_path).map_err(|error| error.to_string())?;
        let mut roots = RootCertStore::empty();
        roots
            .add(CertificateDer::from(ca))
            .map_err(|error| error.to_string())?;
        let config =
            ClientConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
                .with_safe_default_protocol_versions()
                .map_err(|error| error.to_string())?
                .with_root_certificates(roots)
                .with_no_client_auth();
        let connection = ClientConnection::new(
            Arc::new(config),
            ServerName::try_from(host.to_owned()).map_err(|error| error.to_string())?,
        )
        .map_err(|error| error.to_string())?;
        let tcp =
            TcpStream::connect_timeout(&([127, 0, 0, 1], port).into(), Duration::from_secs(5))
                .map_err(|error| error.to_string())?;
        tcp.set_read_timeout(Some(Duration::from_secs(5)))
            .map_err(|error| error.to_string())?;
        tcp.set_write_timeout(Some(Duration::from_secs(5)))
            .map_err(|error| error.to_string())?;
        let mut stream = StreamOwned::new(connection, tcp);
        write!(
            stream,
            "GET /livez HTTP/1.1\r\nHost: {host}\r\nConnection: close\r\n\r\n"
        )
        .map_err(|error| error.to_string())?;
        stream.flush().map_err(|error| error.to_string())?;
        let mut response = Vec::new();
        stream
            .read_to_end(&mut response)
            .map_err(|error| error.to_string())?;
        let body = response
            .windows(4)
            .position(|window| window == b"\r\n\r\n")
            .ok_or("HTTP response headers missing")?
            + 4;
        if !response.starts_with(b"HTTP/1.1 200 OK\r\n") {
            return Err("boundary liveness request failed".to_owned());
        }
        Ok(response[body..].to_vec())
    });
}
