import { Flex, Text } from '@chakra-ui/react';
import { isEqual } from 'es-toolkit';
import type { ChangeEvent } from 'react';
import React from 'react';

import { ADVANCED_FILTER_AGES, type AdvancedFilterAge } from 'types/api/advancedFilter';

import dayjs from 'lib/date/dayjs';
import { Button } from 'toolkit/chakra/button';
import { Field } from 'toolkit/chakra/field';
import { Input } from 'toolkit/chakra/input';
import { PopoverCloseTriggerWrapper } from 'toolkit/chakra/popover';
import { ndash } from 'toolkit/utils/htmlEntities';

import TableColumnFilter from '../ActivityFilter';
import { getDurationFromAge } from '../lib';

const defaultValue = { age: '', from: '', to: '' } as const;
type AgeFromToValue = { age: AdvancedFilterAge | ''; from: string; to: string };

type DateConverter = {
  toFilterValue: (date: string) => string;
  fromFilterValue: (value: string | undefined) => string;
  getCurrentValue: (value: string | undefined) => string;
};

type Props<T> = {
  value?: AgeFromToValue;
  handleFilterChange: (field: keyof T, value?: string) => void;
  columnName: string;
  isLoading?: boolean;
  onClose?: () => void;
  fieldNames: {
    from: keyof T;
    to: keyof T;
    age: keyof T;
  };
  dateConverter: DateConverter;
};

const DateInput = ({ value, onChange, placeholder, max, min, error }: {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  max: string;
  min?: string;
  error?: string;
}) => {
  const [ interacted, setInteracted ] = React.useState(false);
  const handleChange = React.useCallback((event: ChangeEvent<HTMLInputElement>) => {
    onChange(event.target.value);
  }, [ onChange ]);

  const handleBlur = React.useCallback(() => setInteracted(true), []);

  return (
    <Field
      label={ placeholder }
      size="sm"
      minW={ 0 }
      flexGrow={ 1 }
      flexBasis="var(--chakra-spacing-40)"
      invalid={ interacted && Boolean(error) }
      errorText={ interacted ? error : undefined }
    >
      <Input
        name={ placeholder === 'From' ? 'age_from' : 'age_to' }
        value={ value }
        onChange={ handleChange }
        onBlur={ handleBlur }
        placeholder={ placeholder }
        type="date"
        max={ max }
        min={ min }
        size="sm"
      />
    </Field>
  );
};

