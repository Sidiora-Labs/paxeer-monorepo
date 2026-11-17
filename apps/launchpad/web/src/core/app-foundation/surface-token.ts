/** Resolve a surface length or duration for browser geometry and timers. */
export function surfaceToken(
  name: `--sl-${string}`,
  element: Element = document.documentElement
): number {
  const value = getComputedStyle(element).getPropertyValue(name).trim();
  if (CSS.supports('width', value) && /^(calc|min|max|clamp)\(/.test(value)) {
    const measure = document.createElement('span');
    measure.style.cssText = 'position:fixed;visibility:hidden;pointer-events:none;padding:0;border:0';
    measure.style.fontSize = getComputedStyle(element).fontSize;
    measure.style.width = value;
    document.body.append(measure);
    const length = Number.parseFloat(getComputedStyle(measure).width);
    measure.remove();
    if (!Number.isFinite(length)) throw new Error(`Surface token ${name} must resolve to a length`);
    return length;
  }
  const amount = Number.parseFloat(value);
  if (!Number.isFinite(amount)) throw new Error(`Surface token ${name} must be a numeric value`);
  if (value.endsWith('rem'))
    return amount * Number.parseFloat(getComputedStyle(document.documentElement).fontSize);
  if (value.endsWith('ms')) return amount;
  if (value.endsWith('s')) return amount * 1000;
  return amount;
}
