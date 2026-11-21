import { castArray } from 'es-toolkit/compat';
import React from 'react';

import type { AdvancedFilterAge } from 'types/api/advancedFilter';
import type { TokenInfo } from 'types/api/token';
import { ZETA_CHAIN_CCTX_COIN_TYPE_FILTER, type ZetaChainCCTXFilterParams } from 'types/client/zetaChain';

import { Button } from 'toolkit/chakra/button';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'ui/marketplace/NativePopover';
import IconSvg from 'ui/shared/IconSvg';

import ZetaChainAgeFilter from './ZetaChainAgeFilter';
import ZetaChainAssetFilter, { ZETA_NATIVE_TOKEN } from './ZetaChainAssetFilter';
import ZetaChainReceiverFilter from './ZetaChainReceiverFilter';
import ZetaChainSenderFilter from './ZetaChainSenderFilter';
import ZetaChainStatusFilter from './ZetaChainStatusFilter';

type ColumnId = 'age' | 'status' | 'sender' | 'receiver' | 'asset';

type Props = {
  filters: ZetaChainCCTXFilterParams;
  column: ColumnId;
  columnName: string;
  handleFilterChange: <T extends keyof ZetaChainCCTXFilterParams>(field: T, val: ZetaChainCCTXFilterParams[T]) => void;
  isLoading?: boolean;
};

type FilterPopoverProps = React.ComponentProps<typeof PopoverContent> & {
  columnName: string;
  isLoading?: boolean;
  selected?: boolean;
};

function ZetaChainFilterPopover({ columnName, isLoading, selected, children, ...contentProps }: FilterPopoverProps) {
  return (
    <PopoverRoot>
      <PopoverTrigger>
        <Button
          aria-label={ `filter by ${ columnName }` }
          variant="icon_secondary"
          size="sm"
          disabled={ isLoading }
          selected={ selected }
          p={ 0 }
        >
          <IconSvg name="filter" boxSize={ 5 }/>
        </Button>
      </PopoverTrigger>
      <PopoverContent maxH="80dvh" overflowY="auto" overscrollBehavior="contain" { ...contentProps }>
        <PopoverBody display="flex" flexDir="column" rowGap={ 3 }>
          { children }
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
}

const ZetaChainFilterByColumn = ({ column, filters, columnName, handleFilterChange, isLoading }: Props) => {
  const commonProps = { columnName, handleFilterChange, isLoading };

  switch (column) {
    case 'age': {
      const value = { age: (filters.age || '') as AdvancedFilterAge | '', from: filters.start_timestamp || '', to: filters.end_timestamp || '' };
      return (
        <ZetaChainFilterPopover
          maxW="calc(100dvw - var(--chakra-spacing-8))"
          columnName="Age"
          isLoading={ isLoading }
          selected={ Boolean(value.age || value.from || value.to) }
          w="calc(var(--chakra-spacing-1) * 95.5)"
        >
          <ZetaChainAgeFilter { ...commonProps } value={ value }/>
        </ZetaChainFilterPopover>
      );
    }
    case 'status': {
      const value = filters.status_reduced ? castArray(filters.status_reduced) : [];
      return (
        <ZetaChainFilterPopover
          maxW="calc(100dvw - var(--chakra-spacing-8))"
          columnName="Status"
          isLoading={ isLoading }
          selected={ Boolean(value && value.length) }
          w="calc(var(--chakra-spacing-1) * 50)"
        >
          <ZetaChainStatusFilter { ...commonProps } value={ value }/>
        </ZetaChainFilterPopover>
      );
    }
    case 'sender': {
      const value = filters.sender_address ? castArray(filters.sender_address) : [];
      const chainValue = filters.source_chain_id ? castArray(filters.source_chain_id) : [];
      return (
        <ZetaChainFilterPopover
          maxW="calc(100dvw - var(--chakra-spacing-8))"
          columnName="Sender"
          isLoading={ isLoading }
          selected={ Boolean(value && value.length) || Boolean(chainValue && chainValue.length) }
          w="calc(var(--chakra-spacing-1) * 120)"
        >
          <ZetaChainSenderFilter { ...commonProps } value={ value } chainValue={ chainValue }/>
        </ZetaChainFilterPopover>
      );
    }
    case 'receiver': {
      const value = filters.receiver_address ? castArray(filters.receiver_address) : [];
      const chainValue = filters.target_chain_id ? castArray(filters.target_chain_id) : [];
      return (
        <ZetaChainFilterPopover
          maxW="calc(100dvw - var(--chakra-spacing-8))"
          columnName="Receiver"
          isLoading={ isLoading }
          selected={ Boolean(value && value.length) || Boolean(chainValue && chainValue.length) }
          w="calc(var(--chakra-spacing-1) * 120)"
        >
          <ZetaChainReceiverFilter { ...commonProps } value={ value } chainValue={ chainValue }/>
        </ZetaChainFilterPopover>
      );
    }
    case 'asset': {
      let value: TokenInfo | null = null;
      if (filters.coin_type && castArray(filters.coin_type).includes(ZETA_CHAIN_CCTX_COIN_TYPE_FILTER)) {
        value = ZETA_NATIVE_TOKEN;
      }
      if (filters.token_symbol?.[0]) {
        value = {
          address_hash: '',
          symbol: filters.token_symbol[0],
          name: filters.token_symbol[0],
          decimals: '18',
          total_supply: '0',
          icon_url: null,
          type: 'ERC-20',
          holders_count: null,
          exchange_rate: null,
          circulating_market_cap: null,
          reputation: null,
        };
      }

      return (
        <ZetaChainFilterPopover
          maxW="calc(100dvw - var(--chakra-spacing-8))"
          columnName="Asset"
          isLoading={ isLoading }
          selected={ Boolean(value) }
          w="calc(var(--chakra-spacing-1) * 87.5)"
        >
          <ZetaChainAssetFilter { ...commonProps } value={ value }/>
        </ZetaChainFilterPopover>
      );
    }
    default:
      return null;
  }
};

export default ZetaChainFilterByColumn;
