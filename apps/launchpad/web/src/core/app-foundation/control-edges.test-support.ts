import { JSDOM } from 'jsdom';
import { expect } from 'vitest';

export function assertControlEdges(markup: string): string {
  const fragment = JSDOM.fragment(markup);
  const container = fragment.ownerDocument.createElement('div');
  container.appendChild(fragment);
  for (const element of container.querySelectorAll('[class]')) {
    const edges = [...element.classList].filter(name => /^(?:hover:)?border(?:-|$)/.test(name) && name !== 'border-0' && name !== 'border-none');
    if (edges.length === 0) continue;
    expect(element.matches('button, a[href], input, select, textarea, [role="button"], [role="tab"], [role="checkbox"], [role="radio"], [role="switch"], [role="slider"]')).toBe(true);
    expect(edges.sort()).toEqual((element.classList.contains('border-transparent')
      ? ['border', 'border-transparent', 'hover:border-control-edge']
      : ['border', 'border-control-edge']).sort());
    element.classList.remove(...edges);
  }
  return container.innerHTML;
}
