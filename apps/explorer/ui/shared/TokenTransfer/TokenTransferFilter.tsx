import { Text, Stack } from '@chakra-ui/react';
import React from 'react';

import type { AddressFromToFilter } from 'types/api/address';
import type { TokenType } from 'types/api/token';
import type { ClusterChainConfig } from 'types/multichain';

import useIsInitialLoading from 'lib/hooks/useIsInitialLoading';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import { Radio, RadioGroup } from 'toolkit/chakra/radio';
import FilterButton from 'ui/shared/filters/FilterButton';
import TokenTypeFilter from 'ui/shared/filters/TokenTypeFilter';

interface Props {
  appliedFiltersNum?: number;
  defaultTypeFilters: Array<TokenType> | undefined;
  onTypeFilterChange: (nextValue: Array<TokenType>) => void;
  withAddressFilter?: boolean;
  onAddressFilterChange?: (nextValue: string) => void;
  defaultAddressFilter?: AddressFromToFilter;
  isLoading?: boolean;
  chainConfig?: Array<ClusterChainConfig['app_config']> | ClusterChainConfig['app_config'];
}

const TokenTransferFilter = ({
  onTypeFilterChange,
  defaultTypeFilters,
  appliedFiltersNum,
  withAddressFilter,
  onAddressFilterChange,
  defaultAddressFilter,
  isLoading,
  chainConfig,
}: Props) => {
  const isInitialLoading = useIsInitialLoading(isLoading);

  const handleAddressFilterChange = React.useCallback(({ value }: { value: string | null }) => {
    if (!value) {
      return;
    }

    onAddressFilterChange?.(value);
  }, [ onAddressFilterChange ]);

  return (
    <PopoverRoot autoFocus>
      <PopoverTrigger className="group">
        <FilterButton
          aria-label="Filter token transfers"
          appliedFiltersNum={ appliedFiltersNum }
          isLoading={ isInitialLoading }
          minH="touchTarget"
          minW="touchTarget"
        />
      </PopoverTrigger>
      <PopoverContent
        aria-label="Filter token transfers"
        w="calc(var(--chakra-spacing-40) + var(--chakra-spacing-15))"
        maxW="calc(100vw - var(--chakra-spacing-8))"
      >
        <PopoverBody display="flex" flexDir="column" rowGap={ 5 }>
          { withAddressFilter && (
            <>
              <Text color="text.secondary" fontWeight="semibold">Address</Text>
              <RadioGroup
                aria-label="Transfer direction"
                size="lg"
                onValueChange={ handleAddressFilterChange }
                defaultValue={ defaultAddressFilter || 'all' }
                paddingBottom={ 4 }
                borderBottomWidth="var(--chakra-border-widths-1)"
                borderColor="border.divider"
              >
                <Stack gap={ 4 }>
                  <Radio value="all"><Text fontSize="md">All</Text></Radio>
                  <Radio value="from"><Text fontSize="md">Outgoing transfers</Text></Radio>
                  <Radio value="to"><Text fontSize="md">Incoming transfers</Text></Radio>
                </Stack>
              </RadioGroup>
            </>
          ) }
          <TokenTypeFilter<TokenType>
            onChange={ onTypeFilterChange }
            defaultValue={ defaultTypeFilters }
            nftOnly={ false }
            chainConfig={ chainConfig }
          />
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default React.memo(TokenTransferFilter);
