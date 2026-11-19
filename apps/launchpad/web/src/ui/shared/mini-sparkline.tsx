'use client';

import { useMemo } from 'react';
import { cn } from '@/lib/utils';

interface MiniSparklineProps {
  data: number[];
  width?: number;
  height?: number;
  className?: string;
  positive?: boolean;
  /** Spoken description; omit to mark the sparkline decorative. */
  label?: string;
}

export default function MiniSparkline({
  data,
  width: displayWidth,
  height: displayHeight,
  className = '',
  positive = true,
  label,
}: MiniSparklineProps) {
  const width = displayWidth ?? 64;
  const height = displayHeight ?? 24;
  const path = useMemo(() => {
    if (data.length < 2) return '';
    const min = Math.min(...data);
    const max = Math.max(...data);
    const range = max - min || 1;
    const step = width / (data.length - 1);

    const points = data.map((v, i) => ({
      x: i * step,
      y: height - ((v - min) / range) * (height - 2) - 1,
    }));

    let d = `M${points[0].x},${points[0].y}`;
    for (let i = 1; i < points.length; i++) {
      const prev = points[i - 1];
      const curr = points[i];
      const cpx = (prev.x + curr.x) / 2;
      d += ` C${cpx},${prev.y} ${cpx},${curr.y} ${curr.x},${curr.y}`;
    }
    return d;
  }, [data, width, height]);

  if (data.length < 2) return null;

  const stroke = positive ? 'stroke-candle-up' : 'stroke-candle-down';
  const fill = positive ? 'var(--sl-tint-accent-fill-chart)' : 'var(--sl-tint-danger-fill-chart)';

  return (
    <svg
      style={{ width: displayWidth, height: displayHeight }}
      viewBox={`0 0 ${width} ${height}`}
      className={cn('w-[var(--sl-chart-mini-width)] h-[var(--sl-chart-mini-height)]', className)}
      preserveAspectRatio="none"
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
    >
      <path d={`${path} L${width},${height} L0,${height} Z`} style={{ fill }} />
      <path
        d={path}
        fill="none"
        className={stroke}
        style={{ strokeWidth: 'var(--sl-chart-stroke)' }}
        strokeLinecap="round"
        strokeLinejoin="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
