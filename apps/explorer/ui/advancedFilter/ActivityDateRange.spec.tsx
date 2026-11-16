// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/react';
import React from 'react';
import { FormProvider, useForm } from 'react-hook-form';

import type { FormFields } from 'ui/csvExport/types';

import theme from 'toolkit/theme/theme';
import CsvExportFormField from 'ui/csvExport/CsvExportFormField';
import { afterEach, expect, test } from 'vitest';

afterEach(cleanup);

function DateRangeForm({ submissions }: { submissions: Array<FormFields> }) {
  const formApi = useForm<FormFields>({ mode: 'onBlur', defaultValues: { from: '', to: '' } });
  return (
    <ChakraProvider value={ theme }>
      <FormProvider { ...formApi }>
        <form noValidate aria-label="Export dates" onSubmit={ formApi.handleSubmit(values => {
          submissions.push(values);
        }) }>
          <CsvExportFormField name="from" formApi={ formApi }/>
          <CsvExportFormField name="to" formApi={ formApi }/>
          <button type="submit">Download</button>
        </form>
      </FormProvider>
    </ChakraProvider>
  );
}

test('exportDatesValidateAfterInteraction', async function exportDatesValidateAfterInteraction() {
  const submissions: Array<FormFields> = [];
  const view = render(<DateRangeForm submissions={ submissions }/>);
  const from = view.getByLabelText(/^From/);
  const to = view.getByLabelText(/^To/);

  expect(from.getAttribute('name')).toBe('from');
  expect(to.getAttribute('name')).toBe('to');
  expect(from.getAttribute('autocomplete')).toBe('on');
  expect(view.queryByText('Incorrect date')).toBeNull();

  fireEvent.change(from, { target: { value: '2099-01-02T12:00' } });
  expect(view.queryByText('Incorrect date')).toBeNull();
  fireEvent.blur(from);
  await waitFor(() => expect(from.getAttribute('aria-invalid')).toBe('true'));
  const errorId = from.getAttribute('aria-describedby');
  expect(errorId).toBeTruthy();
  expect(document.getElementById(errorId!)?.textContent).toBe('Incorrect date');

  fireEvent.change(from, { target: { value: '2020-01-02T12:00' } });
  fireEvent.blur(from);
  fireEvent.change(to, { target: { value: '2020-01-01T12:00' } });
  fireEvent.submit(view.getByRole('form', { name: 'Export dates' }));
  await waitFor(() => expect(to.getAttribute('aria-invalid')).toBe('true'));
  expect(submissions).toEqual([]);

  fireEvent.change(to, { target: { value: '2020-01-03T12:00' } });
  fireEvent.blur(to);
  await waitFor(() => expect(view.queryByText('Incorrect date')).toBeNull());
  fireEvent.submit(view.getByRole('form', { name: 'Export dates' }));
  await waitFor(() => expect(submissions).toEqual([ { from: '2020-01-02T12:00', to: '2020-01-03T12:00' } ]));
  expect(view.getByLabelText(/^From/)).toBe(from);
  expect(view.getByLabelText(/^To/)).toBe(to);
});

test('exportDatesRequireBothBoundsOnSubmit', async function exportDatesRequireBothBoundsOnSubmit() {
  const submissions: Array<FormFields> = [];
  const view = render(<DateRangeForm submissions={ submissions }/>);
  const from = view.getByLabelText(/^From/);
  const to = view.getByLabelText(/^To/);

  expect(from.getAttribute('aria-invalid')).not.toBe('true');
  expect(to.getAttribute('aria-invalid')).not.toBe('true');
  fireEvent.submit(view.getByRole('form', { name: 'Export dates' }));
  await waitFor(() => {
    expect(from.getAttribute('aria-invalid')).toBe('true');
    expect(to.getAttribute('aria-invalid')).toBe('true');
    expect(document.activeElement).toBe(from);
  });
  expect(submissions).toEqual([]);
});
