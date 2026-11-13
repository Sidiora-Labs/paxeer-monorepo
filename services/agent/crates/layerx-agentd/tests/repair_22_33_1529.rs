//! Audit 22.30.94 closure proofs. Each original regression suite is also compiled
//! into this focused gate, so fixes cannot pass by adding disconnected assertions.
//! Finding-to-case mapping and qualification constraints: `repair_22_33_1529.kvx`.

#[path = "suites/budget_schema.rs"]
mod budget_schema;
#[path = "suites/disclosure.rs"]
mod disclosure;
#[path = "suites/grant_revocation_on_chain.rs"]
mod grant_revocation_on_chain;
#[path = "human_node.rs"]
mod human_node;
#[path = "suites/identity.rs"]
mod identity;
#[path = "suites/operator_route.rs"]
mod operator_route;
#[path = "suites/policy_version.rs"]
mod policy_version;
#[path = "suites/protocol2_restart.rs"]
mod protocol2_restart;

mod support;
use support::send_authorization;
#[path = "support/repair_22_33_1529.rs"]
mod repair_support;
