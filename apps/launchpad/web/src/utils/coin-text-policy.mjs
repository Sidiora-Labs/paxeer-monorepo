/** Control codes and Unicode direction controls are never shown in coin labels. */
export const COIN_TEXT_CONTROLS = /[\p{Cc}\p{Bidi_Control}\u206a-\u206f]/u;

export function hasCoinTextControls(value) {
  return COIN_TEXT_CONTROLS.test(value);
}

/** Authority bounds use the existing JavaScript string lengths. */
export function validCreationCoinText(name, symbol) {
  return typeof name === 'string' && name.length >= 1 && name.length <= 64 &&
    typeof symbol === 'string' && symbol.length >= 1 && symbol.length <= 16 &&
    !hasCoinTextControls(name) && !hasCoinTextControls(symbol);
}
