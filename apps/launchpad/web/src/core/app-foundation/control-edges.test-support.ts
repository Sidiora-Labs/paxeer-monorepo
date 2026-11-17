import { JSDOM } from 'jsdom';
import { expect } from 'vitest';

export function assertControlEdges(markup: string): string {
  const fragment = JSDOM.fragment(markup);
  const container = fragment.ownerDocument.createElement('div');
  container.appendChild(fragment);
  for (const element of container.querySelectorAll('[class]')) {
    element.classList.replace('border-[var(--sl-control-edge)]', 'border-control-edge');
    const edges = [...element.classList].filter(name => /^(?:hover:)?border(?:-|$)/.test(name) && name !== 'border-0' && name !== 'border-none');
    if (edges.length === 0) continue;
    expect(element.matches('button, a[href], input, select, textarea, [role="button"], [role="tab"], [role="checkbox"], [role="radio"], [role="switch"], [role="slider"]')).toBe(true);
    const segmented = element.matches('[role="radiogroup"] > button[role="radio"]');
    const expected = segmented
      ? ['border', element.getAttribute('aria-checked') === 'true' && !element.classList.contains('bg-accent') && !element.classList.contains('bg-danger') ? 'border-control-edge' : 'border-transparent']
      : element.classList.contains('border-transparent')
        ? ['border', 'border-transparent', 'hover:border-control-edge']
        : ['border', 'border-control-edge'];
    expect(edges.sort()).toEqual(expected.sort());
    element.classList.remove(...edges);
  }
  return container.innerHTML;
}
