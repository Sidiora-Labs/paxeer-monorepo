# Outbound TLS test targets

`tests.rs` is registered as `outbound_tls::tests` in the library and included
by the `repair_22_33_1531` integration target. Both execute
`private_ca_is_required_and_hostname_verification_remains_enabled` and
`human_authority_refuses_empty_ca_and_preserves_input_bounds`.

The loopback HTTPS server generates its CA and server certificates in process.
The linked production `RemoteHumanAuthority` sends an authenticated registry
request and decodes the response into the real module registry. The tests check
the bearer, trusted CA success, unrelated CA and wrong hostname rejection,
empty CA rejection, HTTPS, bearer length, deadline and response size bounds.
`outbound_tls::private_ca_refuses_empty_trust_roots` also preserves the direct
empty-root assertion in the library target.

The focused integration gate is:

```sh
cargo test --locked --manifest-path services/agent/crates/layerx-agentd/Cargo.toml --test repair_22_33_1531
```

It needs no environment variables, credentials, live endpoint or subprocesses.
The existing `outbound_tls::private_ca_checks_the_actual_server_identity`
library test remains a separate shared boundary harness with its existing
external binary requirements; this integration target does not invoke it.
