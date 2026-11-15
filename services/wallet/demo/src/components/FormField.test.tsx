import { act, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, test } from 'vitest';
import { FormField } from './FormField';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

function EmailForm() {
  const [email, setEmail] = useState('');
  return (
    <form onSubmit={(event) => event.preventDefault()}>
      <FormField
        label="Email"
        name="email"
        type="email"
        autoComplete="email"
        value={email}
        onValueChange={setEmail}
        errorMessage="Enter a valid email address."
      />
      <button type="submit">Send magic link</button>
    </form>
  );
}

async function emailValidationFollowsInteraction() {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => root.render(<EmailForm />));
    const input = container.querySelector('input')!;
    const label = container.querySelector('label')!;
    expect(label.htmlFor).toBe(input.id);
    expect(input.name).toBe('email');
    expect(input.autocomplete).toBe('email');
    expect(input.required).toBe(true);
    expect(input.getAttribute('aria-invalid')).toBe('false');
    expect(input.hasAttribute('aria-describedby')).toBe(false);
    expect(container.textContent).not.toContain('Enter a valid email address.');

    const setValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
    await act(async () => {
      setValue.call(input, 'invalid');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(input.getAttribute('aria-invalid')).toBe('false');
    await act(async () => input.dispatchEvent(new FocusEvent('focusout', { bubbles: true })));
    expect(input.getAttribute('aria-invalid')).toBe('true');
    const error = document.getElementById(input.getAttribute('aria-describedby')!);
    expect(error?.textContent).toBe('Enter a valid email address.');

    await act(async () => {
      setValue.call(input, 'wallet@example.com');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(input.validity.valid).toBe(true);
    expect(input.getAttribute('aria-invalid')).toBe('false');
    expect(input.hasAttribute('aria-describedby')).toBe(false);
    expect(container.textContent).not.toContain('Enter a valid email address.');
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
}

async function emptyEmailSubmissionIdentifiesTheField() {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => root.render(<EmailForm />));
    const input = container.querySelector('input')!;
    await act(async () => container.querySelector('button')!.click());
    expect(input.validity.valueMissing).toBe(true);
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect(document.getElementById(input.getAttribute('aria-describedby')!)?.textContent).toBe(
      'Enter a valid email address.'
    );
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
}

test(emailValidationFollowsInteraction);
test(emptyEmailSubmissionIdentifiesTheField);
