import { Box } from '@chakra-ui/react';
import React from 'react';

import breakpoints from 'toolkit/theme/foundations/breakpoints';
import ListItemMobileGrid from 'ui/shared/ListItemMobile/ListItemMobileGrid';

const Label = (props: React.ComponentProps<typeof ListItemMobileGrid.Label>) => (
  <ListItemMobileGrid.Label { ...props } fontWeight="medium" my={ 1 } minW={ 0 }/>
);

const Value = (props: React.ComponentProps<typeof ListItemMobileGrid.Value>) => (
  <ListItemMobileGrid.Value
    { ...props }
    py={ 1 }
    minW={ 0 }
    overflow="visible"
    whiteSpace="normal"
    css={{
      '& > *, & [data-entity]': { minW: 0, maxW: 'full' },
      '& [data-entity-content]': { whiteSpace: 'normal', overflowWrap: 'anywhere' },
    }}
  />
);

const Container = ({ children, ...props }: React.ComponentProps<typeof ListItemMobileGrid.Container>) => (
  <Box containerType="inline-size" containerName="activity-item" minW={ 0 } w="full">
    <ListItemMobileGrid.Container
      { ...props }
      gridTemplateColumns="minmax(0, 1fr)"
      borderTopWidth="var(--chakra-border-widths-1)"
      _last={{ borderBottomWidth: 'var(--chakra-border-widths-1)' }}
      css={{
        '& button': { minW: 'touchTarget', minH: 'touchTarget' },
        '& > *': { minW: 0, maxW: 'full', overflowWrap: 'anywhere' },
        [`@container activity-item (min-width: ${ breakpoints.sm })`]: {
          gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 2fr)',
        },
        _motionReduce: { animation: 'none' },
      }}
    >
      { children }
    </ListItemMobileGrid.Container>
  </Box>
);

const ActivityListItem = { Container, Label, Value };

export default ActivityListItem;
