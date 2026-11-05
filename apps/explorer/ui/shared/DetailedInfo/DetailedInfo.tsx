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

export const ITEM_VALUE_LINE_HEIGHT = { base: '30px', lg: '32px' };

export const Container = (props: GridProps) => {
  return (
    <Grid
      columnGap={{ base: 4, lg: 6 }}
      rowGap={ 0 }
      templateColumns={{ base: 'minmax(0, 1fr)', lg: '220px minmax(0, 1fr)' }}
      textStyle={{ base: 'sm', lg: 'md' }}
      data-detailed-info
      css={{
        '& > [data-detailed-info-label]': { borderTop: '1px solid', borderColor: 'hairline', pt: '8px' },
        '& > [data-detailed-info-value]': { borderTop: { base: 'none', lg: '1px solid' }, borderColor: 'hairline', py: '8px' },
      }}
      { ...props }
    />
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
  <Text fontWeight={ 500 } color="text.secondary" textStyle="xs" className="note" textAlign="right">
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
        <Skeleton loading={ isLoading } fontWeight={{ base: 700, lg: 500 }} py={{ base: '5px', lg: '4px' }} flexGrow={ 1 }>
          { children }
          { hasScroll && <ItemLabelScrollText/> }
        </Skeleton>
        { hint && <Hint label={ hint } isLoading={ isLoading } my={{ base: '5px', lg: '6px' }}/> }
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
      whiteSpace={{ base: 'normal', lg: 'nowrap' }}
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
      mt={{ base: 2, lg: 3 }}
      mb={{ base: 0, lg: 3 }}
      borderBottom="1px solid"
      borderColor="border.divider"
      { ...rest }
    />
  );
};
