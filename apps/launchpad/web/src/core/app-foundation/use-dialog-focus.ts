'use client';
import { useEffect, type RefObject } from 'react';

/**
 * Background ownership shared by every open dialog. A dialog only ever adds
 * `inert` / `aria-hidden` where the element had none, and only removes what a
 * dialog added, once the last dialog holding that element closes. It never
 * writes a saved value back: some of these attributes are React-managed (the
 * private-session wrapper in query-provider toggles `inert` itself), and
 * restoring a stale snapshot after React has cleared it left the whole app
 * inert after the first-visit rules modal closed.
 */
const owners = new WeakMap<HTMLElement, { count: number; addedInert: boolean; addedHidden: boolean }>();

function claimBackground(element: HTMLElement) {
  const owner = owners.get(element);
  if (owner) { owner.count += 1; return; }
  const addedInert = !element.hasAttribute('inert');
  const addedHidden = !element.hasAttribute('aria-hidden');
  if (addedInert) element.setAttribute('inert', '');
  if (addedHidden) element.setAttribute('aria-hidden', 'true');
  owners.set(element, { count: 1, addedInert, addedHidden });
}

function releaseBackground(element: HTMLElement) {
  const owner = owners.get(element);
  if (!owner) return;
  owner.count -= 1;
  if (owner.count > 0) return;
  owners.delete(element);
  if (owner.addedInert) element.removeAttribute('inert');
  if (owner.addedHidden) element.removeAttribute('aria-hidden');
}
interface DialogEntry {
  dialog: HTMLElement;
  opener: HTMLElement | null;
  focus: () => void;
  background: HTMLElement[];
}
const dialogs: DialogEntry[] = [];
const topDialog = () => dialogs[dialogs.length - 1];

function updateBackground() {
  // Release the old branch before claiming the new one, even when roots differ.
  for (const entry of dialogs) {
    for (const element of entry.background) releaseBackground(element);
    entry.background = [];
  }
  const top = topDialog();
  if (!top) return;
  for (let branch: HTMLElement = top.dialog; branch.parentElement; branch = branch.parentElement) {
    for (const sibling of Array.from(branch.parentElement.children)) {
      if (sibling === branch || !(sibling instanceof HTMLElement)) continue;
      claimBackground(sibling);
      top.background.push(sibling);
    }
    if (branch.parentElement === document.body) break;
  }
}

/** Scope keyboard focus to the topmost mounted dialog; cleanup returns its opener. */
export function useDialogFocus(ref: RefObject<HTMLElement | null>, open: boolean, restoreFocus?: () => boolean) {
  useEffect(() => {
    if (!open || !ref.current) return;
    const dialog = ref.current;
    const controls = () => Array.from(dialog.querySelectorAll<HTMLElement>('button, a[href], input, select, textarea, [tabindex]')).filter(item => {
      if (item.matches(':disabled, [tabindex="-1"]') || item.closest('[hidden], [inert], [aria-hidden="true"]')) return false;
      for (let node: HTMLElement | null = item; node; node = node.parentElement) {
        const style = getComputedStyle(node);
        if (style.display === 'none' || style.visibility === 'hidden') return false;
        if (node === dialog) break;
      }
      return item.tabIndex >= 0;
    });
    const previousTabIndex = dialog.getAttribute('tabindex');
    const entry: DialogEntry = {
      dialog, opener: document.activeElement instanceof HTMLElement ? document.activeElement : null,
      focus: () => {
        const first = controls()[0];
        if (!first && !dialog.hasAttribute('tabindex')) dialog.tabIndex = -1;
        (first || dialog).focus();
      },
      background: [],
    };
    dialogs.push(entry);
    updateBackground();
    entry.focus();
    const trap = (event: KeyboardEvent) => {
      if (topDialog() !== entry || event.key !== 'Tab') return;
      const items = controls(); const first = items[0]; const last = items[items.length - 1];
      if (!first) { event.preventDefault(); dialog.focus(); return; }
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    const containFocus = (event: FocusEvent) => {
      if (topDialog() === entry && event.target instanceof Node && !dialog.contains(event.target)) entry.focus();
    };
    dialog.addEventListener('keydown', trap);
    document.addEventListener('focusin', containFocus);
    return () => {
      const wasTop = topDialog() === entry;
      dialog.removeEventListener('keydown', trap);
      document.removeEventListener('focusin', containFocus);
      if (previousTabIndex === null) dialog.removeAttribute('tabindex'); else dialog.setAttribute('tabindex', previousTabIndex);
      for (const element of entry.background) releaseBackground(element);
      dialogs.splice(dialogs.indexOf(entry), 1);
      // If a lower dialog closes first, its eventual successor must still reach the page opener.
      for (const next of dialogs) if (next.opener && dialog.contains(next.opener)) next.opener = entry.opener;
      updateBackground();
      if (!wasTop) return;
      const top = topDialog();
      if (entry.opener?.isConnected && (!top || top.dialog.contains(entry.opener)) && (!restoreFocus || restoreFocus())) entry.opener.focus();
      else if (top) top.focus();
    };
  }, [open, ref, restoreFocus]);
}
