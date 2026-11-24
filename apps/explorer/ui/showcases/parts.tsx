import type { BoxProps, StackProps, TabsContentProps } from '@chakra-ui/react';
import { Box, Code, Grid, HStack, VStack } from '@chakra-ui/react';
import React from 'react';

import { Heading } from 'toolkit/chakra/heading';
import { TabsContent } from 'toolkit/chakra/tabs';
import breakpoints from 'toolkit/theme/foundations/breakpoints';

export const Container = (props: TabsContentProps) => (
  <TabsContent
    display="flex"
    flexDirection="column"
    gap={ 6 }
    w="100%"
    minW={ 0 }
    containerType="inline-size"
    { ...props }
  />
);
export const Section = (props: BoxProps) => <Box minW={ 0 } w="full" { ...props } as="section"/>;
export const SectionHeader = ({ children }: { children: React.ReactNode }) => <Heading level="2" mb={ 4 }>{ children }</Heading>;
export const SectionSubHeader = ({ children }: { children: React.ReactNode }) => (
  <Heading
    level="3"
    mb={ 3 }
    _notFirst={{ mt: 4 }}
  >{ children }</Heading>
);
export const SamplesStack = ({ children }: { children: React.ReactNode }) => (
  <Grid
    rowGap={ 4 }
    columnGap={ 8 }
    w="full"
    minW={ 0 }
    gridTemplateColumns="fit-content(var(--chakra-spacing-15)) minmax(0, 1fr)"
    css={{ [ `@container (min-width: ${ breakpoints.lg })` ]: { gridTemplateColumns: 'fit-content(35%) minmax(0, 1fr)' } }}
    justifyItems="flex-start"
    alignItems="flex-start"
  >
    { children }
  </Grid>
);
export const Sample = ({ children, label, vertical, ...props }: { children: React.ReactNode; vertical?: boolean; label?: string } & StackProps) => {
  const Stack = vertical ? VStack : HStack;
  return (
    <>
      { label && <Code w="fit-content" maxW="full" overflowWrap="anywhere">{ label }</Code> }
      <Stack
        gap={ 3 }
        whiteSpace="pre-wrap"
        flexWrap="wrap"
        gridColumn={ label ? 'auto' : '1 / -1' }
        minW={ 0 }
        maxW="full"
        css={{
          [ `@container (min-width: ${ breakpoints.lg })` ]: { gridColumn: label ? 'auto' : '1 / -1' },
          '& > :not(dialog)': { maxWidth: 'full', minWidth: 0 },
          '& .chakra-button': { whiteSpace: 'normal', overflowWrap: 'anywhere' },
        }}
        alignItems={ vertical ? 'flex-start' : 'center' }
        { ...props }
      >
        { children }
      </Stack>
    </>
  );
};
