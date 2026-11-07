# Disposable beta cluster

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`).
The kernel runs inside `paxd` and participates in block proposal and execution.
A receipt is final when its block is committed. Hosted services provide APIs,
indexing, identity and user interfaces around that chain. Sub-chains are tenant
domains in the database and do not require another node deployment.

The disposable Kubernetes harness is
`services/platform/hosted/tests/beta-cluster.sh`. It retains pre-merge bootstrap
and service wiring in this checkout: `beta_cluster_up` still calls
`wait_for_node_genesis`, `mirror_publish` and `settlement_publish`, and
`node_exec` still targets the retired `layerxd` container. These are migration
limitations of the harness, not the topology of the merged network. Do not use
its full `up` flow as a qualified merged-network deployment procedure.

## Make goals

`services/platform/Makefile.inc`, included by the root Makefile, exposes these
operator entry points:

| Goal | Behavior in this checkout |
| --- | --- |
| `make platform-test-tooling` | Runs the CLI tooling tests and script syntax checks; it does not start a cluster. |
| `make platform-beta-cluster-up` | Calls `beta-cluster.sh up $(PLATFORM_BETA_CLUSTER_FLAGS)`; subject to the migration limitations above. |
| `make platform-hosted-smoke` | Sources `PLATFORM_BETA_CLUSTER_ENV` (default `build/beta-cluster/env`), requires the smoke inputs, then calls `hosted-smoke.sh`. |
| `make platform-beta-cluster-down` | Calls `beta-cluster.sh down`; deletes disposable resources as described below. |

Make serializes the cluster up/down goals. A smoke environment file or a
successful image build does not establish that the merged node and its hosted
consumers are ready.

The script also accepts `render`, `images` and `publish-images <mode>`.
`render` writes manifests and material without applying Kubernetes resources;
it still needs the image used by `module_registry_generate` and the required
owner inputs. It is not a dependency-free preview. Image commands and their
publication modes are described below.

## Bring-up order

For a merged deployment, prepare and validate the chain genesis and node home,
start `paxd`, check the root JSON-RPC and committed chain progress, and then
configure hosted consumers against that chain. Receipt verification must use
the governed native signing keys and their valid block-height ranges.

The chain executable is built from `daemon/paxd`.
`docker/paxeer/Dockerfile.paxd-node` builds the kernel and program sandbox
libraries, links them into `paxd` through `modules/lxphost`, and supplies the
runtime libraries. `node/app.go` installs the host and the native sequencing
integration. The `paxd` image extends that node image with the disposable-chain
initializer; see `services/platform/hosted/tests/beta-images.sh` for the image
recipes and dependency order.

For an already prepared home, the primary container in
`services/platform/hosted/paxeer/deployment.yaml` runs:

```bash
paxd start --home /var/lib/paxeer
```

The merged network replaces the pre-merge independent execution and settlement
flow. The retired `layerxd --custody-profile` launch, standalone receipt-replica
bootstrap, GuarantorBond/CheckpointRegistry deployment and archived-batch
settlement are not steps in merged-node bring-up.

Hosted components remain independently deployable consumers. See
[Hosted gateway](HostedGateway.md), [Hosted identity](HostedIdentity.md),
[Hosted registry](HostedRegistry.md), [Hosted Human](HostedHuman.md),
[Hosted webhooks](HostedWebhooks.md) and [Hosted internal](HostedInternal.md)
for their service-specific configuration. A service manifest or image name does
not introduce another consensus or finality authority.

## Ordinary-genesis bootstrap

`services/platform/hosted/paxeer/init-chain.sh` is a compatibility entry
point for the disposable Paxeer StatefulSet. It refuses any `LAYERX_PAXEER_*`
variable and the retired `--custody-genesis`, `--deposit-root` and
`--sequencer-authorizations` arguments with exit 2, and otherwise runs
`paxeer-native-tool bootstrap` with
[`services/platform/hosted/node/bootstrap.sh`](../../services/platform/hosted/node/bootstrap.sh).
Supply that bootstrap's `--data-dir`, `--native-inputs` and signing-file
arguments explicitly; native bank and ledger genesis funding belongs to it.
Preserve the native chain's configured genesis and upgrade state; do not copy
old separate-node genesis artifacts into a new merged deployment.

The harness configuration uses these local listeners in `config.toml` and `app.toml`:

| Listener | Default | Configuration |
| --- | --- | --- |
| Comet RPC | `127.0.0.1:26657` | `config.toml` `[rpc].laddr` |
| P2P | `127.0.0.1:26656` | `config.toml` `[p2p].laddr` |
| gRPC | `127.0.0.1:9090` | `app.toml` `[grpc].address` |
| EVM HTTP | `127.0.0.1:8545` | `app.toml` `[evm].http_enabled`, `http_address`, `http_port` |

The primary TLS
boundary listens on `0.0.0.0:9443`, proxies root JSON-RPC to
`http://127.0.0.1:8545`, and uses `LAYERX_PAXEER_COMET_URL` set to
`http://127.0.0.1:26657` for genesis reads. These are local deployment settings,
not public-node addresses.

