import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Privacy Policy — Paxport',
  description: 'Privacy policy for the Paxport installable web wallet.',
};

export default function PrivacyPage() {
  return (
    <div className="min-h-screen bg-pax-bg text-white">
      <div className="max-w-2xl mx-auto px-6 py-16">
        <h1 className="text-3xl font-bold mb-2">Privacy Policy</h1>
        <p className="text-sm text-gray-400 mb-10">Last updated: November 1, 2026</p>

        <div className="space-y-8 text-sm leading-relaxed text-gray-300">
          <section>
            <h2 className="text-lg font-semibold text-white mb-3">1. Introduction</h2>
            <p>
              Paxport (&quot;we&quot; or &quot;our&quot;) is an installable web wallet for Paxeer
              Network. The app uses the wallet SDK to hold an embedded wallet or connect an injected
              wallet supplied by your chosen wallet provider. This privacy policy explains what data
              the web wallet stores and sends when you use its features.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">2. Data We Collect</h2>
            <p className="mb-3">
              The data handled depends on your wallet provider and the features you use, including
              sign-in, blockchain access, push notifications, and assistant chat.
            </p>
            <h3 className="text-base font-medium text-white mb-2">
              2.1 Data Stored Locally on Your Device
            </h3>
            <ul className="list-disc list-inside space-y-1.5 ml-2">
              <li>
                <strong>Embedded wallet vault</strong> — For the self-custody embedded wallet,
                encrypted private keys and recovery material are held in the browser wallet vault in
                IndexedDB. An injected wallet manages its own keys and storage through its provider.
              </li>
              <li>
                <strong>Account addresses</strong> — Your public wallet addresses are stored locally
                to display balances and transaction history.
              </li>
              <li>
                <strong>Contacts</strong> — Saved wallet addresses you choose to label for
                convenience.
              </li>
              <li>
                <strong>Connected sites</strong> — A list of website origins you have approved for
                wallet connection.
              </li>
              <li>
                <strong>Session and preferences</strong> — Wallet session state, display
                preferences, and notification preferences support the web wallet across browser
                sessions. Sign-in session state is managed by the authentication adapter.
              </li>
            </ul>

            <h3 className="text-base font-medium text-white mb-2 mt-4">
              2.2 Data Transmitted to External Services
            </h3>
            <ul className="list-disc list-inside space-y-1.5 ml-2">
              <li>
                <strong>Sign-in</strong> — Sign-in goes through Supabase. Supabase processes the
                authentication information you submit and manages your sign-in session.
              </li>
              <li>
                <strong>Wallet connection</strong> — The wallet SDK uses an embedded wallet or
                connects an injected wallet. The selected provider handles account access and
                signing requests you authorize.
              </li>
              <li>
                <strong>Blockchain RPC requests</strong> — To read balances and send transactions,
                the web wallet sends your public wallet address and signed transactions to the
                configured Paxeer Network chain RPC at its root endpoint. If you select a custom RPC
                or another network in wallet settings, that endpoint receives those requests.
              </li>
              <li>
                <strong>Portfolio and price data</strong> — Your public wallet address is sent to
                the wallet server through a same-origin proxy to fetch token balances, transaction
                history, and price data. The server forwards these queries to the configured Paxeer
                indexer service (default: <code>api.paxscan.io</code>).
              </li>
              <li>
                <strong>Block explorer queries</strong> — Your public address and transaction hashes
                may be sent to the configured explorer (default: <code>paxscan.io</code>) when you
                open explorer links. In-app explorer data is read through the wallet server&apos;s
                same-origin proxy, which forwards queries to the configured Paxeer indexer service
                (default: <code>api.paxscan.io</code>) using the Blockscout API.
              </li>
              <li>
                <strong>Push subscription</strong> — If you enable push notifications, the wallet
                server stores a push subscription record containing your wallet address, push
                endpoint, subscription keys, timestamps, and optional browser user-agent information
                and notification tags. This record persists on the server so notifications can be
                delivered.
              </li>
              <li>
                <strong>Assistant chat</strong> — When you use the assistant, your chat messages are
                sent through the wallet server to OpenAI at <code>api.openai.com</code>. The server
                also sends whether a wallet is connected; it does not automatically forward your
                wallet address or balances to OpenAI. Information you include in messages is part of
                the content sent.
              </li>
            </ul>

            <h3 className="text-base font-medium text-white mb-2 mt-4">
              2.3 Keep Your Wallet Secrets Private
            </h3>
            <ul className="list-disc list-inside space-y-1.5 ml-2">
              <li>
                Do not include private keys, recovery phrases, or your PIN in assistant messages or
                support requests.
              </li>
              <li>
                Sign-in, wallet providers, push notifications, and assistant chat involve the
                additional data flows described above, alongside blockchain RPC, indexer, and
                explorer requests.
              </li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">3. How Your Data Is Used</h2>
            <ul className="list-disc list-inside space-y-1.5 ml-2">
              <li>
                Encrypted wallet data is used solely to derive signing keys when you authorize a
                transaction.
              </li>
              <li>
                Public addresses are used to query the blockchain for balances and transaction
                history.
              </li>
              <li>
                Connected site origins are used to determine which websites may request wallet
                actions.
              </li>
              <li>Session state supports sign-in and wallet locking.</li>
              <li>
                Push subscription records associate a notification destination with your wallet
                address.
              </li>
              <li>Assistant messages are processed by OpenAI to generate chat responses.</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">4. Data Storage and Security</h2>
            <ul className="list-disc list-inside space-y-1.5 ml-2">
              <li>
                The self-custody embedded wallet encrypts its vault with AES-256 and derives
                encryption keys from your PIN using PBKDF2 with a unique salt, attempt throttling,
                and optional device-bound biometric protection.
              </li>
              <li>
                The web wallet uses browser storage, including IndexedDB for the self-custody vault
                and local storage for application preferences and records. Browser storage is scoped
                to the app&apos;s origin.
              </li>
              <li>
                The wallet server persists push subscription records, including the wallet address,
                on server storage. Clearing browser data does not delete those server records.
              </li>
              <li>
                Supabase manages sign-in data, and assistant chat content is processed by OpenAI.
                These services have their own data handling policies.
              </li>
              <li>
                The web app&apos;s Content Security Policy restricts script execution and network
                connections to configured sources.
              </li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">5. Third-Party Services</h2>
            <p>
              The installable web wallet communicates with network services and external providers.
              Supabase handles sign-in, the wallet SDK accesses the embedded wallet or your chosen
              injected wallet provider, the wallet server stores push subscription records with your
              wallet address, and OpenAI processes assistant chat messages.
            </p>
            <div className="mt-3 rounded-xl bg-white/5 overflow-hidden">
              <table className="w-full text-xs">
                <thead>
                  <tr className=" ">
                    <th className="text-left px-4 py-2.5 text-gray-400 font-medium">Service</th>
                    <th className="text-left px-4 py-2.5 text-gray-400 font-medium">Purpose</th>
                    <th className="text-left px-4 py-2.5 text-gray-400 font-medium">Data Sent</th>
                  </tr>
                </thead>
                <tbody className=" divide-white/5">
                  <tr>
                    <td className="px-4 py-2.5">Supabase</td>
                    <td className="px-4 py-2.5">Sign-in and authentication sessions</td>
                    <td className="px-4 py-2.5">
                      Authentication information you submit and session data
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-2.5">Wallet SDK and selected wallet provider</td>
                    <td className="px-4 py-2.5">Embedded or injected wallet access</td>
                    <td className="px-4 py-2.5">Account access and authorized signing requests</td>
                  </tr>
                  <tr>
                    <td className="px-4 py-2.5">Wallet push server</td>
                    <td className="px-4 py-2.5">
                      Push subscription storage and notification delivery
                    </td>
                    <td className="px-4 py-2.5">
                      Wallet address, push endpoint, subscription keys, timestamps, optional user
                      agent and tags
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-2.5">
                      OpenAI (<code>api.openai.com</code>)
                    </td>
                    <td className="px-4 py-2.5">Assistant chat responses</td>
                    <td className="px-4 py-2.5">Chat messages and whether a wallet is connected</td>
                  </tr>
                  <tr>
                    <td className="px-4 py-2.5">
                      Configured chain RPC or selected network endpoint
                    </td>
                    <td className="px-4 py-2.5">Blockchain RPC</td>
                    <td className="px-4 py-2.5">Public address, signed transactions</td>
                  </tr>
                  <tr>
                    <td className="px-4 py-2.5">
                      Configured explorer (default: <code>paxscan.io</code>)
                    </td>
                    <td className="px-4 py-2.5">Explorer links</td>
                    <td className="px-4 py-2.5">Public address, transaction hashes</td>
                  </tr>
                  <tr>
                    <td className="px-4 py-2.5">
                      Wallet server and configured Paxeer indexer (default:{' '}
                      <code>api.paxscan.io</code>)
                    </td>
                    <td className="px-4 py-2.5">
                      Portfolio, price and in-app explorer data via the same-origin Blockscout proxy
                    </td>
                    <td className="px-4 py-2.5">
                      Public address, token addresses, transaction hashes
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p className="mt-3">
              Data sent to external providers is subject to their privacy policies. Review your
              selected wallet provider&apos;s policy for its handling of accounts and signing
              requests.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">
              6. Installation and Browser Permissions
            </h2>
            <ul className="list-disc list-inside space-y-1.5 ml-2">
              <li>
                <strong>Installation</strong> — Paxport can be installed as a progressive web app
                (PWA). It uses browser storage and a service worker for its web app features.
              </li>
              <li>
                <strong>Notifications</strong> — Push notifications require your browser&apos;s
                permission and create the server-side push subscription described above. You can
                revoke notification permission in your browser settings.
              </li>
              <li>
                <strong>Wallet connection</strong> — Connecting an injected wallet requests account
                access from your selected provider. That provider presents its own connection and
                signing approvals.
              </li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">7. Your Rights</h2>
            <ul className="list-disc list-inside space-y-1.5 ml-2">
              <li>
                <strong>Wallet control</strong> — Self-custody wallet secrets are managed in the
                embedded wallet vault or by your injected wallet provider.
              </li>
              <li>
                <strong>Local data</strong> — Clearing the app&apos;s site data removes local
                browser records. Back up your self-custody recovery material before deleting the
                vault.
              </li>
              <li>
                <strong>Connection management</strong> — You can revoke connected site access and
                manage injected wallet permissions through your wallet provider.
              </li>
              <li>
                <strong>Server data</strong> — Deleting local data does not delete Supabase sign-in
                data or server-side push subscription records. Contact us about access or deletion
                requests for data held by the wallet service.
              </li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">8. Children&apos;s Privacy</h2>
            <p>
              The web wallet is not directed at children under the age of 13. We do not knowingly
              collect personal information from children.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">9. Changes to This Policy</h2>
            <p>
              We may update this privacy policy from time to time. Changes will be reflected by
              updating the &quot;Last updated&quot; date at the top of this page. Continued use of
              the web wallet after changes constitutes acceptance of the updated policy.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">10. Contact</h2>
            <p>
              If you have questions about this privacy policy or the web wallet&apos;s data
              practices, contact us at{' '}
              <a href="mailto:privacy@paxeer.app" className="text-pax-accent hover:underline">
                privacy@paxeer.app
              </a>
              .
            </p>
          </section>
        </div>

        <div className="mt-16 pt-8   text-center">
          <p className="text-xs text-gray-500">Paxport v1.0.0 — Paxeer Network</p>
        </div>
      </div>
    </div>
  );
}
