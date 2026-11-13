# Wallet gateway load test

[`wallet.js`](wallet.js) is a k6 script for the wallet gateway. Run it from
`services/wallet`. `API_BASE` sets the gateway base; the default is local HTTP
on port 8787.

```sh
k6 run gateway/loadtest/wallet.js
```

The `ramp` scenario starts at zero VUs and ramps to 10 over 30 seconds, to 100
over 60 seconds, to 500 over 60 seconds and back to 10 over 30 seconds. It allows
10 seconds for graceful ramp-down.

Without `PAXEER_JWT`, each iteration calls only `/healthz`. To include
`/v1/wallet/me`, supply a valid Supabase access token in the `PAXEER_JWT`
environment variable; the script sends it as a bearer token.

k6 exits non-zero when a threshold fails:

| Metric | Threshold |
| --- | --- |
| HTTP errors | < 1% |
| `/healthz` p95 / p99 | < 50 ms / < 200 ms |
| `/v1/wallet/me` p95 / p99 | < 200 ms / < 500 ms |
| `/v1/wallet/me` errors | < 1% |

The script sends no transactions.
