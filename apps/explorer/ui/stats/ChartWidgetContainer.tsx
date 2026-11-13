import { chakra } from '@chakra-ui/react';
import React, { useEffect } from 'react';

import { Resolution } from '@blockscout/stats-types';
import type { StatsIntervalIds } from 'types/client/stats';

import type { Route } from 'nextjs-routes';

import useChartQuery from 'ui/shared/chart/useChartQuery';

import ChartWidgetPanel from './ChartWidgetPanel';

type Props = {
  id: string;
  title: string;
  description: string;
  interval: StatsIntervalIds;
  onLoadingError: () => void;
  isPlaceholderData: boolean;
  className?: string;
  href?: Route;
};

export function formatChartValue(value: number, units?: string): string {
  const formatted = value.toLocaleString(undefined, { maximumFractionDigits: 4 });

  return units ? `${ formatted } ${ units }` : formatted;
}

const ChartWidgetContainer = ({
  id,
  title,
  description,
  interval,
  onLoadingError,
  isPlaceholderData,
  className,
  href,
}: Props) => {
  const { items, lineQuery } = useChartQuery(id, Resolution.DAY, interval, !isPlaceholderData);
  useEffect(() => {
    if (lineQuery.isError) {
      onLoadingError();
    }
  }, [ lineQuery.isError, onLoadingError ]);

  const units = lineQuery.data?.info?.units;
  const lastItem = items?.[items.length - 1];
  const currentValue = lastItem && Number.isFinite(lastItem.value) ? formatChartValue(lastItem.value, units) : undefined;

  return (
    <ChartWidgetPanel
      id={ id }
      title={ title }
      description={ description }
      interval={ interval }
      className={ className }
      href={ href }
      items={ lineQuery.data?.info ? items : undefined }
      units={ units }
      currentValue={ currentValue }
      isLoading={ lineQuery.isPlaceholderData }
      isError={ lineQuery.isError }
    />
  );
};

export default chakra(ChartWidgetContainer);
