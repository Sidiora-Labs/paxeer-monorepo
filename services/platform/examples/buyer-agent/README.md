# Buyer agent deployment

The beta profile sends Human payment requests to
`<human-service-origin>`, an operator-supplied HTTPS origin for the Human
Service declared in `services/platform/hosted/human/deployment.yaml`. Its
`/v1/moves/quote`, `/v1/moves`, journey and evidence routes belong to the Human
HTTP API. `rpcUrl` remains the public node JSON-RPC root for chain receipts.

For a buyer outside that cluster, set `LAYERX_TESTNET_HUMAN_URL` to the
operator's reachable HTTPS Human service origin, with no path, query or user
information. The operator must supply DNS, TLS trust and network admission to
that service. The checked-in Service admits qualification pods and selected
internal adapters; it is not a public ingress. For a private CA, use Node's
`NODE_EXTRA_CA_CERTS` with the operator-provided PEM CA before starting Node.
Certificate verification must remain enabled.

Set `LAYERX_TESTNET_TOKEN` to a bearer assertion admitted by that Human
deployment's identity provider and provisioned principal policy, authorized for
the source account in `LAYERX_TESTNET_SOURCE`. A node RPC credential, browser
cookie or arbitrary token is not a Human bearer assertion. Set
`LAYERX_TESTNET_PAYMENT_KEY` to a unique payment idempotency key and
`LAYERX_TESTNET_RECEIPT_KEYS_JSON` to the independently provisioned node receipt
key pins (key ID, public key and inclusive height bounds).

Use `LAYERX_EXAMPLE_CONFIG` for a deployment-specific copy of the profile,
including the real seller's `resourceUrl`. Keep credentials in environment
inputs. Start with `npm run start:beta --workspace
@sidiora/layerx-example-buyer-agent` after the documented workspace install and
build. The buyer verifies the Human-produced payment evidence and the seller's
settlement before reporting `paid`.

`LAYERX_EXAMPLE_ENDPOINT` rebases several endpoint fields in the shared example
loader. When it is set, the beta buyer also requires the dedicated
`LAYERX_TESTNET_HUMAN_URL`; the Human origin must differ from the resulting
`rpcUrl`. Public beta node hostnames are always refused as Human origins.

The repair integration test runs in-process against loopback Human HTTPS and
node JSON-RPC fixtures, using the committed Human API transcript and the node's
signed `payExact` receipt vector. It exercises the actual beta configuration
loader, buyer, HTTP transport, paid API server, seller middleware and pinned
node receipt verifier. The test trusts only its committed loopback certificate;
TLS verification remains enabled. It requires no deployment inputs or live
credentials and checks that the Human bearer never reaches node RPC or the
seller. The test entry loads the production middleware TypeScript sources
in-process so the declared gate needs only the SDK build. Gate evidence is
retained privately by `hf-build` and `hf-done`; this fixture proof does not
certify a live deployment.
