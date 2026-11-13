import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/rest-grpc')

export default function RestGrpc() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">REST & gRPC APIs</h1>
        <p className="page-description">
          Cosmos SDK REST and gRPC interfaces for Paxeer chain modules and administration.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>api/</code> proto definitions, <code>node/app.go</code> route registration
      </div>

      <h2>Overview</h2>

      <p>
        Paxeer exposes Cosmos SDK-style gRPC and REST (gRPC-Gateway) endpoints for chain modules. These APIs complement the <Link href="/json-rpc">JSON-RPC interface</Link> and provide access to chain state, module parameters, and administrative operations.
      </p>

      <h3>Protocol Buffers</h3>

      <p>
        Chain-module and admin service definitions use Protocol Buffers v3 under <code>api/</code>. SDK, consensus and Wasm definitions also live in their own trees. Generation provides:
      </p>

      <ul>
        <li>Go types at each file’s go_package path, including modules, precompiles and admin types</li>
        <li>gRPC server and client interfaces</li>
        <li>REST endpoints via <code>google.api.http</code> annotations</li>
        <li>Swagger input from HTTP-annotated query definitions</li>
      </ul>

      <h3>Code Generation</h3>

      <p>
        Regenerate Go code from proto files, from the repository root:
      </p>

      <pre><code>{`./scripts/protoc.sh`}</code></pre>

      <p>
        The script builds <code>protoc-gen-gocosmos</code> from cosmos-proto v0.3.1 and runs <code>buf generate</code> through <code>buf@v1.58.0</code>. It uses root <code>buf.yaml</code> / <code>buf.gen.yaml</code> and the two consensus/internal templates, then copies generated Go files into their package directories. See <code>api/README.md</code>.
      </p>

      <h2>EVM Module Query Service</h2>

      <p>
        The EVM module provides address mapping, static calls, and pointer queries.
      </p>

      <h3>Address Mapping</h3>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>REST Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>PaxAddressByEVMAddress</code></td>
            <td><code>GET /pax-protocol/paxchain/evm/pax_address</code></td>
            <td>Get Cosmos address from EVM address</td>
          </tr>
          <tr>
            <td><code>EVMAddressByPaxAddress</code></td>
            <td><code>GET /pax-protocol/paxchain/evm/evm_address</code></td>
            <td>Get EVM address from Cosmos address</td>
          </tr>
        </tbody>
      </table>

      <h3>Static Calls</h3>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>REST Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>StaticCall</code></td>
            <td><code>GET /pax-protocol/paxchain/evm/static_call</code></td>
            <td>Execute read-only contract call</td>
          </tr>
        </tbody>
      </table>

      <h3>Pointer Queries</h3>

      <p>
        Pointer queries resolve native denominations and CW20, CW721, CW1155 or ERC token mappings according to the requested pointer type:
      </p>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>REST Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>Pointer</code></td>
            <td><code>GET /pax-protocol/paxchain/evm/pointer</code></td>
            <td>Get a registered pointer for the requested type and pointee</td>
          </tr>
          <tr>
            <td><code>Pointee</code></td>
            <td><code>GET /pax-protocol/paxchain/evm/pointee</code></td>
            <td>Get the registered pointee for the requested pointer type and address</td>
          </tr>
          <tr>
            <td><code>PointerVersion</code></td>
            <td><code>GET /pax-protocol/paxchain/evm/pointer_version</code></td>
            <td>Get current pointer contract version</td>
          </tr>
        </tbody>
      </table>

      <div className="source-note">
        <strong>Proto:</strong> <code>api/evm/query.proto</code>
      </div>

      <h2>Epoch Module Query Service</h2>

      <p>
        The Epoch module tracks the current chain epoch for time-based operations:
      </p>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>REST Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>Epoch</code></td>
            <td><code>GET /pax-protocol/paxchain/epoch/epoch</code></td>
            <td>Get current epoch number and metadata</td>
          </tr>
          <tr>
            <td><code>Params</code></td>
            <td><code>GET /pax-protocol/paxchain/epoch/params</code></td>
            <td>Get epoch module parameters</td>
          </tr>
        </tbody>
      </table>

      <div className="source-note">
        <strong>Proto:</strong> <code>api/epoch/query.proto</code>
      </div>

      <h2>Oracle Module Query Service</h2>

      <p>
        The Oracle module provides exchange rates, price feeds, and validator vote tracking:
      </p>

      <h3>Exchange Rates & Prices</h3>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>REST Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>ExchangeRate</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/denoms/{`{denom}`}/exchange_rate</code></td>
            <td>Get exchange rate for a denom</td>
          </tr>
          <tr>
            <td><code>ExchangeRates</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/denoms/exchange_rates</code></td>
            <td>Get all exchange rates</td>
          </tr>
          <tr>
            <td><code>Actives</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/denoms/actives</code></td>
            <td>List all active denoms</td>
          </tr>
          <tr>
            <td><code>VoteTargets</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/denoms/vote_targets</code></td>
            <td>List vote target denoms</td>
          </tr>
          <tr>
            <td><code>PriceSnapshotHistory</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/denoms/price_snapshot_history</code></td>
            <td>Get historical price snapshots</td>
          </tr>
          <tr>
            <td><code>Twaps</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/denoms/twaps/{`{lookback_seconds}`}</code></td>
            <td>Get time-weighted average prices</td>
          </tr>
        </tbody>
      </table>

      <h3>Validator Vote Tracking</h3>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>REST Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>FeederDelegation</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/validators/{`{validator_addr}`}/feeder</code></td>
            <td>Get feeder delegation for validator</td>
          </tr>
          <tr>
            <td><code>VotePenaltyCounter</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/validators/{`{validator_addr}`}/vote_penalty_counter</code></td>
            <td>Get oracle miss counter</td>
          </tr>
          <tr>
            <td><code>SlashWindow</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/slash_window</code></td>
            <td>Get slash window information</td>
          </tr>
          <tr>
            <td><code>Params</code></td>
            <td><code>GET /pax-protocol/pax-chain/oracle/params</code></td>
            <td>Get oracle module parameters</td>
          </tr>
        </tbody>
      </table>

      <div className="source-note">
        <strong>Proto:</strong> <code>api/oracle/query.proto</code>
      </div>

      <h2>TokenFactory Module Query Service</h2>

      <p>
        The TokenFactory module manages custom token creation and metadata:
      </p>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>REST Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>Params</code></td>
            <td><code>GET /pax-protocol/paxchain/tokenfactory/params</code></td>
            <td>Get tokenfactory parameters</td>
          </tr>
          <tr>
            <td><code>DenomAuthorityMetadata</code></td>
            <td><code>GET /pax-protocol/paxchain/tokenfactory/denoms/{`{denom}`}/authority_metadata</code> (additional binding: <code>GET /pax-protocol/paxchain/tokenfactory/denom_authority_metadata</code>)</td>
            <td>Get denom authority metadata</td>
          </tr>
          <tr>
            <td><code>DenomMetadata</code></td>
            <td><code>GET /pax-protocol/paxchain/tokenfactory/denoms/metadata</code></td>
            <td>Get denom metadata</td>
          </tr>
          <tr>
            <td><code>DenomsFromCreator</code></td>
            <td><code>GET /pax-protocol/paxchain/tokenfactory/denoms_from_creator/{`{creator}`}</code></td>
            <td>List denoms created by an address</td>
          </tr>
          <tr>
            <td><code>DenomAllowList</code></td>
            <td><code>GET /pax-protocol/paxchain/tokenfactory/denoms/allow_list</code></td>
            <td>Get denom allow list</td>
          </tr>
          <tr>
            <td><code>DenomPaused</code></td>
            <td><code>GET /pax-protocol/paxchain/tokenfactory/denoms/{`{denom}`}/paused</code> (additional binding: <code>GET /pax-protocol/paxchain/tokenfactory/denom_paused</code>)</td>
            <td>Query the pause status and issuer DID of a denom</td>
          </tr>
        </tbody>
      </table>

      <div className="source-note">
        <strong>Proto:</strong> <code>api/tokenfactory/query.proto</code>
      </div>

      <h2>Mint Module Query Service</h2>

      <p>
        The Mint module exposes minting parameters and state:
      </p>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>REST Endpoint</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>Params</code></td>
            <td><code>GET /paxchain/mint/v1beta1/params</code></td>
            <td>Get mint module parameters</td>
          </tr>
          <tr>
            <td><code>Minter</code></td>
            <td><code>GET /paxchain/mint/v1beta1/minter</code></td>
            <td>Get minter state (start/end dates, amounts)</td>
          </tr>
        </tbody>
      </table>

      <div className="source-note">
        <strong>Proto:</strong> <code>api/mint/v1beta1/query.proto</code>
      </div>

      <h2>Other Module Query Services</h2>

      <p>
        The <code>api/</code> tree also contains definitions for <code>launchpad</code>, <code>layerxagent</code>, <code>layerxpay</code>, <code>layerxprograms</code>, <code>pricefeed</code>, <code>xweb</code> and other chain modules. Read each service’s <code>query.proto</code> and its registration in <code>node/app.go</code> before selecting a method. HTTP bindings are declared per method; a gRPC method does not automatically have a REST route. This page details EVM, Epoch, Oracle, TokenFactory and Mint queries.
      </p>

      <h2>Admin gRPC Service</h2>

      <p>
        The Admin service provides runtime log level control. It runs on a separate loopback-only gRPC server (default port <code>9095</code>) and is disabled by default:
      </p>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>SetLogLevel</code></td>
            <td>Change log level for loggers matching a pattern</td>
          </tr>
          <tr>
            <td><code>GetLogLevel</code></td>
            <td>Get current log level for a logger</td>
          </tr>
          <tr>
            <td><code>ListLoggers</code></td>
            <td>List all registered loggers and their levels</td>
          </tr>
        </tbody>
      </table>

      <p>
        Enable in <code>app.toml</code>, replacing the address placeholder with a loopback IP and port:
      </p>

      <pre><code>{`[admin_server]
admin_enabled = true
admin_address = "<loopback-ip>:9095"`}</code></pre>

      <div className="source-note">
        <strong>Proto:</strong> <code>api/pax/admin/v0/admin.proto</code><br />
        <strong>Implementation:</strong> <code>admin/</code>
      </div>

      <h2>OpenAPI / Swagger Documentation</h2>

      <p>
        The Swagger script generates OpenAPI documentation with Ignite, then embeds prepared assets with statik. To regenerate from the repository root:
      </p>

      <pre><code>{`./scripts/update-swagger-ui-statik.sh`}</code></pre>

      <p>
        The script prepares <code>docs/swagger-ui/</code>, including <code>swagger.yml</code>, and writes the embedded assets to <code>docs/swagger/statik.go</code>. Use <code>./scripts/update-swagger-ui-statik.sh --assets-only</code> to embed existing assets without OpenAPI generation.
      </p>

      <h3>Serving Swagger UI</h3>

      <p>
        Enable in <code>app.toml</code>:
      </p>

      <pre><code>{`[api]
enable = true
swagger = true`}</code></pre>

      <p>
        The enabled API listener serves the embedded assets at <code>/swagger/</code>.
      </p>

      <div className="source-note">
        <strong>See:</strong> <code>scripts/update-swagger-ui-statik.sh</code> and <code>scripts/swagger_assets.py</code>
      </div>

      <h2>gRPC Endpoints</h2>

      <p>
        Enabled gRPC listeners use port <code>9090</code> by default. Set <code>GRPC_ADDRESS</code> to your listener’s host and port; it is an example client variable. For a plaintext listener, <code>grpcurl</code> can list services and request epoch parameters:
      </p>

      <pre><code>{`grpcurl -plaintext "$GRPC_ADDRESS" list
grpcurl -plaintext "$GRPC_ADDRESS" paxprotocol.paxchain.epoch.Query/Params`}</code></pre>

      <h2>REST Gateway</h2>

      <p>
        REST query routes are served via gRPC-Gateway on the API listener, whose default port is <code>1317</code>. The API is enabled by the full and archive initialization profiles. Only registered methods with HTTP bindings have the annotated REST paths.
      </p>

      <h2>Transaction Services</h2>

      <p>
        State-changing messages are declared in <code>Msg</code> services, including <code>api/evm/tx.proto</code> and <code>api/tokenfactory/tx.proto</code>. These differ from query methods. Encode and sign the appropriate transaction, then submit through the Cosmos SDK transaction service.
      </p>

      <div className="prev-next">
        <Link href="/json-rpc-unsupported">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Unsupported Methods</div>
        </Link>
        <Link href="/contracts">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Contracts</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
