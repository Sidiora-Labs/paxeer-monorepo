/** Resolve a surface length or duration for browser geometry and timers. */
export function surfaceToken(
  name: `--sl-${string}`,
  element: Element = document.documentElement
): number {
  const value = getComputedStyle(element).getPropertyValue(name).trim();
  const amount = Number.parseFloat(value);
  if (!Number.isFinite(amount)) throw new Error(`Surface token ${name} must be a numeric value`);
  if (value.endsWith('rem'))
    return amount * Number.parseFloat(getComputedStyle(document.documentElement).fontSize);
  if (value.endsWith('ms')) return amount;
  if (value.endsWith('s')) return amount * 1000;
  return amount;
}
