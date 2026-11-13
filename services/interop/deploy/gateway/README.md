# Interop gateway deployment

The `layerx-interop-gateway` binary is the hosted protocol adapter service for
one Paxeer network. `paxd` includes the kernel that proposes blocks; EVM chain
id 125 (`0x7d`) identifies mainnet-beta. Public node JSON-RPC uses the root
path. The hosted gateway in `services/platform/hosted/gateway` supplies the
`px_*` methods, registration and funding; its contract is `openrpc.json`.
Sub-chains are tenant domains in the database, not additional node deployments.

The service relays payer-signed ledger transactions through the merged node
using `eth_sendRawTransaction` and reads `pax_sequencerReceipt`. Its Redis relay
journal retains the transaction before submission, rejects replacement bytes
for the same idempotency key, and resumes receipt lookup or submission on retry.
`LAYERX_INTEROP_SEQUENCER_URL` selects the transaction and receipt endpoint.

Signed receipt and log verification establish execution authenticity. The
relay reports `committed` only when the receipt response also names a block
whose hash matches `eth_getBlockByNumber`; otherwise it retains `receipted`.
This RPC block check is distinct from independently verifying a consensus
commit proof. On the network a receipt is final once its block is committed.
See the [x402 conformance notes](../../specs/conformance/x402/README.md) for
wire validation and recorded proof coverage, and the
[websearch sidecar](../../crates/x-websearch/README.md) for its payment and
program-relay configuration.

`runtime.env.example` lists the protocol scope and TAP clock inputs that the
interop service requires. `LAYERX_INTEROP_MODULE_REGISTRY_FILE` must point
to the authoritative module registry mounted from the same core declaration as
the hosted gateway;
[module-registry.example.json](module-registry.example.json) documents its
accepted shape and is not a production registry.

The main file selected by `LAYERX_INTEROP_CONFIG` must include the two arrays
shown in [visa-trust.example.json](visa-trust.example.json). Every `visa_agents`
entry declares an explicit `active` or `revoked` status. Every authenticated merchant principal
has exactly one canonical lowercase authority and canonical query-free path in
`visa_targets`. Missing targets, unknown keys, revoked keys, expired keys,
non-canonical targets, and duplicate principal targets fail closed at startup
or request admission.

The TAP skew is a server-owned deployment value in seconds, from zero through
300. It is never accepted from the public request. Production deployments must
replace every example identity, key, expiry, principal, module, and ordinal
with authenticated operator configuration.

Fiat provider callbacks carry an opaque `token_reference` beside a signed
evidence envelope. The evidence `facts` object must include
`token_reference_sha256`, the lowercase hexadecimal SHA-256 digest of those
exact token bytes. Providers sign the UTF-8 bytes
`LayerX/interop/fiat/provider-evidence/v1\0` followed immediately by the
compact JSON serialization of `facts`. The configured Ed25519 provider key
must verify that signature; evidence signed without the domain or for another
token is refused before any hold or protocol activity is admitted.
The callback does not accept an activity idempotency override. The service
derives the economic key from the authenticated provider, settlement, rail,
and evidence class, so retries converge and one settlement cannot be credited
again under a fresh caller-selected key.

Every AP2 asset binding declares one deployment-owned `audience`; all currency
bindings use canonical lowercase principal digests and are bounded and unique
per principal and currency. AP2 request bodies carry the signed nonce but
cannot override time, clock skew, audience, currency exponent, or activity
idempotency. The service verifies against its own clock with zero skew and
tries only the deployment-owned audience and exponent pairs for the
authenticated principal. Exactly one verified pair whose currency matches its
binding is required. The hosted execution key is derived from the canonical
authenticated principal and both verified mandate references. AP2 execution and
receipt export also require the protected signer file selected by
`LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE`.

## Image, configuration and cluster bring-up

`docker/interop-gateway/Dockerfile` builds `layerx-interop-gateway` from the
`layerx-interop-service` crate and runs it as UID/GID 4020. Configuration is
loaded from environment variables and the file named by
`LAYERX_INTEROP_CONFIG`; the binary accepts no command-line arguments.

