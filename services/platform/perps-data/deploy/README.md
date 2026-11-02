# Perps data deployment

`services.json` declares the service layout and environment inputs. The
`railway/` configurations declare the variables for the corresponding images.

Set `PERPS_TRUSTED_PROXY_CIDRS` on `perps-gateway` and `perps-explorer` to a
comma-separated list of the IPv4 or IPv6 CIDRs of the proxies in front of each
service. `config.Load` parses this input into `Config.TrustedProxies`, which
the gateway and explorer use to resolve client IPs for rate limiting.

An unset or empty value trusts no proxy and keys requests by their TCP peer.
For trusted peers, `X-Real-IP` takes precedence; otherwise the service walks
`X-Forwarded-For` from the right to the first untrusted hop. Forwarding headers
from untrusted peers are ignored. Configure only the actual proxy ranges.

Whitespace and empty entries are ignored. Invalid CIDRs or more than 32 entries
refuse startup. The indexer, portal, replay and loadtest do not read this input.
