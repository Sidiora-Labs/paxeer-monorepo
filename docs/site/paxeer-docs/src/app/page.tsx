import Link from 'next/link'
import { Icon } from '@/components/Icon'
import { PaxeerMark } from '@/components/PaxeerMark'
import { DeveloperQuickstart } from '@/components/DeveloperQuickstart'
import { ArtGrid, type DocArtName } from '@/components/DocArt'
import { jsonLd, metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/')

function SectionHead({ id, eyebrow, title }: { id: string; eyebrow: string; title: string }) {
  return <div className="cd-head"><p className="eyebrow">{eyebrow}</p><h2 id={id}>{title}</h2></div>
}

function Band() {
  return <div className="art-band" aria-hidden="true" />
}

function MediaCard({ art, alt, kicker, title, text, href, linkLabel, media = '', width = 1536, height = 1024 }: { art: DocArtName; alt: string; kicker: string; title: string; text: string; href: string; linkLabel: string; media?: string; width?: number; height?: number }) {
  return <Link href={href} className="cd-media-card">
    <span className={`cd-media${media ? ` ${media}` : ''}`}><img src={`/art/docs/${art}.webp`} alt={alt} width={width} height={height} loading="lazy" decoding="async" /></span>
    <span className="cd-media-card-body"><span className="cd-kicker">{kicker}</span><strong>{title}</strong><span>{text}</span><span className="cd-card-link">{linkLabel} <Icon name="arrow" size={15} /></span></span>
  </Link>
}

export default function Home() {
  return <main id="main-content" className="landing">
    <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLd('/') }} />
    <div className="landing-col">
      <section className="home-hero">
        <p className="eyebrow">PAXEER X / NETWORK DOCUMENTATION</p>
        <h1>Applications. Payments. Programs.<br /><span>Build on Paxeer.</span></h1>
        <p className="page-description">Paxeer mainnet-beta runs EVM applications and the embedded LayerX kernel on one BFT network, with native payments, markets and deterministic programs.</p>
        <div className="hero-actions"><Link className="pill-button primary-button" href="/unified-network">Explore the network <Icon name="arrow" size={16} /></Link><Link className="pill-button" href="/xdk">Start with the XDK</Link></div>
        <div className="hero-network"><span>One BFT network</span><i /><span>EVM chain ID <code>125</code></span><i /><span>Native token <code>PAX</code></span></div>
      </section>
      <Band />
      <figure className="cd-hero-panel">
        <img src="/art/docs/system-overview.webp" alt="Paxeer Docs: explore the connected system" width={1536} height={1024} decoding="async" />
        <figcaption><span>Paxeer mainnet-beta: EVM applications and the LayerX kernel execute on one BFT network.</span><span>EVM 125 · PAX</span></figcaption>
      </figure>
      <Band />
      <section className="cd-strip" aria-label="Network interfaces">
        <p className="eyebrow">NETWORK INTERFACES</p>
        <div className="cd-strip-grid">
          <Link href="/xdk"><strong>XDK</strong><span>Developer toolkit</span></Link>
          <Link href="/sdk-quickstarts"><strong>7 SDKs</strong><span>Language quickstarts</span></Link>
          <Link href="/mcp"><strong>MCP</strong><span>Scoped daemon tools</span></Link>
          <Link href="/json-rpc"><strong>JSON-RPC</strong><span>EVM chain 125</span></Link>
          <Link href="/cli"><strong>CLI</strong><span>Command line</span></Link>
        </div>
      </section>
      <Band />
      <section aria-labelledby="problem-title">
        <SectionHead id="problem-title" eyebrow="INTEGRATION" title={"Identity, payments\nand execution"} />
        <div className="cd-grid-3">
          <MediaCard art="authentication" alt="Paxeer X authentication card with a key and a verified shield" kicker="IDENTITY" title="Bind an account" text="Bind an EVM address to a LayerX identity and configure the authority used for native activities." href="/identity" linkLabel="Bind the identities" />
          <MediaCard art="request-envelope" alt="Request envelope carrying intent, context and payload" kicker="PAYMENTS" title="Authorize a payment" text="Declare the asset, amount, recipient and spending authority, then verify the resulting payment evidence." href="/payments" linkLabel="Attach the evidence" />
          <MediaCard art="debugging" alt="Debugging: a magnifying glass over terminal output" kicker="EXECUTION" title="Inspect a result" text="Read receipts and committed state to check the outcome of a submitted transaction." href="/proofs" linkLabel="Verify the result" />
        </div>
      </section>
      <section id="solution" aria-labelledby="solution-title">
        <SectionHead id="solution-title" eyebrow="APPLICATIONS" title={"Markets, assets\nand integrations"} />
        <div className="cd-grid-3">
          <MediaCard art="market-growth" alt="Portfolio growth dashboard with a weekly bar chart for BTC, ETH, SOL and PAX, framed by painted clouds" kicker="MARKETS" title="Perpetual markets" text="Read positions, margin, funding and oracle observations used by the perpetual markets module." href="/perpetuals" linkLabel="Perpetual markets" media="cd-media--panel" width={1374} height={1145} />
          <MediaCard art="asset-allocation" alt="Portfolio allocation card showing Bitcoin, Ethereum, Solana and PAX shares, framed by painted clouds" kicker="ASSETS" title="Native and EVM assets" text="Work with native assets, tokenfactory denominations and ERC20 pointers." href="/assets" linkLabel="Issue and manage assets" media="cd-media--panel" width={1374} height={1145} />
          <MediaCard art="activity-notifications" alt="Notification panel with a 2FA prompt, a completed PAX to USDC swap and trading rewards, framed by painted clouds" kicker="NOTIFICATIONS" title="Webhook delivery" text="Receive webhook events and verify signatures before processing their payloads." href="/webhooks" linkLabel="Set up webhooks" media="cd-media--panel" width={1374} height={1145} />
        </div>
        <div className="cd-grid-2">
          <MediaCard art="market-prices" alt="Wide market price list for Bitcoin, Ethereum, Paxeer PAX, XRP, Tether and Tron, framed by painted clouds" kicker="SPOT" title="Multi-asset spot markets" text="Place and cancel orders, manage collateral and follow settlement through receipts and committed state." href="/spot-exchange" linkLabel="Spot exchange" media="cd-media--wide" width={1774} height={887} />
          <MediaCard art="wallet-connect" alt="Wide wallet connector panel listing MetaMask, Binance, Coinbase, WalletConnect and Phantom, framed by painted clouds" kicker="WALLETS" title="Connect an EVM wallet" text="Connect wallets and call the network from your frontend over standard EVM JSON-RPC on chain 125." href="/building-a-frontend" linkLabel="Build a frontend" media="cd-media--wide" width={1774} height={887} />
        </div>
      </section>
      <ArtGrid id="start-title" eyebrow="START BUILDING" title={"From install to first request.\nIn a few steps."} rows={[
        [
          { art: 'quickstart', title: 'SDK quickstarts', text: 'Register, fund, send and verify with seven language SDKs.', href: '/sdk-quickstarts' },
          { art: 'installation', title: 'Installation & build', text: 'Build the node with its embedded kernel, hosted services and contracts from source.', href: '/installation' },
          { art: 'configuration', title: 'Configuration', text: 'Node modes, RPC, storage, snapshots and metrics in one place.', href: '/configuration' },
        ],
        [
          { art: 'authentication', title: 'Authentication and signing', text: 'Scoped credentials, LayerX keys and signed requests for the hosted platform API.', href: '/platform-api' },
          { art: 'chain-configuration', title: 'Network parameters', text: 'Chain identifiers, denominations, gas limits and endpoints for EVM 125.', href: '/network-parameters' },
        ],
      ]} />
      <Band />
      <section id="benefits" aria-labelledby="benefits-title">
        <SectionHead id="benefits-title" eyebrow="EXECUTION" title={"Authority, receipts\nand committed state"} />
        <div className="cd-grid-3">
          <Link href="/layerx-kernel" className="cd-benefit"><span className="cd-benefit-visual"><Icon name="node" size={44} /></span><span className="cd-media-card-body"><strong>Execution you can reproduce</strong><span>The node orders native activities and commits their state with BFT blocks. Deterministic execution supports replay with the required state and artifacts.</span></span></Link>
          <Link href="/identity" className="cd-benefit"><span className="cd-benefit-visual"><Icon name="layers" size={44} /></span><span className="cd-media-card-body"><strong>Authority with clear boundaries</strong><span>Identity keys, scoped capabilities, delegated spending limits and revocation constrain authorized actions.</span></span></Link>
          <Link href="/evm" className="cd-benefit"><span className="cd-benefit-visual"><Icon name="code" size={44} /></span><span className="cd-media-card-body"><strong>Tools you already know</strong><span>Deploy Solidity, connect wallets and call native precompiles through standard EVM tooling and JSON-RPC on chain 125.</span></span></Link>
        </div>
      </section>
      <Band />
      <section id="comparison" aria-labelledby="comparison-title">
        <SectionHead id="comparison-title" eyebrow="INTEGRATION MAP" title="Application needs and network features" />
        <div className="cd-compare">
          <div className="cd-compare-col">
            <p className="cd-compare-head cd-compare-head--other">APPLICATION NEEDS</p>
            <ul>
              <li>Reconcile payment outcomes</li>
              <li>Identify an authorized actor</li>
              <li>Link payment and delivery</li>
              <li>Reproduce program execution</li>
              <li>Check settlement evidence</li>
              <li>Connect agent tools</li>
            </ul>
          </div>
          <div className="cd-compare-col">
            <p className="cd-compare-head cd-compare-head--paxeer">PAXEER X</p>
            <ul>
              <li>Native transfer settlement (402LXP)</li>
              <li>EVM address bound to a LayerX DID</li>
              <li>Escrow, acceptance and receipts linked</li>
              <li>Deterministic, replayable transitions</li>
              <li>Signed receipts and committed blocks</li>
              <li>Scoped MCP tools and agent budgets</li>
            </ul>
          </div>
        </div>
      </section>
      <ArtGrid id="agents-art-title" eyebrow="AGENTS" title={"Give agents work.\nKeep the authority explicit."} rows={[
        [
          { art: 'agent-lifecycle', title: 'Agent lifecycle', text: 'Read state, prepare, disclose, sign and track every action.', href: '/agents' },
          { art: 'task-queue', title: 'Agent-to-agent tasks', text: 'Send structured tasks and follow their state between agents.', href: '/a2a' },
          { art: 'tool-discovery', title: 'MCP tools', text: 'Daemon tools for state reads, authorized writes, sessions and paid web requests.', href: '/mcp' },
        ],
        [
          { art: 'intent-builder', title: 'Human interface', text: 'Turn user intent into typed economic actions with visible limits.', href: '/human-interface' },
          { art: 'prompt-templates', title: 'Agent runtime', text: 'Enrol the daemon, admit actions and operate the runtime connection.', href: '/agent-runtime' },
        ],
      ]} />
      <ArtGrid id="payments-art-title" eyebrow="PAYMENTS" title={"Payments and\nexecution evidence."} rows={[
        [
          { art: 'request-envelope', title: 'Activities', text: 'The exact envelope an activity is admitted, executed and recovered with.', href: '/activities' },
          { art: 'service-payment', title: 'Paid APIs and x402', text: 'Charge for HTTP services and settle through 402LXP.', href: '/paid-apis' },
          { art: 'payment-receipt', title: 'Receipts & proofs', text: 'Verify signed receipts, payment evidence and settlement.', href: '/receipts' },
        ],
      ]} />
      <ArtGrid id="programs-art-title" eyebrow="PROGRAMS AND SERVICES" title={"Run the workload.\nVerify the result."} rows={[
        [
          { art: 'workload-inputs', title: 'Bounded sandboxes', text: 'Lease execution with explicit inputs, limits and closure.', href: '/sandbox' },
          { art: 'job-progress', title: 'Services and agreements', text: 'From offer to invocation, delivery and review.', href: '/services' },
          { art: 'results-artifacts', title: 'Deterministic programs', text: 'Deploy, call and verify program outputs.', href: '/programs' },
        ],
        [
          { art: 'model-registry', title: 'Program registry', text: 'Deployed artifacts, interfaces and receipt-bound program records.', href: '/registry' },
          { art: 'resource-browser', title: 'Program workspace', text: 'Runtime crates, guest SDKs, reference applications and porting kits.', href: '/program-workspace' },
        ],
      ]} />
      <ArtGrid id="operate-art-title" eyebrow="OBSERVE AND OPERATE" title={"Read state.\nCheck the evidence."} rows={[
        [
          { art: 'state-snapshots', title: 'Checkpoints', text: 'Block commitments, trusted state sync and receipt finality on Paxeer.', href: '/checkpoints' },
          { art: 'event-subscriptions', title: 'Event subscriptions', text: 'Scoped subscriptions with acknowledgement and gap recovery.', href: '/subscriptions' },
          { art: 'debugging', title: 'Debug transactions', text: 'Collect the transaction and receipt, then check the call.', href: '/debugging-transactions' },
        ],
        [
          { art: 'system-overview', title: 'Unified network', text: 'How the EVM chain and LayerX kernel form one system.', href: '/unified-network' },
          { art: 'reference-guide', title: 'LayerX kernel', text: 'Sequencing, accounting, modules and deterministic execution.', href: '/layerx-kernel' },
        ],
      ]} />
      <Band />
      <section id="how-it-works" aria-labelledby="how-title">
        <SectionHead id="how-title" eyebrow="HOW IT WORKS" title={"Connect, authorize\nand verify"} />
        <div className="cd-grid-3">
          <MediaCard art="installation" alt="Installation card: a download arrow above a terminal showing install and Ready" kicker="STEP 1" title="Install and connect" text="Choose an SDK and configure the node or hosted service endpoint required by the operation." href="/sdk-quickstarts" linkLabel="SDK quickstarts" />
          <MediaCard art="service-payment" alt="Service payment: an application pays through 402LXP and receives a receipt" kicker="STEP 2" title="Fund and authorize" text="Fund the account, bind identities and set the budgets and mandates that bound what can be spent." href="/payments-quickstart" linkLabel="Make your first payment" />
          <MediaCard art="payment-receipt" alt="Payment receipt: protocol 402LXP, service Compute, status Settled" kicker="STEP 3" title="Execute and verify" text="Submit a signed transaction, verify its execution receipt and confirm its covering block is committed." href="/receipts" linkLabel="Verify a receipt" />
        </div>
        <div className="cd-center-cta"><Link className="pill-button primary-button" href="/xdk">Start with the XDK <Icon name="arrow" size={16} /></Link></div>
        <div className="cd-inset"><DeveloperQuickstart /></div>
      </section>
      <Band />
      <section id="paths" aria-labelledby="paths-title">
        <SectionHead id="paths-title" eyebrow="BUILD PATHS" title={"Pick a path.\nGo deeper when ready."} />
        <div className="cd-grid-3 cd-paths">
          <article className="cd-path">
            <p className="cd-path-head">AGENTS</p>
            <div className="cd-path-body">
              <h3>Configure agent authority.</h3>
              <p>Create identities, delegate scoped authority, fund accounts and connect tools through SDKs, MCP and the agent daemon.</p>
              <p className="cd-path-includes">This path covers:</p>
              <ul>
                <li>Agent identities and DIDs</li>
                <li>Budgets, mandates and revocation</li>
                <li>Scoped MCP tools</li>
                <li>Agent-to-agent tasks</li>
              </ul>
              <Link href="/agents" className="pill-button cd-path-cta">Build an agent <Icon name="arrow" size={14} /></Link>
            </div>
          </article>
          <article className="cd-path">
            <p className="cd-path-head cd-path-head--hot">PAYMENTS <span>START HERE</span></p>
            <div className="cd-path-body">
              <h3>Integrate native payments.</h3>
              <p>Integrate transfers, escrow, budgets and payment streams. Connect service delivery to acceptance, receipts and disputes.</p>
              <p className="cd-path-includes">This path covers:</p>
              <ul>
                <li>Transfers, holds and escrow</li>
                <li>Paid APIs and x402 / 402LXP</li>
                <li>Payment streams and budgets</li>
                <li>Receipts and settlement proof</li>
              </ul>
              <Link href="/payments-quickstart" className="pill-button primary-button cd-path-cta">Make your first payment <Icon name="arrow" size={14} /></Link>
            </div>
          </article>
          <article className="cd-path">
            <p className="cd-path-head">EVM APPLICATIONS</p>
            <div className="cd-path-body">
              <h3>Use the tools you know.</h3>
              <p>Deploy Solidity, connect wallets and call native precompiles for account binding, native payments, exchange and asset launches.</p>
              <p className="cd-path-includes">This path covers:</p>
              <ul>
                <li>Solidity with Foundry and Hardhat</li>
                <li>JSON-RPC on chain 125</li>
                <li>Native precompiles</li>
                <li>Assets, launchpad and markets</li>
              </ul>
              <Link href="/evm" className="pill-button cd-path-cta">Build on the EVM <Icon name="arrow" size={14} /></Link>
            </div>
          </article>
        </div>
      </section>
      <Band />
      <section aria-labelledby="evidence-title">
        <SectionHead id="evidence-title" eyebrow="EVIDENCE" title={"Receipts, blocks\nand execution records"} />
        <div className="cd-grid-3">
          <MediaCard art="payment-receipt" alt="Payment receipt: protocol 402LXP, service Compute, status Settled" kicker="RECEIPTS" title="Signed at execution" text="Verify a signed execution receipt against trusted receipt keys, then check its covering block." href="/receipts" linkLabel="Verify signed receipts" />
          <MediaCard art="state-snapshots" alt="Stacked state snapshots" kicker="BLOCKS" title="Committed on Paxeer" text="Validators commit blocks on the BFT network. A receipt is final when its covering block is committed." href="/checkpoints" linkLabel="Verify block commitment" />
          <MediaCard art="job-progress" alt="Job progress: submitted, processing, output ready" kicker="COMMITMENT" title="Submitted, receipted, committed" text="Track submission, verify the signed execution result and confirm block commitment before treating it as final." href="/finality" linkLabel="Read commitment levels" />
          <MediaCard art="results-artifacts" alt="Results and artifacts: output, logs and metadata ready to download" kicker="REPLAY" title="Replayable history" text="Replay deterministic execution with the required input history, state, runtime version and program artifacts." href="/sequencing" linkLabel="How sequencing works" />
          <MediaCard art="event-subscriptions" alt="Event subscriptions for task, payment and chain events" kicker="ACCOUNTS" title="Fund a bound account" text="Bind the owner identity, transfer native assets and verify the recipient, amount and transaction receipt." href="/custody" linkLabel="Account funding and keys" />
          <MediaCard art="debugging" alt="Debugging: a magnifying glass over terminal output" kicker="INSPECTION" title="Debug from evidence" text="Collect the transaction and its receipt, then check the call against committed state instead of guessing." href="/debugging-transactions" linkLabel="Debug a transaction" />
        </div>
      </section>
      <Band />
      <section className="reference-section cd-inset" aria-labelledby="reference-title"><div className="section-heading"><div><p className="eyebrow">BUILD WITH THE FULL SYSTEM</p><h2 id="reference-title">Find your interface.<br />Follow it deeper.</h2></div><p>Architecture, integration guides and operational reference for Paxeer X.</p></div><div className="reference-grid">
        <Link href="/layerx-kernel" className="reference-card"><h3>LayerX kernel</h3><p>Canonical sequencing, accounting, modules and deterministic execution.</p><Icon name="arrow" size={16} /></Link>
        <Link href="/platform-api" className="reference-card"><h3>Platform APIs</h3><p>Application gateways, hosted services, webhooks and integration surfaces.</p><Icon name="arrow" size={16} /></Link>
        <Link href="/json-rpc" className="reference-card"><h3>EVM JSON-RPC</h3><p>Wallet integration, chain requests, contracts and transaction inspection.</p><Icon name="arrow" size={16} /></Link>
        <Link href="/operators" className="reference-card"><h3>Operator guides</h3><p>Nodes, configuration, network services and settlement infrastructure.</p><Icon name="arrow" size={16} /></Link>
      </div></section>
      <Band />
      <section id="faqs" aria-labelledby="faq-title">
        <SectionHead id="faq-title" eyebrow="FAQS" title="Frequently asked questions" />
        <div className="cd-faq">
          <details><summary>What is the difference between Paxeer X and LayerX?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Paxeer mainnet-beta is one BFT network. LayerX is its embedded kernel for native activities and deterministic execution. EVM transactions call native precompiles, and receipt finality comes from the covering block commit. <Link href="/paxeer-vs-layerx">Explore the network architecture</Link></p></details>
          <details><summary>Which chain ID and token does the network use?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>EVM chain ID <code>125</code> (<code>hyperpax_125-1</code>) with native token <code>PAX</code>. Denominations, gas limits and endpoints are listed with the network parameters. <Link href="/network-parameters">Network parameters</Link></p></details>
          <details><summary>How does an agent get authority to spend?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Through scoped, revocable delegation: primary and session keys, budget limits and mandates define exactly what an agent may do with money, and owners can revoke or rotate that authority. <Link href="/agent-budgets">Agent budgets</Link></p></details>
          <details><summary>How do I verify a payment actually settled?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Verify the signed execution receipt against trusted receipt keys, check its transaction and outcome, and confirm the covering BFT block is committed. <Link href="/receipts">Receipts and proofs</Link></p></details>
          <details><summary>Can I use my existing EVM tooling?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Yes. Deploy Solidity with Foundry or Hardhat, connect wallets over JSON-RPC, and call native precompiles for account binding, native payments and markets on chain 125. <Link href="/evm">Build on the EVM</Link></p></details>
          <details><summary>How do I run a node or operate the network?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Start with installation and configuration, then the operator guides for BFT nodes, validators, RPC endpoints and retained chain history. <Link href="/run-node">Run a node</Link></p></details>
        </div>
      </section>
      <Band />
      <section className="cd-cta">
        <img className="cd-cta-bg" src="/art/docs/system-overview.webp" alt="" aria-hidden="true" width={1536} height={1024} loading="lazy" decoding="async" />
        <div className="cd-cta-inner"><PaxeerMark /><h2>Build an application.<br />Connect to Paxeer.</h2><p>Choose a domain, integrate an interface and follow the evidence through the network.</p><div className="hero-actions"><Link href="/xdk" className="pill-button primary-button">Open the developer toolkit <Icon name="arrow" size={16} /></Link><Link href="/unified-network" className="pill-button cd-cta-ghost">Explore the network</Link></div></div>
      </section>
    </div>
    <footer className="site-footer"><div className="landing-col cd-footer-col"><div className="cd-footer-top"><div className="footer-intro"><Link href="/" className="brand"><PaxeerMark />Paxeer X Docs</Link><p>The chain, agent economy and developer platform in one reference.</p></div><div className="footer-links"><div><span>BUILD</span><Link href="/xdk">Developer toolkit</Link><Link href="/agents">Agents</Link><Link href="/programs">Programs</Link></div><div><span>USE</span><Link href="/payments-quickstart">Payments</Link><Link href="/spot-exchange">Markets</Link><Link href="/human-interface">Human interface</Link></div><div><span>UNDERSTAND</span><Link href="/unified-network">Unified network</Link><Link href="/proofs">Proofs</Link><Link href="/operators">Operations</Link></div></div></div><nav className="cd-footer-quick" aria-label="Landing sections"><span>ON THIS PAGE:</span><a href="#solution">Applications</a><a href="#benefits">Execution</a><a href="#comparison">Integration map</a><a href="#how-it-works">How it works</a><a href="#paths">Build paths</a><a href="#faqs">FAQs</a></nav><div className="footer-bottom"><span>Paxeer X Network</span><span>EVM 125 · hyperpax_125-1 · LayerX</span></div></div></footer>
  </main>
}
