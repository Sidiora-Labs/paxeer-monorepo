import { Box } from '@chakra-ui/react';
import React from 'react';

import ListItemMobileGrid from 'ui/shared/ListItemMobile/ListItemMobileGrid';

const Label = (props: React.ComponentProps<typeof ListItemMobileGrid.Label>) => (
  <ListItemMobileGrid.Label { ...props } fontWeight="medium" my={ 1.25 } minW={ 0 }/>
);

const Value = (props: React.ComponentProps<typeof ListItemMobileGrid.Value>) => (
  <ListItemMobileGrid.Value
    { ...props }
    py={ 1.25 }
    minW={ 0 }
    overflow="visible"
    whiteSpace="normal"
    css={{
      '& > *, & [data-entity]': { minW: 0, maxW: 'full' },
      '& [data-entity]': { flexWrap: 'wrap', rowGap: 'max(0px, calc(var(--chakra-sizes-touch-target) - 1lh))' },
      '& [data-entity-content]': { whiteSpace: 'normal', overflowWrap: 'anywhere' },
    }}
  />
);

const Container = ({ children, ...props }: React.ComponentProps<typeof ListItemMobileGrid.Container>) => (
  <Box containerType="inline-size" containerName="activity-item" minW={ 0 } w="full">
    <ListItemMobileGrid.Container
      { ...props }
      gridTemplateColumns={ 'calc(var(--chakra-spacing-20) + var(--chakra-spacing-1\\.5)) minmax(0, 1fr)' }
      borderTopWidth="var(--chakra-border-widths-1)"
      _last={{ borderBottomWidth: 'var(--chakra-border-widths-1)' }}
      css={{
        '& button': { },
        '& > *': { minW: 0, maxW: 'full', overflowWrap: 'anywhere' },
        _motionReduce: { animation: 'none' },
      }}
    >
      { children }
    </ListItemMobileGrid.Container>
  </Box>
);

const ActivityListItem = { Container, Label, Value };

export default ActivityListItem;
