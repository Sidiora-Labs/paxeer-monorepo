import { Box, chakra, Flex } from '@chakra-ui/react';
import React from 'react';

import type { StatsIntervalIds } from 'types/client/stats';

import { route, type Route } from 'nextjs-routes';

import { IconButton } from 'toolkit/chakra/icon-button';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Tooltip } from 'toolkit/chakra/tooltip';
import type { TimeChartItem } from 'toolkit/components/charts';
import { ChartWidgetContent, useChartZoom } from 'toolkit/components/charts';
import { Hint } from 'toolkit/components/Hint/Hint';
import { useChartsConfig, useDefaultBarColor } from 'ui/shared/chart/config';
import IconSvg from 'ui/shared/IconSvg';
import { STATS_INTERVALS } from 'ui/stats/constants';

type Props = {
  id: string;
  title: string;
  description: string;
  interval: StatsIntervalIds;
  items?: Array<TimeChartItem>;
  units?: string;
  currentValue?: string;
  isLoading: boolean;
  isError: boolean;
  showMissingValue?: boolean;
  minPoints?: number;
  className?: string;
  href?: Route;
};

const ChartWidgetPanel = ({
  id,
  title,
  description,
  interval,
  items,
  units,
  currentValue,
  isLoading,
  isError,
  showMissingValue,
  minPoints = 3,
  className,
  href,
}: Props) => {
  const chartsConfig = useChartsConfig();
  const { zoomRange, handleZoom, handleZoomReset } = useChartZoom();
  const barColor = useDefaultBarColor();

  const charts = React.useMemo(() => {
    if (!items) {
      return [];
    }

    return [
      {
        id,
        name: 'Value',
        items,
        charts: chartsConfig,
        units,
      },
    ];
  }, [ id, items, chartsConfig, units ]);

  const hasNonEmptyCharts = charts.some((chart) => chart.items.length >= minPoints);

  return (
    <Box
      className={ className }
      data-chart-card={ id }
      display="flex"
      flexDirection="column"
      bg="surface"
      borderWidth="1px"
      borderStyle="solid"
      borderColor="hairline"
      borderRadius="card"
      boxShadow="card"
      p={ 4 }
      minH="300px"
    >
      <Flex data-chart-card-header alignItems="flex-start" justifyContent="space-between" columnGap={ 3 }>
        <Flex alignItems="center" columnGap={ 1.5 } minW={ 0 }>
          <Skeleton loading={ isLoading } textStyle="label" color="text.muted">
            <chakra.span data-chart-title>
              { title } ({ STATS_INTERVALS[interval].shortTitle })
            </chakra.span>
          </Skeleton>
          { description && <Hint label={ description } isLoading={ isLoading } boxSize={ 4 }/> }
        </Flex>
        <Flex alignItems="center" columnGap={ 2 } flexShrink={ 0 }>
          { zoomRange && (
            <Tooltip content="Reset zoom">
              <IconButton
                aria-label="Reset zoom"
                size="md"
                variant="icon_background"
                onClick={ handleZoomReset }
              >
                <IconSvg name="repeat" boxSize={ 5 }/>
              </IconButton>
            </Tooltip>
          ) }
          { href && (
            <Link href={ route(href) } data-chart-view textStyle="xs" fontWeight="500" display="inline-flex" alignItems="center" columnGap={ 1 }>
              View
              <IconSvg name="arrows/east-mini" boxSize={ 3 }/>
            </Link>
          ) }
        </Flex>
      </Flex>

      { (currentValue !== undefined || showMissingValue) && (
        <Skeleton loading={ isLoading } mt={ 1 } textStyle="statValue" fontWeight="600" color="text.primary" w="fit-content">
          <chakra.span data-chart-value color={ currentValue === undefined ? 'text.muted' : undefined }>{ currentValue ?? '—' }</chakra.span>
        </Skeleton>
      ) }

      <Box
        data-chart-surface
        display="flex"
        flexDirection="column"
        flexGrow={ 1 }
        mt={ 3 }
        minH="230px"
        css={{
          '& svg text': { color: 'text.secondary' },
          '--chart-bar-color': barColor,
        }}
      >
        <ChartWidgetContent
          charts={ charts }
          isError={ isError }
          isLoading={ isLoading }
          empty={ !hasNonEmptyCharts }
          handleZoom={ handleZoom }
          zoomRange={ zoomRange }
        />
      </Box>
    </Box>
  );
};

export default chakra(ChartWidgetPanel);
