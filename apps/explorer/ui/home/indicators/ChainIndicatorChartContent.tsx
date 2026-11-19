import { Box } from '@chakra-ui/react';
import React from 'react';

import type { TimeChartData } from 'toolkit/components/charts/types';

import { ChartArea, ChartAxis, ChartLine, ChartOverlay, ChartTooltip, useTimeChartController } from 'toolkit/components/charts';
import { useDefaultGradient, useDefaultLineColor } from 'ui/shared/chart/config';

interface Props {
  data: TimeChartData;
  caption?: string;
}

const X_AXIS_TICKS = 3;
const Y_AXIS_TICKS = 2;
const CHART_MARGIN = { left: 9, right: 1 };

const ChainIndicatorChartContent = ({ data, caption }: Props) => {
  const overlayRef = React.useRef<SVGRectElement>(null);
  const descriptionId = React.useId();
  const lineColor = useDefaultLineColor();
  const gradient = useDefaultGradient();

  const axesConfig = React.useMemo(() => {
    return {
      x: { ticks: X_AXIS_TICKS },
      y: { ticks: Y_AXIS_TICKS, nice: true },
    };
  }, [ ]);

  const { rect, ref, axes, innerWidth, innerHeight, chartMargin } = useTimeChartController({
    data,
    axesConfig,
    margin: CHART_MARGIN,
  });

  return (
    <Box pt={ 1.25 } pb={ 6 } w="full" h="full" css={{
      '& .tick text': { color: 'text.muted' },
      '& [data-label="sparkline-x-axis"] .tick:first-of-type text': { textAnchor: 'start' },
      '& [data-label="sparkline-x-axis"] .tick:last-of-type text': { textAnchor: 'end' },
    }}>
      <svg overflow="visible" width="100%" height="100%" ref={ ref } cursor="pointer" data-label="sparkline" role="img"
        aria-label={ caption ?? data[0].name } aria-describedby={ descriptionId }>
        <desc id={ descriptionId }>
          { data[0].items.map(({ date, value }) =>
            `${ date.toLocaleDateString() }: ${ value.toLocaleString(undefined, { maximumSignificantDigits: 21 }) }`,
          ).join('; ') }
        </desc>
        <g transform={ `translate(${ chartMargin.left || 0 },${ chartMargin.top || 0 })` } opacity={ rect ? 1 : 0 }>
          <ChartArea
            id={ data[0].id }
            data={ data[0].items }
            xScale={ axes.x.scale }
            yScale={ axes.y.scale }
            gradient={ gradient }
            noAnimation
          />
          <ChartLine
            data={ data[0].items }
            xScale={ axes.x.scale }
            yScale={ axes.y.scale }
            stroke={ lineColor }
            strokeWidth="var(--chakra-border-widths-2)"
            animation="none"
          />
          <ChartAxis
            data-label="sparkline-y-axis"
            type="left"
            scale={ axes.y.scale }
            ticks={ Y_AXIS_TICKS }
            tickFormatGenerator={ axes.y.tickFormatter }
            noAnimation
          />
          <ChartAxis
            data-label="sparkline-x-axis"
            type="bottom"
            scale={ axes.x.scale }
            transform={ `translate(0, ${ innerHeight })` }
            ticks={ X_AXIS_TICKS }
            tickFormatGenerator={ axes.x.tickFormatter }
            noAnimation
          />
          <ChartOverlay ref={ overlayRef } width={ innerWidth } height={ innerHeight }>
            <ChartTooltip
              anchorEl={ overlayRef.current }
              width={ innerWidth }
              height={ innerHeight }
              xScale={ axes.x.scale }
              yScale={ axes.y.scale }
              data={ data }
            />
          </ChartOverlay>
        </g>
      </svg>
    </Box>
  );
};

export default React.memo(ChainIndicatorChartContent);