## Custody and module ownership

Ledger, authority, payments and program execution belong to the merged native
chain. The old custody-profile and LNI bootstrap procedure is retired as a way
to start an independent executor. Do not generate a separate trust history or
mount a custody profile to establish merged-network finality.

The harness still records the legacy custody address
`0x0000000000000000000000000000000000001013` and anchor address
`0x0000000000000000000000000000000000001014`. Their presence in configuration
does not make them active settlement services. The merge retirement gates are
implemented in `precompiles/layerxcustody/layerxcustody.go` and
`precompiles/layerxanchor/layerxanchor.go`: retired custody calls and anchor
writes refuse with `LXP_MODULE_DISABLED`; anchor reads also depend on the later
store-deletion gate. Do not use a legacy custody deposit or checkpoint write as
a readiness test for the merged chain.

## Primary node and observer node

The Paxeer StatefulSet declares one primary replica and a 40Gi `data` PVC at
`/var/lib/paxeer`. `paxeer_observer_render` appends an observer with its own
`observer-data` PVC and home `/var/lib/paxeer-observer`. Its init container copies
and compares the primary genesis, sets mode `full`, and connects to the primary
node ID at `127.0.0.1:26656` as a persistent peer.

The observer runs `paxd start --home /var/lib/paxeer-observer`. Its Comet
RPC/P2P ports are `26667`/`26666`, and its EVM HTTP port is `8555`.
`observer-boundary` listens on `9444` and proxies to that EVM listener; Service
`paxeer-observer-boundary` exposes port `9443` targeting `observer-https`.
Both nodes follow the same chain and genesis. The observer is not an additional
execution or settlement network.

## TLS origins, CA bundle and root JSON-RPC

For the local harness, `paxeer_origins_write` records these values in
`build/beta-cluster/paxeer/rpc-origins.json`:

| Field | Local value |
| --- | --- |
| `rpc_origins` | `https://localhost:19449`, `https://localhost:19452` |
| `ca_bundle` | `build/beta-cluster/ca/ca.pem` |
| `key_file` | `build/beta-cluster/secrets/paxeer-deployer.key` |
| `backends` | Primary and observer container names and their separate homes |

The forwards are primary `19449:9443` and observer `19452:9443`.
`paxeer-identity.py` checks both origins with that CA, rejects redirects,
requires byte-identical genesis bodies, validates the
`X-LayerX-Genesis-SHA256` header, compares the Comet chain ID and requires
`eth_chainId` to return `125`. It records `genesis_sha256`, `comet_chain_id`,
`chain_id` and `ca_sha256`. Chain ID alone cannot distinguish this disposable
chain from mainnet-beta. These genesis routes belong to the local boundary;
do not assume the public nodes expose them.

After starting the local node and boundary, confirm the chain at the root:

