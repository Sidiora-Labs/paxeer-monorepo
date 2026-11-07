import React from 'react';

type SearchTarget = {
  ref: React.RefObject<HTMLElement | null>;
  activate: () => void;
};

const targets = new Set<SearchTarget>();

function isVisible(element: HTMLElement): boolean {
  for (let parent: HTMLElement | null = element; parent; parent = parent.parentElement) {
    const style = window.getComputedStyle(parent);
    if (parent.hidden || parent.inert || style.display === 'none' || style.visibility === 'hidden') {
      return false;
    }
  }
  const rect = element.getBoundingClientRect();
  return rect.bottom >= 0 && rect.top <= window.innerHeight && rect.right >= 0 && rect.left <= window.innerWidth;
}

function distanceFromFocus(element: HTMLElement): number {
  const active = document.activeElement;
  if (!active || active === document.body) {
    return 0;
  }
  let distance = 0;
  for (let ancestor: Element | null = active; ancestor; ancestor = ancestor.parentElement) {
    if (ancestor.contains(element)) {
      return distance;
    }
    distance++;
  }
  return distance;
}

function handleSearchShortcut(event: KeyboardEvent): void {
  if (event.defaultPrevented || event.repeat || event.isComposing || event.altKey) {
    return;
  }
  const isCommand = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k';
  const isSlash = event.key === '/' && !event.metaKey && !event.ctrlKey &&
    !(event.target instanceof Element && event.target.closest('input, textarea, select, [contenteditable="true"]'));
  if (!isCommand && !isSlash) {
    return;
  }
  const nearest = Array.from(targets)
    .filter(target => target.ref.current && isVisible(target.ref.current))
    .sort((a, b) => distanceFromFocus(a.ref.current!) - distanceFromFocus(b.ref.current!))[0];
  if (nearest) {
    event.preventDefault();
    nearest.activate();
  }
}

export default function useSearchShortcut(ref: React.RefObject<HTMLElement | null>, activate: () => void): void {
  React.useEffect(() => {
    const target = { ref, activate };
    targets.add(target);
    if (targets.size === 1) {
      window.addEventListener('keydown', handleSearchShortcut);
    }
    return () => {
      targets.delete(target);
      if (targets.size === 0) {
        window.removeEventListener('keydown', handleSearchShortcut);
      }
    };
  }, [ ref, activate ]);
}