The runtime needs its manifest and authoritative module registry, outbound CA
and PKCS#12 client identity, Redis credentials and a receipt-key file. A TLS
listener also needs its server certificate and key. `LAYERX_INTEROP_LISTENER=plain`
requires both server TLS variables to be unset. Required scope values include
`LAYERX_INTEROP_NETWORK_ID=eip155-125`, `LAYERX_INTEROP_CHAIN_ID=125`, a supported
`LAYERX_INTEROP_WIRE_VERSION`, and the deployment's nonzero
`LAYERX_INTEROP_PROTOCOL_NETWORK_ID`. The protocol network id is not inferred
from the EVM chain id. Set `LAYERX_INTEROP_READINESS_MAX_AGE_MS` to an integer
from 1 through 30000.

`LAYERX_INTEROP_SEQUENCER_URL` selects the merged node endpoint. Use HTTPS;
the client also allows loopback HTTP. Setting
`LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK=1` additionally allows
plain HTTP to a validated Railway-private endpoint with an explicit port. The
Redis URL requires `rediss`.

`LAYERX_INTEROP_RECEIPT_KEYS_FILE` selects a nonempty JSON array of objects
with exactly these camelCase fields:

| Field | Value |
| --- | --- |
| `keyId` | `0x`-prefixed 32-byte hexadecimal receipt key id. |
| `publicKey` | `0x`-prefixed 32-byte Ed25519 public key; the renderer requires lowercase hex and a nonzero key. |
| `firstHeight` | Inclusive unsigned 64-bit first authorized height. |
| `lastHeight` | Inclusive unsigned 64-bit last authorized height, at least `firstHeight`. |

Every `keyId` must be unique across the entire file, even for disjoint height
windows. The proof verifier selects by id before checking the receipt height;
repeating an id makes that selection ambiguous. The renderer refuses repeated
ids before writing the manifest. Different ids may share a public key, and
every distinct public key still appears in the advertised signer set.

The service and renderer reject overlapping ranges, including ranges assigned
to different keys. At startup the service fetches `px_getReceiptKeys` using
the configured endpoint and requires every local pin's id, public key and
height bounds to match a published entry. This check does not require every
published key to be pinned. Deploy the complete signer set needed for current
and retained receipts, and render `x402_supported` from that same file. The
service binds its advertised signer list to the distinct public keys in its
local pins. A key-list lookup failure or any absent or differing pin refuses
startup. Configure a node or hosted endpoint that serves `px_getReceiptKeys`
with the pinned receipt-key entries.

[`services/platform/hosted/interop/deployment.yaml`](../../../platform/hosted/interop/deployment.yaml)
provides the Kubernetes listener, runtime secret, receipt-key secret and network
policies. It mounts the receipt-key array separately at
`/run/layerx/receipt-keys/receipt-keys.json`. Set the protocol network id and
trust material for the deployment.

The image's `layerx-env-files` entrypoint stages file variables listed in
`docker/interop-gateway/files.tsv`. Supply `LAYERX_INTEROP_SEQUENCER_URL` and
provision the file selected by `LAYERX_INTEROP_RECEIPT_KEYS_FILE` before startup.
Mount the receipt-key array at the configured path independently of the
entrypoint's listed mappings. The [environment example](runtime.env.example)
lists protocol network, module-registry and TAP-skew settings.

[config.example.json](config.example.json) illustrates the manifest shape.
Derive suite counts and digests for all five adapters and three transport
bindings from the checkout being deployed. The example trust inputs are
[trust-inputs.example.json](trust-inputs.example.json) and
[receipt-keys.example.json](receipt-keys.example.json); the latter combines
two public fixture keys in disjoint windows.
These identities, keys, principals and accounts are examples, not production
trust. Generate pins from the checkout that will be deployed, using authenticated
counterparty material. The `layerx-beta-*` roots authenticate the network's own
test clients only.

