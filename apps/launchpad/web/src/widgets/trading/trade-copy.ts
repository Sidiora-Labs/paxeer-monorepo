/**
 * Plain-language copy for the buy/sell panel.
 *
 * The trade journey, provider and wallet report precise internal reasons (error codes,
 * viem short messages, recovery invariants). None of that is shown as is: each one is
 * turned into a short sentence here. The original stays available to the console.
 */

export const GENERIC_TRADE_ERROR = 'Something went wrong, try again';

const walletRefusalPattern = /user rejected|user denied|rejected the request|denied transaction|cancelled|canceled/i;

export function isWalletRefusal(error: unknown): boolean {
  const seen = new Set<unknown>();
  while (error && !seen.has(error)) {
    if (typeof error === 'string') return walletRefusalPattern.test(error);
    if (typeof error !== 'object') return false;
    seen.add(error);
    const detail = error as { code?: unknown; name?: unknown; shortMessage?: unknown; message?: unknown; cause?: unknown };
    if (detail.code === 4001 || detail.name === 'UserRejectedRequestError') return true;
    for (const field of ['shortMessage', 'message'] as const) {
      if (typeof detail[field] === 'string' && walletRefusalPattern.test(detail[field])) return true;
    }
    error = detail.cause;
  }
  return false;
}

/** Ordered: the first matching rule wins. */
const rules: ReadonlyArray<readonly [RegExp, string]> = [
  [/insufficient funds|exceeds balance|insufficient_balance|insufficient balance/i, 'Not enough balance for this trade'],
  [/network switch|switch.*paxeer|wrong network|chain mismatch|invalid_chain/i, "Your wallet didn't switch to Paxeer. Switch in your wallet and try again"],
  [/cooldown/i, 'This coin has a short cooldown between trades. Try again in a moment'],
  [/maxwallet|max(imum)? wallet/i, 'This trade would go over the wallet limit for this coin'],
  [/protection|anti-?snipe/i, 'Buying is limited right after launch. Try again soon'],
  [/vesting|cliff/i, 'These coins are still locked'],
  [/portfolio snapshot|share/i, 'Your PnL card will be ready once your trade settles'],
  [/connect wallet and refresh balances|fresh balances/i, 'Your balance is still loading. Try again in a moment'],
  [/unresolved|receipt recovery|resolve an existing attempt|200 unresolved/i, 'You have a trade still confirming. Wait for it to finish'],
  [/timed out|timeout|outcome (remains )?unknown/i, 'Taking longer than usual. Check again in a moment'],
  [/approval reverted|allowance/i, "Approval didn't go through. Try again"],
  [/reverted/i, "Your trade didn't go through. Try again"],
  [/quote|simulation|review again|context changed|stale|expired|wallet changed/i, 'The price moved. Refresh and try again'],
  [/deployment|authority|provider unavailable|attested/i, "Trading isn't available right now. Try again soon"],
  [/corrupt|storage|persistence|integrity|history/i, "Your recent trades couldn't be saved on this device"],
  [/balance/i, "Your balance can't load right now"],
];

/** One plain sentence for a trade-panel failure; '' stays ''. */
export function plainTradeError(text: string | null | undefined, fallback: string = GENERIC_TRADE_ERROR): string {
  if (!text) return '';
  if (isWalletRefusal(text)) return 'You cancelled in your wallet';
  for (const [pattern, copy] of rules) if (pattern.test(text)) return copy;
  return fallback;
}

/** Plain wording for a transaction attempt's state. */
export const attemptStateLabel: Record<string, string> = {
  submitted: 'sent',
  checking: 'confirming...',
  delayed: 'taking longer than usual',
  confirmed_success: 'confirmed',
  confirmed_reverted: "didn't go through",
  quarantined: "couldn't check",
  cancelled: 'cancelled',
  replaced: 'replaced',
  changed_action: 'changed in your wallet',
  replacement_validated: 'replaced',
};
