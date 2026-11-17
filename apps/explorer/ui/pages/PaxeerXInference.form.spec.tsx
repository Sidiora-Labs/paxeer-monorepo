// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import React from 'react';

import { inferenceQueryKey } from 'lib/api/services/general/paxeerXInference';
import theme from 'toolkit/theme/theme';
import { afterEach, expect, test } from 'vitest';

import PaxeerXInference from './PaxeerXInference';

afterEach(cleanup);

function inferenceDealValidationFollowsInteraction() {
  const client = new QueryClient();
  client.setQueryData(inferenceQueryKey('px_getInferenceRooms'), []);
  try {
    render(
      <ChakraProvider value={ theme }>
        <QueryClientProvider client={ client }><PaxeerXInference/></QueryClientProvider>
      </ChakraProvider>,
    );
    const input = screen.getByLabelText('Deal ID');
    const submit = screen.getByRole('button', { name: 'Look up deal' });
    expect(input.getAttribute('aria-invalid')).toBe('false');
    expect(screen.queryByRole('alert')).toBeNull();

    fireEvent.change(input, { target: { value: 'invalid' } });
    expect(screen.queryByRole('alert')).toBeNull();
    submit.focus();
    fireEvent.click(submit);
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect(document.activeElement).toBe(input);
    expect(screen.getByRole('alert').textContent).toBe('Enter a nonzero deal ID of 64 lowercase hexadecimal characters.');
    expect(input.getAttribute('aria-describedby')?.split(' ')).toContain(screen.getByRole('alert').id);
    expect(client.isFetching()).toBe(0);

    fireEvent.change(input, { target: { value: '01'.repeat(32) } });
    expect(input.getAttribute('aria-invalid')).toBe('false');
    expect(screen.queryByRole('alert')).toBeNull();
    expect(input.getAttribute('aria-describedby')).toBe('inference-deal-hint');
  } finally {
    client.clear();
  }
}

test(inferenceDealValidationFollowsInteraction);
