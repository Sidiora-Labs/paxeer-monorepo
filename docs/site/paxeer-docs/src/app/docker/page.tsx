import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/docker')

export default function Docker() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Docker Development</h1>
        <p className="page-description">
          Local Docker clusters, state-sync RPC nodes, and monitoring for Paxeer X development and testing.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>docker/</code>, <code>Makefile</code>, <code>chain.mk</code> (monorepo root)
      </div>

      <h2>Containers for the complete Paxeer X stack</h2>
      <p>Chain development containers cover EVM and Cosmos execution. The monorepo also provides images for the kernel, hosted gateway and core, receipt authority, registry, agent boundary, Human services, bridge relayer, and application surfaces. See <Link href="/relay-archive">archive access</Link> for retained evidence and mirrors.</p>
      <table><thead><tr><th>Image source</th><th>Role</th></tr></thead><tbody>
        <tr><td><code>docker/localnode/</code></td><td>Local chain node, genesis tooling and process supervision</td></tr>
        <tr><td><code>docker/platform-gateway/</code> and <code>docker/platform-core/</code></td><td>Public receipt-verifying ingress and internal core boundary</td></tr>
        <tr><td><code>docker/platform-authority/</code> and <code>docker/platform-agent-boundary/</code></td><td>Receipt authority and agent authorization boundary</td></tr>
        <tr><td><code>docker/platform-registry/</code> and <code>docker/human-service/</code></td><td>Discovery registry and Human service components</td></tr>
        <tr><td><code>docker/bridge-relayer/</code></td><td>Bridge delivery</td></tr>
      </tbody></table>
      <p>Hosted manifests under <code>services/platform/hosted/</code> define the service topology, persistent storage, probes, and infrastructure boundaries. The chain Docker Compose files below run their declared development topology; hosted services have their own configuration and dependencies.</p>
      <p>Mount persistent state separately from runtime sockets and credentials. Pin image revisions and matching genesis artifacts. Publish the gateway through TLS ingress and keep core administration, signer channels, and node-local listeners private. Check <code>/readyz</code> and <code>/livez</code> where the selected service defines them, then verify an application receipt and its block-commit evidence. See <Link href="/operators">operator roles</Link> and <Link href="/settlement">settlement</Link>.</p>

      <h2>Prerequisites</h2>

      <h3>macOS</h3>

      <p>
        Install Docker Desktop from <a href="https://docs.docker.com/desktop/install/mac-install/">docs.docker.com/desktop/install/mac-install/</a>
      </p>

      <h3>Ubuntu</h3>

      <p>
        Install Docker Engine and Docker Compose:
      </p>

      <ul>
        <li>Docker: <a href="https://docs.docker.com/engine/install/ubuntu/">docs.docker.com/engine/install/ubuntu/</a></li>
        <li>Docker Compose: <a href="https://docs.docker.com/compose/install/other/">docs.docker.com/compose/install/other/</a></li>
      </ul>

      <h2>Four-Node Local Cluster</h2>

      <p>
        The base Compose file declares four validators on the <code>localnet</code> bridge. Each local node process requires <code>LOCALNODE_DISPOSABLE=1</code> and the same fresh <code>LOCALNODE_RUN_ID</code>, a 16–128 character identifier starting with a letter or digit and containing only letters, digits, hyphens and underscores. Supply these through the container environment in your local deployment configuration.
      </p>

      <h3>Cluster Recipes</h3>

      <pre><code>{`# Cluster recipe: rebuild the localnode image and initialize node state
make docker-cluster-start

# Rebuild the image but reuse an existing paxd binary
make docker-cluster-start-skipbuild`}</code></pre>

      <p>
        The recipes invoke <code>docker/docker-compose.yml</code>, which declares four <code>pax-node-*</code> containers and <code>x-websearch</code> (<code>pax-x-websearch</code>). Set <code>X_WEBSEARCH_HOME</code> to its configured data directory. Image selection uses <code>LOCALNODE_IMAGE_TAG</code> and <code>X_WEBSEARCH_IMAGE_TAG</code>. The node coordinator stores genesis, node homes and logs under <code>build/localnode-runs/&lt;run-id&gt;/</code>. These recipes stop the cluster and remove <code>build/generated/</code> before starting.
      </p>

      <h3>Published Host Ports</h3>

      <table>
        <thead>
          <tr>
            <th>Node</th>
            <th>P2P</th>
            <th>RPC</th>
            <th>gRPC / gRPC-web</th>
            <th>EVM RPC</th>
            <th>Additional TCP port</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>node0</td>
            <td>26656</td>
            <td>26657</td>
            <td>9090-9091</td>
            <td>8545-8546</td>
            <td>26658</td>
          </tr>
          <tr>
            <td>node1</td>
            <td>26659</td>
            <td>26660</td>
            <td>9092-9093</td>
            <td>8547-8548</td>
            <td>26661</td>
          </tr>
          <tr>
            <td>node2</td>
            <td>26662</td>
            <td>26663</td>
            <td>9094-9095</td>
            <td>8549-8550</td>
            <td>26664</td>
          </tr>
          <tr>
            <td>node3</td>
            <td>26665</td>
            <td>26666</td>
            <td>9096-9097</td>
            <td>8551-8552</td>
            <td>26667</td>
          </tr>
        </tbody>
      </table>

      <h3>Monitor Logs</h3>

      <pre><code>{`# Tail logs for node 0
tail -f "build/localnode-runs/$LOCALNODE_RUN_ID/logs/paxd-0.log"

# View all logs
ls -l "build/localnode-runs/$LOCALNODE_RUN_ID/logs/"`}</code></pre>

      <h3>Open a Container Shell</h3>

      <pre><code>{`# List containers
docker ps -a

# Open a shell in a node
docker exec -it pax-node-0 /bin/bash`}</code></pre>

      <h3>Environment Variables</h3>

      <p>
        The Compose file passes the following settings to the nodes. The Make cluster recipes set <code>NUM_ACCOUNTS=10</code>; the initialization script defaults to 5 when it is unset.
      </p>

      <table>
        <thead>
          <tr>
            <th>Variable</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>NUM_ACCOUNTS</code></td>
            <td>Number of development accounts to create</td>
          </tr>
          <tr>
            <td><code>SKIP_BUILD</code></td>
            <td>Skip binary rebuild</td>
          </tr>
          <tr>
            <td><code>INVARIANT_CHECK_INTERVAL</code></td>
            <td>State invariant check frequency</td>
          </tr>
          <tr>
            <td><code>UPGRADE_VERSION_LIST</code></td>
            <td>Upgrade version list passed through the node environment</td>
          </tr>
          <tr>
            <td><code>MOCK_BALANCES</code></td>
            <td>Use mock balance data</td>
          </tr>
          <tr>
            <td><code>GIGA_EXECUTOR</code></td>
            <td>Enable Giga executor backend</td>
          </tr>
          <tr>
            <td><code>GIGA_OCC</code></td>
            <td>Enable optimistic concurrency control</td>
          </tr>
          <tr>
            <td><code>RECEIPT_BACKEND</code></td>
            <td>Receipt storage backend</td>
          </tr>
          <tr>
            <td><code>AUTOBAHN</code></td>
            <td>Enable Autobahn optimizations</td>
          </tr>
          <tr>
            <td><code>GIGA_STORAGE</code></td>
            <td>Use test_only_dual_write with the EVM state-store split</td>
          </tr>
          <tr>
            <td><code>GIGA_MIGRATE_FROM_MEMIAVL</code></td>
            <td>Start in memiavl_only mode for a migration scenario</td>
          </tr>
          <tr>
            <td><code>GIGA_FLATKV_ONLY</code></td>
            <td>Start in flatkv_only mode; cannot combine with GIGA_MIGRATE_FROM_MEMIAVL</td>
          </tr>
        </tbody>
      </table>

      <h2>Single Node (Not Recommended)</h2>

      <p>
        The single-node recipe builds its image and launches one container. Its node process requires the same explicit disposable-run environment described above:
      </p>

      <pre><code>{`make run-local-node`}</code></pre>

      <p>
        Use multiple validators when examining consensus behavior.
      </p>

      <h2>Compose Files</h2>

      <table>
        <thead>
          <tr>
            <th>File</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>docker/docker-compose.yml</code></td>
            <td>Four-node cluster (base configuration)</td>
          </tr>
          <tr>
            <td><code>docker/docker-compose.monitoring.yml</code></td>
            <td>Prometheus + Grafana monitoring overlay</td>
          </tr>
          <tr>
            <td><code>docker/docker-compose.giga-mixed.yml</code></td>
            <td>Giga executor with OCC on node0 and standard execution on nodes1–3</td>
          </tr>
        </tbody>
      </table>

      <h2>Monitoring with Prometheus & Grafana</h2>

      <p>
        The monitoring overlay requires <code>GRAFANA_ADMIN_PASSWORD_FILE</code> to name a private password file. These recipes start and stop the cluster with the monitoring overlay:
      </p>

      <pre><code>{`# Start cluster + monitoring together
make docker-cluster-start-monitoring

# Stop cluster + monitoring together
make docker-cluster-stop-monitoring`}</code></pre>

      <p>
        The standalone monitoring scripts use separate configuration under <code>docker/monitornode/</code>. Their Prometheus listener uses port 9091; select them separately from the Compose overlay, whose Prometheus host port is 9099:
      </p>

      <pre><code>{`# Start Prometheus
./docker/monitornode/scripts/start-prometheus.sh

# Start Grafana
./docker/monitornode/scripts/start-grafana.sh

# Stop
./docker/monitornode/scripts/stop-prometheus.sh
./docker/monitornode/scripts/stop-grafana.sh`}</code></pre>

      <h3>Access UIs</h3>

      <ul>
        <li><strong>Grafana:</strong> <code>3000</code>, using the configured password</li>
        <li><strong>Prometheus:</strong> <code>9099</code> for the Compose overlay; <code>9091</code> for the standalone script</li>
      </ul>

      <h2>State Sync RPC Node</h2>

      <p>
        RPC initialization requires <code>STATE_SYNC_RPC_SERVERS</code>, <code>STATE_SYNC_TRUST_HEIGHT</code> and <code>STATE_SYNC_TRUST_HASH</code> in the RPC container environment, plus a matching genesis at <code>build/generated/genesis.json</code>. It validates the selected checkpoint against the configured providers:
      </p>

      <pre><code>{`# Build the RPC-node image
make build-rpc-node

# RPC-node recipe, after supplying its container configuration
make run-rpc-node`}</code></pre>

      <p>
        State sync restores a trusted snapshot and then replays subsequent blocks. Select a retained snapshot height and matching 32-byte hexadecimal trust hash; there is no fixed 500-block startup threshold.
      </p>

      <div className="source-note">
        <strong>Scripts:</strong> <code>docker/rpcnode/scripts/</code>
      </div>

      <h2>Fast Iteration & Local Development</h2>

      <p>
        The node containers mount the source tree and Go caches. The declared image and cluster recipes support this workflow:
      </p>

      <ol>
        <li>Edit code at the monorepo root (modules, consensus, SDK, storage, WASM)</li>
        <li>Rebuild the node image: <code>make build-docker-node</code></li>
        <li>Restart the cluster: <code>make docker-cluster-start</code></li>
      </ol>

      <p>
        The root <code>go.mod</code> declares module dependencies and replacements. External Go modules, base images and system libraries must also be available to the build.
      </p>

      <h3>Volumes</h3>

      <p>
        The compose files mount:
      </p>

      <ul>
        <li><code>$PROJECT_HOME</code> → <code>/pax-protocol/pax-chain</code> (source tree)</li>
        <li><code>$GO_PKG_PATH/mod</code> → <code>/root/go/pkg/mod</code> (Go module cache)</li>
        <li><code>$GOCACHE</code> → <code>/root/.cache/go-build</code> (Go build cache)</li>
      </ul>

      <h2>Deployment Scripts</h2>

      <p>
        Each node type has a multi-step initialization flow:
      </p>

      <h3>Local Node</h3>

      <ul>
        <li><code>step0_build.sh</code> — Build <code>paxd</code> binary</li>
        <li><code>step1_configure_init.sh</code> — Initialize node configuration</li>
        <li><code>step2_genesis.sh</code> — Generate genesis file</li>
        <li><code>step3_add_validator_to_genesis.sh</code> — Add validator to genesis</li>
        <li><code>step4_config_override.sh</code> — Override config files (ports, peers)</li>
        <li><code>step5_start_pax.sh</code> — Start <code>paxd</code></li>
        <li><code>deploy.sh</code> — Orchestrates all steps</li>
      </ul>

      <h3>RPC Node</h3>

      <ul>
        <li><code>step0_build.sh</code> — Build <code>paxd</code></li>
        <li><code>step1_configure_init.sh</code> — Initialize for state-sync</li>
        <li><code>step2_start_pax.sh</code> — Start with state-sync enabled</li>
        <li><code>deploy.sh</code> — Orchestrates RPC node setup</li>
      </ul>

      <div className="source-note">
        <strong>Scripts:</strong> <code>docker/localnode/scripts/</code>, <code>docker/rpcnode/scripts/</code>
      </div>

      <h2>Docker Image</h2>

      <p>
        The <code>build-docker-node</code> target builds <code>docker/localnode/Dockerfile</code> and tags the image for local use and the Compose reference. <code>chain.mk</code> selects <code>linux/arm64</code> when <code>uname -m</code> reports arm64, otherwise <code>linux/amd64</code>. <code>DOCKER_PLATFORM</code> overrides that selection.
      </p>

      <h2>Network Configuration</h2>

      <p>
        The base Compose file declares a <code>localnet</code> bridge with fixed per-node addresses for peer configuration. Its published host port ranges map to the same internal node ports; application clients select the appropriate published RPC port.
      </p>

      <div className="prev-next">
        <Link href="/contracts">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Contracts</div>
        </Link>
        <Link href="/sdk">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">SDK</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
