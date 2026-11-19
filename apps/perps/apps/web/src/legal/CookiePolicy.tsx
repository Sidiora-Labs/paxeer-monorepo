"use client";

import React from "react";

import { LegalLayout } from "./LegalLayout";
import { useCookieConsent } from "./useCookieConsent";
import { Button } from "@paxeer-network/ui/button/button";

function ConsentControls() {
  const { value, accept, decline } = useCookieConsent();

  return (
    <div className="oui-rounded-xl oui-border oui-border-line-12 oui-bg-base-9 oui-p-4 oui-my-6">
      <p className="oui-text-xs oui-text-base-contrast-54 oui-mb-3">
        <strong className="oui-text-base-contrast">
          Your current preference:{" "}
        </strong>
        {value === "accepted" && (
          <span className="oui-text-success">Accepting all cookies</span>
        )}
        {value === "declined" && (
          <span className="oui-text-warning">Essential cookies only</span>
        )}
        {value === null && (
          <span className="oui-text-base-contrast-54">Not yet set</span>
        )}
      </p>
      <div className="oui-flex oui-flex-wrap oui-gap-2">
        <Button
          type="button"
          onClick={decline}
          size="sm"
          variant="outlined"
          color="secondary"
        >
          Essential only
        </Button>
        <Button type="button" onClick={accept} size="sm">
          Accept all cookies
        </Button>
      </div>
    </div>
  );
}

