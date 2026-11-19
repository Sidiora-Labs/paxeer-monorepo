import type { GridProps } from '@chakra-ui/react';
import { Box } from '@chakra-ui/react';
import React from 'react';

import breakpoints from 'toolkit/theme/foundations/breakpoints';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';

export * from 'ui/shared/DetailedInfo/DetailedInfo';

export const Container = (props: GridProps) => (
  <Box containerType="inline-size" containerName="activity-details" minW={ 0 } w="full" data-activity-details>
    <DetailedInfo.Container
      { ...props }
      templateColumns="minmax(0, 1fr)"
      textStyle="body"
      css={{
        '& button': { },
        '& > [data-detailed-info-label]': { borderTopWidth: 'var(--chakra-border-widths-1)', pt: 2, minW: 0, minH: 8 },
        '& > [data-detailed-info-value]': {
          borderTopWidth: 0, py: 2, minH: 8, lineHeight: 'body', whiteSpace: 'normal', overflowWrap: 'anywhere', flexWrap: 'wrap',
        },
        '& > [data-detailed-info-label] > div > div': { py: 0, fontWeight: 'medium' },
        '& > :not([data-detailed-info-label]):not([data-detailed-info-value])': { gridColumn: '1 / -1' },
        [`@container activity-details (min-width: ${ breakpoints.lg })`]: {
          gridTemplateColumns: 'var(--chakra-spacing-56) minmax(0, 1fr)',
          '& > [data-detailed-info-value]': { borderTopWidth: 'var(--chakra-border-widths-1)' },
        },
      }}
    />
  </Box>
);

export const Group = ({ label, children, ...props }: GridProps & { label: string }) => (
  <DetailedInfo.Section label={ label }>
    <Container { ...props }>{ children }</Container>
  </DetailedInfo.Section>
);