function BaseAgeFilter<T>({
  value = defaultValue,
  handleFilterChange,
  onClose,
  fieldNames,
  dateConverter,
}: Props<T>) {
  const getFromValue = () => {
    if (value.age) return '';
    return value.from ? dateConverter.getCurrentValue(value.from) : '';
  };

  const getToValue = () => {
    if (value.age) return '';
    return value.to ? dateConverter.getCurrentValue(value.to) : '';
  };

  const [ currentValue, setCurrentValue ] = React.useState<AgeFromToValue>({
    age: value.age || '',
    from: getFromValue(),
    to: getToValue(),
  });

  const dateError = (date: string) => date && (!dayjs(date).isValid() || dayjs(date).isAfter(dayjs(), 'day')) ?
    'Enter a valid date on or before today' : undefined;
  const fromError = dateError(currentValue.from);
  const toError = dateError(currentValue.to) || (
    currentValue.from && currentValue.to && currentValue.to < currentValue.from ? 'End date must be on or after the start date' : undefined
  );
  const hasInvalidRange = Boolean(fromError || toError);

  const handleFromChange = React.useCallback((newValue: string) => {
    setCurrentValue(prev => ({ age: '', to: prev.to, from: newValue }));
  }, []);

  const handleToChange = React.useCallback((newValue: string) => {
    setCurrentValue(prev => ({ age: '', from: prev.from, to: newValue }));
  }, []);

  const onPresetChange = React.useCallback((age: AdvancedFilterAge) => {
    const from = dateConverter.toFilterValue(dayjs((dayjs().valueOf() - getDurationFromAge(age))).toISOString());
    handleFilterChange(fieldNames.from, from);
    const to = dateConverter.toFilterValue(dayjs().toISOString());
    handleFilterChange(fieldNames.to, to);
    handleFilterChange(fieldNames.age, age);
    onClose?.();
  }, [ handleFilterChange, onClose, fieldNames, dateConverter ]);

  const handlePresetClick = React.useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
    const age = ADVANCED_FILTER_AGES.find(age => age === event.currentTarget.value);
    if (age) onPresetChange(age);
  }, [ onPresetChange ]);

  const onReset = React.useCallback(() => {
    setCurrentValue(defaultValue);
  }, []);

  const onFilter = React.useCallback(() => {
    if (hasInvalidRange) return;
    if (!currentValue.age && !currentValue.to && !currentValue.from) {
      handleFilterChange(fieldNames.from, undefined);
      handleFilterChange(fieldNames.to, undefined);
      handleFilterChange(fieldNames.age, undefined);
      return;
    }

    if (currentValue.age) {
      // Age preset is selected, calculate timestamps
      const from = dateConverter.toFilterValue(dayjs((dayjs().valueOf() - getDurationFromAge(currentValue.age))).toISOString());
      const to = dateConverter.toFilterValue(dayjs().toISOString());
      handleFilterChange(fieldNames.from, from);
      handleFilterChange(fieldNames.to, to);
      handleFilterChange(fieldNames.age, currentValue.age);
    } else {
      // Custom date range is selected
      const from = currentValue.from ? dateConverter.toFilterValue(dayjs(currentValue.from).startOf('day').toISOString()) : undefined;
      const to = currentValue.to ? dateConverter.toFilterValue(dayjs(currentValue.to).endOf('day').toISOString()) : undefined;
      handleFilterChange(fieldNames.from, from);
      handleFilterChange(fieldNames.to, to);
      handleFilterChange(fieldNames.age, undefined);
    }
  }, [ handleFilterChange, currentValue, fieldNames, dateConverter, hasInvalidRange ]);

  // Check if the current values differ from the original values
  const isTouched = React.useMemo(() => {
    if (currentValue.age) {
      return value.age !== currentValue.age;
    }

    // If both current values are empty and both original values are empty, not touched
    if (!currentValue.from && !currentValue.to && !value.from && !value.to) {
      return false;
    }

    // Convert original values to date strings for comparison
    const originalValueAsDates = {
      age: '',
      from: value.from ? dateConverter.getCurrentValue(value.from) : '',
      to: value.to ? dateConverter.getCurrentValue(value.to) : '',
    };

    // Compare the date strings directly
    return !isEqual(currentValue, originalValueAsDates);
  }, [ currentValue, value, dateConverter ]);

  return (
    <TableColumnFilter
      title="Set last duration"
      isFilled={ Boolean(currentValue.from || currentValue.to || currentValue.age) }
      isTouched={ isTouched && !hasInvalidRange }
      onFilter={ onFilter }
      onReset={ onReset }
      hasReset
    >
      <Flex gap={ 3 } flexWrap="wrap" minW={ 0 } role="group" aria-label="Duration presets">
        { ADVANCED_FILTER_AGES.map(age => (
          <PopoverCloseTriggerWrapper key={ age } aria-label={ age }>
            <Button
              type="button"
              variant="outline"
              size="sm"
              selected={ currentValue.age === age }
              aria-pressed={ currentValue.age === age }
              value={ age }
              onClick={ handlePresetClick }
            >
              { age }
            </Button>
          </PopoverCloseTriggerWrapper>
        )) }
      </Flex>
      <Flex mt={ 3 } gap={ 3 } flexWrap="wrap" alignItems="center">
        <DateInput
          value={ currentValue.age ? '' : currentValue.from }
          onChange={ handleFromChange }
          placeholder="From"
          error={ fromError }
          max={ dayjs().format('YYYY-MM-DD') }
        />
        <Text aria-hidden>{ ndash }</Text>
        <DateInput
          value={ currentValue.age ? '' : currentValue.to }
          onChange={ handleToChange }
          placeholder="To"
          min={ currentValue.from || undefined }
          error={ toError }
          max={ dayjs().format('YYYY-MM-DD') }
        />
      </Flex>
    </TableColumnFilter>
  );
}

export default BaseAgeFilter;
export type { Props, DateConverter };