export default function CookiePolicy() {
  return (
    <LegalLayout title="Cookie Policy" lastUpdated="4 November 2026">
      <p>
        This Cookie Policy explains what cookies are, how{" "}
        <strong>Paxeer Network Foundation</strong> ("we", "us", or "our") uses
        them on the HyperPax Interface, and how you can control them. Optional
        cookie choices also control browser error reporting as described below.
      </p>

      <h2>What Are Cookies?</h2>
      <p>
        Cookies are small text files placed on your device by a website when you
        visit it. They allow the site to remember information about your visit,
        such as your preferences or authentication state. Cookies can be
        "session" cookies (deleted when you close your browser) or "persistent"
        cookies (retained for a set period or until deleted).
      </p>
      <p>
        We also use similar technologies such as <strong>localStorage</strong>{" "}
        and <strong>sessionStorage</strong> in your browser — these work in a
        similar way to cookies and are covered by this policy.
      </p>

      <h2>Categories of Cookies We Use</h2>

      <h2>1. Strictly Necessary Cookies</h2>
      <p>
        These cookies are essential for the Interface to function. They cannot
        be disabled. They do not store any personally identifying information.
      </p>
      <ul>
        <li>
          <strong>Wallet session state.</strong> Stored in localStorage to
          remember which wallet provider you connected and the network you
          selected, so you don't need to reconnect on every page visit.
        </li>
        <li>
          <strong>Theme preference.</strong> Stores your chosen colour theme
          (dark, light, etc.) in localStorage.
        </li>
        <li>
          <strong>Cookie consent preference.</strong> Stores your response to
          this cookie banner in localStorage (key:{" "}
          <code className="oui-text-xs oui-bg-base-8 oui-px-1.5 oui-py-0.5 oui-rounded">
            hyperpax_cookie_consent
          </code>
          ).
        </li>
        <li>
          <strong>Trading settings.</strong> Stores locally-configured
          preferences such as default leverage, slippage tolerance, and
          interface layout in localStorage.
        </li>
      </ul>

      <h2>2. Cookie Choices and Operational Error Reporting</h2>
      <p>
        The cookie controls save your preference for optional cookies and
        browser error reporting. When Sentry is configured, browser error
        reporting starts only after "Accept all cookies" and stops when you
        select "Essential only". Server exceptions may be sent before you choose
        a preference, after "Accept all cookies", after "Essential only", and
        after you withdraw optional consent by selecting "Essential only" again.
        Error-event transmission is separate from placing cookies or storing a
        consent preference in your browser.
      </p>
      <p>
        Sentry helps diagnose failures. Reports may include error messages,
        stack traces, page or request URLs, browser and device information,
        breadcrumbs, environment and release identifiers, error codes, context,
        tags and additional diagnostic fields. If you submit error feedback,
        your message and any name or email you provide may also be sent. Network
        requests expose connection metadata, including your IP address, to the
        receiving service. Reports are not guaranteed to be anonymous or
        aggregated; diagnostic text or URLs may contain wallet-linked
        information. Do not include private keys or seed phrases in feedback.
      </p>
      <p>
        Default personal-information collection is disabled. The browser
        configuration applies best-effort redaction to error messages and
        exception text; this does not sanitise every field, and the server
        configuration does not apply that same redaction. Performance tracing
        and browser session replay are disabled in the current configuration.
      </p>
      <p>
        Error reports are retained according to the deployed Sentry project's
        retention settings; the Interface does not configure a fixed retention
        period or automatically delete reports when you change cookie
        preferences. Contact{" "}
        <a href="mailto:privacy@paxeer.network">privacy@paxeer.network</a> for
        the applicable retention period or to request access or deletion.
        Withdrawing optional consent stops browser error reporting but does not
        stop server error reporting or erase reports already sent.
      </p>

      <h2>3. Third-Party Cookies</h2>
      <p>
        Some third-party services embedded in the Interface may set their own
        cookies:
      </p>
      <ul>
        <li>
          <strong>TradingView.</strong> The charting widget embedded on the
          trading page may set session and preference cookies managed by
          TradingView. These are subject to TradingView's own cookie and privacy
          policy.
        </li>
        <li>
          <strong>StatusPage.io (Atlassian).</strong> The system status embed
          script may set cookies to cache status data. Subject to Atlassian's
          privacy policy.
        </li>
        <li>
          <strong>WalletConnect.</strong> If you connect via WalletConnect,
          their relay service may set cookies related to the connection session.
        </li>
      </ul>
      <p>
        We have no control over third-party cookies. You should review the
        respective providers' cookie policies for details.
      </p>

      <h2>Manage Your Preferences</h2>
      <p>
        You can update your cookie consent at any time using the controls below.
        Changes take effect immediately for the stored preference. Selecting
        "Essential only" withdraws optional consent and stops browser error
        reporting, but does not delete cookies already placed — you may need to
        clear them manually via your browser settings.
      </p>

      <ConsentControls />

      <h2>How to Control Cookies via Your Browser</h2>
      <p>
        Most browsers allow you to refuse or delete cookies through their
        settings. The links below provide guidance for common browsers:
      </p>
      <ul>
        <li>
          <a
            href="https://support.google.com/chrome/answer/95647"
            target="_blank"
            rel="noopener noreferrer"
          >
            Google Chrome
          </a>
        </li>
        <li>
          <a
            href="https://support.mozilla.org/en-US/kb/cookies-information-websites-store-on-your-computer"
            target="_blank"
            rel="noopener noreferrer"
          >
            Mozilla Firefox
          </a>
        </li>
        <li>
          <a
            href="https://support.apple.com/guide/safari/manage-cookies-sfri11471/mac"
            target="_blank"
            rel="noopener noreferrer"
          >
            Apple Safari
          </a>
        </li>
        {/* prettier-ignore */}
        <li><a href="https://support.microsoft.com/en-us/edge/manage-cookies-in-microsoft-edge-view-allow-block-delete-and-use" target="_blank" rel="noopener noreferrer">Microsoft Edge</a></li>
      </ul>
      <p>
        Note that disabling cookies may affect the functionality of the
        Interface, particularly wallet connection and trading settings.
      </p>

      <h2>Changes to This Policy</h2>
      <p>
        We may update this Cookie Policy from time to time. The "last updated"
        date at the top of this page reflects the most recent changes. Continued
        use of the Interface after an update constitutes acceptance of the
        revised policy.
      </p>

      <h2>Contact</h2>
      <p>
        For cookie-related questions, contact us at:{" "}
        <a href="mailto:privacy@paxeer.network">privacy@paxeer.network</a>.
      </p>
    </LegalLayout>
  );
}
