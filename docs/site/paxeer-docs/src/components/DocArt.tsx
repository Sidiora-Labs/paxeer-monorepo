import Link from 'next/link'

const art = {
  'quickstart': 'Paxeer X Documentation quickstart card with an open book and an arrow',
  'installation': 'Installation card: a download arrow above a terminal showing install and Ready',
  'configuration': 'Configuration card with three adjustable sliders',
  'authentication': 'Paxeer X authentication card with a key and a verified shield',
  'agent-lifecycle': 'Agent lifecycle: create, run, complete',
  'task-queue': 'Task queue with one ready task and two waiting tasks',
  'tool-discovery': 'Tool discovery: search, compute and storage tools, each ready to connect',
  'resource-browser': 'Resource browser listing documents, datasets and artifacts in a workspace',
  'prompt-templates': 'Prompt template with role, context and task fields',
  'intent-builder': 'Build an intent from a goal, inputs and constraints, then preview it',
  'request-envelope': 'Request envelope carrying intent, context and payload',
  'payment-receipt': 'Payment receipt: protocol 402LXP, service Compute, status Settled',
  'service-payment': 'Service payment: an application pays through 402LXP and receives a receipt',
  'workload-inputs': 'Workload inputs: model, data and parameters',
  'model-registry': 'Registry listing language, embedding and custom models',
  'job-progress': 'Job progress: submitted, processing, output ready',
  'results-artifacts': 'Results and artifacts: output, logs and metadata ready to download',
  'chain-configuration': 'Chain configuration with name, workload and permissions fields',
  'state-snapshots': 'Stacked state snapshots',
  'event-subscriptions': 'Event subscriptions for task, payment and chain events',
  'debugging': 'Debugging: a magnifying glass over terminal output',
  'reference-guide': 'Reference: find the details, build with clarity',
  'system-overview': 'Paxeer Docs: explore the connected system',
  'market-growth': 'Portfolio growth dashboard with a weekly bar chart for BTC, ETH, SOL and PAX, framed by painted clouds',
  'asset-allocation': 'Portfolio allocation card showing Bitcoin, Ethereum, Solana and PAX shares, framed by painted clouds',
  'activity-notifications': 'Notification panel with a 2FA prompt, a completed PAX to USDC swap and trading rewards, framed by painted clouds',
  'market-prices': 'Wide market price list for Bitcoin, Ethereum, Paxeer PAX, XRP, Tether and Tron, framed by painted clouds',
  'wallet-connect': 'Wide wallet connector panel listing MetaMask, Binance, Coinbase, WalletConnect and Phantom, framed by painted clouds',
} as const

export type DocArtName = keyof typeof art

/** Static artwork from public/art/docs, framed as a card. All sources are 1536 × 1024. */
export function DocArt({ name, className }: { name: DocArtName; className?: string }) {
  return <figure className={`doc-art${className ? ` ${className}` : ''}`}>
    <img src={`/art/docs/${name}.webp`} alt={art[name]} width={1536} height={1024} loading="lazy" decoding="async" />
  </figure>
}

export type ArtCard = { art: DocArtName; title: string; text: string; href: string }

/** A bordered section of artwork cards: an eyebrow, a centered heading, then rows of cards. */
export function ArtGrid({ id, eyebrow, title, rows }: { id: string; eyebrow: string; title: string; rows: ArtCard[][] }) {
  return <section className="art-section" aria-labelledby={id}>
    <div className="art-band" aria-hidden="true" />
    <div className="art-section-head"><p className="eyebrow">{eyebrow}</p><h2 id={id}>{title}</h2></div>
    {rows.map((row, index) => <div className={`art-row art-row--${row.length}`} key={index}>
      {row.map(card => <Link href={card.href} className="art-card" key={card.art}>
        <span className="art-card-media"><img src={`/art/docs/${card.art}.webp`} alt={art[card.art]} width={1536} height={1024} loading="lazy" decoding="async" /></span>
        <span className="art-card-body"><strong>{card.title}</strong><span>{card.text}</span></span>
      </Link>)}
    </div>)}
  </section>
}
