# Database TLS

The shared [database TLS policy](../database_tls.exs) configures
`ssl_opts` for the primary, replica, account, auxiliary and chain-specific
Explorer repositories. It sets `verify: :verify_peer` and HTTPS hostname
matching through `:public_key.pkix_verify_hostname_match_fun(:https)`.

`DATABASE_SSL_CA_CERT_FILE` selects the CA bundle. When unset, the policy uses
`/etc/ssl/certs/ca-certificates.crt`. The configured path is converted to a
character list and passed as `cacertfile`; the policy does not inspect the
bundle or substitute another path when a supplied path cannot be loaded.
The database certificate must validate against the selected trust roots and
the database endpoint's hostname when TLS is enabled.

[config/runtime/prod.exs](../runtime/prod.exs) loads this policy at startup
after the database configuration. `ECTO_USE_SSL` controls whether the
repositories use SSL. Changing the bundle selection requires restarting the
service so configuration is read again.

[fixtures/new_head_full.json](fixtures/new_head_full.json) contains a Paxeer
block notification used as input data by the database integration tests.
