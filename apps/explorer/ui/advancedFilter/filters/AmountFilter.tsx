import { Flex, Text } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import { isEqual } from 'es-toolkit';
import type { ChangeEvent } from 'react';
import React from 'react';

import type { AdvancedFilterParams } from 'types/api/advancedFilter';

import { Button } from 'toolkit/chakra/button';
import { Field } from 'toolkit/chakra/field';
import { Input } from 'toolkit/chakra/input';
import { PopoverCloseTriggerWrapper } from 'toolkit/chakra/popover';
import { ndash } from 'toolkit/utils/htmlEntities';

import TableColumnFilter from '../ActivityFilter';

const FILTER_PARAM_FROM = 'amount_from';
const FILTER_PARAM_TO = 'amount_to';

const PRESETS = [
  {
    value: '10',
    name: '<10',
  },
  {
    value: '100',
    name: '<100',
  },
  {
    value: '1000',
    name: '<1K',
  },
  {
    value: '10000',
    name: '<10K',
  },
  {
    value: '100000',
    name: '<100K',
  },
  {
    value: '1000000',
    name: '<1M',
  },
];

const defaultValue = { from: '', to: '' };
type AmountValue = { from?: string; to?: string };

type Props = {
  value?: AmountValue;
  handleFilterChange: (filed: keyof AdvancedFilterParams, value?: string) => void;
};

const AmountFilter = ({ value = {}, handleFilterChange }: Props) => {
  const [ currentValue, setCurrentValue ] = React.useState<AmountValue>(value || defaultValue);

  const [ interacted, setInteracted ] = React.useState({ from: false, to: false });
  const from = currentValue.from ? BigNumber(currentValue.from) : undefined;
  const to = currentValue.to ? BigNumber(currentValue.to) : undefined;
  const fromError = from && (!from.isFinite() || from.isNegative()) ? 'Enter a non-negative amount' : undefined;
  const invalidToError = to && (!to.isFinite() || to.isNegative()) ? 'Enter a non-negative amount' : undefined;
  const rangeError = from && to && to.isLessThan(from) ? 'Maximum amount must be at least the minimum amount' : undefined;
  const toError = invalidToError || rangeError;
  const hasInvalidRange = Boolean(fromError || toError);

  const handleFromChange = React.useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setCurrentValue(prev => ({ ...prev, from: event.target.value }));
  }, []);

  const handleToChange = React.useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setCurrentValue(prev => ({ ...prev, to: event.target.value }));
  }, []);

  const onReset = React.useCallback(() => setCurrentValue(defaultValue), []);

  const onFilter = React.useCallback(() => {
    if (hasInvalidRange) return;
    handleFilterChange(FILTER_PARAM_FROM, currentValue.from);
    handleFilterChange(FILTER_PARAM_TO, currentValue.to);
  }, [ handleFilterChange, currentValue, hasInvalidRange ]);

  const onPresetChange = React.useCallback((to: string) => {
    setCurrentValue({ from: '', to });
    handleFilterChange(FILTER_PARAM_FROM, '');
    handleFilterChange(FILTER_PARAM_TO, to);
  }, [ handleFilterChange ]);

  const handlePresetClick = React.useCallback((event: React.MouseEvent<HTMLButtonElement>) => onPresetChange(event.currentTarget.value), [ onPresetChange ]);
  const handleFromBlur = React.useCallback(() => setInteracted(prev => ({ ...prev, from: true })), []);
  const handleToBlur = React.useCallback(() => setInteracted(prev => ({ ...prev, to: true })), []);

  return (
    <TableColumnFilter
      title="Amount"
      isFilled={ Boolean(currentValue.from || currentValue.to) }
      isTouched={ !isEqual(currentValue, value) && !hasInvalidRange }
      onFilter={ onFilter }
      onReset={ onReset }
      hasReset
    >
      <Flex gap={ 3 } flexWrap="wrap" minW={ 0 } role="group" aria-label="Amount presets">
        { PRESETS.map(preset => (
          <PopoverCloseTriggerWrapper key={ preset.value } aria-label={ preset.name }>
            <Button
              type="button"
              variant="outline"
              size="sm"
              selected={ (!currentValue.from || currentValue.from === '0') && currentValue.to === preset.value }
              aria-pressed={ (!currentValue.from || currentValue.from === '0') && currentValue.to === preset.value }
              value={ preset.value }
              onClick={ handlePresetClick }
            >
              { preset.name }
            </Button>
          </PopoverCloseTriggerWrapper>
        )) }
      </Flex>
      <Flex mt={ 3 } alignItems="center" flexWrap="wrap" gap={ 3 }>
        <Field
          label="From" size="sm" flexGrow={ 1 } flexBasis="var(--chakra-spacing-32)" minW={ 0 }
          invalid={ interacted.from && Boolean(fromError) } errorText={ interacted.from ? fromError : undefined }
        >
          <Input
            name="amount_from" value={ currentValue.from } onChange={ handleFromChange } placeholder="From" type="number" inputMode="decimal"
            min="0" step="any" onBlur={ handleFromBlur }
          />
        </Field>
        <Text aria-hidden>{ ndash }</Text>
        <Field
          label="To" size="sm" flexGrow={ 1 } flexBasis="var(--chakra-spacing-32)" minW={ 0 }
          invalid={ interacted.to && Boolean(toError) } errorText={ interacted.to ? toError : undefined }
        >
          <Input
            name="amount_to" value={ currentValue.to } onChange={ handleToChange } placeholder="To" type="number" inputMode="decimal"
            min={ currentValue.from || '0' } step="any" onBlur={ handleToBlur }
          />
        </Field>
      </Flex>
    </TableColumnFilter>
  );
};

export default AmountFilter;