To render from the example trust inputs at the repository root without
inherited manifest or conformance overrides:

```sh
env -i PATH="$PATH" \
  LAYERX_BETA_INTEROP_MANIFEST_FILE=services/interop/deploy/gateway/trust-inputs.example.json \
  python3 -B services/interop/deploy/gateway/render.py \
  --network-id eip155-125 \
  --receipt-keys-file services/interop/deploy/gateway/receipt-keys.example.json \
  --out services/interop/deploy/gateway/config.example.json
```

Rendering derives metadata; it does not establish that the conformance suites
passed. Run the adapter suites separately to check their behavior.

[render.py](render.py) derives specification and conformance pins and combines
them with deployment trust roots. From the repository root, after provisioning
the receipt-key file and the trust-root variables documented below, render with:

```sh
python3 services/interop/deploy/gateway/render.py \
  --network-id eip155-125 \
  --receipt-keys-file "$RECEIPT_KEYS_FILE" \
  --out "$INTEROP_CONFIG_FILE"
```

For private beta clients, add `--beta-roots-file "$BETA_ROOTS_FILE"` using the
generated roots described below. The renderer takes every distinct public key
from the receipt-key file and advertises it as `did:layerx:<lowercase-hex>` on
`eip155:125`. It validates overridden `x402_supported` networks and signers
against those inputs, so an override cannot silently select another authority.

`render.py --check` validates environment and manifest inputs without rendering
a deployment or loading beta roots into that check. It reports missing or
malformed external roots by variable name. Full rendering uses the
`--receipt-keys-file` argument shown above.

### Derived by `render.py`, never a deployment input

| Field | Source |
| --- | --- |
| `x402` specification, version `2.0.0`, digest | `services/interop/specs/vendor/x402/x402-specification-v2.md` |
| `ap2` specification, version `1.0.0`, digest | `services/interop/specs/vendor/ap2/specification.md` |
| `ucp` specification `ucp-checkout`, version `20260408`, digest | `services/interop/specs/vendor/ucp/specification-checkout.html` at the vendored `2026-04-08` revision |
| `visa-tap` specification, version `1`, digest | `services/interop/specs/vendor/visa-tap/README.md` |
| `fiat` specification `layerx-fiat-settlement`, version `1`, digest | `docs/wiki/FiatRamps.md`, the adapter's own surface description; there is no upstream |
| `http`, `mcp` and `a2a` binding version `2` and specification digests | `services/interop/specs/vendor/x402/transports/*.md`, the x402 v2 transport bindings vendored at the same pinned commit |
| `x402` suite `layerx-x402-conformance-v1`, its vector count and digest | `services/interop/specs/conformance/x402`, the model vectors read by `services/interop/crates/layerx-x402/tests/vectors.rs` and settlement vectors read by `sequenced_vectors.rs` |
| `ap2` suite `layerx-ap2-conformance-v1`, its vector count and digest | `services/interop/specs/conformance/ap2`, the vector files `services/interop/crates/layerx-ap2/tests/mandates.rs` runs through `MandateVerifier` |
| `ucp` suite `layerx-ucp-conformance-v1`, its vector count and digest | `services/interop/specs/conformance/ucp`, the JSON files under that directory; model and receipt harnesses are described in its README |
| `visa-tap` suite `layerx-visa-tap-conformance-v1`, its vector count and digest | `services/interop/specs/conformance/visa-tap`, the vector files `services/interop/crates/layerx-visa-tap/tests/conformance.rs` runs through `TapRequest` and credential verification |
| `fiat` suite `layerx-fiat-conformance-v1`, its vector count and digest | `services/interop/specs/conformance/fiat`, the token and journey files read by `adapter.rs` and `sequenced_vectors.rs` under `services/interop/crates/layerx-fiat/tests/` |
| `http`, `mcp` and `a2a` binding conformance digests | `services/interop/specs/conformance/transport-{http,mcp,a2a}`, the role-message vector files `services/interop/crates/layerx-x402/tests/transports.rs` runs through the production encoders and decoders |
| every adapter's `evidence_policy` | the policy the service already requires per adapter |
| `x402_supported` | this cluster's own facilitator declaration: the CAIP-2 form of the network the deployment serves, the `exact` scheme, and all distinct public keys in `--receipt-keys-file` as its signers |
| `ucp_payment_handler` | the `layerx-ucp-handler` declaration of the vendored UCP revision |

