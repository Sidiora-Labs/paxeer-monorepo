# Identity provider availability

The gateway verifies Supabase access tokens against the project's JWKS at
`${SUPABASE_URL}/auth/v1/.well-known/jwks.json`. Its
[verifier](../../../../services/wallet/gateway/src/auth/jwt.ts) uses a ten-minute
key cache, a thirty-second refresh cooldown and a five-second fetch timeout.

Each attestor independently verifies tokens against `ATTESTOR_JWKS_URL`,
`ATTESTOR_JWT_ISSUER` and `ATTESTOR_JWT_AUDIENCE`. Its
[verifier](../../../../services/wallet/attestor/internal/auth/jwt/jwt.go)
refreshes the key set periodically, including for known key IDs. The defaults
are a five-minute refresh interval and a fifteen-minute maximum key-set age.
Verification refuses stale keys after that maximum age. `ATTESTOR_JWT_MAX_AGE`
bounds token age separately.

## Diagnose

Read the public gateway `/readyz` response. An identity dependency problem
appears under `components.identity_provider` with state `down` and a bounded
reason such as `jwks_http_error`, `jwks_empty` or `jwks_unavailable`.
Public readiness exposes states and reasons only.

The following commands use an operator-supplied gateway base in `GATEWAY_URL`:

```sh
curl --silent --show-error --max-time 10 "$GATEWAY_URL/readyz"
curl --silent --show-error --max-time 10 \
  -o /dev/null -w '%{http_code}\n' \
  "$SUPABASE_URL/auth/v1/.well-known/jwks.json"
```

A successful JWKS response must contain usable keys. If it answers from the
operator machine but not the gateway or attestors, check their egress, TLS trust
and configured issuer, audience and JWKS URL. Inspect the other readiness
components to distinguish identity availability from attestor, nonce-store or
RPC availability.

## Recover

Restore the configured provider or its connectivity. Keep valid cached key sets
available while recovery proceeds; restarting a process discards its in-memory
cache. Cached keys do not bypass token expiry, token age or key-set age checks.
Keep issuer, audience and signature checks enabled.

Read authenticated diagnostics when key counts are needed:

```sh
curl --silent --show-error --max-time 10 \
  -H "Authorization: Bearer $GATEWAY_ADMIN_TOKEN" \
  "$GATEWAY_URL/v1/admin/readiness"
```

The identity component reports `up` and a positive key count after recovery.
Confirm `/readyz` answers 200 with `ready: true`, then verify a fresh sign-in and
a consented signature through the gateway. Attestors fetch an unknown signing
key subject to their refresh limits; a key rotation does not require replacing
the configured issuer or audience.
