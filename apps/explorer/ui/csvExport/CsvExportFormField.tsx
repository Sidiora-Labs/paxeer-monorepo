import { capitalize } from 'es-toolkit';
import React from 'react';
import type { UseFormReturn } from 'react-hook-form';

import type { FormFields } from './types';

import dayjs from 'lib/date/dayjs';
import { FormFieldText } from 'toolkit/components/forms/fields/FormFieldText';

interface Props {
  formApi: UseFormReturn<FormFields>;
  name: 'from' | 'to';
}

const CsvExportFormField = ({ formApi, name }: Props) => {
  const { getFieldState, getValues, trigger } = formApi;
  const dependentField = name === 'from' ? 'to' : 'from';

  const validate = React.useCallback((newValue: string) => {
    const date = dayjs(newValue);
    if (!date.isValid() || date > dayjs()) {
      return 'Incorrect date';
    }
    if (name === 'from') {
      const toValue = getValues('to');
      if (toValue && dayjs(newValue) > dayjs(toValue)) {
        return 'Incorrect date';
      }
    } else {
      const fromValue = getValues('from');
      if (fromValue && dayjs(fromValue) > dayjs(newValue)) {
        return 'Incorrect date';
      }
    }
  }, [ getValues, name ]);

  const handleBlur = React.useCallback(() => {
    const { isTouched, invalid } = getFieldState(dependentField);
    if (isTouched || invalid) {
      void trigger(dependentField);
    }
  }, [ dependentField, getFieldState, trigger ]);

  return (
    <FormFieldText<FormFields, typeof name>
      name={ name }
      inputProps={{ type: 'datetime-local', max: dayjs().format('YYYY-MM-DDTHH:mm'), autoComplete: 'on', minH: 'touchTarget' }}
      placeholder={ capitalize(name) }
      label={ capitalize(name) }
      floating={ false }
      required
      onBlur={ handleBlur }
      rules={{ validate }}
      maxW="full"
      minW={ 0 }
      flexGrow={ 1 }
      flexBasis="var(--chakra-spacing-56)"
    />
  );
};

export default React.memo(CsvExportFormField);
