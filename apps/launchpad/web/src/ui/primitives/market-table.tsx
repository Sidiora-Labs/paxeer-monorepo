import type { HTMLAttributes } from 'react';
import { cn } from '@/lib/utils';
import styles from './market-table.module.css';

export function MarketTable({ children, className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div {...props} className={cn(styles.market, 'flex min-w-0 flex-col gap-2 py-1', className)}>{children}</div>;
}
