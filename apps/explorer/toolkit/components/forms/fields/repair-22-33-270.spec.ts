// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { ResizeObserver } from '@juggle/resize-observer';
import 'intersection-observer';
import React from 'react';
import type { FieldErrors, RegisterOptions } from 'react-hook-form';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { afterEach, test, expect } from 'vitest';
import { act, cleanup, fireEvent, render, waitFor } from 'vitest/lib';

import theme from '../../../theme/theme';
import { FormFieldColor } from './FormFieldColor';
import { FormFieldRadio } from './FormFieldRadio';

globalThis.ResizeObserver = ResizeObserver;
globalThis.IntersectionObserver = window.IntersectionObserver;

function escapeCssIdentifier(value: string): string {
  const characters = Array.from(value);
  return characters.map((character, index) => {
    const code = character.charCodeAt(0);
    if (code === 0) return '\uFFFD';
    if (
      (code >= 1 && code <= 31) || code === 127 ||
      (index === 0 && code >= 48 && code <= 57) ||
      (index === 1 && code >= 48 && code <= 57 && characters[0] === '-')
    ) {
      return `\\${ code.toString(16) } `;
    }
    if (index === 0 && character === '-' && characters.length === 1) return '\\-';
    if (
      code >= 128 || character === '-' || character === '_' ||
      (code >= 48 && code <= 57) || (code >= 65 && code <= 90) || (code >= 97 && code <= 122)
    ) {
      return character;
    }
    return `\\${ character }`;
  }).join('');
}

for (const target of [ globalThis, window ]) {
  if (!target.CSS) {
    Object.defineProperty(target, 'CSS', {
      configurable: true,
      value: { escape: escapeCssIdentifier },
    });
  } else if (!target.CSS.escape) {
    target.CSS.escape = escapeCssIdentifier;
  }
}

afterEach(cleanup);

function inputElement(element: HTMLElement): HTMLInputElement {
  if (!(element instanceof HTMLInputElement)) {
    throw new TypeError('Expected a real input element');
  }
  return element;
}

interface ColorValues {
  color: string;
}

interface ColorFormProps {
  initialColor?: string;
  controllerDefault?: string;
  submissions: Array<ColorValues>;
  refusals: Array<FieldErrors<ColorValues>>;
}

function ColorForm({ initialColor, controllerDefault, submissions, refusals }: ColorFormProps) {
  const methods = useForm<ColorValues>({
    defaultValues: initialColor === undefined ? {} : { color: initialColor },
  });
  const color = useWatch({ control: methods.control, name: 'color' });

  return React.createElement(ChakraProvider, {
    value: theme,
    children: React.createElement(FormProvider<ColorValues>, {
      ...methods,
      children: React.createElement('form', {
        'aria-label': 'Color form',
        noValidate: true,
        onSubmit: methods.handleSubmit(
          (values) => {
            submissions.push({ ...values });
          },
          (errors) => {
            refusals.push(errors);
          },
        ),
      },
      React.createElement(FormFieldColor<ColorValues>, {
        name: 'color',
        placeholder: 'Color',
        sampleDefaultBgColor: '#eeeeee',
        controllerProps: { defaultValue: controllerDefault },
        group: { endElementProps: { id: 'color-swatch' } },
      }),
      React.createElement('output', { 'data-testid': 'form-color' }, color),
      React.createElement('button', { type: 'button', onClick: () => methods.reset({ color: '#102030' }) }, 'Load color'),
      React.createElement('button', { type: 'button', onClick: () => methods.reset({ color: '' }) }, 'Clear color'),
      React.createElement('button', { type: 'button', onClick: () => methods.reset() }, 'Reset color'),
      React.createElement('button', { type: 'submit' }, 'Save color')),
    }),
  });
}

function expectSwatchColor(container: HTMLElement, color: string) {
  const swatch = container.querySelector('#color-swatch > div');
  expect(swatch).not.toBeNull();
  const backgrounds: Array<string> = [];

  function visit(rules: CSSRuleList) {
    for (const rule of Array.from(rules)) {
      if (rule.type === CSSRule.STYLE_RULE) {
        const styleRule = rule as CSSStyleRule;
        const selector = styleRule.selectorText;
        if (!selector.includes('::') && !selector.includes(':-') && swatch?.matches(selector)) {
          const background = styleRule.style.getPropertyValue('background-color');
          if (background) {
            const declaration = document.createElement('div');
            declaration.style.backgroundColor = background;
            backgrounds.push(declaration.style.backgroundColor);
          }
        }
      }
      if ('cssRules' in rule) visit((rule as CSSGroupingRule).cssRules);
    }
  }

  for (const sheet of Array.from(document.styleSheets)) visit(sheet.cssRules);
  const expected = document.createElement('div');
  expected.style.backgroundColor = color;
  expect(backgrounds).toContain(expected.style.backgroundColor);
}

