# Database TLS

Database TLS authenticates each server before Postgrex sends its startup message,
credentials or explorer data. The primary, replica and auxiliary repositories all
use `verify_peer` and HTTPS hostname matching. Postgrex derives SNI and the expected
certificate hostname from each repository's database endpoint; do not disable SNI
or set a shared hostname when repositories use different hosts.

Mount a PEM bundle containing the issuing CA trust roots into the running service
and set `DATABASE_SSL_CA_CERT_FILE` to its absolute path. The shared policy is
reevaluated by `config/runtime/prod.exs` at startup, after the umbrella database
configuration. With the variable unset, the system bundle at
`/etc/ssl/certs/ca-certificates.crt` supplies trust roots. A private database CA must
be installed in that bundle or supplied through the variable. Include the issuing
root and any required intermediate CA material; the server should send its leaf
and intermediate chain. The leaf must be valid for the hostname in that database's
URL. An empty, missing, unreadable or invalid bundle causes TLS connection failure;
it does not select another bundle or disable verification. Restart after changing
the bundle selection. Existing `ECTO_USE_SSL` behavior is preserved; remote
production database connections must keep TLS enabled.

The PEM private keys under `fixtures/` are public, disposable test identities.
Their certificates cover localhost, an unrelated hostname, and an untrusted CA.
Never use these keys or the fixture CA for a deployed database. `new_head_full.json`
is copied unchanged from the committed EthereumJSONRPC PaxeerX fixture; the test
runs its block through the real RPC decoder, block schema, Postgrex TLS transport,
and disposable PostgreSQL database. The loopback TLS proxy forwards PostgreSQL
bytes unchanged and provides no simulated database responses.
