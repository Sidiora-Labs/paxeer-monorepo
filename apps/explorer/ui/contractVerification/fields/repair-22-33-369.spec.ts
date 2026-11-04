// @vitest-environment jsdom

import { fireEvent, waitFor } from '@testing-library/react';
import React, { act } from 'react';
import { FormProvider, useForm } from 'react-hook-form';
import { decodeAbiParameters, encodeAbiParameters } from 'viem';

import type { FormFields } from '../types';

import { render } from 'ui/paxeerX/account/testWrapper';
import { expect, test } from 'vitest';

import { DEFAULT_VALUES, prepareRequestBody } from '../utils';
import ContractVerificationFieldAutodetectArgs from './ContractVerificationFieldAutodetectArgs';
import ContractVerificationFieldConstructorArgs from './ContractVerificationFieldConstructorArgs';

type Method = 'flattened-code' | 'standard-input' | 'vyper-code';
const METHODS: Array<Method> = [ 'flattened-code', 'standard-input', 'vyper-code' ];

async function openConstructorForm(method: Method) {
  const requests: Array<ReturnType<typeof prepareRequestBody>> = [];

  function VerificationForm() {
    const formApi = useForm<FormFields>({ defaultValues: DEFAULT_VALUES[method] });
    const [ outcome, setOutcome ] = React.useState('idle');
    const submit = formApi.handleSubmit((data) => {
      requests.push(prepareRequestBody(data));
      setOutcome('accepted');
    }, () => setOutcome('rejected'));

    return React.createElement(FormProvider<FormFields>, {
      ...formApi,
      children: React.createElement('form', {
        'aria-label': 'Contract verification',
        onSubmit: (event) => {
          setOutcome('pending');
          void submit(event);
        },
      },
      React.createElement(method === 'vyper-code' ? ContractVerificationFieldConstructorArgs : ContractVerificationFieldAutodetectArgs),
      React.createElement('output', { 'data-testid': 'submission-result' }, outcome)),
    });
  }

  const view = render(React.createElement(VerificationForm));
  if (method !== 'vyper-code') {
    const autodetect = view.getByRole('checkbox', { name: 'Try to fetch constructor arguments automatically' });
    expect((autodetect as HTMLInputElement).checked).toBe(true);
    await act(async() => {
      fireEvent.click(autodetect);
    });
    await waitFor(() => {
      expect((autodetect as HTMLInputElement).checked).toBe(false);
      expect(autodetect.closest('label')?.getAttribute('data-state')).toBe('unchecked');
    });
  }
  const input = await view.findByRole('textbox', { name: /ABI-encoded Constructor Arguments/ });

  return {
    view,
    input,
    requests,
    async submit(value: string, expected: 'accepted' | 'rejected') {
      const before = requests.length;
      fireEvent.change(input, { target: { value } });
      fireEvent.submit(view.getByRole('form', { name: 'Contract verification' }));
      await waitFor(() => expect(view.getByTestId('submission-result').textContent).toBe(expected));
      expect(requests).toHaveLength(before + (expected === 'accepted' ? 1 : 0));
      expect(input.getAttribute('aria-invalid')).toBe(expected === 'rejected' ? 'true' : null);
    },
    submittedArguments() {
      const body = requests[requests.length - 1];
      if (body instanceof FormData) {
        expect(body.get('autodetect_constructor_args')).toBe('false');
        return body.get('constructor_args');
      }
      expect(body).toBeTypeOf('object');
      expect(body).toHaveProperty('constructor_args');
      if (method === 'flattened-code') {
        expect(body).toHaveProperty('autodetect_constructor_args', false);
      }
      return (body as { constructor_args: string }).constructor_args;
    },
  };
}

async function constructorLengthAcceptsFourAbiWords() {
  const parameters = [ { type: 'uint256' }, { type: 'uint256' }, { type: 'uint256' }, { type: 'uint256' } ] as const;
  const values = [ BigInt(1), BigInt(2), BigInt(3), BigInt(255) ] as const;
  const encoded = encodeAbiParameters(parameters, values);
  expect(encoded.length).toBe(258);

  for (const method of METHODS) {
    const fixture = await openConstructorForm(method);
    try {
      for (const input of [ encoded, encoded.slice(2), `0x${ encoded.slice(2).toUpperCase() }` ]) {
        await fixture.submit(input, 'accepted');
        expect(fixture.submittedArguments()).toBe(input);
        const transported = fixture.submittedArguments() as string;
        expect(decodeAbiParameters(parameters, `0x${ transported.replace(/^0x/, '') }`)).toEqual(values);
      }
    } finally {
      fixture.view.unmount();
    }
  }
}

