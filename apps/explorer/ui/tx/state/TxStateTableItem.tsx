import { Box } from '@chakra-ui/react';
import React from 'react';

import type { TxStateChange } from 'types/api/txStateChanges';

import { TableCell, TableRow } from 'toolkit/chakra/table';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';

import { getStateElements } from './utils';

interface Props {
  data: TxStateChange;
  isLoading?: boolean;
}

const TxStateTableItem = ({ data, isLoading }: Props) => {
  const { before, after, change, tag, tokenId } = getStateElements(data, isLoading);

  return (
    <TableRow>
      <TableCell>
        <Box py="var(--chakra-spacing-0\.5)">
          { tag }
        </Box>
      </TableCell>
      <TableCell>
        <AddressEntity
          address={ data.address }
          isLoading={ isLoading }
          truncation="constant"
          my="var(--chakra-spacing-1\.5)"
        />
      </TableCell>
      <TableCell isNumeric><Box py="var(--chakra-spacing-1\.5)">{ before }</Box></TableCell>
      <TableCell isNumeric><Box py="var(--chakra-spacing-1\.5)">{ after }</Box></TableCell>
      <TableCell isNumeric><Box py="var(--chakra-spacing-1\.5)">{ change }</Box></TableCell>
      <TableCell>{ tokenId }</TableCell>
    </TableRow>
  );
};

export default React.memo(TxStateTableItem);
