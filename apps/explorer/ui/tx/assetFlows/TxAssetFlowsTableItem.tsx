import React from 'react';

import { TableRow, TableCell } from 'toolkit/chakra/table';
import NovesFromTo from 'ui/shared/Noves/NovesFromTo';

import NovesActionSnippet from './components/NovesActionSnippet';
import type { NovesFlowViewItem } from './utils/generateFlowViewData';

type Props = {
  isPlaceholderData: boolean;
  item: NovesFlowViewItem;
};

const TxAssetFlowsTableItem = (props: Props) => {

  return (
    <TableRow>
      <TableCell px={ 3 } py={ 5 } textStyle="sm" borderColor="border.divider">
        <NovesActionSnippet item={ props.item } isLoaded={ !props.isPlaceholderData }/>
      </TableCell>
      <TableCell px={ 3 } py="var(--chakra-spacing-4\.5)" textStyle="sm" borderColor="border.divider">
        <NovesFromTo item={ props.item } isLoaded={ !props.isPlaceholderData }/>
      </TableCell>
    </TableRow>
  );
};

export default React.memo(TxAssetFlowsTableItem);
