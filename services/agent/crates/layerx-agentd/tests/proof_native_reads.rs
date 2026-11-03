//! Proves: NativeReadRoute::read -> daemon::serve_read_routes HTTP -> ProgramReads::execute.
//!
//! Shared real producer and HTTP server harness for the consumer-owned proof in
//! layerx-mcp/tests/proof_native_reads.rs. The consumer proof constructs this server,
//! stops it, and reopens it against the same sequencer state. The explicit
//! native-reads-http target keeps this shared source from becoming an agentd test
//! target named proof_native_reads; agentd has no dependency on layerx-mcp.
//! The consumer proof repairs AgentSurface::native_read accepting non-object
//! evidence and losing the unavailable px_getHistory method's identity.

#[allow(dead_code)]
#[path = "proof_serve_errors.rs"]
pub(crate) mod daemon;

use std::net::{SocketAddr, TcpListener};

use ed25519_dalek::SigningKey;
use layerx_agentd::node_rpc::NodeRpcConfig;
use layerx_agentd::read::{
    LayerxdProgramBalanceReader, NativeReadRoute, ProgramAuthority, ProgramBalanceReadRoute,
};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_programs::chain_proof::validators::TrustedValidator;
use layerx_programs::{ProtocolDeploymentVerifier, Registry};
use openssl::asn1::Asn1Time;
use openssl::hash::MessageDigest;
use openssl::pkey::PKey;
use openssl::x509::extension::BasicConstraints;
use openssl::x509::{X509NameBuilder, X509};

const BEARER: &str = "native-reads-proof-agent-credential";

#[track_caller]
fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("native reads producer: {error:?}"))
}

fn endpoint(config: &NodeRpcConfig) -> SequencerEndpoint {
    must(SequencerEndpoint::new(&config.url, &[]))
}

fn authority_ca() -> Vec<u8> {
    let key = must(PKey::generate_ed25519());
    let mut name = must(X509NameBuilder::new());
    must(name.append_entry_by_text("CN", "native-reads-proof"));
    let name = name.build();
    let mut certificate = must(X509::builder());
    must(certificate.set_version(2));
    must(certificate.set_subject_name(&name));
    must(certificate.set_issuer_name(&name));
    must(certificate.set_pubkey(&key));
    let not_before = must(Asn1Time::days_from_now(0));
    let not_after = must(Asn1Time::days_from_now(1));
    must(certificate.set_not_before(not_before.as_ref()));
    must(certificate.set_not_after(not_after.as_ref()));
    must(certificate.append_extension(must(BasicConstraints::new().critical().ca().build())));
    must(certificate.sign(&key, MessageDigest::null()));
    must(certificate.build().to_der())
}

pub(crate) struct Service {
    pub(crate) address: SocketAddr,
    _pool: daemon::HttpPool,
}

impl Service {
    pub(crate) fn start(config: &NodeRpcConfig, address: SocketAddr) -> Self {
        // The production read server also owns the program route. Construct its
        // real reader; native reads never contact this separate program authority.
        let ca = authority_ca();
        let validator = SigningKey::from_bytes(&[21; 32]).verifying_key().to_bytes();
        let verifier = must(ProtocolDeploymentVerifier::from_chain_trust(
            format!("paxeer-{}", config.chain_id),
            vec![TrustedValidator::new(validator, 1)],
            1_000,
        ));
        let reader = must(LayerxdProgramBalanceReader::connect(
            &config.url,
            "native-reads-node-credential".to_owned(),
            ProgramAuthority {
                endpoint: "https://localhost:1",
                authorization: "native-reads-authority-credential".to_owned(),
                replica_id: [21; 32],
                ca_der: &ca,
            },
            verifier,
            Registry::new(),
        ));
        let listener = must(TcpListener::bind(address));
        let address = must(listener.local_addr());
        let pool = must(daemon::serve_read_routes(
            listener,
            BEARER.to_owned(),
            ProgramBalanceReadRoute::new(reader),
            Some(NativeReadRoute::new(endpoint(config), config.chain_id)),
            config.clone(),
            ca,
            endpoint(config),
        ));
        Self {
            address,
            _pool: pool,
        }
    }
}