```bash
curl --fail --silent --show-error --max-time 20 \
  --cacert build/beta-cluster/ca/ca.pem \
  https://localhost:19449 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected result is `"0x7d"`. Compare `eth_blockNumber` and committed block
evidence on the primary and observer before relying on indexed results or
receipts. Keep private keys, generated credentials and local inventories out of
published documentation and application bundles.

Public mainnet-beta JSON-RPC uses the root of
`https://api-mainnet-beta.paxeer.network` or the numbered hosts
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. The public node namespaces are
`debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`.
The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in `services/platform/hosted/gateway/openrpc.json`.
The former router and prefixed RPC endpoints are retired.
`node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.
See the site's [public RPC reference](../site/docs/reference/public-rpc.md)
for the node list and client examples.

## Image source, GHCR pull and publication

`IMAGE_NAMES` and `image_source` in
`services/platform/hosted/tests/beta-images.sh` are the shared image inventory.
They cover `paxd-node`, `paxd`, the hosted services, bridge, explorer, wallet and
supporting workloads. Use that table instead of maintaining a copied list.
Some entries still belong to the pre-merge harness; the inventory is not a list
of processes required for consensus. The reference ramp image is selected only
when the optional ramp is enabled.

`LAYERX_BETA_IMAGE_SOURCE=build` is the default. `build_images` packages tracked
working-tree files and builds the selected images, recording their IDs in
`build/beta-cluster/images` and logs in `build/beta-cluster/logs`.
`LAYERX_BETA_IMAGE_SOURCE=ghcr` uses `LAYERX_BETA_IMAGE_TAG` (default `beta`),
requires `LAYERX_BETA_RELEASE_CANDIDATE` and `LAYERX_BETA_PUBLICATION_DIR`,
verifies the publication, and pulls by the expected registry digest before
retagging locally. A moving tag alone is insufficient.

`beta-cluster.sh images` prepares images without creating a cluster.
`beta-cluster.sh publish-images check` validates the local inventory without
publishing. Publication requires every entry in `IMAGE_NAMES`, all from one
revision and matching the local image IDs; a partial workload inventory is
refused. The wrapper also accepts `dry-run`, `push`, `verify`, `promote` and
`self-test`, forwarding options to `publish-images.sh`. Push publishes immutable
revision tags with signed release evidence; promotion verifies signatures,
SBOM attestations and provenance before moving release tags. Image publication
is a separate release operation from starting or checking a disposable node.

## Builder environment and sealed upload

`LAYERX_BETA_BUILDER_ENVIRONMENT_DIR` can name a hermetic builder root with
`bin/layerx-build`. Otherwise `require_builder_environment` constructs it with
`services/platform/hosted/registry/builder-environment/build-env.sh` from the
committed `programs/vendor` and recipe trees. Construction requires Docker,
git, tar, Python and sha256sum, and refuses uncommitted changes to those trees.

`LAYERX_BETA_BUILDER_ENVIRONMENT_CACHE_DIR` defaults to
`build/builder-environment`. Entries are keyed by a SHA-256 over the committed
vendor and recipe tree IDs. Reuse checks `environment-tree-digest`; fresh builds
use a staging directory renamed into place only on success. Teardown of the
cluster leaves this cache intact.

`builder_release_publish` records the environment, bwrap and cgroup-exec
digests, applies PVC `layerx-program-builder-release`, and starts
`layerx-program-builder-loader` with a pinned image. It copies the rootfs,
removes write permissions and creates `/opt/layerx-builder/.sealed`. The loader
must finish successfully before it is deleted. Builder readiness is distinct
from chain readiness.

## Registry and retained material

`module_registry_generate` still invokes `layerx-module-registry` in the
`layerx-node` image. `module_registry_verify` still reads the old LNI socket;
this is pre-merge harness wiring, not a merged public interface. Do not use an
example registry or successful LNI preparation as proof of the merged chain's
module configuration.

The harness supports `LAYERX_BETA_RETAIN_MATERIAL=1` for reuse against a live
cluster. `material_prepare` checks the inventory, ownership and permissions,
configured profile, regenerated registry and live KMS seal. It also requires
saved context and the native `receipt-keys.json` list. Missing or inconsistent
material refuses reuse. Retained `render` performs those live checks too.
This mechanism preserves an existing harness installation; it does not migrate
its pre-merge architecture into the merged network.

Fresh gateway material requires `LAYERX_BETA_RECEIPT_KEYS_FILE`, an export of the
native governed ReceiptKeys list. `sequencer-pins.py` validates nonzero base64
32-byte key IDs and public keys and inclusive `first_height`/`last_height`
ranges, then mounts the normalized list through Secret
`layerx-gateway-receipt-keys` and `LAYERX_GATEWAY_RECEIPT_KEYS_FILE`.
The gateway manifest removes the old single-key and batch-range pins. Do not
substitute a generated local trust history for the governed list.

## Explorer read principal

`explorer_read_principal_generate` creates an Ed25519 key and seed with the
other service credentials; the explorer read key is not an owner input.
The registry deployment mounts it through `LAYERX_EXPLORER_READ_KEY_FILE` at
`/run/layerx/explorer-read/read-key`, together with a verification key and CA.
These credentials are part of the retained material inventory.

The checked-in registry manifest still points `LAYERX_EXPLORER_READ_ENDPOINT`
at the pre-merge pending-core service and names a receipt-authority upstream.
Those bindings need migration with the harness. A generated read principal,
rendered explorer manifest or populated index alone does not prove that reads
come from the merged native chain. Verify the read consumer against its actual
chain producer before exposing the explorer as ready.

## Teardown

Without `LAYERX_BETA_KUBECONFIG`, the harness selects kind mode with
`LAYERX_BETA_CLUSTER_NAME` defaulting to `layerx-beta`. A nonempty kubeconfig
selects owner mode. Work products, material, logs and state live under
`build/beta-cluster`.

`beta-cluster.sh down` stops port-forwards. With recorded kind mode (or no
recorded mode) it deletes the named kind cluster when the kind tool and cluster
are present. In owner mode, with executable `build/bin/kubectl` and a readable
owner kubeconfig, it deletes namespaces `layerx-testnet`, `layerx-developer`
and `layerx-internal`. It removes inventoried and cluster-labeled images and
`build/beta-cluster`; `LAYERX_BETA_KEEP_TOOLS=1` preserves the pinned tools.
Retained-material mode does not survive deletion of the cluster, its volumes
and local material. Confirm the target context before using this destructive
operator command.

## Not covered

This page documents the merged chain behavior and the remaining limits of the
checked-in disposable harness. Image preparation, rendering and legacy hosted
smoke results do not qualify a merged deployment, production upgrade, bridge
funding path or explorer index. Those require evidence from the actual native
producer, transport and consumer against committed chain state, including
recovery where state persists. The site operator page also retains pre-merge
instructions and must not override the merged topology described here.

[Home](Home.md)
