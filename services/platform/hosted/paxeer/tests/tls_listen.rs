#[path = "support/node_tls.rs"]
pub mod node_tls;

#[test]
fn tls_listen_reported_address() {
    node_tls::qualify_listener();
    node_tls::qualify_tls_identity();
}
