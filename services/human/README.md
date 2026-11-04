# Human interface

`services/human/` contains the Rust services for person-facing authentication,
onboarding, custody, money movement, approvals and receipt-backed activity.
Paxeer mainnet-beta is one BFT network, with EVM chain id **125** (`0x7d`).
The kernel is part of that network and proposes blocks. A receipt is final when
its block is committed. Tenant domains are database namespaces, so a sub-chain
does not require a new chain deployment.

## Layout

The members in [`Cargo.toml`](Cargo.toml) are:

| Path | Contents |
| --- | --- |
| [`crates/layerx-human-service`](crates/layerx-human-service/README.md) | HTTP service and privileged components for authentication, onboarding, journeys, approvals, custody and activity |
| [`crates/layerx-intents`](crates/layerx-intents) | Typed intent compilation, disclosure checks and canonical protocol codecs |
| [`crates/layerx-human-identity-provider`](crates/layerx-human-identity-provider/README.md) | LXIP identity directory and optional tenant-bound wallet-binding reader |
| [`crates/layerx-human-security-provider`](crates/layerx-human-security-provider/README.md) | LXSP authenticator management and verified recovery-receipt disclosure |
| [`crates/layerx-human-kms`](crates/layerx-human-kms/README.md) | LXKP mutual-TLS custody provider and threshold attestor client |
| [`crates/layerx-network-gateway`](crates/layerx-network-gateway) | Typed client for hosted gateway account, balance, asset and network reads |
| [`crates/layerx-human-test-support`](crates/layerx-human-test-support/README.md) | Development helpers for stores, evidence and durable transaction admission |

Related applications and contracts:

| Path | Contents |
| --- | --- |
| [`schema/human-api`](schema/human-api/README.md) | Versioned HTTP/JSON contract consumed by the web application |
| [`apps/web`](../../apps/web) | Human web application and [`@layerx/ui`](../../apps/web/packages/layerx-ui/README.md) components |
| [`apps/wallet`](../../apps/wallet/README.md) | Wallet web application |
| [`services/wallet`](../wallet/README.md) | Wallet gateway, SDK workspace, demo, attestor, ceremony and deployment files |
| [`tools`](tools) | API generation, boundary, copy, schema, UI and dependency checks |

## Transaction and evidence boundaries

`layerx-intents` compiles the retained typed protocol intents and their
disclosures. It is not the encoder for every transaction byte: the shared
`layerx-crypto` EIP-1559 codecs and the KMS attestor path handle unsigned calls,
signatures and signed type-2 transactions. Bank sends target the bank precompile
at `0x0000000000000000000000000000000000001001`; KMS checks the recipient,
denomination and amount against the disclosed call before signing.

The service binds custody and authorizations to the authenticated principal and
tenant. EVM wallets use configured attestor key identities; their secp256k1
private material is not returned to the Human service. Onboarding includes a
gas-station sponsorship path, with successful matching binding evidence required
before that path reports completion. See the [service guide](crates/layerx-human-service/README.md)
and [KMS guide](crates/layerx-human-kms/README.md) for the corresponding configuration.

Submission acknowledgement, a verified execution receipt and committed block
evidence are distinct observations. A submitted transaction hash alone does not
prove execution or commitment. Hosted gateway reads are explicitly labeled
`gateway-reported`, not promoted to cryptographically verified evidence.

Public node JSON-RPC uses the root path of
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, plus
`https://api-mainnet-beta.paxeer.network`. The node namespaces are `debug`, `echo`,
`eth`, `net`, `pax`, `test` and `txpool`. Hosted `px_*` methods have their own
[OpenRPC contract](../platform/hosted/gateway/openrpc.json); do not assume that
public nodes serve those methods. `node.hyperpaxeer.com` is the library mirror,
not a JSON-RPC endpoint.

## Build and test

The repository-root Makefile provides these development targets:

```sh
make human-js-install   # agent TypeScript SDK and Human web dependencies
make human-build        # SDK, API checks, Rust workspace and web build
make human-test         # provisioning, Rust workspace and web tests
make human-check        # Rust, boundary, schema and web type checks
make human-lint         # copy, Rust, dependency and web lint checks
make human-check-ui     # UI checks, component build and web checks
```

`human-test` also builds native custody prerequisites and webhooks; its hosted
provisioning tests require the Python dependencies checked by that target.
Rust tests need the authenticated runtime clock. For a focused suite, use the
repository wrapper, for example:

```sh
sh tools/runtime/run-with-clock.sh cargo test --locked \
  --manifest-path services/human/Cargo.toml \
  -p layerx-human-identity-provider --test probe
```

The wrapper uses `LAYERX_RUNTIME_CLOCK_BIN` when provided, otherwise builds the
clock supervisor. If attaching to an existing clock, supply all three bindings
`LAYERX_RUNTIME_CLOCK_SOCKET`, `LAYERX_RUNTIME_CLOCK_PID` and
`LAYERX_RUNTIME_CLOCK_UID`. Production listener requirements are documented in
the provider guides above. The boundary checker used by `human-check`,
`human-lint` and `human-check-bundle` remains the source for encoding policy.
