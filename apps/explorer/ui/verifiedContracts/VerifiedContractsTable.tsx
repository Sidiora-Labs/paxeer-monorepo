import React from 'react';

import type { VerifiedContract } from 'types/api/contracts';
import type { VerifiedContractsSortingField, VerifiedContractsSortingValue } from 'types/api/verifiedContracts';

import { useMultichainContext } from 'lib/contexts/multichain';
import { currencyUnits } from 'lib/units';
import { TableBody, TableColumnHeader, TableColumnHeaderSortable, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import { ACTION_BAR_HEIGHT_DESKTOP } from 'ui/shared/ActionBar';
import getNextSortValue from 'ui/shared/sort/getNextSortValue';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import { ResponsiveTableRegion } from 'ui/token/ResponsiveTableRegion';
import { SORT_SEQUENCE } from 'ui/verifiedContracts/utils';

import VerifiedContractsTableItem from './VerifiedContractsTableItem';

interface Props {
  data: Array<VerifiedContract>;
  sort: VerifiedContractsSortingValue;
  setSorting: ({ value }: { value: Array<string> }) => void;
  isLoading?: boolean;
}

const VerifiedContractsTable = ({ data, sort, setSorting, isLoading }: Props) => {
  const multichainContext = useMultichainContext();
  const chainData = multichainContext?.chain;

  const onSortToggle = React.useCallback((field: VerifiedContractsSortingField) => {
    const value = getNextSortValue<VerifiedContractsSortingField, VerifiedContractsSortingValue>(SORT_SEQUENCE, field)(sort);
    setSorting({ value: [ value ] });
  }, [ sort, setSorting ]);

  return (
    <ResponsiveTableRegion label="Verified contracts" minWidth="6xl">
      <TableRoot minW="6xl">
        <TableHeaderSticky top={ ACTION_BAR_HEIGHT_DESKTOP }>
          <TableRow>
            { chainData && <TableColumnHeader width="9"/> }
            <TableColumnHeader width="50%">Contract</TableColumnHeader>
            <TableColumnHeaderSortable
              width="32"
              isNumeric
              sortField="balance"
              sortValue={ sort }
              onSortToggle={ onSortToggle }
              disabled={ isLoading }
            >
              Balance { currencyUnits.ether }
            </TableColumnHeaderSortable>
            <TableColumnHeaderSortable
              width="32"
              isNumeric
              sortField="transactions_count"
              sortValue={ sort }
              onSortToggle={ onSortToggle }
              disabled={ isLoading }
            >
              Txs
            </TableColumnHeaderSortable>
            <TableColumnHeader width="50%">Language / Compiler version</TableColumnHeader>
            <TableColumnHeader width="20">Settings</TableColumnHeader>
            <TableColumnHeader width="48">
              Verified
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader width="32">License</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { data.map((item, index) => (
            <VerifiedContractsTableItem
              key={ item.address.hash + (isLoading ? index : '') }
              data={ item }
              isLoading={ isLoading }
              chainData={ chainData }
            />
          )) }
        </TableBody>
      </TableRoot>
    </ResponsiveTableRegion>
  );
};

export default React.memo(VerifiedContractsTable);
