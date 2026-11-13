# Disposable beta cluster

The Kubernetes harness starts a disposable Paxeer BFT chain with EVM chain ID
`125` (`0x7d`) and Comet chain ID `hyperpax_125-1`. The kernel runs inside
`paxd`; a transaction is final when its block is committed. This local chain
has its own genesis and trust material, even though its EVM chain ID matches
mainnet-beta. Sub-chains are tenant domains within a chain.

The entry point is
[`beta-cluster.sh`](../../services/platform/hosted/tests/beta-cluster.sh).
Its `up` and `render` flows delegate to
[`beta-native.sh`](../../services/platform/hosted/tests/beta-native.sh).

## Commands

Run these Make goals from the repository root:

| Goal | Behavior |
| --- | --- |
| `make platform-test-tooling` | CLI tooling tests and script syntax checks |
| `make platform-beta-cluster-up` | Native cluster bring-up with `PLATFORM_BETA_CLUSTER_FLAGS` |
| `make platform-hosted-smoke` | Hosted-service checks using `PLATFORM_BETA_CLUSTER_ENV`, default `build/beta-cluster/env` |
| `make platform-beta-cluster-down` | Disposable cluster teardown |

The script also accepts `render`, `images` and `publish-images <mode>`.
Native `up` checks committed-receipt readiness and applies the node manifest.
Hosted services are separately configured consumers; the native environment
file alone does not supply their smoke inputs. `--boundary-checks` is refused
by the native bring-up command.

## Bring-up

`up` requires Docker, curl, Python 3, git, Go, jq, OpenSSL and tar. It installs
the pinned cluster tools, prepares and loads images, creates or selects the
cluster, prepares native material, applies the native Secret and node manifest,
waits for node readiness, then checks a committed transaction receipt.

Without `LAYERX_BETA_KUBECONFIG`, it uses a local kind cluster named by
`LAYERX_BETA_CLUSTER_NAME` (default `layerx-beta`). A configured kubeconfig
selects owner-cluster mode. Working material is under `build/beta-cluster`.
`LAYERX_BETA_READY_TIMEOUT` defaults to `900` seconds.

The default genesis preparation initializes a validator with:

```sh
paxd init layerx-beta --chain-id hyperpax_125-1 --mode validator
```

The harness then funds validator and treasury accounts in native genesis,
collects the validator gentx, generates a receipt-signing key, configures the
native runtime and validates genesis. `LAYERX_BETA_NATIVE_HOME` instead imports
a prepared home after validation and requires `LAYERX_BETA_TREASURY_KEY_FILE`.
Keep validator, treasury and receipt keys private.

## Node and retained material

The [native deployment](../../services/platform/hosted/node/deployment.yaml)
runs one `layerx-node` StatefulSet replica with a `40Gi` data PVC mounted at
`/data`. The `paxd` container runs:

```sh
python3 /opt/layerx/kernel_runtime.py --home /data/layerx/node
```

| Listener | Container port | Local forward |
| --- | --- | --- |
| P2P | `26656` | None |
| EVM JSON-RPC | `8545` | `19446` |
| Comet RPC | `26657` | `19461` |
| Health (`/readyz`) | `9460` | None |

The init container validates native material before populating the home. The
material manifest binds genesis, configuration, validator and node keys,
receipt key, validator state, treasury policy and receipt-key list. On a retained
home it compares the manifest and immutable files; validator state may advance.
It refuses partial state or changed chain and treasury material.

`LAYERX_BETA_RETAIN_MATERIAL=1` reopens the saved native material and requires
the retained readiness transaction. Reuse preserves the original chain and
trust roots; it does not initialize another validator over the retained home.

## Readiness and client environment

The harness forwards the node root RPC to local port `19446` and Comet RPC to
`19461`. It submits a zero-value treasury transaction when there is no retained
readiness transaction. The native readiness checker verifies signed receipt
and committed block evidence against the saved chain and key material. A
successful chain-ID response alone is insufficient.

On success, `build/beta-cluster/env` exports `LAYERX_NODE_URL`,
`LAYERX_PAXEER_RPC_URL` and `LAYERX_CHAIN_ID=125` for the local node. Use the
exported root URL for JSON-RPC:

```sh
. build/beta-cluster/env
curl --fail --silent --show-error --max-time 20 "$LAYERX_NODE_URL" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected result is `0x7d`. Public mainnet-beta endpoints are listed in the
[public RPC reference](../site/docs/reference/public-rpc.md); their genesis and
receipt trust material are distinct from this disposable chain. The node's
registered namespaces and methods are described in [EVM JSON-RPC](../system/rpc.md).

## Rendering and images

`render` writes native material and manifests without applying Kubernetes
resources. It requires Docker, Python 3, git, Go, jq and OpenSSL, and uses the
native image for genesis preparation. It is not an image-free preview.

[Image recipes](../../services/platform/hosted/tests/beta-images.sh) define the
shared inventory and build dependencies. `LAYERX_BETA_IMAGE_SOURCE=build` is the
default. `ghcr` mode uses the configured image tag, release candidate and
publication directory to verify and pull the expected registry digests. The
image inventory also contains independently deployable hosted consumers.

`beta-cluster.sh images` prepares images without starting a cluster.
`publish-images` supports `check`, `dry-run`, `push`, `verify`, `promote` and
`self-test`, forwarding publication options to
[`publish-images.sh`](../../services/platform/hosted/tests/publish-images.sh).
Publication verifies the complete inventory and its source and image identities;
it is separate from node bring-up.

## Teardown

`beta-cluster.sh down` stops port-forwards. In kind mode it deletes the selected
disposable kind cluster. In owner mode it deletes the managed testnet,
developer and internal namespaces using the recorded owner context. It removes
the harness work directory and inventoried or cluster-labeled images.
`LAYERX_BETA_KEEP_TOOLS=1` preserves the pinned tools. Confirm the selected
cluster before teardown; retained-material mode does not preserve volumes or
material after their deletion.

[Home](Home.md)
