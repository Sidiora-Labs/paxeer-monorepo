import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

import type { TableHeaderProps } from 'toolkit/chakra/table';
import { TableHeader, TableRoot } from 'toolkit/chakra/table';

interface Props extends React.ComponentProps<typeof TableRoot> {
  label: string;
}

const ActivityTable = React.forwardRef<HTMLTableElement, Props>(function ActivityTable({ label, children, ...props }, ref) {
  return (
    <Box
      role="region"
      aria-label={ label }
      tabIndex={ 0 }
      minW={ 0 }
      maxW="full"
      overflow="auto"
      maxH="calc(100dvh - var(--chakra-spacing-24))"
      overscrollBehaviorX="contain"
      _focusVisible={{
        outlineWidth: 'var(--chakra-border-widths-2)',
        outlineStyle: 'solid',
        outlineColor: 'focusRing',
        outlineOffset: 'var(--chakra-spacing-0\\.5)',
      }}
    >
      <TableRoot
        ref={ ref }
        { ...props }
        css={{
          ...props.css,
          '& [data-is-numeric]': { fontVariantNumeric: 'tabular-nums', textAlign: 'end' },
          '& [data-is-numeric] :is(a, span, div)': { fontVariantNumeric: 'tabular-nums' },
          '& tr': { _motionReduce: { animation: 'none' } },
          '& button': { },
        }}
      >
        <chakra.caption srOnly>{ label }</chakra.caption>
        { children }
      </TableRoot>
    </Box>
  );
});

export const ActivityTableHeader = (props: TableHeaderProps) => (
  <TableHeader { ...props } data-scroll-region-header position="sticky" top={ 0 } backgroundColor="canvas" zIndex="sticky"/>
);

export default ActivityTable;
