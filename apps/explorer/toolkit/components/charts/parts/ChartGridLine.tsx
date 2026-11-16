import { useToken } from '@chakra-ui/react';
import * as d3 from 'd3';
import React from 'react';

import { useReducedMotion } from '../../../hooks/useReducedMotion';
import { motionDuration } from '../../../theme/motion';

export interface ChartGridLineProps extends Omit<React.SVGProps<SVGGElement>, 'scale'> {
  type: 'vertical' | 'horizontal';
  scale: d3.ScaleTime<number, number> | d3.ScaleLinear<number, number>;
  noAnimation?: boolean;
  size: number;
  ticks: number;
}

export const ChartGridLine = React.memo(({ type, scale, ticks, size, noAnimation, ...props }: ChartGridLineProps) => {
  const reducedMotion = useReducedMotion();
  const ref = React.useRef<SVGGElement>(null);

  const strokeColor = useToken('colors', 'border.divider');

  React.useEffect(() => {
    if (!ref.current) {
      return;
    }

    const axisGenerator = type === 'vertical' ? d3.axisBottom : d3.axisLeft;
    const axis = axisGenerator(scale).ticks(ticks).tickSize(-size);

    const gridGroup = d3.select(ref.current);
    if (noAnimation || reducedMotion) {
      gridGroup.call(axis);
    } else {
      gridGroup.transition().duration(motionDuration('chart')).ease(d3.easeLinear).call(axis);
    }
    gridGroup.select('.domain').remove();
    gridGroup.selectAll('text').remove();
    gridGroup.selectAll('line').attr('stroke', strokeColor);
  }, [ scale, ticks, size, noAnimation, reducedMotion, type, strokeColor ]);

  return <g ref={ ref } { ...props }/>;
});