A first-party suite is derived from the JSON files in its conformance directory
and tied to those files by its pin: the identifier is
`layerx-<adapter>-conformance-v1`, the count is the number of vector records
under `services/interop/specs/conformance/<adapter>` (a transport binding's suite lives
under `transport-<binding>` and contributes its digest only), and the digest
covers each JSON file's path relative to its suite directory and its raw bytes,
with a NUL after each path and after each file's bytes. Editing bytes changes
the digest; adding or removing vector records changes the count too.
The renderer discovers files recursively in sorted path order. Array entries
count individually; a non-array document counts once. Rendering computes
metadata and does not run the adapter tests. `--self-test` contains source
inclusion assertions for the test harnesses it names; it is a separate check.

All eight suites are first party. Additional Rust cases are outside the JSON
record count: the fault-injected settlement cases in
`services/interop/crates/layerx-x402/tests/transports.rs`, which drive a receipt
signer and the canonical receipt encoder; the credential-binding case
`binding_is_scoped_non_authoritative_and_success_requires_a_real_receipt` in
`services/interop/crates/layerx-visa-tap/tests/conformance.rs`, which drives the binding
store and a canonical receipt; and the descriptor, exported identifier,
vendored-revision and receipt checks in
`services/interop/crates/layerx-ucp/tests/conformance_vectors.rs`.
Visa TAP presentations and the `journeys.json` fiat receipts are signed in Rust
from the declared vector inputs. Fiat's `sequenced-journeys.json` instead names
recorded producer evidence.

`x402_supported` and `ucp_payment_handler` default to the configured receipt
signers and the pinned UCP handler declaration. `LAYERX_BETA_INTEROP_X402_SUPPORTED`
and `LAYERX_BETA_INTEROP_UCP_PAYMENT_HANDLER` accept JSON overrides. A full
render still requires the x402 declaration's network and signer set to match
`--network-id` and `--receipt-keys-file`.

### Generated beta trust roots

The AP2 issuer keys, the AP2 asset binding, the Visa TAP agent and merchant
target and the fiat provider callback key are counterparty credentials. On a
private network the counterparties are the cluster's own test clients, so
`component_secrets_generate`, called by `secrets_generate` in
`services/platform/hosted/tests/beta-cluster.sh`, generates them —
three uncompressed SEC1 P-256 mandate keys, one ed25519 TAP agent key and one
ed25519 fiat provider key — and writes their public halves with the cluster
facts they bind to (the smoke client's principal digest and accounts, the node
asset, the interop service audience) to `$SECRETS_DIR/interop-beta-roots.json`.
`render.py --beta-roots-file` builds the pins from that material.

Every generated identifier says so: `layerx-beta-<use-case>-key`,
`layerx-beta-tap-key-1`, `layerx-beta-trusted-agent`, `layerx-beta-merchant`
and `layerx-beta-fiat-provider`. The rendered configuration carries no field
for provenance — the service refuses unknown fields — so the render prints the
roots it generated and the bring-up logs that line. These roots trust nothing
outside the cluster: they authenticate the network's own clients only.

### Deployment variables

No conformance variable is required. All eight suites are derived from this
checkout, so a bring-up that imports no external suite declares none of them.
External trust roots are required from variables, manifest overrides, or
`--beta-roots-file` during rendering; `--check` requires variables or manifest
overrides because it does not use beta-root arguments.

Optional — each overrides a value this checkout or this cluster already
produces:

