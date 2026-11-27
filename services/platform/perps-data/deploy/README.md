# Perps data deployment

`services.json` declares the service layout and environment inputs. The
`railway/` configurations declare the variables for the corresponding images.

Set `PERPS_TRUSTED_PROXY_CIDRS` on `perps-gateway` and `perps-explorer` to a
comma-separated list of the IPv4 or IPv6 CIDRs of the proxies in front of each
service. `config.Load` parses this input into `Config.TrustedProxies`, which
the gateway and explorer use to resolve client IPs for rate limiting.

Set the optional `PERPS_ALLOWED_ORIGINS` on `perps-gateway` to a comma-separated
list of `http` or `https` origins without paths, queries, fragments,
credentials or a trailing slash. Spaces around entries are trimmed; an origin
outside the list cannot use the gateway's WebSocket upgrades.

An unset or empty value trusts no proxy and keys requests by their TCP peer.
For trusted peers, `X-Real-IP` takes precedence; otherwise the service walks
`X-Forwarded-For` from the right to the first untrusted hop. Forwarding headers
from untrusted peers are ignored. Configure only the actual proxy ranges.

Whitespace and empty entries are ignored. Invalid CIDRs or more than 32 entries
refuse startup. The indexer, portal, replay and loadtest do not read this input.

`perps-quoter` is an owner-started manual process with zero default replicas and
no automatic restart. Start exactly one process for a dedicated account after
mounting its private key file. Its health listener serves `/healthz`,
`/healthz/live` and `/healthz/ready` on `PORT` (8081 by default); keep it private.
The shared Dockerfile accepts `SERVICE=perps-quoter`. The quoter image is
published as `ghcr.io/sidiora-labs/perps-quoter` alongside the indexer, gateway,
explorer and portal images; the owner pulls it by its published digest.
The public Railway services remain independent of this optional quoting account.
Configuration, collateral funding and shutdown are described in [the quoter guide](../../../../docs/perps/quoter.md).
