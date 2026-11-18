import { useToken } from '@chakra-ui/react';
import * as d3 from 'd3';
import React from 'react';

import { useColorModeValue } from '../../../chakra/color-mode';
import { useReducedMotion } from '../../../hooks/useReducedMotion';
import { motionDuration } from '../../../theme/motion';

export interface ChartAxisProps extends Omit<React.SVGProps<SVGGElement>, 'scale'> {
  type: 'left' | 'bottom';
  scale: d3.ScaleTime<number, number> | d3.ScaleLinear<number, number>;
  noAnimation?: boolean;
  ticks: number;
  tickFormatGenerator?: (axis: d3.Axis<d3.NumberValue>) => (domainValue: d3.AxisDomain, index: number) => string;
  anchorEl?: SVGRectElement | null;
}

export const ChartAxis = React.memo(({ type, scale, ticks, tickFormatGenerator, noAnimation, anchorEl, ...props }: ChartAxisProps) => {
  const reducedMotion = useReducedMotion();
  const ref = React.useRef<SVGGElement>(null);

  const textColor = useToken('colors', useColorModeValue('blackAlpha.600', 'whiteAlpha.500'));

  React.useEffect(() => {
    if (!ref.current) {
      return;
    }

    const axisGenerator = type === 'left' ? d3.axisLeft : d3.axisBottom;
    const axis = axisGenerator(scale).ticks(ticks);

    if (tickFormatGenerator) {
      axis.tickFormat(tickFormatGenerator(axis));
    }

    const axisGroup = d3.select(ref.current);
    const canvas = document.createElement('canvas');
    const context = canvas.getContext('2d');
    let animating = !noAnimation && !reducedMotion;
    const fitLabels = () => {
      const text = ref.current?.querySelector('text');
      if (!text || !context || animating) {
        return;
      }

      const style = window.getComputedStyle(text);
      context.font = `${ style.fontWeight } ${ style.fontSize } ${ style.fontFamily }`;
      const referenceWidth = context.measureText(text.textContent ?? '').width;
      const widthRatio = referenceWidth > 0 ? text.getBBox().width / referenceWidth : 1;
      const svg = ref.current?.ownerSVGElement;
      const viewport = svg?.getBoundingClientRect();
      const origin = ref.current?.getScreenCTM();
      const left = Math.max(viewport?.left ?? -Infinity,
        ...Array.from(svg?.querySelectorAll('[data-chart-axis="left"] .tick text') ?? [], (label) => label.getBoundingClientRect().right));
      const right = viewport?.right ?? Infinity;
      const format = axis.tickFormat();
      const candidates = scale.ticks(ticks).sort((a, b) => scale(Number(a)) - scale(Number(b)));
      const values: Array<d3.NumberValue> = [];
      let previousEnd = -Infinity;
      candidates.forEach((value, index) => {
        const label = format ? format(value, index) : String(value);
        const metrics = context.measureText(label);
        const extent = type === 'bottom' ? metrics.width * widthRatio : text.getBBox().height;
        const position = scale(Number(value)) + (type === 'bottom' ? origin?.e ?? 0 : origin?.f ?? 0);
        const start = type === 'bottom' ? Math.max(left, Math.min(position - extent / 2, right - extent)) : position - extent / 2;
        if (start >= previousEnd) {
          values.push(value);
          previousEnd = start + extent;
        }
      });
      axisGroup.call(axis.tickValues(values));
      axisGroup.select('.domain').remove();
      axisGroup.selectAll('line').remove();
      axisGroup.selectAll<SVGTextElement, d3.NumberValue>('text')
        .filter(function() {
          return !this.style.fontSize;
        })
        .style('font-size', style.fontSize)
        .style('font-variant-numeric', 'tabular-nums')
        .attr('color', textColor);

      if (viewport) {
        axisGroup.selectAll<SVGTextElement, d3.NumberValue>('text').each(function() {
          this.removeAttribute('text-anchor');
          this.removeAttribute('transform');
          const box = this.getBoundingClientRect();
          const minimumLeft = type === 'bottom' ? left : viewport.left;
          const x = Math.max(minimumLeft - box.left, Math.min(0, viewport.right - box.right));
          const y = Math.max(viewport.top - box.top, Math.min(0, viewport.bottom - box.bottom));
          this.setAttribute('transform', `translate(${ x }, ${ y })`);
        });
      }
    };

    if (noAnimation || reducedMotion) {
      axisGroup.call(axis);
    } else {
      axisGroup.transition().duration(motionDuration('chart')).ease(d3.easeLinear).call(axis).on('end.fit', () => {
        animating = false;
        fitLabels();
      });
    }
    axisGroup.select('.domain').remove();
    axisGroup.selectAll('line').remove();
    axisGroup.selectAll('text')
      .attr('opacity', 1)
      .attr('color', textColor)
      .style('font-size', 'var(--chakra-font-sizes-text-xs)')
      .style('font-variant-numeric', 'tabular-nums');

    let frame = 0;
    const observer = new ResizeObserver(() => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(fitLabels);
    });
    axisGroup.selectAll<SVGTextElement, d3.NumberValue>('text').each(function() {
      observer.observe(this);
    });
    fitLabels();
    return () => {
      observer.disconnect();
      window.cancelAnimationFrame(frame);
    };
  }, [ scale, ticks, tickFormatGenerator, noAnimation, reducedMotion, type, textColor ]);

  React.useEffect(() => {
    if (!anchorEl) {
      return;
    }

    const anchorD3 = d3.select(anchorEl);

    anchorD3
      .on('mouseout.axisX', () => {
        d3.select(ref.current)
          .selectAll('text')
          .style('font-weight', 'normal');
      })
      .on('mousemove.axisX', (event) => {
        const [ x ] = d3.pointer(event, anchorEl);
        const xDate = scale.invert(x);
        const textElements = d3.select(ref.current).selectAll('text');
        const data = textElements.data();
        const index = d3.bisector((d) => d).left(data, xDate);
        textElements
          .style('font-weight', (_, i) => i === index - 1 ? 'bold' : 'normal');
      });

    return () => {
      anchorD3.on('mouseout.axisX mousemove.axisX', null);
    };
  }, [ anchorEl, scale ]);

  return <g ref={ ref } data-chart-axis={ type } { ...props }/>;
});