async function constructorLengthAcceptsDynamicAbiArguments() {
  const parameters = [ { type: 'string' }, { type: 'uint256[]' } ] as const;
  const values = [ 'Constructor payload with dynamic arguments. '.repeat(30), [ BigInt(1), BigInt(2), BigInt(3), BigInt(4), BigInt(5) ] ] as const;
  const encoded = encodeAbiParameters(parameters, values);
  expect(encoded.length).toBeGreaterThan(255);

  for (const method of METHODS) {
    const fixture = await openConstructorForm(method);
    try {
      await fixture.submit(encoded, 'accepted');
      expect(fixture.submittedArguments()).toBe(encoded);
      expect(decodeAbiParameters(parameters, fixture.submittedArguments() as `0x${ string }`)).toEqual(values);
    } finally {
      fixture.view.unmount();
    }
  }
}

async function constructorLengthAcceptsEmptyArgumentsWithAutodetectionDisabled() {
  const encoded = encodeAbiParameters([], []);
  expect(encoded).toBe('0x');

  for (const method of METHODS) {
    const fixture = await openConstructorForm(method);
    try {
      expect((fixture.input as HTMLTextAreaElement).required).toBe(false);
      for (const input of [ '', encoded ]) {
        await fixture.submit(input, 'accepted');
        expect(fixture.submittedArguments()).toBe(input);
        expect(decodeAbiParameters([], `0x${ input.replace(/^0x/, '') }`)).toEqual([]);
      }
    } finally {
      fixture.view.unmount();
    }
  }
}

async function constructorLengthRejectsMalformedHexBeforeSubmission() {
  const fixture = await openConstructorForm('flattened-code');
  try {
    for (const input of [ 'a', '0x123', '0xzz', '0x00xz', '0x0x', ' 00', '00 ', '00\n00', 'abc\n', '0x00\n', '-1', '０１' ]) {
      await fixture.submit(input, 'rejected');
      expect(fixture.view.getByText(/Enter an even number of hexadecimal characters/)).toBeDefined();
    }
    await fixture.submit('0x00aB', 'accepted');
    expect(fixture.submittedArguments()).toBe('0x00aB');
  } finally {
    fixture.view.unmount();
  }
}

async function constructorLengthUsesBackendPayloadBound() {
  // The smart-contract verification router sets Plug.Parsers length: 20_000_000.
  const backendPayloadLimit = 20_000_000;
  const fixture = await openConstructorForm('standard-input');
  try {
    const atLimit = '00'.repeat(backendPayloadLimit / 2);
    await fixture.submit(atLimit, 'accepted');
    expect(fixture.submittedArguments()).toBe(atLimit);
    await fixture.submit(`0x${ atLimit.slice(2) }`, 'accepted');
    expect(fixture.submittedArguments()).toHaveLength(backendPayloadLimit);
    for (const input of [ `${ atLimit }00`, `0x${ atLimit }` ]) {
      await fixture.submit(input, 'rejected');
      expect(fixture.view.getByText(/exceed the verification payload limit of 20,000,000 bytes/)).toBeDefined();
    }
    await fixture.submit('', 'accepted');
    expect(fixture.submittedArguments()).toBe('');
  } finally {
    fixture.view.unmount();
  }
}

test(constructorLengthAcceptsFourAbiWords, constructorLengthAcceptsFourAbiWords);
test(constructorLengthAcceptsDynamicAbiArguments, constructorLengthAcceptsDynamicAbiArguments);
test(constructorLengthAcceptsEmptyArgumentsWithAutodetectionDisabled, constructorLengthAcceptsEmptyArgumentsWithAutodetectionDisabled);
test(constructorLengthRejectsMalformedHexBeforeSubmission, constructorLengthRejectsMalformedHexBeforeSubmission);
test(constructorLengthUsesBackendPayloadBound, { timeout: 15000 }, constructorLengthUsesBackendPayloadBound);
