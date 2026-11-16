import type { GridItemProps, GridProps } from '@chakra-ui/react';
import { Box, Flex, Grid, GridItem, Text } from '@chakra-ui/react';
import React from 'react';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { Hint } from 'toolkit/components/Hint/Hint';
import * as ContainerWithScrollY from 'ui/shared/ContainerWithScrollY';

interface SectionProps {
  label: string;
  children: React.ReactNode;
  className?: string;
}

export const Section = ({ label, children, className }: SectionProps) => {
  const id = React.useId();
  return (
    <Box as="section" aria-labelledby={ id } className={ className } minW={ 0 } data-section-group={ label }>
      <Text as="h2" id={ id } textStyle="label" color="text.muted" textTransform="uppercase" mb={ 2 }>{ label }</Text>
      { children }
    </Box>
  );
};

export const Group = ({ label, children, className, ...props }: SectionProps & Omit<GridProps, 'children'>) => (
  <Section label={ label } className={ className }>
    <Container { ...props }>{ children }</Container>
  </Section>
);

export const ITEM_VALUE_LINE_HEIGHT = { base: 'var(--chakra-spacing-7\\.5)', lg: 'var(--chakra-spacing-8)' };

export const Container = (props: GridProps) => {
  return (
    <Box minW={ 0 } containerType="inline-size" containerName="detailed-info">
      <Grid
        columnGap={{ base: 4, lg: 6 }}
        rowGap={ 0 }
        templateColumns={{ base: 'minmax(0, 1fr)', lg: '220px minmax(0, 1fr)' }}
        textStyle={{ base: 'sm', lg: 'md' }}
        data-detailed-info
        css={{
          '& > [data-detailed-info-label]': { borderTopWidth: 'var(--chakra-border-widths-1)', borderColor: 'hairline', pt: 2 },
          '& > [data-detailed-info-value]': { borderTopWidth: { base: 0, lg: 'var(--chakra-border-widths-1)' }, borderColor: 'hairline', py: 2 },
          '& > [data-detailed-info-divider]': { gridColumn: '1 / -1' },
          '@supports (container-type: inline-size)': {
            gridTemplateColumns: 'minmax(0, 1fr)',
            '& > [data-detailed-info-value]': { borderTopWidth: 0 },
            '@container detailed-info (min-width: 48rem)': {
              gridTemplateColumns: 'calc(var(--chakra-spacing-40) + var(--chakra-spacing-15)) minmax(0, 1fr)',
              '& > [data-detailed-info-value]': { borderTopWidth: 'var(--chakra-border-widths-1)' },
            },
          },
        }}
        { ...props }
      />
    </Box>
  );
};

interface ItemLabelProps extends GridItemProps {
  hint?: React.ReactNode;
  children: React.ReactNode;
  isLoading?: boolean;
  id?: string;
  hasScroll?: boolean;
}

const ItemLabelScrollText = () => (
  <Text fontWeight="medium" color="text.secondary" textStyle="xs" className="note" textAlign="right">
    Scroll to see more
  </Text>
);

export const ItemLabel = ({ hint, children, isLoading, id, hasScroll, ...rest }: ItemLabelProps) => {
  return (
    <GridItem
      id={ id }
      minH={ ITEM_VALUE_LINE_HEIGHT }
      data-detailed-info-label
      { ...rest }
    >
      <Flex columnGap={{ base: 1, lg: 2 }} alignItems="flex-start" w="100%" color="text.secondary">
        <Skeleton loading={ isLoading } fontWeight={{ base: 'bold', lg: 'medium' }} py={{ base: 1.25, lg: 1 }} flexGrow={ 1 }>
          { children }
          { hasScroll && <ItemLabelScrollText/> }
        </Skeleton>
        { hint && <Hint label={ hint } isLoading={ isLoading } my={{ base: 1.25, lg: 1.5 }}/> }
      </Flex>
    </GridItem>
  );
};

interface ItemValueProps extends GridItemProps {
  children: React.ReactNode;
  multiRow?: boolean;
}

export const ItemValue = ({ children, multiRow = false, ...rest }: ItemValueProps) => {
  return (
    <GridItem
      display="flex"
      alignItems="center"
      flexWrap={ multiRow ? 'wrap' : { base: 'wrap', lg: 'nowrap' } }
      rowGap={{ base: 1, lg: 0 }}
      pl={ 0 }
      pr={{ base: 1, lg: 0 }}
      minW={ 0 }
      minH={ ITEM_VALUE_LINE_HEIGHT }
      color="text.primary"
      whiteSpace="normal"
      overflowWrap="anywhere"
      data-detailed-info-value
      data-stack-below="lg"
      { ...(multiRow ? {
        lineHeight: ITEM_VALUE_LINE_HEIGHT,
      } : {}) }
      { ...rest }
    >
      { children }
    </GridItem>
  );
};

export const ItemValueWithScroll = ({ children, gradientHeight, onScrollVisibilityChange, ...rest }: ContainerWithScrollY.Props) => {
  return (
    <ItemValue position="relative" >
      <ContainerWithScrollY.default
        { ...rest }
        gradientHeight={ gradientHeight }
        onScrollVisibilityChange={ onScrollVisibilityChange }
      >
        { children }
      </ContainerWithScrollY.default>
    </ItemValue>
  );
};

export const ItemDivider = (props: GridItemProps) => {
  const { colSpan = { base: undefined, lg: 2 }, ...rest } = props;
  return (
    <GridItem
      colSpan={ colSpan }
      data-detailed-info-divider
      mt={{ base: 2, lg: 3 }}
      mb={{ base: 0, lg: 3 }}
      borderBottomWidth="var(--chakra-border-widths-1)"
      borderColor="border.divider"
      { ...rest }
    />
  );
};
