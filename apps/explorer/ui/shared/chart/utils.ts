import type { TimeChartItem } from 'toolkit/components/charts/types';

export function formatDate(date: Date) {
  return date.toISOString().substring(0, 10);
}

export const sortByDateDesc = (a: Pick<TimeChartItem, 'date'>, b: Pick<TimeChartItem, 'date'>) => {
  return a.date.getTime() - b.date.getTime();
};

export type ChartColorRole = 'line' | 'areaStart' | 'areaStop' | 'bar' | 'axis' | 'grid';

export type ChartColorMode = 'light' | 'dark';

interface ChartColorToken {
  light: string;
  dark: string;
}

export const CHART_COLOR_TOKENS: Record<ChartColorRole, ChartColorToken> = {
  line: {
    light: 'accent',
    dark: 'accent',
  },
  areaStart: {
    light: 'chart.area.start',
    dark: 'chart.area.start',
  },
  areaStop: {
    light: 'chart.area.stop',
    dark: 'chart.area.stop',
  },
  bar: {
    light: 'accent',
    dark: 'accent',
  },
  axis: {
    light: 'text.secondary',
    dark: 'text.secondary',
  },
  grid: {
    light: 'chart.grid',
    dark: 'chart.grid',
  },
};

export function getChartColorToken(role: ChartColorRole, colorMode: ChartColorMode): string {
  return CHART_COLOR_TOKENS[role][colorMode];
}
