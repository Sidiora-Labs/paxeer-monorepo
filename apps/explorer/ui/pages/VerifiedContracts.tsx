import { Box, createListCollection, HStack } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import useIsMobile from 'lib/hooks/useIsMobile';
import ActionBar from 'ui/shared/ActionBar';
import PageTitle from 'ui/shared/Page/PageTitle';
import Pagination from 'ui/shared/pagination/Pagination';
import Sort from 'ui/shared/sort/Sort';
import useVerifiedContractsQuery from 'ui/verifiedContracts/useVerifiedContractsQuery';
import { SORT_OPTIONS } from 'ui/verifiedContracts/utils';
import VerifiedContractsCounters from 'ui/verifiedContracts/VerifiedContractsCounters';
import VerifiedContractsFilter from 'ui/verifiedContracts/VerifiedContractsFilter';
import VerifiedContractsList from 'ui/verifiedContracts/VerifiedContractsList';
import VerifiedContractsTable from 'ui/verifiedContracts/VerifiedContractsTable';

import PageDataListDisplay from './PageDataListDisplay';
import { PageFilterInput } from './PageFilterInput';

const sortCollection = createListCollection({
  items: SORT_OPTIONS,
});

const VerifiedContracts = () => {
  const isMobile = useIsMobile();

  const { query, type, searchTerm, debouncedSearchTerm, sort, onSearchTermChange, onTypeChange, onSortChange } = useVerifiedContractsQuery();
  const { isError, isPlaceholderData, data, pagination } = query;

  const typeFilter = (
    <VerifiedContractsFilter
      onChange={ onTypeChange }
      defaultValue={ type }
      hasActiveFilter={ Boolean(type) }
    />
  );

  const filterInput = (
    <PageFilterInput
      w={{ base: '100%', lg: 'var(--chakra-spacing-80)' }}
      size="sm"
      onChange={ onSearchTermChange }
      placeholder="Search by contract name or address"
      initialValue={ searchTerm }
    />
  );

  const sortButton = (
    <Sort
      name="verified_contracts_sorting"
      defaultValue={ [ sort ] }
      collection={ sortCollection }
      onValueChange={ onSortChange }
      isLoading={ isPlaceholderData }
    />
  );

  const actionBar = (
    <>
      <HStack gap={ 3 } mb={ 6 } display={{ base: 'flex', lg: 'none' }} flexWrap="wrap" minW={ 0 }>
        { typeFilter }
        { sortButton }
        { filterInput }
      </HStack>
      { (!isMobile || pagination.isVisible) && (
        <ActionBar w="100%" mx={ 0 } px={ 0 } mt={ -6 }>
          <HStack gap={ 3 } display={{ base: 'none', lg: 'flex' }} flexWrap="wrap" minW={ 0 }>
            { typeFilter }
            { filterInput }
          </HStack>
          <Pagination ml="auto" { ...pagination }/>
        </ActionBar>
      ) }
    </>
  );

  const content = data?.items ? (
    <>
      <Box hideFrom="lg">
        <VerifiedContractsList data={ data.items } isLoading={ isPlaceholderData }/>
      </Box>
      <Box hideBelow="lg">
        <VerifiedContractsTable data={ data.items } sort={ sort } setSorting={ onSortChange } isLoading={ isPlaceholderData }/>
      </Box>
    </>
  ) : null;

  return (
    <Box>
      <PageTitle
        title={ config.meta.seo.enhancedDataEnabled ? `Verified ${ config.chain.name } contracts` : 'Verified contracts' }
        withTextAd
      />
      <VerifiedContractsCounters/>
      <PageDataListDisplay
        isError={ isError }
        itemsNum={ data?.items.length }
        emptyText="There are no verified contracts."
        hasActiveFilters={ Boolean(debouncedSearchTerm || type) }
        emptyStateProps={{
          term: 'contract',
        }}
        actionBar={ actionBar }
      >
        { content }
      </PageDataListDisplay>
    </Box>
  );
};

export default VerifiedContracts;