| Variable | Value | How to produce it |
| --- | --- |---|
| `LAYERX_BETA_INTEROP_CONFORMANCE_X402` | `<suite-identifier>,<vector-count>,<suite-sha256>` | overrides the first-party x402 suite with an imported one |
| `LAYERX_BETA_INTEROP_CONFORMANCE_AP2` | same form | overrides the first-party AP2 suite with an imported one |
| `LAYERX_BETA_INTEROP_CONFORMANCE_UCP` | same form | overrides the first-party UCP suite with an imported one |
| `LAYERX_BETA_INTEROP_CONFORMANCE_VISA_TAP` | same form | overrides the first-party Visa TAP suite with an imported one |
| `LAYERX_BETA_INTEROP_CONFORMANCE_FIAT` | same form | overrides the first-party fiat provider-callback suite with an imported one |
| `LAYERX_BETA_INTEROP_CONFORMANCE_HTTP` | `<suite-sha256>` | overrides the first-party HTTP transport suite digest with the SHA-256 of an imported one |
| `LAYERX_BETA_INTEROP_CONFORMANCE_MCP` | `<suite-sha256>` | as above for MCP |
| `LAYERX_BETA_INTEROP_CONFORMANCE_A2A` | `<suite-sha256>` | as above for A2A |
| `LAYERX_BETA_INTEROP_AP2_KEYS` | JSON array | the mandate issuer keys the AP2 credential provider publishes |
| `LAYERX_BETA_INTEROP_AP2_ASSETS` | JSON array | one binding per principal and currency, from the merchant agreement and the asset the deployment settles in |
| `LAYERX_BETA_INTEROP_VISA_AGENTS` | JSON array | the trusted-agent keys the Visa TAP registry publishes |
| `LAYERX_BETA_INTEROP_VISA_TARGETS` | JSON array | the merchant authority and path each principal is authorised for |
| `LAYERX_BETA_INTEROP_FIAT_PROVIDERS` | JSON array | the ed25519 callback key of each card, bank or RTP provider under contract |
| `LAYERX_BETA_INTEROP_X402_SUPPORTED` | JSON object | x402 declaration override; its network and complete signer set must still match the render inputs |
| `LAYERX_BETA_INTEROP_UCP_PAYMENT_HANDLER` | JSON object | the declaration of a different UCP payment handler |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE` | path to a JSON document | overrides any rendered field, field by field |

The repository carries first-party vectors for all eight suites.
`services/interop/specs/vendor/CONFORMANCE.md` describes the vendored upstream
conformance material. The renderer refuses a suite with no vectors, a zero
digest, a malformed pin or an out-of-charset identifier. `ConformanceSuite`
records the declared suite metadata. Rendering computes pins; the adapter test
harnesses define the behavior exercised against their inputs. Declaring a trust
root replaces the generated beta root; authenticate that counterparty material.
The renderer refuses empty or malformed roots, and names required roots when
neither explicit inputs nor beta material supply them.

`LAYERX_BETA_INTEROP_MANIFEST_FILE` accepts a JSON document with optional
`adapters`, `transports`, `x402_supported`, `ap2_keys`, `ap2_assets`,
`ucp_payment_handler`, `visa_agents`, `visa_targets` and `fiat_providers` fields.
Each `adapters` entry can override `conformance_suite`, `conformance_vectors`
and `conformance_sha256`; each `transports` entry can override
`conformance_sha256`. The document is applied last, field by field.

Every field it sets is validated exactly as a variable is, and an unknown
adapter, transport or field is refused rather than ignored.

`render.py --self-test` contains renderer checks against the vendored documents
and vectors in this checkout: the refusal list
when nothing is declared, which names no conformance variable at all, the
derived digests against the provenance records, each of the eight first-party
suites against its vector files and the test that `include_str!`-s them
(including that editing one byte of one vector changes the digest), the
in-cluster defaults, the generated beta roots and the refusals for incomplete
key material, the field-by-field override, the variable overrides for an adapter
and a transport suite, and the refusals for an empty suite, a zero digest, a
malformed pin and an out-of-charset suite identifier.
