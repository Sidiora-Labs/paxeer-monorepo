import React from 'react';

import type { AdvancedFilterParams, AdvancedFiltersSearchParams } from 'types/api/advancedFilter';
import type { ClusterChainConfig } from 'types/multichain';

import useApiQuery from 'lib/api/useApiQuery';

import type { ColumnsIds } from './constants';
import FilterByColumn from './FilterByColumn';

type Props = {
  column: ColumnsIds;
  initialFilters?: AdvancedFilterParams;
  searchParams?: AdvancedFiltersSearchParams;
  chain?: ClusterChainConfig;
};

export function AdvancedFilterInteractionFixture({ column, initialFilters = {}, searchParams, chain }: Props) {
  const [ filters, setFilters ] = React.useState(initialFilters);
  const handleFilterChange = React.useCallback(<T extends keyof AdvancedFilterParams>(field: T, value: AdvancedFilterParams[T]) => {
    setFilters(previous => ({ ...previous, [field]: value }));
  }, []);
  const result = useApiQuery('general:advanced_filter', { queryParams: filters, chain });

  return (
    <>
      <FilterByColumn
        column={ column }
        columnName={ column }
        filters={ filters }
        searchParams={ searchParams }
        handleFilterChange={ handleFilterChange }
      />
      <output data-testid="applied-filters">{ JSON.stringify(filters) }</output>
      <output data-testid="filter-response">{ JSON.stringify(result.data) }</output>
    </>
  );
}
