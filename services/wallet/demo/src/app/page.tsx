import { ConnectButton } from '@/components/ConnectButton';

export default function Home() {
  return (
    <main className="page">
      <BackgroundGrid />

      <header className="shell page-header">
        <div className="brand">
          <PaxeerLogo />
          <span className="brand-name">Paxeer</span>
          <span className="badge demo-badge">Wallet · Demo</span>
        </div>
        <ConnectButton />
      </header>

      <section className="shell hero">
        <span className="badge">Embedded and Injected Wallets · HyperPaxeer · Chain 125</span>
        <h1 className="hero-title">
          One sign-in.
          <br />
          One wallet.
          <br />
          <span className="muted">Every Paxeer app.</span>
        </h1>
        <p className="hero-copy">
          Sign in to explicitly provision an embedded threshold-custody wallet, or connect an
          injected wallet. Review and approve each signing request.
        </p>

        <div className="actions">
          <ConnectButton />
          <a
            href="https://github.com/paxeer-network/paxeer-embedded-wallet"
            target="_blank"
            rel="noreferrer"
            className="button quiet"
          >
            View on GitHub →
          </a>
        </div>
      </section>

      <section className="shell features">
        <FeatureCard
          title="Threshold custody"
          body="Embedded signing uses the threshold custody service. Injected signing stays with your chosen wallet. No private key is exported to this example."
        />
        <FeatureCard
          title="Same wallet, every app"
          body="One Supabase identity, one EVM address, persisted across every paxeer.network surface."
        />
        <FeatureCard
          title="Drop-in for Next.js"
          body="Three components: ConnectButton, WalletModal, SendTxForm. Copy them straight into your app."
        />
      </section>

      <footer className="shell page-footer">
        Aligned with Paxeer Brand Identity System v2.0 · 2026
      </footer>
    </main>
  );
}

function FeatureCard({ title, body }: { title: string; body: string }) {
  return (
    <article className="card">
      <h2>{title}</h2>
      <p>{body}</p>
    </article>
  );
}

function PaxeerLogo() {
  return (
    <svg
      className="brand-logo"
      width="22"
      height="22"
      viewBox="0 0 22 22"
      fill="none"
      aria-hidden="true"
    >
      <rect x="1" y="1" width="20" height="20" rx="5" />
      <path d="M5.5 16.5L16.5 5.5" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

function BackgroundGrid() {
  return <div aria-hidden="true" className="background-grid" />;
}
