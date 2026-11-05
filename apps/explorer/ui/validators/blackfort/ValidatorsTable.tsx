import React from 'react';

import type {
  ValidatorBlackfort,
  ValidatorsBlackfortSortingField,
  ValidatorsBlackfortSortingValue,
} from 'types/api/validators';

import { TableBody, TableColumnHeader, TableColumnHeaderSortable, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import getNextSortValue from 'ui/shared/sort/getNextSortValue';

import { VALIDATORS_BLACKFORT_SORT_SEQUENCE } from './utils';
import ValidatorsTableItem from './ValidatorsTableItem';
import { hasValidatorField } from '../columns';

interface Props {
  data: Array<ValidatorBlackfort>;
  sort: ValidatorsBlackfortSortingValue;
  setSorting: ({ value }: { value: Array<string> }) => void;
  isLoading?: boolean;
  top: number;
}

const ValidatorsTable = ({ data, sort, setSorting, isLoading, top }: Props) => {

  const onSortToggle = React.useCallback((field: ValidatorsBlackfortSortingField) => {
    const value = getNextSortValue<ValidatorsBlackfortSortingField, ValidatorsBlackfortSortingValue>(VALIDATORS_BLACKFORT_SORT_SEQUENCE, field)(sort);
    setSorting({ value: [ value ] });
  }, [ sort, setSorting ]);

  const columns = {
    stake: hasValidatorField(data, 'self_bonded_amount'),
    delegated: hasValidatorField(data, 'delegated_amount'),
    commission: hasValidatorField(data, 'commission'),
  };

  return (
    <TableRoot variant="scan" css={{ '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'canvas' } }}>
      <TableHeaderSticky top={ top } bg="canvas">
        <TableRow>
          <TableColumnHeaderSortable
            sortField="address_hash"
            sortValue={ sort }
            onSortToggle={ onSortToggle }
            indicatorPosition="right"
          >
            Validator
          </TableColumnHeaderSortable>
          { columns.stake && <TableColumnHeader isNumeric title="Self-bonded amount">Stake</TableColumnHeader> }
          { columns.delegated && <TableColumnHeader isNumeric>Delegated amount</TableColumnHeader> }
          { columns.commission && <TableColumnHeader isNumeric>Commission</TableColumnHeader> }
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { data.map((item, index) => (
          <ValidatorsTableItem
            key={ item.address.hash + (isLoading ? index : '') }
            data={ item }
            columns={ columns }
            isLoading={ isLoading }/>
        )) }
      </TableBody>
    </TableRoot>
  );
};

export default React.memo(ValidatorsTable);
