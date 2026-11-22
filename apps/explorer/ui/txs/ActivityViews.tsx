import { Box } from '@chakra-ui/react';
import React from 'react';

import breakpoints from 'toolkit/theme/foundations/breakpoints';

interface Props {
  list: React.ReactNode;
  table: React.ReactNode;
  tableView?: boolean;
  tableBreakpoint?: keyof typeof breakpoints;
}

const ActivityViews = ({ list, table, tableView = false, tableBreakpoint = 'lg' }: Props) => {
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [ width, setWidth ] = React.useState(() => typeof window === 'undefined' ? 0 : window.innerWidth);

  React.useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) setWidth(entry.contentRect.width);
    });
    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  const showTable = tableView || width >= parseFloat(breakpoints[tableBreakpoint]);

  return (
    <Box ref={ containerRef } containerType="inline-size" containerName="activity-views" minW={ 0 } w="full">
      <Box css={{ '& button': { } }}>
        { !showTable && <Box data-activity-list minW={ 0 }>{ list }</Box> }
        { showTable && <Box data-activity-table minW={ 0 }>{ table }</Box> }
      </Box>
    </Box>
  );
};

export default ActivityViews;
