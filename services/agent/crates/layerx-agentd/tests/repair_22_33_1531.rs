// Run the registered TLS cases against the linked production crate as well as
// the library target; no boundary binary or subprocess harness is involved.
use layerx_agentd::{human, human_runtime};

#[path = "../src/outbound_tls/tests.rs"]
mod outbound_tls;
