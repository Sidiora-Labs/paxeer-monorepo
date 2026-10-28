import { chooseWalletFee, sidioraAmount, walletFeePreference, type WalletFeePreference } from "../../api/wallet.ts";
import type { NotificationSummary } from "../../api/index.ts";

export function newestWalletSecurityNotification(
  notifications: readonly NotificationSummary[],
): NotificationSummary | undefined {
  return notifications
    .filter((notification) =>
      notification.class === "security-wallet-rebinding"
      && notification.deep_link === "/app/settings/wallet"
      && notification.action_copy_key !== undefined)
    .sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at))[0];
}

export interface WalletFeeSnapshot {
  readonly currency: WalletFeePreference["currency"];
  readonly maximum: string;
  readonly valid: boolean;
}

export class WalletFeeController {
  #snapshot: WalletFeeSnapshot;

  constructor() {
    const preference = walletFeePreference();
    this.#snapshot = Object.freeze({
      currency: preference.currency,
      maximum: preference.maximum === undefined ? "" : sidioraAmount(preference.maximum),
      valid: preference.currency === "paxeer" || preference.maximum !== undefined,
    });
  }

  get snapshot(): WalletFeeSnapshot {
    return this.#snapshot;
  }

  choose(currency: WalletFeePreference["currency"], maximum: string): WalletFeeSnapshot {
    const valid = chooseWalletFee(currency, maximum);
    this.#snapshot = Object.freeze({ currency, maximum, valid });
    return this.#snapshot;
  }
}
