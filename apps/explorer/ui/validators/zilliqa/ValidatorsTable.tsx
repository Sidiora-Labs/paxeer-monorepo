import React from 'react';

import type { ValidatorsZilliqaItem } from 'types/api/validators';

import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';

import ValidatorsTableItem from './ValidatorsTableItem';
import { hasValidatorField } from '../columns';

interface Props {
  data: Array<ValidatorsZilliqaItem>;
  isLoading?: boolean;
  top?: number;
}

const ValidatorsTable = ({ data, isLoading, top }: Props) => {
  const columns = {
    index: hasValidatorField(data, 'index'),
    stake: hasValidatorField(data, 'balance'),
  };

  return (
    <TableRoot variant="scan" css={{ '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'canvas' } }}>
      <TableHeaderSticky top={ top } bg="canvas">
        <TableRow>
          <TableColumnHeader width="50%">Validator</TableColumnHeader>
          { columns.index && <TableColumnHeader width="15%" isNumeric>Index</TableColumnHeader> }
          { columns.stake && <TableColumnHeader width="35%" isNumeric>Stake</TableColumnHeader> }
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { data.map((item, index) => (
          <ValidatorsTableItem
            key={ item.bls_public_key + (isLoading ? index : '') }
            data={ item }
            columns={ columns }
            isLoading={ isLoading }/>
        )) }
      </TableBody>
    </TableRoot>
  );
};

export default React.memo(ValidatorsTable);
