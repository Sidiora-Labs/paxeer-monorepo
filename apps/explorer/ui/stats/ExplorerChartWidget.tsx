import React from 'react';

import type { StatsIntervalIds } from 'types/client/stats';

import useApiQuery from 'lib/api/useApiQuery';

import { formatChartValue } from './ChartWidgetContainer';
import ChartWidgetPanel from './ChartWidgetPanel';
import { STATS_INTERVALS } from './constants';
import type { ExplorerChart } from './explorerCharts';
import { explorerChartValue } from './explorerCharts';

export default function ExplorerChartWidget({ chart, interval }: { chart: ExplorerChart; interval: StatsIntervalIds }) {
  const query = useApiQuery(chart.resource, { queryOptions: { refetchOnMount: false } });
  const points = React.useMemo(() => {
    const start = STATS_INTERVALS[interval].start;
    const from = start ? new Date(start.toISOString().slice(0, 10)).getTime() : -Infinity;
    const to = Date.now();
    return query.data?.chart_data
      .map((item) => ({ date: new Date(item.date), value: explorerChartValue(item, chart.field) }))
      .filter((item) => Number.isFinite(item.date.getTime()) && item.date.getTime() >= from && item.date.getTime() <= to)
      .sort((a, b) => a.date.getTime() - b.date.getTime());
  }, [ query.data, chart.field, interval ]);
  const items = React.useMemo(() => points?.flatMap((item) => item.value === undefined ? [] : [ { date: item.date, value: item.value } ]), [ points ]);
  const latest = points?.[points.length - 1]?.value;

  return (
    <ChartWidgetPanel
      id={ chart.id }
      title={ chart.title }
      description={ chart.description }
      interval={ interval }
      items={ items }
      units={ chart.units }
      currentValue={ latest === undefined ? undefined : formatChartValue(latest, chart.units) }
      isLoading={ query.isPending }
      isError={ query.isError }
      showMissingValue
      minPoints={ 2 }
    />
  );
}
