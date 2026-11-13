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
        <h1>Money. Markets. Machines.<br /><span>One network to build on.</span></h1>
        <p className="page-description">Paxeer X unites EVM applications, the LayerX agent economy, payments, markets and deterministic programs with verifiable execution and settlement.</p>
        <div className="hero-actions"><Link className="pill-button primary-button" href="/unified-network">Explore the network <Icon name="arrow" size={16} /></Link><Link className="pill-button" href="/xdk">Start with the XDK</Link></div>
        <div className="hero-network"><span>Two execution domains</span><i /><span>EVM chain ID <code>125</code></span><i /><span>Native token <code>PAX</code></span></div>
      </section>
      <Band />
      <figure className="cd-hero-panel">
        <img src="/art/docs/system-overview.webp" alt="Paxeer Docs: explore the connected system" width={1536} height={1024} decoding="async" />
        <figcaption><span>Paxeer X chain and LayerX kernel: connected through identity, evidence and settlement.</span><span>EVM 125 · PAX</span></figcaption>
      </figure>
      <Band />
      <section className="cd-strip" aria-label="Network interfaces">
        <p className="eyebrow">ONE NETWORK — EVERY INTERFACE</p>
        <div className="cd-strip-grid">
          <Link href="/xdk"><strong>XDK</strong><span>Developer toolkit</span></Link>
          <Link href="/sdk-quickstarts"><strong>7 SDKs</strong><span>Language quickstarts</span></Link>
          <Link href="/mcp"><strong>MCP</strong><span>21 scoped tools</span></Link>
          <Link href="/json-rpc"><strong>JSON-RPC</strong><span>EVM chain 125</span></Link>
          <Link href="/cli"><strong>CLI</strong><span>Command line</span></Link>
        </div>
      </section>
      <Band />
      <section aria-labelledby="problem-title">
        <SectionHead id="problem-title" eyebrow="PROBLEM" title={"Building across fragmented\nrails is messy"} />
        <div className="cd-grid-3">
          <MediaCard art="authentication" alt="Paxeer X authentication card with a key and a verified shield" kicker="PROBLEM 1" title="Identity scattered per app" text="A chain address here, an agent account there. Nothing binds them, so authority has to be rebuilt for every integration." href="/identity" linkLabel="Bind the identities" />
          <MediaCard art="request-envelope" alt="Request envelope carrying intent, context and payload" kicker="PROBLEM 2" title="Payments without evidence" text="Value moves in one system while authorization, delivery and proof live in others — if they exist at all." href="/payments" linkLabel="Attach the evidence" />
          <MediaCard art="debugging" alt="Debugging: a magnifying glass over terminal output" kicker="PROBLEM 3" title="Zero execution visibility" text="Results with no canonical history cannot be replayed, audited or independently checked after the fact." href="/proofs" linkLabel="Verify the result" />
        </div>
      </section>
      <section id="solution" aria-labelledby="solution-title">
        <SectionHead id="solution-title" eyebrow="SOLUTION" title={"One network. Every domain.\nTotal clarity."} />
        <div className="cd-grid-3">
          <MediaCard art="market-growth" alt="Portfolio growth dashboard with a weekly bar chart for BTC, ETH, SOL and PAX, framed by painted clouds" kicker="MARKETS" title="Market performance" text="Track growth across positions with margin, funding and oracle inputs as explicit protocol concepts." href="/perpetuals" linkLabel="Perpetual markets" media="cd-media--panel" width={1374} height={1145} />
          <MediaCard art="asset-allocation" alt="Portfolio allocation card showing Bitcoin, Ethereum, Solana and PAX shares, framed by painted clouds" kicker="ASSETS" title="Asset allocation" text="Native assets, tokenfactory denoms and ERC20 pointers — see exactly where value sits across both domains." href="/assets" linkLabel="Issue and manage assets" media="cd-media--panel" width={1374} height={1145} />
          <MediaCard art="activity-notifications" alt="Notification panel with a 2FA prompt, a completed PAX to USDC swap and trading rewards, framed by painted clouds" kicker="NOTIFICATIONS" title="Activity notifications" text="Swaps, rewards and security events pushed to your application the moment they settle." href="/webhooks" linkLabel="Set up webhooks" media="cd-media--panel" width={1374} height={1145} />
        </div>
        <div className="cd-grid-2">
          <MediaCard art="market-prices" alt="Wide market price list for Bitcoin, Ethereum, Paxeer PAX, XRP, Tether and Tron, framed by painted clouds" kicker="SPOT" title="Multi-asset spot markets" text="Place and cancel orders, manage collateral and follow settlement through receipts and committed state." href="/spot-exchange" linkLabel="Spot exchange" media="cd-media--wide" width={1774} height={887} />
          <MediaCard art="wallet-connect" alt="Wide wallet connector panel listing MetaMask, Binance, Coinbase, WalletConnect and Phantom, framed by painted clouds" kicker="WALLETS" title="Connect any wallet" text="Connect wallets and call the network from your frontend over standard EVM JSON-RPC on chain 125." href="/building-a-frontend" linkLabel="Build a frontend" media="cd-media--wide" width={1774} height={887} />
        </div>
      </section>
      <ArtGrid id="start-title" eyebrow="START BUILDING" title={"From install to first request.\nIn a few steps."} rows={[
        [
          { art: 'quickstart', title: 'SDK quickstarts', text: 'Register, fund, send and verify with seven language SDKs.', href: '/sdk-quickstarts' },
          { art: 'installation', title: 'Installation & build', text: 'Build the chain node, kernel services and contracts from source.', href: '/installation' },
          { art: 'configuration', title: 'Configuration', text: 'Node modes, RPC, storage, snapshots and metrics in one place.', href: '/configuration' },
        ],
        [
          { art: 'authentication', title: 'Authentication and signing', text: 'Bearer [REDACTED], LayerX keys and signed canonical requests for the platform API.', href: '/platform-api' },
          { art: 'chain-configuration', title: 'Network parameters', text: 'Chain identifiers, denominations, gas limits and endpoints for EVM 125.', href: '/network-parameters' },
        ],
      ]} />
      <Band />
      <section id="benefits" aria-labelledby="benefits-title">
        <SectionHead id="benefits-title" eyebrow="BENEFITS" title={"The guarantees that\nmatter most to builders"} />
        <div className="cd-grid-3">
          <Link href="/layerx-kernel" className="cd-benefit"><span className="cd-benefit-visual"><Icon name="node" size={44} /></span><span className="cd-media-card-body"><strong>Execution you can reproduce</strong><span>Global sequencing and chained state roots create a canonical history. Deterministic execution makes independent replay and verification possible.</span></span></Link>
          <Link href="/identity" className="cd-benefit"><span className="cd-benefit-visual"><Icon name="layers" size={44} /></span><span className="cd-media-card-body"><strong>Authority with clear boundaries</strong><span>Primary and session keys, scoped capabilities, delegated limits, revocation, rotation and recovery for every autonomous actor.</span></span></Link>
          <Link href="/evm" className="cd-benefit"><span className="cd-benefit-visual"><Icon name="code" size={44} /></span><span className="cd-media-card-body"><strong>Tools you already know</strong><span>Deploy Solidity, connect wallets and call native precompiles through standard EVM tooling and JSON-RPC on chain 125.</span></span></Link>
        </div>
      </section>
      <Band />
      <section id="comparison" aria-labelledby="comparison-title">
        <SectionHead id="comparison-title" eyebrow="COMPARISON" title="Fragmented stack vs Paxeer X" />
        <div className="cd-compare">
          <div className="cd-compare-col">
            <p className="cd-compare-head cd-compare-head--other">FRAGMENTED STACK</p>
            <ul>
              <li>Manual reconciliation per app</li>
              <li>Identity siloed per domain</li>
              <li>Payments detached from delivery</li>
              <li>Opaque, unrepeatable execution</li>
              <li>Trust the operator&rsquo;s word</li>
              <li>Custom glue for every agent</li>
            </ul>
          </div>
          <div className="cd-compare-col">
            <p className="cd-compare-head cd-compare-head--paxeer">PAXEER X</p>
            <ul>
              <li>One accounting doorway (402LXP)</li>
              <li>EVM address bound to a LayerX DID</li>
              <li>Escrow, acceptance and receipts linked</li>
              <li>Deterministic, replayable transitions</li>
              <li>Signed receipts, batches and checkpoints</li>
              <li>Scoped MCP tools and agent budgets</li>
            </ul>
          </div>
        </div>
      </section>
      <ArtGrid id="agents-art-title" eyebrow="AGENTS" title={"Give agents work.\nKeep the authority explicit."} rows={[
        [
          { art: 'agent-lifecycle', title: 'Agent lifecycle', text: 'Read state, prepare, disclose, sign and track every action.', href: '/agents' },
          { art: 'task-queue', title: 'Agent-to-agent tasks', text: 'Send structured tasks and follow their state between agents.', href: '/a2a' },
          { art: 'tool-discovery', title: 'MCP tools', text: '21 scoped tools for verified reads and policy-governed writes.', href: '/mcp' },
        ],
        [
          { art: 'intent-builder', title: 'Human interface', text: 'Turn user intent into typed economic actions with visible limits.', href: '/human-interface' },
          { art: 'prompt-templates', title: 'Agent runtime', text: 'Enrol the daemon, admit actions and operate the runtime connection.', href: '/agent-runtime' },
        ],
      ]} />
      <ArtGrid id="payments-art-title" eyebrow="PAYMENTS" title={"Every payment carries\nits own evidence."} rows={[
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
          { art: 'model-registry', title: 'Program registry', text: 'A verified record of deployed programs and their source.', href: '/registry' },
          { art: 'resource-browser', title: 'Program workspace', text: 'Packages, guest applications and release evidence.', href: '/program-workspace' },
        ],
      ]} />
      <ArtGrid id="operate-art-title" eyebrow="OBSERVE AND OPERATE" title={"Know what happened.\nProve it."} rows={[
        [
          { art: 'state-snapshots', title: 'Checkpoints', text: 'Compact commitments to complete execution, settled on Paxeer X.', href: '/checkpoints' },
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
        <SectionHead id="how-title" eyebrow="HOW IT WORKS" title={"Connected in\n3 simple steps"} />
        <div className="cd-grid-3">
          <MediaCard art="installation" alt="Installation card: a download arrow above a terminal showing install and Ready" kicker="STEP 1" title="Install and connect" text="Pick one of seven SDKs, the XDK or JSON-RPC and point it at your node or a public endpoint." href="/sdk-quickstarts" linkLabel="SDK quickstarts" />
          <MediaCard art="service-payment" alt="Service payment: an application pays through 402LXP and receives a receipt" kicker="STEP 2" title="Fund and authorize" text="Fund the account, bind identities and set the budgets and mandates that bound what can be spent." href="/payments-quickstart" linkLabel="Make your first payment" />
          <MediaCard art="payment-receipt" alt="Payment receipt: protocol 402LXP, service Compute, status Settled" kicker="STEP 3" title="Execute and verify" text="Send the activity, then follow its signed receipt through batch sealing to checkpoint settlement." href="/receipts" linkLabel="Verify a receipt" />
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
              <h3>Give agents economic agency.</h3>
              <p>Create identities, delegate scoped authority, fund accounts and connect tools through SDKs, MCP and the agent daemon.</p>
              <p className="cd-path-includes">This path covers:</p>
              <ul>
                <li>Agent identities and DIDs</li>
                <li>Budgets, mandates and revocation</li>
                <li>21 scoped MCP tools</li>
                <li>Agent-to-agent tasks</li>
              </ul>
              <Link href="/agents" className="pill-button cd-path-cta">Build an agent <Icon name="arrow" size={14} /></Link>
            </div>
          </article>
          <article className="cd-path">
            <p className="cd-path-head cd-path-head--hot">PAYMENTS <span>START HERE</span></p>
            <div className="cd-path-body">
              <h3>Move value with purpose.</h3>
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
              <p>Deploy Solidity, connect wallets and call native precompiles for account binding, custody, checkpoints, exchange and asset launches.</p>
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
        <SectionHead id="evidence-title" eyebrow="EVIDENCE, NOT PROMISES" title={"Every claim carries\nits own proof"} />
        <div className="cd-grid-3">
          <MediaCard art="payment-receipt" alt="Payment receipt: protocol 402LXP, service Compute, status Settled" kicker="RECEIPTS" title="Signed at execution" text="Every activity returns a signed receipt you can verify against committed state — no operator statement required." href="/receipts" linkLabel="Verify signed receipts" />
          <MediaCard art="state-snapshots" alt="Stacked state snapshots" kicker="CHECKPOINTS" title="Sealed on Paxeer X" text="Batches of execution compact into checkpoints settled on the chain, with guarantor attestations behind them." href="/checkpoints" linkLabel="Follow a checkpoint" />
          <MediaCard art="job-progress" alt="Job progress: submitted, processing, output ready" kicker="COMMITMENT" title="Instant, sealed, final" text="Distinguish instant execution, sealed batches and final settlement, and match the level to the economic action." href="/finality" linkLabel="Read commitment levels" />
          <MediaCard art="results-artifacts" alt="Results and artifacts: output, logs and metadata ready to download" kicker="REPLAY" title="Replayable history" text="Global sequencing and chained state roots make the canonical history independently replayable by anyone." href="/sequencing" linkLabel="How sequencing works" />
          <MediaCard art="event-subscriptions" alt="Event subscriptions for task, payment and chain events" kicker="CUSTODY EXITS" title="A verifiable way out" text="Withdrawals and forced exits use verified receipt or state evidence, recipient authorization and nullifiers." href="/custody" linkLabel="Custody and exits" />
          <MediaCard art="debugging" alt="Debugging: a magnifying glass over terminal output" kicker="INSPECTION" title="Debug from evidence" text="Collect the transaction and its receipt, then check the call against committed state instead of guessing." href="/debugging-transactions" linkLabel="Debug a transaction" />
        </div>
      </section>
      <Band />
      <section className="reference-section cd-inset" aria-labelledby="reference-title"><div className="section-heading"><div><p className="eyebrow">BUILD WITH THE FULL SYSTEM</p><h2 id="reference-title">Find your interface.<br />Follow it deeper.</h2></div><p>Architecture, integration guides and operational reference for every part of Paxeer X.</p></div><div className="reference-grid">
        <Link href="/layerx-kernel" className="reference-card"><h3>LayerX kernel</h3><p>Canonical sequencing, accounting, modules and deterministic execution.</p><Icon name="arrow" size={16} /></Link>
        <Link href="/platform-api" className="reference-card"><h3>Platform APIs</h3><p>Application gateways, hosted services, webhooks and integration surfaces.</p><Icon name="arrow" size={16} /></Link>
        <Link href="/json-rpc" className="reference-card"><h3>EVM JSON-RPC</h3><p>Wallet integration, chain requests, contracts and transaction inspection.</p><Icon name="arrow" size={16} /></Link>
        <Link href="/operators" className="reference-card"><h3>Operator guides</h3><p>Nodes, configuration, network services and settlement infrastructure.</p><Icon name="arrow" size={16} /></Link>
      </div></section>
      <Band />
      <section id="faqs" aria-labelledby="faq-title">
        <SectionHead id="faq-title" eyebrow="FAQS" title="Frequently asked questions" />
        <div className="cd-faq">
          <details><summary>What is the difference between Paxeer X and LayerX?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Paxeer X is the EVM chain supplying execution, custody and settlement. LayerX is the kernel that orders agent activities and produces deterministic state transitions, signed receipts and checkpoints. Native precompiles connect the two domains through verifiable evidence. <Link href="/paxeer-vs-layerx">Compare the domains</Link></p></details>
          <details><summary>Which chain ID and token does the network use?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>EVM chain ID <code>125</code> (<code>hyperpax_125-1</code>) with native token <code>PAX</code>. Denominations, gas limits and endpoints are listed with the network parameters. <Link href="/network-parameters">Network parameters</Link></p></details>
          <details><summary>How does an agent get authority to spend?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Through scoped, revocable delegation: primary and session keys, budget limits and mandates define exactly what an agent may do with money, and owners can revoke or rotate that authority. <Link href="/agent-budgets">Agent budgets</Link></p></details>
          <details><summary>How do I verify a payment actually settled?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Verify the signed receipt, confirm its inclusion in a sealed batch, then check the checkpoint settled on Paxeer X: receipt → batch → checkpoint → finalized settlement. <Link href="/receipts">Receipts and proofs</Link></p></details>
          <details><summary>Can I use my existing EVM tooling?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Yes. Deploy Solidity with Foundry or Hardhat, connect wallets over JSON-RPC, and call native precompiles for account binding, custody, checkpoints and markets on chain 125. <Link href="/evm">Build on the EVM</Link></p></details>
          <details><summary>How do I run a node or operate the network?<span className="cd-faq-toggle" aria-hidden="true" /></summary><p>Start with installation and configuration, then the operator guides for chain nodes, sequencers, replicas, guarantors and archive services. <Link href="/run-node">Run a node</Link></p></details>
        </div>
      </section>
      <Band />
      <section className="cd-cta">
        <img className="cd-cta-bg" src="/art/docs/system-overview.webp" alt="" aria-hidden="true" width={1536} height={1024} loading="lazy" decoding="async" />
        <div className="cd-cta-inner"><PaxeerMark /><h2>Build the application.<br />Connect the economy.</h2><p>Choose a domain, integrate an interface and follow the evidence through the network.</p><div className="hero-actions"><Link href="/xdk" className="pill-button primary-button">Open the developer toolkit <Icon name="arrow" size={16} /></Link><Link href="/unified-network" className="pill-button cd-cta-ghost">Explore the network</Link></div></div>
      </section>
    </div>
    <footer className="site-footer"><div className="landing-col cd-footer-col"><div className="cd-footer-top"><div className="footer-intro"><Link href="/" className="brand"><PaxeerMark />Paxeer X Docs</Link><p>The chain, agent economy and developer platform in one reference.</p></div><div className="footer-links"><div><span>BUILD</span><Link href="/xdk">Developer toolkit</Link><Link href="/agents">Agents</Link><Link href="/programs">Programs</Link></div><div><span>USE</span><Link href="/payments-quickstart">Payments</Link><Link href="/spot-exchange">Markets</Link><Link href="/human-interface">Human interface</Link></div><div><span>UNDERSTAND</span><Link href="/unified-network">Unified network</Link><Link href="/proofs">Proofs</Link><Link href="/operators">Operations</Link></div></div></div><nav className="cd-footer-quick" aria-label="Landing sections"><span>ON THIS PAGE:</span><a href="#solution">Solution</a><a href="#benefits">Benefits</a><a href="#comparison">Comparison</a><a href="#how-it-works">How it works</a><a href="#paths">Build paths</a><a href="#faqs">FAQs</a></nav><div className="footer-bottom"><span>Paxeer X Network</span><span>EVM 125 · hyperpax_125-1 · LayerX</span></div></div></footer>
  </main>
}