test('colorFieldKeepsExistingEditsAndResetsAligned', async function colorFieldKeepsExistingEditsAndResetsAligned() {
  const submissions: Array<ColorValues> = [];
  const refusals: Array<FieldErrors<ColorValues>> = [];
  const view = render(React.createElement(ColorForm, { initialColor: '#123456', submissions, refusals }));
  const input = inputElement(view.getByRole('textbox', { name: 'Color' }));
  const form = view.getByRole('form', { name: 'Color form' });

  function expectColor(value: string) {
    expect(input.value).toBe(value);
    expect(view.getByTestId('form-color').textContent).toBe(value);
    expectSwatchColor(view.container, value);
  }

  expectColor('#123456');
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions).toEqual([ { color: '#123456' } ]);

  await act(async() => {
    fireEvent.change(input, { target: { value: '#abcdef' } });
  });
  expectColor('#abcdef');
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions).toEqual([ { color: '#123456' }, { color: '#abcdef' } ]);

  await act(async() => {
    fireEvent.click(view.getByRole('button', { name: 'Load color' }));
  });
  expectColor('#102030');
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions).toEqual([ { color: '#123456' }, { color: '#abcdef' }, { color: '#102030' } ]);

  await act(async() => {
    fireEvent.change(input, { target: { value: '#ggg' } });
  });
  expect(input.value).toBe('#ggg');
  expect(view.getByTestId('form-color').textContent).toBe('#ggg');
  expectSwatchColor(view.container, '#eeeeee');
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions).toHaveLength(3);
  expect(refusals).toHaveLength(1);
  expect(refusals[0].color?.message).toBe('Invalid hex code');
  expect(view.getByText(/Invalid hex code/)).toBeTruthy();

  await act(async() => {
    fireEvent.click(view.getByRole('button', { name: 'Reset color' }));
  });
  expectColor('#102030');
  expect(view.queryByText(/Invalid hex code/)).toBeNull();
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions[3]).toEqual({ color: '#102030' });
});

test('colorFieldUsesControllerDefaultAndResetsToEmpty', async function colorFieldUsesControllerDefaultAndResetsToEmpty() {
  const submissions: Array<ColorValues> = [];
  const refusals: Array<FieldErrors<ColorValues>> = [];
  const view = render(React.createElement(ColorForm, { controllerDefault: '#aabbcc', submissions, refusals }));
  const input = inputElement(view.getByRole('textbox', { name: 'Color' }));
  const form = view.getByRole('form', { name: 'Color form' });

  expect(input.value).toBe('#aabbcc');
  expectSwatchColor(view.container, '#aabbcc');
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions).toEqual([ { color: '#aabbcc' } ]);

  await act(async() => {
    fireEvent.click(view.getByRole('button', { name: 'Clear color' }));
  });
  expect(input.value).toBe('');
  expect(view.getByTestId('form-color').textContent).toBe('');
  expectSwatchColor(view.container, '#eeeeee');

  await act(async() => {
    fireEvent.change(input, { target: { value: 'f' } });
  });
  expect(input.value).toBe('#f');
  expect(view.getByTestId('form-color').textContent).toBe('#f');
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(refusals[0].color?.message).toBe('Invalid length');
  expect(submissions).toHaveLength(1);

  await act(async() => {
    fireEvent.click(view.getByRole('button', { name: 'Reset color' }));
  });
  expect(input.value).toBe('');
  expect(view.getByTestId('form-color').textContent).toBe('');
  expectSwatchColor(view.container, '#eeeeee');
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions).toEqual([ { color: '#aabbcc' }, { color: '' } ]);
  expect(refusals).toHaveLength(1);
});

interface RadioValues {
  choice: string;
}

interface RadioFormProps {
  initialChoice: string;
  rules: RegisterOptions<RadioValues, 'choice'>;
  submissions: Array<RadioValues>;
  refusals: Array<FieldErrors<RadioValues>>;
  changes: Array<string | null>;
}

