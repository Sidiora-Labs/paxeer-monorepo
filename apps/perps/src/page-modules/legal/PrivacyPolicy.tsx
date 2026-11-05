import { LegalLayout } from "./LegalLayout";

export default function PrivacyPolicy() {
  return (
    <LegalLayout title="Privacy Policy" lastUpdated="4 November 2026">
      <p>
        This Privacy Policy explains how <strong>Paxeer Network Foundation</strong> ("we", "us", or "our")
        collects, uses, and protects information when you access or use the HyperPax decentralised
        exchange interface (the "Interface") available at hyperpax.io and associated subdomains
        (collectively, the "Services").
      </p>
      <p>
        <strong>The Interface does not hold your private keys.</strong> Perpetuals execute in native
        chain software on the Paxeer Network, with USDL collateral held by the perps module account.
        Wallet-linked off-chain application records are separate from this public-chain state. See
        the <a href="/terms">Terms of Service</a> for execution, custody and governance risks.
      </p>

      <h2>1. Information We Collect</h2>
      <p>We collect the minimum information necessary to operate the Interface:</p>
      <ul>
        <li>
          <strong>Blockchain address.</strong> When you connect a wallet, your public wallet address is
          read from your wallet provider and used to display your balances, positions, and transaction
          history. Your private key is never transmitted to us.
        </li>
        <li>
          <strong>Operational error reports.</strong> When Sentry is configured, browser and server
          exceptions may be transmitted regardless of cookie preference. Reports can include messages,
          stack traces, URLs, device information, breadcrumbs, environment and release, diagnostic
          context and optional feedback contact details. They are not guaranteed to be anonymous.
          See the <a href="/cookies">Cookie Policy</a> for fields, controls and retention limits.
        </li>
        <li>
          <strong>Wallet authentication.</strong> The perps data service stores your wallet address,
          challenge nonce, sign-in message, issue and expiry timestamps, and a used-at timestamp to
          verify ownership and prevent replay. A challenge is valid for five minutes; a bearer token
          is valid for one hour, with refresh limited to 24 hours from sign-in. These validity periods
          do not delete stored challenge records.
        </li>
        <li>
          <strong>Referrals.</strong> Off-chain records link your wallet address to a referral code,
          the referring wallet address, the time a referral was applied and total rewards earned.
          They support referral attribution, reward accounting, tiers and fee discounts.
        </li>
        <li>
          <strong>Competitions.</strong> Off-chain entries store competition identifier, wallet address
          and join time. They support participation and rankings derived from trading activity;
          rankings may display your wallet address and score publicly.
        </li>
        <li>
          <strong>Alerts and account views.</strong> The service stores wallet-linked alert identifiers,
          alert types, market identifiers, thresholds, enabled state, creation and trigger times.
          It also indexes chain-derived balances, deposits, withdrawals, positions, trades and daily
          profit, loss and volume to provide account history and statistics.
        </li>
        <li>
          <strong>Cookies and local storage.</strong> We use essential cookies required for the Interface
          to function and optional analytics cookies if you consent. See our{" "}
          <a href="/cookies">Cookie Policy</a> for full details.
        </li>
        <li>
          <strong>Communications.</strong> If you contact us via email or our community channels, we may
          retain the content of your message and your contact details to respond to you.
        </li>
      </ul>
      <p>
        Wallet addresses and linked records can be personal data. Wallet sign-in does not require
        a name, email address or government-issued identification. You may voluntarily provide contact
        information in communications or error feedback. Protected account features create wallet-linked
        records even without a conventional username or email registration.
      </p>

      <h2>2. On-Chain Data</h2>
      <p>
        Confirmed transactions submitted through the Interface are recorded on the Paxeer Network public
        blockchain. By nature of blockchain technology, this data is publicly visible, immutable, and
        outside our control. We are not responsible for the privacy implications of publicly visible
        on-chain activity.
      </p>

      <h2>3. How We Use Information</h2>
      <ul>
        <li>Provide, operate, and improve the Interface.</li>
        <li>Diagnose and fix bugs and performance issues.</li>
        <li>Understand how traders and builders use the platform to prioritise features.</li>
        <li>Comply with applicable laws and regulations where required.</li>
        <li>Detect, prevent, and investigate fraudulent or prohibited activity.</li>
      </ul>
      <p>We do not sell, rent, or trade any information about you to third parties for commercial gain.</p>

      <h2>4. Third-Party Services</h2>
      <p>The Interface integrates with the following third-party services that may process data:</p>
      <ul>
        <li>
          <strong>TradingView.</strong> Charting widgets embedded in the Interface. TradingView's own
          privacy policy governs data processed through their charting library.
        </li>
        <li>
          <strong>Pyth Network &amp; Ojo Network (oracle providers).</strong> On-chain price oracles;
          no personal data is transmitted.
        </li>
        <li>
          <strong>Wallet providers (MetaMask, WalletConnect, etc.).</strong> Third-party wallet software
          is subject to each provider's own privacy policy. We receive only the public address you
          authorise.
        </li>
        <li>
          <strong>Sentry.</strong> Processes operational error reports and any submitted error feedback.
          Collection and retention are described in the <a href="/cookies">Cookie Policy</a>.
        </li>
        <li>
          <strong>StatusPage.io.</strong> Used for system status monitoring. A small status embed script
          may load from their CDN. See Atlassian's privacy policy for details.
        </li>
      </ul>

      <h2>5. Data Retention</h2>
      <p>
        Off-chain authentication challenges, referral relationships and competition entries persist
        in the application database. The current service does not implement automatic expiry-based
        deletion for these records or a self-service deletion endpoint for them. Challenge or token
        expiry prevents authentication use; it does not erase a database row. Disconnecting a wallet,
        clearing browser storage or withdrawing optional cookie consent does not delete these records.
      </p>
      <p>
        Alert records remain until deleted using the authenticated alert controls. Chain-derived
        account history is retained in the service's index and can be reconstructed from public chain
        records. No fixed maximum retention period for these application records is enforced by the
        current service. Sentry retention depends on the deployed project's settings; changing cookie
        preferences does not erase error reports. Browser preferences remain until overwritten or
        cleared in your browser.
      </p>
      <p>
        Request access, correction, the applicable retention period or deletion of wallet-linked
        off-chain records at <a href="mailto:privacy@paxeer.network">privacy@paxeer.network</a>.
        Requests require verification of wallet ownership; never send a private key or seed phrase.
        Deletion is handled by the service operator subject to applicable legal obligations and
        technical limits, rather than promised as an automatic consequence of a browser action.
        Deleting an off-chain record cannot erase immutable public-chain history or independent
        copies held by other parties.
      </p>

      <h2>6. International Transfers</h2>
      <p>
        Our application and error-reporting infrastructure may process data in jurisdictions outside your home country,
        including the United States and European Union. Where required by law, we implement appropriate
        safeguards such as Standard Contractual Clauses for transfers outside the EEA.
      </p>

      <h2>7. Your Rights</h2>
      <p>
        Depending on your jurisdiction, you may have the following rights with respect to your personal data:
      </p>
      <ul>
        <li><strong>Right of access</strong> — request a copy of the data we hold about you.</li>
        <li><strong>Right to rectification</strong> — request correction of inaccurate data.</li>
        <li><strong>Right to erasure</strong> — request deletion of your personal data.</li>
        <li><strong>Right to restrict processing</strong> — request that we limit how we use your data.</li>
        <li><strong>Right to data portability</strong> — receive your data in a structured, machine-readable format.</li>
        <li><strong>Right to object</strong> — object to processing based on our legitimate interests.</li>
        <li><strong>Right to withdraw consent</strong> — withdraw cookie consent at any time via the Cookie Settings link in the footer.</li>
      </ul>
      <p>
        These rights apply to wallet-linked off-chain records as well as information you voluntarily
        provide, subject to applicable law. Public-chain records cannot be erased by the operator.
        To verify ownership and discuss a request, contact us at <a href="mailto:privacy@paxeer.network">privacy@paxeer.network</a>.
      </p>

      <h2>8. Security</h2>
      <p>
        We implement commercially reasonable technical and organisational measures to protect the
        information we collect. However, no Internet transmission or electronic storage is perfectly
        secure. We encourage you to take appropriate precautions to protect your wallet credentials and
        private keys, which are entirely within your control.
      </p>

      <h2>9. Children's Privacy</h2>
      <p>
        The Services are not directed to individuals under the age of 18. We do not knowingly collect
        personal information from minors. If you believe a minor has provided us with personal data,
        please contact us immediately.
      </p>

      <h2>10. Changes to This Policy</h2>
      <p>
        We may update this Privacy Policy from time to time. The "last updated" date at the top of this
        page indicates when the most recent changes were made. Continued use of the Interface after a
        policy update constitutes your acceptance of the revised policy.
      </p>

      <h2>11. Contact Us</h2>
      <p>
        For privacy-related enquiries, please contact us at:{" "}
        <a href="mailto:privacy@paxeer.network">privacy@paxeer.network</a>.
      </p>
    </LegalLayout>
  );
}
