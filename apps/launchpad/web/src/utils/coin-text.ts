import { shortAddress } from './format';
import { COIN_TEXT_CONTROLS } from './coin-text-policy.mjs';

/** Preserve coin text except control and direction characters; identify blank labels by address. */
export function displayCoinText(value: string | null | undefined, tokenAddress: string | null | undefined): string {
  const display = (value ?? '').replace(new RegExp(COIN_TEXT_CONTROLS.source, 'gu'), '');
  return display.trim() ? display : shortAddress(tokenAddress);
}
