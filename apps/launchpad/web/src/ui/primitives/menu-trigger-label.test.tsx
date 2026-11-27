import { renderToStaticMarkup } from 'react-dom/server';
import { JSDOM } from 'jsdom';
import { expect, test } from 'vitest';
import { DropdownMenu, type DropdownMenuProps } from './overlays';

const sizes = ['sm', undefined] as const;

function renderMenuTrigger(props: Partial<DropdownMenuProps> = {}) {
  const markup = renderToStaticMarkup(
    <DropdownMenu label="More actions" items={[{ id: 'open', label: 'Open' }]} {...props} />
  );
  const button = JSDOM.fragment(markup).querySelector<HTMLButtonElement>(
    'button[aria-haspopup="menu"]'
  );
  expect(button).not.toBeNull();
  return button!;
}

function expectContentForm(button: HTMLButtonElement, size: 'sm' | 'md') {
  expect(button.classList).toContain(`min-h-control-height-${size}`);
  expect(button.classList).not.toContain('size-control-height-sm');
  expect(button.classList).not.toContain('size-control-height-md');
  expect(button.classList).toContain(size === 'sm' ? 'px-2' : 'px-3');
  expect(button.classList).toContain(size === 'sm' ? 'py-1' : 'py-2');
  expect(button.classList).toContain(size === 'sm' ? 'text-sm' : 'text-base');
  if (size === 'sm') {
    expect(button.classList).toContain('leading-[calc(var(--sl-leading-sm)/var(--sl-text-sm))]');
  }
}

function menu_trigger_with_a_label_and_no_body_is_not_a_square() {
  for (const size of sizes) {
    const button = renderMenuTrigger({ showLabel: true, size });
    expectContentForm(button, size ?? 'md');
    expect(button.textContent).toBe('More actions');
    expect(button.hasAttribute('aria-label')).toBe(false);
    expect(button.hasAttribute('title')).toBe(false);
  }
}

function menu_trigger_label_stands_on_one_line() {
  for (const size of sizes) {
    for (const trigger of [undefined, <span key="body">Details</span>]) {
      const button = renderMenuTrigger({ showLabel: true, size, trigger });
      const label = button.firstElementChild!;
      expect(label.textContent).toBe('More actions');
      expect(label.classList).toContain('whitespace-nowrap');
    }
  }
}

function menu_trigger_with_a_label_has_one_caret_after_it() {
  for (const size of sizes) {
    const button = renderMenuTrigger({ showLabel: true, size });
    expect(button.textContent).toBe('More actions');
    expect(button.children).toHaveLength(2);
    const label = button.firstElementChild!;
    const caret = button.querySelector('svg')!;
    expect(button.querySelectorAll('svg')).toHaveLength(1);
    expect(label.textContent).toBe('More actions');
    expect(label.nextElementSibling).toBe(caret);
    expect(caret.classList).toContain('lucide-chevron-down');
    expect(caret.classList).toContain('size-icon-sm');
    expect(caret.classList).not.toContain('size-icon-md');
    expect(caret.getAttribute('aria-hidden')).toBe('true');
  }
}

function menu_trigger_without_label_and_body_stays_a_square() {
  for (const size of sizes) {
    const button = renderMenuTrigger({ size });
    expect(button.classList).toContain(`size-control-height-${size ?? 'md'}`);
    expect(button.classList).not.toContain('min-h-control-height-sm');
    expect(button.classList).not.toContain('min-h-control-height-md');
    expect(button.textContent).toBe('');
    expect(button.getAttribute('aria-label')).toBe('More actions');
    expect(button.getAttribute('title')).toBe('More actions');
    expect(button.children).toHaveLength(1);
    const caret = button.querySelector('svg')!;
    expect(caret.classList).toContain('lucide-chevron-down');
    expect(caret.classList).toContain('size-icon-md');
    expect(caret.classList).not.toContain('size-icon-sm');
    expect(caret.getAttribute('aria-hidden')).toBe('true');
  }
}

function menu_trigger_with_a_body_keeps_its_form() {
  for (const size of sizes) {
    for (const showLabel of [true, false]) {
      const button = renderMenuTrigger({
        size,
        showLabel,
        trigger: <span data-menu-body>Details</span>,
        triggerClassName: 'text-text',
      });
      expectContentForm(button, size ?? 'md');
      expect(button.classList.contains('text-text-muted')).toBe(false);
      expect(button.className.endsWith('text-text')).toBe(true);
      const body = button.querySelector('[data-menu-body]')!;
      expect(body.textContent).toBe('Details');
      if (showLabel) {
        expect(button.hasAttribute('aria-label')).toBe(false);
        expect(button.hasAttribute('title')).toBe(false);
        expect(button.children).toHaveLength(3);
        const label = button.firstElementChild!;
        const caret = button.querySelector('svg')!;
        expect(label.textContent).toBe('More actions');
        expect(label.nextElementSibling).toBe(body);
        expect(body.nextElementSibling).toBe(caret);
        expect(button.querySelectorAll('svg')).toHaveLength(1);
        expect(caret.classList).toContain('lucide-chevron-down');
        expect(caret.classList).toContain('size-icon-sm');
        expect(caret.getAttribute('aria-hidden')).toBe('true');
      } else {
        expect(button.getAttribute('aria-label')).toBe('More actions');
        expect(button.getAttribute('title')).toBe('More actions');
        expect(button.children).toHaveLength(1);
        expect(button.firstElementChild).toBe(body);
        expect(button.querySelectorAll('svg')).toHaveLength(0);
      }
    }
  }
}

for (const check of [
  menu_trigger_with_a_label_and_no_body_is_not_a_square,
  menu_trigger_label_stands_on_one_line,
  menu_trigger_with_a_label_has_one_caret_after_it,
  menu_trigger_without_label_and_body_stays_a_square,
  menu_trigger_with_a_body_keeps_its_form,
]) test(check.name, check);
