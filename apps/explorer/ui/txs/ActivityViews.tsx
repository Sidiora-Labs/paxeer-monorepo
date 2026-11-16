import { Box } from '@chakra-ui/react';
import React from 'react';

import breakpoints from 'toolkit/theme/foundations/breakpoints';

interface Props {
  list: React.ReactNode;
  table: React.ReactNode;
  tableView?: boolean;
  tableBreakpoint?: keyof typeof breakpoints;
}

const ActivityViews = ({ list, table, tableView = false, tableBreakpoint = 'lg' }: Props) => (
  <Box containerType="inline-size" containerName="activity-views" minW={ 0 } w="full">
    <Box
      css={{
        '& button': { minW: 'touchTarget', minH: 'touchTarget' },
        '& > [data-activity-list]': { display: tableView ? 'none' : 'block' },
        '& > [data-activity-table]': { display: tableView ? 'block' : 'none' },
        [`@container activity-views (min-width: ${ breakpoints[tableBreakpoint] })`]: {
          '& > [data-activity-list]': { display: 'none' },
          '& > [data-activity-table]': { display: 'block' },
        },
      }}
    >
      <Box data-activity-list minW={ 0 }>{ list }</Box>
      <Box data-activity-table minW={ 0 }>{ table }</Box>
    </Box>
  </Box>
);

export default ActivityViews;
