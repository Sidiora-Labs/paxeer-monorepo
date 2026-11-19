import type { GridProps } from '@chakra-ui/react';
import { Box } from '@chakra-ui/react';
import React from 'react';

import { INLINE_CONTROL_STYLES } from 'toolkit/utils/inlineControl';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';

export * from 'ui/shared/DetailedInfo/DetailedInfo';

export const Container = (props: GridProps) => (
  <Box containerType="inline-size" containerName="activity-details" minW={ 0 } w="full" data-activity-details>
    <DetailedInfo.Container
      { ...props }
      templateColumns={{ base: 'minmax(0, 1fr)', lg: 'calc(var(--chakra-spacing-40) + var(--chakra-spacing-15)) minmax(0, 1fr)' }}
      textStyle={{ base: 'sm', lg: 'md' }}
      css={{
        '& a': INLINE_CONTROL_STYLES,
        '& > [data-detailed-info-label]': {
          borderTopWidth: 'var(--chakra-border-widths-1)', pt: 2, minW: 0, minH: DetailedInfo.ITEM_VALUE_LINE_HEIGHT,
        },
        '& > [data-detailed-info-value]': {
          borderTopWidth: { base: 0, lg: 'var(--chakra-border-widths-1)' },
          py: 2,
          minH: DetailedInfo.ITEM_VALUE_LINE_HEIGHT,
          whiteSpace: 'normal',
          overflowWrap: 'anywhere',
          flexWrap: 'wrap',
        },
        '& > [data-detailed-info-label] > div > div': { py: { base: 1.25, lg: 1 }, fontWeight: { base: 'bold', lg: 'medium' } },
        '& > :not([data-detailed-info-label]):not([data-detailed-info-value])': { gridColumn: '1 / -1' },
      }}
    />
  </Box>
);

export const Group = ({ label, children, ...props }: GridProps & { label: string }) => (
  <DetailedInfo.Section label={ label }>
    <Container { ...props }>{ children }</Container>
  </DetailedInfo.Section>
);
