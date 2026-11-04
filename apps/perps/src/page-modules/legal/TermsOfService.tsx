import { LegalLayout } from "./LegalLayout";

export default function TermsOfService() {
  return (
    <LegalLayout title="Terms of Service" lastUpdated="4 November 2026">
      <p>
        These Terms of Service ("Terms") govern your access to and use of the HyperPax interface
        (the "Interface") provided by <strong>Paxeer Network Foundation</strong> ("Foundation", "we",
        "us", or "our"). By accessing or using the Interface, you agree to be bound by these Terms.
        If you do not agree, do not use the Interface.
      </p>
      <p>
        Please read these Terms carefully. They contain important information about your legal rights,
        including mandatory risk disclosures, limitations of liability, and a prohibition on use by
        residents of certain jurisdictions.
      </p>

      <h2>1. The Interface and the Protocol</h2>
      <p>
        The Interface provides access to the HyperPax perpetual futures venue (the "Protocol") on
        the Paxeer Network blockchain (Chain ID 125). Perpetuals execute in native Go chain software
        in <code>modules/perps</code>, accessed from EVM transactions through the perps precompile
        at <code>0x0000000000000000000000000000000000001015</code>. The Solidity
        <code>Perps.sol</code> file defines the calling interface; it is not the venue's execution
        implementation. USDL is a separate ERC20 collateral token.
      </p>
      <p>
        Chain governance can change venue parameters, including fees, margin requirements, market
        configuration and pause controls. Network software upgrades can change native execution rules.
        These controls and the separate USDL token contract create dependencies and risks beyond
        the Interface; open-source code does not mean the venue is immutable or free of governance
        control.
      </p>
      <p>
        These Terms make no claim that any particular release or component has been independently
        audited. Any audit report you rely on must identify its exact release or code revision and
        component scope. A review of the Solidity calling interface alone does not establish review
        of native execution, upgrade controls or the USDL token contract.
      </p>
      <p>
        <strong>The Interface is one means of accessing the Protocol.</strong> Other clients and
        command-line tools may submit transactions to the native venue through its supported
        interfaces. These Terms govern use of this Interface; direct Protocol access remains subject
        to the chain's execution rules and applicable law.
      </p>

      <h2>2. Eligibility and Prohibited Use</h2>
      <p>You may only use the Interface if you:</p>
      <ul>
        <li>Are at least 18 years old (or the age of majority in your jurisdiction).</li>
        <li>Have full legal capacity to enter into a binding agreement.</li>
        <li>Are not a resident or citizen of a Restricted Jurisdiction (defined below).</li>
        <li>Are not accessing the Interface on behalf of a person in a Restricted Jurisdiction.</li>
        <li>Are not a Specially Designated National or subject to applicable sanctions lists.</li>
      </ul>
      <p>
        <strong>Restricted Jurisdictions</strong> include, without limitation: the United States of
        America, Cuba, Iran, North Korea, Syria, Russia (in respect of sanctioned activities),
        and any other jurisdiction in which the use of decentralised perpetual trading is prohibited
        or would require regulatory authorisation that has not been obtained. This list may be updated
        from time to time without notice.
      </p>
      <p>
        By using the Interface you represent and warrant that you meet these eligibility requirements.
        We reserve the right to restrict access to the Interface based on geographic or other criteria
        at any time in our sole discretion.
      </p>

      <h2>3. Wallet Keys and USDL Custody</h2>
      <p>
        The Interface does not hold your private keys, seed phrases or wallet credentials. You are
        responsible for their security. Depositing USDL transfers ERC20 collateral to the perps module
        account's EVM address, where it backs native account balances and trading obligations.
        Deposited collateral is no longer held in your wallet. Withdrawals depend on available margin,
        venue rules and successful chain execution; holding your keys does not guarantee immediate
        withdrawal of collateral committed to trading.
      </p>
      <p>
        Interface support cannot recover lost keys or reverse confirmed transactions. Native execution,
        governance and upgrade controls, and the separate USDL token contract can affect access to or
        movement of collateral. Do not treat the Interface's lack of key custody as a guarantee that
        deposited funds cannot be restricted or lost.
      </p>

      <h2>4. Risk Disclosures</h2>
      <p>
        Trading perpetual futures contracts involves <strong>substantial risk of loss</strong>. By
        using the Interface you acknowledge and accept the following risks:
      </p>
      <ul>
        <li>
          <strong>Leverage risk.</strong> Perpetual futures may be traded with significant leverage.
          Leveraged positions can result in losses that exceed your deposited margin. You may lose
          your entire deposited collateral.
        </li>
        <li>
          <strong>Liquidation risk.</strong> Positions may be automatically liquidated by the Protocol
          if your margin ratio falls below the maintenance threshold. Liquidation may occur rapidly
          in volatile market conditions.
        </li>
        <li>
          <strong>Execution and custody risk.</strong> Native venue software, precompile integration,
          upgrades and the separate USDL ERC20 contract may contain bugs or vulnerabilities or be
          exploited, resulting in total loss of funds. Governance changes or pauses can affect trading
          and withdrawals. No audit eliminates these risks.
        </li>
        <li>
          <strong>Oracle risk.</strong> The Protocol relies on oracle price feeds that may be
          manipulated, become stale, or diverge from true market prices, resulting in adverse fills
          or liquidations.
        </li>
        <li>
          <strong>Market risk.</strong> Digital asset markets are highly volatile and subject to
          sudden price movements, thin liquidity, and regulatory uncertainty.
        </li>
        <li>
          <strong>Regulatory risk.</strong> The regulatory treatment of decentralised protocols and
          digital assets is uncertain and subject to rapid change. Future regulation may restrict your
          ability to use the Protocol.
        </li>
        <li>
          <strong>Network risk.</strong> The Paxeer Network may experience downtime, congestion, or
          forks that prevent transaction processing or lead to unexpected outcomes.
        </li>
        <li>
          <strong>Frontend risk.</strong> The Interface may become unavailable, be subject to DNS
          attacks, or serve malicious content if its delivery infrastructure is compromised. Always
          verify the URL and independently verify transaction details and the native venue interface
          before critical operations.
        </li>
      </ul>
      <p>
        <strong>Nothing on this Interface constitutes financial, investment, legal, or tax advice.</strong>{" "}
        You should consult independent professional advisors before making any trading or investment
        decisions.
      </p>

      <h2>5. No Warranties</h2>
      <p>
        The Interface is provided on an <strong>"AS IS" and "AS AVAILABLE"</strong> basis without
        warranties of any kind, express or implied, including but not limited to warranties of
        merchantability, fitness for a particular purpose, accuracy, completeness, or non-infringement.
        We do not warrant that the Interface will be uninterrupted, error-free, or free of harmful
        components.
      </p>
      <p>
        We make no representations about the accuracy of pricing data, funding rate calculations,
        oracle values, or any other information displayed on the Interface. Market data is provided
        for informational purposes only.
      </p>

      <h2>6. Limitation of Liability</h2>
      <p>
        To the maximum extent permitted by applicable law, the Foundation and its officers, directors,
        employees, agents, and affiliates shall not be liable for:
      </p>
      <ul>
        <li>Any loss of funds resulting from trading activity, liquidation, or smart contract failure.</li>
        <li>Any indirect, incidental, special, consequential, or punitive damages.</li>
        <li>Loss of profits, revenue, data, goodwill, or business opportunities.</li>
        <li>Damages arising from reliance on information displayed on the Interface.</li>
        <li>Failures, delays, or errors in the blockchain, oracles, or third-party services.</li>
      </ul>
      <p>
        In jurisdictions that do not permit the exclusion or limitation of certain liabilities, our
        liability is limited to the maximum extent permitted by law.
      </p>

      <h2>7. Indemnification</h2>
      <p>
        You agree to defend, indemnify, and hold harmless the Foundation and its affiliates from any
        claims, damages, losses, liabilities, costs, or expenses (including reasonable legal fees)
        arising from or related to your use of the Interface, your violation of these Terms, your
        violation of applicable laws, or your infringement of third-party rights.
      </p>

      <h2>8. Intellectual Property</h2>
      <p>
        The Interface, including its design, code (to the extent not open-sourced), trademarks, and
        content, is the property of Paxeer Network Foundation and is protected by applicable
        intellectual property laws. The underlying native venue software and contract interfaces are
        open source and governed by their respective licences. Nothing in these Terms transfers any intellectual
        property rights to you.
      </p>

      <h2>9. Fees</h2>
      <p>
        The Protocol charges trading fees in accordance with its published fee schedule. The Foundation
        may receive a portion of Protocol fees as described in the tokenomics documentation. Network
        gas fees may also apply. All fees are disclosed within the Interface before you confirm a
        transaction. Fees are non-refundable once a transaction is confirmed on-chain.
      </p>

      <h2>10. Modifications and Termination</h2>
      <p>
        We may modify, suspend, or discontinue the Interface (or any part of it) at any time without
        notice. We may also update these Terms at any time. Continued use of the Interface after an
        update constitutes acceptance of the revised Terms. It is your responsibility to review these
        Terms regularly.
      </p>

      <h2>11. Governing Law and Disputes</h2>
      <p>
        These Terms are governed by and construed in accordance with the laws of the British Virgin
        Islands, without regard to conflict-of-law principles. Any dispute arising from or relating to
        these Terms or the Interface shall be resolved by binding arbitration administered by the BVI
        International Arbitration Centre, with proceedings conducted in English. You waive any right to
        a jury trial or to participate in a class action.
      </p>

      <h2>12. Entire Agreement and Severability</h2>
      <p>
        These Terms, together with the Privacy Policy and Cookie Policy, constitute the entire
        agreement between you and the Foundation regarding the Interface. If any provision of these
        Terms is found to be unenforceable, the remaining provisions shall continue in full force and
        effect. Our failure to enforce any provision shall not be deemed a waiver of our right to do
        so in the future.
      </p>

      <h2>13. Contact</h2>
      <p>
        Questions regarding these Terms may be directed to:{" "}
        <a href="mailto:legal@paxeer.network">legal@paxeer.network</a>.
      </p>
    </LegalLayout>
  );
}