function RadioForm({ initialChoice, rules, submissions, refusals, changes }: RadioFormProps) {
  const methods = useForm<RadioValues>({ defaultValues: { choice: initialChoice }, shouldFocusError: false });
  const choice = useWatch({ control: methods.control, name: 'choice' });

  return React.createElement(ChakraProvider, {
    value: theme,
    children: React.createElement(FormProvider<RadioValues>, {
      ...methods,
      children: React.createElement('form', {
        'aria-label': 'Radio form',
        noValidate: true,
        onSubmit: methods.handleSubmit(
          (values) => {
            submissions.push({ ...values });
          },
          (errors) => {
            refusals.push(errors);
          },
        ),
      },
      React.createElement(FormFieldRadio<RadioValues, 'choice'>, {
        name: 'choice',
        'aria-label': 'Choice',
        orientation: 'horizontal',
        rules,
        options: [ { value: 'blocked', label: 'Blocked' }, { value: 'allowed', label: 'Allowed' } ],
        onValueChange: ({ value }) => {
          changes.push(value);
        },
      }),
      React.createElement('output', { 'data-testid': 'form-choice' }, choice),
      React.createElement('output', { 'data-testid': 'choice-error' }, methods.formState.errors.choice?.message),
      React.createElement('button', { type: 'button', onClick: () => methods.reset() }, 'Reset choice'),
      React.createElement('button', { type: 'submit' }, 'Save choice')),
    }),
  });
}

test('radioFieldRequiredRuleRefusesSubmission', async function radioFieldRequiredRuleRefusesSubmission() {
  const submissions: Array<RadioValues> = [];
  const refusals: Array<FieldErrors<RadioValues>> = [];
  const changes: Array<string | null> = [];
  const view = render(React.createElement(RadioForm, {
    initialChoice: '', rules: { required: 'Choose an option' }, submissions, refusals, changes,
  }));
  const form = view.getByRole('form', { name: 'Radio form' });
  const group = view.getByRole('radiogroup', { name: 'Choice' });

  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions).toEqual([]);
  expect(refusals).toHaveLength(1);
  expect(refusals[0].choice?.type).toBe('required');
  expect(view.getByTestId('choice-error').textContent).toBe('Choose an option');
  expect(group.hasAttribute('rules')).toBe(false);

  await act(async() => {
    fireEvent.click(view.getByRole('radio', { name: 'Allowed' }));
  });
  await waitFor(() => {
    expect(inputElement(view.getByRole('radio', { name: 'Allowed' })).checked).toBe(true);
  });
  expect(view.getByTestId('form-choice').textContent).toBe('allowed');
  expect(changes).toEqual([ 'allowed' ]);
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(submissions).toEqual([ { choice: 'allowed' } ]);
  expect(view.getByTestId('choice-error').textContent).toBe('');
  expect(refusals).toHaveLength(1);
});

test('radioFieldCustomValidatorRefusesSubmission', async function radioFieldCustomValidatorRefusesSubmission() {
  const submissions: Array<RadioValues> = [];
  const refusals: Array<FieldErrors<RadioValues>> = [];
  const changes: Array<string | null> = [];
  const validated: Array<string> = [];
  const view = render(React.createElement(RadioForm, {
    initialChoice: 'blocked',
    rules: {
      validate: (choice) => {
        validated.push(choice);
        return choice === 'allowed' || 'This option is unavailable';
      },
    },
    submissions, refusals, changes,
  }));
  const form = view.getByRole('form', { name: 'Radio form' });

  expect(inputElement(view.getByRole('radio', { name: 'Blocked' })).checked).toBe(true);
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(validated).toEqual([ 'blocked' ]);
  expect(submissions).toEqual([]);
  expect(refusals).toHaveLength(1);
  expect(refusals[0].choice?.type).toBe('validate');
  expect(view.getByTestId('choice-error').textContent).toBe('This option is unavailable');

  await act(async() => {
    fireEvent.click(view.getByRole('radio', { name: 'Allowed' }));
  });
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(validated).toContain('allowed');
  expect(submissions).toEqual([ { choice: 'allowed' } ]);
  expect(changes).toEqual([ 'allowed' ]);
  expect(view.getByTestId('form-choice').textContent).toBe('allowed');
  expect(view.getByTestId('choice-error').textContent).toBe('');

  await act(async() => {
    fireEvent.click(view.getByRole('button', { name: 'Reset choice' }));
  });
  expect(inputElement(view.getByRole('radio', { name: 'Blocked' })).checked).toBe(true);
  expect(view.getByTestId('form-choice').textContent).toBe('blocked');
  await act(async() => {
    fireEvent.submit(form);
  });
  expect(inputElement(view.getByRole('radio', { name: 'Blocked' })).checked).toBe(true);
  expect(view.getByTestId('form-choice').textContent).toBe('blocked');
  expect(submissions).toEqual([ { choice: 'allowed' } ]);
  expect(refusals).toHaveLength(2);
  expect(refusals[1].choice?.message).toBe('This option is unavailable');
});
