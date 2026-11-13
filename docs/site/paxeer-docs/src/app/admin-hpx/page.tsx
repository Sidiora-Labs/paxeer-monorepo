import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/admin-hpx')

export default function AdminHpx() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Admin & HPX</h1>
        <p className="page-description">
          Runtime administration, the HPX node distribution, and operational controls for Paxeer X.
        </p>
      </div>

      <div className="source-note">
        <strong>Admin:</strong> <code>admin/</code><br />
        <strong>HPX:</strong> <code>tools/hpx/</code>
      </div>

      <h2>Operating the unified network</h2>
      <p>The HPX distribution installs and manages the Paxeer mainnet-beta BFT node, paxd. The node includes the EVM and native kernel on one network. EVM chain ID 125 identifies this network, and a receipt is final when its block is committed.</p>
      <p>Runtime logging administration is separate from economic and security administration. Chain governance controls native parameter and admin messages through <code>modules/layerxgov/</code>. Module configuration follows the authorization model implemented by that module.</p>
      <p>Monitor consensus health, block progress, transaction receipts, node connectivity, and the state of the modules used by your applications. The <Link href="/network-parameters">parameter reference</Link>, <Link href="/settlement">settlement lifecycle</Link>, and <Link href="/layerx-kernel">kernel architecture</Link> explain those responsibilities. Application operators integrate through the <Link href="/platform-api">platform API</Link>.</p>

      <h2>Admin gRPC Service</h2>

      <p>
        The Admin service provides runtime log level control for <code>paxd</code> without restarting the node. It runs on a dedicated loopback-only gRPC server for security.
      </p>

      <h3>Configuration</h3>

      <p>
        Enable in <code>app.toml</code>:
      </p>

      <pre><code>{`[admin_server]
admin_enabled = true
# admin_address defaults to a loopback listener on port 9095.`}</code></pre>

      <p>
        The configured <strong>admin_address</strong> must contain a numeric loopback IP and port, such as an <code>IPv4</code> or <code>IPv6</code> loopback listener. The default port is 9095. The enabled server validates this at startup.
      </p>

      <h3>gRPC Methods</h3>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>Request</th>
            <th>Response</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>SetLogLevel</code></td>
            <td><code>pattern</code>, <code>level</code></td>
            <td><code>pattern</code>, level and affected count</td>
            <td>Change log level for loggers matching pattern</td>
          </tr>
          <tr>
            <td><code>GetLogLevel</code></td>
            <td><code>logger</code></td>
            <td><code>logger</code> and level</td>
            <td>Get current log level for a logger</td>
          </tr>
          <tr>
            <td><code>ListLoggers</code></td>
            <td><code>prefix</code> (optional)</td>
            <td>List of loggers</td>
            <td>List all loggers and their levels</td>
          </tr>
        </tbody>
      </table>

      <h3>SetLogLevel Pattern Matching</h3>

      <p>
        The <code>pattern</code> parameter supports:
      </p>

      <ul>
        <li><strong>Exact match:</strong> <code>"evm"</code> (sets log level for the EVM logger only)</li>
        <li><strong>Glob:</strong> <code>"evm*"</code> (sets log level for all loggers starting with "evm")</li>
        <li><strong>All loggers:</strong> <code>"*"</code> (sets log level for every logger)</li>
      </ul>

      <h3>Log Levels</h3>

      <table>
        <thead>
          <tr>
            <th>Level</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>debug</code></td>
            <td>Verbose debugging information</td>
          </tr>
          <tr>
            <td><code>info</code></td>
            <td>General operational information</td>
          </tr>
          <tr>
            <td><code>warn</code></td>
            <td>Warning messages (potential issues)</td>
          </tr>
          <tr>
            <td><code>error</code></td>
            <td>Error messages (failures)</td>
          </tr>
        </tbody>
      </table>

      <h3>Example: Using grpcurl with the protocol definition</h3>

      <pre><code>{`# Run from the repository root; set ADMIN_ENDPOINT to the loopback listener.
# List all loggers
grpcurl -plaintext -import-path api -import-path sdk/third_party/proto \\
  -proto pax/admin/v0/admin.proto "$ADMIN_ENDPOINT" \\
  paxprotocol.paxchain.admin.v0.AdminService/ListLoggers

# Set EVM logger to debug
grpcurl -plaintext -import-path api -import-path sdk/third_party/proto \\
  -proto pax/admin/v0/admin.proto -d '{"pattern":"evm","level":"debug"}' \\
  "$ADMIN_ENDPOINT" \\
  paxprotocol.paxchain.admin.v0.AdminService/SetLogLevel

# Get log level for a specific logger
grpcurl -plaintext -import-path api -import-path sdk/third_party/proto \\
  -proto pax/admin/v0/admin.proto -d '{"logger":"evm"}' \\
  "$ADMIN_ENDPOINT" \\
  paxprotocol.paxchain.admin.v0.AdminService/GetLogLevel`}</code></pre>

      <h3>Security</h3>

      <p>
        The Admin service is loopback-only by design:
      </p>

      <ul>
        <li>The server applies no application authentication; access is bounded by loopback.</li>
        <li>Cannot be bound to external interfaces</li>
        <li>Must access from the same machine as <code>paxd</code></li>
      </ul>

      <p>
        To use the same client through SSH, set the example shell variables to your forwarding specification and SSH login:
      </p>

      <pre><code>{`ssh -N -L "$ADMIN_FORWARD" "$NODE_LOGIN"`}</code></pre>

      <div className="source-note">
        <strong>Proto:</strong> <code>api/pax/admin/v0/admin.proto</code><br />
        <strong>Implementation:</strong> <code>admin/server.go</code>, <code>admin/service.go</code>, <code>admin/config.go</code>
      </div>

      <h2>HPX: Paxeer Node Distribution</h2>

      <p>
        HPX provides the installer, node manager, and peer registry for Paxeer mainnet-beta (<code>hyperpax_125-1</code>). Nodes run a native <code>paxd</code> binary under systemd. Release artifacts are served by the HPX mirror.
      </p>

      <h3>Chain Identifier</h3>

      <p>
        <code>hyperpax_125-1</code> (Cosmos-style, EVM chain ID <code>125</code>)
      </p>

      <h3>Install a Node</h3>

      <pre><code>{`# From the repository root on a supported Linux node
sudo bash tools/hpx/get-hpx.sh`}</code></pre>

      <p>
        The installer:
      </p>

      <ol>
        <li>Downloads and verifies the HPX CLI against <code>checksums.txt</code></li>
        <li>Places <code>hpx</code> in <code>/usr/local/bin/</code> and offers to run setup</li>
      </ol>

      <h3>Setup a Node</h3>

      <pre><code>{`# Set node type (fullnode or validator)
export HPX_TYPE=fullnode

# Run setup
hpx setup`}</code></pre>

      <p>
        The setup flow:
      </p>

      <ol>
        <li>Downloads <code>paxd</code>, <code>libwasmvm</code> runtimes, genesis, and configuration</li>
        <li>Verifies all artifacts against <code>checksums.txt</code></li>
        <li>Installs artifacts under <code>/root/.paxeer/</code>, overridable with HPX_HOME</li>
        <li>Configures systemd service</li>
        <li>Starts <code>paxd</code></li>
      </ol>

      <h3>HPX CLI Commands</h3>

      <table>
        <thead>
          <tr>
            <th>Command</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>hpx status</code></td>
            <td>Show node sync status</td>
          </tr>
          <tr>
            <td><code>hpx info</code></td>
            <td>Show node configuration</td>
          </tr>
          <tr>
            <td><code>hpx logs</code></td>
            <td>Tail <code>paxd</code> logs</td>
          </tr>
          <tr>
            <td><code>hpx</code></td>
            <td>Interactive menu</td>
          </tr>
          <tr>
            <td><code>hpx start</code> / <code>hpx stop</code> / <code>hpx restart</code></td>
            <td>Control the <code>paxd</code> systemd service</td>
          </tr>
          <tr>
            <td><code>hpx validator keygen</code></td>
            <td>Generate this node's operator account</td>
          </tr>
          <tr>
            <td><code>hpx validator stake</code></td>
            <td>Submit create-validator from the funded operator account</td>
          </tr>
          <tr>
            <td><code>hpx validator status</code></td>
            <td>Show validator state</td>
          </tr>
          <tr>
            <td><code>hpx update</code></td>
            <td>Update <code>paxd</code> and libraries</td>
          </tr>
          <tr>
            <td><code>hpx peers show</code></td>
            <td>Show known peers</td>
          </tr>
          <tr>
            <td><code>hpx peers refresh</code></td>
            <td>Fetch latest peer list from registry</td>
          </tr>
          <tr>
            <td><code>hpx register</code></td>
            <td>Request registration with the peer registry</td>
          </tr>
          <tr>
            <td><code>hpx statesync</code></td>
            <td>Enable state-sync for fast bootstrap</td>
          </tr>
          <tr>
            <td><code>hpx remove</code></td>
            <td>Uninstall the node service and optionally remove its data</td>
          </tr>
        </tbody>
      </table>

      <h3>Node Types</h3>

      <table>
        <thead>
          <tr>
            <th>Type</th>
            <th>Purpose</th>
            <th>Config</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>fullnode</code></td>
            <td>Non-validating full node (RPC, indexing)</td>
            <td><code>/config/fullnode/</code></td>
          </tr>
          <tr>
            <td><code>validator</code></td>
            <td>Validating node (must register validator key)</td>
            <td><code>/config/validator/</code></td>
          </tr>
        </tbody>
      </table>

      <h3>Public Registry</h3>

      <p>
        The HPX registry in <code>tools/hpx/registry/</code> serves mirror artifacts and peer discovery through these routes:
      </p>

      <table>
        <thead>
          <tr>
            <th>Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>GET /healthz</code></td>
            <td>Registry liveness, chain identity and software identity</td>
          </tr>
          <tr>
            <td><code>GET /checksums.txt</code></td>
            <td>SHA-256 checksums for all artifacts</td>
          </tr>
          <tr>
            <td><code>GET /chain-info.json</code></td>
            <td>Chain IDs, binary version and digest, seed peers and release metadata</td>
          </tr>
          <tr>
            <td><code>GET /paxd</code></td>
            <td>Native <code>paxd</code> binary</td>
          </tr>
          <tr>
            <td><code>GET /lib/libwasmvm.&lt;arch&gt;.so</code></td>
            <td>Declared <code>libwasmvm</code> runtime artifacts; versioned names are listed below</td>
          </tr>
          <tr>
            <td><code>GET /genesis.json</code></td>
            <td>Chain genesis file</td>
          </tr>
          <tr>
            <td><code>GET /config/&lt;type&gt;/&lt;file&gt;</code></td>
            <td>Node configuration files (<code>config.toml</code>, <code>app.toml</code>)</td>
          </tr>
          <tr>
            <td><code>GET /api/myip</code></td>
            <td>Caller's public IP address</td>
          </tr>
          <tr>
            <td><code>POST /api/register</code></td>
            <td>Register a peer with a fresh challenge and signed node-ownership proof</td>
          </tr>
          <tr>
            <td><code>GET /api/peers</code></td>
            <td>JSON list of registered peers</td>
          </tr>
          <tr>
            <td><code>GET /api/peers.txt</code></td>
            <td>Text list of peer addresses (Tendermint format)</td>
          </tr>
          <tr>
            <td><code>GET /api/nodes</code></td>
            <td>Detailed node metadata</td>
          </tr>
          <tr>
            <td><code>GET /api/statesync</code></td>
            <td>Current state-sync trust parameters</td>
          </tr>
        </tbody>
      </table>

      <h3>Artifact Publishing</h3>

      <p>
        To publish a new <code>paxd</code> release or chain configuration:
      </p>

      <pre><code>{`# From the monorepo root
sudo tools/hpx/publish.sh`}</code></pre>

      <p>
        This script:
      </p>

      <ol>
        <li>Collects <code>paxd</code> from <code>build/paxd</code> together with build provenance and the version manifest</li>
        <li>Collects native libraries from <code>wasm-runtime/</code> and <code>wasm/x/wasm/artifacts/</code></li>
        <li>Collects genesis and chain configuration from <code>/root/.paxeer/config/</code> (or <code>$SRC_CFG</code>); HPX_SEED_PEERS supplies at least three distinct seeds</li>
        <li>Stages artifacts in <code>/srv/hpx/artifacts/releases/&lt;release-id&gt;/</code></li>
        <li>Generates <code>checksums.txt</code></li>
        <li>Atomically moves <code>current</code> symlink to new release</li>
      </ol>

      <p>
        The publisher validates the binary identity and all required inputs before updating the served release.
      </p>

      <h3>Registry Runtime</h3>

      <p>
        The registry service under <code>tools/hpx/registry/</code> is distributed in these forms:
      </p>

      <ol>
        <li>Linux executables for x86-64 and AArch64</li>
        <li>GitHub release assets with SHA-256 checksum files</li>
        <li>A GHCR image for Linux amd64 and arm64</li>
      </ol>

      <p>
        After publishing artifacts and setting HPX_SEED_PEERS, HPX_STATESYNC_RPC and HPX_STATESYNC_RPC_SERVERS, deploy the registry:
      </p>

      <pre><code>{`sudo tools/hpx/hosting/deploy.sh`}</code></pre>

      <p>
        This script:
      </p>

      <ol>
        <li>Downloads and verifies the registry executable selected by HPX_SOURCE_REVISION or the registry source identity in Git</li>
        <li>Installs it as a systemd service (loopback-only)</li>
        <li>Obtains a TLS certificate for the origin configured in <code>tools/hpx/hosting/deploy.sh</code></li>
        <li>Configures Nginx reverse proxy with rate-limiting</li>
      </ol>

      <p>
        Registry state is persisted at <code>/srv/hpx/data/registry.json</code>.
      </p>

      <h3>Registry Authentication</h3>

      <p>
        Registration requires a fresh challenge and an Ed25519 node-ownership proof. An optional registration token adds another requirement:
      </p>

      <pre><code>{`# Set HPX_REGISTER_TOKEN in the protected deployment environment.
# Set HPX_TOKEN for the node manager when the registry requires it.
sudo tools/hpx/hosting/deploy.sh`}</code></pre>

      <p>
        When configured, the registry also requires an <code>X-HPX-Token</code> header on <code>POST /api/register</code>. Obtain a challenge with GET /api/register/challenge before signing the node registration.
      </p>

      <h3>Using an Alternate Mirror</h3>

      <p>
        Set <code>HPX_MIRROR</code> only when operating an explicitly trusted alternate mirror:
      </p>

      <pre><code>{`export HPX_MIRROR="<trusted HTTPS mirror origin>"
hpx setup`}</code></pre>

      <div className="source-note">
        <strong>Warning:</strong> The mirror must serve the same artifact structure and checksums. Untrusted mirrors can serve malicious binaries.
      </div>

      <h3>Native Libraries</h3>

      <p>
        HPX distributes architecture-specific <code>libwasmvm</code> runtimes:
      </p>

      <ul>
        <li><code>libwasmvm.x86_64.so</code> — x86-64 Linux</li>
        <li><code>libwasmvm.aarch64.so</code> — AArch64 Linux</li>
        <li><code>libwasmvm152.&lt;arch&gt;.so</code> — version 1.5.2 for x86_64 and aarch64</li>
        <li><code>libwasmvm155.&lt;arch&gt;.so</code> — version 1.5.5 for x86_64 and aarch64</li>
      </ul>

      <p>
        The node manager detects the host architecture and installs all three runtime variants for that architecture.
      </p>

      <h3>State Sync</h3>

      <p>
        New nodes can bootstrap from a recent state snapshot instead of replaying full history:
      </p>

      <pre><code>{`hpx statesync`}</code></pre>

      <p>
        This fetches trust parameters from <code>/api/statesync</code> and updates <code>config.toml</code> to enable state-sync.
      </p>

      <h3>Uninstall</h3>

      <pre><code>{`hpx remove`}</code></pre>

      <p>
        This stops <code>paxd</code> and removes the node service and peer-refresh units. A separate confirmation controls deletion of <code>/root/.paxeer/</code>, or the configured HPX_HOME.
      </p>

      <div className="source-note">
        <strong>Source:</strong> <code>tools/hpx/README.md</code>, <code>tools/hpx/hosting/</code>
      </div>

      <div className="prev-next">
        <Link href="/interchain">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Interchain</div>
        </Link>
        <div></div>
      </div>
    </DocsLayout>
  )
}
