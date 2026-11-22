import type { BoxProps } from '@chakra-ui/react';
import { Box } from '@chakra-ui/react';
import React from 'react';

import { INLINE_CONTROL_STYLES } from 'toolkit/utils/inlineControl';

interface Props {
  label: string;
  children: React.ReactNode;
  minTableWidth?: BoxProps['minW'];
}

const TableScrollArea = ({ label, children, minTableWidth = 'full' }: Props) => {
  const handleFocus = React.useCallback((event: React.FocusEvent<HTMLDivElement>) => {
    const region = event.currentTarget;
    if (event.target === region) return;
    const bounds = region.getBoundingClientRect();
    const target = event.target.getBoundingClientRect();
    const firstCell = event.target.closest('tr')?.firstElementChild;
    const visibleLeft = firstCell && !firstCell.contains(event.target) && getComputedStyle(firstCell).position === 'sticky' ?
      firstCell.getBoundingClientRect().right : bounds.left;
    if (target.left < visibleLeft) {
      region.scrollLeft -= visibleLeft - target.left;
    } else if (target.right > bounds.right) {
      region.scrollLeft += target.right - bounds.right;
    }
  }, []);

  return (
    <Box
      role="region"
      aria-label={ label }
      tabIndex={ 0 }
      onFocusCapture={ handleFocus }
      minW={ 0 }
      maxW="full"
      overflowX="auto"
      overscrollBehaviorX="contain"
      _focusVisible={{ outlineWidth: '2', outlineStyle: 'solid', outlineColor: 'focusRing', outlineOffset: 'calc(var(--chakra-border-widths-2) * -1)' }}
      css={{
        '& a': { ...INLINE_CONTROL_STYLES, display: 'inline-flex', alignItems: 'center' },
        '& a, & button': { _motionReduce: { transitionProperty: 'none' } },
        '& table': { minWidth: minTableWidth, width: 'full', tableLayout: 'auto' },
        '& thead': { top: 0 },
        '& th:first-of-type, & td:first-of-type': { position: { base: 'static', lg: 'sticky' }, insetInlineStart: 0, bg: 'canvas', zIndex: 1 },
        '& tr:hover > td, & tr:focus-within > td': { bg: 'raised' },
      }}
    >
      { children }
    </Box>
  );
};

export default TableScrollArea;
