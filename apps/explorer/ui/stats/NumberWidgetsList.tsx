import { chakra, Grid } from '@chakra-ui/react';
import React from 'react';

import type * as stats from '@blockscout/stats-types';

import useApiQuery from 'lib/api/useApiQuery';
import { STATS_COUNTER } from 'stubs/stats';
import type { ScanStatDeltaDirection } from 'ui/shared/scan';
import { ScanStatCard } from 'ui/shared/scan';

import DataFetchAlert from '../shared/DataFetchAlert';

const PERCENT_UNITS = '%';

const UNITS_WITHOUT_SPACE = [ 's', PERCENT_UNITS ];

export function formatCounterUnits(units?: string): string {
  if (!units) {
    return '';
  }

  return UNITS_WITHOUT_SPACE.includes(units) ? units : ' ' + units;
}

export function formatCounterValue(value: string): string {
  const valueNum = Number(value);
  const maximumFractionDigits = valueNum < 10 ** -3 ? undefined : 3;

  return valueNum.toLocaleString(undefined, { maximumFractionDigits, notation: 'compact' });
}

export type CounterValueSemantics = 'absolute' | 'change';

interface Props {
  counterSemantics?: Readonly<Record<string, CounterValueSemantics>>;
}

export function getCounterDeltaDirection(
  counter: stats.Counter,
  semantics: CounterValueSemantics = 'absolute',
): ScanStatDeltaDirection | undefined {
  if (semantics !== 'change') {
    return undefined;
  }

  const valueNum = Number(counter.value);

  if (!Number.isFinite(valueNum)) {
    return undefined;
  }

  return valueNum < 0 ? 'down' : 'up';
}

const NumberWidgetsList = ({ counterSemantics }: Props) => {
  const { data, isPlaceholderData, isError } = useApiQuery('stats:counters', {
    queryOptions: {
      placeholderData: { counters: Array(10).fill(STATS_COUNTER) },
    },
  });

  if (isError) {
    return <DataFetchAlert/>;
  }

  return (
    <Grid
      data-stats-number-widgets
      gridTemplateColumns={{ base: 'repeat(1, minmax(0, 1fr))', sm: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(4, minmax(0, 1fr))' }}
      gap={{ base: 2, lg: 3 }}
    >
      {
        data?.counters?.map((counter, index) => {
          const direction = getCounterDeltaDirection(counter, counterSemantics?.[counter.id]);
          const text = `${ formatCounterValue(counter.value) }${ formatCounterUnits(counter.units) }`;

          return (
            <ScanStatCard
              key={ counter.id + (isPlaceholderData ? index : '') }
              label={ counter.title }
              value={ direction ? <chakra.span data-delta={ direction }>{ text }</chakra.span> : text }
              hint={ counter.description }
              isLoading={ isPlaceholderData }
            />
          );
        })
      }
    </Grid>
  );
};

export default NumberWidgetsList;
