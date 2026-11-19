import { Text, Flex, Box, chakra } from '@chakra-ui/react';
import React from 'react';

import type { TChainIndicator } from './types';
import type { ChainIndicatorId } from 'types/homepage';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { mdash } from 'toolkit/utils/htmlEntities';
interface Props {
  indicator: TChainIndicator;
  isSelected: boolean;
  onClick: (id: ChainIndicatorId) => void;
  isLoading: boolean;
}

const ChainIndicatorItem = ({ indicator, isSelected, onClick, isLoading }: Props) => {
  const handleClick = React.useCallback(() => {
    onClick(indicator.id);
  }, [ indicator.id, onClick ]);

  const valueContent = (() => {
    if (indicator.value.includes('N/A')) {
      return <Text opacity="control.disabled" fontWeight="normal">{ mdash }</Text>;
    }

    return (
      <Skeleton loading={ isLoading } fontWeight="semibold" minW="var(--chakra-spacing-7\.5)">
        { indicator.value }
      </Skeleton>
    );
  })();

  const valueDiffContent = (() => {
    if (indicator.valueDiff === undefined || (!isLoading && indicator.value.includes('N/A'))) {
      return null;
    }

    const diffColor = indicator.valueDiff >= 0 ? 'positive' : 'negative';

    return (
      <Skeleton loading={ isLoading } ml={ 1 } display="flex" alignItems="center" color={ diffColor }>
        <span>{ indicator.valueDiff >= 0 ? '+' : '-' }</span>
        <Text color={ diffColor } fontWeight="semibold">{ Math.abs(indicator.valueDiff) }%</Text>
      </Skeleton>
    );
  })();

  return (
    <Box as="li" listStyleType="none" minW={ 0 }>
      <chakra.button
        type="button"
        aria-label={ indicator.title }
        aria-pressed={ isSelected }
        disabled={ isLoading }
        display="flex"
        minH="touchTarget"
        minW="touchTarget"
        w="full"
        alignItems="center"
        columnGap={ 2 }
        flexGrow={{ base: 0, lg: 1 }}
        px={{ base: 'var(--chakra-spacing-1\\.5)', lg: 2 }}
        py="var(--chakra-spacing-1\.5)"
        borderRadius="base"
        cursor="pointer"
        color={ isSelected ? 'text.secondary' : 'link.primary' }
        bgColor={ isSelected ? 'bg.primary' : undefined }
        onClick={ handleClick }
        fontSize="xs"
        fontWeight="medium"
        _focusVisible={{ outlineWidth: '2', outlineStyle: 'solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-border-widths-2)' }}
        _disabled={{ cursor: 'default', opacity: 'control.disabled' }}
        _pressed={{ borderColor: 'accent' }}
        borderWidth="1"
        borderColor={ isSelected ? 'accent' : 'transparent' }
        _hover={{
          bgColor: 'bg.primary',
          color: isSelected ? 'text.secondary' : 'hover',
          zIndex: 1,
        }}
      >
        { indicator.icon }
        <Box display={{ base: 'none', lg: 'block' }}>
          <span>{ indicator.titleShort || indicator.title }</span>
          <Flex alignItems="center" color="text.primary">
            { valueContent }
            { valueDiffContent }
          </Flex>
        </Box>
      </chakra.button>
    </Box>
  );
};

export default React.memo(ChainIndicatorItem);
