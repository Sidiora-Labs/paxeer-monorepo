/** Resolve a scheme-aware surface role for canvas and iframe renderers. */
export function surfaceColor(name: `--sl-${string}`): string | undefined {
  if (typeof document === 'undefined') return undefined;
  const root = document.documentElement;
  if (!getComputedStyle(root).getPropertyValue(name).trim()) return undefined;
  const probe = document.createElement('span');
  probe.style.color = `var(${name})`;
  probe.hidden = true;
  document.body.append(probe);
  try {
    return getComputedStyle(probe).color;
  } finally {
    probe.remove();
  }
}
